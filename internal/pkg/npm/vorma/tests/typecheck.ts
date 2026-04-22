/// <reference types="vite/client" />

import type { ReadonlySignal } from "@preact/signals";
import type { ComponentType as PreactComponentType } from "preact";
import type { ComponentType as ReactComponentType } from "react";
import type { Accessor, Component as SolidComponent } from "solid-js";
import type {
	ActionKind,
	AppConfig,
	ProgressIndicatorConfig,
	RevalidationResult,
	RouteDefinition,
	RouteErrorState,
	RouteRenderEntry,
	RouteRenderState,
	RouteState,
	RouteUpdateReason,
	ScrollState,
	SubmitResult,
	ToActionInput,
	ToActionKind,
	ToActionMethod,
	ToActionOutput,
	ToActionPattern,
	ToActionSubmitArgs,
	ToActionSubmitArgsByKind,
	ToActionSubmitError,
	ToActionSubmitOutput,
	ToAPIClient,
	ToAPIDecorator,
	ToAPIDecoratorContext,
	ToClientLoaderArgs,
	ToDefineRouteArgs,
	ToLinkProps,
	ToLoaderInput,
	ToLoaderOutput,
	ToLoaderPattern,
	ToNavigateArgs,
	ToNavigationTarget,
	ToRouteComponentProps,
	ToRouteDestination,
	ToRouteSyncArgs,
	WorkState,
} from "vorma/__internal";
import { SubmitError } from "vorma/__internal";
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
			return { headers: [["x-get-action", "1"]] };
		}
		expect_type<"PATCH" | "POST">(context.method);
		return { headers: [["x-body-action", "1"]] };
	},
});
const preact = Preact__createVormaClient(vorma_app_config);
const solid = Solid__createVormaClient(vorma_app_config);

/////// Exported Type Assertions

