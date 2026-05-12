import { describe, expect, it } from "vitest";
import type { BrowserKey, Effect, PublicCallID, RouteToken } from "./events.ts";
import type {
	ActiveRouteIntent,
	BootingModel,
	BrowserPosition,
	CurrentRoute,
	NavigationIntent,
	PreparedRoute,
	ReadyModel,
	RefreshDemand,
	RoutePayload,
	WorkIndicatorOptions,
} from "./model.ts";
import { client_loader_id_for } from "./model.ts";
import type { AbortHandle } from "./platform.ts";
import {
	render_plan_from_prepared,
	route_state_from_prepared,
} from "./reducer-core.ts";
import {
	apply_hmr_route_update,
	begin_api_submission,
	begin_boot,
	build_publish_effect,
	commit_publication,
	fail_publication,
	settle_api_submission,
	settle_route_preparation,
	settle_route_response,
	supersede_active_route,
} from "./transitions.ts";

const client_build_id = "client-build";
const server_build_id = "server-build";
const deployment_id = "deployment-id";
const base_href = "https://example.test/";
const next_href = "https://example.test/next";
const api_href = "https://example.test/api";
const base_key = "history-base" as BrowserKey;
const next_key = "history-next" as BrowserKey;
const public_call_id = "public-call" as PublicCallID;
const route_token = "route-token" as RouteToken;
const abort_handle = 1 as unknown as AbortHandle;
const client_loader_id = client_loader_id_for(route_token, 0);

function make_prepared_route(href = base_href): PreparedRoute {
	return {
		matches: [
			{
				pattern: "/",
				input: { href },
				module_url: "/route.js",
				loader_data: { href },
				client_loader_data: { client: href },
				client_loader_id,
			},
		],
		params: {},
		splat_values: [],
		error: null,
		client_build_id,
		title: "Title",
		meta_head_els: [],
		rest_head_els: [],
		css_bundles: [],
		deps: [],
	};
}

function make_browser(href = base_href, key = base_key): BrowserPosition {
	return {
		href,
		key,
		state: { href },
	};
}

function make_current(position = make_browser()): CurrentRoute {
	const prepared = make_prepared_route(position.href);
	return {
		position,
		prepared,
		route_state: route_state_from_prepared(
			prepared,
			position.href,
			position.state,
		),
		sequence: 1,
	};
}

function make_ready_model(): ReadyModel {
	const browser = make_browser();
	return {
		phase: "ready",
		config: {
			client_build_id,
			deployment_id,
			use_view_transitions: false,
			revalidate_on_focus: null,
			work_indicator: null,
		},
		browser,
		current: make_current(browser),
		active_route: null,
		prefetch: null,
		refresh: { kind: "idle" },
		submissions: {},
		submissions_by_dedupe: {},
		deferred_api_redirect: null,
		counters: {
			sequence: 1,
			route_token: 1,
			api_token: 1,
			public_call: 1,
			browser_key: 1,
			refresh_timer: 1,
		},
		activity: { last_activity_ms: 0 },
		work_indicator: { should_be_active: false },
	};
}

function make_navigation_intent(
	input?: Partial<NavigationIntent>,
): ActiveRouteIntent {
	return {
		kind: "navigation",
		nav: {
			href: input?.href ?? next_href,
			replace: input?.replace ?? false,
			scroll_to_top: input?.scroll_to_top ?? true,
			state: input?.state ?? { href: next_href },
			skip_work_indicator: input?.skip_work_indicator ?? false,
			source: input?.source ?? "navigate",
			is_popstate: input?.is_popstate ?? false,
			is_initial: input?.is_initial ?? false,
			popstate_restored_scroll: input?.popstate_restored_scroll,
			browser_key: input?.browser_key ?? next_key,
		},
		public_calls: [],
	};
}

function make_refresh_demand(): RefreshDemand {
	return {
		after_sequence: 1,
		reason: "manual",
		skip_work_indicator: false,
		waiters: [],
	};
}

function make_boot_payload(): RoutePayload & {
	server_build_id: string;
	deployment_id: string;
} {
	return {
		routes: [
			{
				pattern: "/",
				input: null,
				module_url: "/route.js",
				loader_data: { boot: true },
				server_error: undefined,
			},
		],
		params: {},
		splat_values: [],
		title: undefined,
		meta_head_els: [],
		rest_head_els: [],
		css_bundles: [],
		deps: [],
		server_build_id: client_build_id,
		deployment_id,
	};
}

