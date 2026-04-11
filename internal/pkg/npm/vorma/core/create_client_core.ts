/// <reference types="vite/client" />

import { addOnWindowFocusListener } from "vorma/kit/listeners";
import { findNestedMatches } from "vorma/kit/matcher/find-nested";
import {
	createPatternRegistry,
	registerPattern,
} from "vorma/kit/matcher/register";
import { R, type Result } from "vorma/kit/result";
import {
	BUILD_ID_HEADER,
	DATA_SCRIPT_ID,
	HISTORY_KEY_FIELD,
	HISTORY_USER_STATE_FIELD,
	SCROLL_STORAGE_KEY,
	SCROLL_STORAGE_RELOAD_KEY,
	VERCEL_DPL_QUERY_PARAM_KEY,
	VERCEL_X_DEPLOYMENT_ID,
	VORMA_JSON_KEY,
	VORMA_ROOT_EL_ID,
	X_ACCEPTS_CLIENT_REDIRECT,
	X_CLIENT_REDIRECT,
	X_VORMA_RELOAD,
} from "./constants.ts";
import { apply_css_bundles, preload_css, wait_for_css } from "./css.ts";
import { apply_head_and_title, type HeadEl } from "./head.ts";
import { preload_modules } from "./modules.ts";
import type { AppConfig, RevalidationResult } from "./types.ts";

export type ScrollState = { x: number; y: number } | { hash: string };

export type ScrollIntent = {
	scroll: ScrollState;
	target_route_id: string;
};

export type RouteEntry = {
	pattern: string;
	module_url: string;
	module: Record<string, unknown>;
	data: unknown;
	client_data: unknown;
	error: unknown;
};

export type RouteState = {
	entries: RouteEntry[];
	params: Record<string, string>;
	splat_values: string[];
	client_build_id: string;
	history_state: unknown;
};

export type StatusInfo = {
	isNavigating: boolean;
	isRevalidating: boolean;
	isSubmitting: boolean;
};

export type InitOptions = {
	renderFn?: () => void | Promise<void>;
	defaultErrorBoundary?: (props: { error: unknown }) => any;
	useViewTransitions?: boolean;
	onStatusChange?: (status: StatusInfo) => void;
	onRouteChange?: () => void;
	onClientBuildIDChange?: (prev: string, next: string) => void;
};

export type CommitFn = (
	state: RouteState,
	scroll_intent?: ScrollIntent,
) => void;

export type GlobalLoadingIndicatorConfig = {
	start: () => void;
	stop: () => void;
	isRunning: () => boolean;
	include?: "all" | Array<"navigations" | "submissions" | "revalidations">;
	startDelayMS?: number;
	stopDelayMS?: number;
};

export type RouteDefinition = {
	pattern: string;
	component: (props: any) => any;
	error_boundary?: (props: { error: unknown }) => any;
	client_loader?: ClientLoaderFn;
};

type freshness_waiter = {
	resolve: (result: RevalidationResult) => void;
};

const revalidation_succeeded: RevalidationResult = { ok: true };
const revalidation_exhausted_result: RevalidationResult = {
	ok: false,
	reason: "max_retries_exhausted",
};

export const MAX_SCROLL_ENTRIES = 50;
export const REFRESH_MAX_AGE_MS = 3333;
export const MAX_REDIRECTS = 10;
export const REVALIDATION_DEBOUNCE_MS = 8;
export const MAX_REVALIDATION_RETRIES = 8;
export const REVALIDATION_BACKOFF_BASE_MS = 500;
export const REVALIDATION_BACKOFF_CAP_MS = 30000;

type __T__Options = {
	reload?: () => void;
	hard_redirect?: (url: string) => void;
	scroll_to?: (x: number, y: number) => void;
};

export function apply_scroll(
	scroll: ScrollState | undefined,
	__t__options?: __T__Options,
): void {
	if (!scroll) {
		return;
	}
	if ("hash" in scroll) {
		const id = scroll.hash.startsWith("#")
			? scroll.hash.slice(1)
			: scroll.hash;
		try {
			document.getElementById(decodeURIComponent(id))?.scrollIntoView();
		} catch {
			document.getElementById(id)?.scrollIntoView();
		}
	} else {
		const scroll_to =
			__t__options?.scroll_to ??
			((x: number, y: number) => {
				return window.scrollTo(x, y);
			});
		scroll_to(scroll.x, scroll.y);
	}
}

type ClientLoaderFn = (props: {
	params: Record<string, string>;
	splatValues: string[];
	serverDataPromise: Promise<{
		matchedPatterns: string[];
		rootData: unknown;
		loaderData: unknown;
		clientBuildID: string;
	}>;
	signal: AbortSignal;
}) => Promise<unknown>;

type ClientLoaderPrefetch = {
	pattern: string;
	resolve_server_data: (data: unknown) => void;
	result_promise: Promise<unknown>;
};

type DecodedRoute = {
	pattern: string;
	module_url: string;
	server_data: unknown;
	server_error: unknown;
};

type DecodedPayload = {
	routes: DecodedRoute[];
	params: Record<string, string>;
	splat_values: string[];
	title: string | undefined;
	meta_head_els: HeadEl[];
	rest_head_els: HeadEl[];
	css_bundles: string[];
	deps: string[];
};

type FetchResult =
	| { kind: "data"; data: unknown; response: Response }
	| { kind: "redirect"; href: string; hard: boolean; response: Response }
	| {
			kind: "error";
			status: number;
			status_text: string;
			response?: Response;
	  };

type NavEntry = {
	url: URL;
	ac: AbortController;
	data_promise: Promise<FetchResult>;
	cl_prefetches: ClientLoaderPrefetch[];
	start_ts: number;
	is_revalidation: boolean;
};

type NavOptions = {
	replace?: boolean;
	scroll_to_top?: boolean;
	state?: unknown;
	is_popstate?: boolean;
	popstate_scroll?: ScrollState;
};

type StoredScrollEntry = [string, { x: number; y: number }];

declare global {
	interface Window {
		__vorma_hmr_route_update?: (
			raw_url: string,
			mod: Record<string, unknown>,
		) => Promise<void>;
	}
}

