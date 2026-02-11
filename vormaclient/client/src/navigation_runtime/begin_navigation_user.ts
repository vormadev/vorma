import type { BeginNavigationContext } from "./begin_navigation.ts";
import type { NavigateProps, NavigationControl } from "./types.ts";
import { hasSameDataTarget } from "./url_identity.ts";

function applyUserNavigationProps(props: {
	targetUrl: string;
	entry: {
		targetUrl: string;
		scrollToTop?: boolean;
		replace?: boolean;
		state?: unknown;
		type: string;
		intent: string;
	};
	navigationProps: NavigateProps;
}): void {
	const { targetUrl, entry, navigationProps } = props;
	entry.targetUrl = targetUrl;
	entry.scrollToTop = navigationProps.scrollToTop;
	entry.replace = navigationProps.replace;
	entry.state = navigationProps.state;
	entry.type = "userNavigation";
	entry.intent = "navigate";
}

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
	const activeHasSameDataTarget = !!(
		activeNavigation &&
		hasSameDataTarget(activeNavigation.targetUrl, targetUrl)
	);

	let prefetchUpgradeKey: string | undefined;
	let prefetchToUpgrade: ReturnType<typeof getActiveNavigation> | undefined;

	if (!activeHasSameDataTarget) {
		const exactPrefetch = prefetchCache.get(targetUrl);
		if (exactPrefetch) {
			prefetchUpgradeKey = targetUrl;
			prefetchToUpgrade = exactPrefetch;
		} else {
			for (const [url, prefetch] of prefetchCache.entries()) {
				if (hasSameDataTarget(url, targetUrl)) {
					prefetchUpgradeKey = url;
					prefetchToUpgrade = prefetch;
					break;
				}
			}
		}
	}

	// Abort active navigation if it targets different route data.
	if (activeNavigation && !activeHasSameDataTarget) {
		activeNavigation.control.abortController?.abort();
		setActiveNavigation(null);
	}

	// Abort all prefetches except a same-data target candidate that we will
	// upgrade below.
	for (const [url, prefetch] of prefetchCache.entries()) {
		if (url !== prefetchUpgradeKey) {
			prefetch.control.abortController?.abort();
			prefetchCache.delete(url);
		}
	}

	// Abort pending revalidation only if it targets different route data.
	if (
		pendingRevalidation &&
		!hasSameDataTarget(pendingRevalidation.targetUrl, targetUrl)
	) {
		pendingRevalidation.control.abortController?.abort();
		setPendingRevalidation(null);
	}

	// Reuse same-data active navigation and retarget final URL semantics.
	if (activeNavigation && activeHasSameDataTarget) {
		applyUserNavigationProps({
			targetUrl,
			entry: activeNavigation,
			navigationProps: props,
		});
		return activeNavigation.control;
	}

	// Upgrade same-data prefetch to active navigation.
	if (prefetchToUpgrade && prefetchUpgradeKey) {
		prefetchCache.delete(prefetchUpgradeKey);
		applyUserNavigationProps({
			targetUrl,
			entry: prefetchToUpgrade,
			navigationProps: props,
		});
		setActiveNavigation(prefetchToUpgrade);
		scheduleStatusUpdate();
		return prefetchToUpgrade.control;
	}

	// Upgrade same-data pending revalidation to active user navigation.
	if (
		pendingRevalidation &&
		hasSameDataTarget(pendingRevalidation.targetUrl, targetUrl)
	) {
		setPendingRevalidation(null);
		applyUserNavigationProps({
			targetUrl,
			entry: pendingRevalidation,
			navigationProps: props,
		});
		setActiveNavigation(pendingRevalidation);
		scheduleStatusUpdate();
		return pendingRevalidation.control;
	}

	return createActiveNavigation(props, "navigate");
}
