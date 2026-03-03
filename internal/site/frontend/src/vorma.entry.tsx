import { render } from "solid-js/web";
import { getRootEl, initClient } from "vorma/client";
import { App } from "./components/app.tsx";
import { vormaAppConfig } from "./vorma.gen/index.ts";

await initClient({
	vormaAppConfig,
	renderFn: () => {
		render(() => <App />, getRootEl());
	},
});

void import("./highlight.ts"); // warm up highlighter
void import("./html_to_md.ts"); // warm up markdown converter
void import("./components/md.tsx"); // warm up markdown route component
