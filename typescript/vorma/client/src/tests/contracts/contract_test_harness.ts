import { afterEach, beforeEach, expect, vi } from "vitest";
import { createPatternRegistry } from "vorma/kit/matcher/register";
import { runtimeRouteSnapshotFieldKeys } from "../../runtime.ts";

const VORMA_INTERNAL_SYMBOL = Symbol.for("__vorma_internal__");

type AnyRecord = Record<string, any>;
type AnyPropertyRecord = Record<PropertyKey, any>;
type StatusSnapshot = {
	isNavigating: boolean;
	isSubmitting: boolean;
	isRevalidating: boolean;
};

const DEFAULT_VORMA_APP_CONFIG = {
	actionsRouterMountRoot: "/api/",
	actionsDynamicRune: ":",
	actionsSplatRune: "*",
	loadersDynamicRune: ":",
	loadersSplatRune: "*",
	loadersExplicitIndexSegmentIdentifier: "_index",
};

const RUNTIME_ROUTE_SNAPSHOT_TOP_LEVEL_KEYS = runtimeRouteSnapshotFieldKeys;

function stripRuntimeRouteSnapshotTopLevelFields(
	globals: AnyRecord,
): AnyRecord {
	const cleanedGlobals: AnyRecord = {
		...globals,
	};

	for (const key of RUNTIME_ROUTE_SNAPSHOT_TOP_LEVEL_KEYS) {
		delete cleanedGlobals[key];
	}

	delete cleanedGlobals.runtimeRouteSnapshot;
	return cleanedGlobals;
}

