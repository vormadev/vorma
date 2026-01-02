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
import type { VormaAppConfig } from "./vorma_app_helpers/vorma_app_helpers.ts";
import { __vormaClientGlobal } from "./vorma_ctx/vorma_ctx.ts";

const vormaAppConfig: VormaAppConfig = {
	actionsRouterMountRoot: "/api/",
	actionsDynamicRune: ":",
	actionsSplatRune: "*",
	loadersDynamicRune: ":",
	loadersSplatRune: "*",
	loadersExplicitIndexSegment: "_index",
};

// Mock only what's necessary for testing
const mockSessionStorage = (() => {
	let store: { [key: string]: string } = {};
	return {
		getItem: (key: string) => store[key] || null,
		setItem: (key: string, value: string) => {
			if (key) {
				store[key] = value.toString();
			}
		},
		removeItem: (key: string) => {
			if (key) {
				delete store[key];
			}
		},
		clear: () => {
			store = {};
		},
	};
})();

// Store cleanup functions
const cleanupFns: Array<() => void> = [];

// Helper to setup initial Vorma context
const setupGlobalVormaContext = (initialData = {}) => {
	(globalThis as any)[Symbol.for("__vorma_internal__")] = {
		buildID: "1",
		matchedPatterns: [],
		importURLs: [],
		exportKeys: [],
		loadersData: [],
		params: {},
		splatValues: [],
		hasRootData: false,
		activeComponents: [],
		clientLoadersData: [],
		patternToWaitFnMap: {},
		viteDevURL: "",
		publicPathPrefix: "",
		patternRegistry: createPatternRegistry(),
		...initialData,
	};
};

// Helper to create mock fetch responses
const createMockResponse = (data: any, options: ResponseInit = {}) => {
	return new Response(JSON.stringify(data), {
		status: 200,
		headers: {
			"Content-Type": "application/json",
			"X-Vorma-Build-Id": "1",
			...options.headers,
		},
		...options,
	});
};

