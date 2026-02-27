import { beforeEach, describe, expect, it, vi } from "vitest";
import {
	createPatternRegistry,
	registerPattern,
} from "vorma/kit/matcher/register";
import {
	AssetManager,
	ComponentLoader,
	VORMA_ROUTE_CHANGE_EVENT_KEY,
	VORMA_SYMBOL,
	__reRenderApp,
	__vormaClientGlobal,
	buildClientLoaderServerData,
	completeClientLoaders,
	findPartialMatchesOnClient,
	getRuntimeRouteSnapshot,
	registerClientLoaderPattern,
	setClientLoadersState,
	setupClientLoaders,
} from "../../runtime.ts";

const TEST_VORMA_APP_CONFIG = {
	actionsRouterMountRoot: "/api/",
	actionsDynamicRune: ":",
	actionsSplatRune: "*",
	loadersDynamicRune: ":",
	loadersSplatRune: "*",
	loadersExplicitIndexSegmentIdentifier: "_index",
};

function createRegisteredPatternRegistry(patterns: string[]) {
	const registry = createPatternRegistry({
		dynamicParamPrefixRune: TEST_VORMA_APP_CONFIG.loadersDynamicRune,
		splatSegmentRune: TEST_VORMA_APP_CONFIG.loadersSplatRune,
		explicitIndexSegment:
			TEST_VORMA_APP_CONFIG.loadersExplicitIndexSegmentIdentifier,
	});
	for (const pattern of patterns) {
		registerPattern(registry, pattern);
	}
	return registry;
}

