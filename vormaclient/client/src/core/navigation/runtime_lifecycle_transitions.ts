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
