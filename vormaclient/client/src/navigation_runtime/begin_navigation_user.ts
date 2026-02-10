import type { BeginNavigationContext } from "./begin_navigation.ts";
import type { NavigateProps, NavigationControl } from "./types.ts";

export function beginUserNavigation(
	context: BeginNavigationContext,
	props: NavigateProps,
	targetUrl: string,
): NavigationControl {
	const {
		getActiveNavigation,
		setActiveNavigation,
		getPendingRevalidation,
		setPendingRevalidation,
		prefetchCache,
		scheduleStatusUpdate,
		createActiveNavigation,
	} = context;

	const activeNavigation = getActiveNavigation();
	const pendingRevalidation = getPendingRevalidation();

	// Abort active navigation if it's to a different URL
	if (activeNavigation && activeNavigation.targetUrl !== targetUrl) {
		activeNavigation.control.abortController?.abort();
		setActiveNavigation(null);
	}

	// Abort all prefetches except the one we might upgrade
	for (const [url, prefetch] of prefetchCache.entries()) {
		if (url !== targetUrl) {
			prefetch.control.abortController?.abort();
			prefetchCache.delete(url);
		}
	}

	// Abort pending revalidation only if it's to a different URL
	if (pendingRevalidation && pendingRevalidation.targetUrl !== targetUrl) {
		pendingRevalidation.control.abortController?.abort();
		setPendingRevalidation(null);
	}

	// Check if there's already an active navigation to this URL
	if (activeNavigation?.targetUrl === targetUrl) {
		return activeNavigation.control;
	}

	// Check if there's a prefetch to upgrade
	const existingPrefetch = prefetchCache.get(targetUrl);
	if (existingPrefetch) {
		// Upgrade prefetch: move from cache to active slot, change intent
		prefetchCache.delete(targetUrl);
		existingPrefetch.type = "userNavigation";
		existingPrefetch.intent = "navigate";
		existingPrefetch.scrollToTop = props.scrollToTop;
		existingPrefetch.replace = props.replace;
		existingPrefetch.state = props.state;
		setActiveNavigation(existingPrefetch);
		scheduleStatusUpdate();
		return existingPrefetch.control;
	}

	// Check if there's a pending revalidation to the same URL - upgrade it
	if (pendingRevalidation?.targetUrl === targetUrl) {
		// Upgrade revalidation: change intent so user gets proper link semantics
		pendingRevalidation.type = "userNavigation";
		pendingRevalidation.intent = "navigate";
		pendingRevalidation.scrollToTop = props.scrollToTop;
		pendingRevalidation.replace = props.replace;
		pendingRevalidation.state = props.state;
		return pendingRevalidation.control;
	}

	return createActiveNavigation(props, "navigate");
}
