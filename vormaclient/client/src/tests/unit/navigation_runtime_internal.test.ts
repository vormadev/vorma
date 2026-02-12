import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import * as matcherFindNestedModule from "vorma/kit/matcher/find-nested";
import {
	createPatternRegistry,
	registerPattern,
} from "vorma/kit/matcher/register";
import {
	createNavigationRuntime,
	deleteNavigationFromSlots,
	findNavigationEntryInSlots,
	transitionNavigationPhaseInSlots,
	type NavigationSlots,
} from "../../core/navigation/runtime.ts";
import {
	executeSubmitRuntime,
	type SubmitExecutionContext,
} from "../../core/navigation/runtime_submit.ts";
import {
	handleNavigationOutcome,
	processSuccessfulNavigationRuntime,
} from "../../core/navigation/runtime_navigation_outcome.ts";
import type { NavigationEntry } from "../../core/navigation/types.ts";
import { VORMA_SYMBOL } from "../../app/context.ts";
import { fetchRouteData } from "../../core/navigation/fetch_route_data.ts";
import { canSkipServerFetch } from "../../core/navigation/fetch_route_data_skip.ts";
import {
	isSkipEligibilityViolated,
	type SkipCheckContext,
} from "../../core/navigation/fetch_route_data_skip_match.ts";
import {
	__registerClientLoaderPattern,
	findPartialMatchesOnClient,
	setupClientLoaders,
} from "../../core/render_runtime.ts";
import * as renderRuntimeModule from "../../core/render_runtime.ts";
import * as redirectsModule from "../../core/redirects.ts";
import type { NavigationOutcome } from "../../core/navigation/types.ts";
import { addBuildIDListener } from "../../platform/events.ts";

const TEST_VORMA_APP_CONFIG = {
	actionsRouterMountRoot: "/api/",
	actionsDynamicRune: ":",
	actionsSplatRune: "*",
	loadersDynamicRune: ":",
	loadersSplatRune: "*",
	loadersExplicitIndexSegment: "_index",
};

function createEntry(props: {
	targetUrl: string;
	type: NavigationEntry["type"];
	intent: NavigationEntry["intent"];
}): NavigationEntry {
	return {
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
		cssBundlePromises?: Array<Promise<unknown>>;
		waitFnPromise?: Promise<{
			data: Array<unknown>;
			errorMessage?: string;
		}>;
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
				"X-Vorma-Build-Id": "1",
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
		cssBundlePromises: overrides.cssBundlePromises ?? [],
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
});

afterEach(() => {
	vi.restoreAllMocks();
	delete (globalThis as any)[VORMA_SYMBOL];
});

