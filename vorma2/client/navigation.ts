/// <reference types="vite/client" />

import { jsonDeepEquals } from "vorma/kit/json";
import { findNestedMatches } from "vorma/kit/matcher/find-nested";
import { ABORT_REASON, encode_abort_reason, is_abort_error } from "./abort.ts";
import {
	complete_client_loaders,
	create_prestarts,
	type ClientLoaderPrestart,
} from "./client_loaders.ts";
import {
	build_active_components,
	compute_effective_error_idx,
	load_modules,
	resolve_error_boundary,
} from "./components.ts";
import {
	dispatch_client_build_id,
	dispatch_route_change,
	dispatch_status,
} from "./events.ts";
import {
	ensure_global,
	get_global,
	get_snapshot,
	set_snapshot,
} from "./global_state.ts";
import { apply_head_and_title } from "./head.ts";
import { commit_history, perform_hard_redirect } from "./history.ts";
import {
	apply_css_bundles,
	fetch_route_data,
	make_hard_reload_href,
	wait_for_css,
} from "./network.ts";
import {
	apply_scroll_state,
	get_scroll_for_current_key_or_top,
	normalize_hash,
} from "./scroll.ts";
import type {
	ComponentModulesMap,
	NavigateProps,
	NavigationArtifacts,
	NavigationOperation,
	NavigationStateManager,
	NavigationStore,
	RuntimeRouteSnapshot,
	ScrollState,
	StatusEventDetail,
	SubmitOptions,
	SubmitResult,
} from "./types.ts";
import {
	assert_same_origin,
	classify_target,
	get_target_data_key,
	is_same_origin,
	make_absolute,
} from "./url.ts";

const MAX_REDIRECTS = 10;

export function get_manager(): NavigationStateManager {
	const g = ensure_global();
	if (!g.nav_state_manager) {
		g.nav_state_manager = create_nav_manager();
	}
	return g.nav_state_manager;
}

// ─── Store Creation ──────────────────────────────────────────────

function create_store(): NavigationStore {
	return {
		navigate_op: null,
		revalidate_op: null,
		queued_revalidate_data_key: null,
		queued_revalidate_promise: null,
		prefetch_ops: new Map(),
		prefetch_cache: new Map(),
		skipped_loading_nav_ids: new Set(),
		skipped_loading_sub_ids: new Set(),
		next_nav_op_id: 1,
		next_sub_op_id: 1,
		latest_started_sub_op_id: 0,
		active_sub_ids: new Set(),
		sub_abort_controllers: new Map(),
		sub_id_by_dedupe_key: new Map(),
		last_status: {
			isNavigating: false,
			isSubmitting: false,
			isRevalidating: false,
		},
		last_nav_or_revalidate_ts: Date.now(),
	};
}

// ─── Status ──────────────────────────────────────────────────────

function has_visible_submissions(store: NavigationStore): boolean {
	for (const id of store.active_sub_ids) {
		if (!store.skipped_loading_sub_ids.has(id)) {
			return true;
		}
	}
	return false;
}

function emit_status(store: NavigationStore): void {
	const next = get_loading_status(store);
	if (jsonDeepEquals(store.last_status, next)) {
		return;
	}
	store.last_status = next;
	dispatch_status(next);
}

export function get_loading_status(store: NavigationStore): StatusEventDetail {
	return {
		isNavigating:
			store.navigate_op !== null &&
			!store.skipped_loading_nav_ids.has(store.navigate_op.id),
		isSubmitting: has_visible_submissions(store),
		isRevalidating:
			store.revalidate_op !== null &&
			!store.skipped_loading_nav_ids.has(store.revalidate_op.id),
	};
}

// ─── Client Build ID Sync ────────────────────────────────────────

function sync_client_build_id(next_id: string): void {
	if (!next_id) {
		return;
	}
	const prev = get_snapshot().client_build_id;
	if (prev === next_id) {
		return;
	}
	set_snapshot({ ...get_snapshot(), client_build_id: next_id });
	dispatch_client_build_id({
		oldClientBuildID: prev,
		newClientBuildID: next_id,
	});
}

// ─── Operation Helpers ───────────────────────────────────────────

function is_active(store: NavigationStore, op: NavigationOperation): boolean {
	if (op.intent === "navigate") {
		return store.navigate_op?.id === op.id;
	}
	if (op.intent === "revalidate") {
		return store.revalidate_op?.id === op.id;
	}
	return (
		store.prefetch_ops.get(get_target_data_key(op.target_url))?.id === op.id
	);
}

