/////// PRIMITIVE TYPES

export type ResourceKind = "query" | "mutation";

export type ViewBase = {
	params?: ReadonlyArray<string>;
	parents?: ReadonlyArray<string>;
	pattern: string;
	__i?: unknown;
	__o?: unknown;
};

export type ResourceBase = {
	method: string;
	params?: ReadonlyArray<string>;
	pattern: string;
	kind?: ResourceKind;
	__i?: unknown;
	__o?: unknown;
};

/////// APP CONFIG

export type AppConfig = {
	apiMountRoot: string;
	__vorma_views: readonly ViewBase[];
	__vorma_resources: readonly ResourceBase[];
};

/////// APP TYPE EXTRACTORS

export type AppView<A extends AppConfig> = A["__vorma_views"][number];
export type AppResource<A extends AppConfig> = A["__vorma_resources"][number];

export type ViewByPattern<A extends AppConfig, P extends string> = Extract<
	AppView<A>,
	{ pattern: P }
>;

export type ResourceByMethodAndPattern<
	A extends AppConfig,
	M extends string,
	P extends string,
> = Extract<
	AppResource<A>,
	{
		method: M;
		pattern: P;
	}
>;

export type ResolvedResourceKind<Resource> = Resource extends {
	kind: infer T extends ResourceKind;
}
	? T
	: Resource extends {
				method: "GET" | "HEAD";
		  }
		? "query"
		: "mutation";

/////// SPLAT DETECTION

type IsSplat<P extends string> = P extends `${string}/*` ? true : false;

export type ConditionalSplat<P extends string> =
	IsSplat<P> extends true ? { splatValues: Array<string> } : {};

/////// PARAMS

export type ConditionalViewParams<A extends AppConfig, P extends string> =
	ViewByPattern<A, P> extends { params: ReadonlyArray<infer Params> }
		? Params extends string
			? { params: { [K in Params]: string } }
			: {}
		: {};

export type ConditionalResourceParams<Resource> = Resource extends {
	params: ReadonlyArray<infer Params>;
}
	? Params extends string
		? { params: { [K in Params]: string } }
		: {}
	: {};

export type ViewParamsRecord<A extends AppConfig, P extends string> =
	ViewByPattern<A, P> extends { params: ReadonlyArray<infer Params> }
		? Params extends string
			? { [K in Params]: string }
			: Record<string, string>
		: Record<string, string>;

/////// INPUT EMPTINESS

export type IsEmptyInput<T> = [T] extends [null | undefined] ? true : false;

type IsUnion<T, U = T> = [T] extends [never]
	? false
	: T extends unknown
		? [U] extends [T]
			? false
			: true
		: false;

export type ResourceInputField<Input> =
	IsEmptyInput<Input> extends true ? { input?: Input } : { input: Input };

/////// RESOURCE LOOKUPS

export type ResourceMethod<A extends AppConfig> = AppResource<A>["method"];

export type ResourcePattern<
	A extends AppConfig,
	M extends ResourceMethod<A> = ResourceMethod<A>,
> = Extract<AppResource<A>, { method: M }>["pattern"];

export type ResourceMethodsForPattern<A extends AppConfig, P extends string> = Extract<
	AppResource<A>,
	{ pattern: P }
>["method"];

export type ResourceMethodField<A extends AppConfig, P extends string, M extends string> =
	ResourceMethodsForPattern<A, P> extends "GET"
		? IsUnion<ResourceMethodsForPattern<A, P>> extends true
			? { method: M }
			: { method?: M }
		: { method: M };

export type ResourcesByKind<A extends AppConfig, T extends ResourceKind> =
	AppResource<A> extends infer Resource
		? Resource extends unknown
			? ResolvedResourceKind<Resource> extends T
				? Resource
				: never
			: never
		: never;

export type ResourceMethodByKind<A extends AppConfig, T extends ResourceKind> =
	ResourcesByKind<A, T> extends infer Resource
		? Resource extends { method: infer M extends string }
			? M
			: never
		: never;

export type ResourcePatternByKind<
	A extends AppConfig,
	T extends ResourceKind,
	M extends ResourceMethodByKind<A, T> = ResourceMethodByKind<A, T>,
> =
	Extract<ResourcesByKind<A, T>, { method: M }> extends infer Resource
		? Resource extends { pattern: infer P extends string }
			? P
			: never
		: never;

export type ResourceByKindMethodAndPattern<
	A extends AppConfig,
	T extends ResourceKind,
	M extends ResourceMethodByKind<A, T>,
	P extends ResourcePatternByKind<A, T, M>,
> = Extract<ResourcesByKind<A, T>, { method: M; pattern: P }>;
