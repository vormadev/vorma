import { createMemo, mergeProps, splitProps, type JSX } from "solid-js";
import type {
	ExtractApp,
	VormaAppBase,
	VormaLoaderPattern,
} from "vorma/client";
import {
	type VormaAppConfig,
	type VormaLinkPropsBase,
	makeFinalLinkProps,
} from "vorma/client/__internal";
import {
	buildTypedLinkDisplayName,
	buildTypedLinkHrefForRouteResolution,
	type TypedAdapterLinkDefaultProps,
	type TypedAdapterLinkProps,
} from "../../shared/src/typed_link_props.ts";

type VormaLinkEvent = Event;

export function VormaLink(
	props: JSX.AnchorHTMLAttributes<HTMLAnchorElement> &
		VormaLinkPropsBase<VormaLinkEvent>,
) {
	const finalLinkProps = createMemo(() =>
		makeFinalLinkProps<VormaLinkEvent>(props),
	);
	const [, rest] = splitProps(props, [
		"prefetch",
		"scrollToTop",
		"replace",
		"state",
	]);

	return (
		<a
			data-external={finalLinkProps().dataExternal}
			{...rest}
			onPointerEnter={finalLinkProps().onPointerEnter}
			onFocus={finalLinkProps().onFocus}
			onPointerLeave={finalLinkProps().onPointerLeave}
			onBlur={finalLinkProps().onBlur}
			onTouchCancel={finalLinkProps().onTouchCancel}
			onClick={finalLinkProps().onClick}
		>
			{props.children}
		</a>
	);
}

type TypedVormaLinkProps<
	App extends VormaAppBase,
	Pattern extends VormaLoaderPattern<App> = VormaLoaderPattern<App>,
> = TypedAdapterLinkProps<
	App,
	Pattern,
	JSX.AnchorHTMLAttributes<HTMLAnchorElement>,
	VormaLinkEvent
>;

type SplittableTypedVormaLinkProps<
	App extends VormaAppBase,
	Pattern extends VormaLoaderPattern<App>,
> = TypedVormaLinkProps<App, Pattern> & {
	params?: Record<string, string>;
	splatValues?: string[];
};

export function makeTypedLink<C extends VormaAppConfig>(
	vormaAppConfig: C,
	defaultProps?: TypedAdapterLinkDefaultProps<
		ExtractApp<C>,
		JSX.AnchorHTMLAttributes<HTMLAnchorElement>,
		VormaLinkEvent
	>,
) {
	type App = ExtractApp<C>;

	const TypedLink = <Pattern extends VormaLoaderPattern<App>>(
		props: TypedVormaLinkProps<App, Pattern>,
	) => {
		const mergedProps = mergeProps(defaultProps || {}, props);

		const [local, linkProps] = splitProps(
			mergedProps as SplittableTypedVormaLinkProps<App, Pattern>,
			[
				"pattern",
				"params",
				"splatValues",
				"search",
				"hash",
				"state",
			] as const,
		);

		const href = createMemo(() => {
			return buildTypedLinkHrefForRouteResolution({
				vormaAppConfig,
				routeResolutionInput: local,
			});
		});

		return <VormaLink {...linkProps} href={href()} state={local.state} />;
	};

	(TypedLink as any).displayName = buildTypedLinkDisplayName({
		defaultProps: defaultProps as Record<string, unknown> | undefined,
	});

	return TypedLink;
}
