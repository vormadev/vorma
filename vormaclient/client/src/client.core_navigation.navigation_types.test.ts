// READY_TO_DELETE_AFTER_SIGNOFF
import { describe, expect, it, vi } from "vitest";

import {
	beginNavigation,
	getHistoryInstance,
	revalidate,
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
	});
});
