import { debounce } from "vorma/kit/debounce";
import { jsonDeepEquals } from "vorma/kit/json";
import type { StatusEventDetail } from "../../platform/events.ts";
import {
	findMapEntryByNavigationTarget,
	hasSameNavigationTarget,
} from "../../platform/url.ts";
import type {
	NavigationEntry,
	NavigationPhase,
	SubmissionEntry,
} from "./types.ts";

export function computeNavigationStatus(props: {
	activeNavigation: NavigationEntry | null;
	pendingRevalidation: NavigationEntry | null;
	submissions: Map<string | symbol, SubmissionEntry>;
}): StatusEventDetail {
	const { activeNavigation, pendingRevalidation, submissions } = props;

	const isNavigating =
		activeNavigation !== null &&
		activeNavigation.intent === "navigate" &&
		activeNavigation.phase !== "complete";

	const isRevalidating =
		pendingRevalidation !== null &&
		pendingRevalidation.phase !== "complete";

	const isSubmitting = Array.from(submissions.values()).some(
		(x) => !x.skipGlobalLoadingIndicator,
	);

	return { isNavigating, isSubmitting, isRevalidating };
}

export function createStatusSignaler(props: {
	getStatus: () => StatusEventDetail;
	dispatchStatusEvent: (status: StatusEventDetail) => void;
	debounceMS?: number;
}): { scheduleStatusUpdate: () => void } {
	const { getStatus, dispatchStatusEvent, debounceMS = 8 } = props;
	let lastDispatchedStatus: StatusEventDetail | null = null;

	function dispatchStatusEventInternal(): void {
		const newStatus = getStatus();
		if (jsonDeepEquals(lastDispatchedStatus, newStatus)) {
			return;
		}
		lastDispatchedStatus = newStatus;
		dispatchStatusEvent(newStatus);
	}

	const scheduleStatusUpdate = debounce(() => {
		dispatchStatusEventInternal();
	}, debounceMS);

	return {
		scheduleStatusUpdate,
	};
}

export type NavigationSlots = {
	activeNavigation: NavigationEntry | null;
	prefetchCache: Map<string, NavigationEntry>;
	pendingRevalidation: NavigationEntry | null;
};

type NavigationSlotMatch =
	| {
			slot: "active";
			entry: NavigationEntry;
	  }
	| {
			slot: "prefetch";
			key: string;
			entry: NavigationEntry;
	  }
	| {
			slot: "pendingRevalidation";
			entry: NavigationEntry;
	  };

export function findNavigationEntryInSlots(
	slots: NavigationSlots,
	targetUrl: string,
): NavigationEntry | undefined {
	return matchSlotByTargetURL(slots, targetUrl)?.entry;
}

export function getNavigationsSizeFromSlots(slots: NavigationSlots): number {
	let size = 0;
	if (slots.activeNavigation) size++;
	size += slots.prefetchCache.size;
	if (slots.pendingRevalidation) size++;
	return size;
}

export function buildNavigationsMapFromSlots(
	slots: NavigationSlots,
): Map<string, NavigationEntry> {
	const map = new Map<string, NavigationEntry>();
	if (slots.activeNavigation) {
		map.set(slots.activeNavigation.targetUrl, slots.activeNavigation);
	}
	for (const [key, entry] of slots.prefetchCache) {
		map.set(key, entry);
	}
	if (slots.pendingRevalidation) {
		map.set(slots.pendingRevalidation.targetUrl, slots.pendingRevalidation);
	}
	return map;
}

function findMatchingPrefetchKey(
	slots: NavigationSlots,
	key: string,
): string | undefined {
	return findMapEntryByNavigationTarget({
		map: slots.prefetchCache,
		targetHref: key,
	})?.[0];
}

function matchSlotByTargetURL(
	slots: NavigationSlots,
	targetUrl: string,
): NavigationSlotMatch | undefined {
	if (
		slots.activeNavigation &&
		hasSameNavigationTarget({
			firstHref: slots.activeNavigation.targetUrl,
			secondHref: targetUrl,
		})
	) {
		return {
			slot: "active",
			entry: slots.activeNavigation,
		};
	}

	const prefetchKey = findMatchingPrefetchKey(slots, targetUrl);
	if (prefetchKey) {
		return {
			slot: "prefetch",
			key: prefetchKey,
			entry: slots.prefetchCache.get(prefetchKey)!,
		};
	}

	if (
		slots.pendingRevalidation &&
		hasSameNavigationTarget({
			firstHref: slots.pendingRevalidation.targetUrl,
			secondHref: targetUrl,
		})
	) {
		return {
			slot: "pendingRevalidation",
			entry: slots.pendingRevalidation,
		};
	}

	return undefined;
}

