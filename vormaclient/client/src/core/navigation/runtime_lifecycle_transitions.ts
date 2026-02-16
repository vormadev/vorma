import {
	deleteNavigationFromNavigationLanes,
	findNavigationEntryInNavigationLanes,
	transitionNavigationPhaseInNavigationLanes,
	type NavigationLanes,
} from "./runtime_slots.ts";
import {
	buildNavigationEntriesByOperationIDFromNavigationLanes,
	type RuntimeTransitionEvent,
} from "./runtime_state_machine.ts";
import type {
	NavigateProps,
	NavigationEntry,
	NavigationPhase,
	SubmissionEntry,
} from "./types.ts";

export type NavigationLifecycleTransitionAction =
	| {
			type: "remove_navigation";
			targetUrl: string;
			reason: string;
			causedByOperationID?: number | null;
	  }
	| {
			type: "transition_navigation_phase";
			targetUrl: string;
			phase: NavigationPhase;
			reason: string;
	  }
	| {
			type: "begin_navigation_arbitrated";
			navigationType: NavigateProps["navigationType"];
			targetUrl: string;
			beforeEntriesByOperationID: Map<number, NavigationEntry>;
			winnerEntry: NavigationEntry | undefined;
	  }
	| {
			type: "submission_state_transition";
			submissionEntry: SubmissionEntry;
			targetUrl: string;
			fromState: string;
			toState: string;
			reason: string;
			causedByOperationID?: number | null;
	  }
	| {
			type: "navigation_failed";
			targetUrl: string;
			entry: NavigationEntry | undefined;
			reason: string;
	  }
	| {
			type: "clear_all";
			navigationEntries: NavigationEntry[];
			submissionEntries: SubmissionEntry[];
			targetUrl: string;
	  };

export type NavigationLifecycleTransitionReductionResult = {
	transitionEvent: RuntimeTransitionEvent | null;
	deleted: boolean | null;
};

export function applyNavigationRemovalLifecycleTransition(props: {
	lanes: NavigationLanes;
	targetUrl: string;
	scheduleStatusUpdate: () => void;
	reason: string;
	causedByOperationID?: number | null;
}): {
	deleted: boolean;
	transitionEvent: RuntimeTransitionEvent | null;
} {
	const {
		lanes,
		targetUrl,
		scheduleStatusUpdate,
		reason,
		causedByOperationID,
	} = props;
	const entry = findNavigationEntryInNavigationLanes({
		lanes,
		targetUrl,
	});
	const deleted = deleteNavigationFromNavigationLanes({
		lanes,
		targetUrl,
		onStatusRelevantChange: scheduleStatusUpdate,
	});

	if (!deleted || !entry) {
		return {
			deleted,
			transitionEvent: null,
		};
	}

	return {
		deleted,
		transitionEvent: {
			type: "navigation_removed",
			entry,
			reason,
			causedByOperationID: causedByOperationID ?? null,
		},
	};
}

export function applyNavigationPhaseLifecycleTransition(props: {
	lanes: NavigationLanes;
	targetUrl: string;
	phase: NavigationPhase;
	scheduleStatusUpdate: () => void;
	reason: string;
}): RuntimeTransitionEvent | null {
	const { lanes, targetUrl, phase, scheduleStatusUpdate, reason } = props;
	const entry = findNavigationEntryInNavigationLanes({
		lanes,
		targetUrl,
	});
	const previousPhase = entry?.phase;

	transitionNavigationPhaseInNavigationLanes({
		lanes,
		targetUrl,
		phase,
		onStatusRelevantChange: scheduleStatusUpdate,
	});

	const currentEntry = findNavigationEntryInNavigationLanes({
		lanes,
		targetUrl,
	});
	if (
		!entry ||
		currentEntry !== entry ||
		!previousPhase ||
		previousPhase === phase
	) {
		return null;
	}

	return {
		type: "navigation_phase_transitioned",
		entry,
		fromPhase: previousPhase,
		toPhase: phase,
		reason,
	};
}