function assert_exported_type_contracts(): void {
	// Loader patterns
	type _loader_patterns = Assert<
		IsExact<
			ToLoaderPattern<App>,
			"/" | "/users" | "/users/:userID" | "/docs/*" | "/blog/_index"
		>
	>;

	// Action methods and patterns
	type _action_methods = Assert<
		IsExact<ToActionMethod<App>, "GET" | "PATCH" | "POST">
	>;
	type _action_patterns = Assert<
		IsExact<
			ToActionPattern<App>,
			"/users/:userID" | "/health" | "/sessions" | "/logout"
		>
	>;
	type _get_action_patterns = Assert<
		IsExact<ToActionPattern<App, "GET">, "/users/:userID" | "/health">
	>;
	type _post_action_patterns = Assert<
		IsExact<
			ToActionPattern<App, "POST">,
			"/users/:userID" | "/sessions" | "/logout"
		>
	>;
	type _action_type_default_get = Assert<
		IsExact<ToActionKind<App, "GET", "/users/:userID">, "query">
	>;
	type _action_type_default_post = Assert<
		IsExact<ToActionKind<App, "POST", "/users/:userID">, "mutation">
	>;
	type _action_type_override_get = Assert<
		IsExact<ToActionKind<App, "GET", "/health">, "mutation">
	>;
	type _action_type_override_post = Assert<
		IsExact<ToActionKind<App, "POST", "/sessions">, "query">
	>;
	type _action_type_union = Assert<IsExact<ActionKind, "query" | "mutation">>;

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

	// Action I/O types
	type _get_i_users = Assert<
		IsExact<
			ToActionInput<App, "GET", "/users/:userID">,
			{ includePosts: boolean }
		>
	>;
	type _get_o_users = Assert<
		IsExact<
			ToActionOutput<App, "GET", "/users/:userID">,
			{ id: string; posts: number }
		>
	>;
	type _get_i_health = Assert<
		IsExact<ToActionInput<App, "GET", "/health">, null>
	>;
	type _get_o_health = Assert<
		IsExact<ToActionOutput<App, "GET", "/health">, { ok: true }>
	>;
	type _patch_i_users = Assert<
		IsExact<
			ToActionInput<App, "PATCH", "/users/:userID">,
			{ nickname: string }
		>
	>;
	type _patch_o_users = Assert<
		IsExact<ToActionOutput<App, "PATCH", "/users/:userID">, { saved: true }>
	>;
	type _post_i_users = Assert<
		IsExact<
			ToActionInput<App, "POST", "/users/:userID">,
			{ inviteEmail: string }
		>
	>;
	type _post_o_users = Assert<
		IsExact<
			ToActionOutput<App, "POST", "/users/:userID">,
			{ invited: true }
		>
	>;
	type _post_i_sessions = Assert<
		IsExact<
			ToActionInput<App, "POST", "/sessions">,
			{ email: string; password: string }
		>
	>;
	type _post_o_sessions = Assert<
		IsExact<ToActionOutput<App, "POST", "/sessions">, { token: string }>
	>;
	type _post_i_logout = Assert<
		IsExact<ToActionInput<App, "POST", "/logout">, undefined>
	>;
	type _post_o_logout = Assert<
		IsExact<ToActionOutput<App, "POST", "/logout">, { done: true }>
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

	// ToDefineRouteArgs
	type _define_route_pattern = Assert<
		IsExact<ToDefineRouteArgs<App, "/">["pattern"], "/">
	>;
	type _define_route_error_boundary_optional =
		undefined extends ToDefineRouteArgs<App, "/">["errorBoundary"]
			? true
			: false;
	type _define_route_client_loader_optional =
		undefined extends ToDefineRouteArgs<App, "/">["clientLoader"]
			? true
			: false;
	type _define_route_hmr_optional = undefined extends ToDefineRouteArgs<
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
			"submit" | "submitOrThrow" | "toIdentityArray"
		>
	>;
	type _api_client_submit_return = Assert<
		IsExact<
			Awaited<ReturnType<ToAPIClient<App>["submit"]>>,
			SubmitResult<
				| ToActionOutput<App, "GET", "/users/:userID">
				| ToActionOutput<App, "GET", "/health">
				| ToActionOutput<App, "PATCH", "/users/:userID">
				| ToActionOutput<App, "POST", "/users/:userID">
				| ToActionOutput<App, "POST", "/sessions">
				| ToActionOutput<App, "POST", "/logout">
			>
		>
	>;
	type _api_client_submit_or_throw_return = Assert<
		IsExact<
			Awaited<ReturnType<ToAPIClient<App>["submitOrThrow"]>>,
			| ToActionOutput<App, "GET", "/users/:userID">
			| ToActionOutput<App, "GET", "/health">
			| ToActionOutput<App, "PATCH", "/users/:userID">
			| ToActionOutput<App, "POST", "/users/:userID">
			| ToActionOutput<App, "POST", "/sessions">
			| ToActionOutput<App, "POST", "/logout">
		>
	>;
	expect_type<
		SubmitError<
			ToActionSubmitOutput<App, ToActionSubmitArgsByKind<App, "query">>
		>
	>(
		null as unknown as ToActionSubmitError<
			App,
			ToActionSubmitArgsByKind<App, "query">
		>,
	);
	expect_type<
		ToActionSubmitError<App, ToActionSubmitArgsByKind<App, "query">>
	>(
		null as unknown as SubmitError<
			ToActionSubmitOutput<App, ToActionSubmitArgsByKind<App, "query">>
		>,
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
		submissions: [
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
	expect_type<string>(work_state.submissions[0]!.key);

	// SubmitResult
	const success_result: SubmitResult<number> = {
		success: true,
		data: 42,
		response: new Response(),
		revalidationPromise: Promise.resolve({ ok: true }),
	};
	const fail_result: SubmitResult<number> = {
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
	const submit_error = new SubmitError<number>(fail_result);
	expect_type<SubmitResult<number> & { success: false }>(submit_error.result);
	void submit_error;

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
	expect_type<"init" | "navigation" | "popstate" | "revalidation">(
		route_update_reason,
	);

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
		kind: "mutation",
	});
	expect_type<unknown[]>(health_identity);

	// @ts-expect-error identity arrays use the same action identity typing.
	void react.apiClient.toIdentityArray({
		pattern: "/logout",
	});

	// Valid GET action with required params and input. Method is required
	// because this pattern has multiple action methods.
	const user_get_result = react.apiClient.submit({
		method: "GET",
		pattern: "/users/:userID",
		params: { userID: "u-1" },
		input: { includePosts: true },
	});
	expect_type<
		Promise<SubmitResult<ToActionOutput<App, "GET", "/users/:userID">>>
	>(user_get_result);

	// Valid GET-only action with nullable input. Method can be omitted.
	void react.apiClient.submit({
		pattern: "/health",
		kind: "mutation",
	});

	// Valid GET-only action with nullable input (explicit null).
	void react.apiClient.submit({
		pattern: "/health",
		kind: "mutation",
		input: null,
	});

	// Valid GET-only action with flattened submit options.
	void react.apiClient.submit({
		pattern: "/health",
		kind: "mutation",
		dedupeKey: "health-check",
		revalidate: false,
	});

	// @ts-expect-error method is required for patterns with multiple action methods.
	void react.apiClient.submit({
		pattern: "/users/:userID",
		params: { userID: "u-1" },
		input: { includePosts: true },
	});

	// @ts-expect-error object action inputs are required when input type is non-empty.
	void react.apiClient.submit({
		method: "GET",
		pattern: "/users/:userID",
		params: { userID: "u-1" },
	});

	void react.apiClient.submit({
		method: "GET",
		pattern: "/users/:userID",
		// @ts-expect-error action params must match route parameter names.
		params: { id: "u-1" },
		input: { includePosts: true },
	});

	void react.apiClient.submit({
		pattern: "/health",
		kind: "mutation",
		// @ts-expect-error action input root must be object, null, or undefined.
		input: "invalid",
	});

	void react.apiClient.submit({
		method: "GET",
		// @ts-expect-error action pattern must come from action patterns.
		pattern: "/docs/*",
		splatValues: ["x"],
		// @ts-expect-error action input must match one of the action inputs.
		input: {},
	});

	// Valid PATCH action with required params and input.
	const patch_result = react.apiClient.submit({
		method: "PATCH",
		pattern: "/users/:userID",
		params: { userID: "u-1" },
		input: { nickname: "neo" },
	});
	expect_type<
		Promise<SubmitResult<ToActionOutput<App, "PATCH", "/users/:userID">>>
	>(patch_result);

	// Valid same-pattern POST action with different input and output.
	const user_post_result = react.apiClient.submit({
		method: "POST",
		pattern: "/users/:userID",
		params: { userID: "u-1" },
		input: { inviteEmail: "ada@example.com" },
	});
	expect_type<
		Promise<SubmitResult<ToActionOutput<App, "POST", "/users/:userID">>>
	>(user_post_result);

	// Valid POST action.
	const post_result = react.apiClient.submit({
		method: "POST",
		pattern: "/sessions",
		kind: "query",
		input: { email: "a@b.com", password: "pw" },
	});
	expect_type<
		Promise<SubmitResult<ToActionOutput<App, "POST", "/sessions">>>
	>(post_result);

	// Valid POST action with optional input (omitted).
	void react.apiClient.submit({
		method: "POST",
		pattern: "/logout",
	});

	// @ts-expect-error method is required for non-GET actions.
	void react.apiClient.submit({
		pattern: "/logout",
	});

	void react.apiClient.submit({
		method: "POST",
		pattern: "/users/:userID",
		params: { userID: "u-1" },
		// @ts-expect-error method/pattern identity selects POST input.
		input: { nickname: "neo" },
	});

	// @ts-expect-error non-empty action input is required.
	void react.apiClient.submit({
		method: "POST",
		pattern: "/sessions",
	});

	void react.apiClient.submit({
		// @ts-expect-error method must come from action methods.
		method: "PUT",
		pattern: "/sessions",
		kind: "query",
		input: { email: "a@b.com", password: "pw" },
	});

	void react.apiClient.submit({
		method: "POST",
		pattern: "/health",
		// @ts-expect-error method/pattern pair must exist.
		input: {},
	});
}
void assert_api_client_contracts;

/////// Action Submit Props Type Safety

function assert_action_submit_props_contracts(): void {
	// Required GET input.
	const required_get_props: ToActionSubmitArgs<App> = {
		method: "GET",
		pattern: "/users/:userID",
		params: { userID: "u-1" },
		input: { includePosts: true },
	};
	expect_type<{ includePosts: boolean }>(required_get_props.input);

	// Optional GET input (omitted, null, undefined).
	const optional_get_omitted: ToActionSubmitArgs<App> = {
		pattern: "/health",
		kind: "mutation",
	};
	const optional_get_null: ToActionSubmitArgs<App> = {
		pattern: "/health",
		kind: "mutation",
		input: null,
	};
	const optional_get_undefined: ToActionSubmitArgs<App> = {
		pattern: "/health",
		kind: "mutation",
		input: undefined,
	};
	void optional_get_omitted;
	void optional_get_null;
	void optional_get_undefined;

	// @ts-expect-error non-empty GET input must be required.
	const missing_get_input: ToActionSubmitArgs<App> = {
		method: "GET",
		pattern: "/users/:userID",
		params: { userID: "u-1" },
	};
	void missing_get_input;

	const wrong_get_params: ToActionSubmitArgs<App> = {
		method: "GET",
		pattern: "/users/:userID",
		// @ts-expect-error action params keys must match route params.
		params: { id: "u-1" },
		input: { includePosts: true },
	};
	void wrong_get_params;

	// Required PATCH action.
	const required_patch: ToActionSubmitArgs<App> = {
		method: "PATCH",
		pattern: "/users/:userID",
		params: { userID: "u-1" },
		input: { nickname: "neo" },
	};
	expect_type<{ nickname: string }>(required_patch.input);

	// Required same-pattern POST action.
	const required_user_post: ToActionSubmitArgs<App> = {
		method: "POST",
		pattern: "/users/:userID",
		params: { userID: "u-1" },
		input: { inviteEmail: "ada@example.com" },
	};
	expect_type<{ inviteEmail: string }>(required_user_post.input);

	// Required POST action.
	const required_post: ToActionSubmitArgs<App> = {
		method: "POST",
		pattern: "/sessions",
		kind: "query",
		input: { email: "a@b.com", password: "pw" },
	};
	void required_post;

	// Optional POST input (omitted, undefined).
	const optional_post_omitted: ToActionSubmitArgs<App> = {
		method: "POST",
		pattern: "/logout",
	};
	const optional_post_undefined: ToActionSubmitArgs<App> = {
		method: "POST",
		pattern: "/logout",
		input: undefined,
	};
	const query_get_props: ToActionSubmitArgsByKind<App, "query"> = {
		method: "GET",
		pattern: "/users/:userID",
		params: { userID: "u-1" },
		input: { includePosts: true },
	};
	const query_post_props: ToActionSubmitArgsByKind<App, "query"> = {
		method: "POST",
		pattern: "/sessions",
		kind: "query",
		input: { email: "a@b.com", password: "pw" },
	};
	const mutation_get_props: ToActionSubmitArgsByKind<App, "mutation"> = {
		pattern: "/health",
		kind: "mutation",
	};
	void optional_post_omitted;
	void optional_post_undefined;
	void query_get_props;
	void query_post_props;
	void mutation_get_props;

	// @ts-expect-error non-empty POST input must be required.
	const missing_post_input: ToActionSubmitArgs<App> = {
		method: "POST",
		pattern: "/sessions",
		kind: "query",
	};
	void missing_post_input;

	const wrong_patch_input: ToActionSubmitArgs<App> = {
		method: "PATCH",
		pattern: "/users/:userID",
		params: { userID: "u-1" },
		// @ts-expect-error method/pattern identity selects PATCH input.
		input: { inviteEmail: "ada@example.com" },
	};
	void wrong_patch_input;

	const wrong_post_method: ToActionSubmitArgs<App> = {
		// @ts-expect-error method/pattern pair must exist.
		method: "PUT",
		pattern: "/sessions",
		kind: "query",
		input: { email: "a@b.com", password: "pw" },
	};
	void wrong_post_method;

	const wrong_action_params: ToActionSubmitArgs<App> = {
		method: "PATCH",
		pattern: "/users/:userID",
		// @ts-expect-error action params keys must match route params.
		params: { id: "u-1" },
		input: { nickname: "neo" },
	};
	void wrong_action_params;

	const body_not_allowed: ToActionSubmitArgs<App> = {
		method: "POST",
		pattern: "/logout",
		// @ts-expect-error typed action submit uses input, not body.
		body: "raw",
	};
	void body_not_allowed;
}
void assert_action_submit_props_contracts;

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
			expect_type<ToLoaderOutput<App, "/docs/*">>(data);
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
			expect_type<"init" | "navigation" | "revalidation" | "prefetch">(
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
			expect_type<Exclude<RouteUpdateReason, "init">>(trigger);
			expect_type<AbortSignal>(signal);
			expect_type<string>(current.href);
			expect_type<string>(next.href);
		},
		beforeRouteYield: async ({ current, next, signal, trigger }) => {
			expect_type<Exclude<RouteUpdateReason, "init">>(trigger);
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
	// init
	const init_promise = react.init({
		render: async ({ App, el }) => {
			expect_type<ReactComponentType>(App);
			expect_type<HTMLElement>(el);
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
		onClientBuildIDChange: (prev, next) => {
			expect_type<string>(prev);
			expect_type<string>(next);
		},
	});
	expect_type<Promise<Result<void>>>(init_promise);

	const preact_init_promise = preact.init({
		render: async ({ App, el }) => {
			expect_type<PreactComponentType>(App);
			expect_type<HTMLElement>(el);
		},
	});
	expect_type<Promise<Result<void>>>(preact_init_promise);

	const solid_init_promise = solid.init({
		render: async ({ App, el }) => {
			expect_type<SolidComponent>(App);
			expect_type<HTMLElement>(el);
		},
	});
	expect_type<Promise<Result<void>>>(solid_init_promise);

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
	expect_type<WorkState["submissions"]>(work.submissions);

	// revalidateOnWindowFocus
	void react.init({
		revalidateOnWindowFocus: true,
	});
	void react.init({
		revalidateOnWindowFocus: false,
	});
	void react.init({
		revalidateOnWindowFocus: { staleTimeMS: 3000 },
	});

	// progressIndicator (all categories)
	const init_with_progress = react.init({
		progressIndicator: {
			start: () => {},
			stop: () => {},
			isRunning: () => {
				return false;
			},
			include: ["navigations", "submissions", "revalidations"],
			startDelayMS: 30,
			stopDelayMS: 40,
		},
	});
	expect_type<Promise<Result<void>>>(init_with_progress);

	// progressIndicator (include "all")
	void react.init({
		progressIndicator: {
			start: () => {},
			stop: () => {},
			isRunning: () => {
				return false;
			},
			include: "all",
		},
	});

	// progressIndicator (subset)
	void react.init({
		progressIndicator: {
			start: () => {},
			stop: () => {},
			isRunning: () => {
				return false;
			},
			include: ["navigations"],
		},
	});
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
		return work.submissions.length;
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

	// defineRoute
	void preact.defineRoute({
		pattern: "/users/:userID",
		component: (props) => {
			const data = preact.useLoaderData(props);
			expect_type<ReadonlySignal<ToLoaderOutput<App, "/users/:userID">>>(
				data,
			);
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
	void preact.Link({ pattern: "/users/:userID" });
}
void assert_preact_adapter_contracts;

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
		return work.submissions.length;
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

	// defineRoute
	void solid.defineRoute({
		pattern: "/users/:userID",
		component: (props) => {
			const data = solid.useLoaderData(props);
			expect_type<Accessor<ToLoaderOutput<App, "/users/:userID">>>(data);
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
	void solid.Link({ pattern: "/users/:userID" });
}
void assert_solid_adapter_contracts;
