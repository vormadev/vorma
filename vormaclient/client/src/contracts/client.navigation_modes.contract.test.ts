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

describe("client navigation mode contracts", () => {
	it("fetches route data for user navigation with vorma_json marker", async () => {
		const api = await loadClientAPI();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValue(createRouteDataResponse());

		await api.vormaNavigate("/user-nav");
		await vi.runAllTimersAsync();

		expect(fetchSpy).toHaveBeenCalledTimes(1);
		const fetchURL = fetchSpy.mock.calls[0]?.[0] as URL;
		expect(fetchURL.href).toContain("/user-nav?vorma_json=1");
	});

	it("uses browser history flow on POP updates", async () => {
		const api = await loadClientAPI();
		api.getHistoryInstance();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValue(createRouteDataResponse());

		window.history.replaceState({}, "", "/back-target");

		const { customHistoryListener } = await import("../history/history.ts");
		await customHistoryListener({
			action: "POP",
			location: {
				pathname: "/back-target",
				search: "",
				hash: "",
				state: null,
				key: "pop-key-1",
			},
		} as any);
		await vi.runAllTimersAsync();

		expect(fetchSpy).toHaveBeenCalledTimes(1);
		const fetchURL = fetchSpy.mock.calls[0]?.[0] as URL;
		expect(fetchURL.href).toContain("/back-target?vorma_json=1");
	});

	it("does not push or replace history during revalidation", async () => {
		const api = await loadClientAPI();
		vi.spyOn(window, "fetch").mockResolvedValue(createRouteDataResponse());

		const history = api.getHistoryInstance();
		const pushSpy = vi.spyOn(history, "push");
		const replaceSpy = vi.spyOn(history, "replace");

		await api.revalidate();
		await vi.runAllTimersAsync();

		expect(pushSpy).not.toHaveBeenCalled();
		expect(replaceSpy).not.toHaveBeenCalled();
	});

	it("follows server redirects and renders redirected destination", async () => {
		const api = await loadClientAPI();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse(
					{},
					{ headers: { "X-Client-Redirect": "/redirected" } },
				),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					title: { dangerousInnerHTML: "Redirected Page" },
				}),
			);

		await api.vormaNavigate("/original");
		await vi.runAllTimersAsync();

		expect(fetchSpy).toHaveBeenCalledTimes(2);
		const firstFetchURL = fetchSpy.mock.calls[0]?.[0] as URL;
		const secondFetchURL = fetchSpy.mock.calls[1]?.[0] as URL;
		expect(firstFetchURL.href).toContain("/original?vorma_json=1");
		expect(secondFetchURL.href).toContain("/redirected?vorma_json=1");
		expect(window.location.pathname).toBe("/redirected");
		expect(document.title).toBe("Redirected Page");
	});

	it("follows native fetch redirects for GET navigation requests", async () => {
		const api = await loadClientAPI();
		const nativeRedirectResponse = createRouteDataResponse();
		Object.defineProperty(nativeRedirectResponse, "redirected", {
			value: true,
			configurable: true,
		});
		Object.defineProperty(nativeRedirectResponse, "url", {
			value: "http://localhost:3000/native-get-redirect",
			configurable: true,
		});

		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValueOnce(nativeRedirectResponse)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					title: { dangerousInnerHTML: "Native GET Redirected Page" },
				}),
			);

		await api.vormaNavigate("/native-get-start");
		await vi.runAllTimersAsync();

		expect(fetchSpy).toHaveBeenCalledTimes(2);
		const secondFetchURL = fetchSpy.mock.calls[1]?.[0] as URL;
		expect(secondFetchURL.pathname).toBe("/native-get-redirect");
		expect(window.location.pathname).toBe("/native-get-redirect");
		expect(document.title).toBe("Native GET Redirected Page");
	});

	it("does not re-follow native redirects that land on an encoding-equivalent current hash target", async () => {
		window.history.replaceState({}, "", "/native-current#~");
		const api = await loadClientAPI();

		const nativeRedirectResponse = createRouteDataResponse();
		Object.defineProperty(nativeRedirectResponse, "redirected", {
			value: true,
			configurable: true,
		});
		Object.defineProperty(nativeRedirectResponse, "url", {
			value: "http://localhost:3000/native-current#%7E",
			configurable: true,
		});

		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValueOnce(nativeRedirectResponse);

		await api.vormaNavigate("/native-start");
		await vi.runAllTimersAsync();

		expect(fetchSpy).toHaveBeenCalledTimes(1);
		const firstFetchURL = fetchSpy.mock.calls[0]?.[0] as URL;
		expect(firstFetchURL.pathname).toBe("/native-start");
		expect(window.location.pathname).toBe("/native-current");
		expect(window.location.hash).toBe("#~");
	});

	it("does not emit loading status when running prefetch only", async () => {
		const api = await loadClientAPI();
		vi.spyOn(window, "fetch").mockResolvedValue(createRouteDataResponse());

		const statusEvents: Array<{
			isNavigating: boolean;
			isSubmitting: boolean;
			isRevalidating: boolean;
		}> = [];
		const removeStatusListener = api.addStatusListener((event) => {
			statusEvents.push(event.detail);
		});

		const handlers = api.__getPrefetchHandlers({ href: "/prefetch-only" });
		handlers?.start(new Event("mouseenter"));

		await vi.advanceTimersByTimeAsync(100);
		await vi.runAllTimersAsync();

		expect(statusEvents).toEqual([]);
		removeStatusListener();
	});

	it("uses history.replace when navigate is called with replace=true", async () => {
		const api = await loadClientAPI();
		vi.spyOn(window, "fetch").mockResolvedValue(createRouteDataResponse());

		const history = api.getHistoryInstance();
		const replaceSpy = vi.spyOn(history, "replace");
		const pushSpy = vi.spyOn(history, "push");

		await api.vormaNavigate("/replace-target", { replace: true });
		await vi.runAllTimersAsync();

		expect(replaceSpy).toHaveBeenCalled();
		expect(pushSpy).not.toHaveBeenCalled();
		expect(replaceSpy).toHaveBeenCalledWith(
			expect.stringContaining("/replace-target"),
			undefined,
		);
	});

	it("applies explicit search/hash overrides on programmatic navigation", async () => {
		const api = await loadClientAPI();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValue(createRouteDataResponse());

		await api.vormaNavigate("/destination", {
			search: "?q=contract",
			hash: "#anchor",
		});
		await vi.runAllTimersAsync();

		expect(fetchSpy).toHaveBeenCalledTimes(1);
		const fetchURL = fetchSpy.mock.calls[0]?.[0] as URL;
		expect(fetchURL.href).toContain("/destination?");
		expect(fetchURL.href).toContain("q=contract");
		expect(fetchURL.href).toContain("vorma_json=1");
		expect(window.location.search).toBe("?q=contract");
		expect(window.location.hash).toBe("#anchor");
	});

	it("reuses in-flight navigation when only hash changes on the same data target", async () => {
		const api = await loadClientAPI();
		let resolveFetch: ((value: Response) => void) | undefined;
		const fetchSpy = vi.spyOn(window, "fetch").mockImplementation(
			() =>
				new Promise<Response>((resolve) => {
					resolveFetch = resolve;
				}) as any,
		);

		const firstNavigation = api.vormaNavigate("/hash-only#first");
		await vi.advanceTimersByTimeAsync(8);

		const secondNavigation = api.vormaNavigate("/hash-only#second");
		await vi.advanceTimersByTimeAsync(8);

		expect(fetchSpy).toHaveBeenCalledTimes(1);
		expect(api.getStatus().isNavigating).toBe(true);

		resolveFetch?.(
			createRouteDataResponse({
				title: { dangerousInnerHTML: "Hash Stable Page" },
			}),
		);

		await Promise.all([firstNavigation, secondNavigation]);
		await vi.runAllTimersAsync();

		expect(window.location.pathname).toBe("/hash-only");
		expect(window.location.hash).toBe("#second");
		expect(document.title).toBe("Hash Stable Page");
		expect(api.getStatus()).toEqual({
			isNavigating: false,
			isSubmitting: false,
			isRevalidating: false,
		});
	});
});
