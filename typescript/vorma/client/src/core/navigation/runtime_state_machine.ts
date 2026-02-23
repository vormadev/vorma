import type { NavigationLanes } from "./runtime_slots.ts";
import type {
	NavigateProps,
	NavigationDebugJournalEntry,
	NavigationEntry,
	NavigationLane,
	NavigationPhase,
	SubmissionEntry,
} from "./types.ts";

const DEBUG_JOURNAL_CAPACITY = 200;

function resolveNavigationLaneFromEntryType(
	type: NavigationEntry["type"],
): NavigationLane {
	switch (type) {
		case "prefetch":
			return "prefetch";
		case "revalidation":
			return "revalidation";
		case "action":
		case "browserHistory":
		case "redirect":
		case "userNavigation":
		default:
			return "active";
	}
}

function appendDebugJournalEntriesWithCapacity(props: {
	debugJournal: NavigationDebugJournalEntry[];
	entries: Omit<NavigationDebugJournalEntry, "timestampMS">[];
}): void {
	const { debugJournal, entries } = props;
	const timestampMS = Date.now();

	for (const entry of entries) {
		debugJournal.push({
			timestampMS,
			...entry,
		});
	}

	if (debugJournal.length > DEBUG_JOURNAL_CAPACITY) {
		debugJournal.splice(0, debugJournal.length - DEBUG_JOURNAL_CAPACITY);
	}
}

function buildNavigationBeginArbitratedJournalEntries(props: {
	navigationType: NavigateProps["navigationType"];
	targetUrl: string;
	beforeEntriesByOperationID: Map<number, NavigationEntry>;
	afterEntriesByOperationID: Map<number, NavigationEntry>;
	winnerEntry: NavigationEntry | undefined;
}): Omit<NavigationDebugJournalEntry, "timestampMS">[] {
	const {
		navigationType,
		targetUrl,
		beforeEntriesByOperationID,
		afterEntriesByOperationID,
		winnerEntry,
	} = props;
	const winnerOperationID = winnerEntry?.operationID ?? null;
	const entries: Omit<NavigationDebugJournalEntry, "timestampMS">[] = [];

	for (const [operationID, beforeEntry] of beforeEntriesByOperationID) {
		if (!afterEntriesByOperationID.has(operationID)) {
			entries.push({
				operationID: beforeEntry.operationID,
				lane: resolveNavigationLaneFromEntryType(beforeEntry.type),
				targetUrl: beforeEntry.targetUrl,
				fromState: beforeEntry.phase,
				toState: "removed",
				reason: `superseded_by_${navigationType}`,
				causedByOperationID: winnerOperationID,
			});
		}
	}

	for (const [operationID, afterEntry] of afterEntriesByOperationID) {
		if (!beforeEntriesByOperationID.has(operationID)) {
			entries.push({
				operationID: afterEntry.operationID,
				lane: resolveNavigationLaneFromEntryType(afterEntry.type),
				targetUrl: afterEntry.targetUrl,
				fromState: "none",
				toState: afterEntry.phase,
				reason: `started_by_${navigationType}`,
				causedByOperationID: null,
			});
		}
	}

	if (
		winnerEntry &&
		beforeEntriesByOperationID.has(winnerEntry.operationID)
	) {
		entries.push({
			operationID: winnerEntry.operationID,
			lane: resolveNavigationLaneFromEntryType(winnerEntry.type),
			targetUrl: winnerEntry.targetUrl,
			fromState: winnerEntry.phase,
			toState: winnerEntry.phase,
			reason: `reused_by_${navigationType}`,
			causedByOperationID: null,
		});
	}

	if (!winnerEntry && navigationType === "prefetch") {
		entries.push({
			operationID: null,
			lane: "prefetch",
			targetUrl,
			fromState: "none",
			toState: "aborted",
			reason: "prefetch_aborted_immediately",
			causedByOperationID: null,
		});
	}

	return entries;
}

function buildClearAllJournalEntries(props: {
	navigationEntries: NavigationEntry[];
	submissionEntries: SubmissionEntry[];
	targetUrl: string;
}): Omit<NavigationDebugJournalEntry, "timestampMS">[] {
	const { navigationEntries, submissionEntries, targetUrl } = props;
	const entries: Omit<NavigationDebugJournalEntry, "timestampMS">[] = [];

	for (const entry of navigationEntries) {
		entries.push({
			operationID: entry.operationID,
			lane: resolveNavigationLaneFromEntryType(entry.type),
			targetUrl: entry.targetUrl,
			fromState: entry.phase,
			toState: "removed",
			reason: "clear_all",
			causedByOperationID: null,
		});
	}

	for (const submissionEntry of submissionEntries) {
		entries.push({
			operationID: submissionEntry.operationID,
			lane: "submission",
			targetUrl,
			fromState: "submitting",
			toState: "removed",
			reason: "clear_all",
			causedByOperationID: null,
		});
	}

	return entries;
}