function find_effect<Type extends Effect["type"]>(
	effects: readonly Effect[],
	type: Type,
): Extract<Effect, { type: Type }> | undefined {
	return effects.find((effect): effect is Extract<Effect, { type: Type }> => {
		return effect.type === type;
	});
}

function expect_publish_route_effect(
	effect: Effect,
): Extract<Effect, { type: "publish_route" }> {
	if (effect.type !== "publish_route") {
		throw new Error("Expected publish route effect");
	}
	return effect;
}

function expect_boot_active(
	model: BootingModel,
): NonNullable<BootingModel["active_route"]> {
	if (!model.active_route) {
		throw new Error("Expected boot to have an active route");
	}
	return model.active_route;
}

/////////////////////////////////////////////////////////////////////
/////// Model Purity Laws
/////////////////////////////////////////////////////////////////////

describe("core5 model purity laws", () => {
	it("derives client loader identity from route token and match index", () => {
		const other_token = "other-route-token" as RouteToken;

		expect(client_loader_id_for(route_token, 0)).toBe(
			client_loader_id_for(route_token, 0),
		);
		expect(client_loader_id_for(route_token, 0)).not.toBe(
			client_loader_id_for(route_token, 1),
		);
		expect(client_loader_id_for(route_token, 0)).not.toBe(
			client_loader_id_for(other_token, 0),
		);
	});

	it("keeps imported modules out of prepared routes and render plans", () => {
		const prepared = make_prepared_route();
		const match = prepared.matches[0]!;
		const render_plan = render_plan_from_prepared(prepared, { user: true });
		const render_entry = render_plan.entries[0]!;

		// @ts-expect-error Prepared route matches are logical facts only.
		match.module;
		// @ts-expect-error Render plans are materialized by the runtime.
		render_entry.module;

		expect("module" in match).toBe(false);
		expect("module" in render_entry).toBe(false);
	});

	it("keeps work indicator callbacks out of model config", () => {
		const model = make_ready_model();
		const callbacks: WorkIndicatorOptions = {
			start: () => {},
			stop: () => {},
			skipNavigations: true,
		};

		model.config.work_indicator = {
			skipNavigations: callbacks.skipNavigations,
		};

		// @ts-expect-error Callback ownership belongs to the runtime.
		model.config.work_indicator.start;

		expect(model.config.work_indicator).toEqual({
			skipNavigations: true,
		});
	});
});

/////////////////////////////////////////////////////////////////////
/////// Publication Laws
/////////////////////////////////////////////////////////////////////

describe("core5 publication laws", () => {
	it("builds logical publish effects with browser-keyed history actions", () => {
		const model = make_ready_model();
		const prepared = make_prepared_route(next_href);
		const effect = expect_publish_route_effect(
			build_publish_effect(
				model,
				route_token,
				abort_handle,
				prepared,
				next_href,
				{ href: next_href },
				make_navigation_intent(),
			),
		);
		const route_render = effect.commit.route_render;

		expect(effect.abort_handle).toBe(abort_handle);
		expect(effect.history_action).toEqual({
			kind: "push",
			href: next_href,
			state: { href: next_href },
			browser_key: next_key,
		});
		expect(route_render?.state.entries[0]).toEqual({
			pattern: "/",
			input: { href: next_href },
			module_url: "/route.js",
			loader_data: { href: next_href },
			client_loader_data: { client: next_href },
		});
	});

	it("does not write browser history for initial publication", () => {
		const effect = expect_publish_route_effect(
			build_publish_effect(
				make_ready_model(),
				route_token,
				abort_handle,
				make_prepared_route(base_href),
				base_href,
				undefined,
				make_navigation_intent({
					href: base_href,
					state: undefined,
					is_initial: true,
					browser_key: base_key,
				}),
			),
		);

		expect(effect.history_action).toEqual({ kind: "none" });
	});

	it("does not notify route updates when publication preserves route state", () => {
		const model = make_ready_model();
		const active_route = {
			phase: "publishing" as const,
			token: route_token,
			abort_handle,
			url: base_href,
			intent: {
				kind: "revalidation" as const,
				reval: {
					attempt: 1,
					reason: "manual" as const,
					skip_work_indicator: false,
				},
			},
			redirect_count: 0,
			sequence: model.current.sequence,
			prepared: model.current.prepared,
		};
		const transition = commit_publication(
			{
				...model,
				active_route,
				refresh: {
					kind: "running",
					demand: make_refresh_demand(),
					attempt: 1,
					route_token,
				},
			},
			{ token: route_token, now_ms: 1 },
		);

		expect(transition.model.phase).toBe("ready");
		if (transition.model.phase === "ready") {
			expect(transition.model.refresh.kind).toBe("idle");
		}
	});
});

