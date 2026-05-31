import { createElement } from "react";
import { createRoot } from "react-dom/client";
import { createVormaClient } from "vorma/react";
import { vormaClientSeed } from "./vorma.gen.ts";

export const app = createVormaClient(vormaClientSeed, {
	linkDefaultProps: { prefetch: "intent" },
	render: ({ RootOutlet, rootEl }) => {
		createRoot(rootEl).render(createElement(RootOutlet));
	},
});

export const { Link, apiClient, defineView, useLoaderData } = app;
