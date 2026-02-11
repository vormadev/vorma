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

	it("does not save scroll state for modifier-key same-document hash clicks", async () => {
		const api = await loadClientAPI();
		window.history.replaceState({}, "", "/current-page");
		api.getHistoryInstance();
		const initialScrollStateMap = sessionStorage.getItem(
			"__vorma__scrollStateMap",
		);

		const { event } = createClickEvent("/current-page#section", {
			ctrlKey: true,
		});
		const onClick = api.__makeLinkOnClickFn({});
		await onClick(event);

		expect(sessionStorage.getItem("__vorma__scrollStateMap")).toBe(
			initialScrollStateMap,
		);
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

	it("handles same-document hash removal links without navigation fetch", async () => {
		const api = await loadClientAPI();
		window.history.replaceState({}, "", "/current-page#section-a");
		// saveScrollState expects history manager state to be initialized.
		api.getHistoryInstance();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValue(createRouteDataResponse());

		const { event } = createClickEvent("/current-page");
		const preventDefault = vi.spyOn(event, "preventDefault");
		const onClick = api.__makeLinkOnClickFn({});
		await onClick(event);

		expect(preventDefault).not.toHaveBeenCalled();
		expect(fetchSpy).not.toHaveBeenCalled();

		const scrollStateMapRaw = sessionStorage.getItem(
			"__vorma__scrollStateMap",
		);
		expect(scrollStateMapRaw).toBeTruthy();
		const scrollStateMap = JSON.parse(scrollStateMapRaw || "[]");
		expect(Array.isArray(scrollStateMap)).toBe(true);
	});

	it("does not trigger navigation for same-document no-op hash links", async () => {
		const api = await loadClientAPI();
		window.history.replaceState({}, "", "/current-page#section-a");
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValue(createRouteDataResponse());
		const initialScrollStateMap = sessionStorage.getItem(
			"__vorma__scrollStateMap",
		);

		const { event } = createClickEvent("/current-page#section-a");
		const preventDefault = vi.spyOn(event, "preventDefault");
		const onClick = api.__makeLinkOnClickFn({});
		await onClick(event);

		expect(preventDefault).not.toHaveBeenCalled();
		expect(fetchSpy).not.toHaveBeenCalled();
		expect(sessionStorage.getItem("__vorma__scrollStateMap")).toBe(
			initialScrollStateMap,
		);
	});

	it("does not trigger navigation for encoding-equivalent hash links", async () => {
		const api = await loadClientAPI();
		window.history.replaceState({}, "", "/current-page#~");
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValue(createRouteDataResponse());
		const initialScrollStateMap = sessionStorage.getItem(
			"__vorma__scrollStateMap",
		);

		const { event } = createClickEvent("/current-page#%7E");
		const preventDefault = vi.spyOn(event, "preventDefault");
		const onClick = api.__makeLinkOnClickFn({});
		await onClick(event);

		expect(preventDefault).not.toHaveBeenCalled();
		expect(fetchSpy).not.toHaveBeenCalled();
		expect(sessionStorage.getItem("__vorma__scrollStateMap")).toBe(
			initialScrollStateMap,
		);
	});

	it("does not treat cross-origin hash links as same-document hash changes", async () => {
		const api = await loadClientAPI();
		window.history.replaceState({}, "", "/current-page");
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValue(createRouteDataResponse());
		const initialScrollStateMap = sessionStorage.getItem(
			"__vorma__scrollStateMap",
		);

		const { event } = createClickEvent(
			"https://external.com/current-page#x",
		);
		const preventDefault = vi.spyOn(event, "preventDefault");
		const onClick = api.__makeLinkOnClickFn({});
		await onClick(event);

		expect(preventDefault).not.toHaveBeenCalled();
		expect(fetchSpy).not.toHaveBeenCalled();
		expect(sessionStorage.getItem("__vorma__scrollStateMap")).toBe(
			initialScrollStateMap,
		);
	});

	it("updates build ID before following redirects triggered by link clicks", async () => {
		const api = await loadClientAPI();
		let buildIdDuringEvent: string | undefined;
		const cleanup = api.addBuildIDListener(() => {
			buildIdDuringEvent = api.getBuildID();
		});

		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse(
					{},
					{
						headers: {
							"X-Client-Redirect": "/redirected-click",
							"X-Vorma-Build-Id": "build-click-2",
						},
					},
				),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse(
					{},
					{ headers: { "X-Vorma-Build-Id": "build-click-2" } },
				),
			);

		const { event } = createClickEvent("/click-start");
		const onClick = api.__makeLinkOnClickFn({});
		await onClick(event);
		await vi.runAllTimersAsync();

		expect(fetchSpy).toHaveBeenCalledTimes(2);
		const secondFetchURL = fetchSpy.mock.calls[1]?.[0] as URL;
		expect(secondFetchURL.pathname).toBe("/redirected-click");
		expect(secondFetchURL.searchParams.get("vorma_json")).toBe(
			"build-click-2",
		);
		expect(buildIdDuringEvent).toBe("build-click-2");
		expect(api.getBuildID()).toBe("build-click-2");

		cleanup();
	});
});
