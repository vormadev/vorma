import { describe, expect, it } from "vitest";
import { buildServerSuccessPreloadPlan } from "../../core/navigation/fetch_route_data_server.ts";

describe("server-success preload state machine", () => {
	it("returns empty preload plan when signal is already aborted", () => {
		const preloadPlan = buildServerSuccessPreloadPlan({
			signalAborted: true,
			isDev: false,
			importURLs: ["/a.js"],
			deps: ["/a.js"],
			cssBundles: ["/a.css"],
		});

		expect(preloadPlan).toEqual({
			moduleDependencies: [],
			cssBundles: [],
		});
	});

	it("uses deduped import URLs for dev preload module dependencies", () => {
		const preloadPlan = buildServerSuccessPreloadPlan({
			signalAborted: false,
			isDev: true,
			importURLs: ["/a.js", "/a.js", "/b.js"],
			deps: ["/prod-only.js"],
			cssBundles: ["/a.css"],
		});

		expect(preloadPlan).toEqual({
			moduleDependencies: ["/a.js", "/b.js"],
			cssBundles: ["/a.css"],
		});
	});

	it("uses production deps for non-dev preload module dependencies", () => {
		const preloadPlan = buildServerSuccessPreloadPlan({
			signalAborted: false,
			isDev: false,
			importURLs: ["/dev-only.js"],
			deps: ["/prod-a.js", "/prod-b.js"],
			cssBundles: ["/a.css", "/b.css"],
		});

		expect(preloadPlan).toEqual({
			moduleDependencies: ["/prod-a.js", "/prod-b.js"],
			cssBundles: ["/a.css", "/b.css"],
		});
	});
});
