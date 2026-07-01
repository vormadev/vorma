import { QueryClientProvider } from "@tanstack/react-query";
import NProgress from "nprogress";
import { createElement } from "react";
import { createRoot } from "react-dom/client";
import { getCsrfToken } from "vorma/kit/csrf";
import { createVormaClient } from "vorma/react";
import { query_client } from "./query_client.ts";
import {
	record_build_skew,
	record_route_update,
	record_work_update,
} from "./runtime_observer.ts";
import { csrf_header, vormaClientSeed } from "./vorma.gen.ts";

const board_api_client_header = "x-board-api-client";
const board_api_method_header = "x-board-api-method";
const board_api_pattern_header = "x-board-api-pattern";

/*
`createVormaClient` binds the generated route/resource contract to a UI
adapter. The returned object is the app's public client surface: export
from here, then import from `./app.tsx` in views so all views share one
router, one work indicator, and one generated API client.
*/
export const app = createVormaClient(vormaClientSeed, {
	/*
	`apiDecorator` is the app-level hook for request headers or other
	cross-cutting fetch options. It runs for generated API calls without
	each call site remembering those headers.
	*/
	apiDecorator: ({ method, pattern }) => {
		const headers: Array<[string, string]> = [
			[board_api_client_header, "board"],
			[board_api_method_header, method],
			[board_api_pattern_header, pattern],
		];
		const csrf_token = getCsrfToken({ isDev: import.meta.env.DEV });
		if (csrf_token) {
			headers.push([csrf_header, csrf_token]);
		}
		return {
			headers,
		};
	},
	defaultErrorBoundary: ({ error }) => {
		return createElement(
			"p",
			{ className: "error" },
			error instanceof Error ? error.message : "Something went wrong.",
		);
	},
	/*
	`linkDefaultProps` sets app-wide navigation defaults. Individual links
	can still opt into different behavior, but the common case stays terse.
	*/
	linkDefaultProps: { prefetch: "intent" },
	/*
	Window-focus revalidation is useful for data that may have changed while
	the tab was inactive. `skipWorkIndicator` keeps this background refresh
	from flashing the global progress bar.
	*/
	revalidateOnWindowFocus: { staleTimeMs: 30_000, skipWorkIndicator: true },
	useViewTransitions: true,
	onBuildSkewDetected: record_build_skew,
	onRouteUpdate: record_route_update,
	onWorkUpdate: record_work_update,
	/*
	The work indicator is intentionally app-provided. Vorma reports
	navigations, prefetches, revalidations, and API calls; the app decides
	how to render that activity. Board maps it to nprogress, with small
	delays so fast transitions do not flicker.
	*/
	workIndicator: {
		start: () => {
			NProgress.start();
		},
		stop: () => {
			NProgress.done();
		},
		startDelayMs: 150,
		stopDelayMs: 50,
	},
	render: ({ RootOutlet, rootEl }) => {
		createRoot(rootEl).render(
			createElement(
				QueryClientProvider,
				{ client: query_client },
				createElement(RootOutlet),
			),
		);
	},
});

export const {
	Link,
	apiClient,
	cancelPrefetch,
	defineView,
	getRouteState,
	getWorkState,
	navigate,
	prefetch,
	revalidate,
	toHref,
	useClientLoaderData,
	usePatternClientLoaderData,
	usePatternViewData,
	useRouteState,
	useRouteSync,
	useViewData,
	useWorkState,
	workIndicator,
} = app;
