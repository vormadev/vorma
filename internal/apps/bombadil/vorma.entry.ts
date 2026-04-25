/// <reference types="vite/client" />

import { render_vorma, variant } from "#variant-runtime";
import { install_vorma_probe } from "./shared/instrumentation.ts";
import { app } from "./vorma.app.ts";

await install_vorma_probe({
	variant,
	app,
	render: render_vorma,
});
