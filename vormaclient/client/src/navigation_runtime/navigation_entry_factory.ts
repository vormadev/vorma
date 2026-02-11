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

function createEntryControl(options: CreateEntryOptions): {
	abortController: AbortController;
	promise: Promise<NavigationOutcome>;
} {
	const { props, fetchRouteData, onFetchError } = options;
	const abortController = new AbortController();

	return {
		abortController,
		promise: fetchRouteData(abortController, props).catch((error) => {
			onFetchError(error);
			throw error;
		}),
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

export function createActiveNavigationEntry(
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

export function createPrefetchNavigationEntry(
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

export function createRevalidationNavigationEntry(
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
