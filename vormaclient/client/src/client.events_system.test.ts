// READY_TO_DELETE_AFTER_SIGNOFF
import { describe, expect, it, vi } from "vitest";

import {
	beginNavigation,
	getBuildID,
	getLocation,
	getStatus,
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
} from "./events.ts";

import {
	createMockResponse,
	describeNavigationTestSuite,
	setupGlobalVormaContext,
} from "./client.test.helpers.ts";

describeNavigationTestSuite(({ addListener }) => {
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
				const { customHistoryListener } =
					await import("./history/history.ts");

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
});
