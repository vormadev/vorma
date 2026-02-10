// READY_TO_DELETE_AFTER_SIGNOFF
import { describe, expect, it, vi } from "vitest";

import { beginNavigation, getStatus, vormaNavigate } from "./client";

import {
	createMockResponse,
	describeNavigationTestSuite,
	setupGlobalVormaContext,
} from "./client.test.helpers.ts";

describeNavigationTestSuite(() => {
	describe("2. Navigation Lifecycle", () => {
		describe("2.2 Fetch Route Data Phase", () => {
			it("should construct URL with vorma_json and buildID", async () => {
				setupGlobalVormaContext({ buildID: "test-build-123" });
				vi.mocked(fetch).mockResolvedValue(
					createMockResponse({ importURLs: [], cssBundles: [] }),
				);

				await vormaNavigate("/test-url");
				await vi.runAllTimersAsync();

				expect(fetch).toHaveBeenCalledWith(
					expect.objectContaining({
						href: "http://localhost:3000/test-url?vorma_json=test-build-123",
					}),
					expect.any(Object),
				);
			});

			it("should include X-Accepts-Client-Redirect header", async () => {
				vi.mocked(fetch).mockResolvedValue(
					createMockResponse({ importURLs: [], cssBundles: [] }),
				);

				await vormaNavigate("/test-headers");
				await vi.runAllTimersAsync();

				expect(fetch).toHaveBeenCalledWith(
					expect.any(URL),
					expect.objectContaining({
						headers: expect.any(Headers),
					}),
				);

				const headers = vi.mocked(fetch).mock.calls[0]?.[1]
					?.headers as Headers;
				expect(headers.get("X-Accepts-Client-Redirect")).toBe("1");
			});

			it("should handle redirect responses correctly", async () => {
				vi.mocked(fetch)
					.mockResolvedValueOnce(
						createMockResponse(null, {
							headers: { "X-Client-Redirect": "/new-location" },
						}),
					)
					.mockResolvedValueOnce(
						createMockResponse({
							title: { dangerousInnerHTML: "Redirected" },
							importURLs: [],
							cssBundles: [],
						}),
					);

				await vormaNavigate("/original");
				await vi.runAllTimersAsync();

				expect(fetch).toHaveBeenCalledTimes(2);
			});

			it("should handle empty JSON as failure", async () => {
				vi.mocked(fetch).mockResolvedValue(
					new Response("", { status: 200 }),
				);

				// Check initial state
				expect(getStatus().isNavigating).toBe(false);

				const navPromise = vormaNavigate("/empty-json");

				// Should be navigating immediately (synchronous check)
				expect(getStatus().isNavigating).toBe(true);

				// Let the navigation attempt to complete
				await navPromise;
				await vi.runAllTimersAsync();

				// Should be done navigating after empty response (navigation failed)
				expect(getStatus().isNavigating).toBe(false);

				// Title shouldn't have changed
				expect(document.title).toBe("Initial Page");

				// Verify cleanup by trying to navigate to same URL again
				vi.clearAllMocks();
				await vormaNavigate("/empty-json");

				// Should make a new fetch call (proves previous navigation was cleaned up)
				expect(fetch).toHaveBeenCalledTimes(1);
			});

			it("should preload modules in production mode", async () => {
				const originalEnv = import.meta.env.DEV;
				(import.meta.env as any).DEV = false;

				// Spy on appendChild to verify links are created
				const appendChildSpy = vi.spyOn(document.head, "appendChild");

				vi.mocked(fetch).mockResolvedValue(
					createMockResponse({
						importURLs: ["/module1.js", "/module2.js"],
						deps: ["/dep1.js", "/dep2.js", "/module1.js"],
						cssBundles: [],
					}),
				);

				const control = beginNavigation({
					href: "/with-deps",
					navigationType: "userNavigation",
				});

				await control.promise;

				// Verify appendChild was called with modulepreload links
				const modulepreloadCalls = appendChildSpy.mock.calls.filter(
					(call) => {
						const element = call[0] as HTMLElement;
						return (
							element.tagName === "LINK" &&
							element.getAttribute("rel") === "modulepreload"
						);
					},
				);

				// Should create modulepreload for unique deps
				expect(modulepreloadCalls.length).toBe(3);

				(import.meta.env as any).DEV = originalEnv;
			});

			it("should preload CSS bundles", async () => {
				const appendChildSpy = vi.spyOn(document.head, "appendChild");

				vi.mocked(fetch).mockResolvedValue(
					createMockResponse({
						importURLs: [],
						cssBundles: ["/styles1.css", "/styles2.css"],
					}),
				);

				const control = beginNavigation({
					href: "/with-css",
					navigationType: "userNavigation",
				});

				await control.promise;

				// Verify appendChild was called with CSS preload links
				const cssPreloadCalls = appendChildSpy.mock.calls.filter(
					(call) => {
						const element = call[0] as HTMLElement;
						return (
							element.tagName === "LINK" &&
							element.getAttribute("rel") === "preload" &&
							element.getAttribute("as") === "style"
						);
					},
				);

				expect(cssPreloadCalls.length).toBe(2);
			});

			it("should execute client wait functions", async () => {
				const waitFn = vi
					.fn()
					.mockResolvedValue({ clientData: "test" });
				setupGlobalVormaContext({
					patternToWaitFnMap: {
						"/pattern": waitFn,
					},
				});

				vi.mocked(fetch).mockResolvedValue(
					createMockResponse({
						importURLs: [],
						cssBundles: [],
						matchedPatterns: ["/pattern"],
						loadersData: [{ serverData: "test" }],
						hasRootData: true,
					}),
				);

				await vormaNavigate("/pattern/test");
				await vi.runAllTimersAsync();

				expect(waitFn).toHaveBeenCalledWith(
					expect.objectContaining({
						params: expect.any(Object),
						splatValues: expect.any(Array),
						serverDataPromise: expect.any(Promise),
						signal: expect.any(AbortSignal),
					}),
				);

				// Verify the promise resolves to the correct data
				const call = waitFn.mock.calls[0]?.[0];
				const serverData = await call.serverDataPromise;
				expect(serverData).toEqual({
					matchedPatterns: ["/pattern"],
					loaderData: { serverData: "test" },
					rootData: { serverData: "test" },
					buildID: "1",
				});
			});

			it("should cleanup navigation on completion", async () => {
				vi.mocked(fetch).mockResolvedValue(
					createMockResponse({ importURLs: [], cssBundles: [] }),
				);

				// Ensure clean state first
				await vi.runAllTimersAsync();

				// Check initial state
				expect(getStatus().isNavigating).toBe(false);

				const navPromise = vormaNavigate("/cleanup");

				// Should be navigating immediately
				expect(getStatus().isNavigating).toBe(true);

				await navPromise;
				await vi.runAllTimersAsync();

				// Should be cleaned up after completion
				expect(getStatus().isNavigating).toBe(false);

				// Verify cleanup by trying to navigate to same URL again
				vi.clearAllMocks();
				await vormaNavigate("/cleanup");

				// Should make a new fetch call (proves previous navigation was cleaned up)
				expect(fetch).toHaveBeenCalledTimes(1);
			});
		});
	});
});
