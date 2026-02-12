import { resolveAbsoluteHref } from "../../platform/url.ts";
import { observePromiseRejection } from "../../platform/safety.ts";
import type {
	NavigateProps,
	NavigationControl,
	NavigationEntry,
	NavigationIntent,
	NavigationOutcome,
} from "./types.ts";

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

export type CreateNavigationControlsContext = {
	fetchRouteData: (
		controller: AbortController,
		props: NavigateProps,
	) => Promise<NavigationOutcome>;
	getActiveNavigation: () => NavigationEntry | null;
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
	const { fetchRouteData } = context;

	function createControlEntry(props: {
		navigationProps: NavigateProps;
		targetUrl: string;
		type: NavigationEntry["type"];
		intent: NavigationIntent;
		onFetchError: (error: unknown) => void;
	}): NavigationEntry {
		return createNavigationEntry({
			options: {
				props: props.navigationProps,
				fetchRouteData,
				onFetchError: props.onFetchError,
			},
			type: props.type,
			intent: props.intent,
			targetUrl: props.targetUrl,
		});
	}

	function createActiveNavigationControl(
		props: NavigateProps,
		intent: NavigationIntent,
	): NavigationControl {
		const {
			getActiveNavigation,
			setActiveNavigation,
			scheduleStatusUpdate,
			deleteNavigation,
		} = context;

		const targetUrl = resolveAbsoluteHref(props.href);
		let entry: NavigationEntry;
		entry = createControlEntry({
			navigationProps: props,
			targetUrl,
			type: props.navigationType,
			intent,
			onFetchError: () => {
				if (getActiveNavigation() === entry) {
					deleteNavigation(targetUrl);
				}
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
		const { prefetchCache } = context;

		let entry: NavigationEntry;
		entry = createControlEntry({
			navigationProps: props,
			targetUrl,
			type: "prefetch",
			intent: "none",
			onFetchError: () => {
				if (prefetchCache.get(targetUrl) === entry) {
					prefetchCache.delete(targetUrl);
				}
			},
		});

		prefetchCache.set(targetUrl, entry);
		return entry.control;
	}

	function createRevalidationControl(
		props: NavigateProps,
	): NavigationControl {
		const {
			getPendingRevalidation,
			setPendingRevalidation,
			scheduleStatusUpdate,
		} = context;

		const targetUrl = resolveAbsoluteHref(props.href);
		let entry: NavigationEntry;
		entry = createControlEntry({
			navigationProps: props,
			targetUrl,
			type: "revalidation",
			intent: "revalidate",
			onFetchError: () => {
				const pendingRevalidation = getPendingRevalidation();
				if (pendingRevalidation === entry) {
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
