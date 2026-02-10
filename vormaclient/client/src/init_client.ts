import { HistoryManager } from "./history/history.ts";
import { bootstrapInitialClientRuntime } from "./init_client_bootstrap.ts";
import {
	registerBeforeUnloadScrollStatePersistence,
	registerTouchDetection,
} from "./init_client_events.ts";
import { initHMR } from "./hmr/hmr.ts";
import { loadRouteManifestProgressively } from "./init_client_manifest.ts";
import { initializeClientModuleMapFromInitialRouteState } from "./init_client_module_map.ts";
import {
	applyInitClientOptions,
	type InitClientOptions,
} from "./init_client_options.ts";
import { initializeClientPatternRegistry } from "./init_client_pattern_registry.ts";
import { cleanupHardReloadQueryParam } from "./init_client_url_cleanup.ts";
import { scrollStateManager } from "./scroll_state_manager.ts";
import type { VormaAppConfig } from "./vorma_app_helpers/vorma_app_helpers.ts";
import { __vormaClientGlobal } from "./vorma_ctx/vorma_ctx.ts";

type InitClientInput = InitClientOptions & {
	vormaAppConfig: VormaAppConfig;
	renderFn: () => void;
};

export async function initClient(options: InitClientInput): Promise<void> {
	initHMR();

	// Setup beforeunload handler for scroll restoration
	registerBeforeUnloadScrollStatePersistence();

	__vormaClientGlobal.set("vormaAppConfig", options.vormaAppConfig);
	initializeClientModuleMapFromInitialRouteState();
	initializeClientPatternRegistry(options.vormaAppConfig);

	loadRouteManifestProgressively();
	applyInitClientOptions(options);

	// Initialize history
	HistoryManager.init();

	// Clean URL
	cleanupHardReloadQueryParam();

	const importURLs = __vormaClientGlobal.get("importURLs");

	await bootstrapInitialClientRuntime(importURLs);

	// Render
	options.renderFn();

	// Restore scroll
	scrollStateManager.restorePageRefreshState();

	// Touch detection
	registerTouchDetection();
}
