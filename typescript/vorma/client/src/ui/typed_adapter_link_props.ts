import type {
	PermissivePatternBasedProps,
	VormaAppBase,
	VormaLoaderPattern,
} from "../app/helpers.ts";
import { type VormaAppConfig } from "../app/helpers.ts";
import { type VormaLinkPropsBase, resolveTypedLinkHref } from "./helpers.ts";

type TypedLinkRouteResolutionInput = {
	pattern: string;
	params?: Record<string, string>;
	splatValues?: string[];
	search?: string;
	hash?: string;
};

type TypedLinkMergedPropsBase = TypedLinkRouteResolutionInput & {
	state?: unknown;
};

export type TypedAdapterLinkProps<
	App extends VormaAppBase,
	Pattern extends VormaLoaderPattern<App>,
	AnchorProps,
	LinkEvent,
> = Omit<AnchorProps, "href" | "pattern"> &
	VormaLinkPropsBase<LinkEvent> &
	PermissivePatternBasedProps<App, Pattern> & {
		search?: string;
		hash?: string;
	};

export type TypedAdapterLinkDefaultProps<
	App extends VormaAppBase,
	AnchorProps,
	LinkEvent,
> = Partial<
	Omit<
		TypedAdapterLinkProps<
			App,
			VormaLoaderPattern<App>,
			AnchorProps,
			LinkEvent
		>,
		"pattern" | "params" | "splatValues"
	>
>;

type TypedLinkResolvedNonRouteProps<
	MergedProps extends TypedLinkMergedPropsBase,
> = Omit<MergedProps, keyof TypedLinkRouteResolutionInput | "state">;

export function buildTypedLinkHrefForRouteResolution(props: {
	vormaAppConfig: VormaAppConfig;
	routeResolutionInput: TypedLinkRouteResolutionInput;
}): string {
	const { routeResolutionInput } = props;

	return resolveTypedLinkHref({
		vormaAppConfig: props.vormaAppConfig,
		pattern: routeResolutionInput.pattern,
		...(routeResolutionInput.params && {
			params: routeResolutionInput.params,
		}),
		...(routeResolutionInput.splatValues && {
			splatValues: routeResolutionInput.splatValues,
		}),
		search: routeResolutionInput.search,
		hash: routeResolutionInput.hash,
	});
}

export function buildTypedLinkResolvedProps<
	MergedProps extends TypedLinkMergedPropsBase,
>(props: {
	vormaAppConfig: VormaAppConfig;
	mergedProps: MergedProps;
}): {
	href: string;
	state: MergedProps["state"];
	linkProps: TypedLinkResolvedNonRouteProps<MergedProps>;
} {
	const { pattern, params, splatValues, search, hash, state, ...linkProps } =
		props.mergedProps;

	const href = buildTypedLinkHrefForRouteResolution({
		vormaAppConfig: props.vormaAppConfig,
		routeResolutionInput: {
			pattern,
			params,
			splatValues,
			search,
			hash,
		},
	});

	return {
		href,
		state,
		linkProps,
	};
}

export function buildTypedLinkDisplayName(props: {
	defaultProps: Record<string, unknown> | undefined;
}): string {
	return `TypedLink(${Object.keys(props.defaultProps || {}).join(", ")})`;
}