export type RuntimeTransitionEvent =
	| {
			type: "navigation_begin_arbitrated";
			navigationType: NavigateProps["navigationType"];
			targetUrl: string;
			beforeEntriesByOperationID: Map<number, NavigationEntry>;
			afterEntriesByOperationID: Map<number, NavigationEntry>;
			winnerEntry: NavigationEntry | undefined;
	  }
	| {
			type: "navigation_phase_transitioned";
			entry: NavigationEntry;
			fromPhase: NavigationPhase;
			toPhase: NavigationPhase;
			reason: string;
	  }
	| {
			type: "navigation_removed";
			entry: NavigationEntry;
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
			type: "submission_state_transitioned";
			submissionEntry: SubmissionEntry;
			targetUrl: string;
			fromState: string;
			toState: string;
			reason: string;
			causedByOperationID?: number | null;
	  }
	| {
			type: "clear_all";
			navigationEntries: NavigationEntry[];
			submissionEntries: SubmissionEntry[];
			targetUrl: string;
	  };

function buildJournalEntriesFromRuntimeTransitionEvent(
	event: RuntimeTransitionEvent,
): Omit<NavigationDebugJournalEntry, "timestampMS">[] {
	switch (event.type) {
		case "navigation_begin_arbitrated":
			return buildNavigationBeginArbitratedJournalEntries({
				navigationType: event.navigationType,
				targetUrl: event.targetUrl,
				beforeEntriesByOperationID: event.beforeEntriesByOperationID,
				afterEntriesByOperationID: event.afterEntriesByOperationID,
				winnerEntry: event.winnerEntry,
			});
		case "navigation_phase_transitioned":
			return [
				{
					operationID: event.entry.operationID,
					lane: resolveNavigationLaneFromEntryType(event.entry.type),
					targetUrl: event.entry.targetUrl,
					fromState: event.fromPhase,
					toState: event.toPhase,
					reason: event.reason,
					causedByOperationID: null,
				},
			];
		case "navigation_removed":
			return [
				{
					operationID: event.entry.operationID,
					lane: resolveNavigationLaneFromEntryType(event.entry.type),
					targetUrl: event.entry.targetUrl,
					fromState: event.entry.phase,
					toState: "removed",
					reason: event.reason,
					causedByOperationID: event.causedByOperationID ?? null,
				},
			];
		case "navigation_failed":
			return [
				{
					operationID: event.entry?.operationID ?? null,
					lane: event.entry
						? resolveNavigationLaneFromEntryType(event.entry.type)
						: "active",
					targetUrl: event.entry?.targetUrl ?? event.targetUrl,
					fromState: event.entry?.phase ?? "none",
					toState: "failed",
					reason: event.reason,
					causedByOperationID: null,
				},
			];
		case "submission_state_transitioned":
			return [
				{
					operationID: event.submissionEntry.operationID,
					lane: "submission",
					targetUrl: event.targetUrl,
					fromState: event.fromState,
					toState: event.toState,
					reason: event.reason,
					causedByOperationID: event.causedByOperationID ?? null,
				},
			];
		case "clear_all":
			return buildClearAllJournalEntries({
				navigationEntries: event.navigationEntries,
				submissionEntries: event.submissionEntries,
				targetUrl: event.targetUrl,
			});
	}
}

export function buildNavigationEntriesByOperationIDFromNavigationLanes(props: {
	lanes: NavigationLanes;
}): Map<number, NavigationEntry> {
	const { lanes } = props;
	const entriesByOperationID = new Map<number, NavigationEntry>();

	if (lanes.active) {
		entriesByOperationID.set(lanes.active.operationID, lanes.active);
	}

	for (const prefetchEntry of lanes.prefetch.values()) {
		entriesByOperationID.set(prefetchEntry.operationID, prefetchEntry);
	}

	if (lanes.revalidation) {
		entriesByOperationID.set(
			lanes.revalidation.operationID,
			lanes.revalidation,
		);
	}

	return entriesByOperationID;
}

export type NavigationRuntimeStateMachine = {
	dispatchTransitionEvent: (event: RuntimeTransitionEvent) => void;
	getDebugJournal: () => ReadonlyArray<NavigationDebugJournalEntry>;
	clearDebugJournal: () => void;
};

function createNavigationRuntimeStateMachineWithoutDebugJournal(): NavigationRuntimeStateMachine {
	return {
		dispatchTransitionEvent: () => {},
		getDebugJournal: () => [],
		clearDebugJournal: () => {},
	};
}

function createNavigationRuntimeStateMachineWithDebugJournal(): NavigationRuntimeStateMachine {
	const debugJournal: NavigationDebugJournalEntry[] = [];

	return {
		dispatchTransitionEvent: (event) => {
			appendDebugJournalEntriesWithCapacity({
				debugJournal,
				entries: buildJournalEntriesFromRuntimeTransitionEvent(event),
			});
		},
		getDebugJournal: () => [...debugJournal],
		clearDebugJournal: () => {
			debugJournal.length = 0;
		},
	};
}

export const createNavigationRuntimeStateMachine: () => NavigationRuntimeStateMachine =
	import.meta.env.DEV
		? createNavigationRuntimeStateMachineWithDebugJournal
		: createNavigationRuntimeStateMachineWithoutDebugJournal;
