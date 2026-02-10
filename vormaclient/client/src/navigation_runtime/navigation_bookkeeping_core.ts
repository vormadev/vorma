import type {
	NavigationEntry,
	NavigationPhase,
	SubmissionEntry,
} from "./types.ts";
import {
	buildNavigationsMapFromSlots,
	findNavigationEntryInSlots,
	getNavigationsSizeFromSlots,
	type NavigationSlots,
} from "./navigation_slots.ts";
import {
	clearSlotsAndSubmissions,
	deleteNavigationFromSlots,
	transitionNavigationPhaseInSlots,
} from "./navigation_slot_mutations.ts";

export type NavigationBookkeepingCore = {
	findNavigationEntry: (targetUrl: string) => NavigationEntry | undefined;
	deleteNavigation: (key: string) => boolean;
	removeNavigation: (key: string) => void;
	getNavigation: (key: string) => NavigationEntry | undefined;
	hasNavigation: (key: string) => boolean;
	getNavigationsSize: () => number;
	getNavigations: () => Map<string, NavigationEntry>;
	transitionPhase: (targetUrl: string, phase: NavigationPhase) => void;
	clearNavigationsAndSubmissions: (
		submissions: Map<string | symbol, SubmissionEntry>,
	) => void;
};

export type CreateNavigationBookkeepingCoreOptions = {
	slots: NavigationSlots;
	onStatusRelevantChange: () => void;
};

export function createNavigationBookkeepingCore(
	options: CreateNavigationBookkeepingCoreOptions,
): NavigationBookkeepingCore {
	const { slots, onStatusRelevantChange } = options;

	function findNavigationEntry(
		targetUrl: string,
	): NavigationEntry | undefined {
		return findNavigationEntryInSlots(slots, targetUrl);
	}

	function deleteNavigation(key: string): boolean {
		return deleteNavigationFromSlots(slots, key, onStatusRelevantChange);
	}

	function removeNavigation(key: string): void {
		const entry = findNavigationEntry(key);
		if (entry) {
			entry.control.abortController?.abort();
			deleteNavigation(key);
		}
	}

	function getNavigation(key: string): NavigationEntry | undefined {
		return findNavigationEntry(key);
	}

	function hasNavigation(key: string): boolean {
		return findNavigationEntry(key) !== undefined;
	}

	function getNavigationsSize(): number {
		return getNavigationsSizeFromSlots(slots);
	}

	function getNavigations(): Map<string, NavigationEntry> {
		return buildNavigationsMapFromSlots(slots);
	}

	function transitionPhase(targetUrl: string, phase: NavigationPhase): void {
		transitionNavigationPhaseInSlots(
			slots,
			targetUrl,
			phase,
			onStatusRelevantChange,
		);
	}

	function clearNavigationsAndSubmissions(
		submissions: Map<string | symbol, SubmissionEntry>,
	): void {
		clearSlotsAndSubmissions(slots, submissions, onStatusRelevantChange);
	}

	return {
		findNavigationEntry,
		deleteNavigation,
		removeNavigation,
		getNavigation,
		hasNavigation,
		getNavigationsSize,
		getNavigations,
		transitionPhase,
		clearNavigationsAndSubmissions,
	};
}
