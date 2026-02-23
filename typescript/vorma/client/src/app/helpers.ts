import { serializeToSearchParams } from "vorma/kit/json";
import { resolveRequestBodyForTransport } from "../platform/request_body.ts";
import type { SubmitOptions } from "../client.ts";

export type VormaAppConfig = {
	actionsRouterMountRoot: string;
	actionsDynamicRune: string;
	actionsSplatRune: string;
	loadersDynamicRune: string;
	loadersSplatRune: string;
	loadersExplicitIndexSegmentIdentifier: string;
	__phantom?: unknown;
};

type VormaRouteBase = {
	_type: string;
	pattern: string;
	params?: ReadonlyArray<string>;
	isSplat?: boolean;
	[key: string]: unknown;
};

export type VormaAppBase = {
	routes: readonly VormaRouteBase[];
	appConfig: VormaAppConfig;
	rootData: unknown;
};

export type ExtractApp<C extends VormaAppConfig> =
	C["__phantom"] extends VormaAppBase ? C["__phantom"] : VormaAppBase;

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

type QueryInputContractViolation = {
	__queryInputContractViolation: "Query input root must be an object, null, or undefined.";
};

type EnforceQueryInputRootContract<Input> = [Input] extends [
	null | undefined | never,
]
	? Input
	: Input extends Record<string, unknown>
		? Input
		: QueryInputContractViolation;

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
> = P extends `${infer Prefix}/${App["appConfig"]["loadersExplicitIndexSegmentIdentifier"]}`
	? P | (Prefix extends "" ? "/" : Prefix)
	: P;

export type VormaRoutePropsGeneric<
	JSXElement,
	App extends VormaAppBase,
	P extends VormaLoaderPattern<App>,
> = {
	idx: number;
	Outlet: (props: Record<string, unknown>) => JSXElement;
	__phantom_pattern: P;
} & Record<string, unknown>;

/////////////////////////////////////////////////////////////////////
/////// API CLIENT HELPERS
/////////////////////////////////////////////////////////////////////

type Props = {
	pattern: string;
	params?: Record<string, string>;
	splatValues?: Array<string>;
	options?: SubmitOptions;
	requestInit?: RequestInit;
	input?: unknown;
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
	(IsEmptyInput<
		EnforceQueryInputRootContract<VormaQueryInput<App, P>>
	> extends true
		? { input?: EnforceQueryInputRootContract<VormaQueryInput<App, P>> }
		: { input: EnforceQueryInputRootContract<VormaQueryInput<App, P>> });

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

type PathResolutionProps = {
	pattern: string;
	params?: Record<string, unknown>;
	splatValues?: Array<string>;
};

type PathResolutionConfig = {
	actionsDynamicRune: string;
	actionsSplatRune: string;
	loadersDynamicRune: string;
	loadersSplatRune: string;
	loadersExplicitIndexSegmentIdentifier: string;
};

type ResolvePathInput = {
	vormaAppConfig: PathResolutionConfig;
	type: "loader" | "query" | "mutation";
	props: PathResolutionProps;
};

function escapeRegex(value: string): string {
	return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

function replaceDynamicParam(props: {
	path: string;
	token: string;
	value: string;
}): string {
	const { path, token, value } = props;
	const tokenRegex = new RegExp(`${escapeRegex(token)}(?=/|$)`, "g");
	return path.replace(tokenRegex, encodeURIComponent(value));
}

function encodeSplatValues(splatValues: Array<string>): string {
	return splatValues.map((segment) => encodeURIComponent(segment)).join("/");
}

export function resolveVormaPath(input: ResolvePathInput): string {
	const { props, vormaAppConfig } = input;
	let path = props.pattern;

	let dynamicParamPrefixRune = vormaAppConfig.actionsDynamicRune;
	let splatSegmentRune = vormaAppConfig.actionsSplatRune;

	if (input.type === "loader") {
		dynamicParamPrefixRune = vormaAppConfig.loadersDynamicRune;
		splatSegmentRune = vormaAppConfig.loadersSplatRune;
	}

	if ("params" in props && props.params) {
		for (const [key, value] of Object.entries(props.params)) {
			path = replaceDynamicParam({
				path,
				token: `${dynamicParamPrefixRune}${key}`,
				value: String(value),
			});
		}
	}

	if ("splatValues" in props && props.splatValues) {
		const splatPath = encodeSplatValues(props.splatValues);
		path = path.replace(splatSegmentRune, splatPath);
	}

	// Strip explicit index segment
	if (
		input.type === "loader" &&
		vormaAppConfig.loadersExplicitIndexSegmentIdentifier
	) {
		const indexSegment = `/${vormaAppConfig.loadersExplicitIndexSegmentIdentifier}`;
		if (path.endsWith(indexSegment)) {
			path = path.slice(0, -indexSegment.length) || "/";
		}
	}

	return path;
}

type URLBuildProps = {
	pattern: string;
	params?: Record<string, unknown>;
	splatValues?: Array<string>;
	input?: unknown;
};

type URLBuildConfig = {
	actionsRouterMountRoot: string;
	actionsDynamicRune: string;
	actionsSplatRune: string;
	loadersDynamicRune: string;
	loadersSplatRune: string;
	loadersExplicitIndexSegmentIdentifier: string;
};

type URLBuildInput = {
	vormaAppConfig: URLBuildConfig;
	type: "loader" | "query" | "mutation";
	props: URLBuildProps;
};

function getCurrentOrigin(): string {
	return new URL(window.location.href).origin;
}

function stripTrailingSlash(path: string): string {
	return path.endsWith("/") ? path.slice(0, -1) : path;
}

function assertQueryInputRootIsObjectOrNil(
	input: unknown,
): asserts input is Record<string, unknown> | null | undefined {
	if (input === undefined || input === null) {
		return;
	}

	if (typeof input !== "object" || Array.isArray(input)) {
		throw new Error(
			"Query input root must be an object, null, or undefined.",
		);
	}
}

function buildVormaURL(input: URLBuildInput): URL {
	const basePath = stripTrailingSlash(
		input.vormaAppConfig.actionsRouterMountRoot,
	);
	const resolvedPath = resolveVormaPath(input);
	const url = new URL(basePath + resolvedPath, getCurrentOrigin());

	if (input.type === "query") {
		assertQueryInputRootIsObjectOrNil(input.props.input);
		if (input.props.input !== undefined && input.props.input !== null) {
			url.search = serializeToSearchParams(input.props.input).toString();
		}
	}

	return url;
}

export function resolveVormaRequestBody(
	input: unknown,
): BodyInit | null | undefined {
	return resolveRequestBodyForTransport({
		input,
	}).body;
}

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

export function resolvePath(opts: APIClientHelperOpts): string {
	return resolveVormaPath(opts);
}

export const __resolvePath = resolvePath;
