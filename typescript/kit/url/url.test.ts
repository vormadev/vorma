import { JSDOM } from "jsdom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
	getAnchorDetailsFromEvent,
	getHrefDetails,
	getIsErrorRes,
	getIsGETRequest,
	getPrefetchHandlers,
	prefetch,
	resolveAbsoluteHref,
	resolveAbsoluteHrefWithOptionalSearchAndHash,
} from "./url.ts";

describe("getIsErrorRes", () => {
	it("should return true for 4xx status codes", () => {
		const mockResponse = new Response(null, { status: 404 });
		expect(getIsErrorRes(mockResponse)).toBe(true);
	});

	it("should return true for 5xx status codes", () => {
		const mockResponse = new Response(null, { status: 500 });
		expect(getIsErrorRes(mockResponse)).toBe(true);
	});

	it("should return false for non-error status codes", () => {
		const mockResponse = new Response(null, { status: 200 });
		expect(getIsErrorRes(mockResponse)).toBe(false);
	});
});

describe("getIsGETRequest", () => {
	it("should return true for undefined or GET method", () => {
		expect(getIsGETRequest()).toBe(true);
		expect(getIsGETRequest({ method: "GET" })).toBe(true);
		expect(getIsGETRequest({ method: "get" })).toBe(true);
	});

	it("should return true for HEAD method", () => {
		expect(getIsGETRequest({ method: "HEAD" })).toBe(true);
		expect(getIsGETRequest({ method: "head" })).toBe(true);
	});

	it("should return false for non-GET/HEAD methods", () => {
		expect(getIsGETRequest({ method: "POST" })).toBe(false);
		expect(getIsGETRequest({ method: "PUT" })).toBe(false);
	});
});

describe("resolveAbsoluteHref", () => {
	it("resolves relative and URL inputs to absolute hrefs", () => {
		expect(
			resolveAbsoluteHref({
				href: "/next?mode=1#section",
				baseHref: "http://localhost:3000/base",
			}),
		).toBe("http://localhost:3000/next?mode=1#section");
		expect(
			resolveAbsoluteHref({
				href: "child",
				baseHref: "https://example.com/base/",
			}),
		).toBe("https://example.com/base/child");
		expect(
			resolveAbsoluteHref({
				href: new URL("/x?y=1", "https://example.com"),
			}),
		).toBe("https://example.com/x?y=1");
	});
});

describe("resolveAbsoluteHrefWithOptionalSearchAndHash", () => {
	it("applies search/hash overrides when provided", () => {
		expect(
			resolveAbsoluteHrefWithOptionalSearchAndHash({
				href: "/next?mode=1#old",
				search: "?mode=2",
				hash: "#new",
				baseHref: "http://localhost:3000/base",
			}),
		).toBe("http://localhost:3000/next?mode=2#new");
		expect(
			resolveAbsoluteHrefWithOptionalSearchAndHash({
				href: "https://example.com/path?existing=1#old",
				hash: "",
			}),
		).toBe("https://example.com/path?existing=1");
		expect(
			resolveAbsoluteHrefWithOptionalSearchAndHash({
				href: "child",
				baseHref: "https://example.com/base/",
			}),
		).toBe("https://example.com/base/child");
	});
});

let dom: JSDOM;