function clear_op(store: NavigationStore, op: NavigationOperation): void {
	store.skipped_loading_nav_ids.delete(op.id);
	if (op.intent === "navigate" && store.navigate_op?.id === op.id) {
		store.navigate_op = null;
	} else if (
		op.intent === "revalidate" &&
		store.revalidate_op?.id === op.id
	) {
		store.revalidate_op = null;
	} else if (op.intent === "prefetch") {
		const key = get_target_data_key(op.target_url);
		if (store.prefetch_ops.get(key)?.id === op.id) {
			store.prefetch_ops.delete(key);
		}
	}
}

// ─── Component Reuse Check ──────────────────────────────────────

function can_reuse_components(
	intent: string,
	current: RuntimeRouteSnapshot,
	next: RuntimeRouteSnapshot,
): boolean {
	if (intent !== "revalidate") {
		return false;
	}
	return (
		jsonDeepEquals(current.import_urls, next.import_urls) &&
		jsonDeepEquals(current.export_keys, next.export_keys) &&
		jsonDeepEquals(current.error_export_keys, next.error_export_keys) &&
		current.outermost_server_error_idx === next.outermost_server_error_idx
	);
}

// ─── Zero-Server-Loader Skip ────────────────────────────────────
// When the entire target route tree has no server loaders and we
// have module info for every matched pattern (from a prior
// navigation), we can build a synthetic snapshot without a server
// round-trip. Safe because: if no server loaders exist, the
// server response would contain only empty/undefined data anyway.

function try_skip_server_fetch(
	target_url: string,
): { snapshot: RuntimeRouteSnapshot } | null {
	const g = get_global();
	if (!g.route_manifest) {
		return null;
	}

	const url = new URL(target_url, window.location.href);
	const match_result = findNestedMatches(g.pattern_registry, url.pathname);
	if (!match_result) {
		return null;
	}

	const current = get_snapshot();
	const module_map = g.client_module_map;

	const matched_patterns: string[] = [];
	const import_urls: string[] = [];
	const export_keys: string[] = [];
	const error_export_keys: string[] = [];
	const loaders_data: unknown[] = [];

	for (const match of match_result.matches) {
		const pattern = match.registeredPattern.originalPattern;

		// Any server loader in the tree → must fetch
		if (g.route_manifest[pattern] === 1) {
			return null;
		}

		// Must have module info from a previous navigation
		const info = module_map[pattern];
		if (!info) {
			return null;
		}

		matched_patterns.push(pattern);
		import_urls.push(info.import_url);
		export_keys.push(info.export_key);
		error_export_keys.push(info.error_export_key);
		loaders_data.push(undefined);
	}

	return {
		snapshot: {
			outermost_server_error: undefined,
			outermost_server_error_idx: undefined,
			outermost_client_error: undefined,
			outermost_client_error_idx: undefined,
			outermost_error: undefined,
			outermost_error_idx: undefined,
			matched_patterns,
			loaders_data,
			import_urls,
			export_keys,
			error_export_keys,
			has_root_data: false,
			params: match_result.params,
			splat_values: match_result.splatValues,
			client_build_id: current.client_build_id,
			active_components: [],
			active_error_boundary: undefined,
			client_loaders_data: [],
		},
	};
}

// ─── Redirect Following ─────────────────────────────────────────

function assert_http_scheme(href: string): void {
	const scheme = new URL(href, window.location.href).protocol;
	if (scheme !== "http:" && scheme !== "https:") {
		throw new Error(
			`Redirect target must use an HTTP(S) scheme. Received: "${href}".`,
		);
	}
}

async function follow_redirect(props: {
	href: string;
	client_build_id: string;
	is_hard_reload: boolean;
	store?: NavigationStore;
	hop_count?: number;
	skip_loading?: boolean;
}): Promise<{ didNavigate: boolean }> {
	assert_http_scheme(props.href);
	if (props.is_hard_reload) {
		perform_hard_redirect(
			make_hard_reload_href(props.href, props.client_build_id),
		);
		return { didNavigate: false };
	}
	if (!is_same_origin(props.href)) {
		perform_hard_redirect(props.href);
		return { didNavigate: false };
	}
	if (
		classify_target(props.href, window.location.href) ===
		"same-document-noop"
	) {
		return { didNavigate: false };
	}
	const hop = props.hop_count ?? 0;
	if (hop >= MAX_REDIRECTS - 1) {
		console.error("Vorma:", "Too many redirects");
		return { didNavigate: false };
	}
	if (!props.store) {
		const { vormaNavigate } = await import("./public_api.ts");
		return vormaNavigate(props.href, { replace: true });
	}
	return execute_navigation({
		store: props.store,
		props: {
			href: props.href,
			replace: true,
			intent: "navigate",
			redirect_hop_count: hop + 1,
			skip_global_loading_indicator: props.skip_loading,
		},
	});
}