/////////////////////////////////////////////////////////////////////
/////// HMR Laws
/////////////////////////////////////////////////////////////////////

describe("core5 HMR laws", () => {
	it("updates current route data and emits a render commit", () => {
		const model = make_ready_model();
		const client_loader_data = { client: "hmr" };
		const transition = apply_hmr_route_update(model, {
			type: "hmr_route_update",
			route_sequence: model.current.sequence,
			match_idx: 0,
			module_url: "/route.js",
			client_loader_data,
		});
		const emit = find_effect(transition.effects, "emit_client_commit");

		expect(transition.model.current.prepared.matches[0]).toMatchObject({
			client_loader_data,
		});
		expect(transition.model.current.route_state.matches[0]).toMatchObject({
			clientLoaderData: client_loader_data,
		});
		expect(emit?.commit.route_render?.state.entries[0]).toMatchObject({
			client_loader_data,
		});
		expect(emit?.commit.route_update?.route.matches[0]).toMatchObject({
			clientLoaderData: client_loader_data,
		});
	});

	it("emits a render commit without route update when route state is unchanged", () => {
		const model = make_ready_model();
		const client_loader_data =
			model.current.prepared.matches[0]!.client_loader_data;
		const transition = apply_hmr_route_update(model, {
			type: "hmr_route_update",
			route_sequence: model.current.sequence,
			match_idx: 0,
			module_url: "/route.js",
			client_loader_data,
		});
		const emit = find_effect(transition.effects, "emit_client_commit");

		expect(emit?.commit.route_render?.state.entries[0]).toMatchObject({
			client_loader_data,
		});
		expect(emit?.commit.route_update).toBe(undefined);
	});

	it("ignores stale HMR updates", () => {
		const model = make_ready_model();
		const transition = apply_hmr_route_update(model, {
			type: "hmr_route_update",
			route_sequence: model.current.sequence - 1,
			match_idx: 0,
			module_url: "/route.js",
			client_loader_data: { client: "stale" },
		});

		expect(transition.model).toBe(model);
		expect(transition.effects).toEqual([]);
	});
});

/////////////////////////////////////////////////////////////////////
/////// Ownership Laws
/////////////////////////////////////////////////////////////////////

describe("core5 ownership laws", () => {
	it("keeps superseded refresh waiters until a fresh publication settles them", () => {
		const model = make_ready_model();
		const demand: RefreshDemand = {
			...make_refresh_demand(),
			waiters: [{ call_id: public_call_id }],
		};
		const transition = supersede_active_route({
			...model,
			active_route: {
				phase: "fetching",
				token: route_token,
				abort_handle,
				url: base_href,
				intent: {
					kind: "revalidation",
					reval: {
						attempt: 2,
						reason: "manual",
						skip_work_indicator: false,
					},
				},
				redirect_count: 0,
				sequence: model.current.sequence + 1,
			},
			refresh: {
				kind: "running",
				demand,
				attempt: 2,
				route_token,
			},
		});

		expect(transition.model.active_route).toBe(null);
		expect(transition.model.refresh).toEqual({
			kind: "pending",
			demand,
			attempt: 2,
		});
		expect(find_effect(transition.effects, "abort")).toEqual({
			type: "abort",
			handle: abort_handle,
		});
		expect(find_effect(transition.effects, "settle_revalidate_call")).toBe(
			undefined,
		);

		const committed = commit_publication(
			{
				...transition.model,
				active_route: {
					phase: "publishing",
					token: route_token,
					abort_handle,
					url: next_href,
					intent: make_navigation_intent({ href: next_href }),
					redirect_count: 0,
					sequence: demand.after_sequence,
					prepared: make_prepared_route(next_href),
				},
			},
			{ token: route_token, now_ms: 1 },
		);

		expect(committed.model.refresh).toEqual({ kind: "idle" });
		expect(
			find_effect(committed.effects, "settle_revalidate_call"),
		).toEqual({
			type: "settle_revalidate_call",
			call_id: public_call_id,
			result: { ok: true },
		});
	});
});

/////////////////////////////////////////////////////////////////////
/////// Build Skew Laws
/////////////////////////////////////////////////////////////////////

