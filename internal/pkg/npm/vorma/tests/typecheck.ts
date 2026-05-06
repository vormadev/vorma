/// <reference types="vite/client" />

import type { ReadonlySignal } from "@preact/signals";
import type { ComponentType as PreactComponentType } from "preact";
import type { ComponentType as ReactComponentType } from "react";
import type { Handle as RemixHandle, RemixNode } from "remix/ui";
import type { Accessor, Component as SolidComponent } from "solid-js";
import type {
	AppConfig,
	BuildSkewDetectedEvent,
	MutationResult,
	ProgressIndicatorConfig,
	QueryResult,
	RevalidationReason,
	RevalidationResult,
	RouteErrorState,
	RouteRenderEntry,
	RouteRenderState,
	RouteState,
	RouteUpdateReason,
	ScrollState,
	ToAPIClient,
	ToAPIDecorator,
	ToAPIDecoratorContext,
	ToClientLoaderArgs,
	ToDefineViewArgs,
	ToLinkProps,
	ToLoaderInput,
	ToLoaderOutput,
	ToMutationArgs,
	ToMutationError,
	ToMutationInput,
	ToMutationMethod,
	ToMutationOutput,
	ToMutationPattern,
	ToNavigateArgs,
	ToNavigationTarget,
	ToQueryArgs,
	ToQueryError,
	ToQueryInput,
	ToQueryMethod,
	ToQueryOutput,
	ToQueryPattern,
	ToRouteComponentProps,
	ToRouteDestination,
	ToRouteSyncArgs,
	ToViewPattern,
	ViewDefinition,
	WorkState,
} from "vorma/__internal";
import { MutationError, QueryError } from "vorma/__internal";
import { type Result } from "vorma/kit/result";
import { createVormaClient as Preact__createVormaClient } from "vorma/preact";
import { createVormaClient as React__createVormaClient } from "vorma/react";
import {
	createVormaClient as Remix__createVormaClient,
	type RemixComponent,
} from "vorma/remix";
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
	apiMountRoot: "/api/",
	__vormaViews: [
		{
			pattern: "/",
			__I: null as unknown as Record<never, never>,
			__O: null as unknown as { sessionUserID: string | null },
		},
		{
			parents: ["/"],
			pattern: "/users",
			__I: null as unknown as { sort?: "name" | "created" },
			__O: null as unknown as { userCount: number },
		},
		{
			parents: ["/", "/users"],
			pattern: "/users/:userID",
			params: ["userID"] as const,
			__I: null as unknown as { tab?: string; page?: number },
			__O: null as unknown as { userName: string },
		},
		{
			parents: ["/"],
			pattern: "/docs/*",
			__I: null as unknown as Record<never, never>,
			__O: null as unknown as { slugParts: string[] },
		},
		{
			parents: ["/"],
			pattern: "/blog/_index",
			__I: null as unknown as Record<never, never>,
			__O: null as unknown as { posts: string[] },
		},
	] as const,
	__vormaAPIRoutes: [
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
			kind: "mutation" as const,
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
			pattern: "/users/:userID",
			params: ["userID"] as const,
			__I: null as unknown as { inviteEmail: string },
			__O: null as unknown as { invited: true },
		},
		{
			method: "POST" as const,
			pattern: "/sessions",
			kind: "query" as const,
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
		if (context.method === "GET") {
			expect_type<"/users/:userID" | "/health">(context.pattern);
			return { headers: [["x-get-api", "1"]] };
		}
		expect_type<"PATCH" | "POST">(context.method);
		return { headers: [["x-body-api", "1"]] };
	},
});
const preact = Preact__createVormaClient(vorma_app_config);
const remix = Remix__createVormaClient(vorma_app_config);
const solid = Solid__createVormaClient(vorma_app_config);

/////// Exported Type Assertions

