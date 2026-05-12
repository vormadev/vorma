import { R, type Result } from "vorma/kit/result";
import {
	DATA_SCRIPT_ID,
	HISTORY_KEY_FIELD,
	HISTORY_USER_STATE_FIELD,
	SCROLL_STORAGE_KEY,
	SCROLL_STORAGE_RELOAD_KEY,
	VORMA_ROOT_EL_ID,
} from "../core/constants.ts";
import { apply_css_bundles, preload_css, wait_for_css } from "../core/css.ts";
import { apply_head_and_title, type HeadEl } from "../core/head.ts";
import { preload_modules } from "../core/modules.ts";
import type {
	APIRouteKind,
	AppConfig,
	BeforeRouteCommitFn,
	BeforeRouteYieldFn,
} from "../core/types.ts";
import type { BrowserKey } from "./events.ts";
import type {
	ClientCommit,
	RevalidationResult,
	RouteState,
	WorkIndicatorOptions,
} from "./model.ts";
import {
	browser_decode_html_entities,
	create_search_schema_registry,
	decode_route_payload,
} from "./payload.ts";
import type { AbortHandle, Platform, TimerHandle } from "./platform.ts";
import { is_same_document } from "./reducer-core.ts";
import {
	create_loader_registry,
	create_module_cache,
	module_cache_key,
	prepare_route as prepare_route_subsystem,
	type ClientLoaderFn,
	type ClientLoaderServerState,
} from "./runtime-route-prep.ts";
import {
	create_runtime,
	type PrepareRouteInput,
	type PrepareRouteResult,
	type RuntimeHooks,
} from "./runtime.ts";

/////////////////////////////////////////////////////////////////////
/////// Public Types
/////////////////////////////////////////////////////////////////////

export type ViewDefinition = {
	pattern: string;
	component: (props: any) => any;
	error_boundary?: (props: { error: unknown }) => any;
	client_loader?: ClientLoaderFn;
	before_route_commit?: BeforeRouteCommitFn;
	before_route_yield?: BeforeRouteYieldFn;
};

export type ClientOptions = {
	render?: () => void | Promise<void>;
	workIndicator?: WorkIndicatorOptions;
	revalidateOnWindowFocus?:
		| boolean
		| { staleTimeMS: number; skipWorkIndicator?: boolean };
	defaultErrorBoundary?: (props: { error: unknown }) => any;
	useViewTransitions?: boolean;
	onRouteUpdate?: (
		route: RouteState,
		previousRoute: RouteState | null,
		reason: "boot" | "navigation" | "popstate" | "revalidation",
	) => void;
	onWorkUpdate?: (work: import("./events.ts").WorkState) => void;
	onBuildSkewDetected?: (
		event: import("./events.ts").BuildSkewNotification,
	) => void;
};

export type WorkIndicator = {
	track: <T>(promise: PromiseLike<T>) => Promise<T>;
	isActive: () => boolean;
};

export type ClientCore = {
	boot: (options: ClientOptions) => Promise<Result<void>>;
	workIndicator: WorkIndicator;
	navigate: (
		href: string | URL,
		options?: {
			replace?: boolean;
			scrollToTop?: boolean;
			state?: unknown;
			skipWorkIndicator?: boolean;
		},
	) => Promise<{ didNavigate: boolean }>;
	revalidate: () => Promise<RevalidationResult>;
	submit_inner: <T = unknown>(
		url: string | URL,
		requestInit?: RequestInit,
		options?: {
			apiRouteKind?: APIRouteKind;
			dedupeKey?: string;
			revalidate?: boolean;
			skipWorkIndicator?: boolean;
		},
	) => Promise<SubmitResult<T>>;
	getRouteState: () => RouteState;
	getWorkState: () => import("./events.ts").WorkState;
	getClientBuildID: () => string;
	getRootEl: () => HTMLElement;
	defineView: <T = any>(input: {
		pattern: string;
		component: (props: any) => any;
		errorBoundary?: (props: { error: unknown }) => any;
		clientLoader?: (props: any) => Promise<T>;
		beforeRouteCommit?: BeforeRouteCommitFn;
		beforeRouteYield?: BeforeRouteYieldFn;
		runClientLoaderOnHMR?: boolean;
	}) => ViewDefinition & { __phantom_client_loader_data?: T };
	start_prefetch: (href: string) => void;
	stop_prefetch: (href: string) => void;
	save_current_scroll: () => void;
	get_default_error_boundary: () =>
		| ((props: { error: unknown }) => any)
		| undefined;
};

