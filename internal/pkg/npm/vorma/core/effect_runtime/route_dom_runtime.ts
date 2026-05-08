import { Effect } from "effect";
import {
	apply_css_bundles_effect,
	preload_css_effect,
	wait_for_css_effect,
} from "../css.ts";
import { type HeadEl, apply_head_and_title_effect } from "../head.ts";
import { preload_modules_effect } from "../modules.ts";
import type { DecodedPayload } from "./route_preparer.ts";
import { RouteCSSFailed, RouteDOMSideEffectFailed } from "./route_preparer.ts";

export type RouteDOMRuntime = {
	decode_title: (html: string) => string;
	preload_css: (css_bundles: string[]) => Effect.Effect<void, RouteCSSFailed>;
	wait_for_css: (
		css_bundles: string[],
	) => Effect.Effect<void, RouteCSSFailed>;
	apply_payload_side_effects: (
		payload: DecodedPayload,
	) => Effect.Effect<void, RouteDOMSideEffectFailed>;
};

export type RouteDOMRuntimeOptions = {
	apply_css_bundles?: (css_bundles: string[]) => Effect.Effect<void, unknown>;
	apply_head_and_title?: (
		title: string | undefined,
		meta_head_els: HeadEl[],
		rest_head_els: HeadEl[],
	) => Effect.Effect<void, unknown>;
	create_title_decoder?: () => HTMLElement;
	preload_css?: (css_bundles: string[]) => Effect.Effect<void, unknown>;
	preload_modules?: (deps: string[]) => Effect.Effect<void, unknown>;
	wait_for_css?: (css_bundles: string[]) => Effect.Effect<void, unknown>;
};

export function make_route_dom_runtime(
	options: RouteDOMRuntimeOptions = {},
): Effect.Effect<RouteDOMRuntime> {
	return Effect.sync(() => {
		const apply_css_bundles_impl =
			options.apply_css_bundles ?? apply_css_bundles_effect;
		const apply_head_and_title_impl =
			options.apply_head_and_title ??
			((title, meta_head_els, rest_head_els) => {
				return apply_head_and_title_effect(
					title,
					meta_head_els,
					rest_head_els,
					{ allow_missing_empty_sections: true },
				);
			});
		const create_title_decoder =
			options.create_title_decoder ??
			((): HTMLElement => {
				return document.createElement("textarea");
			});
		const preload_css_impl = options.preload_css ?? preload_css_effect;
		const preload_modules_impl =
			options.preload_modules ?? preload_modules_effect;
		const wait_for_css_impl = options.wait_for_css ?? wait_for_css_effect;

		return {
			decode_title: (html) => {
				const element = create_title_decoder();
				element.innerHTML = html;
				return "value" in element && typeof element.value === "string"
					? element.value
					: (element.textContent ?? "");
			},
			preload_css: (css_bundles) => {
				return preload_css_impl(css_bundles).pipe(
					Effect.mapError((error) => {
						return new RouteCSSFailed({ error });
					}),
				);
			},
			wait_for_css: (css_bundles) => {
				return wait_for_css_impl(css_bundles).pipe(
					Effect.mapError((error) => {
						return new RouteCSSFailed({ error });
					}),
				);
			},
			apply_payload_side_effects: (payload) => {
				return Effect.gen(function* () {
					yield* apply_head_and_title_impl(
						payload.title,
						payload.meta_head_els as HeadEl[],
						payload.rest_head_els as HeadEl[],
					);
					yield* apply_css_bundles_impl(payload.css_bundles);
					yield* preload_modules_impl(payload.deps);
				}).pipe(
					Effect.mapError((error) => {
						return new RouteDOMSideEffectFailed({ error });
					}),
				);
			},
		};
	});
}
