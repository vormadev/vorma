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
	createMockResponse,
	describeNavigationTestSuite,
	setupGlobalVormaContext,
} from "./client.test.helpers.ts";
import {
	__vormaClientGlobal,
} from "./vorma_ctx/vorma_ctx.ts";

describeNavigationTestSuite(() => {
	describe("Component and error-boundary conformance", () => {
		it("FEC-COMP-001_FE-COMP-001_FE-COMP-002_dynamic_import_resolution_and_export_key_mapping_match_contract", async () => {
			const originalDev = import.meta.env.DEV;

			// Dev-mode import URL resolution should use viteDevURL base.
			(import.meta.env as any).DEV = true;
			const DevComp = () => "dev";
			setupGlobalVormaContext({
				viteDevURL: "http://localhost:5173",
				publicPathPrefix: "/assets",
			});
			vi.doMock("http://localhost:5173/dev-route.js", () => ({
				default: DevComp,
			}));
			vi.mocked(fetch).mockResolvedValue(
				createMockResponse({
					importURLs: ["/dev-route.js"],
					exportKeys: ["default"],
					cssBundles: [],
				}),
			);
			await vormaNavigate("/comp-dev");
			await vi.runAllTimersAsync();
			expect(__vormaClientGlobal.get("activeComponents")?.[0]).toBe(
				DevComp,
			);

			// Prod-mode import URL resolution should use publicPathPrefix base.
			(import.meta.env as any).DEV = false;
			const ProdDefault = () => "prod-default";
			const ProdNamed = () => "prod-named";
			setupGlobalVormaContext({
				publicPathPrefix: "/assets",
			});
			vi.doMock("/assets/prod-route.js", () => ({
				default: ProdDefault,
				Named: ProdNamed,
			}));
			vi.mocked(fetch).mockResolvedValue(
				createMockResponse({
					importURLs: ["/prod-route.js", "/prod-route.js"],
					exportKeys: ["default", "Named"],
					cssBundles: [],
				}),
			);
			await vormaNavigate("/comp-prod");
			await vi.runAllTimersAsync();
			const active = __vormaClientGlobal.get("activeComponents");
			expect(active?.[0]).toBe(ProdDefault);
			expect(active?.[1]).toBe(ProdNamed);

			(import.meta.env as any).DEV = originalDev;
		});

		it("FEC-COMP-002_FE-COMP-003_component_state_update_is_minimized_when_resolved_components_are_unchanged", async () => {
			const StableComp = () => "stable";
			setupGlobalVormaContext({
				publicPathPrefix: "/",
			});
			vi.doMock("/stable.js", () => ({ default: StableComp }));

			vi.mocked(fetch).mockResolvedValue(
				createMockResponse({
					importURLs: ["/stable.js"],
					exportKeys: ["default"],
					cssBundles: [],
				}),
			);
			await vormaNavigate("/stable-a");
			await vi.runAllTimersAsync();
			const first = __vormaClientGlobal.get("activeComponents");

			vi.mocked(fetch).mockResolvedValue(
				createMockResponse({
					importURLs: ["/stable.js"],
					exportKeys: ["default"],
					cssBundles: [],
				}),
			);
			await vormaNavigate("/stable-b");
			await vi.runAllTimersAsync();
			const second = __vormaClientGlobal.get("activeComponents");

			expect(first).toBe(second);
		});

		it("FEC-COMP-003_FE-COMP-004_route_specific_error_boundary_export_is_used_when_present", async () => {
			const RouteBoundary = ({ error }: { error: string }) =>
				`route-boundary:${error}`;

			setupGlobalVormaContext({
				defaultErrorBoundary: () => "default-boundary",
			});
			vi.doMock("/layout.js", () => ({ default: () => "layout" }));
			vi.doMock("/route-error.js", () => ({
				default: () => "page",
				RouteBoundary,
			}));
			vi.mocked(fetch).mockResolvedValue(
				createMockResponse({
					importURLs: ["/layout.js", "/route-error.js"],
					exportKeys: ["default", "default"],
					outermostServerErrorIdx: 1,
					errorExportKeys: ["", "RouteBoundary"],
					cssBundles: [],
				}),
			);
			await vormaNavigate("/err-specific");
			await vi.runAllTimersAsync();
			expect(__vormaClientGlobal.get("activeErrorBoundary")).toBe(
				RouteBoundary,
			);
		});

		it("FEC-COMP-003_FE-COMP-004_missing_route_error_export_on_existing_module_falls_back_to_default_boundary", async () => {
			const DefaultBoundary = () => "default-boundary";
			setupGlobalVormaContext({
				defaultErrorBoundary: DefaultBoundary,
			});
			__vormaClientGlobal.set("defaultErrorBoundary", DefaultBoundary);
			vi.doMock("/err-layout.js", () => ({ default: () => "layout" }));
			vi.doMock("/err-page.js", () => ({ default: () => "page" }));

			vi.mocked(fetch).mockResolvedValue(
				createMockResponse({
					importURLs: ["/err-layout.js", "/err-page.js"],
					exportKeys: ["default", "default"],
					errorExportKeys: ["", "MissingBoundaryExport"],
					outermostServerErrorIdx: 1,
					outermostServerError: "server-fallback",
					cssBundles: [],
				}),
			);
			await vormaNavigate("/err-fallback");
			await vi.runAllTimersAsync();
			expect(__vormaClientGlobal.get("activeErrorBoundary")).toBe(
				DefaultBoundary,
			);
		});
	});
});