function installVormaGlobal(overrides: Record<string, unknown> = {}): void {
	const baseGlobalState = {
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
		rootElementID: undefined,
		outermostServerError: undefined,
		outermostClientError: undefined,
		outermostServerErrorIdx: undefined,
		outermostClientErrorIdx: undefined,
		outermostError: undefined,
		outermostErrorIdx: undefined,
		isDev: false,
		viteDevURL: "",
		publicPathPrefix: "",
		isTouchInputModalityActive: false,
		patternToWaitFnMap: {},
		clientLoadersData: [],
		defaultErrorBoundary: () => null,
		useViewTransitions: false,
		deploymentID: "",
		vormaAppConfig: TEST_VORMA_APP_CONFIG,
		routeManifestURL: "",
		routeManifest: undefined,
		patternRegistry: createRegisteredPatternRegistry([]),
		...overrides,
	};

	const runtimeRouteSnapshot = {
		buildID: baseGlobalState.buildID,
		matchedPatterns: baseGlobalState.matchedPatterns,
		loadersData: baseGlobalState.loadersData,
		importURLs: baseGlobalState.importURLs,
		exportKeys: baseGlobalState.exportKeys,
		errorExportKeys: baseGlobalState.errorExportKeys,
		hasRootData: baseGlobalState.hasRootData,
		params: baseGlobalState.params,
		splatValues: baseGlobalState.splatValues,
		activeComponents: baseGlobalState.activeComponents,
		activeErrorBoundary: baseGlobalState.activeErrorBoundary,
		outermostServerError: baseGlobalState.outermostServerError,
		outermostClientError: baseGlobalState.outermostClientError,
		outermostServerErrorIdx: baseGlobalState.outermostServerErrorIdx,
		outermostClientErrorIdx: baseGlobalState.outermostClientErrorIdx,
		outermostError: baseGlobalState.outermostError,
		outermostErrorIdx: baseGlobalState.outermostErrorIdx,
		rootElementID: baseGlobalState.rootElementID,
		clientLoadersData: baseGlobalState.clientLoadersData,
		...(typeof overrides.runtimeRouteSnapshot === "object" &&
		overrides.runtimeRouteSnapshot !== null
			? overrides.runtimeRouteSnapshot
			: {}),
	};
	const nonSnapshotGlobalState: Record<string, unknown> = {
		...baseGlobalState,
	};
	delete nonSnapshotGlobalState.buildID;
	delete nonSnapshotGlobalState.matchedPatterns;
	delete nonSnapshotGlobalState.loadersData;
	delete nonSnapshotGlobalState.importURLs;
	delete nonSnapshotGlobalState.exportKeys;
	delete nonSnapshotGlobalState.errorExportKeys;
	delete nonSnapshotGlobalState.hasRootData;
	delete nonSnapshotGlobalState.params;
	delete nonSnapshotGlobalState.splatValues;
	delete nonSnapshotGlobalState.activeComponents;
	delete nonSnapshotGlobalState.activeErrorBoundary;
	delete nonSnapshotGlobalState.rootElementID;
	delete nonSnapshotGlobalState.outermostServerError;
	delete nonSnapshotGlobalState.outermostClientError;
	delete nonSnapshotGlobalState.outermostServerErrorIdx;
	delete nonSnapshotGlobalState.outermostClientErrorIdx;
	delete nonSnapshotGlobalState.outermostError;
	delete nonSnapshotGlobalState.outermostErrorIdx;
	delete nonSnapshotGlobalState.clientLoadersData;
	delete nonSnapshotGlobalState.runtimeRouteSnapshot;

	(globalThis as any)[VORMA_SYMBOL] = {
		...nonSnapshotGlobalState,
		runtimeRouteSnapshot,
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

function getManagedHeadElements(type: "meta" | "rest"): Array<Element> {
	const startComment = Array.from(document.head.childNodes).find((node) => {
		return (
			node.nodeType === Node.COMMENT_NODE &&
			(node as Comment).data === `data-vorma="${type}-start"`
		);
	}) as Comment | undefined;
	const endComment = Array.from(document.head.childNodes).find((node) => {
		return (
			node.nodeType === Node.COMMENT_NODE &&
			(node as Comment).data === `data-vorma="${type}-end"`
		);
	}) as Comment | undefined;

	if (!startComment || !endComment) {
		throw new Error(`Missing managed head markers for '${type}'.`);
	}

	const managedElements: Array<Element> = [];
	let currentNode = startComment.nextSibling;
	while (currentNode && currentNode !== endComment) {
		if (currentNode.nodeType === Node.ELEMENT_NODE) {
			managedElements.push(currentNode as Element);
		}
		currentNode = currentNode.nextSibling;
	}

	return managedElements;
}

beforeEach(() => {
	document.body.innerHTML = "";
	document.head.innerHTML = "";
	document.head.appendChild(
		document.createComment('data-vorma="meta-start"'),
	);
	document.head.appendChild(document.createComment('data-vorma="meta-end"'));
	document.head.appendChild(
		document.createComment('data-vorma="rest-start"'),
	);
	document.head.appendChild(document.createComment('data-vorma="rest-end"'));
	window.history.replaceState({}, "", "/");
	installVormaGlobal();
	vi.restoreAllMocks();
	if (!globalThis.CSS) {
		(globalThis as Record<string, unknown>).CSS = {};
	}
	if (!(globalThis as Record<string, any>).CSS?.escape) {
		(globalThis as Record<string, any>).CSS.escape = (value: string) =>
			value.replace(/[!"#$%&'()*+,./:;<=>?@[\\\]^`{|}~]/g, "\\$&");
	}
});

describe("render runtime internals", () => {
	it("derives effective error from the lower server/client error index", () => {
		installVormaGlobal({
			outermostServerErrorIdx: 5,
			outermostServerError: "server-error",
			outermostClientErrorIdx: 2,
			outermostClientError: "client-error",
		});

		setClientLoadersState({
			data: ["a", "b", "c"],
			errorMessage: "client-error",
		});

		expect(
			__vormaClientGlobal.get("runtimeRouteSnapshot").outermostErrorIdx,
		).toBe(2);
		expect(
			__vormaClientGlobal.get("runtimeRouteSnapshot").outermostError,
		).toBe("client-error");
	});

	it("ignores stale error text when no error index is set", () => {
		installVormaGlobal({
			outermostServerErrorIdx: undefined,
			outermostClientErrorIdx: undefined,
			outermostServerError: "stale-server-error",
			outermostClientError: "stale-client-error",
		});

		setClientLoadersState({
			data: ["ok"],
		});

		expect(
			__vormaClientGlobal.get("runtimeRouteSnapshot").outermostErrorIdx,
		).toBeUndefined();
		expect(
			__vormaClientGlobal.get("runtimeRouteSnapshot").outermostError,
		).toBeUndefined();
	});

	it("uses available error text when indices tie", () => {
		installVormaGlobal({
			outermostServerErrorIdx: 1,
			outermostServerError: undefined,
			outermostClientErrorIdx: 1,
			outermostClientError: "client-error",
		});

		setClientLoadersState({
			data: ["a", "b"],
			errorMessage: "client-error",
		});

		expect(
			__vormaClientGlobal.get("runtimeRouteSnapshot").outermostErrorIdx,
		).toBe(1);
		expect(
			__vormaClientGlobal.get("runtimeRouteSnapshot").outermostError,
		).toBe("client-error");
	});

	it("clears derived error state when no server/client error exists", () => {
		installVormaGlobal({
			outermostServerErrorIdx: undefined,
			outermostClientErrorIdx: undefined,
		});

		setClientLoadersState({
			data: [],
		});

		expect(
			__vormaClientGlobal.get("runtimeRouteSnapshot").outermostErrorIdx,
		).toBeUndefined();
		expect(
			__vormaClientGlobal.get("runtimeRouteSnapshot").outermostError,
		).toBeUndefined();
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

		expect(
			__vormaClientGlobal.get("runtimeRouteSnapshot").activeComponents,
		).toEqual([defaultComponent, null]);
	});

	it("skips error-boundary updates when no effective error index exists", async () => {
		vi.doMock("/err.js", () => ({ ErrorBoundary: () => "err" }));
		installVormaGlobal({
			importURLs: ["/err.js"],
			outermostServerErrorIdx: undefined,
			outermostClientErrorIdx: undefined,
		});

		await ComponentLoader.handleErrorBoundaryComponent(["/err.js"]);
		expect(
			__vormaClientGlobal.get("runtimeRouteSnapshot").activeErrorBoundary,
		).toBeUndefined();
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
				(call) => call[0] === "runtimeRouteSnapshot",
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

		expect(
			__vormaClientGlobal.get("runtimeRouteSnapshot").activeErrorBoundary,
		).toBe(defaultBoundary);
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

	it("uses the current payload server error index for client-loader skip decisions", async () => {
		const waitFnA = vi.fn(async () => "loader-a");
		const waitFnB = vi.fn(async () => "loader-b");
		installVormaGlobal({
			outermostServerErrorIdx: 0,
			patternToWaitFnMap: {
				"/a": waitFnA,
				"/b": waitFnB,
			},
			routeManifest: { "/a": 0, "/b": 0 },
		});

		const result = await completeClientLoaders(
			{
				matchedPatterns: ["/a", "/b"],
				loadersData: [{ id: "a" }, { id: "b" }],
				hasRootData: false,
				importURLs: [],
				params: {},
				splatValues: [],
				outermostServerErrorIdx: 1,
			},
			"1",
			new Map(),
			new AbortController().signal,
		);

		expect(waitFnA).toHaveBeenCalledTimes(1);
		expect(waitFnB).not.toHaveBeenCalled();
		expect(result).toEqual({
			data: ["loader-a", undefined],
			errorMessage: undefined,
		});
	});

	it("starts matched client loaders in parallel before waiting for settlement", async () => {
		const startOrder: string[] = [];
		let resolveFirstLoader: (value: string) => void = () => {};
		const firstLoaderDeferred = new Promise<string>((resolve) => {
			resolveFirstLoader = resolve;
		});
		installVormaGlobal({
			patternToWaitFnMap: {
				"/a": async () => {
					startOrder.push("/a");
					return firstLoaderDeferred;
				},
				"/b": async () => {
					startOrder.push("/b");
					return "loader-b";
				},
			},
			routeManifest: { "/a": 0, "/b": 0 },
		});

		const completionPromise = completeClientLoaders(
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

		await vi.waitFor(() => {
			expect(startOrder).toEqual(["/a", "/b"]);
		});
		expect(startOrder).toEqual(["/a", "/b"]);

		let settled = false;
		void completionPromise.finally(() => {
			settled = true;
		});
		await Promise.resolve();
		expect(settled).toBe(false);

		resolveFirstLoader("loader-a");

		await expect(completionPromise).resolves.toEqual({
			data: ["loader-a", "loader-b"],
			errorMessage: undefined,
		});
	});

	it("reuses running loader promises without invoking duplicate wait functions", async () => {
		const duplicateWaitFn = vi.fn(async () => "duplicate");
		installVormaGlobal({
			patternToWaitFnMap: {
				"/a": duplicateWaitFn,
			},
			routeManifest: { "/a": 0 },
		});

		const runningLoaders = new Map<string, Promise<unknown>>([
			["/a", Promise.resolve("running-loader-result")],
		]);

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
			runningLoaders,
			new AbortController().signal,
		);

		expect(result).toEqual({
			data: ["running-loader-result"],
			errorMessage: undefined,
		});
		expect(duplicateWaitFn).not.toHaveBeenCalled();
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

		expect(
			__vormaClientGlobal.get("runtimeRouteSnapshot").activeComponents,
		).toEqual([moduleDefault]);
	});

	it("setupClientLoaders no-ops cleanly when no result is provided", () => {
		setClientLoadersState(undefined);
		expect(
			__vormaClientGlobal.get("runtimeRouteSnapshot").clientLoadersData,
		).toEqual([]);
	});

	it("sets client error index only when an error message exists", () => {
		setClientLoadersState({
			data: ["ok"],
		});
		expect(
			__vormaClientGlobal.get("runtimeRouteSnapshot")
				.outermostClientErrorIdx,
		).toBeUndefined();
		expect(
			__vormaClientGlobal.get("runtimeRouteSnapshot")
				.outermostClientError,
		).toBeUndefined();
	});

	it("fails loud when client-loader results violate the typed data contract", () => {
		expect(() =>
			setClientLoadersState({
				data: undefined as any,
				errorMessage: "loader-error",
			}),
		).toThrow("Cannot read properties of undefined");
	});

	it("sets outermost client error index to the last loader index when data exists", () => {
		setClientLoadersState({
			data: ["a", "b", "c"],
			errorMessage: "loader-error",
		});

		expect(
			__vormaClientGlobal.get("runtimeRouteSnapshot")
				.outermostClientErrorIdx,
		).toBe(2);
		expect(
			__vormaClientGlobal.get("runtimeRouteSnapshot")
				.outermostClientError,
		).toBe("loader-error");
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
		expect(
			__vormaClientGlobal.get("runtimeRouteSnapshot").clientLoadersData,
		).toEqual([]);
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

		await expect(registerClientLoaderPattern("/x")).rejects.toThrow(
			"Pattern registry has not been initialized.",
		);
	});

	it("re-renders while applying head updates with canonical empty asset arrays", async () => {
		const applyCSSSpy = vi.spyOn(AssetManager, "applyCSS");
		const onFinish = vi.fn();

		await __reRenderApp({
			navigationType: "userNavigation",
			onFinish,
			json: createRouteDataJSON({
				importURLs: [],
				cssBundles: [],
				metaHeadEls: [],
				restHeadEls: [],
			}) as any,
		});

		expect(onFinish).toHaveBeenCalledTimes(1);
		expect(applyCSSSpy).toHaveBeenCalledTimes(1);
		expect(applyCSSSpy).toHaveBeenCalledWith([]);
		expect(getManagedHeadElements("meta")).toEqual([]);
		expect(getManagedHeadElements("rest")).toEqual([]);
	});

	it("applies CSS bundles during rerender when cssBundles are provided", async () => {
		const applyCSSSpy = vi.spyOn(AssetManager, "applyCSS");

		await __reRenderApp({
			navigationType: "userNavigation",
			onFinish: vi.fn(),
			json: createRouteDataJSON({
				cssBundles: ["/a.css", "/b.css"],
			}) as any,
		});

		expect(applyCSSSpy).toHaveBeenCalledTimes(1);
		expect(applyCSSSpy).toHaveBeenCalledWith(["/a.css", "/b.css"]);
	});

	it("skips module loading and commit side effects when shouldCommit is false before module load", async () => {
		const loadComponentsSpy = vi.spyOn(ComponentLoader, "loadComponents");
		const onFinish = vi.fn();

		await __reRenderApp({
			navigationType: "userNavigation",
			onFinish,
			shouldCommit: () => false,
			json: createRouteDataJSON({
				metaHeadEls: [],
				restHeadEls: [],
			}) as any,
		});

		expect(loadComponentsSpy).not.toHaveBeenCalled();
		expect(getManagedHeadElements("meta")).toEqual([]);
		expect(getManagedHeadElements("rest")).toEqual([]);
		expect(onFinish).not.toHaveBeenCalled();
	});

	it("stops commit side effects when shouldCommit turns false after module load", async () => {
		const loadComponentsSpy = vi
			.spyOn(ComponentLoader, "loadComponents")
			.mockResolvedValue(new Map());
		const onFinish = vi.fn();
		let shouldCommitCallCount = 0;

		await __reRenderApp({
			navigationType: "userNavigation",
			onFinish,
			shouldCommit: () => {
				shouldCommitCallCount += 1;
				return shouldCommitCallCount === 1;
			},
			json: createRouteDataJSON({
				metaHeadEls: [],
				restHeadEls: [],
			}) as any,
		});

		expect(shouldCommitCallCount).toBe(2);
		expect(loadComponentsSpy).toHaveBeenCalledTimes(1);
		expect(getManagedHeadElements("meta")).toEqual([]);
		expect(getManagedHeadElements("rest")).toEqual([]);
		expect(onFinish).not.toHaveBeenCalled();
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
		await __reRenderApp({
			navigationType: "userNavigation",
			onFinish: vi.fn(),
			json: createRouteDataJSON({
				metaHeadEls: null,
				restHeadEls: null,
			}) as any,
		});

		expect(getManagedHeadElements("meta")).toEqual([]);
		expect(getManagedHeadElements("rest")).toEqual([]);
	});

	it("commits a coherent runtime snapshot before route-change events are dispatched", async () => {
		const capturedSnapshots: Array<{
			runtimeSnapshot: ReturnType<typeof getRuntimeRouteSnapshot>;
			legacySnapshot: {
				matchedPatterns: unknown;
				loadersData: unknown;
				clientLoadersData: unknown;
				outermostError: unknown;
				outermostErrorIdx: unknown;
			};
		}> = [];
		const listener = () => {
			capturedSnapshots.push({
				runtimeSnapshot: getRuntimeRouteSnapshot(),
				legacySnapshot: {
					matchedPatterns: __vormaClientGlobal.get(
						"runtimeRouteSnapshot",
					).matchedPatterns,
					loadersData: __vormaClientGlobal.get("runtimeRouteSnapshot")
						.loadersData,
					clientLoadersData: __vormaClientGlobal.get(
						"runtimeRouteSnapshot",
					).clientLoadersData,
					outermostError: __vormaClientGlobal.get(
						"runtimeRouteSnapshot",
					).outermostError,
					outermostErrorIdx: __vormaClientGlobal.get(
						"runtimeRouteSnapshot",
					).outermostErrorIdx,
				},
			});
		};
		window.addEventListener(VORMA_ROUTE_CHANGE_EVENT_KEY, listener);

		try {
			await __reRenderApp({
				navigationType: "userNavigation",
				onFinish: vi.fn(),
				clientLoadersResult: {
					data: ["client-new"],
				},
				json: createRouteDataJSON({
					matchedPatterns: ["/new"],
					loadersData: [{ value: "loader-new" }],
					hasRootData: true,
					params: { id: "42" },
					splatValues: ["tail"],
				}) as any,
			});
		} finally {
			window.removeEventListener(VORMA_ROUTE_CHANGE_EVENT_KEY, listener);
		}

		expect(capturedSnapshots).toHaveLength(1);
		const committedSnapshot = capturedSnapshots[0]!;
		expect(committedSnapshot.runtimeSnapshot.matchedPatterns).toEqual([
			"/new",
		]);
		expect(committedSnapshot.runtimeSnapshot.loadersData).toEqual([
			{ value: "loader-new" },
		]);
		expect(committedSnapshot.runtimeSnapshot.clientLoadersData).toEqual([
			"client-new",
		]);
		expect(
			committedSnapshot.runtimeSnapshot.outermostError,
		).toBeUndefined();
		expect(
			committedSnapshot.runtimeSnapshot.outermostErrorIdx,
		).toBeUndefined();
		expect(committedSnapshot.legacySnapshot.matchedPatterns).toEqual(
			committedSnapshot.runtimeSnapshot.matchedPatterns,
		);
		expect(committedSnapshot.legacySnapshot.loadersData).toEqual(
			committedSnapshot.runtimeSnapshot.loadersData,
		);
		expect(committedSnapshot.legacySnapshot.clientLoadersData).toEqual(
			committedSnapshot.runtimeSnapshot.clientLoadersData,
		);
		expect(committedSnapshot.legacySnapshot.outermostError).toBe(
			committedSnapshot.runtimeSnapshot.outermostError,
		);
		expect(committedSnapshot.legacySnapshot.outermostErrorIdx).toBe(
			committedSnapshot.runtimeSnapshot.outermostErrorIdx,
		);
	});
});

describe("render runtime asset manager", () => {
	it("shares the same in-flight CSS preload promise for repeated bundle preloads", async () => {
		const appendChild = document.head.appendChild.bind(document.head);
		vi.spyOn(document.head, "appendChild").mockImplementation((node) => {
			return appendChild(node);
		});

		const firstPreloadPromise = AssetManager.preloadCSS("/shared.css");
		const secondPreloadPromise = AssetManager.preloadCSS("/shared.css");

		let didFirstResolve = false;
		let didSecondResolve = false;
		void firstPreloadPromise.then(() => {
			didFirstResolve = true;
		});
		void secondPreloadPromise.then(() => {
			didSecondResolve = true;
		});

		await Promise.resolve();
		expect(didFirstResolve).toBe(false);
		expect(didSecondResolve).toBe(false);
		expect(
			document.head.querySelectorAll('link[rel="preload"][as="style"]'),
		).toHaveLength(1);

		const preloadLink = document.head.querySelector<HTMLLinkElement>(
			'link[rel="preload"][as="style"][href="/shared.css"]',
		);
		expect(preloadLink).toBeTruthy();
		preloadLink?.onload?.(new Event("load"));

		await Promise.all([firstPreloadPromise, secondPreloadPromise]);
		expect(didFirstResolve).toBe(true);
		expect(didSecondResolve).toBe(true);
	});
});
