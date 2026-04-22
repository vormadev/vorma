/// <reference types="vite/client" />

import { jsonDeepEquals, parseSearchParams } from "vorma/kit/json";
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
import type {
	ActionKind,
	AppConfig,
	BeforeRouteCommitFn,
	BeforeRouteYieldFn,
	RevalidationResult,
	RouteErrorState,
	RouteState,
	RouteUpdateReason,
} from "./types.ts";

/////////////////////////////////////////////////////////////////////
/////// Public Types
/////////////////////////////////////////////////////////////////////

export type ScrollState = { x: number; y: number } | { hash: string };

export type ScrollIntent = {
	scroll: ScrollState;
	target_route_id: string;
};

export type RouteRenderEntry = {
	pattern: string;
	input: unknown;
	module_url: string;
	module: Record<string, unknown>;
	loader_data: unknown;
	client_loader_data: unknown;
};

export type RouteRenderState = {
	entries: RouteRenderEntry[];
	error: RouteErrorState | null;
	params: Record<string, string>;
	splat_values: string[];
	client_build_id: string;
	history_state: unknown;
};

export type WorkState = {
	navigation: null | {
		href: string;
		replace: boolean;
		source: "navigate" | "popstate" | "redirect";
	};
	revalidation: null | {
		status: "debouncing" | "running" | "retrying";
		attempt: number;
	};
	prefetch: null | {
		href: string;
	};
	submissions: Array<{
		key: string;
		method: string;
		href: string;
	}>;
};

export type ProgressIndicatorConfig = {
	start: () => void;
	stop: () => void;
	isRunning: () => boolean;
	include?: "all" | Array<"navigations" | "submissions" | "revalidations">;
	startDelayMS?: number;
	stopDelayMS?: number;
};

export type InitOptions = {
	render?: () => void | Promise<void>;
	progressIndicator?: ProgressIndicatorConfig;
	revalidateOnWindowFocus?: boolean | { staleTimeMS: number };
	defaultErrorBoundary?: (props: { error: unknown }) => any;
	useViewTransitions?: boolean;
	onRouteUpdate?: (
		route: RouteState,
		previousRoute: RouteState | null,
		reason: RouteUpdateReason,
	) => void;
	onWorkUpdate?: (work: WorkState) => void;
	onClientBuildIDChange?: (prev: string, next: string) => void;
};

export type CommitFn = (
	state: RouteRenderState,
	scroll_intent?: ScrollIntent,
) => void;

export type RouteDefinition = {
	pattern: string;
	component: (props: any) => any;
	error_boundary?: (props: { error: unknown }) => any;
	client_loader?: ClientLoaderFn;
	before_route_commit?: BeforeRouteCommitFn;
	before_route_yield?: BeforeRouteYieldFn;
};

type ClientLoaderFn = (args: {
	trigger: "init" | "navigation" | "revalidation" | "prefetch";
	href: string;
	historyState: unknown;
	pattern: string;
	params: Record<string, string>;
	splatValues: string[];
	input: unknown;
	knownMatches: ClientLoaderKnownMatch[];
	serverPromise: Promise<ClientLoaderServerState>;
	signal: AbortSignal;
}) => Promise<unknown>;

type ClientLoaderKnownMatch = {
	pattern: string;
	input: unknown;
};

type ClientLoaderServerState = {
	clientBuildID: string;
	matches: Array<{
		pattern: string;
		input: unknown;
		loaderData: unknown;
	}>;
	outermostServerError: null | {
		idx: number;
		error: unknown;
	};
	loaderData: unknown;
};

type SubmitResult<T> =
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

export type ClientCore = {
	init: (options: InitOptions) => Promise<Result<void>>;
	navigate: (
		href: string | URL,
		options?: {
			replace?: boolean;
			scrollToTop?: boolean;
			state?: unknown;
			skipProgressIndicator?: boolean;
		},
	) => Promise<{ didNavigate: boolean }>;
	revalidate: () => Promise<RevalidationResult>;
	submit_inner: <T = unknown>(
		url: string | URL,
		requestInit?: RequestInit,
		options?: {
			actionKind?: ActionKind;
			dedupeKey?: string;
			revalidate?: boolean;
			skipProgressIndicator?: boolean;
		},
	) => Promise<SubmitResult<T>>;
	getRouteState: () => RouteState;
	getWorkState: () => WorkState;
	getClientBuildID: () => string;
	getRootEl: () => HTMLElement;
	defineRoute: <T = any>(input: {
		pattern: string;
		component: (props: any) => any;
		errorBoundary?: (props: { error: unknown }) => any;
		clientLoader?: (props: any) => Promise<T>;
		beforeRouteCommit?: BeforeRouteCommitFn;
		beforeRouteYield?: BeforeRouteYieldFn;
		runClientLoaderOnHMR?: boolean;
	}) => RouteDefinition & { __phantom_client_loader_data?: T };
	start_prefetch: (href: string) => void;
	stop_prefetch: (href: string) => void;
	save_current_scroll: () => void;
	get_default_error_boundary: () =>
		| ((props: { error: unknown }) => any)
		| undefined;
};

/////////////////////////////////////////////////////////////////////
/////// Constants
/////////////////////////////////////////////////////////////////////

export const MAX_SCROLL_ENTRIES = 50;
export const REFRESH_MAX_AGE_MS = 3333;
export const MAX_REDIRECTS = 10;
export const REVALIDATION_DEBOUNCE_MS = 8;
export const MAX_REVALIDATION_RETRIES = 8;
export const REVALIDATION_BACKOFF_BASE_MS = 500;
export const REVALIDATION_BACKOFF_CAP_MS = 30000;

const REVALIDATION_OK: RevalidationResult = { ok: true };
const REVALIDATION_EXHAUSTED: RevalidationResult = {
	ok: false,
	reason: "max_retries_exhausted",
};
let focus_revalidation_cleanup: (() => void) | null = null;

/////////////////////////////////////////////////////////////////////
/////// Module-Level Utilities
/////////////////////////////////////////////////////////////////////

type TestOptions = {
	reload?: () => void;
	hard_redirect?: (url: string) => void;
	scroll_to?: (x: number, y: number) => void;
};

export function apply_scroll(
	scroll: ScrollState | undefined,
	test_options?: TestOptions,
): void {
	if (!scroll) {
		return;
	}
	if ("hash" in scroll) {
		const raw = scroll.hash.startsWith("#")
			? scroll.hash.slice(1)
			: scroll.hash;
		let id: string;
		try {
			id = decodeURIComponent(raw);
		} catch {
			id = raw;
		}
		document.getElementById(id)?.scrollIntoView();
		return;
	}
	const scroll_to =
		test_options?.scroll_to ??
		((x: number, y: number) => {
			window.scrollTo(x, y);
		});
	scroll_to(scroll.x, scroll.y);
}

export function make_route_id(idx: number, pattern: string): string {
	return `${idx}:${pattern}`;
}

function is_abort_error(e: unknown): boolean {
	return e instanceof DOMException && e.name === "AbortError";
}

function new_abort_error(): DOMException {
	return new DOMException("Aborted", "AbortError");
}

function to_error_string(err: unknown): string {
	return err instanceof Error ? err.message : String(err);
}

function normalize_module_url(url: string): string {
	return new URL(url, window.location.href).pathname;
}

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

/////////////////////////////////////////////////////////////////////
/////// Client Core
/////////////////////////////////////////////////////////////////////

