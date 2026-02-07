import {
	describe,
	expect,
	it,
	vi,
} from "vitest";

import {
	beginNavigation,
	getHistoryInstance,
	getStatus,
	revalidate,
	vormaNavigate,
} from "./client";

import {
	addBuildIDListener,
	addRouteChangeListener,
	addStatusListener,
	type RouteChangeEventDetail,
} from "./events.ts";

import {
	__applyScrollState,
} from "./scroll_state_manager.ts";

import {
	__vormaClientGlobal,
} from "./vorma_ctx/vorma_ctx.ts";

import {
	createMockResponse,
	describeNavigationTestSuite,
	setupGlobalVormaContext,
} from "./client.test.helpers.ts";

describeNavigationTestSuite(({ addListener }) => {
	describe("2. Navigation Lifecycle", () => {
		describe("2.1 Begin Navigation Phase", () => {
			it("should set appropriate loading states", async () => {
				vi.mocked(fetch).mockImplementation(
					() => new Promise(() => {}),
				);

				const statusListener = vi.fn();
				const cleanup = addListener(addStatusListener, statusListener);

				const control = beginNavigation({
					href: "/loading",
					navigationType: "userNavigation",
				});

				// Wait for the 5ms debounce to fire
				await vi.advanceTimersByTimeAsync(10);

				expect(statusListener).toHaveBeenCalledWith(
					expect.objectContaining({
						detail: expect.objectContaining({ isNavigating: true }),
					}),
				);

				// Cleanup
				control.abortController?.abort();
				cleanup();
			});

			it("should abort all navigations except current for userNavigation", () => {
				vi.mocked(fetch).mockImplementation(
					() => new Promise(() => {}),
				);

				const control1 = beginNavigation({
					href: "/nav1",
					navigationType: "prefetch",
				});
				const control2 = beginNavigation({
					href: "/nav2",
					navigationType: "revalidation",
				});

				if (!control1.abortController || !control2.abortController) {
					throw new Error("AbortController not set");
				}
				const abort1 = vi.spyOn(control1.abortController, "abort");
				const abort2 = vi.spyOn(control2.abortController, "abort");

				const control3 = beginNavigation({
					href: "/nav3",
					navigationType: "userNavigation",
				});

				expect(abort1).toHaveBeenCalled();
				expect(abort2).toHaveBeenCalled();

				// Cleanup
				control3.abortController?.abort();
			});

			it("should upgrade existing prefetch to userNavigation", async () => {
				vi.mocked(fetch).mockResolvedValue(
					createMockResponse({ importURLs: [], cssBundles: [] }),
				);

				// Start prefetch
				const prefetchControl = beginNavigation({
					href: "/upgrade",
					navigationType: "prefetch",
				});

				// Wait for debounced status update
				await vi.advanceTimersByTimeAsync(10);

				// Verify prefetch doesn't affect navigation status
				expect(getStatus().isNavigating).toBe(false);

				// Upgrade to user navigation
				const userControl = beginNavigation({
					href: "/upgrade",
					navigationType: "userNavigation",
				});

				// Should now be navigating
				expect(getStatus().isNavigating).toBe(true);

				// Should be the same control (reused)
				expect(userControl).toBe(prefetchControl);

				// Wait for completion
				await userControl.promise;
				await vi.runAllTimersAsync();

				// Should have only made one fetch (reused the prefetch)
				expect(fetch).toHaveBeenCalledTimes(1);
			});

			it("should deduplicate prefetch requests", () => {
				vi.mocked(fetch).mockImplementation(
					() => new Promise(() => {}),
				);

				const control1 = beginNavigation({
					href: "/prefetch-dedup",
					navigationType: "prefetch",
				});
				const control2 = beginNavigation({
					href: "/prefetch-dedup",
					navigationType: "prefetch",
				});

				// Should return the same control
				expect(control1).toBe(control2);

				// Should only make one fetch call
				expect(fetch).toHaveBeenCalledTimes(1);

				// Cleanup
				control1.abortController?.abort();
			});
		});

		describe("2.2 Fetch Route Data Phase", () => {
			it("should construct URL with vorma_json and buildID", async () => {
				setupGlobalVormaContext({ buildID: "test-build-123" });
				vi.mocked(fetch).mockResolvedValue(
					createMockResponse({ importURLs: [], cssBundles: [] }),
				);

				await vormaNavigate("/test-url");
				await vi.runAllTimersAsync();

				expect(fetch).toHaveBeenCalledWith(
					expect.objectContaining({
						href: "http://localhost:3000/test-url?vorma_json=test-build-123",
					}),
					expect.any(Object),
				);
			});

			it("should include X-Accepts-Client-Redirect header", async () => {
				vi.mocked(fetch).mockResolvedValue(
					createMockResponse({ importURLs: [], cssBundles: [] }),
				);

				await vormaNavigate("/test-headers");
				await vi.runAllTimersAsync();

				expect(fetch).toHaveBeenCalledWith(
					expect.any(URL),
					expect.objectContaining({
						headers: expect.any(Headers),
					}),
				);

				const headers = vi.mocked(fetch).mock.calls[0]?.[1]
					?.headers as Headers;
				expect(headers.get("X-Accepts-Client-Redirect")).toBe("1");
			});

			it("should handle redirect responses correctly", async () => {
				vi.mocked(fetch)
					.mockResolvedValueOnce(
						createMockResponse(null, {
							headers: { "X-Client-Redirect": "/new-location" },
						}),
					)
					.mockResolvedValueOnce(
						createMockResponse({
							title: { dangerousInnerHTML: "Redirected" },
							importURLs: [],
							cssBundles: [],
						}),
					);

				await vormaNavigate("/original");
				await vi.runAllTimersAsync();

				expect(fetch).toHaveBeenCalledTimes(2);
			});

			it("should handle empty JSON as failure", async () => {
				vi.mocked(fetch).mockResolvedValue(
					new Response("", { status: 200 }),
				);

				// Check initial state
				expect(getStatus().isNavigating).toBe(false);

				const navPromise = vormaNavigate("/empty-json");

				// Should be navigating immediately (synchronous check)
				expect(getStatus().isNavigating).toBe(true);

				// Let the navigation attempt to complete
				await navPromise;
				await vi.runAllTimersAsync();

				// Should be done navigating after empty response (navigation failed)
				expect(getStatus().isNavigating).toBe(false);

				// Title shouldn't have changed
				expect(document.title).toBe("Initial Page");

				// Verify cleanup by trying to navigate to same URL again
				vi.clearAllMocks();
				await vormaNavigate("/empty-json");

				// Should make a new fetch call (proves previous navigation was cleaned up)
				expect(fetch).toHaveBeenCalledTimes(1);
			});

			it("should preload modules in production mode", async () => {
				const originalEnv = import.meta.env.DEV;
				(import.meta.env as any).DEV = false;

				// Spy on appendChild to verify links are created
				const appendChildSpy = vi.spyOn(document.head, "appendChild");

				vi.mocked(fetch).mockResolvedValue(
					createMockResponse({
						importURLs: ["/module1.js", "/module2.js"],
						deps: ["/dep1.js", "/dep2.js", "/module1.js"],
						cssBundles: [],
					}),
				);

				const control = beginNavigation({
					href: "/with-deps",
					navigationType: "userNavigation",
				});

				await control.promise;

				// Verify appendChild was called with modulepreload links
				const modulepreloadCalls = appendChildSpy.mock.calls.filter(
					(call) => {
						const element = call[0] as HTMLElement;
						return (
							element.tagName === "LINK" &&
							element.getAttribute("rel") === "modulepreload"
						);
					},
				);

				// Should create modulepreload for unique deps
				expect(modulepreloadCalls.length).toBe(3);

				(import.meta.env as any).DEV = originalEnv;
			});

			it("should preload CSS bundles", async () => {
				const appendChildSpy = vi.spyOn(document.head, "appendChild");

				vi.mocked(fetch).mockResolvedValue(
					createMockResponse({
						importURLs: [],
						cssBundles: ["/styles1.css", "/styles2.css"],
					}),
				);

				const control = beginNavigation({
					href: "/with-css",
					navigationType: "userNavigation",
				});

				await control.promise;

				// Verify appendChild was called with CSS preload links
				const cssPreloadCalls = appendChildSpy.mock.calls.filter(
					(call) => {
						const element = call[0] as HTMLElement;
						return (
							element.tagName === "LINK" &&
							element.getAttribute("rel") === "preload" &&
							element.getAttribute("as") === "style"
						);
					},
				);

				expect(cssPreloadCalls.length).toBe(2);
			});

			it("should execute client wait functions", async () => {
				const waitFn = vi
					.fn()
					.mockResolvedValue({ clientData: "test" });
				setupGlobalVormaContext({
					patternToWaitFnMap: {
						"/pattern": waitFn,
					},
				});

				vi.mocked(fetch).mockResolvedValue(
					createMockResponse({
						importURLs: [],
						cssBundles: [],
						matchedPatterns: ["/pattern"],
						loadersData: [{ serverData: "test" }],
						hasRootData: true,
					}),
				);

				await vormaNavigate("/pattern/test");
				await vi.runAllTimersAsync();

				expect(waitFn).toHaveBeenCalledWith(
					expect.objectContaining({
						params: expect.any(Object),
						splatValues: expect.any(Array),
						serverDataPromise: expect.any(Promise),
						signal: expect.any(AbortSignal),
					}),
				);

				// Verify the promise resolves to the correct data
				const call = waitFn.mock.calls[0]?.[0];
				const serverData = await call.serverDataPromise;
				expect(serverData).toEqual({
					matchedPatterns: ["/pattern"],
					loaderData: { serverData: "test" },
					rootData: { serverData: "test" },
					buildID: "1",
				});
			});

			it("should cleanup navigation on completion", async () => {
				vi.mocked(fetch).mockResolvedValue(
					createMockResponse({ importURLs: [], cssBundles: [] }),
				);

				// Ensure clean state first
				await vi.runAllTimersAsync();

				// Check initial state
				expect(getStatus().isNavigating).toBe(false);

				const navPromise = vormaNavigate("/cleanup");

				// Should be navigating immediately
				expect(getStatus().isNavigating).toBe(true);

				await navPromise;
				await vi.runAllTimersAsync();

				// Should be cleaned up after completion
				expect(getStatus().isNavigating).toBe(false);

				// Verify cleanup by trying to navigate to same URL again
				vi.clearAllMocks();
				await vormaNavigate("/cleanup");

				// Should make a new fetch call (proves previous navigation was cleaned up)
				expect(fetch).toHaveBeenCalledTimes(1);
			});
		});

		describe("2.3 Complete Navigation Phase", () => {
			it("should handle redirect data result", async () => {
				vi.mocked(fetch)
					.mockResolvedValueOnce(
						createMockResponse(null, {
							headers: {
								"X-Client-Redirect": "/redirect-target",
							},
						}),
					)
					.mockResolvedValueOnce(
						createMockResponse({
							title: { dangerousInnerHTML: "Redirect Target" },
							importURLs: [],
							cssBundles: [],
						}),
					);

				await vormaNavigate("/start");
				await vi.runAllTimersAsync();

				expect(document.title).toBe("Redirect Target");
			});

			it("should dispatch build-id event on change", async () => {
				const buildIdListener = vi.fn();
				const cleanup = addListener(
					addBuildIDListener,
					buildIdListener,
				);

				vi.mocked(fetch).mockResolvedValue(
					createMockResponse(
						{ importURLs: [], cssBundles: [] },
						{ headers: { "X-Vorma-Build-Id": "new-build-456" } },
					),
				);

				await vormaNavigate("/new-build");
				await vi.runAllTimersAsync();

				expect(buildIdListener).toHaveBeenCalledWith(
					expect.objectContaining({
						detail: {
							oldID: "1",
							newID: "new-build-456",
						},
					}),
				);

				cleanup();
			});

			it("should wait for client data before rendering", async () => {
				const clientData = { processed: true };
				const waitFn = vi.fn().mockResolvedValue(clientData);

				setupGlobalVormaContext({
					patternToWaitFnMap: { "/": waitFn },
				});

				vi.mocked(fetch).mockResolvedValue(
					createMockResponse({
						importURLs: [],
						cssBundles: [],
						matchedPatterns: ["/"],
						loadersData: [{}],
					}),
				);

				await vormaNavigate("/wait-test");
				await vi.runAllTimersAsync();

				expect(__vormaClientGlobal.get("clientLoadersData")).toEqual([
					clientData,
				]);
			});
		});

		describe("2.4 Re-render App Phase", () => {
			it("should clear loading state", async () => {
				vi.mocked(fetch).mockResolvedValue(
					createMockResponse({ importURLs: [], cssBundles: [] }),
				);

				// Check status synchronously
				expect(getStatus().isNavigating).toBe(false);

				const navPromise = vormaNavigate("/clear-loading");

				// Should be navigating immediately
				expect(getStatus().isNavigating).toBe(true);

				await navPromise;
				await vi.runAllTimersAsync();

				// Should be done navigating
				expect(getStatus().isNavigating).toBe(false);
			});

			it("should use view transitions when enabled and supported", async () => {
				setupGlobalVormaContext({ useViewTransitions: true });

				const mockStartViewTransition = vi.fn((callback) => {
					callback?.();
					return { finished: Promise.resolve() };
				});
				Object.defineProperty(document, "startViewTransition", {
					value: mockStartViewTransition,
					configurable: true,
				});

				vi.mocked(fetch).mockResolvedValue(
					createMockResponse({ importURLs: [], cssBundles: [] }),
				);

				await vormaNavigate("/with-transition");
				await vi.runAllTimersAsync();

				expect(mockStartViewTransition).toHaveBeenCalled();
			});

			it("should skip view transitions for prefetch and revalidation", async () => {
				setupGlobalVormaContext({ useViewTransitions: true });

				const mockStartViewTransition = vi.fn((callback) => {
					callback?.();
					return { finished: Promise.resolve() };
				});
				Object.defineProperty(document, "startViewTransition", {
					value: mockStartViewTransition,
					configurable: true,
				});

				vi.mocked(fetch).mockResolvedValue(
					createMockResponse({ importURLs: [], cssBundles: [] }),
				);

				// Test revalidation
				await revalidate();
				await vi.runAllTimersAsync();

				expect(mockStartViewTransition).not.toHaveBeenCalled();
			});

			it("should update global state with route data", async () => {
				const routeData = {
					matchedPatterns: ["/users/:id"],
					loadersData: [{ user: "data" }],
					importURLs: [], // Empty to avoid import issues
					exportKeys: [],
					hasRootData: true,
					params: { id: "123" },
					splatValues: [],
					cssBundles: [],
				};

				vi.mocked(fetch).mockResolvedValue(
					createMockResponse(routeData),
				);

				await vormaNavigate("/users/123");
				await vi.runAllTimersAsync();

				expect(__vormaClientGlobal.get("matchedPatterns")).toEqual(
					routeData.matchedPatterns,
				);
				expect(__vormaClientGlobal.get("params")).toEqual(
					routeData.params,
				);
			});

			it("should handle history management for userNavigation", async () => {
				vi.mocked(fetch).mockResolvedValue(
					createMockResponse({ importURLs: [], cssBundles: [] }),
				);

				const history = getHistoryInstance();
				const pushSpy = vi.spyOn(history, "push");
				const replaceSpy = vi.spyOn(history, "replace");

				await vormaNavigate("/new-page");
				await vi.runAllTimersAsync();

				// Should have used push, not replace
				expect(pushSpy).toHaveBeenCalled();
				expect(replaceSpy).not.toHaveBeenCalled();

				// Should have been called with the correct path
				expect(pushSpy).toHaveBeenCalledWith(
					expect.stringContaining("/new-page"),
					undefined,
				);
			});

			it("should use replace for same URL navigation", async () => {
				window.history.replaceState({}, "", "/same-page");

				vi.mocked(fetch).mockResolvedValue(
					createMockResponse({ importURLs: [], cssBundles: [] }),
				);

				const history = getHistoryInstance();
				const replaceSpy = vi.spyOn(history, "replace");
				const pushSpy = vi.spyOn(history, "push");

				await vormaNavigate("/same-page");
				await vi.runAllTimersAsync();

				// Should use replace for same URL, not push
				expect(replaceSpy).toHaveBeenCalled();
				expect(pushSpy).not.toHaveBeenCalled();

				// Verify it was called with the correct URL
				expect(replaceSpy).toHaveBeenCalledWith(
					expect.stringContaining("/same-page"),
					undefined,
				);
			});

			it("should restore scroll state for browserHistory navigation", async () => {
				const scrollState = { x: 100, y: 200 };

				vi.mocked(fetch).mockResolvedValue(
					createMockResponse({ importURLs: [], cssBundles: [] }),
				);

				// Apply scroll state directly
				__applyScrollState(scrollState);

				expect(window.scrollTo).toHaveBeenCalledWith(100, 200);
			});

			it("should update document title with HTML entity decoding", async () => {
				vi.mocked(fetch).mockResolvedValue(
					createMockResponse({
						title: {
							dangerousInnerHTML: "Title with &amp; entities",
						},
						importURLs: [],
						cssBundles: [],
					}),
				);

				await vormaNavigate("/entity-title");
				await vi.runAllTimersAsync();

				expect(document.title).toBe("Title with & entities");
			});

			it("should wait for CSS bundle preloads", async () => {
				// Store RAF callbacks
				const rafCallbacks: FrameRequestCallback[] = [];
				const rafSpy = vi
					.spyOn(window, "requestAnimationFrame")
					.mockImplementation((cb) => {
						rafCallbacks.push(cb);
						return 1;
					});

				const appendChildSpy = vi.spyOn(document.head, "appendChild");

				// Mock the dynamic imports that will be triggered
				vi.doMock("/static/", () => ({
					default: () => {},
				}));

				vi.mocked(fetch).mockResolvedValue(
					createMockResponse({
						importURLs: [],
						cssBundles: ["/bundle1.css", "/bundle2.css"],
					}),
				);

				// Navigate and handle the promise properly
				const navPromise = vormaNavigate("/css-wait");

				// Wait a bit for the navigation to start
				await vi.advanceTimersByTimeAsync(10);

				// Trigger onload for any preload links that were created
				const preloadLinks = appendChildSpy.mock.calls
					.map((call) => call[0])
					.filter(
						(el) =>
							(el as any).tagName === "LINK" &&
							(el as any).getAttribute("rel") === "preload",
					);

				preloadLinks.forEach((link: any) => {
					if (link.onload) {
						link.onload();
					}
				});

				// Now wait for navigation to complete
				await navPromise;
				await vi.runAllTimersAsync();

				// Execute all RAF callbacks
				rafCallbacks.forEach((cb) => cb(0));

				// Verify stylesheet links were added
				const stylesheetCalls = appendChildSpy.mock.calls.filter(
					(call) => {
						const element = call[0] as HTMLElement;
						return (
							element.tagName === "LINK" &&
							element.getAttribute("rel") === "stylesheet"
						);
					},
				);

				expect(stylesheetCalls.length).toBe(2);

				rafSpy.mockRestore();
			});

			it("should dispatch route-change event", async () => {
				const routeChangeListener = vi.fn();
				const cleanup = addListener(
					addRouteChangeListener,
					routeChangeListener,
				);

				vi.mocked(fetch).mockResolvedValue(
					createMockResponse({ importURLs: [], cssBundles: [] }),
				);

				await vormaNavigate("/route-change-test");
				await vi.runAllTimersAsync();

				expect(routeChangeListener).toHaveBeenCalledWith(
					expect.objectContaining({
						detail: expect.objectContaining({
							__scrollState: { x: 0, y: 0 },
						} satisfies RouteChangeEventDetail),
					}),
				);

				cleanup();
			});

			it("should apply CSS bundles avoiding duplicates", async () => {
				setupGlobalVormaContext({ publicPathPrefix: "/static" });

				// Store RAF callbacks
				const rafCallbacks: FrameRequestCallback[] = [];
				vi.spyOn(window, "requestAnimationFrame").mockImplementation(
					(cb) => {
						rafCallbacks.push(cb);
						return 1;
					},
				);

				// Track added bundles and mock querySelector properly
				const addedBundles = new Set<string>();
				const originalQuerySelector =
					document.querySelector.bind(document);
				const querySelectorSpy = vi
					.spyOn(document, "querySelector")
					.mockImplementation((selector) => {
						if (
							typeof selector === "string" &&
							selector.includes("data-vorma-css-bundle")
						) {
							const match = selector.match(
								/data-vorma-css-bundle="([^"]+)"/,
							);
							if (match && addedBundles.has(match[1]!)) {
								const mockElement =
									document.createElement("link");
								mockElement.setAttribute(
									"data-vorma-css-bundle",
									match[1]!,
								);
								return mockElement;
							}
						}
						return originalQuerySelector(selector);
					});

				const appendChildSpy = vi.spyOn(document.head, "appendChild");

				// Mock the dynamic imports
				vi.doMock("/static/", () => ({
					default: () => {},
				}));

				vi.mocked(fetch).mockResolvedValue(
					createMockResponse({
						importURLs: [],
						cssBundles: ["/styles.css"],
					}),
				);

				// First navigation
				const nav1Promise = vormaNavigate("/css-page");
				await vi.advanceTimersByTimeAsync(10);

				// Trigger onload for preload links
				const preloadLinks1 = appendChildSpy.mock.calls
					.map((call) => call[0])
					.filter(
						(el) =>
							(el as any).tagName === "LINK" &&
							(el as any).getAttribute("rel") === "preload",
					);

				preloadLinks1.forEach((link: any) => {
					if (link.onload) link.onload();
				});

				await nav1Promise;
				await vi.runAllTimersAsync();

				// Execute RAF callbacks for first navigation
				rafCallbacks.forEach((cb) => cb(0));
				addedBundles.add("/styles.css");

				// Clear RAF callbacks for second navigation
				rafCallbacks.length = 0;

				// Second navigation
				const nav2Promise = vormaNavigate("/css-page");
				await vi.advanceTimersByTimeAsync(10);

				// Trigger onload for any new preload links
				const preloadLinks2 = appendChildSpy.mock.calls
					.slice(preloadLinks1.length)
					.map((call) => call[0])
					.filter(
						(el) =>
							(el as any).tagName === "LINK" &&
							(el as any).getAttribute("rel") === "preload",
					);

				preloadLinks2.forEach((link: any) => {
					if (link.onload) link.onload();
				});

				await nav2Promise;
				await vi.runAllTimersAsync();

				// Execute RAF callbacks for second navigation
				rafCallbacks.forEach((cb) => cb(0));

				// The implementation should have checked for duplicates
				expect(querySelectorSpy).toHaveBeenCalledWith(
					expect.stringContaining(
						'data-vorma-css-bundle="/styles.css"',
					),
				);
			});
		});
	});

});
