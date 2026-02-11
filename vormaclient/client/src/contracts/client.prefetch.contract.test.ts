import { describe, expect, it, vi } from "vitest";
import {
	createDeferred,
	createRouteDataResponse,
	setupContractTestSuite,
} from "./contract_test_harness.ts";

setupContractTestSuite();

async function loadClientAPI() {
	vi.resetModules();
	return import("../../index.ts");
}

function buildAnchorClick(href: string): {
	anchor: HTMLAnchorElement;
	event: MouseEvent;
} {
	const anchor = document.createElement("a");
	anchor.href = href;
	document.body.appendChild(anchor);

	const event = new MouseEvent("click", {
		bubbles: true,
		cancelable: true,
	});
	Object.defineProperty(event, "target", { value: anchor });

	return { anchor, event };
}

describe("client prefetch contracts", () => {
	it("creates prefetch handlers only for eligible internal HTTP links", async () => {
		const api = await loadClientAPI();

		const internalHandlers = api.__getPrefetchHandlers({
			href: "/internal",
		});
		expect(internalHandlers).toBeDefined();

		const externalHandlers = api.__getPrefetchHandlers({
			href: "https://external.com",
		});
		expect(externalHandlers).toBeUndefined();

		const mailtoHandlers = api.__getPrefetchHandlers({
			href: "mailto:test@example.com",
		});
		expect(mailtoHandlers).toBeUndefined();
	});

	it("does not prefetch when target is the current page", async () => {
		const api = await loadClientAPI();
		window.history.replaceState({}, "", "/current-page");
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValue(createRouteDataResponse());

		const handlers = api.__getPrefetchHandlers({ href: "/current-page" });
		handlers?.start(new Event("mouseenter"));

		await vi.advanceTimersByTimeAsync(200);
		expect(fetchSpy).not.toHaveBeenCalled();
	});

	it("starts prefetch only after the configured delay", async () => {
		const api = await loadClientAPI();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValue(createRouteDataResponse());

		const handlers = api.__getPrefetchHandlers({
			href: "/delayed-prefetch",
			delayMs: 200,
		});

		handlers?.start(new Event("mouseenter"));

		await vi.advanceTimersByTimeAsync(100);
		expect(fetchSpy).not.toHaveBeenCalled();

		await vi.advanceTimersByTimeAsync(100);
		expect(fetchSpy).toHaveBeenCalledTimes(1);
	});

	it("runs beforeBegin callback before prefetch starts", async () => {
		const api = await loadClientAPI();
		const beforeBegin = vi.fn();
		vi.spyOn(window, "fetch").mockResolvedValue(createRouteDataResponse());

		const handlers = api.__getPrefetchHandlers({
			href: "/callback-test",
			beforeBegin,
		});

		handlers?.start(new Event("mouseenter"));
		await vi.advanceTimersByTimeAsync(100);

		expect(beforeBegin).toHaveBeenCalledTimes(1);
	});

	it("reuses completed prefetch response on click without refetching", async () => {
		const api = await loadClientAPI();
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				title: { dangerousInnerHTML: "Prefetched" },
			}),
		);

		const handlers = api.__getPrefetchHandlers({ href: "/store-result" });
		handlers?.start(new Event("mouseenter"));
		await vi.advanceTimersByTimeAsync(100);
		await vi.runAllTimersAsync();

		fetchSpy.mockClear();

		const { anchor, event } = buildAnchorClick("/store-result");
		await handlers?.onClick(event);
		await vi.runAllTimersAsync();

		expect(fetchSpy).not.toHaveBeenCalled();
		expect(document.title).toBe("Prefetched");

		document.body.removeChild(anchor);
	});

	it("cancels pending prefetch timeout when stop is called", async () => {
		const api = await loadClientAPI();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValue(createRouteDataResponse());

		const handlers = api.__getPrefetchHandlers({ href: "/cancel-timeout" });
		handlers?.start(new Event("mouseenter"));
		handlers?.stop();

		await vi.advanceTimersByTimeAsync(200);
		expect(fetchSpy).not.toHaveBeenCalled();
	});

	it("clears pending prefetch timer even when timer id is zero", async () => {
		const api = await loadClientAPI();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValue(createRouteDataResponse());
		const setTimeoutSpy = vi
			.spyOn(window, "setTimeout")
			.mockImplementation(() => 0 as any);
		const clearTimeoutSpy = vi.spyOn(window, "clearTimeout");

		const handlers = api.__getPrefetchHandlers({
			href: "/zero-timer-stop",
		});
		handlers?.start(new Event("mouseenter"));
		handlers?.stop();

		expect(clearTimeoutSpy).toHaveBeenCalledWith(0);
		await vi.advanceTimersByTimeAsync(200);
		expect(fetchSpy).not.toHaveBeenCalled();

		setTimeoutSpy.mockRestore();
		clearTimeoutSpy.mockRestore();
	});

	it("aborts in-flight prefetch with search/hash overrides when stop is called", async () => {
		const api = await loadClientAPI();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockImplementation(() => new Promise(() => {}) as any);

		const handlers = api.__getPrefetchHandlers({
			href: "/abort-with-overrides",
			search: "?mode=preview",
			hash: "#details",
			delayMs: 0,
		});
		handlers?.start(new Event("mouseenter"));
		await vi.advanceTimersByTimeAsync(1);

		expect(fetchSpy).toHaveBeenCalledTimes(1);
		const signal = fetchSpy.mock.calls[0]?.[1]?.signal;
		expect(signal?.aborted).toBe(false);

		handlers?.stop();
		expect(signal?.aborted).toBe(true);
	});

	it("does not abort upgraded navigation when prefetch handlers stop", async () => {
		const api = await loadClientAPI();
		const fetchDeferred = createDeferred<Response>();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockReturnValue(fetchDeferred.promise as any);

		const handlers = api.__getPrefetchHandlers({ href: "/abort-test" });
		handlers?.start(new Event("mouseenter"));
		await vi.advanceTimersByTimeAsync(100);

		expect(fetchSpy).toHaveBeenCalledTimes(1);
		const signal = fetchSpy.mock.calls[0]?.[1]?.signal;
		expect(signal?.aborted).toBe(false);

		const navPromise = api.vormaNavigate("/abort-test");
		handlers?.stop();

		expect(signal?.aborted).toBe(false);
		expect(api.getStatus().isNavigating).toBe(true);
		expect(fetchSpy).toHaveBeenCalledTimes(1);

		fetchDeferred.resolve(createRouteDataResponse());
		await navPromise;
		await vi.runAllTimersAsync();
	});

	it("does not leak unhandled rejections when pure prefetch resolves to redirect", async () => {
		const api = await loadClientAPI();
		const patternToWaitFnMap =
			api.__vormaClientGlobal.get("patternToWaitFnMap");

		patternToWaitFnMap["/prefetch-redirect"] = async ({
			serverDataPromise,
		}: {
			serverDataPromise: Promise<{ loaderData: { Title: string } }>;
		}) => {
			const { loaderData } = await serverDataPromise;
			return loaderData.Title;
		};
		await api.__registerClientLoaderPattern("/prefetch-redirect");

		vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse(
				{},
				{ headers: { "X-Client-Redirect": "/redirect-target" } },
			),
		);

		const unhandledRejections: Array<unknown> = [];
		const unhandledRejectionHandler = (reason: unknown) => {
			unhandledRejections.push(reason);
		};
		process.on("unhandledRejection", unhandledRejectionHandler);

		try {
			const handlers = api.__getPrefetchHandlers({
				href: "/prefetch-redirect",
			});
			handlers?.start(new Event("mouseenter"));

			await vi.advanceTimersByTimeAsync(100);
			await vi.runAllTimersAsync();
			await Promise.resolve();

			expect(unhandledRejections).toEqual([]);
		} finally {
			process.off("unhandledRejection", unhandledRejectionHandler);
		}
	});

	it("supports click while prefetch is in-flight and runs render callbacks", async () => {
		const api = await loadClientAPI();
		const fetchDeferred = createDeferred<Response>();
		vi.spyOn(window, "fetch").mockReturnValue(fetchDeferred.promise as any);

		const beforeRender = vi.fn();
		const afterRender = vi.fn();

		const handlers = api.__getPrefetchHandlers({
			href: "/click-during",
			beforeRender,
			afterRender,
		});

		handlers?.start(new Event("mouseenter"));
		await vi.advanceTimersByTimeAsync(100);

		const { anchor, event } = buildAnchorClick("/click-during");
		const clickPromise = handlers?.onClick(event);

		fetchDeferred.resolve(
			createRouteDataResponse({
				title: { dangerousInnerHTML: "Eventual" },
			}),
		);

		await clickPromise;
		await vi.runAllTimersAsync();

		expect(beforeRender).toHaveBeenCalledTimes(1);
		expect(afterRender).toHaveBeenCalledTimes(1);
		expect(document.title).toBe("Eventual");

		document.body.removeChild(anchor);
	});

	it("drops completed prefetch cache when stop is called so later prefetches refetch", async () => {
		const api = await loadClientAPI();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValue(createRouteDataResponse());

		const firstHandlers = api.__getPrefetchHandlers({
			href: "/completed-prefetch",
			delayMs: 0,
		});
		firstHandlers?.start(new Event("mouseenter"));
		await vi.advanceTimersByTimeAsync(1);
		await vi.runAllTimersAsync();
		expect(fetchSpy).toHaveBeenCalledTimes(1);

		firstHandlers?.stop();

		const secondHandlers = api.__getPrefetchHandlers({
			href: "/completed-prefetch",
			delayMs: 0,
		});
		secondHandlers?.start(new Event("mouseenter"));
		await vi.advanceTimersByTimeAsync(1);

		expect(fetchSpy).toHaveBeenCalledTimes(2);
		secondHandlers?.stop();
	});
});
