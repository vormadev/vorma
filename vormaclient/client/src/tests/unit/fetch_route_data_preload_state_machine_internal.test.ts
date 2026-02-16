import { describe, expect, it } from "vitest";
import { decideServerSuccessPreloadExecutionPlan } from "../../core/navigation/fetch_route_data_preload_state_machine.ts";

describe("server-success preload state machine", () => {
	it("skips preload plan when signal is already aborted", () => {
		const plan = decideServerSuccessPreloadExecutionPlan({
			signalAborted: true,
			isDev: false,
			importURLs: ["/a.js"],
			deps: ["/a.js"],
			cssBundles: ["/a.css"],
		});

		expect(plan).toEqual({
			type: "skip",
			reason: "server_success_preload_skipped_signal_aborted",
		});
	});

	it("uses deduped import URLs for dev preload module dependencies", () => {
		const plan = decideServerSuccessPreloadExecutionPlan({
			signalAborted: false,
			isDev: true,
			importURLs: ["/a.js", "/a.js", "/b.js"],
			deps: ["/prod-only.js"],
			cssBundles: ["/a.css"],
		});

		expect(plan).toEqual({
			type: "preload",
			moduleDependenciesToPreload: ["/a.js", "/b.js"],
			cssBundlesToPreload: ["/a.css"],
			reason: "server_success_preload_allowed",
		});
	});

	it("uses production deps for non-dev preload module dependencies", () => {
		const plan = decideServerSuccessPreloadExecutionPlan({
			signalAborted: false,
			isDev: false,
			importURLs: ["/dev-only.js"],
			deps: ["/prod-a.js", "/prod-b.js"],
			cssBundles: ["/a.css", "/b.css"],
		});

		expect(plan).toEqual({
			type: "preload",
			moduleDependenciesToPreload: ["/prod-a.js", "/prod-b.js"],
			cssBundlesToPreload: ["/a.css", "/b.css"],
			reason: "server_success_preload_allowed",
		});
	});
});
