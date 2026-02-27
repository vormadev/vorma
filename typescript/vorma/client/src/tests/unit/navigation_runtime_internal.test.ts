import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
	createPatternRegistry,
	registerPattern,
} from "vorma/kit/matcher/register";
import type {
	NavigationEntry,
	NavigationLanes,
	NavigationOutcome,
	SubmitExecutionContext,
} from "../../../src/runtime.ts";
import * as renderRuntimeModule from "../../runtime.ts";
import {
	__vormaClientGlobal,
	addBuildIDListener,
	createInitialNavigationRuntimeEngineState,
	createNavigationRuntime,
	deleteNavigationFromNavigationLanes,
	executeSubmitRuntime,
	fetchRouteData,
	findPartialMatchesOnClient,
	getMatchedPatternsOrThrow,
	handleNavigationOutcome,
	matchNavigationLaneByTargetURL,
	processSuccessfulNavigationRuntime,
	reduceNavigationRuntimeEngineEvent,
	registerClientLoaderPattern,
	setNavigationStateAccess,
	setupClientLoaders,
	VORMA_SYMBOL,
} from "../../runtime.ts";

const TEST_VORMA_APP_CONFIG = {
	actionsRouterMountRoot: "/api/",
	actionsDynamicRune: ":",
	actionsSplatRune: "*",
	loadersDynamicRune: ":",
	loadersSplatRune: "*",
	loadersExplicitIndexSegmentIdentifier: "_index",
};

function createEntry(props: {
	targetUrl: string;
	type: NavigationEntry["type"];
	intent: NavigationEntry["intent"];
}): NavigationEntry {
	return {
		operationID: 1,
		control: {
			abortController: new AbortController(),
			promise: Promise.resolve({ type: "aborted" as const }),
		},
		type: props.type,
		intent: props.intent,
		phase: "fetching",
		startTime: Date.now(),
		targetUrl: props.targetUrl,
		originUrl: window.location.href,
	};
}

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

function createMatch(
	pattern: string,
	props: {
		normalizedSegments?: Array<{ segType: string; normalizedVal: string }>;
		lastSegType?: string;
	} = {},
) {
	return {
		registeredPattern: {
			originalPattern: pattern,
			normalizedSegments: props.normalizedSegments ?? [],
			lastSegType: props.lastSegType ?? "static",
		},
		params: {},
		splatValues: [],
	};
}

function createSuccessNavigationOutcome(
	overrides: {
		preloadPlan?: Extract<
			NavigationOutcome,
			{ type: "success" }
		>["preloadPlan"];
		waitFnPromise?: Promise<{
			data: Array<unknown>;
			errorMessage?: string;
		}>;
		responseBuildID?: string;
		props?: Partial<
			Extract<NavigationOutcome, { type: "success" }>["props"]
		>;
	} = {},
): Extract<NavigationOutcome, { type: "success" }> {
	const defaultProps: Extract<
		NavigationOutcome,
		{ type: "success" }
	>["props"] = {
		href: window.location.href,
		navigationType: "browserHistory",
	};

	return {
		type: "success",
		response: new Response(JSON.stringify({ ok: true }), {
			status: 200,
			headers: {
				"Content-Type": "application/json",
				"X-Vorma-Build-Id": overrides.responseBuildID ?? "1",
			},
		}),
		json: {
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
		},
		preloadPlan: overrides.preloadPlan ?? {
			moduleDependencies: [],
			cssBundles: [],
		},
		waitFnPromise: overrides.waitFnPromise ?? Promise.resolve({ data: [] }),
		props: {
			...defaultProps,
			...overrides.props,
		},
	};
}

function createAbortAwareNeverResolvingFetchSpy() {
	return vi.spyOn(window, "fetch").mockImplementation(
		(_url, init) =>
			new Promise<Response>((_resolve, reject) => {
				const signal = (init as RequestInit | undefined)?.signal as
					| AbortSignal
					| undefined;
				const abortError = new Error("Aborted");
				abortError.name = "AbortError";
				if (signal?.aborted) {
					reject(abortError);
					return;
				}

				signal?.addEventListener(
					"abort",
					() => {
						const nextAbortError = new Error("Aborted");
						nextAbortError.name = "AbortError";
						reject(nextAbortError);
					},
					{ once: true },
				);
			}),
	);
}

function createDeferred<T>() {
	let resolve: (value: T) => void = () => {};
	let reject: (reason?: unknown) => void = () => {};
	const promise = new Promise<T>((res, rej) => {
		resolve = res;
		reject = rej;
	});
	return { promise, resolve, reject };
}

function createSubmitResponse(props: {
	status?: number;
	buildID?: string;
	url?: string;
	json: () => Promise<unknown>;
}): Response {
	const status = props.status ?? 200;
	const headers = new Headers();
	if (props.buildID !== undefined) {
		headers.set("X-Vorma-Build-Id", props.buildID);
	}

	return {
		status,
		ok: status >= 200 && status < 300,
		headers,
		redirected: false,
		url: props.url ?? window.location.href,
		json: props.json,
	} as unknown as Response;
}

beforeEach(() => {
	window.history.replaceState({}, "", "/");
	vi.spyOn(console, "error").mockImplementation(() => {});
	installVormaGlobal();
	setNavigationStateAccess({
		navigate: async () => ({ didNavigate: true }),
		removeNavigation: () => {},
		getNavigations: () => new Map(),
	});
});

afterEach(() => {
	vi.restoreAllMocks();
	delete (globalThis as any)[VORMA_SYMBOL];
});

describe("runtime engine start-event planning", () => {
	it("routes start events into revalidation/hash/noop/single-pass terminal behavior", () => {
		let state = createInitialNavigationRuntimeEngineState();
		const revalidationTransition = reduceNavigationRuntimeEngineEvent({
			state,
			event: {
				type: "start",
				lane: "revalidate",
				navigationProps: {
					href: "/revalidate",
					navigationType: "revalidation",
				},
				targetUrl: "http://localhost:3000/revalidate",
				currentHref: "http://localhost:3000/current",
			},
		});
		expect(revalidationTransition.terminalResult).toEqual({
			type: "run-revalidation-lane",
		});
		expect(revalidationTransition.commands).toEqual([
			{
				type: "fetch",
				lane: "revalidate",
				targetUrl: "http://localhost:3000/revalidate",
				operationID: null,
			},
		]);
		state = revalidationTransition.state;

		window.history.replaceState({}, "", "/same-doc#~");
		const hashTransition = reduceNavigationRuntimeEngineEvent({
			state,
			event: {
				type: "start",
				lane: "navigate",
				navigationProps: {
					href: "/same-doc#details",
					navigationType: "userNavigation",
				},
				targetUrl: "http://localhost:3000/same-doc#details",
				currentHref: window.location.href,
			},
		});
		expect(hashTransition.terminalResult).toEqual({
			type: "return-without-fetch",
			didNavigate: true,
		});
		expect(hashTransition.commands).toEqual([
			{ type: "clear-queued-revalidation-request" },
			{
				type: "commit-same-document-hash-navigation-without-fetch",
				navigationProps: {
					href: "/same-doc#details",
					navigationType: "userNavigation",
				},
				targetUrl: "http://localhost:3000/same-doc#details",
			},
			{
				type: "history-write",
				lane: "navigate",
				targetUrl: "http://localhost:3000/same-doc#details",
				operationID: null,
			},
		]);
		state = hashTransition.state;

		const noopTransition = reduceNavigationRuntimeEngineEvent({
			state,
			event: {
				type: "start",
				lane: "navigate",
				navigationProps: {
					href: "/same-doc#%7E",
					navigationType: "userNavigation",
				},
				targetUrl: "http://localhost:3000/same-doc#%7E",
				currentHref: window.location.href,
			},
		});
		expect(noopTransition.terminalResult).toEqual({
			type: "return-without-fetch",
			didNavigate: false,
		});
		expect(noopTransition.commands).toEqual([
			{ type: "clear-queued-revalidation-request" },
		]);

		const singlePassTransition = reduceNavigationRuntimeEngineEvent({
			state: noopTransition.state,
			event: {
				type: "start",
				lane: "navigate",
				navigationProps: {
					href: "/different-target",
					navigationType: "userNavigation",
				},
				targetUrl: "http://localhost:3000/different-target",
				currentHref: window.location.href,
			},
		});
		expect(singlePassTransition.terminalResult).toEqual({
			type: "run-single-pass",
		});
		expect(singlePassTransition.commands).toEqual([
			{ type: "clear-queued-revalidation-request" },
			{
				type: "fetch",
				lane: "navigate",
				targetUrl: "http://localhost:3000/different-target",
				operationID: null,
			},
		]);
	});
});

describe("navigation bookkeeping key aliasing", () => {
	it("finds active navigation by same data target when hash differs", () => {
		const active = createEntry({
			targetUrl: "http://localhost:3000/alias#first",
			type: "userNavigation",
			intent: "navigate",
		});
		const slots: NavigationLanes = {
			active: active,
			prefetch: new Map(),
			revalidation: null,
		};

		const found = matchNavigationLaneByTargetURL({
			lanes: slots,
			targetUrl: "http://localhost:3000/alias#second",
		})?.entry;
		expect(found).toBe(active);
	});

	it("does not alias active navigation when search params differ", () => {
		const active = createEntry({
			targetUrl: "http://localhost:3000/alias?tab=a#first",
			type: "userNavigation",
			intent: "navigate",
		});
		const slots: NavigationLanes = {
			active: active,
			prefetch: new Map(),
			revalidation: null,
		};

		const found = matchNavigationLaneByTargetURL({
			lanes: slots,
			targetUrl: "http://localhost:3000/alias?tab=b#first",
		})?.entry;
		expect(found).toBeUndefined();
	});

	it("deletes prefetch entry by same data target when hash differs", () => {
		const prefetch = createEntry({
			targetUrl: "http://localhost:3000/prefetch#first",
			type: "prefetch",
			intent: "none",
		});
		const slots: NavigationLanes = {
			active: null,
			prefetch: new Map([[prefetch.targetUrl, prefetch]]),
			revalidation: null,
		};
		const onStatusRelevantChange = vi.fn();

		const deleted = deleteNavigationFromNavigationLanes({
			lanes: slots,
			targetUrl: "http://localhost:3000/prefetch#second",
			onStatusRelevantChange,
		});

		expect(deleted).toBe(true);
		expect(slots.prefetch.size).toBe(0);
		expect(onStatusRelevantChange).not.toHaveBeenCalled();
	});

	it("does not delete prefetch entry when search params differ", () => {
		const prefetch = createEntry({
			targetUrl: "http://localhost:3000/prefetch?tab=a#first",
			type: "prefetch",
			intent: "none",
		});
		const slots: NavigationLanes = {
			active: null,
			prefetch: new Map([[prefetch.targetUrl, prefetch]]),
			revalidation: null,
		};
		const onStatusRelevantChange = vi.fn();

		const deleted = deleteNavigationFromNavigationLanes({
			lanes: slots,
			targetUrl: "http://localhost:3000/prefetch?tab=b#first",
			onStatusRelevantChange,
		});

		expect(deleted).toBe(false);
		expect(slots.prefetch.size).toBe(1);
		expect(onStatusRelevantChange).not.toHaveBeenCalled();
	});
});

