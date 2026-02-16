import { describe, expect, it, vi } from "vitest";
import {
	createAbortAwareFetchRecorder,
	createDeferred,
	createSequencedFetchSpy,
	createRouteDataResponse,
	loadClientAPI,
	setupContractTestSuite,
	stubWindowLocationHref,
	waitForRequestCount,
} from "./contract_test_harness.ts";

setupContractTestSuite();

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

		const { customHistoryListener } =
			await import("../../platform/history.ts");
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

	it("performs hard redirect for external navigation redirect targets", async () => {
		const api = await loadClientAPI();
		const locationHrefStub = stubWindowLocationHref();

		try {
			const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValueOnce(
				createRouteDataResponse(
					{},
					{
						headers: {
							"X-Client-Redirect": "https://external.example",
						},
					},
				),
			);

			await api.vormaNavigate("/external-nav-start");
			await vi.runAllTimersAsync();

			expect(fetchSpy).toHaveBeenCalledTimes(1);
			const redirectedURL = new URL(locationHrefStub.getHref());
			expect(redirectedURL.origin).toBe("https://external.example");
			expect(redirectedURL.pathname).toBe("/");
			expect(api.getStatus()).toEqual({
				isNavigating: false,
				isSubmitting: false,
				isRevalidating: false,
			});
		} finally {
			locationHrefStub.restore();
		}
	});

	it("performs hard reload redirect for navigation X-Vorma-Reload responses", async () => {
		const api = await loadClientAPI();
		const locationHrefStub = stubWindowLocationHref();

		try {
			const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValueOnce(
				createRouteDataResponse(
					{},
					{
						headers: {
							"X-Vorma-Reload": "/force-reload-nav",
							"X-Vorma-Build-Id": "reload-nav-build-1",
						},
					},
				),
			);

			await api.vormaNavigate("/hard-reload-nav-start");
			await vi.runAllTimersAsync();

			expect(fetchSpy).toHaveBeenCalledTimes(1);
			expect(locationHrefStub.getHref()).toContain("/force-reload-nav");
			expect(locationHrefStub.getHref()).toContain(
				"vorma_reload=reload-nav-build-1",
			);
			expect(api.getBuildID()).toBe("reload-nav-build-1");
			expect(api.getStatus()).toEqual({
				isNavigating: false,
				isSubmitting: false,
				isRevalidating: false,
			});
		} finally {
			locationHrefStub.restore();
		}
	});

	it("prioritizes X-Vorma-Reload over X-Client-Redirect for navigation", async () => {
		const api = await loadClientAPI();
		const locationHrefStub = stubWindowLocationHref();

		try {
			vi.spyOn(window, "fetch").mockResolvedValueOnce(
				createRouteDataResponse(
					{},
					{
						headers: {
							"X-Vorma-Reload": "/force-reload-nav-priority",
							"X-Client-Redirect": "/ignored-soft-nav",
							"X-Vorma-Build-Id": "priority-nav-build-1",
						},
					},
				),
			);

			await api.vormaNavigate("/priority-nav-start");
			await vi.runAllTimersAsync();

			expect(locationHrefStub.getHref()).toContain(
				"/force-reload-nav-priority",
			);
			expect(locationHrefStub.getHref()).toContain(
				"vorma_reload=priority-nav-build-1",
			);
			expect(locationHrefStub.getHref()).not.toContain(
				"/ignored-soft-nav",
			);
			expect(api.getBuildID()).toBe("priority-nav-build-1");
		} finally {
			locationHrefStub.restore();
		}
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

	it("does not let a stale browser-history POP completion override a newer user navigation", async () => {
		const api = await loadClientAPI();
		api.getHistoryInstance();
		const stalePOPFetch = createDeferred<Response>();
		const buildIDEvents: Array<{ newID: string; oldID: string }> = [];
		const removeBuildIDListener = api.addBuildIDListener((event) => {
			buildIDEvents.push(event.detail);
		});
		const { calls } = createSequencedFetchSpy([
			() => stalePOPFetch.promise,
			createRouteDataResponse({
				title: { dangerousInnerHTML: "POP Winner" },
			}),
		]);

		try {
			const { customHistoryListener } =
				await import("../../platform/history.ts");
			const stalePOPNavigation = customHistoryListener({
				action: "POP",
				location: {
					pathname: "/stale-pop-target",
					search: "",
					hash: "",
					state: null,
					key: "stale-pop-key",
				},
			} as any);
			await vi.advanceTimersByTimeAsync(8);

			await api.vormaNavigate("/pop-winner");
			await vi.runAllTimersAsync();

			stalePOPFetch.resolve(
				createRouteDataResponse(
					{
						title: { dangerousInnerHTML: "Stale POP Applied" },
					},
					{
						headers: {
							"X-Vorma-Build-Id": "stale-pop-build",
						},
					},
				),
			);
			await stalePOPNavigation;
			await vi.runAllTimersAsync();

			expect(calls).toHaveLength(2);
			expect(window.location.pathname).toBe("/pop-winner");
			expect(document.title).toBe("POP Winner");
			expect(api.getBuildID()).toBe("1");
			expect(buildIDEvents).toEqual([]);
			expect(api.getStatus()).toEqual({
				isNavigating: false,
				isSubmitting: false,
				isRevalidating: false,
			});
		} finally {
			removeBuildIDListener();
		}
	});

	it("does not let stale redirect-follow-up navigation override a newer user navigation", async () => {
		const api = await loadClientAPI();
		const { requests } = createAbortAwareFetchRecorder();

		const staleRedirectStart = api.vormaNavigate("/redirect-race-start");
		await waitForRequestCount({ requests, count: 1 });
		requests[0]?.resolve(
			createRouteDataResponse(
				{},
				{
					headers: {
						"X-Client-Redirect": "/redirect-race-target",
					},
				},
			),
		);

		await waitForRequestCount({ requests, count: 2 });
		const redirectFollowUpRequest = requests[1];
		expect(redirectFollowUpRequest?.signal?.aborted).toBe(false);

		const winnerNavigation = api.vormaNavigate("/redirect-race-winner");
		await waitForRequestCount({ requests, count: 3 });
		expect(redirectFollowUpRequest?.signal?.aborted).toBe(true);

		requests[2]?.resolve(
			createRouteDataResponse({
				title: { dangerousInnerHTML: "Redirect Race Winner" },
			}),
		);
		await winnerNavigation;

		redirectFollowUpRequest?.resolve(
			createRouteDataResponse({
				title: {
					dangerousInnerHTML: "Stale Redirect Follow-Up Applied",
				},
			}),
		);
		await staleRedirectStart;
		await vi.runAllTimersAsync();

		expect(window.location.pathname).toBe("/redirect-race-winner");
		expect(document.title).toBe("Redirect Race Winner");
		expect(api.getStatus()).toEqual({
			isNavigating: false,
			isSubmitting: false,
			isRevalidating: false,
		});
	});

	it("does not follow stale native redirects from an aborted user navigation", async () => {
		const api = await loadClientAPI();
		const staleNativeDeferred = createDeferred<Response>();
		const buildIDEvents: Array<{ newID: string; oldID: string }> = [];
		const removeBuildIDListener = api.addBuildIDListener((event) => {
			buildIDEvents.push(event.detail);
		});

		const staleNativeRedirectResponse = createRouteDataResponse(
			{},
			{
				headers: {
					"X-Vorma-Build-Id": "stale-native-nav-build",
				},
			},
		);
		Object.defineProperty(staleNativeRedirectResponse, "redirected", {
			value: true,
			configurable: true,
		});
		Object.defineProperty(staleNativeRedirectResponse, "url", {
			value: "http://localhost:3000/stale-native-nav-redirect",
			configurable: true,
		});

		const { calls } = createSequencedFetchSpy([
			() => staleNativeDeferred.promise,
			createRouteDataResponse({
				title: { dangerousInnerHTML: "Navigation Winner" },
			}),
			createRouteDataResponse({
				title: { dangerousInnerHTML: "Stale Redirect Was Followed" },
			}),
		]);

		try {
			const staleNavigation = api.vormaNavigate("/stale-native-start");
			await vi.advanceTimersByTimeAsync(8);

			const winnerNavigation = api.vormaNavigate("/winner-page");
			await winnerNavigation;
			await vi.runAllTimersAsync();

			staleNativeDeferred.resolve(staleNativeRedirectResponse);
			await staleNavigation;
			await vi.runAllTimersAsync();

			expect(calls).toHaveLength(2);
			expect(window.location.pathname).toBe("/winner-page");
			expect(document.title).toBe("Navigation Winner");
			expect(api.getBuildID()).toBe("1");
			expect(buildIDEvents).toEqual([]);
			expect(api.getStatus()).toEqual({
				isNavigating: false,
				isSubmitting: false,
				isRevalidating: false,
			});
		} finally {
			removeBuildIDListener();
		}
	});

	it("does not follow stale client-redirect headers from an aborted user navigation", async () => {
		const api = await loadClientAPI();
		const staleClientRedirectDeferred = createDeferred<Response>();
		const buildIDEvents: Array<{ newID: string; oldID: string }> = [];
		const removeBuildIDListener = api.addBuildIDListener((event) => {
			buildIDEvents.push(event.detail);
		});

		const { calls } = createSequencedFetchSpy([
			() => staleClientRedirectDeferred.promise,
			createRouteDataResponse({
				title: { dangerousInnerHTML: "Soft Redirect Winner" },
			}),
			createRouteDataResponse({
				title: {
					dangerousInnerHTML: "Stale Soft Redirect Was Followed",
				},
			}),
		]);

		try {
			const staleNavigation = api.vormaNavigate("/stale-soft-start");
			await vi.advanceTimersByTimeAsync(8);

			const winnerNavigation = api.vormaNavigate("/soft-winner");
			await winnerNavigation;
			await vi.runAllTimersAsync();

			staleClientRedirectDeferred.resolve(
				createRouteDataResponse(
					{},
					{
						headers: {
							"X-Client-Redirect": "/stale-soft-target",
							"X-Vorma-Build-Id": "stale-soft-nav-build",
						},
					},
				),
			);
			await staleNavigation;
			await vi.runAllTimersAsync();

			expect(calls).toHaveLength(2);
			expect(window.location.pathname).toBe("/soft-winner");
			expect(document.title).toBe("Soft Redirect Winner");
			expect(api.getBuildID()).toBe("1");
			expect(buildIDEvents).toEqual([]);
			expect(api.getStatus()).toEqual({
				isNavigating: false,
				isSubmitting: false,
				isRevalidating: false,
			});
		} finally {
			removeBuildIDListener();
		}
	});

	it("does not hard-reload from stale aborted user-navigation responses", async () => {
		const api = await loadClientAPI();
		const staleHardReloadDeferred = createDeferred<Response>();
		const buildIDEvents: Array<{ newID: string; oldID: string }> = [];
		const removeBuildIDListener = api.addBuildIDListener((event) => {
			buildIDEvents.push(event.detail);
		});
		let restoreLocationHref: (() => void) | undefined;
		let getLocationHref: (() => string) | undefined;

		const { calls } = createSequencedFetchSpy([
			() => staleHardReloadDeferred.promise,
			createRouteDataResponse({
				title: { dangerousInnerHTML: "Hard Reload Winner" },
			}),
		]);

		try {
			const staleNavigation = api.vormaNavigate("/stale-hard-start");
			await vi.advanceTimersByTimeAsync(8);

			const winnerNavigation = api.vormaNavigate("/hard-winner");
			await winnerNavigation;
			await vi.runAllTimersAsync();
			expect(window.location.pathname).toBe("/hard-winner");

			const locationHrefStub = stubWindowLocationHref(
				window.location.href,
			);
			restoreLocationHref = locationHrefStub.restore;
			getLocationHref = locationHrefStub.getHref;

			staleHardReloadDeferred.resolve(
				createRouteDataResponse(
					{},
					{
						headers: {
							"X-Vorma-Reload": "/stale-hard-redirect",
							"X-Vorma-Build-Id": "stale-hard-nav-build",
						},
					},
				),
			);
			await staleNavigation;
			await vi.runAllTimersAsync();
			const locationHref = getLocationHref?.() ?? "";

			expect(calls).toHaveLength(2);
			expect(locationHref).toContain("/hard-winner");
			expect(locationHref).not.toContain("/stale-hard-redirect");
			expect(api.getBuildID()).toBe("1");
			expect(buildIDEvents).toEqual([]);
			expect(api.getStatus()).toEqual({
				isNavigating: false,
				isSubmitting: false,
				isRevalidating: false,
			});
		} finally {
			removeBuildIDListener();
			restoreLocationHref?.();
		}
	});
});