export type SubmitResult<T = unknown> =
	| {
			success: true;
			data: T;
			response: Response;
			revalidationPromise: Promise<RevalidationResult>;
	  }
	| {
			success: false;
			error: string;
			response?: Response;
			revalidationPromise: Promise<RevalidationResult>;
	  };

let focus_revalidation_cleanup: (() => void) | null = null;

type DevHotContext = {
	dispose: (cleanup: () => void) => void;
};

type DevImportMeta = {
	env?: { DEV?: boolean };
	hot?: DevHotContext;
};

type HMRWindow = Window & {
	__vorma_hmr_route_update?: (
		raw_url: string,
		module: Record<string, unknown>,
	) => Promise<void>;
};

const RELOAD_SCROLL_MAX_AGE_MS = 3333;

/////////////////////////////////////////////////////////////////////
/////// Test Options
/////////////////////////////////////////////////////////////////////

export type TestOptions = {
	reload?: () => void;
	hard_redirect?: (url: string) => void;
	scroll_to?: (x: number, y: number) => void;
};

/////////////////////////////////////////////////////////////////////
/////// Browser Platform
/////////////////////////////////////////////////////////////////////

function create_browser_platform(test_options?: TestOptions): Platform {
	let abort_counter = 0;
	const abort_controllers = new Map<AbortHandle, AbortController>();

	const platform: Platform = {
		now: () => Date.now(),
		random_id: () => Math.random().toString(36).slice(2, 10),
		set_timeout: (cb, ms) => {
			return window.setTimeout(cb, ms) as unknown as TimerHandle;
		},
		clear_timeout: (handle) => {
			window.clearTimeout(handle as unknown as number);
		},
		create_abort: () => {
			abort_counter++;
			const handle = abort_counter as unknown as AbortHandle;
			const controller = new AbortController();
			abort_controllers.set(handle, controller);
			return { handle, signal: controller.signal };
		},
		trigger_abort: (handle) => {
			const controller = abort_controllers.get(handle);
			if (controller) {
				controller.abort();
				abort_controllers.delete(handle);
			}
		},
		fetch: (url, init) => fetch(url, init),
		current_url: () => window.location.href,
		hard_redirect:
			test_options?.hard_redirect ??
			((url) => {
				window.location.assign(url);
			}),
		reload:
			test_options?.reload ??
			(() => {
				window.location.reload();
			}),
		history_state: () => window.history.state,
		push: (url, state) => {
			window.history.pushState(state, "", url);
		},
		replace: (url, state) => {
			window.history.replaceState(state, "", url);
		},
		set_scroll_restoration_manual: () => {
			try {
				window.history.scrollRestoration = "manual";
			} catch {}
		},
		current_position: () => {
			return { x: window.scrollX, y: window.scrollY };
		},
		scroll_to_xy:
			test_options?.scroll_to ??
			((x, y) => {
				window.scrollTo(x, y);
			}),
		scroll_to_element_id: (id) => {
			document.getElementById(id)?.scrollIntoView();
		},
		get: (key) => {
			try {
				return sessionStorage.getItem(key);
			} catch {
				return null;
			}
		},
		set: (key, value) => {
			try {
				sessionStorage.setItem(key, value);
				return R.ok(undefined);
			} catch (err) {
				return R.err(err instanceof Error ? err.message : String(err));
			}
		},
		remove: (key) => {
			try {
				sessionStorage.removeItem(key);
			} catch {}
		},
		ensure_root_element: (id) => {
			const existing = document.getElementById(id);
			if (existing) {
				return existing as HTMLElement;
			}
			const el = document.createElement("div");
			el.id = id;
			document.body.insertBefore(el, document.body.firstChild);
			return el;
		},
		apply_head_and_title: (title, meta_head_els, rest_head_els) => {
			apply_head_and_title(
				title,
				[...meta_head_els] as HeadEl[],
				[...rest_head_els] as HeadEl[],
			);
		},
		preload_css: (bundles) => {
			preload_css([...bundles]);
		},
		apply_css_bundles: (bundles) => {
			apply_css_bundles([...bundles]);
		},
		wait_for_css: (bundles, signal) => {
			return wait_for_css([...bundles], signal);
		},
		preload_modules: (deps) => {
			preload_modules([...deps]);
		},
		import_module: (url) =>
			import(/* @vite-ignore */ url) as Promise<Record<string, unknown>>,
		decode_html_entities: browser_decode_html_entities,
		supports_view_transitions: () => {
			return (
				typeof (
					document as unknown as {
						startViewTransition?: unknown;
					}
				).startViewTransition === "function"
			);
		},
		start_view_transition: (cb) => {
			const start = (
				document as unknown as {
					startViewTransition: (cb: () => void | Promise<void>) => {
						updateCallbackDone?: Promise<void>;
						finished?: Promise<void>;
					};
				}
			).startViewTransition;
			return start.call(document, cb);
		},
		add_window_listener: (event, handler) => {
			window.addEventListener(event, handler);
			return () => {
				window.removeEventListener(event, handler);
			};
		},
		document_visibility_state: () => {
			return document.visibilityState as
				| "visible"
				| "hidden"
				| "prerender";
		},
	};
	return platform;
}

