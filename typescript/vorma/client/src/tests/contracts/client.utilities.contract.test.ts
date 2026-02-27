import { describe, expect, it, vi } from "vitest";
import {
	createAbortAwareFetchRecorder,
	createRouteDataResponse,
	loadClientAPI,
	patchContractRuntimeRouteSnapshot,
	requestInputToURL,
	setupContractTestSuite,
	waitForRequestCount,
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

function createAnchorClickEvent(href: string): MouseEvent {
	const event = new MouseEvent("click", {
		bubbles: true,
		cancelable: true,
	});
	const anchor = document.createElement("a");
	anchor.href = href;
	Object.defineProperty(event, "target", { value: anchor });
	return event;
}

describe("client utility contracts", () => {
	it("does not expose buildtime-only route registration from the runtime client entry", async () => {
		const api = await loadClientAPI();
		expect("route" in api).toBe(false);
		expect((api as Record<string, unknown>).route).toBeUndefined();
	});

	it("does not expose unstable internal __* helpers from the runtime client entry", async () => {
		const publicApi = await import("../../../index.ts");
		const publicApiAsRecord = publicApi as Record<string, unknown>;

		expect(publicApiAsRecord.__registerClientLoaderPattern).toBeUndefined();
		expect(
			publicApiAsRecord.__runClientLoadersAfterHMRUpdate,
		).toBeUndefined();
		expect(
			publicApiAsRecord.__registerClientLoaderForAdapter,
		).toBeUndefined();
		expect(publicApiAsRecord.__applyScrollState).toBeUndefined();
		expect(publicApiAsRecord.__makeFinalLinkProps).toBeUndefined();
		expect(publicApiAsRecord.__resolvePath).toBeUndefined();
		expect(publicApiAsRecord.__getClientRuntimeRenderState).toBeUndefined();
		expect(publicApiAsRecord.__setClientLoaderWaitFn).toBeUndefined();
	});

	it("exposes route registration from the buildtime entry", async () => {
		const buildtimeApi = await import("../../../buildtime.ts");

		expect(typeof buildtimeApi.route).toBe("function");
		expect(() =>
			buildtimeApi.route(
				"/",
				Promise.resolve({
					default: () => null,
				}),
				"default",
			),
		).not.toThrow();
	});

	it("formats errors through defaultErrorBoundary", async () => {
		const api = await loadClientAPI();
		expect(api.defaultErrorBoundary({ error: "boom" })).toBe(
			"Route Error: boom",
		);
	});

	it("returns listener cleanup functions that unsubscribe handlers", async () => {
		const api = await loadClientAPI();
		const listener = vi.fn();
		const cleanup = api.addStatusListener(listener);

		window.dispatchEvent(
			new CustomEvent("vorma:status", {
				detail: {
					isNavigating: false,
					isSubmitting: false,
					isRevalidating: false,
				},
			}),
		);
		expect(listener).toHaveBeenCalledTimes(1);

		cleanup();

		window.dispatchEvent(
			new CustomEvent("vorma:status", {
				detail: {
					isNavigating: true,
					isSubmitting: false,
					isRevalidating: false,
				},
			}),
		);
		expect(listener).toHaveBeenCalledTimes(1);
	});

	it("registers listeners on window for all event types", async () => {
		const api = await loadClientAPI();
		const addEventListenerSpy = vi.spyOn(window, "addEventListener");

		api.addStatusListener(() => {});
		api.addRouteChangeListener(() => {});
		api.addLocationListener(() => {});
		api.addBuildIDListener(() => {});

		expect(addEventListenerSpy).toHaveBeenCalledWith(
			"vorma:status",
			expect.any(Function),
		);
		expect(addEventListenerSpy).toHaveBeenCalledWith(
			"vorma:route-change",
			expect.any(Function),
		);
		expect(addEventListenerSpy).toHaveBeenCalledWith(
			"vorma:location",
			expect.any(Function),
		);
		expect(addEventListenerSpy).toHaveBeenCalledWith(
			"vorma:build-id",
			expect.any(Function),
		);
	});

	it("returns #vorma-root via getRootEl()", async () => {
		const api = await loadClientAPI();
		const root = document.createElement("div");
		root.id = "vorma-root";
		document.body.appendChild(root);

		expect(api.getRootEl()).toBe(root);
	});

	it("throws when #vorma-root is missing", async () => {
		const api = await loadClientAPI();

		expect(() => api.getRootEl()).toThrow(
			'Expected element with id "vorma-root" to exist',
		);
	});

	it("returns #vorma-root when root is a non-div HTMLElement", async () => {
		const api = await loadClientAPI();
		const root = document.createElement("main");
		root.id = "vorma-root";
		document.body.appendChild(root);

		expect(api.getRootEl()).toBe(root);
	});

	it("uses configured root element id from SSR runtime state", async () => {
		const api = await loadClientAPI();
		patchContractRuntimeRouteSnapshot({
			api,
			patch: {
				rootElementID: "app-root-custom",
			},
		});

		const root = document.createElement("div");
		root.id = "app-root-custom";
		document.body.appendChild(root);

		expect(api.getRootEl()).toBe(root);
	});

	it("applies coordinate and hash-based scroll states", async () => {
		const api = await loadClientAPI();

		api.__applyScrollState({ x: 100, y: 200 });
		expect(window.scrollTo).toHaveBeenCalledWith(100, 200);

		const hashElement = document.createElement("div");
		hashElement.id = "test-hash";
		document.body.appendChild(hashElement);
		const hashScrollSpy = stubElementScrollIntoView(hashElement);

		api.__applyScrollState({ hash: "test-hash" });
		expect(hashScrollSpy).toHaveBeenCalled();

		window.location.hash = "#url-hash";
		const urlHashElement = document.createElement("div");
		urlHashElement.id = "url-hash";
		document.body.appendChild(urlHashElement);
		const urlHashScrollSpy = stubElementScrollIntoView(urlHashElement);

		api.__applyScrollState(undefined);
		expect(urlHashScrollSpy).toHaveBeenCalled();
	});

	it("decodes encoded hash fragments before element lookup", async () => {
		const api = await loadClientAPI();

		const unicodeElement = document.createElement("div");
		unicodeElement.id = "✓";
		document.body.appendChild(unicodeElement);
		const unicodeScrollSpy = stubElementScrollIntoView(unicodeElement);

		api.__applyScrollState({ hash: "%E2%9C%93" });
		expect(unicodeScrollSpy).toHaveBeenCalledTimes(1);

		window.location.hash = "#%E2%9C%93";
		api.__applyScrollState(undefined);
		expect(unicodeScrollSpy).toHaveBeenCalledTimes(2);
	});

	it("falls back to raw hash fragments when decode fails", async () => {
		const api = await loadClientAPI();
		const invalidEncodedHash = "%E0%A4%A";

		const fallbackElement = document.createElement("div");
		fallbackElement.id = invalidEncodedHash;
		document.body.appendChild(fallbackElement);
		const fallbackScrollSpy = stubElementScrollIntoView(fallbackElement);

		expect(() =>
			api.__applyScrollState({ hash: invalidEncodedHash }),
		).not.toThrow();
		expect(fallbackScrollSpy).toHaveBeenCalledTimes(1);

		window.location.hash = `#${invalidEncodedHash}`;
		expect(() => api.__applyScrollState(undefined)).not.toThrow();
		expect(fallbackScrollSpy).toHaveBeenCalledTimes(2);
	});

	it("returns current URL parts from getLocation()", async () => {
		const api = await loadClientAPI();
		window.history.replaceState({}, "", "/test/path?query=value#section");

		expect(api.getLocation()).toEqual({
			pathname: "/test/path",
			search: "?query=value",
			hash: "#section",
			state: null,
		});
	});

	it("returns current build ID from getBuildID()", async () => {
		const api = await loadClientAPI();
		patchContractRuntimeRouteSnapshot({
			api,
			patch: {
				buildID: "test-build-12345",
			},
		});

		expect(api.getBuildID()).toBe("test-build-12345");
	});

	it("builds typed navigation hrefs and forwards navigation options", async () => {
		const api = await loadClientAPI();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValue(createRouteDataResponse());
		const typedNavigate = api.makeTypedNavigate(TEST_APP_CONFIG as any);

		await typedNavigate({
			pattern: "/users/:id",
			params: {
				id: "a/b",
			},
			search: "?tab=activity",
			hash: "#details",
			replace: true,
			scrollToTop: false,
			state: { from: "tests" },
		} as any);

		expect(fetchSpy).toHaveBeenCalledTimes(1);
		const fetchInput = fetchSpy.mock.calls[0]?.[0] as RequestInfo | URL;
		const fetchURL = requestInputToURL(fetchInput);
		expect(fetchURL.pathname).toBe("/users/a%2Fb");
		expect(fetchURL.searchParams.get("tab")).toBe("activity");
		expect(fetchURL.searchParams.get("vorma_json")).toBe("1");
		expect(api.getLocation().pathname).toBe("/users/a%2Fb");
		expect(api.getLocation().search).toBe("?tab=activity");
		expect(api.getLocation().hash).toBe("#details");
	});

	it("builds typed navigation hrefs from splat values", async () => {
		const api = await loadClientAPI();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValue(createRouteDataResponse());
		const typedNavigate = api.makeTypedNavigate(TEST_APP_CONFIG as any);

		await typedNavigate({
			pattern: "/docs/*",
			splatValues: ["guides", "intro"],
		} as any);

		expect(fetchSpy).toHaveBeenCalledTimes(1);
		const fetchInput = fetchSpy.mock.calls[0]?.[0] as RequestInfo | URL;
		const fetchURL = requestInputToURL(fetchInput);
		expect(fetchURL.pathname).toBe("/docs/guides/intro");
	});

	it("maps custom handler keys in __makeFinalLinkProps and preserves callbacks", async () => {
		const api = await loadClientAPI();
		const onMouseEnter = vi.fn();
		const onFocusIn = vi.fn();
		const onMouseLeave = vi.fn();
		const onBlurred = vi.fn();
		const onCancel = vi.fn();
		const onPress = vi.fn();
		const event = { defaultPrevented: true };

		const finalProps = api.__makeFinalLinkProps(
			{
				onMouseEnter,
				onFocusIn,
				onMouseLeave,
				onBlurred,
				onCancel,
				onPress,
			} as any,
			{
				onPointerEnter: "onMouseEnter",
				onFocus: "onFocusIn",
				onPointerLeave: "onMouseLeave",
				onBlur: "onBlurred",
				onTouchCancel: "onCancel",
				onClick: "onPress",
			},
		);

		finalProps.onPointerEnter(event);
		finalProps.onFocus(event);
		finalProps.onPointerLeave(event);
		finalProps.onBlur(event);
		finalProps.onTouchCancel(event);
		await finalProps.onClick(event);

		expect(finalProps.dataExternal).toBeUndefined();
		expect(onMouseEnter).toHaveBeenCalledWith(event);
		expect(onFocusIn).toHaveBeenCalledWith(event);
		expect(onMouseLeave).toHaveBeenCalledWith(event);
		expect(onBlurred).toHaveBeenCalledWith(event);
		expect(onCancel).toHaveBeenCalledWith(event);
		expect(onPress).toHaveBeenCalledWith(event);
	});

	it("marks external hrefs with dataExternal in __makeFinalLinkProps", async () => {
		const api = await loadClientAPI();
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValue(createRouteDataResponse());
		const finalProps = api.__makeFinalLinkProps({
			href: "https://example.com/external",
		} as any);

		const event = createAnchorClickEvent("https://example.com/external");
		const preventDefault = vi.spyOn(event, "preventDefault");
		await finalProps.onClick(event);

		expect(finalProps.dataExternal).toBe(true);
		expect(preventDefault).not.toHaveBeenCalled();
		expect(fetchSpy).not.toHaveBeenCalled();
	});

	it("prevents default for same-document no-op clicks through __makeFinalLinkProps", async () => {
		const api = await loadClientAPI();
		window.history.replaceState({}, "", "/current-page");
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValue(createRouteDataResponse());
		const finalProps = api.__makeFinalLinkProps({
			href: "/current-page",
		} as any);

		const event = createAnchorClickEvent("/current-page");
		const preventDefault = vi.spyOn(event, "preventDefault");
		await finalProps.onClick(event);

		expect(preventDefault).toHaveBeenCalledTimes(1);
		expect(fetchSpy).not.toHaveBeenCalled();
	});

	it("prevents default for same-document no-op prefetch clicks through __makeFinalLinkProps", async () => {
		const api = await loadClientAPI();
		window.history.replaceState({}, "", "/current-page");
		const fetchSpy = vi
			.spyOn(window, "fetch")
			.mockResolvedValue(createRouteDataResponse());
		const finalProps = api.__makeFinalLinkProps({
			href: "/current-page",
			prefetch: "intent",
			prefetchDelayMs: 200,
		} as any);

		finalProps.onPointerEnter(new Event("pointerenter"));

		const event = createAnchorClickEvent("/current-page");
		const preventDefault = vi.spyOn(event, "preventDefault");
		await finalProps.onClick(event);
		await vi.advanceTimersByTimeAsync(200);

		expect(preventDefault).toHaveBeenCalledTimes(1);
		expect(fetchSpy).not.toHaveBeenCalled();
	});

	it("supports intent-prefetch link props without requiring user callbacks", async () => {
		const api = await loadClientAPI();
		api.__vormaClientGlobal.set("isTouchInputModalityActive", true);
		const finalProps = api.__makeFinalLinkProps({
			href: "/prefetch-only",
			prefetch: "intent",
			prefetchDelayMs: 0,
		} as any);

		expect(finalProps.dataExternal).toBeUndefined();
		finalProps.onPointerEnter(new Event("pointerenter"));
		finalProps.onFocus(new Event("focus"));
		finalProps.onPointerLeave(new Event("pointerleave"));
		finalProps.onBlur(new Event("blur"));
		finalProps.onTouchCancel(new Event("touchcancel"));
		await finalProps.onClick({ defaultPrevented: true } as any);
		await vi.runAllTimersAsync();
	});

	it("cancels idle prefetch after touch when pointer modality switches back to mouse", async () => {
		const api = await loadClientAPI();
		await api.initClient({
			vormaAppConfig: TEST_APP_CONFIG,
			renderFn: () => {},
		});
		const { requests } = createAbortAwareFetchRecorder();
		const finalProps = api.__makeFinalLinkProps({
			href: "/hybrid-prefetch",
			prefetch: "intent",
			prefetchDelayMs: 0,
		} as any);

		window.dispatchEvent(new Event("touchstart"));
		expect(api.__vormaClientGlobal.get("isTouchInputModalityActive")).toBe(
			true,
		);

		finalProps.onPointerEnter(new Event("pointerenter"));
		await vi.advanceTimersByTimeAsync(1);
		await waitForRequestCount({ requests, count: 1 });
		expect(requests[0]?.signal?.aborted).toBe(false);

		const pointerMoveEvent = new Event("pointermove");
		Object.defineProperty(pointerMoveEvent, "pointerType", {
			value: "mouse",
		});
		window.dispatchEvent(pointerMoveEvent);
		expect(api.__vormaClientGlobal.get("isTouchInputModalityActive")).toBe(
			false,
		);

		finalProps.onPointerLeave(new Event("pointerleave"));
		expect(requests[0]?.signal?.aborted).toBe(true);
	});
});
