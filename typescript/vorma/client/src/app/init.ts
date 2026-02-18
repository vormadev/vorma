import {
	createPatternRegistry,
	registerPattern,
} from "vorma/kit/matcher/register";
import { setupClientLoaders } from "../core/render_runtime.ts";
import { ComponentLoader } from "../core/render_runtime.ts";
import { buildClientModuleMapFromRouteModuleMetadata } from "../core/navigation/runtime_navigation_successful_runtime.ts";
import { defaultErrorBoundary } from "../ui/helpers.ts";
import { VORMA_HARD_RELOAD_QUERY_PARAM } from "../platform/url.ts";
import { HistoryManager } from "../platform/history.ts";
import { initHMR } from "../core/extras.ts";
import { scrollStateManager } from "../platform/scroll.ts";
import type { VormaAppConfig } from "./helpers.ts";
import {
	__vormaClientGlobal,
	type RouteErrorComponent,
	type VormaClientGlobal,
} from "./context.ts";

type InitClientOptions = {
	defaultErrorBoundary?: RouteErrorComponent;
	useViewTransitions?: boolean;
};

type InitClientInput = InitClientOptions & {
	vormaAppConfig: VormaAppConfig;
	renderFn: () => void;
};

let beforeUnloadRegistered = false;
let touchDetectionRegistered = false;
let latestRouteManifestProgressiveLoadID = 0;

type RouteManifestRecord = NonNullable<VormaClientGlobal["routeManifest"]>;

function onBeforeUnload(): void {
	scrollStateManager.savePageRefreshState();
}

function onFirstTouch(): void {
	__vormaClientGlobal.set("isTouchDevice", true);
}

function registerBeforeUnloadScrollStatePersistence(): void {
	if (beforeUnloadRegistered) return;
	window.addEventListener("beforeunload", onBeforeUnload);
	beforeUnloadRegistered = true;
}

function registerTouchDetection(): void {
	if (touchDetectionRegistered) return;
	window.addEventListener("touchstart", onFirstTouch, { once: true });
	touchDetectionRegistered = true;
}

function applyInitClientOptions(options: InitClientOptions): void {
	if (options.defaultErrorBoundary) {
		__vormaClientGlobal.set(
			"defaultErrorBoundary",
			options.defaultErrorBoundary,
		);
	} else {
		__vormaClientGlobal.set("defaultErrorBoundary", defaultErrorBoundary);
	}

	__vormaClientGlobal.set(
		"useViewTransitions",
		options.useViewTransitions === true,
	);
}

function initializeClientModuleMapFromInitialRouteState(): void {
	const clientModuleMap = buildClientModuleMapFromRouteModuleMetadata({
		routeModuleMetadata: {
			matchedPatterns: __vormaClientGlobal.get("matchedPatterns"),
			importURLs: __vormaClientGlobal.get("importURLs"),
			exportKeys: __vormaClientGlobal.get("exportKeys"),
			errorExportKeys: __vormaClientGlobal.get("errorExportKeys"),
		},
	});
	__vormaClientGlobal.set("clientModuleMap", clientModuleMap);
}

function initializeClientPatternRegistry(vormaAppConfig: VormaAppConfig): void {
	const patternRegistry = createPatternRegistry({
		dynamicParamPrefixRune: vormaAppConfig.loadersDynamicRune,
		splatSegmentRune: vormaAppConfig.loadersSplatRune,
		explicitIndexSegment:
			vormaAppConfig.loadersExplicitIndexSegmentIdentifier,
	});
	__vormaClientGlobal.set("patternRegistry", patternRegistry);
}

function parseRouteManifestPayloadOrThrow(
	manifestPayload: unknown,
): RouteManifestRecord {
	if (
		typeof manifestPayload !== "object" ||
		manifestPayload === null ||
		Array.isArray(manifestPayload)
	) {
		throw new Error(
			"Route manifest must be a non-null object with pattern keys.",
		);
	}

	const parsedManifest: RouteManifestRecord = {};
	for (const [pattern, loaderFlag] of Object.entries(manifestPayload)) {
		if (loaderFlag !== 0 && loaderFlag !== 1) {
			throw new Error(
				`Route manifest value for pattern '${pattern}' must be 0 or 1.`,
			);
		}
		parsedManifest[pattern] = loaderFlag;
	}

	return parsedManifest;
}

function loadRouteManifestProgressively(): void {
	const manifestURL = __vormaClientGlobal.get("routeManifestURL");
	if (!manifestURL) {
		return;
	}

	const patternRegistry = __vormaClientGlobal.get("patternRegistry");
	const routeManifestProgressiveLoadID =
		++latestRouteManifestProgressiveLoadID;

	fetch(manifestURL)
		.then((response) => {
			if (!response.ok) {
				throw new Error(
					`Route manifest request failed with status ${response.status}.`,
				);
			}
			return response.json();
		})
		.then((manifestPayload) => {
			const manifest = parseRouteManifestPayloadOrThrow(manifestPayload);

			if (
				routeManifestProgressiveLoadID !==
				latestRouteManifestProgressiveLoadID
			) {
				return;
			}

			if (
				__vormaClientGlobal.get("patternRegistry") !== patternRegistry
			) {
				return;
			}

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

function cleanupHardReloadQueryParam(): void {
	const url = new URL(window.location.href);
	if (!url.searchParams.has(VORMA_HARD_RELOAD_QUERY_PARAM)) {
		return;
	}

	url.searchParams.delete(VORMA_HARD_RELOAD_QUERY_PARAM);
	HistoryManager.getInstance().replace(url.href);
}

async function bootstrapInitialClientRuntime(
	importURLs: Array<string> | undefined,
): Promise<void> {
	await ComponentLoader.handleComponents(importURLs);
	await setupClientLoaders();
	await ComponentLoader.handleErrorBoundaryComponent(importURLs);
}

export async function initClient(options: InitClientInput): Promise<void> {
	initHMR();

	registerBeforeUnloadScrollStatePersistence();

	__vormaClientGlobal.set("vormaAppConfig", options.vormaAppConfig);
	initializeClientModuleMapFromInitialRouteState();
	initializeClientPatternRegistry(options.vormaAppConfig);

	loadRouteManifestProgressively();
	applyInitClientOptions(options);

	HistoryManager.init();
	cleanupHardReloadQueryParam();

	const importURLs = __vormaClientGlobal.get("importURLs");
	await bootstrapInitialClientRuntime(importURLs);

	options.renderFn();
	scrollStateManager.restorePageRefreshState();
	registerTouchDetection();
}
