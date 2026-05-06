// import { render as preact_render } from "preact";
// import { createRoot } from "react-dom/client";
import { createRoot as create_remix_root } from "remix/ui";
// import { render as solid_render } from "solid-js/web";
import { createVormaClient } from "vorma/remix";
import { vormaClientSeed } from "./types.ts";

export const vorma = createVormaClient(vormaClientSeed, {
	// react:
	// render: ({ RootOutlet, rootEl }) => {
	// 	createRoot(rootEl).render(<RootOutlet />);
	// },

	// preact:
	// 	render: ({ RootOutlet, rootEl }) => {
	// 		preact_render(<RootOutlet />, rootEl);
	// 	},

	// solid:
	// render: ({ RootOutlet, rootEl }) => {
	// 	solid_render(() => RootOutlet({}), rootEl);
	// },

	// remix:
	render: ({ RootOutlet, rootEl }) => {
		const root = create_remix_root(rootEl);
		root.render(<RootOutlet />);
		root.flush();
	},

	linkDefaultProps: {
		prefetch: "intent",
		visitOnPointerDown: true,
	},
});

export const { defineView, Link, useLoaderData, useClientLoaderData } = vorma;
