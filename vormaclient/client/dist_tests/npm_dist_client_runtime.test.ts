import { describe, expect, it, vi } from "vitest";
import { findNestedMatches } from "vorma/kit/matcher/find-nested";
import {
	DIST_TEST_VORMA_APP_CONFIG,
	installDistTestVormaGlobal,
} from "./dist_test_harness.ts";

describe("npm_dist client runtime", () => {
	it("resolves loader paths from compiled exports", async () => {
		installDistTestVormaGlobal();
		vi.resetModules();
		const distClient = await import("vorma/client");

		const path = distClient.__resolvePath({
			vormaAppConfig: DIST_TEST_VORMA_APP_CONFIG,
			type: "loader",
			props: {
				pattern: "/products/:id/_index",
				params: { id: "42" },
			},
		});

		expect(path).toBe("/products/42");
	});

	it("registers adapter client loaders in compiled runtime", async () => {
		const globals = installDistTestVormaGlobal();
		vi.resetModules();
		const distClient = await import("vorma/client");

		const waitFn = vi.fn(async () => "ready");
		distClient.__registerClientLoaderForAdapter({
			pattern: "/items/:id",
			waitFn,
		});

		const patternToWaitFnMap = globals.patternToWaitFnMap;
		expect(typeof patternToWaitFnMap["/items/:id"]).toBe("function");

		const patternRegistry = globals.patternRegistry;
		const nestedMatch = findNestedMatches(
			patternRegistry as any,
			"/items/123",
		);
		expect(nestedMatch).not.toBeNull();
	});
});
