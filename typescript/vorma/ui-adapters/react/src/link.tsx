import { memo, type ComponentProps, type JSX } from "react";
import type {
	ExtractApp,
	VormaAppBase,
	VormaLoaderPattern,
} from "vorma/client";
import {
	buildTypedLinkDisplayName,
	buildTypedLinkResolvedProps,
	makeFinalLinkProps,
	mergeTypedAdapterLinkPropsWithDefaults,
	type TypedAdapterLinkDefaultProps,
	type TypedAdapterLinkProps,
	type VormaAppConfig,
	type VormaLinkPropsBase,
} from "vorma/client/__internal";

export const VormaLink = memo(function VormaLink(
	props: ComponentProps<"a"> &
		VormaLinkPropsBase<React.MouseEvent<HTMLAnchorElement, MouseEvent>>,
) {
	const finalLinkProps = makeFinalLinkProps(props);
	// oxlint-disable-next-line no-unused-vars
	const { prefetch, scrollToTop, replace, state, ...rest } = props;

	return (
		<a
			data-external={finalLinkProps.dataExternal}
			{...(rest as any)}
			onPointerEnter={finalLinkProps.onPointerEnter}
			onFocus={finalLinkProps.onFocus}
			onPointerLeave={finalLinkProps.onPointerLeave}
			onBlur={finalLinkProps.onBlur}
			onTouchCancel={finalLinkProps.onTouchCancel}
			onClick={finalLinkProps.onClick}
		>
			{props.children}
		</a>
	);
});

type TypedVormaLinkProps<
	App extends VormaAppBase,
	Pattern extends VormaLoaderPattern<App> = VormaLoaderPattern<App>,
> = TypedAdapterLinkProps<
	App,
	Pattern,
	ComponentProps<"a">,
	React.MouseEvent<HTMLAnchorElement, MouseEvent>
>;

export function makeTypedLink<C extends VormaAppConfig>(
	vormaAppConfig: C,
	defaultProps?: TypedAdapterLinkDefaultProps<
		ExtractApp<C>,
		ComponentProps<"a">,
		React.MouseEvent<HTMLAnchorElement, MouseEvent>
	>,
) {
	type App = ExtractApp<C>;

	const TypedLink = <Pattern extends VormaLoaderPattern<App>>(
		props: TypedVormaLinkProps<App, Pattern>,
	) => {
		const mergedProps = mergeTypedAdapterLinkPropsWithDefaults<
			App,
			Pattern,
			ComponentProps<"a">,
			React.MouseEvent<HTMLAnchorElement, MouseEvent>
		>({
			defaultProps,
			linkProps: props,
		});
		const resolvedProps = buildTypedLinkResolvedProps({
			vormaAppConfig,
			mergedProps,
		});

		return (
			<VormaLink
				{...resolvedProps.linkProps}
				href={resolvedProps.href}
				state={resolvedProps.state}
			/>
		);
	};

	const MemoizedTypedLink = memo(TypedLink) as <
		Pattern extends VormaLoaderPattern<App>,
	>(
		props: TypedVormaLinkProps<App, Pattern>,
	) => JSX.Element;

	(MemoizedTypedLink as any).displayName = buildTypedLinkDisplayName({
		defaultProps: defaultProps as Record<string, unknown> | undefined,
	});

	return MemoizedTypedLink;
}