describe("navigation runtime bookkeeping lifecycle", () => {
	it("tracks active, prefetch, and revalidation entries and clears them atomically", async () => {
		const fetchSpy = vi.spyOn(window, "fetch").mockImplementation(
			(_url, init) =>
				new Promise<Response>((_resolve, reject) => {
					const signal = (init as RequestInit | undefined)?.signal as
						| AbortSignal
						| undefined;
					const abortError = new Error("Aborted");
					abortError.name = "AbortError";
					if (signal?.aborted) {
						reject(abortError);
						return;
					}

					signal?.addEventListener(
						"abort",
						() => {
							const abortError = new Error("Aborted");
							abortError.name = "AbortError";
							reject(abortError);
						},
						{ once: true },
					);
				}),
		);
		try {
			const runtime = createNavigationRuntime();
			const userControl = runtime.beginNavigation({
				href: "/user-target",
				navigationType: "browserHistory",
			});
			const prefetchControl = runtime.beginNavigation({
				href: "/prefetch-target",
				navigationType: "prefetch",
			});
			const revalidationControl = runtime.beginNavigation({
				href: "/ignored-by-revalidation",
				navigationType: "revalidation",
			});
			const userPromise = userControl.promise.catch((error) => error);
			const prefetchPromise = prefetchControl.promise.catch(
				(error) => error,
			);
			const revalidationPromise = revalidationControl.promise.catch(
				(error) => error,
			);
			const submitPromise = runtime.submit(
				"/api/clear-all",
				{ method: "POST" },
				{
					dedupeKey: "clear-all",
					revalidate: false,
				},
			);

			expect(runtime.getNavigationsSize()).toBe(3);
			const navigations = runtime.getNavigations();
			expect(
				navigations.get(
					new URL("/user-target", window.location.href).href,
				)?.type,
			).toBe("browserHistory");
			expect(
				navigations.get(
					new URL("/prefetch-target", window.location.href).href,
				)?.type,
			).toBe("prefetch");
			expect(navigations.get(window.location.href)?.type).toBe(
				"revalidation",
			);
			expect(runtime._submissions.size).toBe(1);

			runtime.clearAll();

			const [userError, prefetchError, revalidationError] =
				await Promise.all([
					userPromise,
					prefetchPromise,
					revalidationPromise,
				]);
			expect((userError as Error).name).toBe("AbortError");
			expect((prefetchError as Error).name).toBe("AbortError");
			expect((revalidationError as Error).name).toBe("AbortError");
			await expect(submitPromise).resolves.toEqual({
				success: false,
				error: "Aborted",
			});
			expect(runtime.getNavigationsSize()).toBe(0);
			expect(runtime.getNavigations().size).toBe(0);
			expect(runtime._submissions.size).toBe(0);
			expect(fetchSpy).toHaveBeenCalledTimes(4);
		} finally {
			fetchSpy.mockRestore();
		}
	});

	it("returns an already-aborted control when prefetch targets the current location", async () => {
		const fetchSpy = vi.spyOn(window, "fetch");
		try {
			window.history.replaceState({}, "", "/prefetch-current-location");
			const runtime = createNavigationRuntime();

			const control = runtime.beginNavigation({
				href: "/prefetch-current-location#ignored-hash",
				navigationType: "prefetch",
			});

			expect(control.abortController?.signal.aborted).toBe(true);
			await expect(control.promise).resolves.toEqual({ type: "aborted" });
			expect(runtime.getNavigationsSize()).toBe(0);
			expect(fetchSpy).not.toHaveBeenCalled();
		} finally {
			fetchSpy.mockRestore();
		}
	});

	it("reuses active navigation control for same-target prefetch requests", async () => {
		const fetchSpy = createAbortAwareNeverResolvingFetchSpy();
		try {
			const runtime = createNavigationRuntime();
			const activeControl = runtime.beginNavigation({
				href: "/prefetch-active-alias#first",
				navigationType: "browserHistory",
			});
			const activePromise = activeControl.promise.catch((error) => error);

			const prefetchControl = runtime.beginNavigation({
				href: "/prefetch-active-alias#second",
				navigationType: "prefetch",
			});

			expect(prefetchControl).toBe(activeControl);
			expect(runtime.getNavigationsSize()).toBe(1);
			expect(
				runtime.getNavigation(
					new URL(
						"/prefetch-active-alias#first",
						window.location.href,
					).href,
				)?.type,
			).toBe("browserHistory");

			runtime.clearAll();
			await expect(activePromise).resolves.toMatchObject({
				name: "AbortError",
			});
		} finally {
			fetchSpy.mockRestore();
		}
	});

	it("reuses pending revalidation control for same-target prefetch requests", async () => {
		const fetchSpy = createAbortAwareNeverResolvingFetchSpy();
		try {
			window.history.replaceState({}, "", "/prefetch-revalidation");

			const runtime = createNavigationRuntime();
			const revalidationControl = runtime.beginNavigation({
				href: "/ignored-by-revalidation",
				navigationType: "revalidation",
			});
			const revalidationPromise = revalidationControl.promise.catch(
				(error) => error,
			);

			const prefetchControl = runtime.beginNavigation({
				href: "/prefetch-revalidation#fragment",
				navigationType: "prefetch",
			});

			expect(prefetchControl).toBe(revalidationControl);
			expect(runtime.getNavigationsSize()).toBe(1);
			expect(runtime.getNavigation(window.location.href)?.type).toBe(
				"revalidation",
			);

			runtime.clearAll();
			await expect(revalidationPromise).resolves.toMatchObject({
				name: "AbortError",
			});
		} finally {
			fetchSpy.mockRestore();
		}
	});

	it("reuses same-target prefetch control for browser-history navigation and promotes it to active", async () => {
		const fetchSpy = createAbortAwareNeverResolvingFetchSpy();
		try {
			const runtime = createNavigationRuntime();
			const prefetchControl = runtime.beginNavigation({
				href: "/browser-history-prefetch#first",
				navigationType: "prefetch",
			});
			const prefetchPromise = prefetchControl.promise.catch(
				(error) => error,
			);

			const browserHistoryControl = runtime.beginNavigation({
				href: "/browser-history-prefetch#second",
				navigationType: "browserHistory",
			});

			expect(browserHistoryControl).toBe(prefetchControl);
			expect(runtime.getNavigationsSize()).toBe(1);
			const promotedEntry = runtime.getNavigation(
				new URL(
					"/browser-history-prefetch#second",
					window.location.href,
				).href,
			);
			expect(promotedEntry?.type).toBe("browserHistory");
			expect(promotedEntry?.intent).toBe("navigate");

			runtime.clearAll();
			await expect(prefetchPromise).resolves.toMatchObject({
				name: "AbortError",
			});
		} finally {
			fetchSpy.mockRestore();
		}
	});

	it("reuses same-target revalidation control for redirect navigation and promotes it to active", async () => {
		const fetchSpy = createAbortAwareNeverResolvingFetchSpy();
		try {
			window.history.replaceState({}, "", "/redirect-revalidation");

			const runtime = createNavigationRuntime();
			const revalidationControl = runtime.beginNavigation({
				href: "/ignored-by-revalidation",
				navigationType: "revalidation",
			});
			const revalidationPromise = revalidationControl.promise.catch(
				(error) => error,
			);

			const redirectControl = runtime.beginNavigation({
				href: "/redirect-revalidation#target",
				navigationType: "redirect",
			});

			expect(redirectControl).toBe(revalidationControl);
			expect(runtime.getNavigationsSize()).toBe(1);
			const promotedEntry = runtime.getNavigation(
				new URL("/redirect-revalidation#target", window.location.href)
					.href,
			);
			expect(promotedEntry?.type).toBe("redirect");
			expect(promotedEntry?.intent).toBe("navigate");

			runtime.clearAll();
			await expect(revalidationPromise).resolves.toMatchObject({
				name: "AbortError",
			});
		} finally {
			fetchSpy.mockRestore();
		}
	});

	it("aborts stale prefetch and revalidation work when browser-history navigation starts to a different target", async () => {
		const fetchSpy = createAbortAwareNeverResolvingFetchSpy();
		try {
			window.history.replaceState({}, "", "/browser-history-matrix-base");
			const runtime = createNavigationRuntime();
			const stalePrefetchControl = runtime.beginNavigation({
				href: "/browser-history-stale-prefetch",
				navigationType: "prefetch",
			});
			const staleRevalidationControl = runtime.beginNavigation({
				href: "/ignored-by-revalidation",
				navigationType: "revalidation",
			});

			runtime.beginNavigation({
				href: "/browser-history-fresh-target",
				navigationType: "browserHistory",
			});

			expect(stalePrefetchControl.abortController?.signal.aborted).toBe(
				true,
			);
			expect(
				staleRevalidationControl.abortController?.signal.aborted,
			).toBe(true);
			expect(runtime.getNavigationsSize()).toBe(1);
			expect(
				runtime.getNavigation(
					new URL(
						"/browser-history-fresh-target",
						window.location.href,
					).href,
				)?.type,
			).toBe("browserHistory");
		} finally {
			fetchSpy.mockRestore();
		}
	});

	it("removeNavigation aborts and removes entries addressed by hash-alias key", async () => {
		const fetchSpy = vi.spyOn(window, "fetch").mockImplementation(
			(_url, init) =>
				new Promise<Response>((_resolve, reject) => {
					const signal = (init as RequestInit | undefined)?.signal as
						| AbortSignal
						| undefined;
					const abortError = new Error("Aborted");
					abortError.name = "AbortError";
					if (signal?.aborted) {
						reject(abortError);
						return;
					}

					signal?.addEventListener(
						"abort",
						() => {
							const abortError = new Error("Aborted");
							abortError.name = "AbortError";
							reject(abortError);
						},
						{ once: true },
					);
				}),
		);
		try {
			const runtime = createNavigationRuntime();
			const control = runtime.beginNavigation({
				href: "/remove-me#first",
				navigationType: "browserHistory",
			});
			const aliasHref = new URL("/remove-me#second", window.location.href)
				.href;

			const entryBeforeRemoval = runtime.getNavigation(aliasHref);
			expect(entryBeforeRemoval).toBeDefined();

			runtime.removeNavigation(aliasHref);

			expect(
				entryBeforeRemoval?.control.abortController?.signal.aborted,
			).toBe(true);
			expect(runtime.getNavigation(aliasHref)).toBeUndefined();
			await expect(control.promise).rejects.toMatchObject({
				name: "AbortError",
			});
		} finally {
			fetchSpy.mockRestore();
		}
	});

	it("keeps a newer same-target active entry when an older active fetch rejects", async () => {
		const fetchSpy = createAbortAwareNeverResolvingFetchSpy();
		try {
			const runtime = createNavigationRuntime();
			const firstControl = runtime.beginNavigation({
				href: "/stale-active-entry",
				navigationType: "browserHistory",
			});
			const firstControlPromise = firstControl.promise.catch(
				(error) => error,
			);

			const targetUrl = new URL(
				"/stale-active-entry",
				window.location.href,
			).href;
			runtime.removeNavigation(targetUrl);

			const secondControl = runtime.beginNavigation({
				href: "/stale-active-entry",
				navigationType: "browserHistory",
			});
			const secondControlPromise = secondControl.promise.catch(
				(error) => error,
			);
			const secondEntry = runtime.getNavigation(targetUrl);
			expect(secondEntry).toBeDefined();
			if (!secondEntry) return;

			await expect(firstControlPromise).resolves.toMatchObject({
				name: "AbortError",
			});
			expect(runtime.getNavigation(targetUrl)).toBe(secondEntry);

			runtime.clearAll();
			await expect(secondControlPromise).resolves.toMatchObject({
				name: "AbortError",
			});
		} finally {
			fetchSpy.mockRestore();
		}
	});

	it("does not delete a newer same-target entry when stale navigate rejection resolves", async () => {
		const fetchSpy = createAbortAwareNeverResolvingFetchSpy();
		try {
			const runtime = createNavigationRuntime();
			const firstNavigatePromise = runtime.navigate({
				href: "/stale-navigate-rejection",
				navigationType: "browserHistory",
			});

			const targetUrl = new URL(
				"/stale-navigate-rejection",
				window.location.href,
			).href;
			runtime.removeNavigation(targetUrl);

			const secondControl = runtime.beginNavigation({
				href: "/stale-navigate-rejection",
				navigationType: "browserHistory",
			});
			const secondControlPromise = secondControl.promise.catch(
				(error) => error,
			);
			const secondEntry = runtime.getNavigation(targetUrl);
			expect(secondEntry).toBeDefined();
			if (!secondEntry) return;

			await expect(firstNavigatePromise).resolves.toEqual({
				didNavigate: false,
			});
			expect(runtime.getNavigation(targetUrl)).toBe(secondEntry);
			expect(
				runtime
					.getDebugJournal()
					.some(
						(journalEntry) =>
							journalEntry.reason ===
								"navigate_promise_rejected" &&
							journalEntry.toState === "failed" &&
							journalEntry.operationID === null &&
							journalEntry.targetUrl === targetUrl,
					),
			).toBe(true);

			runtime.clearAll();
			await expect(secondControlPromise).resolves.toMatchObject({
				name: "AbortError",
			});
		} finally {
			fetchSpy.mockRestore();
		}
	});

	it("deletes current same-target entry on navigate rejection when reused prefetch ownership still matches", async () => {
		const deferred = createDeferred<Response>();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockImplementation(() => deferred.promise);
		try {
			const runtime = createNavigationRuntime();
			const targetHref = "/navigate-catch-owns-prefetch";
			const targetUrl = new URL(targetHref, window.location.href).href;

			runtime.beginNavigation({
				href: targetHref,
				navigationType: "prefetch",
			});
			const navigatePromise = runtime.navigate({
				href: targetHref,
				navigationType: "userNavigation",
			});

			const ownedEntryBeforeReject = runtime.getNavigation(targetUrl);
			expect(ownedEntryBeforeReject).toBeDefined();
			expect(ownedEntryBeforeReject?.type).toBe("userNavigation");

			deferred.reject(new Error("network failed"));

			await expect(navigatePromise).resolves.toEqual({
				didNavigate: false,
			});
			expect(runtime.getNavigation(targetUrl)).toBeUndefined();
		} finally {
			fetchSpy.mockRestore();
		}
	});

	it("keeps a newer same-target prefetch entry when an older prefetch rejects", async () => {
		const fetchSpy = createAbortAwareNeverResolvingFetchSpy();
		try {
			const runtime = createNavigationRuntime();
			const firstControl = runtime.beginNavigation({
				href: "/stale-prefetch-entry",
				navigationType: "prefetch",
			});
			const firstControlPromise = firstControl.promise.catch(
				(error) => error,
			);

			const targetUrl = new URL(
				"/stale-prefetch-entry",
				window.location.href,
			).href;
			runtime.removeNavigation(targetUrl);

			const secondControl = runtime.beginNavigation({
				href: "/stale-prefetch-entry",
				navigationType: "prefetch",
			});
			const secondControlPromise = secondControl.promise.catch(
				(error) => error,
			);
			const secondEntry = runtime.getNavigation(targetUrl);
			expect(secondEntry).toBeDefined();
			if (!secondEntry) return;
			expect(secondEntry.type).toBe("prefetch");

			await expect(firstControlPromise).resolves.toMatchObject({
				name: "AbortError",
			});
			expect(runtime.getNavigation(targetUrl)).toBe(secondEntry);

			runtime.clearAll();
			await expect(secondControlPromise).resolves.toMatchObject({
				name: "AbortError",
			});
		} finally {
			fetchSpy.mockRestore();
		}
	});

	it("keeps a newer same-target revalidation entry when an older revalidation rejects", async () => {
		const fetchSpy = createAbortAwareNeverResolvingFetchSpy();
		try {
			window.history.replaceState({}, "", "/stale-revalidation-entry");
			const runtime = createNavigationRuntime();

			const firstControl = runtime.beginNavigation({
				href: "/ignored-by-revalidation",
				navigationType: "revalidation",
			});
			const firstControlPromise = firstControl.promise.catch(
				(error) => error,
			);

			const targetUrl = window.location.href;
			runtime.removeNavigation(targetUrl);

			const secondControl = runtime.beginNavigation({
				href: "/ignored-by-revalidation",
				navigationType: "revalidation",
			});
			const secondControlPromise = secondControl.promise.catch(
				(error) => error,
			);
			const secondEntry = runtime.getNavigation(targetUrl);
			expect(secondEntry).toBeDefined();
			if (!secondEntry) return;
			expect(secondEntry.type).toBe("revalidation");

			await expect(firstControlPromise).resolves.toMatchObject({
				name: "AbortError",
			});
			expect(runtime.getNavigation(targetUrl)).toBe(secondEntry);

			runtime.clearAll();
			await expect(secondControlPromise).resolves.toMatchObject({
				name: "AbortError",
			});
		} finally {
			fetchSpy.mockRestore();
		}
	});

	it("removeNavigation no-ops when target does not exist", () => {
		const runtime = createNavigationRuntime();

		expect(runtime.getNavigationsSize()).toBe(0);
		expect(() =>
			runtime.removeNavigation(
				new URL("/does-not-exist", window.location.href).href,
			),
		).not.toThrow();
		expect(runtime.getNavigationsSize()).toBe(0);
	});

	it("records debug-journal navigation transitions with operation IDs and reasons", async () => {
		vi.spyOn(window, "fetch").mockResolvedValue(
			new Response(
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
					outermostServerError: undefined,
					outermostServerErrorIdx: undefined,
					title: undefined,
					metaHeadEls: undefined,
					restHeadEls: undefined,
				}),
				{
					status: 200,
					headers: {
						"Content-Type": "application/json",
						"X-Vorma-Build-Id": "1",
					},
				},
			),
		);
		const runtime = createNavigationRuntime();

		await runtime.navigate({
			href: "/journal-nav",
			navigationType: "userNavigation",
		});

		const journal = runtime.getDebugJournal();
		const startedEntry = journal.find(
			(entry) => entry.reason === "started_by_userNavigation",
		);
		expect(startedEntry).toBeDefined();
		expect(startedEntry?.operationID).toEqual(expect.any(Number));
		expect(startedEntry?.lane).toBe("active");
		expect(
			journal.some(
				(entry) =>
					entry.reason === "process_successful_navigation_waiting",
			),
		).toBe(true);
		expect(
			journal.some(
				(entry) => entry.reason === "successful_navigation_cleanup",
			),
		).toBe(true);
	});

	it("records submission dedupe causal links in debug journal", async () => {
		const firstSubmitRequest = createDeferred<Response>();
		const secondSubmitRequest = createDeferred<Response>();
		let submitRequestCount = 0;
		vi.spyOn(window, "fetch").mockImplementation(() => {
			submitRequestCount += 1;
			if (submitRequestCount === 1) {
				return firstSubmitRequest.promise;
			}
			if (submitRequestCount === 2) {
				return secondSubmitRequest.promise;
			}
			throw new Error(`Unexpected submit fetch #${submitRequestCount}`);
		});
		const runtime = createNavigationRuntime();

		const firstSubmitPromise = runtime.submit(
			"/api/journal-submit",
			{ method: "POST" },
			{
				dedupeKey: "journal-submit",
				revalidate: false,
			},
		);
		await Promise.resolve();

		const secondSubmitPromise = runtime.submit(
			"/api/journal-submit",
			{ method: "POST" },
			{
				dedupeKey: "journal-submit",
				revalidate: false,
			},
		);

		secondSubmitRequest.resolve(
			createSubmitResponse({
				json: async () => ({ ok: true }),
			}),
		);
		firstSubmitRequest.reject(new DOMException("Aborted", "AbortError"));

		await expect(firstSubmitPromise).resolves.toEqual({
			success: false,
			error: "Aborted",
		});
		await expect(secondSubmitPromise).resolves.toEqual({
			success: true,
			data: { ok: true },
		});

		const journal = runtime.getDebugJournal();
		const dedupeEntry = journal.find(
			(entry) =>
				entry.reason === "submission_deduped_by_newer_submission",
		);
		expect(dedupeEntry).toBeDefined();
		expect(dedupeEntry?.lane).toBe("submission");
		expect(dedupeEntry?.operationID).toEqual(expect.any(Number));
		expect(dedupeEntry?.causedByOperationID).toEqual(expect.any(Number));
	});

	it("keeps debug journal APIs as no-ops when journaling is disabled", async () => {
		const originalDev = import.meta.env.DEV;
		(import.meta.env as any).DEV = false;
		vi.resetModules();

		const runtimeModule = await import("../../runtime.ts");
		vi.spyOn(window, "fetch").mockResolvedValue(
			new Response(
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
					outermostServerError: undefined,
					outermostServerErrorIdx: undefined,
					title: { dangerousInnerHTML: "Debug Journal Disabled" },
					metaHeadEls: undefined,
					restHeadEls: undefined,
				}),
				{
					status: 200,
					headers: {
						"Content-Type": "application/json",
						"X-Vorma-Build-Id": "1",
					},
				},
			),
		);
		try {
			const runtime = runtimeModule.createNavigationRuntime();
			await expect(
				runtime.navigate({
					href: "/debug-journal-disabled",
					navigationType: "userNavigation",
				}),
			).resolves.toEqual({ didNavigate: true });
			expect(runtime.getDebugJournal()).toEqual([]);
			expect(() => runtime.clearDebugJournal()).not.toThrow();
			expect(runtime.getDebugJournal()).toEqual([]);
		} finally {
			(import.meta.env as any).DEV = originalDev;
		}
	});

	it("records explicit fetch-rejection reasons across active, prefetch, and revalidation lanes", async () => {
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockRejectedValue(new Error("fetch failed"));

		try {
			const runtime = createNavigationRuntime();
			const activeControl = runtime.beginNavigation({
				href: "/fetch-reject-active",
				navigationType: "userNavigation",
			});
			const prefetchControl = runtime.beginNavigation({
				href: "/fetch-reject-prefetch",
				navigationType: "prefetch",
			});
			window.history.replaceState({}, "", "/fetch-reject-revalidation");
			const revalidationControl = runtime.beginNavigation({
				href: window.location.href,
				navigationType: "revalidation",
			});

			await Promise.allSettled([
				activeControl.promise,
				prefetchControl.promise,
				revalidationControl.promise,
			]);

			const journalReasons = runtime
				.getDebugJournal()
				.map((entry) => entry.reason);
			expect(journalReasons).toContain(
				"active_navigation_fetch_rejected",
			);
			expect(journalReasons).toContain("prefetch_fetch_rejected");
			expect(journalReasons).toContain("revalidation_fetch_rejected");
		} finally {
			fetchSpy.mockRestore();
		}
	});
});

