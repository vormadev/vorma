/// <reference types="vite/client" />

import { jsonDeepEquals, parseSearchParams } from "vorma/kit/json";
import { R, type Result } from "vorma/kit/result";
import {
	BUILD_ID_HEADER,
	CONTENT_TYPE_HEADER,
	DATA_SCRIPT_ID,
	HISTORY_KEY_FIELD,
	HISTORY_USER_STATE_FIELD,
	JSON_CONTENT_TYPE,
	SCROLL_STORAGE_KEY,
	SCROLL_STORAGE_RELOAD_KEY,
	VERCEL_X_DEPLOYMENT_ID,
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
	RevalidationResult,
	RouteState,
	RouteUpdateReason,
} from "../core/types.ts";
import type { Core6APISubmitBuildSkewEvent } from "./api_submit.ts";
import {
	create_core6_client_composition,
	type Core6ClientComposition,
	type Core6ClientCompositionHost,
} from "./composition.ts";
import { core6_route_boot_result_kind } from "./route_boot.ts";
import type { Core6RouteNavigationBuildSkewEvent } from "./route_navigation.ts";
import { core6_route_navigation_owner_result_kind } from "./route_navigation_owner.ts";
import {
	core6_abort_error_name,
	core6_route_payload_field,
	type Core6ClientLoaderFn,
	type Core6RawRoutePayload,
	type Core6RouteModule,
} from "./route_preparation.ts";
import type {
	Core6RouteCommit,
	Core6RoutePublication,
	Core6RoutePublicationSideEffects,
	Core6RouteRenderState,
	Core6RouteScroll,
} from "./route_publication.ts";
import { core6_route_state_from_prepared } from "./route_publication.ts";
import {
	core6_route_revalidation_failure_reason,
	core6_route_revalidation_reason,
	type Core6RouteRevalidationResult,
} from "./route_revalidation.ts";
import { core6_route_transaction_kind } from "./transaction.ts";

const core6_data_revalidate_symbol_key = "vorma-data-revalidate-fn";
const core6_not_booted_error = "Vorma not booted";
const core6_missing_data_script_error_prefix = "Missing element:";
const core6_parse_data_script_error_prefix = "Failed to parse";
const core6_revalidation_build_skew_reason = "build_skew";
const core6_revalidation_exhausted_reason = "max_retries_exhausted";
const core6_focus_revalidation_delay_ms = 30;
const core6_work_navigation_source = {
	navigate: "navigate",
	popstate: "popstate",
} as const;
const core6_work_revalidation_status = {
	debouncing: "debouncing",
	retrying: "retrying",
	running: "running",
} as const;
const core6_default_api_route_kind: APIRouteKind = "mutation";
let core6_focus_revalidation_cleanup: (() => void) | null = null;

type Core6TestOptions = {
	hard_redirect?: (url: string) => void;
	reload?: () => void;
	scroll_to?: (x: number, y: number) => void;
};

type Core6HistoryPosition = {
	href: string;
	key: string;
	state: unknown;
};

type Core6StoredScrollEntry = [string, { x: number; y: number }];

type Core6ScrollState = { x: number; y: number } | { hash: string };

type Core6ScrollIntent = {
	scroll: Core6ScrollState;
	target_route_id: string;
};

type Core6WorkState = {
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
	apiRequests: Array<{
		key: string;
		method: string;
		href: string;
	}>;
};

type Core6WorkIndicator = {
	track: <T>(promise: PromiseLike<T>) => Promise<T>;
	isActive: () => boolean;
};

type Core6WorkIndicatorOptions = {
	start: () => void;
	stop: () => void;
	startDelayMS?: number;
	stopDelayMS?: number;
	skipNavigations?: boolean;
	skipAPIRequests?: boolean;
	skipRevalidations?: boolean;
};

type Core6WorkIndicatorController = {
	indicator: Core6WorkIndicator;
	configure: (options: Core6WorkIndicatorOptions | undefined) => void;
	set_vorma_active: (active: boolean) => void;
};

type Core6WorkProjection =
	| {
			kind: "navigation";
			skip_work_indicator?: boolean;
	  }
	| {
			kind: "revalidation";
			skip_work_indicator?: boolean;
	  }
	| {
			kind: "apiRequest";
			skip_work_indicator?: boolean;
	  }
	| {
			kind: "prefetch";
	  };

type Core6BuildSkewDetectedEvent = {
	activeClientBuildID: string;
	serverBuildID: string;
	triggeringResponse:
		| {
				kind: "route";
				trigger: "navigation" | "popstate" | "prefetch";
				requestedHref: string;
				status: number;
				ok: boolean;
		  }
		| {
				kind: "route";
				trigger: "revalidation";
				revalidationReason:
					| "manual"
					| "retry"
					| "apiRequest"
					| "windowFocus";
				requestedHref: string;
				status: number;
				ok: boolean;
		  }
		| {
				kind: "apiRoute";
				apiRouteKind: APIRouteKind;
				requestedHref: string;
				method: string;
				status: number;
				ok: boolean;
		  };
	currentRouteState: RouteState;
	currentWorkState: Core6WorkState;
};

