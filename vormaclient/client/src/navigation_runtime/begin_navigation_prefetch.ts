import type { BeginNavigationContext } from "./begin_navigation.ts";
import type { NavigateProps, NavigationControl } from "./types.ts";
import { hasSameDataTarget } from "./url_identity.ts";

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
	if (
		activeNavigation &&
		hasSameDataTarget(activeNavigation.targetUrl, targetUrl)
	) {
		return activeNavigation.control;
	}

	// If there's already a prefetch to this URL, return its control
	const existingPrefetch = prefetchCache.get(targetUrl);
	if (existingPrefetch) {
		return existingPrefetch.control;
	}

	// Reuse an existing prefetch when only hash differs (same data target).
	for (const [prefetchUrl, prefetch] of prefetchCache.entries()) {
		if (hasSameDataTarget(prefetchUrl, targetUrl)) {
			return prefetch.control;
		}
	}

	// If there's a pending revalidation to this URL, return its control
	if (
		pendingRevalidation &&
		hasSameDataTarget(pendingRevalidation.targetUrl, targetUrl)
	) {
		return pendingRevalidation.control;
	}

	// Don't prefetch current page
	if (hasSameDataTarget(window.location.href, targetUrl)) {
		return {
			abortController: new AbortController(),
			promise: Promise.resolve({ type: "aborted" as const }),
		};
	}

	return createPrefetch(props, targetUrl);
}
