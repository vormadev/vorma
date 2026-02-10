import type { BeginNavigationContext } from "./begin_navigation.ts";
import type { NavigateProps, NavigationControl } from "./types.ts";

export function beginPrefetch(
	context: BeginNavigationContext,
	props: NavigateProps,
	targetUrl: string,
): NavigationControl {
	const {
		getActiveNavigation,
		getPendingRevalidation,
		prefetchCache,
		createPrefetch,
	} = context;
	const activeNavigation = getActiveNavigation();
	const pendingRevalidation = getPendingRevalidation();

	// If there's already an active navigation to this URL, return its control
	if (activeNavigation?.targetUrl === targetUrl) {
		return activeNavigation.control;
	}

	// If there's already a prefetch to this URL, return its control
	const existingPrefetch = prefetchCache.get(targetUrl);
	if (existingPrefetch) {
		return existingPrefetch.control;
	}

	// If there's a pending revalidation to this URL, return its control
	if (pendingRevalidation?.targetUrl === targetUrl) {
		return pendingRevalidation.control;
	}

	// Don't prefetch current page
	const currentUrl = new URL(window.location.href);
	const targetUrlObj = new URL(targetUrl);
	currentUrl.hash = "";
	targetUrlObj.hash = "";
	if (currentUrl.href === targetUrlObj.href) {
		return {
			abortController: new AbortController(),
			promise: Promise.resolve({ type: "aborted" as const }),
		};
	}

	return createPrefetch(props, targetUrl);
}
