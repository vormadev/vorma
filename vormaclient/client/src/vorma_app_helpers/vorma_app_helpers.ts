import { resolveVormaRequestBody } from "./body_resolution.ts";
import { resolveVormaPath } from "./path_resolution.ts";
import { buildVormaURL } from "./url_build.ts";
import type { SubmitOptions } from "../client.ts";

export type VormaAppConfig = {
	actionsRouterMountRoot: string;
	actionsDynamicRune: string;
	actionsSplatRune: string;
	loadersDynamicRune: string;
	loadersSplatRune: string;
	loadersExplicitIndexSegment: string;
	__phantom?: any;
};

export type VormaAppBase = {
	routes: readonly any[];
	appConfig: VormaAppConfig;
	rootData: any;
};

export type ExtractApp<C extends VormaAppConfig> = C["__phantom"];

type RouteByType<App extends VormaAppBase, T extends string> = Extract<
	App["routes"][number],
	{ _type: T }
>;

type RouteByPattern<Routes, P> = Extract<Routes, { pattern: P }>;

type VormaLoader<App extends VormaAppBase> = RouteByType<App, "loader">;
type VormaQuery<App extends VormaAppBase> = RouteByType<App, "query">;
type VormaMutation<App extends VormaAppBase> = RouteByType<App, "mutation">;

// Pattern types
export type VormaLoaderPattern<App extends VormaAppBase> =
	VormaLoader<App>["pattern"];
export type VormaQueryPattern<App extends VormaAppBase> =
	VormaQuery<App>["pattern"];
export type VormaMutationPattern<App extends VormaAppBase> =
	VormaMutation<App>["pattern"];

// IO types
export type VormaLoaderOutput<
	App extends VormaAppBase,
	P extends VormaLoaderPattern<App>,
> =
	RouteByPattern<VormaLoader<App>, P> extends { phantomOutputType: infer T }
		? T
		: null | undefined;

export type VormaQueryInput<
	App extends VormaAppBase,
	P extends VormaQueryPattern<App>,
> =
	RouteByPattern<VormaQuery<App>, P> extends { phantomInputType: infer T }
		? T
		: null | undefined;

export type VormaQueryOutput<
	App extends VormaAppBase,
	P extends VormaQueryPattern<App>,
> =
	RouteByPattern<VormaQuery<App>, P> extends { phantomOutputType: infer T }
		? T
		: null | undefined;

export type VormaMutationInput<
	App extends VormaAppBase,
	P extends VormaMutationPattern<App>,
> =
	RouteByPattern<VormaMutation<App>, P> extends { phantomInputType: infer T }
		? T
		: null | undefined;

export type VormaMutationOutput<
	App extends VormaAppBase,
	P extends VormaMutationPattern<App>,
> =
	RouteByPattern<VormaMutation<App>, P> extends { phantomOutputType: infer T }
		? T
		: null | undefined;

export type VormaMutationMethod<
	App extends VormaAppBase,
	P extends VormaMutationPattern<App>,
> =
	RouteByPattern<VormaMutation<App>, P> extends { method: infer M }
		? M extends string
			? M
			: "POST"
		: "POST";

// Route metadata
type RouteMetadata<App extends VormaAppBase, P extends string> = Extract<
	App["routes"][number],
	{ pattern: P }
>;

export type GetParams<App extends VormaAppBase, P extends string> =
	RouteMetadata<App, P> extends { params: ReadonlyArray<infer Params> }
		? Params extends string
			? Params
			: never
		: never;

export type VormaRouteParams<
	App extends VormaAppBase,
	P extends VormaLoaderPattern<App>,
> = GetParams<App, P>;

export type HasParams<App extends VormaAppBase, P extends string> =
	GetParams<App, P> extends never ? false : true;

export type IsSplat<App extends VormaAppBase, P extends string> =
	RouteMetadata<App, P> extends { isSplat: true } ? true : false;

export type IsEmptyInput<T> = [T] extends [null | undefined | never]
	? true
	: false;

// Pattern-based props composition
type ConditionalParams<App extends VormaAppBase, P extends string> =
	HasParams<App, P> extends true
		? { params: { [K in GetParams<App, P>]: string } }
		: {};

type ConditionalSplat<App extends VormaAppBase, P extends string> =
	IsSplat<App, P> extends true ? { splatValues: Array<string> } : {};

export type PatternBasedProps<App extends VormaAppBase, P extends string> = {
	pattern: P;
} & ConditionalParams<App, P> &
	ConditionalSplat<App, P>;

export type PermissivePatternBasedProps<
	App extends VormaAppBase,
	P extends VormaLoaderPattern<App>,
> = {
	pattern: PermissiveLoaderPattern<App, P>;
} & ConditionalParams<App, P> &
	ConditionalSplat<App, P>;

type PermissiveLoaderPattern<
	App extends VormaAppBase,
	P extends VormaLoaderPattern<App>,
> = P extends `${infer Prefix}/${App["appConfig"]["loadersExplicitIndexSegment"]}`
	? P | (Prefix extends "" ? "/" : Prefix)
	: P;

export type VormaRoutePropsGeneric<
	JSXElement,
	App extends VormaAppBase,
	P extends VormaLoaderPattern<App>,
> = {
	idx: number;
	Outlet: (props: Record<string, any>) => JSXElement;
	__phantom_pattern: P;
} & Record<string, any>;

/////////////////////////////////////////////////////////////////////
/////// API CLIENT HELPERS
/////////////////////////////////////////////////////////////////////

type Props = PatternBasedProps<any, string> & {
	options?: SubmitOptions;
	requestInit?: RequestInit;
	input?: any;
};

type APIClientHelperOpts = {
	vormaAppConfig: VormaAppConfig;
	type: "loader" | "query" | "mutation";
	props: Props;
};

export type VormaQueryProps<
	App extends VormaAppBase,
	P extends VormaQueryPattern<App>,
> = (PatternBasedProps<App, P> & {
	options?: SubmitOptions;
	requestInit?: Omit<RequestInit, "method"> & { method?: "GET" };
}) &
	(IsEmptyInput<VormaQueryInput<App, P>> extends true
		? { input?: VormaQueryInput<App, P> }
		: { input: VormaQueryInput<App, P> });

export type VormaMutationProps<
	App extends VormaAppBase,
	P extends VormaMutationPattern<App>,
> = PatternBasedProps<App, P> & {
	options?: SubmitOptions;
} & (VormaMutationMethod<App, P> extends "POST"
		? { requestInit?: Omit<RequestInit, "method"> & { method?: "POST" } }
		: {
				requestInit: RequestInit & {
					method: VormaMutationMethod<App, P>;
				};
			}) &
	(IsEmptyInput<VormaMutationInput<App, P>> extends true
		? { input?: VormaMutationInput<App, P> }
		: { input: VormaMutationInput<App, P> });

export function buildQueryURL(
	vormaAppConfig: VormaAppConfig,
	props: Props,
): URL {
	return buildVormaURL({ vormaAppConfig, props, type: "query" });
}

export function buildMutationURL(
	vormaAppConfig: VormaAppConfig,
	props: Props,
): URL {
	return buildVormaURL({ vormaAppConfig, props, type: "mutation" });
}

export function resolveBody(props: Props): BodyInit | null | undefined {
	return resolveVormaRequestBody(props.input);
}

export function __resolvePath(opts: APIClientHelperOpts): string {
	return resolveVormaPath(opts);
}
