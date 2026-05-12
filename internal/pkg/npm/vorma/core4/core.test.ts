import { describe, expect, it } from "vitest";
import {
	BUILD_ID_HEADER,
	VORMA_PROTOCOL_ENABLED,
	X_CLIENT_REDIRECT,
	X_VORMA_BUILD_SKEW,
} from "../core/constants.ts";
import {
	accept_api_submission_outcome,
	accept_boot_provisional_route,
	accept_route_preparation,
	accept_route_response,
	begin_api_submission,
	begin_boot,
	begin_navigation,
	begin_pending_revalidation,
	begin_popstate,
	begin_prefetch,
	cancel_prefetch,
	classify_api_response,
	classify_route_response,
	commit_publication,
	CORE4_MAX_REDIRECTS,
	create_core4_model,
	derive_core4_work_projection,
	derive_core4_work_state,
	fail_publication,
	fire_refresh_timer,
	initialize_core4_boot,
	request_revalidation,
	settle_publication,
} from "./core.ts";
import type {
	APISubmissionOutcome,
	BootActiveRouteSlot,
	BrowserKey,
	BrowserPosition,
	Core4BootingInit,
	Core4BootingModel,
	Core4Effect,
	Core4Model,
	Core4ReadyInit,
	Core4ReadyModel,
	Core4ResponseFacts,
	Core4Token,
	Core4Transition,
	PopstateActiveRouteSlot,
	PreparedRoute,
	PublicCallID,
	RevalidationActiveRouteSlot,
	RoutePayload,
	RouteResponseOutcome,
	RouteResponseTransition,
	RouteSnapshot,
	RouteState,
	SubmissionKey,
	TimerID,
} from "./types.ts";

const client_build_id = "client-build";
const base_href = "https://example.test/";
const first_hash_href = "https://example.test/page#first";
const second_hash_href = "https://example.test/page#second";
const next_href = "https://example.test/next";
const api_href = "https://example.test/api/action";
const other_href = "https://example.test/other";
const base_key = "history-base" as BrowserKey;
const next_key = "history-next" as BrowserKey;
const retargeted_key = "history-retargeted" as BrowserKey;
const second_key = "history-second" as BrowserKey;
const first_token = "token-first" as Core4Token;
const second_token = "token-second" as Core4Token;
const third_token = "token-third" as Core4Token;
const waiter_id = "call-waiter" as PublicCallID;
const second_waiter_id = "call-second" as PublicCallID;
const timer_id = "timer-refresh" as TimerID;
const second_timer_id = "timer-second" as TimerID;
const api_key = "api-key" as SubmissionKey;
const dedupe_key = "dedupe-key";

function must<T>(value: T | undefined): T {
	if (value === undefined) {
		throw new Error("Expected transition to be defined");
	}
	return value;
}

function make_browser(href = base_href, key = base_key): BrowserPosition {
	return {
		href,
		key,
		state: { key },
	};
}

function make_route_state(href = base_href, pattern = "/"): RouteState {
	return {
		clientBuildID: client_build_id,
		error: null,
		historyState: { href },
		href,
		matches: [
			{
				clientLoaderData: undefined,
				input: null,
				loaderData: { href },
				pattern,
			},
		],
		params: {},
		splatValues: [],
	};
}

function make_route_snapshot(position = make_browser()): RouteSnapshot {
	return {
		position,
		route: make_route_state(position.href),
		sequence: 0,
	};
}

function make_route_payload(
	pattern = "/page",
	loader_data: unknown = { page: true },
): RoutePayload {
	return {
		css_bundles: [],
		deps: [],
		meta_head_els: [],
		params: {},
		rest_head_els: [],
		routes: [
			{
				input: null,
				loader_data,
				module_url: "/route.js",
				pattern,
			},
		],
		server_build_id: client_build_id,
		splat_values: [],
	};
}

function make_prepared_route(
	href = next_href,
	pattern = "/next",
): PreparedRoute {
	return {
		css_bundles: [],
		deps: [],
		render_payload: null,
		route: make_route_state(href, pattern),
	};
}

function make_ready_model(): Core4Model {
	const browser = make_browser();
	return create_core4_model({
		browser,
		client_build_id,
		current: make_route_snapshot(browser),
		phase: "ready",
	});
}

function make_response_facts(input?: {
	headers?: Record<string, string>;
	ok?: boolean;
	redirected?: boolean;
	status?: number;
	status_text?: string;
	url?: string;
}): Core4ResponseFacts {
	const headers = input?.headers ?? {};
	return {
		headers: {
			get(name: string): string | null {
				return headers[name] ?? null;
			},
		},
		ok: input?.ok ?? true,
		redirected: input?.redirected ?? false,
		status: input?.status ?? 200,
		status_text: input?.status_text ?? "OK",
		url: input?.url ?? next_href,
	};
}

function route_data_outcome(input: {
	owner_kind: Exclude<RouteResponseOutcome["owner_kind"], "stale">;
	payload?: RoutePayload;
	token: Core4Token;
}): RouteResponseOutcome {
	return {
		kind: "data",
		owner_kind: input.owner_kind,
		payload: input.payload ?? make_route_payload(),
		token: input.token,
	};
}

function api_success_outcome(input: {
	data?: unknown;
	token: Core4Token;
}): APISubmissionOutcome {
	return {
		data: input.data ?? { ok: true },
		kind: "success",
		token: input.token,
	};
}

function expect_ignored_no_op(
	transition: Core4Transition | undefined,
	model: Core4Model,
): void {
	expect(transition).toBeDefined();
	expect(transition?.kind).toBe("ignored_stale");
	expect(transition?.effects).toEqual([]);
	expect(transition?.model).toBe(model);
}

function find_effect<Type extends Core4Effect["type"]>(
	effects: readonly Core4Effect[],
	type: Type,
): Extract<Core4Effect, { type: Type }> | undefined {
	return effects.find(
		(effect): effect is Extract<Core4Effect, { type: Type }> => {
			return effect.type === type;
		},
	);
}

function prepare_navigation_for_publication(input?: {
	href?: string;
	public_call_ids?: readonly PublicCallID[];
	token?: Core4Token;
}): RouteResponseTransition {
	const token = input?.token ?? first_token;
	const started = must(
		begin_navigation(make_ready_model(), {
			browser_key: next_key,
			href: input?.href ?? next_href,
			public_call_ids: input?.public_call_ids ?? [],
			replace: false,
			skip_work_indicator: false,
			state: { next: true },
			token,
		}),
	);
	const active_route = started.model.active_route;
	if (!active_route) {
		throw new Error("Expected navigation to own active route");
	}
	const response = must(
		accept_route_response(
			started.model,
			route_data_outcome({
				owner_kind: "active_route",
				token,
			}),
		),
	);
	expect(response.kind).toBe("preparing_active_route");
	return response;
}

/////////////////////////////////////////////////////////////////////
/////// Model Phase Laws
/////////////////////////////////////////////////////////////////////

