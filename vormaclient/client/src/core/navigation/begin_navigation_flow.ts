import {
	findMapEntryByNavigationTarget,
	hasSameNavigationTarget,
} from "../../platform/url.ts";
import type {
	NavigateProps,
	NavigationControl,
	NavigationEntry,
	NavigationIntent,
} from "./types.ts";

export type PrefetchCacheMatch = {
	key: string;
	entry: NavigationEntry;
};

export function findPrefetchByNavigationTarget(
	prefetchCache: Map<string, NavigationEntry>,
	targetUrl: string,
): PrefetchCacheMatch | undefined {
	const matchedPrefetch = findMapEntryByNavigationTarget({
		map: prefetchCache,
		targetHref: targetUrl,
	});
	if (!matchedPrefetch) {
		return undefined;
	}

	const [key, entry] = matchedPrefetch;
	return { key, entry };
}

export function hasEntryWithSameNavigationTarget(
	entry: Pick<NavigationEntry, "targetUrl"> | null | undefined,
	targetUrl: string,
): boolean {
	return (
		!!entry &&
		hasSameNavigationTarget({
			firstHref: entry.targetUrl,
			secondHref: targetUrl,
		})
	);
}

function promoteEntryToUserNavigation(
	entry: NavigationEntry,
	props: NavigateProps,
	targetUrl: string,
): void {
	entry.targetUrl = targetUrl;
	entry.scrollToTop = props.scrollToTop;
	entry.replace = props.replace;
	entry.state = props.state;
	entry.type = "userNavigation";
	entry.intent = "navigate";
}

type ReusableUserNavigationEntryCandidate =
	| {
			type: "active";
			entry: NavigationEntry;
	  }
	| {
			type: "prefetch";
			key: string;
			entry: NavigationEntry;
	  }
	| {
			type: "pendingRevalidation";
			entry: NavigationEntry;
	  }
	| {
			type: "createNew";
	  };

export function decideReusableUserNavigationEntryCandidate(props: {
	activeNavigation: NavigationEntry | null;
	activeHasSameNavigationTarget: boolean;
	prefetchMatch: PrefetchCacheMatch | undefined;
	pendingRevalidation: NavigationEntry | null;
	pendingHasSameNavigationTarget: boolean;
}): ReusableUserNavigationEntryCandidate {
	const {
		activeNavigation,
		activeHasSameNavigationTarget,
		prefetchMatch,
		pendingRevalidation,
		pendingHasSameNavigationTarget,
	} = props;

	if (activeNavigation && activeHasSameNavigationTarget) {
		return {
			type: "active",
			entry: activeNavigation,
		};
	}

	if (prefetchMatch) {
		return {
			type: "prefetch",
			key: prefetchMatch.key,
			entry: prefetchMatch.entry,
		};
	}

	if (pendingRevalidation && pendingHasSameNavigationTarget) {
		return {
			type: "pendingRevalidation",
			entry: pendingRevalidation,
		};
	}

	return { type: "createNew" };
}

export function executeReusableUserNavigationEntryCandidate(props: {
	candidate: ReusableUserNavigationEntryCandidate;
	navigationProps: NavigateProps;
	targetUrl: string;
	prefetchCache: Map<string, NavigationEntry>;
	setActiveNavigation: (entry: NavigationEntry | null) => void;
	setPendingRevalidation: (entry: NavigationEntry | null) => void;
	scheduleStatusUpdate: () => void;
	createActiveNavigation: (
		props: NavigateProps,
		intent: NavigationIntent,
	) => NavigationControl;
}): NavigationControl {
	const {
		candidate,
		navigationProps,
		targetUrl,
		prefetchCache,
		setActiveNavigation,
		setPendingRevalidation,
		scheduleStatusUpdate,
		createActiveNavigation,
	} = props;

	switch (candidate.type) {
		case "active":
			promoteEntryToUserNavigation(
				candidate.entry,
				navigationProps,
				targetUrl,
			);
			return candidate.entry.control;
		case "prefetch":
			prefetchCache.delete(candidate.key);
			promoteEntryToUserNavigation(
				candidate.entry,
				navigationProps,
				targetUrl,
			);
			setActiveNavigation(candidate.entry);
			scheduleStatusUpdate();
			return candidate.entry.control;
		case "pendingRevalidation":
			setPendingRevalidation(null);
			promoteEntryToUserNavigation(
				candidate.entry,
				navigationProps,
				targetUrl,
			);
			setActiveNavigation(candidate.entry);
			scheduleStatusUpdate();
			return candidate.entry.control;
		case "createNew":
			return createActiveNavigation(navigationProps, "navigate");
	}
}

