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
	describe("13. Utility Functions", () => {
		describe("13.1 Listener Management", () => {
			it("should return cleanup function for removing listeners", () => {
				const listener = vi.fn();
				const cleanup = addStatusListener(listener);

				// Trigger event
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

				// Clean up
				cleanup();

				// Trigger again
				window.dispatchEvent(
					new CustomEvent("vorma:status", {
						detail: {
							isNavigating: true,
							isSubmitting: false,
							isRevalidating: false,
						},
					}),
				);

				// Should not be called again
				expect(listener).toHaveBeenCalledTimes(1);
			});

			it("should use window as event target for all listeners", () => {
				const addEventListenerSpy = vi.spyOn(
					window,
					"addEventListener",
				);

				addStatusListener(() => {});
				addRouteChangeListener(() => {});
				addLocationListener(() => {});
				addBuildIDListener(() => {});

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
		});

		describe("13.2 Public Utilities", () => {
			it("should return root element via getRootEl()", () => {
				const root = document.createElement("div");
				root.id = "vorma-root";
				document.body.appendChild(root);

				expect(getRootEl()).toBe(root);
			});

			it("should apply scroll state correctly", () => {
				// Test coordinate scroll
				__applyScrollState({ x: 100, y: 200 });
				expect(window.scrollTo).toHaveBeenCalledWith(100, 200);

				// Test hash scroll
				const element = document.createElement("div");
				element.id = "test-hash";
				document.body.appendChild(element);
				const scrollSpy = vi.spyOn(element, "scrollIntoView");

				__applyScrollState({ hash: "test-hash" });
				expect(scrollSpy).toHaveBeenCalled();

				// Test no state with hash in URL
				window.location.hash = "#url-hash";
				const urlElement = document.createElement("div");
				urlElement.id = "url-hash";
				document.body.appendChild(urlElement);
				const urlScrollSpy = vi.spyOn(urlElement, "scrollIntoView");

				__applyScrollState(undefined);
				expect(urlScrollSpy).toHaveBeenCalled();
			});

			it("should return current location parts", () => {
				window.history.replaceState(
					{},
					"",
					"/test/path?query=value#section",
				);

				const location = getLocation();
				expect(location).toEqual({
					pathname: "/test/path",
					search: "?query=value",
					hash: "#section",
					state: null,
				});
			});

			it("should return current build ID", () => {
				setupGlobalVormaContext({ buildID: "test-build-12345" });
				expect(getBuildID()).toBe("test-build-12345");
			});
		});
	});

});
