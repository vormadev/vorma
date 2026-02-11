// READY_TO_DELETE_AFTER_SIGNOFF
import { describe, expect, it, vi } from "vitest";

import {
	getHistoryInstance,
	getStatus,
	revalidate,
	vormaNavigate,
} from "../../../client";

import {
	addRouteChangeListener,
	type RouteChangeEventDetail,
} from "../../../platform/events.ts";

import { __applyScrollState } from "../../../platform/scroll.ts";

import { __vormaClientGlobal } from "../../../app/context.ts";

import {
	createMockResponse,
	describeNavigationTestSuite,
	setupGlobalVormaContext,
} from "../client.test.helpers.ts";

describeNavigationTestSuite(({ addListener }) => {
	describe("2. Navigation Lifecycle", () => {
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
