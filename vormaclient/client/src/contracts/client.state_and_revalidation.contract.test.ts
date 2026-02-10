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

describe("client state/revalidation contracts", () => {
	it("aborts an in-flight user navigation when a new target is requested", async () => {
		const api = await loadClientAPI();
		let rejectFirstFetch: ((reason?: unknown) => void) | undefined;
		const secondFetchPromise = Promise.resolve(createRouteDataResponse());

		let firstSignal: AbortSignal | undefined;
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockImplementationOnce((_url, init) => {
				firstSignal = (init as RequestInit | undefined)
					?.signal as AbortSignal;
				return new Promise<Response>((_resolve, reject) => {
					rejectFirstFetch = reject;
				}) as any;
			})
			.mockImplementationOnce(() => secondFetchPromise as any);

		const firstNavigation = api.vormaNavigate("/state-first");
		await Promise.resolve();
		expect(api.getStatus().isNavigating).toBe(true);
		expect(firstSignal).toBeDefined();

		const secondNavigation = api.vormaNavigate("/state-second");
		expect(firstSignal?.aborted).toBe(true);

		rejectFirstFetch?.(new DOMException("Aborted", "AbortError"));

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

	it("stopping a pure prefetch aborts it and allows a fresh prefetch", async () => {
		const api = await loadClientAPI();
		const signals: AbortSignal[] = [];
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockImplementation((_url, init) => {
				const signal = (init as RequestInit | undefined)
					?.signal as AbortSignal;
				if (signal) {
					signals.push(signal);
				}
				return new Promise<Response>(() => {}) as any;
			});

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
