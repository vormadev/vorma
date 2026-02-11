import { describe, expect, it, vi } from "vitest";
import {
	installContractVormaGlobal,
	loadClientAPI,
	setupContractTestSuite,
} from "./contract_test_harness.ts";

setupContractTestSuite();

describe("client utility contracts", () => {
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

	it("applies coordinate and hash-based scroll states", async () => {
		const api = await loadClientAPI();

		api.__applyScrollState({ x: 100, y: 200 });
		expect(window.scrollTo).toHaveBeenCalledWith(100, 200);

		const hashElement = document.createElement("div");
		hashElement.id = "test-hash";
		document.body.appendChild(hashElement);
		const hashScrollSpy = vi.spyOn(hashElement, "scrollIntoView");

		api.__applyScrollState({ hash: "test-hash" });
		expect(hashScrollSpy).toHaveBeenCalled();

		window.location.hash = "#url-hash";
		const urlHashElement = document.createElement("div");
		urlHashElement.id = "url-hash";
		document.body.appendChild(urlHashElement);
		const urlHashScrollSpy = vi.spyOn(urlHashElement, "scrollIntoView");

		api.__applyScrollState(undefined);
		expect(urlHashScrollSpy).toHaveBeenCalled();
	});

	it("decodes encoded hash fragments before element lookup", async () => {
		const api = await loadClientAPI();

		const unicodeElement = document.createElement("div");
		unicodeElement.id = "✓";
		document.body.appendChild(unicodeElement);
		const unicodeScrollSpy = vi.spyOn(unicodeElement, "scrollIntoView");

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
		const fallbackScrollSpy = vi.spyOn(fallbackElement, "scrollIntoView");

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
		const symbol = Symbol.for("__vorma_internal__");
		const currentGlobal = (globalThis as any)[symbol];
		installContractVormaGlobal({
			...currentGlobal,
			buildID: "test-build-12345",
		});

		expect(api.getBuildID()).toBe("test-build-12345");
	});
});