describe("getAnchorDetailsFromEvent", () => {
	beforeEach(() => {
		// Create a fresh JSDOM environment for each test
		dom = new JSDOM("<!DOCTYPE html><body></body>", {
			url: "https://example.com",
		});
		(global as any).window = dom.window as unknown as Window &
			typeof globalThis;
		(global as any).document = dom.window.document;
	});

	afterEach(() => {
		// Clean up the JSDOM environment after each test
		dom.window.close();
		(global as any).window = undefined as unknown as Window &
			typeof globalThis;
		(global as any).document = undefined as unknown as Document;
	});

	it("should return null if the event is not a click event", () => {
		const fakeEvent = new dom.window.Event(
			"keydown",
		) as unknown as MouseEvent;
		const result = getAnchorDetailsFromEvent(fakeEvent);
		expect(result).toBeNull();
	});

	it("should return null if the event target is not inside an anchor tag", () => {
		const div = dom.window.document.createElement("div");
		dom.window.document.body.appendChild(div);

		const clickEvent = new dom.window.MouseEvent("click", {
			bubbles: true,
		});
		div.dispatchEvent(clickEvent);

		const result = getAnchorDetailsFromEvent(clickEvent);
		expect(result).toBeNull();
	});

	it("should return anchor details for a valid click event", () => {
		const anchor = dom.window.document.createElement("a");
		anchor.href = "https://example.com/some-page";
		dom.window.document.body.appendChild(anchor);

		const clickEvent = new dom.window.MouseEvent("click", {
			bubbles: true,
		});
		anchor.dispatchEvent(clickEvent);

		const result = getAnchorDetailsFromEvent(clickEvent);
		expect(result).toEqual({
			anchor,
			isEligibleForDefaultPrevention: true,
			isInternal: true,
		});
	});

	it("should correctly identify external links", () => {
		const anchor = dom.window.document.createElement("a");
		anchor.href = "https://external.com";
		dom.window.document.body.appendChild(anchor);

		const clickEvent = new dom.window.MouseEvent("click", {
			bubbles: true,
		});
		anchor.dispatchEvent(clickEvent);

		const result = getAnchorDetailsFromEvent(clickEvent);
		expect(result).toEqual({
			anchor,
			isEligibleForDefaultPrevention: true,
			isInternal: false,
		});
	});

	it("should mark as not eligible for default prevention on middle-click", () => {
		const anchor = dom.window.document.createElement("a");
		anchor.href = "https://example.com";
		dom.window.document.body.appendChild(anchor);

		const middleClickEvent = new dom.window.MouseEvent("click", {
			bubbles: true,
			button: 1, // middle-click
		});
		anchor.dispatchEvent(middleClickEvent);

		const result = getAnchorDetailsFromEvent(middleClickEvent);
		expect(result).toEqual({
			anchor,
			isEligibleForDefaultPrevention: false,
			isInternal: true,
		});
	});

	it("should mark as not eligible for default prevention on ctrl+click", () => {
		const anchor = dom.window.document.createElement("a");
		anchor.href = "https://example.com";
		dom.window.document.body.appendChild(anchor);

		const ctrlClickEvent = new dom.window.MouseEvent("click", {
			bubbles: true,
			ctrlKey: true, // ctrl+click
		});
		anchor.dispatchEvent(ctrlClickEvent);

		const result = getAnchorDetailsFromEvent(ctrlClickEvent);
		expect(result).toEqual({
			anchor,
			isEligibleForDefaultPrevention: false,
			isInternal: true,
		});
	});

	it("should mark as not eligible for default prevention on shift+click", () => {
		const anchor = dom.window.document.createElement("a");
		anchor.href = "https://example.com";
		dom.window.document.body.appendChild(anchor);

		const shiftClickEvent = new dom.window.MouseEvent("click", {
			bubbles: true,
			shiftKey: true, // shift+click
		});
		anchor.dispatchEvent(shiftClickEvent);

		const result = getAnchorDetailsFromEvent(shiftClickEvent);
		expect(result).toEqual({
			anchor,
			isEligibleForDefaultPrevention: false,
			isInternal: true,
		});
	});
});

describe("getHrefDetails", () => {
	beforeEach(() => {
		// Create a fresh JSDOM environment with a controlled origin
		dom = new JSDOM("<!DOCTYPE html><body></body>", {
			url: "https://example.com",
		});
		(global as any).window = dom.window as unknown as Window &
			typeof globalThis;
		(global as any).document = dom.window.document;
	});

	afterEach(() => {
		dom.window.close();
		(global as any).window = undefined as unknown as Window &
			typeof globalThis;
		(global as any).document = undefined as unknown as Document;
	});

	it("should return isHTTP: false for an empty href", () => {
		const result = getHrefDetails("");
		expect(result).toEqual({ isHTTP: false });
	});

	it("should treat non-standard strings as relative URLs", () => {
		const result = getHrefDetails("not-a-valid-url");
		expect(result).toEqual({
			isHTTP: true,
			absoluteURL: "https://example.com/not-a-valid-url",
			relativeURL: "/not-a-valid-url",
			isExternal: false,
			isInternal: true,
			url: new URL("https://example.com/not-a-valid-url"),
		});
	});

	it("should correctly identify internal HTTP URLs", () => {
		const result = getHrefDetails("https://example.com/page");
		expect(result).toEqual({
			isHTTP: true,
			absoluteURL: "https://example.com/page",
			relativeURL: "/page",
			isExternal: false,
			isInternal: true,
			url: new URL("https://example.com/page"),
		});
	});

	it("should correctly identify external HTTP URLs", () => {
		const result = getHrefDetails("https://external.com");
		expect(result).toEqual({
			isHTTP: true,
			absoluteURL: "https://external.com/", // URL.href always ends with a slash
			relativeURL: "",
			isExternal: true,
			isInternal: false,
			url: new URL("https://external.com/"),
		});
	});

	it("should return isHTTP: false for non-HTTP protocols", () => {
		const result = getHrefDetails("mailto:user@example.com");
		expect(result).toEqual({ isHTTP: false });
	});

	it("should correctly resolve relative paths as internal URLs", () => {
		const result = getHrefDetails("/relative-path");
		expect(result).toEqual({
			isHTTP: true,
			absoluteURL: "https://example.com/relative-path",
			relativeURL: "/relative-path",
			isExternal: false,
			isInternal: true,
			url: new URL("https://example.com/relative-path"),
		});
	});

	it("should correctly handle hash links as internal URLs", () => {
		const result = getHrefDetails("#section");
		expect(result).toEqual({
			isHTTP: true,
			absoluteURL: "https://example.com/#section",
			relativeURL: "/#section",
			isExternal: false,
			isInternal: true,
			url: new URL("https://example.com/#section"),
		});
	});

	it("should correctly handle tel: links", () => {
		const result = getHrefDetails("tel:+1234567890");
		expect(result).toEqual({ isHTTP: false });
	});
});

