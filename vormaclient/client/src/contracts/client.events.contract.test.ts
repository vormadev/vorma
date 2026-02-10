import { describe, expect, it, vi } from "vitest";
import {
	createRouteDataResponse,
	setupContractTestSuite,
} from "./contract_test_harness.ts";

setupContractTestSuite();

async function loadClientAPI() {
	vi.resetModules();
	return import("../../index.ts");
}

describe("client events contracts", () => {
	it("fires location event when history location key changes", async () => {
		const api = await loadClientAPI();
		const locationListener = vi.fn();
		const cleanup = api.addLocationListener(locationListener);
		// Ensure HistoryManager has an initialized lastKnownLocation.
		api.getHistoryInstance();

		const { customHistoryListener } = await import("../history/history.ts");
		await customHistoryListener({
			action: "PUSH",
			location: {
				pathname: "/test",
				search: "",
				hash: "",
				state: {},
				key: "new-test-key-" + Date.now(),
			},
		} as any);

		expect(locationListener).toHaveBeenCalledTimes(1);
		cleanup();
	});

	it("fires build-id event with old/new IDs on navigation mismatch", async () => {
		const api = await loadClientAPI();
		const buildIdListener = vi.fn();
		const cleanup = api.addBuildIDListener(buildIdListener);

		vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse(
				{ importURLs: [], cssBundles: [] },
				{ headers: { "X-Vorma-Build-Id": "new-build-456" } },
			),
		);

		await api.vormaNavigate("/new-build");
		await vi.runAllTimersAsync();

		expect(buildIdListener).toHaveBeenCalledWith(
			expect.objectContaining({
				detail: {
					oldID: "1",
					newID: "new-build-456",
				},
			}),
		);

		cleanup();
	});

	it("updates getBuildID before dispatching build-id events", async () => {
		const api = await loadClientAPI();
		let buildIdDuringEvent: string | undefined;
		const cleanup = api.addBuildIDListener(() => {
			buildIdDuringEvent = api.getBuildID();
		});

		vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse(
				{ importURLs: [], cssBundles: [] },
				{ headers: { "X-Vorma-Build-Id": "new-build-789" } },
			),
		);

		await api.vormaNavigate("/build-update");
		await vi.runAllTimersAsync();

		expect(buildIdDuringEvent).toBe("new-build-789");
		expect(api.getBuildID()).toBe("new-build-789");

		cleanup();
	});
});