describe("core4 model phase laws", () => {
	it("creates booting models with nullable browser and current facts", () => {
		const input: Core4BootingInit = { client_build_id };
		const model: Core4BootingModel = create_core4_model(input);

		expect(model.phase).toBe("booting");
		expect(model.browser).toBeNull();
		expect(model.current).toBeNull();
	});

	it("creates ready models with non-null browser and current facts", () => {
		const browser = make_browser();
		const current = make_route_snapshot(browser);
		const input: Core4ReadyInit = {
			browser,
			client_build_id,
			current,
			phase: "ready",
		};
		const model: Core4ReadyModel = create_core4_model(input);

		expect(model.phase).toBe("ready");
		expect(model.browser.href).toBe(base_href);
		expect(model.current.position).toBe(browser);
	});

	it("rejects ready init and model shapes without browser or current at type level", () => {
		const browser = make_browser();
		const current = make_route_snapshot(browser);
		const ready_model = create_core4_model({
			browser,
			client_build_id,
			current,
			phase: "ready",
		});
		// @ts-expect-error Ready init requires a browser position.
		const _missing_browser_ready_init: Core4ReadyInit = {
			client_build_id,
			current,
			phase: "ready",
		};
		// @ts-expect-error Ready init requires a current route snapshot.
		const _missing_current_ready_init: Core4ReadyInit = {
			browser,
			client_build_id,
			phase: "ready",
		};
		// @ts-expect-error Ready model cannot have a null browser.
		const _missing_browser_ready_model: Core4Model = {
			...ready_model,
			browser: null,
		};
		// @ts-expect-error Ready model cannot have a null current route.
		const _missing_current_ready_model: Core4Model = {
			...ready_model,
			current: null,
		};

		expect(ready_model.browser).toBe(browser);
		expect(ready_model.current).toBe(current);
	});

	it("rejects owner drift between refresh and route/publication slots at type level", () => {
		const ready_model = make_ready_model();
		const demand = {
			after_sequence: 1,
			reason: "manual" as const,
			skip_work_indicator: false,
			waiters: [],
		};

		// @ts-expect-error Running refresh requires an active revalidation route.
		const _orphan_running_refresh: Core4Model = {
			...ready_model,
			active_route: null,
			publication: null,
			refresh: {
				attempt: 0,
				demand,
				kind: "running",
			},
		};
		// @ts-expect-error Settling refresh requires a committed publication.
		const _orphan_settling_refresh: Core4Model = {
			...ready_model,
			active_route: null,
			publication: null,
			refresh: {
				demand,
				kind: "settling",
			},
		};

		expect(ready_model.refresh.kind).toBe("idle");
	});

	it("initializes boot with browser facts but no current route", () => {
		const browser = make_browser();
		const initialized = must(
			initialize_core4_boot(create_core4_model({ client_build_id }), {
				browser,
				client_build_id,
				use_view_transitions: true,
			}),
		);

		expect(initialized.model.phase).toBe("booting");
		expect(initialized.model.browser).toBe(browser);
		expect(initialized.model.current).toBeNull();
		expect(initialized.model.use_view_transitions).toBe(true);
	});

	it("moves from booting to ready only through committed boot publication", () => {
		const browser = make_browser();
		const initialized = must(
			initialize_core4_boot(create_core4_model({ client_build_id }), {
				browser,
				client_build_id,
				use_view_transitions: false,
			}),
		);
		const booting = must(
			begin_boot(initialized.model, {
				browser,
				payload: make_route_payload("/", { boot: true }),
				token: first_token,
			}),
		);
		const provisional = must(
			accept_boot_provisional_route(booting.model, {
				position: browser,
				route: make_route_state(base_href),
				token: first_token,
			}),
		);
		const publishing = must(
			accept_route_preparation(provisional.model, {
				kind: "prepared",
				prepared: make_prepared_route(base_href, "/"),
				token: first_token,
			}),
		);

		expect(booting.model.phase).toBe("booting");
		expect(provisional.model.phase).toBe("booting");
		expect(publishing.model.phase).toBe("booting");
		expect(publishing.model.publication?.plan.previous).toBeNull();
		expect(publishing.model.publication?.plan.hooks).toEqual({
			kind: "none",
		});
		expect(
			settle_publication(publishing.model, first_token),
		).toBeUndefined();

		const committed = must(
			commit_publication(publishing.model, { token: first_token }),
		);
		expect(committed.model.phase).toBe("ready");
		expect(committed.model.current?.route.href).toBe(base_href);
	});

	it("rejects ready-only transition starts before the ready phase", () => {
		const booting = create_core4_model({ client_build_id });

		expect(
			request_revalidation(booting, {
				reason: "manual",
				skip_work_indicator: false,
			}),
		).toBeUndefined();
		expect(
			begin_navigation(booting, {
				browser_key: next_key,
				href: next_href,
				public_call_ids: [],
				replace: false,
				skip_work_indicator: false,
				state: undefined,
				token: first_token,
			}),
		).toBeUndefined();
		expect(
			begin_prefetch(booting, {
				href: next_href,
				token: first_token,
			}),
		).toBeUndefined();
		expect(
			begin_api_submission(booting, {
				dedupe_key: null,
				href: api_href,
				key: api_key,
				method: "POST",
				route_kind: "mutation",
				should_revalidate: true,
				skip_work_indicator: false,
				token: first_token,
			}),
		).toBeUndefined();
	});
});

/////////////////////////////////////////////////////////////////////
/////// Active Route Kind Laws
/////////////////////////////////////////////////////////////////////

describe("core4 active route kind laws", () => {
	it("rejects boot and revalidation navigation policy at type level", () => {
		// @ts-expect-error Boot routes cannot own public navigation waiters.
		const _boot_waiters: BootActiveRouteSlot["public_call_ids"] = [
			waiter_id,
		];
		// @ts-expect-error Boot routes do not expose navigation source.
		const _boot_source: BootActiveRouteSlot["source"] = "navigate";
		// @ts-expect-error Boot routes always replace the initial document.
		const _boot_replace: BootActiveRouteSlot["replace"] = false;
		// @ts-expect-error Boot routes never show work indicator state.
		const _boot_work: BootActiveRouteSlot["skip_work_indicator"] = false;
		// @ts-expect-error Revalidation routes cannot own public navigation waiters.
		const _revalidation_waiters: RevalidationActiveRouteSlot["public_call_ids"] =
			[waiter_id];
		// @ts-expect-error Revalidation routes do not expose navigation source.
		const _revalidation_source: RevalidationActiveRouteSlot["source"] =
			"navigate";
		// @ts-expect-error Revalidation routes do not own history policy.
		const _revalidation_replace: RevalidationActiveRouteSlot["replace"] = false;
		// @ts-expect-error Revalidation routes do not own navigation scroll policy.
		const _revalidation_scroll: RevalidationActiveRouteSlot["scroll_to_top"] = true;
		// @ts-expect-error Popstate routes cannot be retagged as navigate or redirect.
		const _popstate_source: PopstateActiveRouteSlot["source"] = "navigate";
		// @ts-expect-error Popstate routes never push or replace browser history.
		const _popstate_replace: PopstateActiveRouteSlot["replace"] = false;

		expect(true).toBe(true);
	});

	it("converts redirected popstate route work to navigation ownership", () => {
		const started = must(
			begin_popstate(make_ready_model(), {
				browser: make_browser(other_href, second_key),
				public_call_ids: [waiter_id],
				token: first_token,
			}),
		);
		const active_route = started.model.active_route;
		if (!active_route) {
			throw new Error("Expected popstate route to start");
		}
		expect(active_route.kind).toBe("popstate");

		const redirected = must(
			accept_route_response(started.model, {
				href: next_href,
				kind: "soft_redirect",
				owner_kind: "active_route",
				token: first_token,
			}),
		);

		expect(redirected.model.active_route).toMatchObject({
			kind: "navigation",
			replace: true,
			source: "redirect",
			token: first_token,
		});
		expect(find_effect(redirected.effects, "fetch_route")?.trigger).toBe(
			"navigation",
		);
	});

	it("settles ignored popstate calls without changing the model", () => {
		const model = make_ready_model();
		const browser = model.browser;
		if (!browser) {
			throw new Error("Expected ready model to have browser");
		}

		const transition = must(
			begin_popstate(model, {
				browser,
				public_call_ids: [waiter_id],
				restored_scroll: undefined,
				token: first_token,
			}),
		);

		expect(transition).toEqual({
			effects: [
				{
					ids: [waiter_id],
					result: { didNavigate: false },
					type: "settle_navigation_calls",
				},
			],
			kind: "ignored",
			model,
		});
	});

	it("retargets active navigation by preserving token and replacing or appending waiters by intent", () => {
		const first = must(
			begin_navigation(make_ready_model(), {
				browser_key: next_key,
				href: first_hash_href,
				public_call_ids: [waiter_id],
				replace: false,
				scroll_to_top: true,
				skip_work_indicator: false,
				state: { hash: "first" },
				token: first_token,
			}),
		);
		const changed = must(
			begin_navigation(first.model, {
				browser_key: retargeted_key,
				href: second_hash_href,
				public_call_ids: [second_waiter_id],
				replace: true,
				scroll_to_top: false,
				skip_work_indicator: true,
				state: { hash: "second" },
				token: second_token,
			}),
		);
		const same = must(
			begin_navigation(changed.model, {
				browser_key: retargeted_key,
				href: second_hash_href,
				public_call_ids: [waiter_id],
				replace: true,
				scroll_to_top: false,
				skip_work_indicator: true,
				state: changed.model.active_route?.state,
				token: third_token,
			}),
		);

		expect(changed.model.active_route).toMatchObject({
			href: second_hash_href,
			public_call_ids: [second_waiter_id],
			sequence: first.model.active_route?.sequence,
			token: first_token,
		});
		expect(changed.effects).toContainEqual({
			ids: [waiter_id],
			result: { didNavigate: false },
			type: "settle_navigation_calls",
		});
		expect(same.model.active_route?.public_call_ids).toEqual([
			second_waiter_id,
			waiter_id,
		]);
		expect(same.model.active_route?.token).toBe(first_token);
	});

	it("keeps same-document retarget during revalidation out of freshness accounting", () => {
		const model = make_ready_model();
		const requested = must(
			request_revalidation(model, {
				reason: "manual",
				skip_work_indicator: false,
				waiter_id,
			}),
		);
		const started = must(
			begin_pending_revalidation(requested.model, {
				token: first_token,
			}),
		);
		const retargeted = must(
			begin_navigation(started.model, {
				browser_key: retargeted_key,
				href: `${base_href}#section`,
				public_call_ids: [second_waiter_id],
				replace: false,
				skip_work_indicator: false,
				state: undefined,
				token: second_token,
			}),
		);

		expect(retargeted.kind).toBe("same_document");
		expect(retargeted.model.sequence).toBe(started.model.sequence);
		expect(retargeted.model.current?.sequence).toBe(
			model.current?.sequence,
		);
		expect(retargeted.model.refresh.kind).toBe("running");
		expect(retargeted.model.active_route?.kind).toBe("revalidation");
	});

	it("supersedes active route by aborting and settling only that route", () => {
		const first = must(
			begin_navigation(make_ready_model(), {
				browser_key: next_key,
				href: next_href,
				public_call_ids: [waiter_id],
				replace: false,
				skip_work_indicator: false,
				state: undefined,
				token: first_token,
			}),
		);
		const second = must(
			begin_navigation(first.model, {
				browser_key: second_key,
				href: other_href,
				public_call_ids: [second_waiter_id],
				replace: false,
				skip_work_indicator: false,
				state: undefined,
				token: second_token,
			}),
		);

		expect(second.effects).toContainEqual({
			token: first_token,
			type: "abort_route_work",
		});
		expect(second.effects).toContainEqual({
			ids: [waiter_id],
			result: { didNavigate: false },
			type: "settle_navigation_calls",
		});
		expect(second.model.active_route).toMatchObject({
			href: other_href,
			public_call_ids: [second_waiter_id],
			token: second_token,
		});
	});
});

