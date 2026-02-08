import {
	findNestedMatches,
} from "vorma/kit/matcher/find-nested";
import {
	registerPattern,
} from "vorma/kit/matcher/register";
import {
	describe,
	expect,
	it,
	vi,
} from "vitest";

import {
	getHistoryInstance,
} from "./client";
import {
	createMockResponse,
	describeNavigationTestSuite,
	setupGlobalVormaContext,
	vormaAppConfig,
} from "./client.test.helpers.ts";
import {
	defaultErrorBoundary,
} from "./error_boundary.ts";
import {
	initClient,
} from "./init_client.ts";
import {
	__vormaClientGlobal,
} from "./vorma_ctx/vorma_ctx.ts";

function customAppConfig(overrides?: Partial<typeof vormaAppConfig>) {
	return {
		...vormaAppConfig,
		...(overrides || {}),
	};
}

async function flushMicrotasks(times = 3) {
	for (let i = 0; i < times; i++) {
		await Promise.resolve();
	}
}

describeNavigationTestSuite(() => {
	describe("Initialization conformance", () => {
		it("FEC-INIT-001_FE-INIT-001_FE-INIT-009_init_warmup_completes_components_client_loaders_error_boundary_before_render", async () => {
			const waitFn = vi
				.fn()
				.mockRejectedValue(new Error("client-loader-boom"));
			const EntryComp = () => "entry";
			const EntryBoundary = ({ error }: { error: string }) =>
				`boundary:${error}`;

			setupGlobalVormaContext({
				hasRootData: true,
				matchedPatterns: ["/"],
				importURLs: ["/init-entry.js"],
				exportKeys: ["default"],
				errorExportKeys: ["Boundary"],
				loadersData: [{ root: "data" }],
				patternToWaitFnMap: { "/": waitFn },
				params: {},
				splatValues: [],
				publicPathPrefix: "/",
			});

			vi.doMock("/init-entry.js", () => ({
				default: EntryComp,
				Boundary: EntryBoundary,
			}));

			const historyListenSpy = vi.spyOn(getHistoryInstance(), "listen");
			let renderSnapshot: Record<string, any> | undefined;
			const renderFn = vi.fn(() => {
				renderSnapshot = {
					activeComponent: __vormaClientGlobal.get("activeComponents")?.[0],
					clientLoadersData: __vormaClientGlobal.get(
						"clientLoadersData",
					),
					outermostClientErrorIdx: __vormaClientGlobal.get(
						"outermostClientErrorIdx",
					),
					activeErrorBoundary: __vormaClientGlobal.get(
						"activeErrorBoundary",
					),
				};
			});

			await initClient({
				vormaAppConfig,
				renderFn,
			});

			expect(historyListenSpy).toHaveBeenCalled();
			if ((import.meta as any).env?.DEV) {
				expect(typeof (window as any).__waveRevalidate).toBe("function");
			}
			expect(waitFn).toHaveBeenCalledTimes(1);
			expect(renderFn).toHaveBeenCalledTimes(1);
			expect(renderSnapshot).toBeDefined();
			expect(renderSnapshot?.activeComponent).toBe(EntryComp);
			expect(renderSnapshot?.clientLoadersData).toEqual([undefined]);
			expect(renderSnapshot?.outermostClientErrorIdx).toBe(0);
			expect(renderSnapshot?.activeErrorBoundary).toBe(EntryBoundary);
		});

		it("FEC-INIT-002_FE-INIT-002_FE-INIT-010_beforeunload_saves_refresh_scroll_and_recent_refresh_state_restores", async () => {
			const recentRefreshState = {
				x: 333,
				y: 444,
				unix: Date.now() - 500,
				href: window.location.href,
			};
			sessionStorage.setItem(
				"__vorma__pageRefreshScrollState",
				JSON.stringify(recentRefreshState),
			);

			const rafSpy = vi
				.spyOn(window, "requestAnimationFrame")
				.mockImplementation((cb: FrameRequestCallback) => {
					cb(0);
					return 1;
				});

			await initClient({
				vormaAppConfig,
				renderFn: () => {},
			});

			expect(window.scrollTo).toHaveBeenCalledWith(333, 444);
			expect(
				sessionStorage.getItem("__vorma__pageRefreshScrollState"),
			).toBeNull();

			(window as any).scrollX = 111;
			(window as any).scrollY = 222;
			window.dispatchEvent(new Event("beforeunload"));

			const stored = JSON.parse(
				sessionStorage.getItem("__vorma__pageRefreshScrollState") || "{}",
			);
			expect(stored.x).toBe(111);
			expect(stored.y).toBe(222);
			expect(stored.href).toBe(window.location.href);
			expect(typeof stored.unix).toBe("number");

			rafSpy.mockRestore();
		});

		it("FEC-INIT-003_FE-INIT-003_FE-INIT-004_init_seeds_client_module_map_and_initializes_pattern_registry_with_configured_runes", async () => {
			const cfg = customAppConfig({
				loadersDynamicRune: "$",
				loadersSplatRune: "~",
				loadersExplicitIndexSegment: "index",
			});
			const SeedComp = () => "seed";

			setupGlobalVormaContext({
				hasRootData: true,
				matchedPatterns: ["/users/$id"],
				importURLs: ["/seed.js"],
				exportKeys: ["Page"],
				errorExportKeys: ["Err"],
				publicPathPrefix: "/",
			});
			vi.doMock("/seed.js", () => ({ Page: SeedComp, Err: () => "err" }));

			await initClient({
				vormaAppConfig: cfg,
				renderFn: () => {},
			});

			const clientModuleMap = __vormaClientGlobal.get("clientModuleMap");
			expect(clientModuleMap["/users/$id"]).toEqual({
				importURL: "/seed.js",
				exportKey: "Page",
				errorExportKey: "Err",
			});

			const patternRegistry = __vormaClientGlobal.get("patternRegistry");
			registerPattern(patternRegistry, "/files/$id/~");
			const match = findNestedMatches(patternRegistry, "/files/42/a/b");
			expect(match?.params.id).toBe("42");
			expect(match?.splatValues).toEqual(["a", "b"]);
		});

		it("FEC-INIT-004_FE-INIT-005_manifest_success_stores_manifest_and_registers_patterns", async () => {
			const manifest = {
				"/docs": 1,
				"/docs/:id": 1,
			};
			setupGlobalVormaContext({
				routeManifestURL: "/manifest.json",
			});

			vi.mocked(fetch).mockImplementation(async (input: RequestInfo | URL) => {
				if (String(input).includes("/manifest.json")) {
					return createMockResponse(manifest);
				}
				return createMockResponse({});
			});

			await initClient({
				vormaAppConfig,
				renderFn: () => {},
			});
			await flushMicrotasks();

			expect(__vormaClientGlobal.get("routeManifest")).toEqual(manifest);
			const patternRegistry = __vormaClientGlobal.get("patternRegistry");
			const match = findNestedMatches(patternRegistry, "/docs/abc");
			expect(match?.matches.length).toBeGreaterThan(0);
			expect(match?.params.id).toBe("abc");
		});

		it("FEC-INIT-004_FE-INIT-005_manifest_fetch_failure_is_non_fatal_progressive_enhancement", async () => {
			setupGlobalVormaContext({
				routeManifestURL: "/manifest.json",
			});

			const warnSpy = vi
				.spyOn(console, "warn")
				.mockImplementation(() => {});
			vi.mocked(fetch).mockRejectedValue(new Error("manifest failed"));

			const renderFn = vi.fn();
			await initClient({
				vormaAppConfig,
				renderFn,
			});
			await flushMicrotasks();

			expect(renderFn).toHaveBeenCalledTimes(1);
			expect(warnSpy).toHaveBeenCalled();
			expect(__vormaClientGlobal.get("routeManifest")).toBeUndefined();
		});

		it("FEC-INIT-005_FE-INIT-006_default_error_boundary_uses_custom_when_provided_else_builtin", async () => {
			const customBoundary = () => "custom-boundary";

			await initClient({
				vormaAppConfig,
				renderFn: () => {},
				defaultErrorBoundary: customBoundary,
			});
			expect(__vormaClientGlobal.get("defaultErrorBoundary")).toBe(
				customBoundary,
			);

			setupGlobalVormaContext();
			await initClient({
				vormaAppConfig,
				renderFn: () => {},
			});
			expect(__vormaClientGlobal.get("defaultErrorBoundary")).toBe(
				defaultErrorBoundary,
			);
		});

		it("FEC-INIT-006_FE-INIT-007_use_view_transitions_option_enables_runtime_flag", async () => {
			await initClient({
				vormaAppConfig,
				renderFn: () => {},
				useViewTransitions: true,
			});
			expect(__vormaClientGlobal.get("useViewTransitions")).toBe(true);
		});

		it("FEC-INIT-007_FE-INIT-008_hard_reload_query_cleanup_replaces_url_without_vorma_reload", async () => {
			window.history.replaceState(
				{},
				"",
				"/?vorma_reload=old-build&keep=this",
			);
			const replaceSpy = vi.spyOn(getHistoryInstance(), "replace");

			await initClient({
				vormaAppConfig,
				renderFn: () => {},
			});

			expect(replaceSpy).toHaveBeenCalledWith(
				"http://localhost:3000/?keep=this",
			);
		});

		it("FEC-INIT-008_FE-INIT-011_touchstart_sets_touch_device_flag", async () => {
			await initClient({
				vormaAppConfig,
				renderFn: () => {},
			});
			expect(__vormaClientGlobal.get("isTouchDevice")).toBeUndefined();

			window.dispatchEvent(new Event("touchstart"));
			expect(__vormaClientGlobal.get("isTouchDevice")).toBe(true);
		});
	});
});
