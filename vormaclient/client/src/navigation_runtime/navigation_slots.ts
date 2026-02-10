import type { NavigationEntry } from "./types.ts";

export type NavigationSlots = {
	activeNavigation: NavigationEntry | null;
	prefetchCache: Map<string, NavigationEntry>;
	pendingRevalidation: NavigationEntry | null;
};

export function findNavigationEntryInSlots(
	slots: NavigationSlots,
	targetUrl: string,
): NavigationEntry | undefined {
	if (slots.activeNavigation?.targetUrl === targetUrl) {
		return slots.activeNavigation;
	}

	const prefetch = slots.prefetchCache.get(targetUrl);
	if (prefetch) {
		return prefetch;
	}

	if (slots.pendingRevalidation?.targetUrl === targetUrl) {
		return slots.pendingRevalidation;
	}

	return undefined;
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
	// Reconstruct Map for compatibility with existing code
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