type Core6ClientOptions = {
	render?: () => void | Promise<void>;
	workIndicator?: Core6WorkIndicatorOptions;
	revalidateOnWindowFocus?:
		| boolean
		| { staleTimeMS: number; skipWorkIndicator?: boolean };
	defaultErrorBoundary?: (props: { error: unknown }) => any;
	useViewTransitions?: boolean;
	onRouteUpdate?: (
		route: RouteState,
		previousRoute: RouteState | null,
		reason: RouteUpdateReason,
	) => void;
	onWorkUpdate?: (work: Core6WorkState) => void;
	onBuildSkewDetected?: (event: Core6BuildSkewDetectedEvent) => void;
};

type Core6ClientCommit = {
	route_render?: {
		state: Core6RouteRenderState;
		scroll_intent?: Core6ScrollIntent;
	};
	route_update?: {
		previous_route: RouteState | null;
		reason: RouteUpdateReason;
		route: RouteState;
	};
	work?: Core6WorkState;
};

type Core6CommitFn = (commit: Core6ClientCommit) => void;

type Core6ViewDefinition = {
	pattern: string;
	component: (props: any) => any;
	error_boundary?: (props: { error: unknown }) => any;
	client_loader?: Core6ClientLoaderFn;
	before_route_commit?: BeforeRouteCommitFn;
	before_route_yield?: BeforeRouteYieldFn;
};

type Core6APIResult<T> =
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

type Core6ClientCore = {
	boot: (options: Core6ClientOptions) => Promise<Result<void>>;
	workIndicator: Core6WorkIndicator;
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
	) => Promise<Core6APIResult<T>>;
	getRouteState: () => RouteState;
	getWorkState: () => Core6WorkState;
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
	}) => Core6ViewDefinition & { __phantom_client_loader_data?: T };
	start_prefetch: (href: string) => void;
	stop_prefetch: (href: string) => void;
	save_current_scroll: () => void;
	get_default_error_boundary: () =>
		| ((props: { error: unknown }) => any)
		| undefined;
};

