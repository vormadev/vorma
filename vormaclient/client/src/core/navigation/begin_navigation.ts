import { hasSameDataTarget } from "../../platform/url.ts";
import { observePromiseRejection } from "../../platform/safety.ts";
import type {
	NavigateProps,
	NavigationControl,
	NavigationEntry,
	NavigationIntent,
	NavigationOutcome,
} from "./types.ts";

function resolveNavigationTargetURL(href: string): string {
	return new URL(href, window.location.href).href;
}

type FetchRouteDataFn = (
	controller: AbortController,
	props: NavigateProps,
) => Promise<NavigationOutcome>;

type CreateEntryOptions = {
	props: NavigateProps;
	fetchRouteData: FetchRouteDataFn;
	onFetchError: (error: unknown) => void;
};

function createEntryControl(options: CreateEntryOptions): {
	abortController: AbortController;
	promise: Promise<NavigationOutcome>;
} {
	const { props, fetchRouteData, onFetchError } = options;
	const abortController = new AbortController();

	return {
		abortController,
		promise: observePromiseRejection(
			fetchRouteData(abortController, props).catch((error) => {
				onFetchError(error);
				throw error;
			}),
		),
	};
}

function createNavigationEntry(props: {
	options: CreateEntryOptions;
	type: NavigationEntry["type"];
	intent: NavigationIntent;
	targetUrl: string;
}): NavigationEntry {
	const { options, type, intent, targetUrl } = props;
	const control = createEntryControl(options);

	return {
		control,
		type,
		intent,
		phase: "fetching",
		startTime: Date.now(),
		targetUrl,
		originUrl: window.location.href,
		scrollToTop: options.props.scrollToTop,
		replace: options.props.replace,
		state: options.props.state,
	};
}

function createActiveNavigationEntry(
	options: CreateEntryOptions & {
		intent: NavigationIntent;
	},
): NavigationEntry {
	const { props, intent } = options;
	const targetUrl = resolveNavigationTargetURL(props.href);

	return createNavigationEntry({
		options,
		type: props.navigationType,
		intent,
		targetUrl,
	});
}

function createPrefetchNavigationEntry(
	options: CreateEntryOptions & {
		targetUrl: string;
	},
): NavigationEntry {
	const { targetUrl } = options;

	return createNavigationEntry({
		options,
		type: "prefetch",
		intent: "none",
		targetUrl,
	});
}