describe("navigation runtime outcome stale control guards", () => {
	it("does not delete current entry when stale aborted outcome resolves for same target", async () => {
		const targetUrl = new URL(
			"/stale-control-aborted",
			window.location.href,
		).href;
		const currentEntry = createEntry({
			targetUrl,
			type: "userNavigation",
			intent: "navigate",
		});
		currentEntry.control.promise = Promise.resolve(
			createSuccessNavigationOutcome(),
		);

		const deleteNavigation = vi.fn(() => true);
		const processSuccessfulNavigation = vi
			.fn()
			.mockResolvedValue(undefined);

		const result = await handleNavigationOutcome({
			findNavigationEntry: () => currentEntry,
			deleteNavigation,
			processSuccessfulNavigation,
			navigationProps: {
				href: targetUrl,
				navigationType: "browserHistory",
			},
			outcome: { type: "aborted" },
			expectedOperationID: currentEntry.operationID + 1,
		});

		expect(result).toEqual({ didNavigate: false });
		expect(deleteNavigation).not.toHaveBeenCalled();
		expect(processSuccessfulNavigation).not.toHaveBeenCalled();
	});

	it("does not process success when stale operation ID no longer owns target", async () => {
		const targetUrl = new URL(
			"/stale-control-success",
			window.location.href,
		).href;
		const staleOutcome = createSuccessNavigationOutcome({
			props: {
				href: targetUrl,
				navigationType: "browserHistory",
			},
		});
		const currentEntry = createEntry({
			targetUrl,
			type: "userNavigation",
			intent: "navigate",
		});
		currentEntry.control.promise = Promise.resolve(
			createSuccessNavigationOutcome(),
		);

		const deleteNavigation = vi.fn(() => true);
		const processSuccessfulNavigation = vi
			.fn()
			.mockResolvedValue(undefined);

		const result = await handleNavigationOutcome({
			findNavigationEntry: () => currentEntry,
			deleteNavigation,
			processSuccessfulNavigation,
			navigationProps: staleOutcome.props,
			outcome: staleOutcome,
			expectedOperationID: currentEntry.operationID + 1,
		});

		expect(result).toEqual({ didNavigate: false });
		expect(deleteNavigation).not.toHaveBeenCalled();
		expect(processSuccessfulNavigation).not.toHaveBeenCalled();
	});

	it("deletes target for current aborted outcome when operation ID matches", async () => {
		const targetUrl = new URL(
			"/current-control-aborted",
			window.location.href,
		).href;
		const currentEntry = createEntry({
			targetUrl,
			type: "userNavigation",
			intent: "navigate",
		});

		const deleteNavigation = vi.fn(() => true);
		const processSuccessfulNavigation = vi
			.fn()
			.mockResolvedValue(undefined);

		const result = await handleNavigationOutcome({
			findNavigationEntry: () => currentEntry,
			deleteNavigation,
			processSuccessfulNavigation,
			navigationProps: {
				href: targetUrl,
				navigationType: "browserHistory",
			},
			outcome: { type: "aborted" },
			expectedOperationID: currentEntry.operationID,
		});

		expect(result).toEqual({ didNavigate: false });
		expect(deleteNavigation).toHaveBeenCalledOnce();
		expect(deleteNavigation).toHaveBeenCalledWith({
			targetUrl,
			reason: "outcome_aborted",
		});
		expect(processSuccessfulNavigation).not.toHaveBeenCalled();
	});
});

