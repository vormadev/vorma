// @vitest-environment jsdom

import { Effect } from "effect";
import { describe, expect, it } from "vitest";
import { make_route_dom_runtime } from "./effect_runtime/route_dom_runtime.ts";
import type { DecodedPayload } from "./effect_runtime/route_preparer.ts";
import type { HeadEl } from "./head.ts";

function run_effect<A, E>(program: Effect.Effect<A, E, never>): Promise<A> {
	return Effect.runPromise(program);
}

function payload(input: {
	title?: string;
	meta_head_els?: HeadEl[];
	rest_head_els?: HeadEl[];
	css_bundles?: string[];
	deps?: string[];
}): DecodedPayload {
	return {
		routes: [],
		params: {},
		splat_values: [],
		title: input.title,
		meta_head_els: input.meta_head_els ?? [],
		rest_head_els: input.rest_head_els ?? [],
		css_bundles: input.css_bundles ?? [],
		deps: input.deps ?? [],
	};
}

describe("ccc Effect route DOM runtime experiment", () => {
	it("decodes titles and applies route DOM side effects through one service", async () => {
		const events: unknown[] = [];
		const runtime = Effect.runSync(
			make_route_dom_runtime({
				apply_css_bundles: (css_bundles) => {
					events.push(["apply_css", css_bundles]);
				},
				apply_head_and_title: (title, meta_head_els, rest_head_els) => {
					events.push(["head", title, meta_head_els, rest_head_els]);
				},
				preload_css: (css_bundles) => {
					events.push(["preload_css", css_bundles]);
				},
				preload_modules: (deps) => {
					events.push(["preload_modules", deps]);
				},
			}),
		);

		expect(runtime.decode_title("A &amp; B")).toBe("A & B");
		await run_effect(runtime.preload_css(["/app.css"]));
		await run_effect(
			runtime.apply_payload_side_effects(
				payload({
					title: "Next",
					meta_head_els: [
						{
							tag: "meta",
							attributesKnownSafe: { name: "description" },
						},
					],
					rest_head_els: [
						{
							tag: "link",
							attributesKnownSafe: { rel: "canonical" },
						},
					],
					css_bundles: ["/app.css"],
					deps: ["/entry.js"],
				}),
			),
		);

		expect(events).toEqual([
			["preload_css", ["/app.css"]],
			[
				"head",
				"Next",
				[
					{
						tag: "meta",
						attributesKnownSafe: { name: "description" },
					},
				],
				[
					{
						tag: "link",
						attributesKnownSafe: { rel: "canonical" },
					},
				],
			],
			["apply_css", ["/app.css"]],
			["preload_modules", ["/entry.js"]],
		]);
	});

	it("maps CSS wait failures into typed route CSS failures", async () => {
		const runtime = Effect.runSync(
			make_route_dom_runtime({
				wait_for_css: async () => {
					throw new Error("css wait failed");
				},
			}),
		);

		const result = await run_effect(
			Effect.either(runtime.wait_for_css(["/broken.css"])),
		);

		expect(result._tag).toBe("Left");
		if (result._tag === "Left") {
			expect(result.left._tag).toBe("RouteCSSFailed");
		}
	});
});
