import {
	getRootEl,
	initClient,
	setupGlobalLoadingIndicator,
} from "vorma/client";
import { VormaRootOutlet } from "vorma/solid";
import { render } from "solid-js/web";
import { installE2EBridge } from "./vorma.bindings.ts";
import { vormaAppConfig } from "./vorma.gen/index.ts";
import "./styles/main.css";

let isGlobalLoadingIndicatorRunning = false;
document.documentElement.setAttribute("data-e2e-loading", "0");
setupGlobalLoadingIndicator({
	start: () => {
		isGlobalLoadingIndicatorRunning = true;
		document.documentElement.setAttribute("data-e2e-loading", "1");
	},
	stop: () => {
		isGlobalLoadingIndicatorRunning = false;
		document.documentElement.setAttribute("data-e2e-loading", "0");
	},
	isRunning: () => isGlobalLoadingIndicatorRunning,
});

await initClient({
	vormaAppConfig,
	renderFn: () => {
		render(() => <VormaRootOutlet />, getRootEl());
	},
});

installE2EBridge();
