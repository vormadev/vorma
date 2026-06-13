/// <reference types="vite/client" />

import { jsonDeepEquals } from "vorma/kit/json";
import { addOnWindowFocusListener } from "vorma/kit/listeners";
import { R, type Result } from "vorma/kit/result";
import { is_abort_error, to_error_string } from "./abort_error.ts";
import type {
	ActiveFetch,
	BuildSkewDetectedEvent,
	ClientCommit,
	ClientCore,
	ClientLoaderPrefetch,
	ClientOptions,
	CommitFn,
	DecodedPayload,
	Deferred,
	FetchBase,
	FetchIntent,
	FetchResult,
	NavOptions,
	NavResult,
	NavigationSource,
	PrefetchFetch,
	PreparedRoute,
	RedirectResult,
	RouteClientLoaderTrigger,
	RouteRenderCommitReason,
	TestOptions,
	ViewDefinition,
} from "./client_core_types.ts";
import { create_client_loader_orchestrator } from "./client_loaders.ts";
import {
	BUILD_ID_HEADER,
	DATA_SCRIPT_ID,
	SCROLL_STORAGE_RELOAD_KEY,
	VERCEL_DPL_QUERY_PARAM_KEY,
	VORMA_JSON_KEY,
	VORMA_ROOT_EL_ID,
	X_ACCEPTS_CLIENT_REDIRECT,
	X_VORMA_BUILD_SKEW,
} from "./constants.ts";
import { apply_css_bundles, wait_for_css } from "./css.ts";
import { make_deferred } from "./deferred.ts";
import { apply_head_and_title } from "./head.ts";
import { create_history_position } from "./history_position.ts";
import { preload_modules } from "./modules.ts";
import { classify_redirect_target, detect_redirect } from "./redirects.ts";
import {
	create_revalidation_scheduler,
	type RevalidationRun,
} from "./revalidation_scheduler.ts";
import { create_route_modules } from "./route_modules.ts";
import {
	route_record_to_state,
	route_snapshot_to_state,
	route_to_render_state,
	type HistoryPosition,
	type RouteSnapshot,
} from "./route_state_projection.ts";
import {
	apply_scroll,
	get_scroll_for_key,
	get_scroll_pos,
	save_scroll_for_key,
	type ScrollIntent,
	type ScrollState,
} from "./scroll.ts";
import { create_submissions } from "./submissions.ts";
import type {
	AppConfig,
	BeforeRouteCommitFn,
	BeforeRouteYieldFn,
	RevalidationResult,
	RouteState,
	RouteUpdateReason,
} from "./types.ts";
import type { SsrPayload } from "./wire_contracts.gen.ts";
import { decode_payload } from "./wire_payload.ts";
import { create_work_indicator, type WorkIndicatorOptions } from "./work_indicator.ts";
import {
	derive_work_projection,
	derive_work_state,
	type WorkSources,
} from "./work_projection.ts";
import { create_empty_work_state, type WorkState } from "./work_state.ts";

/////////////////////////////////////////////////////////////////////
/////// Public Types
/////////////////////////////////////////////////////////////////////

export type { RouteRenderEntry, RouteRenderState } from "./route_state_projection.ts";
export {
	MAX_SCROLL_ENTRIES,
	apply_scroll,
	type ScrollIntent,
	type ScrollState,
} from "./scroll.ts";

export { create_empty_work_state, type WorkState } from "./work_state.ts";

export type {
	BuildSkewDetectedEvent,
	ClientCommit,
	ClientCore,
	ClientOptions,
	CommitFn,
	RevalidationReason,
	ViewDefinition,
} from "./client_core_types.ts";
export type { WorkIndicator, WorkIndicatorOptions } from "./work_indicator.ts";

/////////////////////////////////////////////////////////////////////
/////// Constants
/////////////////////////////////////////////////////////////////////

export const REFRESH_MAX_AGE_MS = 3333;
export const MAX_REDIRECTS = 10;
export {
	MAX_REVALIDATION_RETRIES,
	REVALIDATION_BACKOFF_BASE_MS,
	REVALIDATION_BACKOFF_CAP_MS,
	REVALIDATION_DEBOUNCE_MS,
} from "./revalidation_scheduler.ts";

/////////////////////////////////////////////////////////////////////
/////// Module-Level Utilities
/////////////////////////////////////////////////////////////////////

export function make_entry_id(idx: number, pattern: string): string {
	return `${idx}:${pattern}`;
}

/////////////////////////////////////////////////////////////////////
/////// Client Core
/////////////////////////////////////////////////////////////////////

