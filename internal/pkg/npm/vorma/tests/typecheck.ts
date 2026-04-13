/// <reference types="vite/client" />

import type { Accessor } from "solid-js";
import type {
	AppConfig,
	LinkPropsBase,
	MakeTypedAPIClient,
	MakeTypedAPIDecorator,
	MakeTypedAPIDecoratorContext,
	MakeTypedClientLoaderProps,
	MakeTypedDefineRouteInput,
	MakeTypedLinkProps,
	MakeTypedLoaderOutput,
	MakeTypedLoaderPattern,
	MakeTypedMutationInput,
	MakeTypedMutationOutput,
	MakeTypedMutationPattern,
	MakeTypedMutationProps,
	MakeTypedNavigateProps,
	MakeTypedQueryInput,
	MakeTypedQueryOutput,
	MakeTypedQueryPattern,
	MakeTypedQueryProps,
	MakeTypedRouteProps,
	MakeTypedRouterData,
	RevalidationResult,
	RouteDefinition,
	RouteEntry,
	RouteState,
	ScrollState,
	StatusInfo,
	SubmitOptions,
	SubmitResult,
} from "vorma/__internal";
import { type Result } from "vorma/kit/result";
import { createVormaClient as Preact__createVormaClient } from "vorma/preact";
import { createVormaClient as React__createVormaClient } from "vorma/react";
import { createVormaClient as Solid__createVormaClient } from "vorma/solid";

type Assert<Condition extends true> = Condition;
type IsExact<Actual, Expected> =
	(<T>() => T extends Actual ? 1 : 2) extends <T>() => T extends Expected
		? 1
		: 2
		? (<T>() => T extends Expected ? 1 : 2) extends <
				T,
			>() => T extends Actual ? 1 : 2
			? true
			: false
		: false;

function expect_type<Expected>(_value: Expected): void {
	void _value;
}

/////// Test App Config

const vorma_app_config = {
	actionsMountRoot: "/api/",
	__phantom_loaders: [
		{
			pattern: "/",
			__O: null as unknown as { sessionUserID: string | null },
		},
		{
			pattern: "/users/:userID",
			params: ["userID"] as const,
			__O: null as unknown as { userName: string },
		},
		{
			pattern: "/docs/*",
			__O: null as unknown as { slugParts: string[] },
		},
		{
			pattern: "/blog/_index",
			__O: null as unknown as { posts: string[] },
		},
	] as const,
	__phantom_actions: [
		{
			method: "GET" as const,
			pattern: "/users/:userID",
			params: ["userID"] as const,
			__I: null as unknown as { includePosts: boolean },
			__O: null as unknown as { id: string; posts: number },
		},
		{
			method: "GET" as const,
			pattern: "/health",
			__I: null as unknown as null,
			__O: null as unknown as { ok: true },
		},
		{
			method: "PATCH" as const,
			pattern: "/users/:userID",
			params: ["userID"] as const,
			__I: null as unknown as { nickname: string },
			__O: null as unknown as { saved: true },
		},
		{
			method: "POST" as const,
			pattern: "/sessions",
			__I: null as unknown as { email: string; password: string },
			__O: null as unknown as { token: string },
		},
		{
			method: "POST" as const,
			pattern: "/logout",
			__I: null as unknown as undefined,
			__O: null as unknown as { done: true },
		},
	] as const,
} as const satisfies AppConfig;

type App = typeof vorma_app_config;

/////// React Adapter

const react = React__createVormaClient(vorma_app_config, {
	apiDecorator: async (context) => {
		if (context.type === "query") {
			expect_type<"/users/:userID" | "/health">(context.pattern);
			return { headers: [["x-query", "1"]] };
		}
		expect_type<"/users/:userID" | "/sessions" | "/logout">(
			context.pattern,
		);
		return { headers: [["x-mutation", "1"]] };
	},
});
const preact = Preact__createVormaClient(vorma_app_config);
const solid = Solid__createVormaClient(vorma_app_config);

/////// Exported Type Assertions

