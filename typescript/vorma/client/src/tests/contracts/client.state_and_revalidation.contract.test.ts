import { describe, expect, it, vi } from "vitest";
import {
	createDeferred,
	createDeferredFetchCall,
	createRouteDataResponse,
	createSequencedFetchSpy,
	createSignalCapturingNeverFetchSpy,
	loadClientAPI,
	setupContractTestSuite,
	stubWindowLocationHref,
} from "./contract_test_harness.ts";

setupContractTestSuite();

describe("client state/revalidation contracts", () => {
	it("aborts an in-flight user navigation when a new target is requested", async () => {
		const api = await loadClientAPI();
		const firstFetchCall = createDeferredFetchCall();
		const secondFetchPromise = Promise.resolve(createRouteDataResponse());

		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockImplementationOnce(firstFetchCall.mock)
			.mockImplementationOnce(() => secondFetchPromise as any);

		const firstNavigation = api.vormaNavigate("/state-first");
		await Promise.resolve();
		expect(api.getStatus().isNavigating).toBe(true);
		expect(firstFetchCall.getSignal()).toBeDefined();

		const secondNavigation = api.vormaNavigate("/state-second");
		expect(firstFetchCall.getSignal()?.aborted).toBe(true);

		firstFetchCall.deferred.reject(
			new DOMException("Aborted", "AbortError"),
		);

		await firstNavigation;
		await secondNavigation;
		await vi.runAllTimersAsync();

		expect(fetchSpy).toHaveBeenCalledTimes(2);
		expect(window.location.pathname).toBe("/state-second");
		expect(api.getStatus()).toEqual({
			isNavigating: false,
			isSubmitting: false,
			isRevalidating: false,
		});
	});

	it("can report navigating and revalidating simultaneously", async () => {
		const api = await loadClientAPI();
		let resolveNavigation: ((value: Response) => void) | undefined;
		let resolveRevalidation: ((value: Response) => void) | undefined;

		vi.spyOn(window, "fetch")
			.mockImplementationOnce(
				() =>
					new Promise<Response>((resolve) => {
						resolveNavigation = resolve;
					}) as any,
			)
			.mockImplementationOnce(
				() =>
					new Promise<Response>((resolve) => {
						resolveRevalidation = resolve;
					}) as any,
			);

		const navPromise = api.vormaNavigate("/status-mix");
		await vi.advanceTimersByTimeAsync(8);

		const revalidatePromise = api.revalidate();
		await vi.advanceTimersByTimeAsync(8);

		expect(api.getStatus()).toEqual({
			isNavigating: true,
			isSubmitting: false,
			isRevalidating: true,
		});

		resolveRevalidation?.(createRouteDataResponse());
		await revalidatePromise;
		await vi.advanceTimersByTimeAsync(8);
		expect(api.getStatus()).toEqual({
			isNavigating: true,
			isSubmitting: false,
			isRevalidating: false,
		});

		resolveNavigation?.(createRouteDataResponse());
		await navPromise;
		await vi.runAllTimersAsync();
		expect(api.getStatus()).toEqual({
			isNavigating: false,
			isSubmitting: false,
			isRevalidating: false,
		});
	});

	it("coalesces rapid revalidate calls into one fetch", async () => {
		const api = await loadClientAPI();
		let resolveFetch: ((value: Response) => void) | undefined;
		const fetchSpy = vi.spyOn(window, "fetch").mockImplementation(
			() =>
				new Promise<Response>((resolve) => {
					resolveFetch = resolve;
				}) as any,
		);

		const first = api.revalidate();
		const second = api.revalidate();
		const third = api.revalidate();

		expect(fetchSpy).toHaveBeenCalledTimes(1);

		resolveFetch?.(createRouteDataResponse());
		await Promise.all([first, second, third]);
		await vi.runAllTimersAsync();

		expect(api.getStatus().isRevalidating).toBe(false);
	});

	it("keeps one revalidation in-flight and runs at most one trailing pass for rapid repeated requests", async () => {
		const api = await loadClientAPI();
		const firstRevalidationFetch = createDeferred<Response>();
		const trailingRevalidationFetch = createDeferred<Response>();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockImplementationOnce(
				() => firstRevalidationFetch.promise as Promise<Response>,
			)
			.mockImplementationOnce(
				() => trailingRevalidationFetch.promise as Promise<Response>,
			);

		const first = api.revalidate();
		await vi.advanceTimersByTimeAsync(8);

		const second = api.revalidate();
		const third = api.revalidate();
		await vi.advanceTimersByTimeAsync(16);

		expect(fetchSpy).toHaveBeenCalledTimes(1);

		firstRevalidationFetch.resolve(
			createRouteDataResponse({
				title: { dangerousInnerHTML: "First Revalidation" },
			}),
		);
		await first;
		await vi.advanceTimersByTimeAsync(8);

		expect(fetchSpy).toHaveBeenCalledTimes(2);

		trailingRevalidationFetch.resolve(
			createRouteDataResponse({
				title: { dangerousInnerHTML: "Trailing Revalidation" },
			}),
		);
		await Promise.all([second, third]);
		await vi.runAllTimersAsync();

		expect(document.title).toBe("Trailing Revalidation");
		expect(api.getStatus()).toEqual({
			isNavigating: false,
			isSubmitting: false,
			isRevalidating: false,
		});
	});

	it("does not coalesce revalidation across data-target changes", async () => {
		const api = await loadClientAPI();
		window.history.replaceState({}, "", "/revalidate-a");

		const firstFetchCall = createDeferredFetchCall();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockImplementationOnce((_url, init) => {
				const promise = firstFetchCall.mock(_url, init);
				firstFetchCall
					.getSignal()
					?.addEventListener(
						"abort",
						() =>
							firstFetchCall.deferred.reject(
								new DOMException("Aborted", "AbortError"),
							),
						{ once: true },
					);
				return promise as any;
			})
			.mockImplementationOnce(
				() => Promise.resolve(createRouteDataResponse()) as any,
			);

		const firstRevalidate = api.revalidate();
		await vi.advanceTimersByTimeAsync(8);

		window.history.replaceState({}, "", "/revalidate-b");
		const secondRevalidate = api.revalidate();

		expect(firstFetchCall.getSignal()?.aborted).toBe(true);

		await Promise.all([firstRevalidate, secondRevalidate]);
		await vi.runAllTimersAsync();

		expect(fetchSpy).toHaveBeenCalledTimes(2);
		const secondFetchURL = fetchSpy.mock.calls[1]?.[0] as URL;
		expect(secondFetchURL.pathname).toBe("/revalidate-b");
		expect(secondFetchURL.searchParams.get("vorma_json")).toBe("1");
	});

	it("keeps same-target revalidation status when user navigation is a same-document no-op", async () => {
		const api = await loadClientAPI();
		let resolveFetch: ((value: Response) => void) | undefined;
		const fetchSpy = vi.spyOn(window, "fetch").mockImplementation(
			() =>
				new Promise<Response>((resolve) => {
					resolveFetch = resolve;
				}) as any,
		);

		const revalidatePromise = api.revalidate();
		await vi.advanceTimersByTimeAsync(8);
		expect(api.getStatus()).toEqual({
			isNavigating: false,
			isSubmitting: false,
			isRevalidating: true,
		});

		const navigatePromise = api.vormaNavigate(window.location.href);
		await vi.advanceTimersByTimeAsync(8);

		expect(fetchSpy).toHaveBeenCalledTimes(1);
		expect(api.getStatus()).toEqual({
			isNavigating: false,
			isSubmitting: false,
			isRevalidating: true,
		});

		resolveFetch?.(createRouteDataResponse());
		await Promise.all([revalidatePromise, navigatePromise]);
		await vi.runAllTimersAsync();

		expect(api.getStatus()).toEqual({
			isNavigating: false,
			isSubmitting: false,
			isRevalidating: false,
		});
	});

	it("revalidates against the current URL including search params", async () => {
		const api = await loadClientAPI();
		window.history.replaceState({}, "", "/current-page?param=value");
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValue(createRouteDataResponse());

		await api.revalidate();
		await vi.runAllTimersAsync();

		const fetchURL = fetchSpy.mock.calls[0]?.[0] as URL;
		expect(fetchURL.href).toBe(
			"http://localhost:3000/current-page?param=value&vorma_json=1",
		);
	});

	it("includes deployment query param on revalidation when deployment ID is configured", async () => {
		const api = await loadClientAPI();
		api.__vormaClientGlobal.set("deploymentID", "deploy-abc");
		window.history.replaceState({}, "", "/revalidate-target?tab=details");
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValue(createRouteDataResponse());

		await api.revalidate();
		await vi.runAllTimersAsync();

		const fetchURL = fetchSpy.mock.calls[0]?.[0] as URL;
		expect(fetchURL.pathname).toBe("/revalidate-target");
		expect(fetchURL.searchParams.get("tab")).toBe("details");
		expect(fetchURL.searchParams.get("vorma_json")).toBe("1");
		expect(fetchURL.searchParams.get("dpl")).toBe("deploy-abc");
	});

	it("follows redirects returned from revalidation when still relevant", async () => {
		const api = await loadClientAPI();
		vi.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse(
					{},
					{ headers: { "X-Client-Redirect": "/revalidate-login" } },
				),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse({
					title: { dangerousInnerHTML: "Revalidate Login" },
				}),
			);

		await api.revalidate();
		await vi.runAllTimersAsync();

		expect(window.location.pathname).toBe("/revalidate-login");
		expect(document.title).toBe("Revalidate Login");
		expect(api.getStatus()).toEqual({
			isNavigating: false,
			isSubmitting: false,
			isRevalidating: false,
		});
	});

	it("applies in-flight revalidation results across hash-only URL changes", async () => {
		const api = await loadClientAPI();
		let resolveFetch: ((value: Response) => void) | undefined;
		vi.spyOn(window, "fetch").mockImplementation(
			() =>
				new Promise<Response>((resolve) => {
					resolveFetch = resolve;
				}) as any,
		);

		const revalidatePromise = api.revalidate();
		await vi.advanceTimersByTimeAsync(8);

		window.history.replaceState({}, "", "/#details");
		resolveFetch?.(
			createRouteDataResponse({
				title: { dangerousInnerHTML: "Revalidated Title" },
			}),
		);

		await revalidatePromise;
		await vi.runAllTimersAsync();

		expect(document.title).toBe("Revalidated Title");
		expect(api.getStatus()).toEqual({
			isNavigating: false,
			isSubmitting: false,
			isRevalidating: false,
		});
	});

	it("treats search-param location changes as stale revalidation boundaries", async () => {
		window.history.replaceState({}, "", "/search-stale?tab=a");
		const api = await loadClientAPI();
		const revalidationFetch = createDeferredFetchCall();
		vi.spyOn(window, "fetch").mockImplementation(revalidationFetch.mock);

		const revalidatePromise = api.revalidate();
		await vi.advanceTimersByTimeAsync(8);

		window.history.replaceState({}, "", "/search-stale?tab=b");
		revalidationFetch.deferred.resolve(
			createRouteDataResponse({
				title: { dangerousInnerHTML: "Stale Search Result" },
			}),
		);

		await revalidatePromise;
		await vi.runAllTimersAsync();

		expect(window.location.pathname).toBe("/search-stale");
		expect(window.location.search).toBe("?tab=b");
		expect(document.title).toBe("Initial Title");
		expect(api.getStatus()).toEqual({
			isNavigating: false,
			isSubmitting: false,
			isRevalidating: false,
		});
	});

	it("ignores stale revalidation side effects after external location change", async () => {
		const api = await loadClientAPI();
		const revalidationFetch = createDeferredFetchCall();
		vi.spyOn(window, "fetch").mockImplementation(revalidationFetch.mock);
		const requestAnimationFrameSpy = vi.spyOn(
			window,
			"requestAnimationFrame",
		);

		const revalidatePromise = api.revalidate();
		await vi.advanceTimersByTimeAsync(8);

		window.history.replaceState({}, "", "/externally-changed");
		const rAFCallCountBeforeResolve =
			requestAnimationFrameSpy.mock.calls.length;

		revalidationFetch.deferred.resolve(
			createRouteDataResponse({
				title: { dangerousInnerHTML: "Stale External Title" },
				cssBundles: ["/stale-external.css"],
			}),
		);

		await revalidatePromise;
		await vi.advanceTimersByTimeAsync(32);
		await vi.runAllTimersAsync();

		expect(window.location.pathname).toBe("/externally-changed");
		expect(document.title).toBe("Initial Title");
		expect(requestAnimationFrameSpy).toHaveBeenCalledTimes(
			rAFCallCountBeforeResolve,
		);
		expect(
			document.head.querySelector(
				'link[data-vorma-css-bundle="/stale-external.css"]',
			),
		).toBeNull();
		expect(api.getStatus()).toEqual({
			isNavigating: false,
			isSubmitting: false,
			isRevalidating: false,
		});
	});

	it("does not follow stale revalidation redirects after external location change", async () => {
		const api = await loadClientAPI();
		const staleRedirectDeferred = createDeferred<Response>();
		const buildIDEvents: Array<{ newID: string; oldID: string }> = [];
		const removeBuildIDListener = api.addBuildIDListener((event) => {
			buildIDEvents.push(event.detail);
		});
		const { calls } = createSequencedFetchSpy([
			() => staleRedirectDeferred.promise,
			createRouteDataResponse({
				title: { dangerousInnerHTML: "Redirect Was Followed" },
			}),
		]);

		const revalidatePromise = api.revalidate();
		await vi.advanceTimersByTimeAsync(8);

		try {
			window.history.replaceState({}, "", "/externally-changed");

			staleRedirectDeferred.resolve(
				createRouteDataResponse(
					{},
					{
						headers: {
							"X-Client-Redirect": "/stale-redirect-target",
							"X-Vorma-Build-Id": "stale-soft-build",
						},
					},
				),
			);

			await revalidatePromise;
			await vi.runAllTimersAsync();

			expect(calls).toHaveLength(1);
			expect(window.location.pathname).toBe("/externally-changed");
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

	it("does not follow stale native revalidation redirects after external location change", async () => {
		const api = await loadClientAPI();
		const buildIDEvents: Array<{ newID: string; oldID: string }> = [];
		const removeBuildIDListener = api.addBuildIDListener((event) => {
			buildIDEvents.push(event.detail);
		});
		const nativeRedirectDeferred = createDeferred<Response>();

		const nativeRedirectResponse = createRouteDataResponse();
		Object.defineProperty(nativeRedirectResponse, "redirected", {
			value: true,
			configurable: true,
		});
		Object.defineProperty(nativeRedirectResponse, "url", {
			value: "http://localhost:3000/stale-native-revalidate-redirect",
			configurable: true,
		});

		const { calls } = createSequencedFetchSpy([
			() => nativeRedirectDeferred.promise,
			createRouteDataResponse({
				title: { dangerousInnerHTML: "Native Redirect Followed" },
			}),
		]);

		try {
			const revalidatePromise = api.revalidate();
			await vi.advanceTimersByTimeAsync(8);
			window.history.replaceState({}, "", "/externally-changed");
			nativeRedirectDeferred.resolve(nativeRedirectResponse);

			await revalidatePromise;
			await vi.runAllTimersAsync();

			expect(calls).toHaveLength(1);
			expect(window.location.pathname).toBe("/externally-changed");
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

	it("does not perform hard reload from stale revalidation responses", async () => {
		const api = await loadClientAPI();
		const revalidationFetch = createDeferredFetchCall();
		vi.spyOn(window, "fetch").mockImplementation(revalidationFetch.mock);
		const buildIDEvents: Array<{ newID: string; oldID: string }> = [];
		const removeBuildIDListener = api.addBuildIDListener((event) => {
			buildIDEvents.push(event.detail);
		});
		let restoreLocationHref: (() => void) | undefined;
		let getLocationHref: (() => string) | undefined;

		try {
			const revalidatePromise = api.revalidate();
			await vi.advanceTimersByTimeAsync(8);

			window.history.replaceState({}, "", "/externally-changed");
			const locationHrefStub = stubWindowLocationHref(
				window.location.href,
			);
			restoreLocationHref = locationHrefStub.restore;
			getLocationHref = locationHrefStub.getHref;

			revalidationFetch.deferred.resolve(
				createRouteDataResponse(
					{},
					{
						headers: {
							"X-Vorma-Reload": "/stale-hard-reload",
							"X-Vorma-Build-Id": "stale-hard-build",
						},
					},
				),
			);

			await revalidatePromise;
			await vi.runAllTimersAsync();
			const locationHref = getLocationHref?.() ?? "";

			expect(locationHref).not.toContain("/stale-hard-reload");
			expect(locationHref).toContain("/externally-changed");
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

	it("does not update build ID from stale revalidation responses", async () => {
		const api = await loadClientAPI();
		const revalidationFetch = createDeferredFetchCall();
		vi.spyOn(window, "fetch").mockImplementation(revalidationFetch.mock);
		const buildIDEvents: Array<{ newID: string; oldID: string }> = [];
		const removeBuildIDListener = api.addBuildIDListener((event) => {
			buildIDEvents.push(event.detail);
		});

		try {
			const revalidatePromise = api.revalidate();
			await vi.advanceTimersByTimeAsync(8);

			window.history.replaceState({}, "", "/externally-changed");

			revalidationFetch.deferred.resolve(
				createRouteDataResponse(
					{
						title: {
							dangerousInnerHTML: "Stale Build Update Attempt",
						},
					},
					{
						headers: {
							"X-Vorma-Build-Id": "stale-revalidation-build",
						},
					},
				),
			);

			await revalidatePromise;
			await vi.runAllTimersAsync();

			expect(api.getBuildID()).toBe("1");
			expect(buildIDEvents).toEqual([]);
			expect(window.location.pathname).toBe("/externally-changed");
		} finally {
			removeBuildIDListener();
		}
	});

	it("stopping a pure prefetch aborts it and allows a fresh prefetch", async () => {
		const api = await loadClientAPI();
		const { fetchSpy, signals } = createSignalCapturingNeverFetchSpy();

		const firstHandlers = api.__getPrefetchHandlers({
			href: "/prefetch-cleanup-contract",
			delayMs: 0,
		});
		firstHandlers?.start(new Event("mouseenter"));
		await vi.advanceTimersByTimeAsync(1);

		expect(fetchSpy).toHaveBeenCalledTimes(1);
		expect(signals[0]?.aborted).toBe(false);

		firstHandlers?.stop();
		expect(signals[0]?.aborted).toBe(true);

		const secondHandlers = api.__getPrefetchHandlers({
			href: "/prefetch-cleanup-contract",
			delayMs: 0,
		});
		secondHandlers?.start(new Event("mouseenter"));
		await vi.advanceTimersByTimeAsync(1);

		expect(fetchSpy).toHaveBeenCalledTimes(2);
		expect(signals[1]?.aborted).toBe(false);

		secondHandlers?.stop();
		expect(signals[1]?.aborted).toBe(true);
	});
});