function assert_exported_type_contracts(): void {
	// View patterns
	type _loader_patterns = Assert<
		IsExact<
			ToViewPattern<App>,
			"/" | "/users" | "/users/:userID" | "/docs/*" | "/blog/_index"
		>
	>;

	// Query methods and patterns
	type _query_methods = Assert<IsExact<ToQueryMethod<App>, "GET" | "POST">>;
	type _query_patterns = Assert<
		IsExact<ToQueryPattern<App>, "/users/:userID" | "/sessions">
	>;
	type _get_query_patterns = Assert<
		IsExact<ToQueryPattern<App, "GET">, "/users/:userID">
	>;
	type _post_query_patterns = Assert<
		IsExact<ToQueryPattern<App, "POST">, "/sessions">
	>;

	// Mutation methods and patterns
	type _mutation_methods = Assert<
		IsExact<ToMutationMethod<App>, "GET" | "PATCH" | "POST">
	>;
	type _mutation_patterns = Assert<
		IsExact<
			ToMutationPattern<App>,
			"/health" | "/users/:userID" | "/logout"
		>
	>;
	type _get_mutation_patterns = Assert<
		IsExact<ToMutationPattern<App, "GET">, "/health">
	>;
	type _post_mutation_patterns = Assert<
		IsExact<ToMutationPattern<App, "POST">, "/users/:userID" | "/logout">
	>;

	// Loader output types
	type _loader_o_root = Assert<
		IsExact<ToLoaderOutput<App, "/">, { sessionUserID: string | null }>
	>;
	type _loader_o_users = Assert<
		IsExact<ToLoaderOutput<App, "/users">, { userCount: number }>
	>;
	type _loader_o_user_detail = Assert<
		IsExact<ToLoaderOutput<App, "/users/:userID">, { userName: string }>
	>;
	type _loader_o_docs = Assert<
		IsExact<ToLoaderOutput<App, "/docs/*">, { slugParts: string[] }>
	>;
	type _loader_o_blog = Assert<
		IsExact<ToLoaderOutput<App, "/blog/_index">, { posts: string[] }>
	>;

	// Loader input types
	type _loader_i_root = Assert<
		IsExact<ToLoaderInput<App, "/">, Record<never, never>>
	>;
	type _loader_i_users = Assert<
		IsExact<ToLoaderInput<App, "/users">, { sort?: "name" | "created" }>
	>;
	type _loader_i_user_detail = Assert<
		IsExact<
			ToLoaderInput<App, "/users/:userID">,
			{ tab?: string; page?: number }
		>
	>;

	// Query I/O types
	type _get_i_users = Assert<
		IsExact<
			ToQueryInput<App, "GET", "/users/:userID">,
			{ includePosts: boolean }
		>
	>;
	type _get_o_users = Assert<
		IsExact<
			ToQueryOutput<App, "GET", "/users/:userID">,
			{ id: string; posts: number }
		>
	>;
	type _post_i_sessions = Assert<
		IsExact<
			ToQueryInput<App, "POST", "/sessions">,
			{ email: string; password: string }
		>
	>;
	type _post_o_sessions = Assert<
		IsExact<ToQueryOutput<App, "POST", "/sessions">, { token: string }>
	>;

	// Mutation I/O types
	type _get_i_health = Assert<
		IsExact<ToMutationInput<App, "GET", "/health">, null>
	>;
	type _get_o_health = Assert<
		IsExact<ToMutationOutput<App, "GET", "/health">, { ok: true }>
	>;
	type _patch_i_users = Assert<
		IsExact<
			ToMutationInput<App, "PATCH", "/users/:userID">,
			{ nickname: string }
		>
	>;
	type _patch_o_users = Assert<
		IsExact<
			ToMutationOutput<App, "PATCH", "/users/:userID">,
			{ saved: true }
		>
	>;
	type _post_i_users = Assert<
		IsExact<
			ToMutationInput<App, "POST", "/users/:userID">,
			{ inviteEmail: string }
		>
	>;
	type _post_o_users = Assert<
		IsExact<
			ToMutationOutput<App, "POST", "/users/:userID">,
			{ invited: true }
		>
	>;
	type _post_i_logout = Assert<
		IsExact<ToMutationInput<App, "POST", "/logout">, undefined>
	>;
	type _post_o_logout = Assert<
		IsExact<ToMutationOutput<App, "POST", "/logout">, { done: true }>
	>;

	// Decorator context
	type _decorator_ctx_get = Assert<
		IsExact<
			Extract<ToAPIDecoratorContext<App>, { method: "GET" }>["pattern"],
			"/users/:userID" | "/health"
		>
	>;
	type _decorator_ctx_post = Assert<
		IsExact<
			Extract<ToAPIDecoratorContext<App>, { method: "POST" }>["pattern"],
			"/users/:userID" | "/sessions" | "/logout"
		>
	>;

	// ToRouteDestination — intersection types are not IsExact-comparable
	// to flat object types. We verify bidirectional assignability for the
	// full type and use IsExact on individual fields. Call-site tests in
	// assert_navigate_contracts prove full correctness.
	type _route_destination_users = Assert<
		ToRouteDestination<App, "/users/:userID"> extends {
			pattern: "/users/:userID";
			params: { userID: string };
			search?: {
				sort?: "name" | "created";
				tab?: string;
				page?: number;
			};
		}
			? {
					pattern: "/users/:userID";
					params: { userID: string };
					search?: {
						sort?: "name" | "created";
						tab?: string;
						page?: number;
					};
				} extends ToRouteDestination<App, "/users/:userID">
				? true
				: false
			: false
	>;
	type _route_destination_docs = Assert<
		ToRouteDestination<App, "/docs/*"> extends {
			pattern: "/docs/*";
			splatValues: string[];
		}
			? {
					pattern: "/docs/*";
					splatValues: string[];
				} extends ToRouteDestination<App, "/docs/*">
				? true
				: false
			: false
	>;
	type _route_destination_index_shorthand = Assert<
		IsExact<
			ToRouteDestination<App, "/blog/_index">["pattern"],
			"/blog/_index" | "/blog"
		>
	>;
	type _route_destination_root = Assert<
		IsExact<ToRouteDestination<App, "/">["pattern"], "/">
	>;
	type _route_target = Assert<
		IsExact<
			Extract<
				ToNavigationTarget<App, "/users/:userID">,
				{ href: string }
			>,
			{
				href: string;
				pattern?: never;
				params?: never;
				splatValues?: never;
				search?: never;
				hash?: never;
			}
		>
	>;
	type _route_target_destination = Assert<
		IsExact<
			Extract<
				ToNavigationTarget<App, "/users/:userID">,
				{ pattern: unknown }
			>,
			ToRouteDestination<App, "/users/:userID">
		>
	>;
	type _navigate_props = Assert<
		IsExact<
			ToNavigateArgs<App, "/users/:userID">,
			ToNavigationTarget<App, "/users/:userID"> & {
				replace?: boolean;
				scrollToTop?: boolean;
				state?: unknown;
				skipProgressIndicator?: boolean;
			}
		>
	>;
	type _route_sync_args = Assert<
		IsExact<
			ToRouteSyncArgs<App, "/users/:userID">,
			ToRouteDestination<App, "/users/:userID"> & {
				enabled?: boolean;
				debounceMs?: number;
				replace?: boolean;
				scrollToTop?: boolean;
			}
		>
	>;

	// ToLinkProps
	type _link_props_state = Assert<
		IsExact<ToLinkProps<App, "/users/:userID">["state"], unknown>
	>;
	type _link_props_prefetch = Assert<
		IsExact<
			ToLinkProps<App, "/users/:userID">["prefetch"],
			"intent" | "none" | undefined
		>
	>;

	// ToClientLoaderArgs
	type _cl_props_params = Assert<
		IsExact<
			ToClientLoaderArgs<App, "/users/:userID">["params"],
			{ userID: string }
		>
	>;
	type _cl_props_splat_values = Assert<
		IsExact<
			ToClientLoaderArgs<App, "/users/:userID">["splatValues"],
			string[]
		>
	>;
	type _cl_props_signal = Assert<
		IsExact<
			ToClientLoaderArgs<App, "/users/:userID">["signal"],
			AbortSignal
		>
	>;
	type _cl_props_href = Assert<
		IsExact<ToClientLoaderArgs<App, "/users/:userID">["href"], string>
	>;
	type _cl_props_history_state = Assert<
		IsExact<
			ToClientLoaderArgs<App, "/users/:userID">["historyState"],
			unknown
		>
	>;
	type _cl_props_pattern = Assert<
		IsExact<
			ToClientLoaderArgs<App, "/users/:userID">["pattern"],
			"/users/:userID"
		>
	>;
	type _cl_props_input = Assert<
		IsExact<
			ToClientLoaderArgs<App, "/users/:userID">["input"],
			{ tab?: string; page?: number }
		>
	>;
	type _cl_props_known_matches = Assert<
		IsExact<
			ToClientLoaderArgs<App, "/users/:userID">["knownMatches"],
			Array<{ pattern: string; input: unknown }>
		>
	>;
	type _cl_props_loader_data = Assert<
		IsExact<
			Awaited<
				ToClientLoaderArgs<App, "/users/:userID">["serverPromise"]
			>["loaderData"],
			{ userName: string }
		>
	>;
	type _cl_props_server_matches = Assert<
		IsExact<
			Awaited<
				ToClientLoaderArgs<App, "/users/:userID">["serverPromise"]
			>["matches"],
			Array<{ pattern: string; input: unknown; loaderData: unknown }>
		>
	>;
	type _cl_props_root_params = Assert<
		IsExact<ToClientLoaderArgs<App, "/">["params"], Record<string, string>>
	>;

	// ToDefineViewArgs
	type _define_view_pattern = Assert<
		IsExact<ToDefineViewArgs<App, "/">["pattern"], "/">
	>;
	type _define_view_error_boundary_optional =
		undefined extends ToDefineViewArgs<App, "/">["errorBoundary"]
			? true
			: false;
	type _define_view_client_loader_optional =
		undefined extends ToDefineViewArgs<App, "/">["clientLoader"]
			? true
			: false;
	type _define_view_hmr_optional = undefined extends ToDefineViewArgs<
		App,
		"/"
	>["runClientLoaderOnHMR"]
		? true
		: false;

	// ToAPIDecorator
	type _decorator_fn = Assert<
		IsExact<
			ToAPIDecorator<App>,
			(
				context: ToAPIDecoratorContext<App>,
			) =>
				| Omit<RequestInit, "method" | "body">
				| undefined
				| Promise<Omit<RequestInit, "method" | "body"> | undefined>
		>
	>;

	// ToAPIClient
	type _api_client_keys = Assert<
		IsExact<
			keyof ToAPIClient<App>,
			| "query"
			| "queryOrThrow"
			| "mutate"
			| "mutateOrThrow"
			| "toIdentityArray"
		>
	>;
	expect_type<QueryError<ToQueryOutput<App, "GET", "/users/:userID">>>(
		null as unknown as ToQueryError<
			App,
			{
				method: "GET";
				pattern: "/users/:userID";
				params: { userID: string };
				input: { includePosts: boolean };
			}
		>,
	);
	expect_type<MutationError<ToMutationOutput<App, "GET", "/health">>>(
		null as unknown as ToMutationError<App, { pattern: "/health" }>,
	);

	// ScrollState
	const scroll_xy: ScrollState = { x: 0, y: 0 };
	const scroll_hash: ScrollState = { hash: "#top" };
	void scroll_xy;
	void scroll_hash;

	// WorkState
	const work_state: WorkState = {
		navigation: {
			href: "/users/u-1",
			replace: false,
			source: "navigate",
		},
		revalidation: {
			status: "running",
			attempt: 1,
		},
		prefetch: {
			href: "/docs/guide",
		},
		apiRequests: [
			{
				key: "create-user",
				method: "POST",
				href: "/api/users",
			},
		],
	};
	expect_type<"navigate" | "popstate" | "redirect">(
		work_state.navigation!.source,
	);
	expect_type<"debouncing" | "running" | "retrying">(
		work_state.revalidation!.status,
	);
	expect_type<number>(work_state.revalidation!.attempt);
	expect_type<string>(work_state.prefetch!.href);
	expect_type<string>(work_state.apiRequests[0]!.key);

	// QueryResult and MutationResult
	const success_result: QueryResult<number> = {
		success: true,
		data: 42,
		response: new Response(),
		revalidationPromise: Promise.resolve({ ok: true }),
	};
	const fail_result: MutationResult<number> = {
		success: false,
		error: "fail",
		response: new Response(),
		revalidationPromise: Promise.resolve({
			ok: false,
			reason: "max_retries_exhausted",
		}),
	};
	void success_result;
	void fail_result;
	const mutation_error = new MutationError<number>(fail_result);
	expect_type<MutationResult<number> & { success: false }>(
		mutation_error.result,
	);
	void mutation_error;

	const progress_indicator_config: ProgressIndicatorConfig = {
		start: () => {},
		stop: () => {},
		isRunning: () => {
			return false;
		},
	};
	void progress_indicator_config;

	const route_error_state: RouteErrorState = {
		idx: 0,
		error: "boom",
		source: "server",
	};
	void route_error_state;

	// RouteRenderEntry
	const entry: RouteRenderEntry = {
		pattern: "/",
		input: {},
		module_url: "/mod.js",
		module: {},
		loader_data: null,
		client_loader_data: undefined,
	};
	expect_type<string>(entry.pattern);
	expect_type<unknown>(entry.input);
	expect_type<string>(entry.module_url);
	expect_type<Record<string, unknown>>(entry.module);
	expect_type<unknown>(entry.loader_data);
	expect_type<unknown>(entry.client_loader_data);

	// RouteRenderState
	const route_render_state: RouteRenderState = {
		entries: [entry],
		error: null,
		params: {},
		splat_values: [],
		client_build_id: "1",
		history_state: undefined,
	};
	expect_type<RouteRenderEntry[]>(route_render_state.entries);
	expect_type<RouteRenderState["error"]>(route_render_state.error);
	expect_type<Record<string, string>>(route_render_state.params);
	expect_type<string[]>(route_render_state.splat_values);
	expect_type<string>(route_render_state.client_build_id);

	// RouteState
	const route_state: RouteState = {
		href: "/users/u-1?tab=posts",
		historyState: undefined,
		clientBuildID: "1",
		params: {
			userID: "u-1",
		},
		splatValues: [],
		matches: [
			{
				pattern: "/users/:userID",
				input: {
					tab: "posts",
				},
				loaderData: {
					userName: "Ada",
				},
				clientLoaderData: undefined,
			},
		],
		error: null,
	};
	expect_type<string>(route_state.href);
	expect_type<unknown>(route_state.historyState);
	expect_type<string>(route_state.clientBuildID);
	expect_type<Record<string, string>>(route_state.params);
	expect_type<string[]>(route_state.splatValues);
	expect_type<unknown>(route_state.matches[0]!.loaderData);
	expect_type<RouteState["error"]>(route_state.error);

	const route_update_reason: RouteUpdateReason = "navigation";
	expect_type<"boot" | "navigation" | "popstate" | "revalidation">(
		route_update_reason,
	);

	// ViewDefinition
	const view_def: ViewDefinition = {
		pattern: "/",
		component: () => {
			return null!;
		},
	};
	expect_type<string>(view_def.pattern);
	expect_type<(props: any) => any>(view_def.component);
}
void assert_exported_type_contracts;

