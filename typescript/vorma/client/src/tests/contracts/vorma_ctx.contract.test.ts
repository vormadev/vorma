import { describe, expect, it } from "vitest";
import {
	__getVormaClientGlobal,
	getRouterData,
	VORMA_SYMBOL,
} from "../../runtime.ts";
import {
	installContractVormaGlobal,
	setupContractTestSuite,
} from "./contract_test_harness.ts";

setupContractTestSuite();

describe("vorma context contracts", () => {
	it("reads values from the shared global state", () => {
		installContractVormaGlobal({
			params: { key: "value" },
		});

		const { get } = __getVormaClientGlobal();
		expect(get("runtimeRouteSnapshot").params).toEqual({ key: "value" });
	});

	it("writes values into the shared global state", () => {
		const { get, set } = __getVormaClientGlobal();
		set("runtimeRouteSnapshot", {
			...get("runtimeRouteSnapshot"),
			buildID: "123",
		});

		expect(get("runtimeRouteSnapshot").buildID).toBe("123");
		expect(
			(globalThis as any)[VORMA_SYMBOL].runtimeRouteSnapshot,
		).toMatchObject({
			buildID: "123",
		});
	});

	it("overwrites existing global values deterministically", () => {
		installContractVormaGlobal({
			activeComponents: [],
		});

		const { get, set } = __getVormaClientGlobal();
		set("runtimeRouteSnapshot", {
			...get("runtimeRouteSnapshot"),
			activeComponents: ["Component1"],
		});

		expect(get("runtimeRouteSnapshot").activeComponents).toEqual([
			"Component1",
		]);
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

	it("throws a clear bootstrap error when runtime globals are missing", () => {
		delete (globalThis as any)[VORMA_SYMBOL];
		const { get } = __getVormaClientGlobal();

		expect(() => get("runtimeRouteSnapshot").buildID).toThrow(
			'Vorma client runtime state is not initialized on globalThis[Symbol.for("__vorma_internal__")].',
		);
	});

	it("fails loud when invalid route-field keys are passed through unsafe casts", () => {
		installContractVormaGlobal();
		const { get, set } = __getVormaClientGlobal();

		expect(() => (get as any)("buildID")).toThrow(
			'Vorma client global get contract violated: unsupported key "buildID".',
		);
		expect(() => (set as any)("buildID", "unsafe")).toThrow(
			'Vorma client global set contract violated: unsupported key "buildID".',
		);
	});
});