describe("getPrefetchHandlers", () => {
	beforeEach(() => {
		// Create a fresh JSDOM environment with a controlled origin
		dom = new JSDOM("<!DOCTYPE html><body></body>", {
			url: "https://example.com",
		});
		(global as any).window = dom.window as unknown as Window &
			typeof globalThis;
		(global as any).document = dom.window.document;
		vi.useFakeTimers(); // Mock timers for the prefetch timeout
	});

	afterEach(() => {
		dom.window.close();
		(global as any).window = undefined as unknown as Window &
			typeof globalThis;
		(global as any).document = undefined as unknown as Document;
		vi.useRealTimers(); // Restore real timers after each test
	});

	it("should return undefined for non-HTTP URLs", () => {
		const result = getPrefetchHandlers("mailto:user@example.com");
		expect(result).toBeUndefined();
	});

	it("should return undefined for external HTTP URLs", () => {
		const result = getPrefetchHandlers("https://external.com");
		expect(result).toBeUndefined();
	});

	it("should return undefined for empty or invalid hrefs", () => {
		const result = getPrefetchHandlers("");
		expect(result).toBeUndefined();
	});

	it("should start and stop prefetch correctly for internal URLs", () => {
		const handlers = getPrefetchHandlers("/relative-path", 100);
		expect(handlers).toBeDefined();

		if (!handlers) {
			throw new Error("Handlers should be defined");
		}

		// Mock prefetch function since we're only testing the handler behavior
		const prefetchSpy = vi.spyOn(window, "setTimeout");

		handlers.start();
		expect(prefetchSpy).toHaveBeenCalledTimes(1); // Ensure the timer was set

		// Fast-forward the timer to trigger the timeout
		vi.runAllTimers();
		expect(
			dom.window.document.querySelector(`link[href="/relative-path"]`),
		).not.toBeNull();

		handlers.stop(); // Stop the prefetch
		expect(
			dom.window.document.querySelector(`link[href="/relative-path"]`),
		).toBeNull();
	});
});

describe("prefetch", () => {
	beforeEach(() => {
		dom = new JSDOM("<!DOCTYPE html><body></body>", {
			url: "https://example.com",
		});
		(global as any).window = dom.window as unknown as Window &
			typeof globalThis;
		(global as any).document = dom.window.document;
	});

	afterEach(() => {
		dom.window.close();
		(global as any).window = undefined as unknown as Window &
			typeof globalThis;
		(global as any).document = undefined as unknown as Document;
	});

	it("should add a prefetch link to the document head", () => {
		prefetch("/relative-path");
		const link = dom.window.document.querySelector(
			`link[href="/relative-path"]`,
		);

		expect(link).not.toBeNull();
		if (!link || !("rel" in link)) {
			throw new Error("Link should not be null");
		}
		expect(link.rel).toBe("prefetch");
	});

	it("should remove any existing prefetch link before adding a new one", () => {
		// Add an initial prefetch link
		const initialLink = dom.window.document.createElement("link");
		initialLink.rel = "prefetch";
		initialLink.href = "/relative-path";
		dom.window.document.head.appendChild(initialLink);

		// Call prefetch again, which should remove the existing link
		prefetch("/relative-path");

		const links = dom.window.document.querySelectorAll(
			`link[href="/relative-path"]`,
		);
		expect(links.length).toBe(1); // Only one link should remain
	});
});
