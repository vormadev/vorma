import { QueryClientProvider } from "@tanstack/react-query";
import NProgress from "nprogress";
import { createElement } from "react";
import { createRoot } from "react-dom/client";
import { createVormaClient } from "vorma/react";
import { queryClient } from "./query_client.ts";
import { vormaClientSeed } from "./vorma.gen.ts";

export const app = createVormaClient(vormaClientSeed, {
	defaultErrorBoundary: ({ error }) => {
		return createElement(
			"p",
			{ className: "error" },
			error instanceof Error ? error.message : "Something went wrong.",
		);
	},
	linkDefaultProps: { prefetch: "intent" },
	revalidateOnWindowFocus: true,
	useViewTransitions: true,
	/*
	nprogress IS the work-indicator contract; vorma navigations,
	revalidations, and api calls all drive one bar. The delays keep
	fast transitions from flashing it.
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
				{ client: queryClient },
				createElement(RootOutlet),
			),
		);
	},
});

export const {
	Link,
	apiClient,
	defineView,
	navigate,
	revalidate,
	useRouteState,
	useViewData,
	useWorkState,
} = app;
