import { createMemo, splitProps, type JSX } from "solid-js";
import type {
	ExtractApp,
	VormaAppBase,
	VormaLoaderPattern,
} from "vorma/client";
import {
	buildTypedLinkDisplayName,
	makeFinalLinkProps,
	navigationInternalLinkPropKeysForAnchors,
	resolveTypedAdapterLinkWithDefaults,
	type TypedAdapterLinkDefaultProps,
	type TypedAdapterLinkProps,
	type VormaAppConfig,
	type VormaLinkPropsBase,
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
		...navigationInternalLinkPropKeysForAnchors,
	] as Array<keyof typeof props>);

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
		const resolvedProps = createMemo(() => {
			return resolveTypedAdapterLinkWithDefaults({
				vormaAppConfig,
				defaultProps,
				linkProps: props,
			});
		});

		return (
			<VormaLink
				{...(resolvedProps()
					.linkProps as JSX.AnchorHTMLAttributes<HTMLAnchorElement>)}
				href={resolvedProps().href}
				state={resolvedProps().state}
			/>
		);
	};

	(TypedLink as any).displayName = buildTypedLinkDisplayName({
		defaultProps: defaultProps as Record<string, unknown> | undefined,
	});

	return TypedLink;
}
