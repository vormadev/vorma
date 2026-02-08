import {
	describe,
	expect,
	it,
	vi,
} from "vitest";

import {
	getBuildID,
	getHistoryInstance,
	getLocation,
	getRootEl,
} from "./client";
import {
	describeNavigationTestSuite,
	setupGlobalVormaContext,
	vormaAppConfig,
} from "./client.test.helpers.ts";
import {
	initClient,
} from "./init_client.ts";
import {
	__vormaClientGlobal,
	getRouterData,
	VORMA_SYMBOL,
} from "./vorma_ctx/vorma_ctx.ts";

describeNavigationTestSuite(() => {
	describe("Client global and accessor conformance", () => {
		it("FEC-CTX-001_FE-CTX-001_global_symbol_namespace_exists_after_bootstrap_init", async () => {
			await initClient({
				vormaAppConfig,
				renderFn: () => {},
			});

			const globalCtx = (globalThis as any)[VORMA_SYMBOL];
			expect(globalCtx).toBeDefined();
			expect(typeof globalCtx).toBe("object");
		});

		it("FEC-CTX-002_FE-CTX-002_get_router_data_includes_required_fields_and_rootdata_nullability", () => {
			setupGlobalVormaContext({
				buildID: "build-ctx-1",
				matchedPatterns: ["/", "/users/:id"],
				params: { id: "42" },
				splatValues: ["profile"],
				hasRootData: true,
				loadersData: [{ root: "yes" }],
			});

			expect(getRouterData()).toEqual({
				buildID: "build-ctx-1",
				matchedPatterns: ["/", "/users/:id"],
				params: { id: "42" },
				splatValues: ["profile"],
				rootData: { root: "yes" },
			});

			setupGlobalVormaContext({
				buildID: "build-ctx-2",
				matchedPatterns: ["/no-root"],
				params: {},
				splatValues: [],
				hasRootData: false,
				loadersData: [{ root: "ignored" }],
			});

			expect(getRouterData()).toEqual({
				buildID: "build-ctx-2",
				matchedPatterns: ["/no-root"],
				params: {},
				splatValues: [],
				rootData: null,
			});
		});

		it("FEC-CTX-003_FE-CTX-003_FE-CTX-004_FE-CTX-005_getters_match_location_buildid_and_root_contracts", () => {
			const root = document.createElement("div");
			root.id = "vorma-root";
			document.body.appendChild(root);

			setupGlobalVormaContext({ buildID: "ctx-build-123" });
			getHistoryInstance().replace("/ctx/path?query=value#section", {
				from: "ctx",
			});

			const location = getLocation();
			expect(location.pathname).toBe("/ctx/path");
			expect(location.search).toBe("?query=value");
			expect(location.hash).toBe("#section");
			expect(location).toHaveProperty("state");

			expect(getBuildID()).toBe("ctx-build-123");
			expect(getRootEl()).toBe(root);
		});

		it("FEC-CTX-004_FE-CTX-006_effective_error_selection_chooses_outermost_index_and_matching_side_message", async () => {
			const RootComp = () => "root";
			const ChildComp = () => "child";
			vi.doMock("/ctx-root.js", () => ({ default: RootComp }));
			vi.doMock("/ctx-child.js", () => ({ default: ChildComp }));

			// Case A: client error index 0 outranks server error index 1.
			setupGlobalVormaContext({
				hasRootData: true,
				matchedPatterns: ["/", "/child"],
				importURLs: ["/ctx-root.js", "/ctx-child.js"],
				exportKeys: ["default", "default"],
				loadersData: [{ root: true }, { child: true }],
				patternToWaitFnMap: {
					"/": vi.fn().mockRejectedValue(new Error("client-top")),
				},
				outermostServerErrorIdx: 1,
				outermostServerError: "server-deeper",
			});

			await initClient({
				vormaAppConfig,
				renderFn: () => {},
			});
			expect(__vormaClientGlobal.get("outermostErrorIdx")).toBe(0);
			expect(__vormaClientGlobal.get("outermostError")).toBe("client-top");

			// Case B: server error index 0 outranks client error index 1.
			setupGlobalVormaContext({
				hasRootData: true,
				matchedPatterns: ["/", "/child"],
				importURLs: ["/ctx-root.js", "/ctx-child.js"],
				exportKeys: ["default", "default"],
				loadersData: [{ root: true }, { child: true }],
				patternToWaitFnMap: {
					"/child": vi
						.fn()
						.mockRejectedValue(new Error("client-deeper")),
				},
				outermostServerErrorIdx: 0,
				outermostServerError: "server-root",
			});

			await initClient({
				vormaAppConfig,
				renderFn: () => {},
			});
			expect(__vormaClientGlobal.get("outermostErrorIdx")).toBe(0);
			expect(__vormaClientGlobal.get("outermostError")).toBe(
				"server-root",
			);
		});
	});
});
