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
import {
	applyNavigationPhaseLifecycleTransition,
	applyNavigationRemovalLifecycleTransition,
	buildClearAllLifecycleTransitionEvent,
	buildNavigationBeginArbitratedLifecycleTransitionEvent,
	buildSubmissionStateLifecycleTransitionEvent,
} from "./runtime_lifecycle_transitions.ts";
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
		const causedByOperationID = deleteProps.causedByOperationID ?? null;
		const { deleted, transitionEvent } =
			applyNavigationRemovalLifecycleTransition({
				lanes: props.lanes,
				targetUrl: deleteProps.key,
				scheduleStatusUpdate: props.getScheduleStatusUpdate(),
				reason: deleteProps.reason,
				causedByOperationID,
			});
		dispatchRuntimeTransitionEvent(transitionEvent);
		return deleted;
	};

	const transitionPhase = (transitionProps: {
		targetUrl: string;
		phase: NavigationPhase;
		reason: string;
	}): void => {
		const transitionEvent = applyNavigationPhaseLifecycleTransition({
			lanes: props.lanes,
			targetUrl: transitionProps.targetUrl,
			phase: transitionProps.phase,
			scheduleStatusUpdate: props.getScheduleStatusUpdate(),
			reason: transitionProps.reason,
		});
		dispatchRuntimeTransitionEvent(transitionEvent);
	};

	const dispatchBeginNavigationArbitrated = (beginProps: {
		navigationType: VormaNavigationType;
		targetUrl: string;
		beforeEntriesByOperationID: Map<number, NavigationEntry>;
	}): void => {
		dispatchRuntimeTransitionEvent(
			buildNavigationBeginArbitratedLifecycleTransitionEvent({
				navigationType: beginProps.navigationType,
				targetUrl: beginProps.targetUrl,
				beforeEntriesByOperationID:
					beginProps.beforeEntriesByOperationID,
				lanes: props.lanes,
				winnerEntry: findNavigationEntry(beginProps.targetUrl),
			}),
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
			buildSubmissionStateLifecycleTransitionEvent({
				submissionEntry: submissionProps.submissionEntry,
				targetUrl: submissionProps.targetUrl,
				fromState: submissionProps.fromState,
				toState: submissionProps.toState,
				reason: submissionProps.reason,
				causedByOperationID: submissionProps.causedByOperationID,
			}),
		);
	};

	const dispatchNavigationFailure = (failureProps: {
		targetUrl: string;
		entry: NavigationEntry | undefined;
		reason: string;
	}): void => {
		dispatchRuntimeTransitionEvent({
			type: "navigation_failed",
			targetUrl: failureProps.targetUrl,
			entry: failureProps.entry,
			reason: failureProps.reason,
		});
	};

	const dispatchClearAll = (clearProps: {
		navigationEntries: NavigationEntry[];
		submissionEntries: SubmissionEntry[];
		targetUrl: string;
	}): void => {
		dispatchRuntimeTransitionEvent(
			buildClearAllLifecycleTransitionEvent({
				navigationEntries: clearProps.navigationEntries,
				submissionEntries: clearProps.submissionEntries,
				targetUrl: clearProps.targetUrl,
			}),
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
