import { createMemo, mergeProps, splitProps, type JSX } from "solid-js";
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

export function VormaLink(
	props: JSX.AnchorHTMLAttributes<HTMLAnchorElement> &
		VormaLinkPropsBase<
			JSX.CustomEventHandlersCamelCase<HTMLAnchorElement>["onClick"]
		>,
) {
	const finalLinkProps = createMemo(() => __makeFinalLinkProps(props));
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
> = Omit<JSX.AnchorHTMLAttributes<HTMLAnchorElement>, "href" | "pattern"> &
	VormaLinkPropsBase<
		JSX.CustomEventHandlersCamelCase<HTMLAnchorElement>["onClick"]
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

	const TypedLink = <Pattern extends VormaLoaderPattern<App>>(
		props: TypedVormaLinkProps<App, Pattern>,
	) => {
		const merged = mergeProps(defaultProps || {}, props);

		const [local, linkProps] = splitProps(merged as any, [
			"pattern",
			"params",
			"splatValues",
			"search",
			"hash",
			"state",
		]);

		const href = createMemo(() => {
			const basePath = __resolvePath({
				vormaAppConfig,
				type: "loader",
				props: {
					pattern: local.pattern,
					...(local.params && { params: local.params }),
					...(local.splatValues && {
						splatValues: local.splatValues,
					}),
				},
			});
			const url = new URL(basePath, window.location.origin);
			if (local.search !== undefined) url.search = local.search;
			if (local.hash !== undefined) url.hash = local.hash;
			return url.href;
		});

		return <VormaLink {...linkProps} href={href()} state={local.state} />;
	};

	return TypedLink;
}
