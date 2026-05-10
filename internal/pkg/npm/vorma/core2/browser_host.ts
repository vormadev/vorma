/// <reference types="vite/client" />

import { parseSearchParams } from "vorma/kit/json";
import {
	createPatternRegistry,
	findNestedMatches,
	registerPattern,
	type FindNestedMatchesResult,
	type PatternRegistry,
} from "vorma/kit/matcher";
import {
	BUILD_ID_HEADER,
	DATA_SCRIPT_ID,
	VERCEL_DPL_QUERY_PARAM_KEY,
	VERCEL_X_DEPLOYMENT_ID,
	VORMA_JSON_KEY,
	X_ACCEPTS_CLIENT_REDIRECT,
	X_CLIENT_REDIRECT,
	X_VORMA_BUILD_SKEW,
} from "../core/constants.ts";
import { is_abort_cause, new_abort_error } from "./abort.ts";
import {
	apply_route_dom_patch,
	clear_browser_timer,
	decode_html_text,
	prepare_css_bundles,
	read_browser_position,
	read_current_scroll,
	read_reload_scroll,
	read_saved_scroll,
	save_current_scroll,
	save_reload_scroll,
	save_scroll_position,
	set_browser_timer,
	write_browser_history,
} from "./browser_dom.ts";
import type {
	BuildSkewDetectedEvent,
	ClientCommit,
	ClientLoaderFn,
	ScrollIntent,
	ViewDefinition,
} from "./client_contract.ts";
import type {
	APIFetchRequest,
	BrowserListenerHandlers,
	CoreHost,
	RouteFetchRequest,
	RouteHookRequest,
	RoutePrepareRequest,
} from "./host.ts";
import {
	api_result_kind,
	route_fetch_trigger,
	route_operation_kind,
	route_response_kind,
	type APIResultData,
	type PreparedRoute,
	type RouteResponseData,
} from "./model.ts";
import {
	build_prepared_route,
	client_loader_outcome_kind,
	decode_route_payload,
	type ClientLoaderOutcome,
	type DecodedRoute,
	type DecodedRoutePayload,
	type RouteModuleMap,
} from "./route_prepare.ts";

const boot_payload_field = {
	client_build_id: "ClientBuildID",
	deployment_id: "DeploymentID",
} as const;

const browser_event = {
	beforeunload: "beforeunload",
	focus: "focus",
	popstate: "popstate",
} as const;

const http_header = {
	content_type: "Content-Type",
} as const;

const header_value = {
	enabled: "1",
} as const;

const content_type = {
	json: "application/json",
	json_token: "json",
} as const;

const api_error_message = {
	redirect_non_http: "Redirect target must use an HTTP(S) scheme. Received:",
	submit_cross_origin: "submit only supports same-origin targets. Received:",
} as const;

const http_method = {
	get: "GET",
	head: "HEAD",
} as const;

const is_dev_build = import.meta.env.DEV;

type BrowserViewTransition = {
	finished?: Promise<unknown>;
	updateCallbackDone?: Promise<unknown>;
};

type BrowserDocumentWithViewTransition = Document & {
	startViewTransition?: (callback: () => unknown) => BrowserViewTransition;
};

type ClientLoaderRegistryEntry = {
	loader: ClientLoaderFn;
	search_schema: unknown;
};

type ClientLoaderServerState = {
	clientBuildID: string;
	loaderData: unknown;
	matches: Array<{
		input: unknown;
		loaderData: unknown;
		pattern: string;
	}>;
	outermostServerError: null | {
		error: unknown;
		idx: number;
	};
};

type ClientLoaderTrigger = Parameters<ClientLoaderFn>[0]["trigger"];

type PrestartedClientLoader = {
	abort: () => void;
	pattern: string;
	resolve_server_state: (state: ClientLoaderServerState) => void;
	result_promise: Promise<unknown>;
};

let active_browser_listeners_cleanup: (() => void) | null = null;

