import type { NavigationBookkeeping } from "./navigation_bookkeeping.ts";
import type {
	NavigationEntry,
	NavigationPhase,
	SubmissionEntry,
} from "./types.ts";

export type NavigationBookkeepingAdapter = {
	transitionPhase: (targetUrl: string, phase: NavigationPhase) => void;
	findNavigationEntry: (targetUrl: string) => NavigationEntry | undefined;
	deleteNavigation: (key: string) => boolean;
	removeNavigation: (key: string) => void;
	getNavigation: (key: string) => NavigationEntry | undefined;
	hasNavigation: (key: string) => boolean;
	getNavigationsSize: () => number;
	getNavigations: () => Map<string, NavigationEntry>;
	clearAll: () => void;
};

export type CreateNavigationBookkeepingAdapterOptions = {
	navigationBookkeeping: NavigationBookkeeping;
	submissions: Map<string | symbol, SubmissionEntry>;
};

export function createNavigationBookkeepingAdapter(
	options: CreateNavigationBookkeepingAdapterOptions,
): NavigationBookkeepingAdapter {
	const { navigationBookkeeping, submissions } = options;

	function clearAll(): void {
		navigationBookkeeping.clearNavigationsAndSubmissions(submissions);
	}

	return {
		transitionPhase: navigationBookkeeping.transitionPhase,
		findNavigationEntry: navigationBookkeeping.findNavigationEntry,
		deleteNavigation: navigationBookkeeping.deleteNavigation,
		removeNavigation: navigationBookkeeping.removeNavigation,
		getNavigation: navigationBookkeeping.getNavigation,
		hasNavigation: navigationBookkeeping.hasNavigation,
		getNavigationsSize: navigationBookkeeping.getNavigationsSize,
		getNavigations: navigationBookkeeping.getNavigations,
		clearAll,
	};
}
