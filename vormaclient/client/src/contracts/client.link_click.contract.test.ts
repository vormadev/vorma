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

function createClickEvent(href: string, options?: MouseEventInit) {
	const event = new MouseEvent("click", { bubbles: true, ...options });
	const anchor = document.createElement("a");
	anchor.href = href;
	Object.defineProperty(event, "target", { value: anchor });
	return { event, anchor };
}

describe("client link click contracts", () => {
	it("prevents default for eligible internal links", async () => {
		const api = await loadClientAPI();
		vi.spyOn(window, "fetch").mockResolvedValue(createRouteDataResponse());

		const { event } = createClickEvent("/internal-link");
		const preventDefault = vi.spyOn(event, "preventDefault");

		const onClick = api.__makeLinkOnClickFn({});
		await onClick(event);
		await vi.runAllTimersAsync();

		expect(preventDefault).toHaveBeenCalledTimes(1);
	});

	it("does not prevent default for external links", async () => {
		const api = await loadClientAPI();

		const { event } = createClickEvent("https://external.com");
		const preventDefault = vi.spyOn(event, "preventDefault");

		const onClick = api.__makeLinkOnClickFn({});
		await onClick(event);

		expect(preventDefault).not.toHaveBeenCalled();
	});

	it("does not prevent default when modifier keys are used", async () => {
		const api = await loadClientAPI();

		const { event } = createClickEvent("/internal", { ctrlKey: true });
		const preventDefault = vi.spyOn(event, "preventDefault");

		const onClick = api.__makeLinkOnClickFn({});
		await onClick(event);

		expect(preventDefault).not.toHaveBeenCalled();
	});

	it("handles hash-only links without navigation fetch", async () => {
		const api = await loadClientAPI();
		window.history.replaceState({}, "", "/current-page");
		// saveScrollState expects history manager state to be initialized.
		api.getHistoryInstance();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValue(createRouteDataResponse());

		const { event } = createClickEvent("/current-page#section");
		const onClick = api.__makeLinkOnClickFn({});
		await onClick(event);

		expect(fetchSpy).not.toHaveBeenCalled();

		const scrollStateMapRaw = sessionStorage.getItem(
			"__vorma__scrollStateMap",
		);
		expect(scrollStateMapRaw).toBeTruthy();
		const scrollStateMap = JSON.parse(scrollStateMapRaw || "[]");
		expect(Array.isArray(scrollStateMap)).toBe(true);
	});
});
