import {
	describe,
	expect,
	it,
	vi,
} from "vitest";

import {
	getStatus,
	navigationStateManager,
	revalidate,
	submit,
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
		describe("6.1 Submit Function", () => {
			it("should deduplicate submissions with same dedupeKey", async () => {
				let firstRequestAborted = false;
				let firstRequestStarted = false;
				let secondRequestStarted = false;

				vi.mocked(fetch).mockImplementation((url, init) => {
					const isFirst = !firstRequestStarted;

					if (isFirst) {
						firstRequestStarted = true;
						return new Promise((resolve, reject) => {
							init?.signal?.addEventListener("abort", () => {
								firstRequestAborted = true;
								const error = new Error(
									"The operation was aborted",
								);
								error.name = "AbortError";
								reject(error);
							});
							// Never resolve - will be aborted
						});
					} else {
						secondRequestStarted = true;
						return Promise.resolve(
							new Response(JSON.stringify({ data: "success" }), {
								status: 200,
								headers: { "Content-Type": "application/json" },
							}),
						);
					}
				});

				// Start both submissions
				const promise1 = submit(
					"/api/resource",
					{ method: "POST" },
					{ dedupeKey: "myKey" },
				);
				const promise2 = submit(
					"/api/resource",
					{ method: "POST" },
					{ dedupeKey: "myKey" },
				);

				// Wait for both to complete
				const [result1, result2] = await Promise.all([
					promise1,
					promise2,
				]);

				// First should have been aborted
				expect(result1).toEqual({ success: false, error: "Aborted" });
				expect(firstRequestAborted).toBe(true);

				// Second should succeed
				expect(result2).toEqual({
					success: true,
					data: { data: "success" },
				});

				// Both requests should have been started
				expect(firstRequestStarted).toBe(true);
				expect(secondRequestStarted).toBe(true);
			});

			it("should NOT deduplicate submissions without dedupeKey", async () => {
				vi.mocked(fetch).mockImplementation(
					() => new Promise(() => {}), // Never resolve
				);

				submit("/api/resource", { method: "POST" });
				submit("/api/resource", { method: "POST" });

				// Without a dedupeKey, both submissions should be tracked individually
				// Check the internal state of the navigationStateManager
				const submissions = (navigationStateManager as any)
					._submissions;
				expect(submissions.size).toBe(2);
			});

			it("should NOT deduplicate submissions with different dedupeKeys", async () => {
				vi.mocked(fetch).mockImplementation(
					() => new Promise(() => {}), // Never resolve
				);

				submit(
					"/api/resource",
					{ method: "POST" },
					{ dedupeKey: "key1" },
				);
				submit(
					"/api/resource",
					{ method: "POST" },
					{ dedupeKey: "key2" },
				);

				// Different keys should not deduplicate
				const submissions = (navigationStateManager as any)
					._submissions;
				expect(submissions.has("submission:key1")).toBe(true);
				expect(submissions.has("submission:key2")).toBe(true);
				expect(submissions.size).toBe(2);
			});

			it("should set isSubmitting loading state", async () => {
				// Wait for any pending status events to clear
				await vi.runAllTimersAsync();

				const statusListener = vi.fn();
				const cleanup = addListener(addStatusListener, statusListener);

				// Create a delayed response to ensure submission takes long enough
				vi.mocked(fetch).mockImplementation(
					() =>
						new Promise((resolve) => {
							setTimeout(() => {
								resolve({
									ok: true,
									status: 200,
									json: () =>
										Promise.resolve({ result: "success" }),
									headers: new Headers(),
								} as any);
							}, 20); // Delay longer than the debounce time
						}),
				);

				const submitPromise = submit("/api/data", { method: "POST" });

				// Wait for the first debounced status event (5ms)
				await vi.advanceTimersByTimeAsync(10);

				// Should have isSubmitting: true
				const submittingEvent = statusListener.mock.calls.find(
					(call) => call[0].detail.isSubmitting === true,
				);
				expect(submittingEvent).toBeDefined();

				// Complete the submission
				await vi.advanceTimersByTimeAsync(20);
				await submitPromise;
				await vi.runAllTimersAsync();

				// Final state should have isSubmitting: false
				const lastCall =
					statusListener.mock.calls[
						statusListener.mock.calls.length - 1
					];
				expect(lastCall?.[0].detail.isSubmitting).toBe(false);

				cleanup();
			});

			it("should clear isSubmitting state when an in-flight submission is aborted", async () => {
				const statusUpdates: StatusEventDetail[] = [];
				const cleanup = addListener(addStatusListener, (e) => {
					statusUpdates.push({ ...(e.detail as any) });
				});

				// 1. The first fetch call will hang, keeping the submission in-flight.
				vi.mocked(fetch).mockImplementationOnce(
					() => new Promise(() => {}),
				);

				// 2. The second fetch call will fail immediately. This is key to preventing
				//    the second submission from persisting.
				vi.mocked(fetch).mockImplementationOnce(() =>
					Promise.reject(new Error("Simulated network failure")),
				);

				// 3. Start the first submission.
				submit(
					"/api/resource",
					{ method: "POST" },
					{ dedupeKey: "myKey" },
				);

				// 4. Wait for the status update and confirm we are submitting.
				await vi.advanceTimersByTimeAsync(10);
				expect(statusUpdates.at(-1)?.isSubmitting).toBe(true);

				// 5. Call submit again with the same key. This will:
				//    a) Abort the first submission.
				//    b) Start a second submission which will immediately fail and clean itself up.
				await submit(
					"/api/resource",
					{ method: "POST" },
					{ dedupeKey: "myKey" },
				);

				// 6. Wait for the final debounced status update to fire after all the cleanup.
				await vi.advanceTimersByTimeAsync(10);

				// 7. Assert that the state is now clean. `isSubmitting` should be false
				//    because the aborted submission was removed and the replacement failed.
				expect(getStatus().isSubmitting).toBe(false);
				expect(statusUpdates.at(-1)?.isSubmitting).toBe(false);

				cleanup();
			});

			it("should maintain isSubmitting state during a successful deduplicated submission", async () => {
				const statusListener = vi.fn();
				const cleanup = addListener(addStatusListener, statusListener);

				// Mock fetch to never resolve, so all submissions remain in-flight.
				vi.mocked(fetch).mockImplementation(
					() => new Promise(() => {}),
				);

				// 1. Start the first submission.
				submit(
					"/api/resource",
					{ method: "POST" },
					{ dedupeKey: "test" },
				);

				// 2. Wait for the status update.
				await vi.advanceTimersByTimeAsync(10);

				// Assert that the listener was called.
				expect(statusListener).toHaveBeenCalled();
				// Get the 'detail' object from the most recent call to the listener.
				const lastStatusDetail =
					statusListener.mock.lastCall?.[0].detail;
				// Assert that the detail object has the correct properties.
				expect(lastStatusDetail.isSubmitting).toBe(true);

				// 3. Start a second, duplicate submission. This aborts the first and replaces it.
				submit(
					"/api/resource",
					{ method: "POST" },
					{ dedupeKey: "test" },
				);

				// 4. Wait for any potential state changes.
				await vi.advanceTimersByTimeAsync(10);

				// 5. Assert that the `isSubmitting` state is still true, because the
				//    second submission has seamlessly taken the place of the first.
				expect(getStatus().isSubmitting).toBe(true);

				// 6. Assert that only one submission is still being tracked.
				const submissions = (navigationStateManager as any)
					._submissions;
				expect(submissions.has("submission:test")).toBe(true);
				expect(submissions.size).toBe(1);

				cleanup();
			});

			it("should send FormData as-is", async () => {
				vi.mocked(fetch).mockResolvedValue(createMockResponse({}));

				const formData = new FormData();
				formData.append("field", "value");

				await submit("/api/form", {
					method: "POST",
					body: formData,
				});

				expect(fetch).toHaveBeenCalledWith(
					expect.any(URL),
					expect.objectContaining({
						body: formData,
					}),
				);
			});

			it("should send string body as-is", async () => {
				vi.mocked(fetch).mockResolvedValue(createMockResponse({}));

				const stringBody = "raw string data";

				await submit("/api/string", {
					method: "POST",
					body: stringBody,
				});

				expect(fetch).toHaveBeenCalledWith(
					expect.any(URL),
					expect.objectContaining({
						body: stringBody,
					}),
				);
			});

			it("should JSON stringify other body types", async () => {
				vi.mocked(fetch).mockResolvedValue(createMockResponse({}));

				const objectBody = { key: "value", nested: { data: true } };

				await submit("/api/json", {
					method: "POST",
					body: objectBody as any,
				});

				expect(fetch).toHaveBeenCalledWith(
					expect.any(URL),
					expect.objectContaining({
						body: JSON.stringify(objectBody),
					}),
				);
			});

			it("should handle redirect responses", async () => {
				// Track the current location
				const initialLocation = window.location.href;
				expect(initialLocation).toBe("http://localhost:3000/");
				const initialTitle = document.title;

				vi.mocked(fetch)
					.mockResolvedValueOnce(
						createMockResponse(null, {
							headers: { "X-Client-Redirect": "/after-submit" },
						}),
					)
					.mockResolvedValueOnce(
						createMockResponse({
							title: { dangerousInnerHTML: "After Submit" },
							importURLs: [],
							cssBundles: [],
						}),
					);

				const result = await submit("/api/action", { method: "POST" });

				// Wait for any async navigation to complete
				await vi.runAllTimersAsync();

				// Verify the redirect was followed
				expect(result.success).toBe(true);

				// CRITICAL: Verify fetch was called twice (submit + redirect navigation)
				expect(fetch).toHaveBeenCalledTimes(2);

				// Verify the second fetch was for the redirect target
				expect(fetch).toHaveBeenNthCalledWith(
					2,
					expect.objectContaining({
						href: expect.stringContaining("/after-submit"),
					}),
					expect.any(Object),
				);

				// Verify the page actually changed
				expect(window.location.pathname).toBe("/after-submit");
				expect(document.title).toBe("After Submit");
				expect(document.title).not.toBe(initialTitle);
			});

			it("should handle X-Vorma-Reload redirects from submit", async () => {
				let locationHref = window.location.href;
				Object.defineProperty(window.location, "href", {
					get: () => locationHref,
					set: (value) => {
						locationHref = value;
					},
					configurable: true,
				});

				vi.mocked(fetch).mockResolvedValueOnce(
					createMockResponse(null, {
						headers: {
							"X-Vorma-Reload": "/force-reload",
							"X-Vorma-Build-Id": "new-build",
						},
					}),
				);

				await submit("/api/action", { method: "POST" });
				await vi.runAllTimersAsync();

				// Should do a hard redirect with vorma_reload param
				expect(locationHref).toContain("/force-reload");
				expect(locationHref).toContain("vorma_reload=new-build");
			});

			it("should handle external redirects from submit", async () => {
				let locationHref = window.location.href;
				Object.defineProperty(window.location, "href", {
					get: () => locationHref,
					set: (value) => {
						locationHref = value;
					},
					configurable: true,
				});

				vi.mocked(fetch).mockResolvedValueOnce(
					createMockResponse(null, {
						headers: {
							"X-Client-Redirect": "https://external.com",
						},
					}),
				);

				await submit("/api/action", { method: "POST" });
				await vi.runAllTimersAsync();

				// Should do a hard redirect to external URL
				expect(locationHref).toBe("https://external.com/");
			});

			it("should auto-revalidate after non-GET submission", async () => {
				vi.mocked(fetch)
					.mockResolvedValueOnce(
						createMockResponse({ submitted: true }),
					)
					.mockResolvedValueOnce(
						createMockResponse({
							title: { dangerousInnerHTML: "Revalidated" },
							importURLs: [],
							cssBundles: [],
						}),
					);

				await submit("/api/mutate", { method: "POST" });
				vi.runAllTimers();

				expect(fetch).toHaveBeenCalledTimes(2);
				expect(document.title).toBe("Revalidated");
			});

			it("should not auto-revalidate after GET submission", async () => {
				vi.mocked(fetch).mockResolvedValueOnce(
					createMockResponse({ data: "search results" }),
				);

				await submit("/api/search", { method: "GET" });
				vi.runAllTimers();

				expect(fetch).toHaveBeenCalledTimes(1);
			});

			it("should manage loading state transition to revalidation", async () => {
				const statusListener = vi.fn();
				const cleanup = addListener(addStatusListener, statusListener);

				vi.mocked(fetch)
					.mockResolvedValueOnce(createMockResponse({}))
					.mockResolvedValueOnce(
						createMockResponse({
							importURLs: [],
							cssBundles: [],
							loadersData: [],
							matchedPatterns: [],
							params: {},
							splatValues: [],
						}),
					);

				await submit("/api/update", { method: "PUT" });
				await vi.runAllTimersAsync();

				cleanup();
			});

			it("should return success with data", async () => {
				const responseData = { id: 123, status: "created" };

				// Mock the implementation to avoid body read issues
				vi.mocked(fetch).mockImplementation(() => {
					return Promise.resolve({
						ok: true,
						status: 200,
						json: () => Promise.resolve(responseData),
						headers: new Headers({
							"Content-Type": "application/json",
						}),
					} as any);
				});

				const result = await submit("/api/create", { method: "POST" });

				expect(result).toEqual({
					success: true,
					data: responseData,
				});
			});

			it("should return error on failure", async () => {
				vi.mocked(fetch).mockResolvedValue(
					new Response(null, {
						status: 500,
						statusText: "Internal Server Error",
					}),
				);

				const result = await submit("/api/fail", { method: "POST" });

				expect(result).toEqual({
					success: false,
					error: "500",
				});
			});

			it("should handle network errors", async () => {
				vi.mocked(fetch).mockRejectedValue(
					new Error("Network failure"),
				);

				const result = await submit("/api/network-error", {
					method: "POST",
				});

				expect(result.success).toBe(false);
				// The implementation logs the error and returns "unknown" for non-Error objects
				expect((result as any).error).toBeDefined();
			});

			it("should handle abort errors silently", async () => {
				const abortError = new Error("The operation was aborted");
				abortError.name = "AbortError";
				vi.mocked(fetch).mockRejectedValue(abortError);

				const result = await submit("/api/abort", { method: "POST" });

				expect(result.success).toBe(false);
				expect((result as any).error).toBe("Aborted");
			});
		});

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