export function create_client_core6(
	_: Omit<AppConfig, "__vormaViews" | "__vormaAPIRoutes">,
	commit: Core6CommitFn,
	test_options: Core6TestOptions = {},
): Result<Core6ClientCore> {
	let composition: Core6ClientComposition | null = null;
	let browser_position: Core6HistoryPosition = {
		href: "",
		key: "",
		state: undefined,
	};
	let provisional_route: RouteState | null = null;
	let client_build_id = "";
	let deployment_id = "";
	let default_error_boundary:
		| ((props: { error: unknown }) => any)
		| undefined;
	let user_on_route_update:
		| ((
				route: RouteState,
				previous_route: RouteState | null,
				reason: RouteUpdateReason,
		  ) => void)
		| undefined;
	let user_on_work_update: ((work: Core6WorkState) => void) | undefined;
	let user_on_build_skew_detected:
		| ((event: Core6BuildSkewDetectedEvent) => void)
		| undefined;
	let last_activity_ts = Date.now();
	let last_work_state: Core6WorkState = empty_work_state();
	let work_indicator_options: Core6WorkIndicatorOptions | undefined;
	const work_indicator = create_core6_work_indicator();
	const module_cache = new Map<string, Core6RouteModule>();

	const reload =
		test_options.reload ??
		(() => {
			window.location.reload();
		});
	const hard_redirect =
		test_options.hard_redirect ??
		((href: string) => {
			window.location.assign(href);
		});

	const host: Core6ClientCompositionHost = {
		clear_timer: (timer) => {
			window.clearTimeout(timer as ReturnType<typeof window.setTimeout>);
		},
		current_href: () => {
			return current_url().href;
		},
		commit_publication: (publication) => {
			publish_client_route(publication);
		},
		fetch_api_response: async ({ href, init }) => {
			return await fetch(href, init);
		},
		fetch_route_payload: async ({ intent, signal }) => {
			const response = await fetch(intent.href, {
				headers: route_fetch_headers(),
				signal,
			});
			return await response.json();
		},
		fetch_route_response: async ({ href, init }) => {
			return await fetch(href, init);
		},
		hard_redirect,
		import_module: async (url, signal) => {
			return await import_core6_module(url, signal);
		},
		notify_api_build_skew: (event) => {
			notify_api_build_skew(event);
		},
		notify_build_skew: (event) => {
			notify_route_build_skew(event);
		},
		parse_input: ({ schema, search_params }) => {
			return parseSearchParams(schema, search_params);
		},
		preload_css: (bundles) => {
			preload_css(bundles);
		},
		set_provisional_route: (prepared) => {
			provisional_route = core6_route_state_from_prepared(
				prepared,
			) as RouteState;
		},
		reload: (href) => {
			if (current_url().href !== href) {
				window.history.replaceState(window.history.state, "", href);
			}
			reload();
		},
		route_state_equal: (previous_route, next_route) => {
			return jsonDeepEquals(previous_route, next_route);
		},
		save_scroll_for_key: (key) => {
			save_scroll_for_key(key, current_scroll_position());
		},
		set_timer: (fn, ms) => {
			return window.setTimeout(fn, ms);
		},
		wait_for_css: async (bundles, signal) => {
			await wait_for_css(bundles, signal);
		},
		warn_redirect_loop: () => {},
	};

	async function boot(options: Core6ClientOptions): Promise<Result<void>> {
		user_on_route_update = options.onRouteUpdate;
		user_on_work_update = options.onWorkUpdate;
		user_on_build_skew_detected = options.onBuildSkewDetected;
		default_error_boundary = options.defaultErrorBoundary;

		const payload_result = read_boot_payload();
		if (!payload_result.ok) {
			return R.err(payload_result.err);
		}
		const raw_payload = payload_result.val;
		client_build_id = String(
			raw_payload[core6_route_payload_field.client_build_id] ?? "",
		);
		deployment_id = String(
			(raw_payload as Core6RawRoutePayload & { DeploymentID?: string })
				.DeploymentID ?? "",
		);

		ensure_history_position();
		browser_position = read_browser_position();
		try {
			window.history.scrollRestoration = "manual";
		} catch {}

		composition = create_core6_client_composition({
			active_client_build_id: () => {
				return client_build_id;
			},
			deployment_id: () => {
				return deployment_id;
			},
			host,
			initial_position: browser_position,
		});

		const css_bundles = payload_array(raw_payload, "css_bundles");
		preload_css(css_bundles);
		const result = await composition.boot.boot({
			history_state: browser_position.state,
			href: browser_position.href,
			payload: raw_payload,
			scroll: boot_scroll(),
		});
		if (
			result.kind !== core6_route_boot_result_kind.booted &&
			result.kind !== core6_route_boot_result_kind.already_booted
		) {
			return R.err(`Initial navigation failed: ${result.kind}`);
		}
		composition.revalidation.start_pending();
		notify_work_update();

		install_browser_listeners();
		await install_dev_hmr();
		setup_work_indicator(options.workIndicator);
		setup_focus_revalidation(options.revalidateOnWindowFocus);

		if (options.render) {
			await options.render();
		}
		(window as unknown as Record<symbol, unknown>)[
			Symbol.for(core6_data_revalidate_symbol_key)
		] = revalidate;
		return R.ok(undefined);
	}

	async function navigate(
		href: string | URL,
		options?: {
			replace?: boolean;
			scrollToTop?: boolean;
			state?: unknown;
			skipWorkIndicator?: boolean;
		},
	): Promise<{ didNavigate: boolean }> {
		const current = require_composition();
		const navigation = current.navigation.navigate({
			href: String(href),
			replace: options?.replace,
			scroll_to_top: options?.scrollToTop,
			skip_work_indicator: options?.skipWorkIndicator,
			state: options?.state,
		});
		notify_work_update();
		try {
			const result = await navigation;
			if (
				result.kind ===
				core6_route_navigation_owner_result_kind.same_document
			) {
				apply_route_scroll(result.scroll);
			}
			return { didNavigate: result.did_navigate };
		} finally {
			current.revalidation.start_pending();
			notify_work_update();
		}
	}

	async function revalidate(): Promise<RevalidationResult> {
		const current = require_composition();
		const revalidation = current.revalidation.request({
			reason: core6_route_revalidation_reason.manual,
		});
		notify_work_update();
		try {
			const result = await revalidation;
			return public_revalidation_result(result);
		} finally {
			notify_work_update();
		}
	}

	async function submit_inner<T = unknown>(
		url: string | URL,
		requestInit?: RequestInit,
		options?: {
			apiRouteKind?: APIRouteKind;
			dedupeKey?: string;
			revalidate?: boolean;
			skipWorkIndicator?: boolean;
		},
	) {
		const current = require_composition();
		const submission = current.api_submit.submit<T>({
			options: {
				api_route_kind:
					options?.apiRouteKind ?? core6_default_api_route_kind,
				dedupe_key: options?.dedupeKey,
				revalidate: options?.revalidate,
				skip_work_indicator: options?.skipWorkIndicator,
			},
			request_init: requestInit,
			url,
		});
		notify_work_update();
		const result = await submission;
		notify_work_update();
		void result.revalidation_promise.finally(() => {
			notify_work_update();
		});
		if (result.success) {
			return {
				data: result.data,
				response: result.response,
				revalidationPromise: result.revalidation_promise.then(
					public_revalidation_result,
				),
				success: true as const,
			};
		}
		return {
			error: result.error,
			response: result.response,
			revalidationPromise: result.revalidation_promise.then(
				public_revalidation_result,
			),
			success: false as const,
		};
	}

	function getRouteState(): RouteState {
		const route = require_composition().runtime.current_route();
		if (!route && provisional_route) {
			return provisional_route;
		}
		if (!route) {
			throw new Error(core6_not_booted_error);
		}
		return route;
	}

	function getWorkState() {
		return derive_work_state();
	}

	function getRootEl(): HTMLElement {
		const existing = document.getElementById(VORMA_ROOT_EL_ID);
		if (existing) {
			return existing;
		}
		const fresh = document.createElement("div");
		fresh.id = VORMA_ROOT_EL_ID;
		document.body.insertBefore(fresh, document.body.firstChild);
		return fresh;
	}

	function defineView<T = any>(input: {
		pattern: string;
		component: (props: any) => any;
		errorBoundary?: (props: { error: unknown }) => any;
		clientLoader?: (props: any) => Promise<T>;
		beforeRouteCommit?: BeforeRouteCommitFn;
		beforeRouteYield?: BeforeRouteYieldFn;
		runClientLoaderOnHMR?: boolean;
	}): Core6ViewDefinition & { __phantom_client_loader_data?: T } {
		if (import.meta.env.DEV) {
			void import("./dev.ts").then(({ configure_core6_dev_hmr_view }) => {
				configure_core6_dev_hmr_view(
					input.pattern,
					input.runClientLoaderOnHMR === true,
				);
			});
		}
		return {
			before_route_commit: input.beforeRouteCommit,
			before_route_yield: input.beforeRouteYield,
			client_loader: input.clientLoader,
			component: input.component,
			error_boundary: input.errorBoundary,
			pattern: input.pattern,
		} as Core6ViewDefinition & { __phantom_client_loader_data?: T };
	}

	function start_prefetch(href: string): void {
		const current = composition;
		if (!current) {
			return;
		}
		void current.prefetch.start({ href });
		notify_work_update();
	}

	function stop_prefetch(href: string): void {
		const current = composition;
		if (!current) {
			return;
		}
		current.prefetch.cancel(href);
		notify_work_update();
	}

	function save_current_scroll(): void {
		if (browser_position.key.length === 0) {
			return;
		}
		save_scroll_for_key(browser_position.key, current_scroll_position());
	}

	function require_composition(): Core6ClientComposition {
		if (!composition) {
			throw new Error(core6_not_booted_error);
		}
		return composition;
	}

	function current_url(): URL {
		return new URL(window.location.href);
	}

	function read_browser_position(): Core6HistoryPosition {
		const state = window.history.state;
		const key =
			state && typeof state === "object" && HISTORY_KEY_FIELD in state
				? String((state as Record<string, unknown>)[HISTORY_KEY_FIELD])
				: "";
		return {
			href: current_url().href,
			key,
			state:
				state &&
				typeof state === "object" &&
				HISTORY_USER_STATE_FIELD in state
					? (state as Record<string, unknown>)[
							HISTORY_USER_STATE_FIELD
						]
					: undefined,
		};
	}

	function ensure_history_position(): void {
		const state = window.history.state;
		if (state && typeof state === "object" && HISTORY_KEY_FIELD in state) {
			return;
		}
		const base = state && typeof state === "object" ? state : {};
		window.history.replaceState(
			{
				...(base as object),
				[HISTORY_KEY_FIELD]: make_history_key(),
			},
			"",
			current_url().href,
		);
	}

	function commit_history(input: {
		href: string;
		replace: boolean;
		state: unknown;
	}): Core6HistoryPosition {
		const key = make_history_key();
		const state = {
			[HISTORY_KEY_FIELD]: key,
			[HISTORY_USER_STATE_FIELD]: input.state,
		};
		if (input.replace) {
			window.history.replaceState(state, "", input.href);
		} else {
			window.history.pushState(state, "", input.href);
		}
		return {
			href: input.href,
			key,
			state: input.state,
		};
	}

	function route_fetch_headers(): Headers {
		const headers = new Headers();
		headers.set(CONTENT_TYPE_HEADER, JSON_CONTENT_TYPE);
		if (client_build_id) {
			headers.set(BUILD_ID_HEADER, client_build_id);
		}
		if (deployment_id) {
			headers.set(VERCEL_X_DEPLOYMENT_ID, deployment_id);
		}
		return headers;
	}

	async function import_core6_module(
		url: string,
		signal: AbortSignal,
	): Promise<Core6RouteModule> {
		if (signal.aborted) {
			throw new DOMException("Aborted", core6_abort_error_name);
		}
		const cache_key = normalize_module_url(url);
		if (import.meta.env.DEV) {
			const cached = module_cache.get(cache_key);
			if (cached) {
				return cached;
			}
		}
		const module = (await import(
			/* @vite-ignore */ url
		)) as Core6RouteModule;
		if (import.meta.env.DEV) {
			module_cache.set(cache_key, module);
		}
		if (signal.aborted) {
			throw new DOMException("Aborted", core6_abort_error_name);
		}
		return module;
	}

	function normalize_module_url(url: string): string {
		return new URL(url, current_url()).pathname;
	}

	function read_boot_payload(): Result<Core6RawRoutePayload> {
		const payload_el = document.getElementById(DATA_SCRIPT_ID);
		if (!payload_el) {
			return R.err(
				`${core6_missing_data_script_error_prefix} #${DATA_SCRIPT_ID}`,
			);
		}
		try {
			return R.ok(
				JSON.parse(
					payload_el.textContent ?? "{}",
				) as Core6RawRoutePayload,
			);
		} catch (error) {
			return R.err(
				`${core6_parse_data_script_error_prefix} #${DATA_SCRIPT_ID}: ${error_string(error)}`,
			);
		}
	}

	function publish_client_route(publication: Core6RoutePublication): void {
		provisional_route = null;
		if (publication.history) {
			save_current_scroll();
			browser_position = commit_history(publication.history);
		}
		apply_route_side_effects(publication.side_effects);
		emit_client_commit(core6_client_commit(publication.commit));
		if (publication.side_effects) {
			last_activity_ts = Date.now();
		}
	}

	function apply_route_side_effects(
		side_effects: Core6RoutePublicationSideEffects | null,
	): void {
		if (!side_effects) {
			return;
		}
		apply_head_and_title(
			decode_title(side_effects.title_html),
			side_effects.meta_head_els as HeadEl[],
			side_effects.rest_head_els as HeadEl[],
		);
		apply_css_bundles(side_effects.css_bundles);
		preload_modules(side_effects.deps);
	}

	function decode_title(html: string | null): string | undefined {
		if (html === null) {
			return undefined;
		}
		const el = document.createElement("textarea");
		el.innerHTML = html;
		return el.value;
	}

	function payload_array(
		raw_payload: Core6RawRoutePayload,
		field: "css_bundles" | "deps",
	): string[] {
		const value =
			field === "css_bundles"
				? raw_payload[core6_route_payload_field.css_bundles]
				: raw_payload[core6_route_payload_field.deps];
		return Array.isArray(value) ? value : [];
	}

	function boot_scroll(): Core6RouteScroll | undefined {
		const restored = read_reload_scroll();
		if (restored) {
			return restored;
		}
		const hash = current_url().hash;
		if (hash.length === 0) {
			return undefined;
		}
		return { hash };
	}

	function read_reload_scroll(): Core6ScrollState | undefined {
		let raw: string | null = null;
		try {
			raw = sessionStorage.getItem(SCROLL_STORAGE_RELOAD_KEY);
			sessionStorage.removeItem(SCROLL_STORAGE_RELOAD_KEY);
		} catch {
			raw = null;
		}
		if (!raw) {
			return undefined;
		}
		try {
			const parsed = JSON.parse(raw);
			if (
				typeof parsed?.x === "number" &&
				typeof parsed?.y === "number" &&
				typeof parsed?.href === "string"
			) {
				const parsed_url = new URL(parsed.href);
				const current = current_url();
				parsed_url.hash = "";
				current.hash = "";
				if (parsed_url.href === current.href) {
					return { x: parsed.x, y: parsed.y };
				}
			}
		} catch {}
		return undefined;
	}

	function core6_client_commit(
		core6_commit: Core6RouteCommit,
	): Core6ClientCommit {
		const client_commit: Core6ClientCommit = {};
		client_commit.route_render = {
			state: core6_commit.route_render.state,
			scroll_intent: scroll_intent_from_core6(
				core6_commit.route_render.state,
				core6_commit.route_render.scroll,
			),
		};
		if (core6_commit.route_update) {
			client_commit.route_update = {
				previous_route: core6_commit.route_update.previous_route,
				reason: core6_commit.route_update.reason,
				route: core6_commit.route_update.route,
			};
		}
		return client_commit;
	}

	function scroll_intent_from_core6(
		state: Core6RouteRenderState,
		scroll: Core6RouteScroll | undefined,
	): Core6ScrollIntent | undefined {
		if (!scroll) {
			return undefined;
		}
		const idx = state.entries.length - 1;
		const pattern = state.entries[idx]?.pattern ?? "";
		return {
			scroll,
			target_route_id: `${idx}:${pattern}`,
		};
	}

	function emit_client_commit(client_commit: Core6ClientCommit): void {
		commit(client_commit);
		if (client_commit.route_update) {
			user_on_route_update?.(
				client_commit.route_update.route,
				client_commit.route_update.previous_route,
				client_commit.route_update.reason,
			);
		}
		if (client_commit.work) {
			user_on_work_update?.(client_commit.work);
		}
	}

	function empty_work_state(): Core6WorkState {
		return {
			apiRequests: [],
			navigation: null,
			prefetch: null,
			revalidation: null,
		};
	}

	function take_work_update(): Core6WorkState | undefined {
		const next = derive_work_state();
		if (jsonDeepEquals(last_work_state, next)) {
			return undefined;
		}
		last_work_state = next;
		return next;
	}

	function notify_work_update(): void {
		const work = take_work_update();
		if (!work) {
			sync_work_indicator();
			return;
		}
		sync_work_indicator();
		emit_client_commit({ work });
	}

	function setup_work_indicator(
		options: Core6WorkIndicatorOptions | undefined,
	): void {
		work_indicator_options = options;
		work_indicator.configure(options);
		sync_work_indicator();
	}

	function setup_focus_revalidation(
		options: Core6ClientOptions["revalidateOnWindowFocus"],
	): void {
		if (core6_focus_revalidation_cleanup) {
			core6_focus_revalidation_cleanup();
			core6_focus_revalidation_cleanup = null;
		}
		if (!options) {
			return;
		}
		const focus_options = typeof options === "object" ? options : null;
		const stale_ms = focus_options?.staleTimeMS ?? 5_000;
		const skip_work_indicator = focus_options?.skipWorkIndicator === true;
		let timer: ReturnType<typeof window.setTimeout> | null = null;

		const revalidate_after_focus = () => {
			if (timer !== null) {
				window.clearTimeout(timer);
			}
			timer = window.setTimeout(() => {
				timer = null;
				const work = derive_work_state();
				if (
					work.navigation ||
					work.revalidation ||
					work.apiRequests.length > 0
				) {
					return;
				}
				if (Date.now() - last_activity_ts < stale_ms) {
					return;
				}
				const current = composition;
				if (!current) {
					return;
				}
				const revalidation = current.revalidation.request({
					reason: core6_route_revalidation_reason.window_focus,
					skip_work_indicator,
				});
				notify_work_update();
				void revalidation.finally(() => {
					notify_work_update();
				});
			}, core6_focus_revalidation_delay_ms);
		};
		const revalidate_after_visibility_change = () => {
			if (document.visibilityState === "visible") {
				revalidate_after_focus();
			}
		};
		window.addEventListener("focus", revalidate_after_focus);
		window.addEventListener(
			"visibilitychange",
			revalidate_after_visibility_change,
		);
		core6_focus_revalidation_cleanup = () => {
			if (timer !== null) {
				window.clearTimeout(timer);
				timer = null;
			}
			window.removeEventListener("focus", revalidate_after_focus);
			window.removeEventListener(
				"visibilitychange",
				revalidate_after_visibility_change,
			);
		};
	}

	function sync_work_indicator(): void {
		const options = work_indicator_options;
		if (!options) {
			work_indicator.set_vorma_active(false);
			return;
		}
		let active_for_vorma = false;
		for (const work of derive_work_projection()) {
			if (
				work.kind === "navigation" &&
				options.skipNavigations !== true &&
				!work.skip_work_indicator
			) {
				active_for_vorma = true;
				break;
			}
			if (
				work.kind === "revalidation" &&
				options.skipRevalidations !== true &&
				!work.skip_work_indicator
			) {
				active_for_vorma = true;
				break;
			}
			if (
				work.kind === "apiRequest" &&
				options.skipAPIRequests !== true &&
				!work.skip_work_indicator
			) {
				active_for_vorma = true;
				break;
			}
		}
		work_indicator.set_vorma_active(active_for_vorma);
	}

	function derive_work_projection(): Core6WorkProjection[] {
		const current = composition;
		const work: Core6WorkProjection[] = [];
		const navigation_status = current?.navigation.current_status() ?? null;
		if (navigation_status) {
			work.push({
				kind: "navigation",
				skip_work_indicator: navigation_status.skip_work_indicator,
			});
		}
		if (current?.popstate.current_status()) {
			work.push({ kind: "navigation" });
		}
		const revalidation_status =
			current?.revalidation.current_status() ?? null;
		if (revalidation_status) {
			work.push({
				kind: "revalidation",
				skip_work_indicator: revalidation_status.skip_work_indicator,
			});
		}
		for (const status of current?.api_submit.current_statuses() ?? []) {
			work.push({
				kind: "apiRequest",
				skip_work_indicator: status.skip_work_indicator,
			});
		}
		if (current?.prefetch.current_status()) {
			work.push({ kind: "prefetch" });
		}
		return work;
	}

	function derive_work_state(): Core6WorkState {
		const current = composition;
		const navigation_status = current?.navigation.current_status() ?? null;
		const popstate_status = current?.popstate.current_status() ?? null;
		const revalidation_status =
			current?.revalidation.current_status() ?? null;
		const prefetch_status = current?.prefetch.current_status() ?? null;
		const api_statuses = current?.api_submit.current_statuses() ?? [];
		return {
			apiRequests: api_statuses.map((status) => {
				return {
					href: status.href,
					key: status.dedupe_key ?? `${status.method}:${status.href}`,
					method: status.method,
				};
			}),
			navigation: navigation_status
				? {
						href: navigation_status.href,
						replace: navigation_status.replace,
						source: core6_work_navigation_source.navigate,
					}
				: popstate_status
					? {
							href: popstate_status.href,
							replace: true,
							source: core6_work_navigation_source.popstate,
						}
					: null,
			prefetch: prefetch_status
				? {
						href: prefetch_status.href,
					}
				: null,
			revalidation: revalidation_status
				? {
						attempt: revalidation_status.attempt,
						status:
							revalidation_status.status ===
							core6_work_revalidation_status.debouncing
								? core6_work_revalidation_status.debouncing
								: revalidation_status.status ===
									  core6_work_revalidation_status.retrying
									? core6_work_revalidation_status.retrying
									: core6_work_revalidation_status.running,
					}
				: null,
		};
	}

	function public_revalidation_result(
		result: Core6RouteRevalidationResult,
	): RevalidationResult {
		if (result.ok) {
			return { ok: true };
		}
		if (
			result.reason === core6_route_revalidation_failure_reason.build_skew
		) {
			return { ok: false, reason: core6_revalidation_build_skew_reason };
		}
		return { ok: false, reason: core6_revalidation_exhausted_reason };
	}

	function notify_route_build_skew(
		event: Core6RouteNavigationBuildSkewEvent,
	): void {
		const current_route = composition?.runtime.current_route();
		if (!current_route) {
			return;
		}
		user_on_build_skew_detected?.({
			activeClientBuildID: event.active_client_build_id,
			currentRouteState: current_route,
			currentWorkState: derive_work_state(),
			serverBuildID: event.response.server_build_id,
			triggeringResponse: route_build_skew_response(event),
		});
	}

	function route_build_skew_response(
		event: Core6RouteNavigationBuildSkewEvent,
	): Core6BuildSkewDetectedEvent["triggeringResponse"] {
		if (event.trigger === core6_route_transaction_kind.revalidation) {
			return {
				kind: "route",
				ok: event.response.ok,
				requestedHref: event.response.requested_href,
				revalidationReason:
					event.revalidation_reason ===
					core6_route_revalidation_reason.window_focus
						? "windowFocus"
						: event.revalidation_reason ===
							  core6_route_revalidation_reason.mutation
							? "apiRequest"
							: "manual",
				status: event.response.status,
				trigger: "revalidation",
			};
		}
		return {
			kind: "route",
			ok: event.response.ok,
			requestedHref: event.response.requested_href,
			status: event.response.status,
			trigger:
				event.trigger === core6_route_transaction_kind.boot
					? core6_route_transaction_kind.navigation
					: event.trigger,
		};
	}

	function notify_api_build_skew(event: Core6APISubmitBuildSkewEvent): void {
		const current_route = composition?.runtime.current_route();
		if (!current_route) {
			return;
		}
		user_on_build_skew_detected?.({
			activeClientBuildID: event.active_client_build_id,
			currentRouteState: current_route,
			currentWorkState: derive_work_state(),
			serverBuildID: event.server_build_id,
			triggeringResponse: {
				apiRouteKind: event.route_kind,
				kind: "apiRoute",
				method: event.method,
				ok: event.ok,
				requestedHref: event.requested_href,
				status: event.status,
			},
		});
	}

	function install_browser_listeners(): void {
		window.addEventListener("popstate", () => {
			const current = composition;
			if (!current) {
				return;
			}
			browser_position = read_browser_position();
			const popstate = current.popstate.handle({
				position: browser_position,
				restored_scroll: read_scroll_for_key(browser_position.key),
			});
			notify_work_update();
			void popstate.finally(() => {
				current.revalidation.start_pending();
				notify_work_update();
			});
		});
		window.addEventListener("beforeunload", () => {
			try {
				sessionStorage.setItem(
					SCROLL_STORAGE_RELOAD_KEY,
					JSON.stringify({
						...current_scroll_position(),
						href: current_url().href,
					}),
				);
			} catch {}
		});
	}

	async function install_dev_hmr(): Promise<void> {
		if (import.meta.env.DEV) {
			const hmr_loader_error_message =
				"Vorma: HMR client loader re-run failed";
			const { install_core6_dev_hmr } = await import("./dev.ts");
			await install_core6_dev_hmr({
				host: {
					cache_module: (module_url, module) => {
						module_cache.set(module_url, module);
					},
					normalize_module_url,
					report_client_loader_error: (error) => {
						console.error(hmr_loader_error_message, error);
					},
					route_state_equal: (previous_route, next_route) => {
						return jsonDeepEquals(previous_route, next_route);
					},
				},
				runtime: require_composition().runtime,
			});
		}
	}

	function current_scroll_position(): { x: number; y: number } {
		return {
			x: window.scrollX,
			y: window.scrollY,
		};
	}

	function apply_route_scroll(scroll: Core6RouteScroll): void {
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
			test_options.scroll_to ??
			((x: number, y: number) => {
				window.scrollTo(x, y);
			});
		scroll_to(scroll.x, scroll.y);
	}

	function read_scroll_entries(): Core6StoredScrollEntry[] {
		try {
			const raw = sessionStorage.getItem(SCROLL_STORAGE_KEY);
			if (!raw) {
				return [];
			}
			const parsed = JSON.parse(raw);
			if (!Array.isArray(parsed)) {
				return [];
			}
			return parsed.filter((entry): entry is Core6StoredScrollEntry => {
				return (
					Array.isArray(entry) &&
					entry.length === 2 &&
					typeof entry[0] === "string" &&
					typeof entry[1]?.x === "number" &&
					typeof entry[1]?.y === "number"
				);
			});
		} catch {
			return [];
		}
	}

	function save_scroll_for_key(
		key: string,
		scroll: { x: number; y: number },
	): void {
		const entries = read_scroll_entries().filter((entry) => {
			return entry[0] !== key;
		});
		entries.push([key, scroll]);
		try {
			sessionStorage.setItem(SCROLL_STORAGE_KEY, JSON.stringify(entries));
		} catch {}
	}

	function read_scroll_for_key(key: string): Core6ScrollState | undefined {
		const entry = read_scroll_entries().find((candidate) => {
			return candidate[0] === key;
		});
		return entry?.[1];
	}

	function make_history_key(): string {
		return Math.random().toString(36).slice(2, 10);
	}

	function error_string(error: unknown): string {
		return error instanceof Error ? error.message : String(error);
	}

	const core: Core6ClientCore = {
		boot,
		defineView,
		getClientBuildID: () => {
			return client_build_id;
		},
		getRootEl,
		getRouteState,
		getWorkState,
		get_default_error_boundary: () => {
			return default_error_boundary;
		},
		navigate,
		revalidate,
		save_current_scroll,
		start_prefetch,
		stop_prefetch,
		submit_inner,
		workIndicator: work_indicator.indicator,
	};
	return R.ok(core);
}