export type BrowserHostOptions = {
	commit: (commit: ClientCommit) => void;
	dev?: boolean;
	fetch_fn?: typeof fetch;
	hard_redirect?: (href: string) => void;
	load_module?: (href: string) => Promise<Record<string, unknown>>;
	now_ms?: () => number;
	render?: () => void | Promise<void>;
	report_build_skew?: (event: BuildSkewDetectedEvent) => void;
	report_unhandled_error?: (cause: unknown) => void;
	scroll_to?: (x: number, y: number) => void;
};

export function create_browser_host(options: BrowserHostOptions): CoreHost {
	const now_ms = options.now_ms ?? Date.now;
	const dev = is_dev_build && options.dev !== false;
	const module_cache = new Map<string, Record<string, unknown>>();
	const pattern_registry_result = createPatternRegistry();
	if (!pattern_registry_result.ok) {
		throw new Error(pattern_registry_result.err);
	}
	const pattern_registry = pattern_registry_result.val;
	const client_loader_registry = new Map<string, ClientLoaderRegistryEntry>();
	const prestarted_client_loaders = new Map<
		string,
		PrestartedClientLoader[]
	>();

	function current_fetch_fn(): typeof fetch {
		return options.fetch_fn ?? fetch;
	}

	async function load_module(href: string): Promise<Record<string, unknown>> {
		if (dev && module_cache.has(href)) {
			return module_cache.get(href)!;
		}
		const mod = options.load_module
			? await options.load_module(href)
			: ((await import(/* @vite-ignore */ href)) as Record<
					string,
					unknown
				>);
		if (dev) {
			module_cache.set(href, mod);
		}
		return mod;
	}

	function remember_route_modules(
		payload: DecodedRoutePayload,
		modules: RouteModuleMap,
	): void {
		for (const route of payload.routes) {
			const module = modules.get(route.module_url);
			const def = module?.default as ViewDefinition | undefined;
			if (def?.client_loader) {
				const registered = registerPattern(
					pattern_registry,
					route.pattern,
				);
				if (!registered.ok) {
					throw new Error(registered.err);
				}
				client_loader_registry.set(route.pattern, {
					loader: def.client_loader,
					search_schema: route.search_schema,
				});
			} else {
				client_loader_registry.delete(route.pattern);
			}
		}
	}

	return {
		apply_route_dom: (prepared) => {
			apply_route_dom_patch(prepared.dom);
		},
		clear_timer: clear_browser_timer,
		commit: (commit) => {
			options.commit(commit);
			apply_scroll_intent(
				commit.route_render?.scroll_intent,
				options.scroll_to,
			);
		},
		fetch_api: (request) => {
			return fetch_api(current_fetch_fn(), request);
		},
		fetch_route: (request) => {
			prestart_known_client_loaders({
				client_loader_registry,
				pattern_registry,
				prestarted_client_loaders,
				request,
			});
			return fetch_route(current_fetch_fn(), request).then(
				(response) => {
					if (response.kind !== route_response_kind.data) {
						abort_prestarted_client_loaders(
							prestarted_client_loaders,
							request.operation_id,
						);
					}
					return response;
				},
				(cause) => {
					abort_prestarted_client_loaders(
						prestarted_client_loaders,
						request.operation_id,
					);
					throw cause;
				},
			);
		},
		hard_redirect:
			options.hard_redirect ??
			((href) => {
				window.location.assign(href);
			}),
		install_browser_listeners: (handlers) => {
			return install_browser_listeners(
				handlers,
				module_cache,
				now_ms,
				dev,
			);
		},
		now_ms,
		prepare_hmr_route: async (request) => {
			if (!dev || !import.meta.env.DEV) {
				return {
					render: request.render,
					route: request.route,
				};
			}
			const hmr_dev = await import("./hmr_dev.ts");
			return hmr_dev.prepare_hmr_route_dev(request);
		},
		prepare_route: (request) => {
			return prepare_route(
				request,
				load_module,
				remember_route_modules,
				take_prestarted_client_loaders(
					prestarted_client_loaders,
					request.operation_id,
				),
			);
		},
		read_boot_payload: async (fallback_browser_key) => {
			return read_boot_payload(fallback_browser_key, now_ms());
		},
		render: options.render ?? (() => {}),
		report_unhandled_error:
			options.report_unhandled_error ??
			((cause) => {
				setTimeout(() => {
					throw cause;
				});
			}),
		report_build_skew: options.report_build_skew ?? (() => {}),
		run_route_hooks,
		run_view_transition,
		save_current_scroll,
		save_scroll_position,
		set_timer: (delay_ms, callback) => {
			return set_browser_timer(delay_ms, callback);
		},
		write_history: write_browser_history,
	};
}

