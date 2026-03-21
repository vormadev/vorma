/// <reference types="vite/client" />

import {
	createPatternRegistry,
	registerPattern,
} from "vorma/kit/matcher/register";
import { complete_client_loaders } from "./client_loaders.ts";
import {
	build_active_components,
	compute_effective_error_idx,
	load_modules,
	resolve_error_boundary,
} from "./components.ts";
import { dispatch_route_change, dispatch_status } from "./events.ts";
import { ensure_global, get_snapshot, set_snapshot } from "./global_state.ts";
import {
	init_popstate_listener,
	register_beforeunload_once,
	set_manual_scroll_restoration,
} from "./history.ts";
import { register_input_modality_listeners } from "./input_modality.ts";
import { create_nav_manager } from "./navigation.ts";
import { getStatus } from "./public_api.ts";
import { restore_page_refresh_scroll } from "./scroll.ts";
import type { InitClientInput } from "./types.ts";
import { classify_target } from "./url.ts";

export async function initClient(options: InitClientInput): Promise<void> {
	const g = ensure_global();

	const current = new URL(window.location.href);
	if (current.searchParams.has("vorma_reload")) {
		current.searchParams.delete("vorma_reload");
		window.history.replaceState(
			window.history.state,
			"",
			`${current.pathname}${current.search}${current.hash}`,
		);
	}

	if (options.isDev !== undefined) {
		g.is_dev = options.isDev;
	}
	if (options.publicPathPrefix !== undefined) {
		g.public_path_prefix = options.publicPathPrefix;
	}
	if (options.defaultErrorBoundary) {
		g.default_error_boundary = options.defaultErrorBoundary;
	}
	if (options.useViewTransitions !== undefined) {
		g.use_view_transitions = options.useViewTransitions;
	}
	if (options.vormaAppConfig) {
		g.app_config = options.vormaAppConfig;
	}
	if (options.routeManifestURL !== undefined) {
		g.route_manifest_url = options.routeManifestURL;
	}
	if (options.rootElementID !== undefined) {
		set_snapshot({
			...get_snapshot(),
			root_element_id: options.rootElementID,
		});
	}

	const registry = createPatternRegistry({
		dynamicParamPrefixRune: g.app_config.loadersDynamicRune,
		splatSegmentRune: g.app_config.loadersSplatRune,
		explicitIndexSegment:
			g.app_config.loadersExplicitIndexSegmentIdentifier,
	});
	for (const p of Object.keys(g.pattern_to_wait_fn)) {
		registerPattern(registry, p);
	}
	g.pattern_registry = registry;

	const initial_snapshot = g.snapshot;
	for (let i = 0; i < initial_snapshot.matched_patterns.length; i++) {
		const pattern = initial_snapshot.matched_patterns[i];
		const import_url = initial_snapshot.import_urls[i];
		if (pattern && import_url) {
			g.client_module_map[pattern] = {
				import_url,
				export_key: initial_snapshot.export_keys[i] ?? "default",
				error_export_key: initial_snapshot.error_export_keys[i] ?? "",
			};
		}
	}

	// Route manifest (non-blocking — progressive enhancement,
	// only needed from first client-side navigation onward)
	void load_route_manifest(g);

	register_input_modality_listeners();

	if (import.meta.env.DEV) {
		void import("./_hmr_dev.ts")
			.then(({ register_vite_listener }) =>
				register_vite_listener(import.meta.hot),
			)
			.catch(() => undefined);
	}

	if (!g.nav_state_manager) {
		g.nav_state_manager = create_nav_manager();
	}

	init_popstate_listener();
	set_manual_scroll_restoration();
	register_beforeunload_once();

	// run module loading and client loader execution in parallel.
	const snapshot = g.snapshot;
	const [modules, cl] = await Promise.all([
		load_modules(snapshot.import_urls),
		complete_client_loaders({
			snapshot,
			signal: new AbortController().signal,
		}),
	]);

	const active_components = build_active_components(snapshot, modules);

	// use shared compute_effective_error_idx.
	// only call resolve_error_boundary when there is an error.
	const effective_error_idx = compute_effective_error_idx(
		snapshot.outermost_server_error_idx,
		cl.outermost_client_error_idx,
	);
	const active_error_boundary =
		effective_error_idx != null
			? resolve_error_boundary(snapshot, modules, effective_error_idx)
			: undefined;

	const effective_error =
		effective_error_idx === cl.outermost_client_error_idx
			? cl.outermost_client_error
			: snapshot.outermost_server_error;

	set_snapshot({
		...snapshot,
		active_components,
		active_error_boundary,
		client_loaders_data: cl.client_loaders_data,
		outermost_client_error: cl.outermost_client_error,
		outermost_client_error_idx: cl.outermost_client_error_idx,
		outermost_error: effective_error ?? snapshot.outermost_server_error,
		outermost_error_idx:
			effective_error_idx ?? snapshot.outermost_server_error_idx,
	});

	if (options.renderFn) {
		await options.renderFn();
	}

	if (import.meta.env.DEV) {
		(window as any).__waveRevalidate = (
			await import("./public_api.ts")
		).revalidate;
	}

	dispatch_route_change({});
	dispatch_status(getStatus());

	restore_page_refresh_scroll(
		(href) =>
			classify_target(href, window.location.href) ===
			"same-document-noop",
	);
}

async function load_route_manifest(
	g: ReturnType<typeof ensure_global>,
): Promise<void> {
	if (g.route_manifest) {
		for (const p of Object.keys(g.route_manifest)) {
			registerPattern(g.pattern_registry, p);
		}
		return;
	}
	const url = g.route_manifest_url;
	if (!url) {
		return;
	}
	const load_id = ++g.next_manifest_load_id;
	const initial_registry = g.pattern_registry;
	try {
		const res = await window.fetch(url, { method: "GET" });
		if (!res.ok) {
			console.warn("Failed to load route manifest:", res.status);
			return;
		}
		const manifest = (await res.json()) as Record<string, unknown>;
		if (
			g.next_manifest_load_id !== load_id ||
			g.pattern_registry !== initial_registry
		) {
			return;
		}
		for (const p of Object.keys(manifest)) {
			registerPattern(g.pattern_registry, p);
		}
		g.route_manifest = manifest;
	} catch (err) {
		console.warn("Failed to load route manifest:", err);
	}
}
