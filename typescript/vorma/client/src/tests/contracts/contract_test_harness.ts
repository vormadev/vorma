import { afterEach, beforeEach, expect, vi } from "vitest";
import { createPatternRegistry } from "vorma/kit/matcher/register";

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

function createRouteData(overrides: AnyRecord = {}): AnyRecord {
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

export function createJSONResponse(
	data: unknown,
	init: ResponseInit = {},
): Response {
	return new Response(JSON.stringify(data), {
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
	__getPrefetchHandlers: typeof import("../../core/links.ts").__getPrefetchHandlers;
	__makeLinkOnClickFn: typeof import("../../core/links.ts").__makeLinkOnClickFn;
	__applyScrollState: typeof import("../../platform/scroll.ts").__applyScrollState;
	__vormaClientGlobal: typeof import("../../app/context.ts").__vormaClientGlobal;
	__getNavigationDebugJournal: typeof import("../../client.ts").getNavigationDebugJournal;
	__clearNavigationDebugJournal: typeof import("../../client.ts").clearNavigationDebugJournal;
	__registerClientLoaderPattern: typeof import("../../core/render_runtime.ts").__registerClientLoaderPattern;
	__makeFinalLinkProps: typeof import("../../ui/helpers.ts").__makeFinalLinkProps;
	__resolvePath: typeof import("../../app/helpers.ts").__resolvePath;
	__runClientLoadersAfterHMRUpdate: typeof import("../../core/extras.ts").__runClientLoadersAfterHMRUpdate;
};

export type ContractClientAPI = PublicClientAPI & ContractInternalAPI;

export async function loadClientAPI(): Promise<ContractClientAPI> {
	vi.resetModules();
	const [
		publicClientAPI,
		linksInternal,
		scrollInternal,
		contextInternal,
		clientRuntimeInternal,
		renderRuntimeInternal,
		extrasInternal,
		uiHelpersInternal,
		appHelpersInternal,
	] = await Promise.all([
		import("../../../index.ts"),
		import("../../core/links.ts"),
		import("../../platform/scroll.ts"),
		import("../../app/context.ts"),
		import("../../client.ts"),
		import("../../core/render_runtime.ts"),
		import("../../core/extras.ts"),
		import("../../ui/helpers.ts"),
		import("../../app/helpers.ts"),
	]);

	// Contract tests may still exercise internal helpers, but we now source
	// those directly from internal modules so tests do not force public exports.
	const clientAPI: ContractClientAPI = {
		...publicClientAPI,
		__getPrefetchHandlers: linksInternal.__getPrefetchHandlers,
		__makeLinkOnClickFn: linksInternal.__makeLinkOnClickFn,
		__applyScrollState: scrollInternal.__applyScrollState,
		__vormaClientGlobal: contextInternal.__vormaClientGlobal,
		__getNavigationDebugJournal:
			clientRuntimeInternal.getNavigationDebugJournal,
		__clearNavigationDebugJournal:
			clientRuntimeInternal.clearNavigationDebugJournal,
		__registerClientLoaderPattern:
			renderRuntimeInternal.__registerClientLoaderPattern,
		__makeFinalLinkProps: uiHelpersInternal.__makeFinalLinkProps,
		__resolvePath: appHelpersInternal.__resolvePath,
		__runClientLoadersAfterHMRUpdate:
			extrasInternal.__runClientLoadersAfterHMRUpdate,
	};

	Object.defineProperty(clientAPI, "__runClientLoadersAfterHMRUpdate", {
		get: () => extrasInternal.__runClientLoadersAfterHMRUpdate,
		enumerable: true,
		configurable: true,
	});

	return clientAPI;
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
