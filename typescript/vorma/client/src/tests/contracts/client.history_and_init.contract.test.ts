import { describe, expect, it, vi } from "vitest";
import { findBestMatch } from "vorma/kit/matcher/find-best";
import { createPatternRegistry } from "vorma/kit/matcher/register";
import {
	createDeferred,
	createRouteDataResponse,
	loadClientAPI,
	setupContractTestSuite,
} from "./contract_test_harness.ts";

setupContractTestSuite();

const TEST_APP_CONFIG = {
	actionsRouterMountRoot: "/api/",
	actionsDynamicRune: ":",
	actionsSplatRune: "*",
	loadersDynamicRune: ":",
	loadersSplatRune: "*",
	loadersExplicitIndexSegmentIdentifier: "_index",
};

function stubElementScrollIntoView(
	element: HTMLElement,
): ReturnType<typeof vi.fn> {
	const scrollIntoView = vi.fn();
	Object.defineProperty(element, "scrollIntoView", {
		value: scrollIntoView,
		configurable: true,
	});
	return scrollIntoView;
}

async function flushProgressiveManifestMicrotasks(): Promise<void> {
	await Promise.resolve();
	await Promise.resolve();
}

describe("client history/init contracts", () => {
	it("exposes a usable history instance", async () => {
		const api = await loadClientAPI();
		const history = api.getHistoryInstance();

		expect(history).toBeDefined();
		expect(typeof history.push).toBe("function");
		expect(typeof history.replace).toBe("function");
		expect(history.location).toBeDefined();
	});

	it("updates history location after push navigation", async () => {
		const api = await loadClientAPI();
		const history = api.getHistoryInstance();

		history.push("/new-location");

		expect(history.location.pathname).toBe("/new-location");
	});

	it("emits location events when history key changes", async () => {
		const api = await loadClientAPI();
		const locationListener = vi.fn();
		const cleanup = api.addLocationListener(locationListener);
		api.getHistoryInstance();

		const { customHistoryListener } =
			await import("../../platform/history.ts");
		await customHistoryListener({
			action: "PUSH",
			location: {
				pathname: "/location-change",
				search: "",
				hash: "",
				state: null,
				key: "location-key-1",
			},
		} as any);

		expect(locationListener).toHaveBeenCalledTimes(1);
		cleanup();
	});

	it("applies hash scroll on same-document POP updates", async () => {
		const api = await loadClientAPI();
		api.getHistoryInstance();
		const { customHistoryListener } =
			await import("../../platform/history.ts");

		await customHistoryListener({
			action: "PUSH",
			location: {
				pathname: "/same-doc",
				search: "",
				hash: "",
				state: null,
				key: "same-doc-base",
			},
		} as any);

		const element = document.createElement("div");
		element.id = "section-a";
		document.body.appendChild(element);
		const scrollSpy = stubElementScrollIntoView(element);

		await customHistoryListener({
			action: "POP",
			location: {
				pathname: "/same-doc",
				search: "",
				hash: "#section-a",
				state: null,
				key: "same-doc-hash",
			},
		} as any);

		expect(scrollSpy).toHaveBeenCalledTimes(1);
		document.body.removeChild(element);
	});

	it("applies decoded hash scroll on same-document POP updates", async () => {
		const api = await loadClientAPI();
		api.getHistoryInstance();
		const { customHistoryListener } =
			await import("../../platform/history.ts");

		await customHistoryListener({
			action: "PUSH",
			location: {
				pathname: "/same-doc-encoded",
				search: "",
				hash: "",
				state: null,
				key: "same-doc-encoded-base",
			},
		} as any);

		const element = document.createElement("div");
		element.id = "✓";
		document.body.appendChild(element);
		const scrollSpy = stubElementScrollIntoView(element);

		await customHistoryListener({
			action: "POP",
			location: {
				pathname: "/same-doc-encoded",
				search: "",
				hash: "#%E2%9C%93",
				state: null,
				key: "same-doc-encoded-target",
			},
		} as any);

		expect(scrollSpy).toHaveBeenCalledTimes(1);
		document.body.removeChild(element);
	});

	it("preserves single-decode semantics for percent-encoded literal IDs on POP", async () => {
		const api = await loadClientAPI();
		api.getHistoryInstance();
		const { customHistoryListener } =
			await import("../../platform/history.ts");

		await customHistoryListener({
			action: "PUSH",
			location: {
				pathname: "/same-doc-percent",
				search: "",
				hash: "",
				state: null,
				key: "same-doc-percent-base",
			},
		} as any);

		const element = document.createElement("div");
		element.id = "%20-literal";
		document.body.appendChild(element);
		const scrollSpy = stubElementScrollIntoView(element);

		await customHistoryListener({
			action: "POP",
			location: {
				pathname: "/same-doc-percent",
				search: "",
				hash: "#%2520-literal",
				state: null,
				key: "same-doc-percent-target",
			},
		} as any);

		expect(scrollSpy).toHaveBeenCalledTimes(1);
		document.body.removeChild(element);
	});

	it("does not re-scroll when POP hash target is encoding-equivalent", async () => {
		const api = await loadClientAPI();
		api.getHistoryInstance();
		const { customHistoryListener } =
			await import("../../platform/history.ts");

		await customHistoryListener({
			action: "PUSH",
			location: {
				pathname: "/same-doc-equiv-hash",
				search: "",
				hash: "#~",
				state: null,
				key: "same-doc-equiv-hash-base",
			},
		} as any);

		const element = document.createElement("div");
		element.id = "~";
		document.body.appendChild(element);
		const scrollSpy = stubElementScrollIntoView(element);

		await customHistoryListener({
			action: "POP",
			location: {
				pathname: "/same-doc-equiv-hash",
				search: "",
				hash: "#%7E",
				state: null,
				key: "same-doc-equiv-hash-target",
			},
		} as any);

		expect(scrollSpy).not.toHaveBeenCalled();
		document.body.removeChild(element);
	});

	it("triggers browser-history navigation fetch for cross-document POP", async () => {
		const api = await loadClientAPI();
		api.getHistoryInstance();
		const { customHistoryListener } =
			await import("../../platform/history.ts");

		await customHistoryListener({
			action: "PUSH",
			location: {
				pathname: "/from-page",
				search: "",
				hash: "",
				state: null,
				key: "from-key",
			},
		} as any);

		window.history.replaceState({}, "", "/to-page");
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValue(createRouteDataResponse());

		await customHistoryListener({
			action: "POP",
			location: {
				pathname: "/to-page",
				search: "",
				hash: "",
				state: null,
				key: "to-key",
			},
		} as any);
		await vi.runAllTimersAsync();

		expect(fetchSpy).toHaveBeenCalled();
		const fetchURL = fetchSpy.mock.calls[0]?.[0] as URL;
		expect(fetchURL.href).toContain("/to-page?vorma_json=1");
	});

	it("follows cross-document POP redirects and renders redirected destination", async () => {
		const api = await loadClientAPI();
		api.getHistoryInstance();
		const { customHistoryListener } =
			await import("../../platform/history.ts");

		await customHistoryListener({
			action: "PUSH",
			location: {
				pathname: "/from-pop-redirect",
				search: "",
				hash: "",
				state: null,
				key: "from-pop-redirect-key",
			},
		} as any);

		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValueOnce(
				createRouteDataResponse(
					{},
					{
						headers: {
							"X-Client-Redirect": "/pop-redirect-destination",
							"X-Vorma-Build-Id": "2",
						},
					},
				),
			)
			.mockResolvedValueOnce(
				createRouteDataResponse(
					{
						title: {
							dangerousInnerHTML: "POP Redirect Destination",
						},
					},
					{
						headers: {
							"X-Vorma-Build-Id": "2",
						},
					},
				),
			);

		await customHistoryListener({
			action: "POP",
			location: {
				pathname: "/pop-redirect-source",
				search: "",
				hash: "",
				state: null,
				key: "pop-redirect-source-key",
			},
		} as any);
		await vi.runAllTimersAsync();

		expect(fetchSpy).toHaveBeenCalledTimes(2);
		const firstFetchURL = fetchSpy.mock.calls[0]?.[0] as URL;
		const secondFetchURL = fetchSpy.mock.calls[1]?.[0] as URL;
		expect(firstFetchURL.pathname).toBe("/pop-redirect-source");
		expect(secondFetchURL.pathname).toBe("/pop-redirect-destination");
		expect(window.location.pathname).toBe("/pop-redirect-destination");
		expect(document.title).toBe("POP Redirect Destination");
		expect(api.getStatus()).toEqual({
			isNavigating: false,
			isSubmitting: false,
			isRevalidating: false,
		});
	});

	it("uses listener location payload as the source of truth for cross-document POP target", async () => {
		const api = await loadClientAPI();
		api.getHistoryInstance();
		const { customHistoryListener } =
			await import("../../platform/history.ts");

		await customHistoryListener({
			action: "PUSH",
			location: {
				pathname: "/from-page",
				search: "",
				hash: "",
				state: null,
				key: "from-key-source-of-truth",
			},
		} as any);

		window.history.replaceState({}, "", "/window-location-only");
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValue(createRouteDataResponse());

		await customHistoryListener({
			action: "POP",
			location: {
				pathname: "/listener-target",
				search: "?q=payload",
				hash: "#details",
				state: null,
				key: "listener-key-source-of-truth",
			},
		} as any);
		await vi.runAllTimersAsync();

		expect(fetchSpy).toHaveBeenCalledTimes(1);
		const fetchURL = fetchSpy.mock.calls[0]?.[0] as URL;
		expect(fetchURL.pathname).toBe("/listener-target");
		expect(fetchURL.searchParams.get("q")).toBe("payload");
		expect(fetchURL.searchParams.get("vorma_json")).toBe("1");
	});

	it("saves scroll state before moving to a different document", async () => {
		const api = await loadClientAPI();
		const history = api.getHistoryInstance();
		(window as any).scrollX = 123;
		(window as any).scrollY = 456;

		const { customHistoryListener } =
			await import("../../platform/history.ts");
		await customHistoryListener({
			action: "PUSH",
			location: {
				pathname: "/different-page",
				search: "",
				hash: "",
				state: null,
				key: "different-key",
			},
		} as any);

		const raw = sessionStorage.getItem("__vorma__scrollStateMap");
		expect(raw).toBeTruthy();
		const entries = JSON.parse(raw || "[]");
		expect(entries).toContainEqual([
			history.location.key,
			{ x: 123, y: 456 },
		]);
	});

	it("stores scroll states in session storage with FIFO eviction at 50 entries", async () => {
		await loadClientAPI();
		const { scrollStateManager } = await import("../../platform/scroll.ts");

		for (let i = 0; i <= 50; i++) {
			scrollStateManager.saveState(`key-${i}`, { x: i, y: i });
		}

		const raw = sessionStorage.getItem("__vorma__scrollStateMap");
		expect(raw).toBeTruthy();
		const entries = JSON.parse(raw || "[]") as Array<[string, unknown]>;
		expect(entries).toHaveLength(50);
		expect(entries[0]?.[0]).toBe("key-1");
		expect(entries.at(-1)?.[0]).toBe("key-50");
	});

	it("saves outgoing scroll position before user navigation pushes a new history entry", async () => {
		const api = await loadClientAPI();
		const history = api.getHistoryInstance();
		const { HistoryManager } = await import("../../platform/history.ts");
		HistoryManager.init();

		history.push("/current-source");
		const sourceKey = history.location.key;

		(window as any).scrollX = 150;
		(window as any).scrollY = 300;
		vi.spyOn(window, "fetch").mockResolvedValue(createRouteDataResponse());

		await api.vormaNavigate("/next-target");
		await vi.runAllTimersAsync();

		const raw = sessionStorage.getItem("__vorma__scrollStateMap");
		expect(raw).toBeTruthy();
		const entries = JSON.parse(raw || "[]");
		expect(entries).toContainEqual([sourceKey, { x: 150, y: 300 }]);
	});

	it("saves scroll state on cross-document POP before restoring the target document", async () => {
		const api = await loadClientAPI();
		api.getHistoryInstance();
		const { customHistoryListener } =
			await import("../../platform/history.ts");

		await customHistoryListener({
			action: "PUSH",
			location: {
				pathname: "/page-two",
				search: "",
				hash: "",
				state: null,
				key: "page-two-key",
			},
		} as any);

		(window as any).scrollX = 50;
		(window as any).scrollY = 100;
		window.history.replaceState({}, "", "/page-one");
		vi.spyOn(window, "fetch").mockResolvedValue(createRouteDataResponse());

		await customHistoryListener({
			action: "POP",
			location: {
				pathname: "/page-one",
				search: "",
				hash: "",
				state: null,
				key: "page-one-key",
			},
		} as any);
		await vi.runAllTimersAsync();

		const raw = sessionStorage.getItem("__vorma__scrollStateMap");
		expect(raw).toBeTruthy();
		const entries = JSON.parse(raw || "[]");
		expect(entries).toContainEqual(["page-two-key", { x: 50, y: 100 }]);
	});

	it("restores saved scroll position when POP removes a hash from the same document", async () => {
		const api = await loadClientAPI();
		api.getHistoryInstance();
		const { customHistoryListener } =
			await import("../../platform/history.ts");
		const { scrollStateManager } = await import("../../platform/scroll.ts");

		await customHistoryListener({
			action: "PUSH",
			location: {
				pathname: "/page",
				search: "",
				hash: "#section",
				state: null,
				key: "hash-key",
			},
		} as any);

		scrollStateManager.saveState("plain-key", { x: 75, y: 150 });

		await customHistoryListener({
			action: "POP",
			location: {
				pathname: "/page",
				search: "",
				hash: "",
				state: null,
				key: "plain-key",
			},
		} as any);

		expect(window.scrollTo).toHaveBeenCalledWith(75, 150);
	});

	it("restores saved scroll position when POP transitions from hash target to empty-fragment '#'", async () => {
		const api = await loadClientAPI();
		api.getHistoryInstance();
		const { customHistoryListener } =
			await import("../../platform/history.ts");
		const { scrollStateManager } = await import("../../platform/scroll.ts");

		await customHistoryListener({
			action: "PUSH",
			location: {
				pathname: "/page-hash-empty-fragment",
				search: "",
				hash: "#section",
				state: null,
				key: "hash-empty-fragment-key",
			},
		} as any);

		scrollStateManager.saveState("hash-empty-fragment-target-key", {
			x: 88,
			y: 166,
		});

		await customHistoryListener({
			action: "POP",
			location: {
				pathname: "/page-hash-empty-fragment",
				search: "",
				hash: "#",
				state: null,
				key: "hash-empty-fragment-target-key",
			},
		} as any);

		expect(window.scrollTo).toHaveBeenCalledWith(88, 166);
	});

	it("initializes client options and calls render function", async () => {
		const api = await loadClientAPI();
		const renderFn = vi.fn();
		const customErrorBoundary = () => null;

		await api.initClient({
			vormaAppConfig: TEST_APP_CONFIG,
			renderFn,
			defaultErrorBoundary: customErrorBoundary,
			useViewTransitions: true,
		});

		expect(renderFn).toHaveBeenCalledTimes(1);
		expect(api.__vormaClientGlobal.get("defaultErrorBoundary")).toBe(
			customErrorBoundary,
		);
		expect(api.__vormaClientGlobal.get("useViewTransitions")).toBe(true);
	});

	it("applies useViewTransitions option on every init call", async () => {
		const api = await loadClientAPI();

		await api.initClient({
			vormaAppConfig: TEST_APP_CONFIG,
			renderFn: () => {},
			useViewTransitions: true,
		});
		expect(api.__vormaClientGlobal.get("useViewTransitions")).toBe(true);

		await api.initClient({
			vormaAppConfig: TEST_APP_CONFIG,
			renderFn: () => {},
			useViewTransitions: false,
		});
		expect(api.__vormaClientGlobal.get("useViewTransitions")).toBe(false);
	});

	it("registers browser history listener during init", async () => {
		const api = await loadClientAPI();
		const history = api.getHistoryInstance();
		const listenSpy = vi.spyOn(history, "listen");

		await api.initClient({
			vormaAppConfig: TEST_APP_CONFIG,
			renderFn: () => {},
		});

		expect(listenSpy).toHaveBeenCalledTimes(1);
	});

	it("registers beforeunload and touch listeners only once across repeated init calls", async () => {
		const api = await loadClientAPI();
		const history = api.getHistoryInstance();
		const unlistenSpy = vi.fn();
		const listenSpy = vi
			.spyOn(history, "listen")
			.mockReturnValue(unlistenSpy);
		const addEventListenerSpy = vi.spyOn(window, "addEventListener");

		await api.initClient({
			vormaAppConfig: TEST_APP_CONFIG,
			renderFn: () => {},
		});
		await api.initClient({
			vormaAppConfig: TEST_APP_CONFIG,
			renderFn: () => {},
		});

		const beforeUnloadCalls = addEventListenerSpy.mock.calls.filter(
			([eventType]) => eventType === "beforeunload",
		);
		const touchStartCalls = addEventListenerSpy.mock.calls.filter(
			([eventType]) => eventType === "touchstart",
		);

		expect(listenSpy).toHaveBeenCalledTimes(2);
		expect(unlistenSpy).toHaveBeenCalledTimes(1);
		expect(beforeUnloadCalls).toHaveLength(1);
		expect(touchStartCalls).toHaveLength(1);
	});

	it("sets history scrollRestoration to manual during init when supported", async () => {
		const api = await loadClientAPI();
		const setterSpy = vi.fn();

		Object.defineProperty(window.history, "scrollRestoration", {
			get: () => "auto",
			set: setterSpy,
			configurable: true,
		});

		await api.initClient({
			vormaAppConfig: TEST_APP_CONFIG,
			renderFn: () => {},
		});

		expect(setterSpy).toHaveBeenCalledWith("manual");
	});

	it("cleans vorma_reload from URL during init", async () => {
		const api = await loadClientAPI();
		window.history.replaceState({}, "", "/?vorma_reload=old&keep=this");
		const history = api.getHistoryInstance();
		const replaceSpy = vi.spyOn(history, "replace");

		await api.initClient({
			vormaAppConfig: TEST_APP_CONFIG,
			renderFn: () => {},
		});

		expect(replaceSpy).toHaveBeenCalledWith(
			"http://localhost:3000/?keep=this",
		);
	});

	it("loads initial components during init from current importURLs", async () => {
		const api = await loadClientAPI();
		const initialComponent = () => "Initial Component";
		vi.doMock("/initial.js", () => ({
			default: initialComponent,
		}));

		api.__vormaClientGlobal.set("importURLs", ["/initial.js"]);
		api.__vormaClientGlobal.set("matchedPatterns", ["/"]);
		api.__vormaClientGlobal.set("exportKeys", ["default"]);

		await api.initClient({
			vormaAppConfig: TEST_APP_CONFIG,
			renderFn: () => {},
		});

		const activeComponents =
			api.__vormaClientGlobal.get("activeComponents");
		if (!activeComponents) {
			throw new Error("Expected activeComponents to be initialized");
		}
		expect(activeComponents).toHaveLength(1);
		expect(activeComponents[0]).toBe(initialComponent);
	});

	it("runs initial client wait functions during init and stores loader data", async () => {
		const api = await loadClientAPI();
		const waitFn = vi.fn().mockResolvedValue({ initialized: true });

		api.__vormaClientGlobal.set("patternToWaitFnMap", { "/": waitFn });
		api.__vormaClientGlobal.set("matchedPatterns", ["/"]);
		api.__vormaClientGlobal.set("loadersData", [{ initial: "data" }]);

		await api.initClient({
			vormaAppConfig: TEST_APP_CONFIG,
			renderFn: () => {},
		});

		expect(waitFn).toHaveBeenCalledTimes(1);
		expect(api.__vormaClientGlobal.get("clientLoadersData")).toEqual([
			{ initialized: true },
		]);
	});

	it("restores recent page-refresh scroll state during init", async () => {
		const api = await loadClientAPI();
		sessionStorage.setItem(
			"__vorma__pageRefreshScrollState",
			JSON.stringify({
				x: 300,
				y: 600,
				unix: Date.now() - 1000,
				href: window.location.href,
			}),
		);
		vi.spyOn(window, "requestAnimationFrame").mockImplementation((cb) => {
			cb(0);
			return 0;
		});

		await api.initClient({
			vormaAppConfig: TEST_APP_CONFIG,
			renderFn: () => {},
		});

		expect(window.scrollTo).toHaveBeenCalledWith(300, 600);
		expect(
			sessionStorage.getItem("__vorma__pageRefreshScrollState"),
		).toBeNull();
	});

	it("saves page-refresh scroll state on beforeunload after init", async () => {
		const api = await loadClientAPI();
		(window as any).scrollX = 200;
		(window as any).scrollY = 400;

		await api.initClient({
			vormaAppConfig: TEST_APP_CONFIG,
			renderFn: () => {},
		});

		window.dispatchEvent(new Event("beforeunload"));

		const raw = sessionStorage.getItem("__vorma__pageRefreshScrollState");
		expect(raw).toBeTruthy();
		const saved = JSON.parse(raw || "{}");
		expect(saved).toMatchObject({
			x: 200,
			y: 400,
			href: window.location.href,
		});
		expect(typeof saved.unix).toBe("number");
	});

	it("does not restore page-refresh scroll state when snapshot URL differs", async () => {
		const api = await loadClientAPI();
		sessionStorage.setItem(
			"__vorma__pageRefreshScrollState",
			JSON.stringify({
				x: 250,
				y: 500,
				unix: Date.now() - 1000,
				href: "http://localhost:3000/different-page",
			}),
		);

		await api.initClient({
			vormaAppConfig: TEST_APP_CONFIG,
			renderFn: () => {},
		});

		expect(window.scrollTo).not.toHaveBeenCalledWith(250, 500);
		expect(
			sessionStorage.getItem("__vorma__pageRefreshScrollState"),
		).toBeNull();
	});

	it("does not restore page-refresh scroll state when snapshot is older than five seconds", async () => {
		const api = await loadClientAPI();
		sessionStorage.setItem(
			"__vorma__pageRefreshScrollState",
			JSON.stringify({
				x: 250,
				y: 500,
				unix: Date.now() - 6000,
				href: window.location.href,
			}),
		);

		await api.initClient({
			vormaAppConfig: TEST_APP_CONFIG,
			renderFn: () => {},
		});

		expect(window.scrollTo).not.toHaveBeenCalledWith(250, 500);
		expect(
			sessionStorage.getItem("__vorma__pageRefreshScrollState"),
		).toBeNull();
	});

	it("marks device as touch-capable on first touch after init", async () => {
		const api = await loadClientAPI();

		await api.initClient({
			vormaAppConfig: TEST_APP_CONFIG,
			renderFn: () => {},
		});

		window.dispatchEvent(new Event("touchstart"));
		expect(api.__vormaClientGlobal.get("isTouchDevice")).toBe(true);
	});

	it("exposes the dev revalidate handle during init", async () => {
		const api = await loadClientAPI();

		await api.initClient({
			vormaAppConfig: TEST_APP_CONFIG,
			renderFn: () => {},
		});

		expect((window as any).__waveRevalidate).toBe(api.revalidate);
	});

	it("installs an HMR update hook during init and keeps it callable", async () => {
		const api = await loadClientAPI();
		const preInitHook = api.__runClientLoadersAfterHMRUpdate;

		await api.initClient({
			vormaAppConfig: TEST_APP_CONFIG,
			renderFn: () => {},
		});

		expect(api.__runClientLoadersAfterHMRUpdate).not.toBe(preInitHook);
		expect(() =>
			api.__runClientLoadersAfterHMRUpdate(
				{ url: "http://localhost:3000/src/routes/users.tsx" } as any,
				"/users/:id",
			),
		).not.toThrow();
	});

	it("ignores HMR updates when the provided hot runtime is invalid", async () => {
		const api = await loadClientAPI();

		await api.initClient({
			vormaAppConfig: TEST_APP_CONFIG,
			renderFn: () => {},
		});

		expect(() =>
			api.__runClientLoadersAfterHMRUpdate(
				{
					url: "http://localhost:3000/src/routes/users.tsx",
					hot: {},
				} as any,
				"/users/:id",
			),
		).not.toThrow();
	});

	it("refreshes matched route loaders after matching HMR js updates", async () => {
		const api = await loadClientAPI();
		await api.initClient({
			vormaAppConfig: TEST_APP_CONFIG,
			renderFn: () => {},
		});

		api.__vormaClientGlobal.set("matchedPatterns", ["/hmr-match"]);

		const routeChangeListener = vi.fn();
		const removeRouteChangeListener =
			api.addRouteChangeListener(routeChangeListener);
		let afterUpdateHandler:
			| ((props: {
					updates: Array<{ type: string; path: string }>;
			  }) => void)
			| undefined;
		const fakeHot = {
			on: vi.fn((event: string, handler: unknown) => {
				if (
					event === "vite:afterUpdate" &&
					typeof handler === "function"
				) {
					afterUpdateHandler = handler as typeof afterUpdateHandler;
				}
			}),
		};

		api.__runClientLoadersAfterHMRUpdate(
			{
				url: "http://localhost:3000/src/routes/hmr-match.tsx?t=1",
				hot: fakeHot,
			} as any,
			"/hmr-match",
		);

		expect(fakeHot.on).toHaveBeenCalledWith(
			"vite:afterUpdate",
			expect.any(Function),
		);
		expect(afterUpdateHandler).toBeDefined();

		afterUpdateHandler?.({
			updates: [
				{
					type: "js-update",
					path: "/src/routes/hmr-match.tsx?t=2",
				},
				{
					type: "css-update",
					path: "/src/routes/hmr-match.css?t=2",
				},
			],
		});

		await vi.advanceTimersByTimeAsync(11);
		await vi.runAllTimersAsync();
		expect(routeChangeListener).toHaveBeenCalledTimes(1);

		afterUpdateHandler?.({
			updates: [
				{
					type: "js-update",
					path: "/src/routes/other.tsx?t=3",
				},
			],
		});

		await vi.advanceTimersByTimeAsync(11);
		await vi.runAllTimersAsync();
		expect(routeChangeListener).toHaveBeenCalledTimes(1);

		api.__runClientLoadersAfterHMRUpdate(
			{
				url: "http://localhost:3000/src/routes/hmr-match.tsx?t=99",
				hot: fakeHot,
			} as any,
			"/hmr-match",
		);
		expect(fakeHot.on).toHaveBeenCalledTimes(1);

		removeRouteChangeListener();
	});

	it("refreshes when any matched pattern is tracked for the same HMR module path", async () => {
		const api = await loadClientAPI();
		await api.initClient({
			vormaAppConfig: TEST_APP_CONFIG,
			renderFn: () => {},
		});

		api.__vormaClientGlobal.set("matchedPatterns", ["/hmr-second"]);

		const routeChangeListener = vi.fn();
		const removeRouteChangeListener =
			api.addRouteChangeListener(routeChangeListener);
		let afterUpdateHandler:
			| ((props: {
					updates: Array<{ type: string; path: string }>;
			  }) => void)
			| undefined;
		const fakeHot = {
			on: vi.fn((event: string, handler: unknown) => {
				if (
					event === "vite:afterUpdate" &&
					typeof handler === "function"
				) {
					afterUpdateHandler = handler as typeof afterUpdateHandler;
				}
			}),
		};

		api.__runClientLoadersAfterHMRUpdate(
			{
				url: "http://localhost:3000/src/routes/hmr-shared.tsx?t=1",
				hot: fakeHot,
			} as any,
			"/hmr-first",
		);
		api.__runClientLoadersAfterHMRUpdate(
			{
				url: "http://localhost:3000/src/routes/hmr-shared.tsx?t=2",
				hot: fakeHot,
			} as any,
			"/hmr-second",
		);

		expect(fakeHot.on).toHaveBeenCalledTimes(1);
		expect(afterUpdateHandler).toBeDefined();

		afterUpdateHandler?.({
			updates: [
				{
					type: "js-update",
					path: "/src/routes/hmr-shared.tsx?t=3",
				},
			],
		});

		await vi.advanceTimersByTimeAsync(11);
		await vi.runAllTimersAsync();
		expect(routeChangeListener).toHaveBeenCalledTimes(1);

		removeRouteChangeListener();
	});

	it("registers same-path HMR listeners for new hot runtimes across repeated init calls", async () => {
		const api = await loadClientAPI();
		await api.initClient({
			vormaAppConfig: TEST_APP_CONFIG,
			renderFn: () => {},
		});

		const firstHotRuntime = {
			on: vi.fn(),
		};
		api.__runClientLoadersAfterHMRUpdate(
			{
				url: "http://localhost:3000/src/routes/hmr-reinit.tsx?t=1",
				hot: firstHotRuntime,
			} as any,
			"/hmr-reinit-first",
		);
		expect(firstHotRuntime.on).toHaveBeenCalledWith(
			"vite:afterUpdate",
			expect.any(Function),
		);

		await api.initClient({
			vormaAppConfig: TEST_APP_CONFIG,
			renderFn: () => {},
		});

		const secondHotRuntime = {
			on: vi.fn(),
		};
		api.__runClientLoadersAfterHMRUpdate(
			{
				url: "http://localhost:3000/src/routes/hmr-reinit.tsx?t=2",
				hot: secondHotRuntime,
			} as any,
			"/hmr-reinit-second",
		);
		expect(secondHotRuntime.on).toHaveBeenCalledWith(
			"vite:afterUpdate",
			expect.any(Function),
		);
	});

	it("does not throw or refresh when matched patterns are unavailable during HMR updates", async () => {
		const api = await loadClientAPI();
		await api.initClient({
			vormaAppConfig: TEST_APP_CONFIG,
			renderFn: () => {},
		});

		api.__vormaClientGlobal.set("matchedPatterns", undefined as any);

		const routeChangeListener = vi.fn();
		const removeRouteChangeListener =
			api.addRouteChangeListener(routeChangeListener);
		let afterUpdateHandler:
			| ((props: {
					updates: Array<{ type: string; path: string }>;
			  }) => void)
			| undefined;
		const fakeHot = {
			on: vi.fn((event: string, handler: unknown) => {
				if (
					event === "vite:afterUpdate" &&
					typeof handler === "function"
				) {
					afterUpdateHandler = handler as typeof afterUpdateHandler;
				}
			}),
		};

		api.__runClientLoadersAfterHMRUpdate(
			{
				url: "http://localhost:3000/src/routes/hmr-match.tsx?t=1",
				hot: fakeHot,
			} as any,
			"/hmr-match",
		);

		expect(afterUpdateHandler).toBeDefined();
		expect(() =>
			afterUpdateHandler?.({
				updates: [
					{
						type: "js-update",
						path: "/src/routes/hmr-match.tsx?t=2",
					},
				],
			}),
		).not.toThrow();
		await vi.advanceTimersByTimeAsync(11);
		await vi.runAllTimersAsync();
		expect(routeChangeListener).not.toHaveBeenCalled();

		removeRouteChangeListener();
	});

	it("initializes route matcher state from precompiled manifest payload without progressive fetch", async () => {
		const api = await loadClientAPI();
		const manifest = {
			"/precompiled/:id": 1,
		};
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValue(new Response("unused"));
		const patternRegistry = api.__vormaClientGlobal.get("patternRegistry");
		expect(findBestMatch(patternRegistry, "/precompiled/123")).toBeNull();

		api.__vormaClientGlobal.set("routeManifest", manifest);
		api.__vormaClientGlobal.set(
			"routeManifestURL",
			"http://localhost:3000/manifest.json",
		);

		await api.initClient({
			vormaAppConfig: TEST_APP_CONFIG,
			renderFn: () => {},
		});
		await flushProgressiveManifestMicrotasks();

		expect(fetchSpy).not.toHaveBeenCalled();
		expect(api.__vormaClientGlobal.get("routeManifest")).toEqual(manifest);
		expect(
			findBestMatch(
				api.__vormaClientGlobal.get("patternRegistry"),
				"/precompiled/123",
			)?.registeredPattern.originalPattern,
		).toBe("/precompiled/:id");
	});

	it("falls back to progressive route-manifest loading when precompiled manifest is absent", async () => {
		const api = await loadClientAPI();
		const manifest = {
			"/progressive/:id": 1,
		};
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			new Response(JSON.stringify(manifest), {
				status: 200,
				headers: { "Content-Type": "application/json" },
			}),
		);
		const patternRegistry = api.__vormaClientGlobal.get("patternRegistry");
		expect(findBestMatch(patternRegistry, "/progressive/123")).toBeNull();

		api.__vormaClientGlobal.set(
			"routeManifestURL",
			"http://localhost:3000/manifest.json",
		);

		await api.initClient({
			vormaAppConfig: TEST_APP_CONFIG,
			renderFn: () => {},
		});
		await flushProgressiveManifestMicrotasks();

		expect(fetchSpy).toHaveBeenCalledWith(
			"http://localhost:3000/manifest.json",
		);
		expect(api.__vormaClientGlobal.get("routeManifest")).toEqual(manifest);
		expect(
			findBestMatch(
				api.__vormaClientGlobal.get("patternRegistry"),
				"/progressive/123",
			)?.registeredPattern.originalPattern,
		).toBe("/progressive/:id");
	});

	it("treats route-manifest progressive loading failures as non-fatal", async () => {
		const api = await loadClientAPI();
		const fetchError = new Error("manifest network failed");
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockRejectedValue(fetchError);
		const warnSpy = vi.spyOn(console, "warn");

		api.__vormaClientGlobal.set(
			"routeManifestURL",
			"http://localhost:3000/manifest.json",
		);

		await expect(
			api.initClient({
				vormaAppConfig: TEST_APP_CONFIG,
				renderFn: () => {},
			}),
		).resolves.toBeUndefined();
		await flushProgressiveManifestMicrotasks();

		expect(fetchSpy).toHaveBeenCalledWith(
			"http://localhost:3000/manifest.json",
		);
		expect(api.__vormaClientGlobal.get("routeManifest")).toBeUndefined();
		expect(warnSpy).toHaveBeenCalledWith(
			"Failed to load route manifest:",
			fetchError,
		);
	});

	it("rejects invalid route-manifest payload shapes without mutating registry state", async () => {
		const api = await loadClientAPI();
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			new Response(JSON.stringify(["/invalid-array-entry"]), {
				status: 200,
				headers: { "Content-Type": "application/json" },
			}),
		);
		const warnSpy = vi.spyOn(console, "warn");

		api.__vormaClientGlobal.set(
			"routeManifestURL",
			"http://localhost:3000/manifest.json",
		);

		await api.initClient({
			vormaAppConfig: TEST_APP_CONFIG,
			renderFn: () => {},
		});
		await flushProgressiveManifestMicrotasks();

		expect(fetchSpy).toHaveBeenCalledWith(
			"http://localhost:3000/manifest.json",
		);
		expect(api.__vormaClientGlobal.get("routeManifest")).toBeUndefined();
		expect(
			findBestMatch(
				api.__vormaClientGlobal.get("patternRegistry"),
				"/invalid-array-entry",
			),
		).toBeNull();
		const manifestWarningCall = warnSpy.mock.calls.find(
			(call) => call[0] === "Failed to load route manifest:",
		);
		expect(manifestWarningCall).toBeDefined();
		if (!manifestWarningCall) {
			return;
		}
		expect(manifestWarningCall[1]).toBeInstanceOf(Error);
		expect((manifestWarningCall[1] as Error).message).toContain(
			"non-null object",
		);
	});

	it("treats non-OK route-manifest responses as non-fatal and leaves state unchanged", async () => {
		const api = await loadClientAPI();
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			new Response("manifest unavailable", {
				status: 503,
				headers: { "Content-Type": "text/plain" },
			}),
		);
		const warnSpy = vi.spyOn(console, "warn");

		api.__vormaClientGlobal.set(
			"routeManifestURL",
			"http://localhost:3000/manifest.json",
		);

		await api.initClient({
			vormaAppConfig: TEST_APP_CONFIG,
			renderFn: () => {},
		});
		await flushProgressiveManifestMicrotasks();

		expect(fetchSpy).toHaveBeenCalledWith(
			"http://localhost:3000/manifest.json",
		);
		expect(api.__vormaClientGlobal.get("routeManifest")).toBeUndefined();
		const manifestWarningCall = warnSpy.mock.calls.find(
			(call) => call[0] === "Failed to load route manifest:",
		);
		expect(manifestWarningCall).toBeDefined();
		if (!manifestWarningCall) {
			return;
		}
		expect(manifestWarningCall[1]).toBeInstanceOf(Error);
		expect((manifestWarningCall[1] as Error).message).toContain(
			"status 503",
		);
	});

	it("rejects route-manifest entries whose loader flags are outside 0 or 1", async () => {
		const api = await loadClientAPI();
		const fetchSpy = vi.spyOn(window, "fetch").mockResolvedValue(
			new Response(
				JSON.stringify({
					"/invalid-loader-flag": 2,
				}),
				{
					status: 200,
					headers: { "Content-Type": "application/json" },
				},
			),
		);
		const warnSpy = vi.spyOn(console, "warn");

		api.__vormaClientGlobal.set(
			"routeManifestURL",
			"http://localhost:3000/manifest.json",
		);

		await api.initClient({
			vormaAppConfig: TEST_APP_CONFIG,
			renderFn: () => {},
		});
		await flushProgressiveManifestMicrotasks();

		expect(fetchSpy).toHaveBeenCalledWith(
			"http://localhost:3000/manifest.json",
		);
		expect(api.__vormaClientGlobal.get("routeManifest")).toBeUndefined();
		expect(
			findBestMatch(
				api.__vormaClientGlobal.get("patternRegistry"),
				"/invalid-loader-flag",
			),
		).toBeNull();
		const manifestWarningCall = warnSpy.mock.calls.find(
			(call) => call[0] === "Failed to load route manifest:",
		);
		expect(manifestWarningCall).toBeDefined();
		if (!manifestWarningCall) {
			return;
		}
		expect(manifestWarningCall[1]).toBeInstanceOf(Error);
		expect((manifestWarningCall[1] as Error).message).toContain("0 or 1");
	});

	it("ignores stale older progressive manifest responses after a newer init", async () => {
		const api = await loadClientAPI();
		const olderManifest = {
			"/stale-manifest-old/:id": 1,
		};
		const newerManifest = {
			"/stale-manifest-new/:id": 1,
		};
		const olderManifestDeferred = createDeferred<Response>();
		const newerManifestDeferred = createDeferred<Response>();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockImplementation((input) => {
				const href =
					typeof input === "string"
						? input
						: input instanceof URL
							? input.href
							: input.url;

				if (href === "http://localhost:3000/manifest-old.json") {
					return olderManifestDeferred.promise;
				}
				if (href === "http://localhost:3000/manifest-new.json") {
					return newerManifestDeferred.promise;
				}

				throw new Error(`Unexpected manifest fetch URL: ${href}`);
			});

		api.__vormaClientGlobal.set(
			"routeManifestURL",
			"http://localhost:3000/manifest-old.json",
		);
		await api.initClient({
			vormaAppConfig: TEST_APP_CONFIG,
			renderFn: () => {},
		});

		api.__vormaClientGlobal.set(
			"routeManifestURL",
			"http://localhost:3000/manifest-new.json",
		);
		await api.initClient({
			vormaAppConfig: TEST_APP_CONFIG,
			renderFn: () => {},
		});

		newerManifestDeferred.resolve(
			new Response(JSON.stringify(newerManifest), {
				status: 200,
				headers: { "Content-Type": "application/json" },
			}),
		);
		await vi.waitFor(() => {
			expect(api.__vormaClientGlobal.get("routeManifest")).toEqual(
				newerManifest,
			);
		});

		olderManifestDeferred.resolve(
			new Response(JSON.stringify(olderManifest), {
				status: 200,
				headers: { "Content-Type": "application/json" },
			}),
		);
		await flushProgressiveManifestMicrotasks();

		expect(fetchSpy).toHaveBeenNthCalledWith(
			1,
			"http://localhost:3000/manifest-old.json",
		);
		expect(fetchSpy).toHaveBeenNthCalledWith(
			2,
			"http://localhost:3000/manifest-new.json",
		);
		expect(api.__vormaClientGlobal.get("routeManifest")).toEqual(
			newerManifest,
		);
		expect(
			findBestMatch(
				api.__vormaClientGlobal.get("patternRegistry"),
				"/stale-manifest-new/123",
			)?.registeredPattern.originalPattern,
		).toBe("/stale-manifest-new/:id");
		expect(
			findBestMatch(
				api.__vormaClientGlobal.get("patternRegistry"),
				"/stale-manifest-old/123",
			),
		).toBeNull();
	});

	it("ignores progressive manifest payload when pattern registry was replaced mid-flight", async () => {
		const api = await loadClientAPI();
		const deferredManifestResponse = createDeferred<Response>();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockImplementation(() => deferredManifestResponse.promise);

		api.__vormaClientGlobal.set(
			"routeManifestURL",
			"http://localhost:3000/manifest.json",
		);
		await api.initClient({
			vormaAppConfig: TEST_APP_CONFIG,
			renderFn: () => {},
		});

		const replacementRegistry = createPatternRegistry({
			dynamicParamPrefixRune: TEST_APP_CONFIG.loadersDynamicRune,
			splatSegmentRune: TEST_APP_CONFIG.loadersSplatRune,
			explicitIndexSegment:
				TEST_APP_CONFIG.loadersExplicitIndexSegmentIdentifier,
		});
		api.__vormaClientGlobal.set("patternRegistry", replacementRegistry);

		deferredManifestResponse.resolve(
			new Response(
				JSON.stringify({
					"/manifest-should-be-ignored/:id": 1,
				}),
				{
					status: 200,
					headers: { "Content-Type": "application/json" },
				},
			),
		);
		await flushProgressiveManifestMicrotasks();

		expect(fetchSpy).toHaveBeenCalledWith(
			"http://localhost:3000/manifest.json",
		);
		expect(api.__vormaClientGlobal.get("routeManifest")).toBeUndefined();
		expect(
			findBestMatch(
				api.__vormaClientGlobal.get("patternRegistry"),
				"/manifest-should-be-ignored/123",
			),
		).toBeNull();
	});
});