function assert_exported_type_contracts(): void {
	// Loader patterns
	type _loader_patterns = Assert<
		IsExact<
			MakeTypedLoaderPattern<App>,
			"/" | "/users/:userID" | "/docs/*" | "/blog/_index"
		>
	>;

	// Query patterns
	type _query_patterns = Assert<
		IsExact<MakeTypedQueryPattern<App>, "/users/:userID" | "/health">
	>;

	// Mutation patterns
	type _mutation_patterns = Assert<
		IsExact<
			MakeTypedMutationPattern<App>,
			"/users/:userID" | "/sessions" | "/logout"
		>
	>;

	// Loader output types
	type _loader_o_root = Assert<
		IsExact<
			MakeTypedLoaderOutput<App, "/">,
			{ sessionUserID: string | null }
		>
	>;
	type _loader_o_users = Assert<
		IsExact<
			MakeTypedLoaderOutput<App, "/users/:userID">,
			{ userName: string }
		>
	>;
	type _loader_o_docs = Assert<
		IsExact<MakeTypedLoaderOutput<App, "/docs/*">, { slugParts: string[] }>
	>;
	type _loader_o_blog = Assert<
		IsExact<MakeTypedLoaderOutput<App, "/blog/_index">, { posts: string[] }>
	>;

	// Query I/O types
	type _query_i_users = Assert<
		IsExact<
			MakeTypedQueryInput<App, "/users/:userID">,
			{ includePosts: boolean }
		>
	>;
	type _query_o_users = Assert<
		IsExact<
			MakeTypedQueryOutput<App, "/users/:userID">,
			{ id: string; posts: number }
		>
	>;
	type _query_i_health = Assert<
		IsExact<MakeTypedQueryInput<App, "/health">, null>
	>;
	type _query_o_health = Assert<
		IsExact<MakeTypedQueryOutput<App, "/health">, { ok: true }>
	>;

	// Mutation I/O types
	type _mutation_i_users = Assert<
		IsExact<
			MakeTypedMutationInput<App, "/users/:userID">,
			{ nickname: string }
		>
	>;
	type _mutation_o_users = Assert<
		IsExact<MakeTypedMutationOutput<App, "/users/:userID">, { saved: true }>
	>;
	type _mutation_i_sessions = Assert<
		IsExact<
			MakeTypedMutationInput<App, "/sessions">,
			{ email: string; password: string }
		>
	>;
	type _mutation_o_sessions = Assert<
		IsExact<MakeTypedMutationOutput<App, "/sessions">, { token: string }>
	>;
	type _mutation_i_logout = Assert<
		IsExact<MakeTypedMutationInput<App, "/logout">, undefined>
	>;
	type _mutation_o_logout = Assert<
		IsExact<MakeTypedMutationOutput<App, "/logout">, { done: true }>
	>;

	// Router data
	type _router_data_params_unscoped = Assert<
		IsExact<MakeTypedRouterData<App>["params"], Record<string, string>>
	>;
	type _router_data_params_scoped = Assert<
		IsExact<
			MakeTypedRouterData<App, "/users/:userID">["params"],
			{ userID: string }
		>
	>;
	type _router_data_root_data = Assert<
		IsExact<
			MakeTypedRouterData<App>["rootData"],
			{ sessionUserID: string | null }
		>
	>;
	type _router_data_client_build_id = Assert<
		IsExact<MakeTypedRouterData<App>["clientBuildID"], string>
	>;
	type _router_data_matched_patterns = Assert<
		IsExact<MakeTypedRouterData<App>["matchedPatterns"], string[]>
	>;
	type _router_data_splat_values = Assert<
		IsExact<MakeTypedRouterData<App>["splatValues"], string[]>
	>;

	// Decorator context
	type _decorator_ctx_query = Assert<
		IsExact<
			Extract<
				MakeTypedAPIDecoratorContext<App>,
				{ type: "query" }
			>["pattern"],
			"/users/:userID" | "/health"
		>
	>;
	type _decorator_ctx_mutation = Assert<
		IsExact<
			Extract<
				MakeTypedAPIDecoratorContext<App>,
				{ type: "mutation" }
			>["pattern"],
			"/users/:userID" | "/sessions" | "/logout"
		>
	>;

	// MakeTypedNavigateProps — intersection types are not IsExact-comparable
	// to flat object types. We verify bidirectional assignability for the
	// full type and use IsExact on individual fields. Call-site tests in
	// assert_navigate_contracts prove full correctness.
	type _nav_props_users = Assert<
		MakeTypedNavigateProps<App, "/users/:userID"> extends {
			pattern: "/users/:userID";
			params: { userID: string };
		}
			? {
					pattern: "/users/:userID";
					params: { userID: string };
				} extends MakeTypedNavigateProps<App, "/users/:userID">
				? true
				: false
			: false
	>;
	type _nav_props_docs = Assert<
		MakeTypedNavigateProps<App, "/docs/*"> extends {
			pattern: "/docs/*";
			splatValues: string[];
		}
			? {
					pattern: "/docs/*";
					splatValues: string[];
				} extends MakeTypedNavigateProps<App, "/docs/*">
				? true
				: false
			: false
	>;
	type _nav_props_index_shorthand = Assert<
		IsExact<
			MakeTypedNavigateProps<App, "/blog/_index">["pattern"],
			"/blog/_index" | "/blog"
		>
	>;
	type _nav_props_root = Assert<
		IsExact<MakeTypedNavigateProps<App, "/">["pattern"], "/">
	>;

	// MakeTypedLinkProps — same intersection caveat as NavigateProps.
	type _link_props_pattern = Assert<
		IsExact<
			MakeTypedLinkProps<App, "/users/:userID">["pattern"],
			"/users/:userID"
		>
	>;
	type _link_props_params = Assert<
		MakeTypedLinkProps<App, "/users/:userID"> extends {
			params: { userID: string };
		}
			? true
			: false
	>;
	type _link_props_splat = Assert<
		MakeTypedLinkProps<App, "/docs/*"> extends { splatValues: string[] }
			? true
			: false
	>;
	type _link_props_search = Assert<
		IsExact<
			MakeTypedLinkProps<App, "/users/:userID">["search"],
			string | undefined
		>
	>;
	type _link_props_hash = Assert<
		IsExact<
			MakeTypedLinkProps<App, "/users/:userID">["hash"],
			string | undefined
		>
	>;
	type _link_props_index_shorthand = Assert<
		IsExact<
			MakeTypedLinkProps<App, "/blog/_index">["pattern"],
			"/blog/_index" | "/blog"
		>
	>;

	// MakeTypedClientLoaderProps
	type _cl_props_params = Assert<
		IsExact<
			MakeTypedClientLoaderProps<App, "/users/:userID">["params"],
			{ userID: string }
		>
	>;
	type _cl_props_splat_values = Assert<
		IsExact<
			MakeTypedClientLoaderProps<App, "/users/:userID">["splatValues"],
			string[]
		>
	>;
	type _cl_props_signal = Assert<
		IsExact<
			MakeTypedClientLoaderProps<App, "/users/:userID">["signal"],
			AbortSignal
		>
	>;
	type _cl_props_loader_data = Assert<
		IsExact<
			Awaited<
				MakeTypedClientLoaderProps<
					App,
					"/users/:userID"
				>["serverDataPromise"]
			>["loaderData"],
			{ userName: string }
		>
	>;
	type _cl_props_root_data = Assert<
		IsExact<
			Awaited<
				MakeTypedClientLoaderProps<
					App,
					"/users/:userID"
				>["serverDataPromise"]
			>["rootData"],
			{ sessionUserID: string | null }
		>
	>;
	type _cl_props_root_params = Assert<
		IsExact<
			MakeTypedClientLoaderProps<App, "/">["params"],
			Record<string, string>
		>
	>;

	// MakeTypedDefineRouteInput
	type _define_route_pattern = Assert<
		IsExact<MakeTypedDefineRouteInput<App, "/">["pattern"], "/">
	>;
	type _define_route_error_boundary_optional =
		undefined extends MakeTypedDefineRouteInput<App, "/">["errorBoundary"]
			? true
			: false;
	type _define_route_client_loader_optional =
		undefined extends MakeTypedDefineRouteInput<App, "/">["clientLoader"]
			? true
			: false;
	type _define_route_hmr_optional =
		undefined extends MakeTypedDefineRouteInput<
			App,
			"/"
		>["runClientLoaderOnHMR"]
			? true
			: false;

	// MakeTypedAPIDecorator
	type _decorator_fn = Assert<
		IsExact<
			MakeTypedAPIDecorator<App>,
			(
				context: MakeTypedAPIDecoratorContext<App>,
			) =>
				| Omit<RequestInit, "method" | "body">
				| undefined
				| Promise<Omit<RequestInit, "method" | "body"> | undefined>
		>
	>;

	// MakeTypedAPIClient
	type _api_client_keys = Assert<
		IsExact<keyof MakeTypedAPIClient<App>, "query" | "mutate">
	>;
	type _api_client_query_return = Assert<
		IsExact<
			Awaited<ReturnType<MakeTypedAPIClient<App>["query"]>>,
			SubmitResult<
				| MakeTypedQueryOutput<App, "/users/:userID">
				| MakeTypedQueryOutput<App, "/health">
			>
		>
	>;

	// ScrollState
	const scroll_xy: ScrollState = { x: 0, y: 0 };
	const scroll_hash: ScrollState = { hash: "#top" };
	void scroll_xy;
	void scroll_hash;

	// StatusInfo
	const status_info: StatusInfo = {
		isNavigating: false,
		isSubmitting: false,
		isRevalidating: false,
	};
	void status_info;

	// SubmitResult
	const success_result: SubmitResult<number> = {
		success: true,
		data: 42,
		revalidationPromise: Promise.resolve({ ok: true }),
	};
	const fail_result: SubmitResult<number> = {
		success: false,
		error: "fail",
		revalidationPromise: Promise.resolve({
			ok: false,
			reason: "max_retries_exhausted",
		}),
	};
	void success_result;
	void fail_result;

	// SubmitOptions
	const submit_opts: SubmitOptions = {
		dedupeKey: "k",
		revalidate: true,
		skipGlobalLoadingIndicator: false,
	};
	void submit_opts;

	// LinkPropsBase
	const link_props: LinkPropsBase = {
		href: "/",
		prefetch: "intent",
		visitOnPointerDown: true,
		prefetchDelayMs: 100,
		replace: false,
		scrollToTop: true,
	};
	void link_props;

	// RouteEntry
	const entry: RouteEntry = {
		pattern: "/",
		module_url: "/mod.js",
		module: {},
		data: null,
		client_data: undefined,
		error: undefined,
	};
	expect_type<string>(entry.pattern);
	expect_type<string>(entry.module_url);
	expect_type<Record<string, unknown>>(entry.module);
	expect_type<unknown>(entry.data);
	expect_type<unknown>(entry.client_data);
	expect_type<unknown>(entry.error);

	// RouteState
	const route_state: RouteState = {
		entries: [entry],
		params: {},
		splat_values: [],
		client_build_id: "1",
		history_state: undefined,
	};
	expect_type<RouteEntry[]>(route_state.entries);
	expect_type<Record<string, string>>(route_state.params);
	expect_type<string[]>(route_state.splat_values);
	expect_type<string>(route_state.client_build_id);

	// RouteDefinition
	const route_def: RouteDefinition = {
		pattern: "/",
		component: () => {
			return null!;
		},
	};
	expect_type<string>(route_def.pattern);
	expect_type<(props: any) => any>(route_def.component);
}
void assert_exported_type_contracts;