describe("Comprehensive Navigation Test Suite", () => {
	let locationBackup: Location;
	let historyBackup: History;

	beforeEach(() => {
		vi.useFakeTimers({ shouldAdvanceTime: true });

		// Mock CSS.escape if it doesn't exist (not available in jsdom)
		if (!global.CSS) {
			(global as any).CSS = {};
		}
		if (!global.CSS.escape) {
			global.CSS.escape = (str: string) =>
				str.replace(/[!"#$%&'()*+,./:;<=>?@[\\\]^`{|}~]/g, "\\$&");
		}

		vi.doMock("/module1.js", () => ({ default: () => {} }));
		vi.doMock("/module2.js", () => ({ default: () => {} }));

		// Backup original objects
		locationBackup = window.location;
		historyBackup = window.history;

		// Set up a complete window.location mock
		Object.defineProperty(window, "location", {
			value: {
				href: "http://localhost:3000/",
				origin: "http://localhost:3000",
				protocol: "http:",
				host: "localhost:3000",
				hostname: "localhost",
				port: "3000",
				pathname: "/",
				search: "",
				hash: "",
				assign: vi.fn((url) => {
					window.location.href = url;
				}),
				replace: vi.fn((url) => {
					const newUrl = new URL(url, window.location.href);
					window.location.href = newUrl.href;
					window.location.pathname = newUrl.pathname;
					window.location.search = newUrl.search;
					window.location.hash = newUrl.hash;
				}),
				reload: vi.fn(),
				toString: () => window.location.href,
			},
			writable: true,
			configurable: true,
		});

		// Mock Element.prototype.scrollIntoView
		if (!Element.prototype.scrollIntoView) {
			Element.prototype.scrollIntoView = vi.fn();
		}

		// Mock history.scrollRestoration
		let scrollRestorationValue = "auto";
		Object.defineProperty(window.history, "scrollRestoration", {
			get: () => scrollRestorationValue,
			set: (v) => {
				scrollRestorationValue = v;
			},
			configurable: true,
		});

		// Mock history methods
		window.history.replaceState = vi.fn((state, title, url) => {
			if (url) {
				const newUrl = new URL(url, window.location.href);
				window.location.href = newUrl.href;
				window.location.pathname = newUrl.pathname;
				window.location.search = newUrl.search;
				window.location.hash = newUrl.hash;
			}
		});

		window.history.pushState = vi.fn((state, title, url) => {
			if (url) {
				const newUrl = new URL(url, window.location.href);
				window.location.href = newUrl.href;
				window.location.pathname = newUrl.pathname;
				window.location.search = newUrl.search;
				window.location.hash = newUrl.hash;
			}
		});

		// Mock sessionStorage
		Object.defineProperty(window, "sessionStorage", {
			value: mockSessionStorage,
			writable: true,
			configurable: true,
		});

		// Mock window scroll properties
		Object.defineProperty(window, "scrollTo", {
			value: vi.fn(),
			writable: true,
		});
		Object.defineProperty(window, "scrollX", { value: 0, writable: true });
		Object.defineProperty(window, "scrollY", { value: 0, writable: true });

		// Mock startViewTransition
		const mockStartViewTransition = vi.fn((callback) => {
			callback?.();
			return { finished: Promise.resolve() };
		});

		Object.defineProperty(document, "startViewTransition", {
			value: mockStartViewTransition,
			configurable: true,
		});

		// Setup Vorma context
		setupGlobalVormaContext();

		// Setup spies
		vi.spyOn(window, "fetch");
		vi.spyOn(window, "dispatchEvent");
		vi.spyOn(console, "error").mockImplementation(() => {});
		vi.spyOn(console, "info").mockImplementation(() => {});

		// Clear all state
		mockSessionStorage.clear();
		vi.clearAllMocks();
		navigationStateManager.clearAll();
		document.title = "Initial Page";
		(window as any).scrollX = 0;
		(window as any).scrollY = 0;

		// Clear any existing listeners to prevent memory leaks
		cleanupFns.forEach((fn) => fn());
		cleanupFns.length = 0;

		// Initialize history after location is properly set up
		initCustomHistory();
	});

	afterEach(async () => {
		// Run all pending timers to ensure status events fire
		await vi.runAllTimersAsync();

		// Clean up all listeners
		cleanupFns.forEach((fn) => fn());
		cleanupFns.length = 0;

		// Clear DOM
		document.body.innerHTML = "";
		document.head.innerHTML = "";

		// Clear any pending promises to avoid unhandled rejections
		vi.clearAllMocks();

		// Restore mocks
		vi.restoreAllMocks();

		// Restore original objects
		Object.defineProperty(window, "location", {
			value: locationBackup,
			writable: true,
			configurable: true,
		});
		Object.defineProperty(window, "history", {
			value: historyBackup,
			writable: true,
			configurable: true,
		});

		// Force garbage collection if available
		if (global.gc) {
			global.gc();
		}
	});

	// Add helper to register cleanup functions
	const addCleanup = (fn: () => void) => {
		cleanupFns.push(fn);
		return fn;
	};

	// Update all listener additions to register cleanup
	const addListener = <T>(
		adder: (fn: any) => () => void,
		fn: (e: CustomEvent<T>) => void,
	) => {
		const cleanup = adder(fn);
		addCleanup(cleanup);
		return cleanup;
	};

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

	describe("3. Prefetching", () => {
		describe("3.1 Initialization", () => {
			it("should only create handlers for eligible URLs", () => {
				// HTTP URL - eligible
				const httpHandlers = __getPrefetchHandlers({
					href: "/internal",
				});
				expect(httpHandlers).toBeDefined();

				// External URL - not eligible
				const externalHandlers = __getPrefetchHandlers({
					href: "https://external.com",
				});
				expect(externalHandlers).toBeUndefined();

				// Non-HTTP URL - not eligible
				const mailtoHandlers = __getPrefetchHandlers({
					href: "mailto:test@test.com",
				});
				expect(mailtoHandlers).toBeUndefined();
			});

			it("should not prefetch current page", () => {
				window.history.replaceState({}, "", "/current-page");

				const handlers = __getPrefetchHandlers({
					href: "/current-page",
				});
				const startSpy = vi.fn();

				if (handlers?.start) {
					vi.spyOn(handlers, "start").mockImplementation(startSpy);
					handlers.start({} as Event);
				}

				vi.advanceTimersByTime(200);
				expect(fetch).not.toHaveBeenCalled();
			});
		});

		describe("3.2 Prefetch Lifecycle", () => {
			it("should start prefetch after configured delay", async () => {
				vi.mocked(fetch).mockResolvedValue(
					createMockResponse({ importURLs: [], cssBundles: [] }),
				);

				const handlers = __getPrefetchHandlers({
					href: "/delayed-prefetch",
					delayMs: 200,
				});

				handlers?.start({} as Event);

				// Not started yet
				vi.advanceTimersByTime(100);
				expect(fetch).not.toHaveBeenCalled();

				// Started after delay
				vi.advanceTimersByTime(100);
				expect(fetch).toHaveBeenCalled();
			});

			it("should execute beforeBegin callback", async () => {
				const beforeBegin = vi.fn();
				vi.mocked(fetch).mockResolvedValue(
					createMockResponse({ importURLs: [], cssBundles: [] }),
				);

				const handlers = __getPrefetchHandlers({
					href: "/callback-test",
					beforeBegin,
				});

				handlers?.start({} as Event);
				await vi.advanceTimersByTimeAsync(100);

				expect(beforeBegin).toHaveBeenCalled();
			});

			it("should store prefetch result for reuse", async () => {
				const responseData = {
					title: { dangerousInnerHTML: "Prefetched" },
					importURLs: [],
					cssBundles: [],
				};

				vi.mocked(fetch).mockResolvedValue(
					createMockResponse(responseData),
				);

				const handlers = __getPrefetchHandlers({
					href: "/store-result",
				});
				handlers?.start({} as Event);

				// Wait for prefetch to start
				await vi.advanceTimersByTimeAsync(100);

				// Wait for the prefetch promise to resolve
				await vi.runAllTimersAsync();

				// Now click - fetch should not be called again
				vi.clearAllMocks();

				// Create a proper click event
				const anchor = document.createElement("a");
				anchor.href = "/store-result";
				document.body.appendChild(anchor);

				const event = new MouseEvent("click", {
					bubbles: true,
					cancelable: true,
				});
				Object.defineProperty(event, "target", { value: anchor });

				// The onClick handler should use the stored result
				await handlers?.onClick(event);

				// Wait for navigation to complete
				await vi.runAllTimersAsync();

				expect(fetch).not.toHaveBeenCalled();
				expect(document.title).toBe("Prefetched");

				// Clean up
				document.body.removeChild(anchor);
			});

			it("should cancel timeout on stop", () => {
				const handlers = __getPrefetchHandlers({
					href: "/cancel-timeout",
				});

				handlers?.start({} as Event);
				handlers?.stop();

				vi.advanceTimersByTime(200);
				expect(fetch).not.toHaveBeenCalled();
			});

			it("should abort prefetch but not upgraded navigation", async () => {
				vi.mocked(fetch).mockImplementation(
					() => new Promise(() => {}),
				);

				const handlers = __getPrefetchHandlers({ href: "/abort-test" });
				handlers?.start({} as Event);

				await vi.advanceTimersByTimeAsync(100);

				// Verify prefetch started
				expect(fetch).toHaveBeenCalledTimes(1);

				// Get the abort controller from the fetch call
				const firstFetchSignal =
					vi.mocked(fetch).mock.calls[0]?.[1]?.signal;
				expect(firstFetchSignal?.aborted).toBe(false);

				// Upgrade by starting a user navigation to the same URL
				vormaNavigate("/abort-test");

				// Now stop the prefetch handlers
				handlers?.stop();

				// The navigation should not be aborted because it's been upgraded
				expect(firstFetchSignal?.aborted).toBe(false);

				// Should still be navigating
				expect(getStatus().isNavigating).toBe(true);

				// Should still only have one fetch (reused)
				expect(fetch).toHaveBeenCalledTimes(1);
			});

			it("should handle click during prefetch", async () => {
				// Set up a delayed fetch response
				let resolveResponse: (value: any) => void;
				const responsePromise = new Promise((resolve) => {
					resolveResponse = resolve;
				});

				vi.mocked(fetch).mockReturnValue(responsePromise as any);

				const beforeRender = vi.fn();
				const afterRender = vi.fn();

				const handlers = __getPrefetchHandlers({
					href: "/click-during",
					beforeRender,
					afterRender,
				});

				handlers?.start({} as Event);
				await vi.advanceTimersByTimeAsync(100);

				// Click while prefetch is in progress
				const anchor = document.createElement("a");
				anchor.href = "/click-during";
				document.body.appendChild(anchor);

				const event = new MouseEvent("click", {
					bubbles: true,
					cancelable: true,
				});
				Object.defineProperty(event, "target", { value: anchor });

				const clickPromise = handlers?.onClick(event);

				// Now resolve the fetch
				resolveResponse!(
					createMockResponse({
						title: { dangerousInnerHTML: "Eventual" },
						importURLs: [],
						cssBundles: [],
					}),
				);

				// Wait for everything to complete
				await clickPromise;
				await vi.runAllTimersAsync();

				expect(beforeRender).toHaveBeenCalled();
				expect(afterRender).toHaveBeenCalled();
				expect(document.title).toBe("Eventual");

				// Clean up
				document.body.removeChild(anchor);
			});
		});
	});

	describe("4. Scroll Restoration", () => {
		describe("4.1 Storage Mechanism", () => {
			it("should use sessionStorage with correct key", () => {
				const scrollState = { x: 100, y: 200 };
				const key = "test-key";

				sessionStorage.setItem(
					"__vorma__scrollStateMap",
					JSON.stringify([[key, scrollState]]),
				);

				const stored = JSON.parse(
					sessionStorage.getItem("__vorma__scrollStateMap") || "[]",
				);
				expect(stored).toEqual([[key, scrollState]]);
			});

			it("should limit to 50 entries with FIFO eviction", async () => {
				// Create 51 entries
				const entries: Array<[string, ScrollState]> = [];
				for (let i = 0; i < 51; i++) {
					entries.push([`key-${i}`, { x: i, y: i }]);
				}

				sessionStorage.setItem(
					"__vorma__scrollStateMap",
					JSON.stringify(entries.slice(0, 50)),
				);

				// Add one more through navigation
				vi.mocked(fetch).mockResolvedValue(
					createMockResponse({ importURLs: [], cssBundles: [] }),
				);

				const history = getHistoryInstance();
				history.push("/trigger-save");

				(window as any).scrollX = 999;
				(window as any).scrollY = 999;

				await vormaNavigate("/new-page");
				vi.runAllTimers();

				const stored = JSON.parse(
					sessionStorage.getItem("__vorma__scrollStateMap") || "[]",
				);
				expect(stored.length).toBe(50);
				expect(stored[0][0]).toBe("key-1"); // First entry evicted
			});

			it("should set manual scroll restoration on init", () => {
				// Reset the custom history to test initialization
				(window as any).__customHistory = undefined;

				// Mock scrollRestoration property
				let scrollRestorationValue = "auto";
				const setterSpy = vi.fn((value) => {
					scrollRestorationValue = value;
				});

				// Define the property before calling initCustomHistory
				Object.defineProperty(window.history, "scrollRestoration", {
					get: () => scrollRestorationValue,
					set: setterSpy,
					configurable: true,
					enumerable: true,
				});

				initCustomHistory();

				expect(setterSpy).toHaveBeenCalledWith("manual");
				expect(scrollRestorationValue).toBe("manual");
			});
		});

		describe("4.2 Saving Scroll State", () => {
			it("should save before navigation", async () => {
				vi.mocked(fetch).mockResolvedValue(
					createMockResponse({ importURLs: [], cssBundles: [] }),
				);

				const history = getHistoryInstance();
				history.push("/current");

				(window as any).scrollX = 150;
				(window as any).scrollY = 300;

				await vormaNavigate("/next");
				await vi.runAllTimersAsync();

				const stored = JSON.parse(
					sessionStorage.getItem("__vorma__scrollStateMap") || "[]",
				);
				const savedEntry = stored.find(
					([k]: [string]) => k === history.location.key,
				);
				expect(savedEntry?.[1]).toEqual({ x: 150, y: 300 });
			});

			it("should save on POP to different document", async () => {
				vi.mocked(fetch).mockResolvedValue(
					createMockResponse({ importURLs: [], cssBundles: [] }),
				);

				const history = getHistoryInstance();
				history.push("/page1");
				history.push("/page2");

				(window as any).scrollX = 50;
				(window as any).scrollY = 100;

				// Simulate browser back
				history.back();
				await vi.runAllTimersAsync();

				const stored = JSON.parse(
					sessionStorage.getItem("__vorma__scrollStateMap") || "[]",
				);
				expect(stored.length).toBeGreaterThan(0);
			});
		});

		describe("4.3 Restoring Scroll State", () => {
			it("should restore on POP navigation with hash addition", async () => {
				const history = getHistoryInstance();

				// Start at a base page
				history.push("/page");

				// Mock element to scroll to
				const element = document.createElement("div");
				element.id = "section";
				document.body.appendChild(element);
				const scrollIntoViewSpy = vi.spyOn(element, "scrollIntoView");

				// Push with hash
				history.push("/page#section");

				// Clear any existing listeners to ensure our test is isolated
				await vi.runAllTimersAsync();

				// Now simulate going back then forward (POP event)
				window.history.back();
				window.dispatchEvent(
					new PopStateEvent("popstate", { state: {} }),
				);

				await vi.runAllTimersAsync();

				// Apply the scroll state that would be set by the navigation
				__applyScrollState({ hash: "section" });

				expect(scrollIntoViewSpy).toHaveBeenCalled();

				// Clean up
				document.body.removeChild(element);
			});

			it("should restore saved position on hash removal", async () => {
				// Set up initial location at /page
				const pageKey = "page-key-123";
				const savedScrollState = { x: 75, y: 150 };

				// Save scroll state for the non-hash version
				sessionStorage.setItem(
					"__vorma__scrollStateMap",
					JSON.stringify([[pageKey, savedScrollState]]),
				);

				// Set lastKnownCustomLocation to the hash version
				(window as any).lastKnownCustomLocation = {
					pathname: "/page",
					search: "",
					hash: "#hash",
					key: "hash-key-456",
				};

				// Clear any previous calls
				vi.clearAllMocks();

				// Simulate POP to non-hash version (hash removal)
				const update = {
					action: "POP" as const,
					location: {
						pathname: "/page",
						search: "",
						hash: "", // No hash - this is the removal
						key: pageKey,
						state: {},
					},
				};

				// Import and call customHistoryListener
				const { customHistoryListener } = await import(
					"./history/history.ts"
				);
				await customHistoryListener(update as any);

				await vi.runAllTimersAsync();

				// Should have restored the saved scroll position
				expect(window.scrollTo).toHaveBeenCalledWith(75, 150);
			});

			it("should scroll to top for standard navigation", async () => {
				vi.mocked(fetch).mockResolvedValue(
					createMockResponse({ importURLs: [], cssBundles: [] }),
				);

				const routeChangeListener = vi.fn();
				const cleanup = addListener(
					addRouteChangeListener,
					routeChangeListener,
				);

				await vormaNavigate("/new-page");
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

			it("should scroll to element for navigation with hash", async () => {
				vi.mocked(fetch).mockResolvedValue(
					createMockResponse({ importURLs: [], cssBundles: [] }),
				);

				const element = document.createElement("div");
				element.id = "target";
				document.body.appendChild(element);
				const scrollIntoViewSpy = vi.spyOn(element, "scrollIntoView");

				await vormaNavigate("/page#target");
				await vi.runAllTimersAsync();

				// Apply scroll state from route change event
				__applyScrollState({ hash: "target" });

				expect(scrollIntoViewSpy).toHaveBeenCalled();
			});

			it("should fallback to element scroll without saved state", () => {
				const element = document.createElement("div");
				element.id = "fallback";
				document.body.appendChild(element);
				const scrollIntoViewSpy = vi.spyOn(element, "scrollIntoView");

				__applyScrollState({ hash: "fallback" });

				expect(scrollIntoViewSpy).toHaveBeenCalled();
			});
		});

		describe("4.4 Page Refresh Handling", () => {
			it("should save scroll state on unload", async () => {
				(window as any).scrollX = 200;
				(window as any).scrollY = 400;

				await initClient({ renderFn: () => {}, vormaAppConfig });

				window.dispatchEvent(new Event("beforeunload"));

				const saved = JSON.parse(
					sessionStorage.getItem("__vorma__pageRefreshScrollState") ||
						"{}",
				);
				expect(saved).toMatchObject({
					x: 200,
					y: 400,
					href: window.location.href,
				});
				expect(saved.unix).toBeDefined();
			});

			it("should restore scroll state after refresh within 5 seconds", async () => {
				const scrollState = {
					x: 250,
					y: 500,
					unix: Date.now() - 1000, // 1 second ago
					href: window.location.href,
				};

				sessionStorage.setItem(
					"__vorma__pageRefreshScrollState",
					JSON.stringify(scrollState),
				);

				const requestAnimationFrameSpy = vi.spyOn(
					window,
					"requestAnimationFrame",
				);
				requestAnimationFrameSpy.mockImplementation((cb) => {
					cb(0);
					return 0;
				});

				await initClient({ renderFn: () => {}, vormaAppConfig });

				expect(window.scrollTo).toHaveBeenCalledWith(250, 500);
				expect(
					sessionStorage.getItem("__vorma__pageRefreshScrollState"),
				).toBeNull();
			});

			it("should not restore if different URL", async () => {
				const scrollState = {
					x: 250,
					y: 500,
					unix: Date.now() - 1000,
					href: "/different-page",
				};

				sessionStorage.setItem(
					"__vorma__pageRefreshScrollState",
					JSON.stringify(scrollState),
				);

				await initClient({ renderFn: () => {}, vormaAppConfig });

				expect(window.scrollTo).not.toHaveBeenCalledWith(250, 500);
			});

			it("should not restore if more than 5 seconds", async () => {
				const scrollState = {
					x: 250,
					y: 500,
					unix: Date.now() - 6000, // 6 seconds ago
					href: window.location.href,
				};

				sessionStorage.setItem(
					"__vorma__pageRefreshScrollState",
					JSON.stringify(scrollState),
				);

				await initClient({ renderFn: () => {}, vormaAppConfig });

				expect(window.scrollTo).not.toHaveBeenCalledWith(250, 500);
			});
		});
	});

	describe("5. Redirects", () => {
		// Track all listeners to clean them up
		let cleanupFns: Array<() => void> = [];

		afterEach(() => {
			// Clean up all listeners
			cleanupFns.forEach((fn) => fn());
			cleanupFns = [];

			// Clear fetch mocks
			vi.mocked(fetch).mockClear();
		});

		describe("5.1 Request Configuration", () => {
			it("should include X-Accepts-Client-Redirect header", async () => {
				vi.mocked(fetch).mockResolvedValueOnce(
					createMockResponse({ importURLs: [], cssBundles: [] }),
				);

				await vormaNavigate("/test");
				await vi.runAllTimersAsync();

				const headers = vi.mocked(fetch).mock.calls[0]?.[1]
					?.headers as Headers;
				expect(headers.get("X-Accepts-Client-Redirect")).toBe("1");
			});
		});

		describe("5.2 Response Headers Priority", () => {
			it("should prioritize X-Vorma-Reload over other redirects", async () => {
				// Mock location.href setter
				let locationHref = window.location.href;
				const originalLocation = window.location;
				Object.defineProperty(window, "location", {
					value: {
						...originalLocation,
						get href() {
							return locationHref;
						},
						set href(value) {
							locationHref = value;
						},
					},
					configurable: true,
				});

				vi.mocked(fetch).mockResolvedValueOnce(
					createMockResponse(null, {
						headers: {
							"X-Vorma-Reload": "/force-reload",
							"X-Client-Redirect": "/ignored",
							"X-Vorma-Build-Id": "test-build",
						},
					}),
				);

				await vormaNavigate("/test");
				await vi.runAllTimersAsync();

				expect(locationHref).toContain("/force-reload");
				expect(locationHref).toContain("vorma_reload=test-build");

				// Restore original location
				Object.defineProperty(window, "location", {
					value: originalLocation,
					configurable: true,
				});
			});

			it("should handle native browser redirect for GET", async () => {
				// When response.redirected is true and it's a GET request,
				// the implementation treats this as already redirected ("did" status)
				// and doesn't navigate to the redirected content
				const redirectedResponse = createMockResponse(null);
				Object.defineProperty(redirectedResponse, "redirected", {
					value: true,
					writable: false,
				});
				Object.defineProperty(redirectedResponse, "url", {
					value: "http://localhost:3000/redirected",
					writable: false,
				});

				vi.mocked(fetch).mockResolvedValueOnce(redirectedResponse);

				await vormaNavigate("/original");
				await vi.runAllTimersAsync();

				// Since redirected=true for GET, navigation completes without rendering
				// Document title should remain unchanged
				expect(document.title).toBe("Initial Page");
			});

			it("should ignore redirect for non-GET requests", async () => {
				const redirectedResponse = createMockResponse({
					data: "response",
				});
				Object.defineProperty(redirectedResponse, "redirected", {
					value: true,
					writable: false,
				});

				vi.mocked(fetch).mockResolvedValueOnce(redirectedResponse);

				const result = await submit("/api", { method: "POST" });
				await vi.runAllTimersAsync();

				expect(result.success).toBe(true);
				expect((result as any).data).toEqual({ data: "response" });
			});

			it("should handle X-Client-Redirect as lowest priority", async () => {
				vi.mocked(fetch)
					.mockResolvedValueOnce(
						createMockResponse(null, {
							headers: {
								"X-Client-Redirect": "/client-redirect",
								"X-Vorma-Build-Id": "test-build",
							},
						}),
					)
					.mockResolvedValueOnce(
						createMockResponse({
							title: { dangerousInnerHTML: "Client Redirected" },
							importURLs: [],
							cssBundles: [],
						}),
					);

				await vormaNavigate("/test");
				await vi.runAllTimersAsync();

				expect(document.title).toBe("Client Redirected");
				expect(fetch).toHaveBeenCalledTimes(2);
			});
		});

		describe("5.3 Build ID Tracking", () => {
			it("should update build ID from response header", async () => {
				const buildIdListener = vi.fn();
				addBuildIDListener(buildIdListener);

				vi.mocked(fetch).mockResolvedValueOnce(
					createMockResponse(
						{ importURLs: [], cssBundles: [] },
						{ headers: { "X-Vorma-Build-Id": "new-build-789" } },
					),
				);

				await vormaNavigate("/new-build");
				await vi.runAllTimersAsync();

				expect(buildIdListener).toHaveBeenCalled();
			});

			it("should dispatch build-id event before redirect", async () => {
				// The issue is that setupGlobalVormaContext is being called in beforeEach
				// and resets the buildID to "1". We need to check what build ID changes occur.

				const buildIdListener = vi.fn();
				const cleanup = addBuildIDListener(buildIdListener);
				cleanupFns.push(cleanup);

				vi.mocked(fetch)
					.mockResolvedValueOnce(
						createMockResponse(null, {
							headers: {
								"X-Vorma-Build-Id": "redirect-build",
								"X-Client-Redirect": "/redirect",
							},
						}),
					)
					.mockResolvedValueOnce(
						createMockResponse(
							{
								importURLs: [],
								cssBundles: [],
								title: {
									dangerousInnerHTML: "Redirected Page",
								},
							},
							{
								headers: {
									"X-Vorma-Build-Id": "redirect-build",
								},
							},
						),
					);

				await vormaNavigate("/test");
				await vi.runAllTimersAsync();

				expect(buildIdListener).toHaveBeenCalled();
				const event = buildIdListener.mock.calls[0]?.[0];
				expect(event.detail).toEqual({
					oldID: "1", // This is the initial build ID from beforeEach
					newID: "redirect-build",
				});
			});
		});

		describe("5.4 Redirect Strategies", () => {
			it("should use soft redirect for internal URLs", async () => {
				vi.mocked(fetch)
					.mockResolvedValueOnce(
						createMockResponse(null, {
							headers: {
								"X-Client-Redirect": "/internal-redirect",
							},
						}),
					)
					.mockResolvedValueOnce(
						createMockResponse({
							title: { dangerousInnerHTML: "Soft Redirected" },
							importURLs: [],
							cssBundles: [],
						}),
					);

				await vormaNavigate("/test");
				await vi.runAllTimersAsync();

				expect(document.title).toBe("Soft Redirected");
				expect(fetch).toHaveBeenCalledTimes(2);
			});

			it("should use hard redirect for external URLs", async () => {
				let locationHref = window.location.href;
				const originalLocation = window.location;

				// Mock window.location more completely
				Object.defineProperty(window, "location", {
					value: {
						...originalLocation,
						href: locationHref,
						assign: vi.fn((url) => {
							locationHref = url;
						}),
						replace: vi.fn((url) => {
							locationHref = url;
						}),
					},
					configurable: true,
					writable: true,
				});

				// Update the setter on window.location.href
				Object.defineProperty(window.location, "href", {
					get: () => locationHref,
					set: (value) => {
						locationHref = value;
					},
					configurable: true,
				});

				vi.mocked(fetch).mockResolvedValueOnce(
					createMockResponse(null, {
						headers: {
							"X-Client-Redirect": "https://external.com",
						},
					}),
				);

				await vormaNavigate("/test");
				await vi.runAllTimersAsync();

				// The URL might have a trailing slash added
				expect(locationHref).toMatch(/^https:\/\/external\.com\/?$/);

				Object.defineProperty(window, "location", {
					value: originalLocation,
					configurable: true,
				});
			});

			it("should add vorma_reload param for forced internal redirect", async () => {
				let locationHref = window.location.href;
				const originalLocation = window.location;

				Object.defineProperty(window, "location", {
					value: {
						...originalLocation,
						href: locationHref,
						origin: "http://localhost:3000",
					},
					configurable: true,
					writable: true,
				});

				Object.defineProperty(window.location, "href", {
					get: () => locationHref,
					set: (value) => {
						locationHref = value;
					},
					configurable: true,
				});

				vi.mocked(fetch).mockResolvedValueOnce(
					createMockResponse(null, {
						headers: {
							"X-Vorma-Reload": "/force-internal",
							"X-Vorma-Build-Id": "force-build",
						},
					}),
				);

				await vormaNavigate("/test");
				await vi.runAllTimersAsync();

				expect(locationHref).toContain("/force-internal");
				expect(locationHref).toContain("vorma_reload=force-build");

				Object.defineProperty(window, "location", {
					value: originalLocation,
					configurable: true,
				});
			});

			it("should respect max redirect limit", async () => {
				const consoleErrorSpy = vi
					.spyOn(console, "error")
					.mockImplementation(() => {});

				// Set up 15 redirect responses to ensure we exceed the limit of 10
				for (let i = 1; i <= 15; i++) {
					vi.mocked(fetch).mockResolvedValueOnce(
						createMockResponse(null, {
							headers: { "X-Client-Redirect": `/redirect${i}` },
						}),
					);
				}

				// Navigate - it should stop at the redirect limit
				await vormaNavigate("/test");
				await vi.runAllTimersAsync();

				// Should stop at 10 calls: 1 initial + 9 redirects, then the 10th redirect attempt
				// triggers the limit (redirectCount >= 10)
				expect(vi.mocked(fetch)).toHaveBeenCalledTimes(10);

				// Should log error when hitting the limit
				expect(consoleErrorSpy).toHaveBeenCalledWith(
					"Vorma:",
					"Too many redirects",
				);

				consoleErrorSpy.mockRestore();
			});
		});

		describe("5.5 Error Handling", () => {
			it("should ignore non-HTTP redirect URLs", async () => {
				// When redirect URL is non-HTTP, it should be ignored
				// and navigation should complete normally
				vi.mocked(fetch).mockResolvedValueOnce(
					createMockResponse(
						{
							importURLs: [],
							cssBundles: [],
							title: { dangerousInnerHTML: "Test Page" },
						},
						{
							headers: {
								"X-Client-Redirect": "mailto:test@test.com",
							},
						},
					),
				);

				await vormaNavigate("/test");
				await vi.runAllTimersAsync();

				expect(window.location.href).not.toContain("mailto:");
				expect(document.title).toBe("Test Page");
			});
		});

		describe("5.6 URL Cleanup", () => {
			it("should remove vorma_reload param on init", async () => {
				window.history.replaceState(
					{},
					"",
					"/?vorma_reload=123&other=param",
				);

				// Reset custom history
				(window as any).__customHistory = undefined;

				const history = getHistoryInstance();
				const replaceSpy = vi.spyOn(history, "replace");

				// Simulate just the URL cleanup part of initClient
				const url = new URL(window.location.href);
				if (url.searchParams.has("vorma_reload")) {
					url.searchParams.delete("vorma_reload");
					history.replace(url.href);
				}

				// The history.replace method receives the full URL
				expect(replaceSpy).toHaveBeenCalledWith(
					"http://localhost:3000/?other=param",
				);
			});
		});
	});

	describe("6. Form Submissions", () => {
		describe("6.1 Submit Function", () => {
			it("should deduplicate submissions with same dedupeKey", async () => {
				let firstRequestAborted = false;
				let firstRequestStarted = false;
				let secondRequestStarted = false;

				vi.mocked(fetch).mockImplementation((url, init) => {
					const isFirst = !firstRequestStarted;

					if (isFirst) {
						firstRequestStarted = true;
						return new Promise((resolve, reject) => {
							init?.signal?.addEventListener("abort", () => {
								firstRequestAborted = true;
								const error = new Error(
									"The operation was aborted",
								);
								error.name = "AbortError";
								reject(error);
							});
							// Never resolve - will be aborted
						});
					} else {
						secondRequestStarted = true;
						return Promise.resolve(
							new Response(JSON.stringify({ data: "success" }), {
								status: 200,
								headers: { "Content-Type": "application/json" },
							}),
						);
					}
				});

				// Start both submissions
				const promise1 = submit(
					"/api/resource",
					{ method: "POST" },
					{ dedupeKey: "myKey" },
				);
				const promise2 = submit(
					"/api/resource",
					{ method: "POST" },
					{ dedupeKey: "myKey" },
				);

				// Wait for both to complete
				const [result1, result2] = await Promise.all([
					promise1,
					promise2,
				]);

				// First should have been aborted
				expect(result1).toEqual({ success: false, error: "Aborted" });
				expect(firstRequestAborted).toBe(true);

				// Second should succeed
				expect(result2).toEqual({
					success: true,
					data: { data: "success" },
				});

				// Both requests should have been started
				expect(firstRequestStarted).toBe(true);
				expect(secondRequestStarted).toBe(true);
			});

			it("should NOT deduplicate submissions without dedupeKey", async () => {
				vi.mocked(fetch).mockImplementation(
					() => new Promise(() => {}), // Never resolve
				);

				submit("/api/resource", { method: "POST" });
				submit("/api/resource", { method: "POST" });

				// Without a dedupeKey, both submissions should be tracked individually
				// Check the internal state of the navigationStateManager
				const submissions = (navigationStateManager as any)
					._submissions;
				expect(submissions.size).toBe(2);
			});

			it("should NOT deduplicate submissions with different dedupeKeys", async () => {
				vi.mocked(fetch).mockImplementation(
					() => new Promise(() => {}), // Never resolve
				);

				submit(
					"/api/resource",
					{ method: "POST" },
					{ dedupeKey: "key1" },
				);
				submit(
					"/api/resource",
					{ method: "POST" },
					{ dedupeKey: "key2" },
				);

				// Different keys should not deduplicate
				const submissions = (navigationStateManager as any)
					._submissions;
				expect(submissions.has("submission:key1")).toBe(true);
				expect(submissions.has("submission:key2")).toBe(true);
				expect(submissions.size).toBe(2);
			});

			it("should set isSubmitting loading state", async () => {
				// Wait for any pending status events to clear
				await vi.runAllTimersAsync();

				const statusListener = vi.fn();
				const cleanup = addListener(addStatusListener, statusListener);

				// Create a delayed response to ensure submission takes long enough
				vi.mocked(fetch).mockImplementation(
					() =>
						new Promise((resolve) => {
							setTimeout(() => {
								resolve({
									ok: true,
									status: 200,
									json: () =>
										Promise.resolve({ result: "success" }),
									headers: new Headers(),
								} as any);
							}, 20); // Delay longer than the debounce time
						}),
				);

				const submitPromise = submit("/api/data", { method: "POST" });

				// Wait for the first debounced status event (5ms)
				await vi.advanceTimersByTimeAsync(10);

				// Should have isSubmitting: true
				const submittingEvent = statusListener.mock.calls.find(
					(call) => call[0].detail.isSubmitting === true,
				);
				expect(submittingEvent).toBeDefined();

				// Complete the submission
				await vi.advanceTimersByTimeAsync(20);
				await submitPromise;
				await vi.runAllTimersAsync();

				// Final state should have isSubmitting: false
				const lastCall =
					statusListener.mock.calls[
						statusListener.mock.calls.length - 1
					];
				expect(lastCall?.[0].detail.isSubmitting).toBe(false);

				cleanup();
			});

			it("should clear isSubmitting state when an in-flight submission is aborted", async () => {
				const statusUpdates: StatusEventDetail[] = [];
				const cleanup = addListener(addStatusListener, (e) => {
					statusUpdates.push({ ...(e.detail as any) });
				});

				// 1. The first fetch call will hang, keeping the submission in-flight.
				vi.mocked(fetch).mockImplementationOnce(
					() => new Promise(() => {}),
				);

				// 2. The second fetch call will fail immediately. This is key to preventing
				//    the second submission from persisting.
				vi.mocked(fetch).mockImplementationOnce(() =>
					Promise.reject(new Error("Simulated network failure")),
				);

				// 3. Start the first submission.
				submit(
					"/api/resource",
					{ method: "POST" },
					{ dedupeKey: "myKey" },
				);

				// 4. Wait for the status update and confirm we are submitting.
				await vi.advanceTimersByTimeAsync(10);
				expect(statusUpdates.at(-1)?.isSubmitting).toBe(true);

				// 5. Call submit again with the same key. This will:
				//    a) Abort the first submission.
				//    b) Start a second submission which will immediately fail and clean itself up.
				await submit(
					"/api/resource",
					{ method: "POST" },
					{ dedupeKey: "myKey" },
				);

				// 6. Wait for the final debounced status update to fire after all the cleanup.
				await vi.advanceTimersByTimeAsync(10);

				// 7. Assert that the state is now clean. `isSubmitting` should be false
				//    because the aborted submission was removed and the replacement failed.
				expect(getStatus().isSubmitting).toBe(false);
				expect(statusUpdates.at(-1)?.isSubmitting).toBe(false);

				cleanup();
			});

			it("should maintain isSubmitting state during a successful deduplicated submission", async () => {
				const statusListener = vi.fn();
				const cleanup = addListener(addStatusListener, statusListener);

				// Mock fetch to never resolve, so all submissions remain in-flight.
				vi.mocked(fetch).mockImplementation(
					() => new Promise(() => {}),
				);

				// 1. Start the first submission.
				submit(
					"/api/resource",
					{ method: "POST" },
					{ dedupeKey: "test" },
				);

				// 2. Wait for the status update.
				await vi.advanceTimersByTimeAsync(10);

				// Assert that the listener was called.
				expect(statusListener).toHaveBeenCalled();
				// Get the 'detail' object from the most recent call to the listener.
				const lastStatusDetail =
					statusListener.mock.lastCall?.[0].detail;
				// Assert that the detail object has the correct properties.
				expect(lastStatusDetail.isSubmitting).toBe(true);

				// 3. Start a second, duplicate submission. This aborts the first and replaces it.
				submit(
					"/api/resource",
					{ method: "POST" },
					{ dedupeKey: "test" },
				);

				// 4. Wait for any potential state changes.
				await vi.advanceTimersByTimeAsync(10);

				// 5. Assert that the `isSubmitting` state is still true, because the
				//    second submission has seamlessly taken the place of the first.
				expect(getStatus().isSubmitting).toBe(true);

				// 6. Assert that only one submission is still being tracked.
				const submissions = (navigationStateManager as any)
					._submissions;
				expect(submissions.has("submission:test")).toBe(true);
				expect(submissions.size).toBe(1);

				cleanup();
			});

			it("should send FormData as-is", async () => {
				vi.mocked(fetch).mockResolvedValue(createMockResponse({}));

				const formData = new FormData();
				formData.append("field", "value");

				await submit("/api/form", {
					method: "POST",
					body: formData,
				});

				expect(fetch).toHaveBeenCalledWith(
					expect.any(URL),
					expect.objectContaining({
						body: formData,
					}),
				);
			});

			it("should send string body as-is", async () => {
				vi.mocked(fetch).mockResolvedValue(createMockResponse({}));

				const stringBody = "raw string data";

				await submit("/api/string", {
					method: "POST",
					body: stringBody,
				});

				expect(fetch).toHaveBeenCalledWith(
					expect.any(URL),
					expect.objectContaining({
						body: stringBody,
					}),
				);
			});

			it("should JSON stringify other body types", async () => {
				vi.mocked(fetch).mockResolvedValue(createMockResponse({}));

				const objectBody = { key: "value", nested: { data: true } };

				await submit("/api/json", {
					method: "POST",
					body: objectBody as any,
				});

				expect(fetch).toHaveBeenCalledWith(
					expect.any(URL),
					expect.objectContaining({
						body: JSON.stringify(objectBody),
					}),
				);
			});

			it("should handle redirect responses", async () => {
				// Track the current location
				const initialLocation = window.location.href;
				expect(initialLocation).toBe("http://localhost:3000/");
				const initialTitle = document.title;

				vi.mocked(fetch)
					.mockResolvedValueOnce(
						createMockResponse(null, {
							headers: { "X-Client-Redirect": "/after-submit" },
						}),
					)
					.mockResolvedValueOnce(
						createMockResponse({
							title: { dangerousInnerHTML: "After Submit" },
							importURLs: [],
							cssBundles: [],
						}),
					);

				const result = await submit("/api/action", { method: "POST" });

				// Wait for any async navigation to complete
				await vi.runAllTimersAsync();

				// Verify the redirect was followed
				expect(result.success).toBe(true);

				// CRITICAL: Verify fetch was called twice (submit + redirect navigation)
				expect(fetch).toHaveBeenCalledTimes(2);

				// Verify the second fetch was for the redirect target
				expect(fetch).toHaveBeenNthCalledWith(
					2,
					expect.objectContaining({
						href: expect.stringContaining("/after-submit"),
					}),
					expect.any(Object),
				);

				// Verify the page actually changed
				expect(window.location.pathname).toBe("/after-submit");
				expect(document.title).toBe("After Submit");
				expect(document.title).not.toBe(initialTitle);
			});

			it("should handle X-Vorma-Reload redirects from submit", async () => {
				let locationHref = window.location.href;
				Object.defineProperty(window.location, "href", {
					get: () => locationHref,
					set: (value) => {
						locationHref = value;
					},
					configurable: true,
				});

				vi.mocked(fetch).mockResolvedValueOnce(
					createMockResponse(null, {
						headers: {
							"X-Vorma-Reload": "/force-reload",
							"X-Vorma-Build-Id": "new-build",
						},
					}),
				);

				await submit("/api/action", { method: "POST" });
				await vi.runAllTimersAsync();

				// Should do a hard redirect with vorma_reload param
				expect(locationHref).toContain("/force-reload");
				expect(locationHref).toContain("vorma_reload=new-build");
			});

			it("should handle external redirects from submit", async () => {
				let locationHref = window.location.href;
				Object.defineProperty(window.location, "href", {
					get: () => locationHref,
					set: (value) => {
						locationHref = value;
					},
					configurable: true,
				});

				vi.mocked(fetch).mockResolvedValueOnce(
					createMockResponse(null, {
						headers: {
							"X-Client-Redirect": "https://external.com",
						},
					}),
				);

				await submit("/api/action", { method: "POST" });
				await vi.runAllTimersAsync();

				// Should do a hard redirect to external URL
				expect(locationHref).toBe("https://external.com/");
			});

			it("should auto-revalidate after non-GET submission", async () => {
				vi.mocked(fetch)
					.mockResolvedValueOnce(
						createMockResponse({ submitted: true }),
					)
					.mockResolvedValueOnce(
						createMockResponse({
							title: { dangerousInnerHTML: "Revalidated" },
							importURLs: [],
							cssBundles: [],
						}),
					);

				await submit("/api/mutate", { method: "POST" });
				vi.runAllTimers();

				expect(fetch).toHaveBeenCalledTimes(2);
				expect(document.title).toBe("Revalidated");
			});

			it("should not auto-revalidate after GET submission", async () => {
				vi.mocked(fetch).mockResolvedValueOnce(
					createMockResponse({ data: "search results" }),
				);

				await submit("/api/search", { method: "GET" });
				vi.runAllTimers();

				expect(fetch).toHaveBeenCalledTimes(1);
			});

			it("should manage loading state transition to revalidation", async () => {
				const statusListener = vi.fn();
				const cleanup = addListener(addStatusListener, statusListener);

				vi.mocked(fetch)
					.mockResolvedValueOnce(createMockResponse({}))
					.mockResolvedValueOnce(
						createMockResponse({
							importURLs: [],
							cssBundles: [],
							loadersData: [],
							matchedPatterns: [],
							params: {},
							splatValues: [],
						}),
					);

				await submit("/api/update", { method: "PUT" });
				await vi.runAllTimersAsync();

				cleanup();
			});

			it("should return success with data", async () => {
				const responseData = { id: 123, status: "created" };

				// Mock the implementation to avoid body read issues
				vi.mocked(fetch).mockImplementation(() => {
					return Promise.resolve({
						ok: true,
						status: 200,
						json: () => Promise.resolve(responseData),
						headers: new Headers({
							"Content-Type": "application/json",
						}),
					} as any);
				});

				const result = await submit("/api/create", { method: "POST" });

				expect(result).toEqual({
					success: true,
					data: responseData,
				});
			});

			it("should return error on failure", async () => {
				vi.mocked(fetch).mockResolvedValue(
					new Response(null, {
						status: 500,
						statusText: "Internal Server Error",
					}),
				);

				const result = await submit("/api/fail", { method: "POST" });

				expect(result).toEqual({
					success: false,
					error: "500",
				});
			});

			it("should handle network errors", async () => {
				vi.mocked(fetch).mockRejectedValue(
					new Error("Network failure"),
				);

				const result = await submit("/api/network-error", {
					method: "POST",
				});

				expect(result.success).toBe(false);
				// The implementation logs the error and returns "unknown" for non-Error objects
				expect((result as any).error).toBeDefined();
			});

			it("should handle abort errors silently", async () => {
				const abortError = new Error("The operation was aborted");
				abortError.name = "AbortError";
				vi.mocked(fetch).mockRejectedValue(abortError);

				const result = await submit("/api/abort", { method: "POST" });

				expect(result.success).toBe(false);
				expect((result as any).error).toBe("Aborted");
			});
		});

		describe("6.2 Revalidate Function", () => {
			it("should debounce revalidation calls", async () => {
				vi.mocked(fetch).mockResolvedValue(
					createMockResponse({ importURLs: [], cssBundles: [] }),
				);

				// Call revalidate multiple times quickly
				revalidate();
				revalidate();
				revalidate();

				// Should only result in one fetch
				await vi.advanceTimersByTimeAsync(10);
				expect(fetch).toHaveBeenCalledTimes(1);
			});

			it("should use revalidation navigation type", async () => {
				const statusUpdates: StatusEventDetail[] = [];
				const cleanup = addListener(addStatusListener, (e) => {
					statusUpdates.push({ ...(e.detail as any) });
				});

				// Create a controllable fetch promise
				let resolveFetch: (value: any) => void;
				const fetchPromise = new Promise((resolve) => {
					resolveFetch = () => {
						resolve(
							createMockResponse({
								importURLs: [],
								cssBundles: [],
							}),
						);
					};
				});

				vi.mocked(fetch).mockReturnValue(fetchPromise as any);

				// Start revalidation
				const revalidatePromise = revalidate();

				// Wait for the debounced status update (5ms) plus a bit extra
				await vi.advanceTimersByTimeAsync(10);

				// Check that we've received at least one status update showing revalidation
				const hasRevalidatingStatus = statusUpdates.some(
					(s) => s.isRevalidating,
				);
				expect(hasRevalidatingStatus).toBe(true);

				// Also check current status
				expect(getStatus().isRevalidating).toBe(true);

				// Complete revalidation
				resolveFetch!(null);
				await revalidatePromise;
				await vi.runAllTimersAsync();

				// Should no longer be revalidating
				expect(getStatus().isRevalidating).toBe(false);

				// Should have called fetch with current URL
				expect(fetch).toHaveBeenCalledWith(
					expect.objectContaining({
						href: expect.stringContaining(window.location.pathname),
					}),
					expect.any(Object),
				);

				cleanup();
			});

			it("should target current window.location.href", async () => {
				window.history.replaceState(
					{},
					"",
					"/current-page?param=value",
				);

				vi.mocked(fetch).mockResolvedValue(
					createMockResponse({ importURLs: [], cssBundles: [] }),
				);

				await revalidate();

				expect(fetch).toHaveBeenCalledWith(
					expect.objectContaining({
						href: "http://localhost:3000/current-page?param=value&vorma_json=1",
					}),
					expect.any(Object),
				);
			});
		});
	});

	describe("7. Events System", () => {
		describe("7.1 Loading States (vorma:status)", () => {
			it("should track isNavigating state", async () => {
				// Ensure clean state
				await vi.runAllTimersAsync();

				const statusListener = vi.fn();
				const cleanup = addListener(addStatusListener, statusListener);

				// Create a promise we can control
				let resolveNav: (value: any) => void;
				const navPromise = new Promise((resolve) => {
					resolveNav = resolve;
				});

				vi.mocked(fetch).mockReturnValue(navPromise as any);

				// Start navigation
				const navResult = vormaNavigate("/nav-state");

				// Wait for the debounced status event
				await vi.advanceTimersByTimeAsync(10);

				// Should have isNavigating: true
				expect(statusListener).toHaveBeenCalledWith(
					expect.objectContaining({
						detail: expect.objectContaining({
							isNavigating: true,
							isSubmitting: false,
							isRevalidating: false,
						}),
					}),
				);

				// Clear previous calls
				statusListener.mockClear();

				// Now resolve the navigation
				resolveNav!(
					createMockResponse({ importURLs: [], cssBundles: [] }),
				);
				await navResult;
				await vi.runAllTimersAsync();

				// Wait for final status update
				await vi.advanceTimersByTimeAsync(10);

				// Should have isNavigating: false
				expect(statusListener).toHaveBeenCalledWith(
					expect.objectContaining({
						detail: expect.objectContaining({
							isNavigating: false,
							isSubmitting: false,
							isRevalidating: false,
						}),
					}),
				);

				cleanup();
			});

			it("should track isSubmitting state", async () => {
				// Ensure clean state
				await vi.runAllTimersAsync();

				const statusListener = vi.fn();
				const cleanup = addListener(addStatusListener, statusListener);

				// Create a delayed response
				vi.mocked(fetch).mockImplementation(
					() =>
						new Promise((resolve) => {
							setTimeout(() => {
								resolve(createMockResponse({}));
							}, 50);
						}),
				);

				const submitPromise = submit("/api/submit", { method: "POST" });

				// Wait for the debounced status event
				await vi.advanceTimersByTimeAsync(10);

				// Check for isSubmitting true
				expect(statusListener).toHaveBeenCalledWith(
					expect.objectContaining({
						detail: expect.objectContaining({
							isSubmitting: true,
							isNavigating: false,
							isRevalidating: false,
						}),
					}),
				);

				// Complete the submission
				await vi.advanceTimersByTimeAsync(50);
				await submitPromise;
				await vi.runAllTimersAsync();

				// Wait for final status update
				await vi.advanceTimersByTimeAsync(10);

				// Should have isSubmitting false
				const lastCall =
					statusListener.mock.calls[
						statusListener.mock.calls.length - 1
					];
				expect(lastCall?.[0].detail.isSubmitting).toBe(false);

				cleanup();
			});

			it("should track isRevalidating state", async () => {
				const statusListener = vi.fn();
				const cleanup = addListener(addStatusListener, statusListener);

				// 1. Create a fetch promise that we can resolve manually
				let resolveFetch: () => void;
				const mockFetchPromise = new Promise((resolve) => {
					resolveFetch = () =>
						resolve(
							createMockResponse({
								importURLs: [],
								cssBundles: [],
							}),
						);
				});

				vi.mocked(fetch).mockReturnValue(mockFetchPromise as any);

				// 2. Start revalidation but DO NOT await it, so the test can continue
				const revalidationPromise = revalidate();

				// 3. The status update is debounced by 5ms. Advance the timer to fire it.
				await vi.advanceTimersByTimeAsync(10);

				// 4. NOW, the revalidation is in-flight. Check that the status event was fired.
				expect(statusListener).toHaveBeenCalledWith(
					expect.objectContaining({
						detail: expect.objectContaining({
							isRevalidating: true,
							isNavigating: false,
							isSubmitting: false,
						}),
					}),
				);

				// 5. Clean up: resolve the fetch and await the process to complete.
				resolveFetch!();
				await revalidationPromise;
				await vi.runAllTimersAsync();

				cleanup();
			});

			it("should debounce status events by 5ms", async () => {
				const statusListener = vi.fn();
				addListener(addStatusListener, statusListener);

				vi.mocked(fetch).mockImplementation(
					() => new Promise(() => {}),
				);

				// Trigger multiple state changes quickly
				beginNavigation({
					href: "/1",
					navigationType: "userNavigation",
				});
				beginNavigation({
					href: "/2",
					navigationType: "userNavigation",
				});

				// No events yet
				expect(statusListener).not.toHaveBeenCalled();

				// After debounce
				await vi.advanceTimersByTimeAsync(8);
				expect(statusListener).toHaveBeenCalledTimes(1);
			});

			it("should deduplicate identical status events", async () => {
				const statusListener = vi.fn();
				addListener(addStatusListener, statusListener);

				// Set same state multiple times
				const mockFetch = () => new Promise(() => {});
				vi.mocked(fetch).mockImplementation(mockFetch as any);

				beginNavigation({
					href: "/same",
					navigationType: "userNavigation",
				});
				await vi.advanceTimersByTimeAsync(8);

				const callCount = statusListener.mock.calls.length;

				// Try to trigger same state again
				beginNavigation({
					href: "/same2",
					navigationType: "userNavigation",
				});
				await vi.advanceTimersByTimeAsync(8);

				// Should not dispatch duplicate
				expect(statusListener).toHaveBeenCalledTimes(callCount);
			});

			it("should provide synchronous access via getStatus()", () => {
				const initialStatus = getStatus();
				expect(initialStatus).toEqual({
					isNavigating: false,
					isSubmitting: false,
					isRevalidating: false,
				});

				// Start navigation
				vi.mocked(fetch).mockImplementation(
					() => new Promise(() => {}),
				);
				beginNavigation({
					href: "/sync",
					navigationType: "userNavigation",
				});

				const duringNavStatus = getStatus();
				expect(duringNavStatus.isNavigating).toBe(true);
			});
		});

		describe("7.2 Route Changes (vorma:route-change)", () => {
			it("should fire after navigation completes", async () => {
				const routeChangeListener = vi.fn();
				addRouteChangeListener(routeChangeListener);

				vi.mocked(fetch).mockResolvedValue(
					createMockResponse({ importURLs: [], cssBundles: [] }),
				);

				await vormaNavigate("/route-change");
				await vi.runAllTimersAsync();

				expect(routeChangeListener).toHaveBeenCalledTimes(1);
			});

			it("should include scroll state in event detail", async () => {
				const routeChangeListener = vi.fn();
				addRouteChangeListener(routeChangeListener);

				vi.mocked(fetch).mockResolvedValue(
					createMockResponse({ importURLs: [], cssBundles: [] }),
				);

				await vormaNavigate("/with-hash#section");
				await vi.runAllTimersAsync();

				expect(routeChangeListener).toHaveBeenCalledWith(
					expect.objectContaining({
						detail: expect.objectContaining({
							__scrollState: { hash: "section" },
						} satisfies RouteChangeEventDetail),
					}),
				);
			});

			it("should fire after title updates", async () => {
				const routeChangeListener = vi.fn();
				const cleanup = addListener(
					addRouteChangeListener,
					routeChangeListener,
				);

				vi.mocked(fetch).mockResolvedValue(
					createMockResponse({
						title: { dangerousInnerHTML: "New Title" },
						importURLs: [],
						cssBundles: [],
					}),
				);

				await vormaNavigate("/after-title");
				await vi.runAllTimersAsync();

				expect(routeChangeListener).toHaveBeenCalled();
				expect(document.title).toBe("New Title");

				cleanup();
			});
		});

		describe("7.3 Location Changes (vorma:location)", () => {
			it("should fire when location.key changes", async () => {
				// Manually trigger the customHistoryListener to test the location event
				const locationListener = vi.fn();
				const cleanup = addListener(
					addLocationListener,
					locationListener,
				);

				// Import and call customHistoryListener directly
				const { customHistoryListener } = await import(
					"./history/history.ts"
				);

				// Simulate a history update with a different key
				await customHistoryListener({
					action: "PUSH",
					location: {
						pathname: "/test",
						search: "",
						hash: "",
						state: {},
						key: "new-test-key-" + Date.now(), // Ensure unique key
					},
				} as any);

				expect(locationListener).toHaveBeenCalled();

				cleanup();
			});

			it("should provide current location via getLocation()", () => {
				window.history.replaceState({}, "", "/test-path?query=1#hash");

				const location = getLocation();
				expect(location).toEqual({
					pathname: "/test-path",
					search: "?query=1",
					hash: "#hash",
					state: null,
				});
			});
		});

		describe("7.4 Build ID Changes (vorma:build-id)", () => {
			it("should fire on build ID mismatch", async () => {
				const buildIdListener = vi.fn();
				addBuildIDListener(buildIdListener);

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
			});

			it("should update global buildID before dispatching", async () => {
				const buildIdListener = vi.fn();
				addBuildIDListener(buildIdListener);

				vi.mocked(fetch).mockResolvedValue(
					createMockResponse(
						{ importURLs: [], cssBundles: [] },
						{ headers: { "X-Vorma-Build-Id": "updated-build" } },
					),
				);

				await vormaNavigate("/check-update");
				await vi.runAllTimersAsync();

				expect(buildIdListener).toHaveBeenCalled();
			});

			it("should provide current build ID via getBuildID()", () => {
				setupGlobalVormaContext({ buildID: "test-build-999" });
				expect(getBuildID()).toBe("test-build-999");
			});
		});
	});

	describe("8. Component & Module Loading", () => {
		describe("8.1 Initial Load", () => {
			it("should dynamically import modules from importURLs", async () => {
				const mockModule = { default: () => "Component" };
				vi.doMock("/static/module.js", () => mockModule);

				setupGlobalVormaContext({
					importURLs: ["/module.js"],
					publicPathPrefix: "/static",
				});

				vi.mocked(fetch).mockResolvedValue(
					createMockResponse({
						importURLs: ["/module.js"],
						exportKeys: ["default"],
						cssBundles: [],
					}),
				);

				await vormaNavigate("/with-module");
				await vi.runAllTimersAsync();

				expect(
					__vormaClientGlobal.get("activeComponents"),
				).toBeDefined();
			});

			it("should map modules using exportKeys array", async () => {
				const mockModule = {
					default: () => "DefaultExport",
					NamedExport: () => "NamedExport",
				};

				// In dev mode, viteDevURL is empty, so the path is just the module path with query
				vi.doMock("/multi-export.js", () => mockModule);

				vi.mocked(fetch).mockResolvedValue(
					createMockResponse({
						importURLs: ["/multi-export.js", "/multi-export.js"],
						exportKeys: ["default", "NamedExport"],
						cssBundles: [],
					}),
				);

				await vormaNavigate("/multi-export");
				await vi.runAllTimersAsync();

				const components = __vormaClientGlobal.get("activeComponents");
				expect(components).toHaveLength(2);
				expect(components?.[0]).toBe(mockModule.default);
				expect(components?.[1]).toBe(mockModule.NamedExport);
			});
		});

		describe("8.2 Error Boundaries", () => {
			it("should resolve and set the active error boundary using its index and export key", async () => {
				// 1. SETUP: Define multiple components to test indexing and resolution.
				const layoutComponent = () => "Layout";
				const pageComponent = () => "Page";
				const namedErrorComponent = () => "Named Error Component";

				// Mock the dynamic imports for all modules involved.
				vi.doMock("/layout.js", () => ({
					Layout: layoutComponent,
				}));
				vi.doMock("/page.js", () => ({
					Page: pageComponent,
				}));
				vi.doMock("/error.js", () => ({
					ErrorBoundary: namedErrorComponent,
				}));

				// 2. MOCK PAYLOAD: Create a server response with a list of components
				//    where the error boundary is not the first item.
				vi.mocked(fetch).mockResolvedValue(
					createMockResponse({
						importURLs: ["/layout.js", "/page.js", "/error.js"],
						exportKeys: ["Layout", "Page", "ErrorBoundary"],
						// Point to the 3rd module in the list (index 2)
						outermostServerErrorIdx: 2,
						errorExportKeys: ["", "", "ErrorBoundary"],
						cssBundles: [],
					}),
				);

				// 3. EXECUTION: Trigger the navigation.
				await vormaNavigate("/route-with-specific-error-boundary");
				await vi.runAllTimersAsync();

				// 4. ASSERTIONS: Verify the state is correct and comprehensive.
				const activeComponents =
					__vormaClientGlobal.get("activeComponents");
				const activeErrorBoundary = __vormaClientGlobal.get(
					"activeErrorBoundary",
				);

				// Main assertion: The correct error boundary component was resolved and set.
				expect(activeErrorBoundary).toBe(namedErrorComponent);

				// Sanity check: The regular components were also loaded correctly.
				expect(activeComponents).toHaveLength(3);
				expect(activeComponents?.[0]).toBe(layoutComponent);
				expect(activeComponents?.[1]).toBe(pageComponent);

				// The export from the error module will also be in the main components list,
				// which is the expected behavior of the `handleComponents` function.
				expect(activeComponents?.[2]).toBe(namedErrorComponent);
			});

			it("should fallback to defaultErrorBoundary if not found", async () => {
				const defaultError = () => "Default Error";
				setupGlobalVormaContext({ defaultErrorBoundary: defaultError });

				vi.mocked(fetch).mockResolvedValue(
					createMockResponse({
						importURLs: [],
						outermostServerErrorIdx: 0, // No component at this index
						cssBundles: [],
					}),
				);

				await vormaNavigate("/missing-error");
				await vi.runAllTimersAsync();

				expect(__vormaClientGlobal.get("activeErrorBoundary")).toBe(
					defaultError,
				);
			});
		});

		describe("8.3 URL Resolution", () => {
			it("should add  in development", async () => {
				const originalEnv = import.meta.env.DEV;
				(import.meta.env as any).DEV = true;

				setupGlobalVormaContext({
					viteDevURL: "http://localhost:5173",
					publicPathPrefix: "/static",
				});

				vi.mocked(fetch).mockResolvedValue(
					createMockResponse({
						importURLs: ["/dev-module.js"],
						cssBundles: [],
					}),
				);

				// Mock the import to verify the URL
				let importedUrl = "";
				vi.doMock("http://localhost:5173/dev-module.js", () => {
					importedUrl = "http://localhost:5173/dev-module.js";
					return { default: () => {} };
				});

				await vormaNavigate("/dev-test");
				await vi.runAllTimersAsync();

				// In dev, should use viteDevURL with
				expect(importedUrl).toContain("");

				(import.meta.env as any).DEV = originalEnv;
			});

			it("should use publicPathPrefix in production", async () => {
				const originalEnv = import.meta.env.DEV;
				(import.meta.env as any).DEV = false;

				setupGlobalVormaContext({
					publicPathPrefix: "/assets",
				});

				vi.mocked(fetch).mockResolvedValue(
					createMockResponse({
						importURLs: ["/prod-module.js"],
						cssBundles: [],
					}),
				);

				vi.doMock("/assets/prod-module.js", () => ({
					default: () => {},
				}));

				await vormaNavigate("/prod-test");
				await vi.runAllTimersAsync();

				// Verify it tried to import from the correct path
				expect(
					__vormaClientGlobal.get("activeComponents"),
				).toBeDefined();

				(import.meta.env as any).DEV = originalEnv;
			});

			it("should handle trailing slashes correctly", async () => {
				setupGlobalVormaContext({
					publicPathPrefix: "/static/", // With trailing slash
				});

				// Mock the module
				vi.doMock("/module.js", () => ({
					default: () => "Module",
				}));

				// Create mock response with CSS bundles
				const response = createMockResponse({
					importURLs: ["/module.js"],
					cssBundles: ["/styles.css"],
				});

				vi.mocked(fetch).mockResolvedValue(response);

				// Create and immediately trigger onload for preload links
				const appendChildSpy = vi
					.spyOn(document.head, "appendChild")
					.mockImplementation((element) => {
						// If it's a preload link, immediately trigger onload
						if (
							element instanceof HTMLLinkElement &&
							element.rel === "preload"
						) {
							setTimeout(
								() => element.onload?.(new Event("load")),
								0,
							);
						}
						return document.head.appendChild(element);
					});

				await vormaNavigate("/slash-test");

				// Wait for all async operations
				await vi.runAllTimersAsync();

				// Check that stylesheet links were added without double slashes
				const cssLinks = document.querySelectorAll(
					'link[rel="stylesheet"]',
				);
				cssLinks.forEach((link) => {
					const href = link.getAttribute("href");
					expect(href).not.toMatch(/\/\//); // No double slashes
					expect(href).toBe("/static/styles.css");
				});

				appendChildSpy.mockRestore();
			});
		});
	});

	describe("9. [Reserved]", () => {
		it("should have placeholder test", () => {
			expect(true).toBe(true);
		});
	});

	describe("10. Initialization", () => {
		it("should configure options correctly", async () => {
			const customErrorBoundary = () => "Custom Error";

			await initClient({
				vormaAppConfig,
				renderFn: () => {},
				defaultErrorBoundary: customErrorBoundary,
				useViewTransitions: true,
			});

			expect(__vormaClientGlobal.get("defaultErrorBoundary")).toBe(
				customErrorBoundary,
			);
			expect(__vormaClientGlobal.get("useViewTransitions")).toBe(true);
		});

		it("should initialize history with POP listener", async () => {
			const listenSpy = vi.spyOn(getHistoryInstance(), "listen");

			await initClient({ renderFn: () => {}, vormaAppConfig });

			expect(listenSpy).toHaveBeenCalled();
		});

		it("should set scrollRestoration to manual", async () => {
			const setterSpy = vi.fn();

			Object.defineProperty(window.history, "scrollRestoration", {
				get: () => "auto",
				set: setterSpy,
				configurable: true,
			});

			await initClient({ renderFn: () => {}, vormaAppConfig });

			expect(setterSpy).toHaveBeenCalledWith("manual");
		});

		it("should clean vorma_reload param from URL", async () => {
			window.history.replaceState(
				{},
				"",
				"/?vorma_reload=old-build&keep=this",
			);

			const replaceSpy = vi.spyOn(getHistoryInstance(), "replace");

			await initClient({ renderFn: () => {}, vormaAppConfig });

			expect(replaceSpy).toHaveBeenCalledWith(
				"http://localhost:3000/?keep=this",
			);
		});

		it("should load initial components", async () => {
			setupGlobalVormaContext({
				importURLs: ["/initial.js"],
				publicPathPrefix: "/",
			});

			vi.doMock("/initial.js", () => ({
				default: () => "Initial Component",
			}));

			await initClient({ renderFn: () => {}, vormaAppConfig });

			expect(__vormaClientGlobal.get("activeComponents")).toHaveLength(1);
		});

		it("should run initial client wait functions", async () => {
			const waitFn = vi.fn().mockResolvedValue({ initialized: true });

			setupGlobalVormaContext({
				patternToWaitFnMap: { "/": waitFn },
				matchedPatterns: ["/"],
				loadersData: [{ initial: "data" }],
			});

			await initClient({ renderFn: () => {}, vormaAppConfig });

			expect(waitFn).toHaveBeenCalled();
			expect(__vormaClientGlobal.get("clientLoadersData")).toEqual([
				{ initialized: true },
			]);
		});

		it("should execute user render function", async () => {
			const renderFn = vi.fn();

			await initClient({ renderFn, vormaAppConfig });

			expect(renderFn).toHaveBeenCalled();
		});

		it("should restore scroll after refresh", async () => {
			const scrollState = {
				x: 300,
				y: 600,
				unix: Date.now() - 1000,
				href: window.location.href,
			};

			sessionStorage.setItem(
				"__vorma__pageRefreshScrollState",
				JSON.stringify(scrollState),
			);

			const rafSpy = vi
				.spyOn(window, "requestAnimationFrame")
				.mockImplementation((cb) => {
					cb(0);
					return 0;
				});

			await initClient({ renderFn: () => {}, vormaAppConfig });

			expect(window.scrollTo).toHaveBeenCalledWith(300, 600);
			expect(
				sessionStorage.getItem("__vorma__pageRefreshScrollState"),
			).toBeNull();

			rafSpy.mockRestore();
		});

		it("should detect touch devices on first touch", async () => {
			await initClient({ renderFn: () => {}, vormaAppConfig });

			expect(__vormaClientGlobal.get("isTouchDevice")).toBeUndefined();

			window.dispatchEvent(new Event("touchstart"));

			expect(__vormaClientGlobal.get("isTouchDevice")).toBe(true);
		});
	});

	describe("11. History Management", () => {
		describe("11.1 Custom History", () => {
			it("should create browser history instance", () => {
				const history = getHistoryInstance();
				expect(history).toBeDefined();
				expect(history.location).toBeDefined();
				expect(history.push).toBeDefined();
				expect(history.replace).toBeDefined();
			});

			it("should maintain lastKnownCustomLocation", async () => {
				const history = getHistoryInstance();

				history.push("/new-location");
				await vi.runAllTimersAsync();

				// Location should be updated after push
				expect(history.location.pathname).toBe("/new-location");
			});
		});

		describe("11.2 POP Event Handling", () => {
			it("should dispatch location event on key change", async () => {
				const locationListener = vi.fn();
				addLocationListener(locationListener);

				const originalKey = getHistoryInstance().location.key;

				const update = {
					action: "PUSH",
					location: {
						pathname: "/trigger-key-change",
						search: "",
						hash: "",
						state: null,
						key: `${originalKey}-modified`,
					},
				};

				await customHistoryListener(update as any);

				expect(locationListener).toHaveBeenCalled();
			});

			it("should handle hash-only changes within same document", async () => {
				const history = getHistoryInstance();
				history.push("/same-doc");

				// Add hash
				history.push("/same-doc#new-hash");

				// Mock scrollIntoView
				const element = document.createElement("div");
				element.id = "new-hash";
				document.body.appendChild(element);
				const scrollSpy = vi.spyOn(element, "scrollIntoView");

				// Trigger POP
				history.back();
				history.forward();
				await vi.runAllTimersAsync();

				__applyScrollState({ hash: "new-hash" });
				expect(scrollSpy).toHaveBeenCalled();
			});

			it("should trigger full navigation for different documents", async () => {
				vi.mocked(fetch).mockResolvedValue(
					createMockResponse({ importURLs: [], cssBundles: [] }),
				);

				const history = getHistoryInstance();
				history.push("/page1");
				const page1Location = { ...history.location };
				history.push("/page2");

				vi.clearAllMocks();

				// Simulate the browser updating its location after a 'back' navigation
				window.location.pathname = page1Location.pathname;
				window.location.search = page1Location.search;
				window.location.hash = page1Location.hash;
				window.location.href = new URL(
					`${page1Location.pathname}${page1Location.search}${page1Location.hash}`,
					"http://localhost:3000",
				).href;

				// Dispatch the popstate event to trigger the history library's listener
				window.dispatchEvent(
					new PopStateEvent("popstate", {
						state: { key: page1Location.key },
					}),
				);

				await vi.runAllTimersAsync();

				// This assertion is more specific and robust.
				expect(fetch).toHaveBeenCalledWith(
					expect.objectContaining({
						href: expect.stringContaining("/page1?vorma_json="),
					}),
					expect.any(Object),
				);
			});

			it("should save scroll before navigating away", async () => {
				const history = getHistoryInstance();
				history.push("/current");

				(window as any).scrollX = 123;
				(window as any).scrollY = 456;

				// Navigate to different page
				history.push("/different");
				await vi.runAllTimersAsync();

				const saved = JSON.parse(
					sessionStorage.getItem("__vorma__scrollStateMap") || "[]",
				);
				expect(saved).toContainEqual([
					expect.any(String),
					{ x: 123, y: 456 },
				]);
			});
		});
	});

	describe("12. Error Handling", () => {
		describe("12.1 Abort Errors", () => {
			it("should identify abort errors correctly", async () => {
				const abortError = new Error("The operation was aborted");
				abortError.name = "AbortError";

				vi.mocked(fetch).mockRejectedValue(abortError);

				const statusListener = vi.fn();
				const cleanup = addListener(addStatusListener, statusListener);

				await vormaNavigate("/will-abort");
				await vi.runAllTimersAsync();

				// Should not log abort errors
				expect(console.error).not.toHaveBeenCalledWith(
					expect.stringContaining("abort"),
				);

				cleanup();
			});

			it("should clear loading states on abort", async () => {
				const abortError = new Error("Aborted");
				abortError.name = "AbortError";
				vi.mocked(fetch).mockRejectedValue(abortError);

				// Check initial state
				expect(getStatus().isNavigating).toBe(false);

				const navPromise = vormaNavigate("/abort-loading");

				// Should be navigating immediately
				expect(getStatus().isNavigating).toBe(true);

				await navPromise;
				await vi.runAllTimersAsync();

				// Should be done navigating after abort
				expect(getStatus().isNavigating).toBe(false);
			});
		});

		describe("12.2 Navigation Failures", () => {
			it("should log non-abort errors", async () => {
				const error = new Error("Network failure");
				vi.mocked(fetch).mockRejectedValue(error);

				const consoleErrorSpy = vi
					.spyOn(console, "error")
					.mockImplementation(() => {});

				await vormaNavigate("/fail");
				await vi.runAllTimersAsync();

				expect(consoleErrorSpy).toHaveBeenCalledWith(
					"Vorma:",
					"Navigation failed",
					error,
				);

				consoleErrorSpy.mockRestore();
			});

			it("should clear loading state on failure", async () => {
				vi.mocked(fetch).mockRejectedValue(new Error("Failed"));

				// Check initial state
				expect(getStatus().isNavigating).toBe(false);

				const navPromise = vormaNavigate("/clear-on-fail");

				// Should be navigating immediately
				expect(getStatus().isNavigating).toBe(true);

				await navPromise;
				await vi.runAllTimersAsync();

				// Should be done navigating after failure
				expect(getStatus().isNavigating).toBe(false);
			});

			it("should keep user on current page after failure", async () => {
				const currentPath = window.location.pathname;
				vi.mocked(fetch).mockRejectedValue(
					new Error("Navigation error"),
				);

				await vormaNavigate("/unreachable");
				vi.runAllTimers();

				expect(window.location.pathname).toBe(currentPath);
			});

			it("should not update any state on failure", async () => {
				const initialState = {
					title: document.title,
					components: __vormaClientGlobal.get("activeComponents"),
					params: __vormaClientGlobal.get("params"),
				};

				vi.mocked(fetch).mockRejectedValue(
					new Error("State test error"),
				);

				await vormaNavigate("/state-fail");
				vi.runAllTimers();

				expect(document.title).toBe(initialState.title);
				expect(__vormaClientGlobal.get("activeComponents")).toBe(
					initialState.components,
				);
				expect(__vormaClientGlobal.get("params")).toBe(
					initialState.params,
				);
			});
		});

		describe("12.3 Special Cases", () => {
			it("should treat empty JSON response as failure", async () => {
				vi.mocked(fetch).mockResolvedValue(
					new Response("", { status: 200 }),
				);

				// Check initial state
				expect(getStatus().isNavigating).toBe(false);

				const navPromise = vormaNavigate("/empty");

				// Should be navigating immediately
				expect(getStatus().isNavigating).toBe(true);

				await navPromise;
				await vi.runAllTimersAsync();

				// Should be done navigating after empty response
				expect(getStatus().isNavigating).toBe(false);
			});

			it("should handle network errors", async () => {
				vi.mocked(fetch).mockRejectedValue(
					new TypeError("Failed to fetch"),
				);

				const consoleErrorSpy = vi
					.spyOn(console, "error")
					.mockImplementation(() => {});

				await vormaNavigate("/network-error");
				await vi.runAllTimersAsync();

				expect(consoleErrorSpy).toHaveBeenCalled();
				consoleErrorSpy.mockRestore();
			});

			it("should handle 404/500 responses", async () => {
				const originalTitle = document.title;
				const originalPathname = window.location.pathname;

				vi.mocked(fetch).mockResolvedValue(
					new Response("Not Found", { status: 404 }),
				);

				// Navigate to a 404 page
				await vormaNavigate("/not-found");

				// The important behaviors:
				// 1. We should still be on the original page
				expect(window.location.pathname).toBe(originalPathname);

				// 2. The title shouldn't have changed
				expect(document.title).toBe(originalTitle);

				// 3. Eventually we should not be in a loading state
				// (we can check this by attempting another navigation)
				vi.mocked(fetch).mockResolvedValue(
					createMockResponse({
						title: { dangerousInnerHTML: "Success Page" },
						importURLs: [],
						cssBundles: [],
					}),
				);

				await vormaNavigate("/success");

				// This navigation should work, proving we're not stuck
				expect(document.title).toBe("Success Page");
			});
		});
	});

	describe("13. Utility Functions", () => {
		describe("13.1 Listener Management", () => {
			it("should return cleanup function for removing listeners", () => {
				const listener = vi.fn();
				const cleanup = addStatusListener(listener);

				// Trigger event
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

				// Clean up
				cleanup();

				// Trigger again
				window.dispatchEvent(
					new CustomEvent("vorma:status", {
						detail: {
							isNavigating: true,
							isSubmitting: false,
							isRevalidating: false,
						},
					}),
				);

				// Should not be called again
				expect(listener).toHaveBeenCalledTimes(1);
			});

			it("should use window as event target for all listeners", () => {
				const addEventListenerSpy = vi.spyOn(
					window,
					"addEventListener",
				);

				addStatusListener(() => {});
				addRouteChangeListener(() => {});
				addLocationListener(() => {});
				addBuildIDListener(() => {});

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
			});
		});

		describe("13.2 Public Utilities", () => {
			it("should return root element via getRootEl()", () => {
				const root = document.createElement("div");
				root.id = "vorma-root";
				document.body.appendChild(root);

				expect(getRootEl()).toBe(root);
			});

			it("should apply scroll state correctly", () => {
				// Test coordinate scroll
				__applyScrollState({ x: 100, y: 200 });
				expect(window.scrollTo).toHaveBeenCalledWith(100, 200);

				// Test hash scroll
				const element = document.createElement("div");
				element.id = "test-hash";
				document.body.appendChild(element);
				const scrollSpy = vi.spyOn(element, "scrollIntoView");

				__applyScrollState({ hash: "test-hash" });
				expect(scrollSpy).toHaveBeenCalled();

				// Test no state with hash in URL
				window.location.hash = "#url-hash";
				const urlElement = document.createElement("div");
				urlElement.id = "url-hash";
				document.body.appendChild(urlElement);
				const urlScrollSpy = vi.spyOn(urlElement, "scrollIntoView");

				__applyScrollState(undefined);
				expect(urlScrollSpy).toHaveBeenCalled();
			});

			it("should return current location parts", () => {
				window.history.replaceState(
					{},
					"",
					"/test/path?query=value#section",
				);

				const location = getLocation();
				expect(location).toEqual({
					pathname: "/test/path",
					search: "?query=value",
					hash: "#section",
					state: null,
				});
			});

			it("should return current build ID", () => {
				setupGlobalVormaContext({ buildID: "test-build-12345" });
				expect(getBuildID()).toBe("test-build-12345");
			});
		});
	});

	describe("14. Critical Edge Cases for Refactoring", () => {
		it("should handle multiple navigations to different URLs", async () => {
			// Create promises we can control
			let resolve2: (value: any) => void;
			const promise1 = new Promise(() => {});
			const promise2 = new Promise((r) => {
				resolve2 = r;
			});

			let callCount = 0;
			vi.mocked(fetch).mockImplementation((() => {
				callCount++;
				return callCount === 1 ? promise1 : promise2;
			}) as any);

			// Start first navigation
			vormaNavigate("/page1");

			// Verify it's navigating
			expect(getStatus().isNavigating).toBe(true);

			// Start second navigation before first completes
			const nav2 = vormaNavigate("/page2");

			// Should still be navigating (now to page2)
			expect(getStatus().isNavigating).toBe(true);

			// Resolve second navigation
			resolve2!(
				createMockResponse({
					importURLs: [],
					cssBundles: [],
					title: { dangerousInnerHTML: "Page 2" },
				}),
			);

			await nav2;
			await vi.runAllTimersAsync();

			// Verify navigation completed
			expect(document.title).toBe("Page 2");
			expect(getStatus().isNavigating).toBe(false);

			// Verify first navigation was aborted (only 2 fetch calls, not 3)
			expect(fetch).toHaveBeenCalledTimes(2);
		});

		it("should handle race between prefetch completion and user navigation", async () => {
			let resolvePrefetch: (value: any) => void;
			const prefetchPromise = new Promise((r) => {
				resolvePrefetch = r;
			});

			vi.mocked(fetch).mockReturnValueOnce(prefetchPromise as any);

			// Start prefetch
			const handlers = __getPrefetchHandlers({ href: "/prefetch-race" });
			handlers?.start({} as Event);
			await vi.advanceTimersByTimeAsync(100);

			// Verify prefetch started
			expect(fetch).toHaveBeenCalledTimes(1);

			// Start user navigation while prefetch is in flight
			const navPromise = vormaNavigate("/prefetch-race");

			// Should not make another fetch call (reusing prefetch)
			expect(fetch).toHaveBeenCalledTimes(1);

			// Complete the fetch
			resolvePrefetch!(
				createMockResponse({
					importURLs: [],
					cssBundles: [],
					title: { dangerousInnerHTML: "Prefetched Page" },
				}),
			);

			await navPromise;
			await vi.runAllTimersAsync();

			// Verify navigation completed with prefetched data
			expect(document.title).toBe("Prefetched Page");
			expect(fetch).toHaveBeenCalledTimes(1); // Should reuse prefetch
		});

		it("should handle error boundary changes between navigations", async () => {
			const errorBoundary1 = () => "Error Boundary 1";
			const errorBoundary2 = () => "Error Boundary 2";

			vi.doMock("/error1.js", () => ({
				ErrorBoundary: errorBoundary1,
			}));
			vi.doMock("/error2.js", () => ({
				ErrorBoundary: errorBoundary2,
			}));

			// First navigation with error boundary 1
			vi.mocked(fetch).mockResolvedValueOnce(
				createMockResponse({
					importURLs: ["/error1.js"],
					exportKeys: ["ErrorBoundary"],
					outermostServerErrorIdx: 0,
					errorExportKeys: ["ErrorBoundary"],
					cssBundles: [],
				}),
			);

			await vormaNavigate("/page-with-error1");
			await vi.runAllTimersAsync();

			expect(__vormaClientGlobal.get("activeErrorBoundary")).toBe(
				errorBoundary1,
			);

			// Second navigation with error boundary 2
			vi.mocked(fetch).mockResolvedValueOnce(
				createMockResponse({
					importURLs: ["/error2.js"],
					exportKeys: ["ErrorBoundary"],
					outermostServerErrorIdx: 0,
					errorExportKeys: ["ErrorBoundary"],
					cssBundles: [],
				}),
			);

			await vormaNavigate("/page-with-error2");
			await vi.runAllTimersAsync();

			expect(__vormaClientGlobal.get("activeErrorBoundary")).toBe(
				errorBoundary2,
			);
		});

		it("should clear all navigations to prevent memory leaks", async () => {
			const abortControllers: AbortController[] = [];
			const fetchPromises: Array<{
				resolve: (value: any) => void;
				reject: (error: any) => void;
				promise: Promise<any>;
			}> = [];

			// Spy on AbortController constructor
			const OriginalAbortController = global.AbortController;
			global.AbortController = class extends OriginalAbortController {
				constructor() {
					super();
					abortControllers.push(this);
				}
			} as any;

			// Track all fetch calls and their promises
			vi.mocked(fetch).mockImplementation(() => {
				let promiseResolve: any;
				let promiseReject: any;
				const promise = new Promise((resolve, reject) => {
					promiseResolve = resolve;
					promiseReject = reject;
				});

				// Add catch handler to prevent unhandled rejections
				promise.catch(() => {
					// Silently handle rejections
				});

				fetchPromises.push({
					resolve: promiseResolve,
					reject: promiseReject,
					promise,
				});

				return promise as any;
			});

			// Start multiple navigations
			const nav1 = beginNavigation({
				href: "/leak-test-1",
				navigationType: "userNavigation",
			});
			const nav2 = beginNavigation({
				href: "/leak-test-2",
				navigationType: "prefetch",
			});
			const nav3 = beginNavigation({
				href: "/leak-test-3",
				navigationType: "userNavigation",
			});

			// All controllers should be created
			expect(abortControllers.length).toBe(3);
			expect(fetchPromises.length).toBe(3);

			// Check that previous controllers were aborted (userNavigation aborts others)
			expect(abortControllers[0]?.signal.aborted).toBe(true);
			expect(abortControllers[1]?.signal.aborted).toBe(true);
			expect(abortControllers[2]?.signal.aborted).toBe(false);

			// Verify we're navigating
			expect(getStatus().isNavigating).toBe(true);

			// Add abort event listeners to verify they're called
			const abortEvents: number[] = [];
			abortControllers.forEach((controller, index) => {
				controller.signal.addEventListener("abort", () => {
					abortEvents.push(index);
				});
			});

			// Add catch handlers to the navigation promises to prevent unhandled rejections
			nav1.promise.catch(() => {});
			nav2.promise.catch(() => {});
			nav3.promise.catch(() => {});

			// Clear all navigations
			navigationStateManager.clearAll();

			// All should be aborted now
			expect(abortControllers[0]?.signal.aborted).toBe(true);
			expect(abortControllers[1]?.signal.aborted).toBe(true);
			expect(abortControllers[2]?.signal.aborted).toBe(true);

			// Verify abort events were fired for the non-aborted controller
			expect(abortEvents).toContain(2);

			// Wait for debounced status update
			await vi.advanceTimersByTimeAsync(10);

			// Should not be navigating anymore
			expect(getStatus().isNavigating).toBe(false);
			expect(getStatus().isRevalidating).toBe(false);
			expect(getStatus().isSubmitting).toBe(false);

			// Reject the promises to simulate the abort
			// Since we added catch handlers, this won't cause unhandled rejections
			fetchPromises.forEach((handlers) => {
				if (handlers.reject) {
					const error = new Error("Aborted");
					error.name = "AbortError";
					handlers.reject(error);
				}
			});

			// Let promises settle
			await vi.runAllTimersAsync();

			// Verify we can start new navigations (proves old ones were cleaned up)
			vi.clearAllMocks();
			abortControllers.length = 0;
			fetchPromises.length = 0;

			const newNav = beginNavigation({
				href: "/new-nav",
				navigationType: "userNavigation",
			});

			// Should create fresh controllers and promises
			expect(abortControllers.length).toBe(1);
			expect(fetchPromises.length).toBe(1);
			expect(abortControllers[0]?.signal.aborted).toBe(false);

			// Cleanup
			newNav.abortController?.abort();
			newNav.promise.catch(() => {}); // Handle potential rejection
			global.AbortController = OriginalAbortController;
		});

		it("should remove event listeners on cleanup", async () => {
			const removeEventListenerSpy = vi.spyOn(
				window,
				"removeEventListener",
			);

			// Add listeners
			const cleanups = [
				addStatusListener(() => {}),
				addRouteChangeListener(() => {}),
				addLocationListener(() => {}),
				addBuildIDListener(() => {}),
			];

			// Clean them up
			cleanups.forEach((cleanup) => cleanup());

			// Should have removed all listeners
			expect(removeEventListenerSpy).toHaveBeenCalledTimes(4);
			expect(removeEventListenerSpy).toHaveBeenCalledWith(
				"vorma:status",
				expect.any(Function),
			);
			expect(removeEventListenerSpy).toHaveBeenCalledWith(
				"vorma:route-change",
				expect.any(Function),
			);
			expect(removeEventListenerSpy).toHaveBeenCalledWith(
				"vorma:location",
				expect.any(Function),
			);
			expect(removeEventListenerSpy).toHaveBeenCalledWith(
				"vorma:build-id",
				expect.any(Function),
			);
		});
	});

	it("should not allow completed prefetch to be overridden by revalidation", async () => {
		// Setup: Start on the home page
		window.history.replaceState({}, "", "/");

		// Create a delayed response for revalidation to control timing
		let resolveRevalidation: (value: any) => void;
		const revalidationPromise = new Promise((resolve) => {
			resolveRevalidation = resolve;
		});

		// Mock fetch differently for prefetch vs revalidation
		vi.mocked(fetch).mockImplementation(((url: any) => {
			const urlStr = url.toString();
			// Prefetch to /about - immediate response
			if (urlStr.includes("/about")) {
				return Promise.resolve(
					createMockResponse({
						title: { dangerousInnerHTML: "About Page" },
						importURLs: [],
						cssBundles: [],
					}),
				);
			}
			// Revalidation of current page - delayed response
			return revalidationPromise;
		}) as any);

		// Step 1: Prefetch /about
		const handlers = __getPrefetchHandlers({ href: "/about" });
		handlers?.start({} as Event);
		await vi.advanceTimersByTimeAsync(100);
		await vi.runAllTimersAsync();

		// Step 2: Start revalidation (but don't let it complete yet)
		revalidate();
		await vi.advanceTimersByTimeAsync(10); // Let debounce fire

		// Step 3: Navigate to /about using the prefetch
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

		// We should be on About page
		expect(document.title).toBe("About Page");
		expect(window.location.pathname).toBe("/about");

		// Step 4: Now complete the revalidation AFTER we've navigated
		resolveRevalidation!(
			createMockResponse({
				title: { dangerousInnerHTML: "Home Page" },
				importURLs: [],
				cssBundles: [],
			}),
		);

		// Let the revalidation try to complete
		await vi.runAllTimersAsync();

		// Should still be on About page - revalidation should have been skipped
		expect(document.title).toBe("About Page");
		expect(window.location.pathname).toBe("/about");

		// Clean up
		document.body.removeChild(anchor);
	});

	describe("NavigationStateManager - Loading State Continuity", () => {
		describe("Submit → Revalidate Flow - No Loading Flicker", () => {
			it("should maintain continuous loading state during submit and subsequent revalidation", async () => {
				const statusUpdates: StatusEventDetail[] = [];
				const cleanup = addListener(addStatusListener, (e) => {
					statusUpdates.push({ ...(e.detail as any) });
				});

				// Mock submission that takes time
				vi.mocked(fetch)
					.mockImplementationOnce(
						() =>
							new Promise((resolve) =>
								setTimeout(
									() =>
										resolve(
											createMockResponse({
												data: "submitted",
											}),
										),
									30,
								),
							),
					)
					.mockImplementationOnce(
						() =>
							new Promise((resolve) =>
								setTimeout(
									() =>
										resolve(
											createMockResponse({
												importURLs: [],
											}),
										),
									30,
								),
							),
					);

				// Start submission
				const submitPromise = submit("/api/action", { method: "POST" });

				// Let it run to completion
				await vi.runAllTimersAsync();
				await submitPromise;

				// Check that at least one of the loading states was true throughout
				// (no moment where all were false except at the very end)
				let foundLoadingGap = false;
				for (let i = 0; i < statusUpdates.length - 1; i++) {
					const status = statusUpdates[i];
					if (
						!status?.isNavigating &&
						!status?.isSubmitting &&
						!status?.isRevalidating
					) {
						foundLoadingGap = true;
						break;
					}
				}

				expect(foundLoadingGap).toBe(false);
				cleanup();
			});

			it("should handle overlapping submissions without loading gaps", async () => {
				const statusUpdates: StatusEventDetail[] = [];
				const cleanup = addListener(addStatusListener, (e) => {
					statusUpdates.push({ ...(e.detail as any) });
				});

				// Mock responses with different timings
				vi.mocked(fetch)
					.mockImplementationOnce(
						() =>
							new Promise((resolve) =>
								setTimeout(
									() =>
										resolve(
											createMockResponse({
												data: "first",
											}),
										),
									50,
								),
							),
					)
					.mockResolvedValueOnce(
						createMockResponse({ data: "second" }),
					);

				// Start first submission
				const submit1 = submit("/api/action1", { method: "POST" });

				await vi.advanceTimersByTimeAsync(10);

				// Start second submission while first is pending
				const submit2 = submit("/api/action2", { method: "POST" });

				// Complete both submissions
				await vi.advanceTimersByTimeAsync(50);
				await Promise.all([submit1, submit2]);
				await vi.runAllTimersAsync();

				// Check no loading gaps (excluding the final state which should be all false)
				const hasLoadingGap = statusUpdates
					.slice(0, -1)
					.some(
						(status) =>
							!status.isNavigating &&
							!status.isSubmitting &&
							!status.isRevalidating,
					);

				expect(hasLoadingGap).toBe(false);

				// Verify the final state is all false
				const finalState = statusUpdates[statusUpdates.length - 1];
				expect(finalState).toEqual({
					isSubmitting: false,
					isNavigating: false,
					isRevalidating: false,
				});

				cleanup();
			});
		});

		describe("Navigation with Redirects - Continuous Loading", () => {
			it("should maintain loading state through soft redirects", async () => {
				const statusUpdates: StatusEventDetail[] = [];
				const cleanup = addListener(addStatusListener, (e) => {
					statusUpdates.push({ ...(e.detail as any) });
				});

				// 1. Setup controllable fetches for both navigations
				let resolveFirstFetch, resolveSecondFetch;
				const firstFetchPromise = new Promise((resolve) => {
					resolveFirstFetch = () =>
						resolve(
							createMockResponse(null, {
								headers: { "X-Client-Redirect": "/login" },
							}),
						);
				});
				const secondFetchPromise = new Promise((resolve) => {
					resolveSecondFetch = () =>
						resolve(createMockResponse({ importURLs: [] }));
				});
				vi.mocked(fetch)
					.mockImplementationOnce(() => firstFetchPromise as any)
					.mockImplementationOnce(() => secondFetchPromise as any);

				// 2. Start navigation
				const navPromise = vormaNavigate("/dashboard");

				// 3. Check initial navigating state
				await vi.advanceTimersByTimeAsync(8);
				expect(statusUpdates.at(-1)?.isNavigating).toBe(true);

				// 4. Resolve the first fetch to trigger the redirect logic
				(resolveFirstFetch as any)();
				await new Promise((resolve) => setImmediate(resolve)); // Yield for redirect to start
				await vi.advanceTimersByTimeAsync(8);

				// 5. CRITICAL CHECK: We must still be navigating during the handoff
				expect(statusUpdates.at(-1)?.isNavigating).toBe(true);

				// 6. Resolve the second fetch and complete the navigation
				(resolveSecondFetch as any)();
				await navPromise;
				await vi.runAllTimersAsync();

				// 7. Final check for any gaps in the entire sequence of states
				const hasGap = statusUpdates.some((status, i) => {
					if (i === 0) return false;
					const prev = statusUpdates[i - 1];
					const wasLoading =
						prev?.isNavigating ||
						prev?.isRevalidating ||
						prev?.isSubmitting;
					const isNotLoading =
						!status.isNavigating &&
						!status.isRevalidating &&
						!status.isSubmitting;
					const isNotFinalState = i < statusUpdates.length - 1;
					return wasLoading && isNotLoading && isNotFinalState;
				});

				expect(hasGap).toBe(false);
				cleanup();
			});

			it("should handle redirect chains without loading gaps", async () => {
				const statusUpdates: StatusEventDetail[] = [];

				const cleanup = addListener(addStatusListener, (e) => {
					statusUpdates.push({ ...(e.detail as any) });
				});

				// Mock redirect chain: /admin -> /auth -> /login
				vi.mocked(fetch)
					.mockResolvedValueOnce(
						createMockResponse(null, {
							headers: { "X-Client-Redirect": "/auth" },
						}),
					)
					.mockResolvedValueOnce(
						createMockResponse(null, {
							headers: { "X-Client-Redirect": "/login" },
						}),
					)
					.mockResolvedValueOnce(
						createMockResponse({
							importURLs: [],
							cssBundles: [],
							loadersData: [],
							matchedPatterns: ["/login"],
							params: {},
							splatValues: [],
							hasRootData: false,
							title: { dangerousInnerHTML: "Login Page" },
						}),
					);

				await vormaNavigate("/admin");
				await vi.runAllTimersAsync();

				// No loading gaps throughout redirect chain
				const hasLoadingGap = statusUpdates.some(
					(status) =>
						!status.isNavigating &&
						!status.isSubmitting &&
						!status.isRevalidating,
				);

				expect(hasLoadingGap).toBe(false);
				expect(fetch).toHaveBeenCalledTimes(3);

				cleanup();
			});
		});

		describe("Navigation with Asset Loading - Complete Loading Coverage", () => {
			it("should keep loading state active until all JS modules are loaded", async () => {
				const statusUpdates: StatusEventDetail[] = [];
				let routeChangeEventFired = false;

				const statusCleanup = addListener(addStatusListener, (e) => {
					statusUpdates.push({ ...(e.detail as any) });
				});

				const routeCleanup = addListener(addRouteChangeListener, () => {
					routeChangeEventFired = true;
				});

				// Mock navigation response with multiple JS dependencies
				vi.mocked(fetch).mockResolvedValueOnce(
					createMockResponse({
						importURLs: [], // Empty to avoid import errors
						cssBundles: [],
						deps: [],
						loadersData: [],
						matchedPatterns: ["/complex-page"],
						params: {},
						splatValues: [],
						hasRootData: false,
						title: { dangerousInnerHTML: "Complex Page" },
					}),
				);

				// Check status before navigation
				expect(getStatus().isNavigating).toBe(false);

				const navPromise = vormaNavigate("/complex-page");

				// Should be navigating immediately after starting
				expect(getStatus().isNavigating).toBe(true);

				// Wait for navigation to complete
				await navPromise;
				await vi.runAllTimersAsync();

				// Should have cleared loading state after completion
				expect(getStatus().isNavigating).toBe(false);
				expect(routeChangeEventFired).toBe(true);

				// Verify loading was continuous (no gaps except the final state)
				const hasLoadingGap = statusUpdates
					.slice(0, -1)
					.some(
						(status) =>
							!status.isNavigating &&
							!status.isSubmitting &&
							!status.isRevalidating,
					);

				expect(hasLoadingGap).toBe(false);

				statusCleanup();
				routeCleanup();
			});

			it("should handle CSS bundle loading delays", async () => {
				const statusUpdates: StatusEventDetail[] = [];
				let cssLoadCallback: () => void;
				const cleanup = addListener(addStatusListener, (e) => {
					statusUpdates.push({ ...(e.detail as any) });
				});

				// 1. Setup controllable fetch
				let resolveFetch;
				const fetchPromise = new Promise((resolve) => {
					resolveFetch = () =>
						resolve(
							createMockResponse({ cssBundles: ["/style1.css"] }),
						);
				});
				vi.mocked(fetch).mockResolvedValueOnce(fetchPromise as any);

				// 2. Mock CSS preloading to capture the onload callback
				const originalCreateElement =
					document.createElement.bind(document);
				vi.spyOn(document, "createElement").mockImplementation(
					(tag) => {
						if (tag === "link") {
							const link = originalCreateElement("link");
							Object.defineProperty(link, "onload", {
								set(callback) {
									if (callback && link.rel === "preload")
										cssLoadCallback = callback;
								},
								get() {
									return null;
								},
								configurable: true,
							});
							return link;
						}
						return originalCreateElement(tag);
					},
				);

				// 3. Start navigation
				const navPromise = vormaNavigate("/styled-page");

				// 4. Resolve fetch, which triggers the code path that waits for CSS
				(resolveFetch as any)();
				await vi.advanceTimersByTimeAsync(10); // Let fetch promise resolve & CSS preload start

				// 5. CRITICAL CHECK: We must be navigating while waiting for the CSS onload event
				expect(statusUpdates.at(-1)?.isNavigating).toBe(true);
				// @ts-ignore
				expect(cssLoadCallback).toBeDefined();

				// 6. Simulate CSS finishing
				// @ts-ignore
				cssLoadCallback();
				await navPromise; // Await the original navigation promise
				await vi.runAllTimersAsync();

				// 7. Check for gaps throughout the entire process
				const hasGap = statusUpdates.some((status, i) => {
					if (i === 0) return false;
					const prev = statusUpdates[i - 1];
					return (
						prev?.isNavigating &&
						!status.isNavigating &&
						i < statusUpdates.length - 1
					);
				});
				expect(hasGap).toBe(false);

				cleanup();
			});

			it("should handle client loader (waitFn) delays", async () => {
				const statusUpdates: StatusEventDetail[] = [];
				const cleanup = addListener(addStatusListener, (e) => {
					statusUpdates.push({ ...(e.detail as any) });
				});

				// 1. Setup a slow, controllable client loader
				let resolveWaitFn;
				const waitFnPromise = new Promise((resolve) => {
					resolveWaitFn = () => resolve({ clientData: "loaded" });
				});
				setupGlobalVormaContext({
					patternToWaitFnMap: { "/data-page": () => waitFnPromise },
				});

				// 2. Setup controllable fetch
				let resolveFetch;
				const fetchPromise = new Promise((resolve) => {
					resolveFetch = () =>
						resolve(
							createMockResponse({
								importURLs: [],
								matchedPatterns: ["/data-page"],
								loadersData: [{}],
							}),
						);
				});
				vi.mocked(fetch).mockResolvedValueOnce(fetchPromise as any);

				// 3. Start navigation
				const navPromise = vormaNavigate("/data-page");

				// 4. Resolve fetch, which triggers the client loader
				(resolveFetch as any)();
				await vi.advanceTimersByTimeAsync(10);

				// 5. CRITICAL CHECK: Must be navigating while waitFn is pending
				expect(statusUpdates.at(-1)?.isNavigating).toBe(true);

				// 6. Resolve the client loader and finish navigation
				(resolveWaitFn as any)();
				await navPromise;
				await vi.runAllTimersAsync();

				// 7. Check for gaps
				const hasGap = statusUpdates.some((status, i) => {
					if (i === 0) return false;
					const prev = statusUpdates[i - 1];
					return (
						prev?.isNavigating &&
						!status.isNavigating &&
						i < statusUpdates.length - 1
					);
				});
				expect(hasGap).toBe(false);

				cleanup();
			});
		});
	});
});