/////////////////////////////////////////////////////////////////////
/////// Identity Laws
/////////////////////////////////////////////////////////////////////

describe("core4 identity laws", () => {
	it("rejects mixed identity classes at type level", () => {
		// @ts-expect-error Tokens are branded and cannot come from raw strings.
		const _raw_token: Core4Token = "token-raw";
		// @ts-expect-error Public call IDs are branded and cannot come from raw strings.
		const _raw_waiter: PublicCallID = "call-raw";
		// @ts-expect-error Browser keys are branded and cannot come from raw strings.
		const _raw_browser_key: BrowserKey = "browser-raw";
		// @ts-expect-error Timer IDs are branded and cannot come from raw strings.
		const _raw_timer: TimerID = "timer-raw";
		// @ts-expect-error Core tokens are not public call IDs.
		const _token_from_waiter: Core4Token = waiter_id;
		// @ts-expect-error Public call IDs are not core tokens.
		const _waiter_from_token: PublicCallID = first_token;
		// @ts-expect-error Timer IDs are not core tokens.
		const _timer_from_token: TimerID = first_token;
		// @ts-expect-error Navigation settlement effects accept public call IDs, not core tokens.
		const _navigation_settlement_token: Extract<
			Core4Effect,
			{ type: "settle_navigation_calls" }
		> = {
			ids: [first_token],
			result: { didNavigate: true },
			type: "settle_navigation_calls",
		};
		// @ts-expect-error Route abort effects accept core tokens, not public call IDs.
		const _route_abort_waiter: Extract<
			Core4Effect,
			{ type: "abort_route_work" }
		> = {
			token: waiter_id,
			type: "abort_route_work",
		};

		expect(timer_id).toBe("timer-refresh");
	});
});

/////////////////////////////////////////////////////////////////////
/////// Capability Acceptance Laws
/////////////////////////////////////////////////////////////////////

describe("core4 capability acceptance laws", () => {
	it("ignores route outcomes whose token no longer owns route work", () => {
		const first = must(
			begin_navigation(make_ready_model(), {
				browser_key: next_key,
				href: next_href,
				public_call_ids: [waiter_id],
				replace: false,
				skip_work_indicator: false,
				state: undefined,
				token: first_token,
			}),
		);
		const stale_active_route = first.model.active_route;
		if (!stale_active_route) {
			throw new Error("Expected first navigation to start");
		}
		const second = must(
			begin_navigation(first.model, {
				browser_key: second_key,
				href: other_href,
				public_call_ids: [],
				replace: false,
				skip_work_indicator: false,
				state: undefined,
				token: second_token,
			}),
		);

		expect_ignored_no_op(
			accept_route_response(
				second.model,
				route_data_outcome({
					owner_kind: "active_route",
					token: first_token,
				}),
			),
			second.model,
		);
	});

	it("ignores route outcomes when the current owner kind differs from the snapshot", () => {
		const prefetched = must(
			begin_prefetch(make_ready_model(), {
				href: next_href,
				token: first_token,
			}),
		);
		const stale_prefetch = prefetched.model.prefetch;
		if (!stale_prefetch) {
			throw new Error("Expected prefetch to start");
		}
		const promoted = must(
			begin_navigation(prefetched.model, {
				browser_key: next_key,
				href: next_href,
				public_call_ids: [waiter_id],
				replace: false,
				skip_work_indicator: false,
				state: undefined,
				token: second_token,
			}),
		);
		expect(promoted.model.active_route?.token).toBe(first_token);
		expect(promoted.model.prefetch).toBe(null);

		expect_ignored_no_op(
			accept_route_response(
				promoted.model,
				route_data_outcome({
					owner_kind: "prefetch",
					token: first_token,
				}),
			),
			promoted.model,
		);
	});

	it("ignores active-route outcomes when the live token belongs to prefetch", () => {
		const prefetched = must(
			begin_prefetch(make_ready_model(), {
				href: next_href,
				token: first_token,
			}),
		);

		expect_ignored_no_op(
			accept_route_response(
				prefetched.model,
				route_data_outcome({
					owner_kind: "active_route",
					token: first_token,
				}),
			),
			prefetched.model,
		);
	});

	it("uses current route owner facts when accepting a matching token", () => {
		const first = must(
			begin_navigation(make_ready_model(), {
				browser_key: next_key,
				href: first_hash_href,
				public_call_ids: [waiter_id],
				replace: false,
				scroll_to_top: true,
				skip_work_indicator: false,
				state: { hash: "first" },
				token: first_token,
			}),
		);
		const stale_active_route = first.model.active_route;
		if (!stale_active_route) {
			throw new Error("Expected first navigation to start");
		}
		const retargeted = must(
			begin_navigation(first.model, {
				browser_key: retargeted_key,
				href: second_hash_href,
				public_call_ids: [second_waiter_id],
				replace: true,
				scroll_to_top: false,
				skip_work_indicator: true,
				state: { hash: "second" },
				token: second_token,
			}),
		);

		const transition = must(
			accept_route_response(
				retargeted.model,
				route_data_outcome({
					owner_kind: "active_route",
					token: first_token,
				}),
			),
		);

		expect(transition.kind).toBe("preparing_active_route");
		const prepare_effect = find_effect(transition.effects, "prepare_route");
		expect(prepare_effect?.href).toBe(second_hash_href);
		expect(prepare_effect?.history_state).toEqual({ hash: "second" });
		expect(transition.model.active_route?.href).toBe(second_hash_href);
	});

	it("ignores stale route preparation and live aborted preparation", () => {
		const started = must(
			begin_navigation(make_ready_model(), {
				browser_key: next_key,
				href: next_href,
				public_call_ids: [],
				replace: false,
				skip_work_indicator: false,
				state: undefined,
				token: first_token,
			}),
		);

		expect_ignored_no_op(
			accept_route_preparation(started.model, {
				kind: "prepared",
				prepared: make_prepared_route(),
				token: second_token,
			}),
			started.model,
		);
		expect(
			accept_route_preparation(started.model, {
				kind: "aborted",
				token: first_token,
			}),
		).toBeUndefined();
	});

	it("lets prefetch preparation update only the matching prefetch token", () => {
		const prefetched = must(
			begin_prefetch(make_ready_model(), {
				href: next_href,
				token: first_token,
			}),
		);
		const prefetch = prefetched.model.prefetch;
		if (!prefetch) {
			throw new Error("Expected prefetch to start");
		}
		const preparing = must(
			accept_route_response(
				prefetched.model,
				route_data_outcome({
					owner_kind: "prefetch",
					token: first_token,
				}),
			),
		);

		expect_ignored_no_op(
			accept_route_preparation(preparing.model, {
				kind: "prepared",
				prepared: make_prepared_route(),
				token: second_token,
			}),
			preparing.model,
		);
		const prepared = must(
			accept_route_preparation(preparing.model, {
				kind: "prepared",
				prepared: make_prepared_route(),
				token: first_token,
			}),
		);
		expect(prepared.model.prefetch?.phase).toBe("prepared");
		expect(prepared.model.prefetch?.token).toBe(first_token);
	});

	it("ignores API outcomes whose submission token is no longer present", () => {
		const first = must(
			begin_api_submission(make_ready_model(), {
				dedupe_key,
				href: api_href,
				key: api_key,
				method: "POST",
				route_kind: "mutation",
				should_revalidate: true,
				skip_work_indicator: false,
				token: first_token,
			}),
		);
		const second = must(
			begin_api_submission(first.model, {
				dedupe_key,
				href: api_href,
				key: api_key,
				method: "POST",
				route_kind: "mutation",
				should_revalidate: false,
				skip_work_indicator: false,
				token: second_token,
			}),
		);

		const transition = must(
			accept_api_submission_outcome(
				second.model,
				api_success_outcome({ token: first_token }),
			),
		);

		expect(transition.kind).toBe("ignored_stale");
		expect(transition.model).toBe(second.model);
		expect(transition.effects).toEqual([
			{
				token: first_token,
				type: "release_api_submission",
			},
		]);
	});

	it("uses current submission facts when accepting a matching token", () => {
		const started = must(
			begin_api_submission(make_ready_model(), {
				dedupe_key: null,
				href: api_href,
				key: api_key,
				method: "POST",
				route_kind: "mutation",
				should_revalidate: false,
				skip_work_indicator: false,
				token: first_token,
			}),
		);
		const current_submission = started.model.submissions[first_token];
		if (!current_submission) {
			throw new Error("Expected API submission to start");
		}
		// @ts-expect-error API outcomes cannot carry stale submission snapshots.
		const _snapshot_outcome: APISubmissionOutcome = {
			data: { ok: true },
			kind: "success",
			submission: current_submission,
			token: first_token,
		};

		const settled = must(
			accept_api_submission_outcome(
				started.model,
				api_success_outcome({
					token: first_token,
				}),
			),
		);

		expect(settled.kind).toBe("settled");
		expect(settled.model.refresh.kind).toBe("idle");
		expect(
			find_effect(settled.effects, "settle_refresh_calls"),
		).toBeUndefined();
		expect(
			find_effect(settled.effects, "settle_api_submission")?.token,
		).toBe(first_token);
		expect(settled.effects).toContainEqual({
			token: first_token,
			type: "release_api_submission",
		});
	});

	it("uses current submission policy for API soft redirects", () => {
		const started = must(
			begin_api_submission(make_ready_model(), {
				dedupe_key: null,
				href: api_href,
				key: api_key,
				method: "POST",
				route_kind: "mutation",
				should_revalidate: false,
				skip_work_indicator: true,
				token: first_token,
			}),
		);
		const redirected = must(
			accept_api_submission_outcome(started.model, {
				browser_key: next_key,
				href: next_href,
				kind: "soft_redirect",
				navigation_token: second_token,
				state: { redirected: true },
				token: first_token,
			}),
		);

		expect(redirected.kind).toBe("soft_redirect");
		expect(redirected.model.refresh.kind).toBe("idle");
		expect(find_effect(redirected.effects, "fetch_route")).toMatchObject({
			href: next_href,
			token: second_token,
			trigger: "navigation",
		});
		expect(
			find_effect(redirected.effects, "settle_refresh_calls"),
		).toBeUndefined();
	});

	it("settles API hard redirects exactly once", () => {
		const started = must(
			begin_api_submission(make_ready_model(), {
				dedupe_key: null,
				href: api_href,
				key: api_key,
				method: "POST",
				refresh_waiter_id: waiter_id,
				route_kind: "mutation",
				should_revalidate: true,
				skip_work_indicator: false,
				token: first_token,
			}),
		);
		const redirected = must(
			accept_api_submission_outcome(started.model, {
				href: "https://elsewhere.test/next",
				kind: "hard_redirect",
				token: first_token,
			}),
		);

		expect(redirected.kind).toBe("hard_redirect");
		expect(
			redirected.effects.filter((effect) => {
				return effect.type === "settle_api_submission";
			}),
		).toHaveLength(1);
		expect(redirected.effects).toContainEqual({
			href: "https://elsewhere.test/next",
			type: "hard_redirect",
		});
		expect(redirected.effects).toContainEqual({
			ids: [waiter_id],
			result: { ok: true },
			type: "settle_refresh_calls",
		});
	});

	it("dedupe replacement removes only the replaced submission token", () => {
		const first = must(
			begin_api_submission(make_ready_model(), {
				dedupe_key,
				href: api_href,
				key: api_key,
				method: "POST",
				route_kind: "mutation",
				should_revalidate: false,
				skip_work_indicator: false,
				token: first_token,
			}),
		);
		const unrelated = must(
			begin_api_submission(first.model, {
				dedupe_key: null,
				href: `${api_href}/other`,
				key: third_token,
				method: "POST",
				route_kind: "mutation",
				should_revalidate: false,
				skip_work_indicator: false,
				token: third_token,
			}),
		);
		const replaced = must(
			begin_api_submission(unrelated.model, {
				dedupe_key,
				href: api_href,
				key: api_key,
				method: "POST",
				route_kind: "mutation",
				should_revalidate: false,
				skip_work_indicator: false,
				token: second_token,
			}),
		);

		expect(replaced.kind).toBe("replaced");
		expect(replaced.model.submissions[first_token]).toBeUndefined();
		expect(replaced.model.submissions[third_token]?.token).toBe(
			third_token,
		);
		expect(replaced.model.submissions[second_token]?.token).toBe(
			second_token,
		);
		expect(replaced.effects).toContainEqual({
			token: first_token,
			type: "abort_api_submission",
		});
	});

	it("schedules API abort revalidation only when the request dispatched", () => {
		const started = must(
			begin_api_submission(make_ready_model(), {
				dedupe_key: null,
				href: api_href,
				key: api_key,
				method: "POST",
				refresh_waiter_id: waiter_id,
				route_kind: "mutation",
				should_revalidate: true,
				skip_work_indicator: false,
				token: first_token,
			}),
		);
		const undispatched = must(
			accept_api_submission_outcome(started.model, {
				dispatched: false,
				kind: "aborted",
				token: first_token,
			}),
		);
		const restarted = must(
			begin_api_submission(make_ready_model(), {
				dedupe_key: null,
				href: api_href,
				key: api_key,
				method: "POST",
				refresh_waiter_id: waiter_id,
				route_kind: "mutation",
				should_revalidate: true,
				skip_work_indicator: false,
				token: first_token,
			}),
		);
		const dispatched = must(
			accept_api_submission_outcome(restarted.model, {
				dispatched: true,
				error: "offline",
				kind: "network_error",
				token: first_token,
			}),
		);

		expect(undispatched.model.refresh.kind).toBe("idle");
		expect(dispatched.model.refresh.kind).toBe("pending");
		expect(
			find_effect(undispatched.effects, "settle_refresh_calls"),
		).toBeUndefined();
		expect(
			find_effect(dispatched.effects, "settle_refresh_calls"),
		).toBeUndefined();
	});
});