/////////////////////////////////////////////////////////////////////
/////// Factory
/////////////////////////////////////////////////////////////////////

export function create_client_core5(
	_app_config: Omit<AppConfig, "__vormaViews" | "__vormaAPIRoutes">,
	commit: (commit: ClientCommit) => void,
	test_options?: TestOptions,
): Result<ClientCore> {
	const platform = create_browser_platform(test_options);
	const loader_registry = create_loader_registry();
	const module_cache = create_module_cache();
	const search_schemas = create_search_schema_registry();

	let user_on_route_update: ClientOptions["onRouteUpdate"];
	let user_on_work_update: ClientOptions["onWorkUpdate"];
	let user_on_build_skew: ClientOptions["onBuildSkewDetected"];
	let default_error_boundary: ClientOptions["defaultErrorBoundary"];

	const hooks: RuntimeHooks = {
		on_client_commit: (c) => commit(c),
		on_route_update: (route, previous, reason) => {
			user_on_route_update?.(route, previous, reason);
		},
		on_work_update: (work) => {
			user_on_work_update?.(work);
		},
		on_build_skew: (event) => {
			user_on_build_skew?.(event);
		},
	};

	const runtime = create_runtime({
		platform,
		loader_registry,
		search_schemas,
		resolve_route_module: (module_url) => {
			return module_cache.get(
				module_cache_key(module_url, platform.current_url()),
			);
		},
		hooks,
		prepare_route: async (
			input: PrepareRouteInput,
		): Promise<PrepareRouteResult> => {
			return prepare_route_subsystem(
				{
					platform,
					loader_registry,
					module_cache,
					current_base_href: () => platform.current_url(),
					register_hooks_for_match: (id, h) => {
						runtime.register_hooks_for_match(id, h);
					},
				},
				{
					payload: input.payload,
					href: input.href,
					history_state: input.history_state,
					signal: input.signal,
					trigger: input.trigger,
					token: input.token,
					client_build_id: input.client_build_id,
					speculative_prefetches: input.speculative_prefetches,
				},
			);
		},
	});

	const client_build_id_holder = { value: "" };

	/////// Work Indicator
	const work_indicator: WorkIndicator = {
		track: <T>(promise: PromiseLike<T>) => {
			return runtime.track_work_indicator(promise);
		},
		isActive: () => runtime.work_indicator_is_active(),
	};

	function parse_scroll_position(
		value: unknown,
	): { x: number; y: number } | undefined {
		if (!value || typeof value !== "object") {
			return undefined;
		}
		const x = (value as { x?: unknown }).x;
		const y = (value as { y?: unknown }).y;
		if (
			typeof x !== "number" ||
			typeof y !== "number" ||
			!Number.isFinite(x) ||
			!Number.isFinite(y)
		) {
			return undefined;
		}
		return { x, y };
	}

	function read_reload_scroll(
		current_href: string,
	): { x: number; y: number } | undefined {
		try {
			const raw = platform.get(SCROLL_STORAGE_RELOAD_KEY);
			if (!raw) {
				return undefined;
			}
			platform.remove(SCROLL_STORAGE_RELOAD_KEY);
			const parsed = JSON.parse(raw);
			const scroll = parse_scroll_position(parsed);
			if (
				scroll &&
				typeof parsed?.unix === "number" &&
				typeof parsed?.href === "string" &&
				platform.now() - parsed.unix <= RELOAD_SCROLL_MAX_AGE_MS &&
				is_same_document(parsed.href, current_href)
			) {
				return scroll;
			}
		} catch {}
		return undefined;
	}

	/////// Boot

	async function boot(options: ClientOptions): Promise<Result<void>> {
		user_on_route_update = options.onRouteUpdate;
		user_on_work_update = options.onWorkUpdate;
		user_on_build_skew = options.onBuildSkewDetected;
		default_error_boundary = options.defaultErrorBoundary;
		runtime.configure_work_indicator(options.workIndicator);

		const payload_el = document.getElementById(DATA_SCRIPT_ID);
		if (!payload_el) {
			return R.err(`Missing element: #${DATA_SCRIPT_ID}`);
		}
		let raw: unknown;
		try {
			raw = JSON.parse(payload_el.textContent ?? "{}");
		} catch (err) {
			return R.err(err instanceof Error ? err.message : String(err));
		}
		const url = new URL(window.location.href);
		const payload = decode_route_payload(raw, url, {
			search_schemas,
			decode_html_entities: browser_decode_html_entities,
		});
		client_build_id_holder.value = payload.server_build_id;

		// Ensure the current history entry has a key.
		const state = window.history.state;
		const has_key =
			state &&
			typeof state === "object" &&
			HISTORY_KEY_FIELD in (state as Record<string, unknown>);
		const browser_key = (
			has_key
				? (state as Record<string, string>)[HISTORY_KEY_FIELD]
				: random_browser_key()
		) as BrowserKey;
		if (!has_key) {
			const base = state && typeof state === "object" ? state : {};
			window.history.replaceState(
				{
					...(base as object),
					[HISTORY_KEY_FIELD]: browser_key,
				},
				"",
				url.href,
			);
		}
		platform.set_scroll_restoration_manual();
		const restored_scroll = read_reload_scroll(url.href);

		const browser_state =
			state &&
			typeof state === "object" &&
			HISTORY_USER_STATE_FIELD in (state as Record<string, unknown>)
				? (state as Record<string, unknown>)[HISTORY_USER_STATE_FIELD]
				: undefined;

		const work_indicator_policy = options.workIndicator
			? {
					skipNavigations: options.workIndicator.skipNavigations,
					skipAPIRequests: options.workIndicator.skipAPIRequests,
					skipRevalidations: options.workIndicator.skipRevalidations,
				}
			: null;

		runtime.dispatch({
			type: "boot",
			payload,
			browser_key,
			browser_state,
			href: url.href,
			restored_scroll,
			options: {
				use_view_transitions: options.useViewTransitions ?? false,
				revalidate_on_focus:
					typeof options.revalidateOnWindowFocus === "object"
						? {
								stale_time_ms:
									options.revalidateOnWindowFocus.staleTimeMS,
								skip_work_indicator:
									options.revalidateOnWindowFocus
										.skipWorkIndicator ?? false,
							}
						: options.revalidateOnWindowFocus
							? {
									stale_time_ms: 5_000,
									skip_work_indicator: false,
								}
							: null,
				work_indicator: work_indicator_policy,
			},
		});

		const boot_result = await runtime.boot_complete();
		if (!boot_result.ok) {
			return boot_result;
		}

		install_browser_listeners(options);

		if (options.render) {
			await options.render();
		}
		install_dev_hmr();
		return R.ok(undefined);
	}

	function install_browser_listeners(options: ClientOptions): void {
		platform.add_window_listener("popstate", () => {
			const url = new URL(window.location.href);
			const state = window.history.state;
			const key = (
				state &&
				typeof state === "object" &&
				HISTORY_KEY_FIELD in (state as Record<string, unknown>)
					? (state as Record<string, string>)[HISTORY_KEY_FIELD]
					: random_browser_key()
			) as BrowserKey;
			const user_state =
				state &&
				typeof state === "object" &&
				HISTORY_USER_STATE_FIELD in (state as Record<string, unknown>)
					? (state as Record<string, unknown>)[
							HISTORY_USER_STATE_FIELD
						]
					: undefined;
			const previous_position = platform.current_position();
			const model = runtime.get_model();
			let restored_scroll: { x: number; y: number } | undefined;
			const should_restore_scroll =
				model.phase === "ready" &&
				!is_same_document(url.href, model.current.position.href);
			const raw_scroll = should_restore_scroll
				? platform.get(SCROLL_STORAGE_KEY)
				: null;
			if (raw_scroll) {
				try {
					const entries: unknown = JSON.parse(raw_scroll);
					if (Array.isArray(entries)) {
						for (const entry of entries) {
							if (
								!Array.isArray(entry) ||
								entry.length !== 2 ||
								entry[0] !== key
							) {
								continue;
							}
							const scroll = entry[1];
							const parsed_scroll = parse_scroll_position(scroll);
							if (parsed_scroll) {
								restored_scroll = parsed_scroll;
								break;
							}
						}
					}
				} catch {}
			}
			runtime.dispatch({
				type: "popstate",
				browser_key: key,
				href: url.href,
				state: user_state,
				restored_scroll,
				previous_scroll_save:
					model.phase === "ready"
						? {
								key: model.browser.key,
								position: previous_position,
							}
						: undefined,
			});
		});
		platform.add_window_listener("beforeunload", () => {
			const model = runtime.get_model();
			if (model.phase !== "ready") {
				return;
			}
			runtime.dispatch({
				type: "before_unload",
				scroll: platform.current_position(),
				now_ms: platform.now(),
				href: model.browser.href,
			});
		});

		focus_revalidation_cleanup?.();
		focus_revalidation_cleanup = null;
		if (!options.revalidateOnWindowFocus) {
			return;
		}

		const handler = (): void => {
			if (platform.document_visibility_state() !== "visible") {
				return;
			}
			runtime.dispatch({
				type: "focus",
				now_ms: platform.now(),
			});
		};
		const cleanup_focus = platform.add_window_listener("focus", handler);
		const cleanup_visibility = platform.add_window_listener(
			"visibilitychange",
			handler,
		);
		focus_revalidation_cleanup = () => {
			cleanup_focus();
			cleanup_visibility();
		};
	}

	function install_dev_hmr(): void {
		const meta = import.meta as unknown as DevImportMeta;
		if (meta.env?.DEV !== true) {
			return;
		}
		const on_route_update = async (
			raw_url: string,
			module: Record<string, unknown>,
		): Promise<void> => {
			await update_hmr_route(raw_url, module);
		};
		const hmr_window = window as HMRWindow;
		hmr_window.__vorma_hmr_route_update = on_route_update;
		meta.hot?.dispose(() => {
			if (hmr_window.__vorma_hmr_route_update === on_route_update) {
				hmr_window.__vorma_hmr_route_update = undefined;
			}
		});
	}

	async function update_hmr_route(
		raw_url: string,
		module: Record<string, unknown>,
	): Promise<void> {
		const model = runtime.get_model();
		if (model.phase !== "ready") {
			return;
		}
		const current = model.current;
		const module_url = module_cache_key(raw_url, platform.current_url());
		module_cache.set(module_url, module);
		const match_idx = current.prepared.matches.findIndex((match) => {
			return (
				module_cache_key(match.module_url, platform.current_url()) ===
				module_url
			);
		});
		if (match_idx === -1) {
			return;
		}
		const match = current.prepared.matches[match_idx]!;
		const view = module.default as ViewDefinition | undefined;
		const rerun_loader = loader_registry.get_rerun_on_hmr(match.pattern);
		runtime.register_view(match.pattern, view?.client_loader, rerun_loader);
		runtime.register_hooks_for_match(match.client_loader_id, {
			before_yield: view?.before_route_yield,
			before_commit: view?.before_route_commit,
		});

		let client_loader_data = match.client_loader_data;
		const loader = loader_registry.get(match.pattern);
		const route_sequence = current.sequence;
		if (loader && rerun_loader) {
			try {
				const server_error: ClientLoaderServerState["outermostServerError"] =
					current.prepared.error?.source === "server"
						? {
								idx: current.prepared.error.idx,
								error: current.prepared.error.error,
							}
						: null;
				client_loader_data = await loader({
					trigger: "revalidation",
					href: current.route_state.href,
					historyState: current.route_state.historyState,
					pattern: match.pattern,
					params: current.route_state.params,
					splatValues: [...current.route_state.splatValues],
					input: match.input,
					knownMatches: current.prepared.matches.map((known) => {
						return {
							pattern: known.pattern,
							input: known.input,
						};
					}),
					serverPromise: Promise.resolve({
						clientBuildID: current.prepared.client_build_id,
						matches: current.prepared.matches.map((known) => {
							return {
								pattern: known.pattern,
								input: known.input,
								loaderData: known.loader_data,
							};
						}),
						outermostServerError: server_error,
						loaderData: match.loader_data,
					}),
					signal: new AbortController().signal,
				});
			} catch (error) {
				console.error("Vorma: HMR client loader re-run failed", error);
			}
		}

		const live_model = runtime.get_model();
		if (
			live_model.phase !== "ready" ||
			live_model.current.sequence !== route_sequence
		) {
			return;
		}
		const live_match = live_model.current.prepared.matches[match_idx];
		if (
			!live_match ||
			module_cache_key(live_match.module_url, platform.current_url()) !==
				module_url
		) {
			return;
		}
		runtime.dispatch({
			type: "hmr_route_update",
			route_sequence,
			match_idx,
			module_url: live_match.module_url,
			client_loader_data,
		});
	}

	function random_browser_key(): string {
		return Math.random().toString(36).slice(2, 10);
	}

	/////// Public Methods

	async function navigate(
		href: string | URL,
		options?: {
			replace?: boolean;
			scrollToTop?: boolean;
			state?: unknown;
			skipWorkIndicator?: boolean;
		},
	): Promise<{ didNavigate: boolean }> {
		const call_id = runtime.mint_call_id();
		const promise = runtime.register_navigation_waiter(call_id);
		runtime.dispatch({
			type: "public_navigate",
			call_id,
			href: String(href),
			replace: options?.replace ?? false,
			scroll_to_top: options?.scrollToTop,
			state: options?.state,
			skip_work_indicator: options?.skipWorkIndicator ?? false,
		});
		return promise;
	}

	async function revalidate(): Promise<RevalidationResult> {
		const call_id = runtime.mint_call_id();
		const promise = runtime.register_revalidate_waiter(call_id);
		runtime.dispatch({
			type: "public_revalidate",
			call_id,
		});
		return promise;
	}

	async function submit_inner<T = unknown>(
		url: string | URL,
		request_init?: RequestInit,
		options?: {
			apiRouteKind?: APIRouteKind;
			dedupeKey?: string;
			revalidate?: boolean;
			skipWorkIndicator?: boolean;
		},
	): Promise<SubmitResult<T>> {
		const call_id = runtime.mint_call_id();
		const method = (request_init?.method ?? "GET").toUpperCase().trim();
		const route_kind =
			options?.apiRouteKind ??
			(method === "GET" || method === "HEAD" ? "query" : "mutation");
		const promise = runtime.register_submit_waiter(call_id);
		runtime.dispatch({
			type: "public_submit",
			call_id,
			href: String(url),
			method,
			request_init,
			route_kind,
			dedupe_key: options?.dedupeKey,
			should_revalidate: options?.revalidate ?? route_kind === "mutation",
			skip_work_indicator: options?.skipWorkIndicator ?? false,
		});
		const settled = await promise;
		if (settled.success) {
			return {
				success: true,
				data: settled.data as T,
				response: settled.response as Response,
				revalidationPromise: settled.revalidation_call_id
					? runtime.register_revalidate_waiter(
							settled.revalidation_call_id,
						)
					: Promise.resolve({ ok: true } as const),
			};
		}
		return {
			success: false,
			error: settled.error,
			response: settled.response,
			revalidationPromise: settled.revalidation_call_id
				? runtime.register_revalidate_waiter(
						settled.revalidation_call_id,
					)
				: Promise.resolve({ ok: true } as const),
		};
	}

	function getRouteState(): RouteState {
		const route_state = runtime.get_route_state();
		if (!route_state) {
			throw new Error("Vorma not booted");
		}
		return route_state;
	}

	function getWorkState(): import("./events.ts").WorkState {
		return runtime.get_work_state();
	}

	function getClientBuildID(): string {
		return client_build_id_holder.value;
	}

	function getRootEl(): HTMLElement {
		return platform.ensure_root_element(VORMA_ROOT_EL_ID);
	}

	function defineView<T = any>(input: {
		pattern: string;
		component: (props: any) => any;
		errorBoundary?: (props: { error: unknown }) => any;
		clientLoader?: (props: any) => Promise<T>;
		beforeRouteCommit?: BeforeRouteCommitFn;
		beforeRouteYield?: BeforeRouteYieldFn;
		runClientLoaderOnHMR?: boolean;
	}): ViewDefinition & { __phantom_client_loader_data?: T } {
		const definition: ViewDefinition = {
			pattern: input.pattern,
			component: input.component,
			error_boundary: input.errorBoundary,
			client_loader: input.clientLoader as ClientLoaderFn | undefined,
			before_route_commit: input.beforeRouteCommit,
			before_route_yield: input.beforeRouteYield,
		};
		runtime.register_view(
			input.pattern,
			definition.client_loader,
			input.runClientLoaderOnHMR ?? false,
		);
		return definition;
	}

	function start_prefetch(href: string): void {
		runtime.dispatch({ type: "public_prefetch_start", href });
	}

	function stop_prefetch(href: string): void {
		runtime.dispatch({ type: "public_prefetch_stop", href });
	}

	function save_current_scroll(): void {
		runtime.save_current_scroll();
	}

	function get_default_error_boundary():
		| ((props: { error: unknown }) => any)
		| undefined {
		return default_error_boundary;
	}

	return R.ok({
		boot,
		workIndicator: work_indicator,
		navigate,
		revalidate,
		submit_inner,
		getRouteState,
		getWorkState,
		getClientBuildID,
		getRootEl,
		defineView,
		start_prefetch,
		stop_prefetch,
		save_current_scroll,
		get_default_error_boundary,
	});
}
