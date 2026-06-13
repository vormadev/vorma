// @vitest-environment jsdom

import { describe, expect, it } from "vitest";
import { X_CLIENT_REDIRECT } from "./constants.ts";
import { classify_redirect_target, detect_redirect, is_http } from "./redirects.ts";

const ORIGIN = "https://app.example.com";

describe("classify_redirect_target", () => {
	it("rejects non-http(s) schemes", () => {
		expect(classify_redirect_target("javascript:alert(1)", false, ORIGIN)).toEqual({
			kind: "invalid",
		});
		expect(classify_redirect_target("mailto:x@example.com", false, ORIGIN)).toEqual({
			kind: "invalid",
		});
		expect(classify_redirect_target("not a url", false, ORIGIN)).toEqual({
			kind: "invalid",
		});
	});

	it("treats the hard flag as hard even for same-origin targets", () => {
		expect(classify_redirect_target(`${ORIGIN}/next`, true, ORIGIN)).toEqual({
			kind: "hard",
			href: `${ORIGIN}/next`,
		});
	});

	it("treats cross-origin targets as hard", () => {
		expect(
			classify_redirect_target("https://other.example.com/next", false, ORIGIN),
		).toEqual({ kind: "hard", href: "https://other.example.com/next" });
	});

	it("classifies same-origin http(s) targets as soft with a parsed URL", () => {
		const result = classify_redirect_target(`${ORIGIN}/next?a=1`, false, ORIGIN);
		expect(result.kind).toBe("soft");
		if (result.kind === "soft") {
			expect(result.url.pathname).toBe("/next");
			expect(result.url.search).toBe("?a=1");
		}
	});
});

describe("detect_redirect", () => {
	const base = new URL(`${ORIGIN}/current`);

	it("prefers the explicit client-redirect header and resolves it against base", () => {
		const res = new Response(null, {
			headers: { [X_CLIENT_REDIRECT]: "/target" },
		});
		expect(detect_redirect(res, base)).toEqual({
			href: `${ORIGIN}/target`,
			hard: false,
		});
	});

	it("returns null when nothing redirected", () => {
		expect(detect_redirect(new Response(null), base)).toBeNull();
	});
});

describe("is_http", () => {
	it("accepts http and https only", () => {
		expect(is_http("http://x.example")).toBe(true);
		expect(is_http("https://x.example")).toBe(true);
		expect(is_http("ftp://x.example")).toBe(false);
		expect(is_http("/relative")).toBe(false);
	});
});