export function installContractVormaGlobal(overrides: AnyRecord = {}): void {
	const defaultRuntimeRouteSnapshot: AnyRecord = {
		outermostServerError: undefined,
		outermostServerErrorIdx: undefined,
		matchedPatterns: [],
		loadersData: [],
		importURLs: [],
		exportKeys: [],
		errorExportKeys: [],
		hasRootData: false,
		params: {},
		splatValues: [],
		outermostClientError: undefined,
		outermostClientErrorIdx: undefined,
		outermostError: undefined,
		outermostErrorIdx: undefined,
		buildID: "1",
		rootElementID: undefined,
		activeComponents: [],
		activeErrorBoundary: undefined,
		clientLoadersData: [],
	};

	const baseGlobals: AnyRecord = {
		isDev: false,
		viteDevURL: "",
		publicPathPrefix: "",
		isTouchInputModalityActive: false,
		patternToWaitFnMap: {},
		defaultErrorBoundary: () => null,
		useViewTransitions: false,
		deploymentID: "",
		vormaAppConfig: DEFAULT_VORMA_APP_CONFIG,
		routeManifestURL: "",
		routeManifest: undefined,
		patternRegistry: createPatternRegistry({
			dynamicParamPrefixRune: DEFAULT_VORMA_APP_CONFIG.loadersDynamicRune,
			splatSegmentRune: DEFAULT_VORMA_APP_CONFIG.loadersSplatRune,
			explicitIndexSegment:
				DEFAULT_VORMA_APP_CONFIG.loadersExplicitIndexSegmentIdentifier,
		}),
	};
	const mergedGlobals: AnyRecord = {
		...baseGlobals,
		...overrides,
	};
	const snapshotOverrides = isRecord(mergedGlobals.runtimeRouteSnapshot)
		? mergedGlobals.runtimeRouteSnapshot
		: {};
	const hasBuildIDOverride = Object.prototype.hasOwnProperty.call(
		overrides,
		"buildID",
	);
	const runtimeRouteSnapshot: AnyRecord = {
		...defaultRuntimeRouteSnapshot,
		outermostServerError:
			mergedGlobals.outermostServerError ??
			defaultRuntimeRouteSnapshot.outermostServerError,
		outermostServerErrorIdx:
			mergedGlobals.outermostServerErrorIdx ??
			defaultRuntimeRouteSnapshot.outermostServerErrorIdx,
		matchedPatterns:
			mergedGlobals.matchedPatterns ??
			defaultRuntimeRouteSnapshot.matchedPatterns,
		loadersData:
			mergedGlobals.loadersData ??
			defaultRuntimeRouteSnapshot.loadersData,
		importURLs:
			mergedGlobals.importURLs ?? defaultRuntimeRouteSnapshot.importURLs,
		exportKeys:
			mergedGlobals.exportKeys ?? defaultRuntimeRouteSnapshot.exportKeys,
		errorExportKeys:
			mergedGlobals.errorExportKeys ??
			defaultRuntimeRouteSnapshot.errorExportKeys,
		hasRootData:
			mergedGlobals.hasRootData ??
			defaultRuntimeRouteSnapshot.hasRootData,
		params: mergedGlobals.params ?? defaultRuntimeRouteSnapshot.params,
		splatValues:
			mergedGlobals.splatValues ??
			defaultRuntimeRouteSnapshot.splatValues,
		outermostClientError:
			mergedGlobals.outermostClientError ??
			defaultRuntimeRouteSnapshot.outermostClientError,
		outermostClientErrorIdx:
			mergedGlobals.outermostClientErrorIdx ??
			defaultRuntimeRouteSnapshot.outermostClientErrorIdx,
		outermostError:
			mergedGlobals.outermostError ??
			defaultRuntimeRouteSnapshot.outermostError,
		outermostErrorIdx:
			mergedGlobals.outermostErrorIdx ??
			defaultRuntimeRouteSnapshot.outermostErrorIdx,
		buildID: hasBuildIDOverride
			? overrides.buildID
			: defaultRuntimeRouteSnapshot.buildID,
		rootElementID:
			mergedGlobals.rootElementID ??
			defaultRuntimeRouteSnapshot.rootElementID,
		activeComponents:
			mergedGlobals.activeComponents ??
			defaultRuntimeRouteSnapshot.activeComponents,
		activeErrorBoundary:
			mergedGlobals.activeErrorBoundary ??
			defaultRuntimeRouteSnapshot.activeErrorBoundary,
		clientLoadersData:
			mergedGlobals.clientLoadersData ??
			defaultRuntimeRouteSnapshot.clientLoadersData,
		...snapshotOverrides,
	};

	const nonSnapshotGlobals =
		stripRuntimeRouteSnapshotTopLevelFields(mergedGlobals);
	(globalThis as AnyPropertyRecord)[VORMA_INTERNAL_SYMBOL] = {
		...nonSnapshotGlobals,
		runtimeRouteSnapshot,
	};
}

function isRecord(value: unknown): value is AnyRecord {
	return typeof value === "object" && value !== null && !Array.isArray(value);
}

function createRouteData(overrides: AnyRecord = {}): AnyRecord {
	const matchedPatterns = Array.isArray(overrides.matchedPatterns)
		? overrides.matchedPatterns
		: [];
	const routeCount = matchedPatterns.length;

	return {
		matchedPatterns,
		loadersData:
			overrides.loadersData ??
			Array.from({ length: routeCount }, () => null),
		importURLs:
			overrides.importURLs ??
			Array.from({ length: routeCount }, () => ""),
		exportKeys:
			overrides.exportKeys ??
			Array.from({ length: routeCount }, () => ""),
		errorExportKeys:
			overrides.errorExportKeys ??
			Array.from({ length: routeCount }, () => ""),
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
			"X-Wave-Framework-Build-Id": "1",
			...init.headers,
		},
		...init,
	});
}

