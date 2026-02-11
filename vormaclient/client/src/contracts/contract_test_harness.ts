import { afterEach, beforeEach, vi } from "vitest";
import { createPatternRegistry } from "vorma/kit/matcher/register";

const VORMA_INTERNAL_SYMBOL = Symbol.for("__vorma_internal__");

type AnyRecord = Record<string, any>;
type AnyPropertyRecord = Record<PropertyKey, any>;

const DEFAULT_VORMA_APP_CONFIG = {
	actionsRouterMountRoot: "/api/",
	actionsDynamicRune: ":",
	actionsSplatRune: "*",
	loadersDynamicRune: ":",
	loadersSplatRune: "*",
	loadersExplicitIndexSegment: "_index",
};

export function installContractVormaGlobal(overrides: AnyRecord = {}): void {
	const globals: AnyRecord = {
		buildID: "1",
		matchedPatterns: [],
		loadersData: [],
		importURLs: [],
		exportKeys: [],
		errorExportKeys: [],
		hasRootData: false,
		params: {},
		splatValues: [],
		activeComponents: [],
		outermostServerError: undefined,
		outermostClientError: undefined,
		outermostServerErrorIdx: undefined,
		outermostClientErrorIdx: undefined,
		outermostError: undefined,
		outermostErrorIdx: undefined,
		isDev: false,
		viteDevURL: "",
		publicPathPrefix: "",
		isTouchDevice: false,
		patternToWaitFnMap: {},
		clientLoadersData: [],
		defaultErrorBoundary: () => null,
		useViewTransitions: false,
		deploymentID: "",
		vormaAppConfig: DEFAULT_VORMA_APP_CONFIG,
		routeManifestURL: "",
		routeManifest: undefined,
		clientModuleMap: {},
		patternRegistry: createPatternRegistry({
			dynamicParamPrefixRune: DEFAULT_VORMA_APP_CONFIG.loadersDynamicRune,
			splatSegmentRune: DEFAULT_VORMA_APP_CONFIG.loadersSplatRune,
			explicitIndexSegment:
				DEFAULT_VORMA_APP_CONFIG.loadersExplicitIndexSegment,
		}),
	};

	(globalThis as AnyPropertyRecord)[VORMA_INTERNAL_SYMBOL] = {
		...globals,
		...overrides,
	};
}

export function createRouteData(overrides: AnyRecord = {}): AnyRecord {
	return {
		matchedPatterns: [],
		loadersData: [],
		importURLs: [],
		exportKeys: [],
		errorExportKeys: [],
		hasRootData: false,
		params: {},
		splatValues: [],
		deps: [],
		cssBundles: [],
		outermostServerError: undefined,
		outermostServerErrorIdx: undefined,
		title: undefined,
		metaHeadEls: undefined,
		restHeadEls: undefined,
		activeComponents: undefined,
		...overrides,
	};
}

export function createRouteDataResponse(
	overrides: AnyRecord = {},
	init: ResponseInit = {},
): Response {
	return new Response(JSON.stringify(createRouteData(overrides)), {
		status: 200,
		headers: {
			"Content-Type": "application/json",
			"X-Vorma-Build-Id": "1",
			...init.headers,
		},
		...init,
	});
}

export function createDeferred<T>() {
	let resolve: (value: T) => void;
	let reject: (reason?: any) => void;
	const promise = new Promise<T>((res, rej) => {
		resolve = res;
		reject = rej;
	});
	return {
		promise,
		resolve: resolve!,
		reject: reject!,
	};
}

export function setupContractTestSuite(): void {
	beforeEach(() => {
		vi.useFakeTimers();
		vi.setSystemTime(new Date("2026-01-01T00:00:00.000Z"));

		document.body.innerHTML = "";
		document.head.innerHTML = "";
		document.title = "Initial Title";
		window.history.replaceState({}, "", "/");

		if (!globalThis.CSS) {
			(globalThis as AnyRecord).CSS = {};
		}
		if (!(globalThis as AnyRecord).CSS.escape) {
			(globalThis as AnyRecord).CSS.escape = (value: string) =>
				value.replace(/[!"#$%&'()*+,./:;<=>?@[\\\]^`{|}~]/g, "\\$&");
		}

		Object.defineProperty(window, "scrollTo", {
			value: vi.fn(),
			writable: true,
			configurable: true,
		});
		if (!Element.prototype.scrollIntoView) {
			Element.prototype.scrollIntoView = vi.fn();
		}

		vi.spyOn(console, "error").mockImplementation(() => {});
		vi.spyOn(console, "info").mockImplementation(() => {});
		vi.spyOn(console, "warn").mockImplementation(() => {});

		installContractVormaGlobal();
	});

	afterEach(async () => {
		await vi.runOnlyPendingTimersAsync();
		vi.useRealTimers();
		vi.restoreAllMocks();
		document.body.innerHTML = "";
		document.head.innerHTML = "";
	});
}
