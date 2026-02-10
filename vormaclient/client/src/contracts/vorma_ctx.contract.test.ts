import { describe, expect, it } from "vitest";
import {
	installContractVormaGlobal,
	setupContractTestSuite,
} from "./contract_test_harness.ts";
import {
	__getVormaClientGlobal,
	getRouterData,
	VORMA_SYMBOL,
} from "../vorma_ctx/vorma_ctx.ts";

setupContractTestSuite();

describe("vorma context contracts", () => {
	it("reads values from the shared global state", () => {
		installContractVormaGlobal({
			params: { key: "value" },
		});

		const { get } = __getVormaClientGlobal();
		expect(get("params")).toEqual({ key: "value" });
	});

	it("writes values into the shared global state", () => {
		const { get, set } = __getVormaClientGlobal();
		set("buildID", "123");

		expect(get("buildID")).toBe("123");
		expect((globalThis as any)[VORMA_SYMBOL].buildID).toBe("123");
	});

	it("overwrites existing global values deterministically", () => {
		installContractVormaGlobal({
			activeComponents: [],
		});

		const { get, set } = __getVormaClientGlobal();
		set("activeComponents", ["Component1"]);

		expect(get("activeComponents")).toEqual(["Component1"]);
	});

	it("returns safe defaults from getRouterData when state is uninitialized", () => {
		installContractVormaGlobal({
			buildID: undefined,
			matchedPatterns: undefined,
			splatValues: undefined,
			params: undefined,
			hasRootData: false,
			loadersData: undefined,
		});

		expect(getRouterData()).toEqual({
			buildID: "",
			matchedPatterns: [],
			splatValues: [],
			params: {},
			rootData: null,
		});
	});

	it("returns root loader data when getRouterData has root data", () => {
		const rootData = { page: "home" };
		installContractVormaGlobal({
			buildID: "build-2",
			hasRootData: true,
			loadersData: [rootData, { ignored: true }],
			matchedPatterns: ["/"],
			splatValues: ["rest"],
			params: { slug: "home" },
		});

		expect(getRouterData<typeof rootData, { slug: string }>()).toEqual({
			buildID: "build-2",
			matchedPatterns: ["/"],
			splatValues: ["rest"],
			params: { slug: "home" },
			rootData,
		});
	});
});
