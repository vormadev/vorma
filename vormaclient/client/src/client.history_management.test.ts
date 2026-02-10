// READY_TO_DELETE_AFTER_SIGNOFF
import { describe, expect, it, vi } from "vitest";

import { getHistoryInstance } from "./client";

import { addLocationListener } from "./events.ts";

import { customHistoryListener } from "./history/history.ts";

import { __applyScrollState } from "./scroll_state_manager.ts";

import {
	createMockResponse,
	describeNavigationTestSuite,
} from "./client.test.helpers.ts";

describeNavigationTestSuite(() => {
	describe("11. History Management", () => {
		describe("11.1 Custom History", () => {
			it("should create browser history instance", () => {
				const history = getHistoryInstance();
				expect(history).toBeDefined();
				expect(history.location).toBeDefined();
				expect(history.push).toBeDefined();
				expect(history.replace).toBeDefined();
			});

			it("should maintain lastKnownCustomLocation", async () => {
				const history = getHistoryInstance();

				history.push("/new-location");
				await vi.runAllTimersAsync();

				// Location should be updated after push
				expect(history.location.pathname).toBe("/new-location");
			});
		});

		describe("11.2 POP Event Handling", () => {
			it("should dispatch location event on key change", async () => {
				const locationListener = vi.fn();
				addLocationListener(locationListener);

				const originalKey = getHistoryInstance().location.key;

				const update = {
					action: "PUSH",
					location: {
						pathname: "/trigger-key-change",
						search: "",
						hash: "",
						state: null,
						key: `${originalKey}-modified`,
					},
				};

				await customHistoryListener(update as any);

				expect(locationListener).toHaveBeenCalled();
			});

			it("should handle hash-only changes within same document", async () => {
				const history = getHistoryInstance();
				history.push("/same-doc");

				// Add hash
				history.push("/same-doc#new-hash");

				// Mock scrollIntoView
				const element = document.createElement("div");
				element.id = "new-hash";
				document.body.appendChild(element);
				const scrollSpy = vi.spyOn(element, "scrollIntoView");

				// Trigger POP
				history.back();
				history.forward();
				await vi.runAllTimersAsync();

				__applyScrollState({ hash: "new-hash" });
				expect(scrollSpy).toHaveBeenCalled();
			});

			it("should trigger full navigation for different documents", async () => {
				vi.mocked(fetch).mockResolvedValue(
					createMockResponse({ importURLs: [], cssBundles: [] }),
				);

				const history = getHistoryInstance();
				history.push("/page1");
				const page1Location = { ...history.location };
				history.push("/page2");

				vi.clearAllMocks();

				// Simulate the browser updating its location after a 'back' navigation
				window.location.pathname = page1Location.pathname;
				window.location.search = page1Location.search;
				window.location.hash = page1Location.hash;
				window.location.href = new URL(
					`${page1Location.pathname}${page1Location.search}${page1Location.hash}`,
					"http://localhost:3000",
				).href;

				// Dispatch the popstate event to trigger the history library's listener
				window.dispatchEvent(
					new PopStateEvent("popstate", {
						state: { key: page1Location.key },
					}),
				);

				await vi.runAllTimersAsync();

				// This assertion is more specific and robust.
				expect(fetch).toHaveBeenCalledWith(
					expect.objectContaining({
						href: expect.stringContaining("/page1?vorma_json="),
					}),
					expect.any(Object),
				);
			});

			it("should save scroll before navigating away", async () => {
				const history = getHistoryInstance();
				history.push("/current");

				(window as any).scrollX = 123;
				(window as any).scrollY = 456;

				// Navigate to different page
				history.push("/different");
				await vi.runAllTimersAsync();

				const saved = JSON.parse(
					sessionStorage.getItem("__vorma__scrollStateMap") || "[]",
				);
				expect(saved).toContainEqual([
					expect.any(String),
					{ x: 123, y: 456 },
				]);
			});
		});
	});
});