function abort_prestarted_client_loaders(
	prestarted_client_loaders: Map<string, PrestartedClientLoader[]>,
	operation_id: string,
): void {
	const loaders = prestarted_client_loaders.get(operation_id) ?? [];
	prestarted_client_loaders.delete(operation_id);
	for (const loader of loaders) {
		loader.abort();
	}
}

function take_prestarted_client_loaders(
	prestarted_client_loaders: Map<string, PrestartedClientLoader[]>,
	operation_id: string,
): PrestartedClientLoader[] {
	const loaders = prestarted_client_loaders.get(operation_id) ?? [];
	prestarted_client_loaders.delete(operation_id);
	return loaders;
}

function prestart_known_client_loaders(input: {
	client_loader_registry: Map<string, ClientLoaderRegistryEntry>;
	pattern_registry: PatternRegistry;
	prestarted_client_loaders: Map<string, PrestartedClientLoader[]>;
	request: RouteFetchRequest;
}): void {
	const url = new URL(input.request.href, window.location.href);
	const matched_routes = find_known_client_loader_match(
		input.pattern_registry,
		url.pathname,
	);
	if (!matched_routes) {
		return;
	}
	const known_matches = matched_routes.matches.map((route_match) => {
		const pattern = route_match.registeredPattern.originalPattern;
		const registered = input.client_loader_registry.get(pattern);
		return {
			pattern,
			input: parseSearchParams(
				registered?.search_schema,
				url.searchParams,
			),
		};
	});
	const input_by_pattern = new Map(
		known_matches.map((match) => {
			return [match.pattern, match.input] as const;
		}),
	);
	const loaders: PrestartedClientLoader[] = [];
	for (const route_match of matched_routes.matches) {
		const pattern = route_match.registeredPattern.originalPattern;
		const registered = input.client_loader_registry.get(pattern);
		if (!registered) {
			continue;
		}
		loaders.push(
			start_prestarted_client_loader({
				current_route: input.request.current_route,
				href: url.href,
				history_state: input.request.history_state,
				input: input_by_pattern.get(pattern),
				known_matches,
				loader: registered.loader,
				params: route_match.params,
				pattern,
				signal: input.request.signal,
				splat_values: route_match.splatValues,
				trigger: client_loader_trigger_from_route_trigger(
					input.request.trigger,
				),
			}),
		);
	}
	if (loaders.length === 0) {
		return;
	}
	input.prestarted_client_loaders.set(input.request.operation_id, loaders);
	input.request.signal.addEventListener(
		"abort",
		() => {
			abort_prestarted_client_loaders(
				input.prestarted_client_loaders,
				input.request.operation_id,
			);
		},
		{ once: true },
	);
}

function find_known_client_loader_match(
	pattern_registry: PatternRegistry,
	pathname: string,
): FindNestedMatchesResult | null {
	const exact = findNestedMatches(pattern_registry, pathname);
	if (exact) {
		return exact;
	}
	const segments = pathname.split("/").filter((segment) => {
		return segment.length > 0;
	});
	for (let idx = segments.length; idx >= 0; idx--) {
		const partial =
			idx === 0 ? "/" : `/${segments.slice(0, idx).join("/")}`;
		const match = findNestedMatches(pattern_registry, partial);
		if (match) {
			return match;
		}
	}
	return null;
}