export function create_client_core(
	_: Omit<AppConfig, "__phantom_loaders" | "__phantom_actions">,
	commit: CommitFn,
	test_options?: TestOptions,
): Result<ClientCore> {
	/////// Pattern Registry
	const registry_res = createPatternRegistry({
		dynamicParamPrefixRune: ":",
		splatSegmentRune: "*",
		explicitIndexSegment: "_index",
	});
	if (!registry_res.ok) {
		return R.err(`Failed to create pattern registry: ${registry_res.err}`);
	}
	const pattern_registry = registry_res.val;

	/////// Internal Types

	type HistoryPosition = {
		href: string;
		key: string;
		state: unknown;
	};

	type RouteSnapshot = {
		position: HistoryPosition;
		route: RouteRecord;
	};

	type RouteRecord = {
		params: Record<string, string>;
		splat_values: string[];
		matches: RouteMatchRecord[];
		error: RouteErrorState | null;
		client_build_id: string;
	};

	type RouteMatchRecord = {
		pattern: string;
		input: unknown;
		module_url: string;
		module: Record<string, unknown>;
		loader_data: unknown;
		client_loader_data: unknown;
	};

	type RouteRenderCommitReason =
		| "initial"
		| "navigation"
		| "popstate"
		| "revalidation"
		| "hmr";

	type WorkProjection =
		| {
				kind: "navigation";
				skip_progress_indicator?: boolean;
		  }
		| {
				kind: "revalidation";
		  }
		| {
				kind: "submission";
				skip_progress_indicator?: boolean;
		  }
		| {
				kind: "prefetch";
		  };

	type NavOptions = {
		replace?: boolean;
		scroll_to_top?: boolean;
		state?: unknown;
		is_popstate?: boolean;
		popstate_scroll?: ScrollState;
		skip_progress_indicator?: boolean;
	};

	type NavResult = { didNavigate: boolean };

	type RedirectResult = "settled" | "transferred";

	type NavigationSource = "navigate" | "popstate" | "redirect";

	type NavFetchIntent = {
		kind: "nav";
		url: URL;
		options: NavOptions;
		source: NavigationSource;
		redirect_count: number;
		deferred: Deferred<NavResult>;
	};

	type RevalidationFetchIntent = { kind: "reval"; attempt: number };

	type ActiveFetchIntent = NavFetchIntent | RevalidationFetchIntent;

	type PrefetchFetchIntent = { kind: "prefetch" };

	type FetchIntent = ActiveFetchIntent | PrefetchFetchIntent;

	type FetchBase = {
		url: URL; // what we asked the server for (minus hash sensitivity)
		ac: AbortController;
		seq: number;
		data_promise: Promise<FetchResult>;
		cl_prefetches: ClientLoaderPrefetch[];
	};

	type ActiveFetch = FetchBase & {
		// Resolves when prefetch-phase preparation (modules, client-loader
		// prefetch seeding) is done after a prefetch is promoted.
		prepare_ready: Promise<void>;
		intent: ActiveFetchIntent;
	};

	type PrefetchFetch = FetchBase & {
		prepare_ready: Promise<void>;
		intent: PrefetchFetchIntent;
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

	type ClientLoaderPrefetch = {
		pattern: string;
		resolve_server_state: (data: ClientLoaderServerState) => void;
		abort: () => void;
		result_promise: Promise<unknown>;
	};

	type DecodedRoute = {
		pattern: string;
		input: unknown;
		module_url: string;
		loader_data: unknown;
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

	type PreparedRoute = {
		route: RouteRecord;
		apply_dom_side_effects: () => void;
	};

	type Submission = {
		ac: AbortController;
		key: string;
		method: string;
		href: string;
		skip_progress_indicator?: boolean;
	};

	type RefreshWaiter = Deferred<RevalidationResult>;

	type RefreshDemand = {
		// A fetch with seq > this value satisfies the demand.
		after_seq: number;
		waiters: RefreshWaiter[];
	};

	type RefreshState =
		| { kind: "idle" }
		| { kind: "pending"; demand: RefreshDemand; attempt: number }
		| {
				kind: "debouncing";
				demand: RefreshDemand;
				timer: ReturnType<typeof setTimeout>;
		  }
		| {
				kind: "retrying";
				demand: RefreshDemand;
				attempt: number;
				timer: ReturnType<typeof setTimeout>;
		  };

	type StoredScrollEntry = [string, { x: number; y: number }];

	/*
	Nine base facts:

	 1. phase: router lifecycle ('booting' | 'ready')
	 2. browser: the browser's current history entry
	 3. route_snapshot: current route data available to Vorma APIs
	 4. active: the one in-flight nav or revalidation (at most one, ever)
	 5. prefetch: the at-most-one in-flight prefetch
	 6. refresh: outstanding route data demand and its retry timing
	 7. submissions: concurrent action requests (independent of routes)
	 8. deferred_submit_redirect: submit redirect waiting for init completion
	 9. seq: monotonic counter; refresh is ordered by seq, never wall clock

	Aborting a fetch and publishing a route are each single operations.

	WorkProjection and WorkState are derived from these facts. RouteRenderState
	is derived only at the adapter commit boundary.
	*/

	/////// Base Facts

	let phase: "booting" | "ready" = "booting";
	let browser: HistoryPosition = { href: "", key: "", state: undefined };
	let route_snapshot: RouteSnapshot | null = null;
	let active: ActiveFetch | null = null;
	let prefetch: PrefetchFetch | null = null;
	let refresh: RefreshState = { kind: "idle" };
	const submissions = new Map<string, Submission>();
	let seq = 0;

	/////// Config And Listeners

	let client_build_id = "";
	let deployment_id = "";
	let use_view_transitions = false;
	let default_error_boundary:
		| ((props: { error: unknown }) => any)
		| undefined;
	let user_on_route_update:
		| ((
				route: RouteState,
				previousRoute: RouteState | null,
				reason: RouteUpdateReason,
		  ) => void)
		| undefined;
	let user_on_work_update: ((work: WorkState) => void) | undefined;
	let user_on_build_id_change:
		| ((prev: string, next: string) => void)
		| undefined;
	let deferred_submit_redirect: URL | null = null;
	let last_activity_ts = Date.now();
	let last_work_state: WorkState = empty_work_state();

	const work_update_listeners = new Set<(work: WorkState) => void>();
	const module_map: Record<string, ClientLoaderFn> = {};
	const search_schema_map: Record<string, unknown> = {};
	const hmr_rerun_patterns = new Set<string>();
	const module_cache = new Map<string, Record<string, unknown>>();

	const reload_page =
		test_options?.reload ??
		(() => {
			window.location.reload();
		});
	const hard_redirect =
		test_options?.hard_redirect ??
		((url: string) => {
			window.location.assign(url);
		});

	/////// URL And History

	function current_url(): URL {
		return new URL(window.location.href);
	}

	function next_seq(): number {
		seq++;
		return seq;
	}

	function is_http(href: string): boolean {
		try {
			const p = new URL(href).protocol;
			return p === "http:" || p === "https:";
		} catch {
			return false;
		}
	}

	function is_same_origin(url: URL): boolean {
		return url.origin === current_url().origin;
	}

	function is_same_origin_href(href: string): boolean {
		try {
			return new URL(href).origin === current_url().origin;
		} catch {
			return false;
		}
	}

	function matches_without_hash(a: URL, b: URL): boolean {
		const x = new URL(a.href);
		x.hash = "";
		const y = new URL(b.href);
		y.hash = "";
		return x.href === y.href;
	}

	function route_snapshot_matches(url: URL): boolean {
		return (
			route_snapshot !== null &&
			matches_without_hash(url, new URL(route_snapshot.position.href))
		);
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

	function read_browser_position(): HistoryPosition {
		const state = window.history.state;
		const key =
			state && typeof state === "object" && HISTORY_KEY_FIELD in state
				? (state as Record<string, string>)[HISTORY_KEY_FIELD]!
				: "";
		return {
			href: current_url().href,
			key,
			state: state?.[HISTORY_USER_STATE_FIELD],
		};
	}

	function commit_history(
		url: string,
		replace: boolean | undefined,
		user_state: unknown,
	): HistoryPosition {
		const key = make_history_key();
		const next_state: Record<string, unknown> = {
			[HISTORY_KEY_FIELD]: key,
			[HISTORY_USER_STATE_FIELD]: user_state,
		};

		if (replace) {
			const existing = window.history.state;
			const base =
				existing && typeof existing === "object" ? existing : {};
			window.history.replaceState(
				{ ...(base as object), ...next_state },
				"",
				url,
			);
		} else {
			window.history.pushState(next_state, "", url);
		}

		browser = { href: url, key, state: user_state };
		return browser;
	}

	function route_snapshot_matches_browser(): boolean {
		return (
			route_snapshot !== null &&
			route_snapshot.position.href === browser.href &&
			route_snapshot.position.key === browser.key
		);
	}

	/////// Scroll Storage

	function get_scroll_pos(): { x: number; y: number } {
		return { x: window.scrollX, y: window.scrollY };
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

	function save_scroll_for_key(
		key: string,
		pos: { x: number; y: number },
	): void {
		const entries = read_scroll_entries().filter((e) => e[0] !== key);
		entries.push([key, pos]);
		if (entries.length > MAX_SCROLL_ENTRIES) {
			entries.splice(0, entries.length - MAX_SCROLL_ENTRIES);
		}
		try {
			sessionStorage.setItem(SCROLL_STORAGE_KEY, JSON.stringify(entries));
		} catch {}
	}

	function get_scroll_for_key(
		key: string,
	): { x: number; y: number } | undefined {
		for (const [k, v] of read_scroll_entries()) {
			if (k === key) {
				return v;
			}
		}
		return undefined;
	}

	function save_current_scroll(): void {
		if (browser.key) {
			save_scroll_for_key(browser.key, get_scroll_pos());
		}
	}

	/////// Payload Decoding

	function decode_payload(raw: unknown, url: URL): DecodedPayload {
		const p = raw as Record<string, any>;
		const patterns: string[] = p.MatchedPatterns ?? [];
		const schemas: unknown[] = Array.isArray(p.SearchSchemas)
			? p.SearchSchemas
			: [];
		const loaders_data: unknown[] = p.LoadersData ?? [];
		const import_urls: string[] = p.ImportURLs ?? [];
		const err_idx: number | null = p.OutermostServerErrIdx ?? null;
		const err_msg: string = p.OutermostServerErr ?? "";
		const search_params = url.searchParams;

		const routes: DecodedRoute[] = patterns.map((pattern, i) => {
			const schema = schemas[i];
			search_schema_map[pattern] = schema;
			return {
				pattern,
				input: parseSearchParams(schema, search_params),
				module_url: import_urls[i] ?? "",
				loader_data: loaders_data[i],
				server_error:
					err_idx !== null && i === err_idx ? err_msg : undefined,
			};
		});

		let title: string | undefined;
		if (p.Title?.dangerousInnerHTML !== undefined) {
			const el = document.createElement("textarea");
			el.innerHTML = p.Title.dangerousInnerHTML;
			title = el.value;
		}

		return {
			routes,
			params: p.Params ?? {},
			splat_values: p.SplatValues ?? [],
			title,
			meta_head_els: p.MetaHeadEls ?? [],
			rest_head_els: p.RestHeadEls ?? [],
			css_bundles: p.CSSBundles ?? [],
			deps: p.Deps ?? [],
		};
	}

	/////// Modules And Client Loader Prefetches

	function make_cl_prefetch(input: {
		href: string;
		history_state: unknown;
		known_matches: ClientLoaderKnownMatch[];
		loader: ClientLoaderFn;
		params: Record<string, string>;
		pattern: string;
		route_input: unknown;
		signal: AbortSignal;
		splat_values: string[];
		trigger: "navigation" | "revalidation" | "prefetch";
	}): ClientLoaderPrefetch {
		let resolve_server_state!: (v: ClientLoaderServerState) => void;
		let reject_server_state!: (err: unknown) => void;
		const server_promise = new Promise<ClientLoaderServerState>(
			(res, rej) => {
				resolve_server_state = res;
				reject_server_state = rej;
			},
		);
		server_promise.catch(() => {});

		const ac = new AbortController();
		if (input.signal.aborted) {
			ac.abort();
			reject_server_state(new_abort_error());
		} else {
			input.signal.addEventListener(
				"abort",
				() => {
					ac.abort();
					reject_server_state(new_abort_error());
				},
				{ once: true },
			);
		}

		const result_promise = input.loader({
			trigger: input.trigger,
			href: input.href,
			historyState: input.history_state,
			pattern: input.pattern,
			params: input.params,
			splatValues: input.splat_values,
			input: input.route_input,
			knownMatches: input.known_matches,
			serverPromise: server_promise,
			signal: ac.signal,
		});
		result_promise.catch(() => {});

		return {
			pattern: input.pattern,
			resolve_server_state,
			abort: () => {
				ac.abort();
				reject_server_state(new_abort_error());
			},
			result_promise,
		};
	}

	function routes_to_known_matches(
		routes: DecodedRoute[],
	): ClientLoaderKnownMatch[] {
		return routes.map((r) => {
			return {
				pattern: r.pattern,
				input: r.input,
			};
		});
	}

	function build_server_state(
		routes: DecodedRoute[],
		idx: number,
	): ClientLoaderServerState {
		const err_idx = routes.findIndex((r) => {
			return r.server_error !== undefined;
		});
		return {
			clientBuildID: client_build_id,
			matches: routes.map((r) => {
				return {
					pattern: r.pattern,
					input: r.input,
					loaderData: r.loader_data,
				};
			}),
			outermostServerError:
				err_idx === -1
					? null
					: {
							idx: err_idx,
							error: routes[err_idx]!.server_error,
						},
			loaderData: routes[idx]?.loader_data,
		};
	}

	async function run_client_loaders(
		routes: DecodedRoute[],
		payload: DecodedPayload,
		cl_prefetches: ClientLoaderPrefetch[],
		trigger: "init" | "navigation" | "revalidation",
		href: string,
		history_state: unknown,
		signal: AbortSignal,
	): Promise<Array<{ data: unknown } | { error: unknown } | undefined>> {
		const by_pattern = new Map<string, ClientLoaderPrefetch>();
		for (const p of cl_prefetches) {
			by_pattern.set(p.pattern, p);
		}
		const err_idx = routes.findIndex((r) => r.server_error !== undefined);
		const known_matches = routes_to_known_matches(routes);

		const abort_later: Array<(() => void) | null> = [];
		const promises: Array<Promise<unknown>> = [];
		const retained_prefetches = new Set<ClientLoaderPrefetch>();

		for (let i = 0; i < routes.length; i++) {
			const route = routes[i]!;
			const existing = by_pattern.get(route.pattern);
			if (err_idx !== -1 && i >= err_idx) {
				existing?.abort();
				promises.push(Promise.resolve(undefined));
				abort_later.push(null);
				continue;
			}
			if (existing) {
				existing.resolve_server_state(build_server_state(routes, i));
				retained_prefetches.add(existing);
				abort_later.push(existing.abort);
				promises.push(existing.result_promise);
				continue;
			}
			const loader = module_map[route.pattern];
			if (!loader) {
				promises.push(Promise.resolve(undefined));
				abort_later.push(null);
				continue;
			}
			const ac = new AbortController();
			abort_later.push(() => {
				ac.abort();
			});
			if (signal.aborted) {
				ac.abort();
			} else {
				signal.addEventListener("abort", () => ac.abort(), {
					once: true,
				});
			}
			promises.push(
				loader({
					trigger,
					href,
					historyState: history_state,
					pattern: route.pattern,
					params: payload.params,
					splatValues: payload.splat_values,
					input: route.input,
					knownMatches: known_matches,
					serverPromise: Promise.resolve(
						build_server_state(routes, i),
					),
					signal: ac.signal,
				}),
			);
		}
		for (const p of cl_prefetches) {
			if (!retained_prefetches.has(p)) {
				p.abort();
			}
		}

		const wrapped = promises.map(async (p, i) => {
			return p.catch((err) => {
				// On non-abort failure, cascade abort to later routes to avoid
				// running loaders that can never be used.
				if (!is_abort_error(err)) {
					for (let j = i + 1; j < abort_later.length; j++) {
						abort_later[j]?.();
					}
				}
				throw err;
			});
		});

		const settled = await Promise.allSettled(wrapped);
		const out: Array<{ data: unknown } | { error: unknown } | undefined> =
			[];
		for (const r of settled) {
			if (r.status === "fulfilled") {
				out.push(r.value !== undefined ? { data: r.value } : undefined);
			} else if (!is_abort_error(r.reason)) {
				out.push({ error: to_error_string(r.reason) });
				break;
			} else {
				out.push(undefined);
				break;
			}
		}
		return out;
	}

	/////// Route Preparation

	async function prepare_modules(
		payload: DecodedPayload,
		signal: AbortSignal,
	): Promise<Map<string, Record<string, unknown>> | null> {
		preload_css(payload.css_bundles);
		const urls = payload.routes.map((r) => {
			return r.module_url;
		});
		const unique = [...new Set(urls.filter(Boolean))];
		const pairs = await Promise.all(
			unique.map(async (url) => {
				if (import.meta.env.DEV) {
					const key = normalize_module_url(url);
					const cached = module_cache.get(key);
					if (cached) {
						return [url, cached] as const;
					}
				}
				const mod = (await import(/* @vite-ignore */ url)) as Record<
					string,
					unknown
				>;
				if (import.meta.env.DEV) {
					module_cache.set(normalize_module_url(url), mod);
				}
				return [url, mod] as const;
			}),
		);
		const modules = new Map(pairs);
		if (signal.aborted) {
			return null;
		}
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
		return modules;
	}

	function build_route_record(
		payload: DecodedPayload,
		modules: Map<string, Record<string, unknown>>,
		cl_results: Array<{ data: unknown } | { error: unknown } | undefined>,
	): RouteRecord {
		const matches: RouteMatchRecord[] = payload.routes.map((route, i) => {
			const cl = cl_results[i];
			return {
				pattern: route.pattern,
				input: route.input,
				module_url: route.module_url,
				module: modules.get(route.module_url) ?? {},
				loader_data: route.loader_data,
				client_loader_data: cl && "data" in cl ? cl.data : undefined,
			};
		});
		let error: RouteErrorState | null = null;
		const server_error_idx = payload.routes.findIndex((route) => {
			return route.server_error !== undefined;
		});
		if (server_error_idx !== -1) {
			error = {
				idx: server_error_idx,
				error: payload.routes[server_error_idx]!.server_error,
				source: "server",
			};
		} else {
			const client_loader_error_idx = cl_results.findIndex((cl) => {
				return cl !== undefined && "error" in cl;
			});
			if (client_loader_error_idx !== -1) {
				error = {
					idx: client_loader_error_idx,
					error: (
						cl_results[client_loader_error_idx] as {
							error: unknown;
						}
					).error,
					source: "clientLoader",
				};
			}
		}
		return {
			params: payload.params,
			splat_values: payload.splat_values,
			matches,
			error,
			client_build_id,
		};
	}

	async function prepare_route(
		payload: DecodedPayload,
		cl_prefetches: ClientLoaderPrefetch[],
		trigger: "navigation" | "revalidation",
		href: string,
		history_state: unknown,
		signal: AbortSignal,
	): Promise<PreparedRoute | null> {
		const modules = await prepare_modules(payload, signal);
		if (!modules) {
			return null;
		}
		const cl_results = await run_client_loaders(
			payload.routes,
			payload,
			cl_prefetches,
			trigger,
			href,
			history_state,
			signal,
		);
		if (signal.aborted) {
			return null;
		}
		await wait_for_css(payload.css_bundles, signal);
		if (signal.aborted) {
			return null;
		}
		return {
			route: build_route_record(payload, modules, cl_results),
			apply_dom_side_effects: () => {
				apply_head_and_title(
					payload.title,
					payload.meta_head_els,
					payload.rest_head_els,
				);
				apply_css_bundles(payload.css_bundles);
				preload_modules(payload.deps);
			},
		};
	}

	/////// Fetch

	function detect_redirect(
		res: Response,
		base: URL,
	): { href: string; hard: boolean } | null {
		const hard = res.headers.get(X_VORMA_RELOAD);
		if (hard) {
			return { href: new URL(hard, base).href, hard: true };
		}
		const soft = res.headers.get(X_CLIENT_REDIRECT);
		if (soft) {
			return { href: new URL(soft, base).href, hard: false };
		}
		if (res.redirected && res.url && res.url !== base.href) {
			return { href: new URL(res.url, base).href, hard: false };
		}
		return null;
	}

	function update_build_id(res: Response): void {
		const next = res.headers.get(BUILD_ID_HEADER) ?? "";
		if (next && next !== client_build_id) {
			const prev = client_build_id;
			client_build_id = next;
			user_on_build_id_change?.(prev, next);
		}
	}

	function history_state_for_fetch(intent: FetchIntent): unknown {
		if (intent.kind === "nav") {
			return intent.options.is_popstate
				? browser.state
				: intent.options.state;
		}
		if (intent.kind === "reval") {
			return browser.state;
		}
		return undefined;
	}

	/////// Fetch Lifecycle

	function start_fetch<T extends FetchIntent>(
		url: URL,
		intent: T,
		is_revalidation: boolean,
		prepare_ready?: Promise<void>,
	): FetchBase & { prepare_ready: Promise<void>; intent: T } {
		const ac = new AbortController();
		const cl_prefetches: ClientLoaderPrefetch[] = [];
		let match = findNestedMatches(pattern_registry, url.pathname);
		if (!match) {
			const segments = url.pathname.split("/").filter(Boolean);
			for (let i = segments.length; i >= 0; i--) {
				const partial =
					i === 0 ? "/" : "/" + segments.slice(0, i).join("/");
				match = findNestedMatches(pattern_registry, partial);
				if (match) {
					break;
				}
			}
		}
		if (match) {
			const known_matches = match.matches.map((m) => {
				const pattern = m.registeredPattern.originalPattern;
				return {
					pattern,
					input: parseSearchParams(
						search_schema_map[pattern],
						url.searchParams,
					),
				};
			});
			const input_by_pattern = new Map(
				known_matches.map((m) => {
					return [m.pattern, m.input] as const;
				}),
			);
			const history_state = history_state_for_fetch(intent);
			for (const m of match.matches) {
				const pattern = m.registeredPattern.originalPattern;
				const loader = module_map[pattern];
				if (loader) {
					cl_prefetches.push(
						make_cl_prefetch({
							href: url.href,
							history_state,
							known_matches,
							loader,
							params: match.params,
							pattern,
							route_input: input_by_pattern.get(pattern),
							signal: ac.signal,
							splat_values: match.splatValues,
							trigger:
								intent.kind === "prefetch"
									? "prefetch"
									: intent.kind === "reval"
										? "revalidation"
										: "navigation",
						}),
					);
				}
			}
		}
		return {
			url,
			ac,
			seq: next_seq(),
			data_promise: (async () => {
				const modified = new URL(url.href);
				modified.searchParams.set(VORMA_JSON_KEY, client_build_id);
				if (is_revalidation && deployment_id) {
					modified.searchParams.set(
						VERCEL_DPL_QUERY_PARAM_KEY,
						deployment_id,
					);
				}
				const res = await fetch(modified, {
					signal: ac.signal,
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
			})(),
			cl_prefetches,
			prepare_ready: prepare_ready ?? Promise.resolve(),
			intent,
		};
	}

	function resolve_nav(f: ActiveFetch, result: NavResult): void {
		if (f.intent.kind === "nav") {
			f.intent.deferred.resolve(result);
		}
	}

	function can_commit(f: ActiveFetch): boolean {
		return active === f && !f.ac.signal.aborted;
	}

	// Core lifecycle for any fetch intent. The only place that drives
	// fetch-to-publish: resolve response, follow redirects, prepare, publish.
	async function run_active(f: ActiveFetch): Promise<void> {
		active = f;
		notify_work_update();

		let published = false;
		let nav_transferred = false;
		try {
			const result = await f.data_promise;
			if (!can_commit(f)) {
				return;
			}

			if (result.kind === "error") {
				return;
			}

			if (result.response) {
				update_build_id(result.response);
			}

			if (result.kind === "redirect") {
				nav_transferred = handle_redirect(f, result) === "transferred";
				return;
			}

			const payload = decode_payload(result.data, f.url);
			preload_modules(payload.deps);

			await f.prepare_ready;
			if (!can_commit(f)) {
				return;
			}

			const prepared = await prepare_route(
				payload,
				f.cl_prefetches,
				f.intent.kind === "reval" ? "revalidation" : "navigation",
				f.url.href,
				history_state_for_fetch(f.intent),
				f.ac.signal,
			);
			if (!prepared || !can_commit(f)) {
				return;
			}

			const was_published = await publish(f, prepared);
			if (was_published) {
				published = true;
				last_activity_ts = Date.now();
				mark_fresh(f);
			}
		} catch {
			// swallow
		} finally {
			if (f.intent.kind === "nav" && !nav_transferred) {
				resolve_nav(f, { didNavigate: published });
			}
			if (active === f) {
				active = null;
			}
			notify_work_update();
			maybe_revalidate();
		}
	}

	// Publish a prepared route. Clears `active` as soon as the commit lands
	// inside the view transition so subsequent same-page navs can fire
	// while the transition animation is still running. Returns true if the
	// commit actually happened (it may be superseded mid-callback).
	async function publish(
		f: ActiveFetch,
		prepared: PreparedRoute,
	): Promise<boolean> {
		const commit_reason: Exclude<RouteUpdateReason, "init"> =
			f.intent.kind === "reval"
				? "revalidation"
				: f.intent.options.is_popstate
					? "popstate"
					: "navigation";

		const prev_snapshot = route_snapshot;
		if (!prev_snapshot) {
			return false;
		}

		const next_href =
			f.intent.kind === "nav"
				? f.intent.url.href
				: prev_snapshot.position.href;
		const next_history_state =
			f.intent.kind === "nav"
				? f.intent.options.is_popstate
					? browser.state
					: f.intent.options.state
				: prev_snapshot.position.state;
		const yield_hooks = prev_snapshot.route.matches
			.map((m) => {
				return (m.module.default as RouteDefinition | undefined)
					?.before_route_yield;
			})
			.filter((h): h is BeforeRouteYieldFn => {
				return typeof h === "function";
			});
		const commit_hooks = prepared.route.matches
			.map((m) => {
				return (m.module.default as RouteDefinition | undefined)
					?.before_route_commit;
			})
			.filter((h): h is BeforeRouteCommitFn => {
				return typeof h === "function";
			});
		if (yield_hooks.length > 0 || commit_hooks.length > 0) {
			const current = route_snapshot_to_state(prev_snapshot);
			const next = route_record_to_state(
				prepared.route,
				next_href,
				next_history_state,
			);
			await Promise.all(
				yield_hooks
					.map((h) => {
						return h({
							trigger: commit_reason,
							signal: f.ac.signal,
							current,
							next,
						});
					})
					.concat(
						commit_hooks.map((h) => {
							return h({
								trigger: commit_reason,
								signal: f.ac.signal,
								current,
								next,
							});
						}),
					),
			);
			if (!can_commit(f)) {
				return false;
			}
		}

		let did_publish = false;

		const do_publish = () => {
			if (!can_commit(f)) {
				return;
			}

			let position: HistoryPosition;
			if (f.intent.kind === "nav" && !f.intent.options.is_popstate) {
				save_current_scroll();
				position = commit_history(
					f.intent.url.href,
					f.intent.options.replace,
					f.intent.options.state,
				);
			} else {
				// popstate: browser already at the new URL; revalidation: URL unchanged
				position = browser;
			}

			const prev = route_snapshot;
			prepared.apply_dom_side_effects();
			route_snapshot = { position, route: prepared.route };

			let scroll_intent: ScrollIntent | undefined;
			if (f.intent.kind === "nav") {
				let ss: ScrollState | undefined;
				if (f.intent.options.is_popstate) {
					if (normalize_hash(f.intent.url.hash).length > 0) {
						ss = { hash: f.intent.url.hash };
					} else {
						ss = f.intent.options.popstate_scroll ?? { x: 0, y: 0 };
					}
				} else if (normalize_hash(f.intent.url.hash).length > 0) {
					ss = { hash: f.intent.url.hash };
				} else if (f.intent.options.scroll_to_top !== false) {
					ss = { x: 0, y: 0 };
				}
				if (ss) {
					scroll_intent = {
						scroll: ss,
						target_route_id: make_route_id(
							prepared.route.matches.length - 1,
							prepared.route.matches[
								prepared.route.matches.length - 1
							]?.pattern ?? "",
						),
					};
				}
			}

			commit_route_snapshot(
				commit_reason,
				prev,
				route_snapshot,
				scroll_intent,
			);
			active = null;
			did_publish = true;
		};

		const vt = (document as any).startViewTransition;
		if (
			f.intent.kind === "nav" &&
			use_view_transitions &&
			typeof vt === "function"
		) {
			const transition = vt.call(document, do_publish);
			// Prefer updateCallbackDone (modern API); fall back to finished.
			await (transition.updateCallbackDone ?? transition.finished);
			if (transition.finished) {
				await transition.finished;
			}
		} else {
			do_publish();
		}
		return did_publish;
	}

	function route_to_render_state(
		route: RouteRecord,
		history_state: unknown,
	): RouteRenderState {
		return {
			entries: route.matches.map((m) => {
				return {
					pattern: m.pattern,
					input: m.input,
					module_url: m.module_url,
					module: m.module,
					loader_data: m.loader_data,
					client_loader_data: m.client_loader_data,
				};
			}),
			error: route.error,
			params: route.params,
			splat_values: route.splat_values,
			client_build_id: route.client_build_id,
			history_state,
		};
	}

	function route_snapshot_to_state(snapshot: RouteSnapshot): RouteState {
		return route_record_to_state(
			snapshot.route,
			snapshot.position.href,
			snapshot.position.state,
		);
	}

	function route_record_to_state(
		route: RouteRecord,
		href: string,
		history_state: unknown,
	): RouteState {
		return {
			href,
			historyState: history_state,
			clientBuildID: route.client_build_id,
			params: route.params,
			splatValues: route.splat_values,
			matches: route.matches.map((m) => {
				return {
					pattern: m.pattern,
					input: m.input,
					loaderData: m.loader_data,
					clientLoaderData: m.client_loader_data,
				};
			}),
			error: route.error,
		};
	}

	function to_route_update_reason(
		reason: RouteRenderCommitReason,
	): RouteUpdateReason {
		if (reason === "initial") {
			return "init";
		}
		if (reason === "hmr") {
			return "revalidation";
		}
		return reason;
	}

	function commit_route_snapshot(
		reason: RouteRenderCommitReason,
		prev: RouteSnapshot | null,
		next: RouteSnapshot,
		scroll_intent?: ScrollIntent,
	): void {
		commit(
			route_to_render_state(next.route, next.position.state),
			scroll_intent,
		);
		if (!user_on_route_update) {
			return;
		}

		const previous_route = prev ? route_snapshot_to_state(prev) : null;
		const route = route_snapshot_to_state(next);
		if (previous_route && jsonDeepEquals(previous_route, route)) {
			return;
		}
		user_on_route_update(
			route,
			previous_route,
			to_route_update_reason(reason),
		);
	}

	function handle_redirect(
		f: ActiveFetch,
		redirect: Extract<FetchResult, { kind: "redirect" }>,
	): RedirectResult {
		const nav_intent = f.intent.kind === "nav" ? f.intent : null;

		// Invalid scheme: treat as failure for nav, no-op otherwise.
		if (!is_http(redirect.href)) {
			if (nav_intent) {
				nav_intent.deferred.resolve({ didNavigate: false });
			}
			active = null;
			return "settled";
		}

		// Hard redirect or cross-origin: leave the page.
		if (redirect.hard || !is_same_origin_href(redirect.href)) {
			hard_redirect(redirect.href);
			if (nav_intent) {
				nav_intent.deferred.resolve({ didNavigate: false });
			}
			active = null;
			return "settled";
		}

		// Redirect loop guard (only meaningful for nav, which tracks count).
		if (nav_intent && nav_intent.redirect_count >= MAX_REDIRECTS) {
			nav_intent.deferred.resolve({ didNavigate: false });
			active = null;
			return "settled";
		}

		const target = new URL(redirect.href);
		active = null;

		// Same-page redirect (same path, maybe different hash): no re-fetch,
		// just handle as same-page nav.
		if (route_snapshot_matches(target)) {
			const result = handle_same_page_nav(
				target,
				nav_intent?.options ?? {},
			);
			if (nav_intent) {
				nav_intent.deferred.resolve(result);
			}
			return "settled";
		}

		// Continue nav chain at redirect target.
		if (nav_intent) {
			void start_nav_inner(
				target,
				nav_intent.options,
				nav_intent.redirect_count + 1,
				{
					reuse_deferred: nav_intent.deferred,
					source: "redirect",
				},
			);
			return "transferred";
		} else {
			// Revalidation redirect becomes a replace navigation.
			void start_nav_inner(target, { replace: true }, 0, {
				source: "redirect",
			});
			return "settled";
		}
	}

	/////// Navigation Entry

	function try_merge_active_nav(
		url: URL,
		options: NavOptions,
		source: NavigationSource,
		deferred: Deferred<NavResult>,
	): boolean {
		if (
			!active ||
			active.intent.kind !== "nav" ||
			!matches_without_hash(active.url, url)
		) {
			return false;
		}

		// Identical intent: share the result promise.
		const cur = active.intent;
		if (
			cur.url.href === url.href &&
			cur.options.replace === options.replace &&
			cur.options.scroll_to_top === options.scroll_to_top &&
			cur.options.state === options.state &&
			cur.options.is_popstate === options.is_popstate &&
			cur.options.popstate_scroll === options.popstate_scroll &&
			cur.options.skip_progress_indicator ===
				options.skip_progress_indicator &&
			cur.source === source
		) {
			cur.deferred.promise.then(deferred.resolve, () =>
				deferred.resolve({ didNavigate: false }),
			);
			return true;
		}

		// Same path, different hash or options: swap the request.
		cur.deferred.resolve({ didNavigate: false });
		active.intent = {
			kind: "nav",
			url,
			options,
			source,
			redirect_count: cur.redirect_count,
			deferred,
		};
		notify_work_update();
		return true;
	}

	function cancel_prefetch(): void {
		if (prefetch) {
			const p = prefetch;
			prefetch = null;
			p.ac.abort();
			notify_work_update();
		}
	}

	function handle_same_page_nav(url: URL, options: NavOptions): NavResult {
		if (!route_snapshot) {
			return { didNavigate: false };
		}
		const cur_url = new URL(route_snapshot.position.href);
		const target_hash = normalize_hash(url.hash);
		const current_hash = normalize_hash(cur_url.hash);

		if (target_hash !== current_hash) {
			save_current_scroll();
			const position = commit_history(
				url.href,
				options.replace,
				options.state,
			);
			const prev = route_snapshot;
			route_snapshot = { position, route: route_snapshot.route };
			commit_route_snapshot("navigation", prev, route_snapshot);
			apply_scroll(
				target_hash.length > 0 ? { hash: url.hash } : { x: 0, y: 0 },
				test_options,
			);
			return { didNavigate: true };
		}

		if (options.replace) {
			const position = commit_history(url.href, true, options.state);
			const prev = route_snapshot;
			route_snapshot = { position, route: route_snapshot.route };
			commit_route_snapshot("navigation", prev, route_snapshot);
		}
		apply_scroll({ x: 0, y: 0 }, test_options);
		return { didNavigate: false };
	}

	function start_nav_inner(
		url: URL,
		options: NavOptions,
		redirect_count: number,
		reuse?: {
			reuse_deferred?: Deferred<NavResult>;
			source?: NavigationSource;
		},
	): Promise<NavResult> {
		const deferred = reuse?.reuse_deferred ?? make_deferred<NavResult>();
		const source =
			reuse?.source ?? (options.is_popstate ? "popstate" : "navigate");

		// Same-page short-circuit (not for popstate, which always owns the commit).
		if (!options.is_popstate && route_snapshot_matches(url)) {
			const result = handle_same_page_nav(url, options);
			deferred.resolve(result);
			return deferred.promise;
		}

		// Merge into active nav if URLs match by path.
		if (try_merge_active_nav(url, options, source, deferred)) {
			return deferred.promise;
		}

		// Supersede any other active work.
		if (active) {
			const prev = active;
			active = null;
			resolve_nav(prev, { didNavigate: false });
			prev.ac.abort();
		}

		// Promote matching prefetch if present.
		let promoted: PrefetchFetch | null = null;
		if (prefetch && matches_without_hash(prefetch.url, url)) {
			promoted = prefetch;
			prefetch = null;
		}
		const fetch_record: ActiveFetch = promoted
			? {
					...promoted,
					intent: {
						kind: "nav",
						url,
						options,
						source,
						redirect_count,
						deferred,
					},
				}
			: (cancel_prefetch(),
				start_fetch(
					url,
					{
						kind: "nav",
						url,
						options,
						source,
						redirect_count,
						deferred,
					},
					false,
				));

		void run_active(fetch_record);
		return deferred.promise;
	}

	/////// Refresh

	function mark_fresh(f: ActiveFetch): void {
		const demand = refresh_demand();
		if (!demand) {
			return;
		}
		if (f.seq <= demand.after_seq) {
			return;
		}
		const waiters = demand.waiters;
		clear_refresh();
		for (const w of waiters) {
			w.resolve(REVALIDATION_OK);
		}
	}

	function refresh_demand(): RefreshDemand | null {
		if (refresh.kind === "idle") {
			return null;
		}
		return refresh.demand;
	}

	function clear_refresh(): void {
		if (refresh.kind === "debouncing" || refresh.kind === "retrying") {
			clearTimeout(refresh.timer);
		}
		refresh = { kind: "idle" };
	}

	function require_refresh(waiter?: RefreshWaiter, debounce?: boolean): void {
		const waiters = refresh_demand()?.waiters ?? [];
		clear_refresh();
		if (waiter) {
			waiters.push(waiter);
		}
		const demand: RefreshDemand = { after_seq: next_seq(), waiters };

		if (debounce) {
			let next_refresh!: Extract<RefreshState, { kind: "debouncing" }>;
			const timer = setTimeout(() => {
				if (refresh !== next_refresh) {
					return;
				}
				refresh = { kind: "pending", demand, attempt: 0 };
				maybe_revalidate();
			}, REVALIDATION_DEBOUNCE_MS);
			next_refresh = { kind: "debouncing", demand, timer };
			refresh = next_refresh;
		} else {
			refresh = { kind: "pending", demand, attempt: 0 };
		}
	}

	function active_will_refresh(): boolean {
		const demand = refresh_demand();
		return (
			active !== null && demand !== null && active.seq > demand.after_seq
		);
	}

	function maybe_revalidate(): void {
		if (phase !== "ready") {
			return;
		}
		if (refresh.kind === "idle") {
			return;
		}
		if (refresh.kind === "debouncing") {
			return;
		}
		if (active_will_refresh() || active) {
			return;
		}
		if (refresh.kind === "retrying") {
			return;
		}

		const demand = refresh.demand;
		const attempt = refresh.attempt;
		if (attempt >= MAX_REVALIDATION_RETRIES) {
			clear_refresh();
			for (const w of demand.waiters) {
				w.resolve(REVALIDATION_EXHAUSTED);
			}
			notify_work_update();
			return;
		}

		const delay =
			attempt === 0
				? 0
				: Math.min(
						REVALIDATION_BACKOFF_BASE_MS * Math.pow(2, attempt - 1),
						REVALIDATION_BACKOFF_CAP_MS,
					);

		if (delay === 0) {
			refresh = { kind: "pending", demand, attempt: attempt + 1 };
			void run_revalidation().catch(() => {});
		} else {
			let next_refresh!: Extract<RefreshState, { kind: "retrying" }>;
			const timer = setTimeout(() => {
				if (refresh !== next_refresh) {
					return;
				}
				refresh = { kind: "pending", demand, attempt: attempt + 1 };
				if (active) {
					return;
				}
				void run_revalidation().catch(() => {});
			}, delay);
			next_refresh = {
				kind: "retrying",
				demand,
				attempt: attempt + 1,
				timer,
			};
			refresh = next_refresh;
			notify_work_update();
		}
	}

	async function run_revalidation(): Promise<void> {
		const url = new URL(browser.href || window.location.href);
		const attempt = refresh.kind === "pending" ? refresh.attempt : 0;
		const f = start_fetch(url, { kind: "reval", attempt }, true);

		// Guard: discard if URL path changes during flight (hash-only changes
		// are OK; matches_without_hash on publish time handles it).
		await run_active_with_url_guard(f, url);
	}

	async function run_active_with_url_guard(
		f: ActiveFetch,
		expected: URL,
	): Promise<void> {
		active = f;
		notify_work_update();

		try {
			const result = await f.data_promise;
			if (!can_commit(f)) {
				return;
			}
			if (!matches_without_hash(new URL(browser.href), expected)) {
				return;
			}
			if (result.response) {
				update_build_id(result.response);
			}
			if (result.kind === "error") {
				return;
			}
			if (result.kind === "redirect") {
				handle_redirect(f, result);
				return;
			}

			const payload = decode_payload(result.data, f.url);
			preload_modules(payload.deps);

			const prepared = await prepare_route(
				payload,
				f.cl_prefetches,
				"revalidation",
				f.url.href,
				history_state_for_fetch(f.intent),
				f.ac.signal,
			);
			if (!prepared || !can_commit(f)) {
				return;
			}
			if (!matches_without_hash(new URL(browser.href), expected)) {
				return;
			}

			const did_publish = await publish(f, prepared);
			if (did_publish) {
				last_activity_ts = Date.now();
				mark_fresh(f);
			}
		} catch {
			// swallow
		} finally {
			if (active === f) {
				active = null;
			}
			notify_work_update();
			maybe_revalidate();
		}
	}

	/////// Prefetch

	async function prepare_prefetch(f: PrefetchFetch): Promise<void> {
		try {
			const result = await f.data_promise;
			if (f.ac.signal.aborted || result.kind !== "data") {
				if (prefetch === f) {
					prefetch = null;
					notify_work_update();
				}
				return;
			}
			const payload = decode_payload(result.data, f.url);
			preload_modules(payload.deps);
			const modules = await prepare_modules(payload, f.ac.signal);
			if (!modules || f.ac.signal.aborted) {
				if (prefetch === f) {
					prefetch = null;
					notify_work_update();
				}
				return;
			}
			const by_pattern = new Map<string, ClientLoaderPrefetch>();
			for (const p of f.cl_prefetches) {
				by_pattern.set(p.pattern, p);
			}
			const err_idx = payload.routes.findIndex((r) => {
				return r.server_error !== undefined;
			});
			const known_matches = routes_to_known_matches(payload.routes);
			const next_cl_prefetches: ClientLoaderPrefetch[] = [];
			const retained_prefetches = new Set<ClientLoaderPrefetch>();
			for (let i = 0; i < payload.routes.length; i++) {
				const route = payload.routes[i]!;
				const existing = by_pattern.get(route.pattern);
				if (err_idx !== -1 && i >= err_idx) {
					existing?.abort();
					continue;
				}
				if (existing) {
					existing.resolve_server_state(
						build_server_state(payload.routes, i),
					);
					next_cl_prefetches.push(existing);
					retained_prefetches.add(existing);
					continue;
				}
				const loader = module_map[route.pattern];
				if (!loader) {
					continue;
				}
				const p = make_cl_prefetch({
					href: f.url.href,
					history_state: undefined,
					known_matches,
					loader,
					params: payload.params,
					pattern: route.pattern,
					route_input: route.input,
					signal: f.ac.signal,
					splat_values: payload.splat_values,
					trigger: "prefetch",
				});
				p.resolve_server_state(build_server_state(payload.routes, i));
				next_cl_prefetches.push(p);
				retained_prefetches.add(p);
			}
			for (const p of f.cl_prefetches) {
				if (!retained_prefetches.has(p)) {
					p.abort();
				}
			}
			f.cl_prefetches = next_cl_prefetches;
		} catch {
			if (prefetch === f) {
				prefetch = null;
				notify_work_update();
			}
		}
	}

	function start_prefetch(href: string): void {
		if (phase !== "ready") {
			return;
		}
		const url = new URL(href, window.location.href);
		if (!is_same_origin(url) || route_snapshot_matches(url)) {
			return;
		}
		if (
			active?.intent.kind === "nav" &&
			matches_without_hash(active.url, url)
		) {
			return;
		}
		if (prefetch && matches_without_hash(prefetch.url, url)) {
			return;
		}
		cancel_prefetch();

		// Prefetch's prepare_ready resolves when modules + CL seeding are done.
		let resolve_prepare!: () => void;
		const prepare_ready = new Promise<void>((r) => {
			resolve_prepare = r;
		});

		const f = start_fetch(url, { kind: "prefetch" }, false, prepare_ready);
		prefetch = f;
		notify_work_update();
		void prepare_prefetch(f).finally(() => resolve_prepare());
	}

	function stop_prefetch(href: string): void {
		const url = new URL(href, window.location.href);
		if (prefetch && matches_without_hash(prefetch.url, url)) {
			cancel_prefetch();
		}
	}

	/////// Submit

	async function submit_inner<T = unknown>(
		url: string | URL,
		request_init?: RequestInit,
		options?: {
			actionKind?: ActionKind;
			dedupeKey?: string;
			revalidate?: boolean;
			skipProgressIndicator?: boolean;
		},
	): Promise<SubmitResult<T>> {
		if (!route_snapshot) {
			throw new Error("Vorma not initialized");
		}
		const resolved = new URL(String(url), window.location.href);

		if (!is_same_origin(resolved)) {
			return {
				success: false,
				error: `submit only supports same-origin targets. Received: "${resolved.href}".`,
				revalidationPromise: Promise.resolve(REVALIDATION_OK),
			};
		}

		const method = request_init?.method
			? request_init.method.toUpperCase().trim()
			: "GET";
		const action_kind =
			options?.actionKind ??
			(method === "GET" || method === "HEAD" ? "query" : "mutation");
		let should_revalidate = action_kind === "mutation";
		if (options?.revalidate !== undefined) {
			should_revalidate = options.revalidate;
		}

		let dedupe_key = options?.dedupeKey;
		if (dedupe_key) {
			submissions.get(dedupe_key)?.ac.abort();
		} else {
			dedupe_key = crypto.randomUUID();
		}

		const ac = new AbortController();
		const sub: Submission = {
			ac,
			key: dedupe_key,
			method,
			href: resolved.href,
			skip_progress_indicator: options?.skipProgressIndicator,
		};
		submissions.set(dedupe_key, sub);
		notify_work_update();

		let revalidation_promise: Promise<RevalidationResult> =
			Promise.resolve(REVALIDATION_OK);
		let did_dispatch = false;

		function schedule_revalidation(): void {
			if (!should_revalidate) {
				return;
			}
			if (phase === "ready") {
				const waiter = make_deferred<RevalidationResult>();
				revalidation_promise = waiter.promise;
				require_refresh(waiter);
				maybe_revalidate();
			} else {
				// During boot, register refresh demand so post-init
				// maybe_revalidate will fire. Do not attach a waiter;
				// the returned revalidationPromise stays resolved so initial
				// client loaders awaiting it do not deadlock.
				require_refresh();
			}
		}

		try {
			const is_get = method === "GET" || method === "HEAD";

			const headers = new Headers();
			if (deployment_id) {
				headers.set(VERCEL_X_DEPLOYMENT_ID, deployment_id);
			}
			new Headers(request_init?.headers ?? undefined).forEach((v, k) => {
				headers.set(k, v);
			});
			headers.set(X_ACCEPTS_CLIENT_REDIRECT, "1");

			const body = request_init?.body;
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
				...request_init,
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

			did_dispatch = true;
			const res = await fetch(resolved, final_init);

			update_build_id(res);

			const redirect = detect_redirect(res, resolved);
			if (redirect && !ac.signal.aborted) {
				if (!is_http(redirect.href)) {
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
				if (phase !== "ready") {
					deferred_submit_redirect = new URL(redirect.href);
					return {
						success: true,
						data: undefined as T,
						response: res,
						revalidationPromise: revalidation_promise,
					};
				}
				void start_nav_inner(
					new URL(redirect.href),
					{ replace: true },
					0,
					{ source: "redirect" },
				);
				return {
					success: true,
					data: undefined as T,
					response: res,
					revalidationPromise: revalidation_promise,
				};
			}

			if (!res.ok) {
				schedule_revalidation();
				return {
					success: false,
					error: res.statusText,
					response: res,
					revalidationPromise: revalidation_promise,
				};
			}

			let data: unknown;
			if (res.status !== 204) {
				const ct = res.headers.get("Content-Type");
				if (ct?.toLowerCase().includes("json")) {
					data = await res.json();
				} else {
					const t = await res.text();
					data = t.length > 0 ? t : undefined;
				}
			}

			schedule_revalidation();

			return {
				success: true,
				data: data as T,
				response: res,
				revalidationPromise: revalidation_promise,
			};
		} catch (e) {
			if (is_abort_error(e)) {
				if (did_dispatch) {
					schedule_revalidation();
				}
				return {
					success: false,
					error: "Aborted",
					revalidationPromise: revalidation_promise,
				};
			}
			if (did_dispatch) {
				schedule_revalidation();
			}
			return {
				success: false,
				error: String(e instanceof Error ? e.message : e),
				revalidationPromise: revalidation_promise,
			};
		} finally {
			if (submissions.get(dedupe_key)?.ac === ac) {
				submissions.delete(dedupe_key);
			}
			notify_work_update();
		}
	}

	/////// Popstate

	async function handle_popstate(): Promise<void> {
		const prev = browser;
		const next = read_browser_position();
		if (next.key === prev.key) {
			return;
		}

		if (prev.key) {
			save_scroll_for_key(prev.key, get_scroll_pos());
		}
		browser = next;

		const prev_url = new URL(prev.href);
		const next_url = new URL(next.href);

		// Hash-only popstate on same path: scroll, no fetch.
		if (matches_without_hash(prev_url, next_url)) {
			if (route_snapshot) {
				const prev_snapshot = route_snapshot;
				route_snapshot = {
					position: browser,
					route: route_snapshot.route,
				};
				commit_route_snapshot(
					"popstate",
					prev_snapshot,
					route_snapshot,
				);
			}
			const hash = normalize_hash(next_url.hash);
			apply_scroll(
				hash.length > 0
					? { hash: next_url.hash }
					: (get_scroll_for_key(next.key) ?? { x: 0, y: 0 }),
				test_options,
			);
			return;
		}

		const restored_scroll = get_scroll_for_key(next.key);

		try {
			const result = await start_nav_inner(
				next_url,
				{ is_popstate: true, popstate_scroll: restored_scroll },
				0,
			);
			if (
				!result.didNavigate &&
				!active &&
				!route_snapshot_matches_browser()
			) {
				reload_page();
			}
		} catch {
			if (!active && !route_snapshot_matches_browser()) {
				reload_page();
			}
		}
	}

	/////// Status

	function derive_work_projection(): WorkProjection[] {
		const work: WorkProjection[] = [];

		const active_fetch = active;
		if (active_fetch?.intent.kind === "nav") {
			work.push({
				kind: "navigation",
				skip_progress_indicator:
					active_fetch.intent.options.skip_progress_indicator,
			});
		}

		const pending_revalidation =
			refresh.kind !== "idle" &&
			(refresh.kind === "debouncing" ||
				refresh.kind === "retrying" ||
				(!active_will_refresh() && !active));
		if (active_fetch?.intent.kind === "reval" || pending_revalidation) {
			work.push({
				kind: "revalidation",
			});
		}

		for (const s of submissions.values()) {
			work.push({
				kind: "submission",
				skip_progress_indicator: s.skip_progress_indicator,
			});
		}

		if (prefetch) {
			work.push({
				kind: "prefetch",
			});
		}

		return work;
	}

	function empty_work_state(): WorkState {
		return {
			navigation: null,
			revalidation: null,
			prefetch: null,
			submissions: [],
		};
	}

	function derive_work_state(): WorkState {
		const active_fetch = active;
		let revalidation: WorkState["revalidation"] = null;
		if (active_fetch?.intent.kind === "reval") {
			revalidation = {
				status: "running",
				attempt: active_fetch.intent.attempt,
			};
		} else if (refresh.kind === "debouncing") {
			revalidation = { status: "debouncing", attempt: 0 };
		} else if (refresh.kind === "retrying") {
			revalidation = {
				status: "retrying",
				attempt: refresh.attempt,
			};
		}

		return {
			navigation:
				active_fetch?.intent.kind === "nav"
					? {
							href: active_fetch.intent.url.href,
							replace:
								active_fetch.intent.options.replace === true,
							source: active_fetch.intent.source,
						}
					: null,
			revalidation,
			prefetch: prefetch ? { href: prefetch.url.href } : null,
			submissions: Array.from(submissions.values(), (s) => {
				return {
					key: s.key,
					method: s.method,
					href: s.href,
				};
			}),
		};
	}

	function notify_work_update(): void {
		const next = derive_work_state();
		if (jsonDeepEquals(last_work_state, next)) {
			return;
		}
		last_work_state = next;
		for (const fn of work_update_listeners) {
			fn(next);
		}
		user_on_work_update?.(next);
	}

	/////// HMR

	function register_hmr(): void {
		if (!import.meta.env.DEV || !import.meta.hot) {
			return;
		}
		window.__vorma_hmr_route_update = async (raw_url, mod) => {
			if (!route_snapshot) {
				return;
			}
			const url = normalize_module_url(raw_url);
			module_cache.set(url, mod);
			const idx = route_snapshot.route.matches.findIndex(
				(m) => normalize_module_url(m.module_url) === url,
			);
			if (idx === -1) {
				return;
			}

			const match = route_snapshot.route.matches[idx]!;
			const def = mod.default as RouteDefinition | undefined;
			if (def?.client_loader) {
				module_map[match.pattern] = def.client_loader;
				registerPattern(pattern_registry, match.pattern);
			} else {
				delete module_map[match.pattern];
			}

			let client_loader_data = match.client_loader_data;
			if (hmr_rerun_patterns.has(match.pattern)) {
				const loader = module_map[match.pattern];
				if (loader) {
					try {
						const current_route = route_snapshot.route;
						const routes = current_route.matches.map((m, i) => {
							return {
								pattern: m.pattern,
								input: m.input,
								module_url: m.module_url,
								loader_data: m.loader_data,
								server_error:
									current_route.error?.source === "server" &&
									current_route.error.idx === i
										? current_route.error.error
										: undefined,
							};
						});
						client_loader_data = await loader({
							trigger: "revalidation",
							href: route_snapshot.position.href,
							historyState: route_snapshot.position.state,
							pattern: match.pattern,
							params: route_snapshot.route.params,
							splatValues: route_snapshot.route.splat_values,
							input: match.input,
							knownMatches: routes_to_known_matches(routes),
							serverPromise: Promise.resolve(
								build_server_state(routes, idx),
							),
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
			if (!route_snapshot) {
				return;
			}

			const prev = route_snapshot;
			const matches = route_snapshot.route.matches.map((m, i) =>
				i === idx ? { ...m, module: mod, client_loader_data } : m,
			);
			route_snapshot = {
				position: route_snapshot.position,
				route: { ...route_snapshot.route, matches },
			};
			commit_route_snapshot("hmr", prev, route_snapshot);
		};
	}

	/////// Progress Indicator

	function setup_progress_indicator(config: ProgressIndicatorConfig): void {
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

		const should_run = () => {
			for (const w of derive_work_projection()) {
				if (
					w.kind === "navigation" &&
					inc_nav &&
					!w.skip_progress_indicator
				) {
					return true;
				}
				if (w.kind === "revalidation" && inc_rev) {
					return true;
				}
				if (
					w.kind === "submission" &&
					inc_sub &&
					!w.skip_progress_indicator
				) {
					return true;
				}
			}
			return false;
		};

		const sync = () => {
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
		};

		work_update_listeners.add(sync);
		sync();
	}

	/////// Init

	async function init(options: InitOptions): Promise<Result<void>> {
		const payload_el = document.getElementById(DATA_SCRIPT_ID);
		if (!payload_el) {
			return R.err(`Missing element: #${DATA_SCRIPT_ID}`);
		}
		let raw_payload: Record<string, any>;
		try {
			raw_payload = JSON.parse(payload_el.textContent ?? "{}");
		} catch (err) {
			return R.err(
				`Failed to parse #${DATA_SCRIPT_ID}: ${to_error_string(err)}`,
			);
		}
		if (raw_payload.ClientBuildID) {
			client_build_id = raw_payload.ClientBuildID;
		}
		if (raw_payload.DeploymentID) {
			deployment_id = raw_payload.DeploymentID;
		}
		const payload = decode_payload(raw_payload, current_url());

		user_on_route_update = options.onRouteUpdate;
		user_on_work_update = options.onWorkUpdate;
		user_on_build_id_change = options.onClientBuildIDChange;
		default_error_boundary = options.defaultErrorBoundary;
		use_view_transitions = options.useViewTransitions ?? false;

		const history_state = window.history.state;
		const has_history_key =
			history_state &&
			typeof history_state === "object" &&
			HISTORY_KEY_FIELD in (history_state as Record<string, unknown>);
		if (!has_history_key) {
			const base =
				history_state && typeof history_state === "object"
					? history_state
					: {};
			window.history.replaceState(
				{
					...(base as object),
					[HISTORY_KEY_FIELD]: make_history_key(),
				},
				"",
				current_url().href,
			);
		}
		browser = read_browser_position();
		try {
			window.history.scrollRestoration = "manual";
		} catch {}

		const initial_ac = new AbortController();
		const modules = await prepare_modules(payload, initial_ac.signal);
		if (!modules) {
			return R.err("Initial navigation produced no state");
		}

		// Install a provisional snapshot so router APIs (getRouteState, submit)
		// work during initial client-loader execution.
		route_snapshot = {
			position: browser,
			route: build_route_record(payload, modules, []),
		};

		const cl_results = await run_client_loaders(
			payload.routes,
			payload,
			[],
			"init",
			browser.href,
			browser.state,
			initial_ac.signal,
		);
		if (initial_ac.signal.aborted) {
			return R.err("Initial navigation produced no state");
		}

		await wait_for_css(payload.css_bundles, initial_ac.signal);
		if (initial_ac.signal.aborted) {
			return R.err("Initial navigation produced no state");
		}

		const route = build_route_record(payload, modules, cl_results);
		route_snapshot = { position: browser, route };

		apply_head_and_title(
			payload.title,
			payload.meta_head_els,
			payload.rest_head_els,
		);
		apply_css_bundles(payload.css_bundles);
		preload_modules(payload.deps);

		let scroll_intent: ScrollIntent | undefined;
		const make_scroll_intent = (scroll: ScrollState): ScrollIntent => {
			return {
				scroll,
				target_route_id: make_route_id(
					route.matches.length - 1,
					route.matches[route.matches.length - 1]?.pattern ?? "",
				),
			};
		};
		let refresh_scroll_raw: string | null;
		try {
			refresh_scroll_raw = sessionStorage.getItem(
				SCROLL_STORAGE_RELOAD_KEY,
			);
		} catch {
			refresh_scroll_raw = null;
		}
		if (refresh_scroll_raw) {
			try {
				sessionStorage.removeItem(SCROLL_STORAGE_RELOAD_KEY);
			} catch {}
			try {
				const s = JSON.parse(refresh_scroll_raw);
				if (
					typeof s?.x === "number" &&
					typeof s?.y === "number" &&
					typeof s?.unix === "number" &&
					typeof s?.href === "string" &&
					Date.now() - s.unix <= REFRESH_MAX_AGE_MS &&
					matches_without_hash(new URL(s.href), current_url())
				) {
					scroll_intent = make_scroll_intent({ x: s.x, y: s.y });
				}
			} catch {}
		}
		if (!scroll_intent) {
			const hash = normalize_hash(current_url().hash);
			if (hash.length > 0) {
				scroll_intent = make_scroll_intent({
					hash: current_url().hash,
				});
			}
		}
		commit_route_snapshot("initial", null, route_snapshot, scroll_intent);

		if (options.progressIndicator) {
			setup_progress_indicator(options.progressIndicator);
		}
		if (focus_revalidation_cleanup) {
			focus_revalidation_cleanup();
			focus_revalidation_cleanup = null;
		}
		if (options.revalidateOnWindowFocus) {
			const stale_ms =
				typeof options.revalidateOnWindowFocus === "object"
					? options.revalidateOnWindowFocus.staleTimeMS
					: 5_000;
			focus_revalidation_cleanup = addOnWindowFocusListener(() => {
				const work = derive_work_state();
				if (
					work.navigation ||
					work.revalidation ||
					work.submissions.length > 0
				) {
					return;
				}
				if (Date.now() - last_activity_ts >= stale_ms) {
					void revalidate();
				}
			});
		}
		if (options.render) {
			await options.render();
		}

		window.addEventListener("popstate", () => {
			void handle_popstate();
		});
		window.addEventListener("beforeunload", () => {
			try {
				sessionStorage.setItem(
					SCROLL_STORAGE_RELOAD_KEY,
					JSON.stringify({
						...get_scroll_pos(),
						unix: Date.now(),
						href: current_url().href,
					}),
				);
			} catch {}
		});
		register_hmr();

		phase = "ready";
		if (deferred_submit_redirect) {
			const redirect = deferred_submit_redirect;
			deferred_submit_redirect = null;
			void start_nav_inner(redirect, { replace: true }, 0, {
				source: "redirect",
			});
		}
		maybe_revalidate();
		return R.ok(undefined);
	}

	/////// Public API

	async function navigate(
		href: string | URL,
		options?: {
			replace?: boolean;
			scrollToTop?: boolean;
			state?: unknown;
			skipProgressIndicator?: boolean;
		},
	): Promise<NavResult> {
		if (phase !== "ready") {
			throw new Error("Vorma not initialized");
		}
		const url = new URL(String(href), window.location.href);
		if (!is_same_origin(url)) {
			hard_redirect(url.href);
			return { didNavigate: false };
		}
		return start_nav_inner(
			url,
			{
				replace: options?.replace,
				scroll_to_top: options?.scrollToTop,
				state: options?.state,
				skip_progress_indicator: options?.skipProgressIndicator,
			},
			0,
		);
	}

	async function revalidate(): Promise<RevalidationResult> {
		if (phase !== "ready") {
			throw new Error("Vorma not initialized");
		}
		const waiter = make_deferred<RevalidationResult>();
		require_refresh(waiter, true);
		notify_work_update();
		return waiter.promise;
	}

	function getRouteState(): RouteState {
		const snapshot = route_snapshot;
		if (!snapshot) {
			throw new Error("Vorma not initialized");
		}
		return route_snapshot_to_state(snapshot);
	}

	function getWorkState(): WorkState {
		if (!route_snapshot) {
			throw new Error("Vorma not initialized");
		}
		return derive_work_state();
	}

	function getRootEl(): HTMLElement {
		const el = document.getElementById(VORMA_ROOT_EL_ID);
		if (el) {
			return el;
		}
		const fresh = document.createElement("div");
		fresh.id = VORMA_ROOT_EL_ID;
		document.body.insertBefore(fresh, document.body.firstChild);
		return fresh;
	}

	function defineRoute<T = any>(input: {
		pattern: string;
		component: (props: any) => any;
		errorBoundary?: (props: { error: unknown }) => any;
		clientLoader?: (props: any) => Promise<T>;
		beforeRouteCommit?: BeforeRouteCommitFn;
		beforeRouteYield?: BeforeRouteYieldFn;
		runClientLoaderOnHMR?: boolean;
	}): RouteDefinition & { __phantom_client_loader_data?: T } {
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
			before_route_commit: input.beforeRouteCommit,
			before_route_yield: input.beforeRouteYield,
		};
	}

	(window as any)[Symbol.for("vorma-data-revalidate-fn")] = revalidate;

	return R.ok({
		init,
		navigate,
		revalidate,
		submit_inner,
		getRouteState,
		getWorkState,
		getClientBuildID: () => client_build_id,
		getRootEl,
		defineRoute,
		start_prefetch,
		stop_prefetch,
		save_current_scroll,
		get_default_error_boundary: () => default_error_boundary,
	});
}

declare global {
	interface Window {
		__vorma_hmr_route_update?: (
			raw_url: string,
			mod: Record<string, unknown>,
		) => Promise<void>;
	}
}
