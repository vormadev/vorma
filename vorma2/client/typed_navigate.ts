/// <reference types="vite/client" />

import { vormaNavigate } from "./public_api.ts";
import type {
	ExtractApp,
	PermissivePatternBasedProps,
	VormaAppConfig,
	VormaLoaderPattern,
} from "./types.ts";
import { make_absolute_with_search_hash, resolve_path } from "./url.ts";

export function makeTypedNavigate<C extends VormaAppConfig>(cfg: C) {
	type App = ExtractApp<C>;
	return async <P extends VormaLoaderPattern<App>>(
		props: PermissivePatternBasedProps<App, P> & {
			replace?: boolean;
			scrollToTop?: boolean;
			search?: string;
			hash?: string;
		},
	): Promise<{ didNavigate: boolean }> => {
		const a = props as any;
		const href = make_absolute_with_search_hash({
			href: resolve_path({
				type: "loader",
				app_config: cfg,
				pattern: props.pattern,
				params: a.params,
				splat_values: a.splatValues,
			}),
			search: props.search,
			hash: props.hash,
		});
		return vormaNavigate(href, {
			replace: props.replace,
			scrollToTop: props.scrollToTop,
		});
	};
}