describe("navigation runtime outcome redirect signaling", () => {
	it("returns didNavigate true when current redirect effectuation completes", async () => {
		const targetUrl = new URL(
			"/current-control-redirect",
			window.location.href,
		).href;
		const redirectOutcome: Extract<
			NavigationOutcome,
			{ type: "redirect" }
		> = {
			type: "redirect" as const,
			redirectData: {
				status: "should" as const,
				shouldRedirectStrategy: "soft" as const,
				latestBuildID: "2",
				href: "/redirect-destination",
				hrefDetails: {
					url: new URL("http://localhost:3000/redirect-destination"),
					isHTTP: true,
					isInternal: true,
					isExternal: false,
					absoluteURL: "http://localhost:3000/redirect-destination",
					relativeURL: "/redirect-destination",
				},
			},
			props: {
				href: targetUrl,
				navigationType: "browserHistory" as const,
			},
		};
		const controlPromise = Promise.resolve(redirectOutcome);
		const currentEntry = createEntry({
			targetUrl,
			type: "browserHistory",
			intent: "navigate",
		});
		currentEntry.control.promise = controlPromise;

		const deleteNavigation = vi.fn(() => true);
		const processSuccessfulNavigation = vi
			.fn()
			.mockResolvedValue(undefined);
		const redirectNavigateSpy = vi.fn(async () => ({ didNavigate: true }));
		setNavigationStateAccess({
			navigate: redirectNavigateSpy,
			removeNavigation: () => {},
			getNavigations: () => new Map(),
		});

		const result = await handleNavigationOutcome({
			findNavigationEntry: () => currentEntry,
			deleteNavigation,
			processSuccessfulNavigation,
			navigationProps: {
				href: targetUrl,
				navigationType: "browserHistory",
			},
			outcome: redirectOutcome,
			expectedOperationID: currentEntry.operationID,
		});

		expect(result).toEqual({ didNavigate: true });
		expect(deleteNavigation).toHaveBeenCalledOnce();
		expect(deleteNavigation).toHaveBeenCalledWith({
			targetUrl,
			reason: "redirect_effectuate",
		});
		expect(redirectNavigateSpy).toHaveBeenCalledOnce();
		expect(redirectNavigateSpy).toHaveBeenCalledWith({
			href: "/redirect-destination",
			navigationType: "redirect",
			redirectCount: 1,
			state: currentEntry.state,
			replace: currentEntry.replace,
			scrollToTop: currentEntry.scrollToTop,
		});
	});

	it("uses current entry navigation options for redirect effectuation when a prefetch is promoted", async () => {
		const targetUrl = new URL(
			"/promoted-prefetch-redirect",
			window.location.href,
		).href;
		const redirectOutcome: Extract<
			NavigationOutcome,
			{ type: "redirect" }
		> = {
			type: "redirect" as const,
			redirectData: {
				status: "should" as const,
				shouldRedirectStrategy: "soft" as const,
				latestBuildID: "2",
				href: "/redirected-destination",
				hrefDetails: {
					url: new URL(
						"http://localhost:3000/redirected-destination",
					),
					isHTTP: true,
					isInternal: true,
					isExternal: false,
					absoluteURL: "http://localhost:3000/redirected-destination",
					relativeURL: "/redirected-destination",
				},
			},
			props: {
				href: targetUrl,
				navigationType: "prefetch" as const,
				scrollToTop: true,
				replace: false,
				state: { source: "prefetch" },
			},
		};
		const promotedEntry = createEntry({
			targetUrl,
			type: "userNavigation",
			intent: "navigate",
		});
		promotedEntry.scrollToTop = false;
		promotedEntry.replace = true;
		promotedEntry.state = { source: "click" };
		promotedEntry.control.promise = Promise.resolve(redirectOutcome);

		const deleteNavigation = vi.fn(() => true);
		const processSuccessfulNavigation = vi
			.fn()
			.mockResolvedValue(undefined);
		const redirectNavigateSpy = vi.fn(async () => ({ didNavigate: true }));
		setNavigationStateAccess({
			navigate: redirectNavigateSpy,
			removeNavigation: () => {},
			getNavigations: () => new Map(),
		});

		await handleNavigationOutcome({
			findNavigationEntry: () => promotedEntry,
			deleteNavigation,
			processSuccessfulNavigation,
			navigationProps: redirectOutcome.props,
			outcome: redirectOutcome,
			expectedOperationID: promotedEntry.operationID,
		});

		expect(redirectNavigateSpy).toHaveBeenCalledWith({
			href: "/redirected-destination",
			navigationType: "redirect",
			redirectCount: 1,
			state: promotedEntry.state,
			replace: promotedEntry.replace,
			scrollToTop: promotedEntry.scrollToTop,
		});
	});

	it("does not synchronize build IDs or effectuate redirects for stale redirect outcomes", async () => {
		const targetUrl = new URL(
			"/stale-control-redirect",
			window.location.href,
		).href;
		const redirectOutcome = {
			type: "redirect" as const,
			redirectData: {
				status: "should" as const,
				shouldRedirectStrategy: "soft" as const,
				latestBuildID: "stale-redirect-build",
				href: "/stale-redirect-target",
				hrefDetails: {
					url: new URL("http://localhost:3000/stale-redirect-target"),
					isHTTP: true,
					isInternal: true,
					isExternal: false,
					absoluteURL: "http://localhost:3000/stale-redirect-target",
					relativeURL: "/stale-redirect-target",
				},
			},
			props: {
				href: targetUrl,
				navigationType: "browserHistory" as const,
			},
		} as NavigationOutcome;
		const currentEntry = createEntry({
			targetUrl,
			type: "browserHistory",
			intent: "navigate",
		});
		currentEntry.control.promise = Promise.resolve(
			createSuccessNavigationOutcome(),
		);

		const deleteNavigation = vi.fn(() => true);
		const processSuccessfulNavigation = vi
			.fn()
			.mockResolvedValue(undefined);
		const redirectNavigateSpy = vi.fn(async () => ({ didNavigate: true }));
		setNavigationStateAccess({
			navigate: redirectNavigateSpy,
			removeNavigation: () => {},
			getNavigations: () => new Map(),
		});

		const result = await handleNavigationOutcome({
			findNavigationEntry: () => currentEntry,
			deleteNavigation,
			processSuccessfulNavigation,
			navigationProps: {
				href: targetUrl,
				navigationType: "browserHistory",
			},
			outcome: redirectOutcome,
			expectedOperationID: currentEntry.operationID + 1,
		});

		expect(result).toEqual({ didNavigate: false });
		expect(redirectNavigateSpy).not.toHaveBeenCalled();
		expect(deleteNavigation).not.toHaveBeenCalled();
	});
});

