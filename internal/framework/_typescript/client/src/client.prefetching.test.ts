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

});