type BeginPrefetchAction =
	| {
			type: "reuseExisting";
			control: NavigationControl;
	  }
	| {
			type: "abortImmediately";
	  }
	| {
			type: "createNew";
	  };

export function decideBeginPrefetchAction(props: {
	activeNavigation: NavigationEntry | null;
	targetUrl: string;
	prefetchMatch: PrefetchCacheMatch | undefined;
	pendingRevalidation: NavigationEntry | null;
}): BeginPrefetchAction {
	const { activeNavigation, targetUrl, prefetchMatch, pendingRevalidation } =
		props;

	if (
		activeNavigation &&
		hasEntryWithSameNavigationTarget(activeNavigation, targetUrl)
	) {
		return {
			type: "reuseExisting",
			control: activeNavigation.control,
		};
	}

	if (prefetchMatch) {
		return {
			type: "reuseExisting",
			control: prefetchMatch.entry.control,
		};
	}

	if (
		pendingRevalidation &&
		hasEntryWithSameNavigationTarget(pendingRevalidation, targetUrl)
	) {
		return {
			type: "reuseExisting",
			control: pendingRevalidation.control,
		};
	}

	if (
		hasSameNavigationTarget({
			firstHref: window.location.href,
			secondHref: targetUrl,
		})
	) {
		return { type: "abortImmediately" };
	}

	return { type: "createNew" };
}

function createAbortedNavigationControl(): NavigationControl {
	return {
		abortController: new AbortController(),
		promise: Promise.resolve({ type: "aborted" as const }),
	};
}

export function executeBeginPrefetchAction(props: {
	action: BeginPrefetchAction;
	navigationProps: NavigateProps;
	targetUrl: string;
	createPrefetch: (
		props: NavigateProps,
		targetUrl: string,
	) => NavigationControl;
}): NavigationControl {
	const { action, navigationProps, targetUrl, createPrefetch } = props;

	switch (action.type) {
		case "reuseExisting":
			return action.control;
		case "abortImmediately":
			return createAbortedNavigationControl();
		case "createNew":
			return createPrefetch(navigationProps, targetUrl);
	}
}

type BeginRevalidationAction =
	| {
			type: "reusePending";
			control: NavigationControl;
	  }
	| {
			type: "replacePendingAndCreateNew";
			pendingRevalidation: NavigationEntry;
	  }
	| {
			type: "createNew";
	  };

export function decideBeginRevalidationAction(props: {
	pendingRevalidation: NavigationEntry | null;
	currentUrl: string;
	revalidationCoalesceMS: number;
}): BeginRevalidationAction {
	const { pendingRevalidation, currentUrl, revalidationCoalesceMS } = props;

	if (
		pendingRevalidation &&
		hasEntryWithSameNavigationTarget(pendingRevalidation, currentUrl) &&
		Date.now() - pendingRevalidation.startTime < revalidationCoalesceMS
	) {
		return {
			type: "reusePending",
			control: pendingRevalidation.control,
		};
	}

	if (pendingRevalidation) {
		return {
			type: "replacePendingAndCreateNew",
			pendingRevalidation,
		};
	}

	return { type: "createNew" };
}

export function executeBeginRevalidationAction(props: {
	action: BeginRevalidationAction;
	navigationProps: NavigateProps;
	currentUrl: string;
	setPendingRevalidation: (entry: NavigationEntry | null) => void;
	createRevalidation: (props: NavigateProps) => NavigationControl;
}): NavigationControl {
	const {
		action,
		navigationProps,
		currentUrl,
		setPendingRevalidation,
		createRevalidation,
	} = props;

	switch (action.type) {
		case "reusePending":
			return action.control;
		case "replacePendingAndCreateNew":
			action.pendingRevalidation.control.abortController?.abort();
			setPendingRevalidation(null);
			return createRevalidation({ ...navigationProps, href: currentUrl });
		case "createNew":
			return createRevalidation({ ...navigationProps, href: currentUrl });
	}
}