function create_core6_work_indicator(): Core6WorkIndicatorController {
	let options: Core6WorkIndicatorOptions | undefined;
	let visible = false;
	let show_timer: number | undefined;
	let hide_timer: number | undefined;
	const active_tokens = new Set<symbol>();
	let release_vorma_work: (() => void) | undefined;

	function clear_show_timer(): void {
		if (show_timer === undefined) {
			return;
		}
		clearTimeout(show_timer);
		show_timer = undefined;
	}

	function clear_hide_timer(): void {
		if (hide_timer === undefined) {
			return;
		}
		clearTimeout(hide_timer);
		hide_timer = undefined;
	}

	function sync(): void {
		const current_options = options;
		if (!current_options) {
			clear_show_timer();
			clear_hide_timer();
			return;
		}
		if (active_tokens.size > 0) {
			clear_hide_timer();
			if (visible || show_timer !== undefined) {
				return;
			}
			show_timer = window.setTimeout(() => {
				show_timer = undefined;
				const latest_options = options;
				if (!latest_options || active_tokens.size === 0 || visible) {
					return;
				}
				latest_options.start();
				visible = true;
			}, current_options.startDelayMS ?? 12);
			return;
		}
		clear_show_timer();
		if (!visible || hide_timer !== undefined) {
			return;
		}
		hide_timer = window.setTimeout(() => {
			hide_timer = undefined;
			const latest_options = options;
			if (!latest_options || active_tokens.size > 0) {
				return;
			}
			latest_options.stop();
			visible = false;
		}, current_options.stopDelayMS ?? 12);
	}

	function begin(): () => void {
		const token = Symbol("core6-work-indicator");
		let released = false;
		active_tokens.add(token);
		sync();
		return () => {
			if (released) {
				return;
			}
			released = true;
			active_tokens.delete(token);
			sync();
		};
	}

	function configure(next_options: Core6WorkIndicatorOptions | undefined) {
		const previous_options = options;
		clear_show_timer();
		clear_hide_timer();
		if (visible && previous_options && previous_options !== next_options) {
			previous_options.stop();
			visible = false;
		}
		options = next_options;
		sync();
	}

	function set_vorma_active(active: boolean): void {
		if (active) {
			if (!release_vorma_work) {
				release_vorma_work = begin();
			}
			return;
		}
		if (!release_vorma_work) {
			sync();
			return;
		}
		release_vorma_work();
		release_vorma_work = undefined;
	}

	return {
		configure,
		indicator: {
			isActive: () => {
				return active_tokens.size > 0;
			},
			track: <T>(promise: PromiseLike<T>): Promise<T> => {
				const release = begin();
				return Promise.resolve(promise).finally(() => {
					release();
				});
			},
		},
		set_vorma_active,
	};
}