/////////////////////////////////////////////////////////////////////
/////// Publication Transaction Laws
/////////////////////////////////////////////////////////////////////

describe("core4 publication transaction laws", () => {
	it("does not settle a publication before the commit boundary", () => {
		const publishing = must(
			accept_route_preparation(
				prepare_navigation_for_publication().model,
				{
					kind: "prepared",
					prepared: make_prepared_route(),
					token: first_token,
				},
			),
		);

		expect(publishing.kind).toBe("publishing");
		expect(publishing.model.publication?.phase).toBe("publishing");
		expect(publishing.model.publication?.plan).toBe(
			find_effect(publishing.effects, "publish_route")?.plan,
		);
		expect(
			settle_publication(publishing.model, first_token),
		).toBeUndefined();
	});

	it("commits route state atomically before public settlement", () => {
		const preparing = prepare_navigation_for_publication({
			public_call_ids: [waiter_id],
		});
		const publishing = must(
			accept_route_preparation(preparing.model, {
				kind: "prepared",
				prepared: make_prepared_route(),
				token: first_token,
			}),
		);
		const publication = publishing.model.publication;
		const publish_effect = find_effect(publishing.effects, "publish_route");
		const committed = must(
			commit_publication(publishing.model, {
				token: first_token,
			}),
		);

		expect(committed.kind).toBe("committed");
		expect(committed.effects).toEqual([]);
		expect(committed.model.active_route).toBe(null);
		expect(committed.model.browser?.href).toBe(next_href);
		expect(committed.model.current?.route.href).toBe(next_href);
		expect(publication?.plan).toBe(publish_effect?.plan);
		expect(committed.model.publication?.phase).toBe("committed");
		expect(committed.model.publication?.plan).toBe(publication?.plan);

		const settled = must(settle_publication(committed.model, first_token));
		expect(settled.model.publication).toBe(null);
		expect(find_effect(settled.effects, "settle_navigation_calls")).toEqual(
			{
				ids: [waiter_id],
				result: { didNavigate: true },
				type: "settle_navigation_calls",
			},
		);
		expect(settled.effects).toContainEqual({
			token: first_token,
			type: "release_route_work",
		});
	});

	it("stores publication plan facts for navigation and revalidation without duplicating the plan", () => {
		const navigating = must(
			accept_route_preparation(
				prepare_navigation_for_publication({
					public_call_ids: [waiter_id],
				}).model,
				{
					kind: "prepared",
					prepared: make_prepared_route(),
					token: first_token,
				},
			),
		);
		const demanded = must(
			request_revalidation(make_ready_model(), {
				reason: "manual",
				skip_work_indicator: false,
			}),
		);
		const revalidating = must(
			begin_pending_revalidation(demanded.model, {
				token: second_token,
			}),
		);
		const revalidation_response = must(
			accept_route_response(
				revalidating.model,
				route_data_outcome({
					owner_kind: "active_route",
					token: second_token,
				}),
			),
		);
		const publishing_revalidation = must(
			accept_route_preparation(revalidation_response.model, {
				kind: "prepared",
				prepared: make_prepared_route(base_href),
				token: second_token,
			}),
		);

		expect(navigating.effects).not.toContainEqual({
			token: first_token,
			type: "release_route_work",
		});
		expect(navigating.model.publication?.plan).toBe(
			find_effect(navigating.effects, "publish_route")?.plan,
		);
		expect(navigating.model.publication?.plan.previous).not.toBeNull();
		expect(navigating.model.publication?.plan.history).toMatchObject({
			kind: "push",
		});
		expect(navigating.model.publication?.plan.hooks).toEqual({
			kind: "run",
			trigger: "navigation",
		});
		expect(publishing_revalidation.model.publication?.plan.history).toEqual(
			{ kind: "none" },
		);
		expect(
			publishing_revalidation.model.publication?.plan.use_view_transition,
		).toBe(false);
	});

	it("stores popstate publication with no history action", () => {
		const started = must(
			begin_popstate(make_ready_model(), {
				browser: make_browser(other_href, second_key),
				public_call_ids: [waiter_id],
				token: first_token,
			}),
		);
		const response = must(
			accept_route_response(
				started.model,
				route_data_outcome({
					owner_kind: "active_route",
					token: first_token,
				}),
			),
		);
		const publishing = must(
			accept_route_preparation(response.model, {
				kind: "prepared",
				prepared: make_prepared_route(other_href),
				token: first_token,
			}),
		);

		expect(publishing.model.publication?.plan.history).toEqual({
			kind: "none",
		});
	});

	it("does not commit a publishing publication without the active route transaction", () => {
		const publishing = must(
			accept_route_preparation(
				prepare_navigation_for_publication().model,
				{
					kind: "prepared",
					prepared: make_prepared_route(),
					token: first_token,
				},
			),
		);
		const orphaned: Core4Model = {
			...publishing.model,
			active_route: null,
		} as Core4Model;
		const publication = publishing.model.publication;
		if (!publication || publication.phase !== "publishing") {
			throw new Error("Expected publishing publication");
		}
		// @ts-expect-error Publishing publication requires an active route owner.
		const _orphaned_publication: Core4Model = {
			...publishing.model,
			active_route: null,
			publication,
		};

		expect(
			commit_publication(orphaned, { token: first_token }),
		).toBeUndefined();
	});

	it("ignores commits and settlements for unrelated tokens", () => {
		const preparing = prepare_navigation_for_publication();
		const publishing = must(
			accept_route_preparation(preparing.model, {
				kind: "prepared",
				prepared: make_prepared_route(),
				token: first_token,
			}),
		);

		expect(
			commit_publication(publishing.model, {
				token: second_token,
			}),
		).toBeUndefined();
		expect(
			settle_publication(publishing.model, second_token),
		).toBeUndefined();
	});

	it("failed publication requires and clears the matching active route", () => {
		const publishing = must(
			accept_route_preparation(
				prepare_navigation_for_publication({
					public_call_ids: [waiter_id],
				}).model,
				{
					kind: "prepared",
					prepared: make_prepared_route(),
					token: first_token,
				},
			),
		);
		const orphaned = {
			...publishing.model,
			active_route: null,
		} as Core4Model;

		expect(fail_publication(orphaned, first_token)).toBeUndefined();
		const failed = must(fail_publication(publishing.model, first_token));
		expect(failed.model.active_route).toBeNull();
		expect(failed.model.publication).toBeNull();
		expect(failed.effects).toContainEqual({
			ids: [waiter_id],
			result: { didNavigate: false },
			type: "settle_navigation_calls",
		});
		expect(failed.effects).toContainEqual({
			token: first_token,
			type: "release_route_work",
		});
	});
});

