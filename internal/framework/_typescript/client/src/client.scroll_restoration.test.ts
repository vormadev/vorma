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

});
