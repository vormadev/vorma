import { describe, expect, it } from "vitest";
import { buildServerSuccessPreloadPlan } from "../../runtime.ts";

describe("server-success preload plan builder", () => {
	it("returns empty preload plan for aborted preloads", () => {
		const preloadPlan = buildServerSuccessPreloadPlan({
			signalAborted: true,
			isDev: true,
			importURLs: ["/a.js"],
			deps: ["/prod-a.js"],
			cssBundles: ["/a.css"],
		});

		expect(preloadPlan).toEqual({
			moduleDependencies: [],
			cssBundles: [],
		});
	});

	it("maps dependencies and css bundles to ordered preload arrays", () => {
		const preloadPlan = buildServerSuccessPreloadPlan({
			signalAborted: false,
			isDev: false,
			importURLs: ["/ignored-dev.js"],
			deps: ["/a.js", "/b.js"],
			cssBundles: ["/a.css", "/b.css"],
		});

		expect(preloadPlan).toEqual({
			moduleDependencies: ["/a.js", "/b.js"],
			cssBundles: ["/a.css", "/b.css"],
		});
	});

	it("filters empty-string preload entries", () => {
		const preloadPlan = buildServerSuccessPreloadPlan({
			signalAborted: false,
			isDev: false,
			importURLs: [],
			deps: ["", "/a.js"],
			cssBundles: ["", "/a.css"],
		});

		expect(preloadPlan).toEqual({
			moduleDependencies: ["/a.js"],
			cssBundles: ["/a.css"],
		});
	});

	it("fails loud for non-string preload entries that violate the typed contract", () => {
		expect(() =>
			buildServerSuccessPreloadPlan({
				signalAborted: false,
				isDev: false,
				importURLs: [],
				deps: ["/a.js", null as unknown as string],
				cssBundles: ["/a.css"],
			}),
		).toThrow("Cannot read properties of null");
	});
});