/////////////////////////////////////////////////////////////////////
/////// Prefetch Laws
/////////////////////////////////////////////////////////////////////

describe("core4 prefetch laws", () => {
	it("does not prefetch current, active, or already-prefetched documents", () => {
		const active = must(
			begin_navigation(make_ready_model(), {
				browser_key: next_key,
				href: next_href,
				public_call_ids: [],
				replace: false,
				skip_work_indicator: false,
				state: undefined,
				token: first_token,
			}),
		);
		const prefetched = must(
			begin_prefetch(make_ready_model(), {
				href: next_href,
				token: first_token,
			}),
		);

		expect(
			begin_prefetch(make_ready_model(), {
				href: base_href,
				token: first_token,
			}),
		).toBeUndefined();
		expect(
			begin_prefetch(active.model, {
				href: next_href,
				token: second_token,
			}),
		).toBeUndefined();
		expect(
			begin_prefetch(prefetched.model, {
				href: next_href,
				token: second_token,
			}),
		).toBeUndefined();
	});

	it("replaces only the previous prefetch token", () => {
		const first = must(
			begin_prefetch(make_ready_model(), {
				href: next_href,
				token: first_token,
			}),
		);
		const second = must(
			begin_prefetch(first.model, {
				href: other_href,
				token: second_token,
			}),
		);

		expect(second.model.prefetch?.token).toBe(second_token);
		expect(second.effects).toContainEqual({
			token: first_token,
			type: "abort_route_work",
		});
		expect(find_effect(second.effects, "fetch_route")).toMatchObject({
			href: other_href,
			token: second_token,
			trigger: "prefetch",
		});
	});

	it("cancels only a matching prefetch document", () => {
		const prefetched = must(
			begin_prefetch(make_ready_model(), {
				href: next_href,
				token: first_token,
			}),
		);

		expect(cancel_prefetch(prefetched.model, other_href)).toBeUndefined();
		const canceled = must(cancel_prefetch(prefetched.model, next_href));
		expect(canceled.model.prefetch).toBeNull();
		expect(canceled.effects).toEqual([
			{
				token: first_token,
				type: "abort_route_work",
			},
		]);
	});

	it("promotes fetching prefetch by moving its token to active route", () => {
		const prefetched = must(
			begin_prefetch(make_ready_model(), {
				href: next_href,
				token: first_token,
			}),
		);
		const promoted = must(
			begin_navigation(prefetched.model, {
				browser_key: next_key,
				href: next_href,
				public_call_ids: [waiter_id],
				replace: false,
				skip_work_indicator: false,
				state: undefined,
				token: second_token,
			}),
		);

		expect(promoted.kind).toBe("promoted_prefetch");
		expect(promoted.model.prefetch).toBeNull();
		expect(promoted.model.active_route?.token).toBe(first_token);
		expect(promoted.effects).toEqual([]);
	});

	it("promotes prepared prefetch through publication without fetching", () => {
		const prefetched = must(
			begin_prefetch(make_ready_model(), {
				href: next_href,
				token: first_token,
			}),
		);
		const preparing = must(
			accept_route_response(
				prefetched.model,
				route_data_outcome({
					owner_kind: "prefetch",
					token: first_token,
				}),
			),
		);
		const prepared = must(
			accept_route_preparation(preparing.model, {
				kind: "prepared",
				prepared: make_prepared_route(),
				token: first_token,
			}),
		);
		const promoted = must(
			begin_navigation(prepared.model, {
				browser_key: next_key,
				href: next_href,
				public_call_ids: [waiter_id],
				replace: false,
				skip_work_indicator: false,
				state: undefined,
				token: second_token,
			}),
		);

		expect(promoted.model.active_route?.token).toBe(second_token);
		expect(find_effect(promoted.effects, "fetch_route")).toBeUndefined();
		expect(find_effect(promoted.effects, "publish_route")).toBeDefined();
	});
});

/////////////////////////////////////////////////////////////////////
/////// Freshness Demand Laws
/////////////////////////////////////////////////////////////////////