export type ClientCore = {
	init: (options: InitOptions) => Promise<Result<void>>;

	navigate: (
		href: string | URL,
		options?: {
			replace?: boolean;
			scrollToTop?: boolean;
			state?: unknown;
			skipGlobalLoadingIndicator?: boolean;
		},
	) => Promise<{ didNavigate: boolean }>;

	revalidate: () => Promise<RevalidationResult>;

	submit: <T = unknown>(
		url: string | URL,
		requestInit?: RequestInit,
		options?: {
			dedupeKey?: string;
			revalidate?: boolean;
			skipGlobalLoadingIndicator?: boolean;
		},
	) => Promise<
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
		  }
	>;

	getStatus: () => StatusInfo;
	getClientBuildID: () => string;
	getRootEl: () => HTMLElement;

	getRouterData: () => {
		clientBuildID: string;
		matchedPatterns: string[];
		splatValues: string[];
		params: Record<string, string>;
		historyState: unknown;
		rootData: unknown;
	};

	defineRoute: <T = any>(input: {
		pattern: string;
		component: (props: any) => any;
		errorBoundary?: (props: { error: unknown }) => any;
		clientLoader?: (props: any) => Promise<T>;
		runClientLoaderOnHMR?: boolean;
	}) => RouteDefinition & { __phantom_client_data?: T };

	setupGlobalLoadingIndicator: (
		config: GlobalLoadingIndicatorConfig,
	) => () => void;

	revalidateOnWindowFocus: (options?: { staleTimeMS?: number }) => () => void;

	start_prefetch: (href: string) => void;
	stop_prefetch: (href: string) => void;
	save_current_scroll: () => void;
	get_current_state: () => RouteState | null;

	get_default_error_boundary: () =>
		| ((props: { error: unknown }) => any)
		| undefined;
};

