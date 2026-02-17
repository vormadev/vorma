import { describe, expect, it } from "vitest";
import {
	resolveAbsoluteHref,
	resolveAbsoluteHrefWithOptionalSearchAndHash,
} from "vorma/kit/url";
import {
	findMapEntryByNavigationTarget,
	hasSameDataTarget,
	hasSameNavigationTarget,
	hashFragmentFromHash,
	hashFragmentFromHref,
	hrefWithoutHash,
	isSameDocumentHashChange,
	isSameDocumentLocation,
	normalizedHashFragmentFromHash,
	normalizedHashFragmentFromHref,
	resolvePublicHref,
} from "../../platform/url.ts";
import { VORMA_SYMBOL } from "../../app/context.ts";

function setPublicHrefResolutionBase(props: {
	viteDevURL: string;
	publicPathPrefix: string;
}): void {
	const vormaGlobal = globalThis as typeof globalThis & {
		[VORMA_SYMBOL]?: { viteDevURL?: string; publicPathPrefix?: string };
	};
	vormaGlobal[VORMA_SYMBOL] = {
		...vormaGlobal[VORMA_SYMBOL],
		viteDevURL: props.viteDevURL,
		publicPathPrefix: props.publicPathPrefix,
	};
}

describe("hash fragment helpers", () => {
	it("normalizes and decodes hash fragments", () => {
		expect(normalizedHashFragmentFromHash("#%E2%9C%93")).toBe("✓");
		expect(normalizedHashFragmentFromHash("%E2%9C%93")).toBe("✓");
		expect(normalizedHashFragmentFromHash("#plain-id")).toBe("plain-id");
	});

	it("extracts raw hash fragments without decoding", () => {
		expect(hashFragmentFromHash("#%2520")).toBe("%2520");
		expect(hashFragmentFromHash("%2520")).toBe("%2520");
	});

	it("falls back to raw fragments when decode fails", () => {
		const invalidEncodedHash = "%E0%A4%A";
		expect(normalizedHashFragmentFromHash(invalidEncodedHash)).toBe(
			invalidEncodedHash,
		);
	});

	it("extracts and normalizes hash fragments from href values", () => {
		window.history.replaceState({}, "", "/base-path");
		expect(hashFragmentFromHref("/next#%2520")).toBe("%2520");
		expect(normalizedHashFragmentFromHref("/next#%E2%9C%93")).toBe("✓");
		expect(normalizedHashFragmentFromHref("/next#%2520")).toBe("%20");
		expect(normalizedHashFragmentFromHref("/next")).toBe("");
	});

	it("normalizes href identity without hash fragments", () => {
		window.history.replaceState({}, "", "/same-doc?mode=1#base");

		expect(hrefWithoutHash({ href: "/same-doc?mode=1#one" })).toBe(
			"http://localhost:3000/same-doc?mode=1",
		);
		expect(hrefWithoutHash({ href: "/same-doc?mode=1#two" })).toBe(
			"http://localhost:3000/same-doc?mode=1",
		);
		expect(
			hasSameDataTarget({
				firstHref: "/same-doc?mode=1#one",
				secondHref: "/same-doc?mode=1#two",
			}),
		).toBe(true);
		expect(
			hasSameDataTarget({
				firstHref: "/same-doc?b=2&a=1#one",
				secondHref: "/same-doc?a=1&b=2#two",
			}),
		).toBe(false);
	});

	it("normalizes navigation target identity with exact and hash-insensitive checks", () => {
		window.history.replaceState({}, "", "/same-doc?mode=1#base");

		expect(
			hasSameNavigationTarget({
				firstHref: "/same-doc?mode=1#stable",
				secondHref: "/same-doc?mode=1#stable",
			}),
		).toBe(true);
		expect(
			hasSameNavigationTarget({
				firstHref: "/same-doc?mode=1#one",
				secondHref: "/same-doc?mode=1#two",
			}),
		).toBe(true);
		expect(
			hasSameNavigationTarget({
				firstHref: "/same-doc?b=2&a=1#one",
				secondHref: "/same-doc?a=1&b=2#one",
			}),
		).toBe(false);
	});

	it("finds map entries by navigation target with exact-key precedence", () => {
		window.history.replaceState({}, "", "/same-doc?mode=1#base");
		const map = new Map<string, string>([
			["/same-doc?mode=1#first", "first"],
			["/same-doc?mode=1#second", "second"],
		]);

		expect(
			findMapEntryByNavigationTarget({
				map: map,
				targetHref: "/same-doc?mode=1#first",
			}),
		).toEqual(["/same-doc?mode=1#first", "first"]);
		expect(
			findMapEntryByNavigationTarget({
				map: map,
				targetHref: "/same-doc?mode=1#alias",
			}),
		).toEqual(["/same-doc?mode=1#first", "first"]);
		expect(
			findMapEntryByNavigationTarget({
				map: map,
				targetHref: "/same-doc?mode=2#first",
			}),
		).toBe(undefined);
	});

	it("resolves relative and URL inputs to absolute hrefs", () => {
		window.history.replaceState({}, "", "/base-path");

		expect(resolveAbsoluteHref({ href: "/next?mode=1#section" })).toBe(
			"http://localhost:3000/next?mode=1#section",
		);
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

	it("resolves absolute hrefs with optional search and hash overrides", () => {
		window.history.replaceState({}, "", "/base-path");

		expect(
			resolveAbsoluteHrefWithOptionalSearchAndHash({
				href: "/next?mode=1#old",
				search: "?mode=2",
				hash: "#new",
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

	it("detects same-document hash-only transitions", () => {
		window.history.replaceState({}, "", "/same-doc?mode=1#first");

		expect(
			isSameDocumentHashChange({ targetHref: "/same-doc?mode=1#second" }),
		).toBe(true);
		expect(
			isSameDocumentHashChange({ targetHref: "/same-doc?mode=1" }),
		).toBe(true);
		expect(
			isSameDocumentHashChange({ targetHref: "/same-doc?mode=1#first" }),
		).toBe(false);
		expect(
			isSameDocumentHashChange({
				targetHref: "/same-doc?mode=1#%66irst",
			}),
		).toBe(false);
		expect(
			isSameDocumentHashChange({ targetHref: "/same-doc?mode=2#second" }),
		).toBe(false);
		expect(
			isSameDocumentHashChange({
				targetHref: "https://example.com/same-doc?mode=1#x",
			}),
		).toBe(false);
	});

	it("detects same-document location no-op targets", () => {
		window.history.replaceState({}, "", "/same-doc?mode=1#~");

		expect(
			isSameDocumentLocation({ targetHref: "/same-doc?mode=1#~" }),
		).toBe(true);
		expect(
			isSameDocumentLocation({ targetHref: "/same-doc?mode=1#%7E" }),
		).toBe(true);
		expect(
			isSameDocumentLocation({ targetHref: "/same-doc?mode=1#other" }),
		).toBe(false);
		expect(
			isSameDocumentLocation({ targetHref: "/same-doc?mode=2#~" }),
		).toBe(false);
	});

	it("preserves absolute module URLs when resolving public hrefs", () => {
		setPublicHrefResolutionBase({
			viteDevURL: "",
			publicPathPrefix: "/public",
		});

		expect(resolvePublicHref("https://cdn.example.com/entry.js")).toBe(
			"https://cdn.example.com/entry.js",
		);
	});

	it("preserves protocol-relative module URLs when resolving public hrefs", () => {
		setPublicHrefResolutionBase({
			viteDevURL: "",
			publicPathPrefix: "/public",
		});

		expect(resolvePublicHref("//cdn.example.com/entry.js")).toBe(
			"//cdn.example.com/entry.js",
		);
	});
});
