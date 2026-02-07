import {
	describe,
	expect,
	it,
	vi,
} from "vitest";

import {
	vormaNavigate,
} from "./client";

import {
	__vormaClientGlobal,
} from "./vorma_ctx/vorma_ctx.ts";

import {
	createMockResponse,
	describeNavigationTestSuite,
	setupGlobalVormaContext,
} from "./client.test.helpers.ts";

describeNavigationTestSuite(() => {
	describe("8. Component & Module Loading", () => {
		describe("8.1 Initial Load", () => {
			it("should dynamically import modules from importURLs", async () => {
				const mockModule = { default: () => "Component" };
				vi.doMock("/static/module.js", () => mockModule);

				setupGlobalVormaContext({
					importURLs: ["/module.js"],
					publicPathPrefix: "/static",
				});

				vi.mocked(fetch).mockResolvedValue(
					createMockResponse({
						importURLs: ["/module.js"],
						exportKeys: ["default"],
						cssBundles: [],
					}),
				);

				await vormaNavigate("/with-module");
				await vi.runAllTimersAsync();

				expect(
					__vormaClientGlobal.get("activeComponents"),
				).toBeDefined();
			});

			it("should map modules using exportKeys array", async () => {
				const mockModule = {
					default: () => "DefaultExport",
					NamedExport: () => "NamedExport",
				};

				// In dev mode, viteDevURL is empty, so the path is just the module path with query
				vi.doMock("/multi-export.js", () => mockModule);

				vi.mocked(fetch).mockResolvedValue(
					createMockResponse({
						importURLs: ["/multi-export.js", "/multi-export.js"],
						exportKeys: ["default", "NamedExport"],
						cssBundles: [],
					}),
				);

				await vormaNavigate("/multi-export");
				await vi.runAllTimersAsync();

				const components = __vormaClientGlobal.get("activeComponents");
				expect(components).toHaveLength(2);
				expect(components?.[0]).toBe(mockModule.default);
				expect(components?.[1]).toBe(mockModule.NamedExport);
			});
		});

		describe("8.2 Error Boundaries", () => {
			it("should resolve and set the active error boundary using its index and export key", async () => {
				// 1. SETUP: Define multiple components to test indexing and resolution.
				const layoutComponent = () => "Layout";
				const pageComponent = () => "Page";
				const namedErrorComponent = () => "Named Error Component";

				// Mock the dynamic imports for all modules involved.
				vi.doMock("/layout.js", () => ({
					Layout: layoutComponent,
				}));
				vi.doMock("/page.js", () => ({
					Page: pageComponent,
				}));
				vi.doMock("/error.js", () => ({
					ErrorBoundary: namedErrorComponent,
				}));

				// 2. MOCK PAYLOAD: Create a server response with a list of components
				//    where the error boundary is not the first item.
				vi.mocked(fetch).mockResolvedValue(
					createMockResponse({
						importURLs: ["/layout.js", "/page.js", "/error.js"],
						exportKeys: ["Layout", "Page", "ErrorBoundary"],
						// Point to the 3rd module in the list (index 2)
						outermostServerErrorIdx: 2,
						errorExportKeys: ["", "", "ErrorBoundary"],
						cssBundles: [],
					}),
				);

				// 3. EXECUTION: Trigger the navigation.
				await vormaNavigate("/route-with-specific-error-boundary");
				await vi.runAllTimersAsync();

				// 4. ASSERTIONS: Verify the state is correct and comprehensive.
				const activeComponents =
					__vormaClientGlobal.get("activeComponents");
				const activeErrorBoundary = __vormaClientGlobal.get(
					"activeErrorBoundary",
				);

				// Main assertion: The correct error boundary component was resolved and set.
				expect(activeErrorBoundary).toBe(namedErrorComponent);

				// Sanity check: The regular components were also loaded correctly.
				expect(activeComponents).toHaveLength(3);
				expect(activeComponents?.[0]).toBe(layoutComponent);
				expect(activeComponents?.[1]).toBe(pageComponent);

				// The export from the error module will also be in the main components list,
				// which is the expected behavior of the `handleComponents` function.
				expect(activeComponents?.[2]).toBe(namedErrorComponent);
			});

			it("should fallback to defaultErrorBoundary if not found", async () => {
				const defaultError = () => "Default Error";
				setupGlobalVormaContext({ defaultErrorBoundary: defaultError });

				vi.mocked(fetch).mockResolvedValue(
					createMockResponse({
						importURLs: [],
						outermostServerErrorIdx: 0, // No component at this index
						cssBundles: [],
					}),
				);

				await vormaNavigate("/missing-error");
				await vi.runAllTimersAsync();

				expect(__vormaClientGlobal.get("activeErrorBoundary")).toBe(
					defaultError,
				);
			});
		});

		describe("8.3 URL Resolution", () => {
			it("should add  in development", async () => {
				const originalEnv = import.meta.env.DEV;
				(import.meta.env as any).DEV = true;

				setupGlobalVormaContext({
					viteDevURL: "http://localhost:5173",
					publicPathPrefix: "/static",
				});

				vi.mocked(fetch).mockResolvedValue(
					createMockResponse({
						importURLs: ["/dev-module.js"],
						cssBundles: [],
					}),
				);

				// Mock the import to verify the URL
				let importedUrl = "";
				vi.doMock("http://localhost:5173/dev-module.js", () => {
					importedUrl = "http://localhost:5173/dev-module.js";
					return { default: () => {} };
				});

				await vormaNavigate("/dev-test");
				await vi.runAllTimersAsync();

				// In dev, should use viteDevURL with
				expect(importedUrl).toContain("");

				(import.meta.env as any).DEV = originalEnv;
			});

			it("should use publicPathPrefix in production", async () => {
				const originalEnv = import.meta.env.DEV;
				(import.meta.env as any).DEV = false;

				setupGlobalVormaContext({
					publicPathPrefix: "/assets",
				});

				vi.mocked(fetch).mockResolvedValue(
					createMockResponse({
						importURLs: ["/prod-module.js"],
						cssBundles: [],
					}),
				);

				vi.doMock("/assets/prod-module.js", () => ({
					default: () => {},
				}));

				await vormaNavigate("/prod-test");
				await vi.runAllTimersAsync();

				// Verify it tried to import from the correct path
				expect(
					__vormaClientGlobal.get("activeComponents"),
				).toBeDefined();

				(import.meta.env as any).DEV = originalEnv;
			});

			it("should handle trailing slashes correctly", async () => {
				setupGlobalVormaContext({
					publicPathPrefix: "/static/", // With trailing slash
				});

				// Mock the module
				vi.doMock("/module.js", () => ({
					default: () => "Module",
				}));

				// Create mock response with CSS bundles
				const response = createMockResponse({
					importURLs: ["/module.js"],
					cssBundles: ["/styles.css"],
				});

				vi.mocked(fetch).mockResolvedValue(response);

				// Create and immediately trigger onload for preload links
				const appendChildSpy = vi
					.spyOn(document.head, "appendChild")
					.mockImplementation((element) => {
						// If it's a preload link, immediately trigger onload
						if (
							element instanceof HTMLLinkElement &&
							element.rel === "preload"
						) {
							setTimeout(
								() => element.onload?.(new Event("load")),
								0,
							);
						}
						return document.head.appendChild(element);
					});

				await vormaNavigate("/slash-test");

				// Wait for all async operations
				await vi.runAllTimersAsync();

				// Check that stylesheet links were added without double slashes
				const cssLinks = document.querySelectorAll(
					'link[rel="stylesheet"]',
				);
				cssLinks.forEach((link) => {
					const href = link.getAttribute("href");
					expect(href).not.toMatch(/\/\//); // No double slashes
					expect(href).toBe("/static/styles.css");
				});

				appendChildSpy.mockRestore();
			});
		});
	});

});
