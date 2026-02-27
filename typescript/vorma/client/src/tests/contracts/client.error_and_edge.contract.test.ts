import { describe, expect, it, vi } from "vitest";
import {
	createDeferred,
	createRouteDataResponse,
	loadClientAPI,
	registerServerDataFieldProbeLoader,
	requestInputToHref,
	setupContractTestSuite,
	withUnhandledRejectionCapture,
} from "./contract_test_harness.ts";

setupContractTestSuite();

describe("client error and edge contracts", () => {
	it("clears navigating state for AbortError failures", async () => {
		const api = await loadClientAPI();
		const abortError = new Error("Aborted");
		abortError.name = "AbortError";
		vi.spyOn(window, "fetch").mockRejectedValueOnce(abortError);

		expect(api.getStatus().isNavigating).toBe(false);
		const navPromise = api.vormaNavigate("/abort-nav");
		expect(api.getStatus().isNavigating).toBe(true);

		await navPromise;
		await vi.runAllTimersAsync();

		expect(api.getStatus().isNavigating).toBe(false);
	});

	it("handles non-abort failures by clearing loading state and staying operable", async () => {
		const api = await loadClientAPI();
		window.history.replaceState({}, "", "/stable");
		document.title = "Stable Title";
		vi.spyOn(window, "fetch").mockRejectedValueOnce(
			new Error("Network failure"),
		);

		await api.vormaNavigate("/unreachable");
		await vi.runAllTimersAsync();

		expect(api.getStatus()).toEqual({
			isNavigating: false,
			isSubmitting: false,
			isRevalidating: false,
		});
		expect(window.location.pathname).toBe("/stable");
		expect(document.title).toBe("Stable Title");
		expect(console.error).toHaveBeenCalledTimes(1);
		expect(console.error).toHaveBeenCalledWith(
			"Vorma:",
			"Navigation failed",
			expect.any(Error),
		);

		vi.spyOn(window, "fetch").mockResolvedValueOnce(
			createRouteDataResponse({
				title: { dangerousInnerHTML: "Recovered Page" },
			}),
		);
		await api.vormaNavigate("/recovered-non-abort");
		await vi.runAllTimersAsync();
		expect(document.title).toBe("Recovered Page");
	});

	it("does not mutate established router state after a failed navigation", async () => {
		const api = await loadClientAPI();
		vi.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse({
					matchedPatterns: ["/stable/:id"],
					loadersData: [{ user: { id: "1" } }],
					hasRootData: true,
					params: { id: "1" },
					splatValues: ["tail"],
					title: { dangerousInnerHTML: "Stable Route" },
				}),
			)
			.mockRejectedValueOnce(new Error("boom"));

		await api.vormaNavigate("/stable/1");
		await vi.runAllTimersAsync();

		const stableRouterData = api.getRouterData();
		const stableTitle = document.title;

		await api.vormaNavigate("/failing-route");
		await vi.runAllTimersAsync();

		expect(api.getRouterData()).toEqual(stableRouterData);
		expect(document.title).toBe(stableTitle);
		expect(api.getStatus()).toEqual({
			isNavigating: false,
			isSubmitting: false,
			isRevalidating: false,
		});
	});

	it("treats empty JSON response as failed and then recovers", async () => {
		const api = await loadClientAPI();
		window.history.replaceState({}, "", "/before-empty");
		document.title = "Before Empty";
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValueOnce(new Response("", { status: 200 }))
			.mockResolvedValueOnce(
				createRouteDataResponse({
					title: { dangerousInnerHTML: "Recovered" },
				}),
			);

		await api.vormaNavigate("/empty");
		await vi.runAllTimersAsync();

		expect(api.getStatus()).toEqual({
			isNavigating: false,
			isSubmitting: false,
			isRevalidating: false,
		});
		expect(window.location.pathname).toBe("/before-empty");
		expect(document.title).toBe("Before Empty");
		expect(console.error).toHaveBeenCalledTimes(1);
		expect(console.error).toHaveBeenCalledWith(
			"Vorma:",
			"Navigation failed",
			expect.any(Error),
		);

		await api.vormaNavigate("/recovered");
		await vi.runAllTimersAsync();

		expect(fetchSpy).toHaveBeenCalledTimes(2);
		expect(window.location.pathname).toBe("/recovered");
		expect(document.title).toBe("Recovered");
	});

	it("handles non-ok responses and still allows later navigation", async () => {
		const api = await loadClientAPI();
		window.history.replaceState({}, "", "/before-404");
		document.title = "Before 404";
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValueOnce(new Response("Not Found", { status: 404 }))
			.mockResolvedValueOnce(
				createRouteDataResponse({
					title: { dangerousInnerHTML: "Success Page" },
				}),
			);

		await api.vormaNavigate("/missing");
		await vi.runAllTimersAsync();
		expect(api.getStatus()).toEqual({
			isNavigating: false,
			isSubmitting: false,
			isRevalidating: false,
		});
		expect(window.location.pathname).toBe("/before-404");
		expect(document.title).toBe("Before 404");
		expect(console.error).toHaveBeenCalledTimes(1);
		expect(console.error).toHaveBeenCalledWith(
			"Vorma:",
			"Navigation failed",
			expect.any(Error),
		);

		await api.vormaNavigate("/success");
		await vi.runAllTimersAsync();

		expect(fetchSpy).toHaveBeenCalledTimes(2);
		expect(window.location.pathname).toBe("/success");
		expect(document.title).toBe("Success Page");
	});

	it("does not leak unhandled rejections when user navigation gets a failed response", async () => {
		const api = await loadClientAPI();
		const { serverDataPromiseErrors } =
			await registerServerDataFieldProbeLoader({
				api,
				pattern: "/failed-with-loader",
				requiredField: "Title",
			});

		vi.spyOn(window, "fetch").mockResolvedValue(
			new Response("Server error", { status: 500 }),
		);

		const { unhandledRejections } = await withUnhandledRejectionCapture({
			run: async () => {
				await api.vormaNavigate("/failed-with-loader");
				await vi.runAllTimersAsync();
			},
		});

		expect(unhandledRejections).toEqual([]);
		expect(serverDataPromiseErrors).toHaveLength(1);
		expect(serverDataPromiseErrors[0]).toBeInstanceOf(Error);
		expect((serverDataPromiseErrors[0] as Error).name).toBe("AbortError");
	});

	it("does not leak unhandled rejections when user navigation redirects before loader data is available", async () => {
		const api = await loadClientAPI();
		const { serverDataPromiseErrors } =
			await registerServerDataFieldProbeLoader({
				api,
				pattern: "/redirect-with-loader",
				requiredField: "Title",
			});

		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse(
					{},
					{ headers: { "X-Client-Redirect": "/redirect-target" } },
				),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					title: { dangerousInnerHTML: "Redirect Target" },
				}),
			);

		const { unhandledRejections } = await withUnhandledRejectionCapture({
			run: async () => {
				await api.vormaNavigate("/redirect-with-loader");
				await vi.runAllTimersAsync();
			},
		});

		expect(unhandledRejections).toEqual([]);
		expect(serverDataPromiseErrors).toHaveLength(1);
		expect(serverDataPromiseErrors[0]).toBeInstanceOf(Error);
		expect((serverDataPromiseErrors[0] as Error).name).toBe("AbortError");
		expect(fetchSpy).toHaveBeenCalledTimes(2);
	});

	it("does not leak unhandled rejections when stale prefetch success is dropped before wait phase", async () => {
		const api = await loadClientAPI();
		const { ComponentLoader } =
			await import("../../runtime.ts");

		vi.spyOn(ComponentLoader, "loadComponents").mockRejectedValueOnce(
			new Error("Module preload failed"),
		);

		const fetchDeferred = createDeferred<Response>();
		vi.spyOn(window, "fetch").mockImplementation(
			() => fetchDeferred.promise as any,
		);

		const handlers = api.__getPrefetchHandlers({
			href: "/stale-prefetch-drop",
		});

		const { unhandledRejections } = await withUnhandledRejectionCapture({
			run: async () => {
				handlers?.start(new Event("mouseenter"));
				await vi.advanceTimersByTimeAsync(100);

				// Explicitly drop the idle prefetch entry before the fetch resolves.
				handlers?.stop();

				fetchDeferred.resolve(
					createRouteDataResponse({
						matchedPatterns: ["/stale-prefetch-drop"],
						importURLs: ["/stale-prefetch-drop.module.js"],
						exportKeys: ["default"],
						loadersData: [null],
						hasRootData: false,
					}),
				);

				await vi.runAllTimersAsync();
			},
		});

		expect(unhandledRejections).toEqual([]);
	});

	it("rejects client-loader serverDataPromise with AbortError when required server loader payload is missing", async () => {
		const api = await loadClientAPI();
		const pattern = "/missing-server-loader-payload";
		api.__vormaClientGlobal.set("routeManifest", { [pattern]: 1 });
		const { serverDataPromiseErrors } =
			await registerServerDataFieldProbeLoader({
				api,
				pattern,
				requiredField: "Title",
			});

		vi.spyOn(window, "fetch").mockResolvedValueOnce(
			createRouteDataResponse({
				matchedPatterns: [pattern],
				loadersData: [],
				hasRootData: false,
			}),
		);

		await api.vormaNavigate(pattern);
		await vi.runAllTimersAsync();

		expect(serverDataPromiseErrors).toHaveLength(1);
		expect(serverDataPromiseErrors[0]).toBeInstanceOf(Error);
		expect((serverDataPromiseErrors[0] as Error).name).toBe("AbortError");
	});

	it("contains client-loader data-shape failures without leaking unhandled rejections", async () => {
		const api = await loadClientAPI();
		const pattern = "/client-loader-data-shape-failure";
		const patternToWaitFnMap =
			api.__vormaClientGlobal.get("patternToWaitFnMap");

		patternToWaitFnMap[pattern] = async ({ serverDataPromise }) => {
			const { loaderData } = (await serverDataPromise) as {
				loaderData: { LatestVersion: string };
			};
			return loaderData.LatestVersion;
		};
		await api.__registerClientLoaderPattern(pattern);

		vi.spyOn(window, "fetch").mockResolvedValueOnce(
			createRouteDataResponse({
				matchedPatterns: [pattern],
				importURLs: [],
				exportKeys: [],
				loadersData: [],
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
		expect(api.getStatus()).toEqual({
			isNavigating: false,
			isSubmitting: false,
			isRevalidating: false,
		});
	});

	it("ignores late stale navigation responses with loader-data shape failures after newer navigation wins", async () => {
		const api = await loadClientAPI();
		const stalePattern = "/stale-loader-shape";
		const patternToWaitFnMap =
			api.__vormaClientGlobal.get("patternToWaitFnMap");

		patternToWaitFnMap[stalePattern] = async ({ serverDataPromise }) => {
			const { loaderData } = (await serverDataPromise) as {
				loaderData: { Title: string };
			};
			return loaderData.Title;
		};
		await api.__registerClientLoaderPattern(stalePattern);

		const staleDeferred = createDeferred<Response>();
		const freshDeferred = createDeferred<Response>();
		const signals: AbortSignal[] = [];
		let fetchCallCount = 0;

		vi.spyOn(window, "fetch").mockImplementation((_url, init) => {
			const signal = (init as RequestInit | undefined)
				?.signal as AbortSignal;
			if (signal) {
				signals.push(signal);
			}

			fetchCallCount++;
			return (
				fetchCallCount === 1
					? staleDeferred.promise
					: freshDeferred.promise
			) as any;
		});

		const { unhandledRejections } = await withUnhandledRejectionCapture({
			run: async () => {
				const staleNavigation = api.vormaNavigate(stalePattern);
				await Promise.resolve();

				const freshNavigation = api.vormaNavigate("/fresh-wins");
				await Promise.resolve();
				expect(signals[0]?.aborted).toBe(true);

				freshDeferred.resolve(
					createRouteDataResponse({
						title: { dangerousInnerHTML: "Fresh Wins" },
					}),
				);
				await freshNavigation;
				await vi.runAllTimersAsync();

				staleDeferred.resolve(
					createRouteDataResponse({
						matchedPatterns: [stalePattern],
						importURLs: ["/stale-loader-shape.js"],
						exportKeys: ["default"],
						loadersData: [],
						hasRootData: false,
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
		expect(api.getStatus()).toEqual({
			isNavigating: false,
			isSubmitting: false,
			isRevalidating: false,
		});
	});

	it("does not apply side effects from stale aborted navigation successes", async () => {
		const api = await loadClientAPI();
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
			fetchCallCount++;
			if (fetchCallCount === 1) {
				return staleDeferred.promise as any;
			}
			return Promise.resolve(
				createRouteDataResponse(
					{
						title: {
							dangerousInnerHTML: "Winner Navigation Title",
						},
					},
					{
						headers: {
							"X-Vorma-Build-Id": "winner-build-1",
						},
					},
				),
			) as any;
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
						title: {
							dangerousInnerHTML: "Stale Side Effect Title",
						},
						cssBundles: ["/stale-side-effects.css"],
					},
					{
						headers: {
							"X-Vorma-Build-Id": "stale-build-2",
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
			expect(api.getBuildID()).toBe("winner-build-1");
			expect(
				buildIDEvents.some((event) => event.newID === "stale-build-2"),
			).toBe(false);
			expect(requestAnimationFrameSpy).toHaveBeenCalledTimes(
				rAFCallCountBeforeStale,
			);
			expect(
				document.head.querySelector(
					'link[data-vorma-css-bundle="/stale-side-effects.css"]',
				),
			).toBeNull();
			expect(api.getStatus()).toEqual({
				isNavigating: false,
				isSubmitting: false,
				isRevalidating: false,
			});
		} finally {
			removeBuildIDListener();
		}
	});

	it("does not let stale revalidation override later prefetched navigation", async () => {
		const api = await loadClientAPI();
		window.history.replaceState({}, "", "/");

		const revalidationDeferred = createDeferred<Response>();
		vi.spyOn(window, "fetch").mockImplementation(
			(input: RequestInfo | URL) => {
				const href = requestInputToHref(input);
				if (href.includes("/about")) {
					return Promise.resolve(
						createRouteDataResponse({
							title: { dangerousInnerHTML: "About Page" },
						}),
					) as any;
				}
				return revalidationDeferred.promise as any;
			},
		);

		const handlers = api.__getPrefetchHandlers({ href: "/about" });
		handlers?.start(new Event("mouseenter"));
		await vi.advanceTimersByTimeAsync(100);
		await vi.runAllTimersAsync();

		const revalidatePromise = api.revalidate();
		await vi.advanceTimersByTimeAsync(8);

		const anchor = document.createElement("a");
		anchor.href = "/about";
		document.body.appendChild(anchor);
		const clickEvent = new MouseEvent("click", {
			bubbles: true,
			cancelable: true,
		});
		Object.defineProperty(clickEvent, "target", { value: anchor });

		await handlers?.onClick(clickEvent);
		await vi.runAllTimersAsync();
		expect(window.location.pathname).toBe("/about");
		expect(document.title).toBe("About Page");

		revalidationDeferred.resolve(
			createRouteDataResponse({
				title: { dangerousInnerHTML: "Home Page" },
			}),
		);
		await revalidatePromise;
		await vi.runAllTimersAsync();

		expect(window.location.pathname).toBe("/about");
		expect(document.title).toBe("About Page");
		document.body.removeChild(anchor);
	});

	it("does not apply CSS bundles from stale revalidation responses", async () => {
		const api = await loadClientAPI();
		window.history.replaceState({}, "", "/");
		const requestAnimationFrameSpy = vi.spyOn(
			window,
			"requestAnimationFrame",
		);

		const revalidationDeferred = createDeferred<Response>();
		vi.spyOn(window, "fetch").mockImplementation(
			(input: RequestInfo | URL) => {
				const href = requestInputToHref(input);
				if (href.includes("/about")) {
					return Promise.resolve(
						createRouteDataResponse({
							title: { dangerousInnerHTML: "About Page" },
						}),
					) as any;
				}
				return revalidationDeferred.promise as any;
			},
		);

		const handlers = api.__getPrefetchHandlers({ href: "/about" });
		handlers?.start(new Event("mouseenter"));
		await vi.advanceTimersByTimeAsync(100);
		await vi.runAllTimersAsync();

		const revalidatePromise = api.revalidate();
		await vi.advanceTimersByTimeAsync(8);

		const anchor = document.createElement("a");
		anchor.href = "/about";
		document.body.appendChild(anchor);
		const clickEvent = new MouseEvent("click", {
			bubbles: true,
			cancelable: true,
		});
		Object.defineProperty(clickEvent, "target", { value: anchor });

		await handlers?.onClick(clickEvent);
		await vi.runAllTimersAsync();
		expect(window.location.pathname).toBe("/about");
		const rAFCallCountBeforeStale =
			requestAnimationFrameSpy.mock.calls.length;

		revalidationDeferred.resolve(
			createRouteDataResponse({
				title: { dangerousInnerHTML: "Home Page (Stale)" },
				cssBundles: ["/stale-only.css"],
			}),
		);
		await revalidatePromise;
		await vi.advanceTimersByTimeAsync(32);
		await vi.runAllTimersAsync();

		expect(window.location.pathname).toBe("/about");
		expect(document.title).toBe("About Page");
		expect(requestAnimationFrameSpy).toHaveBeenCalledTimes(
			rAFCallCountBeforeStale,
		);
		expect(
			document.head.querySelector(
				'link[data-vorma-css-bundle="/stale-only.css"]',
			),
		).toBeNull();

		document.body.removeChild(anchor);
	});
});
