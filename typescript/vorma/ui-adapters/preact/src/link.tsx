import { h, type HTMLAttributes, type TargetedMouseEvent } from "preact";
import { memo } from "preact/compat";
import type {
	ExtractApp,
	VormaAppBase,
	VormaLoaderPattern,
} from "vorma/client";
import {
	buildTypedLinkDisplayName,
	buildTypedLinkResolvedProps,
	makeFinalLinkProps,
	type TypedAdapterLinkDefaultProps,
	type TypedAdapterLinkProps,
	type VormaAppConfig,
	type VormaLinkPropsBase,
} from "vorma/client/__internal";

export const VormaLink = memo(function VormaLink(
	props: HTMLAttributes<HTMLAnchorElement> &
		VormaLinkPropsBase<TargetedMouseEvent<HTMLAnchorElement>>,
) {
	const finalLinkProps = makeFinalLinkProps(props);
	// oxlint-disable-next-line no-unused-vars
	const { prefetch, scrollToTop, replace, state, ...rest } = props;

	return h(
		"a",
		{
			"data-external": finalLinkProps.dataExternal,
			...(rest as any),
			onPointerEnter: finalLinkProps.onPointerEnter,
			onFocus: finalLinkProps.onFocus,
			onPointerLeave: finalLinkProps.onPointerLeave,
			onBlur: finalLinkProps.onBlur,
			onTouchCancel: finalLinkProps.onTouchCancel,
			onClick: finalLinkProps.onClick,
		},
		props.children,
	);
});

type TypedVormaLinkProps<
	App extends VormaAppBase,
	Pattern extends VormaLoaderPattern<App> = VormaLoaderPattern<App>,
> = TypedAdapterLinkProps<
	App,
	Pattern,
	HTMLAttributes<HTMLAnchorElement>,
	TargetedMouseEvent<HTMLAnchorElement>
>;

export function makeTypedLink<C extends VormaAppConfig>(
	vormaAppConfig: C,
	defaultProps?: TypedAdapterLinkDefaultProps<
		ExtractApp<C>,
		HTMLAttributes<HTMLAnchorElement>,
		TargetedMouseEvent<HTMLAnchorElement>
	>,
) {
	type App = ExtractApp<C>;

	const TypedLink = memo(function TypedLink<
		Pattern extends VormaLoaderPattern<App>,
	>(props: TypedVormaLinkProps<App, Pattern>) {
		const mergedProps: TypedVormaLinkProps<App, Pattern> = {
			...defaultProps,
			...props,
		};
		const resolvedProps = buildTypedLinkResolvedProps({
			vormaAppConfig,
			mergedProps,
		});

		return h(VormaLink, {
			...resolvedProps.linkProps,
			href: resolvedProps.href,
			state: resolvedProps.state,
		});
	});

	(TypedLink as any).displayName = buildTypedLinkDisplayName({
		defaultProps: defaultProps as Record<string, unknown> | undefined,
	});
	return TypedLink;
}
