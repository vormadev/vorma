import { describe, expect, it, vi } from "vitest";

import { __getPrefetchHandlers, __makeLinkOnClickFn } from "./links.ts";

import {
	createMockResponse,
	describeNavigationTestSuite,
} from "./client.test.helpers.ts";

describeNavigationTestSuite(() => {
	describe("Link conformance", () => {
		it("FEC-LINK-001_FE-LINK-001_prefetch_handlers_exist_only_for_internal_http_targets", () => {
			const internal = __getPrefetchHandlers({ href: "/internal" });
			expect(internal).toBeDefined();
			expect(typeof internal?.start).toBe("function");
			expect(typeof internal?.stop).toBe("function");
			expect(typeof internal?.onClick).toBe("function");

			const external = __getPrefetchHandlers({
				href: "https://external.example.com/path",
			});
			expect(external).toBeUndefined();

			const nonHTTP = __getPrefetchHandlers({ href: "mailto:test@example.com" });
			expect(nonHTTP).toBeUndefined();
		});

		it("FEC-LINK-002_FE-LINK-002_FE-LINK-003_prefetch_delay_and_stop_behavior", async () => {
			vi.mocked(fetch).mockResolvedValue(
				createMockResponse({ importURLs: [], cssBundles: [] }),
			);

			const delayed = __getPrefetchHandlers({ href: "/delayed" });
			delayed?.start({} as Event);
			await vi.advanceTimersByTimeAsync(90);
			expect(fetch).not.toHaveBeenCalled();
			await vi.advanceTimersByTimeAsync(10);
			expect(fetch).toHaveBeenCalledTimes(1);

			vi.clearAllMocks();
			const cancelBeforeStart = __getPrefetchHandlers({ href: "/cancel-before" });
			cancelBeforeStart?.start({} as Event);
			cancelBeforeStart?.stop();
			await vi.advanceTimersByTimeAsync(200);
			expect(fetch).not.toHaveBeenCalled();

			vi.clearAllMocks();
			vi.mocked(fetch).mockImplementation(() => new Promise(() => {}));
			const cancelInFlight = __getPrefetchHandlers({ href: "/cancel-after" });
			cancelInFlight?.start({} as Event);
			await vi.advanceTimersByTimeAsync(100);
			expect(fetch).toHaveBeenCalledTimes(1);

			const signal = vi.mocked(fetch).mock.calls[0]?.[1]?.signal;
			expect(signal?.aborted).toBe(false);
			cancelInFlight?.stop();
			expect(signal?.aborted).toBe(true);
		});

		it("FEC-LINK-003_FE-LINK-004_hash_only_click_saves_scroll_without_route_fetch", async () => {
			window.history.pushState({}, "", "/same-document");

			const e = new MouseEvent("click", {
				bubbles: true,
				cancelable: true,
			});
			const anchor = document.createElement("a");
			anchor.href = "/same-document#section-a";
			Object.defineProperty(e, "target", { value: anchor });

			const onClick = __makeLinkOnClickFn({});
			await onClick(e);

			expect(fetch).not.toHaveBeenCalled();
			const raw = sessionStorage.getItem("__vorma__scrollStateMap");
			expect(raw).toBeTruthy();
			const parsed = JSON.parse(raw || "[]");
			expect(Array.isArray(parsed)).toBe(true);
			expect(parsed.length).toBeGreaterThan(0);
		});

		it("FEC-LINK-004_FE-LINK-005_FE-LINK-006_internal_click_hijack_and_callback_ordering", async () => {
			vi.mocked(fetch).mockResolvedValue(
				createMockResponse({ importURLs: [], cssBundles: [] }),
			);

			const callbacks: string[] = [];
			const onClick = __makeLinkOnClickFn({
				beforeBegin: () => callbacks.push("beforeBegin"),
				beforeRender: () => callbacks.push("beforeRender"),
				afterRender: () => callbacks.push("afterRender"),
			});

			const internalEvent = new MouseEvent("click", {
				bubbles: true,
				cancelable: true,
			});
			const preventDefault = vi.spyOn(internalEvent, "preventDefault");
			const internalAnchor = document.createElement("a");
			internalAnchor.href = "/internal-target";
			Object.defineProperty(internalEvent, "target", {
				value: internalAnchor,
			});

			await onClick(internalEvent);
			await vi.runAllTimersAsync();

			expect(preventDefault).toHaveBeenCalledTimes(1);
			expect(callbacks).toEqual([
				"beforeBegin",
				"beforeRender",
				"afterRender",
			]);

			const externalEvent = new MouseEvent("click", {
				bubbles: true,
				cancelable: true,
				ctrlKey: true,
			});
			const externalPreventDefault = vi.spyOn(externalEvent, "preventDefault");
			const externalAnchor = document.createElement("a");
			externalAnchor.href = "https://external.example.com";
			Object.defineProperty(externalEvent, "target", {
				value: externalAnchor,
			});

			await onClick(externalEvent);
			expect(externalPreventDefault).not.toHaveBeenCalled();
			expect(callbacks).toEqual([
				"beforeBegin",
				"beforeRender",
				"afterRender",
			]);
		});
	});
});

