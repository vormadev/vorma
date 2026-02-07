import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { createPatternRegistry } from "vorma/kit/matcher/register";
import {
	beginNavigation,
	getBuildID,
	getHistoryInstance,
	getLocation,
	getRootEl,
	getStatus,
	navigationStateManager,
	revalidate,
	submit,
	vormaNavigate,
} from "./client";
import {
	addBuildIDListener,
	addLocationListener,
	addRouteChangeListener,
	addStatusListener,
	type RouteChangeEventDetail,
	type StatusEventDetail,
} from "./events.ts";
import { customHistoryListener, initCustomHistory } from "./history/history.ts";
import { initClient } from "./init_client.ts";
import { __getPrefetchHandlers, __makeLinkOnClickFn } from "./links.ts";
import {
	__applyScrollState,
	type ScrollState,
} from "./scroll_state_manager.ts";
import { __vormaClientGlobal } from "./vorma_ctx/vorma_ctx.ts";
import {
	createMockResponse,
	describeNavigationTestSuite,
	setupGlobalVormaContext,
	vormaAppConfig,
} from "./client.test.helpers.ts";

describeNavigationTestSuite(({ addCleanup, addListener }) => {
	describe("9. [Reserved]", () => {
		it("should have placeholder test", () => {
			expect(true).toBe(true);
		});
	});

	describe("10. Initialization", () => {
		it("should configure options correctly", async () => {
			const customErrorBoundary = () => "Custom Error";

			await initClient({
				vormaAppConfig,
				renderFn: () => {},
				defaultErrorBoundary: customErrorBoundary,
				useViewTransitions: true,
			});

			expect(__vormaClientGlobal.get("defaultErrorBoundary")).toBe(
				customErrorBoundary,
			);
			expect(__vormaClientGlobal.get("useViewTransitions")).toBe(true);
		});

		it("should initialize history with POP listener", async () => {
			const listenSpy = vi.spyOn(getHistoryInstance(), "listen");

			await initClient({ renderFn: () => {}, vormaAppConfig });

			expect(listenSpy).toHaveBeenCalled();
		});

		it("should set scrollRestoration to manual", async () => {
			const setterSpy = vi.fn();

			Object.defineProperty(window.history, "scrollRestoration", {
				get: () => "auto",
				set: setterSpy,
				configurable: true,
			});

			await initClient({ renderFn: () => {}, vormaAppConfig });

			expect(setterSpy).toHaveBeenCalledWith("manual");
		});

		it("should clean vorma_reload param from URL", async () => {
			window.history.replaceState(
				{},
				"",
				"/?vorma_reload=old-build&keep=this",
			);

			const replaceSpy = vi.spyOn(getHistoryInstance(), "replace");

			await initClient({ renderFn: () => {}, vormaAppConfig });

			expect(replaceSpy).toHaveBeenCalledWith(
				"http://localhost:3000/?keep=this",
			);
		});

		it("should load initial components", async () => {
			setupGlobalVormaContext({
				importURLs: ["/initial.js"],
				publicPathPrefix: "/",
			});

			vi.doMock("/initial.js", () => ({
				default: () => "Initial Component",
			}));

			await initClient({ renderFn: () => {}, vormaAppConfig });

			expect(__vormaClientGlobal.get("activeComponents")).toHaveLength(1);
		});

		it("should run initial client wait functions", async () => {
			const waitFn = vi.fn().mockResolvedValue({ initialized: true });

			setupGlobalVormaContext({
				patternToWaitFnMap: { "/": waitFn },
				matchedPatterns: ["/"],
				loadersData: [{ initial: "data" }],
			});

			await initClient({ renderFn: () => {}, vormaAppConfig });

			expect(waitFn).toHaveBeenCalled();
			expect(__vormaClientGlobal.get("clientLoadersData")).toEqual([
				{ initialized: true },
			]);
		});

		it("should execute user render function", async () => {
			const renderFn = vi.fn();

			await initClient({ renderFn, vormaAppConfig });

			expect(renderFn).toHaveBeenCalled();
		});

		it("should restore scroll after refresh", async () => {
			const scrollState = {
				x: 300,
				y: 600,
				unix: Date.now() - 1000,
				href: window.location.href,
			};

			sessionStorage.setItem(
				"__vorma__pageRefreshScrollState",
				JSON.stringify(scrollState),
			);

			const rafSpy = vi
				.spyOn(window, "requestAnimationFrame")
				.mockImplementation((cb) => {
					cb(0);
					return 0;
				});

			await initClient({ renderFn: () => {}, vormaAppConfig });

			expect(window.scrollTo).toHaveBeenCalledWith(300, 600);
			expect(
				sessionStorage.getItem("__vorma__pageRefreshScrollState"),
			).toBeNull();

			rafSpy.mockRestore();
		});

		it("should detect touch devices on first touch", async () => {
			await initClient({ renderFn: () => {}, vormaAppConfig });

			expect(__vormaClientGlobal.get("isTouchDevice")).toBeUndefined();

			window.dispatchEvent(new Event("touchstart"));

			expect(__vormaClientGlobal.get("isTouchDevice")).toBe(true);
		});
	});

});
