import { resolveAbsoluteHref } from "vorma/kit/url";
import type {
	NavigateProps,
	NavigationControl,
	NavigationEntry,
	NavigationIntent,
} from "./types.ts";
import {
	decideBeginPrefetchAction,
	decideBeginRevalidationAction,
	decideReusableUserNavigationEntryCandidate,
	executeBeginPrefetchAction,
	executeBeginRevalidationAction,
	executeReusableUserNavigationEntryCandidate,
	findPrefetchByNavigationTarget,
	hasEntryWithSameNavigationTarget,
} from "./begin_navigation_flow.ts";
export { createNavigationControls } from "./navigation_controls.ts";
export type {
	CreateNavigationControlsContext,
	NavigationControls,
} from "./navigation_controls.ts";

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

export function beginNavigation(
	context: BeginNavigationContext,
	props: NavigateProps,
): NavigationControl {
	const targetUrl = resolveAbsoluteHref({ href: props.href });

	switch (props.navigationType) {
		case "userNavigation":
			return beginUserNavigation(context, props, targetUrl);
		case "prefetch":
			return beginPrefetch(context, props, targetUrl);
		case "revalidation":
			return beginRevalidation(context, props);
		case "browserHistory":
		case "redirect":
		default:
			return context.createActiveNavigation(props, "navigate");
	}
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
	const activeHasSameNavigationTarget = hasEntryWithSameNavigationTarget(
		activeNavigation,
		targetUrl,
	);
	const pendingHasSameNavigationTarget = hasEntryWithSameNavigationTarget(
		pendingRevalidation,
		targetUrl,
	);
	const prefetchMatch = !activeHasSameNavigationTarget
		? findPrefetchByNavigationTarget(prefetchCache, targetUrl)
		: undefined;

	if (activeNavigation && !activeHasSameNavigationTarget) {
		activeNavigation.control.abortController?.abort();
		setActiveNavigation(null);
	}

	for (const [url, prefetch] of prefetchCache.entries()) {
		if (url !== prefetchMatch?.key) {
			prefetch.control.abortController?.abort();
			prefetchCache.delete(url);
		}
	}

	if (pendingRevalidation && !pendingHasSameNavigationTarget) {
		pendingRevalidation.control.abortController?.abort();
		setPendingRevalidation(null);
	}

	const reusableEntryCandidate = decideReusableUserNavigationEntryCandidate({
		activeNavigation,
		activeHasSameNavigationTarget,
		prefetchMatch,
		pendingRevalidation,
		pendingHasSameNavigationTarget,
	});

	return executeReusableUserNavigationEntryCandidate({
		candidate: reusableEntryCandidate,
		navigationProps: props,
		targetUrl,
		prefetchCache,
		setActiveNavigation,
		setPendingRevalidation,
		scheduleStatusUpdate,
		createActiveNavigation,
	});
}

export function beginPrefetch(
	context: BeginNavigationContext,
	props: NavigateProps,
	targetUrl: string,
): NavigationControl {
	const { getActiveNavigation, getPendingRevalidation, prefetchCache } =
		context;
	const activeNavigation = getActiveNavigation();
	const pendingRevalidation = getPendingRevalidation();
	const prefetchMatch = findPrefetchByNavigationTarget(
		prefetchCache,
		targetUrl,
	);

	const prefetchAction = decideBeginPrefetchAction({
		activeNavigation,
		targetUrl,
		prefetchMatch,
		pendingRevalidation,
	});

	return executeBeginPrefetchAction({
		action: prefetchAction,
		navigationProps: props,
		targetUrl,
		createPrefetch: context.createPrefetch,
	});
}

export function beginRevalidation(
	context: BeginNavigationContext,
	props: NavigateProps,
): NavigationControl {
	const currentUrl = window.location.href;
	const pendingRevalidation = context.getPendingRevalidation();
	const revalidationAction = decideBeginRevalidationAction({
		pendingRevalidation,
		currentUrl,
		revalidationCoalesceMS: context.revalidationCoalesceMS,
	});

	return executeBeginRevalidationAction({
		action: revalidationAction,
		navigationProps: props,
		currentUrl,
		setPendingRevalidation: context.setPendingRevalidation,
		createRevalidation: context.createRevalidation,
	});
}
