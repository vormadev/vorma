import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { createPatternRegistry } from "vorma/kit/matcher/register";
import {
	beginNavigation,
	getBuildID,
	getHistoryInstance,
	getLocation,
	getRootEl,
	getStatus,
	navigationStateManager,
	revalidate,
	submit,
	vormaNavigate,
} from "./client";
import {
	addBuildIDListener,
	addLocationListener,
	addRouteChangeListener,
	addStatusListener,
	type RouteChangeEventDetail,
	type StatusEventDetail,
} from "./events.ts";
import { customHistoryListener, initCustomHistory } from "./history/history.ts";
import { initClient } from "./init_client.ts";
import { __getPrefetchHandlers, __makeLinkOnClickFn } from "./links.ts";
import {
	__applyScrollState,
	type ScrollState,
} from "./scroll_state_manager.ts";
import { __vormaClientGlobal } from "./vorma_ctx/vorma_ctx.ts";
import {
	createMockResponse,
	describeNavigationTestSuite,
	setupGlobalVormaContext,
	vormaAppConfig,
} from "./client.test.helpers.ts";

describeNavigationTestSuite(({ addCleanup, addListener }) => {
	describe("1. Core Navigation", () => {
		describe("1.1 Navigation Types", () => {
			it("should handle userNavigation type correctly", async () => {
				vi.mocked(fetch).mockResolvedValue(
					createMockResponse({
						title: { dangerousInnerHTML: "User Nav Page" },
						importURLs: [],
						cssBundles: [],
					}),
				);

				await vormaNavigate("/user-nav");
				await vi.runAllTimersAsync();

				expect(fetch).toHaveBeenCalledWith(
					expect.objectContaining({
						href: "http://localhost:3000/user-nav?vorma_json=1",
					}),
					expect.any(Object),
				);
			});

			it("should handle browserHistory navigation (back/forward)", async () => {
				// Setup: Navigate to create history
				vi.mocked(fetch).mockResolvedValue(
					createMockResponse({
						title: { dangerousInnerHTML: "Page 2" },
						importURLs: [],
						cssBundles: [],
					}),
				);

				const history = getHistoryInstance();

				// Navigate to create history entries
				history.push("/page1");
				const page1Key = history.location.key;

				history.push("/page2");

				// Clear any fetch calls from initialization
				vi.clearAllMocks();

				// Now we need to simulate going back
				// The key insight is that when going back, the location.key changes
				// and the customHistoryListener in the implementation detects this

				// Simulate the browser going back by:
				// 1. Changing the URL back to page1
				window.history.replaceState({}, "", "/page1");

				// 2. Dispatching a popstate event which the history library listens to
				const popstateEvent = new PopStateEvent("popstate", {
					state: { key: page1Key },
				});
				window.dispatchEvent(popstateEvent);

				// Give the async operations time to complete
				await vi.runAllTimersAsync();

				// Should trigger navigation with browserHistory type
				expect(fetch).toHaveBeenCalled();
				expect(fetch).toHaveBeenCalledWith(
					expect.objectContaining({
						href: expect.stringContaining("/page1"),
					}),
					expect.any(Object),
				);
			});

			it("should handle browserHistory navigation (back/forward) -- approach 2", async () => {
				// Setup: Navigate to create history
				vi.mocked(fetch).mockResolvedValue(
					createMockResponse({
						title: { dangerousInnerHTML: "Page 2" },
						importURLs: [],
						cssBundles: [],
					}),
				);

				// Instead of trying to simulate browser back,
				// we can directly test that __navigate handles browserHistory type correctly

				// Clear any existing calls
				vi.clearAllMocks();

				// Directly call navigate with browserHistory type
				// This is what happens internally when the browser back button is pressed
				await beginNavigation({
					href: "/previous-page",
					navigationType: "browserHistory",
				}).promise;

				await vi.runAllTimersAsync();

				// Should trigger navigation with browserHistory type
				expect(fetch).toHaveBeenCalled();
				expect(fetch).toHaveBeenCalledWith(
					expect.objectContaining({
						href: expect.stringContaining("/previous-page"),
					}),
					expect.any(Object),
				);
			});

			it("should handle revalidation type correctly", async () => {
				vi.mocked(fetch).mockResolvedValue(
					createMockResponse({
						title: { dangerousInnerHTML: "Current Page" },
						importURLs: [],
						cssBundles: [],
					}),
				);

				const history = getHistoryInstance();
				const pushSpy = vi.spyOn(history, "push");
				const replaceSpy = vi.spyOn(history, "replace");

				await revalidate();
				await vi.runAllTimersAsync();

				// Revalidation should not change history
				expect(pushSpy).not.toHaveBeenCalled();
				expect(replaceSpy).not.toHaveBeenCalled();
			});

			it("should handle redirect type from server response", async () => {
				vi.mocked(fetch)
					.mockResolvedValueOnce(
						createMockResponse(null, {
							headers: { "X-Client-Redirect": "/redirected" },
						}),
					)
					.mockResolvedValueOnce(
						createMockResponse({
							title: { dangerousInnerHTML: "Redirected Page" },
							importURLs: [],
							cssBundles: [],
						}),
					);

				await vormaNavigate("/original");
				await vi.runAllTimersAsync();

				expect(fetch).toHaveBeenCalledTimes(2);
				expect(window.location.pathname).toBe("/redirected");
			});

			it("should handle prefetch type without affecting loading states", async () => {
				vi.mocked(fetch).mockResolvedValue(
					createMockResponse({ importURLs: [], cssBundles: [] }),
				);

				const statusListener = vi.fn();
				const cleanup = addListener(addStatusListener, statusListener);

				const handlers = __getPrefetchHandlers({
					href: "/prefetch-target",
				});
				handlers?.start({} as Event);

				await vi.advanceTimersByTimeAsync(100);
				await vi.runAllTimersAsync();

				// Prefetch should not trigger loading states
				expect(statusListener).not.toHaveBeenCalledWith(
					expect.objectContaining({
						detail: expect.objectContaining({ isNavigating: true }),
					}),
				);

				cleanup();
			});
		});

		describe("1.2 Navigation State Management", () => {
			it("should enforce single active user navigation", async () => {
				let rejectPromise1: (reason: any) => void;
				let resolvePromise2: (value: any) => void;

				const promise1 = new Promise((resolve, reject) => {
					rejectPromise1 = reject;
				});
				const promise2 = new Promise((resolve) => {
					resolvePromise2 = resolve;
				});

				let callCount = 0;
				vi.mocked(fetch).mockImplementation(() => {
					callCount++;
					if (callCount === 1) return promise1 as any;
					return promise2 as any;
				});

				const control1 = beginNavigation({
					href: "/page1",
					navigationType: "userNavigation",
				});

				expect(control1.abortController).toBeDefined();
				const abortSpy1 = vi.spyOn(control1.abortController!, "abort");

				// Should have one active navigation
				expect(getStatus().isNavigating).toBe(true);

				// Capture any unhandled rejections from the navigation
				const unhandledRejectionHandler = (e: any) => {
					// If it's our abort error, prevent it from failing the test
					if (e.reason?.name === "AbortError") {
						e.preventDefault();
					}
				};

				process.on("unhandledRejection", unhandledRejectionHandler);

				// Start second navigation
				beginNavigation({
					href: "/page2",
					navigationType: "userNavigation",
				});

				// First should be aborted
				expect(abortSpy1).toHaveBeenCalled();

				// Should still have exactly one active navigation
				expect(getStatus().isNavigating).toBe(true);

				// Clean up - reject the aborted promise
				const abortError = new Error("Aborted");
				abortError.name = "AbortError";

				rejectPromise1!(abortError);

				// Resolve the second promise normally
				resolvePromise2!(
					createMockResponse({ importURLs: [], cssBundles: [] }),
				);

				await vi.runAllTimersAsync();

				// Clean up the handler
				process.off("unhandledRejection", unhandledRejectionHandler);
			});

			it("should track all navigation types in navigations Map", async () => {
				vi.mocked(fetch).mockImplementation(
					() => new Promise(() => {}), // Never resolve
				);

				const statusListener = vi.fn();
				const cleanup = addListener(addStatusListener, statusListener);

				// Start navigations of different types
				beginNavigation({
					href: "/nav1",
					navigationType: "userNavigation",
				});
				beginNavigation({ href: "/nav2", navigationType: "prefetch" });
				beginNavigation({
					href: "/nav3",
					navigationType: "revalidation",
				});

				// Wait for debounced status update
				await vi.advanceTimersByTimeAsync(10);

				// Verify through status that we're tracking multiple operations
				expect(statusListener).toHaveBeenCalledWith(
					expect.objectContaining({
						detail: expect.objectContaining({
							isNavigating: true,
							isRevalidating: true,
							isSubmitting: false,
						}),
					}),
				);

				// Verify each type is actually running by checking fetch was called 3 times
				expect(fetch).toHaveBeenCalledTimes(3);

				// Get the actual URL objects that were passed to fetch
				const fetchCalls = vi.mocked(fetch).mock.calls;

				// Verify each navigation type was started with correct URL
				expect(fetchCalls[0]?.[0]).toBeInstanceOf(URL);
				expect((fetchCalls[0]?.[0] as any)?.href).toBe(
					"http://localhost:3000/nav1?vorma_json=1",
				);

				expect(fetchCalls[1]?.[0]).toBeInstanceOf(URL);
				expect((fetchCalls[1]?.[0] as any)?.href).toBe(
					"http://localhost:3000/nav2?vorma_json=1",
				);

				// Revalidation uses current window.location.href (which is "/")
				expect(fetchCalls[2]?.[0]).toBeInstanceOf(URL);
				expect((fetchCalls[2]?.[0] as any)?.href).toBe(
					"http://localhost:3000/?vorma_json=1",
				);

				// Verify the options object structure
				expect(fetchCalls[0]?.[1]).toMatchObject({
					headers: expect.any(Headers),
					signal: expect.any(AbortSignal),
				});
				expect(fetchCalls[1]?.[1]).toMatchObject({
					headers: expect.any(Headers),
					signal: expect.any(AbortSignal),
				});
				expect(fetchCalls[2]?.[1]).toMatchObject({
					headers: expect.any(Headers),
					signal: expect.any(AbortSignal),
				});

				// Verify different navigation types have different effects on status
				// User navigation and revalidation affect loading states
				expect(getStatus().isNavigating).toBe(true);
				expect(getStatus().isRevalidating).toBe(true);

				// Clear all and verify cleanup
				navigationStateManager.clearAll();

				// Wait for status update
				await vi.advanceTimersByTimeAsync(10);

				// All loading states should be cleared
				expect(getStatus()).toEqual({
					isNavigating: false,
					isSubmitting: false,
					isRevalidating: false,
				});

				// Cleanup
				cleanup();
			});

			it("should clean up navigations from map when complete", async () => {
				// Mock a slightly delayed response to ensure we catch the navigating state
				vi.mocked(fetch).mockImplementation(
					() =>
						new Promise((resolve) =>
							setTimeout(
								() =>
									resolve(
										createMockResponse({
											importURLs: [],
											cssBundles: [],
										}),
									),
								20,
							),
						),
				);

				const statusListener = vi.fn();
				const cleanup = addListener(addStatusListener, statusListener);

				// Clear any pending status events first
				await vi.runAllTimersAsync();
				statusListener.mockClear();

				// Create an AbortController spy to verify cleanup
				const abortControllers: AbortController[] = [];
				const OriginalAbortController = global.AbortController;
				global.AbortController = class extends OriginalAbortController {
					constructor() {
						super();
						abortControllers.push(this);
					}
				} as any;

				// Start navigation
				const navPromise = vormaNavigate("/cleanup-test");

				// IMMEDIATELY check status synchronously - should be navigating
				expect(getStatus().isNavigating).toBe(true);

				// Should have created an AbortController
				expect(abortControllers.length).toBe(1);
				const firstController = abortControllers[0];

				// Wait for debounced status event
				await vi.advanceTimersByTimeAsync(10);

				// Now the listener should have been called with isNavigating: true
				expect(statusListener).toHaveBeenCalledWith(
					expect.objectContaining({
						detail: {
							isNavigating: true,
							isSubmitting: false,
							isRevalidating: false,
						},
					}),
				);

				// Complete the navigation
				await vi.advanceTimersByTimeAsync(20);
				await navPromise;
				await vi.runAllTimersAsync();

				// Status should now be false
				expect(getStatus().isNavigating).toBe(false);

				// The AbortController should not be aborted (navigation completed successfully)
				expect(firstController?.signal.aborted).toBe(false);

				// Verify cleanup by starting a new navigation to same URL - should trigger new fetch
				vi.clearAllMocks();
				abortControllers.length = 0;

				const navPromise2 = vormaNavigate("/cleanup-test");

				// Should create a new AbortController (proves the old one was cleaned up)
				expect(abortControllers.length).toBe(1);
				expect(abortControllers[0]).not.toBe(firstController);

				// Should make a new fetch call (proves previous navigation was cleaned up)
				expect(fetch).toHaveBeenCalledTimes(1);

				// Complete the second navigation
				await vi.advanceTimersByTimeAsync(20);
				await navPromise2;
				await vi.runAllTimersAsync();

				// Restore original AbortController
				global.AbortController = OriginalAbortController;
				cleanup();
			});

			it("should clean up prefetch navigations from map when complete", async () => {
				vi.mocked(fetch).mockResolvedValue(
					createMockResponse({ importURLs: [], cssBundles: [] }),
				);

				const handlers = __getPrefetchHandlers({
					href: "/prefetch-cleanup",
					delayMs: 50,
				});
				handlers?.start({} as Event);

				// Let prefetch start after the 50ms delay
				await vi.advanceTimersByTimeAsync(50);

				// Let prefetch complete
				await vi.runAllTimersAsync();

				// Stop the prefetch to trigger cleanup
				handlers?.stop();

				// Verify cleanup by starting a new prefetch - should trigger new fetch
				vi.clearAllMocks();

				const handlers2 = __getPrefetchHandlers({
					href: "/prefetch-cleanup",
					delayMs: 0, // No delay this time
				});
				handlers2?.start({} as Event);

				// Wait for the prefetch to actually start
				await vi.advanceTimersByTimeAsync(10);

				expect(fetch).toHaveBeenCalledTimes(1); // Would be 0 if old prefetch was still tracked

				// Clean up
				handlers2?.stop();
			});

			it("should properly clean up prefetch navigations and their resources", async () => {
				const abortControllers: AbortController[] = [];
				const OriginalAbortController = global.AbortController;
				global.AbortController = class extends OriginalAbortController {
					constructor() {
						super();
						abortControllers.push(this);
					}
				} as any;

				// Mock fetch with a controllable promise
				let resolveFetch: (value: any) => void;
				const fetchPromise = new Promise((resolve) => {
					resolveFetch = resolve;
				});
				vi.mocked(fetch).mockReturnValue(fetchPromise as any);

				const handlers = __getPrefetchHandlers({
					href: "/prefetch-cleanup-verify",
					delayMs: 50,
				});

				// Start prefetch
				handlers?.start({} as Event);

				// Let prefetch start after the 50ms delay
				await vi.advanceTimersByTimeAsync(50);

				// Should have created an AbortController
				expect(abortControllers.length).toBe(1);
				const prefetchController = abortControllers[0];
				expect(prefetchController?.signal.aborted).toBe(false);

				// Track if abort event fires
				let abortFired = false;
				prefetchController?.signal.addEventListener("abort", () => {
					abortFired = true;
				});

				// Complete the prefetch
				resolveFetch!(
					createMockResponse({ importURLs: [], cssBundles: [] }),
				);
				await vi.runAllTimersAsync();

				// Prefetch should still have its controller (not aborted)
				expect(prefetchController?.signal.aborted).toBe(false);

				// Stop the prefetch to trigger cleanup
				handlers?.stop();

				// Controller should now be aborted
				expect(prefetchController?.signal.aborted).toBe(true);
				expect(abortFired).toBe(true);

				// Verify cleanup by starting a new prefetch - should create new resources
				vi.clearAllMocks();
				abortControllers.length = 0;

				const handlers2 = __getPrefetchHandlers({
					href: "/prefetch-cleanup-verify",
					delayMs: 0, // No delay this time
				});
				handlers2?.start({} as Event);

				// Wait for the prefetch to actually start
				await vi.advanceTimersByTimeAsync(10);

				// Should have created a new AbortController
				expect(abortControllers.length).toBe(1);
				expect(abortControllers[0]).not.toBe(prefetchController);

				// Should make a new fetch call (proves old prefetch was cleaned up)
				expect(fetch).toHaveBeenCalledTimes(1);

				// Clean up
				handlers2?.stop();
				global.AbortController = OriginalAbortController;
			});
		});

		describe("1.3 Link Click Handling", () => {
			it("should prevent default for eligible internal links", async () => {
				vi.mocked(fetch).mockResolvedValue(
					createMockResponse({ importURLs: [], cssBundles: [] }),
				);

				const event = new MouseEvent("click", { bubbles: true });
				const preventDefault = vi.spyOn(event, "preventDefault");
				const anchor = document.createElement("a");
				anchor.href = "/internal-link";

				Object.defineProperty(event, "target", { value: anchor });

				const onClick = __makeLinkOnClickFn({});
				await onClick(event);

				expect(preventDefault).toHaveBeenCalled();

				await vi.runAllTimersAsync();
			});

			it("should ignore external links", async () => {
				const event = new MouseEvent("click", { bubbles: true });
				const preventDefault = vi.spyOn(event, "preventDefault");
				const anchor = document.createElement("a");
				anchor.href = "https://external.com";

				Object.defineProperty(event, "target", { value: anchor });

				const onClick = __makeLinkOnClickFn({});
				await onClick(event);

				expect(preventDefault).not.toHaveBeenCalled();
			});

			it("should ignore clicks with modifier keys", async () => {
				const event = new MouseEvent("click", {
					bubbles: true,
					ctrlKey: true,
				});
				const preventDefault = vi.spyOn(event, "preventDefault");
				const anchor = document.createElement("a");
				anchor.href = "/internal";

				Object.defineProperty(event, "target", { value: anchor });

				const onClick = __makeLinkOnClickFn({});
				await onClick(event);

				expect(preventDefault).not.toHaveBeenCalled();
			});

			it("should handle hash-only links without navigation", async () => {
				window.history.pushState({}, "", "/current-page");

				const event = new MouseEvent("click", { bubbles: true });
				const anchor = document.createElement("a");
				anchor.href = "/current-page#section";

				Object.defineProperty(event, "target", { value: anchor });

				const onClick = __makeLinkOnClickFn({});
				await onClick(event);

				// Should save scroll state but not navigate
				expect(fetch).not.toHaveBeenCalled();

				const scrollState = JSON.parse(
					sessionStorage.getItem("__vorma__scrollStateMap") || "[]",
				);
				expect(scrollState).toBeDefined();
			});

			it("should use prefetch data immediately on click if available", async () => {
				const prefetchData = {
					title: { dangerousInnerHTML: "Prefetched Content" },
					importURLs: [],
					cssBundles: [],
				};

				vi.mocked(fetch).mockResolvedValue(
					createMockResponse(prefetchData),
				);

				// Start prefetch
				const handlers = __getPrefetchHandlers({
					href: "/prefetch-click",
				});
				handlers?.start({} as Event);
				await vi.advanceTimersByTimeAsync(100);
				await vi.runAllTimersAsync();

				// Create a proper click event with an anchor element
				const anchor = document.createElement("a");
				anchor.href = "/prefetch-click";
				document.body.appendChild(anchor);

				const event = new MouseEvent("click", {
					bubbles: true,
					cancelable: true,
				});
				Object.defineProperty(event, "target", { value: anchor });

				const preventDefault = vi.spyOn(event, "preventDefault");

				// Click while prefetch is complete
				await handlers?.onClick(event);
				await vi.runAllTimersAsync();

				expect(preventDefault).toHaveBeenCalled();
				expect(document.title).toBe("Prefetched Content");

				// Clean up
				document.body.removeChild(anchor);
				handlers?.stop();
			});
		});

		describe("1.4 Programmatic Navigation", () => {
			it("should support navigate() with replace option", async () => {
				vi.mocked(fetch).mockResolvedValue(
					createMockResponse({ importURLs: [], cssBundles: [] }),
				);

				const history = getHistoryInstance();
				const replaceSpy = vi.spyOn(history, "replace");
				const pushSpy = vi.spyOn(history, "push");

				await vormaNavigate("/replace-test", { replace: true });
				await vi.runAllTimersAsync();

				// Should have called replace, not push
				expect(replaceSpy).toHaveBeenCalled();
				expect(pushSpy).not.toHaveBeenCalled();

				// Verify it was called with a URL containing our path
				expect(replaceSpy).toHaveBeenCalledWith(
					expect.stringContaining("/replace-test"),
					undefined,
				);
			});
		});
	});

});
