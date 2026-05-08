// @vitest-environment jsdom

import { Effect } from "effect";
import { describe, expect, it } from "vitest";
import { make_browser_location } from "./effect_runtime/browser_location.ts";

function run_effect<A, E>(program: Effect.Effect<A, E, never>): Promise<A> {
	return Effect.runPromise(program);
}

describe("ccc Effect browser location experiment", () => {
	it("resolves hrefs, route keys, hashes, and same-origin checks from the current location", () => {
		const location = Effect.runSync(
			make_browser_location({
				get_href: () => {
					return "https://app.example.test/root/current?x=1#old";
				},
			}),
		);

		expect(location.resolve_href("../next?y=2#new")).toBe(
			"https://app.example.test/next?y=2#new",
		);
		expect(location.resolve_href_or_null("http://[bad")).toBeNull();
		expect(location.route_key("../next?y=2#new")).toBe(
			"https://app.example.test/next?y=2",
		);
		expect(location.hash_fragment("../next?y=2#new")).toBe("#new");
		expect(location.is_http_href("https://app.example.test/next")).toBe(
			true,
		);
		expect(location.is_http_href("mailto:hello@example.test")).toBe(false);
		expect(
			location.is_same_origin_href("https://app.example.test/next"),
		).toBe(true);
		expect(
			location.is_same_origin_href("https://external.example.test/next"),
		).toBe(false);
	});

	it("keeps hard redirects behind an Effect boundary", async () => {
		const redirects: string[] = [];
		const location = Effect.runSync(
			make_browser_location({
				get_href: () => {
					return "https://app.example.test/";
				},
				hard_redirect: (href) => {
					redirects.push(href);
				},
			}),
		);

		await run_effect(location.hard_redirect("https://next.example.test/"));

		expect(redirects).toEqual(["https://next.example.test/"]);
	});
});