export function create_client_core(
	_: Omit<AppConfig, "__phantom_loaders" | "__phantom_actions">, // currently unused
	commit: CommitFn,
	__t__options?: __T__Options,
): Result<ClientCore> {
	const pattern_registry_res = createPatternRegistry({
		dynamicParamPrefixRune: ":",
		splatSegmentRune: "*",
		explicitIndexSegment: "_index",
	});
	if (!pattern_registry_res.ok) {
		return R.err(
			`Failed to create pattern registry: ${pattern_registry_res.err}`,
		);
	}
	const pattern_registry = pattern_registry_res.val;

	let initialized = false;
	let client_build_id = "";
	let deployment_id = "";
	let current_state: RouteState | null = null;
	let default_error_boundary:
		| ((props: { error: unknown }) => any)
		| undefined;
	let use_view_transitions = false;

	let nav_singleton: NavEntry | null = null;
	let prefetch_singleton: NavEntry | null = null;

	const active_submissions = new Map<
		string,
		{ ac: AbortController; skip_global_loading_indicator?: boolean }
	>();

	let module_map: Record<string, ClientLoaderFn> = {};

	let last_known_history_key = "";
	let last_known_history_href = "";

	const status_listeners = new Set<(s: StatusInfo) => void>();
	let last_status: StatusInfo = {
		isNavigating: false,
		isRevalidating: false,
		isSubmitting: false,
	};

	let user_on_route_change: (() => void) | undefined;
	let user_on_build_id_change:
		| ((prev: string, next: string) => void)
		| undefined;

	let mutation_response_ts = 0;
	let last_successful_nav_start_ts = 0;
	let last_activity_ts = Date.now();
	let freshness_waiters: freshness_waiter[] = [];
	let revalidation_backoff_count = 0;
	let revalidation_backoff_timer: ReturnType<typeof setTimeout> | null = null;
	let revalidate_debounce_timer: ReturnType<typeof setTimeout> | null = null;
	let revalidation_exhausted = false;

	const hmr_rerun_patterns = new Set<string>();
	const module_cache = new Map<string, Record<string, unknown>>();

	const reload_page =
		__t__options?.reload ??
		(() => {
			window.location.reload();
		});

	function get_current_url(): URL {
		return new URL(window.location.href);
	}

	const hard_redirect =
		__t__options?.hard_redirect ??
		((url: string) => {
			window.location.assign(url);
		});

	function is_http_scheme(href: string): boolean {
		try {
			const scheme = new URL(href).protocol;
			return scheme === "http:" || scheme === "https:";
		} catch {
			return false;
		}
	}

	function is_same_origin(url: URL): boolean {
		return url.origin === get_current_url().origin;
	}

	function is_same_origin_href(href: string): boolean {
		try {
			return new URL(href).origin === get_current_url().origin;
		} catch {
			return false;
		}
	}

	function href_without_hash(url: URL): string {
		const copy = new URL(url.href);
		copy.hash = "";
		return copy.href;
	}

	function matches_without_hash(a: URL, b: URL): boolean {
		return href_without_hash(a) === href_without_hash(b);
	}

	function is_same_page(url: URL): boolean {
		return matches_without_hash(url, get_current_url());
	}

	function normalize_hash(hash: string): string {
		const s = hash.startsWith("#") ? hash.slice(1) : hash;
		if (s.length === 0) {
			return s;
		}
		try {
			return decodeURIComponent(s);
		} catch {
			return s;
		}
	}

	function make_history_key(): string {
		return Math.random().toString(36).slice(2, 10);
	}

	function read_history_key(): string {
		const state = window.history.state;
		if (state && typeof state === "object" && HISTORY_KEY_FIELD in state) {
			return (state as Record<string, string>)[HISTORY_KEY_FIELD]!;
		}
		return "";
	}

	function seed_history_key(): void {
		const state = window.history.state;
		if (
			state &&
			typeof state === "object" &&
			HISTORY_KEY_FIELD in (state as Record<string, unknown>)
		) {
			last_known_history_key = (state as Record<string, string>)[
				HISTORY_KEY_FIELD
			]!;
		} else {
			const key = make_history_key();
			const existing = state && typeof state === "object" ? state : {};
			window.history.replaceState(
				{ ...(existing as object), [HISTORY_KEY_FIELD]: key },
				"",
				get_current_url().href,
			);
			last_known_history_key = key;
		}
		last_known_history_href = get_current_url().href;
	}

	function commit_history_entry(
		url: string,
		replace?: boolean,
		state?: unknown,
	): void {
		const key = make_history_key();
		const history_state: Record<string, unknown> = {
			[HISTORY_KEY_FIELD]: key,
			[HISTORY_USER_STATE_FIELD]: state,
		};

		if (replace) {
			const existing = window.history.state;
			const base =
				existing && typeof existing === "object" ? existing : {};
			window.history.replaceState(
				{ ...(base as object), ...history_state },
				"",
				url,
			);
		} else {
			window.history.pushState(history_state, "", url);
		}

		last_known_history_key = key;
		last_known_history_href = url;
	}

	function get_scroll_position(): { x: number; y: number } {
		return { x: window.scrollX, y: window.scrollY };
	}

	function make_freshness_promise(): {
		promise: Promise<RevalidationResult>;
		waiter: freshness_waiter;
	} {
		let resolve!: (result: RevalidationResult) => void;
		const promise = new Promise<RevalidationResult>((res) => {
			resolve = res;
		});
		return { promise, waiter: { resolve } };
	}

	function read_scroll_entries(): StoredScrollEntry[] {
		try {
			const raw = sessionStorage.getItem(SCROLL_STORAGE_KEY);
			if (!raw) {
				return [];
			}
			const parsed = JSON.parse(raw);
			if (!Array.isArray(parsed)) {
				return [];
			}
			return parsed.filter((e: unknown): e is StoredScrollEntry => {
				return (
					Array.isArray(e) &&
					e.length === 2 &&
					typeof e[0] === "string" &&
					!!e[1] &&
					typeof e[1] === "object" &&
					Number.isFinite((e[1] as any).x) &&
					Number.isFinite((e[1] as any).y)
				);
			});
		} catch {
			return [];
		}
	}

	function write_scroll_entries(entries: StoredScrollEntry[]): void {
		try {
			sessionStorage.setItem(SCROLL_STORAGE_KEY, JSON.stringify(entries));
		} catch {}
	}

	function save_scroll_for_key(
		key: string,
		pos: { x: number; y: number },
	): void {
		const entries = read_scroll_entries().filter((e) => {
			return e[0] !== key;
		});
		entries.push([key, { x: pos.x, y: pos.y }]);
		if (entries.length > MAX_SCROLL_ENTRIES) {
			entries.splice(0, entries.length - MAX_SCROLL_ENTRIES);
		}
		write_scroll_entries(entries);
	}

	function get_scroll_for_key(
		key: string,
	): { x: number; y: number } | undefined {
		const result = (() => {
			for (const [k, s] of read_scroll_entries()) {
				if (k === key) {
					return s;
				}
			}
			return undefined;
		})();
		return result;
	}

	function save_current_scroll(): void {
		if (last_known_history_key) {
			const pos = get_scroll_position();
			save_scroll_for_key(last_known_history_key, pos);
		}
	}

	function save_refresh_scroll(): void {
		const pos = get_scroll_position();
		const href = get_current_url().href;
		try {
			sessionStorage.setItem(
				SCROLL_STORAGE_RELOAD_KEY,
				JSON.stringify({
					...pos,
					unix: Date.now(),
					href,
				}),
			);
		} catch {}
	}

	function consume_page_reload_scroll_state(): ScrollState | undefined {
		let raw: string | null;
		try {
			raw = sessionStorage.getItem(SCROLL_STORAGE_RELOAD_KEY);
		} catch {
			return undefined;
		}
		if (!raw) {
			return undefined;
		}
		try {
			sessionStorage.removeItem(SCROLL_STORAGE_RELOAD_KEY);
		} catch {}
		try {
			const s = JSON.parse(raw);
			if (
				typeof s?.x !== "number" ||
				typeof s?.y !== "number" ||
				typeof s?.unix !== "number" ||
				typeof s?.href !== "string"
			) {
				return undefined;
			}
			if (Date.now() - s.unix > REFRESH_MAX_AGE_MS) {
				return undefined;
			}
			if (!matches_without_hash(new URL(s.href), get_current_url())) {
				return undefined;
			}
			return { x: s.x, y: s.y };
		} catch {
			return undefined;
		}
	}

	function update_build_id(res: Response): void {
		const next = res.headers.get(BUILD_ID_HEADER) ?? "";
		if (next && next !== client_build_id) {
			const prev = client_build_id;
			client_build_id = next;
			user_on_build_id_change?.(prev, next);
		}
	}

	function decode_payload(raw: unknown): DecodedPayload {
		const p = raw as Record<string, any>;
		const patterns: string[] = p.MatchedPatterns ?? [];
		const loaders_data: unknown[] = p.LoadersData ?? [];
		const import_urls: string[] = p.ImportURLs ?? [];
		const err_idx: number | null = p.OutermostServerErrIdx ?? null;
		const err_msg: string = p.OutermostServerErr ?? "";

		const routes: DecodedRoute[] = patterns.map((pattern, i) => {
			return {
				pattern,
				module_url: import_urls[i] ?? "",
				server_data: loaders_data[i],
				server_error:
					err_idx !== null && i === err_idx ? err_msg : undefined,
			};
		});

		const raw_title = p.Title?.dangerousInnerHTML;

		return {
			routes,
			params: p.Params ?? {},
			splat_values: p.SplatValues ?? [],
			title:
				raw_title !== undefined
					? decode_html_to_text(raw_title)
					: undefined,
			meta_head_els: p.MetaHeadEls ?? [],
			rest_head_els: p.RestHeadEls ?? [],
			css_bundles: p.CSSBundles ?? [],
			deps: p.Deps ?? [],
		};
	}

	function decode_html_to_text(html: string | undefined): string {
		if (!html) {
			return "";
		}
		const el = document.createElement("textarea");
		el.innerHTML = html;
		return el.value;
	}

	function parse_initial_payload(): Result<DecodedPayload> {
		const el = document.getElementById(DATA_SCRIPT_ID);
		if (!el) {
			return R.err(`Missing element: #${DATA_SCRIPT_ID}`);
		}
		const raw = JSON.parse(el.textContent ?? "{}");
		if (raw.ClientBuildID) {
			client_build_id = raw.ClientBuildID;
		}
		if (raw.DeploymentID) {
			deployment_id = raw.DeploymentID;
		}
		return R.ok(decode_payload(raw));
	}

	async function load_modules(
		urls: string[],
	): Promise<Map<string, Record<string, unknown>>> {
		const unique = [...new Set(urls.filter(Boolean))];
		const loaded = await Promise.all(
			unique.map(async (url) => {
				if (import.meta.env.DEV) {
					const key = normalize_module_url(url);
					const cached = module_cache.get(key);
					if (cached) {
						return [url, cached] as const;
					}
				}
				const mod = await import(/* @vite-ignore */ url);
				if (import.meta.env.DEV) {
					module_cache.set(normalize_module_url(url), mod);
				}
				return [url, mod as Record<string, unknown>] as const;
			}),
		);
		return new Map(loaded);
	}

	function find_partial_client_matches(pathname: string) {
		const full_result = findNestedMatches(pattern_registry, pathname);
		if (full_result) {
			return full_result;
		}
		const segments = pathname.split("/").filter(Boolean);
		for (let i = segments.length; i >= 0; i--) {
			const partial =
				i === 0 ? "/" : "/" + segments.slice(0, i).join("/");
			const result = findNestedMatches(pattern_registry, partial);
			if (result) {
				return result;
			}
		}
		return null;
	}

	function create_client_loader_prefetches(
		url: URL,
		signal: AbortSignal,
	): ClientLoaderPrefetch[] {
		const match_result = find_partial_client_matches(url.pathname);
		if (!match_result) {
			return [];
		}

		const prefetches: ClientLoaderPrefetch[] = [];

		for (const match of match_result.matches) {
			const pattern = match.registeredPattern.originalPattern;
			const loader = module_map[pattern];
			if (!loader) {
				continue;
			}

			let resolve_server_data!: (v: unknown) => void;
			const server_data_promise = new Promise<any>((res) => {
				resolve_server_data = res;
			});
			server_data_promise.catch(() => {});

			const result_promise = loader({
				params: match_result.params,
				splatValues: match_result.splatValues,
				serverDataPromise: server_data_promise,
				signal,
			});
			result_promise.catch(() => {});

			prefetches.push({ pattern, resolve_server_data, result_promise });
		}

		return prefetches;
	}

	function build_server_data(routes: DecodedRoute[], route_idx: number) {
		const matched_patterns = routes.map((r) => {
			return r.pattern;
		});
		return {
			matchedPatterns: matched_patterns,
			rootData:
				matched_patterns[0] === "/"
					? routes[0]?.server_data
					: undefined,
			loaderData: routes[route_idx]?.server_data,
			clientBuildID: client_build_id,
		};
	}

	async function run_client_loaders(
		routes: DecodedRoute[],
		payload: DecodedPayload,
		cl_prefetches: ClientLoaderPrefetch[],
		signal: AbortSignal,
	): Promise<Array<{ data: unknown } | { error: unknown } | undefined>> {
		const prefetch_by_pattern = new Map<string, ClientLoaderPrefetch>();
		for (const ps of cl_prefetches) {
			prefetch_by_pattern.set(ps.pattern, ps);
		}

		const server_err_idx = routes.findIndex((r) => {
			return r.server_error !== undefined;
		});

		const abort_controllers: Array<AbortController | null> = [];
		const raw_promises: Array<Promise<unknown>> = [];

		for (let i = 0; i < routes.length; i++) {
			const route = routes[i]!;

			if (server_err_idx !== -1 && i >= server_err_idx) {
				raw_promises.push(Promise.resolve(undefined));
				abort_controllers.push(null);
				continue;
			}

			const prefetch = prefetch_by_pattern.get(route.pattern);
			if (prefetch) {
				prefetch.resolve_server_data(build_server_data(routes, i));
				abort_controllers.push(null);
				raw_promises.push(prefetch.result_promise);
				continue;
			}

			const loader = module_map[route.pattern];
			if (!loader) {
				raw_promises.push(Promise.resolve(undefined));
				abort_controllers.push(null);
				continue;
			}

			const controller = new AbortController();
			abort_controllers.push(controller);

			if (signal.aborted) {
				controller.abort();
			} else {
				signal.addEventListener(
					"abort",
					() => {
						return controller.abort();
					},
					{
						once: true,
					},
				);
			}

			raw_promises.push(
				loader({
					params: payload.params,
					splatValues: payload.splat_values,
					serverDataPromise: Promise.resolve(
						build_server_data(routes, i),
					),
					signal: controller.signal,
				}),
			);
		}

		const wrapped = raw_promises.map(async (promise, index) => {
			return promise.catch((err) => {
				if (!is_abort_error(err)) {
					for (let j = index + 1; j < abort_controllers.length; j++) {
						abort_controllers[j]?.abort();
					}
				}
				throw err;
			});
		});

		const results = await Promise.allSettled(wrapped);

		const out: Array<{ data: unknown } | { error: unknown } | undefined> =
			[];

		for (let i = 0; i < results.length; i++) {
			const result = results[i]!;
			if (result.status === "fulfilled") {
				out.push(
					result.value !== undefined
						? { data: result.value }
						: undefined,
				);
			} else {
				if (!is_abort_error(result.reason)) {
					out.push({ error: to_error_string(result.reason) });
				} else {
					out.push(undefined);
				}
				break;
			}
		}

		return out;
	}

	async function prepare_route_state(
		payload: DecodedPayload,
		cl_prefetches: ClientLoaderPrefetch[],
		signal: AbortSignal,
	): Promise<{
		state: Omit<RouteState, "history_state">;
		apply_dom_side_effects: () => void;
	} | null> {
		preload_css(payload.css_bundles);

		const modules = await load_modules(
			payload.routes.map((r) => {
				return r.module_url;
			}),
		);

		for (const route of payload.routes) {
			const mod = modules.get(route.module_url);
			if (!mod) {
				continue;
			}
			const def = mod.default as RouteDefinition | undefined;
			if (def?.client_loader) {
				module_map[route.pattern] = def.client_loader;
				registerPattern(pattern_registry, route.pattern);
			}
		}

		const cl_results = await run_client_loaders(
			payload.routes,
			payload,
			cl_prefetches,
			signal,
		);
		if (signal.aborted) {
			return null;
		}

		await wait_for_css(payload.css_bundles, signal);
		if (signal.aborted) {
			return null;
		}

		const entries: RouteEntry[] = payload.routes.map((route, i) => {
			const mod = modules.get(route.module_url) ?? {};
			const cl = cl_results[i];

			let error: unknown;
			if (route.server_error !== undefined) {
				error = route.server_error;
			} else if (cl && "error" in cl) {
				error = cl.error;
			}

			return {
				pattern: route.pattern,
				module_url: route.module_url,
				module: mod,
				data: route.server_data,
				client_data: cl && "data" in cl ? cl.data : undefined,
				error,
			};
		});

		const state: Omit<RouteState, "history_state"> = {
			entries,
			params: payload.params,
			splat_values: payload.splat_values,
			client_build_id,
		};

		const apply_dom_side_effects = () => {
			apply_head_and_title(
				payload.title,
				payload.meta_head_els,
				payload.rest_head_els,
			);
			apply_css_bundles(payload.css_bundles);
			preload_modules(payload.deps);
		};

		return { state, apply_dom_side_effects };
	}

	function detect_redirect(
		res: Response,
		base_url: URL,
	): { href: string; hard: boolean } | null {
		const hard_reload = res.headers.get(X_VORMA_RELOAD);
		if (hard_reload) {
			return { href: new URL(hard_reload, base_url).href, hard: true };
		}

		const client_redirect = res.headers.get(X_CLIENT_REDIRECT);
		if (client_redirect) {
			return {
				href: new URL(client_redirect, base_url).href,
				hard: false,
			};
		}

		if (res.redirected && res.url && res.url !== base_url.href) {
			return { href: new URL(res.url, base_url).href, hard: false };
		}

		return null;
	}

	async function nav_fetch(
		url: URL,
		signal: AbortSignal,
		purpose: "navigate" | "revalidate" | "prefetch",
	): Promise<FetchResult> {
		const modified = new URL(url.href);
		modified.searchParams.set(VORMA_JSON_KEY, client_build_id);

		if (purpose === "revalidate" && deployment_id) {
			modified.searchParams.set(
				VERCEL_DPL_QUERY_PARAM_KEY,
				deployment_id,
			);
		}

		const res = await fetch(modified, {
			signal,
			headers: { [X_ACCEPTS_CLIENT_REDIRECT]: "1" },
		});

		const redirect = detect_redirect(res, modified);
		if (redirect) {
			return { kind: "redirect", ...redirect, response: res };
		}

		if (!res.ok) {
			return {
				kind: "error",
				status: res.status,
				status_text: res.statusText,
				response: res,
			};
		}

		try {
			const data = await res.json();
			return { kind: "data", data, response: res };
		} catch {
			return {
				kind: "error",
				status: res.status,
				status_text: res.statusText,
				response: res,
			};
		}
	}

	function is_revalidation_pending(): boolean {
		if (revalidate_debounce_timer !== null) {
			return true;
		}
		if (revalidation_exhausted) {
			return false;
		}
		if (mutation_response_ts === 0) {
			return false;
		}
		if (last_successful_nav_start_ts >= mutation_response_ts) {
			return false;
		}
		if (nav_singleton && nav_singleton.start_ts >= mutation_response_ts) {
			return false;
		}
		return true;
	}

	function get_status(): StatusInfo {
		return {
			isNavigating:
				nav_singleton !== null && !nav_singleton.is_revalidation,
			isRevalidating:
				(nav_singleton !== null && nav_singleton.is_revalidation) ||
				is_revalidation_pending(),
			isSubmitting:
				active_submissions.size > 0 &&
				Array.from(active_submissions.values()).some((s) => {
					return !s.skip_global_loading_indicator;
				}),
		};
	}

	function notify_status(): void {
		const next = get_status();
		if (
			next.isNavigating !== last_status.isNavigating ||
			next.isRevalidating !== last_status.isRevalidating ||
			next.isSubmitting !== last_status.isSubmitting
		) {
			last_status = next;
			status_listeners.forEach((fn) => {
				return fn(next);
			});
		}
	}

	function compute_scroll_intent(
		url: URL,
		options: NavOptions,
	): ScrollState | undefined {
		let result: ScrollState | undefined;

		if (options.is_popstate) {
			const hash = normalize_hash(url.hash);
			if (hash.length > 0) {
				result = { hash: url.hash };
			} else {
				result = options.popstate_scroll ?? { x: 0, y: 0 };
			}
		} else {
			const hash = normalize_hash(url.hash);
			if (hash.length > 0) {
				result = { hash: url.hash };
			} else if (options.scroll_to_top === false) {
				result = undefined;
			} else {
				result = { x: 0, y: 0 };
			}
		}

		return result;
	}

	function handle_same_page_nav(
		url: URL,
		options: NavOptions,
	): { didNavigate: boolean } {
		const target_hash = normalize_hash(url.hash);
		const current_hash = normalize_hash(get_current_url().hash);

		function update_state() {
			current_state = {
				...current_state!,
				history_state: window.history.state?.[HISTORY_USER_STATE_FIELD],
			};
			commit(current_state);
		}

		if (target_hash !== current_hash) {
			save_current_scroll();
			commit_history_entry(url.href, options.replace, options.state);
			update_state();
			if (target_hash.length > 0) {
				apply_scroll({ hash: url.hash }, __t__options);
			} else {
				apply_scroll({ x: 0, y: 0 }, __t__options);
			}
			return { didNavigate: true };
		}

		if (options.replace) {
			commit_history_entry(url.href, true, options.state);
			update_state();
		}
		apply_scroll({ x: 0, y: 0 }, __t__options);
		return { didNavigate: false };
	}

	async function navigate_inner(
		url: URL,
		options: NavOptions,
		redirect_count: number,
	): Promise<{ didNavigate: boolean }> {
		if (!options.is_popstate && is_same_page(url)) {
			return handle_same_page_nav(url, options);
		}

		if (nav_singleton) {
			if (
				!nav_singleton.is_revalidation &&
				matches_without_hash(nav_singleton.url, url)
			) {
				return { didNavigate: false };
			}
			nav_singleton.ac.abort();
			nav_singleton = null;
		}

		let this_nav: NavEntry;

		if (
			prefetch_singleton &&
			matches_without_hash(prefetch_singleton.url, url)
		) {
			this_nav = {
				...prefetch_singleton,
				is_revalidation: false,
			};
			prefetch_singleton = null;
		} else {
			cancel_prefetch();
			const ac = new AbortController();
			this_nav = {
				url,
				ac,
				data_promise: nav_fetch(url, ac.signal, "navigate"),
				cl_prefetches: create_client_loader_prefetches(url, ac.signal),
				start_ts: Date.now(),
				is_revalidation: false,
			};
		}

		nav_singleton = this_nav;
		notify_status();

		try {
			const result = await this_nav.data_promise;
			if (nav_singleton !== this_nav) {
				return { didNavigate: false };
			}

			if (result.response) {
				update_build_id(result.response);
			}

			if (result.kind === "redirect") {
				if (!is_http_scheme(result.href)) {
					return { didNavigate: false };
				}
				if (result.hard || !is_same_origin_href(result.href)) {
					hard_redirect(result.href);
					return { didNavigate: false };
				}
				if (redirect_count >= MAX_REDIRECTS) {
					return { didNavigate: false };
				}
				nav_singleton = null;
				return navigate_inner(
					new URL(result.href),
					{
						replace: options.replace,
						scroll_to_top: options.scroll_to_top,
						state: options.state,
					},
					redirect_count + 1,
				);
			}

			if (result.kind === "error") {
				return { didNavigate: false };
			}

			const payload = decode_payload(result.data);
			preload_modules(payload.deps);

			const nav_result = await prepare_route_state(
				payload,
				this_nav.cl_prefetches,
				this_nav.ac.signal,
			);

			if (
				!nav_result ||
				this_nav.ac.signal.aborted ||
				nav_singleton !== this_nav
			) {
				return { didNavigate: false };
			}

			if (!options.is_popstate) {
				save_current_scroll();
			}

			const scroll = compute_scroll_intent(url, options);

			const scroll_intent: ScrollIntent | undefined = scroll
				? {
						scroll,
						target_route_id: make_route_id(
							nav_result.state.entries.length - 1,
							nav_result.state.entries[
								nav_result.state.entries.length - 1
							]?.pattern ?? "",
						),
					}
				: undefined;

			const do_commit = () => {
				if (!options.is_popstate) {
					commit_history_entry(
						url.href,
						options.replace,
						options.state,
					);
				}
				nav_result.apply_dom_side_effects();
				// nav_result.state.history_state =
				// 	window.history.state?.[HISTORY_USER_STATE_FIELD];
				current_state = {
					...nav_result.state,
					history_state:
						window.history.state?.[HISTORY_USER_STATE_FIELD],
				};
				commit(current_state, scroll_intent);
			};

			if (
				use_view_transitions &&
				typeof (document as any).startViewTransition === "function"
			) {
				await (document as any).startViewTransition(do_commit).finished;
			} else {
				do_commit();
			}

			user_on_route_change?.();
			last_activity_ts = Date.now();

			if (this_nav.start_ts >= mutation_response_ts) {
				last_successful_nav_start_ts = this_nav.start_ts;
				resolve_freshness_waiters();
			}

			return { didNavigate: true };
		} catch {
			return { didNavigate: false };
		} finally {
			if (nav_singleton === this_nav) {
				nav_singleton = null;
			}
			notify_status();
			maybe_revalidate();
		}
	}

	async function run_revalidation(): Promise<void> {
		const url = new URL(last_known_history_href);
		const ac = new AbortController();

		const this_nav: NavEntry = {
			url,
			ac,
			data_promise: nav_fetch(url, ac.signal, "revalidate"),
			cl_prefetches: create_client_loader_prefetches(url, ac.signal),
			start_ts: Date.now(),
			is_revalidation: true,
		};

		nav_singleton = this_nav;
		notify_status();

		try {
			const result = await this_nav.data_promise;
			if (nav_singleton !== this_nav) {
				return;
			}

			if (!matches_without_hash(get_current_url(), url)) {
				return;
			}

			if (result.response) {
				update_build_id(result.response);
			}

			if (result.kind === "redirect") {
				if (!is_http_scheme(result.href)) {
					return;
				}
				if (result.hard || !is_same_origin_href(result.href)) {
					hard_redirect(result.href);
					return;
				}
				nav_singleton = null;
				void navigate_inner(new URL(result.href), { replace: true }, 0);
				return;
			}

			if (result.kind === "error") {
				return;
			}

			const payload = decode_payload(result.data);
			preload_modules(payload.deps);

			const nav_result = await prepare_route_state(
				payload,
				this_nav.cl_prefetches,
				ac.signal,
			);

			if (!nav_result || nav_singleton !== this_nav) {
				return;
			}

			if (!matches_without_hash(get_current_url(), url)) {
				return;
			}

			nav_result.apply_dom_side_effects();
			current_state = {
				...nav_result.state,
				history_state: window.history.state?.[HISTORY_USER_STATE_FIELD],
			};
			commit(current_state!);

			user_on_route_change?.();
			last_activity_ts = Date.now();

			if (this_nav.start_ts >= mutation_response_ts) {
				last_successful_nav_start_ts = this_nav.start_ts;
				resolve_freshness_waiters();
			}
		} finally {
			if (nav_singleton === this_nav) {
				nav_singleton = null;
			}
			notify_status();
			maybe_revalidate();
		}
	}

	function mark_mutation_response(): void {
		mutation_response_ts = Date.now();
		revalidation_exhausted = false;
		revalidation_backoff_count = 0;
		if (revalidation_backoff_timer !== null) {
			clearTimeout(revalidation_backoff_timer);
			revalidation_backoff_timer = null;
		}
	}

	function reset_backoff(): void {
		revalidation_backoff_count = 0;
		if (revalidation_backoff_timer !== null) {
			clearTimeout(revalidation_backoff_timer);
			revalidation_backoff_timer = null;
		}
	}

	function resolve_freshness_waiters(): void {
		if (
			freshness_waiters.length > 0 &&
			last_successful_nav_start_ts >= mutation_response_ts
		) {
			const waiters = freshness_waiters;
			freshness_waiters = [];
			for (const waiter of waiters) {
				waiter.resolve(revalidation_succeeded);
			}
		}
	}

	function resolve_exhausted_freshness_waiters(): void {
		if (freshness_waiters.length === 0) {
			revalidation_exhausted = true;
			return;
		}

		const waiters = freshness_waiters;
		freshness_waiters = [];
		revalidation_exhausted = true;

		for (const waiter of waiters) {
			waiter.resolve(revalidation_exhausted_result);
		}
	}

	function maybe_revalidate(): void {
		if (mutation_response_ts === 0) {
			return;
		}

		if (last_successful_nav_start_ts >= mutation_response_ts) {
			reset_backoff();
			resolve_freshness_waiters();
			return;
		}

		if (nav_singleton && nav_singleton.start_ts >= mutation_response_ts) {
			return;
		}

		if (nav_singleton) {
			return;
		}

		if (revalidation_backoff_timer !== null) {
			return;
		}

		if (revalidation_backoff_count >= MAX_REVALIDATION_RETRIES) {
			resolve_exhausted_freshness_waiters();
			notify_status();
			return;
		}

		const delay =
			revalidation_backoff_count === 0
				? 0
				: Math.min(
						REVALIDATION_BACKOFF_BASE_MS *
							Math.pow(2, revalidation_backoff_count - 1),
						REVALIDATION_BACKOFF_CAP_MS,
					);

		revalidation_backoff_count++;

		if (delay === 0) {
			void run_revalidation().catch(() => {});
		} else {
			revalidation_backoff_timer = setTimeout(() => {
				revalidation_backoff_timer = null;
				if (last_successful_nav_start_ts >= mutation_response_ts) {
					reset_backoff();
					return;
				}
				if (nav_singleton) {
					return;
				}
				void run_revalidation().catch(() => {});
			}, delay);
		}
	}

	function start_prefetch(href: string): void {
		if (!initialized) {
			return;
		}
		const url = new URL(href, window.location.href);

		if (!is_same_origin(url) || is_same_page(url)) {
			return;
		}

		if (
			nav_singleton &&
			!nav_singleton.is_revalidation &&
			matches_without_hash(nav_singleton.url, url)
		) {
			return;
		}

		if (
			prefetch_singleton &&
			matches_without_hash(prefetch_singleton.url, url)
		) {
			return;
		}

		cancel_prefetch();

		const ac = new AbortController();
		const data_promise = nav_fetch(url, ac.signal, "prefetch");

		void data_promise
			.then((result) => {
				if (result.kind === "data") {
					const payload = decode_payload(result.data);
					preload_modules(payload.deps);
					preload_css(payload.css_bundles);
				}
			})
			.catch(() => {});

		prefetch_singleton = {
			url,
			ac,
			data_promise,
			cl_prefetches: create_client_loader_prefetches(url, ac.signal),
			start_ts: Date.now(),
			is_revalidation: false,
		};
	}

	function stop_prefetch(href: string): void {
		if (!prefetch_singleton) {
			return;
		}
		const url = new URL(href, window.location.href);
		if (!matches_without_hash(prefetch_singleton.url, url)) {
			return;
		}
		cancel_prefetch();
	}

	function cancel_prefetch(): void {
		if (prefetch_singleton) {
			prefetch_singleton.ac.abort();
			prefetch_singleton = null;
		}
	}

	async function submit<T = unknown>(
		url: string | URL,
		requestInit?: RequestInit,
		options?: {
			dedupeKey?: string;
			revalidate?: boolean;
			skipGlobalLoadingIndicator?: boolean;
		},
	): Promise<
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
		  }
	> {
		if (!initialized) {
			throw new Error("Vorma not initialized");
		}
		const resolved = new URL(String(url), window.location.href);

		if (!is_same_origin(resolved)) {
			return {
				success: false,
				error: `submit only supports same-origin targets. Received: "${resolved.href}".`,
				revalidationPromise: Promise.resolve(revalidation_succeeded),
			};
		}

		let dedupe_key = options?.dedupeKey;
		if (dedupe_key) {
			active_submissions.get(dedupe_key)?.ac.abort();
		}
		if (!dedupe_key) {
			dedupe_key = crypto.randomUUID();
		}

		const ac = new AbortController();
		active_submissions.set(dedupe_key, {
			ac,
			skip_global_loading_indicator: options?.skipGlobalLoadingIndicator,
		});
		notify_status();

		let revalidation_promise: Promise<RevalidationResult> = Promise.resolve(
			revalidation_succeeded,
		);

		try {
			const method = requestInit?.method
				? requestInit.method.toUpperCase().trim()
				: "GET";
			const is_get = method === "GET" || method === "HEAD";

			const headers = new Headers();
			if (deployment_id) {
				headers.set(VERCEL_X_DEPLOYMENT_ID, deployment_id);
			}
			new Headers(requestInit?.headers ?? undefined).forEach((v, k) => {
				headers.set(k, v);
			});
			headers.set(X_ACCEPTS_CLIENT_REDIRECT, "1");

			const body = requestInit?.body;
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

			const final_init: RequestInit = {
				...requestInit,
				method,
				headers,
				signal: ac.signal,
			};
			if (is_get) {
				delete final_init.body;
			} else if (should_json) {
				final_init.body = JSON.stringify(body);
				if (!headers.has("Content-Type")) {
					headers.set("Content-Type", "application/json");
				}
			}

			const res = await fetch(resolved, final_init);

			const should_revalidate =
				options?.revalidate !== undefined
					? options.revalidate
					: !is_get;

			if (should_revalidate && !ac.signal.aborted) {
				mark_mutation_response();
				maybe_revalidate();

				const freshness = make_freshness_promise();
				revalidation_promise = freshness.promise;
				freshness_waiters.push(freshness.waiter);
			}

			update_build_id(res);

			const redirect = detect_redirect(res, resolved);
			if (redirect && !ac.signal.aborted) {
				if (!is_http_scheme(redirect.href)) {
					return {
						success: false,
						error: `Redirect target must use an HTTP(S) scheme. Received: "${redirect.href}".`,
						response: res,
						revalidationPromise: revalidation_promise,
					};
				}
				if (redirect.hard || !is_same_origin_href(redirect.href)) {
					hard_redirect(redirect.href);
					return {
						success: true,
						data: undefined as T,
						response: res,
						revalidationPromise: revalidation_promise,
					};
				}
				void navigate_inner(
					new URL(redirect.href),
					{ replace: true },
					0,
				);
				return {
					success: true,
					data: undefined as T,
					response: res,
					revalidationPromise: revalidation_promise,
				};
			}

			if (!res.ok) {
				return {
					success: false,
					error: res.statusText,
					response: res,
					revalidationPromise: revalidation_promise,
				};
			}

			let data: unknown;
			const ct = res.headers.get("Content-Type");
			if (res.status !== 204) {
				if (ct?.toLowerCase().includes("json")) {
					data = await res.json();
				} else {
					const t = await res.text();
					data = t.length > 0 ? t : undefined;
				}
			}

			return {
				success: true,
				data: data as T,
				response: res,
				revalidationPromise: revalidation_promise,
			};
		} catch (e) {
			if (is_abort_error(e)) {
				return {
					success: false,
					error: "Aborted",
					revalidationPromise: revalidation_promise,
				};
			}
			return {
				success: false,
				error: String(e instanceof Error ? e.message : e),
				revalidationPromise: revalidation_promise,
			};
		} finally {
			if (active_submissions.get(dedupe_key)?.ac === ac) {
				active_submissions.delete(dedupe_key);
			}
			notify_status();
		}
	}

	async function handle_popstate(): Promise<void> {
		const prev_key = last_known_history_key;
		const prev_href = last_known_history_href;
		const next_key = read_history_key();
		const next_href = get_current_url().href;

		if (next_key === prev_key) {
			return;
		}

		if (prev_key) {
			save_scroll_for_key(prev_key, get_scroll_position());
		}

		// Advance bookkeeping eagerly. The browser has already changed
		// the URL, so last_known_* must reflect the new position even
		// if the navigation ultimately fails. The hard-reload fallback
		// in the catch block ensures we never leave URL and UI out of
		// sync — a failed apply reloads the destination rather than
		// reverting to stale bookkeeping.
		last_known_history_key = next_key;
		last_known_history_href = next_href;

		const prev_url = new URL(prev_href);
		const next_url = new URL(next_href);

		if (matches_without_hash(prev_url, next_url)) {
			const hash = normalize_hash(next_url.hash);
			const scroll: ScrollState =
				hash.length > 0
					? { hash: next_url.hash }
					: (get_scroll_for_key(next_key) ?? { x: 0, y: 0 });
			apply_scroll(scroll, __t__options);
			return;
		}

		const restored_scroll = get_scroll_for_key(next_key);

		try {
			const result = await navigate_inner(
				next_url,
				{
					is_popstate: true,
					popstate_scroll: restored_scroll,
				},
				0,
			);

			if (!result.didNavigate && !nav_singleton) {
				reload_page();
			}
		} catch {
			if (!nav_singleton) {
				reload_page();
			}
		}
	}

	function register_hmr_listener(): void {
		if (!import.meta.env.DEV || !import.meta.hot) {
			return;
		}

		window.__vorma_hmr_route_update = async (
			raw_url: string,
			mod: Record<string, unknown>,
		) => {
			if (!current_state) {
				return;
			}

			const url = normalize_module_url(raw_url);
			module_cache.set(url, mod);
			const idx = current_state.entries.findIndex((e) => {
				return normalize_module_url(e.module_url) === url;
			});
			if (idx === -1) {
				return;
			}

			const entry = current_state.entries[idx]!;
			const def = mod.default as RouteDefinition | undefined;

			if (def?.client_loader) {
				module_map[entry.pattern] = def.client_loader;
				registerPattern(pattern_registry, entry.pattern);
			} else {
				delete module_map[entry.pattern];
			}

			let client_data = entry.client_data;
			if (hmr_rerun_patterns.has(entry.pattern)) {
				const loader = module_map[entry.pattern];
				if (loader && current_state) {
					try {
						const matched_patterns = current_state.entries.map(
							(e) => {
								return e.pattern;
							},
						);
						client_data = await loader({
							params: current_state.params,
							splatValues: current_state.splat_values,
							serverDataPromise: Promise.resolve({
								matchedPatterns: matched_patterns,
								rootData:
									matched_patterns[0] === "/"
										? current_state.entries[0]?.data
										: undefined,
								loaderData: current_state.entries[idx]?.data,
								clientBuildID: current_state.client_build_id,
							}),
							signal: new AbortController().signal,
						});
					} catch (err) {
						console.error(
							"Vorma: HMR client loader re-run failed",
							err,
						);
					}
				}
			}

			if (!current_state) {
				return;
			}

			const entries = current_state.entries.map((e, i) => {
				if (i !== idx) {
					return e;
				}
				return { ...e, module: mod, client_data };
			});
			current_state = {
				...current_state,
				entries,
				history_state: window.history.state?.[HISTORY_USER_STATE_FIELD],
			};
			commit(current_state);
		};
	}

	async function init(options: InitOptions): Promise<Result<void>> {
		if (options.onStatusChange) {
			status_listeners.add(options.onStatusChange);
		}
		user_on_route_change = options.onRouteChange;
		user_on_build_id_change = options.onClientBuildIDChange;
		default_error_boundary = options.defaultErrorBoundary;
		use_view_transitions = options.useViewTransitions ?? false;

		const payload_res = parse_initial_payload();
		if (!payload_res.ok) {
			return R.err(payload_res.err);
		}
		const payload = payload_res.val;

		seed_history_key();
		try {
			window.history.scrollRestoration = "manual";
		} catch {}

		const on_popstate = () => {
			void handle_popstate();
		};
		window.addEventListener("popstate", on_popstate);

		window.addEventListener("beforeunload", save_refresh_scroll);

		register_hmr_listener();

		const initial_ac = new AbortController();
		const init_result = await prepare_route_state(
			payload,
			[],
			initial_ac.signal,
		);
		if (!init_result) {
			return R.err("Initial navigation produced no state");
		}

		init_result.apply_dom_side_effects();
		current_state = {
			...init_result.state,
			history_state: window.history.state?.[HISTORY_USER_STATE_FIELD],
		};
		commit(current_state!);

		if (options.renderFn) {
			await options.renderFn();
		}

		const refresh_scroll = consume_page_reload_scroll_state();
		if (refresh_scroll) {
			window.requestAnimationFrame(() => {
				apply_scroll(refresh_scroll, __t__options);
			});
		}

		initialized = true;
		return R.ok(undefined);
	}

	function defineRoute<T = any>(input: {
		pattern: string;
		component: (props: any) => any;
		errorBoundary?: (props: { error: unknown }) => any;
		clientLoader?: (props: any) => Promise<T>;
		runClientLoaderOnHMR?: boolean;
	}): RouteDefinition & { __phantom_client_data?: T } {
		if (import.meta.env.DEV) {
			if (input.runClientLoaderOnHMR) {
				hmr_rerun_patterns.add(input.pattern);
			} else {
				hmr_rerun_patterns.delete(input.pattern);
			}
		}

		return {
			pattern: input.pattern,
			component: input.component,
			error_boundary: input.errorBoundary,
			client_loader: input.clientLoader,
		};
	}

	function setupGlobalLoadingIndicator(
		config: GlobalLoadingIndicatorConfig,
	): () => void {
		const inc_all = !config.include || config.include === "all";
		const inc_nav =
			inc_all ||
			(Array.isArray(config.include) &&
				config.include.includes("navigations"));
		const inc_sub =
			inc_all ||
			(Array.isArray(config.include) &&
				config.include.includes("submissions"));
		const inc_rev =
			inc_all ||
			(Array.isArray(config.include) &&
				config.include.includes("revalidations"));
		const start_delay = config.startDelayMS ?? 12;
		const stop_delay = config.stopDelayMS ?? 12;
		let start_timer: number | null = null;
		let stop_timer: number | null = null;

		function should_run(): boolean {
			const s = get_status();
			return (
				(inc_nav && s.isNavigating) ||
				(inc_sub && s.isSubmitting) ||
				(inc_rev && s.isRevalidating)
			);
		}

		function sync(): void {
			if (should_run()) {
				if (stop_timer !== null) {
					clearTimeout(stop_timer);
					stop_timer = null;
				}
				if (config.isRunning() || start_timer !== null) {
					return;
				}
				start_timer = window.setTimeout(() => {
					start_timer = null;
					if (!should_run() || config.isRunning()) {
						return;
					}
					config.start();
				}, start_delay);
			} else {
				if (start_timer !== null) {
					clearTimeout(start_timer);
					start_timer = null;
				}
				if (!config.isRunning() || stop_timer !== null) {
					return;
				}
				stop_timer = window.setTimeout(() => {
					stop_timer = null;
					if (should_run() || !config.isRunning()) {
						return;
					}
					config.stop();
				}, stop_delay);
			}
		}

		status_listeners.add(sync);
		sync();

		return () => {
			status_listeners.delete(sync);
			if (start_timer !== null) {
				clearTimeout(start_timer);
			}
			if (stop_timer !== null) {
				clearTimeout(stop_timer);
			}
			if (config.isRunning()) {
				config.stop();
			}
		};
	}

	function revalidateOnWindowFocus(options?: {
		staleTimeMS?: number;
	}): () => void {
		const stale_ms = options?.staleTimeMS ?? 5_000;
		return addOnWindowFocusListener(() => {
			const s = get_status();
			if (s.isNavigating || s.isSubmitting || s.isRevalidating) {
				return;
			}
			if (Date.now() - last_activity_ts >= stale_ms) {
				void revalidate();
			}
		});
	}

	async function navigate(
		href: string | URL,
		options?: {
			replace?: boolean;
			scrollToTop?: boolean;
			state?: unknown;
			skipGlobalLoadingIndicator?: boolean;
		},
	): Promise<{ didNavigate: boolean }> {
		if (!initialized) {
			throw new Error("Vorma not initialized");
		}
		const url = new URL(String(href), window.location.href);
		if (!is_same_origin(url)) {
			hard_redirect(url.href);
			return { didNavigate: false };
		}
		return navigate_inner(
			url,
			{
				replace: options?.replace,
				scroll_to_top: options?.scrollToTop,
				state: options?.state,
			},
			0,
		);
	}

	async function revalidate(): Promise<RevalidationResult> {
		if (!initialized) {
			throw new Error("Vorma not initialized");
		}
		const freshness = make_freshness_promise();
		freshness_waiters.push(freshness.waiter);

		if (revalidate_debounce_timer !== null) {
			clearTimeout(revalidate_debounce_timer);
		}
		revalidate_debounce_timer = setTimeout(() => {
			revalidate_debounce_timer = null;
			mark_mutation_response();
			maybe_revalidate();
		}, REVALIDATION_DEBOUNCE_MS);

		return freshness.promise;
	}

	function getRouterData() {
		if (!current_state) {
			throw new Error("Vorma not initialized");
		}
		const matched_patterns = current_state.entries.map((e) => {
			return e.pattern;
		});
		return {
			clientBuildID: current_state.client_build_id,
			matchedPatterns: matched_patterns,
			splatValues: current_state.splat_values,
			params: current_state.params,
			historyState: current_state.history_state,
			rootData:
				matched_patterns[0] === "/"
					? current_state.entries[0]?.data
					: undefined,
		};
	}

	function getRootEl(): HTMLElement {
		const el = document.getElementById(VORMA_ROOT_EL_ID);
		if (!el) {
			const new_el = document.createElement("div");
			new_el.id = VORMA_ROOT_EL_ID;
			document.body.insertBefore(new_el, document.body.firstChild);
			return new_el;
		}
		return el;
	}

	(window as any)[Symbol.for("vorma-data-revalidate-fn")] = revalidate;

	return R.ok({
		init,
		navigate,
		revalidate,
		submit,
		getStatus: get_status,
		getClientBuildID: () => {
			return client_build_id;
		},
		getRootEl,
		getRouterData,
		defineRoute,
		setupGlobalLoadingIndicator,
		revalidateOnWindowFocus,
		start_prefetch,
		stop_prefetch,
		save_current_scroll,
		get_current_state: () => {
			return current_state;
		},
		get_default_error_boundary: () => {
			return default_error_boundary;
		},
	});
}

export function make_route_id(idx: number, pattern: string): string {
	return `${idx}:${pattern}`;
}

function is_abort_error(e: unknown): boolean {
	return e instanceof DOMException && e.name === "AbortError";
}

function normalize_module_url(url: string): string {
	return new URL(url, window.location.href).pathname;
}

function to_error_string(err: unknown): string {
	if (err instanceof Error) {
		return err.message;
	}
	return String(err);
}
