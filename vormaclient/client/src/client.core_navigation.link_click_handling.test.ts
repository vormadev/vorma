// READY_TO_DELETE_AFTER_SIGNOFF
import { describe, expect, it, vi } from "vitest";

import { __getPrefetchHandlers, __makeLinkOnClickFn } from "./links.ts";

import {
	createMockResponse,
	describeNavigationTestSuite,
} from "./client.test.helpers.ts";

describeNavigationTestSuite(() => {
	describe("1. Core Navigation", () => {
		describe("1.3 Link Click Handling", () => {
			it("should prevent default for eligible internal links", async () => {
				vi.mocked(fetch).mockResolvedValue(
					createMockResponse({ importURLs: [], cssBundles: [] }),
				);

				const event = new MouseEvent("click", { bubbles: true });
				const preventDefault = vi.spyOn(event, "preventDefault");
				const anchor = document.createElement("a");
				anchor.href = "/internal-link";

				Object.defineProperty(event, "target", { value: anchor });

				const onClick = __makeLinkOnClickFn({});
				await onClick(event);

				expect(preventDefault).toHaveBeenCalled();

				await vi.runAllTimersAsync();
			});

			it("should ignore external links", async () => {
				const event = new MouseEvent("click", { bubbles: true });
				const preventDefault = vi.spyOn(event, "preventDefault");
				const anchor = document.createElement("a");
				anchor.href = "https://external.com";

				Object.defineProperty(event, "target", { value: anchor });

				const onClick = __makeLinkOnClickFn({});
				await onClick(event);

				expect(preventDefault).not.toHaveBeenCalled();
			});

			it("should ignore clicks with modifier keys", async () => {
				const event = new MouseEvent("click", {
					bubbles: true,
					ctrlKey: true,
				});
				const preventDefault = vi.spyOn(event, "preventDefault");
				const anchor = document.createElement("a");
				anchor.href = "/internal";

				Object.defineProperty(event, "target", { value: anchor });

				const onClick = __makeLinkOnClickFn({});
				await onClick(event);

				expect(preventDefault).not.toHaveBeenCalled();
			});

			it("should handle hash-only links without navigation", async () => {
				window.history.pushState({}, "", "/current-page");

				const event = new MouseEvent("click", { bubbles: true });
				const anchor = document.createElement("a");
				anchor.href = "/current-page#section";

				Object.defineProperty(event, "target", { value: anchor });

				const onClick = __makeLinkOnClickFn({});
				await onClick(event);

				// Should save scroll state but not navigate
				expect(fetch).not.toHaveBeenCalled();

				const scrollState = JSON.parse(
					sessionStorage.getItem("__vorma__scrollStateMap") || "[]",
				);
				expect(scrollState).toBeDefined();
			});

			it("should use prefetch data immediately on click if available", async () => {
				const prefetchData = {
					title: { dangerousInnerHTML: "Prefetched Content" },
					importURLs: [],
					cssBundles: [],
				};

				vi.mocked(fetch).mockResolvedValue(
					createMockResponse(prefetchData),
				);

				// Start prefetch
				const handlers = __getPrefetchHandlers({
					href: "/prefetch-click",
				});
				handlers?.start({} as Event);
				await vi.advanceTimersByTimeAsync(100);
				await vi.runAllTimersAsync();

				// Create a proper click event with an anchor element
				const anchor = document.createElement("a");
				anchor.href = "/prefetch-click";
				document.body.appendChild(anchor);

				const event = new MouseEvent("click", {
					bubbles: true,
					cancelable: true,
				});
				Object.defineProperty(event, "target", { value: anchor });

				const preventDefault = vi.spyOn(event, "preventDefault");

				// Click while prefetch is complete
				await handlers?.onClick(event);
				await vi.runAllTimersAsync();

				expect(preventDefault).toHaveBeenCalled();
				expect(document.title).toBe("Prefetched Content");

				// Clean up
				document.body.removeChild(anchor);
				handlers?.stop();
			});
		});
	});
});
