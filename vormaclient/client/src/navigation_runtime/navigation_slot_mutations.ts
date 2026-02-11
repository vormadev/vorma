import type { NavigationPhase, SubmissionEntry } from "./types.ts";
import type { NavigationSlots } from "./navigation_slots.ts";
import { hasSameDataTarget } from "./url_identity.ts";

function findMatchingPrefetchKey(
	slots: NavigationSlots,
	key: string,
): string | undefined {
	if (slots.prefetchCache.has(key)) {
		return key;
	}

	for (const url of slots.prefetchCache.keys()) {
		if (hasSameDataTarget(url, key)) {
			return url;
		}
	}

	return undefined;
}

export function deleteNavigationFromSlots(
	slots: NavigationSlots,
	key: string,
	onStatusRelevantChange: () => void,
): boolean {
	// Check active navigation
	if (
		slots.activeNavigation &&
		(slots.activeNavigation.targetUrl === key ||
			hasSameDataTarget(slots.activeNavigation.targetUrl, key))
	) {
		slots.activeNavigation = null;
		onStatusRelevantChange();
		return true;
	}

	// Check prefetch cache
	const prefetchKey = findMatchingPrefetchKey(slots, key);
	if (prefetchKey) {
		slots.prefetchCache.delete(prefetchKey);
		// No status update for prefetches
		return true;
	}

	// Check pending revalidation
	if (
		slots.pendingRevalidation &&
		(slots.pendingRevalidation.targetUrl === key ||
			hasSameDataTarget(slots.pendingRevalidation.targetUrl, key))
	) {
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
	if (
		slots.activeNavigation &&
		(slots.activeNavigation.targetUrl === targetUrl ||
			hasSameDataTarget(slots.activeNavigation.targetUrl, targetUrl))
	) {
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
	const prefetchAliasKey = findMatchingPrefetchKey(slots, targetUrl);
	if (prefetchAliasKey) {
		const prefetchAlias = slots.prefetchCache.get(prefetchAliasKey);
		if (prefetchAlias) {
			prefetchAlias.phase = phase;
			// No status update for prefetches
			return;
		}
	}

	if (
		slots.pendingRevalidation &&
		(slots.pendingRevalidation.targetUrl === targetUrl ||
			hasSameDataTarget(slots.pendingRevalidation.targetUrl, targetUrl))
	) {
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
