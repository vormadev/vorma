export type RevalidationTriggerTimestampState = {
	lastTriggeredNavOrRevalidateTimestampMS: number;
};

export type RevalidationTriggerTimestampEvent = {
	type: "navigation_or_revalidation_intent_committed";
	committedTimestampMS: number;
};

export function reduceRevalidationTriggerTimestampState(props: {
	state: RevalidationTriggerTimestampState;
	event: RevalidationTriggerTimestampEvent;
}): RevalidationTriggerTimestampState {
	switch (props.event.type) {
		case "navigation_or_revalidation_intent_committed":
			return {
				...props.state,
				lastTriggeredNavOrRevalidateTimestampMS:
					props.event.committedTimestampMS,
			};
	}
}

export type RevalidationTriggerTimestampRuntime = {
	recordNavigationOrRevalidationIntentCommitted: () => void;
	getLastTriggeredNavOrRevalidateTimestampMS: () => number;
};

export function createRevalidationTriggerTimestampRuntime(props?: {
	getNowTimestampMS?: () => number;
	initialState?: RevalidationTriggerTimestampState;
}): RevalidationTriggerTimestampRuntime {
	const getNowTimestampMS = props?.getNowTimestampMS ?? (() => Date.now());
	let state: RevalidationTriggerTimestampState = props?.initialState ?? {
		lastTriggeredNavOrRevalidateTimestampMS: getNowTimestampMS(),
	};

	return {
		recordNavigationOrRevalidationIntentCommitted: () => {
			state = reduceRevalidationTriggerTimestampState({
				state,
				event: {
					type: "navigation_or_revalidation_intent_committed",
					committedTimestampMS: getNowTimestampMS(),
				},
			});
		},
		getLastTriggeredNavOrRevalidateTimestampMS: () =>
			state.lastTriggeredNavOrRevalidateTimestampMS,
	};
}
