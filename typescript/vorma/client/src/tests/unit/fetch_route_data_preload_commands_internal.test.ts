import { describe, expect, it } from "vitest";
import { buildServerSuccessPreloadCommands } from "../../core/navigation/fetch_route_data_server.ts";

describe("server-success preload command builder", () => {
	it("returns no commands for skipped preload plans", () => {
		const commands = buildServerSuccessPreloadCommands({
			executionPlan: {
				type: "skip",
				reason: "server_success_preload_skipped_signal_aborted",
			},
		});

		expect(commands).toEqual([]);
	});

	it("maps preload plan dependencies and css bundles to ordered commands", () => {
		const commands = buildServerSuccessPreloadCommands({
			executionPlan: {
				type: "preload",
				moduleDependenciesToPreload: ["/a.js", "/b.js"],
				cssBundlesToPreload: ["/a.css", "/b.css"],
				reason: "server_success_preload_allowed",
			},
		});

		expect(commands).toEqual([
			{
				type: "preload_module_dependency",
				dependency: "/a.js",
				reason: "server_success_preload_allowed",
			},
			{
				type: "preload_module_dependency",
				dependency: "/b.js",
				reason: "server_success_preload_allowed",
			},
			{
				type: "preload_css_bundle",
				bundle: "/a.css",
				reason: "server_success_preload_allowed",
			},
			{
				type: "preload_css_bundle",
				bundle: "/b.css",
				reason: "server_success_preload_allowed",
			},
		]);
	});

	it("filters empty or non-string preload entries", () => {
		const commands = buildServerSuccessPreloadCommands({
			executionPlan: {
				type: "preload",
				moduleDependenciesToPreload: [
					"",
					"/a.js",
					null as unknown as string,
				],
				cssBundlesToPreload: ["", "/a.css", null as unknown as string],
				reason: "server_success_preload_allowed",
			},
		});

		expect(commands).toEqual([
			{
				type: "preload_module_dependency",
				dependency: "/a.js",
				reason: "server_success_preload_allowed",
			},
			{
				type: "preload_css_bundle",
				bundle: "/a.css",
				reason: "server_success_preload_allowed",
			},
		]);
	});
});
