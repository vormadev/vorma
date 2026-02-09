import {
	createPatternRegistry,
	registerPattern,
	type PatternRegistry,
} from "vorma/kit/matcher/register";
import {
	describe,
	expect,
	it,
	vi,
} from "vitest";

import { vormaNavigate } from "./client";
import {
	createMockResponse,
	describeNavigationTestSuite,
	setupGlobalVormaContext,
} from "./client.test.helpers.ts";
import { __vormaClientGlobal } from "./vorma_ctx/vorma_ctx.ts";

function makeRegistry(patterns: string[]): PatternRegistry {
	const registry = createPatternRegistry();
	for (const pattern of patterns) {
		registerPattern(registry, pattern);
	}
	return registry;
}

describeNavigationTestSuite(() => {
	describe("Client-only skip optimization conformance", () => {
		it("FEC-SKIP-001_FE-SKIP-001_eligible_transition_may_skip_server_fetch", async () => {
			const registry = makeRegistry(["/users/:id"]);
			setupGlobalVormaContext({
				publicPathPrefix: "/",
				routeManifest: { "/users/:id": 1 },
				patternRegistry: registry,
				clientModuleMap: {
					"/users/:id": {
						importURL: "/user.js",
						exportKey: "default",
						errorExportKey: "",
					},
				},
				matchedPatterns: ["/users/:id"],
				params: { id: "1" },
				splatValues: [],
				loadersData: [{ from: "cache" }],
			});
			vi.doMock("/user.js", () => ({ default: () => "user-comp" }));
			vi.mocked(fetch).mockResolvedValue(
				createMockResponse({
					importURLs: ["/user.js"],
					exportKeys: ["default"],
					cssBundles: [],
				}),
			);

			await vormaNavigate("/users/1");
			await vi.runAllTimersAsync();

			expect(fetch).not.toHaveBeenCalled();
			expect(window.location.pathname).toBe("/users/1");
		});

		it("FEC-SKIP-002_FE-SKIP-002_skip_requires_route_manifest_and_registry", async () => {
			const registry = makeRegistry(["/users/:id"]);
			vi.doMock("/user.js", () => ({ default: () => "user-comp" }));
			const serverJSON = {
				matchedPatterns: ["/users/:id"],
				loadersData: [{ from: "server" }],
				importURLs: ["/user.js"],
				exportKeys: ["default"],
				cssBundles: [],
			};

			setupGlobalVormaContext({
				publicPathPrefix: "/",
				routeManifest: undefined,
				patternRegistry: registry,
				clientModuleMap: {
					"/users/:id": {
						importURL: "/user.js",
						exportKey: "default",
						errorExportKey: "",
					},
				},
				matchedPatterns: ["/users/:id"],
				params: { id: "1" },
				splatValues: [],
				loadersData: [{ from: "cache" }],
			});
			vi.mocked(fetch).mockResolvedValue(createMockResponse(serverJSON));
			await vormaNavigate("/users/1");
			await vi.runAllTimersAsync();
			expect(fetch).toHaveBeenCalledTimes(1);

			vi.clearAllMocks();

			setupGlobalVormaContext({
				publicPathPrefix: "/",
				routeManifest: { "/users/:id": 1 },
				patternRegistry: undefined as unknown as PatternRegistry,
				clientModuleMap: {
					"/users/:id": {
						importURL: "/user.js",
						exportKey: "default",
						errorExportKey: "",
					},
				},
				matchedPatterns: ["/users/:id"],
				params: { id: "1" },
				splatValues: [],
				loadersData: [{ from: "cache" }],
			});
			vi.mocked(fetch).mockResolvedValue(createMockResponse(serverJSON));
			await vormaNavigate("/users/1");
			await vi.runAllTimersAsync();
			expect(fetch).toHaveBeenCalledTimes(1);
		});

		it("FEC-SKIP-003_FE-SKIP-003_FE-SKIP-004_FE-SKIP-005_skip_must_not_hide_loader_removal_new_client_loader_or_loader_relevant_param_changes", async () => {
			vi.doMock("/user.js", () => ({ default: () => "user-comp" }));
			vi.doMock("/about.js", () => ({ default: () => "about-comp" }));
			vi.doMock("/app.js", () => ({ default: () => "app-comp" }));
			vi.doMock("/app-child.js", () => ({ default: () => "app-child" }));
			vi.doMock("/user-splat.js", () => ({ default: () => "user-splat" }));

			{
				const registry = makeRegistry(["/users/:id", "/about"]);
				setupGlobalVormaContext({
					publicPathPrefix: "/",
					routeManifest: { "/users/:id": 1, "/about": 1 },
					patternRegistry: registry,
					clientModuleMap: {
						"/users/:id": {
							importURL: "/user.js",
							exportKey: "default",
							errorExportKey: "",
						},
						"/about": {
							importURL: "/about.js",
							exportKey: "default",
							errorExportKey: "",
						},
					},
					matchedPatterns: ["/users/:id"],
					params: { id: "1" },
					splatValues: [],
					loadersData: [{ from: "cache" }],
				});
				vi.mocked(fetch).mockResolvedValue(
					createMockResponse({
						matchedPatterns: ["/about"],
						loadersData: [{ from: "server" }],
						importURLs: ["/about.js"],
						exportKeys: ["default"],
						cssBundles: [],
					}),
				);
				await vormaNavigate("/about");
				await vi.runAllTimersAsync();
				expect(fetch).toHaveBeenCalledTimes(1);
			}

			vi.clearAllMocks();

			{
				const childLoader = vi.fn().mockResolvedValue("child-loader");
				const registry = makeRegistry(["/app", "/app/child"]);
				setupGlobalVormaContext({
					publicPathPrefix: "/",
					routeManifest: { "/app": 0, "/app/child": 0 },
					patternRegistry: registry,
					clientModuleMap: {
						"/app": {
							importURL: "/app.js",
							exportKey: "default",
							errorExportKey: "",
						},
						"/app/child": {
							importURL: "/app-child.js",
							exportKey: "default",
							errorExportKey: "",
						},
					},
					patternToWaitFnMap: {
						"/app/child": childLoader,
					},
					matchedPatterns: ["/app"],
					params: {},
					splatValues: [],
					loadersData: [],
				});
				vi.mocked(fetch).mockResolvedValue(
					createMockResponse({
						matchedPatterns: ["/app", "/app/child"],
						loadersData: [undefined, undefined],
						importURLs: ["/app.js", "/app-child.js"],
						exportKeys: ["default", "default"],
						cssBundles: [],
					}),
				);
				await vormaNavigate("/app/child");
				await vi.runAllTimersAsync();
				expect(fetch).toHaveBeenCalledTimes(1);
			}

			vi.clearAllMocks();

			{
				const registry = makeRegistry(["/users/:id/*"]);
				setupGlobalVormaContext({
					publicPathPrefix: "/",
					routeManifest: { "/users/:id/*": 1 },
					patternRegistry: registry,
					clientModuleMap: {
						"/users/:id/*": {
							importURL: "/user-splat.js",
							exportKey: "default",
							errorExportKey: "",
						},
					},
					matchedPatterns: ["/users/:id/*"],
					params: { id: "1" },
					splatValues: ["a"],
					loadersData: [{ from: "cache" }],
				});
				window.history.replaceState({}, "", "/users/1/a");
				vi.mocked(fetch).mockResolvedValue(
					createMockResponse({
						matchedPatterns: ["/users/:id/*"],
						loadersData: [{ from: "server" }],
						importURLs: ["/user-splat.js"],
						exportKeys: ["default"],
						cssBundles: [],
					}),
				);
				await vormaNavigate("/users/1/a?tab=2");
				await vi.runAllTimersAsync();
				expect(fetch).toHaveBeenCalledTimes(1);
			}
		});

		it("FEC-SKIP-004_FE-SKIP-006_skip_output_arrays_must_remain_index_aligned", async () => {
			const Parent = () => "parent";
			const ChildNamed = () => "child";
			vi.doMock("/parent.js", () => ({ default: Parent }));
			vi.doMock("/child.js", () => ({ Named: ChildNamed }));

			const registry = makeRegistry(["/parent", "/parent/child"]);
			setupGlobalVormaContext({
				publicPathPrefix: "/",
				routeManifest: { "/parent": 1, "/parent/child": 0 },
				patternRegistry: registry,
				clientModuleMap: {
					"/parent": {
						importURL: "/parent.js",
						exportKey: "default",
						errorExportKey: "",
					},
					"/parent/child": {
						importURL: "/child.js",
						exportKey: "Named",
						errorExportKey: "",
					},
				},
				matchedPatterns: ["/parent", "/parent/child"],
				params: {},
				splatValues: [],
				loadersData: ["parent-cache", "child-cache-unused"],
			});
			vi.mocked(fetch).mockResolvedValue(
				createMockResponse({
					matchedPatterns: ["/parent", "/parent/child"],
					loadersData: ["server-parent", undefined],
					importURLs: ["/parent.js", "/child.js"],
					exportKeys: ["default", "Named"],
					cssBundles: [],
				}),
			);

			await vormaNavigate("/parent/child");
			await vi.runAllTimersAsync();

			expect(fetch).not.toHaveBeenCalled();

			const matchedPatterns = __vormaClientGlobal.get("matchedPatterns");
			const loadersData = __vormaClientGlobal.get("loadersData");
			const importURLs = __vormaClientGlobal.get("importURLs");
			const exportKeys = __vormaClientGlobal.get("exportKeys");
			expect(matchedPatterns).toHaveLength(2);
			expect(loadersData).toHaveLength(2);
			expect(importURLs).toHaveLength(2);
			expect(exportKeys).toHaveLength(2);

			expect(matchedPatterns[0]).toBe("/parent");
			expect(importURLs[0]).toBe("/parent.js");
			expect(exportKeys[0]).toBe("default");
			expect(loadersData[0]).toBe("parent-cache");

			expect(matchedPatterns[1]).toBe("/parent/child");
			expect(importURLs[1]).toBe("/child.js");
			expect(exportKeys[1]).toBe("Named");
			expect(loadersData[1]).toBeUndefined();
		});
	});
});