describe("navigation runtime success-processing defensive branches", () => {
	it("hasNavigation reflects tracked entries and hash aliases", async () => {
		const fetchSpy = createAbortAwareNeverResolvingFetchSpy();
		try {
			const runtime = createNavigationRuntime();
			const control = runtime.beginNavigation({
				href: "/has-navigation#first",
				navigationType: "browserHistory",
			});
			const canonicalHref = new URL(
				"/has-navigation#first",
				window.location.href,
			).href;
			const aliasHref = new URL(
				"/has-navigation#second",
				window.location.href,
			).href;

			expect(runtime.hasNavigation(canonicalHref)).toBe(true);
			expect(runtime.hasNavigation(aliasHref)).toBe(true);

			runtime.removeNavigation(aliasHref);
			expect(runtime.hasNavigation(canonicalHref)).toBe(false);
			await expect(control.promise).rejects.toMatchObject({
				name: "AbortError",
			});
		} finally {
			fetchSpy.mockRestore();
		}
	});

	it("continues successful processing when css preload commands reject", async () => {
		const fetchSpy = createAbortAwareNeverResolvingFetchSpy();
		const consoleErrorSpy = vi
			.spyOn(console, "error")
			.mockImplementation(() => {});
		const cssPreloadError = new Error("css preload failed");
		const preloadCSSSpy = vi
			.spyOn(renderRuntimeModule.AssetManager, "preloadCSS")
			.mockRejectedValue(cssPreloadError);

		try {
			const runtime = createNavigationRuntime();
			runtime.beginNavigation({
				href: "/css-preload-failure",
				navigationType: "browserHistory",
			});
			const targetUrl = new URL(
				"/css-preload-failure",
				window.location.href,
			).href;
			const entry = runtime.getNavigation(targetUrl);
			expect(entry).toBeDefined();
			if (!entry) return;

			const outcome = createSuccessNavigationOutcome({
				preloadPlan: {
					moduleDependencies: [],
					cssBundles: ["/failure.css"],
				},
				props: {
					href: targetUrl,
					navigationType: "browserHistory",
				},
			});

			await expect(
				runtime.processSuccessfulNavigation(outcome, entry),
			).resolves.toBeUndefined();
			expect(
				consoleErrorSpy.mock.calls.some(
					(call) =>
						call[0] === "Vorma:" &&
						call[1] === "Error preloading CSS bundles:" &&
						call[2] === cssPreloadError,
				),
			).toBe(true);
		} finally {
			fetchSpy.mockRestore();
			consoleErrorSpy.mockRestore();
			preloadCSSSpy.mockRestore();
		}
	});

	it("marks navigation complete and rethrows when render fails", async () => {
		const fetchSpy = createAbortAwareNeverResolvingFetchSpy();
		const renderError = new Error("render failed");
		const loadComponentsSpy = vi
			.spyOn(renderRuntimeModule.ComponentLoader, "loadComponents")
			.mockRejectedValue(renderError);
		const consoleErrorSpy = vi
			.spyOn(console, "error")
			.mockImplementation(() => {});

		try {
			const runtime = createNavigationRuntime();
			runtime.beginNavigation({
				href: "/render-failure",
				navigationType: "browserHistory",
			});
			const targetUrl = new URL("/render-failure", window.location.href)
				.href;
			const entry = runtime.getNavigation(targetUrl);
			expect(entry).toBeDefined();
			if (!entry) return;

			const outcome = createSuccessNavigationOutcome({
				props: {
					href: targetUrl,
					navigationType: "browserHistory",
				},
			});

			await expect(
				runtime.processSuccessfulNavigation(outcome, entry),
			).rejects.toBe(renderError);
			expect(entry.phase).toBe("complete");
			expect(runtime.getNavigation(targetUrl)).toBeUndefined();
			expect(
				consoleErrorSpy.mock.calls.some(
					(call) =>
						call[0] === "Vorma:" &&
						call[1] === "Error completing navigation" &&
						call[2] === renderError,
				),
			).toBe(true);
		} finally {
			fetchSpy.mockRestore();
			loadComponentsSpy.mockRestore();
			consoleErrorSpy.mockRestore();
		}
	});

	it("rethrows abort render failures without logging completion errors", async () => {
		const fetchSpy = createAbortAwareNeverResolvingFetchSpy();
		const renderAbortError = new Error("Aborted");
		renderAbortError.name = "AbortError";
		const loadComponentsSpy = vi
			.spyOn(renderRuntimeModule.ComponentLoader, "loadComponents")
			.mockRejectedValue(renderAbortError);
		const consoleErrorSpy = vi
			.spyOn(console, "error")
			.mockImplementation(() => {});

		try {
			const runtime = createNavigationRuntime();
			runtime.beginNavigation({
				href: "/render-abort",
				navigationType: "browserHistory",
			});
			const targetUrl = new URL("/render-abort", window.location.href)
				.href;
			const entry = runtime.getNavigation(targetUrl);
			expect(entry).toBeDefined();
			if (!entry) return;

			const outcome = createSuccessNavigationOutcome({
				props: {
					href: targetUrl,
					navigationType: "browserHistory",
				},
			});

			await expect(
				runtime.processSuccessfulNavigation(outcome, entry),
			).rejects.toBe(renderAbortError);
			expect(entry.phase).toBe("complete");
			expect(
				consoleErrorSpy.mock.calls.some(
					(call) =>
						call[0] === "Vorma:" &&
						call[1] === "Error completing navigation" &&
						call[2] === renderAbortError,
				),
			).toBe(false);
		} finally {
			fetchSpy.mockRestore();
			loadComponentsSpy.mockRestore();
			consoleErrorSpy.mockRestore();
		}
	});

	it("returns early when the entry is removed before waiting phase resolves", async () => {
		const fetchSpy = createAbortAwareNeverResolvingFetchSpy();
		const reRenderSpy = vi
			.spyOn(renderRuntimeModule, "__reRenderApp")
			.mockResolvedValue();

		try {
			const runtime = createNavigationRuntime();
			runtime.beginNavigation({
				href: "/removed-before-wait",
				navigationType: "browserHistory",
			});
			const targetUrl = new URL(
				"/removed-before-wait",
				window.location.href,
			).href;
			const entry = runtime.getNavigation(targetUrl);
			expect(entry).toBeDefined();
			if (!entry) return;

			runtime.removeNavigation(targetUrl);

			const outcome = createSuccessNavigationOutcome({
				props: {
					href: targetUrl,
					navigationType: "browserHistory",
				},
			});

			await expect(
				runtime.processSuccessfulNavigation(outcome, entry),
			).resolves.toBeUndefined();
			expect(reRenderSpy).not.toHaveBeenCalled();
		} finally {
			fetchSpy.mockRestore();
			reRenderSpy.mockRestore();
		}
	});

	it("returns early when ownership is lost during waiting-phase transition", async () => {
		const reRenderSpy = vi
			.spyOn(renderRuntimeModule, "__reRenderApp")
			.mockResolvedValue();

		try {
			const targetUrl = new URL(
				"/removed-during-waiting-transition",
				window.location.href,
			).href;
			const entry = createEntry({
				targetUrl,
				type: "browserHistory",
				intent: "navigate",
			});
			let ownedEntry: NavigationEntry | undefined = entry;
			const transitionPhase = vi.fn(
				(props: {
					targetUrl: string;
					phase: "fetching" | "waiting" | "rendering" | "complete";
					reason: string;
				}) => {
					if (props.phase === "waiting") {
						ownedEntry = undefined;
					}
				},
			);
			const findNavigationEntry = vi.fn(
				(_nextTargetUrl: string) => ownedEntry,
			);
			const deleteNavigation = vi.fn(() => true);

			await expect(
				processSuccessfulNavigationRuntime(
					{
						transitionPhase,
						findNavigationEntry: (nextTargetUrl: string) =>
							findNavigationEntry(nextTargetUrl),
						deleteNavigation,
					},
					createSuccessNavigationOutcome({
						props: {
							href: targetUrl,
							navigationType: "browserHistory",
						},
					}),
					entry,
				),
			).resolves.toBeUndefined();

			expect(reRenderSpy).not.toHaveBeenCalled();
			expect(transitionPhase).toHaveBeenCalledWith({
				targetUrl,
				phase: "waiting",
				reason: "process_successful_navigation_waiting",
			});
			expect(deleteNavigation).not.toHaveBeenCalled();
		} finally {
			reRenderSpy.mockRestore();
		}
	});

	it("does not commit client-loader state when ownership is lost during asset wait", async () => {
		const reRenderSpy = vi
			.spyOn(renderRuntimeModule, "__reRenderApp")
			.mockResolvedValue();
		const buildIDEvents: Array<{ oldID: string; newID: string }> = [];
		const removeBuildIDListener = addBuildIDListener((event) => {
			buildIDEvents.push(event.detail);
		});

		try {
			const targetUrl = new URL(
				"/removed-during-asset-wait",
				window.location.href,
			).href;
			const entry = createEntry({
				targetUrl,
				type: "browserHistory",
				intent: "navigate",
			});
			let ownedEntry: NavigationEntry | undefined = entry;
			const transitionPhase = vi.fn();
			const deleteNavigation = vi.fn(() => true);
			installVormaGlobal({
				clientLoadersData: [{ stable: true }],
			});

			await expect(
				processSuccessfulNavigationRuntime(
					{
						transitionPhase,
						findNavigationEntry: (_nextTargetUrl: string) =>
							ownedEntry,
						deleteNavigation,
					},
					createSuccessNavigationOutcome({
						waitFnPromise: Promise.resolve().then(() => {
							ownedEntry = undefined;
							return {
								data: [{ stale: true }],
							};
						}),
						responseBuildID: "2",
						props: {
							href: targetUrl,
							navigationType: "browserHistory",
						},
					}),
					entry,
				),
			).resolves.toBeUndefined();

			expect(reRenderSpy).not.toHaveBeenCalled();
			expect(
				__vormaClientGlobal.get("runtimeRouteSnapshot")
					.clientLoadersData,
			).toEqual([{ stable: true }]);
			expect(
				__vormaClientGlobal.get("runtimeRouteSnapshot").buildID,
			).toBe("1");
			expect(buildIDEvents).toEqual([]);
			expect(deleteNavigation).not.toHaveBeenCalled();
		} finally {
			removeBuildIDListener();
			reRenderSpy.mockRestore();
		}
	});

	it("syncs build ID only after asset wait completes for current browser-history entries", async () => {
		const reRenderSpy = vi
			.spyOn(renderRuntimeModule, "__reRenderApp")
			.mockResolvedValue();
		const buildIDEvents: Array<{ oldID: string; newID: string }> = [];
		const removeBuildIDListener = addBuildIDListener((event) => {
			buildIDEvents.push(event.detail);
		});
		const waitDeferred = createDeferred<{
			data: Array<unknown>;
			errorMessage?: string;
		}>();

		try {
			const targetUrl = new URL(
				"/build-id-sync-after-asset-wait",
				window.location.href,
			).href;
			const entry = createEntry({
				targetUrl,
				type: "browserHistory",
				intent: "navigate",
			});

			const processingPromise = processSuccessfulNavigationRuntime(
				{
					transitionPhase: vi.fn(),
					findNavigationEntry: (_nextTargetUrl: string) => entry,
					deleteNavigation: vi.fn(() => true),
				},
				createSuccessNavigationOutcome({
					waitFnPromise: waitDeferred.promise,
					responseBuildID: "2",
					props: {
						href: targetUrl,
						navigationType: "browserHistory",
					},
				}),
				entry,
			);

			await Promise.resolve();
			expect(
				__vormaClientGlobal.get("runtimeRouteSnapshot").buildID,
			).toBe("1");
			expect(buildIDEvents).toEqual([]);

			waitDeferred.resolve({ data: [] });
			await expect(processingPromise).resolves.toBeUndefined();
			expect(
				__vormaClientGlobal.get("runtimeRouteSnapshot").buildID,
			).toBe("2");
			expect(buildIDEvents).toEqual([
				{
					oldID: "1",
					newID: "2",
				},
			]);
		} finally {
			removeBuildIDListener();
			reRenderSpy.mockRestore();
		}
	});

	it("resolves navigation intent only after successful navigation commit", async () => {
		const fetchSpy = createAbortAwareNeverResolvingFetchSpy();
		const onNavigationIntentResolved = vi.fn();
		try {
			const runtime = createNavigationRuntime({
				onNavigationIntentResolved,
			});
			const control = runtime.beginNavigation({
				href: "/intent-resolution-committed",
				navigationType: "browserHistory",
			});
			const targetUrl = new URL(
				"/intent-resolution-committed",
				window.location.href,
			).href;
			const entry = runtime.getNavigation(targetUrl);
			expect(entry).toBeDefined();
			if (!entry) {
				control.abortController?.abort();
				return;
			}

			await expect(
				runtime.processSuccessfulNavigation(
					createSuccessNavigationOutcome({
						props: {
							href: targetUrl,
							navigationType: "browserHistory",
						},
					}),
					entry,
				),
			).resolves.toBeUndefined();

			expect(onNavigationIntentResolved).toHaveBeenCalledTimes(1);
			control.abortController?.abort();
		} finally {
			fetchSpy.mockRestore();
		}
	});

	it("commits same-document hash-only navigations without fetch and without resolving freshness intent", async () => {
		window.history.replaceState({}, "", "/intent-resolution-hash");
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValue(new Response("unused"));
		const onNavigationIntentResolved = vi.fn();
		const runtime = createNavigationRuntime({
			onNavigationIntentResolved,
		});

		const result = await runtime.navigate({
			href: "/intent-resolution-hash#details",
			navigationType: "userNavigation",
		});

		expect(result).toEqual({ didNavigate: true });
		expect(fetchSpy).not.toHaveBeenCalled();
		expect(window.location.pathname).toBe("/intent-resolution-hash");
		expect(window.location.hash).toBe("#details");
		expect(onNavigationIntentResolved).not.toHaveBeenCalled();
	});

	it("treats programmatic same-document no-op targets as no-op without fetch", async () => {
		window.history.replaceState({}, "", "/same-doc-noop#~");
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValue(new Response("unused"));
		const onNavigationIntentResolved = vi.fn();
		const runtime = createNavigationRuntime({
			onNavigationIntentResolved,
		});

		const result = await runtime.navigate({
			href: "/same-doc-noop#%7E",
			navigationType: "userNavigation",
		});

		expect(result).toEqual({ didNavigate: false });
		expect(fetchSpy).not.toHaveBeenCalled();
		expect(window.location.pathname).toBe("/same-doc-noop");
		expect(window.location.hash).toBe("#~");
		expect(onNavigationIntentResolved).not.toHaveBeenCalled();
	});

	it("does not resolve navigation intent for stale successful completions", async () => {
		const fetchSpy = createAbortAwareNeverResolvingFetchSpy();
		const onNavigationIntentResolved = vi.fn();
		try {
			const runtime = createNavigationRuntime({
				onNavigationIntentResolved,
			});
			const control = runtime.beginNavigation({
				href: "/intent-resolution-stale-first",
				navigationType: "browserHistory",
			});
			const firstTargetUrl = new URL(
				"/intent-resolution-stale-first",
				window.location.href,
			).href;
			const firstEntry = runtime.getNavigation(firstTargetUrl);
			expect(firstEntry).toBeDefined();
			if (!firstEntry) {
				control.abortController?.abort();
				return;
			}

			await expect(
				runtime.processSuccessfulNavigation(
					createSuccessNavigationOutcome({
						waitFnPromise: Promise.resolve().then(() => {
							runtime.beginNavigation({
								href: "/intent-resolution-stale-second",
								navigationType: "browserHistory",
							});
							return { data: [] };
						}),
						props: {
							href: firstTargetUrl,
							navigationType: "browserHistory",
						},
					}),
					firstEntry,
				),
			).resolves.toBeUndefined();

			expect(onNavigationIntentResolved).not.toHaveBeenCalled();
			control.abortController?.abort();
		} finally {
			fetchSpy.mockRestore();
		}
	});

	it("aborts stale revalidation after wait completes without rendering", async () => {
		const fetchSpy = createAbortAwareNeverResolvingFetchSpy();
		const reRenderSpy = vi
			.spyOn(renderRuntimeModule, "__reRenderApp")
			.mockResolvedValue();

		try {
			window.history.replaceState({}, "", "/revalidation-stale");
			const runtime = createNavigationRuntime();
			runtime.beginNavigation({
				href: window.location.href,
				navigationType: "revalidation",
			});

			const entry = runtime.getNavigation(window.location.href);
			expect(entry).toBeDefined();
			if (!entry) return;

			const waitFnPromise = Promise.resolve().then(() => {
				window.history.replaceState(
					{},
					"",
					"/revalidation-stale-after-wait",
				);
				return { data: [] };
			});
			const outcome = createSuccessNavigationOutcome({
				waitFnPromise,
				props: {
					href: entry.targetUrl,
					navigationType: "revalidation",
				},
			});

			await expect(
				runtime.processSuccessfulNavigation(outcome, entry),
			).resolves.toBeUndefined();
			expect(reRenderSpy).not.toHaveBeenCalled();
			expect(runtime.getNavigation(entry.targetUrl)).toBeUndefined();
		} finally {
			fetchSpy.mockRestore();
			reRenderSpy.mockRestore();
		}
	});

	it("does not render or delete a newer same-target entry when an older success finishes late", async () => {
		const fetchSpy = createAbortAwareNeverResolvingFetchSpy();
		const reRenderSpy = vi
			.spyOn(renderRuntimeModule, "__reRenderApp")
			.mockResolvedValue();
		const buildIDEvents: Array<{ oldID: string; newID: string }> = [];
		const removeBuildIDListener = addBuildIDListener((event) => {
			buildIDEvents.push(event.detail);
		});

		try {
			const runtime = createNavigationRuntime();
			const firstControl = runtime.beginNavigation({
				href: "/same-target-replacement",
				navigationType: "browserHistory",
			});
			const firstControlPromise = firstControl.promise.catch(
				(error) => error,
			);

			const targetUrl = new URL(
				"/same-target-replacement",
				window.location.href,
			).href;
			const firstEntry = runtime.getNavigation(targetUrl);
			expect(firstEntry).toBeDefined();
			if (!firstEntry) return;

			const waitDeferred = createDeferred<{
				data: Array<unknown>;
				errorMessage?: string;
			}>();
			const staleSuccessOutcome = createSuccessNavigationOutcome({
				waitFnPromise: waitDeferred.promise,
				responseBuildID: "stale-replaced-build-id",
				props: {
					href: targetUrl,
					navigationType: "browserHistory",
				},
			});
			staleSuccessOutcome.json.matchedPatterns = ["/stale-route"];
			staleSuccessOutcome.json.importURLs = ["/stale-route.js"];
			staleSuccessOutcome.json.exportKeys = ["default"];
			staleSuccessOutcome.json.errorExportKeys = [""];
			const olderSuccessProcessingPromise =
				runtime.processSuccessfulNavigation(
					staleSuccessOutcome,
					firstEntry,
				);

			runtime.removeNavigation(targetUrl);

			const secondControl = runtime.beginNavigation({
				href: "/same-target-replacement",
				navigationType: "browserHistory",
			});
			const secondControlPromise = secondControl.promise.catch(
				(error) => error,
			);
			const secondEntry = runtime.getNavigation(targetUrl);
			expect(secondEntry).toBeDefined();
			if (!secondEntry) return;
			expect(secondEntry).not.toBe(firstEntry);

			waitDeferred.resolve({ data: [] });
			await expect(
				olderSuccessProcessingPromise,
			).resolves.toBeUndefined();

			expect(reRenderSpy).not.toHaveBeenCalled();
			expect(runtime.getNavigation(targetUrl)).toBe(secondEntry);
			expect(
				__vormaClientGlobal.get("runtimeRouteSnapshot").buildID,
			).toBe("1");
			expect(buildIDEvents).toEqual([]);

			runtime.clearAll();
			await expect(firstControlPromise).resolves.toMatchObject({
				name: "AbortError",
			});
			await expect(secondControlPromise).resolves.toMatchObject({
				name: "AbortError",
			});
		} finally {
			fetchSpy.mockRestore();
			reRenderSpy.mockRestore();
			removeBuildIDListener();
		}
	});

	it("does not mark a newer same-target entry complete from a stale render onFinish callback", async () => {
		const fetchSpy = createAbortAwareNeverResolvingFetchSpy();
		const loadComponentsDeferred =
			createDeferred<
				Awaited<
					ReturnType<
						typeof renderRuntimeModule.ComponentLoader.loadComponents
					>
				>
			>();
		const loadComponentsStartedDeferred = createDeferred<void>();
		const loadComponentsSpy = vi
			.spyOn(renderRuntimeModule.ComponentLoader, "loadComponents")
			.mockImplementation(async () => {
				loadComponentsStartedDeferred.resolve();
				return loadComponentsDeferred.promise;
			});

		try {
			const runtime = createNavigationRuntime();
			const firstControl = runtime.beginNavigation({
				href: "/same-target-stale-render-finish",
				navigationType: "browserHistory",
			});
			const firstControlPromise = firstControl.promise.catch(
				(error) => error,
			);

			const targetUrl = new URL(
				"/same-target-stale-render-finish",
				window.location.href,
			).href;
			const firstEntry = runtime.getNavigation(targetUrl);
			expect(firstEntry).toBeDefined();
			if (!firstEntry) return;

			const olderSuccessProcessingPromise =
				runtime.processSuccessfulNavigation(
					createSuccessNavigationOutcome({
						waitFnPromise: Promise.resolve({ data: [] }),
						props: {
							href: targetUrl,
							navigationType: "browserHistory",
						},
					}),
					firstEntry,
				);

			await loadComponentsStartedDeferred.promise;

			runtime.removeNavigation(targetUrl);

			const secondControl = runtime.beginNavigation({
				href: "/same-target-stale-render-finish",
				navigationType: "browserHistory",
			});
			const secondControlPromise = secondControl.promise.catch(
				(error) => error,
			);
			const secondEntry = runtime.getNavigation(targetUrl);
			expect(secondEntry).toBeDefined();
			if (!secondEntry) return;
			expect(secondEntry).not.toBe(firstEntry);
			expect(secondEntry.phase).toBe("fetching");

			loadComponentsDeferred.resolve(new Map());
			await expect(
				olderSuccessProcessingPromise,
			).resolves.toBeUndefined();
			expect(secondEntry.phase).toBe("fetching");
			expect(runtime.getNavigation(targetUrl)).toBe(secondEntry);

			runtime.clearAll();
			await expect(firstControlPromise).resolves.toMatchObject({
				name: "AbortError",
			});
			await expect(secondControlPromise).resolves.toMatchObject({
				name: "AbortError",
			});
		} finally {
			loadComponentsDeferred.resolve(new Map());
			fetchSpy.mockRestore();
			loadComponentsSpy.mockRestore();
		}
	});

	it("does not commit render side effects when ownership is lost during async module resolution", async () => {
		const targetUrl = new URL(
			"/stale-render-commit-fence",
			window.location.href,
		).href;
		const entry = createEntry({
			targetUrl,
			type: "browserHistory",
			intent: "navigate",
		});
		let ownedEntry: NavigationEntry | undefined = entry;
		const transitionPhase = vi.fn();
		const deleteNavigation = vi.fn(() => true);
		document.title = "Before Commit Fence";
		window.history.replaceState({}, "", "/before-commit-fence");

		const staleSuccessOutcome = createSuccessNavigationOutcome({
			props: {
				href: targetUrl,
				navigationType: "browserHistory",
			},
		});
		staleSuccessOutcome.json.title = {
			dangerousInnerHTML: "Stale Commit Should Not Apply",
		};
		staleSuccessOutcome.json.matchedPatterns = ["/stale-commit-fence"];
		staleSuccessOutcome.json.importURLs = ["/stale-commit-fence.js"];
		staleSuccessOutcome.json.exportKeys = ["default"];
		staleSuccessOutcome.json.errorExportKeys = [""];
		staleSuccessOutcome.json.metaHeadEls = [
			{
				tag: "meta",
				attributesKnownSafe: {
					name: "stale-commit-fence",
					content: "1",
				},
			},
		];

		const loadComponentsSpy = vi
			.spyOn(renderRuntimeModule.ComponentLoader, "loadComponents")
			.mockImplementation(async (importURLs) => {
				ownedEntry = undefined;
				const modulesMapEntries: Array<
					[string, Record<string, unknown>]
				> = (importURLs || []).map((importURL) => [
					importURL,
					{ default: () => null },
				]);
				return new Map(modulesMapEntries);
			});

		try {
			await expect(
				processSuccessfulNavigationRuntime(
					{
						transitionPhase,
						findNavigationEntry: (_nextTargetUrl: string) =>
							ownedEntry,
						deleteNavigation,
					},
					staleSuccessOutcome,
					entry,
				),
			).resolves.toBeUndefined();

			expect(document.title).toBe("Before Commit Fence");
			expect(window.location.pathname).toBe("/before-commit-fence");
			expect(
				document.head.querySelector('meta[name="stale-commit-fence"]'),
			).toBeNull();
			expect(
				__vormaClientGlobal.get("runtimeRouteSnapshot").matchedPatterns,
			).toEqual([]);
			expect(deleteNavigation).not.toHaveBeenCalled();
		} finally {
			loadComponentsSpy.mockRestore();
		}
	});
});