describe("navigation bookkeeping key aliasing", () => {
	it("finds active navigation by same data target when hash differs", () => {
		const active = createEntry({
			targetUrl: "http://localhost:3000/alias#first",
			type: "userNavigation",
			intent: "navigate",
		});
		const slots: NavigationSlots = {
			activeNavigation: active,
			prefetchCache: new Map(),
			pendingRevalidation: null,
		};

		const found = findNavigationEntryInSlots(
			slots,
			"http://localhost:3000/alias#second",
		);
		expect(found).toBe(active);
	});

	it("does not alias active navigation when search params differ", () => {
		const active = createEntry({
			targetUrl: "http://localhost:3000/alias?tab=a#first",
			type: "userNavigation",
			intent: "navigate",
		});
		const slots: NavigationSlots = {
			activeNavigation: active,
			prefetchCache: new Map(),
			pendingRevalidation: null,
		};

		const found = findNavigationEntryInSlots(
			slots,
			"http://localhost:3000/alias?tab=b#first",
		);
		expect(found).toBeUndefined();
	});

	it("deletes prefetch entry by same data target when hash differs", () => {
		const prefetch = createEntry({
			targetUrl: "http://localhost:3000/prefetch#first",
			type: "prefetch",
			intent: "none",
		});
		const slots: NavigationSlots = {
			activeNavigation: null,
			prefetchCache: new Map([[prefetch.targetUrl, prefetch]]),
			pendingRevalidation: null,
		};
		const onStatusRelevantChange = vi.fn();

		const deleted = deleteNavigationFromSlots(
			slots,
			"http://localhost:3000/prefetch#second",
			onStatusRelevantChange,
		);

		expect(deleted).toBe(true);
		expect(slots.prefetchCache.size).toBe(0);
		expect(onStatusRelevantChange).not.toHaveBeenCalled();
	});

	it("does not delete prefetch entry when search params differ", () => {
		const prefetch = createEntry({
			targetUrl: "http://localhost:3000/prefetch?tab=a#first",
			type: "prefetch",
			intent: "none",
		});
		const slots: NavigationSlots = {
			activeNavigation: null,
			prefetchCache: new Map([[prefetch.targetUrl, prefetch]]),
			pendingRevalidation: null,
		};
		const onStatusRelevantChange = vi.fn();

		const deleted = deleteNavigationFromSlots(
			slots,
			"http://localhost:3000/prefetch?tab=b#first",
			onStatusRelevantChange,
		);

		expect(deleted).toBe(false);
		expect(slots.prefetchCache.size).toBe(1);
		expect(onStatusRelevantChange).not.toHaveBeenCalled();
	});

	it("transitions pending revalidation phase by same data target alias", () => {
		const pendingRevalidation = createEntry({
			targetUrl: "http://localhost:3000/revalidate#first",
			type: "revalidation",
			intent: "revalidate",
		});
		const slots: NavigationSlots = {
			activeNavigation: null,
			prefetchCache: new Map(),
			pendingRevalidation,
		};
		const onStatusRelevantChange = vi.fn();

		transitionNavigationPhaseInSlots(
			slots,
			"http://localhost:3000/revalidate#second",
			"waiting",
			onStatusRelevantChange,
		);

		expect(slots.pendingRevalidation?.phase).toBe("waiting");
		expect(onStatusRelevantChange).toHaveBeenCalledTimes(1);
	});

	it("does not transition pending revalidation phase when search params differ", () => {
		const pendingRevalidation = createEntry({
			targetUrl: "http://localhost:3000/revalidate?view=a#first",
			type: "revalidation",
			intent: "revalidate",
		});
		const slots: NavigationSlots = {
			activeNavigation: null,
			prefetchCache: new Map(),
			pendingRevalidation,
		};
		const onStatusRelevantChange = vi.fn();

		transitionNavigationPhaseInSlots(
			slots,
			"http://localhost:3000/revalidate?view=b#first",
			"waiting",
			onStatusRelevantChange,
		);

		expect(slots.pendingRevalidation?.phase).toBe("fetching");
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
});

describe("navigation runtime outcome stale control guards", () => {
	it("does not delete current entry when stale aborted outcome resolves for same target", async () => {
		const targetUrl = new URL(
			"/stale-control-aborted",
			window.location.href,
		).href;
		const staleControlPromise = Promise.resolve({
			type: "aborted",
		} as NavigationOutcome);
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
			controlPromise: staleControlPromise,
		});

		expect(result).toEqual({ didNavigate: false });
		expect(deleteNavigation).not.toHaveBeenCalled();
		expect(processSuccessfulNavigation).not.toHaveBeenCalled();
	});

	it("does not process success when stale control promise no longer owns target", async () => {
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
		const staleControlPromise = Promise.resolve(staleOutcome);
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
			controlPromise: staleControlPromise,
		});

		expect(result).toEqual({ didNavigate: false });
		expect(deleteNavigation).not.toHaveBeenCalled();
		expect(processSuccessfulNavigation).not.toHaveBeenCalled();
	});

	it("deletes target for current aborted outcome when control promise matches", async () => {
		const targetUrl = new URL(
			"/current-control-aborted",
			window.location.href,
		).href;
		const controlPromise = Promise.resolve({
			type: "aborted",
		} as NavigationOutcome);
		const currentEntry = createEntry({
			targetUrl,
			type: "userNavigation",
			intent: "navigate",
		});
		currentEntry.control.promise = controlPromise;

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
			controlPromise,
		});

		expect(result).toEqual({ didNavigate: false });
		expect(deleteNavigation).toHaveBeenCalledOnce();
		expect(deleteNavigation).toHaveBeenCalledWith(targetUrl);
		expect(processSuccessfulNavigation).not.toHaveBeenCalled();
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

	it("continues successful processing when css preload promises reject", async () => {
		const fetchSpy = createAbortAwareNeverResolvingFetchSpy();
		const reRenderSpy = vi
			.spyOn(renderRuntimeModule, "__reRenderApp")
			.mockResolvedValue();
		const consoleErrorSpy = vi
			.spyOn(console, "error")
			.mockImplementation(() => {});

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

			const cssPreloadError = new Error("css preload failed");
			const outcome = createSuccessNavigationOutcome({
				cssBundlePromises: [Promise.reject(cssPreloadError)],
				props: {
					href: targetUrl,
					navigationType: "browserHistory",
				},
			});

			await expect(
				runtime.processSuccessfulNavigation(outcome, entry),
			).resolves.toBeUndefined();
			expect(reRenderSpy).toHaveBeenCalledTimes(1);
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
			reRenderSpy.mockRestore();
			consoleErrorSpy.mockRestore();
		}
	});

	it("defaults missing export keys when applying response artifacts for matching builds", async () => {
		const fetchSpy = createAbortAwareNeverResolvingFetchSpy();
		const reRenderSpy = vi
			.spyOn(renderRuntimeModule, "__reRenderApp")
			.mockResolvedValue();

		try {
			const runtime = createNavigationRuntime();
			runtime.beginNavigation({
				href: "/module-map-default-key",
				navigationType: "browserHistory",
			});
			const targetUrl = new URL(
				"/module-map-default-key",
				window.location.href,
			).href;
			const entry = runtime.getNavigation(targetUrl);
			expect(entry).toBeDefined();
			if (!entry) return;

			const outcome = createSuccessNavigationOutcome({
				props: {
					href: targetUrl,
					navigationType: "browserHistory",
				},
			});
			outcome.json.matchedPatterns = ["/module-map-pattern"];
			outcome.json.importURLs = ["/module-map.js"];
			outcome.json.exportKeys = [""];
			outcome.json.errorExportKeys = [""];

			await expect(
				runtime.processSuccessfulNavigation(outcome, entry),
			).resolves.toBeUndefined();

			expect(
				(globalThis as any)[VORMA_SYMBOL].clientModuleMap[
					"/module-map-pattern"
				],
			).toEqual({
				importURL: "/module-map.js",
				exportKey: "default",
				errorExportKey: "",
			});
		} finally {
			fetchSpy.mockRestore();
			reRenderSpy.mockRestore();
		}
	});

	it("tolerates missing response-artifact arrays when build IDs match", async () => {
		const fetchSpy = createAbortAwareNeverResolvingFetchSpy();
		const reRenderSpy = vi
			.spyOn(renderRuntimeModule, "__reRenderApp")
			.mockResolvedValue();

		try {
			const runtime = createNavigationRuntime();
			runtime.beginNavigation({
				href: "/module-map-missing-arrays",
				navigationType: "browserHistory",
			});
			const targetUrl = new URL(
				"/module-map-missing-arrays",
				window.location.href,
			).href;
			const entry = runtime.getNavigation(targetUrl);
			expect(entry).toBeDefined();
			if (!entry) return;

			const outcome = createSuccessNavigationOutcome({
				props: {
					href: targetUrl,
					navigationType: "browserHistory",
				},
			});
			(globalThis as any)[VORMA_SYMBOL].clientModuleMap = undefined;
			outcome.json.matchedPatterns = undefined as any;
			outcome.json.importURLs = undefined as any;
			outcome.json.exportKeys = undefined as any;
			outcome.json.errorExportKeys = undefined as any;

			await expect(
				runtime.processSuccessfulNavigation(outcome, entry),
			).resolves.toBeUndefined();

			expect((globalThis as any)[VORMA_SYMBOL].clientModuleMap).toEqual(
				{},
			);
		} finally {
			fetchSpy.mockRestore();
			reRenderSpy.mockRestore();
		}
	});

	it("marks navigation complete and rethrows when render fails", async () => {
		const fetchSpy = createAbortAwareNeverResolvingFetchSpy();
		const renderError = new Error("render failed");
		const reRenderSpy = vi
			.spyOn(renderRuntimeModule, "__reRenderApp")
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
			reRenderSpy.mockRestore();
			consoleErrorSpy.mockRestore();
		}
	});

	it("rethrows abort render failures without logging completion errors", async () => {
		const fetchSpy = createAbortAwareNeverResolvingFetchSpy();
		const renderAbortError = new Error("Aborted");
		renderAbortError.name = "AbortError";
		const reRenderSpy = vi
			.spyOn(renderRuntimeModule, "__reRenderApp")
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
			reRenderSpy.mockRestore();
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
				(
					_nextTargetUrl: string,
					phase: "fetching" | "waiting" | "rendering" | "complete",
				) => {
					if (phase === "waiting") {
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
			expect(transitionPhase).toHaveBeenCalledWith(targetUrl, "waiting");
			expect(deleteNavigation).not.toHaveBeenCalled();
		} finally {
			reRenderSpy.mockRestore();
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
			const olderSuccessProcessingPromise =
				runtime.processSuccessfulNavigation(
					createSuccessNavigationOutcome({
						waitFnPromise: waitDeferred.promise,
						props: {
							href: targetUrl,
							navigationType: "browserHistory",
						},
					}),
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
		}
	});

	it("does not mark a newer same-target entry complete from a stale render onFinish callback", async () => {
		const fetchSpy = createAbortAwareNeverResolvingFetchSpy();
		const renderDeferred = createDeferred<void>();
		const renderStartedDeferred = createDeferred<void>();
		let staleOnFinishCallback: (() => void) | undefined;
		const reRenderSpy = vi
			.spyOn(renderRuntimeModule, "__reRenderApp")
			.mockImplementation(async (props) => {
				staleOnFinishCallback = props.onFinish;
				renderStartedDeferred.resolve();
				await renderDeferred.promise;
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

			await renderStartedDeferred.promise;
			expect(staleOnFinishCallback).toBeDefined();

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

			staleOnFinishCallback?.();
			expect(secondEntry.phase).toBe("fetching");

			renderDeferred.resolve();
			await expect(
				olderSuccessProcessingPromise,
			).resolves.toBeUndefined();
			expect(runtime.getNavigation(targetUrl)).toBe(secondEntry);

			runtime.clearAll();
			await expect(firstControlPromise).resolves.toMatchObject({
				name: "AbortError",
			});
			await expect(secondControlPromise).resolves.toMatchObject({
				name: "AbortError",
			});
		} finally {
			renderDeferred.resolve();
			fetchSpy.mockRestore();
			reRenderSpy.mockRestore();
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

	it("returns explicit submit error when redirect handling produces no response object", async () => {
		const handleRedirectsSpy = vi
			.spyOn(redirectsModule, "handleRedirects")
			.mockResolvedValue({
				redirectData: null,
				response: undefined,
			} as any);
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
				success: false,
				error: "Submit request completed without a response.",
			});
		} finally {
			handleRedirectsSpy.mockRestore();
			consoleErrorSpy.mockRestore();
		}
	});

	it("returns explicit submit error when redirect effectuation fails", async () => {
		const handleRedirectsSpy = vi
			.spyOn(redirectsModule, "handleRedirects")
			.mockResolvedValue({
				redirectData: {
					status: "should",
					shouldRedirectStrategy: "soft",
					latestBuildID: "1",
					href: "/submit-redirect-target",
					hrefDetails: {
						isHTTP: true,
						isInternal: true,
						isExternal: false,
						absoluteURL:
							"http://localhost:3000/submit-redirect-target",
					},
				},
				response: createSubmitResponse({
					buildID: "1",
					json: async () => ({ ok: true }),
				}),
			} as any);
		const effectuateRedirectSpy = vi
			.spyOn(redirectsModule, "effectuateRedirectDataResult")
			.mockResolvedValue(null);

		try {
			const runtime = createNavigationRuntime();
			const result = await runtime.submit(
				"/api/failed-submit-redirect",
				{ method: "POST" },
				{ revalidate: false },
			);

			expect(effectuateRedirectSpy).toHaveBeenCalledOnce();
			expect(result).toEqual({
				success: false,
				error: "Redirect failed",
			});
		} finally {
			handleRedirectsSpy.mockRestore();
			effectuateRedirectSpy.mockRestore();
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
		const handleRedirectsSpy = vi
			.spyOn(redirectsModule, "handleRedirects")
			.mockResolvedValue({
				redirectData: {
					status: "should",
					shouldRedirectStrategy: "soft",
					latestBuildID: "1",
					href: "/submit-redirect-target",
					hrefDetails: {
						isHTTP: true,
						isInternal: true,
						isExternal: false,
						absoluteURL:
							"http://localhost:3000/submit-redirect-target",
					},
				},
				response: createSubmitResponse({
					buildID: "1",
					json: async () => ({ ok: true }),
				}),
			} as any);
		const effectuateRedirectSpy = vi
			.spyOn(redirectsModule, "effectuateRedirectDataResult")
			.mockImplementation(async () => {
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
				return {
					status: "did",
					href: "/submit-redirect-target",
					hrefDetails: {
						isHTTP: true,
						isInternal: true,
						isExternal: false,
						absoluteURL:
							"http://localhost:3000/submit-redirect-target",
					},
				} as any;
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
			expect(effectuateRedirectSpy).toHaveBeenCalled();
			expect(replacementSubmit).toBeDefined();
			await expect(replacementSubmit).resolves.toEqual({
				success: true,
				data: undefined,
			});
		} finally {
			handleRedirectsSpy.mockRestore();
			effectuateRedirectSpy.mockRestore();
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
		context = {
			submissions: new Map(),
			scheduleStatusUpdate: () => {},
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
		const handleRedirectsSpy = vi
			.spyOn(redirectsModule, "handleRedirects")
			.mockImplementation(async () => {
				requestCount++;
				if (requestCount === 1) {
					return {
						redirectData: null,
						response: createSubmitResponse({
							buildID: "1",
							json: async () => ({ stale: true }),
						}),
					} as any;
				}
				return {
					redirectData: null,
					response: createSubmitResponse({
						buildID: "1",
						json: async () => ({ fresh: true }),
					}),
				} as any;
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
			handleRedirectsSpy.mockRestore();
		}
	});
});

function buildContext(
	targetHref: string,
	overrides: Partial<SkipCheckContext> = {},
): SkipCheckContext {
	return {
		routeManifest: { "/items": 1 },
		patternRegistry: createRegisteredPatternRegistry(["/items"]),
		patternToWaitFnMap: {},
		clientModuleMap: {
			"/items": {
				importURL: "/items.js",
				exportKey: "default",
				errorExportKey: "",
			},
		},
		currentMatchedPatterns: ["/items"],
		currentParams: {},
		currentSplatValues: [],
		currentLoadersData: [{}],
		url: new URL(targetHref),
		matchResult: {
			matches: [createMatch("/items")],
			params: {},
			splatValues: [],
		},
		...overrides,
	};
}

describe("skip server fetch eligibility", () => {
	it("treats query order changes as changed for skip gating", () => {
		window.history.replaceState({}, "", "/items?a=1&b=2");
		const ctx = buildContext("http://localhost:3000/items?b=2&a=1");

		expect(isSkipEligibilityViolated(ctx)).toBe(true);
	});

	it("allows skip gating when query string is exactly unchanged", () => {
		window.history.replaceState({}, "", "/items?a=1&b=2");
		const ctx = buildContext("http://localhost:3000/items?a=1&b=2");

		expect(isSkipEligibilityViolated(ctx)).toBe(false);
	});

	it("blocks skip when a previously matched server-loader route is removed", () => {
		const ctx = buildContext("http://localhost:3000/new", {
			routeManifest: {
				"/old": 1,
				"/new": 1,
			},
			currentMatchedPatterns: ["/old"],
			matchResult: {
				matches: [createMatch("/new")],
				params: {},
				splatValues: [],
			},
		});

		expect(isSkipEligibilityViolated(ctx)).toBe(true);
	});

	it("blocks skip when outermost loader dynamic params change", () => {
		const ctx = buildContext("http://localhost:3000/items/2", {
			routeManifest: { "/items/:id": 1 },
			currentMatchedPatterns: ["/items/:id"],
			currentParams: { id: "1" },
			matchResult: {
				matches: [
					createMatch("/items/:id", {
						normalizedSegments: [
							{
								segType: "dynamic",
								normalizedVal: ":id",
							},
						],
					}),
				],
				params: { id: "2" },
				splatValues: [],
			},
		});

		expect(isSkipEligibilityViolated(ctx)).toBe(true);
	});

	it("does not block skip when outermost loader dynamic params are unchanged", () => {
		const ctx = buildContext("http://localhost:3000/items/1", {
			routeManifest: { "/items/:id": 1 },
			currentMatchedPatterns: ["/items/:id"],
			currentParams: { id: "1" },
			matchResult: {
				matches: [
					createMatch("/items/:id", {
						normalizedSegments: [
							{
								segType: "dynamic",
								normalizedVal: ":id",
							},
						],
					}),
				],
				params: { id: "1" },
				splatValues: [],
			},
		});

		expect(isSkipEligibilityViolated(ctx)).toBe(false);
	});

	it("blocks skip when outermost loader splat values change", () => {
		const ctx = buildContext("http://localhost:3000/files/a/b", {
			routeManifest: { "/files/*": 1 },
			currentMatchedPatterns: ["/files/*"],
			currentSplatValues: ["a"],
			matchResult: {
				matches: [
					createMatch("/files/*", {
						lastSegType: "splat",
					}),
				],
				params: {},
				splatValues: ["a", "b"],
			},
		});

		expect(isSkipEligibilityViolated(ctx)).toBe(true);
	});

	it("does not block skip when outermost loader splat values are unchanged", () => {
		const ctx = buildContext("http://localhost:3000/files/a/b", {
			routeManifest: { "/files/*": 1 },
			currentMatchedPatterns: ["/files/*"],
			currentSplatValues: ["a", "b"],
			matchResult: {
				matches: [
					createMatch("/files/*", {
						lastSegType: "splat",
					}),
				],
				params: {},
				splatValues: ["a", "b"],
			},
		});

		expect(isSkipEligibilityViolated(ctx)).toBe(false);
	});

	it("does not block skip when no loaders are present, even if search changes", () => {
		window.history.replaceState({}, "", "/items?mode=a");
		const ctx = buildContext("http://localhost:3000/items?mode=b", {
			routeManifest: { "/items": 0 },
		});

		expect(isSkipEligibilityViolated(ctx)).toBe(false);
	});
});

describe("canSkipServerFetch decisions", () => {
	it("cannot skip when route manifest is unavailable", () => {
		installVormaGlobal({
			routeManifest: undefined,
			patternRegistry: createRegisteredPatternRegistry(["/items"]),
		});

		expect(canSkipServerFetch("http://localhost:3000/items").canSkip).toBe(
			false,
		);
	});

	it("cannot skip when pattern registry is unavailable", () => {
		installVormaGlobal({
			routeManifest: { "/items": 1 },
			patternRegistry: undefined,
		});

		expect(canSkipServerFetch("http://localhost:3000/items").canSkip).toBe(
			false,
		);
	});

	it("cannot skip when target URL does not match any route", () => {
		installVormaGlobal({
			routeManifest: { "/other": 1 },
			patternRegistry: createRegisteredPatternRegistry(["/other"]),
		});

		expect(
			canSkipServerFetch("http://localhost:3000/not-registered").canSkip,
		).toBe(false);
	});

	it("cannot skip when target introduces a newly matched client loader", () => {
		installVormaGlobal({
			routeManifest: {
				"/existing": 0,
				"/new": 0,
			},
			patternRegistry: createRegisteredPatternRegistry([
				"/existing",
				"/new",
			]),
			patternToWaitFnMap: {
				"/new": vi.fn(),
			},
			matchedPatterns: ["/existing"],
			clientModuleMap: {
				"/existing": {
					importURL: "/existing.js",
					exportKey: "default",
					errorExportKey: "",
				},
				"/new": {
					importURL: "/new.js",
					exportKey: "default",
					errorExportKey: "",
				},
			},
		});

		expect(canSkipServerFetch("http://localhost:3000/new").canSkip).toBe(
			false,
		);
	});

	it("throws when matcher returns sparse route matches during skip checks", () => {
		const findNestedMatchesSpy = vi
			.spyOn(matcherFindNestedModule, "findNestedMatches")
			.mockReturnValue({
				params: {},
				splatValues: [],
				matches: [undefined as any, createMatch("/sparse")],
			} as any);
		installVormaGlobal({
			routeManifest: {
				"/sparse": 0,
			},
			patternRegistry: createRegisteredPatternRegistry(["/sparse"]),
			clientModuleMap: {
				"/sparse": {
					importURL: "/sparse.js",
					exportKey: "default",
					errorExportKey: "",
				},
			},
		});

		try {
			expect(() =>
				canSkipServerFetch("http://localhost:3000/sparse"),
			).toThrow(
				"Route matcher returned a sparse matches array at index 0.",
			);
		} finally {
			findNestedMatchesSpy.mockRestore();
		}
	});

	it("throws when matcher returns an empty route pattern", () => {
		const findNestedMatchesSpy = vi
			.spyOn(matcherFindNestedModule, "findNestedMatches")
			.mockReturnValue({
				params: {},
				splatValues: [],
				matches: [createMatch("")],
			} as any);
		installVormaGlobal({
			routeManifest: {
				"": 0,
			},
			patternRegistry: createRegisteredPatternRegistry(["/placeholder"]),
			clientModuleMap: {
				"": {
					importURL: "/empty-pattern.js",
					exportKey: "default",
					errorExportKey: "",
				},
			},
		});

		try {
			expect(() =>
				canSkipServerFetch("http://localhost:3000/placeholder"),
			).toThrow(
				"Route matcher returned an empty route pattern at index 0.",
			);
		} finally {
			findNestedMatchesSpy.mockRestore();
		}
	});

	it("returns client-only skip payload for routes without server loaders", () => {
		installVormaGlobal({
			routeManifest: {
				"/client-only": 0,
			},
			patternRegistry: createRegisteredPatternRegistry(["/client-only"]),
			clientModuleMap: {
				"/client-only": {
					importURL: "/client-only.js",
					exportKey: "default",
					errorExportKey: "",
				},
			},
		});

		const result = canSkipServerFetch("http://localhost:3000/client-only");
		expect(result.canSkip).toBe(true);
		if (!result.canSkip) {
			throw new Error(
				"Expected canSkipServerFetch to return canSkip=true",
			);
		}

		expect(result.importURLs).toEqual(["/client-only.js"]);
		expect(result.exportKeys).toEqual(["default"]);
		expect(result.loadersData).toEqual([undefined]);
	});

	it("cannot skip when matched route has no client module info", () => {
		installVormaGlobal({
			routeManifest: {
				"/missing-module": 0,
			},
			patternRegistry: createRegisteredPatternRegistry([
				"/missing-module",
			]),
			clientModuleMap: {},
		});

		expect(
			canSkipServerFetch("http://localhost:3000/missing-module").canSkip,
		).toBe(false);
	});

	it("cannot skip when client module map is missing", () => {
		installVormaGlobal({
			routeManifest: {
				"/without-module-map": 0,
			},
			patternRegistry: createRegisteredPatternRegistry([
				"/without-module-map",
			]),
			clientModuleMap: undefined,
		});

		expect(
			canSkipServerFetch("http://localhost:3000/without-module-map")
				.canSkip,
		).toBe(false);
	});

	it("cannot skip when server-loader data is required but missing", () => {
		installVormaGlobal({
			routeManifest: {
				"/needs-data": 1,
			},
			patternRegistry: createRegisteredPatternRegistry(["/needs-data"]),
			clientModuleMap: {
				"/needs-data": {
					importURL: "/needs-data.js",
					exportKey: "default",
					errorExportKey: "",
				},
			},
			matchedPatterns: ["/needs-data"],
			loadersData: [],
		});

		expect(
			canSkipServerFetch("http://localhost:3000/needs-data").canSkip,
		).toBe(false);
	});

	it("cannot skip when server-loader route was not previously matched", () => {
		installVormaGlobal({
			routeManifest: {
				"/needs-data": 1,
			},
			patternRegistry: createRegisteredPatternRegistry(["/needs-data"]),
			clientModuleMap: {
				"/needs-data": {
					importURL: "/needs-data.js",
					exportKey: "default",
					errorExportKey: "",
				},
			},
			matchedPatterns: [],
			loadersData: [],
		});

		expect(
			canSkipServerFetch("http://localhost:3000/needs-data").canSkip,
		).toBe(false);
	});

	it("returns server-loader data when all required skip data is available", () => {
		installVormaGlobal({
			routeManifest: {
				"/with-data": 1,
			},
			patternRegistry: createRegisteredPatternRegistry(["/with-data"]),
			clientModuleMap: {
				"/with-data": {
					importURL: "/with-data.js",
					exportKey: "default",
					errorExportKey: "",
				},
			},
			matchedPatterns: ["/with-data"],
			loadersData: [{ from: "cache" }],
		});

		const result = canSkipServerFetch("http://localhost:3000/with-data");
		expect(result.canSkip).toBe(true);
		if (!result.canSkip) {
			throw new Error(
				"Expected canSkipServerFetch to return canSkip=true",
			);
		}
		expect(result.loadersData).toEqual([{ from: "cache" }]);
	});

	it("uses empty defaults when optional global snapshots are unset", () => {
		installVormaGlobal({
			routeManifest: {
				"/defaults": 0,
			},
			patternRegistry: createRegisteredPatternRegistry(["/defaults"]),
			clientModuleMap: {
				"/defaults": {
					importURL: "/defaults.js",
					exportKey: "default",
					errorExportKey: "",
				},
			},
			patternToWaitFnMap: undefined,
			matchedPatterns: undefined,
			params: undefined,
			splatValues: undefined,
			loadersData: undefined,
		});

		const result = canSkipServerFetch("http://localhost:3000/defaults");
		expect(result.canSkip).toBe(true);
		if (!result.canSkip) {
			throw new Error(
				"Expected canSkipServerFetch to return canSkip=true",
			);
		}
		expect(result.loadersData).toEqual([undefined]);
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

		const runtimeGlobal = (globalThis as any)[VORMA_SYMBOL];
		expect(runtimeGlobal.clientLoadersData).toEqual([]);
		expect(runtimeGlobal.outermostClientError).toBeUndefined();
	});

	it("__registerClientLoaderPattern fails fast without a registry", async () => {
		installVormaGlobal({
			patternRegistry: undefined,
		});

		await expect(__registerClientLoaderPattern("/x")).rejects.toThrow(
			"Pattern registry has not been initialized.",
		);
	});
});

describe("fetchRouteData client-only skip path", () => {
	it("short-circuits server fetch for skippable user navigations", async () => {
		vi.doMock("/client-only.js", () => ({
			default: () => null,
		}));
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValue(new Response("unused"));
		installVormaGlobal({
			buildID: "build-42",
			routeManifest: {
				"/client-only": 0,
			},
			patternRegistry: createRegisteredPatternRegistry(["/client-only"]),
			clientModuleMap: {
				"/client-only": {
					importURL: "/client-only.js",
					exportKey: "default",
					errorExportKey: "",
				},
			},
		});

		const outcome = await fetchRouteData(new AbortController(), {
			href: "/client-only",
			navigationType: "userNavigation",
		});

		expect(fetchSpy).not.toHaveBeenCalled();
		expect(outcome.type).toBe("success");
		if (outcome.type !== "success") {
			throw new Error("Expected a success outcome");
		}

		expect(outcome.response.headers.get("X-Vorma-Build-Id")).toBe(
			"build-42",
		);
		expect(outcome.json.matchedPatterns).toEqual(["/client-only"]);
		expect(outcome.json.importURLs).toEqual(["/client-only.js"]);
		expect(outcome.json.loadersData).toEqual([undefined]);
		await expect(outcome.waitFnPromise).resolves.toEqual({
			data: [undefined],
			errorMessage: undefined,
		});
	});

	it("reuses cached client-loader data in client-only skip outcomes", async () => {
		vi.doMock("/cached-client.js", () => ({
			default: () => null,
		}));
		const waitFn = vi.fn().mockResolvedValue("unexpected");
		installVormaGlobal({
			routeManifest: {
				"/cached-client": 0,
			},
			patternRegistry: createRegisteredPatternRegistry([
				"/cached-client",
			]),
			clientModuleMap: {
				"/cached-client": {
					importURL: "/cached-client.js",
					exportKey: "default",
					errorExportKey: "",
				},
			},
			matchedPatterns: ["/cached-client"],
			patternToWaitFnMap: {
				"/cached-client": waitFn,
			},
			clientLoadersData: [{ cached: true }],
		});

		const outcome = await fetchRouteData(new AbortController(), {
			href: "/cached-client",
			navigationType: "userNavigation",
		});
		expect(outcome.type).toBe("success");
		if (outcome.type !== "success") {
			throw new Error("Expected a success outcome");
		}

		await expect(outcome.waitFnPromise).resolves.toEqual({
			data: [{ cached: true }],
			errorMessage: undefined,
		});
		expect(waitFn).not.toHaveBeenCalled();
	});

	it("does not seed cached client-loader results when current match snapshot is missing", async () => {
		vi.doMock("/uncached-client.js", () => ({
			default: () => null,
		}));
		const waitFn = vi.fn().mockResolvedValue({ fromWaitFn: true });
		installVormaGlobal({
			routeManifest: {
				"/uncached-client": 0,
			},
			patternRegistry: createRegisteredPatternRegistry([
				"/uncached-client",
			]),
			clientModuleMap: {
				"/uncached-client": {
					importURL: "/uncached-client.js",
					exportKey: "default",
					errorExportKey: "",
				},
			},
			matchedPatterns: ["/uncached-client"],
			patternToWaitFnMap: {
				"/uncached-client": waitFn,
			},
			clientLoadersData: [],
		});

		const outcome = await fetchRouteData(new AbortController(), {
			href: "/uncached-client",
			navigationType: "userNavigation",
		});
		expect(outcome.type).toBe("success");
		if (outcome.type !== "success") {
			throw new Error("Expected a success outcome");
		}

		await expect(outcome.waitFnPromise).resolves.toEqual({
			data: [{ fromWaitFn: true }],
			errorMessage: undefined,
		});
		expect(waitFn).toHaveBeenCalledTimes(1);
	});

	it("falls back to default optional globals in skippable client-only outcomes", async () => {
		vi.doMock("/fallback-defaults.js", () => ({
			default: () => null,
		}));
		installVormaGlobal({
			buildID: undefined,
			routeManifest: {
				"/fallback-defaults": 0,
			},
			patternRegistry: createRegisteredPatternRegistry([
				"/fallback-defaults",
			]),
			clientModuleMap: {
				"/fallback-defaults": {
					importURL: "/fallback-defaults.js",
					exportKey: "default",
					errorExportKey: "",
				},
			},
			patternToWaitFnMap: undefined,
			matchedPatterns: undefined,
			params: undefined,
			splatValues: undefined,
			loadersData: undefined,
			clientLoadersData: undefined,
		});

		const outcome = await fetchRouteData(new AbortController(), {
			href: "/fallback-defaults",
			navigationType: "userNavigation",
		});
		expect(outcome.type).toBe("success");
		if (outcome.type !== "success") {
			throw new Error("Expected a success outcome");
		}

		expect(outcome.response.headers.get("X-Vorma-Build-Id")).toBe("1");
		await expect(outcome.waitFnPromise).resolves.toEqual({
			data: [undefined],
			errorMessage: undefined,
		});
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

	it("succeeds when server JSON omits importURLs", async () => {
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

		const outcome = await fetchRouteData(new AbortController(), {
			href: "/missing-import-urls",
			navigationType: "userNavigation",
		});
		expect(outcome.type).toBe("success");
		if (outcome.type !== "success") {
			throw new Error("Expected success outcome");
		}
		await expect(outcome.waitFnPromise).resolves.toEqual({
			data: [],
			errorMessage: undefined,
		});
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
			clientModuleMap: {
				"/build-id-fallback": {
					importURL: "/build-id-fallback.js",
					exportKey: "default",
					errorExportKey: "",
				},
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
					importURLs: [],
					exportKeys: [],
					errorExportKeys: [],
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
			clientModuleMap: {
				"/parent": {
					importURL: "/parent.js",
					exportKey: "default",
					errorExportKey: "",
				},
				"/parent/child": {
					importURL: "/child.js",
					exportKey: "default",
					errorExportKey: "",
				},
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

	it("throws when partial matcher returns sparse route matches", async () => {
		const partialMatchesSpy = vi
			.spyOn(renderRuntimeModule, "findPartialMatchesOnClient")
			.mockResolvedValue({
				params: {},
				splatValues: [],
				matches: [undefined as any, createMatch("/sparse-loader")],
			} as any);
		const waitFn = vi.fn(async ({ serverDataPromise }) => {
			const serverData = await serverDataPromise;
			return serverData.loaderData;
		});
		vi.spyOn(window, "fetch").mockResolvedValue(
			new Response(
				JSON.stringify({
					matchedPatterns: ["/sparse-loader"],
					loadersData: [{ value: "ok" }],
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
			routeManifest: {
				"/sparse-loader": 1,
			},
			patternRegistry: createRegisteredPatternRegistry([
				"/sparse-loader",
			]),
			patternToWaitFnMap: {
				"/sparse-loader": waitFn,
			},
			clientModuleMap: {
				"/sparse-loader": {
					importURL: "/sparse-loader.js",
					exportKey: "default",
					errorExportKey: "",
				},
			},
		});
		const consoleErrorSpy = vi
			.spyOn(console, "error")
			.mockImplementation(() => {});

		try {
			await expect(
				fetchRouteData(new AbortController(), {
					href: "/sparse-loader",
					navigationType: "userNavigation",
				}),
			).rejects.toThrow(
				"Partial route matcher returned a sparse matches array at index 0.",
			);
			expect(waitFn).not.toHaveBeenCalled();
			expect(consoleErrorSpy).toHaveBeenCalled();
		} finally {
			consoleErrorSpy.mockRestore();
			partialMatchesSpy.mockRestore();
		}
	});

	it("lets client loaders handle unavailable server data when payload lacks matched loader arrays", async () => {
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
			clientModuleMap: {
				"/needs-server": {
					importURL: "/needs-server.js",
					exportKey: "default",
					errorExportKey: "",
				},
			},
			matchedPatterns: [],
		});

		const outcome = await fetchRouteData(new AbortController(), {
			href: "/needs-server",
			navigationType: "userNavigation",
		});
		expect(outcome.type).toBe("success");
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
			clientModuleMap: {
				"/rejecting-server": {
					importURL: "/rejecting-server.js",
					exportKey: "default",
					errorExportKey: "",
				},
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

	it("handles production responses with undefined deps and cssBundles", async () => {
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
			expect(outcome.cssBundlePromises).toEqual([]);
		} finally {
			(import.meta.env as any).DEV = originalDev;
		}
	});

	it("preloads only truthy production deps", async () => {
		const originalDev = import.meta.env.DEV;
		(import.meta.env as any).DEV = false;
		const preloadModuleSpy = vi
			.spyOn(renderRuntimeModule.AssetManager, "preloadModule")
			.mockImplementation(() => {});
		try {
			vi.spyOn(window, "fetch").mockResolvedValue(
				new Response(
					JSON.stringify({
						matchedPatterns: [],
						loadersData: [],
						importURLs: ["/ignored-in-prod.js"],
						exportKeys: [],
						errorExportKeys: [],
						hasRootData: false,
						params: {},
						splatValues: [],
						deps: ["", "/prod-dep.js", null],
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
			expect(preloadModuleSpy).toHaveBeenCalledTimes(1);
			expect(preloadModuleSpy).toHaveBeenCalledWith("/prod-dep.js");
		} finally {
			preloadModuleSpy.mockRestore();
			(import.meta.env as any).DEV = originalDev;
		}
	});

	it("throws when the server returns 304 without route JSON payload", async () => {
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
		).rejects.toThrow("No JSON response");
		expect(logErrorSpy).toHaveBeenCalled();
		logErrorSpy.mockRestore();
	});
});