/////// Navigate Type Safety

function assert_navigate_contracts(): void {
	// Valid: dynamic route with params
	void react.navigate({
		pattern: "/users/:userID",
		params: { userID: "u-1" },
	});

	// Valid: splat route with splatValues and hash
	void react.navigate({
		pattern: "/docs/*",
		splatValues: ["guide", "install"],
		hash: "#top",
	});

	// Valid: _index shorthand (blog/_index -> blog)
	void react.navigate({
		pattern: "/blog",
	});

	// Valid: root route
	void react.navigate({
		pattern: "/",
	});

	// Valid: with replace and scrollToTop
	void react.navigate({
		pattern: "/users/:userID",
		params: { userID: "u-1" },
		replace: true,
		scrollToTop: false,
	});

	// Valid: with search and hash
	void react.navigate({
		pattern: "/users/:userID",
		params: { userID: "u-1" },
		search: "?tab=posts",
		hash: "#recent",
	});

	// @ts-expect-error route params are required for /users/:userID.
	void react.navigate({ pattern: "/users/:userID" });
	void react.navigate({
		pattern: "/users/:userID",
		// @ts-expect-error params key must match route parameter names.
		params: { slug: "u-1" },
	});
	void react.navigate({
		pattern: "/users/:userID",
		// @ts-expect-error route params must be string values.
		params: { userID: 123 },
	});
	// @ts-expect-error splatValues are required for splat loader routes.
	void react.navigate({ pattern: "/docs/*" });
	void react.navigate({
		pattern: "/docs/*",
		// @ts-expect-error splatValues must be an array of strings.
		splatValues: "guide",
	});
	// @ts-expect-error unknown route patterns are rejected.
	void react.navigate({ pattern: "/does-not-exist" });
}
void assert_navigate_contracts;

