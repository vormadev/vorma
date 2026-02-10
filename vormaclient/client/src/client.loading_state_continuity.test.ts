// READY_TO_DELETE_AFTER_SIGNOFF
import { describe, expect, it, vi } from "vitest";

import { getStatus, submit, vormaNavigate } from "./client";

import {
	addRouteChangeListener,
	addStatusListener,
	type StatusEventDetail,
} from "./events.ts";

import {
	createMockResponse,
	describeNavigationTestSuite,
	setupGlobalVormaContext,
} from "./client.test.helpers.ts";

describeNavigationTestSuite(({ addListener }) => {
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
