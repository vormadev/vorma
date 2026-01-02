import {
	createPatternRegistry,
	registerPattern,
} from "vorma/kit/matcher/register";
import { setupClientLoaders } from "./client_loaders.ts";
import { ComponentLoader } from "./component_loader.ts";
import { defaultErrorBoundary } from "./error_boundary.ts";
import { VORMA_HARD_RELOAD_QUERY_PARAM } from "./hard_reload.ts";
import { HistoryManager } from "./history/history.ts";
import { initHMR } from "./hmr/hmr.ts";
import { scrollStateManager } from "./scroll_state_manager.ts";
import type { VormaAppConfig } from "./vorma_app_helpers/vorma_app_helpers.ts";
import {
	__vormaClientGlobal,
	type RouteErrorComponent,
	type VormaClientGlobal,
} from "./vorma_ctx/vorma_ctx.ts";

export async function initClient(options: {
	vormaAppConfig: VormaAppConfig;
	renderFn: () => void;
	defaultErrorBoundary?: RouteErrorComponent;
	useViewTransitions?: boolean;
}): Promise<void> {
	initHMR();

	// Setup beforeunload handler for scroll restoration
	window.addEventListener("beforeunload", () => {
		scrollStateManager.savePageRefreshState();
	});

	__vormaClientGlobal.set("vormaAppConfig", options.vormaAppConfig);
	const clientModuleMap: VormaClientGlobal["clientModuleMap"] = {};

	// Populate client module map with initial page's modules
	const initialMatchedPatterns =
		__vormaClientGlobal.get("matchedPatterns") || [];
	const initialImportURLs = __vormaClientGlobal.get("importURLs") || [];
	const initialExportKeys = __vormaClientGlobal.get("exportKeys") || [];
	const initialErrorExportKeys =
		__vormaClientGlobal.get("errorExportKeys") || [];

	for (let i = 0; i < initialMatchedPatterns.length; i++) {
		const pattern = initialMatchedPatterns[i];
		const importURL = initialImportURLs[i];
		const exportKey = initialExportKeys[i];
		const errorExportKey = initialErrorExportKeys[i];

		if (pattern && importURL) {
			clientModuleMap[pattern] = {
				importURL,
				exportKey: exportKey || "default",
				errorExportKey: errorExportKey || "",
			};
		}
	}
	__vormaClientGlobal.set("clientModuleMap", clientModuleMap);

	const patternRegistry = createPatternRegistry({
		dynamicParamPrefixRune: options.vormaAppConfig.loadersDynamicRune,
		splatSegmentRune: options.vormaAppConfig.loadersSplatRune,
		explicitIndexSegment:
			options.vormaAppConfig.loadersExplicitIndexSegment,
	});
	__vormaClientGlobal.set("patternRegistry", patternRegistry);

	const manifestURL = __vormaClientGlobal.get("routeManifestURL");
	if (manifestURL) {
		fetch(manifestURL)
			.then((response) => response.json())
			.then((manifest) => {
				__vormaClientGlobal.set("routeManifest", manifest);

				// Register all patterns from manifest into the existing registry
				for (const pattern of Object.keys(manifest)) {
					registerPattern(patternRegistry, pattern);
				}
			})
			.catch((error) => {
				// This is no biggie -- it's a progressive enhancement
				console.warn("Failed to load route manifest:", error);
			});
	}

	// Set options
	if (options.defaultErrorBoundary) {
		__vormaClientGlobal.set(
			"defaultErrorBoundary",
			options.defaultErrorBoundary,
		);
	} else {
		__vormaClientGlobal.set("defaultErrorBoundary", defaultErrorBoundary);
	}

	if (options.useViewTransitions) {
		__vormaClientGlobal.set("useViewTransitions", true);
	}

	// Initialize history
	HistoryManager.init();

	// Clean URL
	const url = new URL(window.location.href);
	if (url.searchParams.has(VORMA_HARD_RELOAD_QUERY_PARAM)) {
		url.searchParams.delete(VORMA_HARD_RELOAD_QUERY_PARAM);
		HistoryManager.getInstance().replace(url.href);
	}

	const importURLs = __vormaClientGlobal.get("importURLs");

	// Load initial components
	await ComponentLoader.handleComponents(importURLs);

	// Setup client loaders
	await setupClientLoaders();

	// Handle error boundary component (must come after setupClientLoaders)
	await ComponentLoader.handleErrorBoundaryComponent(importURLs);

	// Render
	options.renderFn();

	// Restore scroll
	scrollStateManager.restorePageRefreshState();

	// Touch detection
	window.addEventListener(
		"touchstart",
		() => {
			__vormaClientGlobal.set("isTouchDevice", true);
		},
		{ once: true },
	);
}