/////// API Client Type Safety

function assert_api_client_contracts(): void {
	// Valid query with required params and input
	const user_query_result = react.apiClient.query({
		pattern: "/users/:userID",
		params: { userID: "u-1" },
		input: { includePosts: true },
	});
	expect_type<
		Promise<SubmitResult<MakeTypedQueryOutput<App, "/users/:userID">>>
	>(user_query_result);

	// Valid query with nullable input (omitted)
	void react.apiClient.query({
		pattern: "/health",
	});

	// Valid query with nullable input (explicit null)
	void react.apiClient.query({
		pattern: "/health",
		input: null,
	});

	// Valid query with options
	void react.apiClient.query({
		pattern: "/health",
		options: { dedupeKey: "health-check", revalidate: false },
	});

	// @ts-expect-error object query inputs are required when input type is non-empty.
	void react.apiClient.query({
		pattern: "/users/:userID",
		params: { userID: "u-1" },
	});
	void react.apiClient.query({
		pattern: "/users/:userID",
		params: { userID: "u-1" },
		input: { includePosts: true },
		// @ts-expect-error query requestInit method must remain GET.
		requestInit: { method: "POST" },
	});
	void react.apiClient.query({
		pattern: "/users/:userID",
		// @ts-expect-error query params must match route parameter names.
		params: { id: "u-1" },
		input: { includePosts: true },
	});
	void react.apiClient.query({
		pattern: "/health",
		// @ts-expect-error query input root must be object, null, or undefined.
		input: "invalid",
	});
	// @ts-expect-error query route must come from query patterns.
	void react.apiClient.query<"/docs/*">({
		pattern: "/docs/*",
		splatValues: ["x"],
		input: {},
	});

	// Valid PATCH mutation with required params and input
	const patch_result = react.apiClient.mutate({
		pattern: "/users/:userID",
		params: { userID: "u-1" },
		input: { nickname: "neo" },
		requestInit: { method: "PATCH" },
	});
	expect_type<
		Promise<SubmitResult<MakeTypedMutationOutput<App, "/users/:userID">>>
	>(patch_result);

	// Valid POST mutation
	const post_result = react.apiClient.mutate({
		pattern: "/sessions",
		input: { email: "a@b.com", password: "pw" },
		requestInit: { method: "POST" },
	});
	expect_type<
		Promise<SubmitResult<MakeTypedMutationOutput<App, "/sessions">>>
	>(post_result);

	// Valid mutation with optional input (omitted)
	void react.apiClient.mutate({
		pattern: "/logout",
		requestInit: { method: "POST" },
	});

	// @ts-expect-error requestInit is always required for mutations.
	void react.apiClient.mutate({
		pattern: "/logout",
	});
	void react.apiClient.mutate({
		pattern: "/users/:userID",
		params: { userID: "u-1" },
		input: { nickname: "neo" },
		// @ts-expect-error requestInit method must match mutation method.
		requestInit: { method: "POST" },
	});
	// @ts-expect-error non-empty mutation input is required.
	void react.apiClient.mutate({
		pattern: "/sessions",
		requestInit: { method: "POST" },
	});
	void react.apiClient.mutate({
		pattern: "/sessions",
		input: { email: "a@b.com", password: "pw" },
		// @ts-expect-error POST mutation requestInit.method cannot be changed.
		requestInit: { method: "PUT" },
	});
	// @ts-expect-error mutation route must come from mutation patterns.
	void react.apiClient.mutate<"/health">({
		pattern: "/health",
		input: {},
		requestInit: { method: "POST" },
	});
}
void assert_api_client_contracts;

