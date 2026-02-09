import { h, type JSX } from "preact";
import { memo } from "preact/compat";
import type {
	ExtractApp,
	PermissivePatternBasedProps,
	VormaAppBase,
	VormaLoaderPattern,
} from "vorma/client";
import {
	__makeFinalLinkProps,
	__resolvePath,
	type VormaAppConfig,
	type VormaLinkPropsBase,
} from "vorma/client";

export const VormaLink = memo(function VormaLink(
	props: JSX.HTMLAttributes<HTMLAnchorElement> &
		VormaLinkPropsBase<
			(
				e: JSX.TargetedMouseEvent<HTMLAnchorElement>,
			) => void | Promise<void>
		>,
) {
	const finalLinkProps = __makeFinalLinkProps(props);
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
> = Omit<JSX.HTMLAttributes<HTMLAnchorElement>, "href" | "pattern"> &
	VormaLinkPropsBase<
		(e: JSX.TargetedMouseEvent<HTMLAnchorElement>) => void | Promise<void>
	> &
	PermissivePatternBasedProps<App, Pattern> & {
		search?: string;
		hash?: string;
	};

export function makeTypedLink<C extends VormaAppConfig>(
	vormaAppConfig: C,
	defaultProps?: Partial<
		Omit<
			TypedVormaLinkProps<ExtractApp<C>>,
			"pattern" | "params" | "splatValues"
		>
	>,
) {
	type App = ExtractApp<C>;

	const TypedLink = memo(function TypedLink<
		Pattern extends VormaLoaderPattern<App>,
	>(props: TypedVormaLinkProps<App, Pattern>) {
		const {
			pattern,
			params,
			splatValues,
			search,
			hash,
			state,
			...linkProps
		} = props as any;

		const href = __resolvePath({
			vormaAppConfig,
			type: "loader",
			props: {
				pattern,
				...(params && { params }),
				...(splatValues && { splatValues }),
			},
		});

		const url = new URL(href, window.location.origin);
		if (search !== undefined) url.search = search;
		if (hash !== undefined) url.hash = hash;

		const finalProps = {
			...defaultProps,
			...linkProps,
			href: url.href,
			state,
		};

		return h(VormaLink, finalProps);
	});

	(TypedLink as any).displayName =
		`TypedLink(${Object.keys(defaultProps || {}).join(", ")})`;
	return TypedLink;
}
