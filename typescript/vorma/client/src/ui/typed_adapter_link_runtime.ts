import type {
	VormaAppBase,
	VormaAppConfig,
	VormaLoaderPattern,
} from "../app/helpers.ts";
import {
	buildTypedLinkResolvedProps,
	type TypedAdapterLinkDefaultProps,
	type TypedAdapterLinkProps,
} from "./typed_adapter_link_props.ts";

type TypedAdapterLinkMergedProps<
	App extends VormaAppBase,
	Pattern extends VormaLoaderPattern<App>,
	AnchorProps,
	LinkEvent,
> = TypedAdapterLinkProps<App, Pattern, AnchorProps, LinkEvent>;

export function mergeTypedAdapterLinkPropsWithDefaults<
	App extends VormaAppBase,
	Pattern extends VormaLoaderPattern<App>,
	AnchorProps,
	LinkEvent,
>(props: {
	defaultProps:
		| TypedAdapterLinkDefaultProps<App, AnchorProps, LinkEvent>
		| undefined;
	linkProps: TypedAdapterLinkProps<App, Pattern, AnchorProps, LinkEvent>;
}): TypedAdapterLinkMergedProps<App, Pattern, AnchorProps, LinkEvent> {
	return {
		...props.defaultProps,
		...props.linkProps,
	} as TypedAdapterLinkMergedProps<App, Pattern, AnchorProps, LinkEvent>;
}

export const navigationInternalLinkPropKeysForAnchors = [
	"prefetch",
	"prefetchDelayMs",
	"beforeBegin",
	"beforeRender",
	"afterRender",
	"scrollToTop",
	"replace",
	"state",
] as const;

type NavigationInternalLinkPropKey =
	(typeof navigationInternalLinkPropKeysForAnchors)[number];

export function stripNavigationInternalLinkPropsForAnchor<
	AnchorProps extends Record<string, unknown>,
>(props: AnchorProps): Omit<AnchorProps, NavigationInternalLinkPropKey> {
	const nextProps = { ...props } as Record<string, unknown>;
	for (const key of navigationInternalLinkPropKeysForAnchors) {
		delete nextProps[key];
	}
	return nextProps as Omit<AnchorProps, NavigationInternalLinkPropKey>;
}

export function resolveTypedAdapterLinkWithDefaults<
	App extends VormaAppBase,
	Pattern extends VormaLoaderPattern<App>,
	AnchorProps,
	LinkEvent,
>(props: {
	vormaAppConfig: VormaAppConfig;
	defaultProps:
		| TypedAdapterLinkDefaultProps<App, AnchorProps, LinkEvent>
		| undefined;
	linkProps: TypedAdapterLinkProps<App, Pattern, AnchorProps, LinkEvent>;
}) {
	const mergedProps = mergeTypedAdapterLinkPropsWithDefaults<
		App,
		Pattern,
		AnchorProps,
		LinkEvent
	>({
		defaultProps: props.defaultProps,
		linkProps: props.linkProps,
	});

	return buildTypedLinkResolvedProps({
		vormaAppConfig: props.vormaAppConfig,
		mergedProps,
	});
}
