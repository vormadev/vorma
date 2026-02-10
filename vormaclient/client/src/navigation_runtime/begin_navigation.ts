import type {
	NavigateProps,
	NavigationControl,
	NavigationEntry,
	NavigationIntent,
} from "./types.ts";
import { beginPrefetch as executeBeginPrefetch } from "./begin_navigation_prefetch.ts";
import { beginRevalidation as executeBeginRevalidation } from "./begin_navigation_revalidation.ts";
import { beginUserNavigation as executeBeginUserNavigation } from "./begin_navigation_user.ts";

export type BeginNavigationContext = {
	getActiveNavigation: () => NavigationEntry | null;
	setActiveNavigation: (entry: NavigationEntry | null) => void;
	getPendingRevalidation: () => NavigationEntry | null;
	setPendingRevalidation: (entry: NavigationEntry | null) => void;
	prefetchCache: Map<string, NavigationEntry>;
	scheduleStatusUpdate: () => void;
	revalidationCoalesceMS: number;
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

export function beginUserNavigation(
	context: BeginNavigationContext,
	props: NavigateProps,
	targetUrl: string,
): NavigationControl {
	return executeBeginUserNavigation(context, props, targetUrl);
}

export function beginPrefetch(
	context: BeginNavigationContext,
	props: NavigateProps,
	targetUrl: string,
): NavigationControl {
	return executeBeginPrefetch(context, props, targetUrl);
}

export function beginRevalidation(
	context: BeginNavigationContext,
	props: NavigateProps,
): NavigationControl {
	return executeBeginRevalidation(context, props);
}