describe("core4 freshness demand laws", () => {
	it("coalesces refresh demand with sequence, waiters, skip policy, and timers", () => {
		const first = must(
			request_revalidation(make_ready_model(), {
				reason: "manual",
				skip_work_indicator: true,
				timer_id,
				waiter_id,
			}),
		);
		const second = must(
			request_revalidation(first.model, {
				reason: "windowFocus",
				skip_work_indicator: false,
				timer_id: second_timer_id,
				waiter_id: second_waiter_id,
			}),
		);

		expect(first.model.refresh.kind).toBe("debouncing");
		expect(second.model.refresh.kind).toBe("debouncing");
		if (second.model.refresh.kind !== "debouncing") {
			throw new Error("Expected debounced refresh");
		}
		expect(second.model.refresh.demand).toMatchObject({
			after_sequence: 1,
			reason: "windowFocus",
			skip_work_indicator: false,
		});
		expect(second.model.refresh.demand.waiters).toEqual([
			{ id: waiter_id },
			{ id: second_waiter_id },
		]);
		expect(second.effects).toContainEqual({
			id: timer_id,
			type: "clear_refresh_timer",
		});
		expect(second.effects).toContainEqual({
			id: second_timer_id,
			ms: 8,
			type: "start_refresh_timer",
		});
	});

	it("fires only matching refresh timers into pending demand", () => {
		const debounced = must(
			request_revalidation(make_ready_model(), {
				reason: "manual",
				skip_work_indicator: false,
				timer_id,
			}),
		);
		const ignored = fire_refresh_timer(debounced.model, second_timer_id);
		const fired = fire_refresh_timer(debounced.model, timer_id);

		expect(ignored.kind).toBe("ignored");
		expect(ignored.model).toBe(debounced.model);
		expect(fired.kind).toBe("pending");
		expect(fired.model.refresh).toMatchObject({
			attempt: 0,
			kind: "pending",
		});
	});

	it("starts pending revalidation only from a ready idle route boundary", () => {
		const ready = make_ready_model();
		const pending = must(
			request_revalidation(ready, {
				reason: "manual",
				skip_work_indicator: false,
			}),
		);
		const active = must(
			begin_navigation(pending.model, {
				browser_key: next_key,
				href: next_href,
				public_call_ids: [],
				replace: false,
				skip_work_indicator: false,
				state: undefined,
				token: second_token,
			}),
		);

		expect(
			begin_pending_revalidation(ready, { token: first_token }),
		).toBeUndefined();
		expect(
			begin_pending_revalidation(active.model, { token: first_token }),
		).toBeUndefined();

		const started = must(
			begin_pending_revalidation(pending.model, {
				token: first_token,
			}),
		);
		expect(started.model.active_route?.kind).toBe("revalidation");
		expect(started.model.refresh.kind).toBe("running");
	});

	it("does not let already-running route work satisfy newer freshness demand", () => {
		const started = must(
			begin_navigation(make_ready_model(), {
				browser_key: next_key,
				href: next_href,
				public_call_ids: [],
				replace: false,
				skip_work_indicator: false,
				state: undefined,
				token: first_token,
			}),
		);
		const demanded = must(
			request_revalidation(started.model, {
				reason: "manual",
				skip_work_indicator: false,
				waiter_id,
			}),
		);
		expect(demanded.model.refresh.kind).toBe("pending");

		const active_route = demanded.model.active_route;
		if (!active_route) {
			throw new Error("Expected navigation to remain active");
		}
		const response = must(
			accept_route_response(
				demanded.model,
				route_data_outcome({
					owner_kind: "active_route",
					token: first_token,
				}),
			),
		);
		const publishing = must(
			accept_route_preparation(response.model, {
				kind: "prepared",
				prepared: make_prepared_route(),
				token: first_token,
			}),
		);
		const committed = must(
			commit_publication(publishing.model, {
				token: first_token,
			}),
		);
		const settled = must(settle_publication(committed.model, first_token));

		expect(settled.model.refresh.kind).toBe("pending");
		expect(
			find_effect(settled.effects, "settle_refresh_calls"),
		).toBeUndefined();
	});

	it("lets later route work satisfy an existing freshness demand", () => {
		const demanded = must(
			request_revalidation(make_ready_model(), {
				reason: "manual",
				skip_work_indicator: false,
				waiter_id,
			}),
		);
		const started = must(
			begin_navigation(demanded.model, {
				browser_key: next_key,
				href: next_href,
				public_call_ids: [],
				replace: false,
				skip_work_indicator: false,
				state: undefined,
				token: first_token,
			}),
		);
		const active_route = started.model.active_route;
		if (!active_route) {
			throw new Error("Expected navigation to start");
		}
		const response = must(
			accept_route_response(
				started.model,
				route_data_outcome({
					owner_kind: "active_route",
					token: first_token,
				}),
			),
		);
		const publishing = must(
			accept_route_preparation(response.model, {
				kind: "prepared",
				prepared: make_prepared_route(),
				token: first_token,
			}),
		);
		const committed = must(
			commit_publication(publishing.model, {
				token: first_token,
			}),
		);
		const settled = must(settle_publication(committed.model, first_token));

		expect(settled.model.refresh.kind).toBe("idle");
		expect(find_effect(settled.effects, "settle_refresh_calls")).toEqual({
			ids: [waiter_id],
			result: { ok: true },
			type: "settle_refresh_calls",
		});
	});

	it("keeps running revalidation ownership in the active route slot", () => {
		const demanded = must(
			request_revalidation(make_ready_model(), {
				reason: "manual",
				skip_work_indicator: false,
			}),
		);
		const started = must(
			begin_pending_revalidation(demanded.model, {
				token: first_token,
			}),
		);

		expect(started.model.active_route?.kind).toBe("revalidation");
		expect(started.model.active_route?.token).toBe(first_token);
		expect(started.model.refresh.kind).toBe("running");
		if (started.model.refresh.kind !== "running") {
			throw new Error("Expected refresh to be running");
		}
		// @ts-expect-error Running refresh does not duplicate the active route token.
		const _running_refresh_token = started.model.refresh.token;
		expect(
			accept_route_preparation(started.model, {
				kind: "prepared",
				prepared: make_prepared_route(),
				token: second_token,
			}),
		).toEqual({
			effects: [],
			kind: "ignored_stale",
			model: started.model,
		});
	});

	it("moves committed revalidation out of running work until public settlement", () => {
		const demanded = must(
			request_revalidation(make_ready_model(), {
				reason: "manual",
				skip_work_indicator: false,
				waiter_id,
			}),
		);
		const started = must(
			begin_pending_revalidation(demanded.model, {
				token: first_token,
			}),
		);
		const active_route = started.model.active_route;
		if (!active_route) {
			throw new Error("Expected revalidation to start");
		}
		const response = must(
			accept_route_response(
				started.model,
				route_data_outcome({
					owner_kind: "active_route",
					token: first_token,
				}),
			),
		);
		const publishing = must(
			accept_route_preparation(response.model, {
				kind: "prepared",
				prepared: make_prepared_route(base_href),
				token: first_token,
			}),
		);
		const committed = must(
			commit_publication(publishing.model, {
				token: first_token,
			}),
		);

		expect(committed.model.active_route).toBe(null);
		expect(committed.model.publication?.phase).toBe("committed");
		expect(committed.model.refresh.kind).toBe("settling");
		if (committed.model.refresh.kind !== "settling") {
			throw new Error("Expected refresh to be settling");
		}
		// @ts-expect-error Settling refresh derives ownership from committed publication.
		const _settling_refresh_token = committed.model.refresh.token;
		// @ts-expect-error Settling refresh is past retry-attempt accounting.
		const _settling_refresh_attempt = committed.model.refresh.attempt;

		const settled = must(settle_publication(committed.model, first_token));
		expect(settled.model.refresh.kind).toBe("idle");
		expect(find_effect(settled.effects, "settle_refresh_calls")).toEqual({
			ids: [waiter_id],
			result: { ok: true },
			type: "settle_refresh_calls",
		});
	});

	it("retries only retryable revalidation failures", () => {
		const demanded = must(
			request_revalidation(make_ready_model(), {
				reason: "manual",
				skip_work_indicator: false,
				waiter_id,
			}),
		);
		const started = must(
			begin_pending_revalidation(demanded.model, {
				token: first_token,
			}),
		);
		const retried = must(
			accept_route_response(started.model, {
				kind: "failed",
				owner_kind: "active_route",
				retryable: true,
				token: first_token,
			}),
		);
		const restarted = must(
			begin_pending_revalidation(demanded.model, {
				token: first_token,
			}),
		);
		const failed = must(
			accept_route_response(restarted.model, {
				kind: "failed",
				owner_kind: "active_route",
				retryable: false,
				token: first_token,
			}),
		);

		expect(retried.model.refresh.kind).toBe("retrying");
		expect(
			find_effect(retried.effects, "start_refresh_timer"),
		).toMatchObject({
			ms: 500,
			type: "start_refresh_timer",
		});
		expect(failed.model.refresh.kind).toBe("idle");
		expect(failed.effects).toContainEqual({
			ids: [waiter_id],
			result: { ok: false, reason: "max_retries_exhausted" },
			type: "settle_refresh_calls",
		});
	});
});

/////////////////////////////////////////////////////////////////////
/////// Redirect Laws
/////////////////////////////////////////////////////////////////////

