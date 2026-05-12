import { describe, expect, it } from "vitest";
import {
	core6_is_http_href,
	core6_normalize_hash,
	core6_normalized_hash_from_href,
	core6_same_document_href,
	core6_same_origin_href,
	core6_scroll_for_href,
} from "./route_url.ts";

describe("core6 route URL laws", () => {
	it("classifies HTTP and same-origin hrefs without throwing on invalid input", () => {
		expect(core6_is_http_href("https://example.test/path")).toBe(true);
		expect(core6_is_http_href("mailto:ada@example.test")).toBe(false);
		expect(core6_is_http_href("http://[")).toBe(false);
		expect(
			core6_same_origin_href(
				"https://example.test/a",
				"https://example.test/b",
			),
		).toBe(true);
		expect(
			core6_same_origin_href(
				"https://example.test/a",
				"https://other.test/a",
			),
		).toBe(false);
		expect(core6_same_origin_href("http://[", "https://example.test")).toBe(
			false,
		);
	});

	it("treats origin, pathname, and search as the same-document identity", () => {
		expect(
			core6_same_document_href(
				"https://example.test/root?q=1#top",
				"https://example.test/root?q=1#bottom",
			),
		).toBe(true);
		expect(
			core6_same_document_href(
				"https://example.test/root?q=1",
				"https://example.test/root?q=2",
			),
		).toBe(false);
	});

	it("derives hash scroll before falling back to explicit or top scroll", () => {
		expect(
			core6_scroll_for_href("https://example.test/root#section"),
		).toEqual({ hash: "#section" });
		expect(
			core6_scroll_for_href("https://example.test/root", {
				x: 4,
				y: 8,
			}),
		).toEqual({ x: 4, y: 8 });
		expect(core6_scroll_for_href("https://example.test/root")).toEqual({
			x: 0,
			y: 0,
		});
	});

	it("normalizes hashes using browser hash decoding rules", () => {
		expect(core6_normalize_hash("#Ada%20Lovelace")).toBe("Ada Lovelace");
		expect(
			core6_normalized_hash_from_href("https://example.test/#Ada"),
		).toBe("Ada");
		expect(core6_normalized_hash_from_href("http://[")).toBe("");
	});
});
