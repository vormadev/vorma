import { describe, expect, it, vi } from "vitest";
import {
	createDeferredFetchCall,
	createRouteDataResponse,
	createSignalCapturingNeverFetchSpy,
	loadClientAPI,
	setupContractTestSuite,
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

		firstFetchCall.deferred.reject(new DOMException("Aborted", "AbortError"));

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

	it("does not coalesce revalidation across data-target changes", async () => {
		const api = await loadClientAPI();
		window.history.replaceState({}, "", "/revalidate-a");

		const firstFetchCall = createDeferredFetchCall();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockImplementationOnce((_url, init) => {
				const promise = firstFetchCall.mock(_url, init);
				firstFetchCall.getSignal()?.addEventListener(
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

	it("upgrades a same-target revalidation into navigating status when user navigation starts", async () => {
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
			isNavigating: true,
			isSubmitting: false,
			isRevalidating: false,
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
