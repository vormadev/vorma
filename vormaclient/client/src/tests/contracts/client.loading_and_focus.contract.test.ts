import { describe, expect, it, vi } from "vitest";
import {
	collectStatusEvents,
	createDeferred,
	createJSONResponse,
	createRouteDataResponse,
	createSequencedFetchSpy,
	expectNoLoadingGapBeforeFinalEvent,
	expectStatusIdle,
	installContractVormaGlobal,
	loadClientAPI,
	setupContractTestSuite,
} from "./contract_test_harness.ts";

setupContractTestSuite();

describe("client loading/focus contracts", () => {
	it("reports navigating status while navigation is in flight", async () => {
		const api = await loadClientAPI();
		const { statusEvents, cleanup: removeStatusListener } =
			collectStatusEvents(api);

		const fetchDeferred = createDeferred<Response>();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockReturnValue(fetchDeferred.promise as any);

		const navPromise = api.vormaNavigate("/contracts/navigation");

		expect(api.getStatus()).toEqual({
			isNavigating: true,
			isSubmitting: false,
			isRevalidating: false,
		});

		await vi.advanceTimersByTimeAsync(8);
		expect(statusEvents).toContainEqual({
			isNavigating: true,
			isSubmitting: false,
			isRevalidating: false,
		});

		fetchDeferred.resolve(createRouteDataResponse());
		await navPromise;
		await vi.runAllTimersAsync();

		expect(fetchSpy).toHaveBeenCalledTimes(1);
		expectStatusIdle(api.getStatus());
		expectStatusIdle(statusEvents.at(-1));

		removeStatusListener();
	});

	it("reports submitting status during submit() without auto-revalidate", async () => {
		const api = await loadClientAPI();
		const { statusEvents, cleanup: removeStatusListener } =
			collectStatusEvents(api);

		const submitDeferred = createDeferred<Response>();
		vi.spyOn(window, "fetch").mockReturnValue(
			submitDeferred.promise as any,
		);

		const submitPromise = api.submit<{ ok: boolean }>(
			"/api/save",
			{
				method: "POST",
				body: JSON.stringify({ any: "value" }),
				headers: { "Content-Type": "application/json" },
			},
			{ revalidate: false },
		);

		expect(api.getStatus()).toEqual({
			isNavigating: false,
			isSubmitting: true,
			isRevalidating: false,
		});
		await vi.advanceTimersByTimeAsync(8);
		expect(statusEvents.some((detail) => detail.isSubmitting)).toBe(true);

		submitDeferred.resolve(createJSONResponse({ ok: true }));

		const result = await submitPromise;
		await vi.runAllTimersAsync();

		expect(result).toEqual({ success: true, data: { ok: true } });
		expectStatusIdle(api.getStatus());

		removeStatusListener();
	});

	it("reports revalidating status while revalidate() is in flight", async () => {
		const api = await loadClientAPI();
		const { statusEvents, cleanup: removeStatusListener } =
			collectStatusEvents(api);

		const revalidateDeferred = createDeferred<Response>();
		vi.spyOn(window, "fetch").mockReturnValue(
			revalidateDeferred.promise as any,
		);

		const revalidatePromise = api.revalidate();

		await vi.advanceTimersByTimeAsync(8);
		expect(statusEvents).toContainEqual({
			isNavigating: false,
			isSubmitting: false,
			isRevalidating: true,
		});

		revalidateDeferred.resolve(createRouteDataResponse());
		await revalidatePromise;
		await vi.runAllTimersAsync();

		expectStatusIdle(api.getStatus());

		removeStatusListener();
	});

	it("debounces rapid status transitions into one event", async () => {
		const api = await loadClientAPI();
		const statusListener = vi.fn();
		const removeStatusListener = api.addStatusListener(statusListener);

		vi.spyOn(window, "fetch").mockImplementation(
			() => new Promise(() => {}),
		);

		api.vormaNavigate("/debounce-a");
		api.vormaNavigate("/debounce-b");

		expect(statusListener).not.toHaveBeenCalled();
		await vi.advanceTimersByTimeAsync(8);
		expect(statusListener).toHaveBeenCalledTimes(1);

		removeStatusListener();
	});

	it("does not dispatch duplicate status events for identical status", async () => {
		const api = await loadClientAPI();
		const statusListener = vi.fn();
		const removeStatusListener = api.addStatusListener(statusListener);

		vi.spyOn(window, "fetch").mockImplementation(
			() => new Promise(() => {}),
		);

		api.vormaNavigate("/same-a");
		await vi.advanceTimersByTimeAsync(8);
		const callCount = statusListener.mock.calls.length;

		api.vormaNavigate("/same-b");
		await vi.advanceTimersByTimeAsync(8);
		expect(statusListener).toHaveBeenCalledTimes(callCount);

		removeStatusListener();
	});

	it("provides synchronous status from getStatus()", async () => {
		const api = await loadClientAPI();
		expect(api.getStatus()).toEqual({
			isNavigating: false,
			isSubmitting: false,
			isRevalidating: false,
		});

		vi.spyOn(window, "fetch").mockImplementation(
			() => new Promise(() => {}),
		);
		api.vormaNavigate("/sync-status");

		expect(api.getStatus().isNavigating).toBe(true);
	});

	it("keeps at least one loading state active across submit to auto-revalidate", async () => {
		const api = await loadClientAPI();
		const { statusEvents, cleanup: removeStatusListener } =
			collectStatusEvents(api);

		const submitDeferred = createDeferred<Response>();
		const revalidateDeferred = createDeferred<Response>();

		createSequencedFetchSpy([
			() => submitDeferred.promise,
			() => revalidateDeferred.promise,
		]);

		const submitPromise = api.submit<{ ok: boolean }>("/api/action", {
			method: "POST",
		});

		await vi.advanceTimersByTimeAsync(8);
		expect(api.getStatus()).toEqual({
			isNavigating: false,
			isSubmitting: true,
			isRevalidating: false,
		});

		submitDeferred.resolve(createJSONResponse({ ok: true }));

		await Promise.resolve();
		await vi.advanceTimersByTimeAsync(8);
		expect(api.getStatus().isRevalidating).toBe(true);

		revalidateDeferred.resolve(createRouteDataResponse());
		await submitPromise;
		await vi.runAllTimersAsync();

		expectNoLoadingGapBeforeFinalEvent(statusEvents);
		expectStatusIdle(statusEvents.at(-1));

		removeStatusListener();
	});

	it("keeps submitting state continuous for overlapping submissions", async () => {
		const api = await loadClientAPI();
		const { statusEvents, cleanup: removeStatusListener } =
			collectStatusEvents(api);

		const firstSubmitDeferred = createDeferred<Response>();
		const secondSubmitDeferred = createDeferred<Response>();

		createSequencedFetchSpy([
			() => firstSubmitDeferred.promise,
			() => secondSubmitDeferred.promise,
		]);

		const firstSubmit = api.submit<{ ok: boolean }>(
			"/api/first",
			{ method: "POST" },
			{ revalidate: false },
		);
		await vi.advanceTimersByTimeAsync(8);
		expect(api.getStatus().isSubmitting).toBe(true);

		const secondSubmit = api.submit<{ ok: boolean }>(
			"/api/second",
			{ method: "POST" },
			{ revalidate: false },
		);

		firstSubmitDeferred.resolve(createJSONResponse({ ok: true }));

		await firstSubmit;
		await vi.advanceTimersByTimeAsync(8);
		expect(api.getStatus().isSubmitting).toBe(true);

		secondSubmitDeferred.resolve(createJSONResponse({ ok: true }));

		await secondSubmit;
		await vi.runAllTimersAsync();

		expectNoLoadingGapBeforeFinalEvent(statusEvents);
		expectStatusIdle(statusEvents.at(-1));

		removeStatusListener();
	});

	it("keeps navigating state continuous through soft redirect handoff", async () => {
		const api = await loadClientAPI();
		const { statusEvents, cleanup: removeStatusListener } =
			collectStatusEvents(api);

		const firstFetchDeferred = createDeferred<Response>();
		const secondFetchDeferred = createDeferred<Response>();

		createSequencedFetchSpy([
			() => firstFetchDeferred.promise,
			() => secondFetchDeferred.promise,
		]);

		const navPromise = api.vormaNavigate("/dashboard");

		await vi.advanceTimersByTimeAsync(8);
		expect(api.getStatus().isNavigating).toBe(true);

		firstFetchDeferred.resolve(
			createRouteDataResponse(
				{},
				{ headers: { "X-Client-Redirect": "/login" } },
			),
		);
		await Promise.resolve();
		await vi.advanceTimersByTimeAsync(8);
		expect(api.getStatus().isNavigating).toBe(true);

		secondFetchDeferred.resolve(
			createRouteDataResponse({ title: { dangerousInnerHTML: "Login" } }),
		);
		await navPromise;
		await vi.runAllTimersAsync();

		expectNoLoadingGapBeforeFinalEvent(statusEvents);
		expectStatusIdle(statusEvents.at(-1));

		removeStatusListener();
	});

	it("keeps navigating state continuous through redirect chains", async () => {
		const api = await loadClientAPI();
		const { statusEvents, cleanup: removeStatusListener } =
			collectStatusEvents(api);

		const { fetchSpy } = createSequencedFetchSpy([
			createRouteDataResponse(
				{},
				{ headers: { "X-Client-Redirect": "/auth" } },
			),
			createRouteDataResponse(
				{},
				{ headers: { "X-Client-Redirect": "/login" } },
			),
			createRouteDataResponse({
				matchedPatterns: ["/login"],
				title: { dangerousInnerHTML: "Login Page" },
			}),
		]);

		await api.vormaNavigate("/admin");
		await vi.runAllTimersAsync();

		expect(fetchSpy).toHaveBeenCalledTimes(3);
		expectNoLoadingGapBeforeFinalEvent(statusEvents);
		expectStatusIdle(statusEvents.at(-1));

		removeStatusListener();
	});

	it("keeps loading continuous through a complete navigation render cycle", async () => {
		const api = await loadClientAPI();
		const { statusEvents, cleanup: removeStatusListener } =
			collectStatusEvents(api);
		let routeChangeCount = 0;

		const removeRouteChangeListener = api.addRouteChangeListener(() => {
			routeChangeCount++;
		});

		vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				matchedPatterns: ["/complex-page"],
				title: { dangerousInnerHTML: "Complex Page" },
			}),
		);

		expect(api.getStatus().isNavigating).toBe(false);
		const navPromise = api.vormaNavigate("/complex-page");
		expect(api.getStatus().isNavigating).toBe(true);

		await navPromise;
		await vi.runAllTimersAsync();

		expect(routeChangeCount).toBe(1);
		expectNoLoadingGapBeforeFinalEvent(statusEvents);
		expectStatusIdle(api.getStatus());

		removeRouteChangeListener();
		removeStatusListener();
	});

	it("includes hash scroll state in route-change events", async () => {
		const api = await loadClientAPI();
		const routeChangeListener = vi.fn();
		const removeRouteChangeListener =
			api.addRouteChangeListener(routeChangeListener);

		vi.spyOn(window, "fetch").mockResolvedValue(createRouteDataResponse());

		await api.vormaNavigate("/with-hash#section");
		await vi.runAllTimersAsync();

		expect(routeChangeListener).toHaveBeenCalledWith(
			expect.objectContaining({
				detail: expect.objectContaining({
					__scrollState: { hash: "section" },
				}),
			}),
		);

		removeRouteChangeListener();
	});

	it("dispatches top scroll state in route-change events for standard navigation", async () => {
		const api = await loadClientAPI();
		const routeChangeListener = vi.fn();
		const removeRouteChangeListener =
			api.addRouteChangeListener(routeChangeListener);

		vi.spyOn(window, "fetch").mockResolvedValue(createRouteDataResponse());

		await api.vormaNavigate("/scroll-top-target");
		await vi.runAllTimersAsync();

		expect(routeChangeListener).toHaveBeenCalledWith(
			expect.objectContaining({
				detail: expect.objectContaining({
					__scrollState: { x: 0, y: 0 },
				}),
			}),
		);

		removeRouteChangeListener();
	});

	it("dispatches route-change after title updates", async () => {
		const api = await loadClientAPI();
		const routeChangeListener = vi.fn();
		const removeRouteChangeListener =
			api.addRouteChangeListener(routeChangeListener);

		vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				title: { dangerousInnerHTML: "New Title" },
			}),
		);

		await api.vormaNavigate("/after-title");
		await vi.runAllTimersAsync();

		expect(routeChangeListener).toHaveBeenCalled();
		expect(document.title).toBe("New Title");

		removeRouteChangeListener();
	});

	it("keeps navigating while waiting for CSS preload completion", async () => {
		const api = await loadClientAPI();
		const { statusEvents, cleanup: removeStatusListener } =
			collectStatusEvents(api);

		let cssLoadCallback: (() => void) | undefined;
		const originalCreateElement = document.createElement.bind(document);
		vi.spyOn(document, "createElement").mockImplementation((tagName) => {
			const element = originalCreateElement(tagName);
			if (tagName === "link") {
				const link = element as HTMLLinkElement;
				Object.defineProperty(link, "onload", {
					configurable: true,
					get() {
						return null;
					},
					set(callback) {
						if (callback && link.rel === "preload") {
							cssLoadCallback = callback as () => void;
						}
					},
				});
			}
			return element;
		});

		const fetchDeferred = createDeferred<Response>();
		vi.spyOn(window, "fetch").mockReturnValue(fetchDeferred.promise as any);

		const navPromise = api.vormaNavigate("/styled-page");
		fetchDeferred.resolve(
			createRouteDataResponse({ cssBundles: ["/style1.css"] }),
		);

		await vi.advanceTimersByTimeAsync(10);
		expect(api.getStatus().isNavigating).toBe(true);
		expect(cssLoadCallback).toBeDefined();

		cssLoadCallback?.();
		await navPromise;
		await vi.runAllTimersAsync();

		expectNoLoadingGapBeforeFinalEvent(statusEvents);
		expect(api.getStatus().isNavigating).toBe(false);

		removeStatusListener();
	});

	it("keeps navigating while waiting for client loader completion", async () => {
		const api = await loadClientAPI();
		const { statusEvents, cleanup: removeStatusListener } =
			collectStatusEvents(api);

		const waitFnDeferred = createDeferred<{ clientData: string }>();
		const symbol = Symbol.for("__vorma_internal__");
		const currentGlobal = (globalThis as any)[symbol];
		installContractVormaGlobal({
			...currentGlobal,
			patternToWaitFnMap: {
				"/data-page": () => waitFnDeferred.promise,
			},
		});

		const fetchDeferred = createDeferred<Response>();
		vi.spyOn(window, "fetch").mockReturnValue(fetchDeferred.promise as any);

		const navPromise = api.vormaNavigate("/data-page");
		fetchDeferred.resolve(
			createRouteDataResponse({
				matchedPatterns: ["/data-page"],
				loadersData: [{}],
			}),
		);

		await vi.advanceTimersByTimeAsync(10);
		expect(api.getStatus().isNavigating).toBe(true);

		waitFnDeferred.resolve({ clientData: "loaded" });
		await navPromise;
		await vi.runAllTimersAsync();

		expectNoLoadingGapBeforeFinalEvent(statusEvents);
		expect(api.getStatus().isNavigating).toBe(false);

		removeStatusListener();
	});

	it("starts and stops the global loading indicator around navigation", async () => {
		const api = await loadClientAPI();
		let running = false;
		const start = vi.fn(() => {
			running = true;
		});
		const stop = vi.fn(() => {
			running = false;
		});

		const cleanupIndicator = api.setupGlobalLoadingIndicator({
			start,
			stop,
			isRunning: () => running,
			startDelayMS: 5,
			stopDelayMS: 5,
		});

		const navDeferred = createDeferred<Response>();
		vi.spyOn(window, "fetch").mockReturnValue(navDeferred.promise as any);

		const navPromise = api.vormaNavigate("/contracts/loading-indicator");

		await vi.advanceTimersByTimeAsync(13);
		expect(start).toHaveBeenCalledTimes(1);
		expect(running).toBe(true);

		navDeferred.resolve(createRouteDataResponse());
		await navPromise;
		await vi.runAllTimersAsync();

		expect(stop).toHaveBeenCalledTimes(1);
		expect(running).toBe(false);

		cleanupIndicator();
	});

	it("respects global loading indicator include filters", async () => {
		const api = await loadClientAPI();
		let running = false;
		const start = vi.fn(() => {
			running = true;
		});
		const stop = vi.fn(() => {
			running = false;
		});

		const cleanupIndicator = api.setupGlobalLoadingIndicator({
			start,
			stop,
			isRunning: () => running,
			include: ["submissions"],
			startDelayMS: 0,
			stopDelayMS: 0,
		});

		const submitDeferred = createDeferred<Response>();
		const { fetchSpy } = createSequencedFetchSpy([
			createRouteDataResponse(),
			() => submitDeferred.promise,
		]);

		await api.revalidate();
		await vi.runAllTimersAsync();
		expect(start).not.toHaveBeenCalled();

		const submitPromise = api.submit(
			"/api/loading-indicator",
			{ method: "POST" },
			{ revalidate: false },
		);

		await vi.advanceTimersByTimeAsync(8);
		await vi.runAllTimersAsync();
		expect(start).toHaveBeenCalledTimes(1);
		expect(running).toBe(true);

		submitDeferred.resolve(createJSONResponse({ ok: true }));

		await submitPromise;
		await vi.runAllTimersAsync();

		expect(fetchSpy).toHaveBeenCalledTimes(2);
		expect(stop).toHaveBeenCalledTimes(1);
		expect(running).toBe(false);

		cleanupIndicator();
	});

	it("clears global loading indicator timers when timer id is zero", async () => {
		const api = await loadClientAPI();
		const originalSetTimeout = window.setTimeout.bind(window);
		const setTimeoutSpy = vi
			.spyOn(window, "setTimeout")
			.mockImplementationOnce(() => 0 as any)
			.mockImplementation(
				(handler, timeout, ...args) =>
					originalSetTimeout(
						handler as TimerHandler,
						timeout,
						...args,
					) as any,
			);
		const clearTimeoutSpy = vi.spyOn(window, "clearTimeout");

		const cleanupIndicator = api.setupGlobalLoadingIndicator({
			start: vi.fn(),
			stop: vi.fn(),
			isRunning: () => false,
			startDelayMS: 5,
			stopDelayMS: 5,
		});

		cleanupIndicator();

		expect(setTimeoutSpy).toHaveBeenCalled();
		expect(clearTimeoutSpy).toHaveBeenCalledWith(0);
	});

	it("revalidates on focus after staleTime has elapsed", async () => {
		const api = await loadClientAPI();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValue(createRouteDataResponse());

		const cleanup = api.revalidateOnWindowFocus({ staleTimeMS: 50 });

		await vi.advanceTimersByTimeAsync(51);
		window.dispatchEvent(new Event("focus"));
		await vi.advanceTimersByTimeAsync(30);
		await vi.runAllTimersAsync();

		expect(fetchSpy).toHaveBeenCalledTimes(1);
		const firstURL = fetchSpy.mock.calls[0]?.[0] as URL;
		expect(firstURL.href).toContain("vorma_json=1");

		cleanup();
	});

	it("does not revalidate on focus while navigation is active", async () => {
		const api = await loadClientAPI();
		const navDeferred = createDeferred<Response>();
		const { fetchSpy } = createSequencedFetchSpy([
			() => navDeferred.promise,
			createRouteDataResponse(),
		]);

		const navPromise = api.vormaNavigate("/contracts/busy");
		expect(api.getStatus().isNavigating).toBe(true);

		const cleanup = api.revalidateOnWindowFocus({ staleTimeMS: 0 });
		window.dispatchEvent(new Event("focus"));
		await vi.advanceTimersByTimeAsync(30);

		expect(fetchSpy).toHaveBeenCalledTimes(1);

		navDeferred.resolve(createRouteDataResponse());
		await navPromise;
		await vi.runAllTimersAsync();

		cleanup();
	});

	it("stops listening after cleanup from revalidateOnWindowFocus", async () => {
		const api = await loadClientAPI();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValue(createRouteDataResponse());

		const cleanup = api.revalidateOnWindowFocus({ staleTimeMS: 0 });
		cleanup();

		window.dispatchEvent(new Event("focus"));
		await vi.advanceTimersByTimeAsync(30);

		expect(fetchSpy).not.toHaveBeenCalled();
	});
});
