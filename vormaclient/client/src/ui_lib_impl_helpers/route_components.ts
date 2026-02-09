import {
	__resolvePath,
	type VormaAppBase,
	type VormaLoaderPattern,
	type VormaRouteParams,
} from "../vorma_app_helpers/vorma_app_helpers.ts";
import {
	__vormaClientGlobal,
	type getRouterData,
} from "../vorma_ctx/vorma_ctx.ts";

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
