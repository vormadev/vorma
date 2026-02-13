import { vormaNavigate } from "../client.ts";
import {
	resolvePath,
	type ExtractApp,
	type PermissivePatternBasedProps,
	type VormaAppBase,
	type VormaAppConfig,
	type VormaLoaderPattern,
	type VormaRouteParams,
} from "../app/helpers.ts";
import { __vormaClientGlobal, type getRouterData } from "../app/context.ts";
import type { RouteErrorComponent } from "../app/context.ts";
import { __getPrefetchHandlers, __makeLinkOnClickFn } from "../core/links.ts";

export const defaultErrorBoundary: RouteErrorComponent = (props: {
	error: string;
}) => {
	return "Route Error: " + props.error;
};

type LinkOnClickCallback<LinkEvent = unknown> = (
	event: LinkEvent,
) => void | Promise<void>;

export type VormaLinkPropsBase<LinkEvent = unknown> = {
	href?: string;
	prefetch?: "intent";
	prefetchDelayMs?: number;
	beforeBegin?: LinkOnClickCallback<LinkEvent>;
	beforeRender?: LinkOnClickCallback<LinkEvent>;
	afterRender?: LinkOnClickCallback<LinkEvent>;
	scrollToTop?: boolean;
	replace?: boolean;
	state?: unknown;
};

function adaptDOMEventCallback<LinkEvent>(
	callback: LinkOnClickCallback<LinkEvent> | undefined,
): ((event: Event) => void | Promise<void>) | undefined {
	if (!callback) return undefined;
	return (event: Event) => callback(event as unknown as LinkEvent);
}

function linkPropsToPrefetchObj<LinkEvent>(
	props: VormaLinkPropsBase<LinkEvent>,
) {
	if (!props.href || props.prefetch !== "intent") {
		return undefined;
	}

	return __getPrefetchHandlers({
		href: props.href,
		delayMs: props.prefetchDelayMs,
		beforeBegin: adaptDOMEventCallback(props.beforeBegin),
		beforeRender: adaptDOMEventCallback(props.beforeRender),
		afterRender: adaptDOMEventCallback(props.afterRender),
		scrollToTop: props.scrollToTop,
		replace: props.replace,
		state: props.state,
	});
}

function linkPropsToOnClickFn<LinkEvent>(props: VormaLinkPropsBase<LinkEvent>) {
	return __makeLinkOnClickFn({
		beforeBegin: adaptDOMEventCallback(props.beforeBegin),
		beforeRender: adaptDOMEventCallback(props.beforeRender),
		afterRender: adaptDOMEventCallback(props.afterRender),
		scrollToTop: props.scrollToTop,
		replace: props.replace,
		state: props.state,
	});
}

type HandlerKeys = {
	onPointerEnter: string;
	onFocus: string;
	onPointerLeave: string;
	onBlur: string;
	onTouchCancel: string;
	onClick: string;
};

const standardCamelHandlerKeys = {
	onPointerEnter: "onPointerEnter",
	onFocus: "onFocus",
	onPointerLeave: "onPointerLeave",
	onBlur: "onBlur",
	onTouchCancel: "onTouchCancel",
	onClick: "onClick",
} satisfies HandlerKeys;

type UnknownFn = (...args: ReadonlyArray<unknown>) => unknown;

function isFn(fn: unknown): fn is UnknownFn {
	return typeof fn === "function";
}

