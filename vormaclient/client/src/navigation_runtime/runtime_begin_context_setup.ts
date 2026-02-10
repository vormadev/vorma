import { buildBeginNavigationContext } from "./context.ts";
import { fetchRouteData } from "./fetch_route_data.ts";
import type { NavigationBookkeeping } from "./navigation_bookkeeping.ts";
import { createNavigationControls } from "./navigation_controls.ts";
import type {
	NavigateProps,
	NavigationControl,
	NavigationIntent,
} from "./types.ts";

export type RuntimeBeginContextSetup = {
	beginNavigationContext: ReturnType<typeof buildBeginNavigationContext>;
	createActiveNavigation: (
		props: NavigateProps,
		intent: NavigationIntent,
	) => NavigationControl;
};

export type CreateRuntimeBeginContextSetupOptions = {
	navigationBookkeeping: NavigationBookkeeping;
	scheduleStatusUpdate: () => void;
	revalidationCoalesceMS: number;
};

export function createRuntimeBeginContextSetup(
	options: CreateRuntimeBeginContextSetupOptions,
): RuntimeBeginContextSetup {
	const {
		navigationBookkeeping,
		scheduleStatusUpdate,
		revalidationCoalesceMS,
	} = options;

	const prefetchCache = navigationBookkeeping.getPrefetchCache();
	const getActiveNavigation = navigationBookkeeping.getActiveNavigation;
	const setActiveNavigation = navigationBookkeeping.setActiveNavigation;
	const getPendingRevalidation = navigationBookkeeping.getPendingRevalidation;
	const setPendingRevalidation = navigationBookkeeping.setPendingRevalidation;

	const { createActiveNavigation, createPrefetch, createRevalidation } =
		createNavigationControls({
			fetchRouteData,
			setActiveNavigation,
			prefetchCache,
			getPendingRevalidation,
			setPendingRevalidation,
			scheduleStatusUpdate,
			deleteNavigation: (key: string) =>
				navigationBookkeeping.deleteNavigation(key),
		});

	const beginNavigationContext = buildBeginNavigationContext({
		getActiveNavigation,
		setActiveNavigation,
		getPendingRevalidation,
		setPendingRevalidation,
		prefetchCache,
		scheduleStatusUpdate,
		revalidationCoalesceMS,
		createActiveNavigation,
		createPrefetch,
		createRevalidation,
	});

	return {
		beginNavigationContext,
		createActiveNavigation,
	};
}