// ─── Update Client Module Map ───────────────────────────────────

function update_client_module_map(snapshot: RuntimeRouteSnapshot): void {
	const module_map = get_global().client_module_map;
	for (let i = 0; i < snapshot.matched_patterns.length; i++) {
		const pattern = snapshot.matched_patterns[i];
		const import_url = snapshot.import_urls[i];
		if (pattern && import_url) {
			module_map[pattern] = {
				import_url,
				export_key: snapshot.export_keys[i] ?? "default",
				error_export_key: snapshot.error_export_keys[i] ?? "",
			};
		}
	}
}

// ─── Commit Navigation ──────────────────────────────────────────

async function commit_navigation(props: {
	store: NavigationStore;
	nav_props: NavigateProps;
	target_url: string;
	snapshot: RuntimeRouteSnapshot;
	artifacts?: NavigationArtifacts;
	client_build_id: string;
	intent: string;
	should_commit_history: boolean;
	signal: AbortSignal;
	modules_override?: Map<string, Record<string, unknown>>;
	prestarted?: Record<string, Promise<unknown>>;
}): Promise<void> {
	const current = get_snapshot();
	const reuse = can_reuse_components(props.intent, current, props.snapshot);

	// launch module loading, client loaders, and CSS waiting
	// in parallel — they are independent of each other.
	const modules_promise: Promise<ComponentModulesMap | undefined> = reuse
		? Promise.resolve(undefined)
		: props.modules_override
			? Promise.resolve(props.modules_override)
			: load_modules(props.snapshot.import_urls);

	const cl_promise = complete_client_loaders({
		snapshot: props.snapshot,
		signal: props.signal,
		prestarted: props.prestarted,
	});

	const css_promise = wait_for_css(props.artifacts, props.signal);

	const [loaded_modules, cl] = await Promise.all([
		modules_promise,
		cl_promise,
		css_promise,
	]);

	const effective_error_idx = compute_effective_error_idx(
		props.snapshot.outermost_server_error_idx,
		cl.outermost_client_error_idx,
	);
	const effective_error =
		effective_error_idx === cl.outermost_client_error_idx
			? cl.outermost_client_error
			: props.snapshot.outermost_server_error;

	let active_components: unknown[];
	let active_error_boundary: unknown;
	if (reuse) {
		active_components = current.active_components;
		active_error_boundary = current.active_error_boundary;
	} else {
		const modules = loaded_modules!;
		active_components = build_active_components(props.snapshot, modules);
		active_error_boundary =
			effective_error_idx != null
				? resolve_error_boundary(
						props.snapshot,
						modules,
						effective_error_idx,
					)
				: undefined;
	}

	sync_client_build_id(props.client_build_id);

	const committed = set_snapshot({
		...props.snapshot,
		client_build_id: get_snapshot().client_build_id,
		active_components,
		active_error_boundary,
		client_loaders_data: cl.client_loaders_data,
		outermost_client_error: cl.outermost_client_error,
		outermost_client_error_idx: cl.outermost_client_error_idx,
		outermost_error:
			effective_error ?? props.snapshot.outermost_server_error,
		outermost_error_idx:
			effective_error_idx ?? props.snapshot.outermost_server_error_idx,
	});

	update_client_module_map(committed);

	const use_vt =
		props.intent === "navigate" &&
		get_global().use_view_transitions &&
		typeof (document as any).startViewTransition === "function";

	// Scroll state is computed inside run() (after history commit
	// updates the URL) but applied by the UI adapter's layout
	// effect so that hash targets created by new route components
	// exist in the DOM before we scroll.
	let pending_scroll: ScrollState | undefined;

	const run = () => {
		// commit history BEFORE applying head/title so that
		// the browser history entry captures the correct title.
		if (props.should_commit_history && props.intent === "navigate") {
			commit_history({
				target_url: props.target_url,
				replace: props.nav_props.replace,
			});
		}
		if (props.artifacts) {
			apply_head_and_title(props.artifacts);
			apply_css_bundles(props.artifacts);
		}
		const hash = new URL(props.target_url, window.location.href).hash;
		pending_scroll = props.nav_props.skip_history_commit
			? normalize_hash(hash).length > 0
				? { hash }
				: get_scroll_for_current_key_or_top()
			: hash
				? { hash }
				: props.nav_props.scroll_to_top === false
					? undefined
					: { x: 0, y: 0 };
		// Dispatch with __scrollState so the adapter's layout effect
		// can apply scroll after the DOM commit.
		dispatch_route_change({ __scrollState: pending_scroll });
	};

	if (use_vt) {
		const t = (document as any).startViewTransition(run);
		await t.finished;
	} else {
		run();
	}

	props.store.last_nav_or_revalidate_ts = Date.now();
}

