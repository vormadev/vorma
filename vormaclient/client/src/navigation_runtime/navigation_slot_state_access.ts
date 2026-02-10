import type { NavigationEntry } from "./types.ts";
import type { NavigationSlots } from "./navigation_slots.ts";

export type NavigationSlotStateAccess = {
	getActiveNavigation: () => NavigationEntry | null;
	setActiveNavigation: (entry: NavigationEntry | null) => void;
	getPendingRevalidation: () => NavigationEntry | null;
	setPendingRevalidation: (entry: NavigationEntry | null) => void;
	getPrefetchCache: () => Map<string, NavigationEntry>;
};

export function createNavigationSlotStateAccess(
	slots: NavigationSlots,
): NavigationSlotStateAccess {
	function getActiveNavigation(): NavigationEntry | null {
		return slots.activeNavigation;
	}

	function setActiveNavigation(entry: NavigationEntry | null): void {
		slots.activeNavigation = entry;
	}

	function getPendingRevalidation(): NavigationEntry | null {
		return slots.pendingRevalidation;
	}

	function setPendingRevalidation(entry: NavigationEntry | null): void {
		slots.pendingRevalidation = entry;
	}

	function getPrefetchCache(): Map<string, NavigationEntry> {
		return slots.prefetchCache;
	}

	return {
		getActiveNavigation,
		setActiveNavigation,
		getPendingRevalidation,
		setPendingRevalidation,
		getPrefetchCache,
	};
}
