import type { NavigationPhase, SubmissionEntry } from "./types.ts";
import type { NavigationSlots } from "./navigation_slots.ts";

export function deleteNavigationFromSlots(
	slots: NavigationSlots,
	key: string,
	onStatusRelevantChange: () => void,
): boolean {
	// Check active navigation
	if (slots.activeNavigation?.targetUrl === key) {
		slots.activeNavigation = null;
		onStatusRelevantChange();
		return true;
	}

	// Check prefetch cache
	if (slots.prefetchCache.has(key)) {
		slots.prefetchCache.delete(key);
		// No status update for prefetches
		return true;
	}

	// Check pending revalidation
	if (slots.pendingRevalidation?.targetUrl === key) {
		slots.pendingRevalidation = null;
		onStatusRelevantChange();
		return true;
	}

	return false;
}

export function transitionNavigationPhaseInSlots(
	slots: NavigationSlots,
	targetUrl: string,
	phase: NavigationPhase,
	onStatusRelevantChange: () => void,
): void {
	if (slots.activeNavigation?.targetUrl === targetUrl) {
		slots.activeNavigation.phase = phase;
		onStatusRelevantChange();
		return;
	}

	const prefetch = slots.prefetchCache.get(targetUrl);
	if (prefetch) {
		prefetch.phase = phase;
		// No status update for prefetches
		return;
	}

	if (slots.pendingRevalidation?.targetUrl === targetUrl) {
		slots.pendingRevalidation.phase = phase;
		onStatusRelevantChange();
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