/////// Query and Mutation Props Type Safety

function assert_query_and_mutation_props_contracts(): void {
	// Required query input
	const required_query_props: MakeTypedQueryProps<App, "/users/:userID"> = {
		pattern: "/users/:userID",
		params: { userID: "u-1" },
		input: { includePosts: true },
	};
	expect_type<{ includePosts: boolean }>(required_query_props.input);

	// Optional query input (omitted, null, undefined)
	const optional_query_omitted: MakeTypedQueryProps<App, "/health"> = {
		pattern: "/health",
	};
	const optional_query_null: MakeTypedQueryProps<App, "/health"> = {
		pattern: "/health",
		input: null,
	};
	const optional_query_undefined: MakeTypedQueryProps<App, "/health"> = {
		pattern: "/health",
		input: undefined,
	};
	void optional_query_omitted;
	void optional_query_null;
	void optional_query_undefined;

	// @ts-expect-error non-empty query input must be required.
	const missing_query_input: MakeTypedQueryProps<App, "/users/:userID"> = {
		pattern: "/users/:userID",
		params: { userID: "u-1" },
	};
	void missing_query_input;

	const wrong_query_method: MakeTypedQueryProps<App, "/users/:userID"> = {
		pattern: "/users/:userID",
		params: { userID: "u-1" },
		input: { includePosts: true },
		requestInit: {
			// @ts-expect-error query requestInit.method must remain GET.
			method: "POST",
		},
	};
	void wrong_query_method;

	const wrong_query_params: MakeTypedQueryProps<App, "/users/:userID"> = {
		pattern: "/users/:userID",
		// @ts-expect-error query params keys must match route params.
		params: { id: "u-1" },
		input: { includePosts: true },
	};
	void wrong_query_params;

	// Required PATCH mutation with explicit method
	const required_patch: MakeTypedMutationProps<App, "/users/:userID"> = {
		pattern: "/users/:userID",
		params: { userID: "u-1" },
		input: { nickname: "neo" },
		requestInit: { method: "PATCH" },
	};
	expect_type<{ nickname: string }>(required_patch.input);

	// Required POST mutation
	const required_post: MakeTypedMutationProps<App, "/sessions"> = {
		pattern: "/sessions",
		input: { email: "a@b.com", password: "pw" },
		requestInit: { method: "POST" },
	};
	void required_post;

	// Optional mutation input (omitted, undefined)
	const optional_mutation_omitted: MakeTypedMutationProps<App, "/logout"> = {
		pattern: "/logout",
		requestInit: { method: "POST" },
	};
	const optional_mutation_undefined: MakeTypedMutationProps<App, "/logout"> =
		{
			pattern: "/logout",
			input: undefined,
			requestInit: { method: "POST" },
		};
	void optional_mutation_omitted;
	void optional_mutation_undefined;

	// @ts-expect-error non-empty mutation input must be required.
	const missing_mutation_input: MakeTypedMutationProps<App, "/sessions"> = {
		pattern: "/sessions",
		requestInit: { method: "POST" },
	};
	void missing_mutation_input;

	// @ts-expect-error requestInit is always required for mutations.
	const missing_request_init: MakeTypedMutationProps<App, "/users/:userID"> =
		{
			pattern: "/users/:userID",
			params: { userID: "u-1" },
			input: { nickname: "neo" },
		};
	void missing_request_init;

	const wrong_patch_method: MakeTypedMutationProps<App, "/users/:userID"> = {
		pattern: "/users/:userID",
		params: { userID: "u-1" },
		input: { nickname: "neo" },
		requestInit: {
			// @ts-expect-error non-POST mutation method must match route declaration.
			method: "POST",
		},
	};
	void wrong_patch_method;

	const wrong_post_method: MakeTypedMutationProps<App, "/sessions"> = {
		pattern: "/sessions",
		input: { email: "a@b.com", password: "pw" },
		requestInit: {
			// @ts-expect-error POST mutation requestInit.method cannot be changed.
			method: "PUT",
		},
	};
	void wrong_post_method;

	const wrong_mutation_params: MakeTypedMutationProps<App, "/users/:userID"> =
		{
			pattern: "/users/:userID",
			// @ts-expect-error mutation params keys must match route params.
			params: { id: "u-1" },
			input: { nickname: "neo" },
			requestInit: { method: "PATCH" },
		};
	void wrong_mutation_params;
}
void assert_query_and_mutation_props_contracts;

/////// React Adapter Type Safety

