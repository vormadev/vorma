import {
	createPatternRegistry,
	registerPattern,
} from "vorma/kit/matcher/register";
import { ensureNavigationRuntimeInitialized } from "../client.ts";
import { initHMR } from "../core/extras.ts";
import { ComponentLoader, setupClientLoaders } from "../core/render_runtime.ts";
import { HistoryManager } from "../platform/history.ts";
import { scrollStateManager } from "../platform/scroll.ts";
import { VORMA_HARD_RELOAD_QUERY_PARAM } from "../platform/url.ts";
import { defaultErrorBoundary } from "../ui/helpers.ts";
import {
	__vormaClientGlobal,
	type RouteErrorComponent,
	type VormaClientGlobal,
} from "./context.ts";
import type { VormaAppConfig } from "./helpers.ts";

type InitClientOptions = {
	defaultErrorBoundary?: RouteErrorComponent;
	useViewTransitions?: boolean;
};

type InitClientInput = InitClientOptions & {
	vormaAppConfig: VormaAppConfig;
	renderFn: () => void;
};

let beforeUnloadRegistered = false;
let inputModalityDetectionRegistered = false;
let latestRouteManifestProgressiveLoadID = 0;

type RouteManifestRecord = NonNullable<VormaClientGlobal["routeManifest"]>;

function onBeforeUnload(): void {
	scrollStateManager.savePageRefreshState();
}

function setTouchInputModalityActive(): void {
	if (__vormaClientGlobal.get("isTouchInputModalityActive")) {
		return;
	}
	__vormaClientGlobal.set("isTouchInputModalityActive", true);
}

function setFinePointerInputModalityActive(): void {
	if (!__vormaClientGlobal.get("isTouchInputModalityActive")) {
		return;
	}
	__vormaClientGlobal.set("isTouchInputModalityActive", false);
}

function onPointerModalityChanged(event: Event): void {
	const pointerType = (
		event as Event & {
			pointerType?: unknown;
		}
	).pointerType;
	if (typeof pointerType !== "string") {
		return;
	}

	const normalizedPointerType = pointerType.toLowerCase();
	if (normalizedPointerType === "touch") {
		setTouchInputModalityActive();
		return;
	}

	if (normalizedPointerType === "mouse" || normalizedPointerType === "pen") {
		setFinePointerInputModalityActive();
	}
}

function registerBeforeUnloadScrollStatePersistence(): void {
	if (beforeUnloadRegistered) return;
	window.addEventListener("beforeunload", onBeforeUnload);
	beforeUnloadRegistered = true;
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

function registerManifestPatterns(props: {
	manifest: RouteManifestRecord;
	patternRegistry: VormaClientGlobal["patternRegistry"];
}): void {
	const { manifest, patternRegistry } = props;
	for (const pattern of Object.keys(manifest)) {
		registerPattern(patternRegistry, pattern);
	}
}

function clonePatternRegistry(props: {
	patternRegistry: VormaClientGlobal["patternRegistry"];
}): VormaClientGlobal["patternRegistry"] {
	const { dynamicParamPrefixRune, splatSegmentRune, explicitIndexSegment } =
		props.patternRegistry.config;
	const nextPatternRegistry = createPatternRegistry({
		dynamicParamPrefixRune,
		splatSegmentRune,
		explicitIndexSegment,
	});

	for (const registeredPattern of props.patternRegistry.staticPatterns.values()) {
		registerPattern(nextPatternRegistry, registeredPattern.originalPattern);
	}
	for (const registeredPattern of props.patternRegistry.dynamicPatterns.values()) {
		registerPattern(nextPatternRegistry, registeredPattern.originalPattern);
	}

	return nextPatternRegistry;
}

function applyRouteManifestAtomicallyOrThrow(props: {
	manifest: RouteManifestRecord;
	patternRegistry: VormaClientGlobal["patternRegistry"];
}): void {
	const nextPatternRegistry = clonePatternRegistry({
		patternRegistry: props.patternRegistry,
	});
	registerManifestPatterns({
		manifest: props.manifest,
		patternRegistry: nextPatternRegistry,
	});
	__vormaClientGlobal.set("patternRegistry", nextPatternRegistry);
	__vormaClientGlobal.set("routeManifest", props.manifest);
}

function readPrecompiledRouteManifestOrNull(): RouteManifestRecord | null {
	const precompiledRouteManifest = __vormaClientGlobal.get("routeManifest");
	if (!precompiledRouteManifest) {
		return null;
	}

	return parseRouteManifestPayloadOrThrow(precompiledRouteManifest);
}

function initializePatternRegistryFromPrecompiledRouteManifest(): boolean {
	const manifest = readPrecompiledRouteManifestOrNull();
	if (!manifest) {
		return false;
	}

	const patternRegistry = __vormaClientGlobal.get("patternRegistry");
	applyRouteManifestAtomicallyOrThrow({
		manifest,
		patternRegistry,
	});
	return true;
}

async function loadRouteManifestProgressively(): Promise<void> {
	const manifestURL = __vormaClientGlobal.get("routeManifestURL");
	if (!manifestURL) {
		return;
	}

	const patternRegistry = __vormaClientGlobal.get("patternRegistry");
	const routeManifestProgressiveLoadID =
		++latestRouteManifestProgressiveLoadID;
	let response: Response;
	try {
		response = await fetch(manifestURL);
	} catch (error) {
		console.warn("Failed to load route manifest:", error);
		return;
	}
	if (!response.ok) {
		console.warn(
			"Failed to load route manifest:",
			new Error(
				`Route manifest request failed with status ${response.status}.`,
			),
		);
		return;
	}

	const manifestPayload = await response.json();
	const manifest = parseRouteManifestPayloadOrThrow(manifestPayload);

	if (
		routeManifestProgressiveLoadID !== latestRouteManifestProgressiveLoadID
	) {
		return;
	}

	if (__vormaClientGlobal.get("patternRegistry") !== patternRegistry) {
		return;
	}

	applyRouteManifestAtomicallyOrThrow({ manifest, patternRegistry });
}

function cleanupHardReloadQueryParam(): void {
	const url = new URL(window.location.href);
	if (!url.searchParams.has(VORMA_HARD_RELOAD_QUERY_PARAM)) {
		return;
	}

	url.searchParams.delete(VORMA_HARD_RELOAD_QUERY_PARAM);
	HistoryManager.getInstance().replace(url.href);
}

function registerInputModalityDetection(): void {
	if (inputModalityDetectionRegistered) {
		return;
	}

	window.addEventListener("touchstart", setTouchInputModalityActive);
	window.addEventListener("pointerdown", onPointerModalityChanged);
	window.addEventListener("pointermove", onPointerModalityChanged);
	window.addEventListener("pointerenter", onPointerModalityChanged);
	inputModalityDetectionRegistered = true;
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
	initializeClientPatternRegistry(options.vormaAppConfig);

	const didInitializePatternRegistryFromPrecompiledManifest =
		initializePatternRegistryFromPrecompiledRouteManifest();
	if (!didInitializePatternRegistryFromPrecompiledManifest) {
		void loadRouteManifestProgressively();
	}
	applyInitClientOptions(options);

	ensureNavigationRuntimeInitialized();
	HistoryManager.init();
	cleanupHardReloadQueryParam();

	const importURLs = __vormaClientGlobal.get("importURLs");
	await bootstrapInitialClientRuntime(importURLs);

	options.renderFn();
	scrollStateManager.restorePageRefreshState();
	registerInputModalityDetection();
}

export const __loadRouteManifestProgressively = loadRouteManifestProgressively;
