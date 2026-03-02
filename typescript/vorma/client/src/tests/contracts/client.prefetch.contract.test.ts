import { describe, expect, it, vi } from "vitest";
import {
	createDeferredFetchCall,
	createRouteDataResponse,
	createSignalCapturingNeverFetchSpy,
	loadClientAPI,
	patchContractRuntimeRouteSnapshot,
	registerServerDataFieldProbeLoader,
	setupContractTestSuite,
	withUnhandledRejectionCapture,
} from "./contract_test_harness.ts";

setupContractTestSuite();

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

	it("does not client-navigate prefetch clicks for non-self target anchors", async () => {
		const api = await loadClientAPI();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValue(createRouteDataResponse());

		const handlers = api.__getPrefetchHandlers({
			href: "/targeted-prefetch-click",
			delayMs: 0,
		});
		const { anchor, event } = buildAnchorClick("/targeted-prefetch-click");
		anchor.target = "_top";
		const preventDefault = vi.spyOn(event, "preventDefault");

		await handlers?.onClick(event);
		await vi.runAllTimersAsync();

		expect(preventDefault).not.toHaveBeenCalled();
		expect(fetchSpy).not.toHaveBeenCalled();
		document.body.removeChild(anchor);
	});

	it("retries prefetch after a current-page no-op when location changes", async () => {
		const api = await loadClientAPI();
		window.history.replaceState({}, "", "/current-page");
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValue(createRouteDataResponse());

		const handlers = api.__getPrefetchHandlers({
			href: "/current-page",
			delayMs: 100,
		});

		handlers?.start(new Event("mouseenter"));
		await vi.advanceTimersByTimeAsync(100);
		expect(fetchSpy).not.toHaveBeenCalled();

		window.history.replaceState({}, "", "/other-page");
		handlers?.start(new Event("focus"));
		await vi.advanceTimersByTimeAsync(100);

		expect(fetchSpy).toHaveBeenCalledTimes(1);
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

	it("deduplicates same-data prefetches when only hash differs", async () => {
		const api = await loadClientAPI();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValue(createRouteDataResponse());

		const aHandlers = api.__getPrefetchHandlers({
			href: "/prefetch-dedupe#first",
			delayMs: 0,
		});
		const bHandlers = api.__getPrefetchHandlers({
			href: "/prefetch-dedupe#second",
			delayMs: 0,
		});

		aHandlers?.start(new Event("mouseenter"));
		await vi.advanceTimersByTimeAsync(1);
		bHandlers?.start(new Event("mouseenter"));
		await vi.advanceTimersByTimeAsync(1);

		expect(fetchSpy).toHaveBeenCalledTimes(1);
	});

	it("deduplicates identical prefetches for the same href", async () => {
		const api = await loadClientAPI();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValue(createRouteDataResponse());

		const aHandlers = api.__getPrefetchHandlers({
			href: "/prefetch-identical",
			delayMs: 0,
		});
		const bHandlers = api.__getPrefetchHandlers({
			href: "/prefetch-identical",
			delayMs: 0,
		});

		aHandlers?.start(new Event("mouseenter"));
		await vi.advanceTimersByTimeAsync(1);
		bHandlers?.start(new Event("mouseenter"));
		await vi.advanceTimersByTimeAsync(1);

		expect(fetchSpy).toHaveBeenCalledTimes(1);
	});

	it("aborts a hash-deduped shared prefetch when stop is called from either handler", async () => {
		const api = await loadClientAPI();
		const { fetchSpy, signals } = createSignalCapturingNeverFetchSpy();

		const aHandlers = api.__getPrefetchHandlers({
			href: "/prefetch-stop-alias#first",
			delayMs: 0,
		});
		const bHandlers = api.__getPrefetchHandlers({
			href: "/prefetch-stop-alias#second",
			delayMs: 0,
		});

		aHandlers?.start(new Event("mouseenter"));
		await vi.advanceTimersByTimeAsync(1);
		bHandlers?.start(new Event("mouseenter"));
		await vi.advanceTimersByTimeAsync(1);

		expect(fetchSpy).toHaveBeenCalledTimes(1);
		expect(signals).toHaveLength(1);
		expect(signals[0]?.aborted).toBe(false);

		bHandlers?.stop();
		expect(signals[0]?.aborted).toBe(true);
	});

	it("aborts idle prefetch entries via same-data-target alias lookup", async () => {
		const api = await loadClientAPI();
		const { signals } = createSignalCapturingNeverFetchSpy();
		const firstHandlers = api.__getPrefetchHandlers({
			href: "/prefetch-idle-alias#first",
			delayMs: 0,
		});
		firstHandlers?.start(new Event("mouseenter"));
		await vi.advanceTimersByTimeAsync(1);
		expect(signals[0]?.aborted).toBe(false);

		const aliasHandlers = api.__getPrefetchHandlers({
			href: "/prefetch-idle-alias#second",
			delayMs: 0,
		});
		aliasHandlers?.stop();

		expect(signals[0]?.aborted).toBe(true);
	});

	it("cancels pending prefetch timer on hash-only click", async () => {
		const api = await loadClientAPI();
		window.history.replaceState({}, "", "/hash-page");
		const beforeBegin = vi.fn();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValue(createRouteDataResponse());

		const handlers = api.__getPrefetchHandlers({
			href: "/hash-page#section-a",
			delayMs: 200,
			beforeBegin,
		});

		handlers?.start(new Event("mouseenter"));
		await vi.advanceTimersByTimeAsync(100);
		expect(beforeBegin).not.toHaveBeenCalled();

		const { anchor, event } = buildAnchorClick("/hash-page#section-a");
		await handlers?.onClick(event);
		await vi.advanceTimersByTimeAsync(200);

		expect(beforeBegin).not.toHaveBeenCalled();
		expect(fetchSpy).not.toHaveBeenCalled();
		document.body.removeChild(anchor);
	});

	it("cancels pending prefetch timer on same-document hash removal click", async () => {
		const api = await loadClientAPI();
		window.history.replaceState({}, "", "/hash-page#section-a");
		const beforeBegin = vi.fn();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValue(createRouteDataResponse());

		const handlers = api.__getPrefetchHandlers({
			href: "/hash-page",
			delayMs: 200,
			beforeBegin,
		});

		handlers?.start(new Event("mouseenter"));
		await vi.advanceTimersByTimeAsync(100);
		expect(beforeBegin).not.toHaveBeenCalled();

		const { anchor, event } = buildAnchorClick("/hash-page");
		await handlers?.onClick(event);
		await vi.advanceTimersByTimeAsync(200);

		expect(beforeBegin).not.toHaveBeenCalled();
		expect(fetchSpy).not.toHaveBeenCalled();
		document.body.removeChild(anchor);
	});

	it("cancels pending prefetch timer on same-document no-op hash click", async () => {
		const api = await loadClientAPI();
		window.history.replaceState({}, "", "/hash-page#section-a");
		const beforeBegin = vi.fn();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValue(createRouteDataResponse());

		const handlers = api.__getPrefetchHandlers({
			href: "/hash-page#section-a",
			delayMs: 200,
			beforeBegin,
		});

		handlers?.start(new Event("mouseenter"));
		await vi.advanceTimersByTimeAsync(100);
		expect(beforeBegin).not.toHaveBeenCalled();

		const { anchor, event } = buildAnchorClick("/hash-page#section-a");
		const preventDefault = vi.spyOn(event, "preventDefault");
		await handlers?.onClick(event);
		await vi.advanceTimersByTimeAsync(200);

		expect(preventDefault).toHaveBeenCalledTimes(1);
		expect(beforeBegin).not.toHaveBeenCalled();
		expect(fetchSpy).not.toHaveBeenCalled();
		document.body.removeChild(anchor);
	});

	it("prevents default on same-document no-op prefetch clicks without hash", async () => {
		const api = await loadClientAPI();
		window.history.replaceState({}, "", "/current-page");
		const beforeBegin = vi.fn();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValue(createRouteDataResponse());

		const handlers = api.__getPrefetchHandlers({
			href: "/current-page",
			delayMs: 200,
			beforeBegin,
		});

		handlers?.start(new Event("mouseenter"));
		await vi.advanceTimersByTimeAsync(100);
		expect(beforeBegin).not.toHaveBeenCalled();

		const { anchor, event } = buildAnchorClick("/current-page");
		const preventDefault = vi.spyOn(event, "preventDefault");
		await handlers?.onClick(event);
		await vi.advanceTimersByTimeAsync(200);

		expect(preventDefault).toHaveBeenCalledTimes(1);
		expect(beforeBegin).not.toHaveBeenCalled();
		expect(fetchSpy).not.toHaveBeenCalled();
		document.body.removeChild(anchor);
	});

	it("cancels pending prefetch timer on encoding-equivalent hash click", async () => {
		const api = await loadClientAPI();
		window.history.replaceState({}, "", "/hash-page#~");
		const beforeBegin = vi.fn();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValue(createRouteDataResponse());

		const handlers = api.__getPrefetchHandlers({
			href: "/hash-page#%7E",
			delayMs: 200,
			beforeBegin,
		});

		handlers?.start(new Event("mouseenter"));
		await vi.advanceTimersByTimeAsync(100);
		expect(beforeBegin).not.toHaveBeenCalled();

		const { anchor, event } = buildAnchorClick("/hash-page#%7E");
		await handlers?.onClick(event);
		await vi.advanceTimersByTimeAsync(200);

		expect(beforeBegin).not.toHaveBeenCalled();
		expect(fetchSpy).not.toHaveBeenCalled();
		document.body.removeChild(anchor);
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

	it("recovers from beforeBegin prefetch callback failures and allows retry", async () => {
		const api = await loadClientAPI();
		const beforeBegin = vi
			.fn()
			.mockRejectedValueOnce(new Error("beforeBegin failure"))
			.mockResolvedValue(undefined);
		const logErrorSpy = vi
			.spyOn(console, "error")
			.mockImplementation(() => {});
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValue(createRouteDataResponse());

		const handlers = api.__getPrefetchHandlers({
			href: "/before-begin-retry",
			delayMs: 100,
			beforeBegin,
		});

		handlers?.start(new Event("mouseenter"));
		await vi.advanceTimersByTimeAsync(100);

		expect(beforeBegin).toHaveBeenCalledTimes(1);
		expect(fetchSpy).not.toHaveBeenCalled();
		expect(logErrorSpy).toHaveBeenCalled();

		handlers?.start(new Event("focus"));
		await vi.advanceTimersByTimeAsync(100);
		await vi.runAllTimersAsync();

		expect(beforeBegin).toHaveBeenCalledTimes(2);
		expect(fetchSpy).toHaveBeenCalledTimes(1);
		logErrorSpy.mockRestore();
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

	it("warms internal prefetch artifacts without committing page mutations", async () => {
		const api = await loadClientAPI();
		patchContractRuntimeRouteSnapshot({
			api,
			patch: {
				clientLoadersData: [
					{ keep: "current-page-client-loader-data" },
				],
				outermostClientError: "keep-current-page-client-error",
				outermostClientErrorIdx: 0,
			},
		});

		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse(
				{
					cssBundles: ["/prefetch-internal-warm.css"],
					title: {
						dangerousInnerHTML:
							"Prefetch should not commit this title",
					},
					metaHeadEls: [
						{
							tag: "meta",
							attributesKnownSafe: {
								name: "prefetch-head-marker",
								content: "prefetch-only",
							},
						},
					],
				},
				{ headers: { "X-Wave-Framework-Build-Id": "2" } },
			),
		);

		const handlers = api.__getPrefetchHandlers({
			href: "/prefetch-internal-warm",
			delayMs: 0,
		});
		handlers?.start(new Event("mouseenter"));
		await vi.advanceTimersByTimeAsync(1);
		await vi.runAllTimersAsync();

		expect(fetchSpy).toHaveBeenCalledTimes(1);
		expect(api.getBuildID()).toBe("2");
		expect(
			document.head.querySelector(
				'link[rel="preload"][as="style"][href="/prefetch-internal-warm.css"]',
			),
		).toBeTruthy();
		expect(
			document.head.querySelector(
				'link[rel="stylesheet"][data-vorma-css-bundle="/prefetch-internal-warm.css"]',
			),
		).toBeNull();
		expect(document.title).toBe("Initial Title");
		expect(
			document.head.querySelector('meta[name="prefetch-head-marker"]'),
		).toBeNull();
		expect(window.location.pathname).toBe("/");
		expect(
			api.__vormaClientGlobal.get("runtimeRouteSnapshot")
				.clientLoadersData,
		).toEqual([{ keep: "current-page-client-loader-data" }]);
		expect(
			api.__vormaClientGlobal.get("runtimeRouteSnapshot")
				.outermostClientError,
		).toBe("keep-current-page-client-error");
		expect(
			api.__vormaClientGlobal.get("runtimeRouteSnapshot")
				.outermostClientErrorIdx,
		).toBe(0);
		handlers?.stop();
	});

	it("applies prefetched css only after navigation commit", async () => {
		const api = await loadClientAPI();
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				cssBundles: ["/prefetch-commit.css"],
				title: { dangerousInnerHTML: "Prefetch Commit Boundary" },
			}),
		);
		const appendChild = document.head.appendChild.bind(document.head);
		vi.spyOn(document.head, "appendChild").mockImplementation((node) => {
			if (
				node instanceof HTMLLinkElement &&
				node.rel === "preload" &&
				node.getAttribute("as") === "style"
			) {
				Promise.resolve().then(() => node.onload?.(new Event("load")));
			}
			return appendChild(node);
		});

		const handlers = api.__getPrefetchHandlers({
			href: "/prefetch-commit",
			delayMs: 0,
		});
		handlers?.start(new Event("mouseenter"));
		await vi.advanceTimersByTimeAsync(1);
		await vi.runAllTimersAsync();

		expect(
			document.head.querySelector(
				'link[rel="stylesheet"][data-vorma-css-bundle="/prefetch-commit.css"]',
			),
		).toBeNull();

		fetchSpy.mockClear();
		const { anchor, event } = buildAnchorClick("/prefetch-commit");
		await handlers?.onClick(event);
		await vi.runAllTimersAsync();

		expect(fetchSpy).not.toHaveBeenCalled();
		expect(
			document.querySelectorAll(
				'link[rel="stylesheet"][data-vorma-css-bundle="/prefetch-commit.css"]',
			),
		).toHaveLength(1);
		expect(document.title).toBe("Prefetch Commit Boundary");
		expect(window.location.pathname).toBe("/prefetch-commit");

		document.body.removeChild(anchor);
		handlers?.stop();
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

	it("does not leak orphan timers when start is called multiple times before stop", async () => {
		const api = await loadClientAPI();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValue(createRouteDataResponse());

		const handlers = api.__getPrefetchHandlers({
			href: "/orphan-timer",
			delayMs: 200,
		});
		handlers?.start(new Event("mouseenter"));
		handlers?.start(new Event("focus"));
		handlers?.stop();

		await vi.advanceTimersByTimeAsync(250);
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

	it("aborts in-flight prefetch when stop is called", async () => {
		const api = await loadClientAPI();
		const { fetchSpy, signals } = createSignalCapturingNeverFetchSpy();

		const handlers = api.__getPrefetchHandlers({
			href: "/abort-in-flight-prefetch",
			delayMs: 0,
		});
		handlers?.start(new Event("mouseenter"));
		await vi.advanceTimersByTimeAsync(1);

		expect(fetchSpy).toHaveBeenCalledTimes(1);
		expect(signals).toHaveLength(1);
		expect(signals[0]?.aborted).toBe(false);

		handlers?.stop();
		expect(signals[0]?.aborted).toBe(true);
	});

	it("does not abort upgraded navigation when prefetch handlers stop", async () => {
		const api = await loadClientAPI();
		const fetchCall = createDeferredFetchCall();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockImplementation(fetchCall.mock);

		const handlers = api.__getPrefetchHandlers({ href: "/abort-test" });
		handlers?.start(new Event("mouseenter"));
		await vi.advanceTimersByTimeAsync(100);

		expect(fetchSpy).toHaveBeenCalledTimes(1);
		expect(fetchCall.getSignal()?.aborted).toBe(false);

		const navPromise = api.vormaNavigate("/abort-test");
		handlers?.stop();

		expect(fetchCall.getSignal()?.aborted).toBe(false);
		expect(api.getStatus().isNavigating).toBe(true);
		expect(fetchSpy).toHaveBeenCalledTimes(1);

		fetchCall.deferred.resolve(createRouteDataResponse());
		await navPromise;
		await vi.runAllTimersAsync();
	});

	it("upgrades same-data prefetch to navigation even when only hash differs", async () => {
		const api = await loadClientAPI();
		const fetchCall = createDeferredFetchCall();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockImplementation(fetchCall.mock);

		const handlers = api.__getPrefetchHandlers({
			href: "/prefetch-hash-upgrade#prefetch",
		});
		handlers?.start(new Event("mouseenter"));
		await vi.advanceTimersByTimeAsync(100);

		expect(fetchSpy).toHaveBeenCalledTimes(1);
		expect(api.getStatus().isNavigating).toBe(false);

		const navPromise = api.vormaNavigate("/prefetch-hash-upgrade#final");
		await vi.advanceTimersByTimeAsync(8);

		expect(fetchSpy).toHaveBeenCalledTimes(1);
		expect(api.getStatus().isNavigating).toBe(true);

		fetchCall.deferred.resolve(
			createRouteDataResponse({
				title: { dangerousInnerHTML: "Prefetch Hash Upgrade" },
			}),
		);

		await navPromise;
		await vi.runAllTimersAsync();

		expect(window.location.pathname).toBe("/prefetch-hash-upgrade");
		expect(window.location.hash).toBe("#final");
		expect(document.title).toBe("Prefetch Hash Upgrade");
	});

	it("does not leak unhandled rejections when pure prefetch resolves to redirect", async () => {
		const api = await loadClientAPI();
		const { serverDataPromiseErrors } =
			await registerServerDataFieldProbeLoader({
				api,
				pattern: "/prefetch-redirect",
				requiredField: "Title",
			});

		vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse(
				{},
				{ headers: { "X-Client-Redirect": "/redirect-target" } },
			),
		);

		const { unhandledRejections } = await withUnhandledRejectionCapture({
			run: async () => {
				const handlers = api.__getPrefetchHandlers({
					href: "/prefetch-redirect",
				});
				handlers?.start(new Event("mouseenter"));

				await vi.advanceTimersByTimeAsync(100);
				await vi.runAllTimersAsync();
			},
		});

		expect(unhandledRejections).toEqual([]);
		expect(serverDataPromiseErrors).toHaveLength(1);
		expect(serverDataPromiseErrors[0]).toBeInstanceOf(Error);
		expect((serverDataPromiseErrors[0] as Error).name).toBe("AbortError");
	});

	it("rejects serverDataPromise with AbortError for failed prefetch responses", async () => {
		const api = await loadClientAPI();
		const { serverDataPromiseErrors } =
			await registerServerDataFieldProbeLoader({
				api,
				pattern: "/prefetch-failed-response",
				requiredField: "Title",
			});

		vi.spyOn(window, "fetch").mockResolvedValue(
			new Response("Server error", { status: 500 }),
		);

		const { unhandledRejections } = await withUnhandledRejectionCapture({
			run: async () => {
				const handlers = api.__getPrefetchHandlers({
					href: "/prefetch-failed-response",
				});
				handlers?.start(new Event("mouseenter"));

				await vi.advanceTimersByTimeAsync(100);
				await vi.runAllTimersAsync();
			},
		});

		expect(unhandledRejections).toEqual([]);
		expect(serverDataPromiseErrors).toHaveLength(1);
		expect(serverDataPromiseErrors[0]).toBeInstanceOf(Error);
		expect((serverDataPromiseErrors[0] as Error).name).toBe("AbortError");
	});

	it("rejects serverDataPromise with AbortError when server omits a prestarted matched pattern", async () => {
		const api = await loadClientAPI();
		const { serverDataPromiseErrors } =
			await registerServerDataFieldProbeLoader({
				api,
				pattern: "/prefetch-mismatch",
				requiredField: "Title",
			});

		vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				matchedPatterns: [],
				loadersData: [],
			}),
		);

		const { unhandledRejections } = await withUnhandledRejectionCapture({
			run: async () => {
				const handlers = api.__getPrefetchHandlers({
					href: "/prefetch-mismatch",
				});
				handlers?.start(new Event("mouseenter"));

				await vi.advanceTimersByTimeAsync(100);
				await vi.runAllTimersAsync();
			},
		});

		expect(unhandledRejections).toEqual([]);
		expect(serverDataPromiseErrors).toHaveLength(1);
		expect(serverDataPromiseErrors[0]).toBeInstanceOf(Error);
		expect((serverDataPromiseErrors[0] as Error).name).toBe("AbortError");
	});

	it("supports click while prefetch is in-flight and runs render callbacks", async () => {
		const api = await loadClientAPI();
		const fetchCall = createDeferredFetchCall();
		vi.spyOn(window, "fetch").mockImplementation(fetchCall.mock);

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

		fetchCall.deferred.resolve(
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

	it("runs beforeBegin on direct click when no prefetch has started yet", async () => {
		const api = await loadClientAPI();
		vi.spyOn(window, "fetch").mockResolvedValue(
			createRouteDataResponse({
				title: { dangerousInnerHTML: "Direct Click" },
			}),
		);

		const beforeBegin = vi.fn();
		const beforeRender = vi.fn();
		const afterRender = vi.fn();
		const handlers = api.__getPrefetchHandlers({
			href: "/direct-click",
			beforeBegin,
			beforeRender,
			afterRender,
		});

		const { anchor, event } = buildAnchorClick("/direct-click");
		await handlers?.onClick(event);
		await vi.runAllTimersAsync();

		expect(beforeBegin).toHaveBeenCalledTimes(1);
		expect(beforeRender).toHaveBeenCalledTimes(1);
		expect(afterRender).toHaveBeenCalledTimes(1);
		expect(document.title).toBe("Direct Click");

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
