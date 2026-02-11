import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
	__getPrefetchHandlers,
	__makeLinkOnClickFn,
} from "../../core/links.ts";
import { navigationStateManager } from "../../client.ts";
import type { NavigationEntry } from "../../core/navigation/types.ts";

function createClickEvent(href: string): MouseEvent {
	const event = new MouseEvent("click", { bubbles: true, cancelable: true });
	const anchor = document.createElement("a");
	anchor.href = href;
	Object.defineProperty(event, "target", { value: anchor });
	return event;
}

function createIdlePrefetchEntry(targetHref: string): NavigationEntry {
	return {
		control: {
			abortController: new AbortController(),
			promise: Promise.resolve({ type: "aborted" }),
		},
		type: "prefetch",
		intent: "none",
		phase: "fetching",
		startTime: 0,
		targetUrl: new URL(targetHref, window.location.href).href,
		originUrl: window.location.href,
	};
}

describe("links internal branches", () => {
	beforeEach(() => {
		vi.useFakeTimers();
	});

	afterEach(() => {
		vi.useRealTimers();
		vi.restoreAllMocks();
		document.body.innerHTML = "";
	});

	it("removes the navigation entry when a link click resolves to aborted outcome", async () => {
		const targetHref = "/aborted-outcome";
		const targetUrl = new URL(targetHref, window.location.href).href;
		vi.spyOn(navigationStateManager, "beginNavigation").mockReturnValue({
			abortController: undefined,
			promise: Promise.resolve({ type: "aborted" }),
		});
		const removeNavigationSpy = vi
			.spyOn(navigationStateManager, "removeNavigation")
			.mockImplementation(() => {});

		const onClick = __makeLinkOnClickFn({});
		await onClick(createClickEvent(targetHref));

		expect(removeNavigationSpy).toHaveBeenCalledWith(targetUrl);
	});

	it("finds and aborts idle prefetch entries by same-data-target alias", () => {
		const aliasEntry = createIdlePrefetchEntry("/prefetch-alias#first");
		const targetHref = "/prefetch-alias#second";
		const targetUrl = new URL(targetHref, window.location.href).href;

		vi.spyOn(navigationStateManager, "getNavigation").mockImplementation(
			(key) => (key === targetUrl ? undefined : aliasEntry),
		);
		vi.spyOn(navigationStateManager, "getNavigations").mockReturnValue(
			new Map([[aliasEntry.targetUrl, aliasEntry]]),
		);
		const removeNavigationSpy = vi
			.spyOn(navigationStateManager, "removeNavigation")
			.mockImplementation(() => {});

		const handlers = __getPrefetchHandlers({
			href: targetHref,
			delayMs: 0,
		});
		handlers?.stop();

		expect(aliasEntry.control.abortController?.signal.aborted).toBe(true);
		expect(removeNavigationSpy).toHaveBeenCalledWith(aliasEntry.targetUrl);
	});

	it("does not begin navigation when click event has no eligible anchor details", async () => {
		const beginNavigationSpy = vi.spyOn(
			navigationStateManager,
			"beginNavigation",
		);
		const onClick = __makeLinkOnClickFn({});
		const event = new MouseEvent("click", {
			bubbles: true,
			cancelable: true,
		});
		Object.defineProperty(event, "target", {
			value: document.createElement("div"),
		});

		await onClick(event);

		expect(beginNavigationSpy).not.toHaveBeenCalled();
	});

	it("does not schedule duplicate prefetch when idle prefetch is already active", async () => {
		const targetHref = "/prefetch-already-active";
		const targetUrl = new URL(targetHref, window.location.href).href;
		const idlePrefetchEntry = createIdlePrefetchEntry(targetHref);

		const navigateSpy = vi
			.spyOn(navigationStateManager, "navigate")
			.mockResolvedValue({ didNavigate: true });
		vi.spyOn(navigationStateManager, "getNavigation").mockImplementation(
			(key) => (key === targetUrl ? idlePrefetchEntry : undefined),
		);
		vi.spyOn(navigationStateManager, "getNavigations").mockReturnValue(
			new Map([[targetUrl, idlePrefetchEntry]]),
		);

		const handlers = __getPrefetchHandlers({
			href: targetHref,
			delayMs: 0,
		});
		expect(handlers).toBeDefined();
		if (!handlers) return;

		handlers.start(new Event("mouseenter"));
		await vi.advanceTimersByTimeAsync(1);
		expect(navigateSpy).toHaveBeenCalledTimes(1);

		handlers.start(new Event("mouseenter"));
		await vi.advanceTimersByTimeAsync(1);
		expect(navigateSpy).toHaveBeenCalledTimes(1);
	});
});
