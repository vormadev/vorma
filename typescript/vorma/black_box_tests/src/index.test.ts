import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
	clearAllNavigationStateForTesting,
	isPageRefreshScrollStateStorageKeyForTesting,
	isScrollStateStorageKeyForTesting,
	readIsTouchInputModalityActiveForTesting,
	readPageRefreshScrollStateForTesting,
	readRouteManifestForTesting,
	readScrollStateForTesting,
	registerClientLoaderForTesting,
	replacePatternRegistryForTesting,
	resetClientRuntimeForTesting,
	seedPageRefreshScrollStateForTesting,
	seedRuntimeRouteSnapshotForTesting,
	seedScrollStateForTesting,
	setDeploymentIDForTesting,
	setHardRedirectHandlerForTesting,
	setRouteManifestForTesting,
	simulateViteAfterUpdateForTesting,
	writeRawScrollStateStorageForTesting,
} from "../../client/testing.ts";

type AnyPropertyRecord = Record<PropertyKey, any>;
type StatusSnapshot = {
	isNavigating: boolean;
	isSubmitting: boolean;
	isRevalidating: boolean;
};

const TEST_VORMA_APP_CONFIG = {
	actionsRouterMountRoot: "/api/",
	actionsDynamicRune: ":",
	actionsSplatRune: "*",
	loadersDynamicRune: ":",
	loadersSplatRune: "*",
	loadersExplicitIndexSegmentIdentifier: "_index",
};

type RouteDataOverride = {
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
	outermostServerError?: unknown;
	outermostServerErrorIdx?: number | null | undefined;
	title?: { dangerousInnerHTML: string } | undefined;
	metaHeadEls?: unknown[];
	restHeadEls?: unknown[];
};

function setupBlackBoxTestSuite(): void {
	beforeEach(() => {
		resetClientRuntimeForTesting();
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
			(globalThis as AnyPropertyRecord).CSS = {};
		}
		if (!(globalThis as AnyPropertyRecord).CSS.escape) {
			(globalThis as AnyPropertyRecord).CSS.escape = (value: string) =>
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
	});

	afterEach(async () => {
		await vi.runOnlyPendingTimersAsync();
		vi.useRealTimers();
		vi.restoreAllMocks();
		resetClientRuntimeForTesting();
		document.body.innerHTML = "";
		document.head.innerHTML = "";
	});
}

function createRouteData(overrides: RouteDataOverride = {}): RouteDataOverride {
	const matchedPatterns = overrides.matchedPatterns ?? [];
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
		hasRootData: overrides.hasRootData ?? false,
		params: overrides.params ?? {},
		splatValues: overrides.splatValues ?? [],
		deps: overrides.deps ?? [],
		cssBundles: overrides.cssBundles ?? [],
		outermostServerError: overrides.outermostServerError,
		outermostServerErrorIdx: overrides.outermostServerErrorIdx,
		title: overrides.title,
		metaHeadEls: overrides.metaHeadEls ?? [],
		restHeadEls: overrides.restHeadEls ?? [],
	};
}

function createRouteDataResponse(
	overrides: RouteDataOverride = {},
	init: ResponseInit = {},
): Response {
	const responseHeaders = new Headers({
		"Content-Type": "application/json",
		"X-Wave-Framework-Build-Id": "1",
	});
	new Headers(init.headers ?? undefined).forEach((value, key) => {
		responseHeaders.set(key, value);
	});
	return new Response(JSON.stringify(createRouteData(overrides)), {
		status: 200,
		headers: responseHeaders,
		...init,
	});
}

