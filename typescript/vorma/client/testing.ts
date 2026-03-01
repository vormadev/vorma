import { createPatternRegistry } from "vorma/kit/matcher/register";
import {
	createVormaRuntimeContext,
	registerClientLoaderForAdapter,
	revalidate,
} from "./src/runtime.ts";

/**
 * Test-only runtime helpers for black-box client tests.
 *
 * This entrypoint intentionally provides semantic setup/teardown helpers so
 * tests do not need to touch internal globals, symbols, or storage key names.
 */

const VORMA_TESTING_SYMBOL = Symbol.for("__vorma_internal__");
const SCROLL_STATE_STORAGE_KEY = "__vorma__scrollStateMap";
const PAGE_REFRESH_SCROLL_STATE_STORAGE_KEY = "__vorma__pageRefreshScrollState";

type TestingGlobalRecord = Record<PropertyKey, unknown>;
type TestingRuntimeGlobalState = {
	removeHistoryListener?: () => void;
	historyInstance?: unknown;
	lastKnownHistoryLocationKey?: unknown;
	lastKnownHistoryLocationHref?: unknown;
	vormaAppConfig?: {
		loadersDynamicRune: string;
		loadersSplatRune: string;
		loadersExplicitIndexSegmentIdentifier: string;
	};
	patternRegistry?: unknown;
	routeManifest?: unknown;
	isTouchInputModalityActive?: unknown;
	runtimeRouteSnapshot?: {
		importURLs?: unknown;
	};
	windowEventListenersByEventName?: Map<string, Set<EventListener>>;
	hardRedirectForTesting?: ((href: string) => void) | undefined;
};

export type TestingStoredScrollStateEntry = {
	historyKey: string;
	x: number;
	y: number;
};

export type TestingPageRefreshScrollState = {
	x: number;
	y: number;
	unix: number;
	href: string;
};

export type ResetClientRuntimeForTestingOptions = {
	clearPersistedScrollState?: boolean;
};

export type SeedRuntimeRouteSnapshotForTestingInput = {
	matchedPatterns?: string[];
	loadersData?: unknown[];
	importURLs?: string[];
	exportKeys?: string[];
	errorExportKeys?: string[];
	hasRootData?: boolean;
	params?: Record<string, string>;
	splatValues?: string[];
	deps?: string[];
	cssBundles?: string[];
	title?: { dangerousInnerHTML: string } | undefined;
	metaHeadEls?: unknown[];
	restHeadEls?: unknown[];
	outermostServerError?: unknown;
	outermostServerErrorIdx?: number | null | undefined;
	rootElementID?: string;
};

function clearRuntimeListenersForTesting(): void {
	const runtimeGlobalState = (globalThis as TestingGlobalRecord)[
		VORMA_TESTING_SYMBOL
	] as TestingRuntimeGlobalState | undefined;
	const listenersByEventName =
		runtimeGlobalState?.windowEventListenersByEventName;
	if (listenersByEventName) {
		listenersByEventName.forEach((listenersForEventName, eventName) => {
			listenersForEventName.forEach((listener) => {
				window.removeEventListener(eventName, listener);
			});
		});
		listenersByEventName.clear();
	}
	runtimeGlobalState?.removeHistoryListener?.();
	if (!runtimeGlobalState) {
		return;
	}
	runtimeGlobalState.removeHistoryListener = undefined;
	runtimeGlobalState.historyInstance = undefined;
	runtimeGlobalState.lastKnownHistoryLocationKey = undefined;
	runtimeGlobalState.lastKnownHistoryLocationHref = undefined;
}

export function resetClientRuntimeForTesting(
	options: ResetClientRuntimeForTestingOptions = {},
): void {
	const { clearPersistedScrollState = true } = options;
	clearRuntimeListenersForTesting();
	delete (globalThis as TestingGlobalRecord)[VORMA_TESTING_SYMBOL];
	if (!clearPersistedScrollState) {
		return;
	}
	window.sessionStorage.removeItem(SCROLL_STATE_STORAGE_KEY);
	window.sessionStorage.removeItem(PAGE_REFRESH_SCROLL_STATE_STORAGE_KEY);
}

export function createIsolatedClientTestRuntime(
	options: ResetClientRuntimeForTestingOptions = {},
): {
	reset: () => void;
} {
	resetClientRuntimeForTesting(options);
	return {
		reset: () => resetClientRuntimeForTesting(options),
	};
}