describe("core5 build skew laws", () => {
	it("hard redirects navigation on explicit route build skew and notifies", () => {
		const model = make_ready_model();
		const transition = settle_route_response(
			{
				...model,
				active_route: {
					phase: "fetching",
					token: route_token,
					abort_handle,
					url: next_href,
					intent: make_navigation_intent(),
					redirect_count: 0,
					sequence: model.current.sequence + 1,
				},
			},
			{
				token: route_token,
				outcome: {
					kind: "build_skew",
					server_build_id,
					redirect: null,
					status: 409,
					ok: false,
				},
			},
			() => abort_handle,
		);

		expect(find_effect(transition.effects, "notify_build_skew")).toEqual({
			type: "notify_build_skew",
			event: {
				activeClientBuildID: client_build_id,
				serverBuildID: server_build_id,
				currentRouteState: model.current.route_state,
				currentWorkState: {
					navigation: {
						href: next_href,
						replace: false,
						source: "navigate",
					},
					revalidation: null,
					prefetch: null,
					apiRequests: [],
				},
				triggeringResponse: {
					kind: "route",
					trigger: "navigation",
					requestedHref: next_href,
					status: 409,
					ok: false,
				},
			},
		});
		expect(find_effect(transition.effects, "hard_redirect")).toEqual({
			type: "hard_redirect",
			url: next_href,
		});
	});

	it("settles revalidation on explicit route build skew and notifies", () => {
		const model = make_ready_model();
		const demand: RefreshDemand = {
			...make_refresh_demand(),
			waiters: [{ call_id: public_call_id }],
		};
		const transition = settle_route_response(
			{
				...model,
				active_route: {
					phase: "fetching",
					token: route_token,
					abort_handle,
					url: base_href,
					intent: {
						kind: "revalidation",
						reval: {
							attempt: 0,
							reason: "manual",
							skip_work_indicator: false,
						},
					},
					redirect_count: 0,
					sequence: model.current.sequence + 1,
				},
				refresh: {
					kind: "running",
					demand,
					attempt: 0,
					route_token,
				},
			},
			{
				token: route_token,
				outcome: {
					kind: "build_skew",
					server_build_id,
					redirect: null,
					status: 409,
					ok: false,
				},
			},
			() => abort_handle,
		);

		expect(find_effect(transition.effects, "notify_build_skew")).toEqual({
			type: "notify_build_skew",
			event: {
				activeClientBuildID: client_build_id,
				serverBuildID: server_build_id,
				currentRouteState: model.current.route_state,
				currentWorkState: {
					navigation: null,
					revalidation: { status: "running", attempt: 0 },
					prefetch: null,
					apiRequests: [],
				},
				triggeringResponse: {
					kind: "route",
					trigger: "revalidation",
					revalidationReason: "manual",
					requestedHref: base_href,
					status: 409,
					ok: false,
				},
			},
		});
		expect(
			find_effect(transition.effects, "settle_revalidate_call"),
		).toEqual({
			type: "settle_revalidate_call",
			call_id: public_call_id,
			result: { ok: false, reason: "build_skew" },
		});
		expect(transition.model.phase).toBe("ready");
		if (transition.model.phase === "ready") {
			expect(transition.model.refresh.kind).toBe("idle");
		}
	});

	it("drops mismatched failed prefetch responses and notifies", () => {
		const model = make_ready_model();
		const transition = settle_route_response(
			{
				...model,
				prefetch: {
					phase: "fetching",
					token: route_token,
					abort_handle,
					url: next_href,
				},
			},
			{
				token: route_token,
				outcome: {
					kind: "http_error",
					status: 500,
					server_build_id,
				},
			},
			() => abort_handle,
		);

		expect(find_effect(transition.effects, "notify_build_skew")).toEqual({
			type: "notify_build_skew",
			event: {
				activeClientBuildID: client_build_id,
				serverBuildID: server_build_id,
				currentRouteState: model.current.route_state,
				currentWorkState: {
					navigation: null,
					revalidation: null,
					prefetch: { href: next_href },
					apiRequests: [],
				},
				triggeringResponse: {
					kind: "route",
					trigger: "prefetch",
					requestedHref: next_href,
					status: 500,
					ok: false,
				},
			},
		});
		expect(transition.model.phase).toBe("ready");
		if (transition.model.phase === "ready") {
			expect(transition.model.prefetch).toBe(null);
		}
	});

	it("notifies API build skew without changing successful submission semantics", () => {
		const model = make_ready_model();
		const started = begin_api_submission(
			model,
			{
				href: api_href,
				method: "POST",
				route_kind: "mutation",
				dedupe_key: undefined,
				should_revalidate: false,
				skip_work_indicator: false,
				call_id: public_call_id,
				request_init: undefined,
			},
			abort_handle,
		);
		const fetch_effect = find_effect(started.effects, "fetch_api");
		if (!fetch_effect) {
			throw new Error("Expected fetch API effect");
		}
		const transition = settle_api_submission(started.model, {
			token: fetch_effect.token,
			outcome: {
				kind: "ok",
				data: { ok: true },
				server_build_id,
				redirect: null,
				status: 200,
			},
		});

		expect(find_effect(transition.effects, "notify_build_skew")).toEqual({
			type: "notify_build_skew",
			event: {
				activeClientBuildID: client_build_id,
				serverBuildID: server_build_id,
				currentRouteState: model.current.route_state,
				currentWorkState: {
					navigation: null,
					revalidation: null,
					prefetch: null,
					apiRequests: [
						{
							key: fetch_effect.token,
							method: "POST",
							href: api_href,
						},
					],
				},
				triggeringResponse: {
					kind: "apiRoute",
					apiRouteKind: "mutation",
					requestedHref: api_href,
					method: "POST",
					status: 200,
					ok: true,
				},
			},
		});
		expect(find_effect(transition.effects, "settle_submit_call")).toEqual({
			type: "settle_submit_call",
			call_id: public_call_id,
			result: {
				success: true,
				data: { ok: true },
				revalidation_call_id: null,
			},
		});
	});
});