function assert_react_adapter_contracts(): void {
	const route_props = null as unknown as MakeTypedRouteProps<
		App,
		"/users/:userID"
	>;

	// useLoaderData
	const loader_data = react.useLoaderData(route_props);
	expect_type<MakeTypedLoaderOutput<App, "/users/:userID">>(loader_data);

	// usePatternLoaderData
	const maybe_pattern_data = react.usePatternLoaderData("/docs/*");
	expect_type<MakeTypedLoaderOutput<App, "/docs/*"> | undefined>(
		maybe_pattern_data,
	);

	const maybe_root_data = react.usePatternLoaderData("/");
	expect_type<MakeTypedLoaderOutput<App, "/"> | undefined>(maybe_root_data);

	const maybe_blog_data = react.usePatternLoaderData("/blog/_index");
	expect_type<MakeTypedLoaderOutput<App, "/blog/_index"> | undefined>(
		maybe_blog_data,
	);

	void react.usePatternLoaderData(
		// @ts-expect-error typed pattern loader data rejects unknown patterns.
		"/not-a-route",
	);

	// useRouterData (unscoped)
	const unscoped_router_data = react.useRouterData();
	expect_type<string>(unscoped_router_data.clientBuildID);
	expect_type<string[]>(unscoped_router_data.matchedPatterns);
	expect_type<string[]>(unscoped_router_data.splatValues);
	expect_type<Record<string, string>>(unscoped_router_data.params);
	expect_type<unknown>(unscoped_router_data.historyState);
	expect_type<{ sessionUserID: string | null }>(
		unscoped_router_data.rootData,
	);

	// useRouterData (scoped)
	const scoped_router_data = react.useRouterData(route_props);
	expect_type<{ userID: string }>(scoped_router_data.params);
	expect_type<{ sessionUserID: string | null }>(scoped_router_data.rootData);

	// getRouterData (unscoped)
	const unscoped_get_router_data = react.getRouterData();
	expect_type<Record<string, string>>(unscoped_get_router_data.params);
	expect_type<unknown>(unscoped_get_router_data.historyState);
	expect_type<{ sessionUserID: string | null }>(
		unscoped_get_router_data.rootData,
	);

	// getRouterData (scoped)
	const scoped_get_router_data = react.getRouterData(route_props);
	expect_type<{ userID: string }>(scoped_get_router_data.params);

	// defineRoute: basic
	void react.defineRoute({
		pattern: "/",
		component: () => {
			return null!;
		},
	});

	// defineRoute: with errorBoundary
	void react.defineRoute({
		pattern: "/docs/*",
		component: (props) => {
			const data = react.useLoaderData(props);
			expect_type<MakeTypedLoaderOutput<App, "/docs/*">>(data);
			return null!;
		},
		errorBoundary: (props) => {
			expect_type<unknown>(props.error);
			return null!;
		},
	});

	// defineRoute: with component, clientLoader, and useClientLoaderData
	void react.defineRoute({
		pattern: "/users/:userID",
		component: (props) => {
			const data = react.useLoaderData(props);
			expect_type<MakeTypedLoaderOutput<App, "/users/:userID">>(data);

			const client_data = react.useClientLoaderData(props);
			expect_type<number>(client_data);

			return props.Outlet();
		},
		clientLoader: async ({
			params,
			splatValues,
			serverDataPromise,
			signal,
		}) => {
			expect_type<{ userID: string }>(params);
			expect_type<string>(params.userID);
			// @ts-expect-error only declared param keys are accessible.
			void params.bogus;
			expect_type<string[]>(splatValues);
			expect_type<AbortSignal>(signal);
			const server_data = await serverDataPromise;
			expect_type<MakeTypedLoaderOutput<App, "/users/:userID">>(
				server_data.loaderData,
			);
			expect_type<{ sessionUserID: string | null }>(server_data.rootData);
			expect_type<string[]>(server_data.matchedPatterns);
			expect_type<string>(server_data.clientBuildID);
			return server_data.loaderData.userName.length;
		},
		runClientLoaderOnHMR: true,
	});

	// useClientLoaderData: tested with an explicit T via RouteProps
	const cl_route_props = null as unknown as MakeTypedRouteProps<
		App,
		"/users/:userID",
		number
	>;
	const client_data = react.useClientLoaderData(cl_route_props);
	expect_type<number>(client_data);

	// usePatternClientLoaderData
	const maybe_client_data =
		react.usePatternClientLoaderData<number>("/users/:userID");
	expect_type<number | undefined>(maybe_client_data);

	// defineRoute: invalid pattern
	void react.defineRoute({
		// @ts-expect-error defineRoute pattern must exist in loader route patterns.
		pattern: "/not-a-route",
		component: () => {
			return null!;
		},
	});

	// defineRoute: invalid runClientLoaderOnHMR
	void react.defineRoute({
		pattern: "/",
		component: () => {
			return null!;
		},
		// @ts-expect-error runClientLoaderOnHMR must be boolean.
		runClientLoaderOnHMR: "1",
	});

	// Link: valid cases
	void react.Link({
		pattern: "/users/:userID",
		params: { userID: "u-1" },
	});
	void react.Link({
		pattern: "/docs/*",
		splatValues: ["guide"],
	});
	void react.Link({
		pattern: "/blog",
	});
	void react.Link({
		pattern: "/users/:userID",
		params: { userID: "u-1" },
		prefetch: "intent",
		visitOnPointerDown: true,
		prefetchDelayMs: 100,
		replace: true,
		scrollToTop: false,
	});
	void react.Link({
		pattern: "/users/:userID",
		params: { userID: "u-1" },
		search: "?tab=posts",
		hash: "#recent",
	});
	void react.Link({
		pattern: "/",
		prefetch: "none",
	});

	// Link: invalid cases
	// @ts-expect-error typed links require params for dynamic routes.
	void react.Link({
		pattern: "/users/:userID",
	});
	void react.Link({
		pattern: "/users/:userID",
		// @ts-expect-error typed links enforce exact dynamic param keys.
		params: { id: "u-1" },
	});
	void react.Link({
		pattern: "/users/:userID",
		// @ts-expect-error typed links require string param values.
		params: { userID: 123 },
	});
	// @ts-expect-error typed links require splatValues for splat routes.
	void react.Link({
		pattern: "/docs/*",
	});
	void react.Link({
		pattern: "/docs/*",
		// @ts-expect-error typed links require splatValues to be string[].
		splatValues: "guide",
	});
	void react.Link({
		// @ts-expect-error typed links reject unknown route patterns.
		pattern: "/not-a-route",
	});
	void react.Link({
		pattern: "/",
		// @ts-expect-error prefetch only accepts intent or none.
		prefetch: "hover",
	});

	// RootOutlet
	void react.RootOutlet({ idx: 0 });
}
void assert_react_adapter_contracts;

