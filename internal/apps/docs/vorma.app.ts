import { render } from "solid-js/web";
import { createVormaClient } from "vorma/solid";
import { vormaAppConfig } from "./vorma.gen.ts";

export const app = createVormaClient(vormaAppConfig, {
	linkDefaultProps: { prefetch: "intent", visitOnPointerDown: true },
	render: ({ RootOutlet, rootEl }) => {
		render(() => RootOutlet({}), rootEl);
	},
});

export const { defineView, Link, useLoaderData, useClientLoaderData } = app;