function start_prestarted_client_loader(input: {
	current_route: RouteFetchRequest["current_route"];
	href: string;
	history_state: unknown;
	input: unknown;
	known_matches: Array<{ input: unknown; pattern: string }>;
	loader: ClientLoaderFn;
	params: Record<string, string>;
	pattern: string;
	signal: AbortSignal;
	splat_values: string[];
	trigger: ClientLoaderTrigger;
}): PrestartedClientLoader {
	let resolve_server_state!: (state: ClientLoaderServerState) => void;
	let reject_server_state!: (cause: unknown) => void;
	const server_promise = new Promise<ClientLoaderServerState>(
		(resolve, reject) => {
			resolve_server_state = resolve;
			reject_server_state = reject;
		},
	);
	server_promise.catch(() => {});
	const controller = new AbortController();
	const abort = (): void => {
		controller.abort();
		reject_server_state(new_abort_error());
	};
	if (input.signal.aborted) {
		abort();
	} else {
		input.signal.addEventListener("abort", abort, { once: true });
	}
	let result_promise: Promise<unknown>;
	try {
		result_promise = input.loader({
			current: input.current_route ?? undefined,
			href: input.href,
			historyState: input.history_state,
			input: input.input,
			knownMatches: input.known_matches,
			params: input.params,
			pattern: input.pattern,
			serverPromise: server_promise,
			signal: controller.signal,
			splatValues: input.splat_values,
			trigger: input.trigger,
		});
	} catch (cause) {
		result_promise = Promise.reject(cause);
	}
	result_promise.catch(() => {});
	return {
		abort,
		pattern: input.pattern,
		resolve_server_state,
		result_promise,
	};
}

function client_loader_trigger_from_route_trigger(
	trigger: RouteFetchRequest["trigger"] | RoutePrepareRequest["trigger"],
): ClientLoaderTrigger {
	if (trigger === route_operation_kind.boot) {
		return route_operation_kind.boot;
	}
	if (trigger === route_fetch_trigger.prefetch) {
		return route_fetch_trigger.prefetch;
	}
	if (trigger === route_fetch_trigger.revalidation) {
		return route_fetch_trigger.revalidation;
	}
	return route_fetch_trigger.navigation;
}