export function create_client_core(
	_app_config: Omit<AppConfig, "__vorma_views" | "__vorma_resources">,
	commit: CommitFn,
	test_options?: TestOptions,
): Result<ClientCore> {
	/*
	Nine base facts:

	 1. phase: router lifecycle ('booting' | 'ready')
	 2. browser: the browser's current history entry
	 3. route_snapshot: current RouteState data available to Vorma APIs
	 4. active: the one in-flight nav or revalidation (at most one, ever)
	 5. prefetch: the at-most-one in-flight prefetch
	 6. refresh: outstanding view-data demand and its retry timing
	 7. apiRequests: concurrent API requests (independent of routes)
	 8. deferred_submit_redirect: submit redirect waiting for boot completion
	 9. seq: monotonic counter; refresh is ordered by seq, never wall clock

	Aborting a fetch and publishing a route are each single operations.

	WorkProjection and WorkState are derived from these facts. RouteRenderState
	is derived only at the adapter commit boundary.
	*/

	/////// Base Facts

	let phase: "booting" | "ready" = "booting";
	const browser = create_history_position();
	let route_snapshot: RouteSnapshot | null = null;
	let active: ActiveFetch | null = null;
	let prefetch: PrefetchFetch | null = null;
	let seq = 0;
	const scheduler = create_revalidation_scheduler({
		next_seq: () => next_seq(),
		is_ready: () => phase === "ready",
		active_seq: () => active?.seq ?? null,
		start_revalidation: (run) => start_revalidation_run(run),
		on_work_update: () => notify_work_update(),
	});

	/////// Config And Listeners

	let client_build_id = "";
	let deployment_id = "";
	let use_view_transitions = false;
	let default_error_boundary: ((props: { error: unknown }) => any) | undefined;
	let user_on_route_update:
		| ((
				route: RouteState,
				previousRoute: RouteState | null,
				reason: RouteUpdateReason,
		  ) => void)
		| undefined;
	let user_on_work_update: ((work: WorkState) => void) | undefined;
	let user_on_build_skew_detected:
		| ((event: BuildSkewDetectedEvent) => void)
		| undefined;
	let deferred_submit_redirect: URL | null = null;
	let focus_revalidation_cleanup: (() => void) | null = null;
	let window_listeners_registered = false;
	let last_activity_ts = Date.now();
	let last_work_state: WorkState = create_empty_work_state();
	const loaders = create_client_loader_orchestrator({
		client_build_id: () => client_build_id,
	});

	const work_indicator = create_work_indicator();
	let work_indicator_options: WorkIndicatorOptions | undefined;
	let work_indicator_sync_registered = false;
	const work_update_listeners = new Set<(work: WorkState) => void>();
	const route_modules = create_route_modules({
		client_build_id: () => client_build_id,
		register_loader: (pattern, client_loader) =>
			loaders.register(pattern, client_loader),
	});

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

	function is_same_origin(url: URL): boolean {
		return url.origin === current_url().origin;
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

	function route_snapshot_matches_browser(): boolean {
		return (
			route_snapshot !== null &&
			route_snapshot.position.href === browser.position().href &&
			route_snapshot.position.key === browser.position().key
		);
	}

	/////// Route Preparation

	async function prepare_route(
		payload: DecodedPayload,
		client_loader_prefetches: ClientLoaderPrefetch[],
		trigger: RouteClientLoaderTrigger,
		href: string,
		history_state: unknown,
		signal: AbortSignal,
		on_modules?: (modules: Map<string, Record<string, unknown>>) => void,
	): Promise<PreparedRoute | null> {
		const modules = await route_modules.prepare(payload, signal);
		if (!modules) {
			return null;
		}
		on_modules?.(modules);
		const client_loader_results = await loaders.run(
			payload.routes,
			payload,
			client_loader_prefetches,
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
			route: route_modules.build_route_record(
				payload,
				modules,
				client_loader_results,
			),
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

	function report_build_skew(
		event: Omit<
			BuildSkewDetectedEvent,
			| "activeClientBuildId"
			| "currentRouteState"
			| "currentWorkState"
			| "serverBuildId"
		> & {
			response: Response;
		},
	): boolean {
		const server_build_id = event.response.headers.get(BUILD_ID_HEADER) ?? "";
		if (!server_build_id || server_build_id === client_build_id) {
			return false;
		}
		if (!route_snapshot) {
			return false;
		}
		user_on_build_skew_detected?.({
			activeClientBuildId: client_build_id,
			serverBuildId: server_build_id,
			triggeringResponse: event.triggeringResponse,
			currentRouteState: route_snapshot_to_state(route_snapshot),
			currentWorkState: derive_work_state(collect_work_sources()),
		});
		return true;
	}

	function report_route_build_skew(
		f: FetchBase & { intent: FetchIntent },
		response: Response,
	): boolean {
		const base = {
			kind: "route" as const,
			requestedHref: f.url.href,
			status: response.status,
			ok: response.ok,
		};
		let triggering_response: BuildSkewDetectedEvent["triggeringResponse"];
		if (f.intent.kind === "reval") {
			triggering_response = {
				...base,
				trigger: "revalidation",
				revalidationReason: f.intent.reason,
			};
		} else if (f.intent.kind === "prefetch") {
			triggering_response = {
				...base,
				trigger: "prefetch",
			};
		} else {
			triggering_response = {
				...base,
				trigger: f.intent.source === "popstate" ? "popstate" : "navigation",
			};
		}

		return report_build_skew({
			response,
			triggeringResponse: triggering_response,
		});
	}

	function history_state_for_fetch(intent: FetchIntent): unknown {
		if (intent.kind === "nav") {
			return intent.options.is_popstate
				? browser.position().state
				: intent.options.state;
		}
		if (intent.kind === "reval") {
			return browser.position().state;
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
		const client_loader_prefetches: ClientLoaderPrefetch[] = [];
		let client_loader_prefetches_open = true;
		loaders.prestart({
			url,
			history_state: () => history_state_for_fetch(intent),
			trigger:
				intent.kind === "prefetch"
					? "prefetch"
					: intent.kind === "reval"
						? "revalidation"
						: "navigation",
			signal: ac.signal,
			is_open: () => client_loader_prefetches_open,
			push: (p) => client_loader_prefetches.push(p),
		});
		return {
			url,
			ac,
			seq: next_seq(),
			data_promise: (async () => {
				const modified = new URL(url.href);
				modified.searchParams.set(VORMA_JSON_KEY, client_build_id);
				if (is_revalidation && deployment_id) {
					modified.searchParams.set(VERCEL_DPL_QUERY_PARAM_KEY, deployment_id);
				}
				const res = await fetch(modified, {
					signal: ac.signal,
					headers: { [X_ACCEPTS_CLIENT_REDIRECT]: "1" },
				});

				if (res.headers.get(X_VORMA_BUILD_SKEW) === "1") {
					return { kind: "build_skew", response: res };
				}
				const redirect = detect_redirect(res, modified);
				if (redirect) {
					return { kind: "redirect", ...redirect, response: res };
				}
				if (!res.ok) {
					return { kind: "error", response: res };
				}
				try {
					const data = await res.json();
					return { kind: "data", data, response: res };
				} catch {
					return { kind: "error", response: res };
				}
			})(),
			client_loader_prefetches,
			close_client_loader_prefetches: () => {
				client_loader_prefetches_open = false;
			},
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

	function active_matches_expected_url(expected_url: URL | undefined): boolean {
		return (
			expected_url === undefined ||
			matches_without_hash(new URL(browser.position().href), expected_url)
		);
	}

	async function run_active(f: ActiveFetch, expected_url?: URL): Promise<void> {
		active = f;
		notify_work_update();

		let published = false;
		let nav_transferred = false;
		try {
			const result = await f.data_promise;
			if (!can_commit(f)) {
				return;
			}
			if (!active_matches_expected_url(expected_url)) {
				return;
			}

			if (result.kind === "build_skew") {
				report_route_build_skew(f, result.response);
				if (f.intent.kind === "reval") {
					scheduler.mark_build_skew();
					return;
				}
				hard_redirect(f.url.href);
				return;
			}

			if (result.response) {
				const did_detect_skew = report_route_build_skew(f, result.response);
				if (
					did_detect_skew &&
					result.kind === "redirect" &&
					result.hard &&
					f.intent.kind === "reval"
				) {
					scheduler.mark_build_skew();
					return;
				}
			}

			if (result.kind === "error") {
				return;
			}

			if (result.kind === "redirect") {
				nav_transferred = handle_redirect(f, result) === "transferred";
				return;
			}

			const payload = decode_payload(
				result.data,
				f.url,
				loaders.register_search_schema,
			);
			preload_modules(payload.deps);
			f.close_client_loader_prefetches();
			if (!can_commit(f)) {
				return;
			}

			if (f.intent.kind === "nav") {
				await f.prepare_ready;
				if (!can_commit(f)) {
					return;
				}
			}

			const prepared = await prepare_route(
				payload,
				f.client_loader_prefetches,
				f.intent.kind === "reval" ? "revalidation" : "navigation",
				f.url.href,
				history_state_for_fetch(f.intent),
				f.ac.signal,
			);
			if (!prepared || !can_commit(f)) {
				return;
			}
			if (!active_matches_expected_url(expected_url)) {
				return;
			}

			const was_published = await publish(f, prepared);
			if (was_published) {
				published = true;
				last_activity_ts = Date.now();
				scheduler.mark_fresh(f.seq);
			}
		} catch (err) {
			// Transaction failures resolve as didNavigate=false; surface the
			// underlying cause (e.g. a failed module import) in dev.
			if (import.meta.env.DEV && !is_abort_error(err)) {
				console.error("Vorma: route transaction failed", err);
			}
		} finally {
			if (f.intent.kind === "nav" && !nav_transferred) {
				resolve_nav(f, { didNavigate: published });
			}
			if (active === f) {
				active = null;
			}
			notify_work_update();
			scheduler.maybe_revalidate();
		}
	}

	// Publish a prepared route. Clears `active` as soon as the commit lands
	// inside the view transition so subsequent same-page navs can fire
	// while the transition animation is still running. Returns true if the
	// commit actually happened (it may be superseded mid-callback).
	async function publish(f: ActiveFetch, prepared: PreparedRoute): Promise<boolean> {
		const commit_reason: Exclude<RouteUpdateReason, "boot"> =
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
			f.intent.kind === "nav" ? f.intent.url.href : prev_snapshot.position.href;
		const next_history_state =
			f.intent.kind === "nav"
				? f.intent.options.is_popstate
					? browser.position().state
					: f.intent.options.state
				: prev_snapshot.position.state;
		const yield_hooks = prev_snapshot.route.matches
			.map((m) => {
				return (m.module.default as ViewDefinition | undefined)
					?.before_route_yield;
			})
			.filter((h): h is BeforeRouteYieldFn => {
				return typeof h === "function";
			});
		const commit_hooks = prepared.route.matches
			.map((m) => {
				return (m.module.default as ViewDefinition | undefined)
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
						return Promise.resolve(
							h({
								trigger: commit_reason,
								signal: f.ac.signal,
								current,
								next,
							}),
						);
					})
					.concat(
						commit_hooks.map((h) => {
							return Promise.resolve(
								h({
									trigger: commit_reason,
									signal: f.ac.signal,
									current,
									next,
								}),
							);
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
				browser.save_current_scroll();
				position = browser.commit(
					f.intent.url.href,
					f.intent.options.replace,
					f.intent.options.state,
				);
			} else {
				// popstate: browser already at the new URL; revalidation: URL unchanged
				position = browser.position();
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
						target_entry_id: make_entry_id(
							prepared.route.matches.length - 1,
							prepared.route.matches[prepared.route.matches.length - 1]
								?.pattern ?? "",
						),
					};
				}
			}

			active = null;
			commit_route_snapshot(
				commit_reason,
				prev,
				route_snapshot,
				scroll_intent,
				take_work_update(),
			);
			did_publish = true;
		};

		const vt = (document as any).startViewTransition;
		if (f.intent.kind === "nav" && use_view_transitions && typeof vt === "function") {
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

	function to_route_update_reason(reason: RouteRenderCommitReason): RouteUpdateReason {
		if (reason === "initial") {
			return "boot";
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
		work?: WorkState,
	): void {
		const previous_route = prev ? route_snapshot_to_state(prev) : null;
		const route = route_snapshot_to_state(next);
		const client_commit: ClientCommit = {
			route_render: {
				state: route_to_render_state(next.route, next.position.state),
				scroll_intent,
			},
			work,
		};
		if (!previous_route || !jsonDeepEquals(previous_route, route)) {
			client_commit.route_update = {
				previous_route,
				reason: to_route_update_reason(reason),
				route,
			};
		}
		emit_client_commit(client_commit);
	}

	function handle_redirect(
		f: ActiveFetch,
		redirect: Extract<FetchResult, { kind: "redirect" }>,
	): RedirectResult {
		const nav_intent = f.intent.kind === "nav" ? f.intent : null;
		const classified = classify_redirect_target(
			redirect.href,
			redirect.hard,
			current_url().origin,
		);

		// Invalid scheme: treat as failure for nav, no-op otherwise.
		if (classified.kind === "invalid") {
			nav_intent?.deferred.resolve({ didNavigate: false });
			active = null;
			return "settled";
		}

		// Hard redirect or cross-origin: leave the page.
		if (classified.kind === "hard") {
			hard_redirect(classified.href);
			nav_intent?.deferred.resolve({ didNavigate: false });
			active = null;
			return "settled";
		}

		// Redirect loop guard (only meaningful for nav, which tracks count).
		if (nav_intent && nav_intent.redirect_count >= MAX_REDIRECTS) {
			nav_intent.deferred.resolve({ didNavigate: false });
			active = null;
			return "settled";
		}

		const target = classified.url;
		active = null;

		// Same-page redirect (same path, maybe different hash): no re-fetch,
		// just handle as same-page nav.
		if (route_snapshot_matches(target)) {
			const result = handle_same_page_nav(target, nav_intent?.options ?? {});
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
			jsonDeepEquals(cur.options.state, options.state) &&
			cur.options.is_popstate === options.is_popstate &&
			jsonDeepEquals(cur.options.popstate_scroll, options.popstate_scroll) &&
			cur.options.skip_work_indicator === options.skip_work_indicator &&
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
			browser.save_current_scroll();
			const position = browser.commit(url.href, options.replace, options.state);
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
			const position = browser.commit(url.href, true, options.state);
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
		const source = reuse?.source ?? (options.is_popstate ? "popstate" : "navigate");

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

	function start_revalidation_run(run: RevalidationRun): void {
		const url = new URL(browser.position().href || window.location.href);
		const f = start_fetch(
			url,
			{
				kind: "reval",
				attempt: run.attempt,
				reason: run.reason,
				skip_work_indicator: run.skip_work_indicator,
			},
			true,
		);
		void run_active(f, url);
	}

	/////// Prefetch

	async function prepare_prefetch(f: PrefetchFetch): Promise<void> {
		try {
			const result = await f.data_promise;
			if (f.ac.signal.aborted) {
				if (prefetch === f) {
					prefetch = null;
					notify_work_update();
				}
				return;
			}
			if (result.response) {
				report_route_build_skew(f, result.response);
			}
			if (result.kind !== "data") {
				if (prefetch === f) {
					prefetch = null;
					notify_work_update();
				}
				return;
			}
			const payload = decode_payload(
				result.data,
				f.url,
				loaders.register_search_schema,
			);
			preload_modules(payload.deps);
			f.close_client_loader_prefetches();
			const modules = await route_modules.prepare(payload, f.ac.signal);
			if (!modules || f.ac.signal.aborted) {
				if (prefetch === f) {
					prefetch = null;
					notify_work_update();
				}
				return;
			}
			const known_matches = loaders.routes_to_known_matches(payload.routes);
			const next_client_loader_prefetches: ClientLoaderPrefetch[] = [];
			loaders.reconcile(
				payload.routes,
				f.client_loader_prefetches,
				(route, i, retained, suppressed) => {
					if (suppressed) {
						return;
					}
					if (retained) {
						next_client_loader_prefetches.push(retained);
						return;
					}
					const client_loader = loaders.get(route.pattern);
					if (!client_loader) {
						return;
					}
					const p = loaders.make_prefetch({
						href: f.url.href,
						history_state: undefined,
						known_matches,
						client_loader,
						params: payload.params,
						pattern: route.pattern,
						route_input: route.input,
						signal: f.ac.signal,
						splat_values: payload.splat_values,
						trigger: "prefetch",
					});
					p.resolve_server_state(loaders.build_server_state(payload.routes, i));
					next_client_loader_prefetches.push(p);
				},
			);
			f.client_loader_prefetches = next_client_loader_prefetches;
		} catch {
			if (prefetch === f) {
				prefetch = null;
				notify_work_update();
			}
		} finally {
			if (f.is_pending) {
				f.is_pending = false;
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
		if (active?.intent.kind === "nav" && matches_without_hash(active.url, url)) {
			return;
		}
		if (prefetch && matches_without_hash(prefetch.url, url)) {
			return;
		}
		cancel_prefetch();

		// Prefetch's prepare_ready resolves when modules + client-loader seeding are done.
		let resolve_prepare!: () => void;
		const prepare_ready = new Promise<void>((r) => {
			resolve_prepare = r;
		});

		const f: PrefetchFetch = {
			...start_fetch(url, { kind: "prefetch" }, false, prepare_ready),
			is_pending: true,
		};
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

	const submissions_manager = create_submissions({
		is_ready: () => phase === "ready",
		is_booted: () => route_snapshot !== null,
		is_same_origin: (url) => is_same_origin(url),
		current_origin: () => current_url().origin,
		deployment_id: () => deployment_id,
		require_revalidation: (waiter, skip_work_indicator) =>
			scheduler.require("apiRequest", waiter, undefined, skip_work_indicator),
		start_pending_revalidation: () => scheduler.maybe_revalidate(),
		redirect_to: (url) => {
			if (phase !== "ready") {
				deferred_submit_redirect = url;
				return;
			}
			void start_nav_inner(url, { replace: true }, 0, { source: "redirect" });
		},
		hard_redirect: (href) => hard_redirect(href),
		report_resource_build_skew: (input) => {
			report_build_skew({
				response: input.response,
				triggeringResponse: {
					kind: "resource",
					resourceKind: input.resource_kind,
					requestedHref: input.requested_href,
					method: input.method,
					status: input.response.status,
					ok: input.response.ok,
				},
			});
		},
		on_work_update: () => notify_work_update(),
	});

	/////// Popstate

	async function handle_popstate(): Promise<void> {
		const prev = browser.position();
		const next = browser.read_from_window();
		if (next.key === prev.key && next.href === prev.href) {
			return;
		}

		if (prev.key && prev.key !== next.key) {
			save_scroll_for_key(prev.key, get_scroll_pos());
		}
		browser.adopt(next);

		const prev_url = new URL(prev.href);
		const next_url = new URL(next.href);

		// Hash-only popstate on same path: scroll, no fetch.
		if (matches_without_hash(prev_url, next_url)) {
			if (route_snapshot) {
				const prev_snapshot = route_snapshot;
				route_snapshot = {
					position: browser.position(),
					route: route_snapshot.route,
				};
				commit_route_snapshot("popstate", prev_snapshot, route_snapshot);
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
			if (!result.didNavigate && !active && !route_snapshot_matches_browser()) {
				reload_page();
			}
		} catch {
			if (!active && !route_snapshot_matches_browser()) {
				reload_page();
			}
		}
	}

	/////// Status

	function collect_work_sources(): WorkSources {
		const active_fetch = active;
		return {
			nav:
				active_fetch?.intent.kind === "nav"
					? {
							href: active_fetch.intent.url.href,
							replace: active_fetch.intent.options.replace === true,
							source: active_fetch.intent.source,
							skip_work_indicator:
								active_fetch.intent.options.skip_work_indicator,
						}
					: null,
			active_revalidation:
				active_fetch?.intent.kind === "reval"
					? {
							attempt: active_fetch.intent.attempt,
							skip_work_indicator: active_fetch.intent.skip_work_indicator,
						}
					: null,
			refresh: scheduler.work_view(),
			refresh_demand_skip_work_indicator: scheduler.demand()?.skip_work_indicator,
			refresh_pending_visible: !scheduler.active_will_refresh() && !active,
			submissions: submissions_manager.work_sources(),
			prefetch: prefetch?.is_pending ? { href: prefetch.url.href } : null,
		};
	}

	function take_work_update(): WorkState | undefined {
		const next = derive_work_state(collect_work_sources());
		if (jsonDeepEquals(last_work_state, next)) {
			return undefined;
		}
		last_work_state = next;
		return next;
	}

	function emit_client_commit(client_commit: ClientCommit): void {
		commit(client_commit);
		if (client_commit.route_update && user_on_route_update) {
			user_on_route_update(
				client_commit.route_update.route,
				client_commit.route_update.previous_route,
				client_commit.route_update.reason,
			);
		}
		if (client_commit.work) {
			for (const fn of work_update_listeners) {
				fn(client_commit.work);
			}
			user_on_work_update?.(client_commit.work);
		}
	}

	function notify_work_update(): void {
		const work = take_work_update();
		if (!work) {
			sync_work_indicator();
			return;
		}
		emit_client_commit({ work });
	}

	/////// HMR

	function register_hmr(): void {
		if (!import.meta.env.DEV || !import.meta.hot) {
			return;
		}
		window.__vorma_hmr_view_update = async (raw_url, mod) => {
			if (!route_snapshot) {
				return;
			}
			const { url, hmr_version } = route_modules.hmr_update(raw_url, mod);
			const inspected_snapshot = route_snapshot;
			const idx = inspected_snapshot.route.matches.findIndex(
				(m) => route_modules.normalize(m.module_url) === url,
			);
			if (idx === -1) {
				return;
			}

			const match = inspected_snapshot.route.matches[idx]!;
			const def = mod.default as ViewDefinition | undefined;
			if (def?.client_loader) {
				loaders.register(match.pattern, def.client_loader);
			} else {
				loaders.unregister(match.pattern);
			}

			let client_loader_data = match.client_loader_data;
			if (route_modules.should_rerun_on_hmr(match.pattern)) {
				const client_loader = loaders.get(match.pattern);
				if (client_loader) {
					try {
						const current_route = inspected_snapshot.route;
						const routes = current_route.matches.map((m, i) => {
							return {
								pattern: m.pattern,
								input: m.input,
								module_url: m.module_url,
								view_data: m.view_data,
								server_error:
									current_route.error?.source === "server" &&
									current_route.error.idx === i
										? current_route.error.error
										: undefined,
							};
						});
						client_loader_data = await client_loader({
							trigger: "revalidation",
							href: inspected_snapshot.position.href,
							historyState: inspected_snapshot.position.state,
							pattern: match.pattern,
							params: inspected_snapshot.route.params,
							splatValues: inspected_snapshot.route.splat_values,
							input: match.input,
							knownMatches: loaders.routes_to_known_matches(routes),
							serverPromise: Promise.resolve(
								loaders.build_server_state(routes, idx),
							),
							signal: new AbortController().signal,
						});
					} catch (err) {
						console.error("Vorma: HMR client loader re-run failed", err);
					}
				}
			}
			if (route_snapshot !== inspected_snapshot) {
				return;
			}

			const prev = inspected_snapshot;
			const matches = inspected_snapshot.route.matches.map((m, i) =>
				i === idx ? { ...m, hmr_version, module: mod, client_loader_data } : m,
			);
			route_snapshot = {
				position: inspected_snapshot.position,
				route: { ...inspected_snapshot.route, matches },
			};
			commit_route_snapshot("hmr", prev, route_snapshot);
		};
	}

	/////// Work Indicator

	function setup_work_indicator(options: WorkIndicatorOptions | undefined): void {
		work_indicator_options = options;
		work_indicator.configure(options);
		if (!work_indicator_sync_registered) {
			work_update_listeners.add(sync_work_indicator);
			work_indicator_sync_registered = true;
		}
		sync_work_indicator();
	}

	function sync_work_indicator(_work?: WorkState): void {
		const options = work_indicator_options;
		if (!options) {
			work_indicator.set_vorma_active(false);
			return;
		}

		let active_for_vorma = false;
		for (const work of derive_work_projection(collect_work_sources())) {
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
				options.skipApiRequests !== true &&
				!work.skip_work_indicator
			) {
				active_for_vorma = true;
				break;
			}
		}
		work_indicator.set_vorma_active(active_for_vorma);
	}

	/////// Boot

	async function boot(options: ClientOptions): Promise<Result<void>> {
		const payload_el = document.getElementById(DATA_SCRIPT_ID);
		if (!payload_el) {
			return R.err(`Missing element: #${DATA_SCRIPT_ID}`);
		}
		// The cast target is generated from the Rust SsrPayload struct, so
		// every field access below typechecks against the real wire contract.
		let raw_payload: SsrPayload;
		try {
			raw_payload = JSON.parse(payload_el.textContent ?? "{}") as SsrPayload;
		} catch (err) {
			return R.err(`Failed to parse #${DATA_SCRIPT_ID}: ${to_error_string(err)}`);
		}
		if (raw_payload.client_build_id) {
			client_build_id = raw_payload.client_build_id;
		}
		if (raw_payload.deployment_id) {
			deployment_id = raw_payload.deployment_id;
		}
		const payload = decode_payload(
			raw_payload,
			current_url(),
			loaders.register_search_schema,
		);

		user_on_route_update = options.onRouteUpdate;
		user_on_work_update = options.onWorkUpdate;
		user_on_build_skew_detected = options.onBuildSkewDetected;
		default_error_boundary = options.defaultErrorBoundary;
		use_view_transitions = options.useViewTransitions ?? false;

		browser.ensure_window_key();
		try {
			window.history.scrollRestoration = "manual";
		} catch {}

		// Boot is the standard route transaction with the payload already in
		// hand; the only boot-specific step is the provisional snapshot, which
		// lets router APIs (getRouteState, submit) work during initial
		// client-loader execution.
		const initial_ac = new AbortController();
		const prepared = await prepare_route(
			payload,
			[],
			"boot",
			browser.position().href,
			browser.position().state,
			initial_ac.signal,
			(modules) => {
				route_snapshot = {
					position: browser.position(),
					route: route_modules.build_route_record(payload, modules, []),
				};
			},
		);
		if (!prepared) {
			return R.err("Initial navigation produced no state");
		}

		const route = prepared.route;
		route_snapshot = { position: browser.position(), route };
		prepared.apply_dom_side_effects();

		let scroll_intent: ScrollIntent | undefined;
		const make_scroll_intent = (scroll: ScrollState): ScrollIntent => {
			return {
				scroll,
				target_entry_id: make_entry_id(
					route.matches.length - 1,
					route.matches[route.matches.length - 1]?.pattern ?? "",
				),
			};
		};
		let refresh_scroll_raw: string | null;
		try {
			refresh_scroll_raw = sessionStorage.getItem(SCROLL_STORAGE_RELOAD_KEY);
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

		setup_work_indicator(options.workIndicator);
		if (focus_revalidation_cleanup) {
			focus_revalidation_cleanup();
			focus_revalidation_cleanup = null;
		}
		if (options.revalidateOnWindowFocus) {
			const focus_revalidation_options =
				typeof options.revalidateOnWindowFocus === "object"
					? options.revalidateOnWindowFocus
					: null;
			const stale_ms = focus_revalidation_options?.staleTimeMs ?? 5_000;
			const skip_work_indicator =
				focus_revalidation_options?.skipWorkIndicator === true;
			focus_revalidation_cleanup = addOnWindowFocusListener(() => {
				const work = derive_work_state(collect_work_sources());
				if (work.navigation || work.revalidation || work.apiRequests.length > 0) {
					return;
				}
				if (Date.now() - last_activity_ts >= stale_ms) {
					scheduler.require(
						"windowFocus",
						undefined,
						true,
						skip_work_indicator,
					);
					notify_work_update();
				}
			});
		}
		if (options.render) {
			await options.render();
		}

		if (!window_listeners_registered) {
			window_listeners_registered = true;
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
		}
		register_hmr();

		phase = "ready";
		if (deferred_submit_redirect) {
			const redirect = deferred_submit_redirect;
			deferred_submit_redirect = null;
			void start_nav_inner(redirect, { replace: true }, 0, {
				source: "redirect",
			});
		}
		scheduler.maybe_revalidate();
		return R.ok(undefined);
	}

	/////// Public API

	async function navigate(
		href: string | URL,
		options?: {
			replace?: boolean;
			scrollToTop?: boolean;
			state?: unknown;
			skipWorkIndicator?: boolean;
		},
	): Promise<NavResult> {
		if (phase !== "ready") {
			throw new Error("Vorma not booted");
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
				skip_work_indicator: options?.skipWorkIndicator,
			},
			0,
		);
	}

	async function revalidate(): Promise<RevalidationResult> {
		if (phase !== "ready") {
			throw new Error("Vorma not booted");
		}
		const waiter = make_deferred<RevalidationResult>();
		scheduler.require("manual", waiter, true);
		notify_work_update();
		return waiter.promise;
	}

	function get_route_state(): RouteState {
		const snapshot = route_snapshot;
		if (!snapshot) {
			throw new Error("Vorma not booted");
		}
		return route_snapshot_to_state(snapshot);
	}

	function get_work_state(): WorkState {
		if (!route_snapshot) {
			throw new Error("Vorma not booted");
		}
		return derive_work_state(collect_work_sources());
	}

	function get_root_el(): HTMLElement {
		const el = document.getElementById(VORMA_ROOT_EL_ID);
		if (el) {
			return el;
		}
		const fresh = document.createElement("div");
		fresh.id = VORMA_ROOT_EL_ID;
		document.body.insertBefore(fresh, document.body.firstChild);
		return fresh;
	}

	function define_view<T = any>(input: {
		pattern: string;
		component: (props: any) => any;
		errorBoundary?: (props: { error: unknown }) => any;
		clientLoader?: (props: any) => Promise<T>;
		beforeRouteCommit?: BeforeRouteCommitFn;
		beforeRouteYield?: BeforeRouteYieldFn;
		runClientLoaderOnHmr?: boolean;
	}): ViewDefinition & { __phantom_client_loader_data?: T } {
		route_modules.set_hmr_rerun(input.pattern, input.runClientLoaderOnHmr === true);
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
		boot,
		workIndicator: work_indicator.indicator,
		navigate,
		revalidate,
		submit_inner: submissions_manager.submit,
		getRouteState: get_route_state,
		getWorkState: get_work_state,
		getClientBuildId: () => client_build_id,
		getRootEl: get_root_el,
		defineView: define_view,
		start_prefetch,
		stop_prefetch,
		save_current_scroll: browser.save_current_scroll,
		get_default_error_boundary: () => default_error_boundary,
	});
}

declare global {
	interface Window {
		__vorma_hmr_view_update?: (
			raw_url: string,
			mod: Record<string, unknown>,
		) => Promise<void>;
	}
}
