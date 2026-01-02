import { render } from "solid-js/web";
import { getRootEl, initClient } from "vorma/client";
import { App } from "./components/app.tsx";
import { vormaAppConfig } from "./vorma.gen.ts";

await initClient({
	vormaAppConfig,
	renderFn: () => {
		render(() => <App />, getRootEl());
	},
});

import("./highlight.ts"); // warm up highlighter
import("./html_to_md.ts"); // warm up markdown converter
import("./components/md.tsx"); // warm up  markdown route component
