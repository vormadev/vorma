import { h, render as renderPreact } from "preact";
import {
	useEffect as usePreactEffect,
	useRef as usePreactRef,
	useState as usePreactState,
} from "preact/hooks";
import { act } from "preact/test-utils";
import React from "react";
import { flushSync } from "react-dom";
import { createRoot } from "react-dom/client";
import { createComponent, createEffect, createMemo, onCleanup } from "solid-js";
import { render as renderSolid } from "solid-js/web";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { VormaAppBase, VormaAppConfig } from "vorma/client";
import {
	clearAllNavigationStateForTesting,
	createIsolatedClientTestRuntime,
	readRouterDataForTesting,
	readScrollStateForTesting,
	simulateViteAfterUpdateForTesting,
} from "vorma/testing";

type DistTypeGuardApp = {
	appConfig: VormaAppConfig;
	rootData: null;
	routes: readonly [
		{
			_type: "loader";
			pattern: "/users/:id";
			params: readonly ["id"];
			phantomOutputType: { id: string };
		},
		{
			_type: "loader";
			pattern: "/*";
			isSplat: true;
			phantomOutputType: { slug: string };
		},
	];
};

const DIST_TYPE_GUARD_VORMA_APP_CONFIG = {
	actionsRouterMountRoot: "/api/",
	actionsDynamicRune: ":",
	actionsSplatRune: "*",
	loadersDynamicRune: ":",
	loadersSplatRune: "*",
	loadersExplicitIndexSegmentIdentifier: "_index",
	__phantom: undefined as unknown as DistTypeGuardApp,
} as const satisfies VormaAppConfig;

function assertDistApiTypeGuardrails(): void {
	type ClientSurface = typeof import("vorma/client");
	type PreactAdapterSurface =
		typeof import("../../../../npm_dist/typescript/vorma/ui-adapters/preact/index.js");
	type ReactAdapterSurface =
		typeof import("../../../../npm_dist/typescript/vorma/ui-adapters/react/index.js");
	type SolidAdapterSurface =
		typeof import("../../../../npm_dist/typescript/vorma/ui-adapters/solid/index.js");

	const clientSurface = null as unknown as ClientSurface;
	const preactAdapterSurface = null as unknown as PreactAdapterSurface;
	const reactAdapterSurface = null as unknown as ReactAdapterSurface;
	const solidAdapterSurface = null as unknown as SolidAdapterSurface;

	const typedNavigate = clientSurface.makeTypedNavigate(
		DIST_TYPE_GUARD_VORMA_APP_CONFIG,
	);
	typedNavigate({
		pattern: "/users/:id",
		params: { id: "123" },
	});
	typedNavigate({
		pattern: "/*",
		splatValues: ["docs", "getting-started"],
	});

	// @ts-expect-error params are required for /users/:id.
	typedNavigate({ pattern: "/users/:id" });
	// @ts-expect-error params key must match route param names.
	typedNavigate({ pattern: "/users/:id", params: { slug: "123" } });
	// @ts-expect-error splatValues are required for splat routes.
	typedNavigate({ pattern: "/*" });

	const addSolidClientLoader = solidAdapterSurface.makeTypedAddClientLoader(
		DIST_TYPE_GUARD_VORMA_APP_CONFIG,
	);
	const useSplatClientLoaderData = addSolidClientLoader({
		pattern: "/*",
		clientLoader: async () => "title",
		reRunOnModuleChange: import.meta,
	});
	const maybeClientLoaderData = useSplatClientLoaderData();
	const maybeTitle = maybeClientLoaderData();
	if (maybeTitle !== undefined) {
		const title: string = maybeTitle;
		void title;
	}

	clientSurface.getHistoryInstance();

	// @ts-expect-error raw React store helpers should remain internal-only.
	reactAdapterSurface.useLoadersData;
	// @ts-expect-error raw React store helpers should remain internal-only.
	reactAdapterSurface.useClientLoadersData;
	// @ts-expect-error raw React store helpers should remain internal-only.
	reactAdapterSurface.useRouterData;

	// @ts-expect-error raw Preact stores should remain internal-only.
	preactAdapterSurface.loadersData;
	// @ts-expect-error raw Preact stores should remain internal-only.
	preactAdapterSurface.clientLoadersData;
	// @ts-expect-error raw Preact stores should remain internal-only.
	preactAdapterSurface.routerData;

	// @ts-expect-error raw Solid stores should remain internal-only.
	solidAdapterSurface.loadersData;
	// @ts-expect-error raw Solid stores should remain internal-only.
	solidAdapterSurface.clientLoadersData;
	// @ts-expect-error raw Solid stores should remain internal-only.
	solidAdapterSurface.routerData;
}
void assertDistApiTypeGuardrails;

type DistTestLoaderRoute = {
	_type: "loader";
	pattern: string;
	params?: ReadonlyArray<string>;
	isSplat?: boolean;
	method?: string;
	phantomOutputType: unknown;
};

type DistTestQueryRoute = {
	_type: "query";
	pattern: string;
	method?: "GET";
	params?: ReadonlyArray<string>;
	isSplat?: boolean;
	phantomInputType: Record<string, unknown> | null | undefined;
	phantomOutputType: unknown;
};

type DistTestMutationRoute = {
	_type: "mutation";
	pattern: string;
	method: string;
	params?: ReadonlyArray<string>;
	isSplat?: boolean;
	phantomInputType: Record<string, unknown> | null | undefined;
	phantomOutputType: unknown;
};

type DistTestPhantomApp = VormaAppBase & {
	routes: readonly [
		DistTestLoaderRoute,
		DistTestQueryRoute,
		DistTestMutationRoute,
	];
};

function defineDistTestVormaAppConfig<App extends VormaAppBase>(props: {
	actionsRouterMountRoot: string;
	actionsDynamicRune: string;
	actionsSplatRune: string;
	loadersDynamicRune: string;
	loadersSplatRune: string;
	loadersExplicitIndexSegmentIdentifier: string;
}): VormaAppConfig & { __phantom: App } {
	return props as VormaAppConfig & { __phantom: App };
}

const DIST_TEST_VORMA_APP_CONFIG =
	defineDistTestVormaAppConfig<DistTestPhantomApp>({
		actionsRouterMountRoot: "/api/",
		actionsDynamicRune: ":",
		actionsSplatRune: "*",
		loadersDynamicRune: ":",
		loadersSplatRune: "*",
		loadersExplicitIndexSegmentIdentifier: "_index",
	});

function createRouteDataResponse(
	overrides: Record<string, unknown> = {},
	init: ResponseInit = {},
): Response {
	return new Response(
		JSON.stringify({
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
			metaHeadEls: [],
			restHeadEls: [],
			...overrides,
		}),
		{
			status: init.status ?? 200,
			headers: {
				"Content-Type": "application/json",
				"X-Wave-Framework-Build-Id": "1",
				...init.headers,
			},
			...init,
		},
	);
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

const activeSolidRootDisposers = new Set<() => void>();

function renderSolidWithTrackedDisposer(
	renderer: () => unknown,
	container: Element,
): () => void {
	const dispose = renderSolid(renderer as any, container as any);
	activeSolidRootDisposers.add(dispose);
	return () => {
		dispose();
	};
}

function requestInputToURL(input: RequestInfo | URL): URL {
	if (input instanceof URL) {
		return input;
	}
	if (input instanceof Request) {
		return new URL(input.url, window.location.origin);
	}
	return new URL(String(input), window.location.origin);
}

type RecordedAbortAwareFetchRequest = {
	input: RequestInfo | URL;
	init?: RequestInit;
	url: URL;
	deferred: ReturnType<typeof createDeferred<Response>>;
};

function createAbortAwareFetchRecorder(): {
	requests: RecordedAbortAwareFetchRequest[];
} {
	const requests: RecordedAbortAwareFetchRequest[] = [];
	vi.spyOn(window, "fetch").mockImplementation((input, init) => {
		const deferred = createDeferred<Response>();
		const signal = (init as RequestInit | undefined)?.signal as
			| AbortSignal
			| undefined;
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
			input,
			init: init as RequestInit | undefined,
			url: requestInputToURL(input),
			deferred,
		});
		return deferred.promise;
	});
	return { requests };
}

function createSeededRandom(props: { seed: number }): () => number {
	let state = props.seed >>> 0;
	return () => {
		state = (state * 1664525 + 1013904223) >>> 0;
		return state / 0x100000000;
	};
}

function shuffledIndices(props: { length: number; seed: number }): number[] {
	const random = createSeededRandom({ seed: props.seed });
	const indices = Array.from({ length: props.length }, (_, index) => index);
	for (let i = indices.length - 1; i > 0; i -= 1) {
		const j = Math.floor(random() * (i + 1));
		const valueAtI = indices[i];
		indices[i] = indices[j]!;
		indices[j] = valueAtI!;
	}
	return indices;
}

type GeneratedOperation =
	| {
			kind: "prefetch";
			href: string;
	  }
	| {
			kind: "navigate";
			href: string;
	  };

function buildGeneratedNavigationModelSequence(props: { seed: number }): {
	operations: GeneratedOperation[];
	finalNavigationHref: string;
} {
	const random = createSeededRandom({ seed: props.seed });
	const operations: GeneratedOperation[] = [];
	const basePathnames = ["/model-seq-a", "/model-seq-b", "/model-seq-c"];
	const generatedCount = 14;

	for (let i = 0; i < generatedCount; i += 1) {
		const pathname =
			basePathnames[Math.floor(random() * basePathnames.length)]!;
		const hash = random() < 0.5 ? "" : `#h${Math.floor(random() * 4) + 1}`;
		const href = `${pathname}${hash}`;
		const kind = random() < 0.5 ? "prefetch" : "navigate";
		operations.push({ kind, href } as GeneratedOperation);
	}

	const finalNavigationHref = `/model-seq-final-${props.seed}#winner`;
	operations.push({
		kind: "navigate",
		href: finalNavigationHref,
	});

	return { operations, finalNavigationHref };
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

async function waitForDOMCondition(props: {
	assertion: () => void;
	maxTicks?: number;
}): Promise<void> {
	const { assertion, maxTicks = 30 } = props;
	let lastError: unknown;
	for (let i = 0; i < maxTicks; i += 1) {
		try {
			assertion();
			return;
		} catch (error) {
			lastError = error;
			await Promise.resolve();
			await vi.advanceTimersByTimeAsync(1);
		}
	}
	throw lastError;
}

function dispatchRouteChangeEventForTesting(): void {
	window.dispatchEvent(new CustomEvent("vorma:route-change"));
}

function dispatchLocationEventForTestingFromCurrentWindowLocation(): void {
	window.dispatchEvent(
		new PopStateEvent("popstate", {
			state: window.history.state,
		}),
	);
	dispatchVormaLocationEventForTestingFromCurrentWindowLocation();
}

function dispatchVormaLocationEventForTestingFromCurrentWindowLocation(): void {
	window.dispatchEvent(
		new CustomEvent("vorma:location", {
			detail: {
				pathname: window.location.pathname,
				search: window.location.search,
				hash: window.location.hash,
				state: window.history.state,
			},
		}),
	);
}

async function withUnhandledRejectionCapture<T>(props: {
	run: () => Promise<T>;
}): Promise<{ result: T; unhandledRejections: Array<unknown> }> {
	const unhandledRejections: Array<unknown> = [];
	const unhandledRejectionHandler = (reason: unknown) => {
		unhandledRejections.push(reason);
	};
	process.on("unhandledRejection", unhandledRejectionHandler);
	try {
		const result = await props.run();
		await Promise.resolve();
		return { result, unhandledRejections };
	} finally {
		process.off("unhandledRejection", unhandledRejectionHandler);
	}
}

function renderReactTypedLinkForTesting(props: {
	TypedLink: unknown;
	linkProps: Record<string, unknown>;
}): {
	anchor: HTMLAnchorElement;
	cleanup: () => void;
} {
	const container = document.createElement("div");
	document.body.appendChild(container);
	const root = createRoot(container);
	flushSync(() => {
		root.render(
			React.createElement(props.TypedLink as any, {
				...props.linkProps,
			}),
		);
	});
	const anchor = container.querySelector("a");
	if (!(anchor instanceof HTMLAnchorElement)) {
		root.unmount();
		container.remove();
		throw new Error("Expected react typed link to render an anchor.");
	}
	return {
		anchor,
		cleanup: () => {
			root.unmount();
			container.remove();
		},
	};
}

async function initializeDistRuntimeStateForAdapters(): Promise<void> {
	vi.resetModules();
	createIsolatedClientTestRuntime();
	const client = await import("vorma/client");
	await client.initClient({
		vormaAppConfig: DIST_TEST_VORMA_APP_CONFIG,
	});
}

async function navigateWithDistRouteDataResponse(props: {
	client: Awaited<typeof import("vorma/client")>;
	href: string;
	overrides?: Record<string, unknown>;
}): Promise<void> {
	vi.spyOn(window, "fetch").mockResolvedValueOnce(
		createRouteDataResponse(props.overrides ?? {}),
	);
	await props.client.vormaNavigate(props.href);
	await vi.runAllTimersAsync();
}

type DistAdapterName = "react" | "preact" | "solid";
type TransitionTracePhase = "initial" | "to-root" | "back-to-probe";
type TransitionTraceEntry = {
	phase: TransitionTracePhase;
	value: string;
	matchedPattern: string;
};

function summarizeTransitionTraceByLastPhaseEntry(props: {
	trace: TransitionTraceEntry[];
}): TransitionTraceEntry[] {
	const phases: TransitionTracePhase[] = [
		"initial",
		"to-root",
		"back-to-probe",
	];
	return phases.map((phase) => {
		const lastPhaseEntry = [...props.trace]
			.reverse()
			.find((entry) => entry.phase === phase);
		if (!lastPhaseEntry) {
			throw new Error(
				`Missing transition trace entry for phase "${phase}".`,
			);
		}
		return lastPhaseEntry;
	});
}

async function captureRoutePropsTransitionTraceByAdapter(props: {
	adapterName: DistAdapterName;
}): Promise<TransitionTraceEntry[]> {
	await initializeDistRuntimeStateForAdapters();
	const client = await import("vorma/client");
	let phase: TransitionTracePhase = "initial";
	const trace: TransitionTraceEntry[] = [];
	const modulePath = `/parity-route-props-${props.adapterName}.js`;
	const baseHref = `/parity-route-props-${props.adapterName}`;

	if (props.adapterName === "react") {
		const reactAdapter = await import("vorma/react");
		const useLoaderData = reactAdapter.makeTypedUseLoaderData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const useRouterData = reactAdapter.makeTypedUseRouterData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const usePatternClientLoaderData =
			reactAdapter.makeTypedAddClientLoader(DIST_TEST_VORMA_APP_CONFIG)({
				pattern: "/probe",
				clientLoader: async ({ serverDataPromise }) => {
					const serverData = await serverDataPromise;
					return (
						serverData.loaderData as { value?: string } | undefined
					)?.value;
				},
			});
		const RouteComponent = (routeProps: any) => {
			const loaderData = useLoaderData(routeProps) as
				| { value?: string }
				| undefined;
			const clientLoaderData = usePatternClientLoaderData(routeProps) as
				| string
				| undefined;
			const routerData = useRouterData();
			const combinedValue = `${loaderData?.value ?? "none"}|${clientLoaderData ?? "none"}`;
			trace.push({
				phase,
				value: combinedValue,
				matchedPattern: routerData.matchedPatterns[0] ?? "<none>",
			});
			return React.createElement(
				"div",
				{ "data-react-route-props-parity-probe": true },
				combinedValue,
			);
		};
		vi.doMock(modulePath, () => ({
			default: RouteComponent,
		}));
		vi.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/probe"],
					importURLs: [modulePath],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{ value: "loader-a" }],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/probe"],
					importURLs: [modulePath],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{ value: "loader-b" }],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/probe"],
					importURLs: [modulePath],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{ value: "loader-c" }],
				}),
			);
		const container = document.createElement("div");
		document.body.appendChild(container);
		const root = createRoot(container);
		try {
			flushSync(() => {
				root.render(
					React.createElement(reactAdapter.VormaRootOutlet as any, {
						idx: 0,
					}),
				);
			});
			await client.vormaNavigate(`${baseHref}-a`);
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector(
							"[data-react-route-props-parity-probe]",
						)?.textContent,
					).toBe("loader-a|loader-a");
				},
			});
			phase = "to-root";
			await client.vormaNavigate(`${baseHref}-b`);
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector(
							"[data-react-route-props-parity-probe]",
						)?.textContent,
					).toBe("loader-b|loader-b");
				},
			});
			phase = "back-to-probe";
			await client.vormaNavigate(`${baseHref}-c`);
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector(
							"[data-react-route-props-parity-probe]",
						)?.textContent,
					).toBe("loader-c|loader-c");
				},
			});
			return summarizeTransitionTraceByLastPhaseEntry({ trace });
		} finally {
			flushSync(() => {
				root.unmount();
			});
			container.remove();
		}
	}

	if (props.adapterName === "preact") {
		const preactAdapter = await import("vorma/preact");
		const useLoaderData = preactAdapter.makeTypedUseLoaderData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const useRouterData = preactAdapter.makeTypedUseRouterData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const usePatternClientLoaderData =
			preactAdapter.makeTypedAddClientLoader(DIST_TEST_VORMA_APP_CONFIG)({
				pattern: "/probe",
				clientLoader: async ({ serverDataPromise }) => {
					const serverData = await serverDataPromise;
					return (
						serverData.loaderData as { value?: string } | undefined
					)?.value;
				},
			});
		const RouteComponent = (routeProps: any) => {
			const loaderData = useLoaderData(routeProps) as
				| { value?: string }
				| undefined;
			const clientLoaderData = usePatternClientLoaderData(routeProps) as
				| string
				| undefined;
			const routerData = useRouterData();
			const combinedValue = `${loaderData?.value ?? "none"}|${clientLoaderData ?? "none"}`;
			trace.push({
				phase,
				value: combinedValue,
				matchedPattern: routerData.matchedPatterns[0] ?? "<none>",
			});
			return h(
				"div",
				{ "data-preact-route-props-parity-probe": true },
				combinedValue,
			);
		};
		vi.doMock(modulePath, () => ({
			default: RouteComponent,
		}));
		vi.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/probe"],
					importURLs: [modulePath],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{ value: "loader-a" }],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/probe"],
					importURLs: [modulePath],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{ value: "loader-b" }],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/probe"],
					importURLs: [modulePath],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{ value: "loader-c" }],
				}),
			);
		const container = document.createElement("div");
		document.body.appendChild(container);
		try {
			await act(async () => {
				renderPreact(
					h(preactAdapter.VormaRootOutlet as any, {
						idx: 0,
					}),
					container,
				);
			});
			await client.vormaNavigate(`${baseHref}-a`);
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector(
							"[data-preact-route-props-parity-probe]",
						)?.textContent,
					).toBe("loader-a|loader-a");
				},
			});
			phase = "to-root";
			await client.vormaNavigate(`${baseHref}-b`);
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector(
							"[data-preact-route-props-parity-probe]",
						)?.textContent,
					).toBe("loader-b|loader-b");
				},
			});
			phase = "back-to-probe";
			await client.vormaNavigate(`${baseHref}-c`);
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector(
							"[data-preact-route-props-parity-probe]",
						)?.textContent,
					).toBe("loader-c|loader-c");
				},
			});
			return summarizeTransitionTraceByLastPhaseEntry({ trace });
		} finally {
			renderPreact(null, container);
			container.remove();
		}
	}

	const solidAdapter = await import("vorma/solid");
	const useLoaderData = solidAdapter.makeTypedUseLoaderData(
		DIST_TEST_VORMA_APP_CONFIG,
	);
	const useRouterData = solidAdapter.makeTypedUseRouterData(
		DIST_TEST_VORMA_APP_CONFIG,
	);
	const usePatternClientLoaderData = solidAdapter.makeTypedAddClientLoader(
		DIST_TEST_VORMA_APP_CONFIG,
	)({
		pattern: "/probe",
		clientLoader: async ({ serverDataPromise }) => {
			const serverData = await serverDataPromise;
			return (serverData.loaderData as { value?: string } | undefined)
				?.value;
		},
	});
	const RouteComponent = (routeProps: any) => {
		const loaderData = useLoaderData(routeProps);
		const clientLoaderData = usePatternClientLoaderData(routeProps);
		const routerData = useRouterData();
		createEffect(() => {
			const nextLoaderData = loaderData() as
				| { value?: string }
				| undefined;
			const nextClientLoaderData = clientLoaderData() as
				| string
				| undefined;
			const nextRouterData = routerData();
			const combinedValue = `${nextLoaderData?.value ?? "none"}|${nextClientLoaderData ?? "none"}`;
			trace.push({
				phase,
				value: combinedValue,
				matchedPattern: nextRouterData.matchedPatterns[0] ?? "<none>",
			});
		});
		return "";
	};
	vi.doMock(modulePath, () => ({
		default: RouteComponent,
	}));
	vi.spyOn(window, "fetch")
		.mockResolvedValueOnce(
			createRouteDataResponse({
				matchedPatterns: ["/probe"],
				importURLs: [modulePath],
				exportKeys: ["default"],
				errorExportKeys: [""],
				loadersData: [{ value: "loader-a" }],
			}),
		)
		.mockResolvedValueOnce(
			createRouteDataResponse({
				matchedPatterns: ["/probe"],
				importURLs: [modulePath],
				exportKeys: ["default"],
				errorExportKeys: [""],
				loadersData: [{ value: "loader-b" }],
			}),
		)
		.mockResolvedValueOnce(
			createRouteDataResponse({
				matchedPatterns: ["/probe"],
				importURLs: [modulePath],
				exportKeys: ["default"],
				errorExportKeys: [""],
				loadersData: [{ value: "loader-c" }],
			}),
		);
	const container = document.createElement("div");
	document.body.appendChild(container);
	const dispose = renderSolidWithTrackedDisposer(() => {
		return createComponent(solidAdapter.VormaRootOutlet as any, {
			idx: 0,
		});
	}, container);
	try {
		await client.vormaNavigate(`${baseHref}-a`);
		await waitForDOMCondition({
			assertion: () => {
				expect(
					trace.some((entry) => {
						return (
							entry.phase === "initial" &&
							entry.value === "loader-a|loader-a" &&
							entry.matchedPattern === "/probe"
						);
					}),
				).toBe(true);
			},
		});
		phase = "to-root";
		await client.vormaNavigate(`${baseHref}-b`);
		await waitForDOMCondition({
			assertion: () => {
				expect(
					trace.some((entry) => {
						return (
							entry.phase === "to-root" &&
							entry.value === "loader-b|loader-b" &&
							entry.matchedPattern === "/probe"
						);
					}),
				).toBe(true);
			},
		});
		phase = "back-to-probe";
		await client.vormaNavigate(`${baseHref}-c`);
		await waitForDOMCondition({
			assertion: () => {
				expect(
					trace.some((entry) => {
						return (
							entry.phase === "back-to-probe" &&
							entry.value === "loader-c|loader-c" &&
							entry.matchedPattern === "/probe"
						);
					}),
				).toBe(true);
			},
		});
		return summarizeTransitionTraceByLastPhaseEntry({ trace });
	} finally {
		dispose();
		container.remove();
	}
}

async function captureGlobalSelectorTransitionTraceByAdapter(props: {
	adapterName: DistAdapterName;
}): Promise<TransitionTraceEntry[]> {
	await initializeDistRuntimeStateForAdapters();
	const client = await import("vorma/client");
	let phase: TransitionTracePhase = "initial";
	const trace: TransitionTraceEntry[] = [];
	const rootModulePath = `/parity-global-root-${props.adapterName}.js`;
	const baseHref = `/parity-global-${props.adapterName}`;

	if (props.adapterName === "react") {
		const reactAdapter = await import("vorma/react");
		const useRouterData = reactAdapter.makeTypedUseRouterData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const usePatternLoaderData = reactAdapter.makeTypedUsePatternLoaderData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const usePatternClientLoaderData =
			reactAdapter.makeTypedAddClientLoader(DIST_TEST_VORMA_APP_CONFIG)({
				pattern: "/probe",
				clientLoader: async ({ serverDataPromise }) => {
					const serverData = await serverDataPromise;
					return (
						serverData.loaderData as { value?: string } | undefined
					)?.value;
				},
			});
		const GlobalProbe = () => {
			const routerData = useRouterData();
			const matchedPattern = (routerData.matchedPatterns[0] ??
				"/probe") as any;
			const loaderData = usePatternLoaderData(matchedPattern) as
				| { value?: string }
				| undefined;
			const clientLoaderData = usePatternClientLoaderData() as
				| string
				| undefined;
			const combinedValue = `${loaderData?.value ?? "none"}|${clientLoaderData ?? "none"}`;
			trace.push({
				phase,
				value: combinedValue,
				matchedPattern: routerData.matchedPatterns[0] ?? "<none>",
			});
			return React.createElement(
				"div",
				{ "data-react-global-parity-probe": true },
				combinedValue,
			);
		};
		vi.doMock(rootModulePath, () => ({
			default: () => React.createElement("div", {}, "root"),
		}));
		vi.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/probe"],
					importURLs: [rootModulePath],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{ value: "loader-a" }],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/root"],
					importURLs: [rootModulePath],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{ value: "root-loader-b" }],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/probe"],
					importURLs: [rootModulePath],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{ value: "loader-c" }],
				}),
			);
		const container = document.createElement("div");
		document.body.appendChild(container);
		const root = createRoot(container);
		try {
			flushSync(() => {
				root.render(
					React.createElement(
						React.Fragment,
						{},
						React.createElement(
							reactAdapter.VormaRootOutlet as any,
							{
								idx: 0,
							},
						),
						React.createElement(GlobalProbe),
					),
				);
			});
			await client.vormaNavigate(`${baseHref}-a`);
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector(
							"[data-react-global-parity-probe]",
						)?.textContent,
					).toBe("loader-a|loader-a");
				},
			});
			phase = "to-root";
			await client.vormaNavigate(`${baseHref}-b`);
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector(
							"[data-react-global-parity-probe]",
						)?.textContent,
					).toBe("root-loader-b|none");
				},
			});
			phase = "back-to-probe";
			await client.vormaNavigate(`${baseHref}-c`);
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector(
							"[data-react-global-parity-probe]",
						)?.textContent,
					).toBe("loader-c|loader-c");
				},
			});
			return summarizeTransitionTraceByLastPhaseEntry({ trace });
		} finally {
			flushSync(() => {
				root.unmount();
			});
			container.remove();
		}
	}

	if (props.adapterName === "preact") {
		const preactAdapter = await import("vorma/preact");
		const useRouterData = preactAdapter.makeTypedUseRouterData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const usePatternLoaderData =
			preactAdapter.makeTypedUsePatternLoaderData(
				DIST_TEST_VORMA_APP_CONFIG,
			);
		const usePatternClientLoaderData =
			preactAdapter.makeTypedAddClientLoader(DIST_TEST_VORMA_APP_CONFIG)({
				pattern: "/probe",
				clientLoader: async ({ serverDataPromise }) => {
					const serverData = await serverDataPromise;
					return (
						serverData.loaderData as { value?: string } | undefined
					)?.value;
				},
			});
		const GlobalProbe = () => {
			const routerData = useRouterData();
			const matchedPattern = (routerData.matchedPatterns[0] ??
				"/probe") as any;
			const loaderData = usePatternLoaderData(matchedPattern) as
				| { value?: string }
				| undefined;
			const clientLoaderData = usePatternClientLoaderData() as
				| string
				| undefined;
			const combinedValue = `${loaderData?.value ?? "none"}|${clientLoaderData ?? "none"}`;
			trace.push({
				phase,
				value: combinedValue,
				matchedPattern: routerData.matchedPatterns[0] ?? "<none>",
			});
			return h(
				"div",
				{ "data-preact-global-parity-probe": true },
				combinedValue,
			);
		};
		vi.doMock(rootModulePath, () => ({
			default: () => h("div", {}, "root"),
		}));
		vi.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/probe"],
					importURLs: [rootModulePath],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{ value: "loader-a" }],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/root"],
					importURLs: [rootModulePath],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{ value: "root-loader-b" }],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/probe"],
					importURLs: [rootModulePath],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{ value: "loader-c" }],
				}),
			);
		const container = document.createElement("div");
		document.body.appendChild(container);
		try {
			await act(async () => {
				renderPreact(
					h(
						"div",
						{},
						h(preactAdapter.VormaRootOutlet as any, {
							idx: 0,
						}),
						h(GlobalProbe, {}),
					),
					container,
				);
			});
			await client.vormaNavigate(`${baseHref}-a`);
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector(
							"[data-preact-global-parity-probe]",
						)?.textContent,
					).toBe("loader-a|loader-a");
				},
			});
			phase = "to-root";
			await client.vormaNavigate(`${baseHref}-b`);
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector(
							"[data-preact-global-parity-probe]",
						)?.textContent,
					).toBe("root-loader-b|none");
				},
			});
			phase = "back-to-probe";
			await client.vormaNavigate(`${baseHref}-c`);
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector(
							"[data-preact-global-parity-probe]",
						)?.textContent,
					).toBe("loader-c|loader-c");
				},
			});
			return summarizeTransitionTraceByLastPhaseEntry({ trace });
		} finally {
			renderPreact(null, container);
			container.remove();
		}
	}

	const solidAdapter = await import("vorma/solid");
	const useRouterData = solidAdapter.makeTypedUseRouterData(
		DIST_TEST_VORMA_APP_CONFIG,
	);
	const usePatternLoaderData = solidAdapter.makeTypedUsePatternLoaderData(
		DIST_TEST_VORMA_APP_CONFIG,
	);
	const usePatternClientLoaderData = solidAdapter.makeTypedAddClientLoader(
		DIST_TEST_VORMA_APP_CONFIG,
	)({
		pattern: "/probe",
		clientLoader: async ({ serverDataPromise }) => {
			const serverData = await serverDataPromise;
			return (serverData.loaderData as { value?: string } | undefined)
				?.value;
		},
	});
	const GlobalProbe = () => {
		const routerData = useRouterData();
		const loaderData = createMemo(() => {
			const matchedPattern = (routerData().matchedPatterns[0] ??
				"/probe") as any;
			return usePatternLoaderData(matchedPattern)();
		});
		const clientLoaderData = usePatternClientLoaderData();
		const node = document.createElement("div");
		node.setAttribute("data-solid-global-parity-probe", "true");
		createEffect(() => {
			const nextRouterData = routerData();
			const nextLoaderData = loaderData() as
				| { value?: string }
				| undefined;
			const nextClientLoaderData = clientLoaderData() as
				| string
				| undefined;
			const combinedValue = `${nextLoaderData?.value ?? "none"}|${nextClientLoaderData ?? "none"}`;
			trace.push({
				phase,
				value: combinedValue,
				matchedPattern: nextRouterData.matchedPatterns[0] ?? "<none>",
			});
			node.textContent = combinedValue;
		});
		return node;
	};
	vi.doMock(rootModulePath, () => ({
		default: () => "",
	}));
	vi.spyOn(window, "fetch")
		.mockResolvedValueOnce(
			createRouteDataResponse({
				matchedPatterns: ["/probe"],
				importURLs: [rootModulePath],
				exportKeys: ["default"],
				errorExportKeys: [""],
				loadersData: [{ value: "loader-a" }],
			}),
		)
		.mockResolvedValueOnce(
			createRouteDataResponse({
				matchedPatterns: ["/root"],
				importURLs: [rootModulePath],
				exportKeys: ["default"],
				errorExportKeys: [""],
				loadersData: [{ value: "root-loader-b" }],
			}),
		)
		.mockResolvedValueOnce(
			createRouteDataResponse({
				matchedPatterns: ["/probe"],
				importURLs: [rootModulePath],
				exportKeys: ["default"],
				errorExportKeys: [""],
				loadersData: [{ value: "loader-c" }],
			}),
		);
	const container = document.createElement("div");
	document.body.appendChild(container);
	const dispose = renderSolidWithTrackedDisposer(() => {
		return [
			createComponent(solidAdapter.VormaRootOutlet as any, {
				idx: 0,
			}),
			createComponent(GlobalProbe as any, {}),
		] as any;
	}, container);
	try {
		await client.vormaNavigate(`${baseHref}-a`);
		await waitForDOMCondition({
			assertion: () => {
				expect(
					container.querySelector("[data-solid-global-parity-probe]")
						?.textContent,
				).toBe("loader-a|loader-a");
			},
		});
		phase = "to-root";
		await client.vormaNavigate(`${baseHref}-b`);
		await waitForDOMCondition({
			assertion: () => {
				expect(
					container.querySelector("[data-solid-global-parity-probe]")
						?.textContent,
				).toBe("root-loader-b|none");
			},
		});
		phase = "back-to-probe";
		await client.vormaNavigate(`${baseHref}-c`);
		await waitForDOMCondition({
			assertion: () => {
				expect(
					container.querySelector("[data-solid-global-parity-probe]")
						?.textContent,
				).toBe("loader-c|loader-c");
			},
		});
		return summarizeTransitionTraceByLastPhaseEntry({ trace });
	} finally {
		dispose();
		container.remove();
	}
}

async function captureNestedRouteRemountValuesByAdapter(props: {
	adapterName: DistAdapterName;
}): Promise<[string | null, string | null]> {
	await initializeDistRuntimeStateForAdapters();
	const client = await import("vorma/client");
	const rootModulePath = `/parity-nested-remount-root-${props.adapterName}.js`;
	const childAModulePath = `/parity-nested-remount-child-a-${props.adapterName}.js`;
	const childBModulePath = `/parity-nested-remount-child-b-${props.adapterName}.js`;
	const baseHref = `/parity-nested-remount-${props.adapterName}`;

	if (props.adapterName === "react") {
		const reactAdapter = await import("vorma/react");
		const useLoaderData = reactAdapter.makeTypedUseLoaderData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const RootComponent = (routeProps: any) => {
			return React.createElement(routeProps.Outlet as any, {});
		};
		const SharedChildComponent = (routeProps: any) => {
			const loaderData = useLoaderData(routeProps) as
				| { value?: string }
				| undefined;
			return React.createElement(
				"div",
				{ "data-react-nested-remount-parity-probe": true },
				loaderData?.value ?? "none",
			);
		};
		vi.doMock(rootModulePath, () => ({
			default: RootComponent,
		}));
		vi.doMock(childAModulePath, () => ({
			default: SharedChildComponent,
		}));
		vi.doMock(childBModulePath, () => ({
			default: SharedChildComponent,
		}));
		vi.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/root", "/probe"],
					importURLs: [rootModulePath, childAModulePath],
					exportKeys: ["default", "default"],
					errorExportKeys: ["", ""],
					loadersData: [{ value: "root-a" }, { value: "loader-a" }],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/root", "/other"],
					importURLs: [rootModulePath, childBModulePath],
					exportKeys: ["default", "default"],
					errorExportKeys: ["", ""],
					loadersData: [{ value: "root-b" }, { value: "loader-b" }],
				}),
			);
		const container = document.createElement("div");
		document.body.appendChild(container);
		const root = createRoot(container);
		try {
			flushSync(() => {
				root.render(
					React.createElement(reactAdapter.VormaRootOutlet as any, {
						idx: 0,
					}),
				);
			});
			await client.vormaNavigate(`${baseHref}-a`);
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector(
							"[data-react-nested-remount-parity-probe]",
						)?.textContent,
					).toBe("loader-a");
				},
			});
			const initialValue =
				container.querySelector(
					"[data-react-nested-remount-parity-probe]",
				)?.textContent ?? null;
			await client.vormaNavigate(`${baseHref}-b`);
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector(
							"[data-react-nested-remount-parity-probe]",
						)?.textContent,
					).toBe("loader-b");
				},
			});
			const remountedValue =
				container.querySelector(
					"[data-react-nested-remount-parity-probe]",
				)?.textContent ?? null;
			return [initialValue, remountedValue];
		} finally {
			flushSync(() => {
				root.unmount();
			});
			container.remove();
		}
	}

	if (props.adapterName === "preact") {
		const preactAdapter = await import("vorma/preact");
		const useLoaderData = preactAdapter.makeTypedUseLoaderData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const RootComponent = (routeProps: any) => {
			return h(routeProps.Outlet as any, {});
		};
		const SharedChildComponent = (routeProps: any) => {
			const loaderData = useLoaderData(routeProps) as
				| { value?: string }
				| undefined;
			return h(
				"div",
				{ "data-preact-nested-remount-parity-probe": true },
				loaderData?.value ?? "none",
			);
		};
		vi.doMock(rootModulePath, () => ({
			default: RootComponent,
		}));
		vi.doMock(childAModulePath, () => ({
			default: SharedChildComponent,
		}));
		vi.doMock(childBModulePath, () => ({
			default: SharedChildComponent,
		}));
		vi.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/root", "/probe"],
					importURLs: [rootModulePath, childAModulePath],
					exportKeys: ["default", "default"],
					errorExportKeys: ["", ""],
					loadersData: [{ value: "root-a" }, { value: "loader-a" }],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/root", "/other"],
					importURLs: [rootModulePath, childBModulePath],
					exportKeys: ["default", "default"],
					errorExportKeys: ["", ""],
					loadersData: [{ value: "root-b" }, { value: "loader-b" }],
				}),
			);
		const container = document.createElement("div");
		document.body.appendChild(container);
		try {
			await act(async () => {
				renderPreact(
					h(preactAdapter.VormaRootOutlet as any, { idx: 0 }),
					container,
				);
			});
			await client.vormaNavigate(`${baseHref}-a`);
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector(
							"[data-preact-nested-remount-parity-probe]",
						)?.textContent,
					).toBe("loader-a");
				},
			});
			const initialValue =
				container.querySelector(
					"[data-preact-nested-remount-parity-probe]",
				)?.textContent ?? null;
			await client.vormaNavigate(`${baseHref}-b`);
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector(
							"[data-preact-nested-remount-parity-probe]",
						)?.textContent,
					).toBe("loader-b");
				},
			});
			const remountedValue =
				container.querySelector(
					"[data-preact-nested-remount-parity-probe]",
				)?.textContent ?? null;
			return [initialValue, remountedValue];
		} finally {
			renderPreact(null, container);
			container.remove();
		}
	}

	const solidAdapter = await import("vorma/solid");
	const useLoaderData = solidAdapter.makeTypedUseLoaderData(
		DIST_TEST_VORMA_APP_CONFIG,
	);
	const observedLoaderValues: string[] = [];
	const RootComponent = (routeProps: any) => {
		return createComponent(routeProps.Outlet as any, {});
	};
	const SharedChildComponent = (routeProps: any) => {
		const loaderData = useLoaderData(routeProps);
		createEffect(() => {
			const nextLoaderData = loaderData() as
				| { value?: string }
				| undefined;
			observedLoaderValues.push(nextLoaderData?.value ?? "none");
		});
		return "";
	};
	vi.doMock(rootModulePath, () => ({
		default: RootComponent,
	}));
	vi.doMock(childAModulePath, () => ({
		default: SharedChildComponent,
	}));
	vi.doMock(childBModulePath, () => ({
		default: SharedChildComponent,
	}));
	vi.spyOn(window, "fetch")
		.mockResolvedValueOnce(
			createRouteDataResponse({
				matchedPatterns: ["/root", "/probe"],
				importURLs: [rootModulePath, childAModulePath],
				exportKeys: ["default", "default"],
				errorExportKeys: ["", ""],
				loadersData: [{ value: "root-a" }, { value: "loader-a" }],
			}),
		)
		.mockResolvedValueOnce(
			createRouteDataResponse({
				matchedPatterns: ["/root", "/other"],
				importURLs: [rootModulePath, childBModulePath],
				exportKeys: ["default", "default"],
				errorExportKeys: ["", ""],
				loadersData: [{ value: "root-b" }, { value: "loader-b" }],
			}),
		);
	const container = document.createElement("div");
	document.body.appendChild(container);
	const dispose = renderSolidWithTrackedDisposer(() => {
		return createComponent(solidAdapter.VormaRootOutlet as any, {
			idx: 0,
		});
	}, container);
	try {
		await client.vormaNavigate(`${baseHref}-a`);
		await waitForDOMCondition({
			assertion: () => {
				expect(observedLoaderValues).toContain("loader-a");
			},
		});
		const initialValue =
			observedLoaderValues.find((value) => value === "loader-a") ?? null;
		await client.vormaNavigate(`${baseHref}-b`);
		await waitForDOMCondition({
			assertion: () => {
				expect(observedLoaderValues).toContain("loader-b");
			},
		});
		const remountedValue =
			[...observedLoaderValues]
				.reverse()
				.find((value) => value === "loader-b") ?? null;
		return [initialValue, remountedValue];
	} finally {
		dispose();
		container.remove();
	}
}

async function captureDataOnlyUpdateStabilityByAdapter(props: {
	adapterName: DistAdapterName;
}): Promise<{ mountCount: number }> {
	await initializeDistRuntimeStateForAdapters();
	const client = await import("vorma/client");
	const modulePath = `/parity-data-only-stable-${props.adapterName}.js`;
	const href = `/parity-data-only-stable-${props.adapterName}`;

	if (props.adapterName === "react") {
		const reactAdapter = await import("vorma/react");
		let stableRootMountCount = 0;
		const StableRootComp = () => {
			const mountIDRef = React.useRef<number | null>(null);
			if (mountIDRef.current === null) {
				stableRootMountCount += 1;
				mountIDRef.current = stableRootMountCount;
			}
			return React.createElement("div", {}, `root:${mountIDRef.current}`);
		};
		const useRouterData = reactAdapter.makeTypedUseRouterData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const usePatternLoaderData = reactAdapter.makeTypedUsePatternLoaderData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		let latestDataProbeValue = "";
		const DataProbe = () => {
			const routerData = useRouterData();
			const loaderData = usePatternLoaderData("/" as any) as
				| { value?: string }
				| undefined;
			latestDataProbeValue = `${loaderData?.value ?? ""}|${routerData.matchedPatterns.join(",")}`;
			return React.createElement("div", {}, latestDataProbeValue);
		};
		vi.doMock(modulePath, () => ({
			default: StableRootComp,
		}));
		vi.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/"],
					importURLs: [modulePath],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{ value: "a" }],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/"],
					importURLs: [modulePath],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{ value: "b" }],
				}),
			);
		const container = document.createElement("div");
		document.body.appendChild(container);
		const root = createRoot(container);
		try {
			flushSync(() => {
				root.render(
					React.createElement(
						React.Fragment,
						{},
						React.createElement(
							reactAdapter.VormaRootOutlet as any,
							{
								idx: 0,
							},
						),
						React.createElement(DataProbe),
					),
				);
			});
			await client.vormaNavigate(href);
			await waitForDOMCondition({
				assertion: () => {
					expect(latestDataProbeValue).toBe("a|/");
				},
			});
			await client.revalidate();
			await waitForDOMCondition({
				assertion: () => {
					expect(latestDataProbeValue).toBe("b|/");
				},
			});
			return {
				mountCount: stableRootMountCount,
			};
		} finally {
			flushSync(() => {
				root.unmount();
			});
			container.remove();
		}
	}

	if (props.adapterName === "preact") {
		const preactAdapter = await import("vorma/preact");
		let stableRootMountCount = 0;
		const StableRootComp = () => {
			usePreactState(() => {
				stableRootMountCount += 1;
				return stableRootMountCount;
			});
			return h("div", {}, "root");
		};
		const useRouterData = preactAdapter.makeTypedUseRouterData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const usePatternLoaderData =
			preactAdapter.makeTypedUsePatternLoaderData(
				DIST_TEST_VORMA_APP_CONFIG,
			);
		let latestDataProbeValue = "";
		const DataProbe = () => {
			const routerData = useRouterData();
			const loaderData = usePatternLoaderData("/" as any) as
				| { value?: string }
				| undefined;
			latestDataProbeValue = `${loaderData?.value ?? ""}|${routerData.matchedPatterns.join(",")}`;
			return h("div", {}, latestDataProbeValue);
		};
		vi.doMock(modulePath, () => ({
			default: StableRootComp,
		}));
		vi.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/"],
					importURLs: [modulePath],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{ value: "a" }],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/"],
					importURLs: [modulePath],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{ value: "b" }],
				}),
			);
		const container = document.createElement("div");
		document.body.appendChild(container);
		try {
			await act(async () => {
				renderPreact(
					h(
						"div",
						{},
						h(preactAdapter.VormaRootOutlet as any, { idx: 0 }),
						h(DataProbe, {}),
					),
					container,
				);
			});
			await client.vormaNavigate(href);
			await waitForDOMCondition({
				assertion: () => {
					expect(latestDataProbeValue).toBe("a|/");
				},
			});
			await client.revalidate();
			await waitForDOMCondition({
				assertion: () => {
					expect(latestDataProbeValue).toBe("b|/");
				},
			});
			return {
				mountCount: stableRootMountCount,
			};
		} finally {
			renderPreact(null, container);
			container.remove();
		}
	}

	const solidAdapter = await import("vorma/solid");
	let stableRootMountCount = 0;
	const StableRootComp = () => {
		stableRootMountCount += 1;
		return "solid-root";
	};
	const useRouterData = solidAdapter.makeTypedUseRouterData(
		DIST_TEST_VORMA_APP_CONFIG,
	);
	const usePatternLoaderData = solidAdapter.makeTypedUsePatternLoaderData(
		DIST_TEST_VORMA_APP_CONFIG,
	);
	let latestDataProbeValue = "";
	const DataProbe = () => {
		const routerData = useRouterData();
		const loaderData = usePatternLoaderData("/" as any);
		createEffect(() => {
			latestDataProbeValue = `${(loaderData() as { value?: string } | undefined)?.value ?? ""}|${routerData().matchedPatterns.join(",")}`;
		});
		return "";
	};
	vi.doMock(modulePath, () => ({
		default: StableRootComp,
	}));
	vi.spyOn(window, "fetch")
		.mockResolvedValueOnce(
			createRouteDataResponse({
				matchedPatterns: ["/"],
				importURLs: [modulePath],
				exportKeys: ["default"],
				errorExportKeys: [""],
				loadersData: [{ value: "a" }],
			}),
		)
		.mockResolvedValueOnce(
			createRouteDataResponse({
				matchedPatterns: ["/"],
				importURLs: [modulePath],
				exportKeys: ["default"],
				errorExportKeys: [""],
				loadersData: [{ value: "b" }],
			}),
		);
	const container = document.createElement("div");
	document.body.appendChild(container);
	const dispose = renderSolidWithTrackedDisposer(() => {
		return [
			createComponent(solidAdapter.VormaRootOutlet as any, {
				idx: 0,
			}),
			createComponent(DataProbe as any, {}),
		] as any;
	}, container);
	try {
		await client.vormaNavigate(href);
		await waitForDOMCondition({
			assertion: () => {
				expect(latestDataProbeValue).toBe("a|/");
			},
		});
		await client.revalidate();
		await waitForDOMCondition({
			assertion: () => {
				expect(latestDataProbeValue).toBe("b|/");
			},
		});
		return {
			mountCount: stableRootMountCount,
		};
	} finally {
		dispose();
		container.remove();
	}
}

beforeEach(() => {
	vi.useFakeTimers();
	vi.setSystemTime(new Date("2026-01-01T00:00:00.000Z"));
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
	Object.defineProperty(window, "scrollTo", {
		value: vi.fn(),
		writable: true,
		configurable: true,
	});
});

afterEach(async () => {
	activeSolidRootDisposers.forEach((dispose) => {
		dispose();
	});
	activeSolidRootDisposers.clear();
	await vi.runOnlyPendingTimersAsync();
	vi.useRealTimers();
	vi.restoreAllMocks();
	document.body.innerHTML = "";
	document.head.innerHTML = "";
});

describe("npm_dist adapter authoritative black-box contracts", () => {
	it("resolves loader paths from compiled internal exports", async () => {
		vi.resetModules();
		const distClientInternal = await import("vorma/client/__internal");
		const path = distClientInternal.resolvePath({
			vormaAppConfig: DIST_TEST_VORMA_APP_CONFIG,
			type: "loader",
			pattern: "/products/:id/_index",
			params: {
				id: "42",
			},
		});

		expect(path).toBe("/products/42");
	});

	it("react typed links let per-link props override makeTypedLink defaults", async () => {
		await initializeDistRuntimeStateForAdapters();
		const reactAdapter = await import("vorma/react");

		const TypedLink = reactAdapter.makeTypedLink(
			DIST_TEST_VORMA_APP_CONFIG,
			{
				className: "default-class",
			},
		);
		const { anchor, cleanup } = renderReactTypedLinkForTesting({
			TypedLink,
			linkProps: {
				pattern: "/products/:id",
				params: { id: "42" },
				search: "?q=abc",
				hash: "#panel",
				className: "custom-class",
			},
		});
		try {
			expect(anchor.getAttribute("href")).toBe(
				`${window.location.origin}/products/42?q=abc#panel`,
			);
			expect(anchor.className).toBe("custom-class");
		} finally {
			cleanup();
		}
	});

	it("react typed links apply makeTypedLink defaults when link props omit optional fields", async () => {
		await initializeDistRuntimeStateForAdapters();
		const reactAdapter = await import("vorma/react");

		const TypedLink = reactAdapter.makeTypedLink(
			DIST_TEST_VORMA_APP_CONFIG,
			{
				className: "default-class",
			},
		);
		const { anchor, cleanup } = renderReactTypedLinkForTesting({
			TypedLink,
			linkProps: {
				pattern: "/products/:id",
				params: { id: "42" },
				search: "?q=abc",
				hash: "#panel",
			},
		});
		try {
			expect(anchor.getAttribute("href")).toBe(
				`${window.location.origin}/products/42?q=abc#panel`,
			);
			expect(anchor.className).toBe("default-class");
		} finally {
			cleanup();
		}
	});

	it("preact typed links let per-link props override makeTypedLink defaults", async () => {
		await initializeDistRuntimeStateForAdapters();
		const preactAdapter = await import("vorma/preact");
		const container = document.createElement("div");
		document.body.appendChild(container);

		const TypedLink = preactAdapter.makeTypedLink(
			DIST_TEST_VORMA_APP_CONFIG,
			{
				className: "default-class",
			},
		);
		await act(async () => {
			renderPreact(
				h(TypedLink as any, {
					pattern: "/products/:id",
					params: { id: "42" },
					search: "?q=abc",
					hash: "#panel",
					className: "custom-class",
				}),
				container,
			);
		});

		const anchor = container.querySelector("a");
		expect(anchor).not.toBeNull();
		expect(anchor?.getAttribute("href")).toBe(
			`${window.location.origin}/products/42?q=abc#panel`,
		);
		expect(anchor?.className).toBe("custom-class");
		renderPreact(null, container);
		container.remove();
	});

	it("preact typed links apply makeTypedLink defaults when link props omit optional fields", async () => {
		await initializeDistRuntimeStateForAdapters();
		const preactAdapter = await import("vorma/preact");
		const container = document.createElement("div");
		document.body.appendChild(container);

		const TypedLink = preactAdapter.makeTypedLink(
			DIST_TEST_VORMA_APP_CONFIG,
			{
				className: "default-class",
			},
		);
		await act(async () => {
			renderPreact(
				h(TypedLink as any, {
					pattern: "/products/:id",
					params: { id: "42" },
					search: "?q=abc",
					hash: "#panel",
				}),
				container,
			);
		});

		const anchor = container.querySelector("a");
		expect(anchor).not.toBeNull();
		expect(anchor?.getAttribute("href")).toBe(
			`${window.location.origin}/products/42?q=abc#panel`,
		);
		expect(anchor?.className).toBe("default-class");
		renderPreact(null, container);
		container.remove();
	});

	it("solid typed links let per-link props override makeTypedLink defaults", async () => {
		await initializeDistRuntimeStateForAdapters();
		const solidAdapter = await import("vorma/solid");
		const container = document.createElement("div");
		document.body.appendChild(container);

		const TypedLink = solidAdapter.makeTypedLink(
			DIST_TEST_VORMA_APP_CONFIG,
			{
				class: "default-class",
			},
		);
		const dispose = renderSolidWithTrackedDisposer(() => {
			return (TypedLink as any)({
				pattern: "/products/:id",
				params: { id: "42" },
				search: "?q=abc",
				hash: "#panel",
				class: "custom-class",
				children: "Products",
			});
		}, container);

		const anchor = container.querySelector("a");
		expect(anchor).not.toBeNull();
		expect(anchor?.getAttribute("href")).toBe(
			`${window.location.origin}/products/42?q=abc#panel`,
		);
		expect(anchor?.getAttribute("class")).toBe("custom-class");
		dispose();
		container.remove();
	});

	it("solid typed links apply makeTypedLink defaults when link props omit optional fields", async () => {
		await initializeDistRuntimeStateForAdapters();
		const solidAdapter = await import("vorma/solid");
		const container = document.createElement("div");
		document.body.appendChild(container);

		const TypedLink = solidAdapter.makeTypedLink(
			DIST_TEST_VORMA_APP_CONFIG,
			{
				class: "default-class",
			},
		);
		const dispose = renderSolidWithTrackedDisposer(() => {
			return (TypedLink as any)({
				pattern: "/products/:id",
				params: { id: "42" },
				search: "?q=abc",
				hash: "#panel",
				children: "Products",
			});
		}, container);

		const anchor = container.querySelector("a");
		expect(anchor).not.toBeNull();
		expect(anchor?.getAttribute("href")).toBe(
			`${window.location.origin}/products/42?q=abc#panel`,
		);
		expect(anchor?.getAttribute("class")).toBe("default-class");
		dispose();
		container.remove();
	});

	it("react/preact/solid typed addClientLoader helpers register client loaders", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const reactAdapter = await import("vorma/react");
		const preactAdapter = await import("vorma/preact");
		const solidAdapter = await import("vorma/solid");
		const reactLoader = vi.fn(async ({ serverDataPromise }) => {
			await serverDataPromise;
			return { source: "react" };
		});
		const preactLoader = vi.fn(async ({ serverDataPromise }) => {
			await serverDataPromise;
			return { source: "preact" };
		});
		const solidLoader = vi.fn(async ({ serverDataPromise }) => {
			await serverDataPromise;
			return { source: "solid" };
		});
		reactAdapter.makeTypedAddClientLoader(DIST_TEST_VORMA_APP_CONFIG)({
			pattern: "/adapter-react/:id",
			clientLoader: reactLoader,
		});
		preactAdapter.makeTypedAddClientLoader(DIST_TEST_VORMA_APP_CONFIG)({
			pattern: "/adapter-preact/:id",
			clientLoader: preactLoader,
		});
		solidAdapter.makeTypedAddClientLoader(DIST_TEST_VORMA_APP_CONFIG)({
			pattern: "/adapter-solid/:id",
			clientLoader: solidLoader,
		});

		vi.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/adapter-react/:id"],
					loadersData: [{ id: "1" }],
					importURLs: ["/react.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/adapter-preact/:id"],
					loadersData: [{ id: "2" }],
					importURLs: ["/preact.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/adapter-solid/:id"],
					loadersData: [{ id: "3" }],
					importURLs: ["/solid.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
				}),
			);
		vi.doMock("/react.js", () => ({ default: () => null }));
		vi.doMock("/preact.js", () => ({ default: () => null }));
		vi.doMock("/solid.js", () => ({ default: () => null }));

		await client.vormaNavigate("/adapter-react/1");
		await vi.runAllTimersAsync();
		await client.vormaNavigate("/adapter-preact/2");
		await vi.runAllTimersAsync();
		await client.vormaNavigate("/adapter-solid/3");
		await vi.runAllTimersAsync();

		expect(reactLoader).toHaveBeenCalledTimes(1);
		expect(preactLoader).toHaveBeenCalledTimes(1);
		expect(solidLoader).toHaveBeenCalledTimes(1);
	});

	it("makeTypedAddClientLoader keeps the latest registration for duplicate patterns", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const reactAdapter = await import("vorma/react");
		const firstLoader = vi.fn(async ({ serverDataPromise }) => {
			await serverDataPromise;
			return { source: "first" };
		});
		const secondLoader = vi.fn(async ({ serverDataPromise }) => {
			await serverDataPromise;
			return { source: "second" };
		});
		const addClientLoader = reactAdapter.makeTypedAddClientLoader(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		addClientLoader({
			pattern: "/adapter-dup/:id",
			clientLoader: firstLoader,
		});
		addClientLoader({
			pattern: "/adapter-dup/:id",
			clientLoader: secondLoader,
		});
		vi.spyOn(window, "fetch").mockResolvedValueOnce(
			createRouteDataResponse({
				matchedPatterns: ["/adapter-dup/:id"],
				loadersData: [{ id: "42" }],
				importURLs: ["/dup.js"],
				exportKeys: ["default"],
				errorExportKeys: [""],
			}),
		);
		vi.doMock("/dup.js", () => ({ default: () => null }));

		await client.vormaNavigate("/adapter-dup/42");
		await vi.runAllTimersAsync();

		expect(firstLoader).not.toHaveBeenCalled();
		expect(secondLoader).toHaveBeenCalledTimes(1);
	});

	it("re-runs only opted-in matched client loaders after js HMR updates", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const reactAdapter = await import("vorma/react");
		const optedInLoader = vi.fn(async ({ serverDataPromise }) => {
			await serverDataPromise;
			return "opted-in";
		});
		const nonOptedLoader = vi.fn(async ({ serverDataPromise }) => {
			await serverDataPromise;
			return "non-opted";
		});
		const addClientLoader = reactAdapter.makeTypedAddClientLoader(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		addClientLoader({
			pattern: "/hmr-opted-in",
			clientLoader: optedInLoader,
			reRunOnModuleChange: {
				url: "http://localhost:3000/src/routes/hmr-opted-in.tsx?t=1",
			} as ImportMeta,
		});
		addClientLoader({
			pattern: "/hmr-non-opted",
			clientLoader: nonOptedLoader,
		});
		vi.spyOn(window, "fetch").mockResolvedValueOnce(
			createRouteDataResponse({
				matchedPatterns: ["/hmr-opted-in", "/hmr-non-opted"],
				loadersData: [{ id: "a" }, { id: "b" }],
				importURLs: ["/hmr-opted-in.js", "/hmr-non-opted.js"],
				exportKeys: ["default", "default"],
				errorExportKeys: ["", ""],
			}),
		);
		vi.doMock("/hmr-opted-in.js", () => ({ default: () => null }));
		vi.doMock("/hmr-non-opted.js", () => ({ default: () => null }));

		await client.vormaNavigate("/hmr-opted-in");
		await vi.runAllTimersAsync();
		expect(optedInLoader).toHaveBeenCalledTimes(1);
		expect(nonOptedLoader).toHaveBeenCalledTimes(1);

		await simulateViteAfterUpdateForTesting({
			updates: [
				{
					type: "js-update",
					path: "/src/routes/hmr-opted-in.tsx?t=2",
				},
			],
		});
		await vi.runAllTimersAsync();

		expect(optedInLoader).toHaveBeenCalledTimes(2);
		expect(nonOptedLoader).toHaveBeenCalledTimes(1);
	});

	it("does not re-run opted-in client loaders for css-only HMR updates", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const reactAdapter = await import("vorma/react");
		const optedInLoader = vi.fn(async ({ serverDataPromise }) => {
			await serverDataPromise;
			return "opted-in";
		});
		const addClientLoader = reactAdapter.makeTypedAddClientLoader(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		addClientLoader({
			pattern: "/hmr-css",
			clientLoader: optedInLoader,
			reRunOnModuleChange: {
				url: "http://localhost:3000/src/routes/hmr-css.tsx?t=1",
			} as ImportMeta,
		});
		vi.spyOn(window, "fetch").mockResolvedValueOnce(
			createRouteDataResponse({
				matchedPatterns: ["/hmr-css"],
				loadersData: [{ id: "css" }],
				importURLs: ["/hmr-css.js"],
				exportKeys: ["default"],
				errorExportKeys: [""],
			}),
		);
		vi.doMock("/hmr-css.js", () => ({ default: () => null }));

		await client.vormaNavigate("/hmr-css");
		await vi.runAllTimersAsync();
		expect(optedInLoader).toHaveBeenCalledTimes(1);

		await simulateViteAfterUpdateForTesting({
			updates: [
				{
					type: "css-update",
					path: "/src/routes/hmr-css.tsx?t=2",
				},
			],
		});
		await vi.runAllTimersAsync();

		expect(optedInLoader).toHaveBeenCalledTimes(1);
	});

	it("react/preact/solid links strip navigation-only props from rendered anchors", async () => {
		await initializeDistRuntimeStateForAdapters();
		const reactAdapter = await import("vorma/react");
		const preactAdapter = await import("vorma/preact");
		const solidAdapter = await import("vorma/solid");

		const reactRendered = renderReactTypedLinkForTesting({
			TypedLink: reactAdapter.VormaLink,
			linkProps: {
				href: "/react-link-strip",
				replace: true,
				state: { a: 1 },
				prefetch: "intent",
				prefetchDelayMs: 25,
				beforeBegin: () => {},
				children: "React Strip",
			},
		});
		try {
			expect(reactRendered.anchor.getAttribute("href")).toBe(
				"/react-link-strip",
			);
			expect(reactRendered.anchor.getAttribute("replace")).toBeNull();
			expect(reactRendered.anchor.getAttribute("state")).toBeNull();
			expect(reactRendered.anchor.getAttribute("prefetch")).toBeNull();
			expect(
				reactRendered.anchor.getAttribute("prefetchdelayms"),
			).toBeNull();
			expect(reactRendered.anchor.getAttribute("beforebegin")).toBeNull();
		} finally {
			reactRendered.cleanup();
		}

		const preactContainer = document.createElement("div");
		document.body.appendChild(preactContainer);
		await act(async () => {
			renderPreact(
				h(preactAdapter.VormaLink as any, {
					href: "/preact-link-strip",
					replace: true,
					state: { a: 1 },
					prefetch: "intent",
					prefetchDelayMs: 25,
					beforeBegin: () => {},
					children: "Preact Strip",
				}),
				preactContainer,
			);
		});
		const preactAnchor = preactContainer.querySelector("a");
		expect(preactAnchor).not.toBeNull();
		expect(preactAnchor?.getAttribute("href")).toBe("/preact-link-strip");
		expect(preactAnchor?.getAttribute("replace")).toBeNull();
		expect(preactAnchor?.getAttribute("state")).toBeNull();
		expect(preactAnchor?.getAttribute("prefetch")).toBeNull();
		expect(preactAnchor?.getAttribute("prefetchdelayms")).toBeNull();
		expect(preactAnchor?.getAttribute("beforebegin")).toBeNull();
		renderPreact(null, preactContainer);
		preactContainer.remove();

		const solidContainer = document.createElement("div");
		document.body.appendChild(solidContainer);
		const dispose = renderSolidWithTrackedDisposer(() => {
			return (solidAdapter.VormaLink as any)({
				href: "/solid-link-strip",
				replace: true,
				state: { a: 1 },
				prefetch: "intent",
				prefetchDelayMs: 25,
				beforeBegin: () => {},
				children: "Solid Strip",
			});
		}, solidContainer);
		const solidAnchor = solidContainer.querySelector("a");
		expect(solidAnchor).not.toBeNull();
		expect(solidAnchor?.getAttribute("href")).toBe("/solid-link-strip");
		expect(solidAnchor?.getAttribute("replace")).toBeNull();
		expect(solidAnchor?.getAttribute("state")).toBeNull();
		expect(solidAnchor?.getAttribute("prefetch")).toBeNull();
		expect(solidAnchor?.getAttribute("prefetchdelayms")).toBeNull();
		expect(solidAnchor?.getAttribute("beforebegin")).toBeNull();
		dispose();
		solidContainer.remove();
	});

	it("react root outlet covers fallback, empty, custom/default error, and child-error branches", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const reactAdapter = await import("vorma/react");
		vi.doMock("/missing-root.js", () => ({ default: undefined }));
		vi.doMock("/child.js", () => ({
			default: (props: { idx: number }) =>
				React.createElement("div", {}, `fallback-child:${props.idx}`),
		}));
		vi.doMock("/error-custom.js", () => ({
			default: () => React.createElement("div", {}, "should-not-render"),
			ErrorBoundary: (props: { error: unknown }) =>
				React.createElement(
					"div",
					{},
					`handled:${String(props.error)}`,
				),
		}));
		vi.doMock("/error-default.js", () => ({
			default: () =>
				React.createElement("div", {}, "should-not-render-default"),
		}));
		vi.doMock("/parent.js", () => ({
			default: (props: { Outlet: unknown }) =>
				React.createElement(
					"section",
					{},
					React.createElement(
						"div",
						{ "data-parent": true },
						"parent-node",
					),
					React.createElement(props.Outlet as any, {}),
				),
		}));
		vi.doMock("/child-error.js", () => ({
			default: () => React.createElement("div", {}, "child-node"),
			ErrorBoundary: (props: { error: unknown }) =>
				React.createElement(
					"div",
					{},
					`handled:${String(props.error)}`,
				),
		}));

		const container = document.createElement("div");
		document.body.appendChild(container);
		const root = createRoot(container);
		flushSync(() => {
			root.render(
				React.createElement(reactAdapter.VormaRootOutlet as any, {
					idx: 0,
				}),
			);
		});
		try {
			await navigateWithDistRouteDataResponse({
				client,
				href: "/root-outlet-react/fallback",
				overrides: {
					matchedPatterns: ["/", "/child"],
					importURLs: ["/missing-root.js", "/child.js"],
					exportKeys: ["default", "default"],
					errorExportKeys: ["", ""],
					loadersData: [{}, {}],
				},
			});
			await waitForDOMCondition({
				assertion: () => {
					expect(container.textContent).toBe("");
				},
			});

			await navigateWithDistRouteDataResponse({
				client,
				href: "/root-outlet-react/empty",
				overrides: {
					matchedPatterns: ["/"],
					importURLs: ["/missing-root.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{}],
				},
			});
			await waitForDOMCondition({
				assertion: () => {
					expect(container.textContent).toBe("");
				},
			});

			await navigateWithDistRouteDataResponse({
				client,
				href: "/root-outlet-react/error-custom",
				overrides: {
					matchedPatterns: ["/"],
					importURLs: ["/error-custom.js"],
					exportKeys: ["default"],
					errorExportKeys: ["ErrorBoundary"],
					loadersData: [{}],
					outermostServerError: "boom",
					outermostServerErrorIdx: 0,
				},
			});
			await waitForDOMCondition({
				assertion: () => {
					expect(container.textContent).toContain("handled:boom");
				},
			});
			expect(container.textContent).not.toContain("should-not-render");

			await navigateWithDistRouteDataResponse({
				client,
				href: "/root-outlet-react/error-default-unknown",
				overrides: {
					matchedPatterns: ["/"],
					importURLs: ["/error-default.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{}],
					outermostServerError: undefined,
					outermostServerErrorIdx: 0,
				},
			});
			await waitForDOMCondition({
				assertion: () => {
					expect(container.textContent).toContain("Error: unknown");
				},
			});
			expect(container.textContent).not.toContain(
				"should-not-render-default",
			);

			await navigateWithDistRouteDataResponse({
				client,
				href: "/root-outlet-react/error-default-explicit",
				overrides: {
					matchedPatterns: ["/"],
					importURLs: ["/error-default.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{}],
					outermostServerError: "boom-explicit",
					outermostServerErrorIdx: 0,
				},
			});
			await waitForDOMCondition({
				assertion: () => {
					expect(container.textContent).toContain(
						"Error: boom-explicit",
					);
				},
			});

			await navigateWithDistRouteDataResponse({
				client,
				href: "/root-outlet-react/child-error",
				overrides: {
					matchedPatterns: ["/parent", "/parent/child"],
					importURLs: ["/parent.js", "/child-error.js"],
					exportKeys: ["default", "default"],
					errorExportKeys: ["", "ErrorBoundary"],
					loadersData: [{}, {}],
					outermostServerError: "child-boom",
					outermostServerErrorIdx: 1,
				},
			});
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector("[data-parent]")?.textContent,
					).toBe("parent-node");
					expect(container.textContent).toContain(
						"handled:child-boom",
					);
				},
			});
			expect(container.textContent).not.toContain("child-node");
		} finally {
			root.unmount();
			container.remove();
		}
	});

	it("sets active error boundary from server error index and error export key", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const reactAdapter = await import("vorma/react");
		vi.doMock("/error-index-layout.js", () => ({
			default: (props: { Outlet: unknown }) =>
				React.createElement(
					"section",
					{},
					React.createElement(
						"div",
						{ "data-layout": true },
						"layout-node",
					),
					React.createElement(props.Outlet as any, {}),
				),
		}));
		vi.doMock("/error-index-child.js", () => ({
			default: () => React.createElement("div", {}, "child-node"),
			ErrorBoundary: (props: { error: unknown }) =>
				React.createElement(
					"div",
					{},
					`child-handled:${String(props.error)}`,
				),
		}));

		const container = document.createElement("div");
		document.body.appendChild(container);
		const root = createRoot(container);
		flushSync(() => {
			root.render(
				React.createElement(reactAdapter.VormaRootOutlet as any, {
					idx: 0,
				}),
			);
		});
		try {
			await navigateWithDistRouteDataResponse({
				client,
				href: "/error-index-target",
				overrides: {
					matchedPatterns: ["/layout", "/layout/child"],
					importURLs: [
						"/error-index-layout.js",
						"/error-index-child.js",
					],
					exportKeys: ["default", "default"],
					errorExportKeys: ["", "ErrorBoundary"],
					loadersData: [{}, {}],
					outermostServerError: "boom",
					outermostServerErrorIdx: 1,
				},
			});
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector("[data-layout]")?.textContent,
					).toBe("layout-node");
					expect(container.textContent).toContain(
						"child-handled:boom",
					);
				},
			});
			expect(container.textContent).not.toContain("child-node");
		} finally {
			root.unmount();
			container.remove();
		}
	});

	it("replaces active error boundary when later navigations report a new boundary", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const reactAdapter = await import("vorma/react");
		vi.doMock("/error-replace-a.js", () => ({
			default: () => React.createElement("div", {}, "page-a"),
			ErrorBoundary: (props: { error: unknown }) =>
				React.createElement(
					"div",
					{},
					`boundary-a:${String(props.error)}`,
				),
		}));
		vi.doMock("/error-replace-b.js", () => ({
			default: () => React.createElement("div", {}, "page-b"),
			ErrorBoundary: (props: { error: unknown }) =>
				React.createElement(
					"div",
					{},
					`boundary-b:${String(props.error)}`,
				),
		}));

		const container = document.createElement("div");
		document.body.appendChild(container);
		const root = createRoot(container);
		flushSync(() => {
			root.render(
				React.createElement(reactAdapter.VormaRootOutlet as any, {
					idx: 0,
				}),
			);
		});
		try {
			await navigateWithDistRouteDataResponse({
				client,
				href: "/error-replace-a",
				overrides: {
					matchedPatterns: ["/error-a"],
					importURLs: ["/error-replace-a.js"],
					exportKeys: ["default"],
					errorExportKeys: ["ErrorBoundary"],
					loadersData: [{}],
					outermostServerError: "first",
					outermostServerErrorIdx: 0,
				},
			});
			await waitForDOMCondition({
				assertion: () => {
					expect(container.textContent).toContain("boundary-a:first");
				},
			});

			await navigateWithDistRouteDataResponse({
				client,
				href: "/error-replace-b",
				overrides: {
					matchedPatterns: ["/error-b"],
					importURLs: ["/error-replace-b.js"],
					exportKeys: ["default"],
					errorExportKeys: ["ErrorBoundary"],
					loadersData: [{}],
					outermostServerError: "second",
					outermostServerErrorIdx: 0,
				},
			});
			await waitForDOMCondition({
				assertion: () => {
					expect(container.textContent).toContain(
						"boundary-b:second",
					);
				},
			});
			expect(container.textContent).not.toContain("boundary-a:first");
		} finally {
			root.unmount();
			container.remove();
		}
	});

	it("preact root outlet covers fallback, empty, custom/default error, and child-error branches", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const preactAdapter = await import("vorma/preact");
		vi.doMock("/missing-root.js", () => ({ default: undefined }));
		vi.doMock("/child.js", () => ({
			default: (props: { idx: number }) =>
				h("div", {}, `fallback-child:${props.idx}`),
		}));
		vi.doMock("/error-custom.js", () => ({
			default: () => h("div", {}, "should-not-render"),
			ErrorBoundary: (props: { error: unknown }) =>
				h("div", {}, `handled:${String(props.error)}`),
		}));
		vi.doMock("/error-default.js", () => ({
			default: () => h("div", {}, "should-not-render-default"),
		}));
		vi.doMock("/parent.js", () => ({
			default: (props: { Outlet: unknown }) =>
				h(
					"section",
					{},
					h("div", { "data-parent": true }, "parent-node"),
					h(props.Outlet as any, {}),
				),
		}));
		vi.doMock("/child-error.js", () => ({
			default: () => h("div", {}, "child-node"),
			ErrorBoundary: (props: { error: unknown }) =>
				h("div", {}, `handled:${String(props.error)}`),
		}));

		const container = document.createElement("div");
		document.body.appendChild(container);
		await act(async () => {
			renderPreact(
				h(preactAdapter.VormaRootOutlet as any, {
					idx: 0,
				}),
				container,
			);
		});
		try {
			await navigateWithDistRouteDataResponse({
				client,
				href: "/root-outlet-preact/fallback",
				overrides: {
					matchedPatterns: ["/", "/child"],
					importURLs: ["/missing-root.js", "/child.js"],
					exportKeys: ["default", "default"],
					errorExportKeys: ["", ""],
					loadersData: [{}, {}],
				},
			});
			await waitForDOMCondition({
				assertion: () => {
					expect(container.textContent).toBe("");
				},
			});

			await navigateWithDistRouteDataResponse({
				client,
				href: "/root-outlet-preact/empty",
				overrides: {
					matchedPatterns: ["/"],
					importURLs: ["/missing-root.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{}],
				},
			});
			await waitForDOMCondition({
				assertion: () => {
					expect(container.textContent).toBe("");
				},
			});

			await navigateWithDistRouteDataResponse({
				client,
				href: "/root-outlet-preact/error-custom",
				overrides: {
					matchedPatterns: ["/"],
					importURLs: ["/error-custom.js"],
					exportKeys: ["default"],
					errorExportKeys: ["ErrorBoundary"],
					loadersData: [{}],
					outermostServerError: "boom",
					outermostServerErrorIdx: 0,
				},
			});
			await waitForDOMCondition({
				assertion: () => {
					expect(container.textContent).toContain("handled:boom");
				},
			});
			expect(container.textContent).not.toContain("should-not-render");

			await navigateWithDistRouteDataResponse({
				client,
				href: "/root-outlet-preact/error-default-unknown",
				overrides: {
					matchedPatterns: ["/"],
					importURLs: ["/error-default.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{}],
					outermostServerError: undefined,
					outermostServerErrorIdx: 0,
				},
			});
			await waitForDOMCondition({
				assertion: () => {
					expect(container.textContent).toContain("Error: unknown");
				},
			});
			expect(container.textContent).not.toContain(
				"should-not-render-default",
			);

			await navigateWithDistRouteDataResponse({
				client,
				href: "/root-outlet-preact/error-default-explicit",
				overrides: {
					matchedPatterns: ["/"],
					importURLs: ["/error-default.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{}],
					outermostServerError: "boom-explicit",
					outermostServerErrorIdx: 0,
				},
			});
			await waitForDOMCondition({
				assertion: () => {
					expect(container.textContent).toContain(
						"Error: boom-explicit",
					);
				},
			});

			await navigateWithDistRouteDataResponse({
				client,
				href: "/root-outlet-preact/child-error",
				overrides: {
					matchedPatterns: ["/parent", "/parent/child"],
					importURLs: ["/parent.js", "/child-error.js"],
					exportKeys: ["default", "default"],
					errorExportKeys: ["", "ErrorBoundary"],
					loadersData: [{}, {}],
					outermostServerError: "child-boom",
					outermostServerErrorIdx: 1,
				},
			});
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector("[data-parent]")?.textContent,
					).toBe("parent-node");
					expect(container.textContent).toContain(
						"handled:child-boom",
					);
				},
			});
			expect(container.textContent).not.toContain("child-node");
		} finally {
			renderPreact(null, container);
			container.remove();
		}
	});

	it("solid root outlet covers fallback, empty, custom/default error, and child-error branches", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const solidAdapter = await import("vorma/solid");
		const childDefault = vi.fn((props: { idx: number }) => {
			const node = document.createElement("div");
			node.textContent = `fallback-child:${props.idx}`;
			return node;
		});
		const errorCustomDefault = vi.fn(() => {
			const node = document.createElement("div");
			node.textContent = "should-not-render";
			return node;
		});
		const errorCustomBoundary = vi.fn((props: { error: unknown }) => {
			const node = document.createElement("div");
			node.textContent = `handled:${String(props.error)}`;
			return node;
		});
		const errorDefaultDefault = vi.fn(() => {
			const node = document.createElement("div");
			node.textContent = "should-not-render-default";
			return node;
		});
		const parentDefault = vi.fn(
			(props: { Outlet: (input?: Record<string, unknown>) => any }) => {
				const section = document.createElement("section");
				const parentNode = document.createElement("div");
				parentNode.setAttribute("data-parent", "true");
				parentNode.textContent = "parent-node";
				section.appendChild(parentNode);
				const outletResult = props.Outlet({});
				if (outletResult instanceof Node) {
					section.appendChild(outletResult);
				} else if (typeof outletResult === "string") {
					section.appendChild(document.createTextNode(outletResult));
				}
				return section;
			},
		);
		const childErrorDefault = vi.fn(() => {
			const node = document.createElement("div");
			node.textContent = "child-node";
			return node;
		});
		const childErrorBoundary = vi.fn((props: { error: unknown }) => {
			const node = document.createElement("div");
			node.textContent = `handled:${String(props.error)}`;
			return node;
		});
		vi.doMock("/missing-root.js", () => ({ default: undefined }));
		vi.doMock("/child.js", () => ({
			default: childDefault,
		}));
		vi.doMock("/error-custom.js", () => ({
			default: errorCustomDefault,
			ErrorBoundary: errorCustomBoundary,
		}));
		vi.doMock("/error-default.js", () => ({
			default: errorDefaultDefault,
		}));
		vi.doMock("/parent.js", () => ({
			default: parentDefault,
		}));
		vi.doMock("/child-error.js", () => ({
			default: childErrorDefault,
			ErrorBoundary: childErrorBoundary,
		}));

		const container = document.createElement("div");
		document.body.appendChild(container);
		const dispose = renderSolidWithTrackedDisposer(() => {
			return createComponent(solidAdapter.VormaRootOutlet as any, {
				idx: 0,
			});
		}, container);
		try {
			await Promise.resolve();

			await navigateWithDistRouteDataResponse({
				client,
				href: "/root-outlet-solid/fallback",
				overrides: {
					matchedPatterns: ["/", "/child"],
					importURLs: ["/missing-root.js", "/child.js"],
					exportKeys: ["default", "default"],
					errorExportKeys: ["", ""],
					loadersData: [{}, {}],
				},
			});
			await waitForDOMCondition({
				assertion: () => {
					expect(childDefault).not.toHaveBeenCalled();
				},
			});

			await navigateWithDistRouteDataResponse({
				client,
				href: "/root-outlet-solid/empty",
				overrides: {
					matchedPatterns: ["/"],
					importURLs: ["/missing-root.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{}],
				},
			});
			await waitForDOMCondition({
				assertion: () => {
					expect(childDefault).not.toHaveBeenCalled();
				},
			});

			await navigateWithDistRouteDataResponse({
				client,
				href: "/root-outlet-solid/error-custom",
				overrides: {
					matchedPatterns: ["/"],
					importURLs: ["/error-custom.js"],
					exportKeys: ["default"],
					errorExportKeys: ["ErrorBoundary"],
					loadersData: [{}],
					outermostServerError: "boom",
					outermostServerErrorIdx: 0,
				},
			});
			await waitForDOMCondition({
				assertion: () => {
					expect(errorCustomBoundary).toHaveBeenCalled();
				},
			});
			expect(errorCustomBoundary).toHaveBeenLastCalledWith(
				expect.objectContaining({
					error: "boom",
				}),
			);
			expect(errorCustomDefault).not.toHaveBeenCalled();

			await navigateWithDistRouteDataResponse({
				client,
				href: "/root-outlet-solid/error-default-unknown",
				overrides: {
					matchedPatterns: ["/"],
					importURLs: ["/error-default.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{}],
					outermostServerError: undefined,
					outermostServerErrorIdx: 0,
				},
			});
			await waitForDOMCondition({
				assertion: () => {
					expect(errorDefaultDefault).not.toHaveBeenCalled();
				},
			});

			await navigateWithDistRouteDataResponse({
				client,
				href: "/root-outlet-solid/error-default-explicit",
				overrides: {
					matchedPatterns: ["/"],
					importURLs: ["/error-default.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{}],
					outermostServerError: "boom-explicit",
					outermostServerErrorIdx: 0,
				},
			});
			await waitForDOMCondition({
				assertion: () => {
					expect(errorDefaultDefault).not.toHaveBeenCalled();
				},
			});

			await navigateWithDistRouteDataResponse({
				client,
				href: "/root-outlet-solid/child-error",
				overrides: {
					matchedPatterns: ["/parent", "/parent/child"],
					importURLs: ["/parent.js", "/child-error.js"],
					exportKeys: ["default", "default"],
					errorExportKeys: ["", "ErrorBoundary"],
					loadersData: [{}, {}],
					outermostServerError: "child-boom",
					outermostServerErrorIdx: 1,
				},
			});
			await waitForDOMCondition({
				assertion: () => {
					expect(parentDefault).toHaveBeenCalled();
					expect(childErrorBoundary).toHaveBeenCalledWith(
						expect.objectContaining({
							error: "child-boom",
						}),
					);
				},
			});
			expect(childErrorDefault).not.toHaveBeenCalled();
		} finally {
			dispose();
			await Promise.resolve();
			container.remove();
		}
	});

	it("react root outlet initializes listeners once across remounts", async () => {
		await initializeDistRuntimeStateForAdapters();
		const reactAdapter = await import("vorma/react");
		const addEventListenerSpy = vi.spyOn(window, "addEventListener");
		const containerA = document.createElement("div");
		const containerB = document.createElement("div");
		document.body.appendChild(containerA);
		document.body.appendChild(containerB);
		const rootA = createRoot(containerA);
		const rootB = createRoot(containerB);
		try {
			flushSync(() => {
				rootA.render(
					React.createElement(reactAdapter.VormaRootOutlet as any, {
						idx: 0,
					}),
				);
			});
			flushSync(() => {
				rootA.unmount();
			});
			flushSync(() => {
				rootB.render(
					React.createElement(reactAdapter.VormaRootOutlet as any, {
						idx: 0,
					}),
				);
			});

			expect(
				addEventListenerSpy.mock.calls.filter(
					([eventName]) => eventName === "vorma:route-change",
				).length,
			).toBe(1);
			expect(
				addEventListenerSpy.mock.calls.filter(
					([eventName]) => eventName === "vorma:location",
				).length,
			).toBe(1);
		} finally {
			rootB.unmount();
			addEventListenerSpy.mockRestore();
			containerA.remove();
			containerB.remove();
		}
	});

	it("preact root outlet initializes listeners once across remounts", async () => {
		await initializeDistRuntimeStateForAdapters();
		const preactAdapter = await import("vorma/preact");
		const addEventListenerSpy = vi.spyOn(window, "addEventListener");
		const containerA = document.createElement("div");
		const containerB = document.createElement("div");
		document.body.appendChild(containerA);
		document.body.appendChild(containerB);
		try {
			await act(async () => {
				renderPreact(
					h(preactAdapter.VormaRootOutlet as any, {
						idx: 0,
					}),
					containerA,
				);
			});
			await act(async () => {
				renderPreact(null, containerA);
			});
			await act(async () => {
				renderPreact(
					h(preactAdapter.VormaRootOutlet as any, {
						idx: 0,
					}),
					containerB,
				);
			});

			expect(
				addEventListenerSpy.mock.calls.filter(
					([eventName]) => eventName === "vorma:route-change",
				).length,
			).toBe(1);
			expect(
				addEventListenerSpy.mock.calls.filter(
					([eventName]) => eventName === "vorma:location",
				).length,
			).toBe(1);
		} finally {
			renderPreact(null, containerB);
			addEventListenerSpy.mockRestore();
			containerA.remove();
			containerB.remove();
		}
	});

	it("solid root outlet initializes listeners once across remounts", async () => {
		await initializeDistRuntimeStateForAdapters();
		const solidAdapter = await import("vorma/solid");
		const addEventListenerSpy = vi.spyOn(window, "addEventListener");
		const containerA = document.createElement("div");
		const containerB = document.createElement("div");
		document.body.appendChild(containerA);
		document.body.appendChild(containerB);
		let disposeA: (() => void) | undefined;
		let disposeB: (() => void) | undefined;
		try {
			disposeA = renderSolidWithTrackedDisposer(() => {
				return createComponent(solidAdapter.VormaRootOutlet as any, {
					idx: 0,
				});
			}, containerA);
			disposeA();
			disposeA = undefined;

			disposeB = renderSolidWithTrackedDisposer(() => {
				return createComponent(solidAdapter.VormaRootOutlet as any, {
					idx: 0,
				});
			}, containerB);

			expect(
				addEventListenerSpy.mock.calls.filter(
					([eventName]) => eventName === "vorma:route-change",
				).length,
			).toBe(1);
			expect(
				addEventListenerSpy.mock.calls.filter(
					([eventName]) => eventName === "vorma:location",
				).length,
			).toBe(1);
		} finally {
			disposeA?.();
			disposeB?.();
			addEventListenerSpy.mockRestore();
			containerA.remove();
			containerB.remove();
		}
	});

	it("react root outlet defaults idx to 0 when idx is omitted", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const reactAdapter = await import("vorma/react");
		vi.doMock("/root-default-idx.js", () => ({
			default: (props: { idx: number }) =>
				React.createElement("div", {}, `root-default-idx:${props.idx}`),
		}));
		const container = document.createElement("div");
		document.body.appendChild(container);
		const root = createRoot(container);
		try {
			flushSync(() => {
				root.render(
					React.createElement(reactAdapter.VormaRootOutlet as any),
				);
			});

			await navigateWithDistRouteDataResponse({
				client,
				href: "/root-default-idx-react",
				overrides: {
					matchedPatterns: ["/"],
					importURLs: ["/root-default-idx.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{}],
				},
			});
			await waitForDOMCondition({
				assertion: () => {
					expect(container.textContent).toContain(
						"root-default-idx:0",
					);
				},
			});
		} finally {
			root.unmount();
			container.remove();
		}
	});

	it("preact root outlet defaults idx to 0 when idx is omitted", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const preactAdapter = await import("vorma/preact");
		vi.doMock("/root-default-idx.js", () => ({
			default: (props: { idx: number }) =>
				h("div", {}, `root-default-idx:${props.idx}`),
		}));
		const container = document.createElement("div");
		document.body.appendChild(container);
		try {
			await act(async () => {
				renderPreact(
					h(preactAdapter.VormaRootOutlet as any, {}),
					container,
				);
			});

			await navigateWithDistRouteDataResponse({
				client,
				href: "/root-default-idx-preact",
				overrides: {
					matchedPatterns: ["/"],
					importURLs: ["/root-default-idx.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{}],
				},
			});
			await waitForDOMCondition({
				assertion: () => {
					expect(container.textContent).toContain(
						"root-default-idx:0",
					);
				},
			});
		} finally {
			renderPreact(null, container);
			container.remove();
		}
	});

	it("solid root outlet defaults idx to 0 when idx is omitted", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const solidAdapter = await import("vorma/solid");
		const solidDefaultOutlet = vi.fn((props: { idx: number }) => {
			return `root-default-idx:${props.idx}`;
		});
		vi.doMock("/root-default-idx.js", () => ({
			default: solidDefaultOutlet,
		}));
		const container = document.createElement("div");
		document.body.appendChild(container);
		const dispose = renderSolidWithTrackedDisposer(() => {
			return createComponent(solidAdapter.VormaRootOutlet as any, {});
		}, container);
		try {
			await Promise.resolve();

			await navigateWithDistRouteDataResponse({
				client,
				href: "/root-default-idx-solid",
				overrides: {
					matchedPatterns: ["/"],
					importURLs: ["/root-default-idx.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{}],
				},
			});
			await waitForDOMCondition({
				assertion: () => {
					expect(solidDefaultOutlet).toHaveBeenCalledWith(
						expect.objectContaining({
							idx: 0,
						}),
					);
				},
			});
		} finally {
			dispose();
			container.remove();
		}
	});

	it("react idx>0 outlet does not initialize root listeners", async () => {
		await initializeDistRuntimeStateForAdapters();
		const reactAdapter = await import("vorma/react");
		const addEventListenerSpy = vi.spyOn(window, "addEventListener");
		const container = document.createElement("div");
		document.body.appendChild(container);
		const root = createRoot(container);
		try {
			flushSync(() => {
				root.render(
					React.createElement(reactAdapter.VormaRootOutlet as any, {
						idx: 1,
					}),
				);
			});

			expect(
				addEventListenerSpy.mock.calls.filter(
					([eventName]) => eventName === "vorma:route-change",
				).length,
			).toBe(0);
			expect(
				addEventListenerSpy.mock.calls.filter(
					([eventName]) => eventName === "vorma:location",
				).length,
			).toBe(0);
		} finally {
			root.unmount();
			addEventListenerSpy.mockRestore();
			container.remove();
		}
	});

	it("preact idx>0 outlet does not initialize root listeners", async () => {
		await initializeDistRuntimeStateForAdapters();
		const preactAdapter = await import("vorma/preact");
		const addEventListenerSpy = vi.spyOn(window, "addEventListener");
		const container = document.createElement("div");
		document.body.appendChild(container);
		try {
			await act(async () => {
				renderPreact(
					h(preactAdapter.VormaRootOutlet as any, {
						idx: 1,
					}),
					container,
				);
			});

			expect(
				addEventListenerSpy.mock.calls.filter(
					([eventName]) => eventName === "vorma:route-change",
				).length,
			).toBe(0);
			expect(
				addEventListenerSpy.mock.calls.filter(
					([eventName]) => eventName === "vorma:location",
				).length,
			).toBe(0);
		} finally {
			renderPreact(null, container);
			addEventListenerSpy.mockRestore();
			container.remove();
		}
	});

	it("solid idx>0 outlet does not initialize root listeners", async () => {
		await initializeDistRuntimeStateForAdapters();
		const solidAdapter = await import("vorma/solid");
		const addEventListenerSpy = vi.spyOn(window, "addEventListener");
		const container = document.createElement("div");
		document.body.appendChild(container);
		const dispose = renderSolidWithTrackedDisposer(() => {
			return createComponent(solidAdapter.VormaRootOutlet as any, {
				idx: 1,
			});
		}, container);
		try {
			expect(
				addEventListenerSpy.mock.calls.filter(
					([eventName]) => eventName === "vorma:route-change",
				).length,
			).toBe(0);
			expect(
				addEventListenerSpy.mock.calls.filter(
					([eventName]) => eventName === "vorma:location",
				).length,
			).toBe(0);
		} finally {
			dispose();
			addEventListenerSpy.mockRestore();
			container.remove();
		}
	});

	it("react useLocation only updates on location events, not route events", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const reactAdapter = await import("vorma/react");
		vi.doMock("/location-root-react.js", () => ({
			default: () => React.createElement("div", {}, "root"),
		}));
		vi.spyOn(window, "fetch").mockImplementation(() => {
			return Promise.resolve(
				createRouteDataResponse({
					matchedPatterns: ["/"],
					importURLs: ["/location-root-react.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{}],
				}),
			);
		});
		const container = document.createElement("div");
		document.body.appendChild(container);
		const root = createRoot(container);
		let locationRenderCount = 0;
		const LocationProbe = () => {
			const location = reactAdapter.useLocation();
			locationRenderCount += 1;
			return React.createElement(
				"div",
				{ "data-location-probe": true },
				location.pathname,
			);
		};
		try {
			flushSync(() => {
				root.render(
					React.createElement(
						React.Fragment,
						{},
						React.createElement(
							reactAdapter.VormaRootOutlet as any,
							{
								idx: 0,
							},
						),
						React.createElement(LocationProbe),
					),
				);
			});
			await client.vormaNavigate("/location-base");
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector("[data-location-probe]")
							?.textContent,
					).toBe("/location-base");
				},
			});
			const rendersAfterInitial = locationRenderCount;

			dispatchRouteChangeEventForTesting();
			await vi.runAllTimersAsync();
			expect(locationRenderCount).toBe(rendersAfterInitial);

			window.history.pushState(null, "", "/location-next");
			dispatchLocationEventForTestingFromCurrentWindowLocation();
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector("[data-location-probe]")
							?.textContent,
					).toBe("/location-next");
				},
			});
			expect(locationRenderCount).toBe(rendersAfterInitial + 1);
		} finally {
			flushSync(() => {
				root.unmount();
			});
			container.remove();
		}
	});

	it("preact location signal only updates on location events, not route events", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const preactAdapter = await import("vorma/preact");
		vi.doMock("/location-root-preact.js", () => ({
			default: () => h("div", {}, "root"),
		}));
		vi.spyOn(window, "fetch").mockImplementation(() => {
			return Promise.resolve(
				createRouteDataResponse({
					matchedPatterns: ["/"],
					importURLs: ["/location-root-preact.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{}],
				}),
			);
		});
		const container = document.createElement("div");
		document.body.appendChild(container);
		let locationRenderCount = 0;
		const LocationProbe = () => {
			locationRenderCount += 1;
			return h(
				"div",
				{ "data-location-probe": true },
				preactAdapter.location.value.pathname,
			);
		};
		try {
			await act(async () => {
				renderPreact(
					h(
						"div",
						{},
						h(preactAdapter.VormaRootOutlet as any, {
							idx: 0,
						}),
						h(LocationProbe, {}),
					),
					container,
				);
			});
			await client.vormaNavigate("/location-base");
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector("[data-location-probe]")
							?.textContent,
					).toBe("/location-base");
				},
			});
			const rendersAfterInitial = locationRenderCount;

			dispatchRouteChangeEventForTesting();
			await vi.runAllTimersAsync();
			expect(locationRenderCount).toBe(rendersAfterInitial);

			window.history.pushState(null, "", "/location-next");
			dispatchLocationEventForTestingFromCurrentWindowLocation();
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector("[data-location-probe]")
							?.textContent,
					).toBe("/location-next");
				},
			});
			expect(locationRenderCount).toBe(rendersAfterInitial + 1);
		} finally {
			renderPreact(null, container);
			container.remove();
		}
	});

	it("solid location signal only updates on location events, not route events", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const solidAdapter = await import("vorma/solid");
		vi.doMock("/location-root-solid.js", () => ({
			default: () => {
				return "";
			},
		}));
		vi.spyOn(window, "fetch").mockImplementation(() => {
			return Promise.resolve(
				createRouteDataResponse({
					matchedPatterns: ["/"],
					importURLs: ["/location-root-solid.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{}],
				}),
			);
		});
		const container = document.createElement("div");
		document.body.appendChild(container);
		let locationRenderCount = 0;
		const LocationProbe = () => {
			const node = document.createElement("div");
			node.setAttribute("data-location-probe", "true");
			createEffect(() => {
				locationRenderCount += 1;
				node.textContent = solidAdapter.location().pathname;
			});
			return node;
		};
		const dispose = renderSolidWithTrackedDisposer(() => {
			return [
				createComponent(solidAdapter.VormaRootOutlet as any, {
					idx: 0,
				}),
				LocationProbe(),
			];
		}, container);

		try {
			await client.vormaNavigate("/location-base");
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector("[data-location-probe]")
							?.textContent,
					).toBe("/location-base");
				},
			});
			const rendersAfterInitial = locationRenderCount;

			dispatchRouteChangeEventForTesting();
			await vi.runAllTimersAsync();
			expect(locationRenderCount).toBe(rendersAfterInitial);

			window.history.pushState(null, "", "/location-next");
			dispatchLocationEventForTestingFromCurrentWindowLocation();
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector("[data-location-probe]")
							?.textContent,
					).toBe("/location-next");
				},
			});
			expect(locationRenderCount).toBe(rendersAfterInitial + 1);
		} finally {
			dispose();
			container.remove();
		}
	});

	it("react typed loader/client-loader/router selectors update on route changes and ignore location events", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const reactAdapter = await import("vorma/react");
		vi.doMock("/selectors-root-react.js", () => ({
			default: () => React.createElement("div", {}, "root"),
		}));
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/"],
					importURLs: ["/selectors-root-react.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{ value: "a" }],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/next"],
					importURLs: ["/selectors-root-react.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{ value: "b" }],
				}),
			);
		const useRouterData = reactAdapter.makeTypedUseRouterData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const usePatternLoaderData = reactAdapter.makeTypedUsePatternLoaderData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const useClientLoaderData = reactAdapter.makeTypedAddClientLoader(
			DIST_TEST_VORMA_APP_CONFIG,
		)({
			pattern: "/",
			clientLoader: async () => "ca",
		});
		const DataProbe = () => {
			const routerData = useRouterData();
			const loaderDataForRoot = usePatternLoaderData("/" as any) as
				| { value?: string }
				| undefined;
			const loaderDataForNext = usePatternLoaderData("/next" as any) as
				| { value?: string }
				| undefined;
			const activePattern = routerData.matchedPatterns[0] ?? "/";
			const loaderData =
				activePattern === "/next"
					? loaderDataForNext
					: loaderDataForRoot;
			const clientLoaderData = useClientLoaderData() as
				| string
				| undefined;
			return React.createElement(
				"div",
				{ "data-selector-probe": true },
				`${loaderData?.value ?? ""}|${clientLoaderData ?? ""}|${routerData.matchedPatterns.join(",")}`,
			);
		};
		const container = document.createElement("div");
		document.body.appendChild(container);
		const root = createRoot(container);
		try {
			flushSync(() => {
				root.render(
					React.createElement(
						React.Fragment,
						{},
						React.createElement(
							reactAdapter.VormaRootOutlet as any,
							{
								idx: 0,
							},
						),
						React.createElement(DataProbe),
					),
				);
			});
			await client.vormaNavigate("/selectors-route-a");
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector("[data-selector-probe]")
							?.textContent,
					).toBe("a|ca|/");
				},
			});

			dispatchVormaLocationEventForTestingFromCurrentWindowLocation();
			await vi.runAllTimersAsync();
			expect(fetchSpy).toHaveBeenCalledTimes(1);
			expect(
				container.querySelector("[data-selector-probe]")?.textContent,
			).toBe("a|ca|/");

			await client.vormaNavigate("/selectors-route-b");
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector("[data-selector-probe]")
							?.textContent,
					).toBe("b||/next");
				},
			});
			expect(fetchSpy).toHaveBeenCalledTimes(2);
		} finally {
			flushSync(() => {
				root.unmount();
			});
			container.remove();
		}
	});

	it("preact typed loader/client-loader/router selectors update on route changes and ignore location events", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const preactAdapter = await import("vorma/preact");
		vi.doMock("/selectors-root-preact.js", () => ({
			default: () => h("div", {}, "root"),
		}));
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/"],
					importURLs: ["/selectors-root-preact.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{ value: "a" }],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/next"],
					importURLs: ["/selectors-root-preact.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{ value: "b" }],
				}),
			);
		const useRouterData = preactAdapter.makeTypedUseRouterData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const usePatternLoaderData =
			preactAdapter.makeTypedUsePatternLoaderData(
				DIST_TEST_VORMA_APP_CONFIG,
			);
		const useClientLoaderData = preactAdapter.makeTypedAddClientLoader(
			DIST_TEST_VORMA_APP_CONFIG,
		)({
			pattern: "/",
			clientLoader: async () => "ca",
		});
		const DataProbe = () => {
			const routerData = useRouterData();
			const loaderDataForRoot = usePatternLoaderData("/" as any) as
				| { value?: string }
				| undefined;
			const loaderDataForNext = usePatternLoaderData("/next" as any) as
				| { value?: string }
				| undefined;
			const activePattern = routerData.matchedPatterns[0] ?? "/";
			const loaderData =
				activePattern === "/next"
					? loaderDataForNext
					: loaderDataForRoot;
			const clientLoaderData = useClientLoaderData() as
				| string
				| undefined;
			return h(
				"div",
				{ "data-selector-probe": true },
				`${loaderData?.value ?? ""}|${clientLoaderData ?? ""}|${routerData.matchedPatterns.join(",")}`,
			);
		};
		const container = document.createElement("div");
		document.body.appendChild(container);
		try {
			await act(async () => {
				renderPreact(
					h(
						"div",
						{},
						h(preactAdapter.VormaRootOutlet as any, { idx: 0 }),
						h(DataProbe, {}),
					),
					container,
				);
			});
			await client.vormaNavigate("/selectors-route-a");
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector("[data-selector-probe]")
							?.textContent,
					).toBe("a|ca|/");
				},
			});

			dispatchVormaLocationEventForTestingFromCurrentWindowLocation();
			await vi.runAllTimersAsync();
			expect(fetchSpy).toHaveBeenCalledTimes(1);
			expect(
				container.querySelector("[data-selector-probe]")?.textContent,
			).toBe("a|ca|/");

			await client.vormaNavigate("/selectors-route-b");
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector("[data-selector-probe]")
							?.textContent,
					).toBe("b||/next");
				},
			});
			expect(fetchSpy).toHaveBeenCalledTimes(2);
		} finally {
			renderPreact(null, container);
			container.remove();
		}
	});

	it("solid typed loader/client-loader/router selectors update on route changes and ignore location events", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const solidAdapter = await import("vorma/solid");
		vi.doMock("/selectors-root-solid.js", () => ({
			default: () => "",
		}));
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/"],
					importURLs: ["/selectors-root-solid.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{ value: "a" }],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/next"],
					importURLs: ["/selectors-root-solid.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{ value: "b" }],
				}),
			);
		const useRouterData = solidAdapter.makeTypedUseRouterData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const usePatternLoaderData = solidAdapter.makeTypedUsePatternLoaderData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const useClientLoaderData = solidAdapter.makeTypedAddClientLoader(
			DIST_TEST_VORMA_APP_CONFIG,
		)({
			pattern: "/",
			clientLoader: async () => "ca",
		});
		const DataProbe = () => {
			const routerData = useRouterData();
			const loaderDataForRoot = usePatternLoaderData("/" as any);
			const loaderDataForNext = usePatternLoaderData("/next" as any);
			const clientLoaderData = useClientLoaderData();
			const node = document.createElement("div");
			node.setAttribute("data-selector-probe", "true");
			createEffect(() => {
				const nextRouterData = routerData();
				const activePattern = nextRouterData.matchedPatterns[0] ?? "/";
				const activeLoaderData =
					activePattern === "/next"
						? (loaderDataForNext() as
								| { value?: string }
								| undefined)
						: (loaderDataForRoot() as
								| { value?: string }
								| undefined);
				const nextClientLoaderData = clientLoaderData() as
					| string
					| undefined;
				node.textContent = `${activeLoaderData?.value ?? ""}|${nextClientLoaderData ?? ""}|${nextRouterData.matchedPatterns.join(",")}`;
			});
			return node;
		};

		const container = document.createElement("div");
		document.body.appendChild(container);
		const dispose = renderSolidWithTrackedDisposer(() => {
			return [
				createComponent(solidAdapter.VormaRootOutlet as any, {
					idx: 0,
				}),
				DataProbe(),
			];
		}, container);

		try {
			await client.vormaNavigate("/selectors-route-a");
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector("[data-selector-probe]")
							?.textContent,
					).toBe("a|ca|/");
				},
			});

			dispatchVormaLocationEventForTestingFromCurrentWindowLocation();
			await vi.runAllTimersAsync();
			expect(fetchSpy).toHaveBeenCalledTimes(1);
			expect(
				container.querySelector("[data-selector-probe]")?.textContent,
			).toBe("a|ca|/");

			await client.vormaNavigate("/selectors-route-b");
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector("[data-selector-probe]")
							?.textContent,
					).toBe("b||/next");
				},
			});
			expect(fetchSpy).toHaveBeenCalledTimes(2);
		} finally {
			dispose();
			container.remove();
		}
	});

	it("react typed pattern-based selectors stay fresh across route index changes", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const reactAdapter = await import("vorma/react");
		vi.doMock("/pattern-probe-root-react.js", () => ({
			default: () => React.createElement("div", {}, "root"),
		}));
		vi.doMock("/pattern-probe-child-react.js", () => ({
			default: () => React.createElement("div", {}, "child"),
		}));
		vi.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/root", "/probe"],
					importURLs: [
						"/pattern-probe-root-react.js",
						"/pattern-probe-child-react.js",
					],
					exportKeys: ["default", "default"],
					errorExportKeys: ["", ""],
					loadersData: [{ value: "root-a" }, { value: "probe-a" }],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/root"],
					importURLs: ["/pattern-probe-root-react.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{ value: "root-b" }],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/probe"],
					importURLs: ["/pattern-probe-root-react.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{ value: "probe-c" }],
				}),
			);
		const usePatternLoaderData = reactAdapter.makeTypedUsePatternLoaderData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const usePatternClientLoaderData =
			reactAdapter.makeTypedAddClientLoader(DIST_TEST_VORMA_APP_CONFIG)({
				pattern: "/probe",
				clientLoader: async ({ serverDataPromise }) => {
					const serverData = await serverDataPromise;
					return (
						serverData.loaderData as { value?: string } | undefined
					)?.value;
				},
			});
		const PatternProbe = () => {
			const loaderData = usePatternLoaderData("/probe" as any) as
				| { value?: string }
				| undefined;
			const clientLoaderData = usePatternClientLoaderData() as
				| string
				| undefined;
			return React.createElement(
				"div",
				{ "data-pattern-probe": true },
				`${loaderData?.value ?? "none"}|${clientLoaderData ?? "none"}`,
			);
		};
		const container = document.createElement("div");
		document.body.appendChild(container);
		const root = createRoot(container);
		try {
			flushSync(() => {
				root.render(
					React.createElement(
						React.Fragment,
						{},
						React.createElement(
							reactAdapter.VormaRootOutlet as any,
							{
								idx: 0,
							},
						),
						React.createElement(PatternProbe),
					),
				);
			});

			await client.vormaNavigate("/pattern-probe-react-a");
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector("[data-pattern-probe]")
							?.textContent,
					).toBe("probe-a|probe-a");
				},
			});

			await client.vormaNavigate("/pattern-probe-react-b");
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector("[data-pattern-probe]")
							?.textContent,
					).toBe("none|none");
				},
			});

			await client.vormaNavigate("/pattern-probe-react-c");
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector("[data-pattern-probe]")
							?.textContent,
					).toBe("probe-c|probe-c");
				},
			});
		} finally {
			flushSync(() => {
				root.unmount();
			});
			container.remove();
		}
	});

	it("preact typed pattern-based selectors stay fresh across route index changes", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const preactAdapter = await import("vorma/preact");
		vi.doMock("/pattern-probe-root-preact.js", () => ({
			default: () => h("div", {}, "root"),
		}));
		vi.doMock("/pattern-probe-child-preact.js", () => ({
			default: () => h("div", {}, "child"),
		}));
		vi.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/root", "/probe"],
					importURLs: [
						"/pattern-probe-root-preact.js",
						"/pattern-probe-child-preact.js",
					],
					exportKeys: ["default", "default"],
					errorExportKeys: ["", ""],
					loadersData: [{ value: "root-a" }, { value: "probe-a" }],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/root"],
					importURLs: ["/pattern-probe-root-preact.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{ value: "root-b" }],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/probe"],
					importURLs: ["/pattern-probe-root-preact.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{ value: "probe-c" }],
				}),
			);
		const usePatternLoaderData =
			preactAdapter.makeTypedUsePatternLoaderData(
				DIST_TEST_VORMA_APP_CONFIG,
			);
		const usePatternClientLoaderData =
			preactAdapter.makeTypedAddClientLoader(DIST_TEST_VORMA_APP_CONFIG)({
				pattern: "/probe",
				clientLoader: async ({ serverDataPromise }) => {
					const serverData = await serverDataPromise;
					return (
						serverData.loaderData as { value?: string } | undefined
					)?.value;
				},
			});
		const PatternProbe = () => {
			const loaderData = usePatternLoaderData("/probe" as any) as
				| { value?: string }
				| undefined;
			const clientLoaderData = usePatternClientLoaderData() as
				| string
				| undefined;
			return h(
				"div",
				{ "data-pattern-probe": true },
				`${loaderData?.value ?? "none"}|${clientLoaderData ?? "none"}`,
			);
		};
		const container = document.createElement("div");
		document.body.appendChild(container);
		try {
			await act(async () => {
				renderPreact(
					h(
						"div",
						{},
						h(preactAdapter.VormaRootOutlet as any, { idx: 0 }),
						h(PatternProbe, {}),
					),
					container,
				);
			});

			await client.vormaNavigate("/pattern-probe-preact-a");
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector("[data-pattern-probe]")
							?.textContent,
					).toBe("probe-a|probe-a");
				},
			});

			await client.vormaNavigate("/pattern-probe-preact-b");
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector("[data-pattern-probe]")
							?.textContent,
					).toBe("none|none");
				},
			});

			await client.vormaNavigate("/pattern-probe-preact-c");
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector("[data-pattern-probe]")
							?.textContent,
					).toBe("probe-c|probe-c");
				},
			});
		} finally {
			renderPreact(null, container);
			container.remove();
		}
	});

	it("solid typed pattern-based selectors stay fresh across route index changes", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const solidAdapter = await import("vorma/solid");
		vi.doMock("/pattern-probe-root-solid.js", () => ({
			default: () => "",
		}));
		vi.doMock("/pattern-probe-child-solid.js", () => ({
			default: () => "",
		}));
		vi.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/root", "/probe"],
					importURLs: [
						"/pattern-probe-root-solid.js",
						"/pattern-probe-child-solid.js",
					],
					exportKeys: ["default", "default"],
					errorExportKeys: ["", ""],
					loadersData: [{ value: "root-a" }, { value: "probe-a" }],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/root"],
					importURLs: ["/pattern-probe-root-solid.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{ value: "root-b" }],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/probe"],
					importURLs: ["/pattern-probe-root-solid.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{ value: "probe-c" }],
				}),
			);
		const usePatternLoaderData = solidAdapter.makeTypedUsePatternLoaderData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const usePatternClientLoaderData =
			solidAdapter.makeTypedAddClientLoader(DIST_TEST_VORMA_APP_CONFIG)({
				pattern: "/probe",
				clientLoader: async ({ serverDataPromise }) => {
					const serverData = await serverDataPromise;
					return (
						serverData.loaderData as { value?: string } | undefined
					)?.value;
				},
			});
		const PatternProbe = () => {
			const loaderData = usePatternLoaderData("/probe" as any);
			const clientLoaderData = usePatternClientLoaderData();
			const node = document.createElement("div");
			node.setAttribute("data-pattern-probe", "true");
			createEffect(() => {
				const nextLoaderData = loaderData() as
					| { value?: string }
					| undefined;
				const nextClientLoaderData = clientLoaderData() as
					| string
					| undefined;
				node.textContent = `${nextLoaderData?.value ?? "none"}|${nextClientLoaderData ?? "none"}`;
			});
			return node;
		};
		const container = document.createElement("div");
		document.body.appendChild(container);
		const dispose = renderSolidWithTrackedDisposer(() => {
			return [
				createComponent(solidAdapter.VormaRootOutlet as any, {
					idx: 0,
				}),
				PatternProbe(),
			];
		}, container);

		try {
			await client.vormaNavigate("/pattern-probe-solid-a");
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector("[data-pattern-probe]")
							?.textContent,
					).toBe("probe-a|probe-a");
				},
			});

			await client.vormaNavigate("/pattern-probe-solid-b");
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector("[data-pattern-probe]")
							?.textContent,
					).toBe("none|none");
				},
			});

			await client.vormaNavigate("/pattern-probe-solid-c");
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector("[data-pattern-probe]")
							?.textContent,
					).toBe("probe-c|probe-c");
				},
			});
		} finally {
			dispose();
			container.remove();
		}
	});

	it("react route-props loader/client-loader selectors stay current across stable and scope-change transitions", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const reactAdapter = await import("vorma/react");
		const useLoaderData = reactAdapter.makeTypedUseLoaderData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const usePatternClientLoaderData =
			reactAdapter.makeTypedAddClientLoader(DIST_TEST_VORMA_APP_CONFIG)({
				pattern: "/probe",
				clientLoader: async ({ serverDataPromise }) => {
					const serverData = await serverDataPromise;
					return (
						serverData.loaderData as { value?: string } | undefined
					)?.value;
				},
			});
		const RouteComponent = (routeProps: any) => {
			const loaderData = useLoaderData(routeProps) as
				| { value?: string }
				| undefined;
			const clientLoaderData = usePatternClientLoaderData(routeProps) as
				| string
				| undefined;
			return React.createElement(
				"div",
				{ "data-react-route-props-combined-probe": true },
				`${loaderData?.value ?? "none"}|${clientLoaderData ?? "none"}`,
			);
		};
		vi.doMock("/route-props-combined-react.js", () => ({
			default: RouteComponent,
		}));
		vi.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/probe"],
					importURLs: ["/route-props-combined-react.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{ value: "loader-a" }],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/probe"],
					importURLs: ["/route-props-combined-react.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{ value: "loader-b" }],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/root"],
					importURLs: ["/route-props-combined-react.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{ value: "root-loader-c" }],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/probe"],
					importURLs: ["/route-props-combined-react.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{ value: "loader-d" }],
				}),
			);
		const container = document.createElement("div");
		document.body.appendChild(container);
		const root = createRoot(container);
		try {
			flushSync(() => {
				root.render(
					React.createElement(reactAdapter.VormaRootOutlet as any, {
						idx: 0,
					}),
				);
			});

			await client.vormaNavigate("/route-props-react-a");
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector(
							"[data-react-route-props-combined-probe]",
						)?.textContent,
					).toBe("loader-a|loader-a");
				},
			});

			await client.vormaNavigate("/route-props-react-b");
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector(
							"[data-react-route-props-combined-probe]",
						)?.textContent,
					).toBe("loader-b|loader-b");
				},
			});

			await client.vormaNavigate("/route-props-react-c");
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector(
							"[data-react-route-props-combined-probe]",
						)?.textContent,
					).toBe("root-loader-c|none");
				},
			});

			await client.vormaNavigate("/route-props-react-d");
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector(
							"[data-react-route-props-combined-probe]",
						)?.textContent,
					).toBe("loader-d|loader-d");
				},
			});
		} finally {
			flushSync(() => {
				root.unmount();
			});
			container.remove();
		}
	});

	it("preact route-props loader/client-loader selectors stay current across stable and scope-change transitions", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const preactAdapter = await import("vorma/preact");
		const useLoaderData = preactAdapter.makeTypedUseLoaderData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const usePatternClientLoaderData =
			preactAdapter.makeTypedAddClientLoader(DIST_TEST_VORMA_APP_CONFIG)({
				pattern: "/probe",
				clientLoader: async ({ serverDataPromise }) => {
					const serverData = await serverDataPromise;
					return (
						serverData.loaderData as { value?: string } | undefined
					)?.value;
				},
			});
		const RouteComponent = (routeProps: any) => {
			const loaderData = useLoaderData(routeProps) as
				| { value?: string }
				| undefined;
			const clientLoaderData = usePatternClientLoaderData(routeProps) as
				| string
				| undefined;
			return h(
				"div",
				{ "data-preact-route-props-combined-probe": true },
				`${loaderData?.value ?? "none"}|${clientLoaderData ?? "none"}`,
			);
		};
		vi.doMock("/route-props-combined-preact.js", () => ({
			default: RouteComponent,
		}));
		vi.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/probe"],
					importURLs: ["/route-props-combined-preact.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{ value: "loader-a" }],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/probe"],
					importURLs: ["/route-props-combined-preact.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{ value: "loader-b" }],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/root"],
					importURLs: ["/route-props-combined-preact.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{ value: "root-loader-c" }],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/probe"],
					importURLs: ["/route-props-combined-preact.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{ value: "loader-d" }],
				}),
			);
		const container = document.createElement("div");
		document.body.appendChild(container);
		try {
			await act(async () => {
				renderPreact(
					h(preactAdapter.VormaRootOutlet as any, { idx: 0 }),
					container,
				);
			});

			await client.vormaNavigate("/route-props-preact-a");
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector(
							"[data-preact-route-props-combined-probe]",
						)?.textContent,
					).toBe("loader-a|loader-a");
				},
			});

			await client.vormaNavigate("/route-props-preact-b");
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector(
							"[data-preact-route-props-combined-probe]",
						)?.textContent,
					).toBe("loader-b|loader-b");
				},
			});

			await client.vormaNavigate("/route-props-preact-c");
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector(
							"[data-preact-route-props-combined-probe]",
						)?.textContent,
					).toBe("root-loader-c|none");
				},
			});

			await client.vormaNavigate("/route-props-preact-d");
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector(
							"[data-preact-route-props-combined-probe]",
						)?.textContent,
					).toBe("loader-d|loader-d");
				},
			});
		} finally {
			renderPreact(null, container);
			container.remove();
		}
	});

	it("solid route-props loader/client-loader selectors stay current across stable and scope-change transitions", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const solidAdapter = await import("vorma/solid");
		const useLoaderData = solidAdapter.makeTypedUseLoaderData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const usePatternClientLoaderData =
			solidAdapter.makeTypedAddClientLoader(DIST_TEST_VORMA_APP_CONFIG)({
				pattern: "/probe",
				clientLoader: async ({ serverDataPromise }) => {
					const serverData = await serverDataPromise;
					return (
						serverData.loaderData as { value?: string } | undefined
					)?.value;
				},
			});
		let capturedRouteProps: any = undefined;
		const RouteComponent = (routeProps: any) => {
			createEffect(() => {
				const matchedPattern = (
					routeProps as
						| {
								__vorma_internal_route_scope?: {
									matchedPattern?: string;
								};
						  }
						| undefined
				)?.__vorma_internal_route_scope?.matchedPattern;
				void matchedPattern;
				capturedRouteProps = routeProps;
			});
			return "";
		};
		vi.doMock("/route-props-combined-solid.js", () => ({
			default: RouteComponent,
		}));
		vi.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/probe"],
					importURLs: ["/route-props-combined-solid.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{ value: "loader-a" }],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/probe"],
					importURLs: ["/route-props-combined-solid.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{ value: "loader-b" }],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/root"],
					importURLs: ["/route-props-combined-solid.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{ value: "root-loader-c" }],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/probe"],
					importURLs: ["/route-props-combined-solid.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{ value: "loader-d" }],
				}),
			);
		const outletContainer = document.createElement("div");
		const probeContainer = document.createElement("div");
		document.body.appendChild(outletContainer);
		document.body.appendChild(probeContainer);
		const disposeOutlet = renderSolidWithTrackedDisposer(() => {
			return createComponent(solidAdapter.VormaRootOutlet as any, {
				idx: 0,
			});
		}, outletContainer);
		let disposeProbe: (() => void) | undefined;
		const disposeProbeIfMounted = () => {
			if (disposeProbe === undefined) {
				return;
			}
			disposeProbe();
			disposeProbe = undefined;
		};
		const remountProbeWithCapturedRouteProps = () => {
			disposeProbeIfMounted();
			disposeProbe = renderSolidWithTrackedDisposer(() => {
				if (capturedRouteProps === undefined) {
					return "";
				}
				const loaderData = useLoaderData(capturedRouteProps);
				const clientLoaderData =
					usePatternClientLoaderData(capturedRouteProps);
				const node = document.createElement("div");
				node.setAttribute(
					"data-solid-route-props-combined-probe",
					"true",
				);
				createEffect(() => {
					const nextLoaderData = loaderData() as
						| { value?: string }
						| undefined;
					const nextClientLoaderData = clientLoaderData() as
						| string
						| undefined;
					node.textContent = `${nextLoaderData?.value ?? "none"}|${nextClientLoaderData ?? "none"}`;
				});
				return node;
			}, probeContainer);
		};
		const readCapturedRouteScopeMatchedPattern = () => {
			return (
				capturedRouteProps as
					| {
							__vorma_internal_route_scope?: {
								matchedPattern?: string;
							};
					  }
					| undefined
			)?.__vorma_internal_route_scope?.matchedPattern;
		};
		try {
			await client.vormaNavigate("/route-props-solid-a");
			await waitForDOMCondition({
				assertion: () => {
					expect(capturedRouteProps).toBeDefined();
				},
			});
			remountProbeWithCapturedRouteProps();
			await waitForDOMCondition({
				assertion: () => {
					expect(
						probeContainer.querySelector(
							"[data-solid-route-props-combined-probe]",
						)?.textContent,
					).toBe("loader-a|loader-a");
				},
			});

			await client.vormaNavigate("/route-props-solid-b");
			remountProbeWithCapturedRouteProps();
			await waitForDOMCondition({
				assertion: () => {
					expect(
						probeContainer.querySelector(
							"[data-solid-route-props-combined-probe]",
						)?.textContent,
					).toBe("loader-b|loader-b");
				},
			});

			disposeProbeIfMounted();
			await client.vormaNavigate("/route-props-solid-c");
			await waitForDOMCondition({
				assertion: () => {
					expect(readRouterDataForTesting().matchedPatterns[0]).toBe(
						"/root",
					);
				},
			});
			await waitForDOMCondition({
				assertion: () => {
					expect(readCapturedRouteScopeMatchedPattern()).toBe(
						"/root",
					);
				},
			});
			remountProbeWithCapturedRouteProps();
			await waitForDOMCondition({
				assertion: () => {
					expect(
						probeContainer.querySelector(
							"[data-solid-route-props-combined-probe]",
						)?.textContent,
					).toBe("root-loader-c|none");
				},
			});

			disposeProbeIfMounted();
			await client.vormaNavigate("/route-props-solid-d");
			await waitForDOMCondition({
				assertion: () => {
					expect(readCapturedRouteScopeMatchedPattern()).toBe(
						"/probe",
					);
				},
			});
			remountProbeWithCapturedRouteProps();
			await waitForDOMCondition({
				assertion: () => {
					expect(
						probeContainer.querySelector(
							"[data-solid-route-props-combined-probe]",
						)?.textContent,
					).toBe("loader-d|loader-d");
				},
			});
		} finally {
			disposeProbeIfMounted();
			disposeOutlet();
			outletContainer.remove();
			probeContainer.remove();
			await vi.runAllTimersAsync();
			await Promise.resolve();
		}
	});

	it("react route-props hooks keep owner-consistent values on every transition tick", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const reactAdapter = await import("vorma/react");
		const useLoaderData = reactAdapter.makeTypedUseLoaderData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const useRouterData = reactAdapter.makeTypedUseRouterData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const usePatternClientLoaderData =
			reactAdapter.makeTypedAddClientLoader(DIST_TEST_VORMA_APP_CONFIG)({
				pattern: "/probe",
				clientLoader: async ({ serverDataPromise }) => {
					const serverData = await serverDataPromise;
					return (
						serverData.loaderData as { value?: string } | undefined
					)?.value;
				},
			});
		let phase: "initial" | "to-second" | "to-third" = "initial";
		const trace: Array<{
			phase: string;
			value: string;
			matchedPattern: string;
		}> = [];
		const RouteComponent = (routeProps: any) => {
			const loaderData = useLoaderData(routeProps) as
				| { value?: string }
				| undefined;
			const clientLoaderData = usePatternClientLoaderData(routeProps) as
				| string
				| undefined;
			const routerData = useRouterData();
			const combinedValue = `${loaderData?.value ?? "none"}|${clientLoaderData ?? "none"}`;
			trace.push({
				phase,
				value: combinedValue,
				matchedPattern: routerData.matchedPatterns[0] ?? "<none>",
			});
			return React.createElement(
				"div",
				{ "data-react-route-props-tick-trace": true },
				combinedValue,
			);
		};
		vi.doMock("/route-props-tick-react.js", () => ({
			default: RouteComponent,
		}));
		vi.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/probe"],
					importURLs: ["/route-props-tick-react.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{ value: "loader-a" }],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/probe"],
					importURLs: ["/route-props-tick-react.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{ value: "loader-b" }],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/probe"],
					importURLs: ["/route-props-tick-react.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{ value: "loader-c" }],
				}),
			);
		const container = document.createElement("div");
		document.body.appendChild(container);
		const root = createRoot(container);
		try {
			flushSync(() => {
				root.render(
					React.createElement(reactAdapter.VormaRootOutlet as any, {
						idx: 0,
					}),
				);
			});

			await client.vormaNavigate("/route-props-tick-react-a");
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector(
							"[data-react-route-props-tick-trace]",
						)?.textContent,
					).toBe("loader-a|loader-a");
				},
			});

			phase = "to-second";
			await client.vormaNavigate("/route-props-tick-react-b");
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector(
							"[data-react-route-props-tick-trace]",
						)?.textContent,
					).toBe("loader-b|loader-b");
				},
			});

			phase = "to-third";
			await client.vormaNavigate("/route-props-tick-react-c");
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector(
							"[data-react-route-props-tick-trace]",
						)?.textContent,
					).toBe("loader-c|loader-c");
				},
			});

			const initialTrace = trace.filter((entry) => {
				return (
					entry.phase === "initial" &&
					entry.matchedPattern === "/probe"
				);
			});
			const secondTrace = trace.filter((entry) => {
				return (
					entry.phase === "to-second" &&
					entry.matchedPattern === "/probe"
				);
			});
			const thirdTrace = trace.filter((entry) => {
				return (
					entry.phase === "to-third" &&
					entry.matchedPattern === "/probe"
				);
			});
			expect(initialTrace.length).toBeGreaterThan(0);
			expect(secondTrace.length).toBeGreaterThan(0);
			expect(thirdTrace.length).toBeGreaterThan(0);
			expect(
				initialTrace.every(
					(entry) =>
						entry.value === "loader-a|loader-a" &&
						entry.matchedPattern === "/probe",
				),
			).toBe(true);
			expect(
				secondTrace.every(
					(entry) =>
						entry.value === "loader-b|loader-b" &&
						entry.matchedPattern === "/probe",
				),
			).toBe(true);
			expect(
				thirdTrace.every(
					(entry) =>
						entry.value === "loader-c|loader-c" &&
						entry.matchedPattern === "/probe",
				),
			).toBe(true);
		} finally {
			flushSync(() => {
				root.unmount();
			});
			container.remove();
		}
	});

	it("preact route-props hooks keep owner-consistent values on every transition tick", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const preactAdapter = await import("vorma/preact");
		const useLoaderData = preactAdapter.makeTypedUseLoaderData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const useRouterData = preactAdapter.makeTypedUseRouterData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const usePatternClientLoaderData =
			preactAdapter.makeTypedAddClientLoader(DIST_TEST_VORMA_APP_CONFIG)({
				pattern: "/probe",
				clientLoader: async ({ serverDataPromise }) => {
					const serverData = await serverDataPromise;
					return (
						serverData.loaderData as { value?: string } | undefined
					)?.value;
				},
			});
		let phase: "initial" | "to-second" | "to-third" = "initial";
		const trace: Array<{
			phase: string;
			value: string;
			matchedPattern: string;
		}> = [];
		const RouteComponent = (routeProps: any) => {
			const loaderData = useLoaderData(routeProps) as
				| { value?: string }
				| undefined;
			const clientLoaderData = usePatternClientLoaderData(routeProps) as
				| string
				| undefined;
			const routerData = useRouterData();
			const combinedValue = `${loaderData?.value ?? "none"}|${clientLoaderData ?? "none"}`;
			trace.push({
				phase,
				value: combinedValue,
				matchedPattern: routerData.matchedPatterns[0] ?? "<none>",
			});
			return h(
				"div",
				{ "data-preact-route-props-tick-trace": true },
				combinedValue,
			);
		};
		vi.doMock("/route-props-tick-preact.js", () => ({
			default: RouteComponent,
		}));
		vi.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/probe"],
					importURLs: ["/route-props-tick-preact.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{ value: "loader-a" }],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/probe"],
					importURLs: ["/route-props-tick-preact.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{ value: "loader-b" }],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/probe"],
					importURLs: ["/route-props-tick-preact.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{ value: "loader-c" }],
				}),
			);
		const container = document.createElement("div");
		document.body.appendChild(container);
		try {
			await act(async () => {
				renderPreact(
					h(preactAdapter.VormaRootOutlet as any, { idx: 0 }),
					container,
				);
			});

			await client.vormaNavigate("/route-props-tick-preact-a");
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector(
							"[data-preact-route-props-tick-trace]",
						)?.textContent,
					).toBe("loader-a|loader-a");
				},
			});

			phase = "to-second";
			await client.vormaNavigate("/route-props-tick-preact-b");
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector(
							"[data-preact-route-props-tick-trace]",
						)?.textContent,
					).toBe("loader-b|loader-b");
				},
			});

			phase = "to-third";
			await client.vormaNavigate("/route-props-tick-preact-c");
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector(
							"[data-preact-route-props-tick-trace]",
						)?.textContent,
					).toBe("loader-c|loader-c");
				},
			});

			const initialTrace = trace.filter((entry) => {
				return (
					entry.phase === "initial" &&
					entry.matchedPattern === "/probe"
				);
			});
			const secondTrace = trace.filter((entry) => {
				return (
					entry.phase === "to-second" &&
					entry.matchedPattern === "/probe"
				);
			});
			const thirdTrace = trace.filter((entry) => {
				return (
					entry.phase === "to-third" &&
					entry.matchedPattern === "/probe"
				);
			});
			expect(initialTrace.length).toBeGreaterThan(0);
			expect(secondTrace.length).toBeGreaterThan(0);
			expect(thirdTrace.length).toBeGreaterThan(0);
			expect(
				initialTrace.every(
					(entry) =>
						entry.value === "loader-a|loader-a" &&
						entry.matchedPattern === "/probe",
				),
			).toBe(true);
			expect(
				secondTrace.every(
					(entry) =>
						entry.value === "loader-b|loader-b" &&
						entry.matchedPattern === "/probe",
				),
			).toBe(true);
			expect(
				thirdTrace.every(
					(entry) =>
						entry.value === "loader-c|loader-c" &&
						entry.matchedPattern === "/probe",
				),
			).toBe(true);
		} finally {
			renderPreact(null, container);
			container.remove();
		}
	});

	it("solid route-props hooks keep owner-consistent values on every transition tick", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const solidAdapter = await import("vorma/solid");
		const useLoaderData = solidAdapter.makeTypedUseLoaderData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const useRouterData = solidAdapter.makeTypedUseRouterData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const usePatternClientLoaderData =
			solidAdapter.makeTypedAddClientLoader(DIST_TEST_VORMA_APP_CONFIG)({
				pattern: "/probe",
				clientLoader: async ({ serverDataPromise }) => {
					const serverData = await serverDataPromise;
					return (
						serverData.loaderData as { value?: string } | undefined
					)?.value;
				},
			});
		let phase: "initial" | "to-second" | "to-third" = "initial";
		const trace: Array<{
			phase: string;
			value: string;
			matchedPattern: string;
		}> = [];
		const RouteComponent = (routeProps: any) => {
			const loaderData = useLoaderData(routeProps);
			const clientLoaderData = usePatternClientLoaderData(routeProps);
			const routerData = useRouterData();
			createEffect(() => {
				const nextLoaderData = loaderData() as
					| { value?: string }
					| undefined;
				const nextClientLoaderData = clientLoaderData() as
					| string
					| undefined;
				const nextRouterData = routerData();
				const combinedValue = `${nextLoaderData?.value ?? "none"}|${nextClientLoaderData ?? "none"}`;
				trace.push({
					phase,
					value: combinedValue,
					matchedPattern:
						nextRouterData.matchedPatterns[0] ?? "<none>",
				});
			});
			return "";
		};
		vi.doMock("/route-props-tick-solid.js", () => ({
			default: RouteComponent,
		}));
		vi.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/probe"],
					importURLs: ["/route-props-tick-solid.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{ value: "loader-a" }],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/probe"],
					importURLs: ["/route-props-tick-solid.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{ value: "loader-b" }],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/probe"],
					importURLs: ["/route-props-tick-solid.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{ value: "loader-c" }],
				}),
			);
		const container = document.createElement("div");
		document.body.appendChild(container);
		const dispose = renderSolidWithTrackedDisposer(() => {
			return createComponent(solidAdapter.VormaRootOutlet as any, {
				idx: 0,
			});
		}, container);
		try {
			await client.vormaNavigate("/route-props-tick-solid-a");
			await waitForDOMCondition({
				assertion: () => {
					expect(
						trace.some((entry) => {
							return (
								entry.phase === "initial" &&
								entry.value === "loader-a|loader-a" &&
								entry.matchedPattern === "/probe"
							);
						}),
					).toBe(true);
				},
			});

			phase = "to-second";
			await client.vormaNavigate("/route-props-tick-solid-b");
			await waitForDOMCondition({
				assertion: () => {
					expect(
						trace.some((entry) => {
							return (
								entry.phase === "to-second" &&
								entry.value === "loader-b|loader-b" &&
								entry.matchedPattern === "/probe"
							);
						}),
					).toBe(true);
				},
			});

			phase = "to-third";
			await client.vormaNavigate("/route-props-tick-solid-c");
			await waitForDOMCondition({
				assertion: () => {
					expect(
						trace.some((entry) => {
							return (
								entry.phase === "to-third" &&
								entry.value === "loader-c|loader-c" &&
								entry.matchedPattern === "/probe"
							);
						}),
					).toBe(true);
				},
			});

			const initialTrace = trace.filter((entry) => {
				return (
					entry.phase === "initial" &&
					entry.matchedPattern === "/probe"
				);
			});
			const secondTrace = trace.filter((entry) => {
				return (
					entry.phase === "to-second" &&
					entry.matchedPattern === "/probe"
				);
			});
			const thirdTrace = trace.filter((entry) => {
				return (
					entry.phase === "to-third" &&
					entry.matchedPattern === "/probe"
				);
			});
			expect(initialTrace.length).toBeGreaterThan(0);
			expect(secondTrace.length).toBeGreaterThan(0);
			expect(thirdTrace.length).toBeGreaterThan(0);
			expect(
				initialTrace.every(
					(entry) =>
						entry.value === "loader-a|loader-a" &&
						entry.matchedPattern === "/probe",
				),
			).toBe(true);
			expect(
				secondTrace.every(
					(entry) =>
						entry.value === "loader-b|loader-b" &&
						entry.matchedPattern === "/probe",
				),
			).toBe(true);
			expect(
				thirdTrace.every(
					(entry) =>
						entry.value === "loader-c|loader-c" &&
						entry.matchedPattern === "/probe",
				),
			).toBe(true);
		} finally {
			dispose();
			container.remove();
		}
	});

	it("react global selectors keep snapshot-coherent values on every transition tick", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const reactAdapter = await import("vorma/react");
		const useRouterData = reactAdapter.makeTypedUseRouterData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const usePatternLoaderData = reactAdapter.makeTypedUsePatternLoaderData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const usePatternClientLoaderData =
			reactAdapter.makeTypedAddClientLoader(DIST_TEST_VORMA_APP_CONFIG)({
				pattern: "/probe",
				clientLoader: async ({ serverDataPromise }) => {
					const serverData = await serverDataPromise;
					return (
						serverData.loaderData as { value?: string } | undefined
					)?.value;
				},
			});
		let phase: "initial" | "to-second" | "to-third" = "initial";
		const trace: Array<{
			phase: string;
			value: string;
			matchedPattern: string;
		}> = [];
		const GlobalProbe = () => {
			const routerData = useRouterData();
			const matchedPattern = (routerData.matchedPatterns[0] ??
				"/probe") as any;
			const loaderData = usePatternLoaderData(matchedPattern) as
				| { value?: string }
				| undefined;
			const clientLoaderData = usePatternClientLoaderData() as
				| string
				| undefined;
			const combinedValue = `${loaderData?.value ?? "none"}|${clientLoaderData ?? "none"}`;
			trace.push({
				phase,
				value: combinedValue,
				matchedPattern: routerData.matchedPatterns[0] ?? "<none>",
			});
			return React.createElement(
				"div",
				{ "data-react-global-tick-trace": true },
				combinedValue,
			);
		};
		vi.doMock("/global-tick-react.js", () => ({
			default: () => React.createElement("div", {}, "root"),
		}));
		vi.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/probe"],
					importURLs: ["/global-tick-react.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{ value: "loader-a" }],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/probe"],
					importURLs: ["/global-tick-react.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{ value: "loader-b" }],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/probe"],
					importURLs: ["/global-tick-react.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{ value: "loader-c" }],
				}),
			);
		const container = document.createElement("div");
		document.body.appendChild(container);
		const root = createRoot(container);
		try {
			flushSync(() => {
				root.render(
					React.createElement(
						React.Fragment,
						{},
						React.createElement(
							reactAdapter.VormaRootOutlet as any,
							{
								idx: 0,
							},
						),
						React.createElement(GlobalProbe),
					),
				);
			});

			await client.vormaNavigate("/global-tick-react-a");
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector(
							"[data-react-global-tick-trace]",
						)?.textContent,
					).toBe("loader-a|loader-a");
				},
			});

			phase = "to-second";
			await client.vormaNavigate("/global-tick-react-b");
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector(
							"[data-react-global-tick-trace]",
						)?.textContent,
					).toBe("loader-b|loader-b");
				},
			});

			phase = "to-third";
			await client.vormaNavigate("/global-tick-react-c");
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector(
							"[data-react-global-tick-trace]",
						)?.textContent,
					).toBe("loader-c|loader-c");
				},
			});

			const initialTrace = trace.filter((entry) => {
				return (
					entry.phase === "initial" &&
					entry.matchedPattern === "/probe"
				);
			});
			const secondTrace = trace.filter((entry) => {
				return (
					entry.phase === "to-second" &&
					entry.matchedPattern === "/probe"
				);
			});
			const thirdTrace = trace.filter((entry) => {
				return (
					entry.phase === "to-third" &&
					entry.matchedPattern === "/probe"
				);
			});
			expect(initialTrace.length).toBeGreaterThan(0);
			expect(secondTrace.length).toBeGreaterThan(0);
			expect(thirdTrace.length).toBeGreaterThan(0);
			expect(
				initialTrace.every(
					(entry) =>
						entry.value === "loader-a|loader-a" &&
						entry.matchedPattern === "/probe",
				),
			).toBe(true);
			expect(
				secondTrace.every(
					(entry) =>
						entry.value === "loader-b|loader-b" &&
						entry.matchedPattern === "/probe",
				),
			).toBe(true);
			expect(
				thirdTrace.every(
					(entry) =>
						entry.value === "loader-c|loader-c" &&
						entry.matchedPattern === "/probe",
				),
			).toBe(true);
		} finally {
			flushSync(() => {
				root.unmount();
			});
			container.remove();
		}
	});

	it("preact global selectors keep snapshot-coherent values on every transition tick", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const preactAdapter = await import("vorma/preact");
		const useRouterData = preactAdapter.makeTypedUseRouterData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const usePatternLoaderData =
			preactAdapter.makeTypedUsePatternLoaderData(
				DIST_TEST_VORMA_APP_CONFIG,
			);
		const usePatternClientLoaderData =
			preactAdapter.makeTypedAddClientLoader(DIST_TEST_VORMA_APP_CONFIG)({
				pattern: "/probe",
				clientLoader: async ({ serverDataPromise }) => {
					const serverData = await serverDataPromise;
					return (
						serverData.loaderData as { value?: string } | undefined
					)?.value;
				},
			});
		let phase: "initial" | "to-second" | "to-third" = "initial";
		const trace: Array<{
			phase: string;
			value: string;
			matchedPattern: string;
		}> = [];
		const GlobalProbe = () => {
			const routerData = useRouterData();
			const matchedPattern = (routerData.matchedPatterns[0] ??
				"/probe") as any;
			const loaderData = usePatternLoaderData(matchedPattern) as
				| { value?: string }
				| undefined;
			const clientLoaderData = usePatternClientLoaderData() as
				| string
				| undefined;
			const combinedValue = `${loaderData?.value ?? "none"}|${clientLoaderData ?? "none"}`;
			trace.push({
				phase,
				value: combinedValue,
				matchedPattern: routerData.matchedPatterns[0] ?? "<none>",
			});
			return h(
				"div",
				{ "data-preact-global-tick-trace": true },
				combinedValue,
			);
		};
		vi.doMock("/global-tick-preact.js", () => ({
			default: () => h("div", {}, "root"),
		}));
		vi.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/probe"],
					importURLs: ["/global-tick-preact.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{ value: "loader-a" }],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/probe"],
					importURLs: ["/global-tick-preact.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{ value: "loader-b" }],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/probe"],
					importURLs: ["/global-tick-preact.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{ value: "loader-c" }],
				}),
			);
		const container = document.createElement("div");
		document.body.appendChild(container);
		try {
			await act(async () => {
				renderPreact(
					h(
						"div",
						{},
						h(preactAdapter.VormaRootOutlet as any, { idx: 0 }),
						h(GlobalProbe, {}),
					),
					container,
				);
			});

			await client.vormaNavigate("/global-tick-preact-a");
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector(
							"[data-preact-global-tick-trace]",
						)?.textContent,
					).toBe("loader-a|loader-a");
				},
			});

			phase = "to-second";
			await client.vormaNavigate("/global-tick-preact-b");
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector(
							"[data-preact-global-tick-trace]",
						)?.textContent,
					).toBe("loader-b|loader-b");
				},
			});

			phase = "to-third";
			await client.vormaNavigate("/global-tick-preact-c");
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector(
							"[data-preact-global-tick-trace]",
						)?.textContent,
					).toBe("loader-c|loader-c");
				},
			});

			const initialTrace = trace.filter((entry) => {
				return (
					entry.phase === "initial" &&
					entry.matchedPattern === "/probe"
				);
			});
			const secondTrace = trace.filter((entry) => {
				return (
					entry.phase === "to-second" &&
					entry.matchedPattern === "/probe"
				);
			});
			const thirdTrace = trace.filter((entry) => {
				return (
					entry.phase === "to-third" &&
					entry.matchedPattern === "/probe"
				);
			});
			expect(initialTrace.length).toBeGreaterThan(0);
			expect(secondTrace.length).toBeGreaterThan(0);
			expect(thirdTrace.length).toBeGreaterThan(0);
			expect(
				initialTrace.every(
					(entry) =>
						entry.value.startsWith("loader-a|") &&
						entry.matchedPattern === "/probe",
				),
			).toBe(true);
			expect(
				secondTrace.every(
					(entry) =>
						entry.value.startsWith("loader-b|") &&
						entry.matchedPattern === "/probe",
				),
			).toBe(true);
			expect(
				thirdTrace.every(
					(entry) =>
						entry.value.startsWith("loader-c|") &&
						entry.matchedPattern === "/probe",
				),
			).toBe(true);
		} finally {
			renderPreact(null, container);
			container.remove();
		}
	});

	it("solid global selectors keep snapshot-coherent values on every transition tick", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const solidAdapter = await import("vorma/solid");
		const useRouterData = solidAdapter.makeTypedUseRouterData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const usePatternLoaderData = solidAdapter.makeTypedUsePatternLoaderData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const usePatternClientLoaderData =
			solidAdapter.makeTypedAddClientLoader(DIST_TEST_VORMA_APP_CONFIG)({
				pattern: "/probe",
				clientLoader: async ({ serverDataPromise }) => {
					const serverData = await serverDataPromise;
					return (
						serverData.loaderData as { value?: string } | undefined
					)?.value;
				},
			});
		let phase: "initial" | "to-second" | "to-third" = "initial";
		const trace: Array<{
			phase: string;
			value: string;
			matchedPattern: string;
		}> = [];
		const GlobalProbe = () => {
			const routerData = useRouterData();
			const loaderData = usePatternLoaderData("/probe" as any);
			const clientLoaderData = usePatternClientLoaderData();
			const node = document.createElement("div");
			node.setAttribute("data-solid-global-tick-trace", "true");
			createEffect(() => {
				const nextRouterData = routerData();
				const nextLoaderData = loaderData() as
					| { value?: string }
					| undefined;
				const nextClientLoaderData = clientLoaderData() as
					| string
					| undefined;
				const combinedValue = `${nextLoaderData?.value ?? "none"}|${nextClientLoaderData ?? "none"}`;
				trace.push({
					phase,
					value: combinedValue,
					matchedPattern:
						nextRouterData.matchedPatterns[0] ?? "<none>",
				});
				node.textContent = combinedValue;
			});
			return node;
		};
		vi.doMock("/global-tick-solid.js", () => ({
			default: () => "",
		}));
		vi.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/probe"],
					importURLs: ["/global-tick-solid.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{ value: "loader-a" }],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/probe"],
					importURLs: ["/global-tick-solid.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{ value: "loader-b" }],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/probe"],
					importURLs: ["/global-tick-solid.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{ value: "loader-c" }],
				}),
			);
		const container = document.createElement("div");
		document.body.appendChild(container);
		const dispose = renderSolidWithTrackedDisposer(() => {
			return [
				createComponent(solidAdapter.VormaRootOutlet as any, {
					idx: 0,
				}),
				GlobalProbe(),
			];
		}, container);
		try {
			await client.vormaNavigate("/global-tick-solid-a");
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector(
							"[data-solid-global-tick-trace]",
						)?.textContent,
					).toBe("loader-a|loader-a");
				},
			});

			phase = "to-second";
			await client.vormaNavigate("/global-tick-solid-b");
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector(
							"[data-solid-global-tick-trace]",
						)?.textContent,
					).toBe("loader-b|loader-b");
				},
			});

			phase = "to-third";
			await client.vormaNavigate("/global-tick-solid-c");
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector(
							"[data-solid-global-tick-trace]",
						)?.textContent,
					).toBe("loader-c|loader-c");
				},
			});

			const initialTrace = trace.filter((entry) => {
				return (
					entry.phase === "initial" &&
					entry.matchedPattern === "/probe"
				);
			});
			const secondTrace = trace.filter((entry) => {
				return (
					entry.phase === "to-second" &&
					entry.matchedPattern === "/probe"
				);
			});
			const thirdTrace = trace.filter((entry) => {
				return (
					entry.phase === "to-third" &&
					entry.matchedPattern === "/probe"
				);
			});
			expect(initialTrace.length).toBeGreaterThan(0);
			expect(secondTrace.length).toBeGreaterThan(0);
			expect(thirdTrace.length).toBeGreaterThan(0);
			expect(
				initialTrace.every(
					(entry) =>
						entry.value.startsWith("loader-a|") &&
						entry.matchedPattern === "/probe",
				),
			).toBe(true);
			expect(
				secondTrace.every(
					(entry) =>
						entry.value.startsWith("loader-b|") &&
						entry.matchedPattern === "/probe",
				),
			).toBe(true);
			expect(
				thirdTrace.every(
					(entry) =>
						entry.value.startsWith("loader-c|") &&
						entry.matchedPattern === "/probe",
				),
			).toBe(true);
		} finally {
			dispose();
			container.remove();
		}
	});

	it("react nested route-props loader data follows route-key remounts when component identity is reused", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const reactAdapter = await import("vorma/react");
		const useLoaderData = reactAdapter.makeTypedUseLoaderData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const RootComponent = (routeProps: any) => {
			return routeProps.Outlet(undefined);
		};
		const SharedChildComponent = (routeProps: any) => {
			const loaderData = useLoaderData(routeProps) as
				| { value?: string }
				| undefined;
			return React.createElement(
				"div",
				{ "data-react-nested-route-props-loader-probe": true },
				loaderData?.value ?? "none",
			);
		};
		vi.doMock("/nested-remount-root-react.js", () => ({
			default: RootComponent,
		}));
		vi.doMock("/nested-remount-child-a-react.js", () => ({
			default: SharedChildComponent,
		}));
		vi.doMock("/nested-remount-child-b-react.js", () => ({
			default: SharedChildComponent,
		}));
		vi.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/root", "/probe"],
					importURLs: [
						"/nested-remount-root-react.js",
						"/nested-remount-child-a-react.js",
					],
					exportKeys: ["default", "default"],
					errorExportKeys: ["", ""],
					loadersData: [{ value: "root-a" }, { value: "loader-a" }],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/root", "/other"],
					importURLs: [
						"/nested-remount-root-react.js",
						"/nested-remount-child-b-react.js",
					],
					exportKeys: ["default", "default"],
					errorExportKeys: ["", ""],
					loadersData: [{ value: "root-b" }, { value: "loader-b" }],
				}),
			);
		const container = document.createElement("div");
		document.body.appendChild(container);
		const root = createRoot(container);
		try {
			flushSync(() => {
				root.render(
					React.createElement(reactAdapter.VormaRootOutlet as any, {
						idx: 0,
					}),
				);
			});

			await client.vormaNavigate("/nested-remount-react-a");
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector(
							"[data-react-nested-route-props-loader-probe]",
						)?.textContent,
					).toBe("loader-a");
				},
			});

			await client.vormaNavigate("/nested-remount-react-b");
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector(
							"[data-react-nested-route-props-loader-probe]",
						)?.textContent,
					).toBe("loader-b");
				},
			});
		} finally {
			flushSync(() => {
				root.unmount();
			});
			container.remove();
		}
	});

	it("preact nested route-props loader data follows route-key remounts when component identity is reused", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const preactAdapter = await import("vorma/preact");
		const useLoaderData = preactAdapter.makeTypedUseLoaderData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const RootComponent = (routeProps: any) => {
			return routeProps.Outlet(undefined);
		};
		const SharedChildComponent = (routeProps: any) => {
			const loaderData = useLoaderData(routeProps) as
				| { value?: string }
				| undefined;
			return h(
				"div",
				{ "data-preact-nested-route-props-loader-probe": true },
				loaderData?.value ?? "none",
			);
		};
		vi.doMock("/nested-remount-root-preact.js", () => ({
			default: RootComponent,
		}));
		vi.doMock("/nested-remount-child-a-preact.js", () => ({
			default: SharedChildComponent,
		}));
		vi.doMock("/nested-remount-child-b-preact.js", () => ({
			default: SharedChildComponent,
		}));
		vi.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/root", "/probe"],
					importURLs: [
						"/nested-remount-root-preact.js",
						"/nested-remount-child-a-preact.js",
					],
					exportKeys: ["default", "default"],
					errorExportKeys: ["", ""],
					loadersData: [{ value: "root-a" }, { value: "loader-a" }],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/root", "/other"],
					importURLs: [
						"/nested-remount-root-preact.js",
						"/nested-remount-child-b-preact.js",
					],
					exportKeys: ["default", "default"],
					errorExportKeys: ["", ""],
					loadersData: [{ value: "root-b" }, { value: "loader-b" }],
				}),
			);
		const container = document.createElement("div");
		document.body.appendChild(container);
		try {
			await act(async () => {
				renderPreact(
					h(preactAdapter.VormaRootOutlet as any, { idx: 0 }),
					container,
				);
			});

			await client.vormaNavigate("/nested-remount-preact-a");
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector(
							"[data-preact-nested-route-props-loader-probe]",
						)?.textContent,
					).toBe("loader-a");
				},
			});

			await client.vormaNavigate("/nested-remount-preact-b");
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector(
							"[data-preact-nested-route-props-loader-probe]",
						)?.textContent,
					).toBe("loader-b");
				},
			});
		} finally {
			renderPreact(null, container);
			container.remove();
		}
	});

	it("solid nested route-props loader data follows route-key remounts when component identity is reused", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const solidAdapter = await import("vorma/solid");
		const useLoaderData = solidAdapter.makeTypedUseLoaderData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const RootComponent = (routeProps: any) => {
			return createComponent(routeProps.Outlet as any, {});
		};
		const observedLoaderValues: string[] = [];
		const SharedChildComponent = (routeProps: any) => {
			const loaderData = useLoaderData(routeProps);
			createEffect(() => {
				const nextLoaderData = loaderData() as
					| { value?: string }
					| undefined;
				observedLoaderValues.push(nextLoaderData?.value ?? "none");
			});
			return "";
		};
		vi.doMock("/nested-remount-root-solid.js", () => ({
			default: RootComponent,
		}));
		vi.doMock("/nested-remount-child-a-solid.js", () => ({
			default: SharedChildComponent,
		}));
		vi.doMock("/nested-remount-child-b-solid.js", () => ({
			default: SharedChildComponent,
		}));
		vi.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/root", "/probe"],
					importURLs: [
						"/nested-remount-root-solid.js",
						"/nested-remount-child-a-solid.js",
					],
					exportKeys: ["default", "default"],
					errorExportKeys: ["", ""],
					loadersData: [{ value: "root-a" }, { value: "loader-a" }],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/root", "/other"],
					importURLs: [
						"/nested-remount-root-solid.js",
						"/nested-remount-child-b-solid.js",
					],
					exportKeys: ["default", "default"],
					errorExportKeys: ["", ""],
					loadersData: [{ value: "root-b" }, { value: "loader-b" }],
				}),
			);
		const container = document.createElement("div");
		document.body.appendChild(container);
		const dispose = renderSolidWithTrackedDisposer(() => {
			return createComponent(solidAdapter.VormaRootOutlet as any, {
				idx: 0,
			});
		}, container);
		try {
			await client.vormaNavigate("/nested-remount-solid-a");
			await waitForDOMCondition({
				assertion: () => {
					expect(observedLoaderValues).toContain("loader-a");
				},
			});

			await client.vormaNavigate("/nested-remount-solid-b");
			await waitForDOMCondition({
				assertion: () => {
					expect(observedLoaderValues).toContain("loader-b");
				},
			});
		} finally {
			dispose();
			container.remove();
		}
	});

	it("react route-props ownership remains correct after a render-abort before commit", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const reactAdapter = await import("vorma/react");
		const useLoaderData = reactAdapter.makeTypedUseLoaderData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		let shouldThrowDuringRender = true;
		const RouteComponent = (routeProps: any) => {
			const loaderData = useLoaderData(routeProps) as
				| { value?: string }
				| undefined;
			if (shouldThrowDuringRender) {
				throw new Error("intentional render abort");
			}
			return React.createElement(
				"div",
				{ "data-react-render-abort-loader-probe": true },
				loaderData?.value ?? "none",
			);
		};
		vi.doMock("/route-props-render-abort-react.js", () => ({
			default: RouteComponent,
		}));
		vi.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/probe"],
					importURLs: ["/route-props-render-abort-react.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{ value: "loader-a" }],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/probe"],
					importURLs: ["/route-props-render-abort-react.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{ value: "loader-b" }],
				}),
			);

		class RenderAbortBoundary extends React.Component<
			{ children?: React.ReactNode },
			{ didError: boolean }
		> {
			constructor(props: { children?: React.ReactNode }) {
				super(props);
				this.state = { didError: false };
			}

			static getDerivedStateFromError() {
				return { didError: true };
			}

			override componentDidCatch() {}

			override render() {
				if (this.state.didError) {
					return React.createElement(
						"div",
						{ "data-react-render-abort-fallback": true },
						"render-error",
					);
				}
				return this.props.children;
			}
		}

		const container = document.createElement("div");
		document.body.appendChild(container);
		const root = createRoot(container);
		try {
			flushSync(() => {
				root.render(
					React.createElement(
						RenderAbortBoundary,
						{ key: "first-render" },
						React.createElement(
							reactAdapter.VormaRootOutlet as any,
							{
								idx: 0,
							},
						),
					),
				);
			});
			await client.vormaNavigate("/route-props-render-abort-a");
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector(
							"[data-react-render-abort-fallback]",
						),
					).toBeTruthy();
				},
			});

			shouldThrowDuringRender = false;
			await client.vormaNavigate("/route-props-render-abort-b");
			flushSync(() => {
				root.render(
					React.createElement(
						RenderAbortBoundary,
						{ key: "second-render" },
						React.createElement(
							reactAdapter.VormaRootOutlet as any,
							{
								idx: 0,
							},
						),
					),
				);
			});
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector(
							"[data-react-render-abort-loader-probe]",
						)?.textContent,
					).toBe("loader-b");
				},
			});
		} finally {
			flushSync(() => {
				root.unmount();
			});
			container.remove();
		}
	});

	it("react root outlet stays stable on data-only route updates while data hooks stay fresh", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const reactAdapter = await import("vorma/react");
		let stableRootRefAttachCount = 0;
		let stableRootRefDetachCount = 0;
		let stableRootRenderCount = 0;
		const stableRootRef = (node: HTMLDivElement | null) => {
			if (node === null) {
				stableRootRefDetachCount += 1;
				return;
			}
			stableRootRefAttachCount += 1;
		};
		const StableRootComp = vi.fn(() => {
			stableRootRenderCount += 1;
			return React.createElement("div", { ref: stableRootRef }, "root");
		});
		vi.doMock("/data-stable-react.js", () => ({
			default: StableRootComp,
		}));
		vi.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/"],
					importURLs: ["/data-stable-react.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{ value: "a" }],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/"],
					importURLs: ["/data-stable-react.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{ value: "b" }],
				}),
			);
		const useRouterData = reactAdapter.makeTypedUseRouterData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const usePatternLoaderData = reactAdapter.makeTypedUsePatternLoaderData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		let dataRenderCount = 0;
		let latestDataProbeValue = "";
		const DataProbe = () => {
			dataRenderCount += 1;
			const routerData = useRouterData();
			const loaderData = usePatternLoaderData("/" as any) as
				| { value?: string }
				| undefined;
			latestDataProbeValue = `${loaderData?.value ?? ""}|${routerData.matchedPatterns.join(",")}`;
			return React.createElement("div", {}, latestDataProbeValue);
		};
		const container = document.createElement("div");
		document.body.appendChild(container);
		const root = createRoot(container);
		try {
			flushSync(() => {
				root.render(
					React.createElement(
						React.Fragment,
						{},
						React.createElement(
							reactAdapter.VormaRootOutlet as any,
							{
								idx: 0,
							},
						),
						React.createElement(DataProbe),
					),
				);
			});
			await client.vormaNavigate("/data-stable-react");
			await waitForDOMCondition({
				assertion: () => {
					expect(latestDataProbeValue).toBe("a|/");
				},
			});
			await waitForDOMCondition({
				assertion: () => {
					expect(stableRootRefAttachCount).toBeGreaterThan(0);
				},
			});
			const stableRootAttachCountAfterInitial = stableRootRefAttachCount;
			const stableRootDetachCountAfterInitial = stableRootRefDetachCount;
			const dataRenderCountAfterInitial = dataRenderCount;
			const stableRootRenderCountAfterInitial = stableRootRenderCount;

			await client.revalidate();
			await waitForDOMCondition({
				assertion: () => {
					expect(latestDataProbeValue).toBe("b|/");
				},
			});
			expect(stableRootRefAttachCount).toBe(
				stableRootAttachCountAfterInitial,
			);
			expect(stableRootRefDetachCount).toBe(
				stableRootDetachCountAfterInitial,
			);
			expect(stableRootRenderCount).toBeGreaterThanOrEqual(
				stableRootRenderCountAfterInitial,
			);
			expect(dataRenderCount).toBeGreaterThan(
				dataRenderCountAfterInitial,
			);
		} finally {
			flushSync(() => {
				root.unmount();
			});
			container.remove();
		}
	});

	it("preact root outlet stays stable on data-only route updates while data signals stay fresh", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const preactAdapter = await import("vorma/preact");
		let stableRootMountCount = 0;
		let stableRootRenderCount = 0;
		let stableRootLatestInstanceID = 0;
		const StableRootComp = vi.fn(() => {
			stableRootRenderCount += 1;
			const [instanceID] = usePreactState(() => {
				stableRootMountCount += 1;
				return stableRootMountCount;
			});
			stableRootLatestInstanceID = instanceID;
			return h("div", {}, "root");
		});
		vi.doMock("/data-stable-preact.js", () => ({
			default: StableRootComp,
		}));
		vi.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/"],
					importURLs: ["/data-stable-preact.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{ value: "a" }],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/"],
					importURLs: ["/data-stable-preact.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{ value: "b" }],
				}),
			);
		const useRouterData = preactAdapter.makeTypedUseRouterData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const usePatternLoaderData =
			preactAdapter.makeTypedUsePatternLoaderData(
				DIST_TEST_VORMA_APP_CONFIG,
			);
		let dataRenderCount = 0;
		let latestDataProbeValue = "";
		const DataProbe = () => {
			dataRenderCount += 1;
			const routerData = useRouterData();
			const loaderData = usePatternLoaderData("/" as any) as
				| { value?: string }
				| undefined;
			latestDataProbeValue = `${loaderData?.value ?? ""}|${routerData.matchedPatterns.join(",")}`;
			return h("div", {}, latestDataProbeValue);
		};
		const container = document.createElement("div");
		document.body.appendChild(container);
		try {
			await act(async () => {
				renderPreact(
					h(
						"div",
						{},
						h(preactAdapter.VormaRootOutlet as any, {
							idx: 0,
						}),
						h(DataProbe, {}),
					),
					container,
				);
			});
			await client.vormaNavigate("/data-stable-preact");
			await waitForDOMCondition({
				assertion: () => {
					expect(latestDataProbeValue).toBe("a|/");
				},
			});
			const stableRootMountCountAfterInitial = stableRootMountCount;
			const dataRenderCountAfterInitial = dataRenderCount;
			const stableRootRenderCountAfterInitial = stableRootRenderCount;
			const stableRootInstanceIDAfterInitial = stableRootLatestInstanceID;

			await client.revalidate();
			await waitForDOMCondition({
				assertion: () => {
					expect(latestDataProbeValue).toBe("b|/");
				},
			});
			expect(stableRootMountCount).toBe(stableRootMountCountAfterInitial);
			expect(stableRootLatestInstanceID).toBe(
				stableRootInstanceIDAfterInitial,
			);
			expect(stableRootRenderCount).toBeGreaterThanOrEqual(
				stableRootRenderCountAfterInitial,
			);
			expect(dataRenderCount).toBeGreaterThan(
				dataRenderCountAfterInitial,
			);
		} finally {
			renderPreact(null, container);
			container.remove();
		}
	});

	it("solid root outlet stays stable on data-only route updates while data signals stay fresh", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const solidAdapter = await import("vorma/solid");
		let stableRootCleanupCount = 0;
		const StableRootComp = vi.fn(() => {
			onCleanup(() => {
				stableRootCleanupCount += 1;
			});
			return "solid-root";
		});
		vi.doMock("/data-stable-solid.js", () => ({
			default: StableRootComp,
		}));
		vi.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/"],
					importURLs: ["/data-stable-solid.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{ value: "a" }],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/"],
					importURLs: ["/data-stable-solid.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{ value: "b" }],
				}),
			);
		const useRouterData = solidAdapter.makeTypedUseRouterData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const usePatternLoaderData = solidAdapter.makeTypedUsePatternLoaderData(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		let latestDataProbeValue = "";
		let dataSignalUpdateCount = 0;
		const DataProbe = () => {
			const routerData = useRouterData();
			const loaderData = usePatternLoaderData("/" as any);
			createEffect(() => {
				dataSignalUpdateCount += 1;
				latestDataProbeValue = `${(loaderData() as { value?: string } | undefined)?.value ?? ""}|${routerData().matchedPatterns.join(",")}`;
			});
			return "";
		};

		const container = document.createElement("div");
		document.body.appendChild(container);
		const dispose = renderSolidWithTrackedDisposer(() => {
			return [
				createComponent(solidAdapter.VormaRootOutlet as any, {
					idx: 0,
				}),
				createComponent(DataProbe as any, {}),
			] as any;
		}, container);

		try {
			await client.vormaNavigate("/data-stable-solid-a");
			await waitForDOMCondition({
				assertion: () => {
					expect(latestDataProbeValue).toBe("a|/");
				},
			});
			const stableRootCleanupCountAfterInitial = stableRootCleanupCount;
			const dataSignalUpdateCountAfterInitial = dataSignalUpdateCount;

			await client.revalidate();
			await waitForDOMCondition({
				assertion: () => {
					expect(latestDataProbeValue).toBe("b|/");
				},
			});
			expect(stableRootCleanupCount).toBe(
				stableRootCleanupCountAfterInitial,
			);
			expect(dataSignalUpdateCount).toBeGreaterThan(
				dataSignalUpdateCountAfterInitial,
			);
		} finally {
			dispose();
			container.remove();
		}
	});

	it("cross-adapter route-props transition traces are parity-consistent", async () => {
		const reactTrace = await captureRoutePropsTransitionTraceByAdapter({
			adapterName: "react",
		});
		const preactTrace = await captureRoutePropsTransitionTraceByAdapter({
			adapterName: "preact",
		});
		const solidTrace = await captureRoutePropsTransitionTraceByAdapter({
			adapterName: "solid",
		});

		expect(reactTrace).toEqual([
			{
				phase: "initial",
				value: "loader-a|loader-a",
				matchedPattern: "/probe",
			},
			{
				phase: "to-root",
				value: "loader-b|loader-b",
				matchedPattern: "/probe",
			},
			{
				phase: "back-to-probe",
				value: "loader-c|loader-c",
				matchedPattern: "/probe",
			},
		]);
		expect(preactTrace).toEqual(reactTrace);
		expect(solidTrace).toEqual(reactTrace);
	});

	it("cross-adapter global selector transition traces are parity-consistent", async () => {
		const reactTrace = await captureGlobalSelectorTransitionTraceByAdapter({
			adapterName: "react",
		});
		const preactTrace = await captureGlobalSelectorTransitionTraceByAdapter(
			{
				adapterName: "preact",
			},
		);
		const solidTrace = await captureGlobalSelectorTransitionTraceByAdapter({
			adapterName: "solid",
		});

		expect(reactTrace).toEqual([
			{
				phase: "initial",
				value: "loader-a|loader-a",
				matchedPattern: "/probe",
			},
			{
				phase: "to-root",
				value: "root-loader-b|none",
				matchedPattern: "/root",
			},
			{
				phase: "back-to-probe",
				value: "loader-c|loader-c",
				matchedPattern: "/probe",
			},
		]);
		expect(preactTrace).toEqual(reactTrace);
		expect(solidTrace).toEqual(reactTrace);
	});

	it("cross-adapter nested route remount behavior is parity-consistent", async () => {
		const reactValues = await captureNestedRouteRemountValuesByAdapter({
			adapterName: "react",
		});
		const preactValues = await captureNestedRouteRemountValuesByAdapter({
			adapterName: "preact",
		});
		const solidValues = await captureNestedRouteRemountValuesByAdapter({
			adapterName: "solid",
		});

		expect(reactValues).toEqual(["loader-a", "loader-b"]);
		expect(preactValues).toEqual(reactValues);
		expect(solidValues).toEqual(reactValues);
	});

	it("cross-adapter data-only updates preserve stable component mounts", async () => {
		const reactStability = await captureDataOnlyUpdateStabilityByAdapter({
			adapterName: "react",
		});
		const preactStability = await captureDataOnlyUpdateStabilityByAdapter({
			adapterName: "preact",
		});
		const solidStability = await captureDataOnlyUpdateStabilityByAdapter({
			adapterName: "solid",
		});

		expect(reactStability).toEqual({ mountCount: 1 });
		expect(preactStability).toEqual(reactStability);
		expect(solidStability).toEqual(reactStability);
	});

	it("react root outlet preserves child instance until child module identity changes and ignores location-only updates", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const reactAdapter = await import("vorma/react");
		let childAMountCount = 0;
		let childACleanupCount = 0;
		let childBRenderCount = 0;
		const ChildA = () => {
			const mountIDRef = React.useRef<number | null>(null);
			if (mountIDRef.current === null) {
				childAMountCount += 1;
				mountIDRef.current = childAMountCount;
			}
			React.useEffect(() => {
				return () => {
					childACleanupCount += 1;
				};
			}, []);
			return React.createElement(
				"div",
				{ "data-react-preserve-child": true },
				`child:${mountIDRef.current}`,
			);
		};
		const ChildB = () => {
			childBRenderCount += 1;
			return React.createElement(
				"div",
				{ "data-react-preserve-child": true },
				"child-b",
			);
		};
		const Parent = (props: {
			Outlet: (localProps?: Record<string, unknown>) => unknown;
		}) => React.createElement(props.Outlet as any, {});
		vi.doMock("/react-parent.js", () => ({
			default: Parent,
		}));
		vi.doMock("/react-child-a.js", () => ({
			default: ChildA,
		}));
		vi.doMock("/react-child-b.js", () => ({
			default: ChildB,
		}));
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/parent", "/parent/child"],
					importURLs: ["/react-parent.js", "/react-child-a.js"],
					exportKeys: ["default", "default"],
					errorExportKeys: ["", ""],
					loadersData: [{}, {}],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/parent", "/parent/child"],
					importURLs: ["/react-parent.js", "/react-child-b.js"],
					exportKeys: ["default", "default"],
					errorExportKeys: ["", ""],
					loadersData: [{}, {}],
				}),
			);

		const container = document.createElement("div");
		document.body.appendChild(container);
		const root = createRoot(container);
		flushSync(() => {
			root.render(
				React.createElement(reactAdapter.VormaRootOutlet as any, {
					idx: 0,
				}),
			);
		});

		try {
			await client.vormaNavigate("/react-preserve-child-a");
			await waitForDOMCondition({
				assertion: () => {
					expect(container.textContent).toContain("child:1");
				},
			});

			const childACleanupCountAfterInitial = childACleanupCount;
			window.dispatchEvent(
				new CustomEvent("vorma:location", {
					detail: {
						pathname: window.location.pathname,
						search: window.location.search,
						hash: window.location.hash,
						state: window.history.state,
					},
				}),
			);
			await vi.runAllTimersAsync();
			expect(fetchSpy).toHaveBeenCalledTimes(1);
			expect(childACleanupCount).toBe(childACleanupCountAfterInitial);

			await client.vormaNavigate("/react-preserve-child-b");
			await waitForDOMCondition({
				assertion: () => {
					expect(container.textContent).toContain("child-b");
				},
			});
			expect(childAMountCount).toBeGreaterThan(0);
			expect(childBRenderCount).toBeGreaterThan(0);
			expect(childACleanupCount).toBeGreaterThanOrEqual(1);
		} finally {
			root.unmount();
			container.remove();
		}
	});

	it("react root outlet uses the freshest child component when route updates keep the same child key", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const reactAdapter = await import("vorma/react");
		const ChildA = vi.fn(() =>
			React.createElement("div", {}, "react-child-a"),
		);
		const ChildB = vi.fn(() =>
			React.createElement("div", {}, "react-child-b"),
		);
		const Parent = (props: {
			Outlet: (localProps?: Record<string, unknown>) => unknown;
		}) => React.createElement(props.Outlet as any, {});
		vi.doMock("/react-fresh-parent.js", () => ({
			default: Parent,
		}));
		vi.doMock("/react-fresh-child.js", () => ({
			default: ChildA,
		}));
		vi.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/parent", "/parent/child"],
					importURLs: [
						"/react-fresh-parent.js",
						"/react-fresh-child.js",
					],
					exportKeys: ["default", "default"],
					errorExportKeys: ["", ""],
					loadersData: [{}, {}],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/parent", "/parent/child"],
					importURLs: [
						"/react-fresh-parent.js",
						"/react-fresh-child.js",
					],
					exportKeys: ["default", "default"],
					errorExportKeys: ["", ""],
					loadersData: [{}, {}],
				}),
			);

		const container = document.createElement("div");
		document.body.appendChild(container);
		const root = createRoot(container);
		flushSync(() => {
			root.render(
				React.createElement(reactAdapter.VormaRootOutlet as any, {
					idx: 0,
				}),
			);
		});

		try {
			await client.vormaNavigate("/react-fresh-child-a");
			await waitForDOMCondition({
				assertion: () => {
					expect(ChildA).toHaveBeenCalled();
				},
			});

			vi.doMock("/react-fresh-child.js", () => ({
				default: ChildB,
			}));

			await client.vormaNavigate("/react-fresh-child-b");
			await waitForDOMCondition({
				assertion: () => {
					expect(ChildB).toHaveBeenCalled();
				},
			});
		} finally {
			root.unmount();
			container.remove();
		}
	});

	it("react root outlet uses the freshest root component when route updates keep the same root key", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const reactAdapter = await import("vorma/react");
		const RootA = vi.fn(() =>
			React.createElement("div", {}, "react-root-a"),
		);
		const RootB = vi.fn(() =>
			React.createElement("div", {}, "react-root-b"),
		);
		vi.doMock("/react-fresh-root.js", () => ({
			default: RootA,
		}));
		vi.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/"],
					importURLs: ["/react-fresh-root.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{}],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/"],
					importURLs: ["/react-fresh-root.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{}],
				}),
			);

		const container = document.createElement("div");
		document.body.appendChild(container);
		const root = createRoot(container);
		flushSync(() => {
			root.render(
				React.createElement(reactAdapter.VormaRootOutlet as any, {
					idx: 0,
				}),
			);
		});

		try {
			await client.vormaNavigate("/react-fresh-root-a");
			await waitForDOMCondition({
				assertion: () => {
					expect(RootA).toHaveBeenCalled();
				},
			});

			vi.doMock("/react-fresh-root.js", () => ({
				default: RootB,
			}));

			await client.vormaNavigate("/react-fresh-root-b");
			await waitForDOMCondition({
				assertion: () => {
					expect(RootB).toHaveBeenCalled();
				},
			});
		} finally {
			root.unmount();
			container.remove();
		}
	});

	it("react keeps parent instance and local parent input state when only child route changes", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const reactAdapter = await import("vorma/react");
		let parentMountCount = 0;
		let parentUnmountCount = 0;
		const ChildA = () => React.createElement("div", {}, "react-child-a");
		const ChildB = () => React.createElement("div", {}, "react-child-b");
		const Parent = (props: {
			Outlet: (localProps?: Record<string, unknown>) => unknown;
		}) => {
			const mountIDRef = React.useRef<number | null>(null);
			if (mountIDRef.current === null) {
				parentMountCount += 1;
				mountIDRef.current = parentMountCount;
			}
			React.useEffect(() => {
				return () => {
					parentUnmountCount += 1;
				};
			}, []);
			const [draft, setDraft] = React.useState("initial");
			return React.createElement(
				"section",
				{},
				React.createElement(
					"div",
					{ "data-react-parent-mount-id": true },
					String(mountIDRef.current),
				),
				React.createElement(
					"div",
					{ "data-react-parent-draft": true },
					draft,
				),
				React.createElement("input", {
					value: draft,
					onInput: (event: React.FormEvent<HTMLInputElement>) => {
						setDraft(event.currentTarget.value);
					},
				}),
				React.createElement(props.Outlet as any, {}),
			);
		};
		vi.doMock("/react-parent-state.js", () => ({
			default: Parent,
		}));
		vi.doMock("/react-parent-child-a.js", () => ({
			default: ChildA,
		}));
		vi.doMock("/react-parent-child-b.js", () => ({
			default: ChildB,
		}));
		vi.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/parent", "/parent/child"],
					importURLs: [
						"/react-parent-state.js",
						"/react-parent-child-a.js",
					],
					exportKeys: ["default", "default"],
					errorExportKeys: ["", ""],
					loadersData: [{}, {}],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/parent", "/parent/child"],
					importURLs: [
						"/react-parent-state.js",
						"/react-parent-child-b.js",
					],
					exportKeys: ["default", "default"],
					errorExportKeys: ["", ""],
					loadersData: [{}, {}],
				}),
			);

		const container = document.createElement("div");
		document.body.appendChild(container);
		const root = createRoot(container);
		flushSync(() => {
			root.render(
				React.createElement(reactAdapter.VormaRootOutlet as any, {
					idx: 0,
				}),
			);
		});

		try {
			await client.vormaNavigate("/react-parent-state-a");
			await waitForDOMCondition({
				assertion: () => {
					expect(container.textContent).toContain("react-child-a");
				},
			});

			const input = container.querySelector("input");
			if (!(input instanceof HTMLInputElement)) {
				throw new Error("Expected react parent input.");
			}
			input.value = "typed-value";
			input.dispatchEvent(new Event("input", { bubbles: true }));
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector("[data-react-parent-draft]")
							?.textContent,
					).toBe("typed-value");
				},
			});

			await client.vormaNavigate("/react-parent-state-b");
			await waitForDOMCondition({
				assertion: () => {
					expect(container.textContent).toContain("react-child-b");
				},
			});
			expect(
				container.querySelector("[data-react-parent-draft]")
					?.textContent,
			).toBe("typed-value");
			expect(
				container.querySelector("[data-react-parent-mount-id]")
					?.textContent,
			).toBe("1");
			expect(parentMountCount).toBe(1);
			expect(parentUnmountCount).toBe(0);
		} finally {
			root.unmount();
			container.remove();
		}
	});

	it("react remounts child when child export key changes even if import URL stays the same", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const reactAdapter = await import("vorma/react");
		let childMountCount = 0;
		let childCleanupCount = 0;
		const ChildA = () => {
			const mountIDRef = React.useRef<number | null>(null);
			if (mountIDRef.current === null) {
				childMountCount += 1;
				mountIDRef.current = childMountCount;
			}
			React.useEffect(() => {
				return () => {
					childCleanupCount += 1;
				};
			}, []);
			return React.createElement(
				"div",
				{ "data-react-export-key-child": true },
				`child:${mountIDRef.current}`,
			);
		};
		const Parent = (props: {
			Outlet: (localProps?: Record<string, unknown>) => unknown;
		}) => React.createElement(props.Outlet as any, {});
		vi.doMock("/react-export-parent.js", () => ({
			default: Parent,
		}));
		vi.doMock("/react-export-child.js", () => ({
			childA: ChildA,
			childB: ChildA,
		}));
		vi.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/parent", "/parent/child"],
					importURLs: [
						"/react-export-parent.js",
						"/react-export-child.js",
					],
					exportKeys: ["default", "childA"],
					errorExportKeys: ["", ""],
					loadersData: [{}, {}],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/parent", "/parent/child"],
					importURLs: [
						"/react-export-parent.js",
						"/react-export-child.js",
					],
					exportKeys: ["default", "childB"],
					errorExportKeys: ["", ""],
					loadersData: [{}, {}],
				}),
			);

		const container = document.createElement("div");
		document.body.appendChild(container);
		const root = createRoot(container);
		flushSync(() => {
			root.render(
				React.createElement(reactAdapter.VormaRootOutlet as any, {
					idx: 0,
				}),
			);
		});

		try {
			await client.vormaNavigate("/react-export-key-a");
			await waitForDOMCondition({
				assertion: () => {
					expect(container.textContent).toContain("child:1");
				},
			});

			await client.vormaNavigate("/react-export-key-b");
			await waitForDOMCondition({
				assertion: () => {
					expect(container.textContent).toContain("child:2");
				},
			});
			expect(childCleanupCount).toBeGreaterThanOrEqual(1);
		} finally {
			root.unmount();
			container.remove();
		}
	});

	it("preact root outlet preserves child instance until child module identity changes and ignores location-only updates", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const preactAdapter = await import("vorma/preact");
		let childAMountCount = 0;
		let childACleanupCount = 0;
		let childBRenderCount = 0;
		const ChildA = () => {
			const mountIDRef = usePreactRef<number | null>(null);
			if (mountIDRef.current === null) {
				childAMountCount += 1;
				mountIDRef.current = childAMountCount;
			}
			usePreactEffect(() => {
				return () => {
					childACleanupCount += 1;
				};
			}, []);
			return h(
				"div",
				{ "data-preact-preserve-child": true },
				`child:${mountIDRef.current}`,
			);
		};
		const ChildB = () => {
			childBRenderCount += 1;
			return h("div", { "data-preact-preserve-child": true }, "child-b");
		};
		const Parent = (props: {
			Outlet: (localProps?: Record<string, unknown>) => unknown;
		}) => h(props.Outlet as any, {});
		vi.doMock("/preact-parent.js", () => ({
			default: Parent,
		}));
		vi.doMock("/preact-child-a.js", () => ({
			default: ChildA,
		}));
		vi.doMock("/preact-child-b.js", () => ({
			default: ChildB,
		}));
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/parent", "/parent/child"],
					importURLs: ["/preact-parent.js", "/preact-child-a.js"],
					exportKeys: ["default", "default"],
					errorExportKeys: ["", ""],
					loadersData: [{}, {}],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/parent", "/parent/child"],
					importURLs: ["/preact-parent.js", "/preact-child-b.js"],
					exportKeys: ["default", "default"],
					errorExportKeys: ["", ""],
					loadersData: [{}, {}],
				}),
			);

		const container = document.createElement("div");
		document.body.appendChild(container);
		renderPreact(
			h(preactAdapter.VormaRootOutlet as any, {
				idx: 0,
			}),
			container,
		);

		try {
			await client.vormaNavigate("/preact-preserve-child-a");
			await waitForDOMCondition({
				assertion: () => {
					expect(container.textContent).toContain("child:1");
				},
			});

			const childACleanupCountAfterInitial = childACleanupCount;
			window.dispatchEvent(
				new CustomEvent("vorma:location", {
					detail: {
						pathname: window.location.pathname,
						search: window.location.search,
						hash: window.location.hash,
						state: window.history.state,
					},
				}),
			);
			await vi.runAllTimersAsync();
			expect(fetchSpy).toHaveBeenCalledTimes(1);
			expect(childACleanupCount).toBe(childACleanupCountAfterInitial);

			await client.vormaNavigate("/preact-preserve-child-b");
			await waitForDOMCondition({
				assertion: () => {
					expect(container.textContent).toContain("child-b");
				},
			});
			expect(childAMountCount).toBeGreaterThan(0);
			expect(childBRenderCount).toBeGreaterThan(0);
			expect(childACleanupCount).toBeGreaterThanOrEqual(1);
		} finally {
			renderPreact(null, container);
			container.remove();
		}
	});

	it("preact root outlet uses the freshest child component when route updates keep the same child key", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const preactAdapter = await import("vorma/preact");
		const ChildA = vi.fn(() => h("div", {}, "preact-child-a"));
		const ChildB = vi.fn(() => h("div", {}, "preact-child-b"));
		const Parent = (props: {
			Outlet: (localProps?: Record<string, unknown>) => unknown;
		}) => h(props.Outlet as any, {});
		vi.doMock("/preact-fresh-parent.js", () => ({
			default: Parent,
		}));
		vi.doMock("/preact-fresh-child.js", () => ({
			default: ChildA,
		}));
		vi.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/parent", "/parent/child"],
					importURLs: [
						"/preact-fresh-parent.js",
						"/preact-fresh-child.js",
					],
					exportKeys: ["default", "default"],
					errorExportKeys: ["", ""],
					loadersData: [{}, {}],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/parent", "/parent/child"],
					importURLs: [
						"/preact-fresh-parent.js",
						"/preact-fresh-child.js",
					],
					exportKeys: ["default", "default"],
					errorExportKeys: ["", ""],
					loadersData: [{}, {}],
				}),
			);

		const container = document.createElement("div");
		document.body.appendChild(container);
		renderPreact(
			h(preactAdapter.VormaRootOutlet as any, {
				idx: 0,
			}),
			container,
		);

		try {
			await client.vormaNavigate("/preact-fresh-child-a");
			await waitForDOMCondition({
				assertion: () => {
					expect(ChildA).toHaveBeenCalled();
				},
			});

			vi.doMock("/preact-fresh-child.js", () => ({
				default: ChildB,
			}));

			await client.vormaNavigate("/preact-fresh-child-b");
			await waitForDOMCondition({
				assertion: () => {
					expect(ChildB).toHaveBeenCalled();
				},
			});
		} finally {
			renderPreact(null, container);
			container.remove();
		}
	});

	it("preact root outlet uses the freshest root component when route updates keep the same root key", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const preactAdapter = await import("vorma/preact");
		const RootA = vi.fn(() => h("div", {}, "preact-root-a"));
		const RootB = vi.fn(() => h("div", {}, "preact-root-b"));
		vi.doMock("/preact-fresh-root.js", () => ({
			default: RootA,
		}));
		vi.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/"],
					importURLs: ["/preact-fresh-root.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{}],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/"],
					importURLs: ["/preact-fresh-root.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{}],
				}),
			);

		const container = document.createElement("div");
		document.body.appendChild(container);
		renderPreact(
			h(preactAdapter.VormaRootOutlet as any, {
				idx: 0,
			}),
			container,
		);

		try {
			await client.vormaNavigate("/preact-fresh-root-a");
			await waitForDOMCondition({
				assertion: () => {
					expect(RootA).toHaveBeenCalled();
				},
			});

			vi.doMock("/preact-fresh-root.js", () => ({
				default: RootB,
			}));

			await client.vormaNavigate("/preact-fresh-root-b");
			await waitForDOMCondition({
				assertion: () => {
					expect(RootB).toHaveBeenCalled();
				},
			});
		} finally {
			renderPreact(null, container);
			container.remove();
		}
	});

	it("preact keeps parent instance and local parent input state when only child route changes", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const preactAdapter = await import("vorma/preact");
		let parentMountCount = 0;
		let parentUnmountCount = 0;
		const ChildA = () => h("div", {}, "preact-child-a");
		const ChildB = () => h("div", {}, "preact-child-b");
		const Parent = (props: {
			Outlet: (localProps?: Record<string, unknown>) => unknown;
		}) => {
			const mountIDRef = usePreactRef<number | null>(null);
			if (mountIDRef.current === null) {
				parentMountCount += 1;
				mountIDRef.current = parentMountCount;
			}
			usePreactEffect(() => {
				return () => {
					parentUnmountCount += 1;
				};
			}, []);
			const [draft, setDraft] = usePreactState("initial");
			return h(
				"section",
				{},
				h(
					"div",
					{ "data-preact-parent-mount-id": true },
					String(mountIDRef.current),
				),
				h("div", { "data-preact-parent-draft": true }, draft),
				h("input", {
					value: draft,
					onInput: (event: Event) => {
						const target = event.target as HTMLInputElement;
						setDraft(target.value);
					},
				}),
				h(props.Outlet as any, {}),
			);
		};
		vi.doMock("/preact-parent-state.js", () => ({
			default: Parent,
		}));
		vi.doMock("/preact-parent-child-a.js", () => ({
			default: ChildA,
		}));
		vi.doMock("/preact-parent-child-b.js", () => ({
			default: ChildB,
		}));
		vi.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/parent", "/parent/child"],
					importURLs: [
						"/preact-parent-state.js",
						"/preact-parent-child-a.js",
					],
					exportKeys: ["default", "default"],
					errorExportKeys: ["", ""],
					loadersData: [{}, {}],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/parent", "/parent/child"],
					importURLs: [
						"/preact-parent-state.js",
						"/preact-parent-child-b.js",
					],
					exportKeys: ["default", "default"],
					errorExportKeys: ["", ""],
					loadersData: [{}, {}],
				}),
			);

		const container = document.createElement("div");
		document.body.appendChild(container);
		renderPreact(
			h(preactAdapter.VormaRootOutlet as any, {
				idx: 0,
			}),
			container,
		);

		try {
			await client.vormaNavigate("/preact-parent-state-a");
			await waitForDOMCondition({
				assertion: () => {
					expect(container.textContent).toContain("preact-child-a");
				},
			});

			const input = container.querySelector("input");
			if (!(input instanceof HTMLInputElement)) {
				throw new Error("Expected preact parent input.");
			}
			input.value = "typed-value";
			input.dispatchEvent(new Event("input", { bubbles: true }));
			await waitForDOMCondition({
				assertion: () => {
					expect(
						container.querySelector("[data-preact-parent-draft]")
							?.textContent,
					).toBe("typed-value");
				},
			});

			await client.vormaNavigate("/preact-parent-state-b");
			await waitForDOMCondition({
				assertion: () => {
					expect(container.textContent).toContain("preact-child-b");
				},
			});
			expect(
				container.querySelector("[data-preact-parent-draft]")
					?.textContent,
			).toBe("typed-value");
			expect(
				container.querySelector("[data-preact-parent-mount-id]")
					?.textContent,
			).toBe("1");
			expect(parentMountCount).toBe(1);
			expect(parentUnmountCount).toBe(0);
		} finally {
			renderPreact(null, container);
			container.remove();
		}
	});

	it("preact remounts child when child export key changes even if import URL stays the same", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const preactAdapter = await import("vorma/preact");
		let childMountCount = 0;
		let childCleanupCount = 0;
		const ChildA = () => {
			const mountIDRef = usePreactRef<number | null>(null);
			if (mountIDRef.current === null) {
				childMountCount += 1;
				mountIDRef.current = childMountCount;
			}
			usePreactEffect(() => {
				return () => {
					childCleanupCount += 1;
				};
			}, []);
			return h(
				"div",
				{ "data-preact-export-key-child": true },
				`child:${mountIDRef.current}`,
			);
		};
		const Parent = (props: {
			Outlet: (localProps?: Record<string, unknown>) => unknown;
		}) => h(props.Outlet as any, {});
		vi.doMock("/preact-export-parent.js", () => ({
			default: Parent,
		}));
		vi.doMock("/preact-export-child.js", () => ({
			childA: ChildA,
			childB: ChildA,
		}));
		vi.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/parent", "/parent/child"],
					importURLs: [
						"/preact-export-parent.js",
						"/preact-export-child.js",
					],
					exportKeys: ["default", "childA"],
					errorExportKeys: ["", ""],
					loadersData: [{}, {}],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/parent", "/parent/child"],
					importURLs: [
						"/preact-export-parent.js",
						"/preact-export-child.js",
					],
					exportKeys: ["default", "childB"],
					errorExportKeys: ["", ""],
					loadersData: [{}, {}],
				}),
			);

		const container = document.createElement("div");
		document.body.appendChild(container);
		renderPreact(
			h(preactAdapter.VormaRootOutlet as any, {
				idx: 0,
			}),
			container,
		);

		try {
			await client.vormaNavigate("/preact-export-key-a");
			await waitForDOMCondition({
				assertion: () => {
					expect(container.textContent).toContain("child:1");
				},
			});

			await client.vormaNavigate("/preact-export-key-b");
			await waitForDOMCondition({
				assertion: () => {
					expect(container.textContent).toContain("child:2");
				},
			});
			expect(childMountCount).toBeGreaterThanOrEqual(2);
		} finally {
			renderPreact(null, container);
			container.remove();
		}
	});

	it("solid remounts child when child export key changes even if import URL stays the same", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const solidAdapter = await import("vorma/solid");
		let childAMountCount = 0;
		let childACleanupCount = 0;
		let childBMountCount = 0;
		const ChildA = () => {
			childAMountCount += 1;
			onCleanup(() => {
				childACleanupCount += 1;
			});
			return "solid-child-a";
		};
		const ChildB = () => {
			childBMountCount += 1;
			return "solid-child-b";
		};
		const Parent = (props: {
			Outlet: (localProps?: Record<string, unknown>) => unknown;
		}) => props.Outlet({});
		vi.doMock("/solid-export-parent.js", () => ({
			default: Parent,
		}));
		vi.doMock("/solid-export-child.js", () => ({
			childA: ChildA,
			childB: ChildB,
		}));
		vi.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/parent", "/parent/child"],
					importURLs: [
						"/solid-export-parent.js",
						"/solid-export-child.js",
					],
					exportKeys: ["default", "childA"],
					errorExportKeys: ["", ""],
					loadersData: [{}, {}],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/parent", "/parent/child"],
					importURLs: [
						"/solid-export-parent.js",
						"/solid-export-child.js",
					],
					exportKeys: ["default", "childB"],
					errorExportKeys: ["", ""],
					loadersData: [{}, {}],
				}),
			);

		const container = document.createElement("div");
		document.body.appendChild(container);
		const dispose = renderSolidWithTrackedDisposer(() => {
			return createComponent(solidAdapter.VormaRootOutlet as any, {
				idx: 0,
			});
		}, container);

		try {
			await client.vormaNavigate("/solid-export-key-a");
			await waitForDOMCondition({
				assertion: () => {
					expect(childAMountCount).toBeGreaterThan(0);
				},
			});

			await client.vormaNavigate("/solid-export-key-b");
			await waitForDOMCondition({
				assertion: () => {
					expect(childBMountCount).toBeGreaterThan(0);
				},
			});
			expect(childACleanupCount).toBeGreaterThanOrEqual(1);
		} finally {
			dispose();
			container.remove();
		}
	});

	it("solid remounts child when matched pattern changes even if component identity is reused", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const solidAdapter = await import("vorma/solid");
		let sharedChildMountCount = 0;
		let sharedChildCleanupCount = 0;
		const SharedChild = () => {
			sharedChildMountCount += 1;
			onCleanup(() => {
				sharedChildCleanupCount += 1;
			});
			return "solid-shared-child";
		};
		const Parent = (props: {
			Outlet: (localProps?: Record<string, unknown>) => unknown;
		}) => props.Outlet({});
		vi.doMock("/solid-pattern-remount-parent.js", () => ({
			default: Parent,
		}));
		vi.doMock("/solid-pattern-remount-child.js", () => ({
			default: SharedChild,
		}));
		vi.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/parent", "/parent/child-a"],
					importURLs: [
						"/solid-pattern-remount-parent.js",
						"/solid-pattern-remount-child.js",
					],
					exportKeys: ["default", "default"],
					errorExportKeys: ["", ""],
					loadersData: [{}, { value: "a" }],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/parent", "/parent/child-b"],
					importURLs: [
						"/solid-pattern-remount-parent.js",
						"/solid-pattern-remount-child.js",
					],
					exportKeys: ["default", "default"],
					errorExportKeys: ["", ""],
					loadersData: [{}, { value: "b" }],
				}),
			);

		const container = document.createElement("div");
		document.body.appendChild(container);
		const dispose = renderSolidWithTrackedDisposer(() => {
			return createComponent(solidAdapter.VormaRootOutlet as any, {
				idx: 0,
			});
		}, container);

		try {
			await client.vormaNavigate("/solid-pattern-remount-a");
			await waitForDOMCondition({
				assertion: () => {
					expect(sharedChildMountCount).toBe(1);
				},
			});

			await client.vormaNavigate("/solid-pattern-remount-b");
			await waitForDOMCondition({
				assertion: () => {
					expect(sharedChildMountCount).toBeGreaterThanOrEqual(2);
				},
			});
			expect(sharedChildCleanupCount).toBeGreaterThanOrEqual(1);
		} finally {
			dispose();
			container.remove();
		}
	});

	it("solid remounts a deep child when only the deepest matched pattern changes under stable ancestors", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const solidAdapter = await import("vorma/solid");
		let sharedDeepChildMountCount = 0;
		let sharedDeepChildCleanupCount = 0;
		const SharedDeepChild = () => {
			sharedDeepChildMountCount += 1;
			onCleanup(() => {
				sharedDeepChildCleanupCount += 1;
			});
			return "solid-shared-deep-child";
		};
		const Root = (props: {
			Outlet: (localProps?: Record<string, unknown>) => unknown;
		}) => props.Outlet({});
		const Middle = (props: {
			Outlet: (localProps?: Record<string, unknown>) => unknown;
		}) => props.Outlet({});
		vi.doMock("/solid-deep-remount-root.js", () => ({
			default: Root,
		}));
		vi.doMock("/solid-deep-remount-middle.js", () => ({
			default: Middle,
		}));
		vi.doMock("/solid-deep-remount-child.js", () => ({
			default: SharedDeepChild,
		}));
		vi.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/root", "/root/mid", "/root/mid/a"],
					importURLs: [
						"/solid-deep-remount-root.js",
						"/solid-deep-remount-middle.js",
						"/solid-deep-remount-child.js",
					],
					exportKeys: ["default", "default", "default"],
					errorExportKeys: ["", "", ""],
					loadersData: [{}, {}, { value: "a" }],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/root", "/root/mid", "/root/mid/b"],
					importURLs: [
						"/solid-deep-remount-root.js",
						"/solid-deep-remount-middle.js",
						"/solid-deep-remount-child.js",
					],
					exportKeys: ["default", "default", "default"],
					errorExportKeys: ["", "", ""],
					loadersData: [{}, {}, { value: "b" }],
				}),
			);

		const container = document.createElement("div");
		document.body.appendChild(container);
		const dispose = renderSolidWithTrackedDisposer(() => {
			return createComponent(solidAdapter.VormaRootOutlet as any, {
				idx: 0,
			});
		}, container);

		try {
			await client.vormaNavigate("/solid-deep-remount-a");
			await waitForDOMCondition({
				assertion: () => {
					expect(sharedDeepChildMountCount).toBe(1);
				},
			});

			await client.vormaNavigate("/solid-deep-remount-b");
			await waitForDOMCondition({
				assertion: () => {
					expect(sharedDeepChildMountCount).toBeGreaterThanOrEqual(2);
				},
			});
			expect(sharedDeepChildCleanupCount).toBeGreaterThanOrEqual(1);
		} finally {
			dispose();
			container.remove();
		}
	});

	it("solid keeps parent instance and local parent input state when only child route changes", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const solidAdapter = await import("vorma/solid");
		let parentMountCount = 0;
		let parentCleanupCount = 0;
		let setParentDraftValueForTest:
			| ((nextDraft: string) => void)
			| undefined;
		let readParentDraftValueForTest: (() => string) | undefined;
		let childARenderCount = 0;
		let childBRenderCount = 0;
		const ChildA = () => {
			childARenderCount += 1;
			return "solid-child-a";
		};
		const ChildB = () => {
			childBRenderCount += 1;
			return "solid-child-b";
		};
		const Parent = (props: {
			Outlet: (localProps?: Record<string, unknown>) => unknown;
		}) => {
			parentMountCount += 1;
			onCleanup(() => {
				parentCleanupCount += 1;
			});
			let draft = "initial";
			setParentDraftValueForTest = (nextDraft: string) => {
				draft = nextDraft;
			};
			readParentDraftValueForTest = () => draft;
			return createComponent(props.Outlet as any, {});
		};
		vi.doMock("/solid-parent-state.js", () => ({
			default: Parent,
		}));
		vi.doMock("/solid-parent-child-a.js", () => ({
			default: ChildA,
		}));
		vi.doMock("/solid-parent-child-b.js", () => ({
			default: ChildB,
		}));
		vi.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/parent", "/parent/child"],
					importURLs: [
						"/solid-parent-state.js",
						"/solid-parent-child-a.js",
					],
					exportKeys: ["default", "default"],
					errorExportKeys: ["", ""],
					loadersData: [{}, {}],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/parent", "/parent/child"],
					importURLs: [
						"/solid-parent-state.js",
						"/solid-parent-child-b.js",
					],
					exportKeys: ["default", "default"],
					errorExportKeys: ["", ""],
					loadersData: [{}, {}],
				}),
			);

		const container = document.createElement("div");
		document.body.appendChild(container);
		const dispose = renderSolidWithTrackedDisposer(() => {
			return createComponent(solidAdapter.VormaRootOutlet as any, {
				idx: 0,
			});
		}, container);

		try {
			await client.vormaNavigate("/solid-parent-state-a");
			await waitForDOMCondition({
				assertion: () => {
					expect(parentMountCount).toBeGreaterThan(0);
					expect(childARenderCount).toBeGreaterThan(0);
					expect(readParentDraftValueForTest?.()).toBe("initial");
				},
			});

			setParentDraftValueForTest?.("typed-value");
			expect(readParentDraftValueForTest?.()).toBe("typed-value");

			await client.vormaNavigate("/solid-parent-state-b");
			await waitForDOMCondition({
				assertion: () => {
					expect(childBRenderCount).toBeGreaterThan(0);
				},
			});
			expect(readParentDraftValueForTest?.()).toBe("typed-value");
			expect(parentMountCount).toBe(1);
			expect(parentCleanupCount).toBe(0);
			expect(childARenderCount).toBeGreaterThan(0);
			expect(childBRenderCount).toBeGreaterThan(0);
		} finally {
			dispose();
			container.remove();
		}
	});

	it("solid root outlet preserves child instance until child module identity changes and ignores location-only updates", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const solidAdapter = await import("vorma/solid");
		let childAMountCount = 0;
		let childACleanupCount = 0;
		let childBMountCount = 0;
		const ChildA = vi.fn(() => {
			childAMountCount += 1;
			onCleanup(() => {
				childACleanupCount += 1;
			});
			return "child-a";
		});
		const ChildB = vi.fn(() => {
			childBMountCount += 1;
			return "child-b";
		});
		const Parent = vi.fn(
			(props: {
				Outlet: (localProps?: Record<string, unknown>) => any;
			}) => props.Outlet({}),
		);
		vi.doMock("/solid-parent.js", () => ({
			default: Parent,
		}));
		vi.doMock("/solid-child-a.js", () => ({
			default: ChildA,
		}));
		vi.doMock("/solid-child-b.js", () => ({
			default: ChildB,
		}));
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/parent", "/parent/child"],
					importURLs: ["/solid-parent.js", "/solid-child-a.js"],
					exportKeys: ["default", "default"],
					errorExportKeys: ["", ""],
					loadersData: [{}, {}],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/parent", "/parent/child"],
					importURLs: ["/solid-parent.js", "/solid-child-b.js"],
					exportKeys: ["default", "default"],
					errorExportKeys: ["", ""],
					loadersData: [{}, {}],
				}),
			);

		const container = document.createElement("div");
		document.body.appendChild(container);
		const dispose = renderSolidWithTrackedDisposer(() => {
			return createComponent(solidAdapter.VormaRootOutlet as any, {
				idx: 0,
			});
		}, container);

		try {
			await client.vormaNavigate("/solid-preserve-child");
			await waitForDOMCondition({
				assertion: () => {
					expect(ChildA).toHaveBeenCalled();
				},
			});
			const childACallsAfterInitial = ChildA.mock.calls.length;

			window.dispatchEvent(
				new CustomEvent("vorma:location", {
					detail: {
						pathname: window.location.pathname,
						search: window.location.search,
						hash: window.location.hash,
						state: window.history.state,
					},
				}),
			);
			await vi.runAllTimersAsync();
			expect(fetchSpy).toHaveBeenCalledTimes(1);
			expect(ChildA.mock.calls.length).toBeGreaterThanOrEqual(
				childACallsAfterInitial,
			);
			expect(childBMountCount).toBe(0);
			expect(childACleanupCount).toBe(0);

			await client.vormaNavigate("/solid-preserve-child-v2");
			await waitForDOMCondition({
				assertion: () => {
					expect(ChildB).toHaveBeenCalled();
				},
			});
			expect(childAMountCount).toBeGreaterThan(0);
			expect(childBMountCount).toBeGreaterThan(0);
			expect(childACleanupCount).toBeGreaterThanOrEqual(1);
		} finally {
			dispose();
			container.remove();
		}
	});

	it("solid root outlet uses the freshest child component when route updates keep the same child key", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const solidAdapter = await import("vorma/solid");
		const ChildA = vi.fn(() => "child-a");
		const ChildB = vi.fn(() => "child-b");
		const Parent = vi.fn(
			(props: {
				Outlet: (localProps?: Record<string, unknown>) => any;
			}) => props.Outlet({}),
		);
		vi.doMock("/solid-fresh-parent.js", () => ({
			default: Parent,
		}));
		vi.doMock("/solid-fresh-child.js", () => ({
			default: ChildA,
		}));
		vi.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/parent", "/parent/child"],
					importURLs: [
						"/solid-fresh-parent.js",
						"/solid-fresh-child.js",
					],
					exportKeys: ["default", "default"],
					errorExportKeys: ["", ""],
					loadersData: [{}, {}],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/parent", "/parent/child"],
					importURLs: [
						"/solid-fresh-parent.js",
						"/solid-fresh-child.js",
					],
					exportKeys: ["default", "default"],
					errorExportKeys: ["", ""],
					loadersData: [{}, {}],
				}),
			);

		const container = document.createElement("div");
		document.body.appendChild(container);
		const dispose = renderSolidWithTrackedDisposer(() => {
			return createComponent(solidAdapter.VormaRootOutlet as any, {
				idx: 0,
			});
		}, container);

		try {
			await client.vormaNavigate("/solid-fresh-child-a");
			await waitForDOMCondition({
				assertion: () => {
					expect(ChildA).toHaveBeenCalled();
				},
			});

			vi.doMock("/solid-fresh-child.js", () => ({
				default: ChildB,
			}));

			await client.vormaNavigate("/solid-fresh-child-b");
			await waitForDOMCondition({
				assertion: () => {
					expect(ChildB).toHaveBeenCalled();
				},
			});
		} finally {
			dispose();
			container.remove();
		}
	});

	it("solid root outlet uses the freshest root component when route updates keep the same root key", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const solidAdapter = await import("vorma/solid");
		const RootA = vi.fn(() => "root-a");
		const RootB = vi.fn(() => "root-b");
		vi.doMock("/solid-fresh-root.js", () => ({
			default: RootA,
		}));
		vi.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/"],
					importURLs: ["/solid-fresh-root.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{}],
				}),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/"],
					importURLs: ["/solid-fresh-root.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					loadersData: [{}],
				}),
			);

		const container = document.createElement("div");
		document.body.appendChild(container);
		const dispose = renderSolidWithTrackedDisposer(() => {
			return createComponent(solidAdapter.VormaRootOutlet as any, {
				idx: 0,
			});
		}, container);

		try {
			await client.vormaNavigate("/solid-fresh-root-a");
			await waitForDOMCondition({
				assertion: () => {
					expect(RootA).toHaveBeenCalled();
				},
			});

			vi.doMock("/solid-fresh-root.js", () => ({
				default: RootB,
			}));

			await client.vormaNavigate("/solid-fresh-root-b");
			await waitForDOMCondition({
				assertion: () => {
					expect(RootB).toHaveBeenCalled();
				},
			});
		} finally {
			dispose();
			container.remove();
		}
	});

	it("loads route modules from backend-resolved importURLs and resolves export-key-selected components", async () => {
		vi.resetModules();
		createIsolatedClientTestRuntime();
		const client = await import("vorma/client");
		await client.initClient({
			vormaAppConfig: DIST_TEST_VORMA_APP_CONFIG,
			publicPathPrefix: "/assets",
		});
		const reactAdapter = await import("vorma/react");

		vi.doMock("/assets/prefixed-route.js", () => ({
			default: () =>
				React.createElement("div", {}, "default-prefixed-component"),
			Named: () =>
				React.createElement("div", {}, "named-prefixed-component"),
		}));

		const container = document.createElement("div");
		document.body.appendChild(container);
		const root = createRoot(container);
		flushSync(() => {
			root.render(
				React.createElement(reactAdapter.VormaRootOutlet as any, {
					idx: 0,
				}),
			);
		});
		try {
			await navigateWithDistRouteDataResponse({
				client,
				href: "/prefixed-route",
				overrides: {
					matchedPatterns: ["/prefixed-route"],
					importURLs: ["/assets/prefixed-route.js"],
					exportKeys: ["Named"],
					errorExportKeys: [""],
					loadersData: [{}],
				},
			});
			await waitForDOMCondition({
				assertion: () => {
					expect(container.textContent).toContain(
						"named-prefixed-component",
					);
				},
			});
			expect(container.textContent).not.toContain(
				"default-prefixed-component",
			);
		} finally {
			root.unmount();
			container.remove();
		}
	});

	it("treats null outermostServerErrorIdx as no active server error boundary", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const reactAdapter = await import("vorma/react");

		vi.doMock("/null-error-idx.js", () => ({
			default: () => React.createElement("div", {}, "null-error-idx-ok"),
			ErrorBoundary: (props: { error: unknown }) =>
				React.createElement(
					"div",
					{},
					`should-not-render:${String(props.error)}`,
				),
		}));

		const container = document.createElement("div");
		document.body.appendChild(container);
		const root = createRoot(container);
		flushSync(() => {
			root.render(
				React.createElement(reactAdapter.VormaRootOutlet as any, {
					idx: 0,
				}),
			);
		});
		try {
			await navigateWithDistRouteDataResponse({
				client,
				href: "/null-error-idx",
				overrides: {
					matchedPatterns: ["/null-error-idx"],
					importURLs: ["/null-error-idx.js"],
					exportKeys: ["default"],
					errorExportKeys: ["ErrorBoundary"],
					loadersData: [{}],
					outermostServerErrorIdx: null,
				},
			});
			await waitForDOMCondition({
				assertion: () => {
					expect(container.textContent).toContain(
						"null-error-idx-ok",
					);
				},
			});
			expect(container.textContent).not.toContain("should-not-render:");
		} finally {
			root.unmount();
			container.remove();
		}
	});

	it("accepts route head blocks that omit booleanAttributes", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");

		await navigateWithDistRouteDataResponse({
			client,
			href: "/head-no-bool-attrs",
			overrides: {
				matchedPatterns: [],
				loadersData: [],
				importURLs: [],
				exportKeys: [],
				errorExportKeys: [],
				metaHeadEls: [
					{
						tag: "meta",
						attributesKnownSafe: {
							name: "description",
							content: "dist-head-no-bool-attrs",
						},
					},
				],
				restHeadEls: [
					{
						tag: "link",
						attributesKnownSafe: {
							rel: "canonical",
							href: "/dist-head-no-bool-attrs",
						},
					},
				],
			},
		});

		expect(
			document.head.querySelector('meta[name="description"]'),
		).not.toBeNull();
		expect(
			document.head.querySelector('link[rel="canonical"]'),
		).not.toBeNull();
	});

	it("preserves active error boundary rendering across hash-only navigations", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const reactAdapter = await import("vorma/react");

		vi.doMock("/hash-error.js", () => ({
			default: () => React.createElement("div", {}, "should-not-render"),
			ErrorBoundary: (props: { error: unknown }) =>
				React.createElement(
					"div",
					{},
					`hash-handled:${String(props.error)}`,
				),
		}));

		const container = document.createElement("div");
		document.body.appendChild(container);
		const root = createRoot(container);
		flushSync(() => {
			root.render(
				React.createElement(reactAdapter.VormaRootOutlet as any, {
					idx: 0,
				}),
			);
		});
		try {
			await navigateWithDistRouteDataResponse({
				client,
				href: "/hash-error",
				overrides: {
					matchedPatterns: ["/hash-error"],
					importURLs: ["/hash-error.js"],
					exportKeys: ["default"],
					errorExportKeys: ["ErrorBoundary"],
					loadersData: [{}],
					outermostServerError: "boom",
					outermostServerErrorIdx: 0,
				},
			});
			await waitForDOMCondition({
				assertion: () => {
					expect(container.textContent).toContain(
						"hash-handled:boom",
					);
				},
			});

			vi.spyOn(window, "fetch").mockResolvedValue(
				createRouteDataResponse({
					matchedPatterns: ["/hash-error"],
					importURLs: ["/hash-error.js"],
					exportKeys: ["default"],
					errorExportKeys: ["ErrorBoundary"],
					loadersData: [{}],
					outermostServerError: "boom",
					outermostServerErrorIdx: 0,
				}),
			);
			await client.vormaNavigate("/hash-error#next");
			await vi.runAllTimersAsync();

			expect(container.textContent).toContain("hash-handled:boom");
			expect(container.textContent).not.toContain("should-not-render");
		} finally {
			root.unmount();
			container.remove();
		}
	});

	it("react typed links run intent prefetch after delay without committing navigation", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const reactAdapter = await import("vorma/react");
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				matchedPatterns: ["/prefetch/:id"],
				loadersData: [{ value: "prefetched" }],
			}),
		);

		const TypedLink = reactAdapter.makeTypedLink(
			DIST_TEST_VORMA_APP_CONFIG,
			{
				prefetch: "intent",
				prefetchDelayMs: 50,
			},
		);
		const { anchor, cleanup } = renderReactTypedLinkForTesting({
			TypedLink,
			linkProps: {
				pattern: "/prefetch/:id",
				params: { id: "42" },
				children: "React Prefetch Link",
			},
		});
		try {
			anchor.dispatchEvent(new FocusEvent("focusin", { bubbles: true }));
			await vi.advanceTimersByTimeAsync(49);
			expect(fetchSpy).toHaveBeenCalledTimes(0);

			await vi.advanceTimersByTimeAsync(1);
			await vi.runAllTimersAsync();

			expect(fetchSpy).toHaveBeenCalledTimes(1);
			expect(window.location.pathname).toBe("/");
			expect(client.getStatus()).toEqual({
				isNavigating: false,
				isSubmitting: false,
				isRevalidating: false,
			});
		} finally {
			cleanup();
		}
	});

	it("react links create prefetch behavior only for eligible internal http targets", async () => {
		await initializeDistRuntimeStateForAdapters();
		const reactAdapter = await import("vorma/react");
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				matchedPatterns: ["/internal-prefetch"],
				loadersData: [{ value: "prefetched" }],
			}),
		);

		const external = renderReactTypedLinkForTesting({
			TypedLink: reactAdapter.VormaLink,
			linkProps: {
				href: "https://external.example/path",
				prefetch: "intent",
				prefetchDelayMs: 0,
				children: "External",
			},
		});
		try {
			external.anchor.dispatchEvent(
				new FocusEvent("focusin", { bubbles: true }),
			);
			await vi.runAllTimersAsync();
			expect(fetchSpy).toHaveBeenCalledTimes(0);
		} finally {
			external.cleanup();
		}

		const mailto = renderReactTypedLinkForTesting({
			TypedLink: reactAdapter.VormaLink,
			linkProps: {
				href: "mailto:test@example.com",
				prefetch: "intent",
				prefetchDelayMs: 0,
				children: "Mailto",
			},
		});
		try {
			mailto.anchor.dispatchEvent(
				new FocusEvent("focusin", { bubbles: true }),
			);
			await vi.runAllTimersAsync();
			expect(fetchSpy).toHaveBeenCalledTimes(0);
		} finally {
			mailto.cleanup();
		}

		const internal = renderReactTypedLinkForTesting({
			TypedLink: reactAdapter.VormaLink,
			linkProps: {
				href: "/internal-prefetch",
				prefetch: "intent",
				prefetchDelayMs: 0,
				children: "Internal",
			},
		});
		try {
			internal.anchor.dispatchEvent(
				new FocusEvent("focusin", { bubbles: true }),
			);
			await vi.runAllTimersAsync();
			expect(fetchSpy).toHaveBeenCalledTimes(1);
		} finally {
			internal.cleanup();
		}
	});

	it("react links do not prefetch when target href is the current page", async () => {
		await initializeDistRuntimeStateForAdapters();
		const reactAdapter = await import("vorma/react");
		window.history.replaceState({}, "", "/current-page");
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				loadersData: [],
			}),
		);
		const { anchor, cleanup } = renderReactTypedLinkForTesting({
			TypedLink: reactAdapter.VormaLink,
			linkProps: {
				href: "/current-page",
				prefetch: "intent",
				prefetchDelayMs: 0,
				children: "Current Page",
			},
		});
		try {
			anchor.dispatchEvent(new FocusEvent("focusin", { bubbles: true }));
			await vi.runAllTimersAsync();
			expect(fetchSpy).toHaveBeenCalledTimes(0);
		} finally {
			cleanup();
		}
	});

	it("react links retry prefetch after a current-page no-op once location changes", async () => {
		await initializeDistRuntimeStateForAdapters();
		const reactAdapter = await import("vorma/react");
		window.history.replaceState({}, "", "/current-page");
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				loadersData: [],
			}),
		);
		const { anchor, cleanup } = renderReactTypedLinkForTesting({
			TypedLink: reactAdapter.VormaLink,
			linkProps: {
				href: "/current-page",
				prefetch: "intent",
				prefetchDelayMs: 50,
				children: "Retry Prefetch",
			},
		});
		try {
			anchor.dispatchEvent(new FocusEvent("focusin", { bubbles: true }));
			await vi.advanceTimersByTimeAsync(50);
			expect(fetchSpy).toHaveBeenCalledTimes(0);

			window.history.replaceState({}, "", "/other-page");
			anchor.dispatchEvent(new FocusEvent("focusin", { bubbles: true }));
			await vi.advanceTimersByTimeAsync(50);
			await vi.runAllTimersAsync();
			expect(fetchSpy).toHaveBeenCalledTimes(1);
		} finally {
			cleanup();
		}
	});

	it("deduplicates same-data prefetches when only hash differs", async () => {
		await initializeDistRuntimeStateForAdapters();
		const reactAdapter = await import("vorma/react");
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				loadersData: [],
			}),
		);
		const first = renderReactTypedLinkForTesting({
			TypedLink: reactAdapter.VormaLink,
			linkProps: {
				href: "/prefetch-dedupe#first",
				prefetch: "intent",
				prefetchDelayMs: 0,
				children: "First Hash",
			},
		});
		const second = renderReactTypedLinkForTesting({
			TypedLink: reactAdapter.VormaLink,
			linkProps: {
				href: "/prefetch-dedupe#second",
				prefetch: "intent",
				prefetchDelayMs: 0,
				children: "Second Hash",
			},
		});
		try {
			first.anchor.dispatchEvent(
				new FocusEvent("focusin", { bubbles: true }),
			);
			await vi.runAllTimersAsync();
			second.anchor.dispatchEvent(
				new FocusEvent("focusin", { bubbles: true }),
			);
			await vi.runAllTimersAsync();
			expect(fetchSpy).toHaveBeenCalledTimes(1);
		} finally {
			first.cleanup();
			second.cleanup();
		}
	});

	it("deduplicates identical prefetches for the same href", async () => {
		await initializeDistRuntimeStateForAdapters();
		const reactAdapter = await import("vorma/react");
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				loadersData: [],
			}),
		);
		const first = renderReactTypedLinkForTesting({
			TypedLink: reactAdapter.VormaLink,
			linkProps: {
				href: "/prefetch-identical",
				prefetch: "intent",
				prefetchDelayMs: 0,
				children: "First Href",
			},
		});
		const second = renderReactTypedLinkForTesting({
			TypedLink: reactAdapter.VormaLink,
			linkProps: {
				href: "/prefetch-identical",
				prefetch: "intent",
				prefetchDelayMs: 0,
				children: "Second Href",
			},
		});
		try {
			first.anchor.dispatchEvent(
				new FocusEvent("focusin", { bubbles: true }),
			);
			await vi.runAllTimersAsync();
			second.anchor.dispatchEvent(
				new FocusEvent("focusin", { bubbles: true }),
			);
			await vi.runAllTimersAsync();
			expect(fetchSpy).toHaveBeenCalledTimes(1);
		} finally {
			first.cleanup();
			second.cleanup();
		}
	});

	it("aborts a hash-deduped shared prefetch when stop is called from either handler", async () => {
		await initializeDistRuntimeStateForAdapters();
		const reactAdapter = await import("vorma/react");
		const signals: AbortSignal[] = [];
		const neverSettled = new Promise<Response>(() => {});
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockImplementation((_input, init) => {
				const signal = (init as RequestInit | undefined)?.signal as
					| AbortSignal
					| undefined;
				if (signal) {
					signals.push(signal);
				}
				return neverSettled;
			});
		const first = renderReactTypedLinkForTesting({
			TypedLink: reactAdapter.VormaLink,
			linkProps: {
				href: "/prefetch-stop-alias#first",
				prefetch: "intent",
				prefetchDelayMs: 0,
				children: "First Stop",
			},
		});
		const second = renderReactTypedLinkForTesting({
			TypedLink: reactAdapter.VormaLink,
			linkProps: {
				href: "/prefetch-stop-alias#second",
				prefetch: "intent",
				prefetchDelayMs: 0,
				children: "Second Stop",
			},
		});
		try {
			first.anchor.dispatchEvent(
				new FocusEvent("focusin", { bubbles: true }),
			);
			await vi.runAllTimersAsync();
			second.anchor.dispatchEvent(
				new FocusEvent("focusin", { bubbles: true }),
			);
			await vi.runAllTimersAsync();
			expect(fetchSpy).toHaveBeenCalledTimes(1);
			expect(signals).toHaveLength(1);
			expect(signals[0]?.aborted).toBe(false);

			second.anchor.dispatchEvent(
				new FocusEvent("focusout", { bubbles: true }),
			);
			expect(signals[0]?.aborted).toBe(true);
		} finally {
			first.cleanup();
			second.cleanup();
		}
	});

	it("aborts idle prefetch entries via same-data-target alias lookup", async () => {
		await initializeDistRuntimeStateForAdapters();
		const reactAdapter = await import("vorma/react");
		const signals: AbortSignal[] = [];
		const neverSettled = new Promise<Response>(() => {});
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockImplementation((_input, init) => {
				const signal = (init as RequestInit | undefined)?.signal as
					| AbortSignal
					| undefined;
				if (signal) {
					signals.push(signal);
				}
				return neverSettled;
			});
		const first = renderReactTypedLinkForTesting({
			TypedLink: reactAdapter.VormaLink,
			linkProps: {
				href: "/prefetch-idle-alias#first",
				prefetch: "intent",
				prefetchDelayMs: 0,
				children: "First Alias",
			},
		});
		const alias = renderReactTypedLinkForTesting({
			TypedLink: reactAdapter.VormaLink,
			linkProps: {
				href: "/prefetch-idle-alias#second",
				prefetch: "intent",
				prefetchDelayMs: 0,
				children: "Alias Stop",
			},
		});
		try {
			first.anchor.dispatchEvent(
				new FocusEvent("focusin", { bubbles: true }),
			);
			await vi.runAllTimersAsync();
			expect(fetchSpy).toHaveBeenCalledTimes(1);
			expect(signals).toHaveLength(1);
			expect(signals[0]?.aborted).toBe(false);

			alias.anchor.dispatchEvent(
				new FocusEvent("focusout", { bubbles: true }),
			);
			expect(signals[0]?.aborted).toBe(true);
		} finally {
			first.cleanup();
			alias.cleanup();
		}
	});

	it("cancels pending prefetch timer on hash-only click", async () => {
		await initializeDistRuntimeStateForAdapters();
		const reactAdapter = await import("vorma/react");
		window.history.replaceState({}, "", "/hash-page");
		const beforeBegin = vi.fn();
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				loadersData: [],
			}),
		);
		const { anchor, cleanup } = renderReactTypedLinkForTesting({
			TypedLink: reactAdapter.VormaLink,
			linkProps: {
				href: "/hash-page#section-a",
				prefetch: "intent",
				prefetchDelayMs: 200,
				beforeBegin,
				children: "Hash Click",
			},
		});
		try {
			anchor.dispatchEvent(new FocusEvent("focusin", { bubbles: true }));
			await vi.advanceTimersByTimeAsync(100);
			expect(beforeBegin).not.toHaveBeenCalled();

			anchor.dispatchEvent(
				new MouseEvent("click", {
					bubbles: true,
					cancelable: true,
					button: 0,
				}),
			);
			await vi.advanceTimersByTimeAsync(200);

			expect(beforeBegin).not.toHaveBeenCalled();
			expect(fetchSpy).not.toHaveBeenCalled();
		} finally {
			cleanup();
		}
	});

	it("cancels pending prefetch timer on same-document hash removal click", async () => {
		await initializeDistRuntimeStateForAdapters();
		const reactAdapter = await import("vorma/react");
		window.history.replaceState({}, "", "/hash-page#section-a");
		const beforeBegin = vi.fn();
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				loadersData: [],
			}),
		);
		const { anchor, cleanup } = renderReactTypedLinkForTesting({
			TypedLink: reactAdapter.VormaLink,
			linkProps: {
				href: "/hash-page",
				prefetch: "intent",
				prefetchDelayMs: 200,
				beforeBegin,
				children: "Hash Removal",
			},
		});
		try {
			anchor.dispatchEvent(new FocusEvent("focusin", { bubbles: true }));
			await vi.advanceTimersByTimeAsync(100);
			expect(beforeBegin).not.toHaveBeenCalled();

			anchor.dispatchEvent(
				new MouseEvent("click", {
					bubbles: true,
					cancelable: true,
					button: 0,
				}),
			);
			await vi.advanceTimersByTimeAsync(200);

			expect(beforeBegin).not.toHaveBeenCalled();
			expect(fetchSpy).not.toHaveBeenCalled();
		} finally {
			cleanup();
		}
	});

	it("cancels pending prefetch timer on same-document no-op hash click", async () => {
		await initializeDistRuntimeStateForAdapters();
		const reactAdapter = await import("vorma/react");
		window.history.replaceState({}, "", "/hash-page#section-a");
		const beforeBegin = vi.fn();
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				loadersData: [],
			}),
		);
		const { anchor, cleanup } = renderReactTypedLinkForTesting({
			TypedLink: reactAdapter.VormaLink,
			linkProps: {
				href: "/hash-page#section-a",
				prefetch: "intent",
				prefetchDelayMs: 200,
				beforeBegin,
				children: "Hash No-op",
			},
		});
		try {
			anchor.dispatchEvent(new FocusEvent("focusin", { bubbles: true }));
			await vi.advanceTimersByTimeAsync(100);
			expect(beforeBegin).not.toHaveBeenCalled();

			const clickEvent = new MouseEvent("click", {
				bubbles: true,
				cancelable: true,
				button: 0,
			});
			anchor.dispatchEvent(clickEvent);
			await vi.advanceTimersByTimeAsync(200);

			expect(clickEvent.defaultPrevented).toBe(true);
			expect(beforeBegin).not.toHaveBeenCalled();
			expect(fetchSpy).not.toHaveBeenCalled();
		} finally {
			cleanup();
		}
	});

	it("prevents default on same-document no-op prefetch clicks without hash", async () => {
		await initializeDistRuntimeStateForAdapters();
		const reactAdapter = await import("vorma/react");
		window.history.replaceState({}, "", "/current-page");
		const beforeBegin = vi.fn();
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				loadersData: [],
			}),
		);
		const { anchor, cleanup } = renderReactTypedLinkForTesting({
			TypedLink: reactAdapter.VormaLink,
			linkProps: {
				href: "/current-page",
				prefetch: "intent",
				prefetchDelayMs: 200,
				beforeBegin,
				children: "Current No-op",
			},
		});
		try {
			anchor.dispatchEvent(new FocusEvent("focusin", { bubbles: true }));
			await vi.advanceTimersByTimeAsync(100);
			expect(beforeBegin).not.toHaveBeenCalled();

			const clickEvent = new MouseEvent("click", {
				bubbles: true,
				cancelable: true,
				button: 0,
			});
			anchor.dispatchEvent(clickEvent);
			await vi.advanceTimersByTimeAsync(200);

			expect(clickEvent.defaultPrevented).toBe(true);
			expect(beforeBegin).not.toHaveBeenCalled();
			expect(fetchSpy).not.toHaveBeenCalled();
		} finally {
			cleanup();
		}
	});

	it("cancels pending prefetch timer on encoding-equivalent hash click", async () => {
		await initializeDistRuntimeStateForAdapters();
		const reactAdapter = await import("vorma/react");
		window.history.replaceState({}, "", "/hash-page#~");
		const beforeBegin = vi.fn();
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				loadersData: [],
			}),
		);
		const { anchor, cleanup } = renderReactTypedLinkForTesting({
			TypedLink: reactAdapter.VormaLink,
			linkProps: {
				href: "/hash-page#%7E",
				prefetch: "intent",
				prefetchDelayMs: 200,
				beforeBegin,
				children: "Hash Equivalent",
			},
		});
		try {
			anchor.dispatchEvent(new FocusEvent("focusin", { bubbles: true }));
			await vi.advanceTimersByTimeAsync(100);
			expect(beforeBegin).not.toHaveBeenCalled();

			anchor.dispatchEvent(
				new MouseEvent("click", {
					bubbles: true,
					cancelable: true,
					button: 0,
				}),
			);
			await vi.advanceTimersByTimeAsync(200);

			expect(beforeBegin).not.toHaveBeenCalled();
			expect(fetchSpy).not.toHaveBeenCalled();
		} finally {
			cleanup();
		}
	});

	it("runs beforeBegin callback before prefetch starts", async () => {
		await initializeDistRuntimeStateForAdapters();
		const reactAdapter = await import("vorma/react");
		const beforeBegin = vi.fn();
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				loadersData: [],
			}),
		);
		const { anchor, cleanup } = renderReactTypedLinkForTesting({
			TypedLink: reactAdapter.VormaLink,
			linkProps: {
				href: "/before-begin-prefetch",
				prefetch: "intent",
				prefetchDelayMs: 0,
				beforeBegin,
				children: "Before Begin",
			},
		});
		try {
			anchor.dispatchEvent(new FocusEvent("focusin", { bubbles: true }));
			await vi.runAllTimersAsync();

			expect(beforeBegin).toHaveBeenCalledTimes(1);
			expect(fetchSpy).toHaveBeenCalledTimes(1);
			expect(beforeBegin.mock.invocationCallOrder[0]).toBeLessThan(
				fetchSpy.mock.invocationCallOrder[0] ?? Number.MAX_SAFE_INTEGER,
			);
		} finally {
			cleanup();
		}
	});

	it("recovers from beforeBegin prefetch callback failures and allows retry", async () => {
		await initializeDistRuntimeStateForAdapters();
		const reactAdapter = await import("vorma/react");
		const beforeBegin = vi
			.fn()
			.mockImplementationOnce(() => {
				throw new Error("beforeBegin failed");
			})
			.mockImplementation(() => {});
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				loadersData: [],
			}),
		);
		const { anchor, cleanup } = renderReactTypedLinkForTesting({
			TypedLink: reactAdapter.VormaLink,
			linkProps: {
				href: "/before-begin-retry",
				prefetch: "intent",
				prefetchDelayMs: 0,
				beforeBegin,
				children: "Before Begin Retry",
			},
		});
		try {
			const { unhandledRejections } = await withUnhandledRejectionCapture(
				{
					run: async () => {
						anchor.dispatchEvent(
							new FocusEvent("focusin", { bubbles: true }),
						);
						await vi.runAllTimersAsync();
						anchor.dispatchEvent(
							new FocusEvent("focusin", { bubbles: true }),
						);
						await vi.runAllTimersAsync();
					},
				},
			);

			expect(unhandledRejections).toEqual([]);
			expect(beforeBegin).toHaveBeenCalledTimes(2);
			expect(fetchSpy).toHaveBeenCalledTimes(1);
		} finally {
			cleanup();
		}
	});

	it("reuses completed prefetch response on click without refetching", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const reactAdapter = await import("vorma/react");
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				loadersData: [],
				title: { dangerousInnerHTML: "Prefetch Cache Reuse" },
			}),
		);
		const { anchor, cleanup } = renderReactTypedLinkForTesting({
			TypedLink: reactAdapter.VormaLink,
			linkProps: {
				href: "/prefetch-cache-reuse",
				prefetch: "intent",
				prefetchDelayMs: 0,
				children: "Cache Reuse",
			},
		});
		try {
			anchor.dispatchEvent(new FocusEvent("focusin", { bubbles: true }));
			await vi.runAllTimersAsync();
			expect(fetchSpy).toHaveBeenCalledTimes(1);

			anchor.dispatchEvent(
				new MouseEvent("click", {
					bubbles: true,
					cancelable: true,
					button: 0,
				}),
			);
			await vi.runAllTimersAsync();

			expect(fetchSpy).toHaveBeenCalledTimes(1);
			expect(window.location.pathname).toBe("/prefetch-cache-reuse");
			expect(document.title).toBe("Prefetch Cache Reuse");
			expect(client.getStatus()).toEqual({
				isNavigating: false,
				isSubmitting: false,
				isRevalidating: false,
			});
		} finally {
			cleanup();
		}
	});

	it("warms internal prefetch artifacts without committing page mutations", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const reactAdapter = await import("vorma/react");
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				loadersData: [],
				title: { dangerousInnerHTML: "Prefetch Should Not Commit Yet" },
			}),
		);
		const { anchor, cleanup } = renderReactTypedLinkForTesting({
			TypedLink: reactAdapter.VormaLink,
			linkProps: {
				href: "/prefetch-warm-only",
				prefetch: "intent",
				prefetchDelayMs: 0,
				children: "Prefetch Warm",
			},
		});
		try {
			const initialTitle = document.title;
			const initialPathname = window.location.pathname;
			anchor.dispatchEvent(new FocusEvent("focusin", { bubbles: true }));
			await vi.runAllTimersAsync();

			expect(fetchSpy).toHaveBeenCalledTimes(1);
			expect(window.location.pathname).toBe(initialPathname);
			expect(document.title).toBe(initialTitle);
			expect(client.getStatus()).toEqual({
				isNavigating: false,
				isSubmitting: false,
				isRevalidating: false,
			});
		} finally {
			cleanup();
		}
	});

	it("applies prefetched css only after navigation commit", async () => {
		await initializeDistRuntimeStateForAdapters();
		const reactAdapter = await import("vorma/react");
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				loadersData: [],
				cssBundles: ["/prefetch-commit.css"],
				title: { dangerousInnerHTML: "Prefetch Commit Boundary" },
			}),
		);
		const appendChild = document.head.appendChild.bind(document.head);
		vi.spyOn(document.head, "appendChild").mockImplementation((node) => {
			if (
				node instanceof HTMLLinkElement &&
				node.rel === "preload" &&
				node.getAttribute("as") === "style"
			) {
				Promise.resolve().then(() =>
					node.dispatchEvent(new Event("load")),
				);
			}
			return appendChild(node);
		});
		const { anchor, cleanup } = renderReactTypedLinkForTesting({
			TypedLink: reactAdapter.VormaLink,
			linkProps: {
				href: "/prefetch-commit",
				prefetch: "intent",
				prefetchDelayMs: 0,
				children: "Prefetch Commit",
			},
		});
		try {
			anchor.dispatchEvent(new FocusEvent("focusin", { bubbles: true }));
			await vi.runAllTimersAsync();
			expect(
				document.head.querySelector(
					'link[rel="stylesheet"][data-vorma-css-bundle="/prefetch-commit.css"]',
				),
			).toBeNull();

			fetchSpy.mockClear();
			anchor.dispatchEvent(
				new MouseEvent("click", {
					bubbles: true,
					cancelable: true,
					button: 0,
				}),
			);
			await vi.runAllTimersAsync();

			expect(fetchSpy).not.toHaveBeenCalled();
			expect(
				document.querySelectorAll(
					'link[rel="stylesheet"][data-vorma-css-bundle="/prefetch-commit.css"]',
				),
			).toHaveLength(1);
			expect(document.title).toBe("Prefetch Commit Boundary");
			expect(window.location.pathname).toBe("/prefetch-commit");
		} finally {
			cleanup();
		}
	});

	it("does not prime prefetch module/css artifacts when response build id differs", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const reactAdapter = await import("vorma/react");
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse(
					{
						matchedPatterns: ["/prefetch-mismatch/:id"],
						importURLs: [],
						exportKeys: [],
						errorExportKeys: [],
						loadersData: [],
						cssBundles: ["/prefetch-mismatch.css"],
					},
					{
						headers: {
							"X-Wave-Framework-Build-Id": "build-2",
						},
					},
				),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/prefetch-mismatch/:id"],
					importURLs: [],
					exportKeys: [],
					errorExportKeys: [],
					loadersData: [],
					cssBundles: ["/prefetch-mismatch.css"],
				}),
			);
		const appendChild = document.head.appendChild.bind(document.head);
		vi.spyOn(document.head, "appendChild").mockImplementation((node) => {
			if (
				node instanceof HTMLLinkElement &&
				node.rel === "preload" &&
				node.getAttribute("as") === "style"
			) {
				Promise.resolve().then(() =>
					node.dispatchEvent(new Event("load")),
				);
			}
			return appendChild(node);
		});
		const { anchor, cleanup } = renderReactTypedLinkForTesting({
			TypedLink: reactAdapter.VormaLink,
			linkProps: {
				href: "/prefetch-mismatch/42",
				prefetch: "intent",
				prefetchDelayMs: 0,
				children: "Prefetch Mismatch",
			},
		});
		try {
			anchor.dispatchEvent(new FocusEvent("focusin", { bubbles: true }));
			await vi.runAllTimersAsync();
			expect(fetchSpy).toHaveBeenCalledTimes(1);
			expect(
				document.head.querySelector(
					'link[rel="stylesheet"][data-vorma-css-bundle="/prefetch-mismatch.css"]',
				),
			).toBeNull();
			expect(client.getBuildID()).toBe("build-2");
			expect(client.getStatus()).toEqual({
				isNavigating: false,
				isSubmitting: false,
				isRevalidating: false,
			});
		} finally {
			cleanup();
		}
	});

	it("cancels pending prefetch timeout when stop is called", async () => {
		await initializeDistRuntimeStateForAdapters();
		const reactAdapter = await import("vorma/react");
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				loadersData: [],
			}),
		);
		const { anchor, cleanup } = renderReactTypedLinkForTesting({
			TypedLink: reactAdapter.VormaLink,
			linkProps: {
				href: "/cancel-timeout",
				prefetch: "intent",
				children: "Cancel Timeout",
			},
		});
		try {
			anchor.dispatchEvent(new FocusEvent("focusin", { bubbles: true }));
			anchor.dispatchEvent(new FocusEvent("focusout", { bubbles: true }));
			await vi.advanceTimersByTimeAsync(200);
			expect(fetchSpy).not.toHaveBeenCalled();
		} finally {
			cleanup();
		}
	});

	it("does not leak orphan timers when start is called multiple times before stop", async () => {
		await initializeDistRuntimeStateForAdapters();
		const reactAdapter = await import("vorma/react");
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				loadersData: [],
			}),
		);
		const { anchor, cleanup } = renderReactTypedLinkForTesting({
			TypedLink: reactAdapter.VormaLink,
			linkProps: {
				href: "/orphan-timer",
				prefetch: "intent",
				prefetchDelayMs: 200,
				children: "Orphan Timer",
			},
		});
		try {
			anchor.dispatchEvent(new FocusEvent("focusin", { bubbles: true }));
			anchor.dispatchEvent(new FocusEvent("focusin", { bubbles: true }));
			anchor.dispatchEvent(new FocusEvent("focusout", { bubbles: true }));
			await vi.advanceTimersByTimeAsync(250);
			expect(fetchSpy).not.toHaveBeenCalled();
		} finally {
			cleanup();
		}
	});

	it("clears pending prefetch timer even when timer id is zero", async () => {
		await initializeDistRuntimeStateForAdapters();
		const reactAdapter = await import("vorma/react");
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				loadersData: [],
			}),
		);
		const setTimeoutSpy = vi
			.spyOn(window, "setTimeout")
			.mockImplementation(() => 0 as any);
		const clearTimeoutSpy = vi.spyOn(window, "clearTimeout");
		const { anchor, cleanup } = renderReactTypedLinkForTesting({
			TypedLink: reactAdapter.VormaLink,
			linkProps: {
				href: "/zero-timer-stop",
				prefetch: "intent",
				children: "Zero Timer",
			},
		});
		try {
			anchor.dispatchEvent(new FocusEvent("focusin", { bubbles: true }));
			anchor.dispatchEvent(new FocusEvent("focusout", { bubbles: true }));
			expect(clearTimeoutSpy).toHaveBeenCalledWith(0);
			await vi.advanceTimersByTimeAsync(200);
			expect(fetchSpy).not.toHaveBeenCalled();
		} finally {
			cleanup();
			setTimeoutSpy.mockRestore();
			clearTimeoutSpy.mockRestore();
		}
	});

	it("aborts in-flight prefetch when stop is called", async () => {
		await initializeDistRuntimeStateForAdapters();
		const reactAdapter = await import("vorma/react");
		const signals: AbortSignal[] = [];
		const neverSettled = new Promise<Response>(() => {});
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockImplementation((_input, init) => {
				const signal = (init as RequestInit | undefined)?.signal as
					| AbortSignal
					| undefined;
				if (signal) {
					signals.push(signal);
				}
				return neverSettled;
			});
		const { anchor, cleanup } = renderReactTypedLinkForTesting({
			TypedLink: reactAdapter.VormaLink,
			linkProps: {
				href: "/abort-in-flight-prefetch",
				prefetch: "intent",
				prefetchDelayMs: 0,
				children: "Abort In Flight",
			},
		});
		try {
			anchor.dispatchEvent(new FocusEvent("focusin", { bubbles: true }));
			await vi.runAllTimersAsync();
			expect(fetchSpy).toHaveBeenCalledTimes(1);
			expect(signals).toHaveLength(1);
			expect(signals[0]?.aborted).toBe(false);

			anchor.dispatchEvent(new FocusEvent("focusout", { bubbles: true }));
			expect(signals[0]?.aborted).toBe(true);
		} finally {
			cleanup();
		}
	});

	it("does not abort upgraded navigation when prefetch handlers stop", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const reactAdapter = await import("vorma/react");
		const deferred = createDeferred<Response>();
		let signal: AbortSignal | undefined;
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockImplementation((_input, init) => {
				signal = (init as RequestInit | undefined)?.signal as
					| AbortSignal
					| undefined;
				return deferred.promise;
			});
		const { anchor, cleanup } = renderReactTypedLinkForTesting({
			TypedLink: reactAdapter.VormaLink,
			linkProps: {
				href: "/abort-test",
				prefetch: "intent",
				children: "Abort Test",
			},
		});
		try {
			anchor.dispatchEvent(new FocusEvent("focusin", { bubbles: true }));
			await vi.advanceTimersByTimeAsync(100);
			expect(fetchSpy).toHaveBeenCalledTimes(1);
			expect(signal?.aborted).toBe(false);

			const navigationPromise = client.vormaNavigate("/abort-test");
			anchor.dispatchEvent(new FocusEvent("focusout", { bubbles: true }));
			expect(signal?.aborted).toBe(false);
			expect(client.getStatus().isNavigating).toBe(true);
			expect(fetchSpy).toHaveBeenCalledTimes(1);

			deferred.resolve(
				createRouteDataResponse({
					loadersData: [],
				}),
			);
			await navigationPromise;
			await vi.runAllTimersAsync();
		} finally {
			cleanup();
		}
	});

	it("upgrades same-data prefetch to navigation even when only hash differs", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const reactAdapter = await import("vorma/react");
		const deferred = createDeferred<Response>();
		const fetchSpy = vi.spyOn(window, "fetch").mockImplementation(() => {
			return deferred.promise;
		});
		const { anchor, cleanup } = renderReactTypedLinkForTesting({
			TypedLink: reactAdapter.VormaLink,
			linkProps: {
				href: "/prefetch-hash-upgrade#prefetch",
				prefetch: "intent",
				children: "Hash Upgrade",
			},
		});
		try {
			anchor.dispatchEvent(new FocusEvent("focusin", { bubbles: true }));
			await vi.advanceTimersByTimeAsync(100);
			expect(fetchSpy).toHaveBeenCalledTimes(1);
			expect(client.getStatus().isNavigating).toBe(false);

			const navigationPromise = client.vormaNavigate(
				"/prefetch-hash-upgrade#final",
			);
			await vi.advanceTimersByTimeAsync(8);
			expect(fetchSpy).toHaveBeenCalledTimes(1);
			expect(client.getStatus().isNavigating).toBe(true);

			deferred.resolve(
				createRouteDataResponse({
					loadersData: [],
					title: { dangerousInnerHTML: "Prefetch Hash Upgrade" },
				}),
			);
			await navigationPromise;
			await vi.runAllTimersAsync();

			expect(window.location.pathname).toBe("/prefetch-hash-upgrade");
			expect(window.location.hash).toBe("#final");
			expect(document.title).toBe("Prefetch Hash Upgrade");
		} finally {
			cleanup();
		}
	});

	it("does not leak unhandled rejections when pure prefetch resolves to redirect", async () => {
		await initializeDistRuntimeStateForAdapters();
		const reactAdapter = await import("vorma/react");
		const addClientLoader = reactAdapter.makeTypedAddClientLoader(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const serverDataPromiseErrors: Array<unknown> = [];
		addClientLoader({
			pattern: "/prefetch-redirect",
			clientLoader: async ({ serverDataPromise }) => {
				try {
					await serverDataPromise;
				} catch (error) {
					serverDataPromiseErrors.push(error);
					throw error;
				}
				return null;
			},
		});
		vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse(
				{
					loadersData: [],
				},
				{
					headers: {
						"X-Client-Redirect": "/redirect-target",
					},
				},
			),
		);
		const { anchor, cleanup } = renderReactTypedLinkForTesting({
			TypedLink: reactAdapter.VormaLink,
			linkProps: {
				href: "/prefetch-redirect",
				prefetch: "intent",
				prefetchDelayMs: 0,
				children: "Prefetch Redirect",
			},
		});
		try {
			const { unhandledRejections } = await withUnhandledRejectionCapture(
				{
					run: async () => {
						anchor.dispatchEvent(
							new FocusEvent("focusin", { bubbles: true }),
						);
						await vi.runAllTimersAsync();
					},
				},
			);

			expect(unhandledRejections).toEqual([]);
			expect(serverDataPromiseErrors).toHaveLength(1);
			expect(serverDataPromiseErrors[0]).toBeInstanceOf(Error);
			expect((serverDataPromiseErrors[0] as Error).name).toBe(
				"AbortError",
			);
		} finally {
			cleanup();
		}
	});

	it("rejects serverDataPromise with AbortError for failed prefetch responses", async () => {
		await initializeDistRuntimeStateForAdapters();
		const reactAdapter = await import("vorma/react");
		const addClientLoader = reactAdapter.makeTypedAddClientLoader(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const serverDataPromiseErrors: Array<unknown> = [];
		addClientLoader({
			pattern: "/prefetch-failed-response",
			clientLoader: async ({ serverDataPromise }) => {
				try {
					await serverDataPromise;
				} catch (error) {
					serverDataPromiseErrors.push(error);
					throw error;
				}
				return null;
			},
		});
		vi.spyOn(window, "fetch").mockResolvedValue(
			new Response("Server error", { status: 500 }),
		);
		const { anchor, cleanup } = renderReactTypedLinkForTesting({
			TypedLink: reactAdapter.VormaLink,
			linkProps: {
				href: "/prefetch-failed-response",
				prefetch: "intent",
				prefetchDelayMs: 0,
				children: "Prefetch Failed",
			},
		});
		try {
			const { unhandledRejections } = await withUnhandledRejectionCapture(
				{
					run: async () => {
						anchor.dispatchEvent(
							new FocusEvent("focusin", { bubbles: true }),
						);
						await vi.runAllTimersAsync();
					},
				},
			);

			expect(unhandledRejections).toEqual([]);
			expect(serverDataPromiseErrors).toHaveLength(1);
			expect(serverDataPromiseErrors[0]).toBeInstanceOf(Error);
			expect((serverDataPromiseErrors[0] as Error).name).toBe(
				"AbortError",
			);
		} finally {
			cleanup();
		}
	});

	it("supports click while prefetch is in-flight and runs render callbacks", async () => {
		await initializeDistRuntimeStateForAdapters();
		const reactAdapter = await import("vorma/react");
		const deferred = createDeferred<Response>();
		vi.spyOn(window, "fetch").mockImplementation(() => deferred.promise);
		const beforeBegin = vi.fn();
		const beforeRender = vi.fn();
		const afterRender = vi.fn();
		const { anchor, cleanup } = renderReactTypedLinkForTesting({
			TypedLink: reactAdapter.VormaLink,
			linkProps: {
				href: "/click-during",
				prefetch: "intent",
				beforeBegin,
				beforeRender,
				afterRender,
				children: "Click During Prefetch",
			},
		});
		try {
			anchor.dispatchEvent(new FocusEvent("focusin", { bubbles: true }));
			await vi.advanceTimersByTimeAsync(100);

			anchor.dispatchEvent(
				new MouseEvent("click", {
					bubbles: true,
					cancelable: true,
					button: 0,
				}),
			);
			deferred.resolve(
				createRouteDataResponse({
					loadersData: [],
					title: { dangerousInnerHTML: "Eventual" },
				}),
			);
			await vi.runAllTimersAsync();

			expect(beforeBegin).toHaveBeenCalledTimes(1);
			expect(beforeRender).toHaveBeenCalledTimes(1);
			expect(afterRender).toHaveBeenCalledTimes(1);
			expect(document.title).toBe("Eventual");
		} finally {
			cleanup();
		}
	});

	it("runs beforeBegin on direct click when no prefetch has started yet", async () => {
		await initializeDistRuntimeStateForAdapters();
		const reactAdapter = await import("vorma/react");
		vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				loadersData: [],
				title: { dangerousInnerHTML: "Direct Click" },
			}),
		);
		const beforeBegin = vi.fn();
		const beforeRender = vi.fn();
		const afterRender = vi.fn();
		const { anchor, cleanup } = renderReactTypedLinkForTesting({
			TypedLink: reactAdapter.VormaLink,
			linkProps: {
				href: "/direct-click",
				prefetch: "intent",
				beforeBegin,
				beforeRender,
				afterRender,
				children: "Direct Click",
			},
		});
		try {
			anchor.dispatchEvent(
				new MouseEvent("click", {
					bubbles: true,
					cancelable: true,
					button: 0,
				}),
			);
			await vi.runAllTimersAsync();

			expect(beforeBegin).toHaveBeenCalledTimes(1);
			expect(beforeRender).toHaveBeenCalledTimes(1);
			expect(afterRender).toHaveBeenCalledTimes(1);
			expect(document.title).toBe("Direct Click");
		} finally {
			cleanup();
		}
	});

	it("drops completed prefetch cache when stop is called so later prefetches refetch", async () => {
		await initializeDistRuntimeStateForAdapters();
		const reactAdapter = await import("vorma/react");
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				loadersData: [],
			}),
		);
		const { anchor, cleanup } = renderReactTypedLinkForTesting({
			TypedLink: reactAdapter.VormaLink,
			linkProps: {
				href: "/completed-prefetch",
				prefetch: "intent",
				prefetchDelayMs: 0,
				children: "Completed Prefetch",
			},
		});
		try {
			anchor.dispatchEvent(new FocusEvent("focusin", { bubbles: true }));
			await vi.runAllTimersAsync();
			expect(fetchSpy).toHaveBeenCalledTimes(1);

			anchor.dispatchEvent(new FocusEvent("focusout", { bubbles: true }));
			anchor.dispatchEvent(new FocusEvent("focusin", { bubbles: true }));
			await vi.runAllTimersAsync();
			expect(fetchSpy).toHaveBeenCalledTimes(2);
		} finally {
			cleanup();
		}
	});

	it("prefetch-only link intent does not emit loading status transitions", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const reactAdapter = await import("vorma/react");
		const deferred = createDeferred<Response>();
		const fetchSpy = vi.spyOn(window, "fetch").mockImplementation(() => {
			return deferred.promise;
		});
		let isIndicatorRunning = false;
		const indicatorStart = vi.fn(() => {
			isIndicatorRunning = true;
		});
		const indicatorStop = vi.fn(() => {
			isIndicatorRunning = false;
		});
		const cleanupIndicator = client.setupGlobalLoadingIndicator({
			start: indicatorStart,
			stop: indicatorStop,
			isRunning: () => isIndicatorRunning,
			startDelayMS: 0,
			stopDelayMS: 0,
		});
		const statuses: Array<{
			isNavigating: boolean;
			isSubmitting: boolean;
			isRevalidating: boolean;
		}> = [];
		const removeStatusListener = client.addStatusListener((event) => {
			statuses.push(event.detail);
		});

		const TypedLink = reactAdapter.makeTypedLink(
			DIST_TEST_VORMA_APP_CONFIG,
			{
				prefetch: "intent",
				prefetchDelayMs: 50,
			},
		);
		const { anchor, cleanup } = renderReactTypedLinkForTesting({
			TypedLink,
			linkProps: {
				pattern: "/prefetch-loading-status/:id",
				params: { id: "42" },
				children: "React Prefetch Status Link",
			},
		});
		try {
			anchor.dispatchEvent(new FocusEvent("focusin", { bubbles: true }));
			await vi.advanceTimersByTimeAsync(50);
			expect(fetchSpy).toHaveBeenCalledTimes(1);
			expect(indicatorStart).not.toHaveBeenCalled();
			expect(indicatorStop).not.toHaveBeenCalled();
			expect(isIndicatorRunning).toBe(false);
			expect(
				statuses.every(
					(status) =>
						!status.isNavigating &&
						!status.isSubmitting &&
						!status.isRevalidating,
				),
			).toBe(true);

			deferred.resolve(
				createRouteDataResponse({
					matchedPatterns: ["/prefetch-loading-status/:id"],
					loadersData: [{ value: "prefetched" }],
				}),
			);
			await vi.runAllTimersAsync();

			expect(client.getStatus()).toEqual({
				isNavigating: false,
				isSubmitting: false,
				isRevalidating: false,
			});
			expect(
				statuses.every(
					(status) =>
						!status.isNavigating &&
						!status.isSubmitting &&
						!status.isRevalidating,
				),
			).toBe(true);
			expect(indicatorStart).not.toHaveBeenCalled();
			expect(indicatorStop).not.toHaveBeenCalled();
			expect(isIndicatorRunning).toBe(false);
		} finally {
			cleanupIndicator();
			removeStatusListener();
			cleanup();
		}
	});

	it("react typed links cancel pending intent prefetch on blur and allow a later fresh prefetch", async () => {
		await initializeDistRuntimeStateForAdapters();
		const reactAdapter = await import("vorma/react");
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				matchedPatterns: ["/prefetch-cancel/:id"],
				loadersData: [{ value: "prefetched" }],
			}),
		);

		const TypedLink = reactAdapter.makeTypedLink(
			DIST_TEST_VORMA_APP_CONFIG,
			{
				prefetch: "intent",
				prefetchDelayMs: 50,
			},
		);
		const { anchor, cleanup } = renderReactTypedLinkForTesting({
			TypedLink,
			linkProps: {
				pattern: "/prefetch-cancel/:id",
				params: { id: "42" },
				children: "React Prefetch Cancel Link",
			},
		});
		try {
			anchor.dispatchEvent(new FocusEvent("focusin", { bubbles: true }));
			await vi.advanceTimersByTimeAsync(25);
			anchor.dispatchEvent(new FocusEvent("focusout", { bubbles: true }));
			await vi.advanceTimersByTimeAsync(50);
			await vi.runAllTimersAsync();
			expect(fetchSpy).toHaveBeenCalledTimes(0);

			anchor.dispatchEvent(new FocusEvent("focusin", { bubbles: true }));
			await vi.advanceTimersByTimeAsync(50);
			await vi.runAllTimersAsync();
			expect(fetchSpy).toHaveBeenCalledTimes(1);
		} finally {
			cleanup();
		}
	});

	it("react typed links keep pending intent prefetch timers active on pointerleave while touch modality is active", async () => {
		await initializeDistRuntimeStateForAdapters();
		const reactAdapter = await import("vorma/react");
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				matchedPatterns: ["/touch-pointerleave-pending/:id"],
				loadersData: [{ value: "prefetched" }],
			}),
		);

		const TypedLink = reactAdapter.makeTypedLink(
			DIST_TEST_VORMA_APP_CONFIG,
			{
				prefetch: "intent",
				prefetchDelayMs: 50,
			},
		);
		const { anchor, cleanup } = renderReactTypedLinkForTesting({
			TypedLink,
			linkProps: {
				pattern: "/touch-pointerleave-pending/:id",
				params: { id: "42" },
				children: "React Touch Pending Pointerleave Link",
			},
		});
		try {
			const touchPointerDownEvent = new Event("pointerdown", {
				bubbles: true,
			});
			Object.defineProperty(touchPointerDownEvent, "pointerType", {
				value: "touch",
			});
			anchor.dispatchEvent(touchPointerDownEvent);
			anchor.dispatchEvent(new FocusEvent("focusin", { bubbles: true }));
			await vi.advanceTimersByTimeAsync(25);
			anchor.dispatchEvent(new Event("pointerout", { bubbles: true }));
			await vi.advanceTimersByTimeAsync(25);
			await vi.runAllTimersAsync();

			expect(fetchSpy).toHaveBeenCalledTimes(1);
		} finally {
			cleanup();
		}
	});

	it("react typed links do not abort in-flight intent prefetch on pointerleave while touch modality is active", async () => {
		await initializeDistRuntimeStateForAdapters();
		const reactAdapter = await import("vorma/react");
		const { requests } = createAbortAwareFetchRecorder();
		const TypedLink = reactAdapter.makeTypedLink(
			DIST_TEST_VORMA_APP_CONFIG,
			{
				prefetch: "intent",
				prefetchDelayMs: 0,
			},
		);
		const { anchor, cleanup } = renderReactTypedLinkForTesting({
			TypedLink,
			linkProps: {
				pattern: "/touch-pointerleave-in-flight/:id",
				params: { id: "42" },
				children: "React Touch In-Flight Pointerleave Link",
			},
		});
		try {
			const touchPointerDownEvent = new Event("pointerdown", {
				bubbles: true,
			});
			Object.defineProperty(touchPointerDownEvent, "pointerType", {
				value: "touch",
			});
			anchor.dispatchEvent(touchPointerDownEvent);
			anchor.dispatchEvent(new FocusEvent("focusin", { bubbles: true }));
			await waitForRequestCount({ requests, count: 1 });

			const signal = requests[0]?.init?.signal as AbortSignal | undefined;
			expect(signal?.aborted).toBe(false);

			anchor.dispatchEvent(new Event("pointerout", { bubbles: true }));
			expect(signal?.aborted).toBe(false);

			requests[0]?.deferred.resolve(
				createRouteDataResponse({
					matchedPatterns: ["/touch-pointerleave-in-flight/:id"],
					loadersData: [{ value: "prefetched" }],
				}),
			);
			await vi.runAllTimersAsync();
		} finally {
			cleanup();
		}
	});

	it("react typed links abort in-flight intent prefetch on pointerleave after switching back to fine pointer modality", async () => {
		await initializeDistRuntimeStateForAdapters();
		const reactAdapter = await import("vorma/react");
		const { requests } = createAbortAwareFetchRecorder();
		const TypedLink = reactAdapter.makeTypedLink(
			DIST_TEST_VORMA_APP_CONFIG,
			{
				prefetch: "intent",
				prefetchDelayMs: 0,
			},
		);
		const { anchor, cleanup } = renderReactTypedLinkForTesting({
			TypedLink,
			linkProps: {
				pattern: "/touch-pointerleave-fine-pointer/:id",
				params: { id: "42" },
				children: "React Touch Fine Pointerleave Link",
			},
		});
		try {
			const touchPointerDownEvent = new Event("pointerdown", {
				bubbles: true,
			});
			Object.defineProperty(touchPointerDownEvent, "pointerType", {
				value: "touch",
			});
			anchor.dispatchEvent(touchPointerDownEvent);
			anchor.dispatchEvent(new FocusEvent("focusin", { bubbles: true }));
			await waitForRequestCount({ requests, count: 1 });

			const signal = requests[0]?.init?.signal as AbortSignal | undefined;
			expect(signal?.aborted).toBe(false);

			const mousePointerMoveEvent = new Event("pointermove");
			Object.defineProperty(mousePointerMoveEvent, "pointerType", {
				value: "mouse",
			});
			window.dispatchEvent(mousePointerMoveEvent);

			anchor.dispatchEvent(new Event("pointerout", { bubbles: true }));
			expect(signal?.aborted).toBe(true);
			await vi.runAllTimersAsync();
		} finally {
			cleanup();
		}
	});

	it("react typed links drop stale settled prefetch work without unhandled rejections", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const reactAdapter = await import("vorma/react");
		const fetchDeferred = createDeferred<Response>();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockImplementation(() => fetchDeferred.promise);

		const TypedLink = reactAdapter.makeTypedLink(
			DIST_TEST_VORMA_APP_CONFIG,
			{
				prefetch: "intent",
				prefetchDelayMs: 50,
			},
		);
		const { anchor, cleanup } = renderReactTypedLinkForTesting({
			TypedLink,
			linkProps: {
				pattern: "/stale-prefetch-drop/:id",
				params: { id: "42" },
				children: "React Stale Prefetch Drop Link",
			},
		});
		try {
			const { unhandledRejections } = await withUnhandledRejectionCapture(
				{
					run: async () => {
						anchor.dispatchEvent(
							new FocusEvent("focusin", { bubbles: true }),
						);
						await vi.advanceTimersByTimeAsync(50);
						expect(fetchSpy).toHaveBeenCalledTimes(1);

						anchor.dispatchEvent(
							new FocusEvent("focusout", { bubbles: true }),
						);
						fetchDeferred.resolve(
							createRouteDataResponse({
								matchedPatterns: ["/stale-prefetch-drop/:id"],
								loadersData: [null],
								importURLs: ["/stale-prefetch-drop.module.js"],
								exportKeys: ["default"],
								errorExportKeys: [""],
								hasRootData: false,
							}),
						);
						await vi.runAllTimersAsync();
					},
				},
			);

			expect(unhandledRejections).toEqual([]);
			expect(window.location.pathname).toBe("/");
			expect(client.getStatus()).toEqual({
				isNavigating: false,
				isSubmitting: false,
				isRevalidating: false,
			});
		} finally {
			cleanup();
		}
	});

	it("preact typed links cancel pending intent prefetch on blur and allow a later fresh prefetch", async () => {
		await initializeDistRuntimeStateForAdapters();
		const preactAdapter = await import("vorma/preact");
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				matchedPatterns: ["/preact-prefetch-cancel/:id"],
				loadersData: [{ value: "prefetched" }],
			}),
		);
		const container = document.createElement("div");
		document.body.appendChild(container);

		const TypedLink = preactAdapter.makeTypedLink(
			DIST_TEST_VORMA_APP_CONFIG,
			{
				prefetch: "intent",
				prefetchDelayMs: 50,
			},
		);
		await act(async () => {
			renderPreact(
				h(TypedLink as any, {
					pattern: "/preact-prefetch-cancel/:id",
					params: { id: "42" },
					children: "Preact Prefetch Cancel Link",
				}),
				container,
			);
		});

		const anchor = container.querySelector("a");
		expect(anchor).not.toBeNull();
		anchor?.dispatchEvent(new Event("focus"));
		await vi.advanceTimersByTimeAsync(25);
		anchor?.dispatchEvent(new Event("blur"));
		await vi.advanceTimersByTimeAsync(50);
		await vi.runAllTimersAsync();
		expect(fetchSpy).toHaveBeenCalledTimes(0);

		anchor?.dispatchEvent(new Event("focus"));
		await vi.advanceTimersByTimeAsync(50);
		await vi.runAllTimersAsync();
		expect(fetchSpy).toHaveBeenCalledTimes(1);
		renderPreact(null, container);
		container.remove();
	});

	it("preact typed links run intent prefetch after delay without committing navigation", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const preactAdapter = await import("vorma/preact");
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				matchedPatterns: ["/preact-prefetch/:id"],
				loadersData: [{ value: "prefetched" }],
			}),
		);
		const container = document.createElement("div");
		document.body.appendChild(container);

		const TypedLink = preactAdapter.makeTypedLink(
			DIST_TEST_VORMA_APP_CONFIG,
			{
				prefetch: "intent",
				prefetchDelayMs: 50,
			},
		);
		await act(async () => {
			renderPreact(
				h(TypedLink as any, {
					pattern: "/preact-prefetch/:id",
					params: { id: "42" },
					children: "Preact Prefetch Link",
				}),
				container,
			);
		});

		const anchor = container.querySelector("a");
		expect(anchor).not.toBeNull();
		anchor?.dispatchEvent(new Event("focus"));
		await vi.advanceTimersByTimeAsync(49);
		expect(fetchSpy).toHaveBeenCalledTimes(0);

		await vi.advanceTimersByTimeAsync(1);
		await vi.runAllTimersAsync();

		expect(fetchSpy).toHaveBeenCalledTimes(1);
		expect(window.location.pathname).toBe("/");
		expect(client.getStatus()).toEqual({
			isNavigating: false,
			isSubmitting: false,
			isRevalidating: false,
		});
		renderPreact(null, container);
		container.remove();
	});

	it("react typed links with prefetch=none do not start prefetch work", async () => {
		await initializeDistRuntimeStateForAdapters();
		const reactAdapter = await import("vorma/react");
		const fetchSpy = vi.spyOn(window, "fetch");
		const TypedLink = reactAdapter.makeTypedLink(
			DIST_TEST_VORMA_APP_CONFIG,
			{
				prefetch: "none",
			},
		);
		const { anchor, cleanup } = renderReactTypedLinkForTesting({
			TypedLink,
			linkProps: {
				pattern: "/prefetch-none/:id",
				params: { id: "42" },
				children: "React No Prefetch Link",
			},
		});
		try {
			anchor.dispatchEvent(new FocusEvent("focusin", { bubbles: true }));
			await vi.advanceTimersByTimeAsync(200);
			await vi.runAllTimersAsync();
			expect(fetchSpy).toHaveBeenCalledTimes(0);
		} finally {
			cleanup();
		}
	});

	it("preact typed links with prefetch=none do not start prefetch work", async () => {
		await initializeDistRuntimeStateForAdapters();
		const preactAdapter = await import("vorma/preact");
		const fetchSpy = vi.spyOn(window, "fetch");
		const container = document.createElement("div");
		document.body.appendChild(container);

		const TypedLink = preactAdapter.makeTypedLink(
			DIST_TEST_VORMA_APP_CONFIG,
			{
				prefetch: "none",
			},
		);
		await act(async () => {
			renderPreact(
				h(TypedLink as any, {
					pattern: "/preact-prefetch-none/:id",
					params: { id: "42" },
					children: "Preact No Prefetch Link",
				}),
				container,
			);
		});

		const anchor = container.querySelector("a");
		expect(anchor).not.toBeNull();
		anchor?.dispatchEvent(new Event("focus"));
		await vi.advanceTimersByTimeAsync(200);
		await vi.runAllTimersAsync();

		expect(fetchSpy).toHaveBeenCalledTimes(0);
		renderPreact(null, container);
		container.remove();
	});

	it("solid typed links with prefetch=none do not start prefetch work", async () => {
		await initializeDistRuntimeStateForAdapters();
		const solidAdapter = await import("vorma/solid");
		const fetchSpy = vi.spyOn(window, "fetch");
		const container = document.createElement("div");
		document.body.appendChild(container);

		const TypedLink = solidAdapter.makeTypedLink(
			DIST_TEST_VORMA_APP_CONFIG,
			{
				prefetch: "none",
			},
		);
		const dispose = renderSolidWithTrackedDisposer(() => {
			return (TypedLink as any)({
				pattern: "/solid-prefetch-none/:id",
				params: { id: "42" },
				children: "Solid No Prefetch Link",
			});
		}, container);

		const anchor = container.querySelector("a");
		expect(anchor).not.toBeNull();
		anchor?.dispatchEvent(new Event("pointerenter", { bubbles: true }));
		await vi.advanceTimersByTimeAsync(200);
		await vi.runAllTimersAsync();

		expect(fetchSpy).toHaveBeenCalledTimes(0);
		dispose();
		container.remove();
	});

	it("solid typed links run intent prefetch after delay without committing navigation", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const solidAdapter = await import("vorma/solid");
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				matchedPatterns: ["/solid-prefetch/:id"],
				loadersData: [{ value: "prefetched" }],
			}),
		);
		const container = document.createElement("div");
		document.body.appendChild(container);

		const TypedLink = solidAdapter.makeTypedLink(
			DIST_TEST_VORMA_APP_CONFIG,
			{
				prefetch: "intent",
				prefetchDelayMs: 50,
			},
		);
		const dispose = renderSolidWithTrackedDisposer(() => {
			return (TypedLink as any)({
				pattern: "/solid-prefetch/:id",
				params: { id: "42" },
				children: "Solid Prefetch Link",
			});
		}, container);

		const anchor = container.querySelector("a");
		expect(anchor).not.toBeNull();
		anchor?.dispatchEvent(new Event("focus"));
		await vi.advanceTimersByTimeAsync(49);
		expect(fetchSpy).toHaveBeenCalledTimes(0);

		await vi.advanceTimersByTimeAsync(1);
		await vi.runAllTimersAsync();

		expect(fetchSpy).toHaveBeenCalledTimes(1);
		expect(window.location.pathname).toBe("/");
		expect(client.getStatus()).toEqual({
			isNavigating: false,
			isSubmitting: false,
			isRevalidating: false,
		});
		dispose();
		container.remove();
	});

	it("solid typed links cancel pending intent prefetch on blur and allow a later fresh prefetch", async () => {
		await initializeDistRuntimeStateForAdapters();
		const solidAdapter = await import("vorma/solid");
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				matchedPatterns: ["/solid-prefetch-cancel/:id"],
				loadersData: [{ value: "prefetched" }],
			}),
		);
		const container = document.createElement("div");
		document.body.appendChild(container);

		const TypedLink = solidAdapter.makeTypedLink(
			DIST_TEST_VORMA_APP_CONFIG,
			{
				prefetch: "intent",
				prefetchDelayMs: 50,
			},
		);
		const dispose = renderSolidWithTrackedDisposer(() => {
			return (TypedLink as any)({
				pattern: "/solid-prefetch-cancel/:id",
				params: { id: "42" },
				children: "Solid Prefetch Cancel Link",
			});
		}, container);

		const anchor = container.querySelector("a");
		expect(anchor).not.toBeNull();
		anchor?.dispatchEvent(new Event("focus"));
		await vi.advanceTimersByTimeAsync(25);
		anchor?.dispatchEvent(new Event("blur"));
		await vi.advanceTimersByTimeAsync(50);
		await vi.runAllTimersAsync();
		expect(fetchSpy).toHaveBeenCalledTimes(0);

		anchor?.dispatchEvent(new Event("focus"));
		await vi.advanceTimersByTimeAsync(50);
		await vi.runAllTimersAsync();
		expect(fetchSpy).toHaveBeenCalledTimes(1);
		dispose();
		container.remove();
	});

	it("react typed links do not intercept modified clicks", async () => {
		await initializeDistRuntimeStateForAdapters();
		const reactAdapter = await import("vorma/react");
		const fetchSpy = vi.spyOn(window, "fetch");
		const TypedLink = reactAdapter.makeTypedLink(
			DIST_TEST_VORMA_APP_CONFIG,
			{},
		);
		const { anchor, cleanup } = renderReactTypedLinkForTesting({
			TypedLink,
			linkProps: {
				pattern: "/modified/:id",
				params: { id: "42" },
				children: "React Modified Link",
			},
		});
		try {
			anchor.addEventListener(
				"click",
				(event) => {
					event.preventDefault();
				},
				{ once: true },
			);
			const clickEvent = new MouseEvent("click", {
				bubbles: true,
				cancelable: true,
				button: 0,
				metaKey: true,
			});
			anchor.dispatchEvent(clickEvent);
			await vi.runAllTimersAsync();

			expect(fetchSpy).toHaveBeenCalledTimes(0);
			expect(window.location.pathname).toBe("/");
		} finally {
			cleanup();
		}
	});

	it("react typed links prevent default and navigate for eligible internal clicks", async () => {
		await initializeDistRuntimeStateForAdapters();
		const reactAdapter = await import("vorma/react");
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				matchedPatterns: ["/react-eligible/:id"],
				loadersData: [{ value: "ok" }],
			}),
		);
		const TypedLink = reactAdapter.makeTypedLink(
			DIST_TEST_VORMA_APP_CONFIG,
			{},
		);
		const { anchor, cleanup } = renderReactTypedLinkForTesting({
			TypedLink,
			linkProps: {
				pattern: "/react-eligible/:id",
				params: { id: "42" },
				children: "React Eligible Link",
			},
		});
		try {
			const clickEvent = new MouseEvent("click", {
				bubbles: true,
				cancelable: true,
				button: 0,
			});
			anchor.dispatchEvent(clickEvent);
			await vi.runAllTimersAsync();

			expect(clickEvent.defaultPrevented).toBe(true);
			expect(fetchSpy).toHaveBeenCalledTimes(1);
			expect(window.location.pathname).toBe("/react-eligible/42");
		} finally {
			cleanup();
		}
	});

	it("react typed links honor consumer onClick preventDefault and skip navigation", async () => {
		await initializeDistRuntimeStateForAdapters();
		const reactAdapter = await import("vorma/react");
		const fetchSpy = vi.spyOn(window, "fetch");
		const consumerOnClick = vi.fn((event: MouseEvent) => {
			event.preventDefault();
		});
		const TypedLink = reactAdapter.makeTypedLink(
			DIST_TEST_VORMA_APP_CONFIG,
			{},
		);
		const { anchor, cleanup } = renderReactTypedLinkForTesting({
			TypedLink,
			linkProps: {
				pattern: "/react-prevent-default/:id",
				params: { id: "42" },
				onClick: consumerOnClick,
				children: "React Prevent Default Link",
			},
		});
		try {
			const clickEvent = new MouseEvent("click", {
				bubbles: true,
				cancelable: true,
				button: 0,
			});
			anchor.dispatchEvent(clickEvent);
			await vi.runAllTimersAsync();

			expect(consumerOnClick).toHaveBeenCalledTimes(1);
			expect(clickEvent.defaultPrevented).toBe(true);
			expect(fetchSpy).toHaveBeenCalledTimes(0);
			expect(window.location.pathname).toBe("/");
		} finally {
			cleanup();
		}
	});

	it("react typed links do not intercept non-primary button clicks", async () => {
		await initializeDistRuntimeStateForAdapters();
		const reactAdapter = await import("vorma/react");
		const fetchSpy = vi.spyOn(window, "fetch");
		const TypedLink = reactAdapter.makeTypedLink(
			DIST_TEST_VORMA_APP_CONFIG,
			{},
		);
		const { anchor, cleanup } = renderReactTypedLinkForTesting({
			TypedLink,
			linkProps: {
				pattern: "/react-secondary-button/:id",
				params: { id: "42" },
				children: "React Secondary Button Link",
			},
		});
		try {
			const clickEvent = new MouseEvent("click", {
				bubbles: true,
				cancelable: true,
				button: 1,
			});
			anchor.dispatchEvent(clickEvent);
			await vi.runAllTimersAsync();

			expect(clickEvent.defaultPrevented).toBe(false);
			expect(fetchSpy).toHaveBeenCalledTimes(0);
			expect(window.location.pathname).toBe("/");
		} finally {
			cleanup();
		}
	});

	it("react typed links handle text-node click targets inside internal anchors", async () => {
		await initializeDistRuntimeStateForAdapters();
		const reactAdapter = await import("vorma/react");
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				matchedPatterns: ["/react-text-node/:id"],
				loadersData: [{ value: "ok" }],
			}),
		);
		const TypedLink = reactAdapter.makeTypedLink(
			DIST_TEST_VORMA_APP_CONFIG,
			{},
		);
		const { anchor, cleanup } = renderReactTypedLinkForTesting({
			TypedLink,
			linkProps: {
				pattern: "/react-text-node/:id",
				params: { id: "42" },
				children: "React Text Node Link",
			},
		});
		try {
			const textNode = anchor.firstChild;
			expect(textNode).not.toBeNull();
			if (textNode === null) {
				throw new Error(
					"Expected rendered link to contain a text node.",
				);
			}
			const clickEvent = new MouseEvent("click", {
				bubbles: true,
				cancelable: true,
				button: 0,
			});
			textNode.dispatchEvent(clickEvent);
			await vi.runAllTimersAsync();

			expect(clickEvent.defaultPrevented).toBe(true);
			expect(fetchSpy).toHaveBeenCalledTimes(1);
			expect(window.location.pathname).toBe("/react-text-node/42");
		} finally {
			cleanup();
		}
	});

	it("react typed links handle hash-only same-document clicks without fetch", async () => {
		await initializeDistRuntimeStateForAdapters();
		window.history.replaceState({}, "", "/react-hash-only/42");
		const reactAdapter = await import("vorma/react");
		const fetchSpy = vi.spyOn(window, "fetch");
		const beforeBegin = vi.fn();
		const beforeRender = vi.fn();
		const afterRender = vi.fn();
		const TypedLink = reactAdapter.makeTypedLink(
			DIST_TEST_VORMA_APP_CONFIG,
			{},
		);
		const { anchor, cleanup } = renderReactTypedLinkForTesting({
			TypedLink,
			linkProps: {
				pattern: "/react-hash-only/:id",
				params: { id: "42" },
				hash: "#details",
				beforeBegin,
				beforeRender,
				afterRender,
				children: "React Hash Only Link",
			},
		});
		try {
			const clickEvent = new MouseEvent("click", {
				bubbles: true,
				cancelable: true,
				button: 0,
			});
			anchor.dispatchEvent(clickEvent);
			await vi.runAllTimersAsync();

			expect(clickEvent.defaultPrevented).toBe(true);
			expect(fetchSpy).toHaveBeenCalledTimes(0);
			expect(window.location.pathname).toBe("/react-hash-only/42");
			expect(window.location.hash).toBe("#details");
			expect(beforeBegin).toHaveBeenCalledTimes(0);
			expect(beforeRender).toHaveBeenCalledTimes(0);
			expect(afterRender).toHaveBeenCalledTimes(0);
		} finally {
			cleanup();
		}
	});

	it("react typed links handle same-document hash-removal clicks without fetch", async () => {
		await initializeDistRuntimeStateForAdapters();
		window.history.replaceState({}, "", "/react-hash-remove/42#details");
		const reactAdapter = await import("vorma/react");
		const fetchSpy = vi.spyOn(window, "fetch");
		const TypedLink = reactAdapter.makeTypedLink(
			DIST_TEST_VORMA_APP_CONFIG,
			{},
		);
		const { anchor, cleanup } = renderReactTypedLinkForTesting({
			TypedLink,
			linkProps: {
				pattern: "/react-hash-remove/:id",
				params: { id: "42" },
				children: "React Hash Remove Link",
			},
		});
		try {
			const clickEvent = new MouseEvent("click", {
				bubbles: true,
				cancelable: true,
				button: 0,
			});
			anchor.dispatchEvent(clickEvent);
			await vi.runAllTimersAsync();

			expect(clickEvent.defaultPrevented).toBe(true);
			expect(fetchSpy).toHaveBeenCalledTimes(0);
			expect(window.location.pathname).toBe("/react-hash-remove/42");
			expect(window.location.hash).toBe("");
		} finally {
			cleanup();
		}
	});

	it("react typed links do not fetch for same-document no-op hash clicks", async () => {
		await initializeDistRuntimeStateForAdapters();
		window.history.replaceState({}, "", "/react-hash-noop/42#details");
		const reactAdapter = await import("vorma/react");
		const fetchSpy = vi.spyOn(window, "fetch");
		const beforeBegin = vi.fn();
		const beforeRender = vi.fn();
		const afterRender = vi.fn();
		const TypedLink = reactAdapter.makeTypedLink(
			DIST_TEST_VORMA_APP_CONFIG,
			{},
		);
		const { anchor, cleanup } = renderReactTypedLinkForTesting({
			TypedLink,
			linkProps: {
				pattern: "/react-hash-noop/:id",
				params: { id: "42" },
				hash: "#details",
				beforeBegin,
				beforeRender,
				afterRender,
				children: "React Hash No-op Link",
			},
		});
		try {
			const clickEvent = new MouseEvent("click", {
				bubbles: true,
				cancelable: true,
				button: 0,
			});
			anchor.dispatchEvent(clickEvent);
			await vi.runAllTimersAsync();

			expect(clickEvent.defaultPrevented).toBe(true);
			expect(fetchSpy).toHaveBeenCalledTimes(0);
			expect(window.location.pathname).toBe("/react-hash-noop/42");
			expect(window.location.hash).toBe("#details");
			expect(beforeBegin).toHaveBeenCalledTimes(0);
			expect(beforeRender).toHaveBeenCalledTimes(0);
			expect(afterRender).toHaveBeenCalledTimes(0);
		} finally {
			cleanup();
		}
	});

	it("react typed links prevent default and do not fetch for same-document no-op links without hash", async () => {
		await initializeDistRuntimeStateForAdapters();
		window.history.replaceState({}, "", "/react-noop-no-hash/42");
		const reactAdapter = await import("vorma/react");
		const fetchSpy = vi.spyOn(window, "fetch");
		const TypedLink = reactAdapter.makeTypedLink(
			DIST_TEST_VORMA_APP_CONFIG,
			{},
		);
		const { anchor, cleanup } = renderReactTypedLinkForTesting({
			TypedLink,
			linkProps: {
				pattern: "/react-noop-no-hash/:id",
				params: { id: "42" },
				children: "React No-op Link",
			},
		});
		try {
			const clickEvent = new MouseEvent("click", {
				bubbles: true,
				cancelable: true,
				button: 0,
			});
			anchor.dispatchEvent(clickEvent);
			await vi.runAllTimersAsync();

			expect(clickEvent.defaultPrevented).toBe(true);
			expect(fetchSpy).toHaveBeenCalledTimes(0);
			expect(window.location.pathname).toBe("/react-noop-no-hash/42");
			expect(window.location.hash).toBe("");
		} finally {
			cleanup();
		}
	});

	it("react typed links do not fetch for same-document encoding-equivalent hash clicks", async () => {
		await initializeDistRuntimeStateForAdapters();
		window.history.replaceState({}, "", "/react-encoding-hash/42#~");
		const reactAdapter = await import("vorma/react");
		const fetchSpy = vi.spyOn(window, "fetch");
		const TypedLink = reactAdapter.makeTypedLink(
			DIST_TEST_VORMA_APP_CONFIG,
			{},
		);
		const { anchor, cleanup } = renderReactTypedLinkForTesting({
			TypedLink,
			linkProps: {
				pattern: "/react-encoding-hash/:id",
				params: { id: "42" },
				hash: "#%7E",
				children: "React Encoding Equivalent Link",
			},
		});
		try {
			const clickEvent = new MouseEvent("click", {
				bubbles: true,
				cancelable: true,
				button: 0,
			});
			anchor.dispatchEvent(clickEvent);
			await vi.runAllTimersAsync();

			expect(clickEvent.defaultPrevented).toBe(true);
			expect(fetchSpy).toHaveBeenCalledTimes(0);
			expect(window.location.pathname).toBe("/react-encoding-hash/42");
			expect(
				window.location.hash === "#~" ||
					window.location.hash === "#%7E",
			).toBe(true);
		} finally {
			cleanup();
		}
	});

	it("react typed links do not intercept target=_blank clicks", async () => {
		await initializeDistRuntimeStateForAdapters();
		const reactAdapter = await import("vorma/react");
		const fetchSpy = vi.spyOn(window, "fetch");
		const TypedLink = reactAdapter.makeTypedLink(
			DIST_TEST_VORMA_APP_CONFIG,
			{},
		);
		const { anchor, cleanup } = renderReactTypedLinkForTesting({
			TypedLink,
			linkProps: {
				pattern: "/new-tab/:id",
				params: { id: "42" },
				target: "_blank",
				children: "React New Tab Link",
			},
		});
		try {
			anchor.addEventListener(
				"click",
				(event) => {
					event.preventDefault();
				},
				{ once: true },
			);
			const clickEvent = new MouseEvent("click", {
				bubbles: true,
				cancelable: true,
				button: 0,
			});
			anchor.dispatchEvent(clickEvent);
			await vi.runAllTimersAsync();

			expect(fetchSpy).toHaveBeenCalledTimes(0);
			expect(window.location.pathname).toBe("/");
		} finally {
			cleanup();
		}
	});

	it("preact typed links do not intercept modified clicks", async () => {
		await initializeDistRuntimeStateForAdapters();
		const preactAdapter = await import("vorma/preact");
		const fetchSpy = vi.spyOn(window, "fetch");
		const container = document.createElement("div");
		document.body.appendChild(container);

		const TypedLink = preactAdapter.makeTypedLink(
			DIST_TEST_VORMA_APP_CONFIG,
			{},
		);
		await act(async () => {
			renderPreact(
				h(TypedLink as any, {
					pattern: "/preact-modified/:id",
					params: { id: "42" },
					children: "Preact Link",
				}),
				container,
			);
		});

		const anchor = container.querySelector("a");
		expect(anchor).not.toBeNull();
		anchor?.addEventListener(
			"click",
			(event) => {
				event.preventDefault();
			},
			{ once: true },
		);
		const clickEvent = new MouseEvent("click", {
			bubbles: true,
			cancelable: true,
			metaKey: true,
			button: 0,
		});
		anchor?.dispatchEvent(clickEvent);
		await vi.runAllTimersAsync();

		expect(fetchSpy).toHaveBeenCalledTimes(0);
		expect(window.location.pathname).toBe("/");
		renderPreact(null, container);
		container.remove();
	});

	it("solid typed links do not intercept modified clicks", async () => {
		await initializeDistRuntimeStateForAdapters();
		const solidAdapter = await import("vorma/solid");
		const fetchSpy = vi.spyOn(window, "fetch");
		const container = document.createElement("div");
		document.body.appendChild(container);

		const TypedLink = solidAdapter.makeTypedLink(
			DIST_TEST_VORMA_APP_CONFIG,
			{},
		);
		const dispose = renderSolidWithTrackedDisposer(() => {
			return (TypedLink as any)({
				pattern: "/solid-modified/:id",
				params: { id: "42" },
				children: "Solid Link",
			});
		}, container);

		const anchor = container.querySelector("a");
		expect(anchor).not.toBeNull();
		anchor?.addEventListener(
			"click",
			(event) => {
				event.preventDefault();
			},
			{ once: true },
		);
		const clickEvent = new MouseEvent("click", {
			bubbles: true,
			cancelable: true,
			metaKey: true,
			button: 0,
		});
		anchor?.dispatchEvent(clickEvent);
		await vi.runAllTimersAsync();

		expect(fetchSpy).toHaveBeenCalledTimes(0);
		expect(window.location.pathname).toBe("/");
		dispose();
		container.remove();
	});

	it("preact typed links do not intercept target=_blank clicks", async () => {
		await initializeDistRuntimeStateForAdapters();
		const preactAdapter = await import("vorma/preact");
		const fetchSpy = vi.spyOn(window, "fetch");
		const container = document.createElement("div");
		document.body.appendChild(container);

		const TypedLink = preactAdapter.makeTypedLink(
			DIST_TEST_VORMA_APP_CONFIG,
			{},
		);
		await act(async () => {
			renderPreact(
				h(TypedLink as any, {
					pattern: "/preact-new-tab/:id",
					params: { id: "42" },
					target: "_blank",
					children: "Preact New Tab Link",
				}),
				container,
			);
		});

		const anchor = container.querySelector("a");
		expect(anchor).not.toBeNull();
		anchor?.addEventListener(
			"click",
			(event) => {
				event.preventDefault();
			},
			{ once: true },
		);
		const clickEvent = new MouseEvent("click", {
			bubbles: true,
			cancelable: true,
			button: 0,
		});
		anchor?.dispatchEvent(clickEvent);
		await vi.runAllTimersAsync();

		expect(fetchSpy).toHaveBeenCalledTimes(0);
		expect(window.location.pathname).toBe("/");
		renderPreact(null, container);
		container.remove();
	});

	it("solid typed links do not intercept target=_blank clicks", async () => {
		await initializeDistRuntimeStateForAdapters();
		const solidAdapter = await import("vorma/solid");
		const fetchSpy = vi.spyOn(window, "fetch");
		const container = document.createElement("div");
		document.body.appendChild(container);

		const TypedLink = solidAdapter.makeTypedLink(
			DIST_TEST_VORMA_APP_CONFIG,
			{},
		);
		const dispose = renderSolidWithTrackedDisposer(() => {
			return (TypedLink as any)({
				pattern: "/solid-new-tab/:id",
				params: { id: "42" },
				target: "_blank",
				children: "Solid New Tab Link",
			});
		}, container);

		const anchor = container.querySelector("a");
		expect(anchor).not.toBeNull();
		anchor?.addEventListener(
			"click",
			(event) => {
				event.preventDefault();
			},
			{ once: true },
		);
		const clickEvent = new MouseEvent("click", {
			bubbles: true,
			cancelable: true,
			button: 0,
		});
		anchor?.dispatchEvent(clickEvent);
		await vi.runAllTimersAsync();

		expect(fetchSpy).toHaveBeenCalledTimes(0);
		expect(window.location.pathname).toBe("/");
		dispose();
		container.remove();
	});

	it("preact typed links call beforeBegin on normal internal clicks", async () => {
		await initializeDistRuntimeStateForAdapters();
		const preactAdapter = await import("vorma/preact");
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				matchedPatterns: ["/preact-before-begin/:id"],
				loadersData: [{ value: "ok" }],
			}),
		);
		const beforeBegin = vi.fn();
		const container = document.createElement("div");
		document.body.appendChild(container);

		const TypedLink = preactAdapter.makeTypedLink(
			DIST_TEST_VORMA_APP_CONFIG,
			{},
		);
		await act(async () => {
			renderPreact(
				h(TypedLink as any, {
					pattern: "/preact-before-begin/:id",
					params: { id: "42" },
					beforeBegin,
					children: "Preact Click Link",
				}),
				container,
			);
		});

		const anchor = container.querySelector("a");
		expect(anchor).not.toBeNull();
		const clickEvent = new MouseEvent("click", {
			bubbles: true,
			cancelable: true,
			button: 0,
		});
		anchor?.dispatchEvent(clickEvent);
		await vi.runAllTimersAsync();

		expect(beforeBegin).toHaveBeenCalledTimes(1);
		expect(fetchSpy).toHaveBeenCalledTimes(1);
		expect(window.location.pathname).toBe("/preact-before-begin/42");
		renderPreact(null, container);
		container.remove();
	});

	it("react typed links call beforeBegin on normal internal clicks", async () => {
		await initializeDistRuntimeStateForAdapters();
		const reactAdapter = await import("vorma/react");
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				matchedPatterns: ["/react-before-begin/:id"],
				loadersData: [{ value: "ok" }],
			}),
		);
		const beforeBegin = vi.fn();
		const container = document.createElement("div");
		document.body.appendChild(container);
		const root = createRoot(container);

		const TypedLink = reactAdapter.makeTypedLink(
			DIST_TEST_VORMA_APP_CONFIG,
			{},
		);
		flushSync(() => {
			root.render(
				React.createElement(TypedLink as any, {
					pattern: "/react-before-begin/:id",
					params: { id: "42" },
					beforeBegin,
					children: "React Click Link",
				}),
			);
		});

		const anchor = container.querySelector("a");
		expect(anchor).not.toBeNull();
		const clickEvent = new MouseEvent("click", {
			bubbles: true,
			cancelable: true,
			button: 0,
		});
		anchor?.dispatchEvent(clickEvent);
		await vi.runAllTimersAsync();

		expect(beforeBegin).toHaveBeenCalledTimes(1);
		expect(fetchSpy).toHaveBeenCalledTimes(1);
		expect(window.location.pathname).toBe("/react-before-begin/42");
		root.unmount();
		container.remove();
	});

	it("solid typed links call beforeBegin on normal internal clicks", async () => {
		await initializeDistRuntimeStateForAdapters();
		const solidAdapter = await import("vorma/solid");
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				matchedPatterns: ["/solid-before-begin/:id"],
				loadersData: [{ value: "ok" }],
			}),
		);
		const beforeBegin = vi.fn();
		const container = document.createElement("div");
		document.body.appendChild(container);

		const TypedLink = solidAdapter.makeTypedLink(
			DIST_TEST_VORMA_APP_CONFIG,
			{},
		);
		const dispose = renderSolidWithTrackedDisposer(() => {
			return (TypedLink as any)({
				pattern: "/solid-before-begin/:id",
				params: { id: "42" },
				beforeBegin,
				children: "Solid Click Link",
			});
		}, container);

		const anchor = container.querySelector("a");
		expect(anchor).not.toBeNull();
		const clickEvent = new MouseEvent("click", {
			bubbles: true,
			cancelable: true,
			button: 0,
		});
		anchor?.dispatchEvent(clickEvent);
		await vi.runAllTimersAsync();

		expect(beforeBegin).toHaveBeenCalledTimes(1);
		expect(fetchSpy).toHaveBeenCalledTimes(1);
		expect(window.location.pathname).toBe("/solid-before-begin/42");
		dispose();
		container.remove();
	});

	it("react links mark external hrefs and do not prevent default for external clicks", async () => {
		await initializeDistRuntimeStateForAdapters();
		const reactAdapter = await import("vorma/react");
		const fetchSpy = vi.spyOn(window, "fetch");
		const { anchor, cleanup } = renderReactTypedLinkForTesting({
			TypedLink: reactAdapter.VormaLink,
			linkProps: {
				href: "https://external.example/current-page#x",
				children: "External Link",
			},
		});
		try {
			expect(anchor.getAttribute("data-external")).toBe("true");

			const clickEvent = new MouseEvent("click", {
				bubbles: true,
				cancelable: true,
				button: 0,
			});
			anchor.dispatchEvent(clickEvent);
			await vi.runAllTimersAsync();

			expect(clickEvent.defaultPrevented).toBe(false);
			expect(fetchSpy).toHaveBeenCalledTimes(0);
		} finally {
			cleanup();
		}
	});

	it("react links do not save scroll state for modifier-key same-document hash clicks", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const reactAdapter = await import("vorma/react");
		window.history.replaceState({}, "", "/current-page");
		client.getHistoryInstance();
		const beforeEntries = readScrollStateForTesting();
		const { anchor, cleanup } = renderReactTypedLinkForTesting({
			TypedLink: reactAdapter.VormaLink,
			linkProps: {
				href: "/current-page#section",
				children: "Hash Link",
			},
		});
		try {
			const clickEvent = new MouseEvent("click", {
				bubbles: true,
				cancelable: true,
				button: 0,
				ctrlKey: true,
			});
			anchor.dispatchEvent(clickEvent);
			await vi.runAllTimersAsync();
			expect(readScrollStateForTesting()).toEqual(beforeEntries);
		} finally {
			cleanup();
		}
	});

	it("react links do not treat cross-origin hash links as same-document hash changes", async () => {
		await initializeDistRuntimeStateForAdapters();
		const reactAdapter = await import("vorma/react");
		const fetchSpy = vi.spyOn(window, "fetch");
		const beforeEntries = readScrollStateForTesting();
		const { anchor, cleanup } = renderReactTypedLinkForTesting({
			TypedLink: reactAdapter.VormaLink,
			linkProps: {
				href: "https://external.example/current-page#x",
				children: "Cross Origin Hash Link",
			},
		});
		try {
			const clickEvent = new MouseEvent("click", {
				bubbles: true,
				cancelable: true,
				button: 0,
			});
			anchor.dispatchEvent(clickEvent);
			await vi.runAllTimersAsync();

			expect(clickEvent.defaultPrevented).toBe(false);
			expect(fetchSpy).toHaveBeenCalledTimes(0);
			expect(readScrollStateForTesting()).toEqual(beforeEntries);
		} finally {
			cleanup();
		}
	});

	it("updates build ID before following redirects triggered by link clicks", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const reactAdapter = await import("vorma/react");
		let buildIdDuringEvent: string | undefined;
		const cleanupBuildListener = client.addBuildIDListener(() => {
			buildIdDuringEvent = client.getBuildID();
		});
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse(
					{},
					{
						headers: {
							"X-Client-Redirect": "/redirected-click",
							"X-Wave-Framework-Build-Id": "build-click-2",
						},
					},
				),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse(
					{},
					{
						headers: {
							"X-Wave-Framework-Build-Id": "build-click-2",
						},
					},
				),
			);
		const TypedLink = reactAdapter.makeTypedLink(
			DIST_TEST_VORMA_APP_CONFIG,
			{},
		);
		const { anchor, cleanup } = renderReactTypedLinkForTesting({
			TypedLink,
			linkProps: {
				pattern: "/click-start",
				children: "Redirect Link",
			},
		});
		try {
			const clickEvent = new MouseEvent("click", {
				bubbles: true,
				cancelable: true,
				button: 0,
			});
			anchor.dispatchEvent(clickEvent);
			await vi.runAllTimersAsync();

			expect(fetchSpy).toHaveBeenCalledTimes(2);
			const secondFetchURL = requestInputToURL(
				fetchSpy.mock.calls[1]?.[0] as RequestInfo | URL,
			);
			expect(secondFetchURL.pathname).toBe("/redirected-click");
			expect(secondFetchURL.searchParams.get("vorma_json")).toBe(
				"build-click-2",
			);
			expect(buildIdDuringEvent).toBe("build-click-2");
			expect(client.getBuildID()).toBe("build-click-2");
		} finally {
			cleanup();
			cleanupBuildListener();
		}
	});

	it("cleans up stale link navigation outcomes when a newer navigation supersedes the click", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const reactAdapter = await import("vorma/react");
		const staleFetch = createDeferred<Response>();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockImplementation((input) => {
				if (fetchSpy.mock.calls.length === 1) {
					return staleFetch.promise;
				}
				const requestURL = requestInputToURL(input);
				return Promise.resolve(
					createRouteDataResponse({
						title: {
							dangerousInnerHTML:
								requestURL.pathname === "/winner"
									? "Winner"
									: "Stale",
						},
					}),
				);
			});

		const { anchor, cleanup } = renderReactTypedLinkForTesting({
			TypedLink: reactAdapter.VormaLink,
			linkProps: {
				href: "/aborted-click",
				children: "Aborted Click",
			},
		});
		try {
			anchor.dispatchEvent(
				new MouseEvent("click", {
					bubbles: true,
					cancelable: true,
					button: 0,
				}),
			);
			await waitForRequestCount({
				requests: fetchSpy.mock.calls,
				count: 1,
			});

			await client.vormaNavigate("/winner");
			await waitForRequestCount({
				requests: fetchSpy.mock.calls,
				count: 2,
			});

			staleFetch.resolve(
				createRouteDataResponse({
					title: { dangerousInnerHTML: "Stale" },
				}),
			);
			await vi.runAllTimersAsync();

			expect(fetchSpy).toHaveBeenCalledTimes(2);
			expect(window.location.pathname).toBe("/winner");
		} finally {
			cleanup();
		}
	});

	it("does not leak unhandled rejections and clears navigating state when link fetch fails", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const reactAdapter = await import("vorma/react");
		const consoleErrorSpy = vi
			.spyOn(console, "error")
			.mockImplementation(() => {});
		vi.spyOn(window, "fetch").mockRejectedValue(
			new Error("network failed"),
		);
		const TypedLink = reactAdapter.makeTypedLink(
			DIST_TEST_VORMA_APP_CONFIG,
			{},
		);
		const { anchor, cleanup } = renderReactTypedLinkForTesting({
			TypedLink,
			linkProps: {
				pattern: "/failing-click",
				children: "Failing Link",
			},
		});
		try {
			const { unhandledRejections } = await withUnhandledRejectionCapture(
				{
					run: async () => {
						anchor.dispatchEvent(
							new MouseEvent("click", {
								bubbles: true,
								cancelable: true,
								button: 0,
							}),
						);
						await vi.runAllTimersAsync();
					},
				},
			);

			expect(unhandledRejections).toEqual([]);
			expect(client.getStatus().isNavigating).toBe(false);
			expect(consoleErrorSpy).toHaveBeenCalled();
		} finally {
			cleanup();
		}
	});

	it("rejects client-loader serverDataPromise with AbortError when scoped data is aborted before prefetch settles", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const reactAdapter = await import("vorma/react");
		const addClientLoader = reactAdapter.makeTypedAddClientLoader(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		const serverDataPromiseErrors: Array<unknown> = [];
		addClientLoader({
			pattern: "/prefetch-server-data-abort/:id",
			clientLoader: async ({ serverDataPromise }) => {
				try {
					await serverDataPromise;
				} catch (error) {
					serverDataPromiseErrors.push(error);
					throw error;
				}
				return null;
			},
		});
		const fetchDeferred = createDeferred<Response>();
		vi.spyOn(window, "fetch").mockImplementation(
			() => fetchDeferred.promise,
		);
		const TypedLink = reactAdapter.makeTypedLink(
			DIST_TEST_VORMA_APP_CONFIG,
			{
				prefetch: "intent",
				prefetchDelayMs: 50,
			},
		);
		const { anchor, cleanup } = renderReactTypedLinkForTesting({
			TypedLink,
			linkProps: {
				pattern: "/prefetch-server-data-abort/:id",
				params: { id: "42" },
				children: "Prefetch Abort Link",
			},
		});
		try {
			anchor.dispatchEvent(new FocusEvent("focusin", { bubbles: true }));
			await vi.advanceTimersByTimeAsync(50);
			anchor.dispatchEvent(new FocusEvent("focusout", { bubbles: true }));
			fetchDeferred.resolve(
				createRouteDataResponse({
					matchedPatterns: ["/prefetch-server-data-abort/:id"],
					loadersData: [{ value: "ignored" }],
					importURLs: [],
					exportKeys: [],
					errorExportKeys: [],
				}),
			);
			await vi.runAllTimersAsync();

			expect(serverDataPromiseErrors).toHaveLength(1);
			expect(serverDataPromiseErrors[0]).toBeInstanceOf(Error);
			expect((serverDataPromiseErrors[0] as Error).name).toBe(
				"AbortError",
			);
			expect(client.getStatus()).toEqual({
				isNavigating: false,
				isSubmitting: false,
				isRevalidating: false,
			});
		} finally {
			cleanup();
		}
	});

	it("contains client-loader failures without leaking unhandled rejections", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const reactAdapter = await import("vorma/react");
		const addClientLoader = reactAdapter.makeTypedAddClientLoader(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		addClientLoader({
			pattern: "/client-loader-failure",
			clientLoader: async () => {
				throw new Error("Client loader failed");
			},
		});
		vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				matchedPatterns: ["/client-loader-failure"],
				loadersData: [{ value: "server" }],
				importURLs: [],
				exportKeys: [],
				errorExportKeys: [],
			}),
		);

		const { unhandledRejections } = await withUnhandledRejectionCapture({
			run: async () => {
				await client.vormaNavigate("/client-loader-failure");
				await vi.runAllTimersAsync();
			},
		});

		expect(unhandledRejections).toEqual([]);
		expect(client.getStatus()).toEqual({
			isNavigating: false,
			isSubmitting: false,
			isRevalidating: false,
		});
	});

	it("ignores stale late navigation outcomes with client-loader failures after a newer navigation wins", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const reactAdapter = await import("vorma/react");
		const addClientLoader = reactAdapter.makeTypedAddClientLoader(
			DIST_TEST_VORMA_APP_CONFIG,
		);
		addClientLoader({
			pattern: "/stale-client-loader-failure",
			clientLoader: async () => {
				throw new Error("Stale client-loader failure");
			},
		});
		const staleDeferred = createDeferred<Response>();
		const freshDeferred = createDeferred<Response>();
		const signals: AbortSignal[] = [];
		let fetchCallCount = 0;
		vi.spyOn(window, "fetch").mockImplementation((_input, init) => {
			const signal = (init as RequestInit | undefined)?.signal as
				| AbortSignal
				| undefined;
			if (signal) {
				signals.push(signal);
			}
			fetchCallCount += 1;
			return fetchCallCount === 1
				? staleDeferred.promise
				: freshDeferred.promise;
		});

		const { unhandledRejections } = await withUnhandledRejectionCapture({
			run: async () => {
				const staleNavigation = client.vormaNavigate(
					"/stale-client-loader-failure",
				);
				await Promise.resolve();
				const freshNavigation = client.vormaNavigate("/fresh-wins");
				await Promise.resolve();
				expect(signals[0]?.aborted).toBe(true);

				freshDeferred.resolve(
					createRouteDataResponse({
						loadersData: [],
						title: { dangerousInnerHTML: "Fresh Wins" },
					}),
				);
				await freshNavigation;
				await vi.runAllTimersAsync();

				staleDeferred.resolve(
					createRouteDataResponse({
						matchedPatterns: ["/stale-client-loader-failure"],
						loadersData: [{ value: "stale" }],
						importURLs: [],
						exportKeys: [],
						errorExportKeys: [],
						title: { dangerousInnerHTML: "Stale Should Not Apply" },
					}),
				);
				await staleNavigation;
				await vi.runAllTimersAsync();
			},
		});

		expect(fetchCallCount).toBe(2);
		expect(unhandledRejections).toEqual([]);
		expect(window.location.pathname).toBe("/fresh-wins");
		expect(document.title).toBe("Fresh Wins");
		expect(client.getStatus()).toEqual({
			isNavigating: false,
			isSubmitting: false,
			isRevalidating: false,
		});
	});

	it("preserves authority across mixed prefetch, navigate, submit, and revalidate flows", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const reactAdapter = await import("vorma/react");
		const TypedLink = reactAdapter.makeTypedLink(
			DIST_TEST_VORMA_APP_CONFIG,
			{},
		);
		const { requests } = createAbortAwareFetchRecorder();

		const { result, unhandledRejections } =
			await withUnhandledRejectionCapture({
				run: async () => {
					const prefetchRendered = renderReactTypedLinkForTesting({
						TypedLink,
						linkProps: {
							pattern: "/prefetch-seed/:id",
							params: {
								id: "one",
							},
							hash: "#one",
							prefetch: "intent",
							prefetchDelayMs: 0,
						},
					});
					try {
						prefetchRendered.anchor.dispatchEvent(
							new FocusEvent("focusin", { bubbles: true }),
						);

						await vi.advanceTimersByTimeAsync(1);
						await waitForRequestCount({ requests, count: 1 });
						const prefetchRequest = requests[0]!;

						const navigationFromPrefetch = client.vormaNavigate(
							"/prefetch-seed/one#two",
						);
						await Promise.resolve();
						expect(requests).toHaveLength(1);

						prefetchRequest.deferred.resolve(
							createRouteDataResponse({
								loadersData: [],
								title: {
									dangerousInnerHTML: "Prefetch Seed",
								},
							}),
						);
						await navigationFromPrefetch;
						await vi.runAllTimersAsync();
						expect(window.location.pathname).toBe(
							"/prefetch-seed/one",
						);
						expect(window.location.hash).toBe("#two");

						const firstNavigation =
							client.vormaNavigate("/race-first");
						await waitForRequestCount({ requests, count: 2 });
						const firstNavigationRequest = requests[1]!;

						const secondNavigation =
							client.vormaNavigate("/race-second");
						await waitForRequestCount({ requests, count: 3 });
						const secondNavigationRequest = requests[2]!;

						secondNavigationRequest.deferred.resolve(
							createRouteDataResponse({
								loadersData: [],
								title: {
									dangerousInnerHTML: "Race Second",
								},
							}),
						);
						await secondNavigation;

						firstNavigationRequest.deferred.resolve(
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

						const submitOne = client.submit(
							"/mutation",
							{
								method: "POST",
								body: JSON.stringify({ step: 1 }),
							},
							{ dedupeKey: "model-seq" },
						);
						await waitForRequestCount({ requests, count: 4 });

						const submitTwo = client.submit(
							"/mutation",
							{
								method: "POST",
								body: JSON.stringify({ step: 2 }),
							},
							{ dedupeKey: "model-seq" },
						);
						await waitForRequestCount({ requests, count: 5 });
						const submitTwoRequest = requests[4]!;
						expect(submitTwoRequest.url.pathname).toBe("/mutation");
						expect(submitTwoRequest.init?.method).toBe("POST");

						submitTwoRequest.deferred.resolve(
							new Response(JSON.stringify({ ok: true }), {
								status: 200,
								headers: {
									"Content-Type": "application/json",
								},
							}),
						);

						await waitForRequestCount({ requests, count: 6 });
						const revalidationRequest = requests[5]!;
						expect(revalidationRequest.url.pathname).toBe(
							"/race-second",
						);
						expect(
							revalidationRequest.url.searchParams.get(
								"vorma_json",
							),
						).toBe("1");

						const thirdNavigation =
							client.vormaNavigate("/race-third");
						await waitForRequestCount({ requests, count: 7 });
						const thirdNavigationRequest = requests[6]!;

						thirdNavigationRequest.deferred.resolve(
							createRouteDataResponse({
								loadersData: [],
								title: {
									dangerousInnerHTML: "Race Third",
								},
							}),
						);
						await thirdNavigation;

						revalidationRequest.deferred.resolve(
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
					} finally {
						prefetchRendered.cleanup();
					}
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
		expect(client.getStatus()).toEqual({
			isNavigating: false,
			isSubmitting: false,
			isRevalidating: false,
		});
	});

	it("aborts stale prefetch and revalidation work when a new user navigation starts", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const reactAdapter = await import("vorma/react");
		const TypedLink = reactAdapter.makeTypedLink(
			DIST_TEST_VORMA_APP_CONFIG,
			{},
		);
		const { requests } = createAbortAwareFetchRecorder();

		const { unhandledRejections } = await withUnhandledRejectionCapture({
			run: async () => {
				const prefetched = renderReactTypedLinkForTesting({
					TypedLink,
					linkProps: {
						pattern: "/stale-prefetch",
						prefetch: "intent",
						prefetchDelayMs: 0,
					},
				});
				try {
					prefetched.anchor.dispatchEvent(
						new FocusEvent("focusin", { bubbles: true }),
					);
					await vi.advanceTimersByTimeAsync(1);
					await waitForRequestCount({ requests, count: 1 });

					const revalidatePromise = client.revalidate();
					await waitForRequestCount({ requests, count: 2 });

					const navigatePromise =
						client.vormaNavigate("/fresh-target");
					await waitForRequestCount({ requests, count: 3 });
					await Promise.resolve();

					const prefetchRequest = requests.find(
						(request) => request.url.pathname === "/stale-prefetch",
					);
					const navigationRequest = requests.find(
						(request) => request.url.pathname === "/fresh-target",
					);
					const revalidationRequest = requests.find(
						(request) =>
							request !== prefetchRequest &&
							request !== navigationRequest,
					);
					expect(prefetchRequest).toBeDefined();
					expect(navigationRequest).toBeDefined();
					expect(revalidationRequest).toBeDefined();
					const requestSummaries = requests.map((request) => ({
						pathname: request.url.pathname,
						aborted: (
							request.init?.signal as AbortSignal | undefined
						)?.aborted,
					}));

					const prefetchSignal = prefetchRequest!.init?.signal as
						| AbortSignal
						| undefined;
					const revalidateSignal = revalidationRequest!.init
						?.signal as AbortSignal | undefined;
					const navigationSignal = navigationRequest!.init?.signal as
						| AbortSignal
						| undefined;
					expect(requestSummaries).toEqual(
						expect.arrayContaining([
							expect.objectContaining({
								pathname: "/stale-prefetch",
							}),
							expect.objectContaining({
								pathname: "/fresh-target",
							}),
						]),
					);
					expect(requestSummaries).toContainEqual({
						pathname: "/stale-prefetch",
						aborted: true,
					});
					expect(prefetchSignal?.aborted).toBe(true);
					expect(revalidateSignal?.aborted).toBe(true);
					expect(navigationSignal?.aborted).toBe(false);

					navigationRequest!.deferred.resolve(
						createRouteDataResponse({
							loadersData: [],
							title: {
								dangerousInnerHTML: "Fresh Target",
							},
						}),
					);

					await navigatePromise;
					await revalidatePromise;
					await vi.runAllTimersAsync();
				} finally {
					prefetched.cleanup();
				}
			},
		});

		expect(unhandledRejections).toEqual([]);
		expect(window.location.pathname).toBe("/fresh-target");
		expect(document.title).toBe("Fresh Target");
		expect(client.getStatus()).toEqual({
			isNavigating: false,
			isSubmitting: false,
			isRevalidating: false,
		});
	});

	it("clearAll aborts in-flight prefetch and revalidation work, then returns idle", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const reactAdapter = await import("vorma/react");
		const TypedLink = reactAdapter.makeTypedLink(
			DIST_TEST_VORMA_APP_CONFIG,
			{},
		);
		const { requests } = createAbortAwareFetchRecorder();

		const prefetched = renderReactTypedLinkForTesting({
			TypedLink,
			linkProps: {
				pattern: "/clear-all-prefetch",
				prefetch: "intent",
				prefetchDelayMs: 0,
			},
		});
		try {
			prefetched.anchor.dispatchEvent(
				new FocusEvent("focusin", { bubbles: true }),
			);
			await vi.advanceTimersByTimeAsync(1);
			await waitForRequestCount({ requests, count: 1 });

			const revalidatePromise = client.revalidate();
			await waitForRequestCount({ requests, count: 2 });
			expect(client.getStatus()).toEqual({
				isNavigating: false,
				isSubmitting: false,
				isRevalidating: true,
			});

			clearAllNavigationStateForTesting();

			const prefetchSignal = requests[0]!.init?.signal as
				| AbortSignal
				| undefined;
			const revalidateSignal = requests[1]!.init?.signal as
				| AbortSignal
				| undefined;
			expect(prefetchSignal?.aborted).toBe(true);
			expect(revalidateSignal?.aborted).toBe(true);

			await revalidatePromise;
			await vi.runAllTimersAsync();
		} finally {
			prefetched.cleanup();
		}

		expect(window.location.pathname).toBe("/");
		expect(client.getStatus()).toEqual({
			isNavigating: false,
			isSubmitting: false,
			isRevalidating: false,
		});
	});

	it("clearAll prevents late side effects from prefetches whose fetch ignores abort", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const reactAdapter = await import("vorma/react");
		const TypedLink = reactAdapter.makeTypedLink(
			DIST_TEST_VORMA_APP_CONFIG,
			{},
		);
		const initialTitle = document.title;
		const fetchCall = createDeferredFetchCall();
		const requestAnimationFrameSpy = vi.spyOn(
			window,
			"requestAnimationFrame",
		);
		vi.spyOn(window, "fetch").mockImplementation(fetchCall.mock);

		const { unhandledRejections } = await withUnhandledRejectionCapture({
			run: async () => {
				const prefetched = renderReactTypedLinkForTesting({
					TypedLink,
					linkProps: {
						pattern: "/clear-prefetch-late",
						prefetch: "intent",
						prefetchDelayMs: 0,
					},
				});
				try {
					prefetched.anchor.dispatchEvent(
						new FocusEvent("focusin", { bubbles: true }),
					);
					await vi.advanceTimersByTimeAsync(1);
					expect(fetchCall.getSignal()?.aborted).toBe(false);

					clearAllNavigationStateForTesting();
					expect(fetchCall.getSignal()?.aborted).toBe(true);

					const requestAnimationFrameCallCountBeforeResolve =
						requestAnimationFrameSpy.mock.calls.length;
					fetchCall.deferred.resolve(
						createRouteDataResponse({
							title: {
								dangerousInnerHTML: "Late Prefetch Title",
							},
							cssBundles: ["/late-prefetch.css"],
						}),
					);
					await vi.advanceTimersByTimeAsync(32);
					await vi.runAllTimersAsync();

					expect(requestAnimationFrameSpy).toHaveBeenCalledTimes(
						requestAnimationFrameCallCountBeforeResolve,
					);
					expect(
						document.head.querySelector(
							'link[data-vorma-css-bundle="/late-prefetch.css"]',
						),
					).toBeNull();
				} finally {
					prefetched.cleanup();
				}
			},
		});

		expect(unhandledRejections).toEqual([]);
		expect(window.location.pathname).toBe("/");
		expect(document.title).toBe(initialTitle);
		expect(client.getStatus()).toEqual({
			isNavigating: false,
			isSubmitting: false,
			isRevalidating: false,
		});
	});

	it("preserves last-navigation authority across a seeded mixed operation sequence", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const reactAdapter = await import("vorma/react");
		const TypedLink = reactAdapter.makeTypedLink(
			DIST_TEST_VORMA_APP_CONFIG,
			{},
		);
		const { requests } = createAbortAwareFetchRecorder();
		const modelSeed = 20260212;
		const { operations, finalNavigationHref } =
			buildGeneratedNavigationModelSequence({
				seed: modelSeed,
			});

		const { unhandledRejections } = await withUnhandledRejectionCapture({
			run: async () => {
				const navigationPromises: Array<
					ReturnType<typeof client.vormaNavigate>
				> = [];
				const prefetchCleanups: Array<() => void> = [];
				try {
					for (const operation of operations) {
						if (operation.kind === "prefetch") {
							const operationURL = new URL(
								operation.href,
								window.location.origin,
							);
							const prefetched = renderReactTypedLinkForTesting({
								TypedLink,
								linkProps: {
									pattern: operationURL.pathname,
									hash: operationURL.hash || undefined,
									prefetch: "intent",
									prefetchDelayMs: 0,
								},
							});
							prefetchCleanups.push(prefetched.cleanup);
							prefetched.anchor.dispatchEvent(
								new FocusEvent("focusin", { bubbles: true }),
							);
							await vi.advanceTimersByTimeAsync(1);
							continue;
						}
						navigationPromises.push(
							client.vormaNavigate(operation.href),
						);
						await Promise.resolve();
					}

					await vi.advanceTimersByTimeAsync(20);
					await Promise.resolve();
					expect(requests.length).toBeGreaterThan(0);

					const resolveOrder = shuffledIndices({
						length: requests.length,
						seed: modelSeed ^ 0x5a5a,
					});
					for (const requestIndex of resolveOrder) {
						const request = requests[requestIndex];
						if (!request) {
							throw new Error(
								`Missing mixed-sequence request at index ${requestIndex}`,
							);
						}
						request.deferred.resolve(
							createRouteDataResponse({
								loadersData: [],
								title: {
									dangerousInnerHTML: `Model Sequence ${request.url.pathname}`,
								},
							}),
						);
						await Promise.resolve();
					}

					await Promise.all(navigationPromises);
				} finally {
					for (const cleanup of prefetchCleanups) {
						cleanup();
					}
				}
			},
		});

		const expectedFinalURL = new URL(
			finalNavigationHref,
			window.location.origin,
		);

		expect(unhandledRejections).toEqual([]);
		expect(window.location.pathname).toBe(expectedFinalURL.pathname);
		expect(window.location.hash).toBe(expectedFinalURL.hash);
		expect(document.title).toBe(
			`Model Sequence ${expectedFinalURL.pathname}`,
		);
		expect(client.getStatus()).toEqual({
			isNavigating: false,
			isSubmitting: false,
			isRevalidating: false,
		});
	});

	it("surfaces explicit terminal outcomes for settled submit operations", async () => {
		await initializeDistRuntimeStateForAdapters();
		const client = await import("vorma/client");
		const { requests } = createAbortAwareFetchRecorder();

		const { result, unhandledRejections } =
			await withUnhandledRejectionCapture({
				run: async () => {
					const firstSubmit = client.submit(
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

					const secondSubmit = client.submit(
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
					requests[1]?.deferred.resolve(
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

					const failedSubmit = client.submit(
						"/terminal-submit-failure",
						{
							method: "POST",
						},
						{
							revalidate: false,
						},
					);
					await waitForRequestCount({ requests, count: 3 });
					requests[2]?.deferred.resolve(
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
		expect(client.getStatus()).toEqual({
			isNavigating: false,
			isSubmitting: false,
			isRevalidating: false,
		});
	});
});
