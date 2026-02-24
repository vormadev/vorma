import { describe, expect, it, vi } from "vitest";
import {
	createAbortAwareFetchRecorder,
	createDeferred,
	createDeferredFetchCall,
	createRouteDataResponse,
	expectStatusIdle,
	installContractVormaGlobal,
	loadClientAPI,
	setupContractTestSuite,
	waitForRequestCount,
	withUnhandledRejectionCapture,
} from "./contract_test_harness.ts";

setupContractTestSuite();

describe("client navigation lifecycle contracts", () => {
	it("aborts stale prefetch and revalidation work when a new user navigation starts", async () => {
		const api = await loadClientAPI();
		const { requests } = createAbortAwareFetchRecorder();

		const prefetchHandlers = api.__getPrefetchHandlers({
			href: "/stale-prefetch",
			delayMs: 0,
		});
		prefetchHandlers?.start(new Event("mouseenter"));
		await vi.advanceTimersByTimeAsync(1);

		const revalidatePromise = api.revalidate();
		await Promise.resolve();

		const navPromise = api.vormaNavigate("/fresh-target");
		await Promise.resolve();

		expect(requests).toHaveLength(3);
		expect(requests[0]?.signal?.aborted).toBe(true);
		expect(requests[1]?.signal?.aborted).toBe(true);
		expect(requests[2]?.signal?.aborted).toBe(false);

		requests[2]?.resolve(createRouteDataResponse());
		await navPromise;
		await revalidatePromise;
		await vi.runAllTimersAsync();

		prefetchHandlers?.stop();
	});

	it("clearAll aborts in-flight navigation and submit work, then returns idle", async () => {
		const api = await loadClientAPI();
		const { navigationStateManager } = await import("../../client.ts");
		const { requests } = createAbortAwareFetchRecorder();

		const { result, unhandledRejections } =
			await withUnhandledRejectionCapture({
				run: async () => {
					const navPromise = api.vormaNavigate("/clear-all-nav");
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

					navigationStateManager.clearAll();
					expect(requests[0]?.signal?.aborted).toBe(true);
					expect(requests[1]?.signal?.aborted).toBe(true);

					const submitResult = await submitPromise;
					await navPromise;
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

	it("clearAll aborts in-flight prefetch and revalidation work, then returns idle", async () => {
		const api = await loadClientAPI();
		const { navigationStateManager } = await import("../../client.ts");
		const { requests } = createAbortAwareFetchRecorder();
		const prefetchHandlers = api.__getPrefetchHandlers({
			href: "/clear-all-prefetch",
			delayMs: 0,
		});

		prefetchHandlers?.start(new Event("mouseenter"));
		await vi.advanceTimersByTimeAsync(1);

		const revalidatePromise = api.revalidate();
		await waitForRequestCount({ requests, count: 2 });
		expect(api.getStatus()).toEqual({
			isNavigating: false,
			isSubmitting: false,
			isRevalidating: true,
		});

		navigationStateManager.clearAll();
		expect(requests[0]?.signal?.aborted).toBe(true);
		expect(requests[1]?.signal?.aborted).toBe(true);

		await revalidatePromise;
		await vi.runAllTimersAsync();
		prefetchHandlers?.stop();

		expect(window.location.pathname).toBe("/");
		expectStatusIdle(api.getStatus());
	});

	it("clearAll prevents late side effects from navigations whose fetch ignores abort", async () => {
		const api = await loadClientAPI();
		const { navigationStateManager } = await import("../../client.ts");
		const fetchCall = createDeferredFetchCall();
		const requestAnimationFrameSpy = vi.spyOn(
			window,
			"requestAnimationFrame",
		);
		vi.spyOn(window, "fetch").mockImplementation(fetchCall.mock);

		const navPromise = api.vormaNavigate("/clear-all-late-success");
		await vi.advanceTimersByTimeAsync(8);
		expect(fetchCall.getSignal()?.aborted).toBe(false);

		navigationStateManager.clearAll();
		expect(fetchCall.getSignal()?.aborted).toBe(true);

		const rAFCallCountBeforeResolve =
			requestAnimationFrameSpy.mock.calls.length;
		fetchCall.deferred.resolve(
			createRouteDataResponse({
				title: { dangerousInnerHTML: "Late Stale Title" },
				cssBundles: ["/late-stale.css"],
			}),
		);

		await navPromise;
		await vi.advanceTimersByTimeAsync(32);
		await vi.runAllTimersAsync();

		expect(window.location.pathname).toBe("/");
		expect(document.title).toBe("Initial Title");
		expect(requestAnimationFrameSpy).toHaveBeenCalledTimes(
			rAFCallCountBeforeResolve,
		);
		expect(
			document.head.querySelector(
				'link[data-vorma-css-bundle="/late-stale.css"]',
			),
		).toBeNull();
		expectStatusIdle(api.getStatus());
	});

	it("clearAll prevents late side effects from prefetches whose fetch ignores abort", async () => {
		const api = await loadClientAPI();
		const { navigationStateManager } = await import("../../client.ts");
		const fetchCall = createDeferredFetchCall();
		const requestAnimationFrameSpy = vi.spyOn(
			window,
			"requestAnimationFrame",
		);
		vi.spyOn(window, "fetch").mockImplementation(fetchCall.mock);

		const { unhandledRejections } = await withUnhandledRejectionCapture({
			run: async () => {
				const prefetchHandlers = api.__getPrefetchHandlers({
					href: "/clear-prefetch-late",
					delayMs: 0,
				});
				prefetchHandlers?.start(new Event("mouseenter"));
				await vi.advanceTimersByTimeAsync(1);
				expect(fetchCall.getSignal()?.aborted).toBe(false);

				navigationStateManager.clearAll();
				expect(fetchCall.getSignal()?.aborted).toBe(true);

				const rAFCallCountBeforeResolve =
					requestAnimationFrameSpy.mock.calls.length;
				fetchCall.deferred.resolve(
					createRouteDataResponse({
						title: { dangerousInnerHTML: "Late Prefetch Title" },
						cssBundles: ["/late-prefetch.css"],
					}),
				);
				await vi.advanceTimersByTimeAsync(32);
				await vi.runAllTimersAsync();

				expect(requestAnimationFrameSpy).toHaveBeenCalledTimes(
					rAFCallCountBeforeResolve,
				);
				expect(
					document.head.querySelector(
						'link[data-vorma-css-bundle="/late-prefetch.css"]',
					),
				).toBeNull();

				prefetchHandlers?.stop();
			},
		});

		expect(unhandledRejections).toEqual([]);
		expect(window.location.pathname).toBe("/");
		expect(document.title).toBe("Initial Title");
		expectStatusIdle(api.getStatus());
	});

	it("remains operable after clearAll by allowing fresh navigation to complete", async () => {
		const api = await loadClientAPI();
		const { navigationStateManager } = await import("../../client.ts");
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

		navigationStateManager.clearAll();
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

	it("deduplicates concurrent prefetch starts for the same URL", async () => {
		const api = await loadClientAPI();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValue(createRouteDataResponse());

		const firstHandlers = api.__getPrefetchHandlers({
			href: "/shared-prefetch",
			delayMs: 0,
		});
		const secondHandlers = api.__getPrefetchHandlers({
			href: "/shared-prefetch",
			delayMs: 0,
		});

		firstHandlers?.start(new Event("mouseenter"));
		secondHandlers?.start(new Event("mouseenter"));
		await vi.advanceTimersByTimeAsync(1);

		expect(fetchSpy).toHaveBeenCalledTimes(1);

		firstHandlers?.stop();
		secondHandlers?.stop();
	});

	it("waits for client loader completion before dispatching route-change", async () => {
		const waitFnDeferred = createDeferred<{ ready: true }>();
		installContractVormaGlobal({
			patternToWaitFnMap: {
				"/loader-gated": () => waitFnDeferred.promise,
			},
		});

		const api = await loadClientAPI();
		const routeChangeListener = vi.fn();
		const removeRouteChangeListener =
			api.addRouteChangeListener(routeChangeListener);

		vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				matchedPatterns: ["/loader-gated"],
				loadersData: [{ serverData: "ok" }],
				hasRootData: true,
			}),
		);

		const navPromise = api.vormaNavigate("/loader-gated");
		await vi.advanceTimersByTimeAsync(10);

		expect(routeChangeListener).not.toHaveBeenCalled();
		expect(api.getStatus().isNavigating).toBe(true);

		waitFnDeferred.resolve({ ready: true });
		await navPromise;
		await vi.runAllTimersAsync();

		expect(routeChangeListener).toHaveBeenCalledTimes(1);
		expect(api.__vormaClientGlobal.get("clientLoadersData")).toEqual([
			{ ready: true },
		]);

		removeRouteChangeListener();
	});

	it("uses document.startViewTransition for user navigation when enabled", async () => {
		installContractVormaGlobal({ useViewTransitions: true });
		const api = await loadClientAPI();

		const startViewTransition = vi.fn((callback?: () => Promise<void>) => {
			return {
				finished: Promise.resolve().then(async () => {
					await callback?.();
				}),
			};
		});
		Object.defineProperty(document, "startViewTransition", {
			value: startViewTransition,
			configurable: true,
		});

		vi.spyOn(window, "fetch").mockResolvedValue(createRouteDataResponse());

		await api.vormaNavigate("/with-transition");
		await vi.runAllTimersAsync();

		expect(startViewTransition).toHaveBeenCalledTimes(1);
	});

	it("skips view transitions for prefetch and revalidation navigation types", async () => {
		installContractVormaGlobal({ useViewTransitions: true });
		const api = await loadClientAPI();

		const startViewTransition = vi.fn();
		Object.defineProperty(document, "startViewTransition", {
			value: startViewTransition,
			configurable: true,
		});

		vi.spyOn(window, "fetch").mockResolvedValue(createRouteDataResponse());

		const prefetchHandlers = api.__getPrefetchHandlers({
			href: "/no-transition-prefetch",
			delayMs: 0,
		});
		prefetchHandlers?.start(new Event("mouseenter"));
		await vi.advanceTimersByTimeAsync(1);
		await vi.runAllTimersAsync();

		await api.revalidate();
		await vi.runAllTimersAsync();

		expect(startViewTransition).not.toHaveBeenCalled();
		prefetchHandlers?.stop();
	});

	it("updates exposed router data after successful navigation", async () => {
		const api = await loadClientAPI();
		vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				matchedPatterns: ["/users/:id"],
				loadersData: [{ user: { id: "123" } }],
				hasRootData: true,
				params: { id: "123" },
				splatValues: ["tail"],
			}),
		);

		await api.vormaNavigate("/users/123");
		await vi.runAllTimersAsync();

		expect(api.getRouterData()).toEqual({
			buildID: "1",
			matchedPatterns: ["/users/:id"],
			params: { id: "123" },
			splatValues: ["tail"],
			rootData: { user: { id: "123" } },
		});
	});

	it("preserves metadata across programmatic hash-only navigations", async () => {
		vi.doMock("/metadata-parity.js", () => ({
			default: () => null,
			RouteError: () => null,
		}));
		installContractVormaGlobal();
		const api = await loadClientAPI();
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValueOnce(
			createRouteDataResponse({
				matchedPatterns: ["/metadata-parity"],
				loadersData: [{}],
				importURLs: ["/metadata-parity.js"],
				exportKeys: ["default"],
				errorExportKeys: ["RouteError"],
				title: { dangerousInnerHTML: "Metadata Parity Title" },
				metaHeadEls: [
					{
						tag: "meta",
						attributesKnownSafe: {
							name: "description",
							content: "Metadata parity description",
						},
					},
				],
			}),
		);

		await api.vormaNavigate("/metadata-parity");
		await vi.runAllTimersAsync();

		const moduleMapBeforeHashNavigation = JSON.parse(
			JSON.stringify(api.__vormaClientGlobal.get("clientModuleMap")),
		);
		expect(document.title).toBe("Metadata Parity Title");

		await api.vormaNavigate("/metadata-parity#details");
		await vi.runAllTimersAsync();

		expect(fetchSpy).toHaveBeenCalledTimes(1);
		expect(document.title).toBe("Metadata Parity Title");
		expect(api.__vormaClientGlobal.get("clientModuleMap")).toEqual(
			moduleMapBeforeHashNavigation,
		);
	});

	it("pushes history for user navigation to a different URL", async () => {
		window.history.replaceState({}, "", "/history-start");
		const api = await loadClientAPI();
		vi.spyOn(window, "fetch").mockResolvedValue(createRouteDataResponse());

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

	it("replaces history when navigating to the current URL", async () => {
		window.history.replaceState({}, "", "/history-same");
		const api = await loadClientAPI();
		vi.spyOn(window, "fetch").mockResolvedValue(createRouteDataResponse());

		const history = api.getHistoryInstance();
		const pushSpy = vi.spyOn(history, "push");
		const replaceSpy = vi.spyOn(history, "replace");

		await api.vormaNavigate("/history-same");
		await vi.runAllTimersAsync();

		expect(replaceSpy).toHaveBeenCalledWith(
			expect.stringContaining("/history-same"),
			undefined,
		);
		expect(pushSpy).not.toHaveBeenCalled();
	});

	it("replaces history when hash target is encoding-equivalent", async () => {
		window.history.replaceState({}, "", "/history-same-hash#~");
		const api = await loadClientAPI();
		vi.spyOn(window, "fetch").mockResolvedValue(createRouteDataResponse());

		const history = api.getHistoryInstance();
		const pushSpy = vi.spyOn(history, "push");
		const replaceSpy = vi.spyOn(history, "replace");

		await api.vormaNavigate("/history-same-hash#%7E");
		await vi.runAllTimersAsync();

		expect(replaceSpy).toHaveBeenCalledWith(
			expect.stringContaining("/history-same-hash#%7E"),
			undefined,
		);
		expect(pushSpy).not.toHaveBeenCalled();
	});

	it("pushes history when hash target changes", async () => {
		window.history.replaceState({}, "", "/history-hash-change#first");
		const api = await loadClientAPI();
		vi.spyOn(window, "fetch").mockResolvedValue(createRouteDataResponse());

		const history = api.getHistoryInstance();
		const pushSpy = vi.spyOn(history, "push");
		const replaceSpy = vi.spyOn(history, "replace");

		await api.vormaNavigate("/history-hash-change#second");
		await vi.runAllTimersAsync();

		expect(pushSpy).toHaveBeenCalledWith(
			expect.stringContaining("/history-hash-change#second"),
			undefined,
		);
		expect(replaceSpy).not.toHaveBeenCalled();
	});

	it("includes restored scroll state in browser-history route-change events", async () => {
		const api = await loadClientAPI();
		api.getHistoryInstance();

		const routeChanges: Array<unknown> = [];
		const removeRouteChangeListener = api.addRouteChangeListener(
			(event) => {
				routeChanges.push(event.detail);
			},
		);

		vi.spyOn(window, "fetch").mockResolvedValue(createRouteDataResponse());
		const { customHistoryListener } =
			await import("../../platform/history.ts");

		await customHistoryListener({
			action: "PUSH",
			location: {
				pathname: "/history-from",
				search: "",
				hash: "",
				state: null,
				key: "history-from-key",
			},
		} as any);

		sessionStorage.setItem(
			"__vorma__scrollStateMap",
			JSON.stringify([["history-to-key", { x: 120, y: 240 }]]),
		);
		window.history.replaceState({}, "", "/history-to");

		await customHistoryListener({
			action: "POP",
			location: {
				pathname: "/history-to",
				search: "",
				hash: "",
				state: null,
				key: "history-to-key",
			},
		} as any);
		await vi.runAllTimersAsync();

		expect(routeChanges.at(-1)).toEqual(
			expect.objectContaining({
				__scrollState: { x: 120, y: 240 },
			}),
		);

		removeRouteChangeListener();
	});

	it("decodes HTML entities before updating document.title", async () => {
		const api = await loadClientAPI();
		vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				title: { dangerousInnerHTML: "Fish &amp; Chips &lt;3" },
			}),
		);

		await api.vormaNavigate("/entity-title");
		await vi.runAllTimersAsync();

		expect(document.title).toBe("Fish & Chips <3");
	});

	it("applies each CSS bundle only once across repeated navigations", async () => {
		const api = await loadClientAPI();
		vi.spyOn(window, "requestAnimationFrame").mockImplementation((cb) => {
			cb(0);
			return 0;
		});

		const appendChild = document.head.appendChild.bind(document.head);
		vi.spyOn(document.head, "appendChild").mockImplementation((node) => {
			if (
				node instanceof HTMLLinkElement &&
				node.rel === "preload" &&
				node.getAttribute("as") === "style"
			) {
				Promise.resolve().then(() => node.onload?.(new Event("load")));
			}
			return appendChild(node);
		});

		vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				cssBundles: ["/shared.css"],
			}),
		);

		await api.vormaNavigate("/css-one");
		await vi.runAllTimersAsync();
		await api.vormaNavigate("/css-two");
		await vi.runAllTimersAsync();

		const stylesheets = document.querySelectorAll(
			'link[rel="stylesheet"][data-vorma-css-bundle="/shared.css"]',
		);
		expect(stylesheets).toHaveLength(1);
	});
});
