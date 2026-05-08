import { Effect } from "effect";
import { apply_css_bundles, preload_css, wait_for_css } from "../css.ts";
import { type HeadEl, apply_head_and_title } from "../head.ts";
import { preload_modules } from "../modules.ts";
import type { DecodedPayload } from "./route_preparer.ts";
import { RouteCSSFailed, RouteDOMSideEffectFailed } from "./route_preparer.ts";

export type RouteDOMRuntime = {
	decode_title: (html: string) => string;
	preload_css: (css_bundles: string[]) => Effect.Effect<void>;
	wait_for_css: (
		css_bundles: string[],
	) => Effect.Effect<void, RouteCSSFailed>;
	apply_payload_side_effects: (
		payload: DecodedPayload,
	) => Effect.Effect<void, RouteDOMSideEffectFailed>;
};

export type RouteDOMRuntimeOptions = {
	apply_css_bundles?: (css_bundles: string[]) => void;
	apply_head_and_title?: (
		title: string | undefined,
		meta_head_els: HeadEl[],
		rest_head_els: HeadEl[],
	) => void;
	create_title_decoder?: () => HTMLElement;
	preload_css?: (css_bundles: string[]) => void;
	preload_modules?: (deps: string[]) => void;
	wait_for_css?: (
		css_bundles: string[],
		signal: AbortSignal,
	) => Promise<void>;
};

export function make_route_dom_runtime(
	options: RouteDOMRuntimeOptions = {},
): Effect.Effect<RouteDOMRuntime> {
	return Effect.sync(() => {
		const apply_css_bundles_impl =
			options.apply_css_bundles ?? apply_css_bundles;
		const apply_head_and_title_impl =
			options.apply_head_and_title ??
			((title, meta_head_els, rest_head_els): void => {
				apply_head_and_title(title, meta_head_els, rest_head_els);
			});
		const create_title_decoder =
			options.create_title_decoder ??
			((): HTMLElement => {
				return document.createElement("textarea");
			});
		const preload_css_impl = options.preload_css ?? preload_css;
		const preload_modules_impl = options.preload_modules ?? preload_modules;
		const wait_for_css_impl = options.wait_for_css ?? wait_for_css;

		return {
			decode_title: (html) => {
				const element = create_title_decoder();
				element.innerHTML = html;
				return "value" in element && typeof element.value === "string"
					? element.value
					: (element.textContent ?? "");
			},
			preload_css: (css_bundles) => {
				return Effect.sync(() => {
					preload_css_impl(css_bundles);
				});
			},
			wait_for_css: (css_bundles) => {
				return Effect.tryPromise({
					try: (signal) => {
						return wait_for_css_impl(css_bundles, signal);
					},
					catch: (error) => {
						return new RouteCSSFailed({ error });
					},
				});
			},
			apply_payload_side_effects: (payload) => {
				return Effect.try({
					try: () => {
						apply_head_and_title_impl(
							payload.title,
							payload.meta_head_els as HeadEl[],
							payload.rest_head_els as HeadEl[],
						);
						apply_css_bundles_impl(payload.css_bundles);
						preload_modules_impl(payload.deps);
					},
					catch: (error) => {
						return new RouteDOMSideEffectFailed({ error });
					},
				});
			},
		};
	});
}
