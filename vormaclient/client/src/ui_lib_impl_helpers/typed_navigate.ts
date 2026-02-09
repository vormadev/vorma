import { vormaNavigate } from "../client.ts";
import {
	__resolvePath,
	type ExtractApp,
	type PermissivePatternBasedProps,
	type VormaAppBase,
	type VormaAppConfig,
	type VormaLoaderPattern,
} from "../vorma_app_helpers/vorma_app_helpers.ts";
import { __vormaClientGlobal } from "../vorma_ctx/vorma_ctx.ts";

type TypedNavigateOptions<
	App extends VormaAppBase,
	Pattern extends VormaLoaderPattern<App>,
> = PermissivePatternBasedProps<App, Pattern> & {
	replace?: boolean;
	scrollToTop?: boolean;
	search?: string;
	hash?: string;
	state?: unknown;
};

export function makeTypedNavigate<C extends VormaAppConfig>(vormaAppConfig: C) {
	type App = ExtractApp<C>;

	return async function typedNavigate<
		Pattern extends VormaLoaderPattern<App>,
	>(options: TypedNavigateOptions<App, Pattern>): Promise<void> {
		const {
			pattern,
			params,
			splatValues,
			replace,
			scrollToTop,
			search,
			hash,
			state,
		} = options as any;

		const href = __resolvePath({
			vormaAppConfig,
			type: "loader",
			props: {
				pattern,
				...(params && { params }),
				...(splatValues && { splatValues }),
			},
		});

		return vormaNavigate(href, {
			replace,
			scrollToTop,
			search,
			hash,
			state,
		});
	};
}
