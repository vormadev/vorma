import type { VormaAppBase, VormaLoaderPattern } from "../app/helpers.ts";
import {
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