/////// Navigate Type Safety

function assert_navigate_contracts(): void {
	// Valid: raw href string
	void react.navigate({ href: "/users/u-1?tab=posts#recent" });

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
		skipProgressIndicator: true,
	});

	// Valid: with search and hash
	void react.navigate({
		pattern: "/users/:userID",
		params: { userID: "u-1" },
		search: { sort: "created", tab: "posts", page: 2 },
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
	void react.navigate({
		pattern: "/users/:userID",
		params: { userID: "u-1" },
		// @ts-expect-error search must match the loader input type.
		search: { page: "2" },
	});
	void react.navigate({
		pattern: "/users/:userID",
		params: { userID: "u-1" },
		// @ts-expect-error search must match parent loader input types.
		search: { sort: "updated" },
	});
	// @ts-expect-error href string cannot be mixed with typed search.
	void react.navigate({
		href: "/users/u-1",
		search: { tab: "posts" },
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

/////// Link Type Safety

function assert_link_contracts(): void {
	void react.Link({
		href: "/users/u-1?tab=posts#recent",
		children: null,
	});

	void react.Link({
		pattern: "/users/:userID",
		params: { userID: "u-1" },
		search: { sort: "created", tab: "posts", page: 2 },
		hash: "#recent",
		prefetch: "intent",
		children: null,
	});

	// @ts-expect-error href string cannot be mixed with typed search.
	void react.Link({
		href: "/users/u-1",
		children: null,
		search: { tab: "posts" },
	});

	// @ts-expect-error route params are required for /users/:userID.
	void react.Link({
		pattern: "/users/:userID",
		children: null,
	});
}
void assert_link_contracts;

/////// API Client Type Safety

function assert_api_client_contracts(): void {
	const health_identity = react.apiClient.toIdentityArray({
		pattern: "/health",
	});
	expect_type<unknown[]>(health_identity);

	// @ts-expect-error identity arrays use the same API route identity typing.
	void react.apiClient.toIdentityArray({
		pattern: "/logout",
	});

	// Valid GET API route with required params and input. Method is required
	// because this pattern has multiple API route methods.
	const user_get_result = react.apiClient.query({
		method: "GET",
		pattern: "/users/:userID",
		params: { userID: "u-1" },
		input: { includePosts: true },
	});
	expect_type<
		Promise<QueryResult<ToQueryOutput<App, "GET", "/users/:userID">>>
	>(user_get_result);

	// Valid GET-only API route with nullable input. Method can be omitted.
	void react.apiClient.mutate({
		pattern: "/health",
	});

	// Valid GET-only API route with nullable input (explicit null).
	void react.apiClient.mutate({
		pattern: "/health",
		input: null,
	});

	// Valid GET-only API route with flattened request options.
	void react.apiClient.mutate({
		pattern: "/health",
		dedupeKey: "health-check",
		revalidate: false,
	});

	// @ts-expect-error method is required for patterns with multiple API route methods.
	void react.apiClient.query({
		pattern: "/users/:userID",
		params: { userID: "u-1" },
		input: { includePosts: true },
	});

	// @ts-expect-error object API route inputs are required when input type is non-empty.
	void react.apiClient.query({
		method: "GET",
		pattern: "/users/:userID",
		params: { userID: "u-1" },
	});

	void react.apiClient.query({
		method: "GET",
		pattern: "/users/:userID",
		// @ts-expect-error API route params must match route parameter names.
		params: { id: "u-1" },
		input: { includePosts: true },
	});

	void react.apiClient.mutate({
		pattern: "/health",
		// @ts-expect-error API route input root must be object, null, or undefined.
		input: "invalid",
	});

	void react.apiClient.query({
		method: "GET",
		// @ts-expect-error API route pattern must come from API route patterns.
		pattern: "/docs/*",
		splatValues: ["x"],
		// @ts-expect-error API route input must match one of the API route inputs.
		input: {},
	});

	// Valid PATCH API route with required params and input.
	const patch_result = react.apiClient.mutate({
		method: "PATCH",
		pattern: "/users/:userID",
		params: { userID: "u-1" },
		input: { nickname: "neo" },
	});
	expect_type<
		Promise<
			MutationResult<ToMutationOutput<App, "PATCH", "/users/:userID">>
		>
	>(patch_result);

	// Valid same-pattern POST API route with different input and output.
	const user_post_result = react.apiClient.mutate({
		method: "POST",
		pattern: "/users/:userID",
		params: { userID: "u-1" },
		input: { inviteEmail: "ada@example.com" },
	});
	expect_type<
		Promise<MutationResult<ToMutationOutput<App, "POST", "/users/:userID">>>
	>(user_post_result);

	// Valid POST API route.
	const post_result = react.apiClient.query({
		method: "POST",
		pattern: "/sessions",
		input: { email: "a@b.com", password: "pw" },
	});
	expect_type<Promise<QueryResult<ToQueryOutput<App, "POST", "/sessions">>>>(
		post_result,
	);

	// Valid POST API route with optional input (omitted).
	void react.apiClient.mutate({
		method: "POST",
		pattern: "/logout",
	});

	// @ts-expect-error method is required for non-GET API routes.
	void react.apiClient.mutate({
		pattern: "/logout",
	});

	void react.apiClient.mutate({
		method: "POST",
		pattern: "/users/:userID",
		params: { userID: "u-1" },
		// @ts-expect-error method/pattern identity selects POST input.
		input: { nickname: "neo" },
	});

	// @ts-expect-error non-empty API route input is required.
	void react.apiClient.query({
		method: "POST",
		pattern: "/sessions",
	});

	void react.apiClient.query({
		// @ts-expect-error method must come from API route methods.
		method: "PUT",
		pattern: "/sessions",
		input: { email: "a@b.com", password: "pw" },
	});

	void react.apiClient.mutate({
		method: "POST",
		pattern: "/health",
		// @ts-expect-error method/pattern pair must exist.
		input: {},
	});
}
void assert_api_client_contracts;

/////// Query and Mutation Args Type Safety

function assert_query_mutation_args_contracts(): void {
	// Required GET input.
	const required_get_props: ToQueryArgs<App> = {
		method: "GET",
		pattern: "/users/:userID",
		params: { userID: "u-1" },
		input: { includePosts: true },
	};
	expect_type<{ includePosts: boolean }>(required_get_props.input);

	// Optional GET input (omitted, null, undefined).
	const optional_get_omitted: ToMutationArgs<App> = {
		pattern: "/health",
	};
	const optional_get_null: ToMutationArgs<App> = {
		pattern: "/health",
		input: null,
	};
	const optional_get_undefined: ToMutationArgs<App> = {
		pattern: "/health",
		input: undefined,
	};
	void optional_get_omitted;
	void optional_get_null;
	void optional_get_undefined;

	// @ts-expect-error non-empty GET input must be required.
	const missing_get_input: ToQueryArgs<App> = {
		method: "GET",
		pattern: "/users/:userID",
		params: { userID: "u-1" },
	};
	void missing_get_input;

	const wrong_get_params: ToQueryArgs<App> = {
		method: "GET",
		pattern: "/users/:userID",
		// @ts-expect-error API route params keys must match route params.
		params: { id: "u-1" },
		input: { includePosts: true },
	};
	void wrong_get_params;

	// Required PATCH API route.
	const required_patch: ToMutationArgs<App> = {
		method: "PATCH",
		pattern: "/users/:userID",
		params: { userID: "u-1" },
		input: { nickname: "neo" },
	};
	expect_type<{ nickname: string }>(required_patch.input);

	// Required same-pattern POST API route.
	const required_user_post: ToMutationArgs<App> = {
		method: "POST",
		pattern: "/users/:userID",
		params: { userID: "u-1" },
		input: { inviteEmail: "ada@example.com" },
	};
	expect_type<{ inviteEmail: string }>(required_user_post.input);

	// Required POST API route.
	const required_post: ToQueryArgs<App> = {
		method: "POST",
		pattern: "/sessions",
		input: { email: "a@b.com", password: "pw" },
	};
	void required_post;

	// Optional POST input (omitted, undefined).
	const optional_post_omitted: ToMutationArgs<App> = {
		method: "POST",
		pattern: "/logout",
	};
	const optional_post_undefined: ToMutationArgs<App> = {
		method: "POST",
		pattern: "/logout",
		input: undefined,
	};
	const query_get_props: ToQueryArgs<App> = {
		method: "GET",
		pattern: "/users/:userID",
		params: { userID: "u-1" },
		input: { includePosts: true },
	};
	const query_post_props: ToQueryArgs<App> = {
		method: "POST",
		pattern: "/sessions",
		input: { email: "a@b.com", password: "pw" },
	};
	const mutation_get_props: ToMutationArgs<App> = {
		pattern: "/health",
	};
	void optional_post_omitted;
	void optional_post_undefined;
	void query_get_props;
	void query_post_props;
	void mutation_get_props;

	// @ts-expect-error non-empty POST input must be required.
	const missing_post_input: ToQueryArgs<App> = {
		method: "POST",
		pattern: "/sessions",
	};
	void missing_post_input;

	const wrong_patch_input: ToMutationArgs<App> = {
		method: "PATCH",
		pattern: "/users/:userID",
		params: { userID: "u-1" },
		// @ts-expect-error method/pattern identity selects PATCH input.
		input: { inviteEmail: "ada@example.com" },
	};
	void wrong_patch_input;

	const wrong_post_method: ToQueryArgs<App> = {
		// @ts-expect-error method/pattern pair must exist.
		method: "PUT",
		pattern: "/sessions",
		input: { email: "a@b.com", password: "pw" },
	};
	void wrong_post_method;

	const wrong_api_route_params: ToMutationArgs<App> = {
		method: "PATCH",
		pattern: "/users/:userID",
		// @ts-expect-error API route params keys must match route params.
		params: { id: "u-1" },
		input: { nickname: "neo" },
	};
	void wrong_api_route_params;

	const body_not_allowed: ToMutationArgs<App> = {
		method: "POST",
		pattern: "/logout",
		// @ts-expect-error typed mutations use input, not body.
		body: "raw",
	};
	void body_not_allowed;
}
void assert_query_mutation_args_contracts;

/////// React Adapter Type Safety

function assert_react_adapter_contracts(): void {
	const route_props = null as unknown as ToRouteComponentProps<
		App,
		"/users/:userID"
	>;

	// useLoaderData
	const loader_data = react.useLoaderData(route_props);
	expect_type<ToLoaderOutput<App, "/users/:userID">>(loader_data);

	// usePatternLoaderData
	const maybe_pattern_data = react.usePatternLoaderData("/docs/*");
	expect_type<ToLoaderOutput<App, "/docs/*"> | undefined>(maybe_pattern_data);

	const maybe_root_data = react.usePatternLoaderData("/");
	expect_type<ToLoaderOutput<App, "/"> | undefined>(maybe_root_data);

	const maybe_blog_data = react.usePatternLoaderData("/blog/_index");
	expect_type<ToLoaderOutput<App, "/blog/_index"> | undefined>(
		maybe_blog_data,
	);

	void react.usePatternLoaderData(
		// @ts-expect-error typed pattern loader data rejects unknown patterns.
		"/not-a-route",
	);

	// useRouteState
	const route_state = react.useRouteState();
	expect_type<RouteState>(route_state);
	expect_type<string>(route_state.clientBuildID);
	expect_type<string[]>(route_state.splatValues);
	expect_type<Record<string, string>>(route_state.params);
	expect_type<unknown>(route_state.historyState);

	const selected_route_href = react.useRouteState((route) => {
		return route.href;
	});
	expect_type<string>(selected_route_href);

	// useWorkState
	const work_state = react.useWorkState();
	expect_type<WorkState>(work_state);
	expect_type<WorkState["navigation"]>(work_state.navigation);

	const selected_navigation_href = react.useWorkState((work) => {
		return work.navigation?.href;
	});
	expect_type<string | undefined>(selected_navigation_href);

	// useRouteSync
	react.useRouteSync({
		pattern: "/users/:userID",
		params: { userID: "u-1" },
		search: { page: 2, tab: "posts" },
		enabled: true,
		debounceMs: 250,
		replace: true,
		scrollToTop: false,
	});

	react.useRouteSync({
		pattern: "/docs/*",
		splatValues: ["guides", "intro"],
	});

	react.useRouteSync({
		// @ts-expect-error route sync rejects unknown patterns.
		pattern: "/not-a-route",
	});

	react.useRouteSync({
		pattern: "/users/:userID",
		// @ts-expect-error route sync requires typed params.
		params: {},
	});

	// getRouteState
	const current_route_state = react.getRouteState();
	expect_type<RouteState>(current_route_state);
	expect_type<Record<string, string>>(current_route_state.params);
	expect_type<unknown>(current_route_state.historyState);

	// getWorkState
	const current_work_state = react.getWorkState();
	expect_type<WorkState>(current_work_state);

	// defineView: basic
	void react.defineView({
		pattern: "/",
		component: () => {
			return null!;
		},
	});

	// defineView: with errorBoundary
	void react.defineView({
		pattern: "/docs/*",
		component: (props) => {
			const data = react.useLoaderData(props);
			expect_type<ToLoaderOutput<App, "/docs/*">>(data);
			return null!;
		},
		errorBoundary: (props) => {
			expect_type<unknown>(props.error);
			return null!;
		},
	});

	// defineView: with component, clientLoader, and useClientLoaderData
	void react.defineView({
		pattern: "/users/:userID",
		component: (props) => {
			const data = react.useLoaderData(props);
			expect_type<ToLoaderOutput<App, "/users/:userID">>(data);

			const client_loader_data = react.useClientLoaderData(props);
			expect_type<number>(client_loader_data);

			return props.Outlet();
		},
		clientLoader: async ({
			href,
			historyState,
			input,
			knownMatches,
			params,
			pattern,
			splatValues,
			serverPromise,
			signal,
			trigger,
		}) => {
			expect_type<{ userID: string }>(params);
			expect_type<string>(params.userID);
			// @ts-expect-error only declared param keys are accessible.
			void params.bogus;
			expect_type<string[]>(splatValues);
			expect_type<AbortSignal>(signal);
			expect_type<"boot" | "navigation" | "revalidation" | "prefetch">(
				trigger,
			);
			expect_type<string>(href);
			expect_type<unknown>(historyState);
			expect_type<"/users/:userID">(pattern);
			expect_type<{ tab?: string; page?: number }>(input);
			expect_type<Array<{ pattern: string; input: unknown }>>(
				knownMatches,
			);
			const server_data = await serverPromise;
			expect_type<ToLoaderOutput<App, "/users/:userID">>(
				server_data.loaderData,
			);
			expect_type<string>(server_data.clientBuildID);
			expect_type<
				Array<{ pattern: string; input: unknown; loaderData: unknown }>
			>(server_data.matches);
			expect_type<null | { idx: number; error: unknown }>(
				server_data.outermostServerError,
			);
			return server_data.loaderData.userName.length;
		},
		beforeRouteCommit: async ({ current, next, signal, trigger }) => {
			expect_type<Exclude<RouteUpdateReason, "boot">>(trigger);
			expect_type<AbortSignal>(signal);
			expect_type<string>(current.href);
			expect_type<string>(next.href);
		},
		beforeRouteYield: async ({ current, next, signal, trigger }) => {
			expect_type<Exclude<RouteUpdateReason, "boot">>(trigger);
			expect_type<AbortSignal>(signal);
			expect_type<string>(current.href);
			expect_type<string>(next.href);
		},
		runClientLoaderOnHMR: true,
	});

	// useClientLoaderData: tested with an explicit T via RouteProps
	const cl_route_props = null as unknown as ToRouteComponentProps<
		App,
		"/users/:userID",
		number
	>;
	const client_loader_data = react.useClientLoaderData(cl_route_props);
	expect_type<number>(client_loader_data);

	// usePatternClientLoaderData
	const maybe_client_loader_data =
		react.usePatternClientLoaderData<number>("/users/:userID");
	expect_type<number | undefined>(maybe_client_loader_data);

	// defineView: invalid pattern
	void react.defineView({
		// @ts-expect-error defineView pattern must exist in loader route patterns.
		pattern: "/not-a-route",
		component: () => {
			return null!;
		},
	});

	// defineView: invalid runClientLoaderOnHMR
	void react.defineView({
		pattern: "/",
		component: () => {
			return null!;
		},
		// @ts-expect-error runClientLoaderOnHMR must be boolean.
		runClientLoaderOnHMR: "1",
	});

	// Link: valid cases
	void react.Link({
		href: "/users/u-1?tab=posts",
	});
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
		search: { sort: "name", tab: "posts" },
		hash: "#recent",
	});
	void react.Link({
		pattern: "/",
		prefetch: "none",
	});

	// Link: invalid cases
	// @ts-expect-error typed links require params for dynamic routes.
	void react.Link({ pattern: "/users/:userID" });
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
	void react.Link({
		pattern: "/users/:userID",
		params: { userID: "u-1" },
		// @ts-expect-error typed links enforce loader search input.
		search: { page: "2" },
	});
	void react.Link({
		pattern: "/users/:userID",
		params: { userID: "u-1" },
		// @ts-expect-error typed links enforce parent loader search input.
		search: { sort: "updated" },
	});
	// @ts-expect-error href string cannot be mixed with typed hash.
	void react.Link({
		href: "/users/u-1",
		hash: "#recent",
	});
	// @ts-expect-error typed links require splatValues for splat routes.
	void react.Link({ pattern: "/docs/*" });
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
	// client options
	const react_with_client_options = React__createVormaClient(
		vorma_app_config,
		{
			render: async ({ RootOutlet, rootEl }) => {
				expect_type<ReactComponentType>(RootOutlet);
				expect_type<HTMLElement>(rootEl);
			},
			useViewTransitions: true,
			onRouteUpdate: (route, previous_route, reason) => {
				expect_type<RouteState>(route);
				expect_type<RouteState | null>(previous_route);
				expect_type<RouteUpdateReason>(reason);
			},
			onWorkUpdate: (work) => {
				expect_type<WorkState>(work);
			},
			onBuildSkewDetected: (event) => {
				expect_type<BuildSkewDetectedEvent>(event);
				expect_type<string>(event.activeClientBuildID);
				expect_type<string>(event.serverBuildID);
				expect_type<RouteState>(event.currentRouteState);
				expect_type<WorkState>(event.currentWorkState);
				expect_type<"dropResponse" | "hardReload" | "notifyOnly">(
					event.defaultBehavior,
				);
				if (event.triggeringResponse.kind === "route") {
					expect_type<
						"navigation" | "popstate" | "revalidation" | "prefetch"
					>(event.triggeringResponse.trigger);
					if (event.triggeringResponse.trigger === "revalidation") {
						expect_type<RevalidationReason>(
							event.triggeringResponse.revalidationReason,
						);
					}
				} else {
					expect_type<"query" | "mutation">(
						event.triggeringResponse.apiRouteKind,
					);
				}
			},
		},
	);
	const boot_promise = react_with_client_options.boot();
	expect_type<Promise<Result<void>>>(boot_promise);

	const preact_boot_promise = Preact__createVormaClient(vorma_app_config, {
		render: async ({ RootOutlet, rootEl }) => {
			expect_type<PreactComponentType>(RootOutlet);
			expect_type<HTMLElement>(rootEl);
		},
	}).boot();
	expect_type<Promise<Result<void>>>(preact_boot_promise);

	const remix_boot_promise = Remix__createVormaClient(vorma_app_config, {
		render: async ({ RootOutlet, rootEl }) => {
			expect_type<RemixComponent>(RootOutlet);
			expect_type<HTMLElement>(rootEl);
		},
	}).boot();
	expect_type<Promise<Result<void>>>(remix_boot_promise);

	const solid_boot_promise = Solid__createVormaClient(vorma_app_config, {
		render: async ({ RootOutlet, rootEl }) => {
			expect_type<SolidComponent>(RootOutlet);
			expect_type<HTMLElement>(rootEl);
		},
	}).boot();
	expect_type<Promise<Result<void>>>(solid_boot_promise);

	// navigate
	const navigate_result = react.navigate({
		pattern: "/users/:userID",
		params: { userID: "u-1" },
		replace: true,
		scrollToTop: false,
	});
	expect_type<Promise<{ didNavigate: boolean }>>(navigate_result);

	// prefetch
	react.prefetch({
		pattern: "/users/:userID",
		params: { userID: "u-1" },
	});
	react.cancelPrefetch({ href: "/users/u-1" });

	// toHref
	const built_href = react.toHref({
		pattern: "/users/:userID",
		params: { userID: "u-1" },
		search: { sort: "name", tab: "posts" },
	});
	expect_type<string>(built_href);

	// revalidate
	const revalidate_result = react.revalidate();
	expect_type<Promise<RevalidationResult>>(revalidate_result);

	// getWorkState
	const work = react.getWorkState();
	expect_type<WorkState>(work);
	expect_type<WorkState["apiRequests"]>(work.apiRequests);

	// revalidateOnWindowFocus
	void React__createVormaClient(vorma_app_config, {
		revalidateOnWindowFocus: true,
	}).boot();
	void React__createVormaClient(vorma_app_config, {
		revalidateOnWindowFocus: false,
	}).boot();
	void React__createVormaClient(vorma_app_config, {
		revalidateOnWindowFocus: { staleTimeMS: 3000 },
	}).boot();

	// progressIndicator (all categories)
	const boot_with_progress = React__createVormaClient(vorma_app_config, {
		progressIndicator: {
			start: () => {},
			stop: () => {},
			isRunning: () => {
				return false;
			},
			include: ["navigations", "apiRequests", "revalidations"],
			startDelayMS: 30,
			stopDelayMS: 40,
		},
	}).boot();
	expect_type<Promise<Result<void>>>(boot_with_progress);

	// progressIndicator (include "all")
	void React__createVormaClient(vorma_app_config, {
		progressIndicator: {
			start: () => {},
			stop: () => {},
			isRunning: () => {
				return false;
			},
			include: "all",
		},
	}).boot();

	// progressIndicator (subset)
	void React__createVormaClient(vorma_app_config, {
		progressIndicator: {
			start: () => {},
			stop: () => {},
			isRunning: () => {
				return false;
			},
			include: ["navigations"],
		},
	}).boot();
}
void assert_public_runtime_contracts;

/////// Preact Adapter Type Safety

function assert_preact_adapter_contracts(): void {
	const route_props = null as unknown as ToRouteComponentProps<
		App,
		"/users/:userID"
	>;

	// useLoaderData returns ReadonlySignal<T>
	const loader_data = preact.useLoaderData(route_props);
	expect_type<ReadonlySignal<ToLoaderOutput<App, "/users/:userID">>>(
		loader_data,
	);
	expect_type<ToLoaderOutput<App, "/users/:userID">>(loader_data.value);

	// usePatternLoaderData
	const maybe_pattern_data = preact.usePatternLoaderData("/docs/*");
	expect_type<ReadonlySignal<ToLoaderOutput<App, "/docs/*"> | undefined>>(
		maybe_pattern_data,
	);

	// useRouteState
	const route_state = preact.useRouteState();
	expect_type<ReadonlySignal<RouteState>>(route_state);
	expect_type<Record<string, string>>(route_state.value.params);
	expect_type<unknown>(route_state.value.historyState);

	const selected_params = preact.useRouteState((route) => {
		return route.params;
	});
	expect_type<ReadonlySignal<Record<string, string>>>(selected_params);

	// useWorkState
	const work_state = preact.useWorkState();
	expect_type<ReadonlySignal<WorkState>>(work_state);

	const selected_submission_count = preact.useWorkState((work) => {
		return work.apiRequests.length;
	});
	expect_type<ReadonlySignal<number>>(selected_submission_count);

	// useRouteSync
	preact.useRouteSync({
		pattern: "/users/:userID",
		params: { userID: "u-1" },
		search: { page: 2, tab: "posts" },
		enabled: true,
		debounceMs: 250,
		replace: true,
		scrollToTop: false,
	});

	// useClientLoaderData returns ReadonlySignal<T>
	const cl_route_props = null as unknown as ToRouteComponentProps<
		App,
		"/users/:userID",
		number
	>;
	const client_loader_data = preact.useClientLoaderData(cl_route_props);
	expect_type<ReadonlySignal<number>>(client_loader_data);
	expect_type<number>(client_loader_data.value);

	// usePatternClientLoaderData
	const maybe_client_loader_data =
		preact.usePatternClientLoaderData<number>("/users/:userID");
	expect_type<ReadonlySignal<number | undefined>>(maybe_client_loader_data);

	// defineView
	void preact.defineView({
		pattern: "/users/:userID",
		component: (props) => {
			const data = preact.useLoaderData(props);
			expect_type<ReadonlySignal<ToLoaderOutput<App, "/users/:userID">>>(
				data,
			);
			return null!;
		},
	});

	void preact.defineView({
		// @ts-expect-error defineView rejects unknown patterns.
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
	void preact.Link({ pattern: "/users/:userID" });
}
void assert_preact_adapter_contracts;

/////// Remix Adapter Type Safety

function assert_remix_adapter_contracts(): void {
	const route_props = null as unknown as ToRouteComponentProps<
		App,
		"/users/:userID"
	>;

	// useLoaderData returns the selected value.
	const loader_data = remix.useLoaderData(route_props);
	expect_type<ToLoaderOutput<App, "/users/:userID">>(loader_data);

	// usePatternLoaderData
	const maybe_pattern_data = remix.usePatternLoaderData("/docs/*");
	expect_type<ToLoaderOutput<App, "/docs/*"> | undefined>(maybe_pattern_data);

	// useRouteState
	const route_state = remix.useRouteState();
	expect_type<RouteState>(route_state);
	expect_type<Record<string, string>>(route_state.params);
	expect_type<unknown>(route_state.historyState);

	const selected_params = remix.useRouteState((route) => {
		return route.params;
	});
	expect_type<Record<string, string>>(selected_params);

	// useWorkState
	const work_state = remix.useWorkState();
	expect_type<WorkState>(work_state);

	const selected_submission_count = remix.useWorkState((work) => {
		return work.apiRequests.length;
	});
	expect_type<number>(selected_submission_count);

	// useRouteSync
	remix.useRouteSync({
		pattern: "/users/:userID",
		params: { userID: "u-1" },
		search: { page: 2, tab: "posts" },
		enabled: true,
		debounceMs: 250,
		replace: true,
		scrollToTop: false,
	});

	// useClientLoaderData returns the selected value.
	const cl_route_props = null as unknown as ToRouteComponentProps<
		App,
		"/users/:userID",
		number
	>;
	const client_loader_data = remix.useClientLoaderData(cl_route_props);
	expect_type<number>(client_loader_data);

	// usePatternClientLoaderData
	const maybe_client_loader_data =
		remix.usePatternClientLoaderData<number>("/users/:userID");
	expect_type<number | undefined>(maybe_client_loader_data);

	// defineView accepts Remix UI component factories.
	void remix.defineView({
		pattern: "/users/:userID",
		component: (
			_handle: RemixHandle<ToRouteComponentProps<App, "/users/:userID">>,
		) => {
			return (props) => {
				const data = remix.useLoaderData(props);
				expect_type<ToLoaderOutput<App, "/users/:userID">>(data);
				return props.Outlet() as RemixNode;
			};
		},
		errorBoundary: (_handle: RemixHandle<{ error: unknown }>) => {
			return (props) => {
				expect_type<unknown>(props.error);
				return null;
			};
		},
	});

	void remix.defineView({
		// @ts-expect-error defineView rejects unknown patterns.
		pattern: "/not-a-route",
		component: () => {
			return () => {
				return null;
			};
		},
	});

	// Link
	void remix.Link({
		pattern: "/users/:userID",
		params: { userID: "u-1" },
	});

	// @ts-expect-error typed links require params for dynamic routes.
	void remix.Link({ pattern: "/users/:userID" });
}
void assert_remix_adapter_contracts;

/////// Solid Adapter Type Safety

function assert_solid_adapter_contracts(): void {
	const route_props = null as unknown as ToRouteComponentProps<
		App,
		"/users/:userID"
	>;

	// useLoaderData returns Accessor<T>
	const loader_data = solid.useLoaderData(route_props);
	expect_type<Accessor<ToLoaderOutput<App, "/users/:userID">>>(loader_data);
	// Calling the accessor returns the value
	expect_type<ToLoaderOutput<App, "/users/:userID">>(loader_data());

	// usePatternLoaderData returns Accessor<T | undefined>
	const maybe_pattern_data = solid.usePatternLoaderData("/docs/*");
	expect_type<Accessor<ToLoaderOutput<App, "/docs/*"> | undefined>>(
		maybe_pattern_data,
	);

	// useRouteState returns Accessor
	const route_state = solid.useRouteState();
	expect_type<Accessor<RouteState>>(route_state);
	expect_type<Record<string, string>>(route_state().params);
	expect_type<unknown>(route_state().historyState);

	const selected_params = solid.useRouteState((route) => {
		return route.params;
	});
	expect_type<Accessor<Record<string, string>>>(selected_params);

	// useWorkState returns Accessor
	const work_state = solid.useWorkState();
	expect_type<Accessor<WorkState>>(work_state);

	const selected_submission_count = solid.useWorkState((work) => {
		return work.apiRequests.length;
	});
	expect_type<Accessor<number>>(selected_submission_count);

	// useRouteSync
	solid.useRouteSync({
		pattern: "/users/:userID",
		params: { userID: "u-1" },
		search: { page: 2, tab: "posts" },
		enabled: true,
		debounceMs: 250,
		replace: true,
		scrollToTop: false,
	});

	// useClientLoaderData returns Accessor<T>
	const cl_route_props = null as unknown as ToRouteComponentProps<
		App,
		"/users/:userID",
		number
	>;
	const client_loader_data = solid.useClientLoaderData(cl_route_props);
	expect_type<Accessor<number>>(client_loader_data);
	expect_type<number>(client_loader_data());

	// usePatternClientLoaderData returns Accessor<T | undefined>
	const maybe_client_loader_data =
		solid.usePatternClientLoaderData<number>("/users/:userID");
	expect_type<Accessor<number | undefined>>(maybe_client_loader_data);

	// defineView
	void solid.defineView({
		pattern: "/users/:userID",
		component: (props) => {
			const data = solid.useLoaderData(props);
			expect_type<Accessor<ToLoaderOutput<App, "/users/:userID">>>(data);
			return null!;
		},
	});

	void solid.defineView({
		// @ts-expect-error defineView rejects unknown patterns.
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
	void solid.Link({ pattern: "/users/:userID" });
}
void assert_solid_adapter_contracts;