export function deleteNavigationFromSlots(
	slots: NavigationSlots,
	key: string,
	onStatusRelevantChange: () => void,
): boolean {
	const action = decideDeleteNavigationSlotAction(slots, key);
	return executeDeleteNavigationSlotAction(
		slots,
		action,
		onStatusRelevantChange,
	);
}

type DeleteNavigationSlotAction =
	| {
			type: "stop";
	  }
	| {
			type: "clearActive";
	  }
	| {
			type: "deletePrefetch";
			key: string;
	  }
	| {
			type: "clearPendingRevalidation";
	  };

function decideDeleteNavigationSlotAction(
	slots: NavigationSlots,
	key: string,
): DeleteNavigationSlotAction {
	const matchedSlot = matchSlotByTargetURL(slots, key);
	if (!matchedSlot) {
		return { type: "stop" };
	}

	switch (matchedSlot.slot) {
		case "active":
			return { type: "clearActive" };
		case "prefetch":
			return {
				type: "deletePrefetch",
				key: matchedSlot.key,
			};
		case "pendingRevalidation":
			return { type: "clearPendingRevalidation" };
	}
}

function executeDeleteNavigationSlotAction(
	slots: NavigationSlots,
	action: DeleteNavigationSlotAction,
	onStatusRelevantChange: () => void,
): boolean {
	switch (action.type) {
		case "stop":
			return false;
		case "clearActive":
			slots.activeNavigation = null;
			onStatusRelevantChange();
			return true;
		case "deletePrefetch":
			slots.prefetchCache.delete(action.key);
			return true;
		case "clearPendingRevalidation":
			slots.pendingRevalidation = null;
			onStatusRelevantChange();
			return true;
	}
}

export function transitionNavigationPhaseInSlots(
	slots: NavigationSlots,
	targetUrl: string,
	phase: NavigationPhase,
	onStatusRelevantChange: () => void,
): void {
	const action = decideTransitionNavigationPhaseAction(
		slots,
		targetUrl,
		phase,
	);
	executeTransitionNavigationPhaseAction(action, onStatusRelevantChange);
}

type TransitionNavigationPhaseAction =
	| {
			type: "stop";
	  }
	| {
			type: "setPhase";
			entry: NavigationEntry;
			phase: NavigationPhase;
			shouldSignalStatusChange: boolean;
	  };

function decideTransitionNavigationPhaseAction(
	slots: NavigationSlots,
	targetUrl: string,
	phase: NavigationPhase,
): TransitionNavigationPhaseAction {
	const matchedSlot = matchSlotByTargetURL(slots, targetUrl);
	if (!matchedSlot) {
		return { type: "stop" };
	}

	return {
		type: "setPhase",
		entry: matchedSlot.entry,
		phase,
		shouldSignalStatusChange: matchedSlot.slot !== "prefetch",
	};
}

function executeTransitionNavigationPhaseAction(
	action: TransitionNavigationPhaseAction,
	onStatusRelevantChange: () => void,
): void {
	switch (action.type) {
		case "stop":
			return;
		case "setPhase":
			action.entry.phase = action.phase;
			if (action.shouldSignalStatusChange) {
				onStatusRelevantChange();
			}
			return;
	}
}

export function clearSlotsAndSubmissions(
	slots: NavigationSlots,
	submissions: Map<string | symbol, SubmissionEntry>,
	onStatusRelevantChange: () => void,
): void {
	if (slots.activeNavigation) {
		slots.activeNavigation.control.abortController?.abort();
		slots.activeNavigation = null;
	}

	for (const prefetch of slots.prefetchCache.values()) {
		prefetch.control.abortController?.abort();
	}
	slots.prefetchCache.clear();

	if (slots.pendingRevalidation) {
		slots.pendingRevalidation.control.abortController?.abort();
		slots.pendingRevalidation = null;
	}

	for (const sub of submissions.values()) {
		sub.control.abortController?.abort();
	}
	submissions.clear();

	onStatusRelevantChange();
}