// ─── Execute Navigation ─────────────────────────────────────────

async function execute_navigation(args: {
	store: NavigationStore;
	props: NavigateProps;
}): Promise<{ didNavigate: boolean }> {
	const { store } = args;
	const p = args.props;
	const intent = p.intent ?? "navigate";
	const target_url = make_absolute(p.href);
	assert_same_origin(target_url, "vormaNavigate(...)");
	const current_href =
		p.current_href_for_classification ?? window.location.href;
	const classification = classify_target(target_url, current_href);

	// Same-document noop
	if (intent !== "revalidate" && classification === "same-document-noop") {
		if (p.replace && !p.skip_history_commit) {
			commit_history({ target_url, replace: true });
		}
		return { didNavigate: false };
	}
	// Prefetch of hash-only change — skip
	if (
		intent === "prefetch" &&
		classification === "same-document-hash-change"
	) {
		return { didNavigate: false };
	}
	// Navigate hash-only change
	if (
		intent === "navigate" &&
		classification === "same-document-hash-change"
	) {
		if (!p.skip_history_commit) {
			commit_history({ target_url, replace: p.replace });
		}
		const hash = new URL(target_url, window.location.href).hash;
		const scroll: ScrollState =
			normalize_hash(hash).length > 0
				? { hash }
				: get_scroll_for_current_key_or_top();
		// Hash target already exists in the DOM — apply directly.
		apply_scroll_state(scroll);
		// Dispatch without __scrollState so adapters don't re-apply.
		dispatch_route_change({});
		return { didNavigate: true };
	}

	const data_key = get_target_data_key(target_url);

	// Coalesce with existing navigate to same target
	if (
		intent === "navigate" &&
		store.navigate_op &&
		get_target_data_key(store.navigate_op.target_url) === data_key
	) {
		await store.navigate_op.settled_promise;
		return execute_navigation(args);
	}

	// Coalesce revalidation
	if (
		intent === "revalidate" &&
		store.revalidate_op &&
		get_target_data_key(store.revalidate_op.target_url) === data_key
	) {
		if (!store.revalidate_op.allow_trailing_revalidate_pass) {
			await store.revalidate_op.settled_promise;
			return { didNavigate: false };
		}
		if (
			store.queued_revalidate_data_key !== data_key ||
			!store.queued_revalidate_promise
		) {
			store.queued_revalidate_data_key = data_key;
			store.queued_revalidate_promise =
				store.revalidate_op.settled_promise.then(async () => {
					if (store.queued_revalidate_data_key !== data_key) {
						return;
					}
					store.queued_revalidate_data_key = null;
					store.queued_revalidate_promise = null;
					await execute_navigation(args);
				});
		}
		await store.queued_revalidate_promise;
		return { didNavigate: false };
	}
	if (intent === "revalidate") {
		store.queued_revalidate_data_key = null;
		store.queued_revalidate_promise = null;
	}

	// Skip duplicate prefetch
	if (
		intent === "prefetch" &&
		(store.prefetch_ops.has(data_key) || store.prefetch_cache.has(data_key))
	) {
		return { didNavigate: false };
	}

	// Create operation
	let notify_settled = () => {};
	const settled_promise = new Promise<void>((r) => {
		notify_settled = r;
	});
	const op: NavigationOperation = {
		id: store.next_nav_op_id++,
		intent,
		target_url,
		allow_trailing_revalidate_pass: false,
		abort_controller: new AbortController(),
		settled_promise,
		notify_settled,
	};
	if (p.skip_global_loading_indicator) {
		store.skipped_loading_nav_ids.add(op.id);
	}

	// Slot the operation and abort superseded ones
	if (intent === "navigate") {
		store.navigate_op?.abort_controller.abort(
			encode_abort_reason(ABORT_REASON.superseded_nav),
		);
		store.revalidate_op?.abort_controller.abort(
			encode_abort_reason(ABORT_REASON.superseded_nav),
		);
		store.revalidate_op = null;
		store.queued_revalidate_data_key = null;
		store.queued_revalidate_promise = null;
		for (const [k, pf] of store.prefetch_ops) {
			if (k !== data_key) {
				pf.abort_controller.abort(
					encode_abort_reason(ABORT_REASON.superseded_nav),
				);
				store.prefetch_ops.delete(k);
			}
		}
		// clear stale prefetch cache entries for other targets
		for (const k of store.prefetch_cache.keys()) {
			if (k !== data_key) {
				store.prefetch_cache.delete(k);
			}
		}
		store.navigate_op = op;
	} else if (intent === "revalidate") {
		store.revalidate_op?.abort_controller.abort(
			encode_abort_reason(ABORT_REASON.superseded_revalidate),
		);
		store.queued_revalidate_data_key = null;
		store.queued_revalidate_promise = null;
		store.revalidate_op = op;
		queueMicrotask(() => {
			if (store.revalidate_op?.id === op.id) {
				op.allow_trailing_revalidate_pass = true;
			}
		});
	} else {
		store.prefetch_ops.set(data_key, op);
	}
	emit_status(store);

	// Prestarts are created lazily — only for fresh fetches, not cache hits.
	// For cache hits from prefetch, we reuse the prestarted loaders that
	// were stored when the prefetch completed.
	let prestarts: ClientLoaderPrestart[] = [];
	let prestarted_from_cache: Record<string, Promise<unknown>> | undefined;

	try {
		// Wait for any existing prefetch to this target
		if (intent !== "prefetch") {
			const existing_pf = store.prefetch_ops.get(data_key);
			if (existing_pf) {
				await existing_pf.settled_promise;
				if (!is_active(store, op)) {
					return { didNavigate: false };
				}
			}
		}

		// Fetch route data: cache > zero-server-loader skip > server fetch
		let result:
			| {
					status: "ready";
					snapshot: RuntimeRouteSnapshot;
					client_build_id: string;
					artifacts?: NavigationArtifacts;
					modules?: Map<string, Record<string, unknown>>;
			  }
			| {
					status: "redirected";
					href: string;
					client_build_id: string;
					is_hard_reload: boolean;
			  };

		const cached =
			intent !== "prefetch"
				? store.prefetch_cache.get(data_key)
				: undefined;
		if (cached) {
			result = {
				status: "ready",
				snapshot: cached.route_data,
				client_build_id: cached.client_build_id,
				artifacts: cached.artifacts,
				modules: cached.modules_map,
			};
			prestarted_from_cache = cached.prestarted_loaders;
			store.prefetch_cache.delete(data_key);
		} else {
			const skip_result = try_skip_server_fetch(target_url);
			if (skip_result) {
				// No artifacts — skip path preserves existing DOM head/CSS
				result = {
					status: "ready",
					snapshot: skip_result.snapshot,
					client_build_id: skip_result.snapshot.client_build_id,
				};
			} else {
				// No cache, no skip — create fresh prestarts and fetch from server
				prestarts = create_prestarts({
					target_url,
					signal: op.abort_controller.signal,
				});

				const fetched = await fetch_route_data({
					target_url,
					signal: op.abort_controller.signal,
				});
				result =
					fetched.status === "redirect"
						? {
								status: "redirected",
								href: fetched.href,
								client_build_id: fetched.client_build_id,
								is_hard_reload: fetched.is_hard_reload,
							}
						: {
								status: "ready",
								snapshot: fetched.route_snapshot,
								client_build_id: fetched.client_build_id,
								artifacts: fetched.artifacts,
							};
			}
		}

		if (!is_active(store, op)) {
			for (const ps of prestarts) {
				ps.abort_if_pending();
			}
			return { didNavigate: false };
		}

		// Check revalidation ownership
		if (
			intent === "revalidate" &&
			get_target_data_key(window.location.href) !==
				get_target_data_key(target_url)
		) {
			for (const ps of prestarts) {
				ps.abort_if_pending();
			}
			return { didNavigate: false };
		}

		if (result.status === "redirected") {
			for (const ps of prestarts) {
				ps.abort_if_pending();
			}
			if (intent === "prefetch") {
				return { didNavigate: false };
			}
			sync_client_build_id(result.client_build_id);
			return follow_redirect({
				href: result.href,
				client_build_id: result.client_build_id,
				is_hard_reload: result.is_hard_reload,
				store,
				hop_count: p.redirect_hop_count,
				skip_loading: p.skip_global_loading_indicator,
			});
		}

		// Prefetch: store result (including prestarted loaders), don't render
		if (intent === "prefetch") {
			for (const ps of prestarts) {
				ps.resolve_from_snapshot(result.snapshot);
			}
			sync_client_build_id(result.client_build_id);
			store.prefetch_cache.set(data_key, {
				target_data_key: data_key,
				target_url,
				client_build_id: result.client_build_id,
				route_data: result.snapshot,
				artifacts: result.artifacts,
				modules_map: result.modules,
				prestarted_loaders:
					prestarts.length > 0
						? Object.fromEntries(
								prestarts.map((ps) => [
									ps.matched_pattern,
									ps.result_promise,
								]),
							)
						: undefined,
			});
			return { didNavigate: false };
		}

		// Navigate or revalidate: commit
		for (const ps of prestarts) {
			ps.resolve_from_snapshot(result.snapshot);
		}
		const effective_prestarted =
			prestarted_from_cache ??
			(prestarts.length > 0
				? Object.fromEntries(
						prestarts.map((ps) => [
							ps.matched_pattern,
							ps.result_promise,
						]),
					)
				: undefined);

		await commit_navigation({
			store,
			nav_props: p,
			target_url,
			snapshot: result.snapshot,
			artifacts: result.artifacts,
			client_build_id: result.client_build_id,
			intent,
			should_commit_history: !p.skip_history_commit,
			signal: op.abort_controller.signal,
			modules_override: result.modules,
			prestarted: effective_prestarted,
		});
		return { didNavigate: intent === "navigate" };
	} catch (err) {
		for (const ps of prestarts) {
			ps.abort_if_pending();
		}
		if (is_abort_error(err)) {
			return { didNavigate: false };
		}
		throw err;
	} finally {
		for (const ps of prestarts) {
			ps.abort_if_pending();
		}
		clear_op(store, op);
		emit_status(store);
		op.notify_settled();
	}
}