export function makeFinalLinkProps<LinkEvent>(
	props: VormaLinkPropsBase<LinkEvent>,
	keys: HandlerKeys = standardCamelHandlerKeys,
) {
	const prefetchObj = linkPropsToPrefetchObj(props);
	const propsBag = props as Record<string, unknown>;

	function callOriginalHandlerIfPresent(
		handlerKey: string,
		event: LinkEvent,
	): void {
		const maybeHandler = propsBag[handlerKey];
		if (isFn(maybeHandler)) {
			maybeHandler(event);
		}
	}

	return {
		dataExternal: prefetchObj?.isExternal || undefined,
		onPointerEnter: (event: LinkEvent) => {
			prefetchObj?.start(event as Event);
			callOriginalHandlerIfPresent(keys.onPointerEnter, event);
		},
		onFocus: (event: LinkEvent) => {
			prefetchObj?.start(event as Event);
			callOriginalHandlerIfPresent(keys.onFocus, event);
		},
		onPointerLeave: (event: LinkEvent) => {
			if (!__vormaClientGlobal.get("isTouchDevice")) {
				prefetchObj?.stop();
			}
			callOriginalHandlerIfPresent(keys.onPointerLeave, event);
		},
		onBlur: (event: LinkEvent) => {
			prefetchObj?.stop();
			callOriginalHandlerIfPresent(keys.onBlur, event);
		},
		onTouchCancel: (event: LinkEvent) => {
			prefetchObj?.stop();
			callOriginalHandlerIfPresent(keys.onTouchCancel, event);
		},
		onClick: async (event: LinkEvent) => {
			callOriginalHandlerIfPresent(keys.onClick, event);
			if (prefetchObj) {
				await prefetchObj.onClick(event as Event);
			} else {
				await linkPropsToOnClickFn(props)(event as Event);
			}
		},
	};
}

export type VormaRoutePropsGeneric<
	JSXElement,
	App extends VormaAppBase,
	Pattern extends VormaLoaderPattern<App> = VormaLoaderPattern<App>,
> = {
	idx: number;
	Outlet: (props: Record<string, unknown>) => JSXElement;
	__phantom_pattern: Pattern;
} & Record<string, unknown>;

export type VormaRouteGeneric<
	JSXElement,
	App extends VormaAppBase,
	Pattern extends VormaLoaderPattern<App> = VormaLoaderPattern<App>,
> = (props: VormaRoutePropsGeneric<JSXElement, App, Pattern>) => JSXElement;

export type ParamsForPattern<
	App extends VormaAppBase,
	Pattern extends VormaLoaderPattern<App>,
> = VormaRouteParams<App, Pattern>;

type BaseRouterData<RootData, Params extends string> = ReturnType<
	typeof getRouterData<RootData, Record<Params, string>>
>;

type Wrapper<UseAccessor extends boolean, T> = UseAccessor extends false
	? T
	: () => T;

export type UseRouterDataFunction<
	App extends VormaAppBase,
	UseAccessor extends boolean = false,
> = {
	<Pattern extends VormaLoaderPattern<App>>(
		props: VormaRoutePropsGeneric<unknown, App, Pattern>,
	): Wrapper<
		UseAccessor,
		BaseRouterData<App["rootData"], ParamsForPattern<App, Pattern>>
	>;
	<Pattern extends VormaLoaderPattern<App>>(): Wrapper<
		UseAccessor,
		BaseRouterData<App["rootData"], ParamsForPattern<App, Pattern>>
	>;
	(): Wrapper<UseAccessor, BaseRouterData<App["rootData"], string>>;
};

type TypedNavigateOptions<
	App extends VormaAppBase,
	Pattern extends VormaLoaderPattern<App>,
> = PermissivePatternBasedProps<App, Pattern> & {
	replace?: boolean;
	scrollToTop?: boolean;
	search?: string;
	hash?: string;
	state?: unknown;
};

export function makeTypedNavigate<C extends VormaAppConfig>(vormaAppConfig: C) {
	type App = ExtractApp<C>;

	return async function typedNavigate<
		Pattern extends VormaLoaderPattern<App>,
	>(options: TypedNavigateOptions<App, Pattern>): Promise<void> {
		const { pattern, replace, scrollToTop, search, hash, state } = options;
		const params = "params" in options ? options.params : undefined;
		const splatValues =
			"splatValues" in options ? options.splatValues : undefined;

		const href = resolvePath({
			vormaAppConfig,
			type: "loader",
			props: {
				pattern,
				...(params && { params }),
				...(splatValues && { splatValues }),
			},
		});

		return vormaNavigate(href, {
			replace,
			scrollToTop,
			search,
			hash,
			state,
		});
	};
}

export const __makeFinalLinkProps = makeFinalLinkProps;
