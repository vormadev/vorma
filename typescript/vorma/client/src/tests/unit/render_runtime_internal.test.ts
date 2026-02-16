import { beforeEach, describe, expect, it, vi } from "vitest";
import {
	__registerClientLoaderPattern,
	__reRenderApp,
	buildClientLoaderServerData,
	ComponentLoader,
	completeClientLoaders,
	deriveAndSetErrorState,
	findPartialMatchesOnClient,
	setClientLoadersState,
	setupClientLoaders,
} from "../../core/render_runtime.ts";
import { VORMA_SYMBOL, __vormaClientGlobal } from "../../app/context.ts";
import {
	createPatternRegistry,
	registerPattern,
} from "vorma/kit/matcher/register";
import { VORMA_ROUTE_CHANGE_EVENT_KEY } from "../../platform/events.ts";
import * as headModule from "../../ui/head.ts";

const TEST_VORMA_APP_CONFIG = {
	actionsRouterMountRoot: "/api/",
	actionsDynamicRune: ":",
	actionsSplatRune: "*",
	loadersDynamicRune: ":",
	loadersSplatRune: "*",
	loadersExplicitIndexSegment: "_index",
};

function createRegisteredPatternRegistry(patterns: string[]) {
	const registry = createPatternRegistry({
		dynamicParamPrefixRune: TEST_VORMA_APP_CONFIG.loadersDynamicRune,
		splatSegmentRune: TEST_VORMA_APP_CONFIG.loadersSplatRune,
		explicitIndexSegment: TEST_VORMA_APP_CONFIG.loadersExplicitIndexSegment,
	});
	for (const pattern of patterns) {
		registerPattern(registry, pattern);
	}
	return registry;
}

function installVormaGlobal(overrides: Record<string, unknown> = {}): void {
	(globalThis as any)[VORMA_SYMBOL] = {
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
		activeErrorBoundary: undefined,
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
		vormaAppConfig: TEST_VORMA_APP_CONFIG,
		routeManifestURL: "",
		routeManifest: undefined,
		clientModuleMap: {},
		patternRegistry: createRegisteredPatternRegistry([]),
		...overrides,
	};
}

function createRouteDataJSON(
	overrides: Record<string, unknown> = {},
): Record<string, unknown> {
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
		...overrides,
	};
}

beforeEach(() => {
	document.body.innerHTML = "";
	document.head.innerHTML = "";
	window.history.replaceState({}, "", "/");
	installVormaGlobal();
	vi.restoreAllMocks();
});

