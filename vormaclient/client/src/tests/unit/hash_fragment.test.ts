import { describe, expect, it } from "vitest";
import {
	findMapEntryByNavigationTarget,
	hasSameDataTarget,
	hasSameNavigationTarget,
	hashFragmentFromHash,
	hashFragmentFromHref,
	hrefWithoutHash,
	isSameDocumentLocation,
	isSameDocumentHashChange,
	normalizedHashFragmentFromHash,
	normalizedHashFragmentFromHref,
	resolveAbsoluteHref,
} from "../../platform/url.ts";

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

		expect(hrefWithoutHash("/same-doc?mode=1#one")).toBe(
			"http://localhost:3000/same-doc?mode=1",
		);
		expect(hrefWithoutHash("/same-doc?mode=1#two")).toBe(
			"http://localhost:3000/same-doc?mode=1",
		);
		expect(
			hasSameDataTarget("/same-doc?mode=1#one", "/same-doc?mode=1#two"),
		).toBe(true);
		expect(
			hasSameDataTarget("/same-doc?b=2&a=1#one", "/same-doc?a=1&b=2#two"),
		).toBe(false);
	});

	it("normalizes navigation target identity with exact and hash-insensitive checks", () => {
		window.history.replaceState({}, "", "/same-doc?mode=1#base");

		expect(
			hasSameNavigationTarget(
				"/same-doc?mode=1#stable",
				"/same-doc?mode=1#stable",
			),
		).toBe(true);
		expect(
			hasSameNavigationTarget(
				"/same-doc?mode=1#one",
				"/same-doc?mode=1#two",
			),
		).toBe(true);
		expect(
			hasSameNavigationTarget(
				"/same-doc?b=2&a=1#one",
				"/same-doc?a=1&b=2#one",
			),
		).toBe(false);
	});

	it("finds map entries by navigation target with exact-key precedence", () => {
		window.history.replaceState({}, "", "/same-doc?mode=1#base");
		const map = new Map<string, string>([
			["/same-doc?mode=1#first", "first"],
			["/same-doc?mode=1#second", "second"],
		]);

		expect(
			findMapEntryByNavigationTarget(map, "/same-doc?mode=1#first"),
		).toEqual(["/same-doc?mode=1#first", "first"]);
		expect(
			findMapEntryByNavigationTarget(map, "/same-doc?mode=1#alias"),
		).toEqual(["/same-doc?mode=1#first", "first"]);
		expect(
			findMapEntryByNavigationTarget(map, "/same-doc?mode=2#first"),
		).toBe(undefined);
	});

	it("resolves relative and URL inputs to absolute hrefs", () => {
		window.history.replaceState({}, "", "/base-path");

		expect(resolveAbsoluteHref("/next?mode=1#section")).toBe(
			"http://localhost:3000/next?mode=1#section",
		);
		expect(resolveAbsoluteHref("child", "https://example.com/base/")).toBe(
			"https://example.com/base/child",
		);
		expect(
			resolveAbsoluteHref(new URL("/x?y=1", "https://example.com")),
		).toBe("https://example.com/x?y=1");
	});

	it("detects same-document hash-only transitions", () => {
		window.history.replaceState({}, "", "/same-doc?mode=1#first");

		expect(isSameDocumentHashChange("/same-doc?mode=1#second")).toBe(true);
		expect(isSameDocumentHashChange("/same-doc?mode=1")).toBe(true);
		expect(isSameDocumentHashChange("/same-doc?mode=1#first")).toBe(false);
		expect(isSameDocumentHashChange("/same-doc?mode=1#%66irst")).toBe(
			false,
		);
		expect(isSameDocumentHashChange("/same-doc?mode=2#second")).toBe(false);
		expect(
			isSameDocumentHashChange("https://example.com/same-doc?mode=1#x"),
		).toBe(false);
	});

	it("detects same-document location no-op targets", () => {
		window.history.replaceState({}, "", "/same-doc?mode=1#~");

		expect(isSameDocumentLocation("/same-doc?mode=1#~")).toBe(true);
		expect(isSameDocumentLocation("/same-doc?mode=1#%7E")).toBe(true);
		expect(isSameDocumentLocation("/same-doc?mode=1#other")).toBe(false);
		expect(isSameDocumentLocation("/same-doc?mode=2#~")).toBe(false);
	});
});
