import { createActiveNavigationControl } from "./navigation_control_active.ts";
import { createPrefetchControl } from "./navigation_control_prefetch.ts";
import { createRevalidationControl } from "./navigation_control_revalidation.ts";
import type {
	NavigateProps,
	NavigationControl,
	NavigationEntry,
	NavigationIntent,
	NavigationOutcome,
} from "./types.ts";

export type CreateNavigationControlsContext = {
	fetchRouteData: (
		controller: AbortController,
		props: NavigateProps,
	) => Promise<NavigationOutcome>;
	setActiveNavigation: (entry: NavigationEntry | null) => void;
	prefetchCache: Map<string, NavigationEntry>;
	getPendingRevalidation: () => NavigationEntry | null;
	setPendingRevalidation: (entry: NavigationEntry | null) => void;
	scheduleStatusUpdate: () => void;
	deleteNavigation: (key: string) => boolean;
};

export type NavigationControls = {
	createActiveNavigation: (
		props: NavigateProps,
		intent: NavigationIntent,
	) => NavigationControl;
	createPrefetch: (
		props: NavigateProps,
		targetUrl: string,
	) => NavigationControl;
	createRevalidation: (props: NavigateProps) => NavigationControl;
};

export function createNavigationControls(
	context: CreateNavigationControlsContext,
): NavigationControls {
	function createActiveNavigation(
		props: NavigateProps,
		intent: NavigationIntent,
	): NavigationControl {
		return createActiveNavigationControl(context, props, intent);
	}

	function createPrefetch(
		props: NavigateProps,
		targetUrl: string,
	): NavigationControl {
		return createPrefetchControl(context, props, targetUrl);
	}

	function createRevalidation(props: NavigateProps): NavigationControl {
		return createRevalidationControl(context, props);
	}

	return {
		createActiveNavigation,
		createPrefetch,
		createRevalidation,
	};
}