// ─── Execute Submit ─────────────────────────────────────────────

async function execute_submit<T>(props: {
	store: NavigationStore;
	url: string | URL;
	request_init?: RequestInit;
	options?: SubmitOptions;
}): Promise<SubmitResult<T>> {
	const store = props.store;
	const g = get_global();
	const sub_id = store.next_sub_op_id++;
	store.latest_started_sub_op_id = sub_id;
	if (props.options?.skipGlobalLoadingIndicator) {
		store.skipped_loading_sub_ids.add(sub_id);
	}
	const ac = new AbortController();
	const dedupe = props.options?.dedupeKey;

	if (dedupe) {
		const prev = store.sub_id_by_dedupe_key.get(dedupe);
		if (prev !== undefined) {
			store.sub_abort_controllers
				.get(prev)
				?.abort(encode_abort_reason(ABORT_REASON.superseded_dedupe));
		}
		store.sub_id_by_dedupe_key.set(dedupe, sub_id);
	}
	store.active_sub_ids.add(sub_id);
	store.sub_abort_controllers.set(sub_id, ac);
	emit_status(store);

	try {
		const submit_url = new URL(make_absolute(props.url));
		assert_same_origin(submit_url.href, "submit(...)");
		const headers = new Headers(props.request_init?.headers ?? undefined);
		headers.set("X-Accepts-Client-Redirect", "1");
		if (g.deployment_id) {
			headers.set("x-deployment-id", g.deployment_id);
		}
		const method = (props.request_init?.method ?? "GET").toUpperCase();
		const is_get = method === "GET" || method === "HEAD";
		const body = props.request_init?.body;
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
			...props.request_init,
			method,
			headers,
			signal: ac.signal,
		};
		if (is_get) {
			delete final_init.body;
		} else if (should_json) {
			final_init.body = JSON.stringify(body);
			if (!headers.has("content-type")) {
				headers.set("content-type", "application/json");
			}
		}

		const response = await window.fetch(submit_url, final_init);

		if (dedupe && store.sub_id_by_dedupe_key.get(dedupe) !== sub_id) {
			return { success: false, error: "Aborted" };
		}

		const is_latest = store.latest_started_sub_op_id === sub_id;
		const bid = response.headers.get("X-Vorma-Client-Build-Id") ?? "";
		const hard = response.headers.get("X-Vorma-Reload");
		const client_redir = response.headers.get("X-Client-Redirect");
		const native_redir =
			response.redirected && response.url
				? (() => {
						const r = new URL(response.url, submit_url.href).href;
						return r !== submit_url.href ? r : undefined;
					})()
				: undefined;
		const redir = hard ?? client_redir ?? native_redir;

		// sync build ID on ALL submit responses
		sync_client_build_id(bid);

		if (redir && is_latest) {
			try {
				await follow_redirect({
					href: hard
						? new URL(hard, submit_url.href).href
						: client_redir
							? new URL(client_redir, submit_url.href).href
							: native_redir!,
					client_build_id: bid,
					is_hard_reload: !!hard,
					store,
				});
			} catch {
				return { success: false, error: "Redirect failed" };
			}
			return { success: true, data: undefined as T };
		}
		if (redir && !is_latest) {
			return { success: true, data: undefined as T };
		}

		if (!response.ok) {
			const t = (await response.text()).trim();
			return {
				success: false,
				error:
					t && t !== response.statusText
						? t
						: String(response.status),
			};
		}

		let data: unknown;
		const ct = response.headers.get("content-type");
		if (response.status !== 204) {
			if (ct?.toLowerCase().includes("json")) {
				data = await response.json();
			} else {
				const t = await response.text();
				data = t.length > 0 ? t : undefined;
			}
		}

		if (dedupe && store.sub_id_by_dedupe_key.get(dedupe) !== sub_id) {
			return { success: false, error: "Aborted" };
		}

		const should_rev =
			props.options?.revalidate !== undefined
				? props.options.revalidate
				: !is_get;
		if (should_rev) {
			const { revalidate } = await import("./public_api.ts");
			await revalidate();
		}
		return { success: true, data: data as T };
	} catch (err) {
		if ((err as any)?.message?.includes("only supports same-origin")) {
			throw err;
		}
		if (is_abort_error(err)) {
			return { success: false, error: "Aborted" };
		}
		return {
			success: false,
			error: String((err as any)?.message ?? err),
		};
	} finally {
		store.active_sub_ids.delete(sub_id);
		store.sub_abort_controllers.delete(sub_id);
		store.skipped_loading_sub_ids.delete(sub_id);
		if (dedupe && store.sub_id_by_dedupe_key.get(dedupe) === sub_id) {
			store.sub_id_by_dedupe_key.delete(dedupe);
		}
		emit_status(store);
	}
}

