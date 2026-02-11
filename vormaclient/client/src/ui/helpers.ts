import { vormaNavigate } from "../client.ts";
import {
	__resolvePath,
	type ExtractApp,
	type PermissivePatternBasedProps,
	type VormaAppBase,
	type VormaAppConfig,
	type VormaLoaderPattern,
	type VormaRouteParams,
} from "../app/helpers.ts";
import {
	__vormaClientGlobal,
	type getRouterData,
} from "../app/context.ts";
import type { RouteErrorComponent } from "../app/context.ts";
import {
	__getPrefetchHandlers,
	__makeLinkOnClickFn,
} from "../core/links.ts";

export const defaultErrorBoundary: RouteErrorComponent = (props: {
	error: string;
}) => {
	return "Route Error: " + props.error;
};

export type VormaLinkPropsBase<LinkOnClickCallback> = {
	href?: string;
	prefetch?: "intent";
	prefetchDelayMs?: number;
	beforeBegin?: LinkOnClickCallback;
	beforeRender?: LinkOnClickCallback;
	afterRender?: LinkOnClickCallback;
	scrollToTop?: boolean;
	replace?: boolean;
	state?: unknown;
};

function linkPropsToPrefetchObj<LinkOnClickCallback>(
	props: VormaLinkPropsBase<LinkOnClickCallback>,
) {
	if (!props.href || props.prefetch !== "intent") {
		return undefined;
	}

	return __getPrefetchHandlers({
		href: props.href,
		delayMs: props.prefetchDelayMs,
		beforeBegin: props.beforeBegin as any,
		beforeRender: props.beforeRender as any,
		afterRender: props.afterRender as any,
		scrollToTop: props.scrollToTop,
		replace: props.replace,
		state: props.state,
	});
}

function linkPropsToOnClickFn<LinkOnClickCallback>(
	props: VormaLinkPropsBase<LinkOnClickCallback>,
) {
	return __makeLinkOnClickFn({
		beforeBegin: props.beforeBegin as any,
		beforeRender: props.beforeRender as any,
		afterRender: props.afterRender as any,
		scrollToTop: props.scrollToTop,
		replace: props.replace,
		state: props.state,
	});
}

type handlerKeys = {
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
} satisfies handlerKeys;

export function __makeFinalLinkProps<LinkOnClickCallback>(
	props: VormaLinkPropsBase<LinkOnClickCallback>,
	keys: {
		onPointerEnter: string;
		onFocus: string;
		onPointerLeave: string;
		onBlur: string;
		onTouchCancel: string;
		onClick: string;
	} = standardCamelHandlerKeys,
) {
	const prefetchObj = linkPropsToPrefetchObj(props);

	return {
		dataExternal: prefetchObj?.isExternal || undefined,
		onPointerEnter: (e: any) => {
			prefetchObj?.start(e);
			if (isFn((props as any)[keys.onPointerEnter])) {
				(props as any)[keys.onPointerEnter](e);
			}
		},
		onFocus: (e: any) => {
			prefetchObj?.start(e);
			if (isFn((props as any)[keys.onFocus])) {
				(props as any)[keys.onFocus](e);
			}
		},
		onPointerLeave: (e: any) => {
			if (!__vormaClientGlobal.get("isTouchDevice")) {
				prefetchObj?.stop();
			}
			if (isFn((props as any)[keys.onPointerLeave])) {
				(props as any)[keys.onPointerLeave](e);
			}
		},
		onBlur: (e: any) => {
			prefetchObj?.stop();
			if (isFn((props as any)[keys.onBlur])) {
				(props as any)[keys.onBlur](e);
			}
		},
		onTouchCancel: (e: any) => {
			prefetchObj?.stop();
			if (isFn((props as any)[keys.onTouchCancel])) {
				(props as any)[keys.onTouchCancel](e);
			}
		},
		onClick: async (e: any) => {
			if (isFn((props as any)[keys.onClick])) {
				(props as any)[keys.onClick](e);
			}
			if (prefetchObj) {
				await prefetchObj.onClick(e);
			} else {
				await linkPropsToOnClickFn(props)(e);
			}
		},
	};
}

function isFn(fn: any): fn is (...args: Array<any>) => any {
	return typeof fn === "function";
}

export type VormaRoutePropsGeneric<
	JSXElement,
	App extends VormaAppBase,
	Pattern extends VormaLoaderPattern<App> = VormaLoaderPattern<App>,
> = {
	idx: number;
	Outlet: (props: Record<string, any>) => JSXElement;
	__phantom_pattern: Pattern;
} & Record<string, any>;

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
		props: VormaRoutePropsGeneric<any, App, Pattern>,
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
		const {
			pattern,
			params,
			splatValues,
			replace,
			scrollToTop,
			search,
			hash,
			state,
		} = options as any;

		const href = __resolvePath({
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