function apply_scroll_intent(
	intent: ScrollIntent | undefined,
	scroll_to: ((x: number, y: number) => void) | undefined,
): void {
	const scroll = intent?.scroll;
	if (!scroll) {
		return;
	}
	if ("hash" in scroll) {
		const id = decodeURIComponent(scroll.hash.replace(/^#/, ""));
		const el = document.getElementById(id);
		el?.scrollIntoView();
		return;
	}
	(scroll_to ?? window.scrollTo.bind(window))(scroll.x, scroll.y);
}

async function run_view_transition(
	publish: () => Promise<void>,
): Promise<void> {
	const view_transition = (document as BrowserDocumentWithViewTransition)
		.startViewTransition;
	if (typeof view_transition !== "function") {
		await publish();
		return;
	}
	const transition = view_transition.call(document, publish);
	await (transition.updateCallbackDone ?? transition.finished);
}

async function run_route_hooks(
	request: RouteHookRequest,
): Promise<PreparedRoute> {
	const trigger = request.trigger;
	if (trigger === route_operation_kind.boot || !request.current_route) {
		return request.prepared;
	}
	const current = request.current_route;
	const next = request.prepared.route;
	const yield_hooks = (request.current_render?.entries ?? [])
		.map((entry) => {
			return (entry.module.default as ViewDefinition | undefined)
				?.before_route_yield;
		})
		.filter((hook) => {
			return typeof hook === "function";
		});
	const commit_hooks = request.prepared.render.entries
		.map((entry) => {
			return (entry.module.default as ViewDefinition | undefined)
				?.before_route_commit;
		})
		.filter((hook) => {
			return typeof hook === "function";
		});
	await Promise.all(
		yield_hooks.concat(commit_hooks).map((hook) => {
			return hook({
				current,
				next,
				signal: request.signal,
				trigger,
			});
		}),
	);
	if (request.signal.aborted) {
		abort_if_needed(request.signal);
	}
	return request.prepared;
}

function install_browser_listeners(
	handlers: BrowserListenerHandlers,
	module_cache: Map<string, Record<string, unknown>>,
	now_ms: () => number,
	dev: boolean,
): () => void {
	active_browser_listeners_cleanup?.();
	active_browser_listeners_cleanup = null;
	const previous_scroll_restoration = window.history.scrollRestoration;
	window.history.scrollRestoration = "manual";
	let hmr_alive = true;
	let hmr_cleanup: (() => void) | undefined;
	const on_focus = (): void => {
		handlers.on_focus({ now_ms: now_ms() });
	};
	const on_popstate = (): void => {
		const browser = read_browser_position();
		handlers.on_popstate({
			browser,
			leaving_scroll: read_current_scroll(),
			scroll: read_saved_scroll(browser.key),
		});
	};
	const on_beforeunload = (): void => {
		save_reload_scroll(now_ms());
		save_current_scroll();
	};
	window.addEventListener(browser_event.focus, on_focus);
	window.addEventListener(browser_event.popstate, on_popstate);
	window.addEventListener(browser_event.beforeunload, on_beforeunload);
	if (dev && import.meta.env.DEV) {
		void import("./hmr_dev.ts").then((hmr_dev) => {
			if (!hmr_alive) {
				return;
			}
			hmr_cleanup = hmr_dev.install_hmr_route_update_handler({
				handlers,
				module_cache,
				now_ms,
			});
		});
	}
	const cleanup = (): void => {
		hmr_alive = false;
		window.removeEventListener(browser_event.focus, on_focus);
		window.removeEventListener(browser_event.popstate, on_popstate);
		window.removeEventListener(browser_event.beforeunload, on_beforeunload);
		hmr_cleanup?.();
		window.history.scrollRestoration = previous_scroll_restoration;
		if (active_browser_listeners_cleanup === cleanup) {
			active_browser_listeners_cleanup = null;
		}
	};
	active_browser_listeners_cleanup = cleanup;
	return cleanup;
}

async function fetch_route(
	fetch_fn: typeof fetch,
	request: RouteFetchRequest,
): Promise<RouteResponseData> {
	const url = new URL(request.href, window.location.href);
	url.searchParams.set(VORMA_JSON_KEY, request.client_build_id);
	if (
		request.trigger === route_fetch_trigger.revalidation &&
		request.deployment_id
	) {
		url.searchParams.set(VERCEL_DPL_QUERY_PARAM_KEY, request.deployment_id);
	}
	const response = await fetch_fn(url, {
		signal: request.signal,
		headers: { [X_ACCEPTS_CLIENT_REDIRECT]: header_value.enabled },
	});
	const server_build_id = response.headers.get(BUILD_ID_HEADER) ?? "";
	if (response.headers.get(X_VORMA_BUILD_SKEW) === header_value.enabled) {
		return {
			kind: route_response_kind.build_skew,
			ok: response.ok,
			server_build_id,
			status: response.status,
		};
	}
	const redirect = detect_redirect(response, url);
	if (redirect) {
		return {
			kind: route_response_kind.redirect,
			hard: redirect.hard,
			href: redirect.href,
			http: redirect.http,
			ok: response.ok,
			server_build_id,
			status: response.status,
		};
	}
	if (!response.ok) {
		return {
			kind: route_response_kind.error,
			ok: response.ok,
			server_build_id,
			status: response.status,
			status_text: response.statusText,
		};
	}
	try {
		return {
			kind: route_response_kind.data,
			ok: response.ok,
			payload: await response.json(),
			server_build_id,
			status: response.status,
		};
	} catch {
		return {
			kind: route_response_kind.error,
			ok: response.ok,
			server_build_id,
			status: response.status,
			status_text: response.statusText,
		};
	}
}

async function fetch_api(
	fetch_fn: typeof fetch,
	request: APIFetchRequest,
): Promise<APIResultData> {
	const url = new URL(request.href, window.location.href);
	if (!is_same_origin(url.href)) {
		return {
			kind: api_result_kind.failure,
			error: `${api_error_message.submit_cross_origin} "${url.href}".`,
			should_revalidate: false,
		};
	}
	const headers = new Headers();
	if (request.deployment_id) {
		headers.set(VERCEL_X_DEPLOYMENT_ID, request.deployment_id);
	}
	new Headers(request.request_init?.headers ?? undefined).forEach(
		(value, key) => {
			headers.set(key, value);
		},
	);
	headers.set(X_ACCEPTS_CLIENT_REDIRECT, header_value.enabled);
	const init: RequestInit = {
		...request.request_init,
		headers,
		method: request.method,
		signal: request.signal,
	};
	const is_get =
		request.method === http_method.get ||
		request.method === http_method.head;
	if (is_get) {
		delete init.body;
	} else if (should_json_body(init.body)) {
		init.body = JSON.stringify(init.body);
		if (!headers.has(http_header.content_type)) {
			headers.set(http_header.content_type, content_type.json);
		}
	}
	const response = await fetch_fn(url, init);
	const server_build_id = response.headers.get(BUILD_ID_HEADER) ?? "";
	const redirect = detect_redirect(response, url);
	if (redirect) {
		if (!redirect.http) {
			return {
				kind: api_result_kind.failure,
				error: `${api_error_message.redirect_non_http} "${redirect.href}".`,
				ok: response.ok,
				response,
				server_build_id,
				should_revalidate: false,
				status: response.status,
			};
		}
		return {
			kind: api_result_kind.redirect,
			hard: redirect.hard,
			href: redirect.href,
			ok: response.ok,
			response,
			server_build_id,
			status: response.status,
		};
	}
	if (!response.ok) {
		return {
			kind: api_result_kind.failure,
			error: response.statusText,
			ok: response.ok,
			response,
			server_build_id,
			status: response.status,
		};
	}
	return {
		kind: api_result_kind.success,
		data: await read_api_response_data(response),
		ok: response.ok,
		response,
		server_build_id,
		status: response.status,
	};
}

async function prepare_route(
	request: RoutePrepareRequest,
	load_module: (href: string) => Promise<Record<string, unknown>>,
	remember_route_modules: (
		payload: DecodedRoutePayload,
		modules: RouteModuleMap,
	) => void,
	prestarted_loaders: PrestartedClientLoader[],
): Promise<PreparedRoute> {
	const payload = decode_route_payload(
		request.payload,
		new URL(request.href, window.location.href),
		{ decode_html_text },
	);
	const modules = await load_route_modules(payload, load_module);
	remember_route_modules(payload, modules);
	abort_if_needed(request.signal);
	request.on_provisional_route?.(
		build_prepared_route({
			client_build_id: request.client_build_id,
			client_loader_outcomes: [],
			history_state: request.history_state,
			href: request.href,
			modules,
			payload,
		}),
	);
	const client_loader_outcomes = await run_client_loaders(
		request,
		payload,
		modules,
		prestarted_loaders,
	);
	abort_if_needed(request.signal);
	await prepare_css_bundles(payload.css_bundles, request.signal);
	abort_if_needed(request.signal);
	return build_prepared_route({
		client_build_id: request.client_build_id,
		client_loader_outcomes,
		history_state: request.history_state,
		href: request.href,
		modules,
		payload,
	});
}

async function load_route_modules(
	payload: DecodedRoutePayload,
	load_module: (href: string) => Promise<Record<string, unknown>>,
): Promise<RouteModuleMap> {
	const modules = new Map<string, Record<string, unknown>>();
	const hrefs = new Set(
		payload.routes
			.map((route) => {
				return route.module_url;
			})
			.filter((href) => {
				return href.length > 0;
			}),
	);
	await Promise.all(
		Array.from(hrefs).map(async (href) => {
			modules.set(href, await load_module(href));
		}),
	);
	return modules;
}

async function run_client_loaders(
	request: RoutePrepareRequest,
	payload: DecodedRoutePayload,
	modules: RouteModuleMap,
	prestarted_loaders: PrestartedClientLoader[],
): Promise<ClientLoaderOutcome[]> {
	const known_matches = payload.routes.map((route) => {
		return { pattern: route.pattern, input: route.input };
	});
	const prestarted_by_pattern = new Map(
		prestarted_loaders.map((loader) => {
			return [loader.pattern, loader] as const;
		}),
	);
	const retained_prestarted_loaders = new Set<PrestartedClientLoader>();
	const server_error_idx = payload.routes.findIndex((route) => {
		return route.server_error !== undefined;
	});
	const controller = new AbortController();
	const abort = (): void => {
		controller.abort();
	};
	if (request.signal.aborted) {
		abort();
	} else {
		request.signal.addEventListener("abort", abort, { once: true });
	}
	const outcomes = await Promise.all(
		payload.routes.map(async (route, idx) => {
			if (request.signal.aborted || controller.signal.aborted) {
				return { kind: client_loader_outcome_kind.skipped };
			}
			const prestarted = prestarted_by_pattern.get(route.pattern);
			if (server_error_idx !== -1 && idx >= server_error_idx) {
				prestarted?.abort();
				return { kind: client_loader_outcome_kind.skipped };
			}
			if (prestarted) {
				prestarted.resolve_server_state(
					client_loader_server_state(
						payload,
						request.client_build_id,
						idx,
					),
				);
				retained_prestarted_loaders.add(prestarted);
				try {
					const data = await prestarted.result_promise;
					return { kind: client_loader_outcome_kind.success, data };
				} catch (cause) {
					if (is_abort_cause(cause)) {
						return { kind: client_loader_outcome_kind.skipped };
					}
					controller.abort();
					return {
						kind: client_loader_outcome_kind.failure,
						error: cause,
					};
				}
			}
			const module = modules.get(route.module_url);
			const def = module?.default as ViewDefinition | undefined;
			const loader = def?.client_loader;
			if (!loader) {
				return { kind: client_loader_outcome_kind.skipped };
			}
			try {
				const data = await run_client_loader(
					loader,
					request,
					payload,
					route,
					idx,
					known_matches,
					controller.signal,
				);
				return { kind: client_loader_outcome_kind.success, data };
			} catch (cause) {
				if (is_abort_cause(cause)) {
					return { kind: client_loader_outcome_kind.skipped };
				}
				controller.abort();
				return {
					kind: client_loader_outcome_kind.failure,
					error: cause,
				};
			}
		}),
	);
	for (const loader of prestarted_loaders) {
		if (!retained_prestarted_loaders.has(loader)) {
			loader.abort();
		}
	}
	request.signal.removeEventListener("abort", abort);
	return outcomes;
}

function client_loader_server_state(
	payload: DecodedRoutePayload,
	client_build_id: string,
	idx: number,
): ClientLoaderServerState {
	const server_error_idx = payload.routes.findIndex((route) => {
		return route.server_error !== undefined;
	});
	const server_error_route =
		server_error_idx === -1 ? undefined : payload.routes[server_error_idx];
	return {
		clientBuildID: client_build_id,
		matches: payload.routes.map((route) => {
			return {
				pattern: route.pattern,
				input: route.input,
				loaderData: route.loader_data,
			};
		}),
		outermostServerError:
			server_error_idx === -1 || !server_error_route
				? null
				: {
						idx: server_error_idx,
						error: server_error_route.server_error,
					},
		loaderData: payload.routes[idx]?.loader_data,
	};
}

function run_client_loader(
	loader: ClientLoaderFn,
	request: RoutePrepareRequest,
	payload: DecodedRoutePayload,
	route: DecodedRoute,
	idx: number,
	known_matches: Array<{ input: unknown; pattern: string }>,
	signal: AbortSignal,
): Promise<unknown> {
	return loader({
		current: request.current_route ?? undefined,
		href: request.href,
		historyState: request.history_state,
		input: route.input,
		knownMatches: known_matches,
		params: payload.params,
		pattern: route.pattern,
		serverPromise: Promise.resolve(
			client_loader_server_state(payload, request.client_build_id, idx),
		),
		signal,
		splatValues: payload.splat_values,
		trigger: client_loader_trigger_from_route_trigger(request.trigger),
	});
}

async function read_boot_payload(
	fallback_browser_key: string,
	now_ms: number,
): Promise<{
	browser: ReturnType<typeof read_browser_position>;
	payload: {
		client_build_id: string;
		deployment_id: string;
		raw: unknown;
	};
	reload_scroll?: ReturnType<typeof read_reload_scroll>;
}> {
	const el = document.getElementById(DATA_SCRIPT_ID);
	if (!el) {
		throw new Error(`Missing element: #${DATA_SCRIPT_ID}`);
	}
	const raw = JSON.parse(el.textContent ?? "{}") as Record<string, unknown>;
	const client_build_id = raw[boot_payload_field.client_build_id];
	const deployment_id = raw[boot_payload_field.deployment_id];
	return {
		browser: read_browser_position(fallback_browser_key),
		payload: {
			client_build_id:
				typeof client_build_id === "string" ? client_build_id : "",
			deployment_id:
				typeof deployment_id === "string" ? deployment_id : "",
			raw,
		},
		reload_scroll: read_reload_scroll(now_ms),
	};
}

function detect_redirect(
	response: Response,
	base: URL,
): { hard: boolean; href: string; http: boolean } | null {
	const soft = response.headers.get(X_CLIENT_REDIRECT);
	if (soft) {
		const href = new URL(soft, base).href;
		const http = is_http(href);
		return { hard: http && !is_same_origin(href), href, http };
	}
	if (response.redirected && response.url && response.url !== base.href) {
		const href = new URL(response.url, base).href;
		const http = is_http(href);
		return { hard: http && !is_same_origin(href), href, http };
	}
	return null;
}

function is_http(href: string): boolean {
	try {
		const protocol = new URL(href).protocol;
		return protocol === "http:" || protocol === "https:";
	} catch {
		return false;
	}
}

function is_same_origin(href: string): boolean {
	try {
		return (
			new URL(href, window.location.href).origin ===
			window.location.origin
		);
	} catch {
		return false;
	}
}

function should_json_body(body: BodyInit | null | undefined): boolean {
	return (
		!!body &&
		typeof body === "object" &&
		!(body instanceof ReadableStream) &&
		!(body instanceof FormData) &&
		!(body instanceof URLSearchParams) &&
		!(body instanceof Blob) &&
		!(body instanceof ArrayBuffer) &&
		!ArrayBuffer.isView(body)
	);
}

async function read_api_response_data(response: Response): Promise<unknown> {
	if (response.status === 204) {
		return undefined;
	}
	const content_type_header = response.headers.get(http_header.content_type);
	if (content_type_header?.toLowerCase().includes(content_type.json_token)) {
		return response.json();
	}
	const text = await response.text();
	return text.length > 0 ? text : undefined;
}

function abort_if_needed(signal: AbortSignal): void {
	if (!signal.aborted) {
		return;
	}
	throw new_abort_error();
}
