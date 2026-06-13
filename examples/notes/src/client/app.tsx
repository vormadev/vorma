import { createElement } from "react";
import { createRoot } from "react-dom/client";
import { createVormaClient } from "vorma/react";
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
	render: ({ RootOutlet, rootEl }) => {
		createRoot(rootEl).render(createElement(RootOutlet));
	},
});

export const {
	Link,
	apiClient,
	cancelPrefetch,
	defineView,
	navigate,
	prefetch,
	revalidate,
	useClientLoaderData,
	useRouteState,
	useViewData,
	useWorkState,
} = app;
