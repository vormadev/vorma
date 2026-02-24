import { describe, expect, it, vi } from "vitest";
import {
	createDeferredFetchCall,
	createRouteDataResponse,
	loadClientAPI,
	setupContractTestSuite,
	waitForRequestCount,
} from "./contract_test_harness.ts";

setupContractTestSuite();

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

	it("honors user onClick preventDefault by skipping internal navigation", async () => {
		const api = await loadClientAPI();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValue(createRouteDataResponse());
		const { event } = createClickEvent("/cancelled-navigation", {
			cancelable: true,
		});
		const consumerOnClick = vi.fn((clickEvent: MouseEvent) => {
			clickEvent.preventDefault();
		});
		const finalLinkProps = api.__makeFinalLinkProps({
			href: "/cancelled-navigation",
			onClick: consumerOnClick,
		} as any);

		await finalLinkProps.onClick(event);
		await vi.runAllTimersAsync();

		expect(consumerOnClick).toHaveBeenCalledTimes(1);
		expect(event.defaultPrevented).toBe(true);
		expect(fetchSpy).not.toHaveBeenCalled();
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

	it("does not prevent default for non-primary button clicks", async () => {
		const api = await loadClientAPI();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValue(createRouteDataResponse());

		const { event } = createClickEvent("/internal", {
			button: 2,
			cancelable: true,
		});
		const preventDefault = vi.spyOn(event, "preventDefault");

		const onClick = api.__makeLinkOnClickFn({});
		await onClick(event);

		expect(preventDefault).not.toHaveBeenCalled();
		expect(fetchSpy).not.toHaveBeenCalled();
	});

	it("does not prevent default for internal links targeting non-self browsing contexts", async () => {
		const api = await loadClientAPI();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValue(createRouteDataResponse());

		const { event, anchor } = createClickEvent("/internal", {
			cancelable: true,
		});
		anchor.target = "_BLANK";
		const preventDefault = vi.spyOn(event, "preventDefault");

		const onClick = api.__makeLinkOnClickFn({});
		await onClick(event);

		expect(preventDefault).not.toHaveBeenCalled();
		expect(fetchSpy).not.toHaveBeenCalled();
	});

	it("handles text-node click targets inside internal anchors", async () => {
		const api = await loadClientAPI();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValue(createRouteDataResponse());

		const anchor = document.createElement("a");
		anchor.href = "/text-node-target";
		const textNode = document.createTextNode("go");
		anchor.appendChild(textNode);
		document.body.appendChild(anchor);

		const event = new MouseEvent("click", {
			bubbles: true,
			cancelable: true,
		});
		Object.defineProperty(event, "target", { value: textNode });
		const preventDefault = vi.spyOn(event, "preventDefault");

		const onClick = api.__makeLinkOnClickFn({});
		await onClick(event);
		await vi.runAllTimersAsync();

		expect(preventDefault).toHaveBeenCalledTimes(1);
		expect(fetchSpy).toHaveBeenCalledTimes(1);
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

		expect(preventDefault).toHaveBeenCalledTimes(1);
		expect(fetchSpy).not.toHaveBeenCalled();
		expect(sessionStorage.getItem("__vorma__scrollStateMap")).toBe(
			initialScrollStateMap,
		);
	});

	it("prevents default for same-document no-op links without hash and does not fetch", async () => {
		const api = await loadClientAPI();
		window.history.replaceState({}, "", "/current-page");
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValue(createRouteDataResponse());

		const { event } = createClickEvent("/current-page");
		const preventDefault = vi.spyOn(event, "preventDefault");
		const onClick = api.__makeLinkOnClickFn({});
		await onClick(event);

		expect(preventDefault).toHaveBeenCalledTimes(1);
		expect(fetchSpy).not.toHaveBeenCalled();
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

		expect(preventDefault).toHaveBeenCalledTimes(1);
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

	it("cleans up aborted link outcomes when a newer navigation supersedes the click", async () => {
		const api = await loadClientAPI();
		const staleFetch = createDeferredFetchCall();
		let fetchCallCount = 0;
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockImplementation((url, init) => {
				fetchCallCount++;
				if (fetchCallCount === 1) {
					return staleFetch.mock(url, init);
				}
				return Promise.resolve(createRouteDataResponse());
			});
		const onClick = api.__makeLinkOnClickFn({});
		const { event } = createClickEvent("/aborted-click");

		const clickPromise = onClick(event);
		await waitForRequestCount({ requests: fetchSpy.mock.calls, count: 1 });

		const winningNavigation = api.vormaNavigate("/winner");
		await waitForRequestCount({
			requests: fetchSpy.mock.calls,
			count: 2,
		});
		await winningNavigation;

		staleFetch.deferred.resolve(createRouteDataResponse());
		await clickPromise;
		await vi.runAllTimersAsync();

		expect(fetchCallCount).toBe(2);
		expect(window.location.pathname).toBe("/winner");
	});

	it("does not throw and clears navigating state when link navigation fetch fails", async () => {
		const api = await loadClientAPI();
		vi.spyOn(window, "fetch").mockRejectedValue(
			new Error("network failed"),
		);

		const { event } = createClickEvent("/failing-click");
		const onClick = api.__makeLinkOnClickFn({});
		await expect(onClick(event)).resolves.toBeUndefined();
		await vi.runAllTimersAsync();

		const status = api.getStatus();
		expect(status.isNavigating).toBe(false);
	});
});
