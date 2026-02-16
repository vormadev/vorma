import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { getHrefDetails } from "vorma/kit/url";
import {
	__getPrefetchHandlers,
	__makeLinkOnClickFn,
} from "../../core/links.ts";
import { navigationStateManager } from "../../client.ts";
import type { NavigationEntry } from "../../core/navigation/types.ts";
import * as redirectsModule from "../../core/redirects.ts";

function createClickEvent(href: string): MouseEvent {
	const event = new MouseEvent("click", { bubbles: true, cancelable: true });
	const anchor = document.createElement("a");
	anchor.href = href;
	Object.defineProperty(event, "target", { value: anchor });
	return event;
}

function createIdlePrefetchEntry(targetHref: string): NavigationEntry {
	return {
		operationID: 1,
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
		vi.spyOn(console, "error").mockImplementation(() => {});
	});

	afterEach(() => {
		vi.useRealTimers();
		vi.restoreAllMocks();
		document.body.innerHTML = "";
	});

	it("removes the navigation entry when a link click resolves to aborted outcome", async () => {
		const targetHref = "/aborted-outcome";
		const targetUrl = new URL(targetHref, window.location.href).href;
		const controlPromise = Promise.resolve({ type: "aborted" as const });
		vi.spyOn(navigationStateManager, "beginNavigation").mockReturnValue({
			abortController: undefined,
			promise: controlPromise,
			operationID: 1,
		});
		vi.spyOn(navigationStateManager, "getNavigation").mockReturnValue({
			operationID: 1,
			control: {
				abortController: new AbortController(),
				promise: controlPromise,
			},
			type: "userNavigation",
			intent: "navigate",
			phase: "fetching",
			startTime: Date.now(),
			targetUrl,
			originUrl: window.location.href,
		});
		const removeNavigationSpy = vi
			.spyOn(navigationStateManager, "removeNavigation")
			.mockImplementation(() => {});

		const onClick = __makeLinkOnClickFn({});
		await onClick(createClickEvent(targetHref));

		expect(removeNavigationSpy).toHaveBeenCalledWith(targetUrl);
	});

	it("does not mutate navigation state for stale aborted link outcomes", async () => {
		const targetHref = "/aborted-stale-outcome";
		const staleControlPromise = Promise.resolve({
			type: "aborted" as const,
		});
		const currentControlPromise = Promise.resolve({
			type: "aborted" as const,
		});
		vi.spyOn(navigationStateManager, "beginNavigation").mockReturnValue({
			abortController: undefined,
			promise: staleControlPromise,
			operationID: 2,
		});
		vi.spyOn(navigationStateManager, "getNavigation").mockReturnValue({
			operationID: 1,
			control: {
				abortController: new AbortController(),
				promise: currentControlPromise,
			},
			type: "userNavigation",
			intent: "navigate",
			phase: "fetching",
			startTime: Date.now(),
			targetUrl: new URL(targetHref, window.location.href).href,
			originUrl: window.location.href,
		});
		const removeNavigationSpy = vi.spyOn(
			navigationStateManager,
			"removeNavigation",
		);
		const processSuccessfulNavigationSpy = vi.spyOn(
			navigationStateManager,
			"processSuccessfulNavigation",
		);

		const onClick = __makeLinkOnClickFn({});
		await onClick(createClickEvent(targetHref));

		expect(removeNavigationSpy).not.toHaveBeenCalled();
		expect(processSuccessfulNavigationSpy).not.toHaveBeenCalled();
	});

	it("uses operation-id ownership for aborted outcomes when promise ownership is stale", async () => {
		const targetHref = "/aborted-operation-id-owned";
		const targetUrl = new URL(targetHref, window.location.href).href;
		const staleControlPromise = Promise.resolve({
			type: "aborted" as const,
		});
		const currentControlPromise = Promise.resolve({
			type: "aborted" as const,
		});

		vi.spyOn(navigationStateManager, "beginNavigation").mockReturnValue({
			abortController: undefined,
			promise: staleControlPromise,
			operationID: 77,
		});
		vi.spyOn(navigationStateManager, "getNavigation").mockReturnValue({
			operationID: 77,
			control: {
				abortController: new AbortController(),
				promise: currentControlPromise,
			},
			type: "userNavigation",
			intent: "navigate",
			phase: "fetching",
			startTime: Date.now(),
			targetUrl,
			originUrl: window.location.href,
		});
		const removeNavigationSpy = vi
			.spyOn(navigationStateManager, "removeNavigation")
			.mockImplementation(() => {});

		const onClick = __makeLinkOnClickFn({});
		await onClick(createClickEvent(targetHref));

		expect(removeNavigationSpy).toHaveBeenCalledWith(targetUrl);
	});

	it("treats mismatched operation-id ownership as stale even when promise matches", async () => {
		const targetHref = "/aborted-operation-id-stale";
		const targetUrl = new URL(targetHref, window.location.href).href;
		const controlPromise = Promise.resolve({
			type: "aborted" as const,
		});

		vi.spyOn(navigationStateManager, "beginNavigation").mockReturnValue({
			abortController: undefined,
			promise: controlPromise,
			operationID: 99,
		});
		vi.spyOn(navigationStateManager, "getNavigation").mockReturnValue({
			operationID: 1,
			control: {
				abortController: new AbortController(),
				promise: controlPromise,
			},
			type: "userNavigation",
			intent: "navigate",
			phase: "fetching",
			startTime: Date.now(),
			targetUrl,
			originUrl: window.location.href,
		});
		const removeNavigationSpy = vi.spyOn(
			navigationStateManager,
			"removeNavigation",
		);

		const onClick = __makeLinkOnClickFn({});
		await onClick(createClickEvent(targetHref));

		expect(removeNavigationSpy).not.toHaveBeenCalled();
	});

	it("cleans up current failed link navigations without throwing", async () => {
		const targetHref = "/failed-link-navigation";
		const targetUrl = new URL(targetHref, window.location.href).href;
		const controlPromise = Promise.reject(new Error("network failed"));
		vi.spyOn(navigationStateManager, "beginNavigation").mockReturnValue({
			abortController: undefined,
			promise: controlPromise,
			operationID: 1,
		});
		vi.spyOn(navigationStateManager, "getNavigation").mockReturnValue({
			operationID: 1,
			control: {
				abortController: new AbortController(),
				promise: controlPromise,
			},
			type: "userNavigation",
			intent: "navigate",
			phase: "fetching",
			startTime: Date.now(),
			targetUrl,
			originUrl: window.location.href,
		});
		const removeNavigationSpy = vi
			.spyOn(navigationStateManager, "removeNavigation")
			.mockImplementation(() => {});

		const onClick = __makeLinkOnClickFn({});
		await expect(
			onClick(createClickEvent(targetHref)),
		).resolves.toBeUndefined();
		expect(removeNavigationSpy).toHaveBeenCalledWith(targetUrl);
	});

	it("does not apply stale redirect side effects when ownership changes during beforeRender callback", async () => {
		const targetHref = "/redirect-stale-before-render";
		const targetUrl = new URL(targetHref, window.location.href).href;
		const redirectHrefDetails = getHrefDetails("/redirect-target");
		if (!redirectHrefDetails.isHTTP) {
			throw new Error("Expected HTTP href details for redirect target.");
		}
		const redirectOutcome = {
			type: "redirect" as const,
			redirectData: {
				status: "should" as const,
				shouldRedirectStrategy: "soft" as const,
				latestBuildID: "1",
				href: "/redirect-target",
				hrefDetails: redirectHrefDetails,
			},
			props: {
				href: targetHref,
				navigationType: "userNavigation" as const,
			},
		};
		const controlPromise = Promise.resolve(redirectOutcome);
		const staleEntry: NavigationEntry = {
			operationID: 1,
			control: {
				abortController: new AbortController(),
				promise: controlPromise,
			},
			type: "userNavigation",
			intent: "navigate",
			phase: "fetching",
			startTime: Date.now(),
			targetUrl,
			originUrl: window.location.href,
		};
		const replacementEntry: NavigationEntry = {
			operationID: 2,
			control: {
				abortController: new AbortController(),
				promise: Promise.resolve({ type: "aborted" as const }),
			},
			type: "userNavigation",
			intent: "navigate",
			phase: "fetching",
			startTime: Date.now(),
			targetUrl,
			originUrl: window.location.href,
		};
		let currentEntry = staleEntry;

		vi.spyOn(navigationStateManager, "beginNavigation").mockReturnValue({
			abortController: undefined,
			promise: controlPromise,
			operationID: 1,
		});
		vi.spyOn(navigationStateManager, "getNavigation").mockImplementation(
			() => currentEntry,
		);
		const removeNavigationSpy = vi.spyOn(
			navigationStateManager,
			"removeNavigation",
		);
		const syncBuildIDSpy = vi.spyOn(
			redirectsModule,
			"syncBuildIDFromRedirectData",
		);
		const effectuateRedirectSpy = vi
			.spyOn(redirectsModule, "effectuateRedirectDataResult")
			.mockResolvedValue(null);

		const onClick = __makeLinkOnClickFn({
			beforeRender: async () => {
				currentEntry = replacementEntry;
				await Promise.resolve();
			},
		});
		await onClick(createClickEvent(targetHref));

		expect(removeNavigationSpy).not.toHaveBeenCalled();
		expect(syncBuildIDSpy).not.toHaveBeenCalled();
		expect(effectuateRedirectSpy).not.toHaveBeenCalled();
	});

	it("does not call afterRender when redirect effectuation does not complete", async () => {
		const targetHref = "/redirect-no-complete-after-render";
		const targetUrl = new URL(targetHref, window.location.href).href;
		const redirectHrefDetails = getHrefDetails("/redirect-target");
		if (!redirectHrefDetails.isHTTP) {
			throw new Error("Expected HTTP href details for redirect target.");
		}
		const redirectOutcome = {
			type: "redirect" as const,
			redirectData: {
				status: "should" as const,
				shouldRedirectStrategy: "soft" as const,
				latestBuildID: "1",
				href: "/redirect-target",
				hrefDetails: redirectHrefDetails,
			},
			props: {
				href: targetHref,
				navigationType: "userNavigation" as const,
			},
		};
		const controlPromise = Promise.resolve(redirectOutcome);
		const entry: NavigationEntry = {
			operationID: 1,
			control: {
				abortController: new AbortController(),
				promise: controlPromise,
			},
			type: "userNavigation",
			intent: "navigate",
			phase: "fetching",
			startTime: Date.now(),
			targetUrl,
			originUrl: window.location.href,
		};

		vi.spyOn(navigationStateManager, "beginNavigation").mockReturnValue({
			abortController: undefined,
			promise: controlPromise,
			operationID: 1,
		});
		vi.spyOn(navigationStateManager, "getNavigation").mockImplementation(
			() => entry,
		);
		vi.spyOn(
			redirectsModule,
			"syncBuildIDFromRedirectData",
		).mockImplementation(() => {});
		vi.spyOn(
			redirectsModule,
			"effectuateRedirectDataResult",
		).mockResolvedValue(null);
		const afterRender = vi.fn();

		const onClick = __makeLinkOnClickFn({
			afterRender,
		});
		await onClick(createClickEvent(targetHref));

		expect(afterRender).not.toHaveBeenCalled();
	});

	it("does not remove navigation when failed link promise is stale", async () => {
		const targetHref = "/failed-link-navigation-stale";
		const staleControlPromise = Promise.reject(new Error("network failed"));
		const currentControlPromise = Promise.resolve({
			type: "aborted" as const,
		});
		vi.spyOn(navigationStateManager, "beginNavigation").mockReturnValue({
			abortController: undefined,
			promise: staleControlPromise,
			operationID: 2,
		});
		vi.spyOn(navigationStateManager, "getNavigation").mockReturnValue({
			operationID: 1,
			control: {
				abortController: new AbortController(),
				promise: currentControlPromise,
			},
			type: "userNavigation",
			intent: "navigate",
			phase: "fetching",
			startTime: Date.now(),
			targetUrl: new URL(targetHref, window.location.href).href,
			originUrl: window.location.href,
		});
		const removeNavigationSpy = vi.spyOn(
			navigationStateManager,
			"removeNavigation",
		);

		const onClick = __makeLinkOnClickFn({});
		await expect(
			onClick(createClickEvent(targetHref)),
		).resolves.toBeUndefined();
		expect(removeNavigationSpy).not.toHaveBeenCalled();
	});

	it("does not call afterRender when successful link processing does not complete", async () => {
		const targetHref = "/success-not-complete-after-render";
		const targetUrl = new URL(targetHref, window.location.href).href;
		const successOutcome = {
			type: "success" as const,
			response: new Response("{}", {
				status: 200,
				headers: {
					"X-Vorma-Build-Id": "build-id-1",
				},
			}),
			json: {
				matchedPatterns: ["/example"],
				loadersData: [{}],
				importURLs: ["/entry.js"],
				exportKeys: ["default"],
				errorExportKeys: [""],
				hasRootData: true,
				params: {},
				splatValues: [],
				title: null,
				metaHeadEls: null,
				restHeadEls: null,
				deps: [],
				cssBundles: [],
			},
			preloadCommands: [],
			waitFnPromise: Promise.resolve({ data: [] }),
			props: {
				href: targetHref,
				navigationType: "userNavigation" as const,
			},
		};
		const controlPromise = Promise.resolve(successOutcome);
		const entry: NavigationEntry = {
			operationID: 1,
			control: {
				abortController: new AbortController(),
				promise: controlPromise,
			},
			type: "userNavigation",
			intent: "navigate",
			phase: "fetching",
			startTime: Date.now(),
			targetUrl,
			originUrl: window.location.href,
		};

		vi.spyOn(navigationStateManager, "beginNavigation").mockReturnValue({
			abortController: undefined,
			promise: controlPromise,
			operationID: 1,
		});
		vi.spyOn(navigationStateManager, "getNavigation").mockImplementation(
			() => entry,
		);
		vi.spyOn(
			navigationStateManager,
			"processSuccessfulNavigation",
		).mockImplementation(async (_outcome, currentEntry) => {
			currentEntry.phase = "waiting";
		});
		const afterRender = vi.fn();

		const onClick = __makeLinkOnClickFn({
			afterRender,
		});
		await onClick(createClickEvent(targetHref));

		expect(afterRender).not.toHaveBeenCalled();
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
