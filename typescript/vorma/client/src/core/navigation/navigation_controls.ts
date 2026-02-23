import { resolveAbsoluteHref } from "vorma/kit/url";
import { observePromiseRejection } from "../../platform/safety.ts";
import {
	hasNavigationOperationOwnership,
	type NavigateProps,
	type NavigationControl,
	type NavigationEntry,
	type NavigationIntent,
	type NavigationOutcome,
} from "./types.ts";

type FetchRouteDataFn = (
	controller: AbortController,
	props: NavigateProps,
) => Promise<NavigationOutcome>;

type CreateEntryOptions = {
	props: NavigateProps;
	fetchRouteData: FetchRouteDataFn;
	onFetchError: (error: unknown) => void;
	operationID: number;
};

function createEntryControl(options: CreateEntryOptions): {
	abortController: AbortController;
	promise: Promise<NavigationOutcome>;
	operationID: number;
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
		operationID: options.operationID,
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
		operationID: options.operationID,
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
	prefetchNavigationsByTargetUrl: Map<string, NavigationEntry>;
	getRevalidationNavigation: () => NavigationEntry | null;
	setRevalidationNavigation: (entry: NavigationEntry | null) => void;
	scheduleStatusUpdate: () => void;
	deleteNavigation: (props: { targetUrl: string; reason: string }) => boolean;
	allocateNavigationOperationID: () => number;
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
				operationID: context.allocateNavigationOperationID(),
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

		const targetUrl = resolveAbsoluteHref({ href: props.href });
		let entry: NavigationEntry;
		entry = createControlEntry({
			navigationProps: props,
			targetUrl,
			type: props.navigationType,
			intent,
			onFetchError: () => {
				if (
					hasNavigationOperationOwnership({
						entry: getActiveNavigation(),
						expectedOperationID: entry.operationID,
					})
				) {
					deleteNavigation({
						targetUrl,
						reason: "active_navigation_fetch_rejected",
					});
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
		const { prefetchNavigationsByTargetUrl } = context;

		let entry: NavigationEntry;
		entry = createControlEntry({
			navigationProps: props,
			targetUrl,
			type: "prefetch",
			intent: "none",
			onFetchError: () => {
				if (
					hasNavigationOperationOwnership({
						entry: prefetchNavigationsByTargetUrl.get(targetUrl),
						expectedOperationID: entry.operationID,
					})
				) {
					context.deleteNavigation({
						targetUrl,
						reason: "prefetch_fetch_rejected",
					});
				}
			},
		});

		prefetchNavigationsByTargetUrl.set(targetUrl, entry);
		return entry.control;
	}

	function createRevalidationControl(
		props: NavigateProps,
	): NavigationControl {
		const {
			getRevalidationNavigation,
			setRevalidationNavigation,
			scheduleStatusUpdate,
		} = context;

		const targetUrl = resolveAbsoluteHref({ href: props.href });
		let entry: NavigationEntry;
		entry = createControlEntry({
			navigationProps: props,
			targetUrl,
			type: "revalidation",
			intent: "revalidate",
			onFetchError: () => {
				const revalidationNavigation = getRevalidationNavigation();
				if (
					hasNavigationOperationOwnership({
						entry: revalidationNavigation,
						expectedOperationID: entry.operationID,
					})
				) {
					context.deleteNavigation({
						targetUrl,
						reason: "revalidation_fetch_rejected",
					});
				}
			},
		});

		setRevalidationNavigation(entry);
		scheduleStatusUpdate();
		return entry.control;
	}

	return {
		createActiveNavigation: createActiveNavigationControl,
		createPrefetch: createPrefetchControl,
		createRevalidation: createRevalidationControl,
	};
}