export function createJSONResponse(
	data: unknown,
	init: ResponseInit = {},
): Response {
	return new Response(JSON.stringify(data), {
		status: 200,
		headers: {
			"Content-Type": "application/json",
			"X-Wave-Framework-Build-Id": "1",
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

export function createDeferredFetchCall() {
	const deferred = createDeferred<Response>();
	let signal: AbortSignal | undefined;

	const mock = (_url: RequestInfo | URL, init?: RequestInit) => {
		signal = (init as RequestInit | undefined)?.signal as
			| AbortSignal
			| undefined;
		return deferred.promise;
	};

	return {
		deferred,
		getSignal: () => signal,
		mock,
	};
}

export function createSignalCapturingNeverFetchSpy() {
	const signals: AbortSignal[] = [];
	const fetchSpy = vi
		.spyOn(window, "fetch")
		.mockImplementation((_url, init) => {
			const signal = (init as RequestInit | undefined)?.signal as
				| AbortSignal
				| undefined;
			if (signal) {
				signals.push(signal);
			}
			return new Promise<Response>(() => {});
		});

	return { fetchSpy, signals };
}

type FetchSequenceCall = {
	input: RequestInfo | URL;
	init?: RequestInit;
};

export function requestInputToURL(input: RequestInfo | URL): URL {
	if (input instanceof URL) {
		return input;
	}
	if (typeof input === "string") {
		return new URL(input, window.location.href);
	}
	return new URL(input.url, window.location.href);
}

export function requestInputToHref(input: RequestInfo | URL): string {
	return requestInputToURL(input).href;
}

export function stubWindowLocationHref(initialHref = window.location.href): {
	getHref: () => string;
	setHref: (href: string) => void;
	restore: () => void;
} {
	const originalLocation = window.location;
	let locationHref = initialHref;

	Object.defineProperty(window, "location", {
		value: {
			...originalLocation,
			get href() {
				return locationHref;
			},
			set href(value) {
				locationHref = String(value);
			},
		},
		configurable: true,
	});

	return {
		getHref: () => locationHref,
		setHref: (href: string) => {
			locationHref = String(href);
		},
		restore: () => {
			Object.defineProperty(window, "location", {
				value: originalLocation,
				configurable: true,
			});
		},
	};
}

type FetchSequenceStep =
	| Response
	| Promise<Response>
	| ((
			props: FetchSequenceCall & { callIndex: number },
	  ) => Response | Promise<Response>);

export function createSequencedFetchSpy(steps: FetchSequenceStep[]) {
	const queue = [...steps];
	const calls: FetchSequenceCall[] = [];

	const fetchSpy = vi
		.spyOn(window, "fetch")
		.mockImplementation((input, init) => {
			const call: FetchSequenceCall = {
				input: input as RequestInfo | URL,
				init: init as RequestInit | undefined,
			};
			calls.push(call);

			const step = queue.shift();
			if (!step) {
				throw new Error(
					`Unexpected fetch call #${calls.length}: no sequenced step available`,
				);
			}

			const value =
				typeof step === "function"
					? step({ ...call, callIndex: calls.length - 1 })
					: step;
			return Promise.resolve(value);
		});

	return { fetchSpy, calls };
}

type AbortAwareFetchRequest = {
	input: RequestInfo | URL;
	init: RequestInit | undefined;
	signal: AbortSignal | undefined;
	resolve: (response: Response) => void;
	reject: (reason?: unknown) => void;
};

export function createAbortAwareFetchRecorder() {
	const requests: AbortAwareFetchRequest[] = [];
	const fetchSpy = vi
		.spyOn(window, "fetch")
		.mockImplementation((input, init) => {
			const signal = (init as RequestInit | undefined)?.signal as
				| AbortSignal
				| undefined;
			const deferred = createDeferred<Response>();

			if (signal?.aborted) {
				deferred.reject(new DOMException("Aborted", "AbortError"));
				return deferred.promise;
			}

			signal?.addEventListener(
				"abort",
				() =>
					deferred.reject(new DOMException("Aborted", "AbortError")),
				{ once: true },
			);

			requests.push({
				input: input as RequestInfo | URL,
				init: init as RequestInit | undefined,
				signal,
				resolve: deferred.resolve,
				reject: deferred.reject,
			});

			return deferred.promise;
		});

	return { fetchSpy, requests };
}

export function collectStatusEvents(api: {
	addStatusListener: (
		listener: (event: CustomEvent<StatusSnapshot>) => void,
	) => () => void;
}) {
	const statusEvents: StatusSnapshot[] = [];
	const cleanup = api.addStatusListener((event) => {
		statusEvents.push(event.detail);
	});

	return { statusEvents, cleanup };
}

export async function waitForRequestCount(props: {
	requests: Array<unknown>;
	count: number;
	maxTicks?: number;
	tickMS?: number;
}): Promise<void> {
	const { requests, count, maxTicks = 300, tickMS = 1 } = props;
	for (let i = 0; i < maxTicks; i++) {
		if (requests.length >= count) {
			return;
		}
		await Promise.resolve();
		await vi.advanceTimersByTimeAsync(tickMS);
	}
	throw new Error(`Timed out waiting for request count ${count}`);
}

export function expectStatusIdle(status: StatusSnapshot | undefined): void {
	expect(status).toEqual({
		isNavigating: false,
		isSubmitting: false,
		isRevalidating: false,
	});
}

export function expectNoLoadingGapBeforeFinalEvent(
	statusEvents: StatusSnapshot[],
): void {
	const nonFinalEvents = statusEvents.slice(0, -1);
	const hasLoadingGap = nonFinalEvents.some(
		(status) =>
			!status.isNavigating &&
			!status.isSubmitting &&
			!status.isRevalidating,
	);
	expect(hasLoadingGap).toBe(false);
}

type PublicClientAPI = typeof import("../../../index.ts");
type ContractInternalAPI = {
	__getPrefetchHandlers: typeof import("../../runtime.ts").createPrefetchHandlers;
	__makeLinkOnClickFn: typeof import("../../runtime.ts").createLinkOnClickFn;
	__applyScrollState: typeof import("../../runtime.ts").applyScrollState;
	__vormaClientGlobal: typeof import("../../runtime.ts").__vormaClientGlobal;
	__getNavigationDebugJournal: typeof import("../../runtime.ts").getNavigationDebugJournal;
	__clearNavigationDebugJournal: typeof import("../../runtime.ts").clearNavigationDebugJournal;
	__registerClientLoaderPattern: typeof import("../../runtime.ts").registerClientLoaderPattern;
	__makeFinalLinkProps: typeof import("../../runtime.ts").makeFinalLinkProps;
	__resolvePath: typeof import("../../runtime.ts").resolvePath;
	__runClientLoadersAfterHMRUpdate: typeof import("../../runtime.ts").__runClientLoadersAfterHMRUpdate;
	__loadRouteManifestProgressively: typeof import("../../runtime.ts").loadRouteManifestProgressively;
};

export type ContractClientAPI = PublicClientAPI & ContractInternalAPI;

export async function loadClientAPI(): Promise<ContractClientAPI> {
	vi.resetModules();
	const [
		publicClientAPI,
		linksPrefetchInternal,
		linksClickInternal,
		scrollInternal,
		contextInternal,
		clientRuntimeInternal,
		renderRuntimeInternal,
		extrasInternal,
		uiHelpersInternal,
		appHelpersInternal,
		initInternal,
	] = await Promise.all([
		import("../../../index.ts"),
		import("../../runtime.ts"),
		import("../../runtime.ts"),
		import("../../runtime.ts"),
		import("../../runtime.ts"),
		import("../../runtime.ts"),
		import("../../runtime.ts"),
		import("../../runtime.ts"),
		import("../../runtime.ts"),
		import("../../runtime.ts"),
		import("../../runtime.ts"),
	]);

	// Contract tests may still exercise internal helpers, but we now source
	// those directly from internal modules so tests do not force public exports.
	const clientAPI: ContractClientAPI = {
		...publicClientAPI,
		__getPrefetchHandlers: linksPrefetchInternal.createPrefetchHandlers,
		__makeLinkOnClickFn: linksClickInternal.createLinkOnClickFn,
		__applyScrollState: scrollInternal.applyScrollState,
		__vormaClientGlobal: contextInternal.__vormaClientGlobal,
		__getNavigationDebugJournal:
			clientRuntimeInternal.getNavigationDebugJournal,
		__clearNavigationDebugJournal:
			clientRuntimeInternal.clearNavigationDebugJournal,
		__registerClientLoaderPattern:
			renderRuntimeInternal.registerClientLoaderPattern,
		__makeFinalLinkProps: uiHelpersInternal.makeFinalLinkProps,
		__resolvePath: appHelpersInternal.resolvePath,
		__runClientLoadersAfterHMRUpdate:
			extrasInternal.__runClientLoadersAfterHMRUpdate,
		__loadRouteManifestProgressively:
			initInternal.loadRouteManifestProgressively,
	};

	Object.defineProperty(clientAPI, "__runClientLoadersAfterHMRUpdate", {
		get: () => extrasInternal.__runClientLoadersAfterHMRUpdate,
		enumerable: true,
		configurable: true,
	});

	return clientAPI;
}

export function patchContractRuntimeRouteSnapshot(props: {
	api: ContractClientAPI;
	patch: Partial<import("../../../src/runtime.ts").RuntimeRouteSnapshot>;
}): void {
	const runtimeRouteSnapshot = props.api.__vormaClientGlobal.get(
		"runtimeRouteSnapshot",
	);
	props.api.__vormaClientGlobal.set("runtimeRouteSnapshot", {
		...runtimeRouteSnapshot,
		...props.patch,
	});
}

export async function registerServerDataFieldProbeLoader(props: {
	api: AnyRecord;
	pattern: string;
	requiredField: string;
}): Promise<{ serverDataPromiseErrors: Array<unknown> }> {
	const { api, pattern, requiredField } = props;
	const patternToWaitFnMap =
		api.__vormaClientGlobal.get("patternToWaitFnMap");
	const serverDataPromiseErrors: Array<unknown> = [];

	patternToWaitFnMap[pattern] = async ({
		serverDataPromise,
	}: {
		serverDataPromise: Promise<{ loaderData: AnyRecord }>;
	}) => {
		const { loaderData } = await serverDataPromise.catch((error) => {
			serverDataPromiseErrors.push(error);
			throw error;
		});
		return loaderData[requiredField];
	};

	await api.__registerClientLoaderPattern(pattern);

	return { serverDataPromiseErrors };
}

export async function withUnhandledRejectionCapture<T>(props: {
	run: () => Promise<T>;
}): Promise<{ result: T; unhandledRejections: Array<unknown> }> {
	const { run } = props;
	const unhandledRejections: Array<unknown> = [];
	const unhandledRejectionHandler = (reason: unknown) => {
		unhandledRejections.push(reason);
	};

	process.on("unhandledRejection", unhandledRejectionHandler);
	try {
		const result = await run();
		await Promise.resolve();
		return { result, unhandledRejections };
	} finally {
		process.off("unhandledRejection", unhandledRejectionHandler);
	}
}

export function setupContractTestSuite(): void {
	beforeEach(() => {
		vi.useFakeTimers();
		vi.setSystemTime(new Date("2026-01-01T00:00:00.000Z"));

		document.body.innerHTML = "";
		document.head.innerHTML = "";
		document.head.appendChild(
			document.createComment('data-vorma="meta-start"'),
		);
		document.head.appendChild(
			document.createComment('data-vorma="meta-end"'),
		);
		document.head.appendChild(
			document.createComment('data-vorma="rest-start"'),
		);
		document.head.appendChild(
			document.createComment('data-vorma="rest-end"'),
		);
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
		vi.spyOn(console, "log").mockImplementation(() => {});
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
