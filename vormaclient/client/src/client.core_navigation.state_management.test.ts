// READY_TO_DELETE_AFTER_SIGNOFF
import { describe, expect, it, vi } from "vitest";

import {
	beginNavigation,
	getStatus,
	navigationStateManager,
	vormaNavigate,
} from "./client";

import { addStatusListener } from "./events.ts";

import { __getPrefetchHandlers } from "./links.ts";

import {
	createMockResponse,
	describeNavigationTestSuite,
} from "./client.test.helpers.ts";

describeNavigationTestSuite(({ addListener }) => {
	describe("1. Core Navigation", () => {
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
	});
});