/////// Public Runtime Contracts

function assert_public_runtime_contracts(): void {
	// init
	const init_promise = react.init({
		renderFn: async () => {},
		useViewTransitions: true,
		onStatusChange: (status) => {
			expect_type<boolean>(status.isNavigating);
			expect_type<boolean>(status.isSubmitting);
			expect_type<boolean>(status.isRevalidating);
		},
		onRouteChange: () => {},
		onClientBuildIDChange: (prev, next) => {
			expect_type<string>(prev);
			expect_type<string>(next);
		},
	});
	expect_type<Promise<Result<void>>>(init_promise);

	// navigate
	const navigate_result = react.navigate({
		pattern: "/users/:userID",
		params: { userID: "u-1" },
		replace: true,
		scrollToTop: false,
	});
	expect_type<Promise<{ didNavigate: boolean }>>(navigate_result);

	// revalidate
	const revalidate_result = react.revalidate();
	expect_type<Promise<RevalidationResult>>(revalidate_result);

	// submit (untyped)
	const submit_result = react.submit("/api/users", { method: "POST" });
	expect_type<
		Promise<
			| {
					success: true;
					data: unknown;
					response: Response;
					revalidationPromise: Promise<RevalidationResult>;
			  }
			| {
					success: false;
					error: string;
					response?: Response;
					revalidationPromise: Promise<RevalidationResult>;
			  }
		>
	>(submit_result);

	// submit (typed)
	const typed_submit_result = react.submit<{ ok: true }>("/api/users", {
		method: "POST",
	});
	expect_type<
		Promise<
			| {
					success: true;
					data: { ok: true };
					response: Response;
					revalidationPromise: Promise<RevalidationResult>;
			  }
			| {
					success: false;
					error: string;
					response?: Response;
					revalidationPromise: Promise<RevalidationResult>;
			  }
		>
	>(typed_submit_result);

	// submit with options
	void react.submit(
		"/api/users",
		{ method: "POST" },
		{
			dedupeKey: "create-user",
			revalidate: true,
			skipGlobalLoadingIndicator: false,
		},
	);

	// getStatus
	const status = react.getStatus();
	expect_type<boolean>(status.isNavigating);
	expect_type<boolean>(status.isSubmitting);
	expect_type<boolean>(status.isRevalidating);

	// getClientBuildID
	expect_type<string>(react.getClientBuildID());

	// getRootEl
	expect_type<HTMLElement>(react.getRootEl());

	// revalidateOnWindowFocus
	const stop_focus_revalidate = react.revalidateOnWindowFocus({
		staleTimeMS: 3000,
	});
	expect_type<() => void>(stop_focus_revalidate);

	// revalidateOnWindowFocus (no options)
	const stop_focus_revalidate_default = react.revalidateOnWindowFocus();
	expect_type<() => void>(stop_focus_revalidate_default);

	// setupGlobalLoadingIndicator (all categories)
	const stop_global_loading = react.setupGlobalLoadingIndicator({
		start: () => {},
		stop: () => {},
		isRunning: () => {
			return false;
		},
		include: ["navigations", "submissions", "revalidations"],
		startDelayMS: 30,
		stopDelayMS: 40,
	});
	expect_type<() => void>(stop_global_loading);

	// setupGlobalLoadingIndicator (include "all")
	void react.setupGlobalLoadingIndicator({
		start: () => {},
		stop: () => {},
		isRunning: () => {
			return false;
		},
		include: "all",
	});

	// setupGlobalLoadingIndicator (subset)
	void react.setupGlobalLoadingIndicator({
		start: () => {},
		stop: () => {},
		isRunning: () => {
			return false;
		},
		include: ["navigations"],
	});
}
void assert_public_runtime_contracts;

