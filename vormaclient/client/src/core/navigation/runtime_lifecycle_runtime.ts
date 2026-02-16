import type {
	NavigationDebugJournalEntry,
	NavigationEntry,
	NavigationPhase,
	SubmissionEntry,
	VormaNavigationType,
} from "./types.ts";
import type { NavigationLanes } from "./runtime_slots.ts";
import {
	buildNavigationEntriesByOperationIDFromNavigationLanes,
	createNavigationRuntimeStateMachine,
} from "./runtime_state_machine.ts";
import { reduceNavigationLifecycleTransition } from "./runtime_lifecycle_transitions.ts";
import { findNavigationEntryInNavigationLanes } from "./runtime_slots.ts";

export type NavigationLifecycleRuntime = {
	findNavigationEntry: (targetUrl: string) => NavigationEntry | undefined;
	deleteNavigation: (props: {
		key: string;
		reason: string;
		causedByOperationID?: number | null;
	}) => boolean;
	transitionPhase: (props: {
		targetUrl: string;
		phase: NavigationPhase;
		reason: string;
	}) => void;
	dispatchBeginNavigationArbitrated: (props: {
		navigationType: VormaNavigationType;
		targetUrl: string;
		beforeEntriesByOperationID: Map<number, NavigationEntry>;
	}) => void;
	dispatchSubmissionStateTransition: (props: {
		submissionEntry: SubmissionEntry;
		targetUrl: string;
		fromState: string;
		toState: string;
		reason: string;
		causedByOperationID?: number | null;
	}) => void;
	dispatchNavigationFailure: (props: {
		targetUrl: string;
		entry: NavigationEntry | undefined;
		reason: string;
	}) => void;
	dispatchClearAll: (props: {
		navigationEntries: NavigationEntry[];
		submissionEntries: SubmissionEntry[];
		targetUrl: string;
	}) => void;
	getDebugJournal: () => ReadonlyArray<NavigationDebugJournalEntry>;
	clearDebugJournal: () => void;
};

export function createNavigationLifecycleRuntime(props: {
	lanes: NavigationLanes;
	getScheduleStatusUpdate: () => () => void;
}): NavigationLifecycleRuntime {
	const runtimeStateMachine = createNavigationRuntimeStateMachine();

	const findNavigationEntry = (
		targetUrl: string,
	): NavigationEntry | undefined =>
		findNavigationEntryInNavigationLanes({
			lanes: props.lanes,
			targetUrl,
		});

	const dispatchRuntimeTransitionEvent = (
		transitionEvent:
			| Parameters<typeof runtimeStateMachine.dispatchTransitionEvent>[0]
			| null,
	): void => {
		if (!transitionEvent) {
			return;
		}
		runtimeStateMachine.dispatchTransitionEvent(transitionEvent);
	};

	const deleteNavigation = (deleteProps: {
		key: string;
		reason: string;
		causedByOperationID?: number | null;
	}): boolean => {
		const reductionResult = reduceNavigationLifecycleTransition({
			lanes: props.lanes,
			scheduleStatusUpdate: props.getScheduleStatusUpdate(),
			action: {
				type: "remove_navigation",
				targetUrl: deleteProps.key,
				reason: deleteProps.reason,
				causedByOperationID: deleteProps.causedByOperationID,
			},
		});
		dispatchRuntimeTransitionEvent(reductionResult.transitionEvent);
		return reductionResult.deleted ?? false;
	};

	const transitionPhase = (transitionProps: {
		targetUrl: string;
		phase: NavigationPhase;
		reason: string;
	}): void => {
		const reductionResult = reduceNavigationLifecycleTransition({
			lanes: props.lanes,
			scheduleStatusUpdate: props.getScheduleStatusUpdate(),
			action: {
				type: "transition_navigation_phase",
				targetUrl: transitionProps.targetUrl,
				phase: transitionProps.phase,
				reason: transitionProps.reason,
			},
		});
		dispatchRuntimeTransitionEvent(reductionResult.transitionEvent);
	};

	const dispatchBeginNavigationArbitrated = (beginProps: {
		navigationType: VormaNavigationType;
		targetUrl: string;
		beforeEntriesByOperationID: Map<number, NavigationEntry>;
	}): void => {
		dispatchRuntimeTransitionEvent(
			reduceNavigationLifecycleTransition({
				lanes: props.lanes,
				scheduleStatusUpdate: props.getScheduleStatusUpdate(),
				action: {
					type: "begin_navigation_arbitrated",
					navigationType: beginProps.navigationType,
					targetUrl: beginProps.targetUrl,
					beforeEntriesByOperationID:
						beginProps.beforeEntriesByOperationID,
					winnerEntry: findNavigationEntry(beginProps.targetUrl),
				},
			}).transitionEvent,
		);
	};

	const dispatchSubmissionStateTransition = (submissionProps: {
		submissionEntry: SubmissionEntry;
		targetUrl: string;
		fromState: string;
		toState: string;
		reason: string;
		causedByOperationID?: number | null;
	}): void => {
		dispatchRuntimeTransitionEvent(
			reduceNavigationLifecycleTransition({
				lanes: props.lanes,
				scheduleStatusUpdate: props.getScheduleStatusUpdate(),
				action: {
					type: "submission_state_transition",
					submissionEntry: submissionProps.submissionEntry,
					targetUrl: submissionProps.targetUrl,
					fromState: submissionProps.fromState,
					toState: submissionProps.toState,
					reason: submissionProps.reason,
					causedByOperationID: submissionProps.causedByOperationID,
				},
			}).transitionEvent,
		);
	};

	const dispatchNavigationFailure = (failureProps: {
		targetUrl: string;
		entry: NavigationEntry | undefined;
		reason: string;
	}): void => {
		dispatchRuntimeTransitionEvent(
			reduceNavigationLifecycleTransition({
				lanes: props.lanes,
				scheduleStatusUpdate: props.getScheduleStatusUpdate(),
				action: {
					type: "navigation_failed",
					targetUrl: failureProps.targetUrl,
					entry: failureProps.entry,
					reason: failureProps.reason,
				},
			}).transitionEvent,
		);
	};

	const dispatchClearAll = (clearProps: {
		navigationEntries: NavigationEntry[];
		submissionEntries: SubmissionEntry[];
		targetUrl: string;
	}): void => {
		dispatchRuntimeTransitionEvent(
			reduceNavigationLifecycleTransition({
				lanes: props.lanes,
				scheduleStatusUpdate: props.getScheduleStatusUpdate(),
				action: {
					type: "clear_all",
					navigationEntries: clearProps.navigationEntries,
					submissionEntries: clearProps.submissionEntries,
					targetUrl: clearProps.targetUrl,
				},
			}).transitionEvent,
		);
	};

	return {
		findNavigationEntry,
		deleteNavigation,
		transitionPhase,
		dispatchBeginNavigationArbitrated,
		dispatchSubmissionStateTransition,
		dispatchNavigationFailure,
		dispatchClearAll,
		getDebugJournal: runtimeStateMachine.getDebugJournal,
		clearDebugJournal: runtimeStateMachine.clearDebugJournal,
	};
}

export function buildNavigationEntriesBeforeClearAll(props: {
	lanes: NavigationLanes;
}): NavigationEntry[] {
	return [
		...buildNavigationEntriesByOperationIDFromNavigationLanes({
			lanes: props.lanes,
		}).values(),
	];
}
