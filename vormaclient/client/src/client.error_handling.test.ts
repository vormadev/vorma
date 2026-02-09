import {
	describe,
	expect,
	it,
	vi,
} from "vitest";

import {
	getStatus,
	vormaNavigate,
} from "./client";

import {
	addStatusListener,
} from "./events.ts";

import {
	__vormaClientGlobal,
} from "./vorma_ctx/vorma_ctx.ts";

import {
	createMockResponse,
	describeNavigationTestSuite,
} from "./client.test.helpers.ts";

describeNavigationTestSuite(({ addListener }) => {
	describe("12. Error Handling", () => {
		describe("12.1 Abort Errors", () => {
			it("should identify abort errors correctly", async () => {
				const abortError = new Error("The operation was aborted");
				abortError.name = "AbortError";

				vi.mocked(fetch).mockRejectedValue(abortError);

				const statusListener = vi.fn();
				const cleanup = addListener(addStatusListener, statusListener);

				await vormaNavigate("/will-abort");
				await vi.runAllTimersAsync();

				// Should not log abort errors
				expect(console.error).not.toHaveBeenCalledWith(
					expect.stringContaining("abort"),
				);

				cleanup();
			});

			it("should clear loading states on abort", async () => {
				const abortError = new Error("Aborted");
				abortError.name = "AbortError";
				vi.mocked(fetch).mockRejectedValue(abortError);

				// Check initial state
				expect(getStatus().isNavigating).toBe(false);

				const navPromise = vormaNavigate("/abort-loading");

				// Should be navigating immediately
				expect(getStatus().isNavigating).toBe(true);

				await navPromise;
				await vi.runAllTimersAsync();

				// Should be done navigating after abort
				expect(getStatus().isNavigating).toBe(false);
			});
		});

		describe("12.2 Navigation Failures", () => {
			it("should log non-abort errors", async () => {
				const error = new Error("Network failure");
				vi.mocked(fetch).mockRejectedValue(error);

				const consoleErrorSpy = vi
					.spyOn(console, "error")
					.mockImplementation(() => {});

				await vormaNavigate("/fail");
				await vi.runAllTimersAsync();

				expect(consoleErrorSpy).toHaveBeenCalledWith(
					"Vorma:",
					"Navigation failed",
					error,
				);

				consoleErrorSpy.mockRestore();
			});

			it("should clear loading state on failure", async () => {
				vi.mocked(fetch).mockRejectedValue(new Error("Failed"));

				// Check initial state
				expect(getStatus().isNavigating).toBe(false);

				const navPromise = vormaNavigate("/clear-on-fail");

				// Should be navigating immediately
				expect(getStatus().isNavigating).toBe(true);

				await navPromise;
				await vi.runAllTimersAsync();

				// Should be done navigating after failure
				expect(getStatus().isNavigating).toBe(false);
			});

			it("should keep user on current page after failure", async () => {
				const currentPath = window.location.pathname;
				vi.mocked(fetch).mockRejectedValue(
					new Error("Navigation error"),
				);

				await vormaNavigate("/unreachable");
				vi.runAllTimers();

				expect(window.location.pathname).toBe(currentPath);
			});

			it("should not update any state on failure", async () => {
				const initialState = {
					title: document.title,
					components: __vormaClientGlobal.get("activeComponents"),
					params: __vormaClientGlobal.get("params"),
				};

				vi.mocked(fetch).mockRejectedValue(
					new Error("State test error"),
				);

				await vormaNavigate("/state-fail");
				vi.runAllTimers();

				expect(document.title).toBe(initialState.title);
				expect(__vormaClientGlobal.get("activeComponents")).toBe(
					initialState.components,
				);
				expect(__vormaClientGlobal.get("params")).toBe(
					initialState.params,
				);
			});
		});

		describe("12.3 Special Cases", () => {
			it("should treat empty JSON response as failure", async () => {
				vi.mocked(fetch).mockResolvedValue(
					new Response("", { status: 200 }),
				);

				// Check initial state
				expect(getStatus().isNavigating).toBe(false);

				const navPromise = vormaNavigate("/empty");

				// Should be navigating immediately
				expect(getStatus().isNavigating).toBe(true);

				await navPromise;
				await vi.runAllTimersAsync();

				// Should be done navigating after empty response
				expect(getStatus().isNavigating).toBe(false);
			});

			it("should handle network errors", async () => {
				vi.mocked(fetch).mockRejectedValue(
					new TypeError("Failed to fetch"),
				);

				const consoleErrorSpy = vi
					.spyOn(console, "error")
					.mockImplementation(() => {});

				await vormaNavigate("/network-error");
				await vi.runAllTimersAsync();

				expect(consoleErrorSpy).toHaveBeenCalled();
				consoleErrorSpy.mockRestore();
			});

			it("should handle 404/500 responses", async () => {
				const originalTitle = document.title;
				const originalPathname = window.location.pathname;

				vi.mocked(fetch).mockResolvedValue(
					new Response("Not Found", { status: 404 }),
				);

				// Navigate to a 404 page
				await vormaNavigate("/not-found");

				// The important behaviors:
				// 1. We should still be on the original page
				expect(window.location.pathname).toBe(originalPathname);

				// 2. The title shouldn't have changed
				expect(document.title).toBe(originalTitle);

				// 3. Eventually we should not be in a loading state
				// (we can check this by attempting another navigation)
				vi.mocked(fetch).mockResolvedValue(
					createMockResponse({
						title: { dangerousInnerHTML: "Success Page" },
						importURLs: [],
						cssBundles: [],
					}),
				);

				await vormaNavigate("/success");

				// This navigation should work, proving we're not stuck
				expect(document.title).toBe("Success Page");
			});
		});
	});

});