describe("core4 redirect laws", () => {
	it("preserves route token and sequence while incrementing redirect count", () => {
		const started = must(
			begin_navigation(make_ready_model(), {
				browser_key: next_key,
				href: next_href,
				public_call_ids: [waiter_id],
				replace: false,
				skip_work_indicator: false,
				state: undefined,
				token: first_token,
			}),
		);
		const sequence = started.model.active_route?.sequence;
		const redirected = must(
			accept_route_response(started.model, {
				href: other_href,
				kind: "soft_redirect",
				owner_kind: "active_route",
				token: first_token,
			}),
		);

		expect(redirected.model.active_route).toMatchObject({
			href: other_href,
			redirect_count: 1,
			sequence,
			source: "redirect",
			token: first_token,
		});
		expect(find_effect(redirected.effects, "fetch_route")).toMatchObject({
			href: other_href,
			trigger: "navigation",
		});
	});

	it("converts revalidation soft redirects to navigation work", () => {
		const demanded = must(
			request_revalidation(make_ready_model(), {
				reason: "manual",
				skip_work_indicator: false,
			}),
		);
		const started = must(
			begin_pending_revalidation(demanded.model, {
				token: first_token,
			}),
		);
		const redirected = must(
			accept_route_response(started.model, {
				href: next_href,
				kind: "soft_redirect",
				owner_kind: "active_route",
				token: first_token,
			}),
		);

		expect(redirected.model.active_route).toMatchObject({
			kind: "navigation",
			source: "redirect",
			token: first_token,
		});
		expect(find_effect(redirected.effects, "fetch_route")?.trigger).toBe(
			"navigation",
		);
	});

	it("finishes invalid, cross-origin, current-document, and excessive redirects without refetching active work", () => {
		const invalid = must(
			begin_navigation(make_ready_model(), {
				browser_key: next_key,
				href: next_href,
				public_call_ids: [waiter_id],
				replace: false,
				skip_work_indicator: false,
				state: undefined,
				token: first_token,
			}),
		);
		const cross_origin = must(
			begin_navigation(make_ready_model(), {
				browser_key: next_key,
				href: next_href,
				public_call_ids: [waiter_id],
				replace: false,
				skip_work_indicator: false,
				state: undefined,
				token: first_token,
			}),
		);
		const current = must(
			begin_navigation(make_ready_model(), {
				browser_key: next_key,
				href: next_href,
				public_call_ids: [waiter_id],
				replace: false,
				skip_work_indicator: false,
				state: undefined,
				token: first_token,
			}),
		);
		const excessive_model = {
			...invalid.model,
			active_route: invalid.model.active_route
				? {
						...invalid.model.active_route,
						redirect_count: CORE4_MAX_REDIRECTS,
					}
				: null,
		} as Core4Model;

		const invalid_redirect = must(
			accept_route_response(invalid.model, {
				href: "mailto:hello@example.test",
				kind: "soft_redirect",
				owner_kind: "active_route",
				token: first_token,
			}),
		);
		const hard_redirect = must(
			accept_route_response(cross_origin.model, {
				href: "https://elsewhere.test/next",
				kind: "soft_redirect",
				owner_kind: "active_route",
				token: first_token,
			}),
		);
		const no_op_redirect = must(
			accept_route_response(current.model, {
				href: base_href,
				kind: "soft_redirect",
				owner_kind: "active_route",
				token: first_token,
			}),
		);
		const too_many = must(
			accept_route_response(excessive_model, {
				href: other_href,
				kind: "soft_redirect",
				owner_kind: "active_route",
				token: first_token,
			}),
		);

		expect(invalid_redirect.model.active_route).toBeNull();
		expect(
			find_effect(invalid_redirect.effects, "fetch_route"),
		).toBeUndefined();
		expect(hard_redirect.effects).toContainEqual({
			href: "https://elsewhere.test/next",
			type: "hard_redirect",
		});
		expect(
			find_effect(hard_redirect.effects, "fetch_route"),
		).toBeUndefined();
		expect(no_op_redirect.model.active_route).toBeNull();
		expect(
			find_effect(no_op_redirect.effects, "fetch_route"),
		).toBeUndefined();
		expect(too_many.kind).toBe("failed");
		expect(too_many.model.active_route).toBeNull();
	});
});

/////////////////////////////////////////////////////////////////////
/////// Build-Skew Laws
/////////////////////////////////////////////////////////////////////

describe("core4 build-skew laws", () => {
	it("lets the build-skew protocol header win over soft redirects", () => {
		const started = must(
			begin_navigation(make_ready_model(), {
				browser_key: next_key,
				href: next_href,
				public_call_ids: [],
				replace: false,
				skip_work_indicator: false,
				state: undefined,
				token: first_token,
			}),
		);
		const active_route = started.model.active_route;
		if (!active_route) {
			throw new Error("Expected active route");
		}
		const outcome = classify_route_response({
			owner: { active_route, kind: "active_route" },
			payload: make_route_payload(),
			requested_href: next_href,
			response: make_response_facts({
				headers: {
					[BUILD_ID_HEADER]: "server-build",
					[X_CLIENT_REDIRECT]: other_href,
					[X_VORMA_BUILD_SKEW]: VORMA_PROTOCOL_ENABLED,
				},
			}),
			token: first_token,
		});

		expect(outcome).toMatchObject({
			behavior: "reload",
			kind: "build_skew",
			owner_kind: "active_route",
		});
	});

	it("drops build-skew prefetch and revalidation responses", () => {
		const prefetched = must(
			begin_prefetch(make_ready_model(), {
				href: next_href,
				token: first_token,
			}),
		);
		const prefetch = prefetched.model.prefetch;
		if (!prefetch) {
			throw new Error("Expected prefetch");
		}
		const demanded = must(
			request_revalidation(make_ready_model(), {
				reason: "manual",
				skip_work_indicator: false,
			}),
		);
		const revalidating = must(
			begin_pending_revalidation(demanded.model, {
				token: second_token,
			}),
		);
		const active_route = revalidating.model.active_route;
		if (!active_route) {
			throw new Error("Expected revalidation route");
		}
		const response = make_response_facts({
			headers: {
				[X_VORMA_BUILD_SKEW]: VORMA_PROTOCOL_ENABLED,
			},
		});

		expect(
			classify_route_response({
				owner: { kind: "prefetch", prefetch },
				requested_href: next_href,
				response,
				token: first_token,
			}),
		).toMatchObject({
			behavior: "drop",
			default_behavior: "dropResponse",
			kind: "build_skew",
		});
		const dropped_prefetch = must(
			accept_route_response(prefetched.model, {
				behavior: "drop",
				default_behavior: "dropResponse",
				href: next_href,
				kind: "build_skew",
				owner_kind: "prefetch",
				response: {
					ok: true,
					server_build_id: "server-build",
					status: 200,
				},
				token: first_token,
			}),
		);
		expect(dropped_prefetch.model.prefetch).toBeNull();
		expect(
			find_effect(dropped_prefetch.effects, "hard_redirect"),
		).toBeUndefined();
		expect(
			classify_route_response({
				owner: { active_route, kind: "active_route" },
				requested_href: base_href,
				response,
				token: second_token,
			}),
		).toMatchObject({
			behavior: "drop",
			default_behavior: "dropResponse",
			kind: "build_skew",
		});
	});

	it("emits hard redirect for build-skew reload behavior", () => {
		const started = must(
			begin_navigation(make_ready_model(), {
				browser_key: next_key,
				href: next_href,
				public_call_ids: [waiter_id],
				replace: false,
				skip_work_indicator: false,
				state: undefined,
				token: first_token,
			}),
		);
		const transition = must(
			accept_route_response(started.model, {
				behavior: "reload",
				default_behavior: "hardReload",
				href: next_href,
				kind: "build_skew",
				owner_kind: "active_route",
				response: {
					ok: true,
					server_build_id: "server-build",
					status: 200,
				},
				token: first_token,
			}),
		);

		expect(transition.effects).toContainEqual({
			href: next_href,
			type: "hard_redirect",
		});
		expect(transition.effects).toContainEqual({
			token: first_token,
			type: "release_route_work",
		});
		expect(transition.model.active_route).toBeNull();
	});

	it("uses API build-skew hard reload default only for cross-origin redirects", () => {
		const started = must(
			begin_api_submission(make_ready_model(), {
				dedupe_key: null,
				href: api_href,
				key: api_key,
				method: "POST",
				route_kind: "mutation",
				should_revalidate: false,
				skip_work_indicator: false,
				token: first_token,
			}),
		);
		const submission = started.model.submissions[first_token];
		if (!submission) {
			throw new Error("Expected submission");
		}
		const cross_origin = classify_api_response({
			browser_key: next_key,
			data: undefined,
			navigation_token: second_token,
			requested_href: api_href,
			response: make_response_facts({
				headers: {
					[BUILD_ID_HEADER]: "server-build",
					[X_CLIENT_REDIRECT]: "https://elsewhere.test/next",
				},
			}),
			state: undefined,
			submission,
			token: first_token,
		});
		const same_origin = classify_api_response({
			browser_key: next_key,
			data: undefined,
			navigation_token: second_token,
			requested_href: api_href,
			response: make_response_facts({
				headers: {
					[BUILD_ID_HEADER]: "server-build",
					[X_CLIENT_REDIRECT]: next_href,
				},
			}),
			state: undefined,
			submission,
			token: first_token,
		});

		expect(cross_origin.build_skew_report?.default_behavior).toBe(
			"hardReload",
		);
		expect(same_origin.build_skew_report?.default_behavior).toBe(
			"notifyOnly",
		);
	});

	it("suppresses or emits build-skew notifications from current model facts", () => {
		const equal_build = must(
			begin_navigation(make_ready_model(), {
				browser_key: next_key,
				href: next_href,
				public_call_ids: [],
				replace: false,
				skip_work_indicator: false,
				state: undefined,
				token: first_token,
			}),
		);
		const different_build = must(
			begin_navigation(make_ready_model(), {
				browser_key: next_key,
				href: next_href,
				public_call_ids: [],
				replace: false,
				skip_work_indicator: false,
				state: undefined,
				token: first_token,
			}),
		);

		const equal_build_route = equal_build.model.active_route;
		if (!equal_build_route) {
			throw new Error("Expected active route");
		}
		const suppressed = must(
			accept_route_response(
				equal_build.model,
				classify_route_response({
					owner: {
						active_route: equal_build_route,
						kind: "active_route",
					},
					payload: make_route_payload(),
					requested_href: next_href,
					response: make_response_facts({
						headers: {
							[BUILD_ID_HEADER]: client_build_id,
						},
					}),
					token: first_token,
				}),
			),
		);
		const emitted = must(
			accept_route_response(different_build.model, {
				build_skew_report: {
					default_behavior: "notifyOnly",
					requested_href: next_href,
					response: {
						ok: true,
						server_build_id: "server-build",
						status: 200,
					},
				},
				kind: "data",
				owner_kind: "active_route",
				payload: make_route_payload(),
				token: first_token,
			}),
		);

		expect(
			find_effect(suppressed.effects, "notify_build_skew"),
		).toBeUndefined();
		expect(find_effect(emitted.effects, "notify_build_skew")).toMatchObject(
			{
				notification: {
					activeClientBuildID: client_build_id,
					currentRouteState: make_route_state(base_href),
					currentWorkState: {
						navigation: {
							href: next_href,
							replace: false,
							source: "navigate",
						},
					},
					serverBuildID: "server-build",
					triggeringResponse: {
						kind: "route",
						requestedHref: next_href,
						trigger: "navigation",
					},
				},
				type: "notify_build_skew",
			},
		);
	});

	it("uses hard reload default for cross-origin navigation build-skew reports", () => {
		const started = must(
			begin_navigation(make_ready_model(), {
				browser_key: next_key,
				href: next_href,
				public_call_ids: [],
				replace: false,
				skip_work_indicator: false,
				state: undefined,
				token: first_token,
			}),
		);
		const active_route = started.model.active_route;
		if (!active_route) {
			throw new Error("Expected active route");
		}
		const outcome = classify_route_response({
			owner: { active_route, kind: "active_route" },
			requested_href: next_href,
			response: make_response_facts({
				headers: {
					[BUILD_ID_HEADER]: "server-build",
					[X_CLIENT_REDIRECT]: "https://elsewhere.test/next",
				},
			}),
			token: first_token,
		});

		expect(outcome.build_skew_report?.default_behavior).toBe("hardReload");
	});
});

