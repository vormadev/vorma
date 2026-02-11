// READY_TO_DELETE_AFTER_SIGNOFF
import { describe, expect, it, vi } from "vitest";

import { beginNavigation, getStatus } from "../../../client";

import { addStatusListener } from "../../../platform/events.ts";

import {
	createMockResponse,
	describeNavigationTestSuite,
} from "../client.test.helpers.ts";

describeNavigationTestSuite(({ addListener }) => {
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
	});
});