/////////////////////////////////////////////////////////////////////
/////// Boot Laws
/////////////////////////////////////////////////////////////////////

describe("core5 boot laws", () => {
	it("derives build and deployment identity from the boot payload", () => {
		const transition = begin_boot(
			{
				href: base_href,
				browser_key: base_key,
				browser_state: undefined,
				payload: make_boot_payload(),
				restored_scroll: { x: 1, y: 2 },
				options: {
					use_view_transitions: true,
					revalidate_on_focus: null,
					work_indicator: null,
				},
				now_ms: 10,
			},
			abort_handle,
		);

		expect(transition.model.config).toMatchObject({
			client_build_id,
			deployment_id,
			use_view_transitions: true,
		});
		expect(find_effect(transition.effects, "prepare_route")).toMatchObject({
			token: expect_boot_active(transition.model).token,
			href: base_href,
			trigger: "boot",
		});
	});

	it("settles boot explicitly when route preparation fails", () => {
		const boot = begin_boot(
			{
				href: base_href,
				browser_key: base_key,
				browser_state: undefined,
				payload: make_boot_payload(),
				restored_scroll: undefined,
				options: {
					use_view_transitions: false,
					revalidate_on_focus: null,
					work_indicator: null,
				},
				now_ms: 10,
			},
			abort_handle,
		);
		const active = expect_boot_active(boot.model);
		const transition = settle_route_preparation(boot.model, {
			token: active.token,
			outcome: { kind: "failed", error: "boot failed" },
		});

		expect(transition.model.active_route).toBe(null);
		expect(find_effect(transition.effects, "settle_boot")).toEqual({
			type: "settle_boot",
			result: { ok: false, err: "boot failed" },
		});
	});

	it("settles boot explicitly when initial publication fails", () => {
		const boot = begin_boot(
			{
				href: base_href,
				browser_key: base_key,
				browser_state: undefined,
				payload: make_boot_payload(),
				restored_scroll: undefined,
				options: {
					use_view_transitions: false,
					revalidate_on_focus: null,
					work_indicator: null,
				},
				now_ms: 10,
			},
			abort_handle,
		);
		const active = expect_boot_active(boot.model);
		const publishing_model = {
			...boot.model,
			active_route: {
				phase: "publishing" as const,
				token: active.token,
				abort_handle: active.abort_handle,
				url: active.url,
				intent: active.intent,
				redirect_count: active.redirect_count,
				sequence: active.sequence,
				prepared: make_prepared_route(base_href),
			},
		};
		const transition = fail_publication(
			publishing_model,
			active.token,
			"publish failed",
		);

		expect(transition.model.active_route).toBe(null);
		expect(find_effect(transition.effects, "settle_boot")).toEqual({
			type: "settle_boot",
			result: { ok: false, err: "publish failed" },
		});
	});
});