/////////////////////////////////////////////////////////////////////
/////// Work Projection Laws
/////////////////////////////////////////////////////////////////////

describe("core4 work projection laws", () => {
	it("projects each live work slot exactly once with its skip policy", () => {
		const navigation = must(
			begin_navigation(make_ready_model(), {
				browser_key: next_key,
				href: next_href,
				public_call_ids: [],
				replace: false,
				skip_work_indicator: true,
				state: undefined,
				token: first_token,
			}),
		);
		const popstate = must(
			begin_popstate(make_ready_model(), {
				browser: make_browser(other_href, second_key),
				public_call_ids: [],
				token: first_token,
			}),
		);
		const debouncing = must(
			request_revalidation(make_ready_model(), {
				reason: "manual",
				skip_work_indicator: true,
				timer_id,
			}),
		);
		const pending = must(
			request_revalidation(make_ready_model(), {
				reason: "manual",
				skip_work_indicator: false,
			}),
		);
		const api = must(
			begin_api_submission(make_ready_model(), {
				dedupe_key: null,
				href: api_href,
				key: api_key,
				method: "POST",
				route_kind: "mutation",
				should_revalidate: false,
				skip_work_indicator: true,
				token: first_token,
			}),
		);
		const pending_during_navigation = must(
			request_revalidation(navigation.model, {
				reason: "manual",
				skip_work_indicator: false,
			}),
		);

		expect(derive_core4_work_state(navigation.model).navigation).toEqual({
			href: next_href,
			replace: false,
			source: "navigate",
		});
		expect(derive_core4_work_projection(navigation.model)).toContainEqual({
			kind: "navigation",
			skip_work_indicator: true,
		});
		expect(derive_core4_work_state(popstate.model).navigation).toEqual({
			href: other_href,
			replace: true,
			source: "popstate",
		});
		expect(derive_core4_work_projection(debouncing.model)).toContainEqual({
			kind: "revalidation",
			skip_work_indicator: true,
		});
		expect(
			derive_core4_work_projection(pending_during_navigation.model),
		).not.toContainEqual({
			kind: "revalidation",
			skip_work_indicator: false,
		});
		expect(derive_core4_work_state(pending.model).revalidation).toEqual({
			attempt: 0,
			status: "running",
		});
		expect(derive_core4_work_projection(api.model)).toContainEqual({
			kind: "apiRequest",
			skip_work_indicator: true,
		});
	});

	it("does not report prepared prefetch as active work", () => {
		const prefetched = must(
			begin_prefetch(make_ready_model(), {
				href: next_href,
				token: first_token,
			}),
		);
		const prefetch = prefetched.model.prefetch;
		if (!prefetch) {
			throw new Error("Expected prefetch to start");
		}
		const response = must(
			accept_route_response(
				prefetched.model,
				route_data_outcome({
					owner_kind: "prefetch",
					token: first_token,
				}),
			),
		);
		const prepared = must(
			accept_route_preparation(response.model, {
				kind: "prepared",
				prepared: make_prepared_route(),
				token: first_token,
			}),
		);

		expect(prepared.model.prefetch?.phase).toBe("prepared");
		expect(derive_core4_work_state(prepared.model).prefetch).toBe(null);
		expect(derive_core4_work_projection(prepared.model)).not.toContainEqual(
			{
				kind: "prefetch",
			},
		);
	});

	it("does not report removed submissions as API work", () => {
		const model = {
			...make_ready_model(),
			submissions: {
				[first_token]: undefined,
			},
		} as Core4Model;

		expect(derive_core4_work_state(model).apiRequests).toEqual([]);
		expect(derive_core4_work_projection(model)).not.toContainEqual({
			kind: "apiRequest",
			skip_work_indicator: false,
		});
	});

	it("does not report publishing route work as active navigation work", () => {
		const preparing = prepare_navigation_for_publication({
			public_call_ids: [waiter_id],
		});
		const publishing = must(
			accept_route_preparation(preparing.model, {
				kind: "prepared",
				prepared: make_prepared_route(),
				token: first_token,
			}),
		);

		expect(publishing.model.active_route?.phase).toBe("publishing");
		expect(derive_core4_work_state(publishing.model).navigation).toBe(null);
		expect(
			derive_core4_work_projection(publishing.model),
		).not.toContainEqual({
			kind: "navigation",
			skip_work_indicator: false,
		});
	});
});

/////////////////////////////////////////////////////////////////////
/////// Effect Algebra Laws
/////////////////////////////////////////////////////////////////////

describe("core4 effect algebra laws", () => {
	it("keeps hard redirects model-free while settling public callers", () => {
		const model = make_ready_model();
		const transition = must(
			begin_navigation(model, {
				browser_key: next_key,
				href: "https://elsewhere.test/next",
				public_call_ids: [waiter_id],
				replace: false,
				skip_work_indicator: false,
				state: undefined,
				token: first_token,
			}),
		);

		expect(transition.kind).toBe("hard_redirect");
		expect(transition.model).toBe(model);
		expect(transition.effects).toEqual([
			{
				href: "https://elsewhere.test/next",
				type: "hard_redirect",
			},
			{
				ids: [waiter_id],
				result: { didNavigate: false },
				type: "settle_navigation_calls",
			},
		]);
	});

	it("distinguishes same-document no-op from same-document publication", () => {
		const no_op = must(
			begin_navigation(make_ready_model(), {
				browser_key: next_key,
				href: base_href,
				public_call_ids: [waiter_id],
				replace: false,
				skip_work_indicator: false,
				state: undefined,
				token: first_token,
			}),
		);
		const hash = must(
			begin_navigation(make_ready_model(), {
				browser_key: next_key,
				href: `${base_href}#section`,
				public_call_ids: [waiter_id],
				replace: false,
				skip_work_indicator: false,
				state: undefined,
				token: first_token,
			}),
		);

		expect(find_effect(no_op.effects, "apply_scroll")).toEqual({
			scroll: { x: 0, y: 0 },
			type: "apply_scroll",
		});
		expect(find_effect(no_op.effects, "fetch_route")).toBeUndefined();
		expect(find_effect(no_op.effects, "publish_route")).toBeUndefined();
		expect(find_effect(hash.effects, "publish_route")).toBeDefined();
		expect(find_effect(hash.effects, "fetch_route")).toBeUndefined();
		expect(
			find_effect(hash.effects, "publish_route")?.plan
				.use_view_transition,
		).toBe(false);
	});

	it("emits explicit cleanup when dropped prefetch route work is no longer live", () => {
		const prefetched = must(
			begin_prefetch(make_ready_model(), {
				href: next_href,
				token: first_token,
			}),
		);
		const prefetch = prefetched.model.prefetch;
		if (!prefetch) {
			throw new Error("Expected prefetch to start");
		}
		const transition = must(
			accept_route_response(prefetched.model, {
				kind: "failed",
				owner_kind: "prefetch",
				retryable: false,
				token: first_token,
			}),
		);

		expect(transition.effects).toContainEqual({
			token: first_token,
			type: "release_route_work",
		});
	});

	it("emits explicit cleanup when API work finishes", () => {
		const started = must(
			begin_api_submission(make_ready_model(), {
				dedupe_key: null,
				href: api_href,
				key: api_key,
				method: "POST",
				route_kind: "mutation",
				should_revalidate: false,
				skip_work_indicator: false,
				token: first_token,
			}),
		);
		const transition = must(
			accept_api_submission_outcome(
				started.model,
				api_success_outcome({ token: first_token }),
			),
		);

		expect(transition.effects[0]).toEqual({
			token: first_token,
			type: "release_api_submission",
		});
		expect(transition.effects).toContainEqual({
			result: {
				data: { ok: true },
				response: undefined,
				success: true,
			},
			token: first_token,
			type: "settle_api_submission",
		});
	});
});