export function seedRuntimeRouteSnapshotForTesting(
	input: SeedRuntimeRouteSnapshotForTestingInput,
): void {
	createVormaRuntimeContext();
	const runtimeGlobal = (globalThis as TestingGlobalRecord)[
		VORMA_TESTING_SYMBOL
	] as
		| {
				runtimeRouteSnapshot?: Record<string, unknown>;
		  }
		| undefined;
	if (
		!runtimeGlobal ||
		typeof runtimeGlobal !== "object" ||
		!runtimeGlobal.runtimeRouteSnapshot ||
		typeof runtimeGlobal.runtimeRouteSnapshot !== "object"
	) {
		throw new Error(
			"Vorma runtime snapshot must exist before seeding test route snapshot.",
		);
	}
	const existingSnapshot = runtimeGlobal.runtimeRouteSnapshot;
	const matchedPatterns =
		input.matchedPatterns ??
		(Array.isArray(existingSnapshot.matchedPatterns)
			? (existingSnapshot.matchedPatterns as string[])
			: []);
	const routeCount = matchedPatterns.length;

	runtimeGlobal.runtimeRouteSnapshot = {
		...existingSnapshot,
		matchedPatterns,
		loadersData:
			input.loadersData ?? Array.from({ length: routeCount }, () => null),
		importURLs:
			input.importURLs ?? Array.from({ length: routeCount }, () => ""),
		exportKeys:
			input.exportKeys ?? Array.from({ length: routeCount }, () => ""),
		errorExportKeys:
			input.errorExportKeys ??
			Array.from({ length: routeCount }, () => ""),
		hasRootData: input.hasRootData ?? false,
		params: input.params ?? {},
		splatValues: input.splatValues ?? [],
		deps: input.deps ?? [],
		cssBundles: input.cssBundles ?? [],
		title: input.title,
		metaHeadEls: input.metaHeadEls ?? [],
		restHeadEls: input.restHeadEls ?? [],
		outermostServerError: input.outermostServerError,
		outermostServerErrorIdx: input.outermostServerErrorIdx,
		rootElementID: input.rootElementID ?? existingSnapshot.rootElementID,
	};
}

export function setDeploymentIDForTesting(deploymentID: string): void {
	const runtimeGlobal = (globalThis as TestingGlobalRecord)[
		VORMA_TESTING_SYMBOL
	] as
		| {
				deploymentID?: unknown;
		  }
		| undefined;
	if (!runtimeGlobal || typeof runtimeGlobal !== "object") {
		throw new Error(
			"Vorma runtime must be initialized before setting deploymentID.",
		);
	}
	runtimeGlobal.deploymentID = deploymentID;
}

export function setHardRedirectHandlerForTesting(
	hardRedirectHandler: ((href: string) => void) | undefined,
): void {
	createVormaRuntimeContext();
	const runtimeGlobal = (globalThis as TestingGlobalRecord)[
		VORMA_TESTING_SYMBOL
	] as TestingRuntimeGlobalState | undefined;
	if (!runtimeGlobal || typeof runtimeGlobal !== "object") {
		throw new Error(
			"Vorma runtime must be initialized before setting hard redirect handler.",
		);
	}
	runtimeGlobal.hardRedirectForTesting = hardRedirectHandler;
}

export function setRouteManifestForTesting(
	manifest: Record<string, unknown> | undefined,
): void {
	createVormaRuntimeContext();
	const runtimeGlobal = (globalThis as TestingGlobalRecord)[
		VORMA_TESTING_SYMBOL
	] as TestingRuntimeGlobalState | undefined;
	if (!runtimeGlobal || typeof runtimeGlobal !== "object") {
		throw new Error(
			"Vorma runtime must be initialized before setting route manifest.",
		);
	}
	runtimeGlobal.routeManifest = manifest;
}

export function readRouteManifestForTesting():
	| Record<string, unknown>
	| undefined {
	createVormaRuntimeContext();
	const runtimeGlobal = (globalThis as TestingGlobalRecord)[
		VORMA_TESTING_SYMBOL
	] as TestingRuntimeGlobalState | undefined;
	if (!runtimeGlobal || typeof runtimeGlobal !== "object") {
		throw new Error(
			"Vorma runtime must be initialized before reading route manifest.",
		);
	}
	return runtimeGlobal.routeManifest as Record<string, unknown> | undefined;
}

export function replacePatternRegistryForTesting(): void {
	createVormaRuntimeContext();
	const runtimeGlobal = (globalThis as TestingGlobalRecord)[
		VORMA_TESTING_SYMBOL
	] as TestingRuntimeGlobalState | undefined;
	if (!runtimeGlobal || typeof runtimeGlobal !== "object") {
		throw new Error(
			"Vorma runtime must be initialized before replacing pattern registry.",
		);
	}
	const vormaAppConfig = runtimeGlobal.vormaAppConfig;
	if (!vormaAppConfig) {
		throw new Error(
			"Vorma app config must be initialized before replacing pattern registry.",
		);
	}
	runtimeGlobal.patternRegistry = createPatternRegistry({
		dynamicParamPrefixRune: vormaAppConfig.loadersDynamicRune,
		splatSegmentRune: vormaAppConfig.loadersSplatRune,
		explicitIndexSegment:
			vormaAppConfig.loadersExplicitIndexSegmentIdentifier,
	});
}

export function readIsTouchInputModalityActiveForTesting(): boolean {
	createVormaRuntimeContext();
	const runtimeGlobal = (globalThis as TestingGlobalRecord)[
		VORMA_TESTING_SYMBOL
	] as TestingRuntimeGlobalState | undefined;
	if (!runtimeGlobal || typeof runtimeGlobal !== "object") {
		throw new Error(
			"Vorma runtime must be initialized before reading input modality.",
		);
	}
	return runtimeGlobal.isTouchInputModalityActive === true;
}