describe("navigation runtime submit stale checkpoints", () => {
	it("aborts stale deduped submit before finalize response processing after build-id listener replacement", async () => {
		const runtime = createNavigationRuntime();
		const firstJson = vi.fn(async () => ({ stale: true }));
		let replacementSubmit:
			| Promise<
					| { success: true; data: unknown }
					| { success: false; error: string }
			  >
			| undefined;
		let buildIdEventCount = 0;

		const cleanupBuildIDListener = addBuildIDListener(() => {
			buildIdEventCount++;
			if (replacementSubmit) return;
			replacementSubmit = runtime.submit(
				"/api/replacement",
				{ method: "POST" },
				{ dedupeKey: "finalize-stale", revalidate: false },
			);
		});

		let fetchCallCount = 0;
		const fetchSpy = vi.spyOn(window, "fetch").mockImplementation(() => {
			fetchCallCount++;
			if (fetchCallCount === 1) {
				return Promise.resolve(
					createSubmitResponse({
						buildID: "2",
						json: firstJson,
					}),
				);
			}

			return Promise.resolve(
				createSubmitResponse({
					buildID: "2",
					json: async () => ({ fresh: true }),
				}),
			);
		});

		try {
			const firstSubmit = runtime.submit(
				"/api/original",
				{ method: "POST" },
				{ dedupeKey: "finalize-stale", revalidate: false },
			);

			await expect(firstSubmit).resolves.toEqual({
				success: false,
				error: "Aborted",
			});
			expect(firstJson).not.toHaveBeenCalled();
			expect(buildIdEventCount).toBe(1);
			expect(replacementSubmit).toBeDefined();
			await expect(replacementSubmit).resolves.toEqual({
				success: true,
				data: { fresh: true },
			});
			expect(fetchCallCount).toBe(2);
		} finally {
			cleanupBuildIDListener();
			fetchSpy.mockRestore();
		}
	});

	it("aborts stale deduped submit after JSON parsing when a replacement submission starts during parsing", async () => {
		const runtime = createNavigationRuntime();
		const firstJSONDeferred = createDeferred<unknown>();
		let firstJSONWasRequested = false;

		let fetchCallCount = 0;
		const fetchSpy = vi.spyOn(window, "fetch").mockImplementation(() => {
			fetchCallCount++;
			if (fetchCallCount === 1) {
				return Promise.resolve(
					createSubmitResponse({
						buildID: "1",
						json: () => {
							firstJSONWasRequested = true;
							return firstJSONDeferred.promise;
						},
					}),
				);
			}

			return Promise.resolve(
				createSubmitResponse({
					buildID: "1",
					json: async () => ({ fresh: true }),
				}),
			);
		});

		try {
			const firstSubmit = runtime.submit(
				"/api/original",
				{ method: "POST" },
				{ dedupeKey: "json-stale", revalidate: false },
			);

			await vi.waitFor(() => {
				expect(firstJSONWasRequested).toBe(true);
			});

			const replacementSubmit = runtime.submit(
				"/api/replacement",
				{ method: "POST" },
				{ dedupeKey: "json-stale", revalidate: false },
			);
			firstJSONDeferred.resolve({ stale: true });

			await expect(firstSubmit).resolves.toEqual({
				success: false,
				error: "Aborted",
			});
			await expect(replacementSubmit).resolves.toEqual({
				success: true,
				data: { fresh: true },
			});
			expect(fetchCallCount).toBe(2);
		} finally {
			fetchSpy.mockRestore();
		}
	});

	it("aborts stale deduped submit when response classification starts a replacement submission", async () => {
		const runtime = createNavigationRuntime();
		let replacementSubmit:
			| Promise<
					| { success: true; data: unknown }
					| { success: false; error: string }
			  >
			| undefined;
		let okReadCount = 0;

		let fetchCallCount = 0;
		const fetchSpy = vi.spyOn(window, "fetch").mockImplementation(() => {
			fetchCallCount++;
			if (fetchCallCount === 1) {
				return Promise.resolve({
					headers: new Headers({
						"X-Vorma-Build-Id": "1",
					}),
					redirected: false,
					url: window.location.href,
					status: 503,
					get ok() {
						okReadCount++;
						if (!replacementSubmit) {
							replacementSubmit = runtime.submit(
								"/api/replacement",
								{ method: "POST" },
								{
									dedupeKey: "classification-stale",
									revalidate: false,
								},
							);
						}
						return false;
					},
					json: async () => ({ stale: true }),
				} as unknown as Response);
			}

			return Promise.resolve(
				createSubmitResponse({
					buildID: "1",
					json: async () => ({ fresh: true }),
				}),
			);
		});

		try {
			const firstSubmit = runtime.submit(
				"/api/original",
				{ method: "POST" },
				{ dedupeKey: "classification-stale", revalidate: false },
			);

			await expect(firstSubmit).resolves.toEqual({
				success: false,
				error: "Aborted",
			});
			expect(okReadCount).toBeGreaterThan(0);
			expect(replacementSubmit).toBeDefined();
			await expect(replacementSubmit).resolves.toEqual({
				success: true,
				data: { fresh: true },
			});
			expect(fetchCallCount).toBe(2);
		} finally {
			fetchSpy.mockRestore();
		}
	});

	it("aborts stale deduped submit before auto-revalidation when method access starts a replacement submission", async () => {
		const runtime = createNavigationRuntime();
		let replacementSubmit:
			| Promise<
					| { success: true; data: unknown }
					| { success: false; error: string }
			  >
			| undefined;
		let methodReadCount = 0;
		const firstRequestInit: RequestInit = {};

		Object.defineProperty(firstRequestInit, "method", {
			configurable: true,
			enumerable: false,
			get: () => {
				methodReadCount++;
				if (!replacementSubmit) {
					replacementSubmit = runtime.submit(
						"/api/replacement",
						{ method: "POST" },
						{ dedupeKey: "revalidate-stale", revalidate: false },
					);
				}
				return "POST";
			},
		});

		let fetchCallCount = 0;
		const fetchSpy = vi.spyOn(window, "fetch").mockImplementation(() => {
			fetchCallCount++;
			return Promise.resolve(
				createSubmitResponse({
					buildID: "1",
					json: async () => ({ ok: true }),
				}),
			);
		});

		try {
			const firstSubmit = runtime.submit(
				"/api/original",
				firstRequestInit,
				{ dedupeKey: "revalidate-stale" },
			);

			await expect(firstSubmit).resolves.toEqual({
				success: false,
				error: "Aborted",
			});
			expect(methodReadCount).toBeGreaterThan(0);
			expect(replacementSubmit).toBeDefined();
			await expect(replacementSubmit).resolves.toEqual({
				success: true,
				data: { ok: true },
			});
			expect(fetchCallCount).toBe(2);
		} finally {
			fetchSpy.mockRestore();
		}
	});

	it("returns success for submit responses that trigger client redirects", async () => {
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			new Response("{}", {
				status: 200,
				headers: {
					"Content-Type": "application/json",
					"X-Vorma-Build-Id": "1",
					"X-Client-Redirect": "/loop-redirect",
				},
			}),
		);
		const consoleErrorSpy = vi
			.spyOn(console, "error")
			.mockImplementation(() => {});

		try {
			const runtime = createNavigationRuntime();
			const result = await runtime.submit(
				"/api/missing-submit-response",
				{ method: "POST" },
				{ revalidate: false },
			);

			expect(result).toEqual({
				success: true,
				data: undefined,
			});
		} finally {
			fetchSpy.mockRestore();
			consoleErrorSpy.mockRestore();
		}
	});

	it("fails fast and avoids fetch for cross-origin submit targets", async () => {
		const fetchSpy = vi.spyOn(window, "fetch");

		const runtime = createNavigationRuntime();
		const result = await runtime.submit(
			"https://external.example/api",
			{ method: "POST" },
			{ revalidate: false },
		);

		expect(result).toEqual({
			success: false,
			error: expect.stringContaining(
				"submit(...) only supports same-origin targets.",
			),
		});
		expect(fetchSpy).not.toHaveBeenCalled();
	});

	it("returns success with undefined data for 204 submit responses", async () => {
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			new Response(null, {
				status: 204,
				headers: {
					"X-Vorma-Build-Id": "1",
				},
			}),
		);

		try {
			const runtime = createNavigationRuntime();
			const result = await runtime.submit(
				"/api/no-content-submit",
				{ method: "POST" },
				{ revalidate: false },
			);

			expect(result).toEqual({
				success: true,
				data: undefined,
			});
		} finally {
			fetchSpy.mockRestore();
		}
	});

	it("returns text data for successful non-JSON submit responses", async () => {
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			new Response("ok-text", {
				status: 200,
				headers: {
					"Content-Type": "text/plain",
					"X-Vorma-Build-Id": "1",
				},
			}),
		);

		try {
			const runtime = createNavigationRuntime();
			const result = await runtime.submit(
				"/api/non-json-submit",
				{ method: "POST" },
				{ revalidate: false },
			);

			expect(result).toEqual({
				success: true,
				data: "ok-text",
			});
		} finally {
			fetchSpy.mockRestore();
		}
	});

	it("returns text data for successful submit responses without content-type", async () => {
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			new Response("ok-text-no-content-type", {
				status: 200,
				headers: {
					"X-Vorma-Build-Id": "1",
				},
			}),
		);

		try {
			const runtime = createNavigationRuntime();
			const result = await runtime.submit(
				"/api/no-content-type-submit",
				{ method: "POST" },
				{ revalidate: false },
			);

			expect(result).toEqual({
				success: true,
				data: "ok-text-no-content-type",
			});
		} finally {
			fetchSpy.mockRestore();
		}
	});

	it("prioritizes submit redirects even when submit response status is non-OK", async () => {
		const redirectNavigateSpy = vi.fn(async () => ({ didNavigate: true }));
		setNavigationStateAccess({
			navigate: redirectNavigateSpy,
			removeNavigation: () => {},
			getNavigations: () => new Map(),
		});
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			new Response(JSON.stringify({ ok: false }), {
				status: 422,
				headers: {
					"Content-Type": "application/json",
					"X-Vorma-Build-Id": "1",
					"X-Client-Redirect": "/submit-redirect-target",
				},
			}),
		);

		try {
			const runtime = createNavigationRuntime();
			const result = await runtime.submit(
				"/api/submit-redirect-non-ok",
				{ method: "POST" },
				{ revalidate: false },
			);

			expect(redirectNavigateSpy).toHaveBeenCalledOnce();
			expect(result).toEqual({
				success: true,
				data: undefined,
			});
		} finally {
			fetchSpy.mockRestore();
		}
	});

	it("returns explicit submit error when redirect effectuation fails", async () => {
		const redirectNavigateSpy = vi.fn(async () => ({ didNavigate: false }));
		setNavigationStateAccess({
			navigate: redirectNavigateSpy,
			removeNavigation: () => {},
			getNavigations: () => new Map(),
		});
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			new Response(JSON.stringify({ ok: true }), {
				status: 200,
				headers: {
					"Content-Type": "application/json",
					"X-Vorma-Build-Id": "1",
					"X-Client-Redirect": "/submit-redirect-target",
				},
			}),
		);

		try {
			const runtime = createNavigationRuntime();
			const result = await runtime.submit(
				"/api/failed-submit-redirect",
				{ method: "POST" },
				{ revalidate: false },
			);

			expect(redirectNavigateSpy).toHaveBeenCalledOnce();
			expect(result).toEqual({
				success: false,
				error: "Redirect failed",
			});
		} finally {
			fetchSpy.mockRestore();
		}
	});

	it("aborts stale deduped submit when replacement starts during redirect effectuation", async () => {
		const runtime = createNavigationRuntime();
		let replacementSubmit:
			| Promise<
					| { success: true; data: unknown }
					| { success: false; error: string }
			  >
			| undefined;
		setNavigationStateAccess({
			navigate: async () => {
				if (!replacementSubmit) {
					replacementSubmit = runtime.submit(
						"/api/replacement",
						{ method: "POST" },
						{
							dedupeKey: "redirect-effectuation-stale",
							revalidate: false,
						},
					);
				}
				return { didNavigate: true };
			},
			removeNavigation: () => {},
			getNavigations: () => new Map(),
		});
		let fetchCallCount = 0;
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockImplementation(async () => {
				fetchCallCount += 1;
				if (fetchCallCount === 1) {
					return new Response(JSON.stringify({ ok: true }), {
						status: 200,
						headers: {
							"Content-Type": "application/json",
							"X-Vorma-Build-Id": "1",
							"X-Client-Redirect": "/submit-redirect-target",
						},
					});
				}
				return new Response(JSON.stringify({ replacement: true }), {
					status: 200,
					headers: {
						"Content-Type": "application/json",
						"X-Vorma-Build-Id": "1",
					},
				});
			});

		try {
			const firstSubmit = runtime.submit(
				"/api/original",
				{ method: "POST" },
				{
					dedupeKey: "redirect-effectuation-stale",
					revalidate: false,
				},
			);

			await expect(firstSubmit).resolves.toEqual({
				success: false,
				error: "Aborted",
			});
			expect(replacementSubmit).toBeDefined();
			await expect(replacementSubmit).resolves.toEqual({
				success: true,
				data: { replacement: true },
			});
		} finally {
			fetchSpy.mockRestore();
		}
	});

	it("aborts stale deduped submit when replacement starts during auto-revalidation navigation", async () => {
		let replacementSubmit:
			| Promise<
					| { success: true; data: unknown }
					| { success: false; error: string }
			  >
			| undefined;
		let context: SubmitExecutionContext;
		let nextSubmissionOperationID = 1;
		context = {
			submissions: new Map(),
			scheduleStatusUpdate: () => {},
			allocateSubmissionOperationID: () => nextSubmissionOperationID++,
			navigate: async () => {
				if (!replacementSubmit) {
					replacementSubmit = executeSubmitRuntime(
						context,
						"/api/replacement",
						{ method: "POST" },
						{
							dedupeKey: "auto-revalidate-stale",
							revalidate: false,
						},
					);
				}
				return { didNavigate: true };
			},
		};

		let requestCount = 0;
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockImplementation(async () => {
				requestCount += 1;
				if (requestCount === 1) {
					return new Response(JSON.stringify({ stale: true }), {
						status: 200,
						headers: {
							"Content-Type": "application/json",
							"X-Vorma-Build-Id": "1",
						},
					});
				}
				return new Response(JSON.stringify({ fresh: true }), {
					status: 200,
					headers: {
						"Content-Type": "application/json",
						"X-Vorma-Build-Id": "1",
					},
				});
			});

		try {
			const firstSubmit = executeSubmitRuntime(
				context,
				"/api/original",
				{ method: "POST" },
				{
					dedupeKey: "auto-revalidate-stale",
				},
			);

			await expect(firstSubmit).resolves.toEqual({
				success: false,
				error: "Aborted",
			});
			expect(replacementSubmit).toBeDefined();
			await expect(replacementSubmit).resolves.toEqual({
				success: true,
				data: { fresh: true },
			});
			expect(requestCount).toBe(2);
		} finally {
			fetchSpy.mockRestore();
		}
	});

	it("keeps submit ownership when submission map entry instance changes but operation ID stays the same", async () => {
		const parseDeferred = createDeferred<unknown>();
		const submissionKey = "submission:submit-ownership";
		const submissionOperationID = 101;
		let context: SubmitExecutionContext;
		context = {
			submissions: new Map(),
			scheduleStatusUpdate: () => {},
			allocateSubmissionOperationID: () => submissionOperationID,
			navigate: async () => ({ didNavigate: true }),
		};

		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			createSubmitResponse({
				buildID: "1",
				json: () => parseDeferred.promise,
			}),
		);

		try {
			const submitPromise = executeSubmitRuntime(
				context,
				"/api/original",
				{ method: "POST" },
				{
					dedupeKey: "submit-ownership",
					revalidate: false,
				},
			);

			await vi.waitFor(() => {
				expect(context.submissions.has(submissionKey)).toBe(true);
			});
			const currentSubmissionEntry =
				context.submissions.get(submissionKey);
			expect(currentSubmissionEntry).toBeDefined();
			if (!currentSubmissionEntry) {
				throw new Error("Expected current submission entry to exist.");
			}
			context.submissions.set(submissionKey, {
				...currentSubmissionEntry,
			});

			parseDeferred.resolve({ ok: true });

			await expect(submitPromise).resolves.toEqual({
				success: true,
				data: { ok: true },
			});
			expect(context.submissions.has(submissionKey)).toBe(false);
		} finally {
			fetchSpy.mockRestore();
		}
	});
});