describe("render runtime internals", () => {
	it("derives effective error from the lower server/client error index", () => {
		installVormaGlobal({
			outermostServerErrorIdx: 5,
			outermostServerError: "server-error",
			outermostClientErrorIdx: 2,
			outermostClientError: "client-error",
		});

		deriveAndSetErrorState();

		expect(__vormaClientGlobal.get("outermostErrorIdx")).toBe(2);
		expect(__vormaClientGlobal.get("outermostError")).toBe("client-error");
	});

	it("clears derived error state when no server/client error exists", () => {
		installVormaGlobal({
			outermostServerErrorIdx: undefined,
			outermostClientErrorIdx: undefined,
		});

		deriveAndSetErrorState();

		expect(__vormaClientGlobal.get("outermostErrorIdx")).toBeUndefined();
		expect(__vormaClientGlobal.get("outermostError")).toBeUndefined();
	});

	it("loads component modules while tolerating empty import URLs", async () => {
		vi.doMock("/rt-module.js", () => ({
			default: "module-default",
		}));

		const modules = await ComponentLoader.loadComponents([
			"",
			"/rt-module.js",
		]);
		expect(modules.get("")).toBeUndefined();
		expect(modules.get("/rt-module.js")?.default).toBe("module-default");
	});

	it("builds active components using default export keys and null fallback", async () => {
		const defaultComponent = () => "A";
		vi.doMock("/a.js", () => ({ default: defaultComponent }));
		vi.doMock("/b.js", () => ({ MissingExport: null }));
		installVormaGlobal({
			importURLs: ["/a.js", "/b.js"],
			exportKeys: [undefined, "MissingExport"],
			activeComponents: [],
		});

		await ComponentLoader.handleComponents(["/a.js", "/b.js"]);

		expect(__vormaClientGlobal.get("activeComponents")).toEqual([
			defaultComponent,
			null,
		]);
	});

	it("skips error-boundary updates when no effective error index exists", async () => {
		vi.doMock("/err.js", () => ({ ErrorBoundary: () => "err" }));
		installVormaGlobal({
			importURLs: ["/err.js"],
			outermostServerErrorIdx: undefined,
			outermostClientErrorIdx: undefined,
		});

		await ComponentLoader.handleErrorBoundaryComponent(["/err.js"]);
		expect(__vormaClientGlobal.get("activeErrorBoundary")).toBeUndefined();
	});

	it("does not rewrite activeErrorBoundary when it is unchanged", async () => {
		const boundary = () => "same";
		const setSpy = vi.spyOn(__vormaClientGlobal, "set");
		installVormaGlobal({
			importURLs: ["/same.js"],
			errorExportKeys: ["ErrorBoundary"],
			outermostServerErrorIdx: 0,
			activeErrorBoundary: boundary,
		});

		await ComponentLoader.handleErrorBoundaryComponent(
			["/same.js"],
			new Map([
				[
					"/same.js",
					{
						ErrorBoundary: boundary,
					},
				],
			]),
		);

		expect(
			setSpy.mock.calls.some(
				(call) =>
					call[0] === "activeErrorBoundary" && call[1] === boundary,
			),
		).toBe(false);
	});

	it("falls back to the default error boundary when error export keys are missing", async () => {
		const defaultBoundary = () => "default-boundary";
		installVormaGlobal({
			importURLs: ["/missing-error-key.js"],
			errorExportKeys: undefined,
			outermostServerErrorIdx: 0,
			defaultErrorBoundary: defaultBoundary,
			activeErrorBoundary: undefined,
		});

		await ComponentLoader.handleErrorBoundaryComponent(
			["/missing-error-key.js"],
			new Map([
				[
					"/missing-error-key.js",
					{
						ErrorBoundary: () => "unused-boundary",
					},
				],
			]),
		);

		expect(__vormaClientGlobal.get("activeErrorBoundary")).toBe(
			defaultBoundary,
		);
	});

	it("returns null when client-loader server data is missing pattern match", () => {
		const result = buildClientLoaderServerData({
			pattern: "/missing",
			matchedPatterns: ["/present"],
			loadersData: [{ value: "x" }],
			hasRootData: false,
			buildID: "1",
		});
		expect(result).toBeNull();
	});

	it("returns null when root data is required but unavailable", () => {
		const result = buildClientLoaderServerData({
			pattern: "/present",
			matchedPatterns: ["/present"],
			loadersData: [undefined],
			hasRootData: true,
			buildID: "1",
		});
		expect(result).toBeNull();
	});

	it("propagates aborted parent signal to client loader controllers", async () => {
		const waitFn = vi.fn(async ({ signal, serverDataPromise }) => {
			await serverDataPromise;
			return signal.aborted;
		});
		installVormaGlobal({
			patternToWaitFnMap: { "/a": waitFn },
			routeManifest: { "/a": 0 },
		});
		const parentController = new AbortController();
		parentController.abort();

		const result = await completeClientLoaders(
			{
				matchedPatterns: ["/a"],
				loadersData: [{ value: true }],
				hasRootData: false,
				importURLs: [],
				params: {},
				splatValues: [],
			},
			"1",
			new Map(),
			parentController.signal,
		);

		expect(result).toEqual({
			data: [true],
			errorMessage: undefined,
		});
	});

	it("reports non-Error client loader failures using string conversion", async () => {
		const consoleErrorSpy = vi
			.spyOn(console, "error")
			.mockImplementation(() => {});
		installVormaGlobal({
			patternToWaitFnMap: {
				"/a": () => Promise.reject("boom"),
			},
			routeManifest: { "/a": 0 },
		});

		const result = await completeClientLoaders(
			{
				matchedPatterns: ["/a"],
				loadersData: [{}],
				hasRootData: false,
				importURLs: [],
				params: {},
				splatValues: [],
			},
			"1",
			new Map(),
			new AbortController().signal,
		);

		expect(result).toEqual({
			data: [undefined],
			errorMessage: "boom",
		});
		expect(consoleErrorSpy).toHaveBeenCalled();
		consoleErrorSpy.mockRestore();
	});

	it("aborts downstream client loaders when an earlier loader fails with a non-abort error", async () => {
		const consoleErrorSpy = vi
			.spyOn(console, "error")
			.mockImplementation(() => {});
		let secondLoaderObservedAbort = false;
		installVormaGlobal({
			patternToWaitFnMap: {
				"/a": () => Promise.reject(new Error("first-loader-failed")),
				"/b": ({ signal }: { signal: AbortSignal }) =>
					new Promise((_resolve, reject) => {
						if (signal.aborted) {
							secondLoaderObservedAbort = true;
							const abortError = new Error("Aborted");
							abortError.name = "AbortError";
							reject(abortError);
							return;
						}

						signal.addEventListener(
							"abort",
							() => {
								secondLoaderObservedAbort = true;
								const abortError = new Error("Aborted");
								abortError.name = "AbortError";
								reject(abortError);
							},
							{ once: true },
						);
					}),
			},
			routeManifest: { "/a": 0, "/b": 0 },
		});

		const result = await completeClientLoaders(
			{
				matchedPatterns: ["/a", "/b"],
				loadersData: [{}, {}],
				hasRootData: false,
				importURLs: [],
				params: {},
				splatValues: [],
			},
			"1",
			new Map(),
			new AbortController().signal,
		);

		expect(result).toEqual({
			data: [undefined],
			errorMessage: "first-loader-failed",
		});
		expect(secondLoaderObservedAbort).toBe(true);
		expect(consoleErrorSpy).toHaveBeenCalled();
		consoleErrorSpy.mockRestore();
	});

	it("finds partial matches when the full pathname is not registered", async () => {
		installVormaGlobal({
			patternRegistry: createRegisteredPatternRegistry(["/docs"]),
			patternToWaitFnMap: {
				"/docs": async () => undefined,
			},
		});

		const result = await findPartialMatchesOnClient("/docs/a/b");
		expect(
			result?.matches.map((m) => m.registeredPattern.originalPattern),
		).toEqual(["/docs"]);
	});

	it("handles missing export-key snapshots by defaulting to module default exports", async () => {
		const moduleDefault = () => "default-module";
		vi.doMock("/no-export-keys.js", () => ({ default: moduleDefault }));
		installVormaGlobal({
			importURLs: ["/no-export-keys.js"],
			exportKeys: undefined,
			activeComponents: [],
		});

		await ComponentLoader.handleComponents(["/no-export-keys.js"]);

		expect(__vormaClientGlobal.get("activeComponents")).toEqual([
			moduleDefault,
		]);
	});

	it("setupClientLoaders no-ops cleanly when no result is provided", () => {
		setClientLoadersState(undefined);
		expect(__vormaClientGlobal.get("clientLoadersData")).toEqual([]);
	});

	it("sets client error index only when an error message exists", () => {
		setClientLoadersState({
			data: ["ok"],
		});
		expect(
			__vormaClientGlobal.get("outermostClientErrorIdx"),
		).toBeUndefined();
		expect(__vormaClientGlobal.get("outermostClientError")).toBeUndefined();
	});

	it("handles malformed client-loader results with missing data arrays safely", () => {
		expect(() =>
			setClientLoadersState({
				data: undefined as any,
				errorMessage: "loader-error",
			}),
		).not.toThrow();

		expect(__vormaClientGlobal.get("clientLoadersData")).toEqual([]);
		expect(
			__vormaClientGlobal.get("outermostClientErrorIdx"),
		).toBeUndefined();
		expect(__vormaClientGlobal.get("outermostClientError")).toBe(
			"loader-error",
		);
	});

	it("sets outermost client error index to the last loader index when data exists", () => {
		setClientLoadersState({
			data: ["a", "b", "c"],
			errorMessage: "loader-error",
		});

		expect(__vormaClientGlobal.get("outermostClientErrorIdx")).toBe(2);
		expect(__vormaClientGlobal.get("outermostClientError")).toBe(
			"loader-error",
		);
	});

	it("supports setupClientLoaders with missing snapshots and empty maps", async () => {
		installVormaGlobal({
			importURLs: undefined,
			matchedPatterns: undefined,
			loadersData: undefined,
			params: undefined,
			splatValues: undefined,
			patternToWaitFnMap: undefined,
		});

		await expect(setupClientLoaders()).resolves.toBeUndefined();
		expect(__vormaClientGlobal.get("clientLoadersData")).toEqual([]);
	});

	it("passes unavailable server data to client loaders when required server payload is missing", async () => {
		const waitFn = vi.fn(async ({ serverDataPromise }) => {
			try {
				await serverDataPromise;
				return "unexpected";
			} catch (error) {
				return (error as Error).name;
			}
		});
		installVormaGlobal({
			patternToWaitFnMap: { "/needs-server": waitFn },
			routeManifest: { "/needs-server": 1 },
		});

		const result = await completeClientLoaders(
			{
				matchedPatterns: ["/needs-server"],
				loadersData: [],
				hasRootData: false,
				importURLs: [],
				params: {},
				splatValues: [],
			},
			"1",
			new Map(),
			new AbortController().signal,
		);

		expect(result).toEqual({
			data: ["AbortError"],
			errorMessage: undefined,
		});
		expect(waitFn).toHaveBeenCalledTimes(1);
	});

	it("fails fast when registering client loader without pattern registry", async () => {
		installVormaGlobal({
			patternRegistry: undefined,
		});

		await expect(__registerClientLoaderPattern("/x")).rejects.toThrow(
			"Pattern registry has not been initialized.",
		);
	});

	it("re-renders while applying head updates even when importURLs/cssBundles are omitted", async () => {
		const updateHeadElsSpy = vi.spyOn(headModule, "updateHeadEls");
		const onFinish = vi.fn();

		await __reRenderApp({
			navigationType: "userNavigation",
			onFinish,
			json: createRouteDataJSON({
				importURLs: undefined,
				cssBundles: undefined,
				metaHeadEls: [],
				restHeadEls: [],
			}) as any,
		});

		expect(onFinish).toHaveBeenCalledTimes(1);
		expect(updateHeadElsSpy).toHaveBeenCalledWith("meta", []);
		expect(updateHeadElsSpy).toHaveBeenCalledWith("rest", []);
	});

	it("does not dispatch default top-scroll state when explicit user-navigation scrollToTop is false", async () => {
		const routeChangeDetails: Array<unknown> = [];
		const listener = (event: Event) => {
			routeChangeDetails.push(
				(event as CustomEvent<{ __scrollState?: unknown }>).detail,
			);
		};
		window.addEventListener(VORMA_ROUTE_CHANGE_EVENT_KEY, listener);

		try {
			await __reRenderApp({
				navigationType: "userNavigation",
				runHistoryOptions: {
					href: "http://localhost:3000/no-scroll",
					scrollToTop: false,
				},
				onFinish: vi.fn(),
				json: createRouteDataJSON() as any,
			});
		} finally {
			window.removeEventListener(VORMA_ROUTE_CHANGE_EVENT_KEY, listener);
		}

		expect(window.location.pathname).toBe("/no-scroll");
		expect(routeChangeDetails.length).toBe(1);
		expect(
			(routeChangeDetails[0] as { __scrollState?: unknown })
				.__scrollState,
		).toBeUndefined();
	});

	it("falls back to empty title text when title HTML payload is missing", async () => {
		document.title = "before-title";

		await __reRenderApp({
			navigationType: "userNavigation",
			onFinish: vi.fn(),
			json: createRouteDataJSON({
				title: {},
			}) as any,
		});

		expect(document.title).toBe("");
	});

	it("normalizes null head-element arrays to empty lists during rerender", async () => {
		const updateHeadElsSpy = vi.spyOn(headModule, "updateHeadEls");

		await __reRenderApp({
			navigationType: "userNavigation",
			onFinish: vi.fn(),
			json: createRouteDataJSON({
				metaHeadEls: null,
				restHeadEls: null,
			}) as any,
		});

		expect(updateHeadElsSpy).toHaveBeenCalledWith("meta", []);
		expect(updateHeadElsSpy).toHaveBeenCalledWith("rest", []);
	});
});