function createRevalidationNavigationEntry(
	options: CreateEntryOptions,
): NavigationEntry {
	const { props } = options;
	const targetUrl = resolveNavigationTargetURL(props.href);

	return createNavigationEntry({
		options,
		type: "revalidation",
		intent: "revalidate",
		targetUrl,
	});
}

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
	function createActiveNavigationControl(
		props: NavigateProps,
		intent: NavigationIntent,
	): NavigationControl {
		const {
			fetchRouteData,
			setActiveNavigation,
			scheduleStatusUpdate,
			deleteNavigation,
		} = context;

		const targetUrl = resolveNavigationTargetURL(props.href);
		const entry = createActiveNavigationEntry({
			props,
			intent,
			fetchRouteData,
			onFetchError: () => {
				deleteNavigation(targetUrl);
			},
		});

		setActiveNavigation(entry);
		scheduleStatusUpdate();
		return entry.control;
	}

	function createPrefetchControl(
		props: NavigateProps,
		targetUrl: string,
	): NavigationControl {
		const { fetchRouteData, prefetchCache } = context;

		const entry = createPrefetchNavigationEntry({
			props,
			targetUrl,
			fetchRouteData,
			onFetchError: () => {
				prefetchCache.delete(targetUrl);
			},
		});

		prefetchCache.set(targetUrl, entry);
		return entry.control;
	}

	function createRevalidationControl(props: NavigateProps): NavigationControl {
		const {
			fetchRouteData,
			getPendingRevalidation,
			setPendingRevalidation,
			scheduleStatusUpdate,
		} = context;

		const targetUrl = resolveNavigationTargetURL(props.href);
		const entry = createRevalidationNavigationEntry({
			props,
			fetchRouteData,
			onFetchError: () => {
				const pendingRevalidation = getPendingRevalidation();
				if (pendingRevalidation?.targetUrl === targetUrl) {
					setPendingRevalidation(null);
					scheduleStatusUpdate();
				}
			},
		});

		setPendingRevalidation(entry);
		scheduleStatusUpdate();
		return entry.control;
	}

	return {
		createActiveNavigation: (props, intent) =>
			createActiveNavigationControl(props, intent),
		createPrefetch: (props, targetUrl) =>
			createPrefetchControl(props, targetUrl),
		createRevalidation: (props) => createRevalidationControl(props),
	};
}

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

	if (activeNavigation && !activeHasSameDataTarget) {
		activeNavigation.control.abortController?.abort();
		setActiveNavigation(null);
	}

	for (const [url, prefetch] of prefetchCache.entries()) {
		if (url !== prefetchUpgradeKey) {
			prefetch.control.abortController?.abort();
			prefetchCache.delete(url);
		}
	}

	if (
		pendingRevalidation &&
		!hasSameDataTarget(pendingRevalidation.targetUrl, targetUrl)
	) {
		pendingRevalidation.control.abortController?.abort();
		setPendingRevalidation(null);
	}

	if (activeNavigation && activeHasSameDataTarget) {
		activeNavigation.targetUrl = targetUrl;
		activeNavigation.scrollToTop = props.scrollToTop;
		activeNavigation.replace = props.replace;
		activeNavigation.state = props.state;
		activeNavigation.type = "userNavigation";
		activeNavigation.intent = "navigate";
		return activeNavigation.control;
	}

	if (prefetchToUpgrade && prefetchUpgradeKey) {
		prefetchCache.delete(prefetchUpgradeKey);
		prefetchToUpgrade.targetUrl = targetUrl;
		prefetchToUpgrade.scrollToTop = props.scrollToTop;
		prefetchToUpgrade.replace = props.replace;
		prefetchToUpgrade.state = props.state;
		prefetchToUpgrade.type = "userNavigation";
		prefetchToUpgrade.intent = "navigate";
		setActiveNavigation(prefetchToUpgrade);
		scheduleStatusUpdate();
		return prefetchToUpgrade.control;
	}

	if (
		pendingRevalidation &&
		hasSameDataTarget(pendingRevalidation.targetUrl, targetUrl)
	) {
		setPendingRevalidation(null);
		pendingRevalidation.targetUrl = targetUrl;
		pendingRevalidation.scrollToTop = props.scrollToTop;
		pendingRevalidation.replace = props.replace;
		pendingRevalidation.state = props.state;
		pendingRevalidation.type = "userNavigation";
		pendingRevalidation.intent = "navigate";
		setActiveNavigation(pendingRevalidation);
		scheduleStatusUpdate();
		return pendingRevalidation.control;
	}

	return createActiveNavigation(props, "navigate");
}

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

	if (
		activeNavigation &&
		hasSameDataTarget(activeNavigation.targetUrl, targetUrl)
	) {
		return activeNavigation.control;
	}

	const existingPrefetch = prefetchCache.get(targetUrl);
	if (existingPrefetch) {
		return existingPrefetch.control;
	}

	for (const [prefetchUrl, prefetch] of prefetchCache.entries()) {
		if (hasSameDataTarget(prefetchUrl, targetUrl)) {
			return prefetch.control;
		}
	}

	if (
		pendingRevalidation &&
		hasSameDataTarget(pendingRevalidation.targetUrl, targetUrl)
	) {
		return pendingRevalidation.control;
	}

	if (hasSameDataTarget(window.location.href, targetUrl)) {
		return {
			abortController: new AbortController(),
			promise: Promise.resolve({ type: "aborted" as const }),
		};
	}

	return createPrefetch(props, targetUrl);
}

export function beginRevalidation(
	context: BeginNavigationContext,
	props: NavigateProps,
): NavigationControl {
	const {
		getPendingRevalidation,
		setPendingRevalidation,
		revalidationCoalesceMS,
		createRevalidation,
	} = context;
	const currentUrl = window.location.href;
	const pendingRevalidation = getPendingRevalidation();

	if (
		pendingRevalidation &&
		hasSameDataTarget(pendingRevalidation.targetUrl, currentUrl) &&
		Date.now() - pendingRevalidation.startTime < revalidationCoalesceMS
	) {
		return pendingRevalidation.control;
	}

	if (pendingRevalidation) {
		pendingRevalidation.control.abortController?.abort();
		setPendingRevalidation(null);
	}

	return createRevalidation({ ...props, href: currentUrl });
}