// ─── Create Navigation State Manager ────────────────────────────

export function create_nav_manager(): NavigationStateManager {
	const store = create_store();
	return {
		navigate: (p) => execute_navigation({ store, props: p }),
		submit: (url, ri, opts) =>
			execute_submit({ store, url, request_init: ri, options: opts }),
		getStatus: () => get_loading_status(store),
		clearAll: () => {
			store.navigate_op?.abort_controller.abort(
				encode_abort_reason(ABORT_REASON.clear_all),
			);
			store.revalidate_op?.abort_controller.abort(
				encode_abort_reason(ABORT_REASON.clear_all),
			);
			for (const op of store.prefetch_ops.values()) {
				op.abort_controller.abort(
					encode_abort_reason(ABORT_REASON.clear_all),
				);
			}
			for (const ac of store.sub_abort_controllers.values()) {
				ac.abort(encode_abort_reason(ABORT_REASON.clear_all));
			}
			store.prefetch_ops.clear();
			store.prefetch_cache.clear();
			store.navigate_op = null;
			store.revalidate_op = null;
			store.queued_revalidate_data_key = null;
			store.queued_revalidate_promise = null;
			store.sub_abort_controllers.clear();
			store.sub_id_by_dedupe_key.clear();
			store.skipped_loading_nav_ids.clear();
			store.skipped_loading_sub_ids.clear();
			store.active_sub_ids.clear();
			store.latest_started_sub_op_id = 0;
			emit_status(store);
		},
		get_store: () => store,
	};
}