export function buildNavigationBeginArbitratedLifecycleTransitionEvent(props: {
	navigationType: NavigateProps["navigationType"];
	targetUrl: string;
	beforeEntriesByOperationID: Map<number, NavigationEntry>;
	lanes: NavigationLanes;
	winnerEntry: NavigationEntry | undefined;
}): RuntimeTransitionEvent {
	return {
		type: "navigation_begin_arbitrated",
		navigationType: props.navigationType,
		targetUrl: props.targetUrl,
		beforeEntriesByOperationID: props.beforeEntriesByOperationID,
		afterEntriesByOperationID:
			buildNavigationEntriesByOperationIDFromNavigationLanes({
				lanes: props.lanes,
			}),
		winnerEntry: props.winnerEntry,
	};
}

export function buildSubmissionStateLifecycleTransitionEvent(props: {
	submissionEntry: SubmissionEntry;
	targetUrl: string;
	fromState: string;
	toState: string;
	reason: string;
	causedByOperationID?: number | null;
}): RuntimeTransitionEvent {
	return {
		type: "submission_state_transitioned",
		submissionEntry: props.submissionEntry,
		targetUrl: props.targetUrl,
		fromState: props.fromState,
		toState: props.toState,
		reason: props.reason,
		causedByOperationID: props.causedByOperationID ?? null,
	};
}

export function buildClearAllLifecycleTransitionEvent(props: {
	navigationEntries: NavigationEntry[];
	submissionEntries: SubmissionEntry[];
	targetUrl: string;
}): RuntimeTransitionEvent {
	return {
		type: "clear_all",
		navigationEntries: props.navigationEntries,
		submissionEntries: props.submissionEntries,
		targetUrl: props.targetUrl,
	};
}

export function reduceNavigationLifecycleTransition(props: {
	lanes: NavigationLanes;
	scheduleStatusUpdate: () => void;
	action: NavigationLifecycleTransitionAction;
}): NavigationLifecycleTransitionReductionResult {
	const { lanes, scheduleStatusUpdate, action } = props;

	switch (action.type) {
		case "remove_navigation": {
			const removalTransitionResult =
				applyNavigationRemovalLifecycleTransition({
					lanes,
					targetUrl: action.targetUrl,
					scheduleStatusUpdate,
					reason: action.reason,
					causedByOperationID: action.causedByOperationID,
				});
			return {
				transitionEvent: removalTransitionResult.transitionEvent,
				deleted: removalTransitionResult.deleted,
			};
		}
		case "transition_navigation_phase":
			return {
				transitionEvent: applyNavigationPhaseLifecycleTransition({
					lanes,
					targetUrl: action.targetUrl,
					phase: action.phase,
					scheduleStatusUpdate,
					reason: action.reason,
				}),
				deleted: null,
			};
		case "begin_navigation_arbitrated":
			return {
				transitionEvent:
					buildNavigationBeginArbitratedLifecycleTransitionEvent({
						navigationType: action.navigationType,
						targetUrl: action.targetUrl,
						beforeEntriesByOperationID:
							action.beforeEntriesByOperationID,
						lanes,
						winnerEntry: action.winnerEntry,
					}),
				deleted: null,
			};
		case "submission_state_transition":
			return {
				transitionEvent: buildSubmissionStateLifecycleTransitionEvent({
					submissionEntry: action.submissionEntry,
					targetUrl: action.targetUrl,
					fromState: action.fromState,
					toState: action.toState,
					reason: action.reason,
					causedByOperationID: action.causedByOperationID,
				}),
				deleted: null,
			};
		case "navigation_failed":
			return {
				transitionEvent: {
					type: "navigation_failed",
					targetUrl: action.targetUrl,
					entry: action.entry,
					reason: action.reason,
				},
				deleted: null,
			};
		case "clear_all":
			return {
				transitionEvent: buildClearAllLifecycleTransitionEvent({
					navigationEntries: action.navigationEntries,
					submissionEntries: action.submissionEntries,
					targetUrl: action.targetUrl,
				}),
				deleted: null,
			};
	}
}