/////// Preact Adapter Type Safety

function assert_preact_adapter_contracts(): void {
	const route_props = null as unknown as MakeTypedRouteProps<
		App,
		"/users/:userID"
	>;

	// useLoaderData returns T (same as React)
	const loader_data = preact.useLoaderData(route_props);
	expect_type<MakeTypedLoaderOutput<App, "/users/:userID">>(loader_data);

	// usePatternLoaderData
	const maybe_pattern_data = preact.usePatternLoaderData("/docs/*");
	expect_type<MakeTypedLoaderOutput<App, "/docs/*"> | undefined>(
		maybe_pattern_data,
	);

	// useRouterData (unscoped)
	const unscoped_router_data = preact.useRouterData();
	expect_type<Record<string, string>>(unscoped_router_data.params);
	expect_type<unknown>(unscoped_router_data.historyState);
	expect_type<{ sessionUserID: string | null }>(
		unscoped_router_data.rootData,
	);

	// useRouterData (scoped)
	const scoped_router_data = preact.useRouterData(route_props);
	expect_type<{ userID: string }>(scoped_router_data.params);

	// useClientLoaderData
	const cl_route_props = null as unknown as MakeTypedRouteProps<
		App,
		"/users/:userID",
		number
	>;
	const client_data = preact.useClientLoaderData(cl_route_props);
	expect_type<number>(client_data);

	// usePatternClientLoaderData
	const maybe_client_data =
		preact.usePatternClientLoaderData<number>("/users/:userID");
	expect_type<number | undefined>(maybe_client_data);

	// defineRoute
	void preact.defineRoute({
		pattern: "/users/:userID",
		component: (props) => {
			const data = preact.useLoaderData(props);
			expect_type<MakeTypedLoaderOutput<App, "/users/:userID">>(data);
			return null!;
		},
	});

	void preact.defineRoute({
		// @ts-expect-error defineRoute rejects unknown patterns.
		pattern: "/not-a-route",
		component: () => {
			return null!;
		},
	});

	// Link
	void preact.Link({
		pattern: "/users/:userID",
		params: { userID: "u-1" },
	});

	// @ts-expect-error typed links require params for dynamic routes.
	void preact.Link({
		pattern: "/users/:userID",
	});
}
void assert_preact_adapter_contracts;

/////// Solid Adapter Type Safety

function assert_solid_adapter_contracts(): void {
	const route_props = null as unknown as MakeTypedRouteProps<
		App,
		"/users/:userID"
	>;

	// useLoaderData returns Accessor<T>
	const loader_data = solid.useLoaderData(route_props);
	expect_type<Accessor<MakeTypedLoaderOutput<App, "/users/:userID">>>(
		loader_data,
	);
	// Calling the accessor returns the value
	expect_type<MakeTypedLoaderOutput<App, "/users/:userID">>(loader_data());

	// usePatternLoaderData returns Accessor<T | undefined>
	const maybe_pattern_data = solid.usePatternLoaderData("/docs/*");
	expect_type<Accessor<MakeTypedLoaderOutput<App, "/docs/*"> | undefined>>(
		maybe_pattern_data,
	);

	// useRouterData returns Accessor (unscoped)
	const unscoped_router_data = solid.useRouterData();
	expect_type<Accessor<MakeTypedRouterData<App>>>(unscoped_router_data);
	expect_type<Record<string, string>>(unscoped_router_data().params);
	expect_type<unknown>(unscoped_router_data().historyState);
	expect_type<{ sessionUserID: string | null }>(
		unscoped_router_data().rootData,
	);

	// useRouterData returns Accessor (scoped)
	const scoped_router_data = solid.useRouterData(route_props);
	expect_type<Accessor<MakeTypedRouterData<App, "/users/:userID">>>(
		scoped_router_data,
	);
	expect_type<{ userID: string }>(scoped_router_data().params);

	// useClientLoaderData returns Accessor<T>
	const cl_route_props = null as unknown as MakeTypedRouteProps<
		App,
		"/users/:userID",
		number
	>;
	const client_data = solid.useClientLoaderData(cl_route_props);
	expect_type<Accessor<number>>(client_data);
	expect_type<number>(client_data());

	// usePatternClientLoaderData returns Accessor<T | undefined>
	const maybe_client_data =
		solid.usePatternClientLoaderData<number>("/users/:userID");
	expect_type<Accessor<number | undefined>>(maybe_client_data);

	// defineRoute
	void solid.defineRoute({
		pattern: "/users/:userID",
		component: (props) => {
			const data = solid.useLoaderData(props);
			expect_type<Accessor<MakeTypedLoaderOutput<App, "/users/:userID">>>(
				data,
			);
			return null!;
		},
	});

	void solid.defineRoute({
		// @ts-expect-error defineRoute rejects unknown patterns.
		pattern: "/not-a-route",
		component: () => {
			return null!;
		},
	});

	// Link
	void solid.Link({
		pattern: "/users/:userID",
		params: { userID: "u-1" },
	});

	// @ts-expect-error typed links require params for dynamic routes.
	void solid.Link({
		pattern: "/users/:userID",
	});
}
void assert_solid_adapter_contracts;