function createDeferred<T>() {
	let resolve: (value: T) => void;
	let reject: (reason?: unknown) => void;
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

function createDeferredFetchCall() {
	const deferred = createDeferred<Response>();
	let signal: AbortSignal | undefined;

	const mock = (_input: RequestInfo | URL, init?: RequestInit) => {
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

function createSeededPseudoRandomNumberGenerator(props: {
	seed: number;
}): () => number {
	let generatorState = props.seed >>> 0;
	return () => {
		generatorState = (generatorState * 1664525 + 1013904223) >>> 0;
		return generatorState / 0x100000000;
	};
}

function buildShuffledIndicesForSeededRandomization(props: {
	length: number;
	seed: number;
}): number[] {
	const random = createSeededPseudoRandomNumberGenerator({
		seed: props.seed,
	});
	const indices = Array.from(
		{ length: props.length },
		(_value, index) => index,
	);
	for (let i = indices.length - 1; i > 0; i -= 1) {
		const j = Math.floor(random() * (i + 1));
		const valueAtI = indices[i];
		indices[i] = indices[j]!;
		indices[j] = valueAtI!;
	}
	return indices;
}

type AbortAwareFetchRequest = {
	input: RequestInfo | URL;
	init: RequestInit | undefined;
	resolve: (response: Response) => void;
	reject: (reason?: unknown) => void;
};

function createAbortAwareFetchRecorder(): {
	requests: AbortAwareFetchRequest[];
} {
	const requests: AbortAwareFetchRequest[] = [];
	vi.spyOn(window, "fetch").mockImplementation((_input, init) => {
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
			() => deferred.reject(new DOMException("Aborted", "AbortError")),
			{ once: true },
		);
		requests.push({
			input: _input as RequestInfo | URL,
			init: init as RequestInit | undefined,
			resolve: deferred.resolve,
			reject: deferred.reject,
		});
		return deferred.promise;
	});
	return { requests };
}

async function waitForRequestCount(props: {
	requests: Array<unknown>;
	count: number;
	maxTicks?: number;
	tickMS?: number;
}): Promise<void> {
	const { requests, count, maxTicks = 300, tickMS = 1 } = props;
	for (let i = 0; i < maxTicks; i += 1) {
		if (requests.length >= count) {
			return;
		}
		await Promise.resolve();
		await vi.advanceTimersByTimeAsync(tickMS);
	}
	throw new Error(`Timed out waiting for request count ${count}`);
}

function expectStatusIdle(status: StatusSnapshot | undefined): void {
	expect(status).toEqual({
		isNavigating: false,
		isSubmitting: false,
		isRevalidating: false,
	});
}

function expectNoLoadingGapBeforeFinalEvent(statuses: StatusSnapshot[]): void {
	const nonFinalStatuses = statuses.slice(0, -1);
	const hasLoadingGap = nonFinalStatuses.some(
		(status) =>
			!status.isNavigating &&
			!status.isSubmitting &&
			!status.isRevalidating,
	);
	expect(hasLoadingGap).toBe(false);
}

async function withUnhandledRejectionCapture<T>(props: {
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

type PublicClientAPI = Pick<
	typeof import("../../client/index.ts"),
	| "buildMutationURL"
	| "buildQueryURL"
	| "resolveBody"
	| "makeTypedAPIClient"
	| "makeTypedNavigate"
	| "defaultErrorBoundary"
	| "getRootEl"
	| "getRouterData"
	| "initClient"
	| "revalidateOnWindowFocus"
	| "setupGlobalLoadingIndicator"
	| "addBuildIDListener"
	| "addLocationListener"
	| "addRouteChangeListener"
	| "addStatusListener"
	| "getBuildID"
	| "getLocation"
	| "getHistoryInstance"
	| "getStatus"
	| "revalidate"
	| "submit"
	| "vormaNavigate"
>;

async function loadPublicClientAPI(): Promise<PublicClientAPI> {
	vi.resetModules();
	const api = await import("../../client/index.ts");
	return {
		buildMutationURL: api.buildMutationURL,
		buildQueryURL: api.buildQueryURL,
		resolveBody: api.resolveBody,
		makeTypedAPIClient: api.makeTypedAPIClient,
		makeTypedNavigate: api.makeTypedNavigate,
		defaultErrorBoundary: api.defaultErrorBoundary,
		getRootEl: api.getRootEl,
		getRouterData: api.getRouterData,
		initClient: api.initClient,
		revalidateOnWindowFocus: api.revalidateOnWindowFocus,
		setupGlobalLoadingIndicator: api.setupGlobalLoadingIndicator,
		addBuildIDListener: api.addBuildIDListener,
		addLocationListener: api.addLocationListener,
		addRouteChangeListener: api.addRouteChangeListener,
		addStatusListener: api.addStatusListener,
		getBuildID: api.getBuildID,
		getLocation: api.getLocation,
		getHistoryInstance: api.getHistoryInstance,
		getStatus: api.getStatus,
		revalidate: api.revalidate,
		submit: api.submit,
		vormaNavigate: api.vormaNavigate,
	};
}

function collectStatusSnapshots(api: PublicClientAPI): {
	statuses: StatusSnapshot[];
	cleanup: () => void;
} {
	const statuses: StatusSnapshot[] = [];
	const cleanup = api.addStatusListener((event) => {
		statuses.push(event.detail);
	});
	return { statuses, cleanup };
}

async function navigateWithRouteDataResponse(props: {
	api: PublicClientAPI;
	href: string;
	overrides?: RouteDataOverride;
	responseInit?: ResponseInit;
}): Promise<void> {
	vi.spyOn(window, "fetch").mockResolvedValueOnce(
		createRouteDataResponse(props.overrides ?? {}, props.responseInit),
	);
	await props.api.vormaNavigate(props.href);
	await vi.runAllTimersAsync();
}

function requestInputToURL(input: RequestInfo | URL): URL {
	if (typeof input === "string") {
		return new URL(input, window.location.href);
	}
	if (input instanceof URL) {
		return new URL(input.href);
	}
	return new URL(input.url, window.location.href);
}

function stubWindowLocationHref(initialHref = window.location.href): {
	getHref: () => string;
	setHref: (href: string) => void;
	restore: () => void;
} {
	const originalLocation = window.location;
	let locationHref = initialHref;
	const resolveCurrentURL = (): URL => {
		return new URL(locationHref, originalLocation.href);
	};
	const locationStub = {
		get href() {
			return locationHref;
		},
		set href(value: string) {
			locationHref = new URL(String(value), locationHref).href;
		},
		get origin() {
			return resolveCurrentURL().origin;
		},
		get pathname() {
			return resolveCurrentURL().pathname;
		},
		get search() {
			return resolveCurrentURL().search;
		},
		get hash() {
			return resolveCurrentURL().hash;
		},
		assign(value: string | URL): void {
			locationHref = new URL(String(value), locationHref).href;
		},
		replace(value: string | URL): void {
			locationHref = new URL(String(value), locationHref).href;
		},
		toString(): string {
			return locationHref;
		},
	} as Location;

	Object.defineProperty(window, "location", {
		value: locationStub,
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

type HeadSection = "meta" | "rest";

function getHeadSectionComments(props: { type: HeadSection }): {
	startComment: Comment;
	endComment: Comment;
} {
	const startToken = `data-vorma="${props.type}-start"`;
	const endToken = `data-vorma="${props.type}-end"`;
	let startComment: Comment | undefined;
	for (const node of Array.from(document.head.childNodes)) {
		if (node.nodeType !== Node.COMMENT_NODE) {
			continue;
		}
		const commentNode = node as Comment;
		const commentValue = commentNode.nodeValue?.trim();
		if (commentValue === startToken) {
			startComment = commentNode;
			continue;
		}
		if (commentValue === endToken && startComment) {
			return {
				startComment,
				endComment: commentNode,
			};
		}
	}
	throw new Error(
		`Missing managed head markers for section "${props.type}" in test setup.`,
	);
}

function getNodesBetweenHeadSectionMarkers(props: {
	type: HeadSection;
}): Node[] {
	const { startComment, endComment } = getHeadSectionComments({
		type: props.type,
	});
	const nodes: Node[] = [];
	let currentNode: Node | null = startComment.nextSibling;
	while (currentNode && currentNode !== endComment) {
		nodes.push(currentNode);
		currentNode = currentNode.nextSibling;
	}
	return nodes;
}

function getElementsBetweenHeadSectionMarkers(props: {
	type: HeadSection;
}): Element[] {
	return getNodesBetweenHeadSectionMarkers({
		type: props.type,
	}).filter((node): node is Element => node.nodeType === Node.ELEMENT_NODE);
}

function insertHeadSectionElement(props: {
	type: HeadSection;
	element: Element;
}): void {
	const { endComment } = getHeadSectionComments({
		type: props.type,
	});
	document.head.insertBefore(props.element, endComment);
}

function getPageRefreshScrollStateStorageKeyOrThrow(): string {
	for (let index = 0; index < window.sessionStorage.length; index += 1) {
		const maybeKey = window.sessionStorage.key(index);
		if (
			typeof maybeKey === "string" &&
			isPageRefreshScrollStateStorageKeyForTesting(maybeKey)
		) {
			return maybeKey;
		}
	}
	throw new Error(
		"Unable to locate page-refresh scroll state storage key in test runtime.",
	);
}

setupBlackBoxTestSuite();

describe("authoritative black-box contracts", () => {
	it("commits navigation location and title from server route-data", async () => {
		const api = await loadPublicClientAPI();
		vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				loadersData: [],
				title: { dangerousInnerHTML: "Black Box A" },
			}),
		);

		await api.vormaNavigate("/black-box-a");
		await vi.runAllTimersAsync();

		expect(window.location.pathname).toBe("/black-box-a");
		expect(document.title).toBe("Black Box A");
		expectStatusIdle(api.getStatus());
	});

	it("decodes HTML entities before updating document.title", async () => {
		const api = await loadPublicClientAPI();
		vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				title: { dangerousInnerHTML: "Fish &amp; Chips &lt;3" },
			}),
		);

		await api.vormaNavigate("/entity-title");
		await vi.runAllTimersAsync();

		expect(document.title).toBe("Fish & Chips <3");
	});

	it("clears document.title when route-data omits title payload", async () => {
		const api = await loadPublicClientAPI();
		document.title = "Stale Old Title";
		vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				loadersData: [],
				title: undefined,
			}),
		);

		await api.vormaNavigate("/title-cleared");
		await vi.runAllTimersAsync();

		expect(document.title).toBe("");
	});

	it("uses nearest valid managed head marker pair and ignores stray markers", async () => {
		const api = await loadPublicClientAPI();
		document.head.innerHTML = "";
		document.head.appendChild(
			document.createComment('data-vorma="meta-end"'),
		);
		document.head.appendChild(
			document.createComment('data-vorma="meta-start"'),
		);
		const strayMeta = document.createElement("meta");
		strayMeta.setAttribute("name", "stray");
		strayMeta.setAttribute("content", "keep");
		document.head.appendChild(strayMeta);
		document.head.appendChild(
			document.createComment('data-vorma="meta-start"'),
		);
		const staleDescription = document.createElement("meta");
		staleDescription.setAttribute("name", "description");
		staleDescription.setAttribute("content", "stale");
		document.head.appendChild(staleDescription);
		document.head.appendChild(
			document.createComment('data-vorma="meta-end"'),
		);
		const outsideMeta = document.createElement("meta");
		outsideMeta.setAttribute("name", "outside");
		outsideMeta.setAttribute("content", "keep");
		document.head.appendChild(outsideMeta);
		document.head.appendChild(
			document.createComment('data-vorma="rest-start"'),
		);
		document.head.appendChild(
			document.createComment('data-vorma="rest-end"'),
		);

		await navigateWithRouteDataResponse({
			api,
			href: "/head-nearest-markers",
			overrides: {
				metaHeadEls: [
					{
						tag: "meta",
						attributesKnownSafe: {
							name: "description",
							content: "fresh",
						},
						booleanAttributes: [],
					},
				],
			},
		});

		expect(
			document.head
				.querySelector('meta[name="description"]')
				?.getAttribute("content"),
		).toBe("fresh");
		expect(
			document.head.querySelector('meta[name="stray"]'),
		).not.toBeNull();
		expect(
			document.head.querySelector('meta[name="outside"]'),
		).not.toBeNull();
	});

	it("fails loud when managed head markers are missing or invalid", async () => {
		const api = await loadPublicClientAPI();
		document.head.innerHTML = "";
		document.head.appendChild(
			document.createComment('data-vorma="meta-end"'),
		);
		document.head.appendChild(
			document.createComment('data-vorma="meta-start"'),
		);
		const sentinel = document.createElement("meta");
		sentinel.setAttribute("name", "sentinel");
		sentinel.setAttribute("content", "keep");
		document.head.appendChild(sentinel);
		document.head.appendChild(
			document.createComment('data-vorma="rest-start"'),
		);
		document.head.appendChild(
			document.createComment('data-vorma="rest-end"'),
		);
		vi.spyOn(window, "fetch").mockResolvedValueOnce(
			createRouteDataResponse({
				metaHeadEls: [
					{
						tag: "meta",
						attributesKnownSafe: {
							name: "description",
							content: "new",
						},
						booleanAttributes: [],
					},
				],
			}),
		);

		await expect(
			api.vormaNavigate("/head-invalid-markers"),
		).rejects.toThrow(
			"Managed head section markers for 'meta' are missing or invalid.",
		);
		await vi.runAllTimersAsync();
		expect(
			document.head
				.querySelector('meta[name="sentinel"]')
				?.getAttribute("content"),
		).toBe("keep");
		expect(
			document.head.querySelector('meta[name="description"]'),
		).toBeNull();
	});

	it("reconciles meta head blocks with deterministic add update remove reorder behavior", async () => {
		const api = await loadPublicClientAPI();
		const description = document.createElement("meta");
		description.setAttribute("name", "description");
		description.setAttribute("content", "Original description");
		const keywords = document.createElement("meta");
		keywords.setAttribute("name", "keywords");
		keywords.setAttribute("content", "alpha, beta");
		const canonical = document.createElement("link");
		canonical.setAttribute("rel", "canonical");
		canonical.setAttribute("href", "/original");
		insertHeadSectionElement({
			type: "meta",
			element: description,
		});
		insertHeadSectionElement({
			type: "meta",
			element: keywords,
		});
		insertHeadSectionElement({
			type: "meta",
			element: canonical,
		});

		await navigateWithRouteDataResponse({
			api,
			href: "/head-reconcile",
			overrides: {
				metaHeadEls: [
					{
						tag: "meta",
						attributesKnownSafe: {
							name: "keywords",
							content: "alpha, beta",
						},
						booleanAttributes: [],
					},
					{
						tag: "meta",
						attributesKnownSafe: {
							name: "description",
							content: "Updated description",
						},
						booleanAttributes: [],
					},
					{
						tag: "meta",
						attributesKnownSafe: {
							name: "viewport",
							content: "width=device-width",
						},
						booleanAttributes: [],
					},
				],
			},
		});

		const elements = getElementsBetweenHeadSectionMarkers({
			type: "meta",
		});
		const { startComment } = getHeadSectionComments({
			type: "meta",
		});
		expect(elements).toHaveLength(3);
		expect(startComment.nextElementSibling).toBe(elements[0]);
		expect(elements[0]).toBe(keywords);
		expect(elements[0]?.getAttribute("name")).toBe("keywords");
		expect(elements[1]).toBe(description);
		expect(elements[1]?.getAttribute("content")).toBe(
			"Updated description",
		);
		expect(elements[2]?.getAttribute("name")).toBe("viewport");
		expect(document.head.querySelector('link[rel="canonical"]')).toBeNull();
	});

	it("reuses matching meta elements and collapses duplicates to the requested count", async () => {
		const api = await loadPublicClientAPI();
		const metaA = document.createElement("meta");
		metaA.setAttribute("name", "description");
		metaA.setAttribute("content", "A");
		const metaB = document.createElement("meta");
		metaB.setAttribute("name", "description");
		metaB.setAttribute("content", "A");
		insertHeadSectionElement({
			type: "meta",
			element: metaA,
		});
		insertHeadSectionElement({
			type: "meta",
			element: metaB,
		});

		await navigateWithRouteDataResponse({
			api,
			href: "/head-collapse-duplicates",
			overrides: {
				metaHeadEls: [
					{
						tag: "meta",
						attributesKnownSafe: {
							name: "description",
							content: "A",
						},
						booleanAttributes: [],
					},
				],
			},
		});

		const finalElements = getElementsBetweenHeadSectionMarkers({
			type: "meta",
		});
		expect(finalElements).toHaveLength(1);
		expect(finalElements[0]).toBe(metaA);
	});

	it("keeps managed head sections unchanged when requested blocks are empty", async () => {
		const api = await loadPublicClientAPI();
		const initialNodeCount = document.head.childNodes.length;

		await navigateWithRouteDataResponse({
			api,
			href: "/head-empty",
			overrides: {
				metaHeadEls: [],
				restHeadEls: [],
			},
		});

		expect(document.head.childNodes.length).toBe(initialNodeCount);
		expect(
			getElementsBetweenHeadSectionMarkers({
				type: "meta",
			}),
		).toHaveLength(0);
		expect(
			getElementsBetweenHeadSectionMarkers({
				type: "rest",
			}),
		).toHaveLength(0);
	});

	it("inserts requested managed meta elements between markers when the section is empty", async () => {
		const api = await loadPublicClientAPI();
		await navigateWithRouteDataResponse({
			api,
			href: "/head-insert-empty-meta",
			overrides: {
				metaHeadEls: [
					{
						tag: "meta",
						attributesKnownSafe: {
							name: "description",
							content: "Inserted description",
						},
						booleanAttributes: [],
					},
					{
						tag: "link",
						attributesKnownSafe: {
							rel: "stylesheet",
							href: "/inserted.css",
						},
						booleanAttributes: [],
					},
				],
			},
		});

		const metaElements = getElementsBetweenHeadSectionMarkers({
			type: "meta",
		});
		expect(metaElements).toHaveLength(2);
		expect(metaElements[0]?.tagName.toLowerCase()).toBe("meta");
		expect(metaElements[0]?.getAttribute("name")).toBe("description");
		expect(metaElements[1]?.tagName.toLowerCase()).toBe("link");
		expect(metaElements[1]?.getAttribute("href")).toBe("/inserted.css");
	});

	it("keeps attribute-order-independent fingerprint matches stable across updates", async () => {
		const api = await loadPublicClientAPI();
		const meta = document.createElement("meta");
		meta.setAttribute("content", "Order test");
		meta.setAttribute("name", "description");
		insertHeadSectionElement({
			type: "meta",
			element: meta,
		});

		await navigateWithRouteDataResponse({
			api,
			href: "/head-attr-order",
			overrides: {
				metaHeadEls: [
					{
						tag: "meta",
						attributesKnownSafe: {
							name: "description",
							content: "Order test",
						},
						booleanAttributes: [],
					},
				],
			},
		});

		const elements = getElementsBetweenHeadSectionMarkers({
			type: "meta",
		});
		expect(elements).toHaveLength(1);
		expect(elements[0]).toBe(meta);
	});

	it("remains idempotent across repeated identical managed head updates", async () => {
		const api = await loadPublicClientAPI();
		await navigateWithRouteDataResponse({
			api,
			href: "/head-idempotent-a",
			overrides: {
				metaHeadEls: [
					{
						tag: "meta",
						attributesKnownSafe: {
							name: "description",
							content: "Idempotent",
						},
						booleanAttributes: [],
					},
				],
			},
		});
		const firstElement = getElementsBetweenHeadSectionMarkers({
			type: "meta",
		})[0];

		await navigateWithRouteDataResponse({
			api,
			href: "/head-idempotent-b",
			overrides: {
				metaHeadEls: [
					{
						tag: "meta",
						attributesKnownSafe: {
							name: "description",
							content: "Idempotent",
						},
						booleanAttributes: [],
					},
				],
			},
		});
		const secondElement = getElementsBetweenHeadSectionMarkers({
			type: "meta",
		})[0];

		expect(secondElement).toBe(firstElement);
		expect(
			getElementsBetweenHeadSectionMarkers({
				type: "meta",
			}),
		).toHaveLength(1);
	});

	it("updates style and script rest-head elements across consecutive navigations", async () => {
		const api = await loadPublicClientAPI();
		await navigateWithRouteDataResponse({
			api,
			href: "/head-rest-a",
			overrides: {
				restHeadEls: [
					{
						tag: "style",
						attributesKnownSafe: {},
						booleanAttributes: [],
						dangerousInnerHTML: "body { color: red; }",
					},
					{
						tag: "script",
						attributesKnownSafe: { src: "/runtime-a.js" },
						booleanAttributes: [],
					},
				],
			},
		});
		let restElements = getElementsBetweenHeadSectionMarkers({
			type: "rest",
		});
		expect(restElements).toHaveLength(2);
		expect(restElements[0]?.tagName.toLowerCase()).toBe("style");
		expect(restElements[0]?.innerHTML).toBe("body { color: red; }");
		expect(restElements[1]?.tagName.toLowerCase()).toBe("script");
		expect(restElements[1]?.getAttribute("src")).toBe("/runtime-a.js");

		await navigateWithRouteDataResponse({
			api,
			href: "/head-rest-b",
			overrides: {
				restHeadEls: [
					{
						tag: "style",
						attributesKnownSafe: {},
						booleanAttributes: [],
						dangerousInnerHTML: ".bar { font-weight: bold; }",
					},
					{
						tag: "script",
						attributesKnownSafe: { src: "/runtime-a.js" },
						booleanAttributes: ["async"],
					},
				],
			},
		});
		restElements = getElementsBetweenHeadSectionMarkers({
			type: "rest",
		});
		expect(restElements).toHaveLength(2);
		expect(restElements[0]?.innerHTML).toBe(".bar { font-weight: bold; }");
		expect(restElements[1]?.getAttribute("src")).toBe("/runtime-a.js");
		expect(restElements[1]?.hasAttribute("async")).toBe(true);

		await navigateWithRouteDataResponse({
			api,
			href: "/head-rest-c",
			overrides: {
				restHeadEls: [],
			},
		});
		restElements = getElementsBetweenHeadSectionMarkers({
			type: "rest",
		});
		expect(restElements).toHaveLength(0);
	});

	it("keeps meta and rest managed sections isolated from each other", async () => {
		const api = await loadPublicClientAPI();
		const meta = document.createElement("meta");
		meta.setAttribute("name", "description");
		meta.setAttribute("content", "Meta keep");
		insertHeadSectionElement({
			type: "meta",
			element: meta,
		});

		await navigateWithRouteDataResponse({
			api,
			href: "/head-section-isolation",
			overrides: {
				metaHeadEls: [
					{
						tag: "meta",
						attributesKnownSafe: {
							name: "description",
							content: "Meta keep",
						},
						booleanAttributes: [],
					},
				],
				restHeadEls: [
					{
						tag: "script",
						attributesKnownSafe: { src: "/rest-only.js" },
						booleanAttributes: [],
					},
				],
			},
		});

		const metaElements = getElementsBetweenHeadSectionMarkers({
			type: "meta",
		});
		const restElements = getElementsBetweenHeadSectionMarkers({
			type: "rest",
		});
		expect(metaElements).toHaveLength(1);
		expect(metaElements[0]).toBe(meta);
		expect(restElements).toHaveLength(1);
		expect(restElements[0]?.getAttribute("src")).toBe("/rest-only.js");
	});

	it("cleans interleaved text nodes between managed head markers during reconciliation", async () => {
		const api = await loadPublicClientAPI();
		const { endComment } = getHeadSectionComments({
			type: "meta",
		});
		document.head.insertBefore(document.createTextNode("\n  "), endComment);
		const meta = document.createElement("meta");
		meta.setAttribute("name", "description");
		meta.setAttribute("content", "Text cleanup");
		document.head.insertBefore(meta, endComment);
		document.head.insertBefore(document.createTextNode("\n"), endComment);

		await navigateWithRouteDataResponse({
			api,
			href: "/head-clean-text",
			overrides: {
				metaHeadEls: [
					{
						tag: "meta",
						attributesKnownSafe: {
							name: "description",
							content: "Text cleanup",
						},
						booleanAttributes: [],
					},
				],
			},
		});

		expect(
			getNodesBetweenHeadSectionMarkers({
				type: "meta",
			}).some((node) => node.nodeType === Node.TEXT_NODE),
		).toBe(false);
		const elements = getElementsBetweenHeadSectionMarkers({
			type: "meta",
		});
		expect(elements).toHaveLength(1);
		expect(elements[0]).toBe(meta);
	});

	it("dispatches location listeners on commit and stops after cleanup", async () => {
		const api = await loadPublicClientAPI();
		const locations = new Array<{
			pathname: string;
			search: string;
			hash: string;
		}>();
		const cleanup = api.addLocationListener((event) => {
			locations.push(event.detail);
		});
		try {
			vi.spyOn(window, "fetch")
				.mockResolvedValueOnce(
					createRouteDataResponse({
						loadersData: [],
					}),
				)
				.mockResolvedValueOnce(
					createRouteDataResponse({
						loadersData: [],
					}),
				);

			await api.vormaNavigate("/location-a?one=1#hash-a");
			await vi.runAllTimersAsync();
			expect(locations.at(-1)).toEqual({
				pathname: "/location-a",
				search: "?one=1",
				hash: "#hash-a",
				state: null,
			});

			cleanup();
			const locationCountAfterCleanup = locations.length;

			await api.vormaNavigate("/location-b?two=2#hash-b");
			await vi.runAllTimersAsync();
			expect(locations).toHaveLength(locationCountAfterCleanup);
		} finally {
			cleanup();
		}
	});

	it("exposes a usable history instance", async () => {
		const api = await loadPublicClientAPI();
		const history = api.getHistoryInstance();
		expect(typeof history.push).toBe("function");
		expect(typeof history.replace).toBe("function");
		expect(typeof history.listen).toBe("function");
	});

	it("treats direct history pushes as outside vorma navigation lifecycle", async () => {
		const api = await loadPublicClientAPI();
		const history = api.getHistoryInstance();
		const fetchSpy = vi.spyOn(window, "fetch");

		history.push("/history-direct-push");
		await vi.runAllTimersAsync();

		expect(window.location.pathname).toBe("/history-direct-push");
		expect(fetchSpy).toHaveBeenCalledTimes(0);
		expectStatusIdle(api.getStatus());
	});

	it("emits location events when history keys change via direct history pushes", async () => {
		const api = await loadPublicClientAPI();
		const history = api.getHistoryInstance();
		const observedPaths: string[] = [];
		const cleanup = api.addLocationListener((event) => {
			observedPaths.push(event.detail.pathname);
		});
		try {
			history.push("/history-key-a");
			await vi.runAllTimersAsync();
			history.push("/history-key-b");
			await vi.runAllTimersAsync();

			expect(observedPaths).toContain("/history-key-a");
			expect(observedPaths).toContain("/history-key-b");
		} finally {
			cleanup();
		}
	});

	it("reflects direct history push state through getLocation", async () => {
		const api = await loadPublicClientAPI();
		const history = api.getHistoryInstance();

		history.push("/history-state", {
			source: "direct-push",
			id: 42,
		});
		await vi.runAllTimersAsync();

		expect(api.getLocation()).toEqual({
			pathname: "/history-state",
			search: "",
			hash: "",
			state: {
				source: "direct-push",
				id: 42,
			},
		});
	});

	it("treats direct history replace as outside navigation lifecycle", async () => {
		const api = await loadPublicClientAPI();
		const history = api.getHistoryInstance();
		const fetchSpy = vi.spyOn(window, "fetch");

		history.replace("/history-direct-replace", {
			source: "direct-replace",
		});
		await vi.runAllTimersAsync();

		expect(window.location.pathname).toBe("/history-direct-replace");
		expect(api.getLocation().state).toEqual({
			source: "direct-replace",
		});
		expect(fetchSpy).toHaveBeenCalledTimes(0);
		expectStatusIdle(api.getStatus());
	});

	it("applies hash-element scroll on same-document POP transitions", async () => {
		const api = await loadPublicClientAPI();
		const history = api.getHistoryInstance();
		const sectionElement = document.createElement("div");
		sectionElement.id = "same-pop-section";
		const scrollIntoViewSpy = vi.fn();
		Object.defineProperty(sectionElement, "scrollIntoView", {
			value: scrollIntoViewSpy,
			configurable: true,
		});
		document.body.appendChild(sectionElement);
		try {
			history.push("/same-pop#same-pop-section");
			await vi.runAllTimersAsync();
			history.push("/same-pop");
			await vi.runAllTimersAsync();

			history.back();
			await vi.runAllTimersAsync();

			expect(window.location.pathname).toBe("/same-pop");
			expect(window.location.hash).toBe("#same-pop-section");
			expect(scrollIntoViewSpy).toHaveBeenCalledTimes(1);
		} finally {
			sectionElement.remove();
		}
	});

	it("applies coordinate and hash-based scroll states on same-document POP transitions", async () => {
		const api = await loadPublicClientAPI();
		const history = api.getHistoryInstance();
		const coordinateScrollSpy = vi.spyOn(window, "scrollTo");
		const hashElement = document.createElement("div");
		hashElement.id = "scroll-hash-target";
		const hashScrollSpy = vi.fn();
		Object.defineProperty(hashElement, "scrollIntoView", {
			value: hashScrollSpy,
			configurable: true,
		});
		document.body.appendChild(hashElement);
		try {
			history.push("/scroll-coordinates");
			await vi.runAllTimersAsync();
			const coordinateLocationKey = history.location.key;
			seedScrollStateForTesting([
				{
					historyKey: coordinateLocationKey,
					x: 100,
					y: 200,
				},
			]);
			history.push("/scroll-coordinates#section");
			await vi.runAllTimersAsync();
			history.back();
			await vi.runAllTimersAsync();

			expect(coordinateScrollSpy).toHaveBeenCalledWith(100, 200);

			history.push("/scroll-hash#scroll-hash-target");
			await vi.runAllTimersAsync();
			history.push("/scroll-hash");
			await vi.runAllTimersAsync();
			history.back();
			await vi.runAllTimersAsync();

			expect(hashScrollSpy).toHaveBeenCalledTimes(1);
		} finally {
			hashElement.remove();
		}
	});

	it("includes restored scroll state in browser-history route-change events", async () => {
		const api = await loadPublicClientAPI();
		const history = api.getHistoryInstance();
		const routeChangeDetails: Array<unknown> = [];
		const removeRouteChangeListener = api.addRouteChangeListener(
			(event) => {
				routeChangeDetails.push(event.detail);
			},
		);
		try {
			vi.spyOn(window, "fetch").mockResolvedValue(
				createRouteDataResponse({
					loadersData: [],
				}),
			);
			history.push("/history-pop-scroll-target");
			await vi.runAllTimersAsync();
			const targetHistoryKey = history.location.key;
			history.push("/history-pop-scroll-source");
			await vi.runAllTimersAsync();

			seedScrollStateForTesting([
				{
					historyKey: targetHistoryKey,
					x: 120,
					y: 240,
				},
			]);
			history.back();
			await vi.runAllTimersAsync();

			expect(window.location.pathname).toBe("/history-pop-scroll-target");
			expect(routeChangeDetails.at(-1)).toEqual(
				expect.objectContaining({
					__scrollState: { x: 120, y: 240 },
				}),
			);
		} finally {
			removeRouteChangeListener();
		}
	});

	it("uses one decode step when resolving same-document POP hash targets", async () => {
		const api = await loadPublicClientAPI();
		const history = api.getHistoryInstance();
		const percentLiteralElement = document.createElement("div");
		percentLiteralElement.id = "%20-literal";
		const scrollIntoViewSpy = vi.fn();
		Object.defineProperty(percentLiteralElement, "scrollIntoView", {
			value: scrollIntoViewSpy,
			configurable: true,
		});
		document.body.appendChild(percentLiteralElement);
		try {
			history.push("/single-decode#%2520-literal");
			await vi.runAllTimersAsync();
			history.push("/single-decode");
			await vi.runAllTimersAsync();

			history.back();
			await vi.runAllTimersAsync();

			expect(window.location.pathname).toBe("/single-decode");
			expect(window.location.hash).toBe("#%2520-literal");
			expect(scrollIntoViewSpy).toHaveBeenCalledTimes(1);
		} finally {
			percentLiteralElement.remove();
		}
	});

	it("resolves encoded unicode hash targets on same-document POP transitions", async () => {
		const api = await loadPublicClientAPI();
		const history = api.getHistoryInstance();
		const checkmarkElement = document.createElement("div");
		checkmarkElement.id = "✓";
		const scrollIntoViewSpy = vi.fn();
		Object.defineProperty(checkmarkElement, "scrollIntoView", {
			value: scrollIntoViewSpy,
			configurable: true,
		});
		document.body.appendChild(checkmarkElement);
		try {
			history.push("/decoded-unicode#%E2%9C%93");
			await vi.runAllTimersAsync();
			history.push("/decoded-unicode");
			await vi.runAllTimersAsync();

			history.back();
			await vi.runAllTimersAsync();

			expect(window.location.pathname).toBe("/decoded-unicode");
			expect(window.location.hash).toBe("#%E2%9C%93");
			expect(scrollIntoViewSpy).toHaveBeenCalledTimes(1);
		} finally {
			checkmarkElement.remove();
		}
	});

	it("falls back to raw hash fragments when decode fails during same-document POP", async () => {
		const api = await loadPublicClientAPI();
		const history = api.getHistoryInstance();
		const invalidEncodedHashFragment = "%E0%A4%A";
		const rawFragmentElement = document.createElement("div");
		rawFragmentElement.id = invalidEncodedHashFragment;
		const scrollIntoViewSpy = vi.fn();
		Object.defineProperty(rawFragmentElement, "scrollIntoView", {
			value: scrollIntoViewSpy,
			configurable: true,
		});
		document.body.appendChild(rawFragmentElement);
		try {
			history.push(`/decode-fallback#${invalidEncodedHashFragment}`);
			await vi.runAllTimersAsync();
			history.push("/decode-fallback");
			await vi.runAllTimersAsync();

			history.back();
			await vi.runAllTimersAsync();

			expect(window.location.pathname).toBe("/decode-fallback");
			expect(window.location.hash).toBe(`#${invalidEncodedHashFragment}`);
			expect(scrollIntoViewSpy).toHaveBeenCalledTimes(1);
		} finally {
			rawFragmentElement.remove();
		}
	});

	it("does not re-scroll when same-document POP hash targets are encoding-equivalent", async () => {
		const api = await loadPublicClientAPI();
		const history = api.getHistoryInstance();
		const tildeElement = document.createElement("div");
		tildeElement.id = "~";
		const scrollIntoViewSpy = vi.fn();
		Object.defineProperty(tildeElement, "scrollIntoView", {
			value: scrollIntoViewSpy,
			configurable: true,
		});
		document.body.appendChild(tildeElement);
		try {
			history.push("/equivalent-hash#~");
			await vi.runAllTimersAsync();
			history.push("/equivalent-hash#%7E");
			await vi.runAllTimersAsync();

			history.back();
			await vi.runAllTimersAsync();

			expect(window.location.pathname).toBe("/equivalent-hash");
			expect(scrollIntoViewSpy).toHaveBeenCalledTimes(0);
		} finally {
			tildeElement.remove();
		}
	});

	it("fetches and commits route data for cross-document POP transitions", async () => {
		const api = await loadPublicClientAPI();
		const history = api.getHistoryInstance();
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				loadersData: [],
				title: {
					dangerousInnerHTML: "POP Commit Title",
				},
			}),
		);

		history.push("/cross-pop-target");
		await vi.runAllTimersAsync();
		history.push("/cross-pop-source");
		await vi.runAllTimersAsync();

		history.back();
		await vi.runAllTimersAsync();

		expect(fetchSpy).toHaveBeenCalledTimes(1);
		const firstPOPFetchURL = requestInputToURL(
			fetchSpy.mock.calls[0]?.[0] as RequestInfo | URL,
		);
		expect(firstPOPFetchURL.pathname).toBe("/cross-pop-target");
		expect(firstPOPFetchURL.searchParams.has("vorma_json")).toBe(true);
		expect(window.location.pathname).toBe("/cross-pop-target");
		expect(document.title).toBe("POP Commit Title");
		expectStatusIdle(api.getStatus());
	});

	it("follows cross-document POP redirects and commits redirected destination", async () => {
		const api = await loadPublicClientAPI();
		const history = api.getHistoryInstance();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				new Response("", {
					status: 302,
					headers: {
						"X-Client-Redirect": "/pop-redirect-destination",
						"X-Wave-Framework-Build-Id": "2",
					},
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse(
					{
						loadersData: [],
						title: {
							dangerousInnerHTML: "POP Redirect Destination",
						},
					},
					{
						headers: {
							"X-Wave-Framework-Build-Id": "2",
						},
					},
				),
			);

		history.push("/pop-redirect-target");
		await vi.runAllTimersAsync();
		history.push("/pop-redirect-source");
		await vi.runAllTimersAsync();

		history.back();
		await vi.runAllTimersAsync();

		expect(fetchSpy).toHaveBeenCalledTimes(2);
		expect(
			requestInputToURL(fetchSpy.mock.calls[0]?.[0] as RequestInfo | URL)
				.pathname,
		).toBe("/pop-redirect-target");
		expect(
			requestInputToURL(fetchSpy.mock.calls[1]?.[0] as RequestInfo | URL)
				.pathname,
		).toBe("/pop-redirect-destination");
		expect(window.location.pathname).toBe("/pop-redirect-destination");
		expect(document.title).toBe("POP Redirect Destination");
		expectStatusIdle(api.getStatus());
	});

	it("uses the POP listener payload URL as fetch source-of-truth", async () => {
		const api = await loadPublicClientAPI();
		const history = api.getHistoryInstance();
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				loadersData: [],
			}),
		);

		history.push("/listener-source?q=1#a");
		await vi.runAllTimersAsync();
		history.push("/listener-target?q=2#details");
		await vi.runAllTimersAsync();

		window.history.replaceState(
			window.history.state,
			"",
			"/window-location-only?q=99#ignored",
		);
		history.back();
		await vi.runAllTimersAsync();

		expect(fetchSpy).toHaveBeenCalledTimes(1);
		const popFetchURL = requestInputToURL(
			fetchSpy.mock.calls[0]?.[0] as RequestInfo | URL,
		);
		expect(popFetchURL.pathname).toBe("/listener-source");
		expect(popFetchURL.searchParams.get("q")).toBe("1");
	});

	it("treats query-order changes as different POP targets", async () => {
		const api = await loadPublicClientAPI();
		const history = api.getHistoryInstance();
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				loadersData: [],
			}),
		);

		history.push("/pop-query-order?a=1&b=2");
		await vi.runAllTimersAsync();
		history.push("/pop-query-order?b=2&a=1");
		await vi.runAllTimersAsync();

		history.back();
		await vi.runAllTimersAsync();

		expect(fetchSpy).toHaveBeenCalledTimes(1);
		const popFetchURL = requestInputToURL(
			fetchSpy.mock.calls[0]?.[0] as RequestInfo | URL,
		);
		expect(popFetchURL.pathname).toBe("/pop-query-order");
		expect(popFetchURL.searchParams.get("a")).toBe("1");
		expect(popFetchURL.searchParams.get("b")).toBe("2");
	});

	it("reloads the browser when cross-document POP navigation fails in client runtime", async () => {
		const api = await loadPublicClientAPI();
		const history = api.getHistoryInstance();
		const hardRedirectSpy = vi.fn();
		setHardRedirectHandlerForTesting((href) => {
			hardRedirectSpy(href);
		});
		const consoleErrorSpy = vi
			.spyOn(console, "error")
			.mockImplementation(() => {});
		vi.spyOn(window, "fetch").mockRejectedValue(
			new Error("pop-fetch-failure"),
		);
		try {
			history.push("/pop-reload-target");
			await vi.runAllTimersAsync();
			history.push("/pop-reload-source");
			await vi.runAllTimersAsync();

			history.back();
			await vi.runAllTimersAsync();

			expect(hardRedirectSpy).toHaveBeenCalledTimes(1);
			expect(String(hardRedirectSpy.mock.calls[0]?.[0])).toContain(
				"/pop-reload-target",
			);
			const hadPopFailureLog = consoleErrorSpy.mock.calls.some((args) =>
				args.some(
					(value) =>
						typeof value === "string" &&
						value.includes("Cross-document POP navigation failed"),
				),
			);
			expect(hadPopFailureLog).toBe(true);
		} finally {
			setHardRedirectHandlerForTesting(undefined);
		}
	});

	it("logs when hard-reload fallback fails after cross-document POP navigation errors", async () => {
		const api = await loadPublicClientAPI();
		const history = api.getHistoryInstance();
		setHardRedirectHandlerForTesting(() => {
			throw new Error("reload-failed");
		});
		const consoleErrorSpy = vi
			.spyOn(console, "error")
			.mockImplementation(() => {});
		vi.spyOn(window, "fetch").mockRejectedValue(
			new Error("pop-fetch-failure"),
		);
		try {
			history.push("/pop-reload-error-target");
			await vi.runAllTimersAsync();
			history.push("/pop-reload-error-source");
			await vi.runAllTimersAsync();

			history.back();
			await vi.runAllTimersAsync();

			const hadHardRedirectFailureLog = consoleErrorSpy.mock.calls.some(
				(args) =>
					args.some(
						(value) =>
							typeof value === "string" &&
							value.includes("Hard redirect fallback failed"),
					),
			);
			expect(hadHardRedirectFailureLog).toBe(true);
		} finally {
			setHardRedirectHandlerForTesting(undefined);
		}
	});

	it("saves outgoing scroll state before cross-document POP commits", async () => {
		const api = await loadPublicClientAPI();
		const history = api.getHistoryInstance();
		vi.spyOn(window, "fetch").mockImplementation(() =>
			Promise.resolve(
				createRouteDataResponse({
					loadersData: [],
				}),
			),
		);

		history.push("/scroll-pop-target");
		await vi.runAllTimersAsync();
		history.push("/scroll-pop-source");
		await vi.runAllTimersAsync();
		const sourceHistoryKey = history.location.key;
		(window as AnyPropertyRecord).scrollX = 50;
		(window as AnyPropertyRecord).scrollY = 100;

		history.back();
		await vi.runAllTimersAsync();

		expect(readScrollStateForTesting()).toContainEqual({
			historyKey: sourceHistoryKey,
			x: 50,
			y: 100,
		});
	});

	it("ignores malformed session-stored scroll entries and persists fresh valid state", async () => {
		const api = await loadPublicClientAPI();
		const history = api.getHistoryInstance();
		history.push("/malformed-scroll-map-source");
		await vi.runAllTimersAsync();
		const sourceHistoryKey = history.location.key;
		writeRawScrollStateStorageForTesting(
			JSON.stringify([
				["valid-old", { x: 10, y: 20 }],
				["missing-y", { x: 1 }],
				["bad-x", { x: "10", y: 20 }],
				[123, { x: 1, y: 2 }],
				["extra-shape", { x: 1, y: 2, z: 3 }],
				["nan-shape", { x: null, y: 2 }],
				"not-an-entry",
			]),
		);
		(window as AnyPropertyRecord).scrollX = 150;
		(window as AnyPropertyRecord).scrollY = 300;
		vi.spyOn(window, "fetch").mockImplementation(() =>
			Promise.resolve(
				createRouteDataResponse({
					loadersData: [],
				}),
			),
		);

		await api.vormaNavigate("/malformed-scroll-map-target");
		await vi.runAllTimersAsync();

		const storedEntries = readScrollStateForTesting();
		expect(storedEntries).toContainEqual({
			historyKey: "valid-old",
			x: 10,
			y: 20,
		});
		expect(storedEntries).toContainEqual({
			historyKey: sourceHistoryKey,
			x: 150,
			y: 300,
		});
		for (const entry of storedEntries) {
			expect(typeof entry.historyKey).toBe("string");
			expect(Number.isFinite(entry.x)).toBe(true);
			expect(Number.isFinite(entry.y)).toBe(true);
		}
	});

	it("keeps scroll-state storage at 50 entries with oldest-first eviction", async () => {
		const api = await loadPublicClientAPI();
		const history = api.getHistoryInstance();
		seedScrollStateForTesting([]);
		vi.spyOn(window, "fetch").mockImplementation(() =>
			Promise.resolve(
				createRouteDataResponse({
					loadersData: [],
				}),
			),
		);

		history.push("/scroll-fifo-start");
		await vi.runAllTimersAsync();
		const oldestHistoryKey = history.location.key;

		for (let index = 0; index <= 50; index += 1) {
			(window as AnyPropertyRecord).scrollX = index;
			(window as AnyPropertyRecord).scrollY = index;
			await api.vormaNavigate(`/scroll-fifo-target-${index}`);
			await vi.runAllTimersAsync();
		}

		const storedEntries = readScrollStateForTesting();
		expect(storedEntries).toHaveLength(50);
		expect(
			storedEntries.some(
				(entry) => entry.historyKey === oldestHistoryKey,
			),
		).toBe(false);
		expect(
			storedEntries.some((entry) => entry.x === 50 && entry.y === 50),
		).toBe(true);
	});

	it("restores saved scroll coordinates when same-document POP removes a hash", async () => {
		const api = await loadPublicClientAPI();
		const history = api.getHistoryInstance();
		history.push("/hash-remove-target");
		await vi.runAllTimersAsync();
		const targetHistoryKey = history.location.key;
		seedScrollStateForTesting([
			{
				historyKey: targetHistoryKey,
				x: 75,
				y: 150,
			},
		]);
		history.push("/hash-remove-target#section");
		await vi.runAllTimersAsync();

		history.back();
		await vi.runAllTimersAsync();

		expect(window.location.pathname).toBe("/hash-remove-target");
		expect(window.location.hash).toBe("");
		expect(window.scrollTo).toHaveBeenCalledWith(75, 150);
	});

	it("falls back to origin scroll when same-document POP removes hash without stored state", async () => {
		const api = await loadPublicClientAPI();
		const history = api.getHistoryInstance();
		history.push("/hash-remove-origin-target");
		await vi.runAllTimersAsync();
		history.push("/hash-remove-origin-target#section");
		await vi.runAllTimersAsync();

		history.back();
		await vi.runAllTimersAsync();

		expect(window.location.pathname).toBe("/hash-remove-origin-target");
		expect(window.location.hash).toBe("");
		expect(window.scrollTo).toHaveBeenCalledWith(0, 0);
	});

	it("restores saved scroll coordinates when same-document POP targets empty fragment '#'", async () => {
		const api = await loadPublicClientAPI();
		const history = api.getHistoryInstance();
		history.push("/hash-empty-fragment-target#");
		await vi.runAllTimersAsync();
		const targetHistoryKey = history.location.key;
		seedScrollStateForTesting([
			{
				historyKey: targetHistoryKey,
				x: 88,
				y: 166,
			},
		]);
		history.push("/hash-empty-fragment-target#section");
		await vi.runAllTimersAsync();

		history.back();
		await vi.runAllTimersAsync();

		expect(window.location.pathname).toBe("/hash-empty-fragment-target");
		expect(window.location.hash).toBe("");
		expect(window.scrollTo).toHaveBeenCalledWith(88, 166);
	});

	it("saves outgoing scroll state before programmatic navigation pushes a new history entry", async () => {
		const api = await loadPublicClientAPI();
		const history = api.getHistoryInstance();
		history.push("/navigate-scroll-source");
		await vi.runAllTimersAsync();
		const sourceHistoryKey = history.location.key;
		(window as AnyPropertyRecord).scrollX = 150;
		(window as AnyPropertyRecord).scrollY = 300;
		vi.spyOn(window, "fetch").mockImplementation(() =>
			Promise.resolve(
				createRouteDataResponse({
					loadersData: [],
				}),
			),
		);

		await api.vormaNavigate("/navigate-scroll-target");
		await vi.runAllTimersAsync();

		expect(readScrollStateForTesting()).toContainEqual({
			historyKey: sourceHistoryKey,
			x: 150,
			y: 300,
		});
	});

	it("does not dispatch location events when history keys do not change", async () => {
		const api = await loadPublicClientAPI();
		const observedPathnames: string[] = [];
		const removeLocationListener = api.addLocationListener((event) => {
			observedPathnames.push(event.detail.pathname);
		});
		try {
			window.history.replaceState(window.history.state, "", "/same-key");
			await vi.runAllTimersAsync();

			expect(observedPathnames).toEqual([]);
		} finally {
			removeLocationListener();
		}
	});

	it("dispatches route-change after title is committed", async () => {
		const api = await loadPublicClientAPI();
		const observedTitles = new Array<string>();
		const cleanup = api.addRouteChangeListener(() => {
			observedTitles.push(document.title);
		});
		try {
			vi.spyOn(window, "fetch").mockResolvedValue(
				createRouteDataResponse({
					loadersData: [],
					title: { dangerousInnerHTML: "Committed Before Event" },
				}),
			);

			await api.vormaNavigate("/title-order");
			await vi.runAllTimersAsync();

			expect(observedTitles).toContain("Committed Before Event");
		} finally {
			cleanup();
		}
	});

	it("dispatches route-change only after committed location and router snapshot are coherent", async () => {
		const api = await loadPublicClientAPI();
		const observedSnapshots: Array<{
			pathname: string;
			routerData: ReturnType<PublicClientAPI["getRouterData"]>;
		}> = [];
		const cleanup = api.addRouteChangeListener(() => {
			observedSnapshots.push({
				pathname: api.getLocation().pathname,
				routerData: api.getRouterData(),
			});
		});
		try {
			vi.spyOn(window, "fetch").mockResolvedValue(
				createRouteDataResponse({
					loadersData: [],
				}),
			);

			await api.vormaNavigate("/snapshot-coherence");
			await vi.runAllTimersAsync();

			expect(observedSnapshots).toHaveLength(1);
			expect(observedSnapshots[0]?.pathname).toBe("/snapshot-coherence");
			expect(observedSnapshots[0]?.routerData).toEqual(
				api.getRouterData(),
			);
		} finally {
			cleanup();
		}
	});

	it("emits hash scroll state in route-change events for hash-only navigation", async () => {
		const api = await loadPublicClientAPI();
		window.history.replaceState({}, "", "/hash-scroll-base");
		const routeChangeDetails: Array<any> = [];
		const cleanup = api.addRouteChangeListener((event) => {
			routeChangeDetails.push(event.detail);
		});
		try {
			await api.vormaNavigate("/hash-scroll-base#chapter-1");
			await vi.runAllTimersAsync();

			expect(window.location.hash).toBe("#chapter-1");
			expect(routeChangeDetails.at(-1)).toEqual({
				__scrollState: {
					hash: "#chapter-1",
				},
			});
		} finally {
			cleanup();
		}
	});

	it("emits top scroll state in route-change events for standard navigation", async () => {
		const api = await loadPublicClientAPI();
		const routeChangeDetails: Array<any> = [];
		const cleanup = api.addRouteChangeListener((event) => {
			routeChangeDetails.push(event.detail);
		});
		try {
			vi.spyOn(window, "fetch").mockResolvedValue(
				createRouteDataResponse({
					loadersData: [],
				}),
			);

			await api.vormaNavigate("/top-scroll-target");
			await vi.runAllTimersAsync();

			expect(routeChangeDetails.at(-1)).toEqual({
				__scrollState: {
					x: 0,
					y: 0,
				},
			});
		} finally {
			cleanup();
		}
	});

	it("emits undefined scroll state when scrollToTop is disabled", async () => {
		const api = await loadPublicClientAPI();
		const routeChangeDetails: Array<any> = [];
		const cleanup = api.addRouteChangeListener((event) => {
			routeChangeDetails.push(event.detail);
		});
		try {
			vi.spyOn(window, "fetch").mockResolvedValue(
				createRouteDataResponse({
					loadersData: [],
				}),
			);

			await api.vormaNavigate("/no-top-scroll-target", {
				scrollToTop: false,
			});
			await vi.runAllTimersAsync();

			expect(routeChangeDetails.at(-1)).toEqual({
				__scrollState: undefined,
			});
		} finally {
			cleanup();
		}
	});

	it("keeps the latest started navigation authoritative when completions race", async () => {
		const api = await loadPublicClientAPI();
		const { requests } = createAbortAwareFetchRecorder();

		const first = api.vormaNavigate("/race-first");
		const second = api.vormaNavigate("/race-second");

		await waitForRequestCount({ requests, count: 2 });

		requests[1]?.resolve(
			createRouteDataResponse({
				loadersData: [],
				title: { dangerousInnerHTML: "Race Second" },
			}),
		);
		await Promise.resolve();
		requests[0]?.resolve(
			createRouteDataResponse({
				loadersData: [],
				title: { dangerousInnerHTML: "Race First" },
			}),
		);

		await Promise.allSettled([first, second]);
		await vi.runAllTimersAsync();

		expect(window.location.pathname).toBe("/race-second");
		expect(document.title).toBe("Race Second");
		expectStatusIdle(api.getStatus());
	});

	it.each([11, 29, 47, 83, 131])(
		"keeps last-started navigation authoritative across generated resolve permutations (seed=%d)",
		async (seed) => {
			const api = await loadPublicClientAPI();
			const { requests } = createAbortAwareFetchRecorder();

			const { result, unhandledRejections } =
				await withUnhandledRejectionCapture({
					run: async () => {
						const navigationCount = 6;
						const navigationPromises: Array<
							ReturnType<PublicClientAPI["vormaNavigate"]>
						> = [];

						for (let i = 0; i < navigationCount; i += 1) {
							navigationPromises.push(
								api.vormaNavigate(`/generated-${seed}-${i}`),
							);
						}

						await waitForRequestCount({
							requests,
							count: navigationCount,
						});

						const resolveOrder =
							buildShuffledIndicesForSeededRandomization({
								length: navigationCount,
								seed,
							});
						for (const requestIndex of resolveOrder) {
							const request = requests[requestIndex];
							if (!request) {
								throw new Error(
									`Missing generated request at index ${requestIndex}`,
								);
							}
							request.resolve(
								createRouteDataResponse({
									loadersData: [],
									title: {
										dangerousInnerHTML: `Generated ${requestIndex}`,
									},
								}),
							);
							await Promise.resolve();
						}

						await Promise.all(navigationPromises);
						return {
							expectedPathname: `/generated-${seed}-${navigationCount - 1}`,
							expectedTitle: `Generated ${navigationCount - 1}`,
						};
					},
				});

			expect(unhandledRejections).toEqual([]);
			expect(window.location.pathname).toBe(result.expectedPathname);
			expect(document.title).toBe(result.expectedTitle);
			expectStatusIdle(api.getStatus());
		},
	);

	it("preserves invariants across mixed prefetch/navigate/submit/revalidate sequences", async () => {
		const api = await loadPublicClientAPI();
		const { requests } = createAbortAwareFetchRecorder();

		const { result, unhandledRejections } =
			await withUnhandledRejectionCapture({
				run: async () => {
					const prefetchPromise = api.vormaNavigate(
						"/prefetch-seed#one",
						{
							intent: "prefetch",
						} as never,
					);
					await waitForRequestCount({ requests, count: 1 });

					const navigationFromPrefetch =
						api.vormaNavigate("/prefetch-seed#two");
					await Promise.resolve();
					expect(requests).toHaveLength(1);

					requests[0]?.resolve(
						createRouteDataResponse({
							loadersData: [],
							title: {
								dangerousInnerHTML: "Prefetch Seed",
							},
						}),
					);
					await Promise.all([
						prefetchPromise,
						navigationFromPrefetch,
					]);
					await vi.runAllTimersAsync();
					expect(window.location.pathname).toBe("/prefetch-seed");
					expect(window.location.hash).toBe("#two");

					const firstNavigation = api.vormaNavigate("/race-first");
					await waitForRequestCount({ requests, count: 2 });
					const secondNavigation = api.vormaNavigate("/race-second");
					await waitForRequestCount({ requests, count: 3 });
					expect(api.getStatus().isNavigating).toBe(true);

					requests[2]?.resolve(
						createRouteDataResponse({
							loadersData: [],
							title: {
								dangerousInnerHTML: "Race Second",
							},
						}),
					);
					await secondNavigation;

					requests[1]?.resolve(
						createRouteDataResponse({
							loadersData: [],
							title: {
								dangerousInnerHTML: "Race First Stale",
							},
						}),
					);
					await Promise.allSettled([firstNavigation]);

					expect(window.location.pathname).toBe("/race-second");
					expect(document.title).toBe("Race Second");

					const submitOne = api.submit(
						"/mutation",
						{ method: "POST", body: JSON.stringify({ step: 1 }) },
						{ dedupeKey: "model-seq" },
					);
					await waitForRequestCount({ requests, count: 4 });

					const submitTwo = api.submit(
						"/mutation",
						{ method: "POST", body: JSON.stringify({ step: 2 }) },
						{ dedupeKey: "model-seq" },
					);
					await waitForRequestCount({ requests, count: 5 });
					expect(api.getStatus().isSubmitting).toBe(true);

					const submitTwoRequest = requests[4];
					if (!submitTwoRequest) {
						throw new Error("Missing second submit request.");
					}
					const submitTwoURL = requestInputToURL(
						submitTwoRequest.input,
					);
					expect(submitTwoURL.pathname).toBe("/mutation");
					expect(submitTwoRequest.init?.method).toBe("POST");

					submitTwoRequest.resolve(
						new Response(JSON.stringify({ ok: true }), {
							status: 200,
							headers: {
								"Content-Type": "application/json",
							},
						}),
					);

					await waitForRequestCount({ requests, count: 6 });
					const revalidationRequest = requests[5];
					if (!revalidationRequest) {
						throw new Error("Missing revalidation request.");
					}
					const revalidationURL = requestInputToURL(
						revalidationRequest.input,
					);
					expect(revalidationURL.pathname).toBe("/race-second");
					expect(revalidationURL.searchParams.get("vorma_json")).toBe(
						"1",
					);

					const thirdNavigation = api.vormaNavigate("/race-third");
					await waitForRequestCount({ requests, count: 7 });
					const thirdNavigationRequest = requests[6];
					if (!thirdNavigationRequest) {
						throw new Error("Missing third navigation request.");
					}

					thirdNavigationRequest.resolve(
						createRouteDataResponse({
							loadersData: [],
							title: {
								dangerousInnerHTML: "Race Third",
							},
						}),
					);
					await thirdNavigation;

					revalidationRequest.resolve(
						createRouteDataResponse({
							loadersData: [],
							title: {
								dangerousInnerHTML: "Stale Revalidation",
							},
						}),
					);

					const [submitOneResult, submitTwoResult] =
						await Promise.all([submitOne, submitTwo]);
					return { submitOneResult, submitTwoResult };
				},
			});

		expect(unhandledRejections).toEqual([]);
		expect(result.submitOneResult).toEqual({
			success: false,
			error: "Aborted",
		});
		expect(result.submitTwoResult).toEqual({
			success: true,
			data: { ok: true },
		});
		expect(window.location.pathname).toBe("/race-third");
		expect(document.title).toBe("Race Third");
		expectStatusIdle(api.getStatus());
	});

	it("records terminal lifecycle states with explicit reasons for settled operations", async () => {
		const api = await loadPublicClientAPI();
		const { requests } = createAbortAwareFetchRecorder();

		const { result, unhandledRejections } =
			await withUnhandledRejectionCapture({
				run: async () => {
					const firstSubmit = api.submit(
						"/terminal-submit",
						{
							method: "POST",
							body: JSON.stringify({ step: 1 }),
						},
						{
							dedupeKey: "terminal-key",
							revalidate: false,
						},
					);
					await waitForRequestCount({ requests, count: 1 });

					const secondSubmit = api.submit(
						"/terminal-submit",
						{
							method: "POST",
							body: JSON.stringify({ step: 2 }),
						},
						{
							dedupeKey: "terminal-key",
							revalidate: false,
						},
					);
					await waitForRequestCount({ requests, count: 2 });

					const secondSubmitRequest = requests[1];
					if (!secondSubmitRequest) {
						throw new Error(
							"Missing second terminal submit request.",
						);
					}
					secondSubmitRequest.resolve(
						new Response(JSON.stringify({ ok: true }), {
							status: 200,
							headers: {
								"Content-Type": "application/json",
							},
						}),
					);

					const [firstResult, secondResult] = await Promise.all([
						firstSubmit,
						secondSubmit,
					]);

					const failedSubmit = api.submit(
						"/terminal-submit-failure",
						{
							method: "POST",
						},
						{
							revalidate: false,
						},
					);
					await waitForRequestCount({ requests, count: 3 });

					const failedSubmitRequest = requests[2];
					if (!failedSubmitRequest) {
						throw new Error(
							"Missing failed terminal submit request.",
						);
					}
					failedSubmitRequest.resolve(
						new Response("failed", {
							status: 500,
							statusText: "Failure",
						}),
					);
					const failureResult = await failedSubmit;
					return { firstResult, secondResult, failureResult };
				},
			});

		expect(unhandledRejections).toEqual([]);
		expect(result.firstResult).toEqual({
			success: false,
			error: "Aborted",
		});
		expect(result.secondResult).toEqual({
			success: true,
			data: { ok: true },
		});
		expect(result.failureResult.success).toBe(false);
		if (result.failureResult.success) {
			throw new Error("Expected submit failure terminal outcome.");
		}
		expect(result.failureResult.error).not.toHaveLength(0);
		expectStatusIdle(api.getStatus());
	});

	it("does not fetch for same-document no-op programmatic navigation", async () => {
		const api = await loadPublicClientAPI();
		window.history.replaceState({}, "", "/same-noop");
		const fetchSpy = vi.spyOn(window, "fetch");

		await api.vormaNavigate("/same-noop");
		await vi.runAllTimersAsync();

		expect(fetchSpy).not.toHaveBeenCalled();
		expect(window.location.pathname).toBe("/same-noop");
		expectStatusIdle(api.getStatus());
	});

	it("classifies same-document programmatic targets as noop, hash-change, and navigate", async () => {
		const api = await loadPublicClientAPI();
		window.history.replaceState({}, "", "/same-doc-classify?a=1&b=2#one");
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				loadersData: [],
			}),
		);

		await api.vormaNavigate("/same-doc-classify?a=1&b=2#one");
		await vi.runAllTimersAsync();
		expect(fetchSpy).toHaveBeenCalledTimes(0);

		await api.vormaNavigate("/same-doc-classify?a=1&b=2#two");
		await vi.runAllTimersAsync();
		expect(fetchSpy).toHaveBeenCalledTimes(0);
		expect(window.location.hash).toBe("#two");

		await api.vormaNavigate("/same-doc-classify?b=2&a=1#two");
		await vi.runAllTimersAsync();
		expect(fetchSpy).toHaveBeenCalledTimes(1);
		expect(window.location.pathname).toBe("/same-doc-classify");
		expect(window.location.search).toBe("?b=2&a=1");
		expect(window.location.hash).toBe("#two");
	});

	it("replaces history state on same-document no-op when replace=true", async () => {
		const api = await loadPublicClientAPI();
		window.history.replaceState({ from: "old" }, "", "/same-noop-replace");
		const fetchSpy = vi.spyOn(window, "fetch");

		await api.vormaNavigate("/same-noop-replace", {
			replace: true,
			state: {
				from: "new",
			},
		});
		await vi.runAllTimersAsync();

		expect(fetchSpy).not.toHaveBeenCalled();
		expect(api.getLocation()).toEqual({
			pathname: "/same-noop-replace",
			search: "",
			hash: "",
			state: {
				from: "new",
			},
		});
	});

	it("uses history.replace when navigate is called with replace=true", async () => {
		const api = await loadPublicClientAPI();
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				loadersData: [],
			}),
		);
		const pushSpy = vi.spyOn(window.history, "pushState");
		const replaceSpy = vi.spyOn(window.history, "replaceState");

		await api.vormaNavigate("/replace-target", {
			replace: true,
			state: { source: "replace-test" },
		});
		await vi.runAllTimersAsync();

		expect(fetchSpy).toHaveBeenCalledTimes(1);
		expect(pushSpy).toHaveBeenCalledTimes(0);
		expect(replaceSpy).toHaveBeenCalled();
		expect(api.getLocation()).toEqual({
			pathname: "/replace-target",
			search: "",
			hash: "",
			state: {
				source: "replace-test",
			},
		});
	});

	it("applies explicit search/hash overrides on programmatic navigation", async () => {
		const api = await loadPublicClientAPI();
		vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				loadersData: [],
			}),
		);

		await api.vormaNavigate("/search-hash-target?tab=settings#security");
		await vi.runAllTimersAsync();

		expect(api.getLocation()).toEqual({
			pathname: "/search-hash-target",
			search: "?tab=settings",
			hash: "#security",
			state: null,
		});
	});

	it("pushes history for user navigation to a different URL", async () => {
		window.history.replaceState({}, "", "/history-start");
		const api = await loadPublicClientAPI();
		vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				loadersData: [],
			}),
		);

		const history = api.getHistoryInstance();
		const pushSpy = vi.spyOn(history, "push");
		const replaceSpy = vi.spyOn(history, "replace");

		await api.vormaNavigate("/history-destination");
		await vi.runAllTimersAsync();

		expect(pushSpy).toHaveBeenCalledWith(
			expect.stringContaining("/history-destination"),
			undefined,
		);
		expect(replaceSpy).not.toHaveBeenCalled();
	});

	it("does not mutate history when navigating to the current URL", async () => {
		window.history.replaceState({}, "", "/history-same");
		const api = await loadPublicClientAPI();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValue(createRouteDataResponse());

		const history = api.getHistoryInstance();
		const pushSpy = vi.spyOn(history, "push");
		const replaceSpy = vi.spyOn(history, "replace");

		await api.vormaNavigate("/history-same");
		await vi.runAllTimersAsync();

		expect(fetchSpy).not.toHaveBeenCalled();
		expect(replaceSpy).not.toHaveBeenCalled();
		expect(pushSpy).not.toHaveBeenCalled();
	});

	it("throws and avoids fetch when programmatic navigate target is cross-origin", async () => {
		const api = await loadPublicClientAPI();
		const fetchSpy = vi.spyOn(window, "fetch");

		await expect(
			api.vormaNavigate("https://external.example/some-path"),
		).rejects.toThrow(
			"vormaNavigate(...) only supports same-origin targets.",
		);
		expect(fetchSpy).not.toHaveBeenCalled();
	});

	it("allows relative and same-origin absolute targets for programmatic client APIs", async () => {
		const api = await loadPublicClientAPI();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse({
					loadersData: [],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					loadersData: [],
				}),
			)
			.mockResolvedValueOnce(
				new Response(JSON.stringify({ ok: true }), {
					status: 200,
					headers: {
						"Content-Type": "application/json",
					},
				}),
			);

		await api.vormaNavigate("/same-origin-relative-target");
		await vi.runAllTimersAsync();
		await api.vormaNavigate(
			`${window.location.origin}/same-origin-absolute-target`,
		);
		await vi.runAllTimersAsync();
		await api.submit(
			`${window.location.origin}/api/same-origin-submit`,
			{
				method: "POST",
			},
			{
				revalidate: false,
			},
		);
		await vi.runAllTimersAsync();

		expect(fetchSpy).toHaveBeenCalledTimes(3);
		expect(
			requestInputToURL(fetchSpy.mock.calls[0]?.[0] as RequestInfo | URL)
				.pathname,
		).toBe("/same-origin-relative-target");
		expect(
			requestInputToURL(fetchSpy.mock.calls[1]?.[0] as RequestInfo | URL)
				.pathname,
		).toBe("/same-origin-absolute-target");
		expect(
			requestInputToURL(fetchSpy.mock.calls[2]?.[0] as RequestInfo | URL)
				.pathname,
		).toBe("/api/same-origin-submit");
	});

	it("throws and avoids fetch when submit target is cross-origin", async () => {
		const api = await loadPublicClientAPI();
		const fetchSpy = vi.spyOn(window, "fetch");

		await expect(
			api.submit("https://external.example/api", {
				method: "POST",
			}),
		).rejects.toThrow("submit(...) only supports same-origin targets.");
		expect(fetchSpy).not.toHaveBeenCalled();
	});

	it("does not emit duplicate idle status snapshots for same-document no-op navigation", async () => {
		const api = await loadPublicClientAPI();
		const { statuses, cleanup } = collectStatusSnapshots(api);
		try {
			vi.spyOn(window, "fetch").mockResolvedValue(
				createRouteDataResponse({
					loadersData: [],
				}),
			);

			await api.vormaNavigate("/status-dedupe");
			await vi.runAllTimersAsync();
			const eventCountAfterCommit = statuses.length;

			await api.vormaNavigate("/status-dedupe");
			await vi.runAllTimersAsync();

			expect(statuses).toHaveLength(eventCountAfterCommit);
			expect(statuses.at(-1)).toEqual({
				isNavigating: false,
				isSubmitting: false,
				isRevalidating: false,
			});
		} finally {
			cleanup();
		}
	});

	it("commits same-document hash-only programmatic navigation without fetching route data", async () => {
		const api = await loadPublicClientAPI();
		window.history.replaceState({}, "", "/hash-base");
		const fetchSpy = vi.spyOn(window, "fetch");

		await api.vormaNavigate("/hash-base#details");
		await vi.runAllTimersAsync();

		expect(fetchSpy).not.toHaveBeenCalled();
		expect(window.location.pathname).toBe("/hash-base");
		expect(window.location.hash).toBe("#details");
		expectStatusIdle(api.getStatus());
	});

	it("preserves metadata across programmatic hash-only navigations", async () => {
		const api = await loadPublicClientAPI();
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValueOnce(
			createRouteDataResponse({
				matchedPatterns: [],
				loadersData: [],
				title: { dangerousInnerHTML: "Metadata Parity Title" },
				metaHeadEls: [
					{
						tag: "meta",
						attributesKnownSafe: {
							name: "description",
							content: "Metadata parity description",
						},
						booleanAttributes: [],
					},
				],
			}),
		);

		await api.vormaNavigate("/metadata-parity");
		await vi.runAllTimersAsync();
		expect(document.title).toBe("Metadata Parity Title");
		expect(
			document.head
				.querySelector('meta[name="description"]')
				?.getAttribute("content"),
		).toBe("Metadata parity description");

		await api.vormaNavigate("/metadata-parity#details");
		await vi.runAllTimersAsync();
		expect(fetchSpy).toHaveBeenCalledTimes(1);
		expect(document.title).toBe("Metadata Parity Title");
		expect(
			document.head
				.querySelector('meta[name="description"]')
				?.getAttribute("content"),
		).toBe("Metadata parity description");
	});

	it("does not mutate history when hash target is encoding-equivalent", async () => {
		window.history.replaceState({}, "", "/history-same-hash#~");
		const api = await loadPublicClientAPI();
		const history = api.getHistoryInstance();
		const pushSpy = vi.spyOn(history, "push");
		const replaceSpy = vi.spyOn(history, "replace");
		vi.spyOn(window, "fetch").mockResolvedValue(createRouteDataResponse());

		await api.vormaNavigate("/history-same-hash#%7E");
		await vi.runAllTimersAsync();

		expect(window.location.hash).toBe("#~");
		expect(replaceSpy).not.toHaveBeenCalled();
		expect(pushSpy).not.toHaveBeenCalled();
	});

	it("pushes history when hash target changes", async () => {
		window.history.replaceState({}, "", "/history-hash-change#first");
		const api = await loadPublicClientAPI();
		const history = api.getHistoryInstance();
		const pushSpy = vi.spyOn(history, "push");
		const replaceSpy = vi.spyOn(history, "replace");
		vi.spyOn(window, "fetch").mockResolvedValue(createRouteDataResponse());

		await api.vormaNavigate("/history-hash-change#second");
		await vi.runAllTimersAsync();

		expect(pushSpy).toHaveBeenCalledWith(
			expect.stringContaining("/history-hash-change#second"),
			undefined,
		);
		expect(replaceSpy).not.toHaveBeenCalled();
	});

	it("reports revalidation status while in flight and clears on completion", async () => {
		const api = await loadPublicClientAPI();
		const deferred = createDeferred<Response>();
		vi.spyOn(window, "fetch").mockImplementation(() => deferred.promise);

		const revalidatePromise = api.revalidate();
		await Promise.resolve();

		expect(api.getStatus()).toEqual({
			isNavigating: false,
			isSubmitting: false,
			isRevalidating: true,
		});

		deferred.resolve(
			createRouteDataResponse({
				matchedPatterns: [],
				importURLs: [],
				exportKeys: [],
				errorExportKeys: [],
				loadersData: [],
				title: { dangerousInnerHTML: "Revalidated" },
			}),
		);

		await revalidatePromise;
		await vi.runAllTimersAsync();

		expect(document.title).toBe("Revalidated");
		expectStatusIdle(api.getStatus());
	});

	it("aborts an in-flight user navigation when a new target is requested", async () => {
		const api = await loadPublicClientAPI();
		const firstFetchCall = createDeferredFetchCall();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockImplementationOnce(firstFetchCall.mock)
			.mockImplementationOnce(() =>
				Promise.resolve(
					createRouteDataResponse({
						loadersData: [],
					}),
				),
			);

		const firstNavigation = api.vormaNavigate("/state-first");
		await Promise.resolve();
		expect(api.getStatus().isNavigating).toBe(true);
		expect(firstFetchCall.getSignal()).toBeDefined();

		const secondNavigation = api.vormaNavigate("/state-second");
		expect(firstFetchCall.getSignal()?.aborted).toBe(true);

		firstFetchCall.deferred.reject(
			new DOMException("Aborted", "AbortError"),
		);

		await Promise.all([firstNavigation, secondNavigation]);
		await vi.runAllTimersAsync();

		expect(fetchSpy).toHaveBeenCalledTimes(2);
		expect(window.location.pathname).toBe("/state-second");
		expectStatusIdle(api.getStatus());
	});

	it("runs speculative client-loader code for superseded navigations but discards stale commits", async () => {
		const api = await loadPublicClientAPI();
		const speculativeSideEffects: string[] = [];
		const abortStatesAtFailure: boolean[] = [];
		registerClientLoaderForTesting({
			pattern: "/stale-client-loader",
			clientLoader: async (input: unknown) => {
				speculativeSideEffects.push("executed");
				const clientLoaderInput = input as {
					serverDataPromise: Promise<unknown>;
					signal: AbortSignal;
				};
				try {
					await clientLoaderInput.serverDataPromise;
				} catch {
					abortStatesAtFailure.push(clientLoaderInput.signal.aborted);
				}
				return {
					stale: true,
				};
			},
		});
		const staleFetchCall = createDeferredFetchCall();
		const freshFetchCall = createDeferredFetchCall();
		vi.spyOn(window, "fetch")
			.mockImplementationOnce(staleFetchCall.mock)
			.mockImplementationOnce(freshFetchCall.mock);

		const staleNavigationPromise = api.vormaNavigate(
			"/stale-client-loader",
		);
		await Promise.resolve();
		const freshNavigationPromise = api.vormaNavigate("/fresh-target");
		await Promise.resolve();

		expect(staleFetchCall.getSignal()?.aborted).toBe(true);

		freshFetchCall.deferred.resolve(
			createRouteDataResponse({
				title: {
					dangerousInnerHTML: "Fresh Target",
				},
			}),
		);
		staleFetchCall.deferred.reject(
			new DOMException("Aborted", "AbortError"),
		);

		await Promise.all([staleNavigationPromise, freshNavigationPromise]);
		await vi.runAllTimersAsync();

		expect(speculativeSideEffects).toEqual(["executed"]);
		expect(abortStatesAtFailure).toEqual([true]);
		expect(window.location.pathname).toBe("/fresh-target");
		expect(document.title).toBe("Fresh Target");
		expectStatusIdle(api.getStatus());
	});

	it("clearAll aborts in-flight navigation and submit work, then returns idle", async () => {
		const api = await loadPublicClientAPI();
		const { requests } = createAbortAwareFetchRecorder();

		const { result, unhandledRejections } =
			await withUnhandledRejectionCapture({
				run: async () => {
					const navigatePromise = api.vormaNavigate("/clear-all-nav");
					const submitPromise = api.submit(
						"/api/clear-all-submit",
						{ method: "POST" },
						{ revalidate: false },
					);

					await waitForRequestCount({ requests, count: 2 });

					expect(api.getStatus()).toEqual({
						isNavigating: true,
						isSubmitting: true,
						isRevalidating: false,
					});

					clearAllNavigationStateForTesting();

					const submitResult = await submitPromise;
					await navigatePromise;
					await vi.runAllTimersAsync();

					return { submitResult };
				},
			});

		expect(result.submitResult).toEqual({
			success: false,
			error: "Aborted",
		});
		expect(unhandledRejections).toEqual([]);
		expect(window.location.pathname).toBe("/");
		expectStatusIdle(api.getStatus());
	});

	it("clearAll prevents late side effects from navigations whose fetch ignores abort", async () => {
		const api = await loadPublicClientAPI();
		const fetchCall = createDeferredFetchCall();
		const requestAnimationFrameSpy = vi.spyOn(
			window,
			"requestAnimationFrame",
		);
		vi.spyOn(window, "fetch").mockImplementation(fetchCall.mock);

		const navigatePromise = api.vormaNavigate("/clear-all-late-success");
		await vi.advanceTimersByTimeAsync(8);
		expect(fetchCall.getSignal()?.aborted).toBe(false);

		clearAllNavigationStateForTesting();
		expect(fetchCall.getSignal()?.aborted).toBe(true);

		const requestAnimationFrameCallCountBeforeResolve =
			requestAnimationFrameSpy.mock.calls.length;
		fetchCall.deferred.resolve(
			createRouteDataResponse({
				title: { dangerousInnerHTML: "Late Stale Title" },
				cssBundles: ["/late-stale.css"],
			}),
		);

		await navigatePromise;
		await vi.advanceTimersByTimeAsync(32);
		await vi.runAllTimersAsync();

		expect(window.location.pathname).toBe("/");
		expect(document.title).toBe("Initial Title");
		expect(requestAnimationFrameSpy).toHaveBeenCalledTimes(
			requestAnimationFrameCallCountBeforeResolve,
		);
		expect(
			document.head.querySelector(
				'link[data-vorma-css-bundle="/late-stale.css"]',
			),
		).toBeNull();
		expectStatusIdle(api.getStatus());
	});

	it("remains operable after clearAll by allowing fresh navigation to complete", async () => {
		const api = await loadPublicClientAPI();
		const firstFetchCall = createDeferredFetchCall();
		vi.spyOn(window, "fetch")
			.mockImplementationOnce(firstFetchCall.mock)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					title: { dangerousInnerHTML: "Post-Clear Navigation" },
				}),
			);

		const staleNavigation = api.vormaNavigate("/clear-all-stale");
		await vi.advanceTimersByTimeAsync(8);

		clearAllNavigationStateForTesting();
		expect(firstFetchCall.getSignal()?.aborted).toBe(true);

		firstFetchCall.deferred.resolve(
			createRouteDataResponse({
				title: { dangerousInnerHTML: "Should Not Apply" },
			}),
		);
		await staleNavigation;
		await vi.runAllTimersAsync();

		await api.vormaNavigate("/clear-all-fresh");
		await vi.runAllTimersAsync();

		expect(window.location.pathname).toBe("/clear-all-fresh");
		expect(document.title).toBe("Post-Clear Navigation");
		expectStatusIdle(api.getStatus());
	});

	it("can report navigating and revalidating simultaneously", async () => {
		const api = await loadPublicClientAPI();
		const navigationDeferred = createDeferred<Response>();
		const revalidationDeferred = createDeferred<Response>();
		vi.spyOn(window, "fetch")
			.mockImplementationOnce(() => navigationDeferred.promise)
			.mockImplementationOnce(() => revalidationDeferred.promise);

		const navigatePromise = api.vormaNavigate("/dual-lane");
		await Promise.resolve();
		const revalidatePromise = api.revalidate();
		await Promise.resolve();

		expect(api.getStatus()).toEqual({
			isNavigating: true,
			isSubmitting: false,
			isRevalidating: true,
		});

		navigationDeferred.resolve(
			createRouteDataResponse({
				loadersData: [],
			}),
		);
		revalidationDeferred.resolve(
			createRouteDataResponse({
				loadersData: [],
			}),
		);

		await Promise.all([navigatePromise, revalidatePromise]);
		await vi.runAllTimersAsync();

		expectStatusIdle(api.getStatus());
	});

	it("keeps in-flight revalidation status during same-document no-op navigations", async () => {
		const api = await loadPublicClientAPI();
		window.history.replaceState({}, "", "/noop-while-revalidating");
		const deferred = createDeferred<Response>();
		vi.spyOn(window, "fetch").mockImplementation(() => deferred.promise);

		const revalidatePromise = api.revalidate();
		await Promise.resolve();
		await api.vormaNavigate("/noop-while-revalidating");
		await vi.runAllTimersAsync();

		expect(api.getStatus()).toEqual({
			isNavigating: false,
			isSubmitting: false,
			isRevalidating: true,
		});

		deferred.resolve(
			createRouteDataResponse({
				loadersData: [],
			}),
		);
		await revalidatePromise;
		await vi.runAllTimersAsync();

		expectStatusIdle(api.getStatus());
	});

	it("coalesces rapid same-target revalidate calls into one fetch", async () => {
		const api = await loadPublicClientAPI();
		let resolveFetch: ((value: Response) => void) | undefined;
		const fetchSpy = vi.spyOn(window, "fetch").mockImplementation(
			() =>
				new Promise<Response>((resolve) => {
					resolveFetch = resolve;
				}),
		);

		const first = api.revalidate();
		const second = api.revalidate();
		const third = api.revalidate();

		expect(fetchSpy).toHaveBeenCalledTimes(1);

		resolveFetch?.(
			createRouteDataResponse({
				loadersData: [],
			}),
		);
		await Promise.all([first, second, third]);
		await vi.runAllTimersAsync();

		expectStatusIdle(api.getStatus());
	});

	it("keeps one revalidation in-flight and runs at most one trailing pass for rapid repeated requests", async () => {
		const api = await loadPublicClientAPI();
		const firstRevalidationFetch = createDeferred<Response>();
		const trailingRevalidationFetch = createDeferred<Response>();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockImplementationOnce(() => firstRevalidationFetch.promise)
			.mockImplementationOnce(() => trailingRevalidationFetch.promise);

		const first = api.revalidate();
		await vi.advanceTimersByTimeAsync(8);

		const second = api.revalidate();
		const third = api.revalidate();
		await vi.advanceTimersByTimeAsync(16);

		expect(fetchSpy).toHaveBeenCalledTimes(1);

		firstRevalidationFetch.resolve(
			createRouteDataResponse({
				loadersData: [],
				title: { dangerousInnerHTML: "First Revalidation" },
			}),
		);
		await first;
		await vi.advanceTimersByTimeAsync(8);

		expect(fetchSpy).toHaveBeenCalledTimes(2);

		trailingRevalidationFetch.resolve(
			createRouteDataResponse({
				loadersData: [],
				title: { dangerousInnerHTML: "Trailing Revalidation" },
			}),
		);
		await Promise.all([second, third]);
		await vi.runAllTimersAsync();

		expect(document.title).toBe("Trailing Revalidation");
		expectStatusIdle(api.getStatus());
	});

	it("does not coalesce revalidation across data-target changes", async () => {
		const api = await loadPublicClientAPI();
		window.history.replaceState({}, "", "/revalidate-a");
		const firstFetchCall = createDeferredFetchCall();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockImplementationOnce((_input, init) => {
				const promise = firstFetchCall.mock(_input, init);
				firstFetchCall
					.getSignal()
					?.addEventListener(
						"abort",
						() =>
							firstFetchCall.deferred.reject(
								new DOMException("Aborted", "AbortError"),
							),
						{ once: true },
					);
				return promise;
			})
			.mockImplementationOnce(() =>
				Promise.resolve(
					createRouteDataResponse({
						loadersData: [],
					}),
				),
			);

		const firstRevalidate = api.revalidate();
		await vi.advanceTimersByTimeAsync(8);

		window.history.replaceState({}, "", "/revalidate-b");
		const secondRevalidate = api.revalidate();

		expect(firstFetchCall.getSignal()?.aborted).toBe(true);

		await Promise.all([firstRevalidate, secondRevalidate]);
		await vi.runAllTimersAsync();

		expect(fetchSpy).toHaveBeenCalledTimes(2);
		const secondFetchURL = requestInputToURL(
			fetchSpy.mock.calls[1]?.[0] as RequestInfo | URL,
		);
		expect(secondFetchURL.pathname).toBe("/revalidate-b");
		expectStatusIdle(api.getStatus());
	});

	it("does not mutate history during revalidation", async () => {
		const api = await loadPublicClientAPI();
		window.history.replaceState({}, "", "/revalidate-history?x=1");
		const pushSpy = vi.spyOn(window.history, "pushState");
		const replaceSpy = vi.spyOn(window.history, "replaceState");
		vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				loadersData: [],
			}),
		);

		await api.revalidate();
		await vi.runAllTimersAsync();

		expect(pushSpy).not.toHaveBeenCalled();
		expect(replaceSpy).not.toHaveBeenCalled();
		expect(window.location.pathname).toBe("/revalidate-history");
		expect(window.location.search).toBe("?x=1");
	});

	it("revalidates against the current url including search params", async () => {
		const api = await loadPublicClientAPI();
		window.history.replaceState({}, "", "/revalidate-query?foo=bar&n=1");
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				loadersData: [],
			}),
		);

		await api.revalidate();
		await vi.runAllTimersAsync();

		expect(fetchSpy).toHaveBeenCalledTimes(1);
		const revalidateURL = requestInputToURL(
			fetchSpy.mock.calls[0]?.[0] as RequestInfo | URL,
		);
		expect(revalidateURL.pathname).toBe("/revalidate-query");
		expect(revalidateURL.searchParams.get("foo")).toBe("bar");
		expect(revalidateURL.searchParams.get("n")).toBe("1");
	});

	it("includes deployment query param on revalidation when deployment id is configured", async () => {
		const api = await loadPublicClientAPI();
		await api.initClient({});
		setDeploymentIDForTesting("deploy-abc");
		window.history.replaceState({}, "", "/revalidate-target?tab=details");
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				loadersData: [],
			}),
		);

		await api.revalidate();
		await vi.runAllTimersAsync();

		const revalidateURL = requestInputToURL(
			fetchSpy.mock.calls[0]?.[0] as RequestInfo | URL,
		);
		expect(revalidateURL.pathname).toBe("/revalidate-target");
		expect(revalidateURL.searchParams.get("tab")).toBe("details");
		expect(revalidateURL.searchParams.get("vorma_json")).toBe("1");
		expect(revalidateURL.searchParams.get("dpl")).toBe("deploy-abc");
	});

	it("follows redirects returned from revalidation when still relevant", async () => {
		const api = await loadPublicClientAPI();
		window.history.replaceState({}, "", "/revalidate-start");
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				new Response("", {
					status: 302,
					headers: {
						"X-Client-Redirect": "/revalidate-target",
						"X-Wave-Framework-Build-Id": "2",
					},
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					loadersData: [],
				}),
			);

		await api.revalidate();
		await vi.runAllTimersAsync();

		expect(fetchSpy).toHaveBeenCalledTimes(2);
		expect(window.location.pathname).toBe("/revalidate-target");
		expectStatusIdle(api.getStatus());
	});

	it("does not follow stale revalidation redirects after target ownership changes", async () => {
		const api = await loadPublicClientAPI();
		window.history.replaceState({}, "", "/revalidate-stale-a");
		const firstDeferred = createDeferred<Response>();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockImplementationOnce(() => firstDeferred.promise)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					loadersData: [],
					title: {
						dangerousInnerHTML: "Revalidate Stale Winner",
					},
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					loadersData: [],
				}),
			);

		const firstRevalidate = api.revalidate();
		await vi.advanceTimersByTimeAsync(8);

		window.history.replaceState({}, "", "/revalidate-stale-b");
		const secondRevalidate = api.revalidate();
		await secondRevalidate;

		firstDeferred.resolve(
			new Response("", {
				status: 200,
				headers: {
					"X-Client-Redirect": "/revalidate-stale-redirect",
				},
			}),
		);
		await firstRevalidate;
		await vi.runAllTimersAsync();

		expect(window.location.pathname).toBe("/revalidate-stale-b");
		expect(document.title).toBe("Revalidate Stale Winner");
		expect(fetchSpy).toHaveBeenCalledTimes(2);
		expectStatusIdle(api.getStatus());
	});

	it("does not apply stale revalidation hard-reload or build-id side effects after ownership changes", async () => {
		const api = await loadPublicClientAPI();
		const observedBuildIDs: string[] = [];
		const cleanupBuildIDListener = api.addBuildIDListener((event) => {
			observedBuildIDs.push(event.detail.newID);
		});
		let locationHrefStub: ReturnType<typeof stubWindowLocationHref> | null =
			null;
		try {
			window.history.replaceState({}, "", "/revalidate-stale-hard-a");
			const firstDeferred = createDeferred<Response>();
			const fetchSpy = vi
				.spyOn(window, "fetch")
				.mockImplementationOnce(() => firstDeferred.promise)
				.mockResolvedValueOnce(
					createRouteDataResponse(
						{
							loadersData: [],
							title: {
								dangerousInnerHTML: "Revalidate Hard Winner",
							},
						},
						{
							headers: {
								"X-Wave-Framework-Build-Id": "winner-build-id",
							},
						},
					),
				);

			const firstRevalidate = api.revalidate();
			await vi.advanceTimersByTimeAsync(8);

			window.history.replaceState({}, "", "/revalidate-stale-hard-b");
			const secondRevalidate = api.revalidate();
			await secondRevalidate;
			locationHrefStub = stubWindowLocationHref();

			firstDeferred.resolve(
				new Response("", {
					status: 200,
					headers: {
						"X-Wave-Framework-Reload":
							"/revalidate-stale-hard-reload",
						"X-Wave-Framework-Build-Id": "stale-build-id",
					},
				}),
			);
			await firstRevalidate;
			await vi.runAllTimersAsync();

			expect(fetchSpy).toHaveBeenCalledTimes(2);
			expect(window.location.pathname).toBe("/revalidate-stale-hard-b");
			expect(locationHrefStub.getHref()).not.toContain(
				"/revalidate-stale-hard-reload",
			);
			expect(api.getBuildID()).toBe("winner-build-id");
			expect(observedBuildIDs).toEqual(["winner-build-id"]);
			expectStatusIdle(api.getStatus());
		} finally {
			cleanupBuildIDListener();
			locationHrefStub?.restore();
		}
	});

	it("ignores stale revalidation side effects after external location change", async () => {
		const api = await loadPublicClientAPI();
		window.history.replaceState({}, "", "/revalidate-external-a");
		const deferred = createDeferred<Response>();
		vi.spyOn(window, "fetch").mockImplementation(() => deferred.promise);

		const revalidatePromise = api.revalidate();
		await vi.advanceTimersByTimeAsync(8);

		window.history.pushState({}, "", "/revalidate-external-b");
		deferred.resolve(
			createRouteDataResponse({
				loadersData: [],
				title: {
					dangerousInnerHTML: "Stale External Revalidate",
				},
			}),
		);
		await revalidatePromise;
		await vi.runAllTimersAsync();

		expect(window.location.pathname).toBe("/revalidate-external-b");
		expect(document.title).toBe("Initial Title");
		expectStatusIdle(api.getStatus());
	});

	it("does not let stale revalidation results override later navigation state or css", async () => {
		const api = await loadPublicClientAPI();
		window.history.replaceState({}, "", "/");
		const revalidationDeferred = createDeferred<Response>();
		vi.spyOn(window, "fetch").mockImplementation(
			(input: RequestInfo | URL) => {
				const requestURL = requestInputToURL(input);
				if (requestURL.pathname === "/about") {
					return Promise.resolve(
						createRouteDataResponse({
							loadersData: [],
							title: { dangerousInnerHTML: "About Page" },
						}),
					);
				}
				return revalidationDeferred.promise;
			},
		);

		const revalidatePromise = api.revalidate();
		await vi.advanceTimersByTimeAsync(8);

		await api.vormaNavigate("/about");
		await vi.runAllTimersAsync();
		expect(window.location.pathname).toBe("/about");
		expect(document.title).toBe("About Page");

		revalidationDeferred.resolve(
			createRouteDataResponse({
				loadersData: [],
				title: { dangerousInnerHTML: "Stale Revalidation Title" },
				cssBundles: ["/stale-revalidation.css"],
			}),
		);
		await revalidatePromise;
		await vi.advanceTimersByTimeAsync(32);
		await vi.runAllTimersAsync();

		expect(window.location.pathname).toBe("/about");
		expect(document.title).toBe("About Page");
		expect(
			document.head.querySelector(
				'link[data-vorma-css-bundle="/stale-revalidation.css"]',
			),
		).toBeNull();
		expectStatusIdle(api.getStatus());
	});

	it("does not update build id from stale revalidation responses after external location change", async () => {
		const api = await loadPublicClientAPI();
		window.history.replaceState({}, "", "/revalidate-build-id-a");
		const deferred = createDeferred<Response>();
		vi.spyOn(window, "fetch").mockImplementation(() => deferred.promise);

		const revalidatePromise = api.revalidate();
		await vi.advanceTimersByTimeAsync(8);

		window.history.pushState({}, "", "/revalidate-build-id-b");
		deferred.resolve(
			createRouteDataResponse(
				{
					loadersData: [],
				},
				{
					headers: {
						"X-Wave-Framework-Build-Id":
							"stale-revalidate-build-id",
					},
				},
			),
		);
		await revalidatePromise;
		await vi.runAllTimersAsync();

		expect(window.location.pathname).toBe("/revalidate-build-id-b");
		expect(api.getBuildID()).toBe("1");
		expectStatusIdle(api.getStatus());
	});

	it("does not perform hard reload from stale revalidation responses after external location change", async () => {
		const api = await loadPublicClientAPI();
		window.history.replaceState({}, "", "/revalidate-hard-reload-a");
		const deferred = createDeferred<Response>();
		vi.spyOn(window, "fetch").mockImplementation(() => deferred.promise);
		let locationHrefStub: ReturnType<typeof stubWindowLocationHref> | null =
			null;
		try {
			const revalidatePromise = api.revalidate();
			await vi.advanceTimersByTimeAsync(8);

			window.history.pushState({}, "", "/revalidate-hard-reload-b");
			locationHrefStub = stubWindowLocationHref();
			deferred.resolve(
				new Response("", {
					status: 200,
					headers: {
						"X-Wave-Framework-Reload":
							"/revalidate-stale-hard-reload",
						"X-Wave-Framework-Build-Id":
							"stale-hard-reload-build-id",
					},
				}),
			);
			await revalidatePromise;
			await vi.runAllTimersAsync();

			expect(locationHrefStub.getHref()).not.toContain(
				"/revalidate-stale-hard-reload",
			);
			expect(window.location.pathname).toBe("/revalidate-hard-reload-b");
			expect(api.getBuildID()).toBe("1");
			expectStatusIdle(api.getStatus());
		} finally {
			locationHrefStub?.restore();
		}
	});

	it("does not follow stale revalidation redirects after external location change", async () => {
		const api = await loadPublicClientAPI();
		window.history.replaceState({}, "", "/revalidate-external-redirect-a");
		const deferred = createDeferred<Response>();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockImplementationOnce(() => deferred.promise)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					loadersData: [],
				}),
			);

		const revalidatePromise = api.revalidate();
		await vi.advanceTimersByTimeAsync(8);

		window.history.pushState({}, "", "/revalidate-external-redirect-b");
		deferred.resolve(
			new Response("", {
				status: 200,
				headers: {
					"X-Client-Redirect": "/revalidate-external-redirect-stale",
				},
			}),
		);
		await revalidatePromise;
		await vi.runAllTimersAsync();

		expect(fetchSpy).toHaveBeenCalledTimes(1);
		expect(window.location.pathname).toBe(
			"/revalidate-external-redirect-b",
		);
		expectStatusIdle(api.getStatus());
	});

	it("does not follow stale native revalidation redirects after external location change", async () => {
		const api = await loadPublicClientAPI();
		window.history.replaceState(
			{},
			"",
			"/revalidate-external-native-redirect-a",
		);
		const deferred = createDeferred<Response>();
		const nativeRedirectResponse = createRouteDataResponse();
		Object.defineProperty(nativeRedirectResponse, "redirected", {
			value: true,
			configurable: true,
		});
		Object.defineProperty(nativeRedirectResponse, "url", {
			value: `${window.location.origin}/revalidate-external-native-redirect-stale`,
			configurable: true,
		});
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockImplementationOnce(() => deferred.promise);

		const revalidatePromise = api.revalidate();
		await vi.advanceTimersByTimeAsync(8);

		window.history.pushState(
			{},
			"",
			"/revalidate-external-native-redirect-b",
		);
		deferred.resolve(nativeRedirectResponse);
		await revalidatePromise;
		await vi.runAllTimersAsync();

		expect(fetchSpy).toHaveBeenCalledTimes(1);
		expect(window.location.pathname).toBe(
			"/revalidate-external-native-redirect-b",
		);
		expectStatusIdle(api.getStatus());
	});

	it("applies in-flight revalidation results across hash-only location changes", async () => {
		const api = await loadPublicClientAPI();
		window.history.replaceState({}, "", "/revalidate-hash-apply#initial");
		const deferred = createDeferred<Response>();
		vi.spyOn(window, "fetch").mockImplementation(() => deferred.promise);

		const revalidatePromise = api.revalidate();
		await vi.advanceTimersByTimeAsync(8);

		window.history.replaceState({}, "", "/revalidate-hash-apply#next");
		deferred.resolve(
			createRouteDataResponse({
				loadersData: [],
				title: {
					dangerousInnerHTML: "Revalidate Hash Applied",
				},
			}),
		);
		await revalidatePromise;
		await vi.runAllTimersAsync();

		expect(window.location.pathname).toBe("/revalidate-hash-apply");
		expect(window.location.hash).toBe("#next");
		expect(document.title).toBe("Revalidate Hash Applied");
		expectStatusIdle(api.getStatus());
	});

	it("treats search-param location changes as stale revalidation boundaries", async () => {
		const api = await loadPublicClientAPI();
		window.history.replaceState({}, "", "/revalidate-search-boundary?x=1");
		const deferred = createDeferred<Response>();
		vi.spyOn(window, "fetch").mockImplementation(() => deferred.promise);

		const revalidatePromise = api.revalidate();
		await vi.advanceTimersByTimeAsync(8);

		window.history.pushState({}, "", "/revalidate-search-boundary?x=2");
		deferred.resolve(
			createRouteDataResponse({
				loadersData: [],
				title: {
					dangerousInnerHTML: "Stale Search Revalidate",
				},
			}),
		);
		await revalidatePromise;
		await vi.runAllTimersAsync();

		expect(window.location.pathname).toBe("/revalidate-search-boundary");
		expect(window.location.search).toBe("?x=2");
		expect(document.title).toBe("Initial Title");
		expectStatusIdle(api.getStatus());
	});

	it("revalidates on focus only after staleTime has elapsed", async () => {
		const api = await loadPublicClientAPI();
		void api.getStatus();
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				loadersData: [],
			}),
		);
		const cleanup = api.revalidateOnWindowFocus({
			staleTimeMS: 1000,
		});
		try {
			window.dispatchEvent(new Event("focus"));
			await Promise.resolve();
			await vi.runAllTimersAsync();
			expect(fetchSpy).toHaveBeenCalledTimes(0);

			await vi.advanceTimersByTimeAsync(1001);
			window.dispatchEvent(new Event("focus"));
			await Promise.resolve();
			await vi.runAllTimersAsync();

			expect(fetchSpy).toHaveBeenCalledTimes(1);
		} finally {
			cleanup();
		}
	});

	it("resets focus stale-time window after successful navigation", async () => {
		const api = await loadPublicClientAPI();
		const fetchSpy = vi.spyOn(window, "fetch").mockImplementation(() =>
			Promise.resolve(
				createRouteDataResponse({
					loadersData: [],
				}),
			),
		);

		await vi.advanceTimersByTimeAsync(10);
		await api.vormaNavigate("/focus-stale-gate");
		await vi.runAllTimersAsync();

		const cleanup = api.revalidateOnWindowFocus({ staleTimeMS: 100 });
		try {
			window.dispatchEvent(new Event("focus"));
			await Promise.resolve();
			await vi.runAllTimersAsync();
			expect(fetchSpy).toHaveBeenCalledTimes(1);

			await vi.advanceTimersByTimeAsync(101);
			window.dispatchEvent(new Event("focus"));
			await Promise.resolve();
			await vi.runAllTimersAsync();
			expect(fetchSpy).toHaveBeenCalledTimes(2);
		} finally {
			cleanup();
		}
	});

	it("does not reset focus stale-time window for hash-only programmatic navigations", async () => {
		const api = await loadPublicClientAPI();
		void api.getStatus();
		const fetchSpy = vi.spyOn(window, "fetch").mockImplementation(() =>
			Promise.resolve(
				createRouteDataResponse({
					loadersData: [],
				}),
			),
		);
		const cleanup = api.revalidateOnWindowFocus({ staleTimeMS: 100 });
		try {
			await vi.advanceTimersByTimeAsync(90);
			await api.vormaNavigate("/#focus-hash-no-reset");
			await vi.runAllTimersAsync();

			await vi.advanceTimersByTimeAsync(20);
			window.dispatchEvent(new Event("focus"));
			await Promise.resolve();
			await vi.runAllTimersAsync();

			expect(fetchSpy).toHaveBeenCalledTimes(1);
		} finally {
			cleanup();
		}
	});

	it("does not advance focus stale-time timestamp for aborted navigations", async () => {
		const api = await loadPublicClientAPI();
		void api.getStatus();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockRejectedValueOnce(new DOMException("Aborted", "AbortError"))
			.mockResolvedValue(
				createRouteDataResponse({
					loadersData: [],
				}),
			);
		const cleanup = api.revalidateOnWindowFocus({ staleTimeMS: 100 });
		try {
			await vi.advanceTimersByTimeAsync(90);
			await expect(
				api.vormaNavigate("/focus-aborted-navigation"),
			).resolves.toEqual({
				didNavigate: false,
			});
			await vi.runAllTimersAsync();

			await vi.advanceTimersByTimeAsync(20);
			window.dispatchEvent(new Event("focus"));
			await Promise.resolve();
			await vi.runAllTimersAsync();

			expect(fetchSpy).toHaveBeenCalledTimes(2);
		} finally {
			cleanup();
		}
	});

	it("resets focus stale-time window after successful revalidation", async () => {
		const api = await loadPublicClientAPI();
		void api.getStatus();
		const fetchSpy = vi.spyOn(window, "fetch").mockImplementation(() =>
			Promise.resolve(
				createRouteDataResponse({
					loadersData: [],
				}),
			),
		);
		const cleanup = api.revalidateOnWindowFocus({ staleTimeMS: 100 });
		try {
			await vi.advanceTimersByTimeAsync(101);
			window.dispatchEvent(new Event("focus"));
			await Promise.resolve();
			await vi.runAllTimersAsync();
			expect(fetchSpy).toHaveBeenCalledTimes(1);

			window.dispatchEvent(new Event("focus"));
			await Promise.resolve();
			await vi.runAllTimersAsync();
			expect(fetchSpy).toHaveBeenCalledTimes(1);

			await vi.advanceTimersByTimeAsync(101);
			window.dispatchEvent(new Event("focus"));
			await Promise.resolve();
			await vi.runAllTimersAsync();
			expect(fetchSpy).toHaveBeenCalledTimes(2);
		} finally {
			cleanup();
		}
	});

	it("does not advance focus stale-time timestamp for aborted revalidations", async () => {
		const api = await loadPublicClientAPI();
		void api.getStatus();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockRejectedValueOnce(new DOMException("Aborted", "AbortError"))
			.mockResolvedValue(
				createRouteDataResponse({
					loadersData: [],
				}),
			);
		const cleanup = api.revalidateOnWindowFocus({ staleTimeMS: 100 });
		try {
			await vi.advanceTimersByTimeAsync(101);
			window.dispatchEvent(new Event("focus"));
			await Promise.resolve();
			await vi.runAllTimersAsync();
			expect(fetchSpy).toHaveBeenCalledTimes(1);

			window.dispatchEvent(new Event("focus"));
			await Promise.resolve();
			await vi.runAllTimersAsync();
			expect(fetchSpy).toHaveBeenCalledTimes(2);
		} finally {
			cleanup();
		}
	});

	it("does not revalidate on focus while a navigation is active", async () => {
		const api = await loadPublicClientAPI();
		const navigationDeferred = createDeferred<Response>();
		let fetchCallCount = 0;
		const fetchSpy = vi.spyOn(window, "fetch").mockImplementation(() => {
			fetchCallCount += 1;
			if (fetchCallCount === 1) {
				return navigationDeferred.promise;
			}
			return Promise.resolve(
				createRouteDataResponse({
					loadersData: [],
					title: { dangerousInnerHTML: "Focus Revalidate" },
				}),
			);
		});
		const cleanup = api.revalidateOnWindowFocus({
			staleTimeMS: 0,
		});
		try {
			const navigationPromise = api.vormaNavigate(
				"/focus-while-navigation",
			);
			await waitForRequestCount({
				requests: fetchSpy.mock.calls,
				count: 1,
			});

			window.dispatchEvent(new Event("focus"));
			await Promise.resolve();
			await vi.advanceTimersByTimeAsync(16);

			expect(fetchSpy).toHaveBeenCalledTimes(1);

			navigationDeferred.resolve(
				createRouteDataResponse({
					loadersData: [],
					title: { dangerousInnerHTML: "Navigation Complete" },
				}),
			);
			await navigationPromise;
			await vi.runAllTimersAsync();
			expectStatusIdle(api.getStatus());
		} finally {
			cleanup();
		}
	});

	it("does not revalidate on focus while a submission is active", async () => {
		const api = await loadPublicClientAPI();
		const submitDeferred = createDeferred<Response>();
		const fetchSpy = vi.spyOn(window, "fetch").mockImplementation(() => {
			if (fetchSpy.mock.calls.length <= 1) {
				return submitDeferred.promise;
			}
			return Promise.resolve(
				createRouteDataResponse({
					loadersData: [],
				}),
			);
		});
		const cleanup = api.revalidateOnWindowFocus({
			staleTimeMS: 0,
		});
		try {
			const submitPromise = api.submit<{ ok: boolean }>(
				"/focus-while-submit",
				{
					method: "POST",
					body: { ok: true } as any,
				},
				{
					revalidate: false,
				},
			);
			await waitForRequestCount({
				requests: fetchSpy.mock.calls,
				count: 1,
			});

			window.dispatchEvent(new Event("focus"));
			await Promise.resolve();
			await vi.advanceTimersByTimeAsync(16);
			expect(fetchSpy).toHaveBeenCalledTimes(1);

			submitDeferred.resolve(
				new Response(JSON.stringify({ ok: true }), {
					status: 200,
					headers: { "Content-Type": "application/json" },
				}),
			);
			await submitPromise;
			await vi.runAllTimersAsync();
		} finally {
			cleanup();
		}
	});

	it("does not revalidate on focus while a revalidation is already in flight", async () => {
		const api = await loadPublicClientAPI();
		const revalidationDeferred = createDeferred<Response>();
		const fetchSpy = vi.spyOn(window, "fetch").mockImplementation(() => {
			if (fetchSpy.mock.calls.length <= 1) {
				return revalidationDeferred.promise;
			}
			return Promise.resolve(
				createRouteDataResponse({
					loadersData: [],
				}),
			);
		});
		const cleanup = api.revalidateOnWindowFocus({
			staleTimeMS: 0,
		});
		try {
			const firstRevalidatePromise = api.revalidate();
			await waitForRequestCount({
				requests: fetchSpy.mock.calls,
				count: 1,
			});

			window.dispatchEvent(new Event("focus"));
			await Promise.resolve();
			await vi.advanceTimersByTimeAsync(16);
			expect(fetchSpy).toHaveBeenCalledTimes(1);

			revalidationDeferred.resolve(
				createRouteDataResponse({
					loadersData: [],
				}),
			);
			await firstRevalidatePromise;
			await vi.runAllTimersAsync();
		} finally {
			cleanup();
		}
	});

	it("stops revalidate-on-focus listener after cleanup", async () => {
		const api = await loadPublicClientAPI();
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				loadersData: [],
			}),
		);
		const cleanup = api.revalidateOnWindowFocus({
			staleTimeMS: 0,
		});
		cleanup();

		window.dispatchEvent(new Event("focus"));
		await Promise.resolve();
		await vi.runAllTimersAsync();

		expect(fetchSpy).not.toHaveBeenCalled();
	});

	it("builds query and mutation URLs from app config and route params", async () => {
		const api = await loadPublicClientAPI();
		const queryURL = api.buildQueryURL(TEST_VORMA_APP_CONFIG, {
			pattern: "/users/:id",
			params: {
				id: "a/b",
			},
			input: {
				tab: "activity",
				page: 2,
			},
		});
		const mutationURL = api.buildMutationURL(TEST_VORMA_APP_CONFIG, {
			pattern: "/users/:id",
			params: {
				id: "a/b",
			},
		});

		expect(queryURL.pathname).toBe("/api/users/a%2Fb");
		expect(queryURL.searchParams.get("tab")).toBe("activity");
		expect(queryURL.searchParams.get("page")).toBe("2");
		expect(mutationURL.pathname).toBe("/api/users/a%2Fb");
	});

	it("resolveBody serializes plain object input", async () => {
		const api = await loadPublicClientAPI();
		expect(
			api.resolveBody({
				input: {
					a: 1,
				},
			}),
		).toBe(
			JSON.stringify({
				a: 1,
			}),
		);
	});

	it("resolveBody passes through ReadableStream bodies when available", async () => {
		const api = await loadPublicClientAPI();
		const globalRecord = globalThis as AnyPropertyRecord;
		const originalReadableStream = globalRecord.ReadableStream;
		const FakeReadableStream = function FakeReadableStream(this: unknown) {
			return this;
		};
		try {
			globalRecord.ReadableStream = FakeReadableStream;
			const stream = new (FakeReadableStream as any)();
			expect(
				api.resolveBody({
					input: stream,
				}),
			).toBe(stream);
		} finally {
			globalRecord.ReadableStream = originalReadableStream;
		}
	});

	it("resolveBody passes through ArrayBuffer-backed typed-array views without cloning", async () => {
		const api = await loadPublicClientAPI();
		const input = new Uint8Array([1, 2, 3]);
		const resolvedBody = api.resolveBody({ input });
		expect(resolvedBody).toBe(input);
	});

	it("buildMutationURL URL-encodes dynamic params and replaces repeated exact tokens", async () => {
		const api = await loadPublicClientAPI();
		const url = api.buildMutationURL(TEST_VORMA_APP_CONFIG, {
			pattern: "/users/:id/posts/:id",
			params: {
				id: "a/b",
			},
		});
		expect(url.pathname).toBe("/api/users/a%2Fb/posts/a%2Fb");
	});

	it("buildMutationURL URL-encodes splat segments individually", async () => {
		const api = await loadPublicClientAPI();
		const url = api.buildMutationURL(TEST_VORMA_APP_CONFIG, {
			pattern: "/files/*",
			splatValues: ["folder one", "nested/path"],
		});
		expect(url.pathname).toBe("/api/files/folder%20one/nested%2Fpath");
	});

	it("buildQueryURL strips explicit index segments after path resolution", async () => {
		const api = await loadPublicClientAPI();
		const url = api.buildQueryURL(TEST_VORMA_APP_CONFIG, {
			pattern: "/docs/_index",
			input: {
				mode: "full",
			},
		});
		expect(url.pathname).toBe("/api/docs");
	});

	it("buildQueryURL treats null query input as empty", async () => {
		const api = await loadPublicClientAPI();
		const nullInputURL = api.buildQueryURL(TEST_VORMA_APP_CONFIG, {
			pattern: "/users",
			input: null,
		});
		expect(nullInputURL.search).toBe("");
	});

	it("buildMutationURL does not encode input into query params", async () => {
		const api = await loadPublicClientAPI();
		const url = api.buildMutationURL(TEST_VORMA_APP_CONFIG, {
			pattern: "/users/:id",
			params: {
				id: "42",
			},
			input: {
				shouldNotAppear: true,
			},
		});
		expect(url.pathname).toBe("/api/users/42");
		expect(url.search).toBe("");
	});

	it("buildQueryURL and buildMutationURL resolve action paths using action runes", async () => {
		const api = await loadPublicClientAPI();
		const customConfig = {
			...TEST_VORMA_APP_CONFIG,
			actionsDynamicRune: "$",
			actionsSplatRune: "~",
		};
		const queryURL = api.buildQueryURL(customConfig, {
			pattern: "/docs/$id",
			params: {
				id: "a/b",
			},
		});
		const mutationURL = api.buildMutationURL(customConfig, {
			pattern: "/assets/~",
			splatValues: ["icons", "main.svg"],
		});
		expect(queryURL.pathname).toBe("/api/docs/a%2Fb");
		expect(mutationURL.pathname).toBe("/api/assets/icons/main.svg");
	});

	it("resolveBody wrapper returns BodyInit-compatible values for object and URLSearchParams inputs", async () => {
		const api = await loadPublicClientAPI();
		const urlSearchParams = new URLSearchParams({
			a: "1",
		});
		expect(
			api.resolveBody({
				input: {
					a: 1,
				},
			}),
		).toBe(
			JSON.stringify({
				a: 1,
			}),
		);
		expect(
			api.resolveBody({
				input: urlSearchParams,
			}),
		).toBe(urlSearchParams);
	});

	it("includes vorma_json marker query param on navigation route-data requests", async () => {
		const api = await loadPublicClientAPI();
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				loadersData: [],
			}),
		);

		await api.vormaNavigate("/marker-check?x=1");
		await vi.runAllTimersAsync();

		const firstRequestURL = requestInputToURL(
			fetchSpy.mock.calls[0]?.[0] as RequestInfo | URL,
		);
		expect(firstRequestURL.searchParams.get("vorma_json")).toBe("1");
		expect(firstRequestURL.searchParams.get("x")).toBe("1");
	});

	it("includes the current build id in navigation route-data requests", async () => {
		const api = await loadPublicClientAPI();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse(
					{
						loadersData: [],
					},
					{
						headers: {
							"X-Wave-Framework-Build-Id": "build-2",
						},
					},
				),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse(
					{
						loadersData: [],
					},
					{
						headers: {
							"X-Wave-Framework-Build-Id": "build-3",
						},
					},
				),
			);

		await api.vormaNavigate("/build-id-first");
		await vi.runAllTimersAsync();
		await api.vormaNavigate("/build-id-second");
		await vi.runAllTimersAsync();

		const firstRequestURL = requestInputToURL(
			fetchSpy.mock.calls[0]?.[0] as RequestInfo | URL,
		);
		const secondRequestURL = requestInputToURL(
			fetchSpy.mock.calls[1]?.[0] as RequestInfo | URL,
		);
		expect(firstRequestURL.searchParams.get("vorma_json")).toBe("1");
		expect(secondRequestURL.searchParams.get("vorma_json")).toBe("build-2");
	});

	it("uses build-id fallback 1 in route-data request URLs when response header is missing", async () => {
		const api = await loadPublicClientAPI();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				new Response(
					JSON.stringify(
						createRouteData({
							loadersData: [],
						}),
					),
					{
						status: 200,
						headers: {
							"Content-Type": "application/json",
						},
					},
				),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					loadersData: [],
				}),
			);

		await api.vormaNavigate("/build-id-missing-header");
		await vi.runAllTimersAsync();
		await api.vormaNavigate("/build-id-follow-up");
		await vi.runAllTimersAsync();

		const firstRequestURL = requestInputToURL(
			fetchSpy.mock.calls[0]?.[0] as RequestInfo | URL,
		);
		const secondRequestURL = requestInputToURL(
			fetchSpy.mock.calls[1]?.[0] as RequestInfo | URL,
		);
		expect(firstRequestURL.searchParams.get("vorma_json")).toBe("1");
		expect(api.getBuildID()).toBe("1");
		expect(secondRequestURL.pathname).toBe("/build-id-follow-up");
	});

	it("uses server fetch when current snapshot lacks loader data for the matched route", async () => {
		const api = await loadPublicClientAPI();
		vi.doMock("/needs-data-module.js", () => ({
			default: () => null,
		}));
		setRouteManifestForTesting({
			"/needs-data": 1,
		});
		registerClientLoaderForTesting({
			pattern: "/needs-data",
			clientLoader: async ({ serverDataPromise }: any) => {
				await serverDataPromise;
				return {
					fromClient: true,
				};
			},
		});
		seedRuntimeRouteSnapshotForTesting({
			matchedPatterns: ["/needs-data"],
			loadersData: [],
			importURLs: ["/needs-data-module.js"],
			exportKeys: ["default"],
			errorExportKeys: [""],
		});
		await api.initClient({});

		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				matchedPatterns: ["/needs-data"],
				loadersData: [{ serverData: "from-server" }],
				importURLs: ["/needs-data-module.js"],
				exportKeys: ["default"],
				errorExportKeys: [""],
				hasRootData: true,
			}),
		);

		await api.vormaNavigate("/needs-data");
		await vi.runAllTimersAsync();

		expect(fetchSpy).toHaveBeenCalledTimes(1);
		expect(api.getRouterData().rootData).toEqual({
			serverData: "from-server",
		});
	});

	it("typed navigate resolves route params and forwards navigation options", async () => {
		const api = await loadPublicClientAPI();
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				loadersData: [],
			}),
		);
		const typedNavigate = api.makeTypedNavigate(TEST_VORMA_APP_CONFIG);

		await typedNavigate({
			pattern: "/docs/*",
			splatValues: ["guides", "intro"],
			search: "?mode=full",
			hash: "#overview",
			replace: true,
			scrollToTop: false,
			state: {
				source: "black-box",
			},
		} as any);
		await vi.runAllTimersAsync();

		const fetchURL = requestInputToURL(
			fetchSpy.mock.calls[0]?.[0] as RequestInfo | URL,
		);
		expect(fetchURL.pathname).toBe("/docs/guides/intro");
		expect(fetchURL.searchParams.get("mode")).toBe("full");
		expect(fetchURL.searchParams.get("vorma_json")).toBe("1");
		expect(api.getLocation()).toEqual({
			pathname: "/docs/guides/intro",
			search: "?mode=full",
			hash: "#overview",
			state: {
				source: "black-box",
			},
		});
	});

	it("typed API client query/mutate apply decorator and preserve submit result contract", async () => {
		const api = await loadPublicClientAPI();
		const decorator = vi
			.fn()
			.mockResolvedValueOnce(undefined)
			.mockResolvedValueOnce({
				headers: {
					"X-Decorator": "1",
				},
			});
		const typedClient = api.makeTypedAPIClient(
			TEST_VORMA_APP_CONFIG,
			decorator as any,
		);
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				new Response(JSON.stringify({ users: [1, 2] }), {
					status: 200,
					headers: {
						"Content-Type": "application/json",
					},
				}),
			)
			.mockResolvedValueOnce(
				new Response(JSON.stringify({ ok: true }), {
					status: 200,
					headers: {
						"Content-Type": "application/json",
					},
				}),
			);

		const queryResult = await typedClient.query({
			pattern: "/users/:id",
			params: {
				id: "42",
			},
			input: {
				include: "posts",
			},
			options: {
				revalidate: false,
			},
		} as any);
		const mutateResult = await typedClient.mutate({
			pattern: "/users/:id",
			params: {
				id: "42",
			},
			input: {
				name: "Ada",
			},
			options: {
				revalidate: false,
			},
		} as any);
		await vi.runAllTimersAsync();

		expect(queryResult).toEqual({
			success: true,
			data: {
				users: [1, 2],
			},
		});
		expect(mutateResult).toEqual({
			success: true,
			data: {
				ok: true,
			},
		});
		expect(decorator).toHaveBeenCalledTimes(2);
		expect(fetchSpy.mock.calls[0]?.[1]).toEqual(
			expect.objectContaining({
				method: "GET",
			}),
		);
		expect(
			requestInputToURL(fetchSpy.mock.calls[0]?.[0] as RequestInfo | URL)
				.pathname,
		).toBe("/api/users/42");
		expect(
			requestInputToURL(fetchSpy.mock.calls[1]?.[0] as RequestInfo | URL)
				.pathname,
		).toBe("/api/users/42");
		expect(fetchSpy.mock.calls[1]?.[1]).toEqual(
			expect.objectContaining({
				method: "POST",
				headers: expect.any(Headers),
			}),
		);
		const mutationRequestInit = fetchSpy.mock.calls[1]?.[1] as
			| RequestInit
			| undefined;
		const mutationHeaders = mutationRequestInit?.headers;
		expect(
			mutationHeaders
				? new Headers(mutationHeaders).get("X-Decorator")
				: null,
		).toBe("1");
	});

	it("merges resolver defaults with per-call requestInit headers for typed API query calls", async () => {
		const api = await loadPublicClientAPI();
		const decorator = vi.fn().mockResolvedValue({
			credentials: "include",
			headers: {
				Authorization: "Bearer token",
				"X-Defaults": "1",
			},
		});
		const typedClient = api.makeTypedAPIClient(
			TEST_VORMA_APP_CONFIG,
			decorator as any,
		);
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			new Response(JSON.stringify({ ids: ["1"] }), {
				status: 200,
				headers: {
					"Content-Type": "application/json",
				},
			}),
		);

		const result = await typedClient.query({
			pattern: "/users/:id",
			params: { id: "42" },
			input: { include: "posts" },
			requestInit: {
				headers: {
					"X-Trace-ID": "trace-1",
				},
			},
			options: {
				revalidate: false,
			},
		} as any);
		await vi.runAllTimersAsync();

		expect(result).toEqual({
			success: true,
			data: {
				ids: ["1"],
			},
		});
		expect(decorator).toHaveBeenCalledTimes(1);
		expect(fetchSpy).toHaveBeenCalledTimes(1);
		const requestInit = fetchSpy.mock.calls[0]?.[1] as RequestInit;
		const headers = new Headers(requestInit.headers ?? undefined);
		expect(requestInit.method).toBe("GET");
		expect(requestInit.credentials).toBe("include");
		expect(headers.get("Authorization")).toBe("Bearer token");
		expect(headers.get("X-Defaults")).toBe("1");
		expect(headers.get("X-Trace-ID")).toBe("trace-1");
	});

	it("supports mutation calls without request-init decoration", async () => {
		const api = await loadPublicClientAPI();
		const typedClient = api.makeTypedAPIClient(TEST_VORMA_APP_CONFIG);
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			new Response(JSON.stringify({ ok: true }), {
				status: 200,
				headers: {
					"Content-Type": "application/json",
				},
			}),
		);

		const result = await typedClient.mutate({
			pattern: "/users/:id",
			params: { id: "42" },
			input: { username: "alice" },
			requestInit: {
				headers: {
					"X-Request-ID": "req-1",
				},
			},
			options: {
				revalidate: false,
			},
		} as any);
		await vi.runAllTimersAsync();

		expect(result).toEqual({
			success: true,
			data: {
				ok: true,
			},
		});
		expect(fetchSpy).toHaveBeenCalledTimes(1);
		const requestInit = fetchSpy.mock.calls[0]?.[1] as RequestInit;
		const headers = new Headers(requestInit.headers ?? undefined);
		expect(requestInit.method).toBe("POST");
		expect(requestInit.body).toBe(JSON.stringify({ username: "alice" }));
		expect(headers.get("X-Request-ID")).toBe("req-1");
	});

	it("returns formatted errors through defaultErrorBoundary", async () => {
		const api = await loadPublicClientAPI();
		expect(api.defaultErrorBoundary({ error: "boom" })).toBe(
			"Route Error: boom",
		);
	});

	it("does not expose buildtime-only route registration from the runtime client entry", async () => {
		const runtimeEntry = await import("../../client/index.ts");
		expect("route" in runtimeEntry).toBe(false);
		expect((runtimeEntry as Record<string, unknown>).route).toBeUndefined();
	});

	it("does not expose unstable internal __* helpers from the runtime client entry", async () => {
		const runtimeEntry = await import("../../client/index.ts");
		const runtimeEntryAsRecord = runtimeEntry as Record<string, unknown>;

		expect(
			runtimeEntryAsRecord.__registerClientLoaderPattern,
		).toBeUndefined();
		expect(
			runtimeEntryAsRecord.__runClientLoadersAfterHMRUpdate,
		).toBeUndefined();
		expect(
			runtimeEntryAsRecord.__registerClientLoaderForAdapter,
		).toBeUndefined();
		expect(runtimeEntryAsRecord.__applyScrollState).toBeUndefined();
		expect(runtimeEntryAsRecord.__makeFinalLinkProps).toBeUndefined();
		expect(runtimeEntryAsRecord.__resolvePath).toBeUndefined();
		expect(
			runtimeEntryAsRecord.__getClientRuntimeRenderState,
		).toBeUndefined();
		expect(runtimeEntryAsRecord.__setClientLoaderWaitFn).toBeUndefined();
	});

	it("exposes route registration from the buildtime entry", async () => {
		const buildtimeEntry = await import("../../client/buildtime.ts");
		expect(typeof buildtimeEntry.route).toBe("function");
		expect(() =>
			buildtimeEntry.route(
				"/",
				Promise.resolve({
					default: () => null,
				}),
				"default",
			),
		).not.toThrow();
	});

	it("returns listener cleanup functions that unsubscribe handlers", async () => {
		const api = await loadPublicClientAPI();
		const listener = vi.fn();
		const cleanup = api.addStatusListener(listener);

		window.dispatchEvent(
			new CustomEvent("vorma:status", {
				detail: {
					isNavigating: false,
					isSubmitting: false,
					isRevalidating: false,
				},
			}),
		);
		expect(listener).toHaveBeenCalledTimes(1);

		cleanup();
		window.dispatchEvent(
			new CustomEvent("vorma:status", {
				detail: {
					isNavigating: true,
					isSubmitting: false,
					isRevalidating: false,
				},
			}),
		);
		expect(listener).toHaveBeenCalledTimes(1);
	});

	it("registers listeners on window for all event types", async () => {
		const api = await loadPublicClientAPI();
		const addEventListenerSpy = vi.spyOn(window, "addEventListener");

		const removeStatus = api.addStatusListener(() => {});
		const removeRouteChange = api.addRouteChangeListener(() => {});
		const removeLocation = api.addLocationListener(() => {});
		const removeBuildID = api.addBuildIDListener(() => {});

		expect(addEventListenerSpy).toHaveBeenCalledWith(
			"vorma:status",
			expect.any(Function),
		);
		expect(addEventListenerSpy).toHaveBeenCalledWith(
			"vorma:route-change",
			expect.any(Function),
		);
		expect(addEventListenerSpy).toHaveBeenCalledWith(
			"vorma:location",
			expect.any(Function),
		);
		expect(addEventListenerSpy).toHaveBeenCalledWith(
			"vorma:build-id",
			expect.any(Function),
		);

		removeStatus();
		removeRouteChange();
		removeLocation();
		removeBuildID();
	});

	it("returns #vorma-root from getRootEl and throws when missing", async () => {
		const api = await loadPublicClientAPI();
		void api.getStatus();
		const root = document.createElement("div");
		root.id = "vorma-root";
		document.body.appendChild(root);

		expect(api.getRootEl()).toBe(root);

		root.remove();
		expect(() => api.getRootEl()).toThrow(
			'Expected element with id "vorma-root" to exist',
		);
	});

	it("returns #vorma-root when root is a non-div HTMLElement", async () => {
		const api = await loadPublicClientAPI();
		void api.getStatus();
		const root = document.createElement("main");
		root.id = "vorma-root";
		document.body.appendChild(root);

		expect(api.getRootEl()).toBe(root);
	});

	it("returns safe default router data before init and before any route commit", async () => {
		const api = await loadPublicClientAPI();
		expect(api.getRouterData()).toEqual({
			buildID: "1",
			matchedPatterns: [],
			splatValues: [],
			params: {},
			rootData: null,
		});
	});

	it("uses configured root element id via initClient", async () => {
		const api = await loadPublicClientAPI();
		const customRoot = document.createElement("section");
		customRoot.id = "custom-root";
		document.body.appendChild(customRoot);
		vi.spyOn(window, "fetch").mockResolvedValue(
			new Response(JSON.stringify({}), {
				status: 200,
				headers: {
					"Content-Type": "application/json",
				},
			}),
		);

		await api.initClient({
			rootElementID: "custom-root",
		});

		expect(api.getRootEl()).toBe(customRoot);
	});

	it("initializes client options and calls render function", async () => {
		const api = await loadPublicClientAPI();
		const renderFn = vi.fn();

		await api.initClient({
			renderFn,
		});

		expect(renderFn).toHaveBeenCalledTimes(1);
	});

	it("loads initial components during init from current importURLs", async () => {
		const api = await loadPublicClientAPI();
		let didLoadInitialModule = false;
		vi.doMock("/initial.js", () => {
			didLoadInitialModule = true;
			return {
				default: () => null,
			};
		});
		seedRuntimeRouteSnapshotForTesting({
			matchedPatterns: ["/"],
			loadersData: [{ initial: "data" }],
			importURLs: ["/initial.js"],
			exportKeys: ["default"],
			errorExportKeys: [""],
			hasRootData: true,
		});

		await api.initClient({});

		expect(didLoadInitialModule).toBe(true);
	});

	it("accepts null outermostServerErrorIdx during init bootstrap", async () => {
		const api = await loadPublicClientAPI();
		let didLoadInitialModule = false;
		vi.doMock("/initial-null-error-idx.js", () => {
			didLoadInitialModule = true;
			return {
				default: () => null,
			};
		});
		seedRuntimeRouteSnapshotForTesting({
			matchedPatterns: ["/"],
			loadersData: [{ initial: "data" }],
			importURLs: ["/initial-null-error-idx.js"],
			exportKeys: ["default"],
			errorExportKeys: [""],
			hasRootData: true,
			outermostServerErrorIdx: null,
		});

		await expect(api.initClient({})).resolves.toBeUndefined();
		expect(didLoadInitialModule).toBe(true);
	});

	it("preserves server-rendered head on init bootstrap", async () => {
		const api = await loadPublicClientAPI();
		vi.doMock("/initial-head-no-bool-attrs.js", () => ({
			default: () => null,
		}));
		document.title = "Server Rendered Title";
		seedRuntimeRouteSnapshotForTesting({
			matchedPatterns: ["/"],
			loadersData: [{ initial: "data" }],
			importURLs: ["/initial-head-no-bool-attrs.js"],
			exportKeys: ["default"],
			errorExportKeys: [""],
			hasRootData: true,
			metaHeadEls: [
				{
					tag: "meta",
					attributesKnownSafe: {
						name: "description",
						content: "head-no-bool-attrs",
					},
				},
			],
			restHeadEls: [
				{
					tag: "link",
					attributesKnownSafe: {
						rel: "canonical",
						href: "/head-no-bool-attrs",
					},
				},
			],
		});

		await expect(api.initClient({})).resolves.toBeUndefined();
		expect(document.title).toBe("Server Rendered Title");
		expect(
			document.head.querySelector('meta[name="description"]'),
		).toBeNull();
		expect(document.head.querySelector('link[rel="canonical"]')).toBeNull();
	});

	it("runs initial client wait functions during init with current loader data", async () => {
		const api = await loadPublicClientAPI();
		vi.doMock("/initial-client-loader.js", () => ({
			default: () => null,
		}));
		const observedServerData: unknown[] = [];
		registerClientLoaderForTesting({
			pattern: "/",
			clientLoader: async (input: unknown) => {
				const clientLoaderInput = input as {
					serverDataPromise: Promise<unknown>;
				};
				observedServerData.push(
					await clientLoaderInput.serverDataPromise,
				);
				return {
					initialized: true,
				};
			},
		});
		seedRuntimeRouteSnapshotForTesting({
			matchedPatterns: ["/"],
			loadersData: [{ initial: "data" }],
			importURLs: ["/initial-client-loader.js"],
			exportKeys: ["default"],
			errorExportKeys: [""],
			hasRootData: true,
		});

		await api.initClient({});

		expect(observedServerData).toEqual([
			{
				matchedPatterns: ["/"],
				rootData: { initial: "data" },
				loaderData: { initial: "data" },
				buildID: "1",
			},
		]);
	});

	it("exposes a dev revalidate handle on window during init", async () => {
		const api = await loadPublicClientAPI();
		delete (window as AnyPropertyRecord).__waveRevalidate;

		await api.initClient({
			renderFn: () => {},
		});

		expect((window as AnyPropertyRecord).__waveRevalidate).toBe(
			api.revalidate,
		);
	});

	it("switches pointer modality between touch and fine pointers after init", async () => {
		const api = await loadPublicClientAPI();
		await api.initClient({});

		expect(readIsTouchInputModalityActiveForTesting()).toBe(false);

		window.dispatchEvent(new Event("touchstart"));
		expect(readIsTouchInputModalityActiveForTesting()).toBe(true);

		const mousePointerMoveEvent = new Event("pointermove");
		Object.defineProperty(mousePointerMoveEvent, "pointerType", {
			value: "mouse",
		});
		window.dispatchEvent(mousePointerMoveEvent);
		expect(readIsTouchInputModalityActiveForTesting()).toBe(false);

		const touchPointerDownEvent = new Event("pointerdown");
		Object.defineProperty(touchPointerDownEvent, "pointerType", {
			value: "touch",
		});
		window.dispatchEvent(touchPointerDownEvent);
		expect(readIsTouchInputModalityActiveForTesting()).toBe(true);
	});

	it("keeps pointer-modality side effects idempotent for repeated same-modality events", async () => {
		const api = await loadPublicClientAPI();
		await api.initClient({});
		const fetchSpy = vi.spyOn(window, "fetch");
		const routeChangeListener = vi.fn();
		const cleanupRouteChangeListener =
			api.addRouteChangeListener(routeChangeListener);

		window.dispatchEvent(new Event("touchstart"));
		window.dispatchEvent(new Event("touchstart"));
		expect(readIsTouchInputModalityActiveForTesting()).toBe(true);

		const mousePointerMoveEvent = new Event("pointermove");
		Object.defineProperty(mousePointerMoveEvent, "pointerType", {
			value: "mouse",
		});
		window.dispatchEvent(mousePointerMoveEvent);
		const mousePointerEnterEvent = new Event("pointerenter");
		Object.defineProperty(mousePointerEnterEvent, "pointerType", {
			value: "mouse",
		});
		window.dispatchEvent(mousePointerEnterEvent);

		expect(readIsTouchInputModalityActiveForTesting()).toBe(false);
		expect(fetchSpy).not.toHaveBeenCalled();
		expect(routeChangeListener).not.toHaveBeenCalled();
		cleanupRouteChangeListener();
	});

	it("does not ever trigger revalidate() call from an HMR update", async () => {
		const api = await loadPublicClientAPI();
		const optedInLoader = vi.fn(async ({ serverDataPromise }) => {
			await serverDataPromise;
			return "opted-in";
		});
		const nonOptedLoader = vi.fn(async ({ serverDataPromise }) => {
			await serverDataPromise;
			return "non-opted";
		});

		registerClientLoaderForTesting({
			pattern: "/hmr-opted-in",
			clientLoader: optedInLoader,
			reRunOnModuleChange: {
				url: "http://localhost:3000/src/routes/hmr-opted-in.tsx?t=1",
			} as ImportMeta,
		});
		registerClientLoaderForTesting({
			pattern: "/hmr-non-opted",
			clientLoader: nonOptedLoader,
		});

		vi.doMock("/src/routes/hmr-opted-in.tsx", () => ({
			default: () => null,
		}));
		vi.doMock("/src/routes/hmr-non-opted.tsx", () => ({
			default: () => null,
		}));

		seedRuntimeRouteSnapshotForTesting({
			matchedPatterns: ["/hmr-opted-in", "/hmr-non-opted"],
			loadersData: [{ id: "a" }, { id: "b" }],
			importURLs: [
				"/src/routes/hmr-opted-in.tsx",
				"/src/routes/hmr-non-opted.tsx",
			],
			exportKeys: ["default", "default"],
			errorExportKeys: ["", ""],
		});

		await api.initClient({});
		expect(optedInLoader).toHaveBeenCalledTimes(1);
		expect(nonOptedLoader).toHaveBeenCalledTimes(1);

		const routeChangeListener = vi.fn();
		const cleanupRouteChangeListener =
			api.addRouteChangeListener(routeChangeListener);
		const sawRevalidatingStatus = vi.fn();
		const cleanupStatusListener = api.addStatusListener((statusEvent) => {
			if (statusEvent.detail.isRevalidating) {
				sawRevalidatingStatus();
			}
		});
		const fetchSpy = vi.spyOn(window, "fetch");

		const simulateUpdate = async (update: {
			type: string;
			path: string;
		}): Promise<void> => {
			await simulateViteAfterUpdateForTesting([update]);
			await vi.runAllTimersAsync();
			await Promise.resolve();
		};

		await simulateUpdate({
			type: "js-update",
			path: "/src/routes/non-matching.tsx?t=2",
		});
		await simulateUpdate({
			type: "css-update",
			path: "/src/routes/hmr-opted-in.tsx?t=3",
		});
		await simulateUpdate({
			type: "not-supported",
			path: "/src/routes/hmr-opted-in.tsx?t=4",
		});
		await simulateUpdate({
			type: "js-update",
			path: "/src/routes/hmr-non-opted.tsx?t=5",
		});
		await simulateUpdate({
			type: "js-update",
			path: "/src/routes/hmr-opted-in.tsx?t=6",
		});

		expect(fetchSpy).not.toHaveBeenCalled();
		expect(sawRevalidatingStatus).not.toHaveBeenCalled();
		expect(optedInLoader).toHaveBeenCalledTimes(2);
		expect(nonOptedLoader).toHaveBeenCalledTimes(1);
		expect(routeChangeListener).toHaveBeenCalledTimes(1);

		cleanupStatusListener();
		cleanupRouteChangeListener();
	});

	it("does not trigger server-loader revalidation for css-only HMR updates", async () => {
		const api = await loadPublicClientAPI();
		vi.doMock("/src/routes/hmr-css-only.tsx", () => ({
			default: () => null,
		}));
		seedRuntimeRouteSnapshotForTesting({
			matchedPatterns: ["/hmr-css-only"],
			loadersData: [{ from: "initial" }],
			importURLs: ["/src/routes/hmr-css-only.tsx"],
			exportKeys: ["default"],
			errorExportKeys: [""],
			hasRootData: true,
		});
		await api.initClient({});
		const fetchSpy = vi.spyOn(window, "fetch");

		await simulateViteAfterUpdateForTesting([
			{
				type: "css-update",
				path: "/src/routes/hmr-css-only.tsx?t=2",
			},
		]);
		await vi.runAllTimersAsync();
		await Promise.resolve();

		expect(fetchSpy).not.toHaveBeenCalled();
	});

	it("does not trigger server-loader revalidation for non-matching HMR js updates", async () => {
		const api = await loadPublicClientAPI();
		vi.doMock("/src/routes/hmr-current.tsx", () => ({
			default: () => null,
		}));
		seedRuntimeRouteSnapshotForTesting({
			matchedPatterns: ["/hmr-current"],
			loadersData: [{ from: "initial" }],
			importURLs: ["/src/routes/hmr-current.tsx"],
			exportKeys: ["default"],
			errorExportKeys: [""],
			hasRootData: true,
		});
		await api.initClient({});
		const fetchSpy = vi.spyOn(window, "fetch");

		await simulateViteAfterUpdateForTesting([
			{
				type: "js-update",
				path: "/src/routes/other.tsx?t=3",
			},
		]);
		await vi.runAllTimersAsync();
		await Promise.resolve();

		expect(fetchSpy).not.toHaveBeenCalled();
	});

	it("ignores malformed HMR update payloads without throwing or server-loader revalidation", async () => {
		const api = await loadPublicClientAPI();
		vi.doMock("/src/routes/hmr-invalid.tsx", () => ({
			default: () => null,
		}));
		seedRuntimeRouteSnapshotForTesting({
			matchedPatterns: ["/hmr-invalid"],
			loadersData: [{ from: "initial" }],
			importURLs: ["/src/routes/hmr-invalid.tsx"],
			exportKeys: ["default"],
			errorExportKeys: [""],
			hasRootData: true,
		});
		await api.initClient({});
		const fetchSpy = vi.spyOn(window, "fetch");

		await expect(
			simulateViteAfterUpdateForTesting([
				{
					type: "not-supported",
					path: "/src/routes/hmr-invalid.tsx?t=99",
				},
			]),
		).resolves.toBeUndefined();
		await vi.runAllTimersAsync();
		await Promise.resolve();

		expect(fetchSpy).not.toHaveBeenCalled();
	});

	it("does not throw or trigger server-loader revalidation when no matched import URLs are available for HMR updates", async () => {
		const api = await loadPublicClientAPI();
		await api.initClient({});
		const fetchSpy = vi.spyOn(window, "fetch");

		await expect(
			simulateViteAfterUpdateForTesting([
				{
					type: "js-update",
					path: "/src/routes/any.tsx?t=1",
				},
			]),
		).resolves.toBeUndefined();
		await vi.runAllTimersAsync();
		await Promise.resolve();

		expect(fetchSpy).not.toHaveBeenCalled();
	});

	it("does not trigger server-loader revalidation when matched routes share the updated module path", async () => {
		const api = await loadPublicClientAPI();
		vi.doMock("/src/routes/hmr-shared.tsx", () => ({
			default: () => null,
		}));
		seedRuntimeRouteSnapshotForTesting({
			matchedPatterns: ["/hmr-first", "/hmr-second"],
			loadersData: [{ route: "first" }, { route: "second" }],
			importURLs: [
				"/src/routes/hmr-shared.tsx",
				"/src/routes/hmr-shared.tsx",
			],
			exportKeys: ["default", "default"],
			errorExportKeys: ["", ""],
		});
		await api.initClient({});
		const fetchSpy = vi.spyOn(window, "fetch");

		await simulateViteAfterUpdateForTesting([
			{
				type: "js-update",
				path: "/src/routes/hmr-shared.tsx?t=9",
			},
		]);
		await vi.runAllTimersAsync();
		await Promise.resolve();

		expect(fetchSpy).not.toHaveBeenCalled();
	});

	it("preserves HMR no-op behavior across repeated init calls when no opted-in client-loader modules exist", async () => {
		const api = await loadPublicClientAPI();
		vi.doMock("/src/routes/hmr-reinit.tsx", () => ({
			default: () => null,
		}));
		seedRuntimeRouteSnapshotForTesting({
			matchedPatterns: ["/hmr-reinit"],
			loadersData: [{ from: "initial" }],
			importURLs: ["/src/routes/hmr-reinit.tsx"],
			exportKeys: ["default"],
			errorExportKeys: [""],
			hasRootData: true,
		});
		await api.initClient({});
		const routeChangeListener = vi.fn();
		const cleanupRouteChangeListener =
			api.addRouteChangeListener(routeChangeListener);
		const fetchSpy = vi.spyOn(window, "fetch");

		await simulateViteAfterUpdateForTesting([
			{
				type: "js-update",
				path: "/src/routes/hmr-reinit.tsx?t=1",
			},
		]);
		await vi.runAllTimersAsync();
		await Promise.resolve();
		expect(fetchSpy).not.toHaveBeenCalled();
		expect(routeChangeListener).not.toHaveBeenCalled();

		await api.initClient({});
		routeChangeListener.mockClear();
		await simulateViteAfterUpdateForTesting([
			{
				type: "js-update",
				path: "/src/routes/hmr-reinit.tsx?t=2",
			},
		]);
		await vi.runAllTimersAsync();
		await Promise.resolve();
		expect(fetchSpy).not.toHaveBeenCalled();
		expect(routeChangeListener).not.toHaveBeenCalled();

		cleanupRouteChangeListener();
	});

	it("reads values from the shared global state", () => {
		setRouteManifestForTesting({
			"/shared-read/:id": 1,
		});

		expect(readRouteManifestForTesting()).toEqual({
			"/shared-read/:id": 1,
		});
	});

	it("writes values into the shared global state", () => {
		setRouteManifestForTesting({
			"/shared-write/one": 1,
		});
		expect(readRouteManifestForTesting()).toEqual({
			"/shared-write/one": 1,
		});

		setRouteManifestForTesting({
			"/shared-write/two": 1,
		});
		expect(readRouteManifestForTesting()).toEqual({
			"/shared-write/two": 1,
		});
	});

	it("uses precompiled route manifest during init without progressive fetch", async () => {
		const api = await loadPublicClientAPI();
		setRouteManifestForTesting({
			"/precompiled/:id": 1,
		});
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			new Response("unused", {
				status: 200,
				headers: {
					"Content-Type": "application/json",
				},
			}),
		);

		await api.initClient({
			routeManifestURL: "http://localhost:3000/manifest.json",
		});

		expect(fetchSpy).not.toHaveBeenCalled();
		expect(readRouteManifestForTesting()).toEqual({
			"/precompiled/:id": 1,
		});
	});

	it("falls back to progressive route-manifest loading when precompiled manifest is absent", async () => {
		const api = await loadPublicClientAPI();
		setRouteManifestForTesting(undefined);
		const manifest = {
			"/progressive/:id": 1,
		};
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			new Response(JSON.stringify(manifest), {
				status: 200,
				headers: {
					"Content-Type": "application/json",
				},
			}),
		);

		await api.initClient({
			routeManifestURL: "http://localhost:3000/manifest.json",
		});

		expect(fetchSpy).toHaveBeenCalledWith(
			"http://localhost:3000/manifest.json",
			{
				method: "GET",
			},
		);
		expect(readRouteManifestForTesting()).toEqual(manifest);
	});

	it("treats route-manifest progressive loading failures as non-fatal", async () => {
		const api = await loadPublicClientAPI();
		setRouteManifestForTesting(undefined);
		const fetchError = new Error("manifest network failed");
		vi.spyOn(window, "fetch").mockRejectedValue(fetchError);
		const warnSpy = vi.spyOn(console, "warn");

		await expect(
			api.initClient({
				routeManifestURL: "http://localhost:3000/manifest.json",
			}),
		).resolves.toBeUndefined();

		expect(readRouteManifestForTesting()).toBeUndefined();
		expect(warnSpy).toHaveBeenCalledWith(
			"Failed to load route manifest:",
			fetchError,
		);
	});

	it("treats non-OK route-manifest responses as non-fatal and leaves state unchanged", async () => {
		const api = await loadPublicClientAPI();
		setRouteManifestForTesting(undefined);
		vi.spyOn(window, "fetch").mockResolvedValue(
			new Response("manifest unavailable", {
				status: 503,
				headers: {
					"Content-Type": "text/plain",
				},
			}),
		);
		const warnSpy = vi.spyOn(console, "warn");

		await expect(
			api.initClient({
				routeManifestURL: "http://localhost:3000/manifest.json",
			}),
		).resolves.toBeUndefined();

		expect(readRouteManifestForTesting()).toBeUndefined();
		const manifestWarningCall = warnSpy.mock.calls.find(
			(call) => call[0] === "Failed to load route manifest:",
		);
		expect(manifestWarningCall).toBeDefined();
		const manifestWarningError = manifestWarningCall?.[1];
		expect(manifestWarningError).toBeInstanceOf(Error);
		expect((manifestWarningError as Error).message).toContain("status 503");
	});

	it("ignores stale older progressive manifest responses after a newer init", async () => {
		const api = await loadPublicClientAPI();
		setRouteManifestForTesting(undefined);
		const olderManifest = {
			"/stale-manifest-old/:id": 1,
		};
		const newerManifest = {
			"/stale-manifest-new/:id": 1,
		};
		const olderManifestDeferred = createDeferred<Response>();
		const newerManifestDeferred = createDeferred<Response>();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockImplementation((input) => {
				const href =
					typeof input === "string"
						? input
						: input instanceof URL
							? input.href
							: input.url;
				if (href === "http://localhost:3000/manifest-old.json") {
					return olderManifestDeferred.promise;
				}
				if (href === "http://localhost:3000/manifest-new.json") {
					return newerManifestDeferred.promise;
				}
				throw new Error(`Unexpected manifest fetch URL: ${href}`);
			});

		const olderInitPromise = api.initClient({
			routeManifestURL: "http://localhost:3000/manifest-old.json",
		});
		const newerInitPromise = api.initClient({
			routeManifestURL: "http://localhost:3000/manifest-new.json",
		});

		newerManifestDeferred.resolve(
			new Response(JSON.stringify(newerManifest), {
				status: 200,
				headers: {
					"Content-Type": "application/json",
				},
			}),
		);
		await newerInitPromise;
		expect(readRouteManifestForTesting()).toEqual(newerManifest);

		olderManifestDeferred.resolve(
			new Response(JSON.stringify(olderManifest), {
				status: 200,
				headers: {
					"Content-Type": "application/json",
				},
			}),
		);
		await olderInitPromise;

		expect(fetchSpy).toHaveBeenNthCalledWith(
			1,
			"http://localhost:3000/manifest-old.json",
			{
				method: "GET",
			},
		);
		expect(fetchSpy).toHaveBeenNthCalledWith(
			2,
			"http://localhost:3000/manifest-new.json",
			{
				method: "GET",
			},
		);
		expect(readRouteManifestForTesting()).toEqual(newerManifest);
	});

	it("ignores progressive manifest payload when pattern registry is replaced mid-flight", async () => {
		const api = await loadPublicClientAPI();
		setRouteManifestForTesting(undefined);
		const deferredManifestResponse = createDeferred<Response>();
		vi.spyOn(window, "fetch").mockImplementation(
			() => deferredManifestResponse.promise,
		);

		const initPromise = api.initClient({
			routeManifestURL: "http://localhost:3000/manifest.json",
		});
		replacePatternRegistryForTesting();

		deferredManifestResponse.resolve(
			new Response(
				JSON.stringify({
					"/manifest-should-be-ignored/:id": 1,
				}),
				{
					status: 200,
					headers: {
						"Content-Type": "application/json",
					},
				},
			),
		);
		await initPromise;

		expect(readRouteManifestForTesting()).toBeUndefined();
	});

	it("applies useViewTransitions option on every init call", async () => {
		const api = await loadPublicClientAPI();
		const startViewTransitionSpy = vi.fn((runTransition: () => void) => {
			runTransition();
			return { finished: Promise.resolve() };
		});
		Object.defineProperty(document, "startViewTransition", {
			configurable: true,
			value: startViewTransitionSpy,
		});
		vi.spyOn(window, "fetch").mockImplementation(() =>
			Promise.resolve(
				createRouteDataResponse({
					loadersData: [],
					title: { dangerousInnerHTML: "View Transition Navigation" },
				}),
			),
		);

		await api.initClient({
			useViewTransitions: true,
		});
		await api.vormaNavigate("/view-transition-on");
		await vi.runAllTimersAsync();
		expect(startViewTransitionSpy).toHaveBeenCalledTimes(1);

		await api.initClient({
			useViewTransitions: false,
		});
		await api.vormaNavigate("/view-transition-off");
		await vi.runAllTimersAsync();
		expect(startViewTransitionSpy).toHaveBeenCalledTimes(1);
	});

	it("skips view transitions for prefetch and revalidation navigation types", async () => {
		const api = await loadPublicClientAPI();
		const startViewTransitionSpy = vi.fn((runTransition: () => void) => {
			runTransition();
			return { finished: Promise.resolve() };
		});
		Object.defineProperty(document, "startViewTransition", {
			configurable: true,
			value: startViewTransitionSpy,
		});
		vi.spyOn(window, "fetch").mockImplementation(() =>
			Promise.resolve(
				createRouteDataResponse({
					loadersData: [],
				}),
			),
		);

		await api.initClient({
			useViewTransitions: true,
		});

		await api.vormaNavigate("/view-transition-user-nav");
		await vi.runAllTimersAsync();
		expect(startViewTransitionSpy).toHaveBeenCalledTimes(1);

		await api.vormaNavigate("/view-transition-prefetch", {
			intent: "prefetch",
		} as any);
		await vi.runAllTimersAsync();
		expect(startViewTransitionSpy).toHaveBeenCalledTimes(1);

		await api.revalidate();
		await vi.runAllTimersAsync();
		expect(startViewTransitionSpy).toHaveBeenCalledTimes(1);
	});

	it("cleans vorma_reload from url during init", async () => {
		const api = await loadPublicClientAPI();
		window.history.replaceState(
			{},
			"",
			"/init-clean?vorma_reload=abc123&foo=bar#frag",
		);

		await api.initClient({});
		await vi.runAllTimersAsync();

		const currentURL = new URL(window.location.href);
		expect(currentURL.searchParams.has("vorma_reload")).toBe(false);
		expect(currentURL.searchParams.get("foo")).toBe("bar");
		expect(currentURL.pathname).toBe("/init-clean");
		expect(currentURL.hash).toBe("#frag");
	});

	it("sets history scrollRestoration to manual during init when supported", async () => {
		const api = await loadPublicClientAPI();
		const scrollRestorationSetterSpy = vi.fn();
		Object.defineProperty(window.history, "scrollRestoration", {
			configurable: true,
			get: () => "auto",
			set: scrollRestorationSetterSpy,
		});

		await api.initClient({});

		expect(scrollRestorationSetterSpy).toHaveBeenCalledWith("manual");
	});

	it("registers beforeunload scroll persistence once across repeated init calls", async () => {
		const api = await loadPublicClientAPI();
		(window as AnyPropertyRecord).scrollX = 200;
		(window as AnyPropertyRecord).scrollY = 400;
		const setItemSpy = vi.spyOn(Storage.prototype, "setItem");
		const countPageRefreshWrites = () =>
			setItemSpy.mock.calls.filter(
				([key]) =>
					typeof key === "string" &&
					isPageRefreshScrollStateStorageKeyForTesting(key),
			).length;

		await api.initClient({});
		const beforeFirstBeforeUnloadDispatchCount = countPageRefreshWrites();
		window.dispatchEvent(new Event("beforeunload"));
		const firstBeforeUnloadDispatchDelta =
			countPageRefreshWrites() - beforeFirstBeforeUnloadDispatchCount;

		await api.initClient({});
		const beforeSecondBeforeUnloadDispatchCount = countPageRefreshWrites();
		window.dispatchEvent(new Event("beforeunload"));
		const secondBeforeUnloadDispatchDelta =
			countPageRefreshWrites() - beforeSecondBeforeUnloadDispatchCount;

		expect(secondBeforeUnloadDispatchDelta).toBe(
			firstBeforeUnloadDispatchDelta,
		);
		const savedPageRefreshState = readPageRefreshScrollStateForTesting();
		expect(savedPageRefreshState).toBeTruthy();
		expect(savedPageRefreshState).toMatchObject({
			x: 200,
			y: 400,
			href: window.location.href,
		});
		expect(typeof savedPageRefreshState?.unix).toBe("number");
	});

	it("restores recent page-refresh scroll state during init", async () => {
		const api = await loadPublicClientAPI();
		seedPageRefreshScrollStateForTesting({
			x: 300,
			y: 600,
			unix: Date.now() - 1000,
			href: window.location.href,
		});
		vi.spyOn(window, "requestAnimationFrame").mockImplementation((cb) => {
			cb(0);
			return 0;
		});

		await api.initClient({});
		await vi.runAllTimersAsync();

		expect(window.scrollTo).toHaveBeenCalledWith(300, 600);
		expect(readPageRefreshScrollStateForTesting()).toBeNull();
	});

	it("drops malformed page-refresh scroll snapshots during init", async () => {
		const api = await loadPublicClientAPI();
		seedPageRefreshScrollStateForTesting({
			x: 1,
			y: 2,
			unix: Date.now(),
			href: window.location.href,
		});
		const storageKey = getPageRefreshScrollStateStorageKeyOrThrow();
		window.sessionStorage.setItem(storageKey, "{ malformed-json");

		await api.initClient({});
		await vi.runAllTimersAsync();

		expect(window.scrollTo).not.toHaveBeenCalled();
		expect(window.sessionStorage.getItem(storageKey)).toBeNull();
	});

	it("drops parseable non-object and invalid-shape page-refresh snapshots during init", async () => {
		const api = await loadPublicClientAPI();
		seedPageRefreshScrollStateForTesting({
			x: 10,
			y: 20,
			unix: Date.now(),
			href: window.location.href,
		});
		const storageKey = getPageRefreshScrollStateStorageKeyOrThrow();

		window.sessionStorage.setItem(storageKey, JSON.stringify(["bad"]));
		await api.initClient({});
		await vi.runAllTimersAsync();
		expect(window.scrollTo).not.toHaveBeenCalled();
		expect(window.sessionStorage.getItem(storageKey)).toBeNull();

		seedPageRefreshScrollStateForTesting({
			x: 10,
			y: 20,
			unix: Date.now(),
			href: window.location.href,
		});
		window.sessionStorage.setItem(
			storageKey,
			JSON.stringify({
				x: 10,
				y: "20",
				unix: Date.now(),
				href: window.location.href,
			}),
		);
		await api.initClient({});
		await vi.runAllTimersAsync();
		expect(window.scrollTo).not.toHaveBeenCalled();
		expect(window.sessionStorage.getItem(storageKey)).toBeNull();
	});

	it("restores recent page-refresh scroll state for encoding-equivalent same-document hash URLs", async () => {
		const api = await loadPublicClientAPI();
		window.history.replaceState({}, "", "/refresh-hash#%E2%9C%93");
		seedPageRefreshScrollStateForTesting({
			x: 42,
			y: 24,
			unix: Date.now() - 1000,
			href: `${window.location.origin}/refresh-hash#✓`,
		});
		vi.spyOn(window, "requestAnimationFrame").mockImplementation((cb) => {
			cb(0);
			return 0;
		});

		await api.initClient({});
		await vi.runAllTimersAsync();

		expect(window.scrollTo).toHaveBeenCalledWith(42, 24);
		expect(readPageRefreshScrollStateForTesting()).toBeNull();
	});

	it("does not restore page-refresh scroll state when stored href differs", async () => {
		const api = await loadPublicClientAPI();
		seedPageRefreshScrollStateForTesting({
			x: 250,
			y: 500,
			unix: Date.now() - 1000,
			href: `${window.location.origin}/different-page`,
		});

		await api.initClient({});
		await vi.runAllTimersAsync();

		expect(window.scrollTo).not.toHaveBeenCalledWith(250, 500);
		expect(readPageRefreshScrollStateForTesting()).toBeNull();
	});

	it("does not throw when sessionStorage getItem fails for scroll-state reads", async () => {
		const api = await loadPublicClientAPI();
		const callOriginalGetItem = (
			storage: Storage,
			key: string,
		): string | null => {
			return Storage.prototype.getItem.call(storage, key);
		};
		vi.spyOn(Storage.prototype, "getItem").mockImplementation(function (
			this: Storage,
			key: string,
		): string | null {
			if (isScrollStateStorageKeyForTesting(key)) {
				throw new Error("getItem failed");
			}
			return callOriginalGetItem(this, key);
		});
		vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				loadersData: [],
			}),
		);

		await expect(
			api.vormaNavigate("/storage-read-failure"),
		).resolves.toEqual({
			didNavigate: true,
		});
		await vi.runAllTimersAsync();
	});

	it("does not throw when sessionStorage set/remove fails for scroll persistence", async () => {
		const api = await loadPublicClientAPI();
		const callOriginalSetItem = (
			storage: Storage,
			key: string,
			value: string,
		): void => {
			Storage.prototype.setItem.call(storage, key, value);
		};
		const callOriginalRemoveItem = (
			storage: Storage,
			key: string,
		): void => {
			Storage.prototype.removeItem.call(storage, key);
		};
		vi.spyOn(Storage.prototype, "setItem").mockImplementation(function (
			this: Storage,
			key: string,
			value: string,
		): void {
			if (
				isScrollStateStorageKeyForTesting(key) ||
				isPageRefreshScrollStateStorageKeyForTesting(key)
			) {
				throw new Error("setItem failed");
			}
			return callOriginalSetItem(this, key, value);
		});
		vi.spyOn(Storage.prototype, "removeItem").mockImplementation(function (
			this: Storage,
			key: string,
		): void {
			if (isPageRefreshScrollStateStorageKeyForTesting(key)) {
				throw new Error("removeItem failed");
			}
			return callOriginalRemoveItem(this, key);
		});
		vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				loadersData: [],
			}),
		);

		await expect(
			api.vormaNavigate("/storage-write-failure"),
		).resolves.toEqual({
			didNavigate: true,
		});
		await expect(api.initClient({})).resolves.toBeUndefined();
		await vi.runAllTimersAsync();
	});

	it("does not restore page-refresh scroll state when stored snapshot is stale", async () => {
		const api = await loadPublicClientAPI();
		seedPageRefreshScrollStateForTesting({
			x: 250,
			y: 500,
			unix: Date.now() - 6000,
			href: window.location.href,
		});

		await api.initClient({});
		await vi.runAllTimersAsync();

		expect(window.scrollTo).not.toHaveBeenCalledWith(250, 500);
		expect(readPageRefreshScrollStateForTesting()).toBeNull();
	});

	it("starts and stops global loading indicator around navigation", async () => {
		const api = await loadPublicClientAPI();
		let isIndicatorRunning = false;
		const start = vi.fn(() => {
			isIndicatorRunning = true;
		});
		const stop = vi.fn(() => {
			isIndicatorRunning = false;
		});
		const cleanup = api.setupGlobalLoadingIndicator({
			start,
			stop,
			isRunning: () => isIndicatorRunning,
			include: ["navigations"],
			startDelayMS: 0,
			stopDelayMS: 0,
		});
		try {
			const deferred = createDeferred<Response>();
			vi.spyOn(window, "fetch").mockImplementation(
				() => deferred.promise,
			);

			const navigatePromise = api.vormaNavigate("/indicator-nav");
			await Promise.resolve();
			await vi.advanceTimersByTimeAsync(1);
			expect(start).toHaveBeenCalledTimes(1);

			deferred.resolve(
				createRouteDataResponse({
					loadersData: [],
				}),
			);
			await navigatePromise;
			await vi.runAllTimersAsync();

			expect(stop).toHaveBeenCalledTimes(1);
		} finally {
			cleanup();
		}
	});

	it("does not start global loading indicator for instant navigation with default delay", async () => {
		const api = await loadPublicClientAPI();
		let isIndicatorRunning = false;
		const start = vi.fn(() => {
			isIndicatorRunning = true;
		});
		const stop = vi.fn(() => {
			isIndicatorRunning = false;
		});
		const cleanup = api.setupGlobalLoadingIndicator({
			start,
			stop,
			isRunning: () => isIndicatorRunning,
			include: ["navigations"],
		});
		try {
			vi.spyOn(window, "fetch").mockResolvedValue(
				createRouteDataResponse({
					loadersData: [],
				}),
			);

			await api.vormaNavigate("/indicator-instant-default-delay");
			await vi.runAllTimersAsync();

			expect(start).not.toHaveBeenCalled();
			expect(stop).not.toHaveBeenCalled();
			expect(isIndicatorRunning).toBe(false);
		} finally {
			cleanup();
		}
	});

	it("clears pending start timer when work finishes before start delay elapses", async () => {
		const api = await loadPublicClientAPI();
		let isIndicatorRunning = false;
		const start = vi.fn(() => {
			isIndicatorRunning = true;
		});
		const stop = vi.fn(() => {
			isIndicatorRunning = false;
		});
		const clearTimeoutSpy = vi.spyOn(window, "clearTimeout");
		const cleanup = api.setupGlobalLoadingIndicator({
			start,
			stop,
			isRunning: () => isIndicatorRunning,
			include: ["navigations"],
			startDelayMS: 100,
			stopDelayMS: 0,
		});
		try {
			const deferred = createDeferred<Response>();
			vi.spyOn(window, "fetch").mockImplementation(
				() => deferred.promise,
			);

			const navigatePromise = api.vormaNavigate(
				"/indicator-cancel-start",
			);
			await vi.advanceTimersByTimeAsync(8);

			deferred.resolve(
				createRouteDataResponse({
					loadersData: [],
				}),
			);
			await navigatePromise;
			await vi.runAllTimersAsync();

			expect(start).not.toHaveBeenCalled();
			expect(stop).not.toHaveBeenCalled();
			expect(isIndicatorRunning).toBe(false);
			expect(clearTimeoutSpy).toHaveBeenCalled();
		} finally {
			cleanup();
		}
	});

	it("clears global loading indicator timers when timer id is zero", async () => {
		const api = await loadPublicClientAPI();
		const originalSetTimeout = globalThis.setTimeout.bind(globalThis);
		const setTimeoutSpy = vi
			.spyOn(window, "setTimeout")
			.mockImplementationOnce(
				() => 0 as unknown as ReturnType<typeof globalThis.setTimeout>,
			)
			.mockImplementation((handler, timeout, ...args) =>
				originalSetTimeout(handler, timeout, ...args),
			);
		const clearTimeoutSpy = vi.spyOn(window, "clearTimeout");
		const cleanup = api.setupGlobalLoadingIndicator({
			start: vi.fn(),
			stop: vi.fn(),
			isRunning: () => false,
			include: ["navigations"],
			startDelayMS: 5,
			stopDelayMS: 5,
		});
		const deferred = createDeferred<Response>();
		vi.spyOn(window, "fetch").mockImplementation(() => deferred.promise);
		const navigatePromise = api.vormaNavigate("/indicator-zero-timer");
		try {
			await Promise.resolve();
			cleanup();
			expect(setTimeoutSpy).toHaveBeenCalled();
			expect(clearTimeoutSpy).toHaveBeenCalledWith(0);
			deferred.resolve(
				createRouteDataResponse({
					loadersData: [],
				}),
			);
			await navigatePromise;
			await vi.runAllTimersAsync();
		} finally {
			setTimeoutSpy.mockRestore();
		}
	});

	it("stops and detaches global loading indicator behavior after cleanup", async () => {
		const api = await loadPublicClientAPI();
		let isIndicatorRunning = false;
		const start = vi.fn(() => {
			isIndicatorRunning = true;
		});
		const stop = vi.fn(() => {
			isIndicatorRunning = false;
		});
		const deferred = createDeferred<Response>();
		vi.spyOn(window, "fetch").mockImplementation(() => deferred.promise);
		const cleanup = api.setupGlobalLoadingIndicator({
			start,
			stop,
			isRunning: () => isIndicatorRunning,
			include: ["navigations"],
			startDelayMS: 0,
			stopDelayMS: 0,
		});

		const navigatePromise = api.vormaNavigate("/indicator-cleanup");
		await vi.advanceTimersByTimeAsync(8);
		await vi.runAllTimersAsync();
		expect(start).toHaveBeenCalledTimes(1);
		expect(isIndicatorRunning).toBe(true);

		cleanup();
		expect(stop).toHaveBeenCalledTimes(1);
		expect(isIndicatorRunning).toBe(false);

		deferred.resolve(
			createRouteDataResponse({
				loadersData: [],
			}),
		);
		await navigatePromise;
		await vi.runAllTimersAsync();

		expect(start).toHaveBeenCalledTimes(1);
		expect(stop).toHaveBeenCalledTimes(1);
	});

	it("cancels a pending stop timer when new work begins before stop delay elapses", async () => {
		const api = await loadPublicClientAPI();
		let isIndicatorRunning = false;
		const start = vi.fn(() => {
			isIndicatorRunning = true;
		});
		const stop = vi.fn(() => {
			isIndicatorRunning = false;
		});
		const clearTimeoutSpy = vi.spyOn(window, "clearTimeout");
		const firstDeferred = createDeferred<Response>();
		const secondDeferred = createDeferred<Response>();
		vi.spyOn(window, "fetch")
			.mockImplementationOnce(() => firstDeferred.promise)
			.mockImplementationOnce(() => secondDeferred.promise);
		const cleanup = api.setupGlobalLoadingIndicator({
			start,
			stop,
			isRunning: () => isIndicatorRunning,
			include: ["navigations"],
			startDelayMS: 0,
			stopDelayMS: 100,
		});
		try {
			const firstNavigatePromise = api.vormaNavigate(
				"/indicator-stop-cancel-first",
			);
			await vi.advanceTimersByTimeAsync(8);
			await vi.runAllTimersAsync();
			expect(start).toHaveBeenCalledTimes(1);
			expect(isIndicatorRunning).toBe(true);

			firstDeferred.resolve(
				createRouteDataResponse({
					loadersData: [],
				}),
			);
			await firstNavigatePromise;
			await vi.advanceTimersByTimeAsync(8);

			const secondNavigatePromise = api.vormaNavigate(
				"/indicator-stop-cancel-second",
			);
			await vi.advanceTimersByTimeAsync(8);
			await vi.advanceTimersByTimeAsync(100);

			expect(stop).not.toHaveBeenCalled();
			expect(start).toHaveBeenCalledTimes(1);
			expect(isIndicatorRunning).toBe(true);
			expect(clearTimeoutSpy).toHaveBeenCalled();

			secondDeferred.resolve(
				createRouteDataResponse({
					loadersData: [],
				}),
			);
			await secondNavigatePromise;
			await vi.runAllTimersAsync();

			expect(stop).toHaveBeenCalledTimes(1);
			expect(isIndicatorRunning).toBe(false);
		} finally {
			cleanup();
		}
	});

	it("avoids start-stop thrash during overlapping work with non-zero delays", async () => {
		const api = await loadPublicClientAPI();
		let isIndicatorRunning = false;
		const start = vi.fn(() => {
			isIndicatorRunning = true;
		});
		const stop = vi.fn(() => {
			isIndicatorRunning = false;
		});
		const firstDeferred = createDeferred<Response>();
		const secondDeferred = createDeferred<Response>();
		vi.spyOn(window, "fetch")
			.mockImplementationOnce(() => firstDeferred.promise)
			.mockImplementationOnce(() => secondDeferred.promise);
		const cleanup = api.setupGlobalLoadingIndicator({
			start,
			stop,
			isRunning: () => isIndicatorRunning,
			include: ["navigations"],
			startDelayMS: 5,
			stopDelayMS: 5,
		});
		try {
			const firstNavigatePromise = api.vormaNavigate(
				"/indicator-overlap-first",
			);
			await vi.advanceTimersByTimeAsync(13);
			expect(start).toHaveBeenCalledTimes(1);
			expect(isIndicatorRunning).toBe(true);

			const secondNavigatePromise = api.vormaNavigate(
				"/indicator-overlap-second",
			);

			firstDeferred.resolve(
				createRouteDataResponse({
					loadersData: [],
				}),
			);
			await firstNavigatePromise;
			await vi.advanceTimersByTimeAsync(20);

			expect(stop).not.toHaveBeenCalled();
			expect(start).toHaveBeenCalledTimes(1);
			expect(isIndicatorRunning).toBe(true);

			secondDeferred.resolve(
				createRouteDataResponse({
					loadersData: [],
				}),
			);
			await secondNavigatePromise;
			await vi.runAllTimersAsync();

			expect(start).toHaveBeenCalledTimes(1);
			expect(stop).toHaveBeenCalledTimes(1);
			expect(isIndicatorRunning).toBe(false);
		} finally {
			cleanup();
		}
	});

	it("keeps simultaneous indicator registrations isolated across cleanup and timers", async () => {
		const api = await loadPublicClientAPI();
		let firstRunning = false;
		let secondRunning = false;
		const firstStart = vi.fn(() => {
			firstRunning = true;
		});
		const firstStop = vi.fn(() => {
			firstRunning = false;
		});
		const secondStart = vi.fn(() => {
			secondRunning = true;
		});
		const secondStop = vi.fn(() => {
			secondRunning = false;
		});
		const navigationDeferred = createDeferred<Response>();
		vi.spyOn(window, "fetch").mockImplementation(
			() => navigationDeferred.promise,
		);

		const cleanupFirstIndicator = api.setupGlobalLoadingIndicator({
			start: firstStart,
			stop: firstStop,
			isRunning: () => firstRunning,
			include: ["navigations"],
			startDelayMS: 0,
			stopDelayMS: 0,
		});
		const cleanupSecondIndicator = api.setupGlobalLoadingIndicator({
			start: secondStart,
			stop: secondStop,
			isRunning: () => secondRunning,
			include: ["navigations"],
			startDelayMS: 50,
			stopDelayMS: 0,
		});
		try {
			const navigationPromise = api.vormaNavigate(
				"/indicator-two-instances",
			);
			await vi.advanceTimersByTimeAsync(8);
			await vi.advanceTimersByTimeAsync(1);

			expect(firstStart).toHaveBeenCalledTimes(1);
			expect(secondStart).not.toHaveBeenCalled();
			expect(firstRunning).toBe(true);
			expect(secondRunning).toBe(false);

			cleanupFirstIndicator();

			expect(firstStop).toHaveBeenCalledTimes(1);
			expect(firstRunning).toBe(false);

			await vi.advanceTimersByTimeAsync(50);

			expect(secondStart).toHaveBeenCalledTimes(1);
			expect(secondRunning).toBe(true);

			navigationDeferred.resolve(
				createRouteDataResponse({
					loadersData: [],
				}),
			);
			await navigationPromise;
			await vi.runAllTimersAsync();

			expect(firstStart).toHaveBeenCalledTimes(1);
			expect(firstStop).toHaveBeenCalledTimes(1);
			expect(secondStart).toHaveBeenCalledTimes(1);
			expect(secondStop).toHaveBeenCalledTimes(1);
			expect(secondRunning).toBe(false);
		} finally {
			cleanupSecondIndicator();
		}
	});

	it("respects navigation-only global loading indicator filters", async () => {
		const api = await loadPublicClientAPI();
		let isIndicatorRunning = false;
		const start = vi.fn(() => {
			isIndicatorRunning = true;
		});
		const stop = vi.fn(() => {
			isIndicatorRunning = false;
		});
		const cleanup = api.setupGlobalLoadingIndicator({
			start,
			stop,
			isRunning: () => isIndicatorRunning,
			include: ["navigations"],
			startDelayMS: 0,
			stopDelayMS: 0,
		});
		try {
			const navigationDeferred = createDeferred<Response>();
			vi.spyOn(window, "fetch")
				.mockImplementationOnce(() => navigationDeferred.promise)
				.mockImplementationOnce(() =>
					Promise.resolve(
						new Response(JSON.stringify({ ok: true }), {
							status: 200,
							headers: { "Content-Type": "application/json" },
						}),
					),
				);

			const navigationPromise = api.vormaNavigate("/indicator-nav-only");
			await Promise.resolve();
			await vi.advanceTimersByTimeAsync(1);
			expect(start).toHaveBeenCalledTimes(1);

			navigationDeferred.resolve(
				createRouteDataResponse({
					loadersData: [],
				}),
			);
			await navigationPromise;
			await vi.runAllTimersAsync();
			expect(stop).toHaveBeenCalledTimes(1);

			await api.submit(
				"/api/indicator-submit-ignored",
				{
					method: "POST",
				},
				{
					revalidate: false,
				},
			);
			await vi.runAllTimersAsync();
			expect(start).toHaveBeenCalledTimes(1);
		} finally {
			cleanup();
		}
	});

	it("respects revalidation-only global loading indicator filters", async () => {
		const api = await loadPublicClientAPI();
		let isIndicatorRunning = false;
		const start = vi.fn(() => {
			isIndicatorRunning = true;
		});
		const stop = vi.fn(() => {
			isIndicatorRunning = false;
		});
		const cleanup = api.setupGlobalLoadingIndicator({
			start,
			stop,
			isRunning: () => isIndicatorRunning,
			include: ["revalidations"],
			startDelayMS: 0,
			stopDelayMS: 0,
		});
		try {
			const revalidateDeferred = createDeferred<Response>();
			vi.spyOn(window, "fetch")
				.mockImplementationOnce(() => revalidateDeferred.promise)
				.mockImplementationOnce(() =>
					Promise.resolve(
						createRouteDataResponse({
							loadersData: [],
						}),
					),
				);

			const revalidatePromise = api.revalidate();
			await Promise.resolve();
			await vi.advanceTimersByTimeAsync(1);
			expect(start).toHaveBeenCalledTimes(1);

			revalidateDeferred.resolve(
				createRouteDataResponse({
					loadersData: [],
				}),
			);
			await revalidatePromise;
			await vi.runAllTimersAsync();
			expect(stop).toHaveBeenCalledTimes(1);

			await api.vormaNavigate("/indicator-nav-ignored");
			await vi.runAllTimersAsync();
			expect(start).toHaveBeenCalledTimes(1);
		} finally {
			cleanup();
		}
	});

	it("respects submitting-only global loading indicator filters", async () => {
		const api = await loadPublicClientAPI();
		let isIndicatorRunning = false;
		const start = vi.fn(() => {
			isIndicatorRunning = true;
		});
		const stop = vi.fn(() => {
			isIndicatorRunning = false;
		});
		const cleanup = api.setupGlobalLoadingIndicator({
			start,
			stop,
			isRunning: () => isIndicatorRunning,
			include: ["submissions"],
			startDelayMS: 0,
			stopDelayMS: 0,
		});
		try {
			const submitDeferred = createDeferred<Response>();
			vi.spyOn(window, "fetch")
				.mockImplementationOnce(() => submitDeferred.promise)
				.mockResolvedValueOnce(
					createRouteDataResponse({
						loadersData: [],
					}),
				);

			const submitPromise = api.submit(
				"/api/indicator-submit-only",
				{
					method: "POST",
				},
				{
					revalidate: false,
				},
			);
			await Promise.resolve();
			await vi.advanceTimersByTimeAsync(1);
			expect(start).toHaveBeenCalledTimes(1);

			submitDeferred.resolve(
				new Response(JSON.stringify({ ok: true }), {
					status: 200,
					headers: { "Content-Type": "application/json" },
				}),
			);
			await submitPromise;
			await vi.runAllTimersAsync();
			expect(stop).toHaveBeenCalledTimes(1);

			await api.vormaNavigate("/indicator-submit-only-nav-ignored");
			await vi.runAllTimersAsync();
			expect(start).toHaveBeenCalledTimes(1);
		} finally {
			cleanup();
		}
	});

	it("skips global loading indicator when navigate opts out", async () => {
		const api = await loadPublicClientAPI();
		let isIndicatorRunning = false;
		const start = vi.fn(() => {
			isIndicatorRunning = true;
		});
		const stop = vi.fn(() => {
			isIndicatorRunning = false;
		});
		const cleanup = api.setupGlobalLoadingIndicator({
			start,
			stop,
			isRunning: () => isIndicatorRunning,
			include: "all",
			startDelayMS: 0,
			stopDelayMS: 0,
		});
		try {
			const deferred = createDeferred<Response>();
			vi.spyOn(window, "fetch").mockImplementation(
				() => deferred.promise,
			);

			const navigatePromise = api.vormaNavigate("/indicator-skipped", {
				skipGlobalLoadingIndicator: true,
			});
			await Promise.resolve();
			await vi.advanceTimersByTimeAsync(1);

			expect(api.getStatus().isNavigating).toBe(true);
			expect(start).toHaveBeenCalledTimes(0);

			deferred.resolve(
				createRouteDataResponse({
					loadersData: [],
				}),
			);
			await navigatePromise;
			await vi.runAllTimersAsync();

			expect(start).toHaveBeenCalledTimes(0);
			expect(stop).toHaveBeenCalledTimes(0);
		} finally {
			cleanup();
		}
	});

	it("skips global loading indicator when submit opts out", async () => {
		const api = await loadPublicClientAPI();
		let isIndicatorRunning = false;
		const start = vi.fn(() => {
			isIndicatorRunning = true;
		});
		const stop = vi.fn(() => {
			isIndicatorRunning = false;
		});
		const cleanup = api.setupGlobalLoadingIndicator({
			start,
			stop,
			isRunning: () => isIndicatorRunning,
			include: "all",
			startDelayMS: 0,
			stopDelayMS: 0,
		});
		try {
			const deferred = createDeferred<Response>();
			vi.spyOn(window, "fetch").mockImplementation(
				() => deferred.promise,
			);

			const submitPromise = api.submit(
				"/api/indicator-submit-skipped",
				{
					method: "POST",
				},
				{
					skipGlobalLoadingIndicator: true,
					revalidate: false,
				},
			);
			await Promise.resolve();
			await vi.advanceTimersByTimeAsync(1);

			expect(api.getStatus().isSubmitting).toBe(false);
			expect(start).toHaveBeenCalledTimes(0);

			deferred.resolve(
				new Response(JSON.stringify({ ok: true }), {
					status: 200,
					headers: { "Content-Type": "application/json" },
				}),
			);
			await submitPromise;
			await vi.runAllTimersAsync();

			expect(start).toHaveBeenCalledTimes(0);
			expect(stop).toHaveBeenCalledTimes(0);
		} finally {
			cleanup();
		}
	});

	it("reports navigating status while navigation is in flight", async () => {
		const api = await loadPublicClientAPI();
		const deferred = createDeferred<Response>();
		vi.spyOn(window, "fetch").mockImplementation(() => deferred.promise);

		const navigationPromise = api.vormaNavigate("/status-navigating");
		await Promise.resolve();

		expect(api.getStatus()).toEqual({
			isNavigating: true,
			isSubmitting: false,
			isRevalidating: false,
		});

		deferred.resolve(
			createRouteDataResponse({
				loadersData: [],
			}),
		);

		await navigationPromise;
		await vi.runAllTimersAsync();
		expectStatusIdle(api.getStatus());
	});

	it("reports submitting status during submit without auto-revalidate", async () => {
		const api = await loadPublicClientAPI();
		const deferred = createDeferred<Response>();
		vi.spyOn(window, "fetch").mockImplementation(() => deferred.promise);

		const submitPromise = api.submit(
			"/api/status-submit-no-revalidate",
			{
				method: "POST",
			},
			{
				revalidate: false,
			},
		);
		await Promise.resolve();

		expect(api.getStatus()).toEqual({
			isNavigating: false,
			isSubmitting: true,
			isRevalidating: false,
		});

		deferred.resolve(
			new Response(JSON.stringify({ ok: true }), {
				status: 200,
				headers: {
					"Content-Type": "application/json",
				},
			}),
		);

		await submitPromise;
		await vi.runAllTimersAsync();
		expectStatusIdle(api.getStatus());
	});

	it("does not expose submitting status for hidden submissions", async () => {
		const api = await loadPublicClientAPI();
		const { statuses, cleanup } = collectStatusSnapshots(api);
		const deferred = createDeferred<Response>();
		vi.spyOn(window, "fetch").mockImplementation(() => deferred.promise);

		const submitPromise = api.submit(
			"/api/hidden-submit",
			{ method: "POST" },
			{ revalidate: false, skipGlobalLoadingIndicator: true },
		);
		await vi.advanceTimersByTimeAsync(8);

		expect(api.getStatus()).toEqual({
			isNavigating: false,
			isSubmitting: false,
			isRevalidating: false,
		});
		expect(statuses.some((status) => status.isSubmitting)).toBe(false);

		deferred.resolve(
			new Response(JSON.stringify({ ok: true }), {
				status: 200,
				headers: {
					"Content-Type": "application/json",
				},
			}),
		);
		await submitPromise;
		await vi.runAllTimersAsync();

		expectStatusIdle(api.getStatus());
		cleanup();
	});

	it("drops submitting status when only hidden submissions remain in-flight", async () => {
		const api = await loadPublicClientAPI();
		const hiddenDeferred = createDeferred<Response>();
		const visibleDeferred = createDeferred<Response>();
		vi.spyOn(window, "fetch")
			.mockImplementationOnce(() => hiddenDeferred.promise)
			.mockImplementationOnce(() => visibleDeferred.promise);

		const hiddenSubmit = api.submit(
			"/api/hidden",
			{ method: "POST" },
			{ revalidate: false, skipGlobalLoadingIndicator: true },
		);
		const visibleSubmit = api.submit(
			"/api/visible",
			{ method: "POST" },
			{ revalidate: false },
		);
		await vi.advanceTimersByTimeAsync(8);

		expect(api.getStatus()).toEqual({
			isNavigating: false,
			isSubmitting: true,
			isRevalidating: false,
		});

		visibleDeferred.resolve(
			new Response(JSON.stringify({ visible: true }), {
				status: 200,
				headers: {
					"Content-Type": "application/json",
				},
			}),
		);
		await visibleSubmit;
		await vi.runAllTimersAsync();

		expect(api.getStatus()).toEqual({
			isNavigating: false,
			isSubmitting: false,
			isRevalidating: false,
		});

		hiddenDeferred.resolve(
			new Response(JSON.stringify({ hidden: true }), {
				status: 200,
				headers: {
					"Content-Type": "application/json",
				},
			}),
		);
		await hiddenSubmit;
		await vi.runAllTimersAsync();

		expectStatusIdle(api.getStatus());
	});

	it("does not dispatch duplicate status events for identical status", async () => {
		const api = await loadPublicClientAPI();
		const statusListener = vi.fn();
		const cleanup = api.addStatusListener(statusListener);
		vi.spyOn(window, "fetch").mockImplementation(
			() => new Promise(() => {}),
		);

		void api.vormaNavigate("/same-a");
		await vi.advanceTimersByTimeAsync(8);
		const eventCount = statusListener.mock.calls.length;

		void api.vormaNavigate("/same-b");
		await vi.advanceTimersByTimeAsync(8);
		expect(statusListener).toHaveBeenCalledTimes(eventCount);
		cleanup();
	});

	it("debounces rapid status transitions into one event", async () => {
		const api = await loadPublicClientAPI();
		const statusListener = vi.fn();
		const cleanup = api.addStatusListener(statusListener);
		vi.spyOn(window, "fetch").mockImplementation(
			() => new Promise(() => {}),
		);
		try {
			void api.vormaNavigate("/debounce-a");
			void api.vormaNavigate("/debounce-b");

			await vi.advanceTimersByTimeAsync(8);
			expect(statusListener).toHaveBeenCalledTimes(1);
		} finally {
			cleanup();
		}
	});

	it("keeps submitting state continuous for overlapping submissions", async () => {
		const api = await loadPublicClientAPI();
		const { statuses, cleanup } = collectStatusSnapshots(api);
		const firstSubmitDeferred = createDeferred<Response>();
		const secondSubmitDeferred = createDeferred<Response>();
		vi.spyOn(window, "fetch")
			.mockImplementationOnce(() => firstSubmitDeferred.promise)
			.mockImplementationOnce(() => secondSubmitDeferred.promise);

		const firstSubmit = api.submit(
			"/api/first",
			{ method: "POST" },
			{ revalidate: false },
		);
		await vi.advanceTimersByTimeAsync(8);
		expect(api.getStatus().isSubmitting).toBe(true);

		const secondSubmit = api.submit(
			"/api/second",
			{ method: "POST" },
			{ revalidate: false },
		);

		firstSubmitDeferred.resolve(
			new Response(JSON.stringify({ ok: true }), {
				status: 200,
				headers: {
					"Content-Type": "application/json",
				},
			}),
		);
		await firstSubmit;
		await vi.advanceTimersByTimeAsync(8);
		expect(api.getStatus().isSubmitting).toBe(true);

		secondSubmitDeferred.resolve(
			new Response(JSON.stringify({ ok: true }), {
				status: 200,
				headers: {
					"Content-Type": "application/json",
				},
			}),
		);
		await secondSubmit;
		await vi.runAllTimersAsync();

		expectNoLoadingGapBeforeFinalEvent(statuses);
		expectStatusIdle(statuses.at(-1));
		cleanup();
	});

	it("keeps loading continuous through a complete navigation render cycle", async () => {
		const api = await loadPublicClientAPI();
		const { statuses, cleanup } = collectStatusSnapshots(api);
		let routeChangeCount = 0;
		const removeRouteChangeListener = api.addRouteChangeListener(() => {
			routeChangeCount += 1;
		});
		vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				matchedPatterns: ["/complex-page"],
				importURLs: [],
				exportKeys: [],
				errorExportKeys: [],
				loadersData: [],
				title: { dangerousInnerHTML: "Complex Page" },
			}),
		);

		expect(api.getStatus().isNavigating).toBe(false);
		const navigatePromise = api.vormaNavigate("/complex-page");
		expect(api.getStatus().isNavigating).toBe(true);

		await navigatePromise;
		await vi.runAllTimersAsync();

		expect(routeChangeCount).toBe(1);
		expectNoLoadingGapBeforeFinalEvent(statuses);
		expectStatusIdle(api.getStatus());
		removeRouteChangeListener();
		cleanup();
	});

	it("keeps navigating while waiting for css preload completion", async () => {
		const api = await loadPublicClientAPI();
		const { statuses, cleanup } = collectStatusSnapshots(api);
		try {
			const fetchDeferred = createDeferred<Response>();
			vi.spyOn(window, "fetch").mockImplementation(
				() => fetchDeferred.promise,
			);

			const navigatePromise = api.vormaNavigate("/styled-page");
			fetchDeferred.resolve(
				createRouteDataResponse({
					loadersData: [],
					cssBundles: ["/style1.css"],
				}),
			);

			await vi.advanceTimersByTimeAsync(10);
			expect(api.getStatus().isNavigating).toBe(true);
			const preloadLink = Array.from(
				document.head.querySelectorAll<HTMLLinkElement>(
					'link[rel="preload"][as="style"][data-vorma-css-preload-bundle]',
				),
			).find(
				(node) =>
					node.getAttribute("data-vorma-css-preload-bundle") ===
					"/style1.css",
			);
			expect(preloadLink).toBeDefined();

			preloadLink?.dispatchEvent(new Event("load"));
			await navigatePromise;
			await vi.runAllTimersAsync();

			expectNoLoadingGapBeforeFinalEvent(statuses);
			expect(api.getStatus().isNavigating).toBe(false);
		} finally {
			cleanup();
		}
	});

	it("continues successful navigation processing when css preload settles via error", async () => {
		const api = await loadPublicClientAPI();
		const { statuses, cleanup } = collectStatusSnapshots(api);
		try {
			const fetchDeferred = createDeferred<Response>();
			vi.spyOn(window, "fetch").mockImplementation(
				() => fetchDeferred.promise,
			);

			const navigatePromise = api.vormaNavigate("/styled-page-error");
			fetchDeferred.resolve(
				createRouteDataResponse({
					loadersData: [],
					cssBundles: ["/style-error.css"],
					title: {
						dangerousInnerHTML: "Styled Error Settle",
					},
				}),
			);

			await vi.advanceTimersByTimeAsync(10);
			expect(api.getStatus().isNavigating).toBe(true);
			const preloadLink = Array.from(
				document.head.querySelectorAll<HTMLLinkElement>(
					'link[rel="preload"][as="style"][data-vorma-css-preload-bundle]',
				),
			).find(
				(node) =>
					node.getAttribute("data-vorma-css-preload-bundle") ===
					"/style-error.css",
			);
			expect(preloadLink).toBeDefined();

			preloadLink?.dispatchEvent(new Event("error"));
			await navigatePromise;
			await vi.runAllTimersAsync();

			expectNoLoadingGapBeforeFinalEvent(statuses);
			expect(api.getStatus().isNavigating).toBe(false);
			expect(document.title).toBe("Styled Error Settle");
		} finally {
			cleanup();
		}
	});

	it("preloads unique deps as modulepreload links in production mode", async () => {
		const originalDEV = import.meta.env.DEV;
		(import.meta.env as AnyPropertyRecord).DEV = false;
		try {
			const api = await loadPublicClientAPI();
			vi.spyOn(window, "fetch").mockResolvedValue(
				createRouteDataResponse({
					loadersData: [],
					deps: ["/dep-a.js", "/dep-b.js", "/dep-a.js"],
				}),
			);

			await api.vormaNavigate("/with-deps");
			await vi.runAllTimersAsync();

			const modulePreloadLinks = Array.from(
				document.querySelectorAll<HTMLLinkElement>(
					'link[rel="modulepreload"]',
				),
			);
			expect(modulePreloadLinks).toHaveLength(2);
			const hrefs = modulePreloadLinks.map((link) =>
				link.getAttribute("href"),
			);
			expect(hrefs).toEqual(
				expect.arrayContaining(["/dep-a.js", "/dep-b.js"]),
			);
		} finally {
			(import.meta.env as AnyPropertyRecord).DEV = originalDEV;
		}
	});

	it("preloads css bundle assets during fetch route-data phase", async () => {
		const api = await loadPublicClientAPI();
		vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				loadersData: [],
				cssBundles: ["/a.css", "/b.css"],
			}),
		);

		const appendChild = document.head.appendChild.bind(document.head);
		vi.spyOn(document.head, "appendChild").mockImplementation((node) => {
			if (
				node instanceof HTMLLinkElement &&
				node.rel === "preload" &&
				node.getAttribute("as") === "style"
			) {
				void Promise.resolve().then(() =>
					node.dispatchEvent(new Event("load")),
				);
			}
			return appendChild(node);
		});

		await api.vormaNavigate("/with-css-bundles");
		await vi.runAllTimersAsync();

		const preloadLinks = Array.from(
			document.querySelectorAll<HTMLLinkElement>(
				'link[rel="preload"][as="style"]',
			),
		);
		expect(preloadLinks).toHaveLength(2);
		const hrefs = preloadLinks.map((link) => link.getAttribute("href"));
		expect(hrefs).toEqual(expect.arrayContaining(["/a.css", "/b.css"]));
	});

	it("uses backend-resolved css bundle hrefs as-is", async () => {
		const api = await loadPublicClientAPI();
		await api.initClient({
			publicPathPrefix: "/static/",
			renderFn: () => {},
		});
		vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				loadersData: [],
				cssBundles: ["/static/styles.css"],
			}),
		);

		const appendChild = document.head.appendChild.bind(document.head);
		vi.spyOn(document.head, "appendChild").mockImplementation((node) => {
			if (
				node instanceof HTMLLinkElement &&
				node.rel === "preload" &&
				node.getAttribute("as") === "style"
			) {
				void Promise.resolve().then(() =>
					node.dispatchEvent(new Event("load")),
				);
			}
			return appendChild(node);
		});

		await api.vormaNavigate("/with-stylesheet");
		await vi.runAllTimersAsync();

		const stylesheet = document.querySelector<HTMLLinkElement>(
			'link[rel="stylesheet"][data-vorma-css-bundle="/static/styles.css"]',
		);
		expect(stylesheet).toBeTruthy();
		expect(stylesheet?.getAttribute("href")).toBe("/static/styles.css");
	});

	it("resolves module imports from viteDevURL when present", async () => {
		const devComponent = () => null;
		vi.doMock("http://localhost:5173/dev-module.js", () => ({
			default: devComponent,
		}));
		const api = await loadPublicClientAPI();
		await api.initClient({
			viteDevURL: "http://localhost:5173",
			renderFn: () => {},
		});
		vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				importURLs: ["/dev-module.js"],
				exportKeys: ["default"],
				errorExportKeys: [""],
				matchedPatterns: ["/dev"],
				loadersData: [{}],
			}),
		);

		await expect(api.vormaNavigate("/dev-module")).resolves.toEqual({
			didNavigate: true,
		});
		await vi.runAllTimersAsync();
		expect(window.location.pathname).toBe("/dev-module");
		expectStatusIdle(api.getStatus());
	});

	it("keeps css bundle stylesheets after rest-head reconciliation clears managed rest blocks", async () => {
		const api = await loadPublicClientAPI();
		const existingStylesheet = document.createElement("link");
		existingStylesheet.setAttribute("rel", "stylesheet");
		existingStylesheet.setAttribute("data-vorma-css-bundle", "/keep.css");
		existingStylesheet.setAttribute("href", "/keep.css");
		insertHeadSectionElement({
			type: "rest",
			element: existingStylesheet,
		});

		vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				loadersData: [],
				cssBundles: ["/keep.css"],
				restHeadEls: [],
			}),
		);

		const appendChild = document.head.appendChild.bind(document.head);
		vi.spyOn(document.head, "appendChild").mockImplementation((node) => {
			if (
				node instanceof HTMLLinkElement &&
				node.rel === "preload" &&
				node.getAttribute("as") === "style"
			) {
				void Promise.resolve().then(() =>
					node.dispatchEvent(new Event("load")),
				);
			}
			return appendChild(node);
		});

		await api.vormaNavigate("/keep-css-bundle");
		await vi.runAllTimersAsync();

		expect(
			document.querySelector(
				'link[rel="stylesheet"][data-vorma-css-bundle="/keep.css"]',
			),
		).not.toBeNull();
		expect(
			getElementsBetweenHeadSectionMarkers({
				type: "rest",
			}),
		).toHaveLength(0);
	});

	it("deduplicates stylesheet application for repeated css bundles", async () => {
		const api = await loadPublicClientAPI();
		vi.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse({
					loadersData: [],
					cssBundles: ["/dup.css", "/dup.css"],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					loadersData: [],
					cssBundles: ["/dup.css"],
				}),
			);

		const appendChild = document.head.appendChild.bind(document.head);
		vi.spyOn(document.head, "appendChild").mockImplementation((node) => {
			if (
				node instanceof HTMLLinkElement &&
				node.rel === "preload" &&
				node.getAttribute("as") === "style"
			) {
				void Promise.resolve().then(() =>
					node.dispatchEvent(new Event("load")),
				);
			}
			return appendChild(node);
		});

		await api.vormaNavigate("/dup-css-a");
		await vi.runAllTimersAsync();
		await api.vormaNavigate("/dup-css-b");
		await vi.runAllTimersAsync();

		const stylesheets = document.querySelectorAll(
			'link[rel="stylesheet"][data-vorma-css-bundle="/dup.css"]',
		);
		expect(stylesheets).toHaveLength(1);
	});

	it("keeps navigating while waiting for client loader completion", async () => {
		const api = await loadPublicClientAPI();
		const { statuses, cleanup } = collectStatusSnapshots(api);
		const waitFnDeferred = createDeferred<{ clientData: string }>();
		vi.doMock("/data-page.js", () => ({
			default: () => null,
		}));
		registerClientLoaderForTesting({
			pattern: "/data-page",
			clientLoader: () => waitFnDeferred.promise,
		});
		try {
			const fetchDeferred = createDeferred<Response>();
			vi.spyOn(window, "fetch").mockImplementation(
				() => fetchDeferred.promise,
			);

			const navigatePromise = api.vormaNavigate("/data-page");
			fetchDeferred.resolve(
				createRouteDataResponse({
					matchedPatterns: ["/data-page"],
					importURLs: ["/data-page.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{}],
				}),
			);

			await vi.advanceTimersByTimeAsync(10);
			expect(api.getStatus().isNavigating).toBe(true);

			waitFnDeferred.resolve({ clientData: "loaded" });
			await navigatePromise;
			await vi.runAllTimersAsync();

			expectNoLoadingGapBeforeFinalEvent(statuses);
			expect(api.getStatus().isNavigating).toBe(false);
		} finally {
			cleanup();
		}
	});

	it("waits for client loader completion before dispatching route-change", async () => {
		const api = await loadPublicClientAPI();
		const clientLoaderDeferred = createDeferred<{ ready: boolean }>();
		vi.doMock("/route-change-client-loader.js", () => ({
			default: () => null,
		}));
		registerClientLoaderForTesting({
			pattern: "/route-change-client-loader",
			clientLoader: () => clientLoaderDeferred.promise,
		});
		const routeChangeListener = vi.fn();
		const removeRouteChangeListener =
			api.addRouteChangeListener(routeChangeListener);
		try {
			const fetchDeferred = createDeferred<Response>();
			vi.spyOn(window, "fetch").mockImplementation(
				() => fetchDeferred.promise,
			);

			const navigatePromise = api.vormaNavigate(
				"/route-change-client-loader",
			);
			fetchDeferred.resolve(
				createRouteDataResponse({
					matchedPatterns: ["/route-change-client-loader"],
					importURLs: ["/route-change-client-loader.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{}],
				}),
			);

			await vi.advanceTimersByTimeAsync(10);
			expect(routeChangeListener).toHaveBeenCalledTimes(0);

			clientLoaderDeferred.resolve({ ready: true });
			await navigatePromise;
			await vi.runAllTimersAsync();

			expect(routeChangeListener).toHaveBeenCalledTimes(1);
		} finally {
			removeRouteChangeListener();
		}
	});

	it("starts matched client loaders in parallel before waiting for settlement", async () => {
		const api = await loadPublicClientAPI();
		const firstLoaderDeferred = createDeferred<{ ready: boolean }>();
		const secondLoaderDeferred = createDeferred<{ ready: boolean }>();
		const startedPatterns: string[] = [];
		vi.doMock("/parallel-loader-a-module.js", () => ({
			default: () => null,
		}));
		vi.doMock("/parallel-loader-b-module.js", () => ({
			default: () => null,
		}));
		registerClientLoaderForTesting({
			pattern: "/parallel-loader-a",
			clientLoader: async () => {
				startedPatterns.push("/parallel-loader-a");
				return firstLoaderDeferred.promise;
			},
		});
		registerClientLoaderForTesting({
			pattern: "/parallel-loader-b",
			clientLoader: async () => {
				startedPatterns.push("/parallel-loader-b");
				return secondLoaderDeferred.promise;
			},
		});
		vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				matchedPatterns: ["/parallel-loader-a", "/parallel-loader-b"],
				importURLs: [
					"/parallel-loader-a-module.js",
					"/parallel-loader-b-module.js",
				],
				exportKeys: ["default", "default"],
				errorExportKeys: ["", ""],
				loadersData: [{ a: 1 }, { b: 2 }],
			}),
		);

		const navigatePromise = api.vormaNavigate("/parallel-client-loaders");
		await waitForRequestCount({
			requests: startedPatterns,
			count: 2,
		});
		expect(startedPatterns).toEqual(
			expect.arrayContaining([
				"/parallel-loader-a",
				"/parallel-loader-b",
			]),
		);

		firstLoaderDeferred.resolve({ ready: true });
		await vi.advanceTimersByTimeAsync(1);
		expect(api.getStatus().isNavigating).toBe(true);

		secondLoaderDeferred.resolve({ ready: true });
		await navigatePromise;
		await vi.runAllTimersAsync();

		expectStatusIdle(api.getStatus());
	});

	it("reuses running client-loader promises without duplicate invocations", async () => {
		const api = await loadPublicClientAPI();
		const loaderDeferred = createDeferred<{ ready: boolean }>();
		let clientLoaderInvocationCount = 0;
		vi.doMock("/reuse-running-client-loader-module.js", () => ({
			default: () => null,
		}));
		registerClientLoaderForTesting({
			pattern: "/reuse-running-client-loader",
			clientLoader: async () => {
				clientLoaderInvocationCount += 1;
				return loaderDeferred.promise;
			},
		});
		const fetchDeferred = createDeferred<Response>();
		vi.spyOn(window, "fetch").mockImplementation(
			() => fetchDeferred.promise,
		);

		const firstNavigation = api.vormaNavigate(
			"/reuse-running-client-loader#one",
		);
		await vi.advanceTimersByTimeAsync(5);

		fetchDeferred.resolve(
			createRouteDataResponse({
				matchedPatterns: ["/reuse-running-client-loader"],
				importURLs: ["/reuse-running-client-loader-module.js"],
				exportKeys: ["default"],
				errorExportKeys: [""],
				loadersData: [{ value: 1 }],
			}),
		);
		await vi.advanceTimersByTimeAsync(10);
		expect(clientLoaderInvocationCount).toBe(1);

		const secondNavigation = api.vormaNavigate(
			"/reuse-running-client-loader#two",
		);
		await vi.advanceTimersByTimeAsync(10);
		expect(clientLoaderInvocationCount).toBe(1);

		loaderDeferred.resolve({ ready: true });
		await Promise.all([firstNavigation, secondNavigation]);
		await vi.runAllTimersAsync();

		expectStatusIdle(api.getStatus());
		expect(window.location.pathname).toBe("/reuse-running-client-loader");
		expect(window.location.hash).toBe("#two");
	});

	it("passes matched server loader data into registered client loader functions", async () => {
		const api = await loadPublicClientAPI();
		let observedServerData: unknown = undefined;
		registerClientLoaderForTesting({
			pattern: "/matched-server-data",
			clientLoader: async (input: unknown) => {
				const inputRecord = input as {
					serverDataPromise: Promise<unknown>;
				};
				observedServerData = await inputRecord.serverDataPromise;
				return {
					fromClientLoader: true,
				};
			},
		});
		vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				matchedPatterns: ["/matched-server-data"],
				importURLs: [],
				exportKeys: [],
				errorExportKeys: [],
				loadersData: [{ server: "data" }],
			}),
		);

		await api.vormaNavigate("/matched-server-data");
		await vi.runAllTimersAsync();

		expect(observedServerData).toEqual({
			buildID: "1",
			matchedPatterns: ["/matched-server-data"],
			rootData: null,
			loaderData: { server: "data" },
		});
		expectStatusIdle(api.getStatus());
	});

	it("returns synchronous status snapshots from getStatus for in-flight navigation", async () => {
		const api = await loadPublicClientAPI();
		const deferred = createDeferred<Response>();
		vi.spyOn(window, "fetch").mockImplementation(() => deferred.promise);

		const navigationPromise = api.vormaNavigate("/status-sync");
		expect(api.getStatus()).toEqual({
			isNavigating: true,
			isSubmitting: false,
			isRevalidating: false,
		});

		deferred.resolve(
			createRouteDataResponse({
				loadersData: [],
			}),
		);

		await navigationPromise;
		await vi.runAllTimersAsync();
		expectStatusIdle(api.getStatus());
	});

	it("returns submit success payloads for successful JSON responses", async () => {
		const api = await loadPublicClientAPI();
		const deferred = createDeferred<Response>();
		vi.spyOn(window, "fetch").mockImplementation(() => deferred.promise);

		const submitPromise = api.submit<{ ok: boolean }>(
			"/api/submit-success",
			{
				method: "POST",
				headers: { "content-type": "application/json" },
				body: JSON.stringify({ value: 1 }),
			},
			{
				revalidate: false,
			},
		);
		await Promise.resolve();

		expect(api.getStatus()).toEqual({
			isNavigating: false,
			isSubmitting: true,
			isRevalidating: false,
		});

		deferred.resolve(
			new Response(JSON.stringify({ ok: true }), {
				status: 200,
				headers: { "Content-Type": "application/json" },
			}),
		);
		const result = await submitPromise;
		await vi.runAllTimersAsync();

		expect(result).toEqual({
			success: true,
			data: {
				ok: true,
			},
		});
		expectStatusIdle(api.getStatus());
	});

	it("deduplicates submissions with same dedupe key by aborting the first", async () => {
		const api = await loadPublicClientAPI();
		const firstDeferred = createDeferred<Response>();
		let firstSignal: AbortSignal | undefined;
		vi.spyOn(window, "fetch")
			.mockImplementationOnce((_input, init) => {
				firstSignal = (init as RequestInit | undefined)
					?.signal as AbortSignal;
				return firstDeferred.promise;
			})
			.mockResolvedValueOnce(
				new Response(JSON.stringify({ ok: true }), {
					status: 200,
					headers: { "Content-Type": "application/json" },
				}),
			);

		const firstSubmit = api.submit(
			"/api/dedupe",
			{ method: "POST" },
			{
				dedupeKey: "same-key",
				revalidate: false,
			},
		);
		const secondSubmit = api.submit(
			"/api/dedupe",
			{ method: "POST" },
			{
				dedupeKey: "same-key",
				revalidate: false,
			},
		);

		expect(firstSignal?.aborted).toBe(true);
		firstDeferred.reject(new DOMException("Aborted", "AbortError"));

		const [firstResult, secondResult] = await Promise.all([
			firstSubmit,
			secondSubmit,
		]);
		await vi.runAllTimersAsync();

		expect(firstResult).toEqual({ success: false, error: "Aborted" });
		expect(secondResult).toEqual({ success: true, data: { ok: true } });
		expectStatusIdle(api.getStatus());
	});

	it("keeps submitting state active through same-key dedupe handoff", async () => {
		const api = await loadPublicClientAPI();
		const { statuses, cleanup } = collectStatusSnapshots(api);
		const firstDeferred = createDeferred<Response>();
		const secondDeferred = createDeferred<Response>();
		vi.spyOn(window, "fetch")
			.mockImplementationOnce((_input, init) => {
				const signal = (init as RequestInit | undefined)
					?.signal as AbortSignal;
				signal?.addEventListener(
					"abort",
					() =>
						firstDeferred.reject(
							new DOMException("Aborted", "AbortError"),
						),
					{ once: true },
				);
				return firstDeferred.promise;
			})
			.mockImplementationOnce(() => secondDeferred.promise);

		try {
			const firstSubmit = api.submit(
				"/api/dedupe-status",
				{ method: "POST" },
				{
					dedupeKey: "same-key",
					revalidate: false,
				},
			);
			const secondSubmit = api.submit(
				"/api/dedupe-status",
				{ method: "POST" },
				{
					dedupeKey: "same-key",
					revalidate: false,
				},
			);

			const firstResult = await firstSubmit;
			expect(firstResult).toEqual({
				success: false,
				error: "Aborted",
			});

			secondDeferred.resolve(
				new Response(JSON.stringify({ ok: true }), {
					status: 200,
					headers: { "Content-Type": "application/json" },
				}),
			);
			const secondResult = await secondSubmit;
			await vi.runAllTimersAsync();

			expect(secondResult).toEqual({
				success: true,
				data: {
					ok: true,
				},
			});
			expect(statuses.some((status) => status.isSubmitting)).toBe(true);
			expectNoLoadingGapBeforeFinalEvent(statuses);
			expectStatusIdle(api.getStatus());
		} finally {
			cleanup();
		}
	});

	it("aborts stale deduped submits that are superseded during json parsing", async () => {
		const api = await loadPublicClientAPI();
		const firstJsonDeferred = createDeferred<{ ok: string }>();
		const firstResponse = new Response(
			JSON.stringify({
				ok: "stale",
			}),
			{
				status: 200,
				headers: { "Content-Type": "application/json" },
			},
		);
		vi.spyOn(firstResponse, "json").mockImplementation(
			() => firstJsonDeferred.promise,
		);
		vi.spyOn(window, "fetch")
			.mockResolvedValueOnce(firstResponse)
			.mockResolvedValueOnce(
				new Response(JSON.stringify({ ok: "fresh" }), {
					status: 200,
					headers: { "Content-Type": "application/json" },
				}),
			);

		const firstSubmit = api.submit(
			"/api/dedupe-json-parse",
			{ method: "POST" },
			{
				dedupeKey: "same-key",
				revalidate: false,
			},
		);
		await Promise.resolve();
		const secondSubmit = api.submit(
			"/api/dedupe-json-parse",
			{ method: "POST" },
			{
				dedupeKey: "same-key",
				revalidate: false,
			},
		);

		const secondResult = await secondSubmit;
		expect(secondResult).toEqual({
			success: true,
			data: {
				ok: "fresh",
			},
		});

		firstJsonDeferred.resolve({
			ok: "stale",
		});
		const firstResult = await firstSubmit;
		await vi.runAllTimersAsync();

		expect(firstResult).toEqual({
			success: false,
			error: "Aborted",
		});
		expectStatusIdle(api.getStatus());
	});

	it("clears submitting state when a deduped replacement submission fails", async () => {
		const api = await loadPublicClientAPI();
		const firstDeferred = createDeferred<Response>();
		vi.spyOn(window, "fetch")
			.mockImplementationOnce((_input, init) => {
				const signal = (init as RequestInit | undefined)
					?.signal as AbortSignal;
				signal?.addEventListener(
					"abort",
					() =>
						firstDeferred.reject(
							new DOMException("Aborted", "AbortError"),
						),
					{ once: true },
				);
				return firstDeferred.promise;
			})
			.mockResolvedValueOnce(
				new Response("replacement failed", {
					status: 500,
				}),
			);

		const firstSubmit = api.submit(
			"/api/dedupe-replacement-fail",
			{ method: "POST" },
			{
				dedupeKey: "same-key",
			},
		);
		const secondSubmit = api.submit(
			"/api/dedupe-replacement-fail",
			{ method: "POST" },
			{
				dedupeKey: "same-key",
			},
		);

		const [firstResult, secondResult] = await Promise.all([
			firstSubmit,
			secondSubmit,
		]);
		await vi.runAllTimersAsync();

		expect(firstResult).toEqual({
			success: false,
			error: "Aborted",
		});
		expect(secondResult).toEqual({
			success: false,
			error: "replacement failed",
		});
		expectStatusIdle(api.getStatus());
	});

	it("ignores late stale deduped submit redirect responses after replacement submission wins", async () => {
		const api = await loadPublicClientAPI();
		window.history.replaceState({}, "", "/dedupe-stale-redirect-base");
		const firstDeferred = createDeferred<Response>();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockImplementationOnce(() => firstDeferred.promise)
			.mockResolvedValueOnce(
				new Response(JSON.stringify({ ok: "winner" }), {
					status: 200,
					headers: { "Content-Type": "application/json" },
				}),
			);

		const firstSubmit = api.submit(
			"/api/dedupe-stale-redirect",
			{ method: "POST" },
			{
				dedupeKey: "same-key",
				revalidate: false,
			},
		);
		const secondSubmit = api.submit(
			"/api/dedupe-stale-redirect",
			{ method: "POST" },
			{
				dedupeKey: "same-key",
				revalidate: false,
			},
		);
		const secondResult = await secondSubmit;
		expect(secondResult).toEqual({
			success: true,
			data: {
				ok: "winner",
			},
		});

		firstDeferred.resolve(
			new Response("", {
				status: 200,
				headers: {
					"X-Client-Redirect": "/dedupe-stale-redirect-target",
				},
			}),
		);
		const firstResult = await firstSubmit;
		await vi.runAllTimersAsync();

		expect(firstResult).toEqual({
			success: false,
			error: "Aborted",
		});
		expect(fetchSpy).toHaveBeenCalledTimes(2);
		expect(window.location.pathname).toBe("/dedupe-stale-redirect-base");
		expectStatusIdle(api.getStatus());
	});

	it("does not allow late stale deduped submits to change build id or trigger hard reload", async () => {
		const api = await loadPublicClientAPI();
		const observedBuildIDs: string[] = [];
		const cleanupBuildIDListener = api.addBuildIDListener((event) => {
			observedBuildIDs.push(event.detail.newID);
		});
		const firstDeferred = createDeferred<Response>();
		vi.spyOn(window, "fetch")
			.mockImplementationOnce(() => firstDeferred.promise)
			.mockResolvedValueOnce(
				new Response(JSON.stringify({ ok: "winner" }), {
					status: 200,
					headers: { "Content-Type": "application/json" },
				}),
			);

		try {
			const firstSubmit = api.submit(
				"/api/dedupe-stale-build-id",
				{ method: "POST" },
				{
					dedupeKey: "same-key",
					revalidate: false,
				},
			);
			const secondSubmit = api.submit(
				"/api/dedupe-stale-build-id",
				{ method: "POST" },
				{
					dedupeKey: "same-key",
					revalidate: false,
				},
			);
			await secondSubmit;

			const locationHrefStub = stubWindowLocationHref();
			try {
				firstDeferred.resolve(
					new Response("", {
						status: 200,
						headers: {
							"X-Wave-Framework-Reload":
								"/dedupe-stale-hard-target",
							"X-Wave-Framework-Build-Id":
								"stale-dedupe-build-id",
						},
					}),
				);
				const firstResult = await firstSubmit;
				await vi.runAllTimersAsync();

				expect(firstResult).toEqual({
					success: false,
					error: "Aborted",
				});
				expect(api.getBuildID()).toBe("1");
				expect(observedBuildIDs).toEqual([]);
				expect(locationHrefStub.getHref()).not.toContain(
					"/dedupe-stale-hard-target",
				);
				expectStatusIdle(api.getStatus());
			} finally {
				locationHrefStub.restore();
			}
		} finally {
			cleanupBuildIDListener();
		}
	});

	it("does not trigger revalidation from a late stale deduped submit response", async () => {
		const api = await loadPublicClientAPI();
		const firstDeferred = createDeferred<Response>();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockImplementationOnce(() => firstDeferred.promise)
			.mockResolvedValueOnce(
				new Response(JSON.stringify({ ok: "winner" }), {
					status: 200,
					headers: { "Content-Type": "application/json" },
				}),
			);

		const firstSubmit = api.submit(
			"/api/dedupe-stale-revalidate",
			{ method: "POST" },
			{
				dedupeKey: "same-key",
			},
		);
		const secondSubmit = api.submit(
			"/api/dedupe-stale-revalidate",
			{ method: "POST" },
			{
				dedupeKey: "same-key",
				revalidate: false,
			},
		);
		await secondSubmit;

		firstDeferred.resolve(
			new Response(JSON.stringify({ ok: "stale" }), {
				status: 200,
				headers: { "Content-Type": "application/json" },
			}),
		);
		const firstResult = await firstSubmit;
		await vi.runAllTimersAsync();

		expect(firstResult).toEqual({
			success: false,
			error: "Aborted",
		});
		expect(fetchSpy).toHaveBeenCalledTimes(2);
		expectStatusIdle(api.getStatus());
	});

	it("preserves BodyInit payloads and serializes object bodies for submit requests", async () => {
		const api = await loadPublicClientAPI();
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			new Response(JSON.stringify({ ok: true }), {
				status: 200,
				headers: {
					"Content-Type": "application/json",
				},
			}),
		);
		const urlSearchParamsBody = new URLSearchParams({
			a: "1",
		});

		await api.submit(
			"/api/submit-body-preserve",
			{
				method: "POST",
				body: urlSearchParamsBody,
			},
			{
				revalidate: false,
			},
		);
		await api.submit(
			"/api/submit-body-object",
			{
				method: "POST",
				body: {
					a: 1,
				} as any,
			},
			{
				revalidate: false,
			},
		);
		await vi.runAllTimersAsync();

		const firstSubmitInit = fetchSpy.mock.calls[0]?.[1] as RequestInit;
		expect(firstSubmitInit.body).toBe(urlSearchParamsBody);

		const secondSubmitInit = fetchSpy.mock.calls[1]?.[1] as RequestInit;
		expect(secondSubmitInit.body).toBe(
			JSON.stringify({
				a: 1,
			}),
		);
		expect(
			new Headers(secondSubmitInit.headers).get("content-type"),
		).toContain("application/json");
		expectStatusIdle(api.getStatus());
	});

	it("passes through ReadableStream, Blob, ArrayBuffer, and ArrayBufferView submit bodies for non-GET methods", async () => {
		const api = await loadPublicClientAPI();
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			new Response(JSON.stringify({ ok: true }), {
				status: 200,
				headers: {
					"Content-Type": "application/json",
				},
			}),
		);
		const passthroughBodies: BodyInit[] = [];
		if (typeof ReadableStream === "function") {
			const readableStreamBody = new ReadableStream({
				start(controller) {
					controller.close();
				},
			});
			await api.submit(
				"/api/submit-body-readable-stream",
				{
					method: "POST",
					body: readableStreamBody,
				},
				{
					revalidate: false,
				},
			);
			passthroughBodies.push(readableStreamBody);
		}
		const blobBody = new Blob(["blob-body"], {
			type: "text/plain",
		});
		const arrayBufferBody = new ArrayBuffer(16);
		const arrayBufferViewBody = new Uint8Array([1, 2, 3, 4]);

		await api.submit(
			"/api/submit-body-blob",
			{
				method: "POST",
				body: blobBody,
			},
			{
				revalidate: false,
			},
		);
		await api.submit(
			"/api/submit-body-array-buffer",
			{
				method: "POST",
				body: arrayBufferBody,
			},
			{
				revalidate: false,
			},
		);
		await api.submit(
			"/api/submit-body-array-buffer-view",
			{
				method: "POST",
				body: arrayBufferViewBody,
			},
			{
				revalidate: false,
			},
		);
		passthroughBodies.push(blobBody, arrayBufferBody, arrayBufferViewBody);
		await vi.runAllTimersAsync();

		const submitInits = fetchSpy.mock.calls.map((call) => {
			return call[1] as RequestInit;
		});
		expect(submitInits).toHaveLength(passthroughBodies.length);
		passthroughBodies.forEach((expectedBody, index) => {
			const submitInit = submitInits[index] as RequestInit;
			expect(submitInit.body).toBe(expectedBody);
		});
	});

	it("omits null bodies for non-GET submit methods", async () => {
		const api = await loadPublicClientAPI();
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			new Response(JSON.stringify({ ok: true }), {
				status: 200,
				headers: {
					"Content-Type": "application/json",
				},
			}),
		);

		await api.submit(
			"/api/submit-null-body",
			{
				method: "POST",
				body: null,
			},
			{
				revalidate: false,
			},
		);
		await vi.runAllTimersAsync();

		const submitInit = fetchSpy.mock.calls[0]?.[1] as RequestInit;
		expect(submitInit.body).toBeNull();
	});

	it("keeps caller-provided content type when serializing object submit bodies", async () => {
		const api = await loadPublicClientAPI();
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			new Response(JSON.stringify({ ok: true }), {
				status: 200,
				headers: {
					"Content-Type": "application/json",
				},
			}),
		);

		await api.submit(
			"/api/submit-custom-content-type",
			{
				method: "POST",
				body: { a: 1 } as any,
				headers: {
					"Content-Type": "application/merge-patch+json",
				},
			},
			{
				revalidate: false,
			},
		);
		await vi.runAllTimersAsync();

		const submitInit = fetchSpy.mock.calls[0]?.[1] as RequestInit;
		expect(new Headers(submitInit.headers).get("content-type")).toBe(
			"application/merge-patch+json",
		);
		expect(submitInit.body).toBe(
			JSON.stringify({
				a: 1,
			}),
		);
	});

	it("does not deduplicate submissions with different dedupe keys", async () => {
		const api = await loadPublicClientAPI();
		const firstDeferred = createDeferred<Response>();
		let firstSignal: AbortSignal | undefined;
		vi.spyOn(window, "fetch")
			.mockImplementationOnce((_input, init) => {
				firstSignal = (init as RequestInit | undefined)
					?.signal as AbortSignal;
				return firstDeferred.promise;
			})
			.mockResolvedValueOnce(
				new Response(JSON.stringify({ ok: "second" }), {
					status: 200,
					headers: { "Content-Type": "application/json" },
				}),
			);

		const firstSubmit = api.submit(
			"/api/dedupe-different",
			{ method: "POST" },
			{
				dedupeKey: "A",
				revalidate: false,
			},
		);
		const secondSubmit = api.submit(
			"/api/dedupe-different",
			{ method: "POST" },
			{
				dedupeKey: "B",
				revalidate: false,
			},
		);

		expect(firstSignal?.aborted).toBe(false);
		firstDeferred.resolve(
			new Response(JSON.stringify({ ok: "first" }), {
				status: 200,
				headers: { "Content-Type": "application/json" },
			}),
		);

		const [firstResult, secondResult] = await Promise.all([
			firstSubmit,
			secondSubmit,
		]);
		await vi.runAllTimersAsync();

		expect(firstResult).toEqual({
			success: true,
			data: {
				ok: "first",
			},
		});
		expect(secondResult).toEqual({
			success: true,
			data: {
				ok: "second",
			},
		});
		expectStatusIdle(api.getStatus());
	});

	it("drops stale redirect side effects when newer different-key submit completes first", async () => {
		const api = await loadPublicClientAPI();
		window.history.replaceState({}, "", "/submit-stale-base");
		const firstDeferred = createDeferred<Response>();
		vi.spyOn(window, "fetch")
			.mockImplementationOnce(() => firstDeferred.promise)
			.mockResolvedValueOnce(
				new Response(JSON.stringify({ ok: "newer" }), {
					status: 200,
					headers: { "Content-Type": "application/json" },
				}),
			);

		const firstSubmit = api.submit(
			"/api/submit-stale-first",
			{ method: "POST" },
			{
				dedupeKey: "first",
				revalidate: false,
			},
		);
		const secondSubmit = api.submit(
			"/api/submit-stale-second",
			{ method: "POST" },
			{
				dedupeKey: "second",
				revalidate: false,
			},
		);

		const secondResult = await secondSubmit;
		expect(secondResult).toEqual({
			success: true,
			data: {
				ok: "newer",
			},
		});

		firstDeferred.resolve(
			new Response("", {
				status: 200,
				headers: {
					"X-Client-Redirect": "/submit-stale-redirect",
				},
			}),
		);

		const firstResult = await firstSubmit;
		await vi.runAllTimersAsync();

		expect(firstResult).toEqual({
			success: true,
			data: undefined,
		});
		expect(window.location.pathname).toBe("/submit-stale-base");
		expectStatusIdle(api.getStatus());
	});

	it("drops stale hard-reload/build-id submit side effects when newer different-key submit completes first", async () => {
		const api = await loadPublicClientAPI();
		const observedBuildIDs: string[] = [];
		const cleanupBuildIDListener = api.addBuildIDListener((event) => {
			observedBuildIDs.push(event.detail.newID);
		});
		const firstDeferred = createDeferred<Response>();
		vi.spyOn(window, "fetch")
			.mockImplementationOnce(() => firstDeferred.promise)
			.mockResolvedValueOnce(
				new Response(JSON.stringify({ ok: "newer" }), {
					status: 200,
					headers: { "Content-Type": "application/json" },
				}),
			);

		try {
			const firstSubmit = api.submit(
				"/api/submit-stale-hard-first",
				{ method: "POST" },
				{
					dedupeKey: "first",
					revalidate: false,
				},
			);
			const secondSubmit = api.submit(
				"/api/submit-stale-hard-second",
				{ method: "POST" },
				{
					dedupeKey: "second",
					revalidate: false,
				},
			);

			const secondResult = await secondSubmit;
			expect(secondResult).toEqual({
				success: true,
				data: {
					ok: "newer",
				},
			});

			const locationHrefStub = stubWindowLocationHref();
			try {
				firstDeferred.resolve(
					new Response("", {
						status: 200,
						headers: {
							"X-Wave-Framework-Reload":
								"/submit-stale-hard-reload-target",
							"X-Wave-Framework-Build-Id":
								"stale-submit-build-id",
						},
					}),
				);

				const firstResult = await firstSubmit;
				await vi.runAllTimersAsync();

				expect(firstResult).toEqual({
					success: true,
					data: undefined,
				});
				expect(locationHrefStub.getHref()).not.toContain(
					"/submit-stale-hard-reload-target",
				);
				expect(api.getBuildID()).toBe("1");
				expect(observedBuildIDs).toEqual([]);
				expectStatusIdle(api.getStatus());
			} finally {
				locationHrefStub.restore();
			}
		} finally {
			cleanupBuildIDListener();
		}
	});

	it("ignores stale submit-redirect side effects when a newer user navigation commits first", async () => {
		const api = await loadPublicClientAPI();
		const { requests } = createAbortAwareFetchRecorder();
		const observedBuildIDs: string[] = [];
		const cleanupBuildIDListener = api.addBuildIDListener((event) => {
			observedBuildIDs.push(event.detail.newID);
		});
		try {
			const submitPromise = api.submit("/api/submit-redirect", {
				method: "POST",
			});

			await waitForRequestCount({ requests, count: 1 });
			requests[0]!.resolve(
				createRouteDataResponse(
					{},
					{
						headers: {
							"X-Client-Redirect": "/submit-redirect-target",
						},
					},
				),
			);
			await waitForRequestCount({ requests, count: 2 });

			const winnerNavigationPromise = api.vormaNavigate(
				"/submit-redirect-winner",
			);
			await waitForRequestCount({ requests, count: 3 });
			requests[2]!.resolve(
				createRouteDataResponse(
					{
						loadersData: [],
						title: {
							dangerousInnerHTML: "User Navigation Winner",
						},
					},
					{
						headers: {
							"X-Wave-Framework-Build-Id":
								"user-navigation-winner-build",
						},
					},
				),
			);
			await winnerNavigationPromise;

			requests[1]!.resolve(
				createRouteDataResponse(
					{
						loadersData: [],
						title: {
							dangerousInnerHTML: "Submit Redirect Stale",
						},
					},
					{
						headers: {
							"X-Wave-Framework-Build-Id":
								"submit-redirect-stale-build",
						},
					},
				),
			);

			const submitResult = await submitPromise;
			await vi.runAllTimersAsync();

			expect(submitResult).toEqual({
				success: true,
				data: undefined,
			});
			expect(window.location.pathname).toBe("/submit-redirect-winner");
			expect(document.title).toBe("User Navigation Winner");
			expect(api.getBuildID()).toBe("user-navigation-winner-build");
			expect(observedBuildIDs).not.toContain(
				"submit-redirect-stale-build",
			);
			expectStatusIdle(api.getStatus());
		} finally {
			cleanupBuildIDListener();
		}
	});

	it("does not deduplicate submissions when no dedupe key is provided", async () => {
		const api = await loadPublicClientAPI();
		const firstDeferred = createDeferred<Response>();
		let firstSignal: AbortSignal | undefined;
		vi.spyOn(window, "fetch")
			.mockImplementationOnce((_input, init) => {
				firstSignal = (init as RequestInit | undefined)
					?.signal as AbortSignal;
				return firstDeferred.promise;
			})
			.mockResolvedValueOnce(
				new Response(JSON.stringify({ ok: "second" }), {
					status: 200,
					headers: { "Content-Type": "application/json" },
				}),
			);

		const firstSubmit = api.submit(
			"/api/no-dedupe",
			{ method: "POST" },
			{
				revalidate: false,
			},
		);
		const secondSubmit = api.submit(
			"/api/no-dedupe",
			{ method: "POST" },
			{
				revalidate: false,
			},
		);

		expect(firstSignal?.aborted).toBe(false);
		firstDeferred.resolve(
			new Response(JSON.stringify({ ok: "first" }), {
				status: 200,
				headers: { "Content-Type": "application/json" },
			}),
		);

		const [firstResult, secondResult] = await Promise.all([
			firstSubmit,
			secondSubmit,
		]);
		await vi.runAllTimersAsync();

		expect(firstResult).toEqual({
			success: true,
			data: {
				ok: "first",
			},
		});
		expect(secondResult).toEqual({
			success: true,
			data: {
				ok: "second",
			},
		});
		expectStatusIdle(api.getStatus());
	});

	it("returns submit failure payloads for non-ok responses", async () => {
		const api = await loadPublicClientAPI();
		vi.spyOn(window, "fetch").mockResolvedValue(
			new Response("boom", {
				status: 500,
				headers: { "Content-Type": "text/plain" },
			}),
		);

		const result = await api.submit(
			"/api/submit-failure",
			{
				method: "POST",
				body: JSON.stringify({ value: 2 }),
			},
			{
				revalidate: false,
			},
		);
		await vi.runAllTimersAsync();

		expect(result.success).toBe(false);
		if (result.success) {
			throw new Error("Expected submit failure result.");
		}
		expect(result.error).toContain("boom");
		expectStatusIdle(api.getStatus());
	});

	it("returns submit failure results for status and thrown error paths", async () => {
		const api = await loadPublicClientAPI();
		const fetchSpy = vi.spyOn(window, "fetch");

		fetchSpy.mockResolvedValueOnce(
			new Response(null, {
				status: 500,
				statusText: "Internal Server Error",
			}),
		);
		const statusFailureResult = await api.submit(
			"/api/status-failure",
			{
				method: "POST",
			},
			{
				revalidate: false,
			},
		);
		expect(statusFailureResult.success).toBe(false);
		if (statusFailureResult.success) {
			throw new Error("Expected submit status failure result.");
		}
		expect(statusFailureResult.error).toContain("500");

		fetchSpy.mockResolvedValueOnce(
			new Response("Internal Server Error", {
				status: 500,
				statusText: "Internal Server Error",
			}),
		);
		const genericStatusTextBodyFailureResult = await api.submit(
			"/api/generic-status-text-body-failure",
			{
				method: "POST",
			},
			{
				revalidate: false,
			},
		);
		expect(genericStatusTextBodyFailureResult).toEqual({
			success: false,
			error: "500",
		});

		fetchSpy.mockRejectedValueOnce(new Error("Network failure"));
		const networkFailureResult = await api.submit(
			"/api/network-failure",
			{
				method: "POST",
			},
			{
				revalidate: false,
			},
		);
		expect(networkFailureResult).toEqual({
			success: false,
			error: "Network failure",
		});

		fetchSpy.mockRejectedValueOnce("not-an-error-object");
		const thrownPrimitiveResult = await api.submit(
			"/api/thrown-primitive",
			{
				method: "POST",
			},
			{
				revalidate: false,
			},
		);
		expect(thrownPrimitiveResult).toEqual({
			success: false,
			error: "not-an-error-object",
		});

		const abortError = new Error("Aborted");
		abortError.name = "AbortError";
		fetchSpy.mockRejectedValueOnce(abortError);
		const abortResult = await api.submit(
			"/api/aborted-submit",
			{
				method: "POST",
			},
			{
				revalidate: false,
			},
		);
		expect(abortResult).toEqual({
			success: false,
			error: "Aborted",
		});
		expectStatusIdle(api.getStatus());
	});

	it("auto-revalidates after non-GET submissions by default", async () => {
		const api = await loadPublicClientAPI();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				new Response(JSON.stringify({ ok: true }), {
					status: 200,
					headers: { "Content-Type": "application/json" },
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					loadersData: [],
				}),
			);

		const result = await api.submit<{ ok: boolean }>(
			"/api/submit-default-revalidate",
			{
				method: "POST",
				body: JSON.stringify({ value: 9 }),
			},
		);
		await vi.runAllTimersAsync();

		expect(result).toEqual({
			success: true,
			data: {
				ok: true,
			},
		});
		expect(fetchSpy).toHaveBeenCalledTimes(2);
	});

	it("does not auto-revalidate after GET submissions", async () => {
		const api = await loadPublicClientAPI();
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			new Response(JSON.stringify({ ok: true }), {
				status: 200,
				headers: { "Content-Type": "application/json" },
			}),
		);

		const result = await api.submit<{ ok: boolean }>(
			"/api/submit-get-no-revalidate",
			{
				method: "GET",
			},
		);
		await vi.runAllTimersAsync();

		expect(result).toEqual({
			success: true,
			data: {
				ok: true,
			},
		});
		expect(fetchSpy).toHaveBeenCalledTimes(1);
	});

	it("omits request body for GET, HEAD, and implicit-GET submit calls", async () => {
		const api = await loadPublicClientAPI();
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			new Response(JSON.stringify({ ok: true }), {
				status: 200,
				headers: { "Content-Type": "application/json" },
			}),
		);

		await api.submit("/api/get-with-body", {
			method: "GET",
			body: "should-be-omitted",
		});
		await api.submit("/api/head-with-body", {
			method: "HEAD",
			body: "should-be-omitted",
		});
		await api.submit("/api/implicit-get-with-body", {
			body: "should-be-omitted",
		});
		await vi.runAllTimersAsync();

		expect(fetchSpy).toHaveBeenCalledTimes(3);
		const firstInit = fetchSpy.mock.calls[0]?.[1] as RequestInit;
		const secondInit = fetchSpy.mock.calls[1]?.[1] as RequestInit;
		const thirdInit = fetchSpy.mock.calls[2]?.[1] as RequestInit;
		expect(firstInit.body).toBeUndefined();
		expect(secondInit.body).toBeUndefined();
		expect(thirdInit.body).toBeUndefined();
	});

	it("returns text body for successful non-json submit responses", async () => {
		const api = await loadPublicClientAPI();
		vi.spyOn(window, "fetch").mockResolvedValue(
			new Response("plain-text-ok", {
				status: 200,
				headers: { "Content-Type": "text/plain" },
			}),
		);

		const result = await api.submit<string>(
			"/api/submit-text",
			{
				method: "POST",
				body: "hello",
			},
			{
				revalidate: false,
			},
		);
		await vi.runAllTimersAsync();

		expect(result).toEqual({
			success: true,
			data: "plain-text-ok",
		});
	});

	it("returns text data for successful submit responses without content-type", async () => {
		const api = await loadPublicClientAPI();
		const noContentTypeResponse = new Response(
			new Uint8Array([112, 108, 97, 105, 110, 45, 111, 107]),
			{
				status: 200,
			},
		);
		expect(noContentTypeResponse.headers.get("Content-Type")).toBeNull();
		vi.spyOn(window, "fetch").mockResolvedValue(noContentTypeResponse);

		const result = await api.submit<string>(
			"/api/submit-text-no-content-type",
			{
				method: "POST",
				body: "hello",
			},
			{
				revalidate: false,
			},
		);
		await vi.runAllTimersAsync();

		expect(result).toEqual({
			success: true,
			data: "plain-ok",
		});
	});

	it("returns success with undefined data for 204 submit responses", async () => {
		const api = await loadPublicClientAPI();
		vi.spyOn(window, "fetch").mockResolvedValue(
			new Response(null, {
				status: 204,
			}),
		);

		const result = await api.submit(
			"/api/submit-no-content",
			{
				method: "POST",
			},
			{
				revalidate: false,
			},
		);
		await vi.runAllTimersAsync();

		expect(result).toEqual({
			success: true,
			data: undefined,
		});
	});

	it("returns success with undefined data for null-body 200 submit responses without content type", async () => {
		const api = await loadPublicClientAPI();
		vi.spyOn(window, "fetch").mockResolvedValue(
			new Response(null, {
				status: 200,
			}),
		);

		const result = await api.submit(
			"/api/submit-null-body-200",
			{
				method: "POST",
			},
			{
				revalidate: false,
			},
		);
		await vi.runAllTimersAsync();

		expect(result).toEqual({
			success: true,
			data: undefined,
		});
		expectStatusIdle(api.getStatus());
	});

	it("keeps loading continuous from submit into auto-revalidate", async () => {
		const api = await loadPublicClientAPI();
		const { statuses, cleanup } = collectStatusSnapshots(api);
		try {
			vi.spyOn(window, "fetch")
				.mockResolvedValueOnce(
					new Response(JSON.stringify({ ok: true }), {
						status: 200,
						headers: { "Content-Type": "application/json" },
					}),
				)
				.mockResolvedValueOnce(
					createRouteDataResponse({
						loadersData: [],
						title: {
							dangerousInnerHTML: "After Submit Revalidate",
						},
					}),
				);

			const result = await api.submit<{ ok: boolean }>(
				"/api/submit-with-revalidate",
				{
					method: "POST",
					body: JSON.stringify({ value: 3 }),
				},
				{
					revalidate: true,
				},
			);
			await vi.runAllTimersAsync();

			expect(result).toEqual({
				success: true,
				data: {
					ok: true,
				},
			});
			expect(statuses.some((status) => status.isSubmitting)).toBe(true);
			expect(statuses.some((status) => status.isRevalidating)).toBe(true);
			expectNoLoadingGapBeforeFinalEvent(statuses);
			expect(statuses.at(-1)).toEqual({
				isNavigating: false,
				isSubmitting: false,
				isRevalidating: false,
			});
		} finally {
			cleanup();
		}
	});

	it("dispatches build-id updates with old/new values after navigation commit", async () => {
		const api = await loadPublicClientAPI();
		const buildIDEvents: Array<{ oldID: string; newID: string }> = [];
		const cleanup = api.addBuildIDListener((event) => {
			buildIDEvents.push(event.detail);
		});
		try {
			vi.spyOn(window, "fetch").mockResolvedValue(
				createRouteDataResponse(
					{
						loadersData: [],
					},
					{
						headers: {
							"X-Wave-Framework-Build-Id": "build-2",
						},
					},
				),
			);

			await api.vormaNavigate("/buildid-next");
			await vi.runAllTimersAsync();

			expect(api.getBuildID()).toBe("build-2");
			expect(buildIDEvents).toEqual([
				{
					oldID: "1",
					newID: "build-2",
				},
			]);
		} finally {
			cleanup();
		}
	});

	it("syncs build ID from responses only when the value changes", async () => {
		const api = await loadPublicClientAPI();
		const buildIDEvents: Array<{ oldID: string; newID: string }> = [];
		const cleanup = api.addBuildIDListener((event) => {
			buildIDEvents.push({
				oldID: event.detail.oldID,
				newID: event.detail.newID,
			});
		});
		try {
			vi.spyOn(window, "fetch")
				.mockResolvedValueOnce(
					createRouteDataResponse(
						{
							loadersData: [],
						},
						{
							headers: {
								"X-Wave-Framework-Build-Id": "1",
							},
						},
					),
				)
				.mockResolvedValueOnce(
					createRouteDataResponse(
						{
							loadersData: [],
						},
						{
							headers: {
								"X-Wave-Framework-Build-Id": "build-2",
							},
						},
					),
				);

			await api.vormaNavigate("/build-id-noop");
			await vi.runAllTimersAsync();
			await api.vormaNavigate("/build-id-change");
			await vi.runAllTimersAsync();

			expect(buildIDEvents).toEqual([
				{
					oldID: "1",
					newID: "build-2",
				},
			]);
			expect(api.getBuildID()).toBe("build-2");
		} finally {
			cleanup();
		}
	});

	it("updates getBuildID before build-id listeners observe the event", async () => {
		const api = await loadPublicClientAPI();
		const observedInListener: Array<{
			currentBuildID: string;
			eventBuildID: string;
		}> = [];
		const cleanup = api.addBuildIDListener((event) => {
			observedInListener.push({
				currentBuildID: api.getBuildID(),
				eventBuildID: event.detail.newID,
			});
		});
		try {
			vi.spyOn(window, "fetch").mockResolvedValue(
				createRouteDataResponse(
					{
						loadersData: [],
					},
					{
						headers: {
							"X-Wave-Framework-Build-Id": "build-9",
						},
					},
				),
			);

			await api.vormaNavigate("/buildid-order");
			await vi.runAllTimersAsync();

			expect(observedInListener).toEqual([
				{
					currentBuildID: "build-9",
					eventBuildID: "build-9",
				},
			]);
		} finally {
			cleanup();
		}
	});

	it("stops build-id event delivery after listener cleanup", async () => {
		const api = await loadPublicClientAPI();
		const events: Array<{ oldID: string; newID: string }> = [];
		const cleanup = api.addBuildIDListener((event) => {
			events.push(event.detail);
		});

		vi.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse(
					{
						loadersData: [],
					},
					{
						headers: {
							"X-Wave-Framework-Build-Id": "listener-build-a",
						},
					},
				),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse(
					{
						loadersData: [],
					},
					{
						headers: {
							"X-Wave-Framework-Build-Id": "listener-build-b",
						},
					},
				),
			);

		await api.vormaNavigate("/listener-build-a");
		await vi.runAllTimersAsync();
		expect(events).toHaveLength(1);

		cleanup();
		const eventCountAfterCleanup = events.length;

		await api.vormaNavigate("/listener-build-b");
		await vi.runAllTimersAsync();
		expect(events).toHaveLength(eventCountAfterCleanup);
	});

	it("stops status event delivery after listener cleanup", async () => {
		const api = await loadPublicClientAPI();
		const statuses: Array<StatusSnapshot> = [];
		const cleanup = api.addStatusListener((event) => {
			statuses.push(event.detail);
		});

		vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				loadersData: [],
			}),
		);

		await api.vormaNavigate("/event-listener-a");
		await vi.runAllTimersAsync();

		expect(statuses.length).toBeGreaterThan(0);
		expect(statuses.at(-1)).toEqual({
			isNavigating: false,
			isSubmitting: false,
			isRevalidating: false,
		});

		cleanup();
		const eventCountAfterCleanup = statuses.length;

		vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				loadersData: [],
			}),
		);

		await api.vormaNavigate("/event-listener-b");
		await vi.runAllTimersAsync();

		expect(statuses).toHaveLength(eventCountAfterCleanup);
	});

	it("returns current location parts from getLocation", async () => {
		const api = await loadPublicClientAPI();
		window.history.replaceState({ from: "state" }, "", "/loc?q=1#frag");
		expect(api.getLocation()).toEqual({
			pathname: "/loc",
			search: "?q=1",
			hash: "#frag",
			state: null,
		});
	});

	it("returns current build ID from getBuildID", async () => {
		const api = await loadPublicClientAPI();
		void api.getStatus();
		expect(api.getBuildID()).toBe("1");

		vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse(
				{
					loadersData: [],
				},
				{
					headers: {
						"X-Wave-Framework-Build-Id": "build-id-getter-check",
					},
				},
			),
		);

		await api.vormaNavigate("/build-id-getter-check");
		await vi.runAllTimersAsync();
		expect(api.getBuildID()).toBe("build-id-getter-check");
	});

	it("stops route-change event delivery after listener cleanup", async () => {
		const api = await loadPublicClientAPI();
		const routeChangeEvents: Array<unknown> = [];
		const cleanup = api.addRouteChangeListener((event) => {
			routeChangeEvents.push(event.detail);
		});

		vi.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse({
					loadersData: [],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					loadersData: [],
				}),
			);

		await api.vormaNavigate("/route-change-listener-a");
		await vi.runAllTimersAsync();
		expect(routeChangeEvents.length).toBeGreaterThan(0);

		cleanup();
		const eventCountAfterCleanup = routeChangeEvents.length;

		await api.vormaNavigate("/route-change-listener-b");
		await vi.runAllTimersAsync();
		expect(routeChangeEvents).toHaveLength(eventCountAfterCleanup);
	});

	it("updates getRouterData after successful navigation", async () => {
		const api = await loadPublicClientAPI();
		vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				matchedPatterns: ["/router-data/:id"],
				importURLs: [],
				exportKeys: [],
				errorExportKeys: [],
				hasRootData: true,
				loadersData: [{ id: "123", label: "root" }],
				params: { id: "123" },
				splatValues: [],
			}),
		);

		await api.vormaNavigate("/router-data/123");
		await vi.runAllTimersAsync();

		expect(api.getRouterData()).toEqual({
			buildID: "1",
			matchedPatterns: ["/router-data/:id"],
			splatValues: [],
			params: {
				id: "123",
			},
			rootData: {
				id: "123",
				label: "root",
			},
		});
	});

	it("deterministically overwrites router-data snapshot values on later commits", async () => {
		const api = await loadPublicClientAPI();
		vi.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/ctx/:id"],
					importURLs: [],
					exportKeys: [],
					errorExportKeys: [],
					hasRootData: true,
					loadersData: [{ id: "A" }],
					params: { id: "A" },
					splatValues: [],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/ctx/:id"],
					importURLs: [],
					exportKeys: [],
					errorExportKeys: [],
					hasRootData: true,
					loadersData: [{ id: "B" }],
					params: { id: "B" },
					splatValues: [],
				}),
			);

		await api.vormaNavigate("/ctx/A");
		await vi.runAllTimersAsync();
		await api.vormaNavigate("/ctx/B");
		await vi.runAllTimersAsync();

		expect(api.getRouterData()).toEqual({
			buildID: "1",
			matchedPatterns: ["/ctx/:id"],
			splatValues: [],
			params: {
				id: "B",
			},
			rootData: {
				id: "B",
			},
		});
	});

	it("throws a clear bootstrap error when test deployment configuration runs before runtime init", async () => {
		resetClientRuntimeForTesting();
		expect(() => {
			setDeploymentIDForTesting("deploy-before-init");
		}).toThrow(
			"Vorma runtime must be initialized before setting deploymentID.",
		);
	});

	it("fails loud when test-only client-loader registration is passed an invalid pattern through unsafe casts", async () => {
		expect(() => {
			registerClientLoaderForTesting({
				pattern: "/unsafe-trailing-slash/",
				clientLoader: async () => null,
			} as any);
		}).toThrow("bad trailing slash");
	});

	it("does not mutate established router data after failed navigation", async () => {
		const api = await loadPublicClientAPI();
		vi.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/stable-router-data/:id"],
					importURLs: [],
					exportKeys: [],
					errorExportKeys: [],
					hasRootData: true,
					loadersData: [{ id: "A" }],
					params: { id: "A" },
					splatValues: [],
				}),
			)
			.mockResolvedValueOnce(
				new Response("server-boom", {
					status: 500,
					headers: { "Content-Type": "text/plain" },
				}),
			);

		await api.vormaNavigate("/stable-router-data/A");
		await vi.runAllTimersAsync();
		const stableRouterData = api.getRouterData();

		await expect(
			api.vormaNavigate("/stable-router-data/B"),
		).rejects.toThrow("Navigation request failed (500): server-boom.");
		await vi.runAllTimersAsync();

		expect(api.getRouterData()).toEqual(stableRouterData);
	});

	it("follows redirect responses through final destination commit", async () => {
		const api = await loadPublicClientAPI();
		vi.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				new Response("", {
					status: 200,
					headers: {
						"X-Client-Redirect": "/redirect-final",
						"X-Wave-Framework-Build-Id": "redirect-1",
					},
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse(
					{
						loadersData: [],
						title: { dangerousInnerHTML: "Redirect Final" },
					},
					{
						headers: {
							"X-Wave-Framework-Build-Id": "redirect-2",
						},
					},
				),
			);

		const navigationResult = await api.vormaNavigate("/redirect-source");
		await vi.runAllTimersAsync();

		expect(navigationResult).toEqual({
			didNavigate: true,
		});
		expect(window.location.pathname).toBe("/redirect-final");
		expect(document.title).toBe("Redirect Final");
		expect(api.getBuildID()).toBe("redirect-2");
		expectStatusIdle(api.getStatus());
	});

	it("clears navigating state after abort-style navigation failures", async () => {
		const api = await loadPublicClientAPI();
		vi.spyOn(window, "fetch").mockRejectedValue(
			new DOMException("Aborted", "AbortError"),
		);

		await expect(api.vormaNavigate("/abort-navigation")).resolves.toEqual({
			didNavigate: false,
		});
		expectStatusIdle(api.getStatus());
	});

	it("clears loading state after non-abort failures and remains operable", async () => {
		const api = await loadPublicClientAPI();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockRejectedValueOnce(new Error("Network down"))
			.mockResolvedValueOnce(
				createRouteDataResponse({
					loadersData: [],
					title: {
						dangerousInnerHTML: "Recovered After Failure",
					},
				}),
			);

		await expect(api.vormaNavigate("/non-abort-failure")).rejects.toThrow(
			"Network down",
		);
		expectStatusIdle(api.getStatus());

		await api.vormaNavigate("/post-failure-recovery");
		await vi.runAllTimersAsync();

		expect(fetchSpy).toHaveBeenCalledTimes(2);
		expect(window.location.pathname).toBe("/post-failure-recovery");
		expect(document.title).toBe("Recovered After Failure");
		expectStatusIdle(api.getStatus());
	});

	it("recovers from non-ok navigation responses and remains operable", async () => {
		const api = await loadPublicClientAPI();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValueOnce(new Response("boom", { status: 500 }))
			.mockResolvedValueOnce(
				createRouteDataResponse({
					loadersData: [],
					title: {
						dangerousInnerHTML: "Recovered After Non-OK",
					},
				}),
			);

		await expect(api.vormaNavigate("/non-ok-failure")).rejects.toThrow(
			"Navigation request failed (500): boom.",
		);
		expectStatusIdle(api.getStatus());

		await api.vormaNavigate("/non-ok-recovery");
		await vi.runAllTimersAsync();

		expect(fetchSpy).toHaveBeenCalledTimes(2);
		expect(window.location.pathname).toBe("/non-ok-recovery");
		expect(document.title).toBe("Recovered After Non-OK");
		expectStatusIdle(api.getStatus());
	});

	it("fails loud when server route-data requests return 304", async () => {
		const api = await loadPublicClientAPI();
		vi.spyOn(window, "fetch").mockResolvedValue(
			new Response(null, {
				status: 304,
			}),
		);

		await expect(api.vormaNavigate("/navigation-304")).rejects.toThrow(
			/304/,
		);
		expectStatusIdle(api.getStatus());
	});

	it("does not leak unhandled rejections when failed navigation responses are surfaced", async () => {
		const api = await loadPublicClientAPI();
		vi.spyOn(window, "fetch").mockResolvedValue(
			new Response("server-error", { status: 500 }),
		);

		const { unhandledRejections } = await withUnhandledRejectionCapture({
			run: async () => {
				await expect(
					api.vormaNavigate("/failed-response"),
				).rejects.toThrow(
					"Navigation request failed (500): server-error.",
				);
				await vi.runAllTimersAsync();
			},
		});

		expect(unhandledRejections).toEqual([]);
		expectStatusIdle(api.getStatus());
	});

	it("does not leak unhandled rejections when navigation redirects before loader payload resolution", async () => {
		const api = await loadPublicClientAPI();
		vi.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				new Response("", {
					status: 200,
					headers: {
						"X-Client-Redirect": "/redirect-before-loader-ready",
						"X-Wave-Framework-Build-Id":
							"redirect-before-loader-build",
					},
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					loadersData: [],
					title: {
						dangerousInnerHTML: "Redirect Before Loader Ready",
					},
				}),
			);

		const { unhandledRejections } = await withUnhandledRejectionCapture({
			run: async () => {
				await api.vormaNavigate("/redirect-before-loader-start");
				await vi.runAllTimersAsync();
			},
		});

		expect(unhandledRejections).toEqual([]);
		expect(window.location.pathname).toBe("/redirect-before-loader-ready");
		expect(document.title).toBe("Redirect Before Loader Ready");
		expectStatusIdle(api.getStatus());
	});

	it("contains client-loader data-shape failures without leaking unhandled rejections", async () => {
		const api = await loadPublicClientAPI();
		const pattern = "/client-loader-data-shape-failure";
		registerClientLoaderForTesting({
			pattern,
			clientLoader: async (input: unknown) => {
				const clientLoaderInput = input as {
					serverDataPromise: Promise<{
						loaderData: { LatestVersion: string };
					}>;
				};
				const serverData = await clientLoaderInput.serverDataPromise;
				return serverData.loaderData.LatestVersion;
			},
		});
		vi.spyOn(window, "fetch").mockResolvedValueOnce(
			createRouteDataResponse({
				matchedPatterns: [pattern],
				loadersData: [],
				importURLs: [],
				exportKeys: [],
				errorExportKeys: [],
				hasRootData: false,
			}),
		);

		const { unhandledRejections } = await withUnhandledRejectionCapture({
			run: async () => {
				await api.vormaNavigate(pattern);
				await vi.runAllTimersAsync();
			},
		});

		expect(unhandledRejections).toEqual([]);
		expectStatusIdle(api.getStatus());
	});

	it("does not let stale revalidation override later prefetched navigation", async () => {
		const api = await loadPublicClientAPI();
		window.history.replaceState({}, "", "/");
		let aboutRequestCount = 0;
		const revalidationDeferred = createDeferred<Response>();
		vi.spyOn(window, "fetch").mockImplementation(
			(input: RequestInfo | URL) => {
				const requestURL = requestInputToURL(input);
				if (requestURL.pathname === "/about") {
					aboutRequestCount += 1;
					return Promise.resolve(
						createRouteDataResponse({
							title: { dangerousInnerHTML: "About Page" },
						}),
					);
				}
				return revalidationDeferred.promise;
			},
		);

		await api.vormaNavigate("/about", {
			intent: "prefetch",
		} as never);
		await vi.runAllTimersAsync();

		const revalidatePromise = api.revalidate();
		await vi.advanceTimersByTimeAsync(8);

		await api.vormaNavigate("/about");
		await vi.runAllTimersAsync();
		expect(window.location.pathname).toBe("/about");
		expect(document.title).toBe("About Page");
		expect(aboutRequestCount).toBe(1);

		revalidationDeferred.resolve(
			createRouteDataResponse({
				title: { dangerousInnerHTML: "Home Page" },
			}),
		);
		await revalidatePromise;
		await vi.runAllTimersAsync();

		expect(window.location.pathname).toBe("/about");
		expect(document.title).toBe("About Page");
		expectStatusIdle(api.getStatus());
	});

	it("treats empty navigation json payloads as failures and remains recoverable", async () => {
		const api = await loadPublicClientAPI();
		window.history.replaceState({}, "", "/before-empty-json");
		document.title = "Before Empty JSON";
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				new Response("", {
					status: 200,
					headers: { "Content-Type": "application/json" },
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					loadersData: [],
					title: {
						dangerousInnerHTML: "Recovered After Empty JSON",
					},
				}),
			);

		await expect(
			api.vormaNavigate("/empty-json-route-data"),
		).rejects.toThrow();
		expect(window.location.pathname).toBe("/before-empty-json");
		expect(document.title).toBe("Before Empty JSON");
		expectStatusIdle(api.getStatus());

		await api.vormaNavigate("/empty-json-recovered");
		await vi.runAllTimersAsync();

		expect(fetchSpy).toHaveBeenCalledTimes(2);
		expect(window.location.pathname).toBe("/empty-json-recovered");
		expect(document.title).toBe("Recovered After Empty JSON");
		expectStatusIdle(api.getStatus());
	});

	it("follows navigation redirect headers even when first response is non-ok", async () => {
		const api = await loadPublicClientAPI();
		vi.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				new Response("ignored", {
					status: 500,
					headers: {
						"X-Client-Redirect": "/redirect-non-ok",
						"X-Wave-Framework-Build-Id": "redirect-non-ok-1",
					},
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					loadersData: [],
					title: { dangerousInnerHTML: "Redirect Non-OK Final" },
				}),
			);

		await api.vormaNavigate("/redirect-non-ok-start");
		await vi.runAllTimersAsync();

		expect(window.location.pathname).toBe("/redirect-non-ok");
		expect(document.title).toBe("Redirect Non-OK Final");
		expectStatusIdle(api.getStatus());
	});

	it("resolves relative navigation X-Client-Redirect targets against the redirecting request URL path", async () => {
		const api = await loadPublicClientAPI();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				new Response("", {
					status: 200,
					headers: {
						"X-Client-Redirect": "child/final",
						"X-Wave-Framework-Build-Id": "relative-redirect-build",
					},
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					loadersData: [],
					title: { dangerousInnerHTML: "Relative Redirect Final" },
				}),
			);

		await api.vormaNavigate("/relative/base");
		await vi.runAllTimersAsync();

		expect(
			requestInputToURL(fetchSpy.mock.calls[1]?.[0] as RequestInfo | URL)
				.pathname,
		).toBe("/relative/child/final");
		expect(window.location.pathname).toBe("/relative/child/final");
		expect(document.title).toBe("Relative Redirect Final");
		expectStatusIdle(api.getStatus());
	});

	it("does not re-follow navigation X-Client-Redirect targets that resolve to the current same-document location", async () => {
		const api = await loadPublicClientAPI();
		window.history.replaceState({}, "", "/already-here#same");
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValueOnce(
			new Response("", {
				status: 200,
				headers: {
					"X-Client-Redirect": "/already-here#same",
				},
			}),
		);

		await api.vormaNavigate("/redirect-to-current");
		await vi.runAllTimersAsync();

		expect(fetchSpy).toHaveBeenCalledTimes(1);
		expect(window.location.pathname).toBe("/already-here");
		expect(window.location.hash).toBe("#same");
		expectStatusIdle(api.getStatus());
	});

	it("updates build ID before following navigation redirect targets", async () => {
		const api = await loadPublicClientAPI();
		const observedInBuildListener: Array<{
			currentBuildID: string;
			oldID: string;
			newID: string;
		}> = [];
		const cleanup = api.addBuildIDListener((event) => {
			observedInBuildListener.push({
				currentBuildID: api.getBuildID(),
				oldID: event.detail.oldID,
				newID: event.detail.newID,
			});
		});
		try {
			vi.spyOn(window, "fetch")
				.mockResolvedValueOnce(
					new Response("", {
						status: 200,
						headers: {
							"X-Client-Redirect": "/redirect-build-target",
							"X-Wave-Framework-Build-Id": "redirect-build-1",
						},
					}),
				)
				.mockResolvedValueOnce(
					createRouteDataResponse(
						{
							loadersData: [],
							title: {
								dangerousInnerHTML: "Redirect Build Target",
							},
						},
						{
							headers: {
								"X-Wave-Framework-Build-Id": "redirect-build-1",
							},
						},
					),
				);

			await api.vormaNavigate("/redirect-build-start");
			await vi.runAllTimersAsync();

			expect(observedInBuildListener).toEqual([
				{
					currentBuildID: "redirect-build-1",
					oldID: "1",
					newID: "redirect-build-1",
				},
			]);
			expect(api.getBuildID()).toBe("redirect-build-1");
			expect(window.location.pathname).toBe("/redirect-build-target");
		} finally {
			cleanup();
		}
	});

	it("performs hard redirect for external navigation redirect targets", async () => {
		const api = await loadPublicClientAPI();
		const locationHrefStub = stubWindowLocationHref();
		try {
			vi.spyOn(window, "fetch").mockResolvedValueOnce(
				createRouteDataResponse(
					{},
					{
						headers: {
							"X-Client-Redirect": "https://external.example",
						},
					},
				),
			);

			await api.vormaNavigate("/external-nav-start");
			await vi.runAllTimersAsync();

			const redirectedURL = new URL(locationHrefStub.getHref());
			expect(redirectedURL.origin).toBe("https://external.example");
			expect(redirectedURL.pathname).toBe("/");
			expectStatusIdle(api.getStatus());
		} finally {
			locationHrefStub.restore();
		}
	});

	it("performs hard reload redirect for navigation X-Wave-Framework-Reload responses", async () => {
		const api = await loadPublicClientAPI();
		const locationHrefStub = stubWindowLocationHref();
		try {
			vi.spyOn(window, "fetch").mockResolvedValueOnce(
				createRouteDataResponse(
					{},
					{
						headers: {
							"X-Wave-Framework-Reload": "/force-reload-nav",
							"X-Wave-Framework-Build-Id": "reload-nav-build-1",
						},
					},
				),
			);

			await api.vormaNavigate("/hard-reload-nav-start");
			await vi.runAllTimersAsync();

			expect(locationHrefStub.getHref()).toContain("/force-reload-nav");
			expect(locationHrefStub.getHref()).toContain(
				"vorma_reload=reload-nav-build-1",
			);
			expect(api.getBuildID()).toBe("reload-nav-build-1");
			expectStatusIdle(api.getStatus());
		} finally {
			locationHrefStub.restore();
		}
	});

	it("resolves relative X-Wave-Framework-Reload targets against redirecting request URL path", async () => {
		window.history.replaceState({}, "", "/current-parent/");
		const api = await loadPublicClientAPI();
		const locationHrefStub = stubWindowLocationHref();
		try {
			vi.spyOn(window, "fetch").mockResolvedValueOnce(
				createRouteDataResponse(
					{},
					{
						headers: {
							"X-Wave-Framework-Reload": "child-reload",
							"X-Wave-Framework-Build-Id":
								"relative-reload-build-1",
						},
					},
				),
			);

			await api.vormaNavigate("/server/base/start");
			await vi.runAllTimersAsync();

			const redirectedURL = new URL(locationHrefStub.getHref());
			expect(redirectedURL.pathname).toBe("/server/base/child-reload");
			expect(redirectedURL.searchParams.get("vorma_reload")).toBe(
				"relative-reload-build-1",
			);
		} finally {
			locationHrefStub.restore();
		}
	});

	it("prioritizes X-Wave-Framework-Reload over X-Client-Redirect for navigation", async () => {
		const api = await loadPublicClientAPI();
		const locationHrefStub = stubWindowLocationHref();
		try {
			vi.spyOn(window, "fetch").mockResolvedValueOnce(
				createRouteDataResponse(
					{},
					{
						headers: {
							"X-Wave-Framework-Reload":
								"/force-reload-nav-priority",
							"X-Client-Redirect": "/ignored-soft-nav",
							"X-Wave-Framework-Build-Id": "priority-nav-build-1",
						},
					},
				),
			);

			await api.vormaNavigate("/priority-nav-start");
			await vi.runAllTimersAsync();

			expect(locationHrefStub.getHref()).toContain(
				"/force-reload-nav-priority",
			);
			expect(locationHrefStub.getHref()).toContain(
				"vorma_reload=priority-nav-build-1",
			);
			expect(locationHrefStub.getHref()).not.toContain(
				"/ignored-soft-nav",
			);
			expect(api.getBuildID()).toBe("priority-nav-build-1");
		} finally {
			locationHrefStub.restore();
		}
	});

	it("treats non-http navigation redirect targets as explicit navigation errors", async () => {
		const api = await loadPublicClientAPI();
		window.history.replaceState({}, "", "/redirect-non-http-before");
		document.title = "Before Redirect Error";
		vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse(
				{
					loadersData: [],
					title: { dangerousInnerHTML: "Should Not Commit" },
				},
				{
					headers: {
						"X-Client-Redirect": "mailto:test@example.com",
					},
				},
			),
		);

		await expect(
			api.vormaNavigate("/redirect-non-http-start"),
		).rejects.toThrow(/http\(s\)/i);
		await vi.runAllTimersAsync();

		expect(window.location.pathname).toBe("/redirect-non-http-before");
		expect(document.title).toBe("Before Redirect Error");
		expectStatusIdle(api.getStatus());
	});

	it("follows native fetch redirects for GET navigation requests", async () => {
		const api = await loadPublicClientAPI();
		const nativeRedirectResponse = createRouteDataResponse();
		Object.defineProperty(nativeRedirectResponse, "redirected", {
			value: true,
			configurable: true,
		});
		Object.defineProperty(nativeRedirectResponse, "url", {
			value: `${window.location.origin}/native-get-redirect`,
			configurable: true,
		});

		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValueOnce(nativeRedirectResponse)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					loadersData: [],
					title: {
						dangerousInnerHTML: "Native GET Redirected Page",
					},
				}),
			);

		await api.vormaNavigate("/native-get-start");
		await vi.runAllTimersAsync();

		expect(fetchSpy).toHaveBeenCalledTimes(2);
		const secondFetchURL = requestInputToURL(
			fetchSpy.mock.calls[1]?.[0] as RequestInfo | URL,
		);
		expect(secondFetchURL.pathname).toBe("/native-get-redirect");
		expect(window.location.pathname).toBe("/native-get-redirect");
		expect(document.title).toBe("Native GET Redirected Page");
	});

	it("does not re-follow native redirects that land on encoding-equivalent current hash targets", async () => {
		window.history.replaceState({}, "", "/native-current#~");
		const api = await loadPublicClientAPI();
		const nativeRedirectResponse = createRouteDataResponse();
		Object.defineProperty(nativeRedirectResponse, "redirected", {
			value: true,
			configurable: true,
		});
		Object.defineProperty(nativeRedirectResponse, "url", {
			value: `${window.location.origin}/native-current#%7E`,
			configurable: true,
		});

		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValueOnce(nativeRedirectResponse);

		await api.vormaNavigate("/native-start");
		await vi.runAllTimersAsync();

		expect(fetchSpy).toHaveBeenCalledTimes(1);
		const firstFetchURL = requestInputToURL(
			fetchSpy.mock.calls[0]?.[0] as RequestInfo | URL,
		);
		expect(firstFetchURL.pathname).toBe("/native-start");
		expect(window.location.pathname).toBe("/native-current");
		expect(window.location.hash).toBe("#~");
	});

	it("follows submit redirect headers even when submit response is non-ok", async () => {
		const api = await loadPublicClientAPI();
		vi.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				new Response("ignored", {
					status: 500,
					headers: {
						"X-Client-Redirect": "/submit-redirect-non-ok-final",
					},
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					loadersData: [],
					title: {
						dangerousInnerHTML: "Submit Redirect Non-OK Final",
					},
				}),
			);

		const result = await api.submit(
			"/api/submit-redirect-non-ok",
			{
				method: "POST",
			},
			{
				revalidate: false,
			},
		);
		await vi.runAllTimersAsync();

		expect(result).toEqual({
			success: true,
			data: undefined,
		});
		expect(window.location.pathname).toBe("/submit-redirect-non-ok-final");
		expect(document.title).toBe("Submit Redirect Non-OK Final");
	});

	it("follows internal redirect responses from submit", async () => {
		const api = await loadPublicClientAPI();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				new Response("", {
					status: 200,
					headers: {
						"X-Client-Redirect": "/submit-redirect-internal-final",
					},
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					loadersData: [],
					title: {
						dangerousInnerHTML: "Submit Redirect Internal Final",
					},
				}),
			);

		const result = await api.submit("/api/submit-redirect-internal", {
			method: "POST",
		});
		await vi.runAllTimersAsync();

		expect(result).toEqual({
			success: true,
			data: undefined,
		});
		expect(fetchSpy).toHaveBeenCalledTimes(2);
		expect(window.location.pathname).toBe(
			"/submit-redirect-internal-final",
		);
		expect(document.title).toBe("Submit Redirect Internal Final");
	});

	it("does not fetch route data for submit redirects that only change hash", async () => {
		const api = await loadPublicClientAPI();
		window.history.replaceState({}, "", "/submit-hash-redirect");
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			new Response("", {
				status: 200,
				headers: {
					"X-Client-Redirect": "/submit-hash-redirect#frag",
				},
			}),
		);

		const result = await api.submit(
			"/api/submit-hash-redirect",
			{
				method: "POST",
			},
			{
				revalidate: false,
			},
		);
		await vi.runAllTimersAsync();

		expect(result).toEqual({
			success: true,
			data: undefined,
		});
		expect(fetchSpy).toHaveBeenCalledTimes(1);
		expect(window.location.pathname).toBe("/submit-hash-redirect");
		expect(window.location.hash).toBe("#frag");
	});

	it("does not re-follow submit redirects to the current path", async () => {
		const api = await loadPublicClientAPI();
		window.history.replaceState({}, "", "/submit-same-path");
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			new Response("", {
				status: 200,
				headers: {
					"X-Client-Redirect": "/submit-same-path",
				},
			}),
		);

		const result = await api.submit(
			"/api/submit-same-path",
			{
				method: "POST",
			},
			{
				revalidate: false,
			},
		);
		await vi.runAllTimersAsync();

		expect(result).toEqual({
			success: true,
			data: undefined,
		});
		expect(fetchSpy).toHaveBeenCalledTimes(1);
		expect(window.location.pathname).toBe("/submit-same-path");
		expectStatusIdle(api.getStatus());
	});

	it("updates build ID before following submit redirect targets", async () => {
		const api = await loadPublicClientAPI();
		const observedInBuildListener: Array<{
			currentBuildID: string;
			oldID: string;
			newID: string;
		}> = [];
		const cleanup = api.addBuildIDListener((event) => {
			observedInBuildListener.push({
				currentBuildID: api.getBuildID(),
				oldID: event.detail.oldID,
				newID: event.detail.newID,
			});
		});
		try {
			vi.spyOn(window, "fetch")
				.mockResolvedValueOnce(
					new Response("", {
						status: 200,
						headers: {
							"X-Client-Redirect":
								"/submit-redirect-build-target",
							"X-Wave-Framework-Build-Id":
								"submit-redirect-build-1",
						},
					}),
				)
				.mockResolvedValueOnce(
					createRouteDataResponse(
						{
							loadersData: [],
							title: {
								dangerousInnerHTML:
									"Submit Redirect Build Target",
							},
						},
						{
							headers: {
								"X-Wave-Framework-Build-Id":
									"submit-redirect-build-1",
							},
						},
					),
				);

			const result = await api.submit("/api/submit-build-redirect", {
				method: "POST",
			});
			await vi.runAllTimersAsync();

			expect(result).toEqual({
				success: true,
				data: undefined,
			});
			expect(observedInBuildListener).toEqual([
				{
					currentBuildID: "submit-redirect-build-1",
					oldID: "1",
					newID: "submit-redirect-build-1",
				},
			]);
			expect(api.getBuildID()).toBe("submit-redirect-build-1");
			expect(window.location.pathname).toBe(
				"/submit-redirect-build-target",
			);
		} finally {
			cleanup();
		}
	});

	it("returns explicit failure when submit soft redirect follow fails", async () => {
		const api = await loadPublicClientAPI();
		vi.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				new Response("", {
					status: 200,
					headers: {
						"X-Client-Redirect": "/submit-follow-fail-target",
					},
				}),
			)
			.mockRejectedValueOnce(
				new Error("Redirect navigation request failed"),
			);

		const result = await api.submit("/api/submit-follow-fail", {
			method: "POST",
		});
		await vi.runAllTimersAsync();

		expect(result).toEqual({
			success: false,
			error: "Redirect failed",
		});
		expect(window.location.pathname).toBe("/");
		expectStatusIdle(api.getStatus());
	});

	it("performs hard reload redirect when submit response includes X-Wave-Framework-Reload", async () => {
		const api = await loadPublicClientAPI();
		const locationHrefStub = stubWindowLocationHref();
		try {
			vi.spyOn(window, "fetch").mockResolvedValueOnce(
				createRouteDataResponse(
					{},
					{
						headers: {
							"X-Wave-Framework-Reload": "/force-reload-submit",
							"X-Wave-Framework-Build-Id":
								"reload-submit-build-1",
						},
					},
				),
			);

			const result = await api.submit("/api/action", { method: "POST" });
			await vi.runAllTimersAsync();

			expect(result).toEqual({ success: true, data: undefined });
			expect(locationHrefStub.getHref()).toContain(
				"/force-reload-submit",
			);
			expect(locationHrefStub.getHref()).toContain(
				"vorma_reload=reload-submit-build-1",
			);
		} finally {
			locationHrefStub.restore();
		}
	});

	it("performs hard redirect for external submit redirect targets", async () => {
		const api = await loadPublicClientAPI();
		const locationHrefStub = stubWindowLocationHref();
		try {
			vi.spyOn(window, "fetch").mockResolvedValueOnce(
				createRouteDataResponse(
					{},
					{
						headers: {
							"X-Client-Redirect": "https://external.com",
						},
					},
				),
			);

			const result = await api.submit("/api/action", { method: "POST" });
			await vi.runAllTimersAsync();

			expect(result).toEqual({ success: true, data: undefined });
			expect(locationHrefStub.getHref()).toMatch(
				/^https:\/\/external\.com\/?$/,
			);
		} finally {
			locationHrefStub.restore();
		}
	});

	it("prioritizes X-Wave-Framework-Reload over X-Client-Redirect for submit", async () => {
		const api = await loadPublicClientAPI();
		const locationHrefStub = stubWindowLocationHref();
		try {
			vi.spyOn(window, "fetch").mockResolvedValueOnce(
				createRouteDataResponse(
					{},
					{
						headers: {
							"X-Wave-Framework-Reload":
								"/force-reload-submit-priority",
							"X-Client-Redirect": "/ignored-soft-submit",
							"X-Wave-Framework-Build-Id":
								"priority-submit-build-1",
						},
					},
				),
			);

			const result = await api.submit("/api/priority-submit", {
				method: "POST",
			});
			await vi.runAllTimersAsync();

			expect(result).toEqual({ success: true, data: undefined });
			expect(locationHrefStub.getHref()).toContain(
				"/force-reload-submit-priority",
			);
			expect(locationHrefStub.getHref()).toContain(
				"vorma_reload=priority-submit-build-1",
			);
			expect(locationHrefStub.getHref()).not.toContain(
				"/ignored-soft-submit",
			);
		} finally {
			locationHrefStub.restore();
		}
	});

	it("follows native fetch redirects for non-get submit requests", async () => {
		const api = await loadPublicClientAPI();
		const nativeRedirectResponse = createRouteDataResponse();
		Object.defineProperty(nativeRedirectResponse, "redirected", {
			value: true,
			configurable: true,
		});
		Object.defineProperty(nativeRedirectResponse, "url", {
			value: `${window.location.origin}/native-submit-redirect`,
			configurable: true,
		});

		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValueOnce(nativeRedirectResponse)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					loadersData: [],
					title: {
						dangerousInnerHTML:
							"Native Submit Redirect Destination",
					},
				}),
			);

		const result = await api.submit("/api/native-submit-redirect", {
			method: "POST",
		});
		await vi.runAllTimersAsync();

		expect(result).toEqual({
			success: true,
			data: undefined,
		});
		expect(fetchSpy).toHaveBeenCalledTimes(2);
		const secondFetchURL = requestInputToURL(
			fetchSpy.mock.calls[1]?.[0] as RequestInfo | URL,
		);
		expect(secondFetchURL.pathname).toBe("/native-submit-redirect");
		expect(window.location.pathname).toBe("/native-submit-redirect");
		expect(document.title).toBe("Native Submit Redirect Destination");
	});

	it("includes redirect-accept header on navigation and submit fetches", async () => {
		const api = await loadPublicClientAPI();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValue(createRouteDataResponse());

		await api.vormaNavigate("/header-check");
		await vi.runAllTimersAsync();
		await api.submit(
			"/api/header-check",
			{ method: "POST" },
			{
				revalidate: false,
			},
		);

		const navigationHeaders = new Headers(
			(fetchSpy.mock.calls[0]?.[1] as RequestInit | undefined)?.headers,
		);
		const submitHeaders = new Headers(
			(fetchSpy.mock.calls[1]?.[1] as RequestInit | undefined)?.headers,
		);
		expect(navigationHeaders.get("X-Accepts-Client-Redirect")).toBe("1");
		expect(submitHeaders.get("X-Accepts-Client-Redirect")).toBe("1");
	});

	it("includes x-deployment-id on submit requests when deployment ID is configured", async () => {
		const api = await loadPublicClientAPI();
		void api.getStatus();
		setDeploymentIDForTesting("deploy-42");
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValue(createRouteDataResponse());

		await api.submit(
			"/api/with-deployment",
			{
				method: "POST",
				headers: {
					"X-Test-Header": "kept",
				},
				body: JSON.stringify({ ok: true }),
			},
			{
				revalidate: false,
			},
		);

		const submitHeaders = new Headers(
			(fetchSpy.mock.calls[0]?.[1] as RequestInit | undefined)?.headers,
		);
		expect(submitHeaders.get("x-deployment-id")).toBe("deploy-42");
		expect(submitHeaders.get("X-Test-Header")).toBe("kept");
	});

	it("keeps loading status continuous through redirect chains", async () => {
		const api = await loadPublicClientAPI();
		const statuses: StatusSnapshot[] = [];
		const cleanup = api.addStatusListener((event) => {
			statuses.push(event.detail);
		});
		try {
			vi.spyOn(window, "fetch")
				.mockResolvedValueOnce(
					new Response("", {
						status: 200,
						headers: {
							"X-Client-Redirect": "/redirect-mid",
							"X-Wave-Framework-Build-Id": "redirect-build-1",
						},
					}),
				)
				.mockResolvedValueOnce(
					new Response("", {
						status: 200,
						headers: {
							"X-Client-Redirect": "/redirect-final-chain",
							"X-Wave-Framework-Build-Id": "redirect-build-2",
						},
					}),
				)
				.mockResolvedValueOnce(
					createRouteDataResponse(
						{
							loadersData: [],
							title: {
								dangerousInnerHTML: "Redirect Chain Final",
							},
						},
						{
							headers: {
								"X-Wave-Framework-Build-Id": "redirect-build-3",
							},
						},
					),
				);

			await api.vormaNavigate("/redirect-chain-start");
			await vi.runAllTimersAsync();

			expect(window.location.pathname).toBe("/redirect-final-chain");
			expect(document.title).toBe("Redirect Chain Final");
			expect(statuses.length).toBeGreaterThan(0);
			expect(statuses.some((status) => status.isNavigating)).toBe(true);
			expectNoLoadingGapBeforeFinalEvent(statuses);
			expect(statuses.at(-1)).toEqual({
				isNavigating: false,
				isSubmitting: false,
				isRevalidating: false,
			});
		} finally {
			cleanup();
		}
	});

	it("caps redirect chains at ten follows and exits loading state", async () => {
		const api = await loadPublicClientAPI();
		const fetchSpy = vi.spyOn(window, "fetch");
		const consoleErrorSpy = vi
			.spyOn(console, "error")
			.mockImplementation(() => {});

		for (let i = 0; i < 15; i += 1) {
			fetchSpy.mockResolvedValueOnce(
				new Response("", {
					status: 200,
					headers: {
						"X-Client-Redirect": `/redirect-loop-${i}`,
					},
				}),
			);
		}

		const result = await api.vormaNavigate("/redirect-loop-start");
		await vi.runAllTimersAsync();

		expect(result).toEqual({
			didNavigate: false,
		});
		expect(fetchSpy).toHaveBeenCalledTimes(10);
		expect(window.location.pathname).toBe("/");
		expect(consoleErrorSpy).toHaveBeenCalledWith(
			"Vorma:",
			"Too many redirects",
		);
		expectStatusIdle(api.getStatus());
	});

	it("does not require stale route-data responses to settle before winner commit", async () => {
		const api = await loadPublicClientAPI();
		const firstDeferred = createDeferred<Response>();
		const secondDeferred = createDeferred<Response>();
		let fetchCallCount = 0;
		vi.spyOn(window, "fetch").mockImplementation(() => {
			fetchCallCount += 1;
			if (fetchCallCount === 1) {
				return firstDeferred.promise;
			}
			if (fetchCallCount === 2) {
				return secondDeferred.promise;
			}
			throw new Error("Unexpected fetch call");
		});

		const firstNavigation = api.vormaNavigate("/stale-first");
		const secondNavigation = api.vormaNavigate("/stale-winner");

		secondDeferred.resolve(
			createRouteDataResponse({
				loadersData: [],
				title: { dangerousInnerHTML: "Winner" },
			}),
		);

		await secondNavigation;
		await vi.runAllTimersAsync();

		expect(window.location.pathname).toBe("/stale-winner");
		expect(document.title).toBe("Winner");

		firstDeferred.resolve(
			createRouteDataResponse({
				loadersData: [],
				title: { dangerousInnerHTML: "Stale First" },
			}),
		);

		await firstNavigation;
		await vi.runAllTimersAsync();

		expect(window.location.pathname).toBe("/stale-winner");
		expect(document.title).toBe("Winner");
		expectStatusIdle(api.getStatus());
	});

	it("cleans up completed navigation entries so later distinct navigations still fetch", async () => {
		const api = await loadPublicClientAPI();
		const fetchSpy = vi.spyOn(window, "fetch").mockImplementation(() =>
			Promise.resolve(
				createRouteDataResponse({
					loadersData: [],
				}),
			),
		);

		await api.vormaNavigate("/cleanup-fetch-a");
		await vi.runAllTimersAsync();
		await api.vormaNavigate("/cleanup-fetch-b");
		await vi.runAllTimersAsync();

		expect(fetchSpy).toHaveBeenCalledTimes(2);
		const firstRequestURL = requestInputToURL(
			fetchSpy.mock.calls[0]?.[0] as RequestInfo | URL,
		);
		const secondRequestURL = requestInputToURL(
			fetchSpy.mock.calls[1]?.[0] as RequestInfo | URL,
		);
		expect(firstRequestURL.pathname).toBe("/cleanup-fetch-a");
		expect(secondRequestURL.pathname).toBe("/cleanup-fetch-b");
	});

	it("does not apply side effects from stale aborted navigation successes", async () => {
		const api = await loadPublicClientAPI();
		const staleDeferred = createDeferred<Response>();
		const requestAnimationFrameSpy = vi.spyOn(
			window,
			"requestAnimationFrame",
		);
		const buildIDEvents: Array<{ newID: string; oldID: string }> = [];
		const removeBuildIDListener = api.addBuildIDListener((event) => {
			buildIDEvents.push(event.detail);
		});
		let fetchCallCount = 0;
		vi.spyOn(window, "fetch").mockImplementation(() => {
			fetchCallCount += 1;
			if (fetchCallCount === 1) {
				return staleDeferred.promise;
			}
			return Promise.resolve(
				createRouteDataResponse(
					{
						loadersData: [],
						title: {
							dangerousInnerHTML: "Winner Navigation Title",
						},
					},
					{
						headers: {
							"X-Wave-Framework-Build-Id": "winner-build-id",
						},
					},
				),
			);
		});
		try {
			const staleNavigation = api.vormaNavigate("/stale-side-effects");
			await Promise.resolve();
			const winnerNavigation = api.vormaNavigate("/winner-side-effects");
			await winnerNavigation;
			await vi.runAllTimersAsync();

			const rAFCallCountBeforeStale =
				requestAnimationFrameSpy.mock.calls.length;
			staleDeferred.resolve(
				createRouteDataResponse(
					{
						loadersData: [],
						title: {
							dangerousInnerHTML: "Stale Side Effects Title",
						},
						cssBundles: ["/stale-side-effects.css"],
					},
					{
						headers: {
							"X-Wave-Framework-Build-Id": "stale-build-id",
						},
					},
				),
			);
			await staleNavigation;
			await vi.advanceTimersByTimeAsync(32);
			await vi.runAllTimersAsync();

			expect(fetchCallCount).toBe(2);
			expect(window.location.pathname).toBe("/winner-side-effects");
			expect(document.title).toBe("Winner Navigation Title");
			expect(api.getBuildID()).toBe("winner-build-id");
			expect(
				buildIDEvents.some((event) => event.newID === "stale-build-id"),
			).toBe(false);
			expect(requestAnimationFrameSpy).toHaveBeenCalledTimes(
				rAFCallCountBeforeStale,
			);
			expect(
				document.head.querySelector(
					'link[data-vorma-css-bundle="/stale-side-effects.css"]',
				),
			).toBeNull();
			expectStatusIdle(api.getStatus());
		} finally {
			removeBuildIDListener();
		}
	});

	it("reuses in-flight navigation when only hash changes on the same data target", async () => {
		const api = await loadPublicClientAPI();
		const deferred = createDeferred<Response>();
		const fetchSpy = vi.spyOn(window, "fetch").mockImplementation(() => {
			return deferred.promise;
		});

		const firstNavigation = api.vormaNavigate("/hash-only#first");
		await waitForRequestCount({
			requests: fetchSpy.mock.calls,
			count: 1,
		});
		const secondNavigation = api.vormaNavigate("/hash-only#second");
		await vi.advanceTimersByTimeAsync(8);

		expect(fetchSpy).toHaveBeenCalledTimes(1);
		expect(api.getStatus().isNavigating).toBe(true);

		deferred.resolve(
			createRouteDataResponse({
				loadersData: [],
				title: { dangerousInnerHTML: "Hash Stable Page" },
			}),
		);

		await Promise.all([firstNavigation, secondNavigation]);
		await vi.runAllTimersAsync();

		expect(window.location.pathname).toBe("/hash-only");
		expect(window.location.hash).toBe("#second");
		expect(document.title).toBe("Hash Stable Page");
		expectStatusIdle(api.getStatus());
	});

	it("does not alias in-flight navigation when search params differ", async () => {
		const api = await loadPublicClientAPI();
		const firstFetchCall = createDeferredFetchCall();
		const secondFetchCall = createDeferredFetchCall();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockImplementationOnce(firstFetchCall.mock)
			.mockImplementationOnce(secondFetchCall.mock);

		const firstNavigation = api.vormaNavigate("/search-alias?one=1");
		await waitForRequestCount({
			requests: fetchSpy.mock.calls,
			count: 1,
		});
		const secondNavigation = api.vormaNavigate("/search-alias?two=2");
		await waitForRequestCount({
			requests: fetchSpy.mock.calls,
			count: 2,
		});

		expect(firstFetchCall.getSignal()?.aborted).toBe(true);
		const firstFetchURL = requestInputToURL(
			fetchSpy.mock.calls[0]?.[0] as RequestInfo | URL,
		);
		const secondFetchURL = requestInputToURL(
			fetchSpy.mock.calls[1]?.[0] as RequestInfo | URL,
		);
		expect(firstFetchURL.searchParams.get("one")).toBe("1");
		expect(firstFetchURL.searchParams.get("two")).toBeNull();
		expect(secondFetchURL.searchParams.get("two")).toBe("2");
		expect(secondFetchURL.searchParams.get("one")).toBeNull();

		firstFetchCall.deferred.reject(
			new DOMException("Aborted", "AbortError"),
		);
		secondFetchCall.deferred.resolve(
			createRouteDataResponse({
				loadersData: [],
				title: { dangerousInnerHTML: "Search Alias Winner" },
			}),
		);

		await Promise.all([firstNavigation, secondNavigation]);
		await vi.runAllTimersAsync();

		expect(window.location.pathname).toBe("/search-alias");
		expect(window.location.search).toBe("?two=2");
		expect(document.title).toBe("Search Alias Winner");
		expectStatusIdle(api.getStatus());
	});

	it("does not dedupe prefetch work when only search params differ", async () => {
		const api = await loadPublicClientAPI();
		const firstFetchCall = createDeferredFetchCall();
		const secondFetchCall = createDeferredFetchCall();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockImplementationOnce(firstFetchCall.mock)
			.mockImplementationOnce(secondFetchCall.mock);

		const firstPrefetch = api.vormaNavigate("/prefetch-search?one=1", {
			intent: "prefetch",
		});
		await waitForRequestCount({
			requests: fetchSpy.mock.calls,
			count: 1,
		});
		const secondPrefetch = api.vormaNavigate("/prefetch-search?two=2", {
			intent: "prefetch",
		});
		await waitForRequestCount({
			requests: fetchSpy.mock.calls,
			count: 2,
		});

		expect(firstFetchCall.getSignal()?.aborted).toBe(false);
		expect(secondFetchCall.getSignal()?.aborted).toBe(false);
		const firstFetchURL = requestInputToURL(
			fetchSpy.mock.calls[0]?.[0] as RequestInfo | URL,
		);
		const secondFetchURL = requestInputToURL(
			fetchSpy.mock.calls[1]?.[0] as RequestInfo | URL,
		);
		expect(firstFetchURL.searchParams.get("one")).toBe("1");
		expect(firstFetchURL.searchParams.get("two")).toBeNull();
		expect(secondFetchURL.searchParams.get("two")).toBe("2");
		expect(secondFetchURL.searchParams.get("one")).toBeNull();

		firstFetchCall.deferred.resolve(
			createRouteDataResponse({
				loadersData: [],
			}),
		);
		secondFetchCall.deferred.resolve(
			createRouteDataResponse({
				loadersData: [],
			}),
		);

		await Promise.all([firstPrefetch, secondPrefetch]);
		await vi.runAllTimersAsync();

		expect(fetchSpy).toHaveBeenCalledTimes(2);
		expectStatusIdle(api.getStatus());
	});

	it("does not let stale redirect-follow-up navigation override a newer user navigation", async () => {
		const api = await loadPublicClientAPI();
		const staleFollowUpDeferred = createDeferred<Response>();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockImplementation((input: RequestInfo | URL) => {
				const requestURL = requestInputToURL(input);
				if (requestURL.pathname === "/redirect-race-start") {
					return Promise.resolve(
						new Response("", {
							status: 200,
							headers: {
								"X-Client-Redirect": "/redirect-race-target",
								"X-Wave-Framework-Build-Id":
									"redirect-race-build-1",
							},
						}),
					);
				}
				if (requestURL.pathname === "/redirect-race-target") {
					return staleFollowUpDeferred.promise;
				}
				if (requestURL.pathname === "/redirect-race-winner") {
					return Promise.resolve(
						createRouteDataResponse({
							loadersData: [],
							title: {
								dangerousInnerHTML: "Redirect Race Winner",
							},
						}),
					);
				}
				throw new Error(
					`Unexpected fetch path: ${requestURL.pathname}`,
				);
			});

		const staleNavigation = api.vormaNavigate("/redirect-race-start");
		await waitForRequestCount({
			requests: fetchSpy.mock.calls,
			count: 2,
		});

		const winnerNavigation = api.vormaNavigate("/redirect-race-winner");
		await winnerNavigation;
		await vi.runAllTimersAsync();

		staleFollowUpDeferred.resolve(
			createRouteDataResponse({
				loadersData: [],
				title: {
					dangerousInnerHTML: "Stale Redirect Follow-Up Applied",
				},
			}),
		);
		await staleNavigation;
		await vi.runAllTimersAsync();

		expect(fetchSpy).toHaveBeenCalledTimes(3);
		expect(window.location.pathname).toBe("/redirect-race-winner");
		expect(document.title).toBe("Redirect Race Winner");
		expectStatusIdle(api.getStatus());
	});

	it("does not follow stale native redirects from an aborted user navigation", async () => {
		const api = await loadPublicClientAPI();
		const staleNativeDeferred = createDeferred<Response>();
		const buildIDEvents: Array<{ newID: string; oldID: string }> = [];
		const removeBuildIDListener = api.addBuildIDListener((event) => {
			buildIDEvents.push(event.detail);
		});
		const staleNativeRedirectResponse = createRouteDataResponse(
			{},
			{
				headers: {
					"X-Wave-Framework-Build-Id": "stale-native-nav-build",
				},
			},
		);
		Object.defineProperty(staleNativeRedirectResponse, "redirected", {
			value: true,
			configurable: true,
		});
		Object.defineProperty(staleNativeRedirectResponse, "url", {
			value: `${window.location.origin}/stale-native-nav-redirect`,
			configurable: true,
		});
		let fetchCallCount = 0;
		const fetchSpy = vi.spyOn(window, "fetch").mockImplementation(() => {
			fetchCallCount += 1;
			if (fetchCallCount === 1) {
				return staleNativeDeferred.promise;
			}
			if (fetchCallCount === 2) {
				return Promise.resolve(
					createRouteDataResponse({
						loadersData: [],
						title: { dangerousInnerHTML: "Navigation Winner" },
					}),
				);
			}
			return Promise.resolve(
				createRouteDataResponse({
					loadersData: [],
					title: {
						dangerousInnerHTML:
							"Stale Native Redirect Was Followed",
					},
				}),
			);
		});

		try {
			const staleNavigation = api.vormaNavigate("/stale-native-start");
			await vi.advanceTimersByTimeAsync(8);

			await api.vormaNavigate("/winner-page");
			await vi.runAllTimersAsync();

			staleNativeDeferred.resolve(staleNativeRedirectResponse);
			await staleNavigation;
			await vi.runAllTimersAsync();

			expect(fetchSpy).toHaveBeenCalledTimes(2);
			expect(window.location.pathname).toBe("/winner-page");
			expect(document.title).toBe("Navigation Winner");
			expect(api.getBuildID()).toBe("1");
			expect(buildIDEvents).toEqual([]);
			expectStatusIdle(api.getStatus());
		} finally {
			removeBuildIDListener();
		}
	});

	it("does not let a stale browser-history POP completion override a newer user navigation", async () => {
		const api = await loadPublicClientAPI();
		const history = api.getHistoryInstance();
		const stalePOPDeferred = createDeferred<Response>();
		const buildIDEvents: Array<{ newID: string; oldID: string }> = [];
		const removeBuildIDListener = api.addBuildIDListener((event) => {
			buildIDEvents.push(event.detail);
		});
		let fetchCallCount = 0;
		const fetchSpy = vi.spyOn(window, "fetch").mockImplementation(() => {
			fetchCallCount += 1;
			if (fetchCallCount === 1) {
				return stalePOPDeferred.promise;
			}
			if (fetchCallCount === 2) {
				return Promise.resolve(
					createRouteDataResponse({
						loadersData: [],
						title: { dangerousInnerHTML: "POP Winner" },
					}),
				);
			}
			throw new Error("Unexpected fetch call");
		});

		try {
			history.push("/stale-pop-target");
			await vi.runAllTimersAsync();
			history.back();
			await waitForRequestCount({
				requests: fetchSpy.mock.calls,
				count: 1,
			});

			await api.vormaNavigate("/pop-winner");
			await vi.runAllTimersAsync();

			stalePOPDeferred.resolve(
				createRouteDataResponse(
					{
						loadersData: [],
						title: { dangerousInnerHTML: "Stale POP Applied" },
					},
					{
						headers: {
							"X-Wave-Framework-Build-Id": "stale-pop-build",
						},
					},
				),
			);
			await vi.runAllTimersAsync();

			expect(fetchCallCount).toBe(2);
			expect(window.location.pathname).toBe("/pop-winner");
			expect(document.title).toBe("POP Winner");
			expect(api.getBuildID()).toBe("1");
			expect(buildIDEvents).toEqual([]);
			expectStatusIdle(api.getStatus());
		} finally {
			removeBuildIDListener();
		}
	});

	it("does not follow stale client-redirect headers from an aborted user navigation", async () => {
		const api = await loadPublicClientAPI();
		const staleClientRedirectDeferred = createDeferred<Response>();
		const buildIDEvents: Array<{ newID: string; oldID: string }> = [];
		const removeBuildIDListener = api.addBuildIDListener((event) => {
			buildIDEvents.push(event.detail);
		});
		let fetchCallCount = 0;
		const fetchSpy = vi.spyOn(window, "fetch").mockImplementation(() => {
			fetchCallCount += 1;
			if (fetchCallCount === 1) {
				return staleClientRedirectDeferred.promise;
			}
			if (fetchCallCount === 2) {
				return Promise.resolve(
					createRouteDataResponse({
						loadersData: [],
						title: { dangerousInnerHTML: "Soft Redirect Winner" },
					}),
				);
			}
			return Promise.resolve(
				createRouteDataResponse({
					loadersData: [],
					title: {
						dangerousInnerHTML: "Stale Soft Redirect Was Followed",
					},
				}),
			);
		});

		try {
			const staleNavigation = api.vormaNavigate("/stale-soft-start");
			await vi.advanceTimersByTimeAsync(8);

			await api.vormaNavigate("/soft-winner");
			await vi.runAllTimersAsync();

			staleClientRedirectDeferred.resolve(
				createRouteDataResponse(
					{},
					{
						headers: {
							"X-Client-Redirect": "/stale-soft-target",
							"X-Wave-Framework-Build-Id": "stale-soft-nav-build",
						},
					},
				),
			);
			await staleNavigation;
			await vi.runAllTimersAsync();

			expect(fetchSpy).toHaveBeenCalledTimes(2);
			expect(window.location.pathname).toBe("/soft-winner");
			expect(document.title).toBe("Soft Redirect Winner");
			expect(api.getBuildID()).toBe("1");
			expect(buildIDEvents).toEqual([]);
			expectStatusIdle(api.getStatus());
		} finally {
			removeBuildIDListener();
		}
	});

	it("does not hard-reload from stale aborted user-navigation responses", async () => {
		const api = await loadPublicClientAPI();
		const staleHardReloadDeferred = createDeferred<Response>();
		const buildIDEvents: Array<{ newID: string; oldID: string }> = [];
		const removeBuildIDListener = api.addBuildIDListener((event) => {
			buildIDEvents.push(event.detail);
		});
		let fetchCallCount = 0;
		const fetchSpy = vi.spyOn(window, "fetch").mockImplementation(() => {
			fetchCallCount += 1;
			if (fetchCallCount === 1) {
				return staleHardReloadDeferred.promise;
			}
			return Promise.resolve(
				createRouteDataResponse({
					loadersData: [],
					title: { dangerousInnerHTML: "Hard Reload Winner" },
				}),
			);
		});
		let locationHrefStub: ReturnType<typeof stubWindowLocationHref> | null =
			null;
		try {
			const staleNavigation = api.vormaNavigate("/stale-hard-start");
			await vi.advanceTimersByTimeAsync(8);

			await api.vormaNavigate("/hard-winner");
			await vi.runAllTimersAsync();
			expect(window.location.pathname).toBe("/hard-winner");

			locationHrefStub = stubWindowLocationHref(window.location.href);
			staleHardReloadDeferred.resolve(
				createRouteDataResponse(
					{},
					{
						headers: {
							"X-Wave-Framework-Reload": "/stale-hard-redirect",
							"X-Wave-Framework-Build-Id": "stale-hard-nav-build",
						},
					},
				),
			);
			await staleNavigation;
			await vi.runAllTimersAsync();

			expect(fetchSpy).toHaveBeenCalledTimes(2);
			expect(locationHrefStub.getHref()).toContain("/hard-winner");
			expect(locationHrefStub.getHref()).not.toContain(
				"/stale-hard-redirect",
			);
			expect(api.getBuildID()).toBe("1");
			expect(buildIDEvents).toEqual([]);
			expectStatusIdle(api.getStatus());
		} finally {
			removeBuildIDListener();
			locationHrefStub?.restore();
		}
	});
});