describe("findPartialMatchesOnClient guards", () => {
	it("returns null when pattern registry is unavailable", async () => {
		installVormaGlobal({
			patternRegistry: undefined,
			patternToWaitFnMap: {
				"/any": vi.fn(),
			},
		});

		await expect(findPartialMatchesOnClient("/any")).resolves.toBeNull();
	});

	it("returns null when no client loaders are registered", async () => {
		installVormaGlobal({
			patternRegistry: createRegisteredPatternRegistry(["/items"]),
			patternToWaitFnMap: undefined,
		});

		await expect(findPartialMatchesOnClient("/items")).resolves.toBeNull();
	});
});

describe("render runtime initialization guards", () => {
	it("setupClientLoaders tolerates missing route-data snapshots", async () => {
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
		expect(
			__vormaClientGlobal.get("runtimeRouteSnapshot")
				.outermostClientError,
		).toBeUndefined();
	});

	it("registerClientLoaderPattern fails fast without a registry", async () => {
		installVormaGlobal({
			patternRegistry: undefined,
		});

		await expect(registerClientLoaderPattern("/x")).rejects.toThrow(
			"Pattern registry has not been initialized.",
		);
	});
});

describe("fetchRouteData behavior", () => {
	it("always fetches server route data even for client-only manifest routes", async () => {
		vi.doMock("/client-only.js", () => ({
			default: () => null,
		}));
		const waitFn = vi.fn(async ({ serverDataPromise }) => {
			const serverData = await serverDataPromise;
			return serverData.loaderData;
		});
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			new Response(
				JSON.stringify({
					matchedPatterns: ["/client-only"],
					loadersData: [{ fromServer: "fresh" }],
					importURLs: ["/client-only.js"],
					exportKeys: ["default"],
					errorExportKeys: [""],
					hasRootData: false,
					params: {},
					splatValues: [],
					deps: [],
					cssBundles: [],
				}),
				{
					status: 200,
					headers: {
						"Content-Type": "application/json",
						"X-Vorma-Build-Id": "build-42",
					},
				},
			),
		);
		installVormaGlobal({
			buildID: "build-42",
			routeManifest: {
				"/client-only": 0,
			},
			patternRegistry: createRegisteredPatternRegistry(["/client-only"]),
			matchedPatterns: ["/client-only"],
			patternToWaitFnMap: {
				"/client-only": waitFn,
			},
			clientLoadersData: [{ cached: true }],
		});

		const outcome = await fetchRouteData(new AbortController(), {
			href: "/client-only",
			navigationType: "userNavigation",
		});

		expect(fetchSpy).toHaveBeenCalledTimes(1);
		expect(outcome.type).toBe("success");
		if (outcome.type !== "success") {
			throw new Error("Expected a success outcome");
		}
		expect(outcome.response.headers.get("X-Vorma-Build-Id")).toBe(
			"build-42",
		);
		await expect(outcome.waitFnPromise).resolves.toEqual({
			data: [{ fromServer: "fresh" }],
			errorMessage: undefined,
		});
		expect(waitFn).toHaveBeenCalledTimes(1);
	});

	it("uses the current route-data payload server error index for fetch-time client-loader skipping", async () => {
		const waitFnA = vi.fn(async () => "loader-a");
		const waitFnB = vi.fn(async () => "loader-b");
		vi.spyOn(window, "fetch").mockResolvedValue(
			new Response(
				JSON.stringify({
					matchedPatterns: ["/a", "/b"],
					loadersData: [{ fromServer: "a" }, { fromServer: "b" }],
					importURLs: ["", ""],
					exportKeys: ["", ""],
					errorExportKeys: ["", ""],
					hasRootData: false,
					params: {},
					splatValues: [],
					deps: [],
					cssBundles: [],
					outermostServerErrorIdx: 1,
				}),
				{
					status: 200,
					headers: {
						"Content-Type": "application/json",
						"X-Vorma-Build-Id": "1",
					},
				},
			),
		);
		installVormaGlobal({
			outermostServerErrorIdx: 0,
			routeManifest: {
				"/a": 0,
				"/b": 0,
			},
			patternRegistry: createRegisteredPatternRegistry(["/a", "/b"]),
			patternToWaitFnMap: {
				"/a": waitFnA,
				"/b": waitFnB,
			},
		});

		const outcome = await fetchRouteData(new AbortController(), {
			href: "/a",
			navigationType: "userNavigation",
		});
		expect(outcome.type).toBe("success");
		if (outcome.type !== "success") {
			throw new Error("Expected success outcome");
		}

		await expect(outcome.waitFnPromise).resolves.toEqual({
			data: ["loader-a", undefined],
			errorMessage: undefined,
		});
		expect(waitFnA).toHaveBeenCalledTimes(1);
		expect(waitFnB).not.toHaveBeenCalled();
	});

	it("handles server fetch outcomes when client-loader map is undefined", async () => {
		vi.spyOn(window, "fetch").mockResolvedValue(
			new Response(
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
				}),
				{
					status: 200,
					headers: {
						"Content-Type": "application/json",
						"X-Vorma-Build-Id": "1",
					},
				},
			),
		);
		installVormaGlobal({
			routeManifest: undefined,
			patternToWaitFnMap: undefined,
		});

		const outcome = await fetchRouteData(new AbortController(), {
			href: "/server-without-loader-map",
			navigationType: "userNavigation",
		});
		expect(outcome.type).toBe("success");
	});

	it("fails fast when server JSON omits required importURLs", async () => {
		vi.spyOn(window, "fetch").mockResolvedValue(
			new Response(
				JSON.stringify({
					matchedPatterns: [],
					loadersData: [],
					exportKeys: [],
					errorExportKeys: [],
					hasRootData: false,
					params: {},
					splatValues: [],
					deps: [],
					cssBundles: [],
				}),
				{
					status: 200,
					headers: {
						"Content-Type": "application/json",
						"X-Vorma-Build-Id": "1",
					},
				},
			),
		);
		installVormaGlobal({
			routeManifest: undefined,
			patternToWaitFnMap: undefined,
		});

		await expect(
			fetchRouteData(new AbortController(), {
				href: "/missing-import-urls",
				navigationType: "userNavigation",
			}),
		).rejects.toThrow('missing required key "importURLs"');
	});

	it("uses build-id fallback in server route-data request URLs", async () => {
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			new Response(
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
				}),
				{
					status: 200,
					headers: {
						"Content-Type": "application/json",
						"X-Vorma-Build-Id": "1",
					},
				},
			),
		);
		installVormaGlobal({
			buildID: undefined,
			routeManifest: undefined,
		});

		await fetchRouteData(new AbortController(), {
			href: "/server-fallback",
			navigationType: "userNavigation",
		});

		const fetchInput = fetchSpy.mock.calls[0]?.[0] as RequestInfo | URL;
		const fetchURL =
			fetchInput instanceof URL
				? fetchInput
				: new URL(String(fetchInput), window.location.href);
		expect(fetchURL.searchParams.get("vorma_json")).toBe("1");
	});

	it("falls back to build-id 1 when loader server-data response header is missing", async () => {
		const waitFn = vi.fn(async ({ serverDataPromise }) => {
			const serverData = await serverDataPromise;
			return serverData.buildID;
		});
		vi.spyOn(window, "fetch").mockResolvedValue(
			new Response(
				JSON.stringify({
					matchedPatterns: ["/build-id-fallback"],
					loadersData: [{ fromServer: true }],
					importURLs: [""],
					exportKeys: [""],
					errorExportKeys: [""],
					hasRootData: false,
					params: {},
					splatValues: [],
					deps: [],
					cssBundles: [],
				}),
				{
					status: 200,
					headers: {
						"Content-Type": "application/json",
					},
				},
			),
		);
		installVormaGlobal({
			routeManifest: {
				"/build-id-fallback": 1,
			},
			patternRegistry: createRegisteredPatternRegistry([
				"/build-id-fallback",
			]),
			patternToWaitFnMap: {
				"/build-id-fallback": waitFn,
			},
		});

		const outcome = await fetchRouteData(new AbortController(), {
			href: "/build-id-fallback",
			navigationType: "userNavigation",
		});
		expect(outcome.type).toBe("success");
		if (outcome.type !== "success") {
			throw new Error("Expected success outcome");
		}

		await expect(outcome.waitFnPromise).resolves.toEqual({
			data: ["1"],
			errorMessage: undefined,
		});
		expect(waitFn).toHaveBeenCalledTimes(1);
	});

	it("starts only matched client loaders and preserves parent slots without loaders", async () => {
		const waitFn = vi.fn(async ({ serverDataPromise }) => {
			const serverData = await serverDataPromise;
			return serverData.loaderData;
		});
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			new Response(
				JSON.stringify({
					matchedPatterns: ["/parent", "/parent/child"],
					loadersData: [{ root: true }, { fromServer: "child" }],
					importURLs: ["", ""],
					exportKeys: ["", ""],
					errorExportKeys: ["", ""],
					hasRootData: true,
					params: {},
					splatValues: [],
					deps: [],
					cssBundles: [],
				}),
				{
					status: 200,
					headers: {
						"Content-Type": "application/json",
						"X-Vorma-Build-Id": "1",
					},
				},
			),
		);
		installVormaGlobal({
			routeManifest: {
				"/parent": 0,
				"/parent/child": 1,
			},
			patternRegistry: createRegisteredPatternRegistry([
				"/parent",
				"/parent/child",
			]),
			patternToWaitFnMap: {
				"/parent/child": waitFn,
			},
			matchedPatterns: [],
		});

		const outcome = await fetchRouteData(new AbortController(), {
			href: "/parent/child",
			navigationType: "userNavigation",
		});
		expect(fetchSpy).toHaveBeenCalled();
		expect(outcome.type).toBe("success");
		if (outcome.type !== "success") {
			throw new Error("Expected success outcome");
		}
		await expect(outcome.waitFnPromise).resolves.toEqual({
			data: [undefined, { fromServer: "child" }],
			errorMessage: undefined,
		});
		expect(waitFn).toHaveBeenCalledTimes(1);
	});

	it("throws when matched route arrays contain sparse entries", () => {
		expect(() =>
			getMatchedPatternsOrThrow({
				matches: [undefined as any, createMatch("/sparse-loader")],
				context: "Partial route matcher",
			}),
		).toThrow(
			"Partial route matcher returned a sparse matches array at index 0.",
		);
	});

	it("throws when matched route arrays contain empty route patterns", () => {
		expect(() =>
			getMatchedPatternsOrThrow({
				matches: [createMatch("")],
				context: "Partial route matcher",
			}),
		).toThrow(
			"Partial route matcher returned an empty route pattern at index 0.",
		);
	});

	it("aborts waiting client loaders when server JSON omits required matched loader arrays", async () => {
		const unavailableNames: string[] = [];
		const waitFn = vi.fn(async ({ serverDataPromise }) => {
			try {
				await serverDataPromise;
				return "unexpected";
			} catch (error) {
				unavailableNames.push((error as Error).name);
				return "fallback";
			}
		});
		vi.spyOn(window, "fetch").mockResolvedValue(
			new Response(
				JSON.stringify({
					importURLs: [],
					exportKeys: [],
					errorExportKeys: [],
					hasRootData: false,
					params: {},
					splatValues: [],
				}),
				{
					status: 200,
					headers: {
						"Content-Type": "application/json",
					},
				},
			),
		);
		installVormaGlobal({
			routeManifest: {
				"/needs-server": 1,
			},
			patternRegistry: createRegisteredPatternRegistry(["/needs-server"]),
			patternToWaitFnMap: {
				"/needs-server": waitFn,
			},
			matchedPatterns: [],
		});

		await expect(
			fetchRouteData(new AbortController(), {
				href: "/needs-server",
				navigationType: "userNavigation",
			}),
		).rejects.toThrow('missing required key "matchedPatterns"');
		await Promise.resolve();
		await Promise.resolve();
		expect(waitFn).toHaveBeenCalledTimes(1);
		expect(unavailableNames).toEqual(["AbortError"]);
	});

	it("maps server fetch rejections to unavailable server-data for client loaders", async () => {
		const unavailableNames: string[] = [];
		const waitFn = vi.fn(async ({ serverDataPromise }) => {
			try {
				await serverDataPromise;
				return "unexpected";
			} catch (error) {
				unavailableNames.push((error as Error).name);
				return "fallback";
			}
		});
		const logErrorSpy = vi
			.spyOn(console, "error")
			.mockImplementation(() => {});
		vi.spyOn(window, "fetch").mockRejectedValue(
			new Error("network-failure"),
		);
		installVormaGlobal({
			routeManifest: {
				"/rejecting-server": 1,
			},
			patternRegistry: createRegisteredPatternRegistry([
				"/rejecting-server",
			]),
			patternToWaitFnMap: {
				"/rejecting-server": waitFn,
			},
			matchedPatterns: [],
		});

		await expect(
			fetchRouteData(new AbortController(), {
				href: "/rejecting-server",
				navigationType: "userNavigation",
			}),
		).rejects.toThrow("network-failure");
		await Promise.resolve();
		expect(waitFn).toHaveBeenCalledTimes(1);
		expect(unavailableNames).toEqual(["AbortError"]);
		expect(logErrorSpy).toHaveBeenCalled();
		logErrorSpy.mockRestore();
	});

	it("defaults omitted deps and cssBundles to empty arrays in production", async () => {
		const originalDev = import.meta.env.DEV;
		(import.meta.env as any).DEV = false;
		try {
			vi.spyOn(window, "fetch").mockResolvedValue(
				new Response(
					JSON.stringify({
						matchedPatterns: [],
						loadersData: [],
						importURLs: [],
						exportKeys: [],
						errorExportKeys: [],
						hasRootData: false,
						params: {},
						splatValues: [],
					}),
					{
						status: 200,
						headers: {
							"Content-Type": "application/json",
							"X-Vorma-Build-Id": "1",
						},
					},
				),
			);
			installVormaGlobal({
				routeManifest: undefined,
			});

			const outcome = await fetchRouteData(new AbortController(), {
				href: "/prod-no-assets",
				navigationType: "userNavigation",
			});
			expect(outcome.type).toBe("success");
			if (outcome.type !== "success") {
				throw new Error("Expected success outcome");
			}
			expect(outcome.json.deps).toEqual([]);
			expect(outcome.json.cssBundles).toEqual([]);
			expect(outcome.preloadPlan).toEqual({
				moduleDependencies: [],
				cssBundles: [],
			});
		} finally {
			(import.meta.env as any).DEV = originalDev;
		}
	});

	it("preloads only truthy production deps", async () => {
		const originalDev = import.meta.env.DEV;
		(import.meta.env as any).DEV = false;
		try {
			vi.spyOn(window, "fetch").mockResolvedValue(
				new Response(
					JSON.stringify({
						matchedPatterns: ["/prod-deps"],
						loadersData: [{}],
						importURLs: ["/ignored-in-prod.js"],
						exportKeys: [""],
						errorExportKeys: [""],
						hasRootData: false,
						params: {},
						splatValues: [],
						deps: ["", "/prod-dep.js"],
						cssBundles: [],
					}),
					{
						status: 200,
						headers: {
							"Content-Type": "application/json",
							"X-Vorma-Build-Id": "1",
						},
					},
				),
			);
			installVormaGlobal({
				routeManifest: undefined,
			});

			const outcome = await fetchRouteData(new AbortController(), {
				href: "/prod-deps",
				navigationType: "userNavigation",
			});
			expect(outcome.type).toBe("success");
			if (outcome.type !== "success") {
				throw new Error("Expected success outcome");
			}
			expect(outcome.preloadPlan).toEqual({
				moduleDependencies: ["/prod-dep.js"],
				cssBundles: [],
			});
		} finally {
			(import.meta.env as any).DEV = originalDev;
		}
	});

	it("does not preload stale deps or update build ID from superseded browser-history responses", async () => {
		const originalDev = import.meta.env.DEV;
		(import.meta.env as any).DEV = false;
		const preloadModuleSpy = vi
			.spyOn(renderRuntimeModule.AssetManager, "preloadModule")
			.mockImplementation(() => {});
		const staleNavigationFetch = createDeferred<Response>();
		const winnerNavigationFetch = createDeferred<Response>();
		const buildIDEvents: Array<{ oldID: string; newID: string }> = [];
		const removeBuildIDListener = addBuildIDListener((event) => {
			buildIDEvents.push(event.detail);
		});
		let fetchCallCount = 0;
		const fetchSpy = vi.spyOn(window, "fetch").mockImplementation(() => {
			fetchCallCount++;
			if (fetchCallCount === 1) {
				return staleNavigationFetch.promise;
			}
			if (fetchCallCount === 2) {
				return winnerNavigationFetch.promise;
			}

			throw new Error(`Unexpected fetch call #${fetchCallCount}`);
		});

		try {
			const runtime = createNavigationRuntime();
			const staleNavigation = runtime.navigate({
				href: "/stale-browser-history-race",
				navigationType: "browserHistory",
			});
			await Promise.resolve();

			const winnerNavigation = runtime.navigate({
				href: "/winner-browser-history-race",
				navigationType: "userNavigation",
			});
			await Promise.resolve();

			winnerNavigationFetch.resolve(
				new Response(
					JSON.stringify({
						matchedPatterns: [],
						loadersData: [],
						importURLs: [],
						exportKeys: [],
						errorExportKeys: [],
						hasRootData: false,
						params: {},
						splatValues: [],
						deps: ["/winner-dep.js"],
						cssBundles: [],
						outermostServerError: undefined,
						outermostServerErrorIdx: undefined,
						title: { dangerousInnerHTML: "Winner" },
						metaHeadEls: undefined,
						restHeadEls: undefined,
					}),
					{
						status: 200,
						headers: {
							"Content-Type": "application/json",
							"X-Vorma-Build-Id": "1",
						},
					},
				),
			);
			await winnerNavigation;

			staleNavigationFetch.resolve(
				new Response(
					JSON.stringify({
						matchedPatterns: [],
						loadersData: [],
						importURLs: [],
						exportKeys: [],
						errorExportKeys: [],
						hasRootData: false,
						params: {},
						splatValues: [],
						deps: ["/stale-dep.js"],
						cssBundles: [],
						outermostServerError: undefined,
						outermostServerErrorIdx: undefined,
						title: { dangerousInnerHTML: "Stale" },
						metaHeadEls: undefined,
						restHeadEls: undefined,
					}),
					{
						status: 200,
						headers: {
							"Content-Type": "application/json",
							"X-Vorma-Build-Id": "stale-browser-history-build",
						},
					},
				),
			);
			await staleNavigation;
			await Promise.resolve();

			expect(preloadModuleSpy).toHaveBeenCalledTimes(1);
			expect(preloadModuleSpy).toHaveBeenCalledWith("/winner-dep.js");
			expect(
				__vormaClientGlobal.get("runtimeRouteSnapshot").buildID,
			).toBe("1");
			expect(buildIDEvents).toEqual([]);
		} finally {
			removeBuildIDListener();
			fetchSpy.mockRestore();
			preloadModuleSpy.mockRestore();
			(import.meta.env as any).DEV = originalDev;
		}
	});

	it("throws explicit error when server route-data request returns 304", async () => {
		const logErrorSpy = vi
			.spyOn(console, "error")
			.mockImplementation(() => {});
		vi.spyOn(window, "fetch").mockResolvedValue(
			new Response(null, {
				status: 304,
				headers: {
					"X-Vorma-Build-Id": "1",
				},
			}),
		);
		installVormaGlobal({
			routeManifest: undefined,
		});

		await expect(
			fetchRouteData(new AbortController(), {
				href: "/not-modified",
				navigationType: "userNavigation",
			}),
		).rejects.toThrow("Fetch returned 304 without route JSON payload.");
		expect(logErrorSpy).toHaveBeenCalled();
		logErrorSpy.mockRestore();
	});
});
