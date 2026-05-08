// @vitest-environment jsdom

import { Effect, Result as EffectResult } from "effect";
import { describe, expect, it } from "vitest";
import { make_route_dom_runtime } from "./effect_runtime/route_dom_runtime.ts";
import type { DecodedPayload } from "./effect_runtime/route_preparer.ts";
import type { HeadEl } from "./head.ts";

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
					return Effect.sync(() => {
						events.push(["apply_css", css_bundles]);
					});
				},
				apply_head_and_title: (title, meta_head_els, rest_head_els) => {
					return Effect.sync(() => {
						events.push([
							"head",
							title,
							meta_head_els,
							rest_head_els,
						]);
					});
				},
				preload_css: (css_bundles) => {
					return Effect.sync(() => {
						events.push(["preload_css", css_bundles]);
					});
				},
				preload_modules: (deps) => {
					return Effect.sync(() => {
						events.push(["preload_modules", deps]);
					});
				},
			}),
		);

		expect(runtime.decode_title("A &amp; B")).toBe("A & B");
		await Effect.runPromise(runtime.preload_css(["/app.css"]));
		await Effect.runPromise(
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
				wait_for_css: () => {
					return Effect.fail(new Error("css wait failed"));
				},
			}),
		);

		const result = await Effect.runPromise(
			Effect.result(runtime.wait_for_css(["/broken.css"])),
		);

		expect(EffectResult.isFailure(result)).toBe(true);
		if (EffectResult.isFailure(result)) {
			expect(result.failure._tag).toBe("RouteCSSFailed");
		}
	});

	it("treats absent empty head sections as no-op route side effects", async () => {
		const runtime = Effect.runSync(make_route_dom_runtime());

		await Effect.runPromise(
			runtime.apply_payload_side_effects(
				payload({
					css_bundles: [],
					deps: [],
					meta_head_els: [],
					rest_head_els: [],
				}),
			),
		);
	});

	it("fails route side effects when missing head markers have real work", async () => {
		const runtime = Effect.runSync(make_route_dom_runtime());

		const result = await Effect.runPromise(
			Effect.result(
				runtime.apply_payload_side_effects(
					payload({
						meta_head_els: [
							{
								tag: "meta",
								attributesKnownSafe: {
									name: "description",
								},
							},
						],
					}),
				),
			),
		);

		expect(EffectResult.isFailure(result)).toBe(true);
		if (EffectResult.isFailure(result)) {
			expect(result.failure._tag).toBe("RouteDOMSideEffectFailed");
		}
	});
});
