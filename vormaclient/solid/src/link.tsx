import { createMemo, mergeProps, splitProps, type JSX } from "solid-js";
import type {
	ExtractApp,
	PermissivePatternBasedProps,
	VormaAppBase,
	VormaLoaderPattern,
} from "vorma/client";
import {
	type VormaAppConfig,
	type VormaLinkPropsBase,
	makeFinalLinkProps,
	resolveTypedLinkHref,
} from "vorma/client/__internal";

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
> = Omit<JSX.AnchorHTMLAttributes<HTMLAnchorElement>, "href" | "pattern"> &
	VormaLinkPropsBase<VormaLinkEvent> &
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
			return resolveTypedLinkHref({
				vormaAppConfig,
				pattern: local.pattern,
				...(local.params && { params: local.params }),
				...(local.splatValues && {
					splatValues: local.splatValues,
				}),
				search: local.search,
				hash: local.hash,
			});
		});

		return <VormaLink {...linkProps} href={href()} state={local.state} />;
	};

	return TypedLink;
}