export function registerClientLoaderForTesting(props: {
	pattern: string;
	clientLoader: (input: unknown) => Promise<unknown>;
}): void {
	createVormaRuntimeContext();
	registerClientLoaderForAdapter({
		pattern: props.pattern,
		clientLoader: props.clientLoader,
	});
}

export function clearAllNavigationStateForTesting(): void {
	createVormaRuntimeContext().navigationStateManager.clearAll();
}

export async function simulateViteAfterUpdateForTesting(props: {
	updates: Array<{ type: string; path: string }>;
}): Promise<void> {
	createVormaRuntimeContext();
	const runtimeGlobal = (globalThis as TestingGlobalRecord)[
		VORMA_TESTING_SYMBOL
	] as TestingRuntimeGlobalState | undefined;
	if (!runtimeGlobal || typeof runtimeGlobal !== "object") {
		throw new Error(
			"Vorma runtime must be initialized before simulating Vite updates.",
		);
	}
	const currentImportURLs = Array.isArray(
		runtimeGlobal.runtimeRouteSnapshot?.importURLs,
	)
		? (runtimeGlobal.runtimeRouteSnapshot?.importURLs as string[])
		: [];
	const { applyViteAfterUpdatePayload } =
		await import("./src/runtime_hmr_dev.ts");
	applyViteAfterUpdatePayload({
		updates: props.updates,
		currentImportURLs,
		triggerRevalidate: () => {
			void revalidate().catch(() => undefined);
		},
	});
}

export function seedScrollStateForTesting(
	entries: TestingStoredScrollStateEntry[],
): void {
	window.sessionStorage.setItem(
		SCROLL_STATE_STORAGE_KEY,
		JSON.stringify(
			entries.map((entry) => [
				entry.historyKey,
				{
					x: entry.x,
					y: entry.y,
				},
			]),
		),
	);
}

export function writeRawScrollStateStorageForTesting(raw: string | null): void {
	if (raw === null) {
		window.sessionStorage.removeItem(SCROLL_STATE_STORAGE_KEY);
		return;
	}
	window.sessionStorage.setItem(SCROLL_STATE_STORAGE_KEY, raw);
}

export function readScrollStateForTesting(): TestingStoredScrollStateEntry[] {
	const raw = window.sessionStorage.getItem(SCROLL_STATE_STORAGE_KEY);
	if (!raw) {
		return [];
	}
	let parsed: unknown;
	try {
		parsed = JSON.parse(raw);
	} catch {
		return [];
	}
	if (!Array.isArray(parsed)) {
		return [];
	}
	const result: TestingStoredScrollStateEntry[] = [];
	for (const entry of parsed) {
		if (!Array.isArray(entry) || entry.length !== 2) {
			continue;
		}
		const [historyKey, state] = entry;
		if (
			typeof historyKey !== "string" ||
			!state ||
			typeof state !== "object" ||
			typeof (state as { x?: unknown }).x !== "number" ||
			typeof (state as { y?: unknown }).y !== "number"
		) {
			continue;
		}
		result.push({
			historyKey,
			x: (state as { x: number }).x,
			y: (state as { y: number }).y,
		});
	}
	return result;
}

export function seedPageRefreshScrollStateForTesting(
	state: TestingPageRefreshScrollState | null,
): void {
	if (!state) {
		window.sessionStorage.removeItem(PAGE_REFRESH_SCROLL_STATE_STORAGE_KEY);
		return;
	}
	window.sessionStorage.setItem(
		PAGE_REFRESH_SCROLL_STATE_STORAGE_KEY,
		JSON.stringify({
			x: state.x,
			y: state.y,
			unix: state.unix,
			href: state.href,
		}),
	);
}

export function readPageRefreshScrollStateForTesting(): TestingPageRefreshScrollState | null {
	const raw = window.sessionStorage.getItem(
		PAGE_REFRESH_SCROLL_STATE_STORAGE_KEY,
	);
	if (!raw) {
		return null;
	}
	let parsed: unknown;
	try {
		parsed = JSON.parse(raw);
	} catch {
		return null;
	}
	if (!parsed || typeof parsed !== "object") {
		return null;
	}
	const maybeState = parsed as {
		x?: unknown;
		y?: unknown;
		unix?: unknown;
		href?: unknown;
	};
	if (
		typeof maybeState.x !== "number" ||
		typeof maybeState.y !== "number" ||
		typeof maybeState.unix !== "number" ||
		typeof maybeState.href !== "string"
	) {
		return null;
	}
	return {
		x: maybeState.x,
		y: maybeState.y,
		unix: maybeState.unix,
		href: maybeState.href,
	};
}

export function isScrollStateStorageKeyForTesting(key: string): boolean {
	return key === SCROLL_STATE_STORAGE_KEY;
}

export function isPageRefreshScrollStateStorageKeyForTesting(
	key: string,
): boolean {
	return key === PAGE_REFRESH_SCROLL_STATE_STORAGE_KEY;
}
