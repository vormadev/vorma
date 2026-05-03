/// <reference types="vite/client" />

import { variant } from "#variant-runtime";
import { install_vorma_probe } from "./shared/instrumentation.ts";
import "./shared/styles/main.css";
import { app } from "./vorma.app.ts";

await install_vorma_probe({
	variant,
	app,
});
