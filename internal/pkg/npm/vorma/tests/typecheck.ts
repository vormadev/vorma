/// <reference types="vite/client" />

import type { ComponentType as PreactComponentType } from "preact";
import type { ComponentType as ReactComponentType } from "react";
import type { Accessor, Component as SolidComponent } from "solid-js";
import type {
	AppConfig,
	LinkPropsBase,
	MakeTypedActionInput,
	MakeTypedActionMethod,
	MakeTypedActionOutput,
	MakeTypedActionPattern,
	MakeTypedActionSubmitProps,
	MakeTypedAPIClient,
	MakeTypedAPIDecorator,
	MakeTypedAPIDecoratorContext,
	MakeTypedClientLoaderProps,
	MakeTypedDefineRouteInput,
	MakeTypedLinkProps,
	MakeTypedLoaderInput,
	MakeTypedLoaderOutput,
	MakeTypedLoaderPattern,
	MakeTypedNavProps,
	MakeTypedNavTarget,
	MakeTypedRouteDestination,
	MakeTypedRouteProps,
	ProgressIndicatorConfig,
	RevalidationResult,
	RouteDefinition,
	RouteErrorState,
	RouteRenderEntry,
	RouteRenderState,
	RouteState,
	RouteUpdateReason,
	ScrollState,
	SubmitOptions,
	SubmitResult,
	WorkState,
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
			MakeTypedLoaderPattern<App>,
			"/" | "/users" | "/users/:userID" | "/docs/*" | "/blog/_index"
		>
	>;

	// Action methods and patterns
	type _action_methods = Assert<
		IsExact<MakeTypedActionMethod<App>, "GET" | "PATCH" | "POST">
	>;
	type _action_patterns = Assert<
		IsExact<
			MakeTypedActionPattern<App>,
			"/users/:userID" | "/health" | "/sessions" | "/logout"
		>
	>;
	type _get_action_patterns = Assert<
		IsExact<
			MakeTypedActionPattern<App, "GET">,
			"/users/:userID" | "/health"
		>
	>;
	type _post_action_patterns = Assert<
		IsExact<
			MakeTypedActionPattern<App, "POST">,
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
		IsExact<MakeTypedLoaderOutput<App, "/users">, { userCount: number }>
	>;
	type _loader_o_user_detail = Assert<
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

	// Loader input types
	type _loader_i_root = Assert<
		IsExact<MakeTypedLoaderInput<App, "/">, Record<never, never>>
	>;
	type _loader_i_users = Assert<
		IsExact<
			MakeTypedLoaderInput<App, "/users">,
			{ sort?: "name" | "created" }
		>
	>;
	type _loader_i_user_detail = Assert<
		IsExact<
			MakeTypedLoaderInput<App, "/users/:userID">,
			{ tab?: string; page?: number }
		>
	>;

	// Action I/O types
	type _get_i_users = Assert<
		IsExact<
			MakeTypedActionInput<App, "GET", "/users/:userID">,
			{ includePosts: boolean }
		>
	>;
	type _get_o_users = Assert<
		IsExact<
			MakeTypedActionOutput<App, "GET", "/users/:userID">,
			{ id: string; posts: number }
		>
	>;
	type _get_i_health = Assert<
		IsExact<MakeTypedActionInput<App, "GET", "/health">, null>
	>;
	type _get_o_health = Assert<
		IsExact<MakeTypedActionOutput<App, "GET", "/health">, { ok: true }>
	>;
	type _patch_i_users = Assert<
		IsExact<
			MakeTypedActionInput<App, "PATCH", "/users/:userID">,
			{ nickname: string }
		>
	>;
	type _patch_o_users = Assert<
		IsExact<
			MakeTypedActionOutput<App, "PATCH", "/users/:userID">,
			{ saved: true }
		>
	>;
	type _post_i_users = Assert<
		IsExact<
			MakeTypedActionInput<App, "POST", "/users/:userID">,
			{ inviteEmail: string }
		>
	>;
	type _post_o_users = Assert<
		IsExact<
			MakeTypedActionOutput<App, "POST", "/users/:userID">,
			{ invited: true }
		>
	>;
	type _post_i_sessions = Assert<
		IsExact<
			MakeTypedActionInput<App, "POST", "/sessions">,
			{ email: string; password: string }
		>
	>;
	type _post_o_sessions = Assert<
		IsExact<
			MakeTypedActionOutput<App, "POST", "/sessions">,
			{ token: string }
		>
	>;
	type _post_i_logout = Assert<
		IsExact<MakeTypedActionInput<App, "POST", "/logout">, undefined>
	>;
	type _post_o_logout = Assert<
		IsExact<MakeTypedActionOutput<App, "POST", "/logout">, { done: true }>
	>;

	// Decorator context
	type _decorator_ctx_get = Assert<
		IsExact<
			Extract<
				MakeTypedAPIDecoratorContext<App>,
				{ method: "GET" }
			>["pattern"],
			"/users/:userID" | "/health"
		>
	>;
	type _decorator_ctx_post = Assert<
		IsExact<
			Extract<
				MakeTypedAPIDecoratorContext<App>,
				{ method: "POST" }
			>["pattern"],
			"/users/:userID" | "/sessions" | "/logout"
		>
	>;

	// MakeTypedRouteDestination — intersection types are not IsExact-comparable
	// to flat object types. We verify bidirectional assignability for the
	// full type and use IsExact on individual fields. Call-site tests in
	// assert_navigate_contracts prove full correctness.
	type _route_destination_users = Assert<
		MakeTypedRouteDestination<App, "/users/:userID"> extends {
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
				} extends MakeTypedRouteDestination<App, "/users/:userID">
				? true
				: false
			: false
	>;
	type _route_destination_docs = Assert<
		MakeTypedRouteDestination<App, "/docs/*"> extends {
			pattern: "/docs/*";
			splatValues: string[];
		}
			? {
					pattern: "/docs/*";
					splatValues: string[];
				} extends MakeTypedRouteDestination<App, "/docs/*">
				? true
				: false
			: false
	>;
	type _route_destination_index_shorthand = Assert<
		IsExact<
			MakeTypedRouteDestination<App, "/blog/_index">["pattern"],
			"/blog/_index" | "/blog"
		>
	>;
	type _route_destination_root = Assert<
		IsExact<MakeTypedRouteDestination<App, "/">["pattern"], "/">
	>;
	type _route_target = Assert<
		IsExact<
			Extract<
				MakeTypedNavTarget<App, "/users/:userID">,
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
				MakeTypedNavTarget<App, "/users/:userID">,
				{ pattern: unknown }
			>,
			MakeTypedRouteDestination<App, "/users/:userID">
		>
	>;
	type _navigate_props = Assert<
		IsExact<
			MakeTypedNavProps<App, "/users/:userID">,
			MakeTypedNavTarget<App, "/users/:userID"> & {
				replace?: boolean;
				scrollToTop?: boolean;
				state?: unknown;
				skipProgressIndicator?: boolean;
			}
		>
	>;

	// MakeTypedLinkProps
	type _link_props_state = Assert<
		IsExact<MakeTypedLinkProps<App, "/users/:userID">["state"], unknown>
	>;
	type _link_props_prefetch = Assert<
		IsExact<
			MakeTypedLinkProps<App, "/users/:userID">["prefetch"],
			"intent" | "none" | undefined
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
		IsExact<keyof MakeTypedAPIClient<App>, "submit">
	>;
	type _api_client_submit_return = Assert<
		IsExact<
			Awaited<ReturnType<MakeTypedAPIClient<App>["submit"]>>,
			SubmitResult<
				| MakeTypedActionOutput<App, "GET", "/users/:userID">
				| MakeTypedActionOutput<App, "GET", "/health">
				| MakeTypedActionOutput<App, "PATCH", "/users/:userID">
				| MakeTypedActionOutput<App, "POST", "/users/:userID">
				| MakeTypedActionOutput<App, "POST", "/sessions">
				| MakeTypedActionOutput<App, "POST", "/logout">
			>
		>
	>;

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
		skipProgressIndicator: false,
	};
	void submit_opts;

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

	// LinkPropsBase
	const link_props: LinkPropsBase = {
		prefetch: "intent",
		visitOnPointerDown: true,
		prefetchDelayMs: 100,
		replace: false,
		scrollToTop: true,
	};
	void link_props;

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
	// Valid GET action with required params and input. Method is required
	// because this pattern has multiple action methods.
	const user_get_result = react.apiClient.submit({
		method: "GET",
		pattern: "/users/:userID",
		params: { userID: "u-1" },
		input: { includePosts: true },
	});
	expect_type<
		Promise<
			SubmitResult<MakeTypedActionOutput<App, "GET", "/users/:userID">>
		>
	>(user_get_result);

	// Valid GET-only action with nullable input. Method can be omitted.
	void react.apiClient.submit({
		pattern: "/health",
	});

	// Valid GET-only action with nullable input (explicit null).
	void react.apiClient.submit({
		pattern: "/health",
		input: null,
	});

	// Valid GET-only action with flattened submit options.
	void react.apiClient.submit({
		pattern: "/health",
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
		Promise<
			SubmitResult<MakeTypedActionOutput<App, "PATCH", "/users/:userID">>
		>
	>(patch_result);

	// Valid same-pattern POST action with different input and output.
	const user_post_result = react.apiClient.submit({
		method: "POST",
		pattern: "/users/:userID",
		params: { userID: "u-1" },
		input: { inviteEmail: "ada@example.com" },
	});
	expect_type<
		Promise<
			SubmitResult<MakeTypedActionOutput<App, "POST", "/users/:userID">>
		>
	>(user_post_result);

	// Valid POST action.
	const post_result = react.apiClient.submit({
		method: "POST",
		pattern: "/sessions",
		input: { email: "a@b.com", password: "pw" },
	});
	expect_type<
		Promise<SubmitResult<MakeTypedActionOutput<App, "POST", "/sessions">>>
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
	const required_get_props: MakeTypedActionSubmitProps<App> = {
		method: "GET",
		pattern: "/users/:userID",
		params: { userID: "u-1" },
		input: { includePosts: true },
	};
	expect_type<{ includePosts: boolean }>(required_get_props.input);

	// Optional GET input (omitted, null, undefined).
	const optional_get_omitted: MakeTypedActionSubmitProps<App> = {
		pattern: "/health",
	};
	const optional_get_null: MakeTypedActionSubmitProps<App> = {
		pattern: "/health",
		input: null,
	};
	const optional_get_undefined: MakeTypedActionSubmitProps<App> = {
		pattern: "/health",
		input: undefined,
	};
	void optional_get_omitted;
	void optional_get_null;
	void optional_get_undefined;

	// @ts-expect-error non-empty GET input must be required.
	const missing_get_input: MakeTypedActionSubmitProps<App> = {
		method: "GET",
		pattern: "/users/:userID",
		params: { userID: "u-1" },
	};
	void missing_get_input;

	const wrong_get_params: MakeTypedActionSubmitProps<App> = {
		method: "GET",
		pattern: "/users/:userID",
		// @ts-expect-error action params keys must match route params.
		params: { id: "u-1" },
		input: { includePosts: true },
	};
	void wrong_get_params;

	// Required PATCH action.
	const required_patch: MakeTypedActionSubmitProps<App> = {
		method: "PATCH",
		pattern: "/users/:userID",
		params: { userID: "u-1" },
		input: { nickname: "neo" },
	};
	expect_type<{ nickname: string }>(required_patch.input);

	// Required same-pattern POST action.
	const required_user_post: MakeTypedActionSubmitProps<App> = {
		method: "POST",
		pattern: "/users/:userID",
		params: { userID: "u-1" },
		input: { inviteEmail: "ada@example.com" },
	};
	expect_type<{ inviteEmail: string }>(required_user_post.input);

	// Required POST action.
	const required_post: MakeTypedActionSubmitProps<App> = {
		method: "POST",
		pattern: "/sessions",
		input: { email: "a@b.com", password: "pw" },
	};
	void required_post;

	// Optional POST input (omitted, undefined).
	const optional_post_omitted: MakeTypedActionSubmitProps<App> = {
		method: "POST",
		pattern: "/logout",
	};
	const optional_post_undefined: MakeTypedActionSubmitProps<App> = {
		method: "POST",
		pattern: "/logout",
		input: undefined,
	};
	void optional_post_omitted;
	void optional_post_undefined;

	// @ts-expect-error non-empty POST input must be required.
	const missing_post_input: MakeTypedActionSubmitProps<App> = {
		method: "POST",
		pattern: "/sessions",
	};
	void missing_post_input;

	const wrong_patch_input: MakeTypedActionSubmitProps<App> = {
		method: "PATCH",
		pattern: "/users/:userID",
		params: { userID: "u-1" },
		// @ts-expect-error method/pattern identity selects PATCH input.
		input: { inviteEmail: "ada@example.com" },
	};
	void wrong_patch_input;

	const wrong_post_method: MakeTypedActionSubmitProps<App> = {
		// @ts-expect-error method/pattern pair must exist.
		method: "PUT",
		pattern: "/sessions",
		input: { email: "a@b.com", password: "pw" },
	};
	void wrong_post_method;

	const wrong_action_params: MakeTypedActionSubmitProps<App> = {
		method: "PATCH",
		pattern: "/users/:userID",
		// @ts-expect-error action params keys must match route params.
		params: { id: "u-1" },
		input: { nickname: "neo" },
	};
	void wrong_action_params;

	const body_not_allowed: MakeTypedActionSubmitProps<App> = {
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

			const client_loader_data = react.useClientLoaderData(props);
			expect_type<number>(client_loader_data);

			return props.Outlet();
		},
		clientLoader: async ({
			params,
			splatValues,
			serverDataPromise,
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

	// buildHref
	const built_href = react.buildHref({
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

	// useRouteState
	const route_state = preact.useRouteState();
	expect_type<RouteState>(route_state);
	expect_type<Record<string, string>>(route_state.params);
	expect_type<unknown>(route_state.historyState);

	const selected_params = preact.useRouteState((route) => {
		return route.params;
	});
	expect_type<Record<string, string>>(selected_params);

	// useWorkState
	const work_state = preact.useWorkState();
	expect_type<WorkState>(work_state);

	const selected_submission_count = preact.useWorkState((work) => {
		return work.submissions.length;
	});
	expect_type<number>(selected_submission_count);

	// useClientLoaderData
	const cl_route_props = null as unknown as MakeTypedRouteProps<
		App,
		"/users/:userID",
		number
	>;
	const client_loader_data = preact.useClientLoaderData(cl_route_props);
	expect_type<number>(client_loader_data);

	// usePatternClientLoaderData
	const maybe_client_loader_data =
		preact.usePatternClientLoaderData<number>("/users/:userID");
	expect_type<number | undefined>(maybe_client_loader_data);

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
	void preact.Link({ pattern: "/users/:userID" });
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

	// useClientLoaderData returns Accessor<T>
	const cl_route_props = null as unknown as MakeTypedRouteProps<
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
	void solid.Link({ pattern: "/users/:userID" });
}
void assert_solid_adapter_contracts;
