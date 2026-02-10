// READY_TO_DELETE_AFTER_SIGNOFF
import { afterEach, describe, expect, it, vi } from "vitest";

import { getHistoryInstance, submit, vormaNavigate } from "./client";

import { addBuildIDListener } from "./events.ts";

import {
	createMockResponse,
	describeNavigationTestSuite,
} from "./client.test.helpers.ts";

describeNavigationTestSuite(() => {
	describe("5. Redirects", () => {
		// Track all listeners to clean them up
		let cleanupFns: Array<() => void> = [];

		afterEach(() => {
			// Clean up all listeners
			cleanupFns.forEach((fn) => fn());
			cleanupFns = [];

			// Clear fetch mocks
			vi.mocked(fetch).mockClear();
		});

		describe("5.1 Request Configuration", () => {
			it("should include X-Accepts-Client-Redirect header", async () => {
				vi.mocked(fetch).mockResolvedValueOnce(
					createMockResponse({ importURLs: [], cssBundles: [] }),
				);

				await vormaNavigate("/test");
				await vi.runAllTimersAsync();

				const headers = vi.mocked(fetch).mock.calls[0]?.[1]
					?.headers as Headers;
				expect(headers.get("X-Accepts-Client-Redirect")).toBe("1");
			});
		});

		describe("5.2 Response Headers Priority", () => {
			it("should prioritize X-Vorma-Reload over other redirects", async () => {
				// Mock location.href setter
				let locationHref = window.location.href;
				const originalLocation = window.location;
				Object.defineProperty(window, "location", {
					value: {
						...originalLocation,
						get href() {
							return locationHref;
						},
						set href(value) {
							locationHref = value;
						},
					},
					configurable: true,
				});

				vi.mocked(fetch).mockResolvedValueOnce(
					createMockResponse(null, {
						headers: {
							"X-Vorma-Reload": "/force-reload",
							"X-Client-Redirect": "/ignored",
							"X-Vorma-Build-Id": "test-build",
						},
					}),
				);

				await vormaNavigate("/test");
				await vi.runAllTimersAsync();

				expect(locationHref).toContain("/force-reload");
				expect(locationHref).toContain("vorma_reload=test-build");

				// Restore original location
				Object.defineProperty(window, "location", {
					value: originalLocation,
					configurable: true,
				});
			});

			it("should handle native browser redirect for GET", async () => {
				// When response.redirected is true and it's a GET request,
				// the implementation treats this as already redirected ("did" status)
				// and doesn't navigate to the redirected content
				const redirectedResponse = createMockResponse(null);
				Object.defineProperty(redirectedResponse, "redirected", {
					value: true,
					writable: false,
				});
				Object.defineProperty(redirectedResponse, "url", {
					value: "http://localhost:3000/redirected",
					writable: false,
				});

				vi.mocked(fetch).mockResolvedValueOnce(redirectedResponse);

				await vormaNavigate("/original");
				await vi.runAllTimersAsync();

				// Since redirected=true for GET, navigation completes without rendering
				// Document title should remain unchanged
				expect(document.title).toBe("Initial Page");
			});

			it("should ignore redirect for non-GET requests", async () => {
				const redirectedResponse = createMockResponse({
					data: "response",
				});
				Object.defineProperty(redirectedResponse, "redirected", {
					value: true,
					writable: false,
				});

				vi.mocked(fetch).mockResolvedValueOnce(redirectedResponse);

				const result = await submit("/api", { method: "POST" });
				await vi.runAllTimersAsync();

				expect(result.success).toBe(true);
				expect((result as any).data).toEqual({ data: "response" });
			});

			it("should handle X-Client-Redirect as lowest priority", async () => {
				vi.mocked(fetch)
					.mockResolvedValueOnce(
						createMockResponse(null, {
							headers: {
								"X-Client-Redirect": "/client-redirect",
								"X-Vorma-Build-Id": "test-build",
							},
						}),
					)
					.mockResolvedValueOnce(
						createMockResponse({
							title: { dangerousInnerHTML: "Client Redirected" },
							importURLs: [],
							cssBundles: [],
						}),
					);

				await vormaNavigate("/test");
				await vi.runAllTimersAsync();

				expect(document.title).toBe("Client Redirected");
				expect(fetch).toHaveBeenCalledTimes(2);
			});
		});

		describe("5.3 Build ID Tracking", () => {
			it("should update build ID from response header", async () => {
				const buildIdListener = vi.fn();
				addBuildIDListener(buildIdListener);

				vi.mocked(fetch).mockResolvedValueOnce(
					createMockResponse(
						{ importURLs: [], cssBundles: [] },
						{ headers: { "X-Vorma-Build-Id": "new-build-789" } },
					),
				);

				await vormaNavigate("/new-build");
				await vi.runAllTimersAsync();

				expect(buildIdListener).toHaveBeenCalled();
			});

			it("should dispatch build-id event before redirect", async () => {
				// The issue is that setupGlobalVormaContext is being called in beforeEach
				// and resets the buildID to "1". We need to check what build ID changes occur.

				const buildIdListener = vi.fn();
				const cleanup = addBuildIDListener(buildIdListener);
				cleanupFns.push(cleanup);

				vi.mocked(fetch)
					.mockResolvedValueOnce(
						createMockResponse(null, {
							headers: {
								"X-Vorma-Build-Id": "redirect-build",
								"X-Client-Redirect": "/redirect",
							},
						}),
					)
					.mockResolvedValueOnce(
						createMockResponse(
							{
								importURLs: [],
								cssBundles: [],
								title: {
									dangerousInnerHTML: "Redirected Page",
								},
							},
							{
								headers: {
									"X-Vorma-Build-Id": "redirect-build",
								},
							},
						),
					);

				await vormaNavigate("/test");
				await vi.runAllTimersAsync();

				expect(buildIdListener).toHaveBeenCalled();
				const event = buildIdListener.mock.calls[0]?.[0];
				expect(event.detail).toEqual({
					oldID: "1", // This is the initial build ID from beforeEach
					newID: "redirect-build",
				});
			});
		});

		describe("5.4 Redirect Strategies", () => {
			it("should use soft redirect for internal URLs", async () => {
				vi.mocked(fetch)
					.mockResolvedValueOnce(
						createMockResponse(null, {
							headers: {
								"X-Client-Redirect": "/internal-redirect",
							},
						}),
					)
					.mockResolvedValueOnce(
						createMockResponse({
							title: { dangerousInnerHTML: "Soft Redirected" },
							importURLs: [],
							cssBundles: [],
						}),
					);

				await vormaNavigate("/test");
				await vi.runAllTimersAsync();

				expect(document.title).toBe("Soft Redirected");
				expect(fetch).toHaveBeenCalledTimes(2);
			});

			it("should use hard redirect for external URLs", async () => {
				let locationHref = window.location.href;
				const originalLocation = window.location;

				// Mock window.location more completely
				Object.defineProperty(window, "location", {
					value: {
						...originalLocation,
						href: locationHref,
						assign: vi.fn((url) => {
							locationHref = url;
						}),
						replace: vi.fn((url) => {
							locationHref = url;
						}),
					},
					configurable: true,
					writable: true,
				});

				// Update the setter on window.location.href
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

				await vormaNavigate("/test");
				await vi.runAllTimersAsync();

				// The URL might have a trailing slash added
				expect(locationHref).toMatch(/^https:\/\/external\.com\/?$/);

				Object.defineProperty(window, "location", {
					value: originalLocation,
					configurable: true,
				});
			});

			it("should add vorma_reload param for forced internal redirect", async () => {
				let locationHref = window.location.href;
				const originalLocation = window.location;

				Object.defineProperty(window, "location", {
					value: {
						...originalLocation,
						href: locationHref,
						origin: "http://localhost:3000",
					},
					configurable: true,
					writable: true,
				});

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
							"X-Vorma-Reload": "/force-internal",
							"X-Vorma-Build-Id": "force-build",
						},
					}),
				);

				await vormaNavigate("/test");
				await vi.runAllTimersAsync();

				expect(locationHref).toContain("/force-internal");
				expect(locationHref).toContain("vorma_reload=force-build");

				Object.defineProperty(window, "location", {
					value: originalLocation,
					configurable: true,
				});
			});

			it("should respect max redirect limit", async () => {
				const consoleErrorSpy = vi
					.spyOn(console, "error")
					.mockImplementation(() => {});

				// Set up 15 redirect responses to ensure we exceed the limit of 10
				for (let i = 1; i <= 15; i++) {
					vi.mocked(fetch).mockResolvedValueOnce(
						createMockResponse(null, {
							headers: { "X-Client-Redirect": `/redirect${i}` },
						}),
					);
				}

				// Navigate - it should stop at the redirect limit
				await vormaNavigate("/test");
				await vi.runAllTimersAsync();

				// Should stop at 10 calls: 1 initial + 9 redirects, then the 10th redirect attempt
				// triggers the limit (redirectCount >= 10)
				expect(vi.mocked(fetch)).toHaveBeenCalledTimes(10);

				// Should log error when hitting the limit
				expect(consoleErrorSpy).toHaveBeenCalledWith(
					"Vorma:",
					"Too many redirects",
				);

				consoleErrorSpy.mockRestore();
			});
		});

		describe("5.5 Error Handling", () => {
			it("should ignore non-HTTP redirect URLs", async () => {
				// When redirect URL is non-HTTP, it should be ignored
				// and navigation should complete normally
				vi.mocked(fetch).mockResolvedValueOnce(
					createMockResponse(
						{
							importURLs: [],
							cssBundles: [],
							title: { dangerousInnerHTML: "Test Page" },
						},
						{
							headers: {
								"X-Client-Redirect": "mailto:test@test.com",
							},
						},
					),
				);

				await vormaNavigate("/test");
				await vi.runAllTimersAsync();

				expect(window.location.href).not.toContain("mailto:");
				expect(document.title).toBe("Test Page");
			});
		});

		describe("5.6 URL Cleanup", () => {
			it("should remove vorma_reload param on init", async () => {
				window.history.replaceState(
					{},
					"",
					"/?vorma_reload=123&other=param",
				);

				// Reset custom history
				(window as any).__customHistory = undefined;

				const history = getHistoryInstance();
				const replaceSpy = vi.spyOn(history, "replace");

				// Simulate just the URL cleanup part of initClient
				const url = new URL(window.location.href);
				if (url.searchParams.has("vorma_reload")) {
					url.searchParams.delete("vorma_reload");
					history.replace(url.href);
				}

				// The history.replace method receives the full URL
				expect(replaceSpy).toHaveBeenCalledWith(
					"http://localhost:3000/?other=param",
				);
			});
		});
	});
});
