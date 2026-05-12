import {
	createPatternRegistry,
	registerPattern,
	type PatternRegistry,
} from "vorma/kit/matcher";
import { R, type Result } from "vorma/kit/result";
import {
	BUILD_ID_HEADER,
	CONTENT_TYPE_HEADER,
	HISTORY_KEY_FIELD,
	HISTORY_USER_STATE_FIELD,
	JSON_CONTENT_TYPE,
	SCROLL_STORAGE_KEY,
	SCROLL_STORAGE_RELOAD_KEY,
	VERCEL_DPL_QUERY_PARAM_KEY,
	VERCEL_X_DEPLOYMENT_ID,
	VORMA_JSON_KEY,
	VORMA_PROTOCOL_ENABLED,
	X_ACCEPTS_CLIENT_REDIRECT,
	X_CLIENT_REDIRECT,
	X_VORMA_BUILD_SKEW,
} from "../core/constants.ts";
import type {
	APIFetchOutcome,
	APIToken,
	BrowserKey,
	BuildSkewNotification,
	ClientLoaderID,
	Effect,
	HistoryAction,
	InputEvent,
	PublicCallID,
	RefreshTimerID,
	RouteFetchOutcome,
	RouteToken,
	RouteUpdateReason,
	WorkState,
} from "./events.ts";
import type {
	ClientCommit,
	ClientCommitPlan,
	Model,
	PreparedRoute,
	RoutePayload,
	RouteRenderPlanState,
	RouteRenderState,
	RouteState,
	WorkIndicatorOptions,
} from "./model.ts";
import { initial_model } from "./model.ts";
import { decode_route_payload, type SearchSchemaRegistry } from "./payload.ts";
import type { AbortHandle, Platform, TimerHandle } from "./platform.ts";
import {
	derive_work_state,
	route_state_from_prepared,
} from "./reducer-core.ts";
import { reduce } from "./reducer-inputs.ts";
import {
	register_payload_patterns,
	start_speculative_loaders,
	type ClientLoaderFn,
	type ClientLoaderPrefetch,
	type LoaderRegistry,
	type MatchHooks,
} from "./runtime-route-prep.ts";
export type SubmitWaiterResult =
	| {
			success: true;
			data: unknown;
			response: Response | undefined;
			revalidation_call_id: PublicCallID | null;
	  }
	| {
			success: false;
			error: string;
			response: Response | undefined;
			revalidation_call_id: PublicCallID | null;
	  };

type RevalidationWaiterResult =
	| { ok: true }
	| { ok: false; reason: "build_skew" | "max_retries_exhausted" };

export type Runtime = {
	dispatch: (event: InputEvent) => void;
	get_model: () => Model;
	get_route_state: () => RouteState | null;
	get_work_state: () => WorkState;
	boot_complete: () => Promise<Result<void>>;
	configure_work_indicator: (
		options: WorkIndicatorOptions | undefined,
	) => void;
	track_work_indicator: <T>(promise: PromiseLike<T>) => Promise<T>;
	work_indicator_is_active: () => boolean;
	mint_call_id: () => PublicCallID;
	register_navigation_waiter: (
		id: PublicCallID,
	) => Promise<{ didNavigate: boolean }>;
	register_revalidate_waiter: (
		id: PublicCallID,
	) => Promise<RevalidationWaiterResult>;
	register_submit_waiter: (id: PublicCallID) => Promise<SubmitWaiterResult>;
	save_current_scroll: () => void;
	register_hooks_for_match: (id: ClientLoaderID, hooks: MatchHooks) => void;
	register_view: (
		pattern: string,
		client_loader: ClientLoaderFn | undefined,
		run_client_loader_on_hmr: boolean,
	) => void;
};

type Deferred<T> = {
	promise: Promise<T>;
	resolve: (value: T) => void;
};

function make_deferred<T>(): Deferred<T> {
	let resolve!: (v: T) => void;
	const promise = new Promise<T>((r) => {
		resolve = r;
	});
	return { promise, resolve };
}

export type RuntimeHooks = {
	on_client_commit?: (commit: ClientCommit) => void;
	on_route_update?: (
		route: RouteState,
		previous: RouteState | null,
		reason: RouteUpdateReason,
	) => void;
	on_work_update?: (work: WorkState) => void;
	on_build_skew?: (event: BuildSkewNotification) => void;
};

export type PrepareRouteInput = {
	payload: RoutePayload;
	href: string;
	history_state: unknown;
	signal: AbortSignal;
	trigger: "boot" | "navigation" | "prefetch" | "revalidation";
	token: RouteToken;
	client_build_id: string;
	speculative_prefetches: readonly ClientLoaderPrefetch[];
};

