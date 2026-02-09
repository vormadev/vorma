import {
	describe,
	expect,
	it,
	vi,
} from "vitest";

import {
	getStatus,
	revalidate,
} from "./client";

import {
	addStatusListener,
	type StatusEventDetail,
} from "./events.ts";

import {
	createMockResponse,
	describeNavigationTestSuite,
} from "./client.test.helpers.ts";

describeNavigationTestSuite(({ addListener }) => {
	describe("6. Form Submissions", () => {
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
});
