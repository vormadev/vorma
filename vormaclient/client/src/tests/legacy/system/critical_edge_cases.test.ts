// READY_TO_DELETE_AFTER_SIGNOFF
import { describe, expect, it, vi } from "vitest";

import {
	beginNavigation,
	getStatus,
	navigationStateManager,
	revalidate,
	vormaNavigate,
} from "../../../client";

import {
	addBuildIDListener,
	addLocationListener,
	addRouteChangeListener,
	addStatusListener,
} from "../../../platform/events.ts";

import { __getPrefetchHandlers } from "../../../core/links.ts";

import { __vormaClientGlobal } from "../../../app/context.ts";

import {
	createMockResponse,
	describeNavigationTestSuite,
} from "../client.test.helpers.ts";

describeNavigationTestSuite(() => {
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
});
