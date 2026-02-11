import type {
	NavigateProps,
	NavigationEntry,
	NavigationIntent,
	NavigationOutcome,
} from "./types.ts";
import { resolveNavigationTargetURL } from "./target_url.ts";

type FetchRouteDataFn = (
	controller: AbortController,
	props: NavigateProps,
) => Promise<NavigationOutcome>;

type CreateEntryOptions = {
	props: NavigateProps;
	fetchRouteData: FetchRouteDataFn;
	onFetchError: (error: unknown) => void;
};

export function createActiveNavigationEntry(
	options: CreateEntryOptions & {
		intent: NavigationIntent;
	},
): NavigationEntry {
	const { props, fetchRouteData, onFetchError, intent } = options;
	const controller = new AbortController();
	const targetUrl = resolveNavigationTargetURL(props.href);

	return {
		control: {
			abortController: controller,
			promise: fetchRouteData(controller, props).catch((error) => {
				onFetchError(error);
				throw error;
			}),
		},
		type: props.navigationType,
		intent,
		phase: "fetching",
		startTime: Date.now(),
		targetUrl,
		originUrl: window.location.href,
		scrollToTop: props.scrollToTop,
		replace: props.replace,
		state: props.state,
	};
}

export function createPrefetchNavigationEntry(
	options: CreateEntryOptions & {
		targetUrl: string;
	},
): NavigationEntry {
	const { props, fetchRouteData, onFetchError, targetUrl } = options;
	const controller = new AbortController();

	return {
		control: {
			abortController: controller,
			promise: fetchRouteData(controller, props).catch((error) => {
				onFetchError(error);
				throw error;
			}),
		},
		type: "prefetch",
		intent: "none",
		phase: "fetching",
		startTime: Date.now(),
		targetUrl,
		originUrl: window.location.href,
		scrollToTop: props.scrollToTop,
		replace: props.replace,
		state: props.state,
	};
}

export function createRevalidationNavigationEntry(
	options: CreateEntryOptions,
): NavigationEntry {
	const { props, fetchRouteData, onFetchError } = options;
	const controller = new AbortController();
	const targetUrl = resolveNavigationTargetURL(props.href);

	return {
		control: {
			abortController: controller,
			promise: fetchRouteData(controller, props).catch((error) => {
				onFetchError(error);
				throw error;
			}),
		},
		type: "revalidation",
		intent: "revalidate",
		phase: "fetching",
		startTime: Date.now(),
		targetUrl,
		originUrl: window.location.href,
		scrollToTop: props.scrollToTop,
		replace: props.replace,
		state: props.state,
	};
}