export type PrepareRouteResult =
	| { kind: "prepared"; prepared: PreparedRoute }
	| { kind: "failed"; error: string }
	| { kind: "aborted" };

export type RuntimeDeps = {
	platform: Platform;
	prepare_route: (input: PrepareRouteInput) => Promise<PrepareRouteResult>;
	loader_registry: LoaderRegistry;
	search_schemas: SearchSchemaRegistry;
	resolve_route_module: (
		module_url: string,
	) => Record<string, unknown> | undefined;
	hooks: RuntimeHooks;
};

export function create_runtime(deps: RuntimeDeps): Runtime {
	let model: Model = initial_model();

	const navigation_waiters = new Map<
		PublicCallID,
		Deferred<{ didNavigate: boolean }>
	>();
	const revalidate_waiters = new Map<
		PublicCallID,
		Deferred<RevalidationWaiterResult>
	>();
	const settled_revalidations = new Map<
		PublicCallID,
		RevalidationWaiterResult
	>();
	const submit_waiters = new Map<
		PublicCallID,
		Deferred<SubmitWaiterResult>
	>();
	const api_responses_by_token = new Map<APIToken, Response>();
	const call_id_by_api_token = new Map<APIToken, PublicCallID>();
	let last_work_json = JSON.stringify(derive_work_state(model));

	const abort_controllers = new Map<AbortHandle, AbortController>();
	const timers = new Map<RefreshTimerID, TimerHandle>();
	const hooks_by_client_loader_id = new Map<ClientLoaderID, MatchHooks>();
	let work_indicator_options: WorkIndicatorOptions | undefined;

	const pattern_registry_result = createPatternRegistry({
		dynamicParamPrefixRune: ":",
		splatSegmentRune: "*",
		explicitIndexSegment: "_index",
	});
	if (!pattern_registry_result.ok) {
		throw new Error(
			`Failed to create pattern registry: ${pattern_registry_result.err}`,
		);
	}
	const pattern_registry: PatternRegistry = pattern_registry_result.val;

	const speculative_prefetches_by_token = new Map<
		RouteToken,
		ClientLoaderPrefetch[]
	>();

	function register_hooks_for_match(
		id: ClientLoaderID,
		hooks: MatchHooks,
	): void {
		hooks_by_client_loader_id.set(id, hooks);
	}

	let public_call_counter = 0;

	function mint_call_id(): PublicCallID {
		public_call_counter++;
		return `rt-call-${public_call_counter}` as PublicCallID;
	}

	let boot_resolve: ((result: Result<void>) => void) | undefined;
	const boot_promise = new Promise<Result<void>>((resolve) => {
		boot_resolve = resolve;
	});

	function boot_complete(): Promise<Result<void>> {
		return boot_promise;
	}

	function configure_work_indicator(
		options: WorkIndicatorOptions | undefined,
	): void {
		const previous_options = work_indicator_options;
		clear_work_indicator_show_timer();
		clear_work_indicator_hide_timer();
		if (
			work_indicator_visible &&
			previous_options &&
			previous_options !== options
		) {
			previous_options.stop();
			work_indicator_visible = false;
		}
		work_indicator_options = options;
		sync_work_indicator_renderer();
	}

	function get_work_state(): WorkState {
		return derive_work_state(model);
	}

	function get_route_state(): RouteState | null {
		if (model.phase === "ready") {
			return model.current.route_state;
		}
		if (model.phase !== "booting") {
			return null;
		}
		if (model.current) {
			return model.current.route_state;
		}
		const active = model.active_route;
		if (!active) {
			return null;
		}
		if (active.phase === "publishing") {
			return route_state_from_prepared(
				active.prepared,
				active.url,
				model.browser.state,
			);
		}
		if (active.phase !== "preparing") {
			return null;
		}
		const server_error_idx = active.payload.routes.findIndex((route) => {
			return route.server_error !== undefined;
		});
		return {
			href: active.url,
			historyState: model.browser.state,
			clientBuildID: model.config.client_build_id,
			params: active.payload.params,
			splatValues: [...active.payload.splat_values],
			matches: active.payload.routes.map((route) => {
				return {
					pattern: route.pattern,
					input: route.input,
					loaderData: route.loader_data,
					clientLoaderData: undefined,
				};
			}),
			error:
				server_error_idx === -1
					? null
					: {
							idx: server_error_idx,
							error: active.payload.routes[server_error_idx]!
								.server_error,
							source: "server",
						},
		};
	}

	function register_navigation_waiter(
		id: PublicCallID,
	): Promise<{ didNavigate: boolean }> {
		const d = make_deferred<{ didNavigate: boolean }>();
		navigation_waiters.set(id, d);
		return d.promise;
	}

	function register_revalidate_waiter(
		id: PublicCallID,
	): Promise<RevalidationWaiterResult> {
		const settled = settled_revalidations.get(id);
		if (settled) {
			settled_revalidations.delete(id);
			return Promise.resolve(settled);
		}
		const existing = revalidate_waiters.get(id);
		if (existing) {
			return existing.promise;
		}
		const d = make_deferred<RevalidationWaiterResult>();
		revalidate_waiters.set(id, d);
		return d.promise;
	}

	function register_submit_waiter(
		id: PublicCallID,
	): Promise<SubmitWaiterResult> {
		const d = make_deferred<SubmitWaiterResult>();
		submit_waiters.set(id, d);
		return d.promise;
	}

	/////// Dispatch

	let dispatching = false;
	const queue: InputEvent[] = [];

	function dispatch(event: InputEvent): void {
		queue.push(event);
		if (dispatching) {
			return;
		}
		dispatching = true;
		try {
			while (queue.length > 0) {
				const next = queue.shift()!;
				const out = reduce(model, next, deps.platform.now(), () =>
					mint_abort_handle(),
				);
				model = out.model;
				for (const effect of out.effects) {
					run_effect(effect);
				}
				if (model.phase === "ready" && boot_resolve) {
					boot_resolve(R.ok(undefined));
					boot_resolve = undefined;
				}
			}
		} finally {
			dispatching = false;
		}
	}

	let abort_counter = 0;
	function mint_abort_handle(): AbortHandle {
		abort_counter++;
		const handle = abort_counter as unknown as AbortHandle;
		const controller = new AbortController();
		abort_controllers.set(handle, controller);
		return handle;
	}

	function get_signal(handle: AbortHandle): AbortSignal {
		const controller = abort_controllers.get(handle);
		if (!controller) {
			const c = new AbortController();
			c.abort();
			return c.signal;
		}
		return controller.signal;
	}

	function trigger_abort(handle: AbortHandle): void {
		const controller = abort_controllers.get(handle);
		if (!controller) {
			return;
		}
		controller.abort();
		abort_controllers.delete(handle);
	}

	function route_token_is_live(token: RouteToken): boolean {
		if (model.phase !== "ready" && model.phase !== "booting") {
			return false;
		}
		if (model.active_route?.token === token) {
			return true;
		}
		return model.phase === "ready" && model.prefetch?.token === token;
	}

	function abort_handle_is_live(handle: AbortHandle): boolean {
		if (model.phase !== "ready" && model.phase !== "booting") {
			return false;
		}
		if (model.active_route?.abort_handle === handle) {
			return true;
		}
		if (
			model.phase === "ready" &&
			model.prefetch &&
			model.prefetch.phase !== "prepared" &&
			model.prefetch.abort_handle === handle
		) {
			return true;
		}
		return Object.values(model.submissions).some((submission) => {
			return submission.abort_handle === handle;
		});
	}

	function release_abort_if_unused(handle: AbortHandle): void {
		if (!abort_handle_is_live(handle)) {
			abort_controllers.delete(handle);
		}
	}

	function discard_speculative_prefetches(token: RouteToken): void {
		const stashed = speculative_prefetches_by_token.get(token);
		if (!stashed) {
			return;
		}
		for (const pf of stashed) {
			pf.abort();
		}
		speculative_prefetches_by_token.delete(token);
	}

	function run_effect(effect: Effect): void {
		switch (effect.type) {
			case "fetch_route":
				void run_fetch_route(effect);
				return;
			case "prepare_route":
				void run_prepare_route(effect);
				return;
			case "fetch_api": {
				const submission =
					model.phase === "booting" || model.phase === "ready"
						? model.submissions[effect.token]
						: undefined;
				if (submission) {
					call_id_by_api_token.set(
						effect.token,
						submission.public_call_id,
					);
				}
				void run_fetch_api(effect);
				return;
			}
			case "abort":
				trigger_abort(effect.handle);
				return;
			case "start_timer": {
				const handle = deps.platform.set_timeout(() => {
					timers.delete(effect.id);
					dispatch({ type: "refresh_timer_fired", id: effect.id });
				}, effect.ms);
				timers.set(effect.id, handle);
				return;
			}
			case "clear_timer": {
				const handle = timers.get(effect.id);
				if (handle !== undefined) {
					deps.platform.clear_timeout(handle);
					timers.delete(effect.id);
				}
				return;
			}
			case "publish_route":
				void run_publish_route(effect);
				return;
			case "apply_scroll":
				apply_scroll(effect.scroll);
				return;
			case "apply_history":
				apply_history_action(effect.action);
				return;
			case "save_scroll":
				save_scroll_for_key(effect.browser_key, effect.position);
				return;
			case "save_current_scroll":
				save_scroll_for_key(
					effect.browser_key,
					deps.platform.current_position(),
				);
				return;
			case "write_reload_scroll":
				write_reload_scroll(
					effect.href,
					effect.position,
					effect.now_ms,
				);
				return;
			case "hard_redirect":
				deps.platform.hard_redirect(effect.url);
				return;
			case "reload":
				deps.platform.reload();
				return;
			case "emit_client_commit":
				emit_client_commit(materialize_client_commit(effect.commit));
				return;
			case "notify_build_skew":
				deps.hooks.on_build_skew?.(effect.event);
				return;
			case "notify_work_update": {
				const work_json = JSON.stringify(effect.work);
				if (work_json === last_work_json) {
					return;
				}
				last_work_json = work_json;
				deps.hooks.on_work_update?.(effect.work);
				deps.hooks.on_client_commit?.({ work: effect.work });
				return;
			}
			case "sync_work_indicator":
				sync_work_indicator(effect.should_be_active);
				return;
			case "settle_navigation_call": {
				const d = navigation_waiters.get(effect.call_id);
				if (d) {
					d.resolve(effect.result);
					navigation_waiters.delete(effect.call_id);
				}
				return;
			}
			case "settle_revalidate_call": {
				const d = revalidate_waiters.get(effect.call_id);
				if (d) {
					d.resolve(effect.result);
					revalidate_waiters.delete(effect.call_id);
				} else {
					settled_revalidations.set(effect.call_id, effect.result);
				}
				return;
			}
			case "settle_submit_call":
				settle_submit_call_effect(effect);
				return;
			case "settle_boot":
				if (boot_resolve) {
					boot_resolve(effect.result);
					boot_resolve = undefined;
				}
				return;
			case "dispatch_navigation": {
				dispatch({
					type: "public_navigate",
					call_id: mint_call_id(),
					href: effect.href,
					replace: effect.replace,
					scroll_to_top: undefined,
					state: effect.state,
					skip_work_indicator: false,
				});
				return;
			}
			case "warn_redirect_loop":
				console.warn(
					`Vorma: redirect loop detected. Stopped after ${effect.redirect_count} redirects. Last URL: ${effect.url}.`,
				);
				return;
		}
	}

	function settle_submit_call_effect(
		effect: Extract<Effect, { type: "settle_submit_call" }>,
	): void {
		// Register before resolving the submit waiter; callers receive
		// the revalidation call ID through that resolution.
		if (effect.result.revalidation_call_id) {
			void register_revalidate_waiter(effect.result.revalidation_call_id);
		}
		const d = submit_waiters.get(effect.call_id);
		if (!d) {
			return;
		}
		let response: Response | undefined;
		let owning_token: APIToken | undefined;
		for (const [token, cid] of call_id_by_api_token) {
			if (cid === effect.call_id) {
				owning_token = token;
				response = api_responses_by_token.get(token);
				break;
			}
		}
		if (owning_token !== undefined) {
			api_responses_by_token.delete(owning_token);
			call_id_by_api_token.delete(owning_token);
		}
		if (effect.result.success) {
			d.resolve({
				success: true,
				data: effect.result.data,
				response,
				revalidation_call_id: effect.result.revalidation_call_id,
			});
		} else {
			d.resolve({
				success: false,
				error: effect.result.error,
				response,
				revalidation_call_id: effect.result.revalidation_call_id,
			});
		}
		submit_waiters.delete(effect.call_id);
	}

	function emit_client_commit(commit: ClientCommit): void {
		deps.hooks.on_client_commit?.(commit);
		if (commit.route_update) {
			deps.hooks.on_route_update?.(
				commit.route_update.route,
				commit.route_update.previous_route,
				commit.route_update.reason,
			);
		}
	}

	function materialize_client_commit(plan: ClientCommitPlan): ClientCommit {
		const commit: ClientCommit = {};
		if (plan.route_update) {
			commit.route_update = plan.route_update;
		}
		if (plan.work) {
			commit.work = plan.work;
		}
		if (plan.route_render) {
			commit.route_render = {
				state: materialize_route_render_state(plan.route_render.state),
				scroll_intent: plan.route_render.scroll_intent,
			};
		}
		return commit;
	}

	function materialize_route_render_state(
		state: RouteRenderPlanState,
	): RouteRenderState {
		return {
			entries: state.entries.map((entry) => {
				return {
					...entry,
					module: deps.resolve_route_module(entry.module_url) ?? {},
				};
			}),
			error: state.error,
			params: state.params,
			splat_values: state.splat_values,
			client_build_id: state.client_build_id,
			history_state: state.history_state,
		};
	}

	function apply_scroll(scroll: import("./model.ts").ScrollState): void {
		if ("hash" in scroll) {
			const raw = scroll.hash.startsWith("#")
				? scroll.hash.slice(1)
				: scroll.hash;
			let id = raw;
			try {
				id = decodeURIComponent(raw);
			} catch {}
			deps.platform.scroll_to_element_id(id);
		} else {
			deps.platform.scroll_to_xy(scroll.x, scroll.y);
		}
	}

	function apply_history_action(action: HistoryAction): void {
		if (action.kind === "none") {
			return;
		}
		const next_state = {
			[HISTORY_KEY_FIELD]: action.browser_key,
			[HISTORY_USER_STATE_FIELD]: action.state,
		};
		if (action.kind === "push") {
			deps.platform.push(action.href, next_state);
		} else {
			const existing = deps.platform.history_state();
			const base =
				existing && typeof existing === "object" ? existing : {};
			deps.platform.replace(action.href, {
				...(base as object),
				...next_state,
			});
		}
	}

	/////// Scroll Storage

	const MAX_SCROLL_ENTRIES = 50;

	function read_scroll_entries(): Array<[string, { x: number; y: number }]> {
		const raw = deps.platform.get(SCROLL_STORAGE_KEY);
		if (!raw) {
			return [];
		}
		try {
			const parsed = JSON.parse(raw);
			if (!Array.isArray(parsed)) {
				return [];
			}
			return parsed.filter(
				(e): e is [string, { x: number; y: number }] => {
					return (
						Array.isArray(e) &&
						e.length === 2 &&
						typeof e[0] === "string" &&
						!!e[1] &&
						typeof e[1] === "object" &&
						Number.isFinite((e[1] as { x: number }).x) &&
						Number.isFinite((e[1] as { y: number }).y)
					);
				},
			);
		} catch {
			return [];
		}
	}

	function save_scroll_for_key(
		key: BrowserKey | string,
		position: { x: number; y: number },
	): void {
		const entries = read_scroll_entries().filter((e) => e[0] !== key);
		entries.push([key as string, position]);
		if (entries.length > MAX_SCROLL_ENTRIES) {
			entries.splice(0, entries.length - MAX_SCROLL_ENTRIES);
		}
		deps.platform.set(SCROLL_STORAGE_KEY, JSON.stringify(entries));
	}

	function save_current_scroll(): void {
		if (model.phase !== "ready") {
			return;
		}
		save_scroll_for_key(
			model.browser.key,
			deps.platform.current_position(),
		);
	}

	function write_reload_scroll(
		href: string,
		position: { x: number; y: number },
		now_ms: number,
	): void {
		deps.platform.set(
			SCROLL_STORAGE_RELOAD_KEY,
			JSON.stringify({ ...position, href, unix: now_ms }),
		);
	}

	function is_abort_error(error: unknown): boolean {
		return (
			error instanceof DOMException &&
			(error.name === "AbortError" || error.message === "Aborted")
		);
	}

	/////// Response Classification

	function classify_response(
		response: Response,
		requested_href: string,
	): {
		has_build_skew_header: boolean;
		redirect: { href: string; hard: boolean } | null;
		server_build_id: string;
	} {
		const has_build_skew_header =
			response.headers.get(X_VORMA_BUILD_SKEW) === VORMA_PROTOCOL_ENABLED;
		const soft_redirect = response.headers.get(X_CLIENT_REDIRECT);
		let redirect: { href: string; hard: boolean } | null = null;
		if (soft_redirect) {
			try {
				redirect = {
					href: new URL(soft_redirect, requested_href).href,
					hard: false,
				};
			} catch {}
		} else if (
			response.redirected &&
			response.url &&
			response.url !== requested_href
		) {
			redirect = { href: response.url, hard: false };
		}
		return {
			has_build_skew_header,
			redirect,
			server_build_id: response.headers.get(BUILD_ID_HEADER) ?? "",
		};
	}

	async function run_fetch_route(
		effect: Extract<Effect, { type: "fetch_route" }>,
	): Promise<void> {
		const signal = get_signal(effect.abort_handle);
		const trigger_for_prefetch =
			effect.trigger === "prefetch"
				? "prefetch"
				: effect.trigger === "revalidation"
					? "revalidation"
					: effect.trigger === "popstate"
						? "popstate"
						: "navigation";
		try {
			speculative_prefetches_by_token.set(
				effect.token,
				start_speculative_loaders(
					{
						pattern_registry,
						loader_registry: deps.loader_registry,
						search_schemas: deps.search_schemas,
					},
					{
						url: new URL(effect.url),
						href: effect.url,
						history_state: undefined,
						signal,
						trigger: trigger_for_prefetch,
					},
				),
			);
		} catch {
			speculative_prefetches_by_token.set(effect.token, []);
		}

		const url = new URL(effect.url);
		url.searchParams.set(VORMA_JSON_KEY, effect.client_build_id);
		if (effect.is_revalidation && effect.deployment_id) {
			url.searchParams.set(
				VERCEL_DPL_QUERY_PARAM_KEY,
				effect.deployment_id,
			);
		}

		let outcome: RouteFetchOutcome;
		try {
			const response = await deps.platform.fetch(url.href, {
				headers: {
					[X_ACCEPTS_CLIENT_REDIRECT]: VORMA_PROTOCOL_ENABLED,
				},
				signal,
			});
			const c = classify_response(response, effect.url);
			if (c.has_build_skew_header) {
				outcome = {
					kind: "build_skew",
					server_build_id: c.server_build_id,
					redirect: c.redirect,
					status: response.status,
					ok: response.ok,
				};
			} else if (c.redirect) {
				outcome = {
					kind: "redirect_only",
					redirect: c.redirect,
					server_build_id: c.server_build_id,
					status: response.status,
					ok: response.ok,
				};
			} else if (!response.ok) {
				outcome = {
					kind: "http_error",
					status: response.status,
					server_build_id: c.server_build_id,
				};
			} else {
				try {
					const data = await response.json();
					const payload = decode_route_payload(
						data,
						new URL(effect.url),
						{
							search_schemas: deps.search_schemas,
							decode_html_entities:
								deps.platform.decode_html_entities,
						},
					);
					register_payload_patterns(pattern_registry, payload);
					outcome = {
						kind: "data",
						payload,
						server_build_id: c.server_build_id,
						redirect: c.redirect,
						ok: response.ok,
						status: response.status,
						has_build_skew_header: c.has_build_skew_header,
					};
				} catch {
					outcome = {
						kind: "http_error",
						status: response.status,
						server_build_id: c.server_build_id,
					};
				}
			}
		} catch (err) {
			if (signal.aborted) {
				outcome = { kind: "aborted" };
			} else {
				outcome = {
					kind: "network_error",
					error: err instanceof Error ? err.message : String(err),
				};
			}
		}

		if (outcome.kind !== "data") {
			discard_speculative_prefetches(effect.token);
		}
		dispatch({ type: "route_fetch_settled", token: effect.token, outcome });
		if (!route_token_is_live(effect.token)) {
			discard_speculative_prefetches(effect.token);
		}
		release_abort_if_unused(effect.abort_handle);
	}

	async function run_prepare_route(
		effect: Extract<Effect, { type: "prepare_route" }>,
	): Promise<void> {
		const signal = get_signal(effect.abort_handle);
		const speculative_prefetches =
			speculative_prefetches_by_token.get(effect.token) ?? [];
		speculative_prefetches_by_token.delete(effect.token);
		try {
			register_payload_patterns(pattern_registry, effect.payload);
			const result = await deps.prepare_route({
				payload: effect.payload,
				href: effect.href,
				history_state: effect.history_state,
				signal,
				trigger: effect.trigger,
				token: effect.token,
				speculative_prefetches,
				client_build_id: effect.client_build_id,
			});
			dispatch({
				type: "route_preparation_settled",
				token: effect.token,
				outcome: result,
			});
			release_abort_if_unused(effect.abort_handle);
		} catch (err) {
			dispatch({
				type: "route_preparation_settled",
				token: effect.token,
				outcome: {
					kind: "failed",
					error: err instanceof Error ? err.message : String(err),
				},
			});
			release_abort_if_unused(effect.abort_handle);
		}
	}

	async function run_fetch_api(
		effect: Extract<Effect, { type: "fetch_api" }>,
	): Promise<void> {
		const signal = get_signal(effect.abort_handle);
		let dispatched = false;
		let outcome: APIFetchOutcome;
		try {
			const init = normalize_api_request_init(effect, signal);
			dispatched = true;
			const response = await deps.platform.fetch(effect.url, init);
			const c = classify_response(response, effect.url);
			if (c.redirect) {
				outcome = {
					kind: "redirect",
					redirect: c.redirect,
					server_build_id: c.server_build_id,
					status: response.status,
					ok: response.ok,
				};
			} else if (!response.ok) {
				outcome = {
					kind: "http_error",
					status: response.status,
					status_text: response.statusText,
					server_build_id: c.server_build_id,
				};
			} else {
				let data: unknown;
				if (response.status !== 204) {
					const ct = response.headers
						.get("Content-Type")
						?.toLowerCase();
					if (ct?.includes("json")) {
						data = await response.json();
					} else {
						const text = await response.text();
						data = text.length > 0 ? text : undefined;
					}
				}
				outcome = {
					kind: "ok",
					data,
					server_build_id: c.server_build_id,
					redirect: c.redirect,
					status: response.status,
				};
			}
			if (call_id_by_api_token.has(effect.token)) {
				api_responses_by_token.set(effect.token, response);
			}
		} catch (err) {
			if (signal.aborted || is_abort_error(err)) {
				outcome = { kind: "aborted", dispatched };
			} else {
				outcome = {
					kind: "network_error",
					error: err instanceof Error ? err.message : String(err),
					dispatched,
				};
			}
		}
		dispatch({ type: "api_fetch_settled", token: effect.token, outcome });
		release_abort_if_unused(effect.abort_handle);
	}

	function normalize_api_request_init(
		effect: Extract<Effect, { type: "fetch_api" }>,
		signal: AbortSignal,
	): RequestInit & { signal: AbortSignal } {
		const headers = new Headers(effect.request_init?.headers);
		if (effect.deployment_id) {
			headers.set(VERCEL_X_DEPLOYMENT_ID, effect.deployment_id);
		}
		headers.set(X_ACCEPTS_CLIENT_REDIRECT, VORMA_PROTOCOL_ENABLED);
		const method = effect.method;
		const body = effect.request_init?.body;
		const is_get = method === "GET" || method === "HEAD";
		const should_json =
			!is_get &&
			body &&
			typeof body === "object" &&
			!(body instanceof ReadableStream) &&
			!(body instanceof FormData) &&
			!(body instanceof URLSearchParams) &&
			!(body instanceof Blob) &&
			!(body instanceof ArrayBuffer) &&
			!ArrayBuffer.isView(body);
		const init: RequestInit & { signal: AbortSignal } = {
			...effect.request_init,
			headers,
			method,
			signal,
		};
		if (is_get) {
			delete init.body;
		} else if (should_json) {
			init.body = JSON.stringify(body);
			if (!headers.has(CONTENT_TYPE_HEADER)) {
				headers.set(CONTENT_TYPE_HEADER, JSON_CONTENT_TYPE);
			}
		}
		return init;
	}

	async function run_publish_route(
		effect: Extract<Effect, { type: "publish_route" }>,
	): Promise<void> {
		try {
			if (
				effect.use_view_transition &&
				deps.platform.supports_view_transitions()
			) {
				let publication: Promise<void> | undefined;
				const transition = deps.platform.start_view_transition(() => {
					publication = run_publish_transaction(effect);
					publication.catch(() => {});
					return publication;
				});
				await (transition.updateCallbackDone ?? transition.finished);
				if (publication) {
					await publication;
				}
				if (transition.finished) {
					await transition.finished;
				}
			} else {
				await run_publish_transaction(effect);
			}
		} catch (err) {
			dispatch({
				type: "publication_failed",
				token: effect.token,
				error: err instanceof Error ? err.message : String(err),
			});
		} finally {
			if (effect.abort_handle !== null) {
				release_abort_if_unused(effect.abort_handle);
			}
		}
	}

	async function run_publish_transaction(
		effect: Extract<Effect, { type: "publish_route" }>,
	): Promise<void> {
		const signal = publication_signal(effect);
		assert_publication_current(effect.token, signal);
		const commit = materialize_client_commit(effect.commit);
		const current_route =
			model.phase !== "uninitialized" && model.current
				? model.current.route_state
				: null;
		const next_route = effect.commit.route_update?.route ?? current_route;
		if (current_route && next_route) {
			const hook_args = {
				trigger: effect.hook_trigger,
				signal,
				current: current_route,
				next: next_route,
			};
			const yield_promises: Array<Promise<void> | void> = [];
			for (const id of effect.run_yield_hooks_for) {
				const hooks = hooks_by_client_loader_id.get(id);
				if (hooks?.before_yield) {
					yield_promises.push(hooks.before_yield(hook_args));
				}
			}
			const commit_promises: Array<Promise<void> | void> = [];
			for (const id of effect.run_commit_hooks_for) {
				const hooks = hooks_by_client_loader_id.get(id);
				if (hooks?.before_commit) {
					commit_promises.push(hooks.before_commit(hook_args));
				}
			}
			await Promise.all([...yield_promises, ...commit_promises]);
		}
		assert_publication_current(effect.token, signal);
		if (effect.apply_dom_side_effects) {
			deps.platform.apply_head_and_title(
				effect.apply_dom_side_effects.title,
				[...effect.apply_dom_side_effects.meta_head_els],
				[...effect.apply_dom_side_effects.rest_head_els],
			);
			deps.platform.apply_css_bundles([
				...effect.apply_dom_side_effects.css_bundles,
			]);
			deps.platform.preload_modules([
				...effect.apply_dom_side_effects.deps,
			]);
		}
		if (effect.save_current_scroll && model.phase !== "uninitialized") {
			save_scroll_for_key(
				model.browser.key,
				deps.platform.current_position(),
			);
		}
		apply_history_action(effect.history_action);
		assert_publication_current(effect.token, signal);
		dispatch({ type: "publication_committed", token: effect.token });
		emit_client_commit(commit);
		if (effect.scroll) {
			apply_scroll(effect.scroll);
		}
	}

	function publication_signal(
		effect: Extract<Effect, { type: "publish_route" }>,
	): AbortSignal {
		if (effect.abort_handle === null) {
			return new AbortController().signal;
		}
		return get_signal(effect.abort_handle);
	}

	function assert_publication_current(
		token: RouteToken,
		signal: AbortSignal,
	): void {
		if (signal.aborted || !publication_is_current(token)) {
			throw new DOMException("Aborted", "AbortError");
		}
	}

	function publication_is_current(token: RouteToken): boolean {
		if (model.phase === "uninitialized") {
			return false;
		}
		return (
			model.active_route?.phase === "publishing" &&
			model.active_route.token === token
		);
	}

	/////// Work Indicator

	let work_indicator_visible = false;
	let work_indicator_show_timer: TimerHandle | undefined;
	let work_indicator_hide_timer: TimerHandle | undefined;
	const work_indicator_active_tokens = new Set<symbol>();
	let release_vorma_work_indicator: (() => void) | undefined;

	function sync_work_indicator(should_be_active: boolean): void {
		set_vorma_work_indicator_active(should_be_active);
	}

	function track_work_indicator<T>(promise: PromiseLike<T>): Promise<T> {
		const release = begin_work_indicator_activity();
		return Promise.resolve(promise).finally(() => {
			release();
		});
	}

	function work_indicator_is_active(): boolean {
		return work_indicator_active_tokens.size > 0;
	}

	function set_vorma_work_indicator_active(active: boolean): void {
		if (active) {
			if (!release_vorma_work_indicator) {
				release_vorma_work_indicator = begin_work_indicator_activity();
			}
			return;
		}
		if (!release_vorma_work_indicator) {
			sync_work_indicator_renderer();
			return;
		}
		release_vorma_work_indicator();
		release_vorma_work_indicator = undefined;
	}

	function begin_work_indicator_activity(): () => void {
		const token = Symbol();
		let released = false;
		work_indicator_active_tokens.add(token);
		sync_work_indicator_renderer();
		return () => {
			if (released) {
				return;
			}
			released = true;
			work_indicator_active_tokens.delete(token);
			sync_work_indicator_renderer();
		};
	}

	function clear_work_indicator_show_timer(): void {
		if (work_indicator_show_timer === undefined) {
			return;
		}
		deps.platform.clear_timeout(work_indicator_show_timer);
		work_indicator_show_timer = undefined;
	}

	function clear_work_indicator_hide_timer(): void {
		if (work_indicator_hide_timer === undefined) {
			return;
		}
		deps.platform.clear_timeout(work_indicator_hide_timer);
		work_indicator_hide_timer = undefined;
	}

	function sync_work_indicator_renderer(): void {
		const opts = work_indicator_options;
		if (!opts) {
			clear_work_indicator_show_timer();
			clear_work_indicator_hide_timer();
			return;
		}
		if (work_indicator_active_tokens.size > 0) {
			clear_work_indicator_hide_timer();
			if (
				work_indicator_visible ||
				work_indicator_show_timer !== undefined
			) {
				return;
			}
			work_indicator_show_timer = deps.platform.set_timeout(() => {
				work_indicator_show_timer = undefined;
				if (work_indicator_visible) {
					return;
				}
				opts.start();
				work_indicator_visible = true;
			}, opts.startDelayMS ?? 12);
		} else {
			clear_work_indicator_show_timer();
			if (
				!work_indicator_visible ||
				work_indicator_hide_timer !== undefined
			) {
				return;
			}
			work_indicator_hide_timer = deps.platform.set_timeout(() => {
				work_indicator_hide_timer = undefined;
				if (!work_indicator_visible) {
					return;
				}
				opts.stop();
				work_indicator_visible = false;
			}, opts.stopDelayMS ?? 12);
		}
	}

	function register_view(
		pattern: string,
		client_loader: ClientLoaderFn | undefined,
		run_client_loader_on_hmr: boolean,
	): void {
		deps.loader_registry.set(pattern, client_loader);
		deps.loader_registry.set_rerun_on_hmr(
			pattern,
			run_client_loader_on_hmr,
		);
		registerPattern(pattern_registry, pattern);
	}

	return {
		dispatch,
		get_model: () => model,
		get_route_state,
		get_work_state,
		boot_complete,
		configure_work_indicator,
		track_work_indicator,
		work_indicator_is_active,
		mint_call_id,
		register_navigation_waiter,
		register_revalidate_waiter,
		register_submit_waiter,
		save_current_scroll,
		register_hooks_for_match,
		register_view,
	};
}
