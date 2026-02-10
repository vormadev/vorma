import type {
	NavigationEntry,
	NavigationPhase,
	SubmissionEntry,
} from "./types.ts";
import { createNavigationBookkeepingCore } from "./navigation_bookkeeping_core.ts";
import { type NavigationSlots } from "./navigation_slots.ts";
import { createNavigationSlotStateAccess } from "./navigation_slot_state_access.ts";

export type NavigationBookkeeping = {
	getActiveNavigation: () => NavigationEntry | null;
	setActiveNavigation: (entry: NavigationEntry | null) => void;
	getPendingRevalidation: () => NavigationEntry | null;
	setPendingRevalidation: (entry: NavigationEntry | null) => void;
	getPrefetchCache: () => Map<string, NavigationEntry>;
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

export type CreateNavigationBookkeepingOptions = {
	onStatusRelevantChange: () => void;
};

export function createNavigationBookkeeping(
	options: CreateNavigationBookkeepingOptions,
): NavigationBookkeeping {
	const { onStatusRelevantChange } = options;

	const slots: NavigationSlots = {
		// Single slot for active user/browser/redirect navigation
		activeNavigation: null,
		// Separate cache for prefetches (can have multiple to different URLs)
		prefetchCache: new Map<string, NavigationEntry>(),
		// Single slot for pending revalidation (at most one, coalesced)
		pendingRevalidation: null,
	};

	const stateAccess = createNavigationSlotStateAccess(slots);
	const core = createNavigationBookkeepingCore({
		slots,
		onStatusRelevantChange,
	});

	return {
		...stateAccess,
		...core,
	};
}
