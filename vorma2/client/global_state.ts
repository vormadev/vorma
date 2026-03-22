/// <reference types="vite/client" />

import { createPatternRegistry } from "vorma/kit/matcher/register";
import { default_error_boundary } from "./error_boundary.ts";
import type {
	RuntimeRouteSnapshot,
	VormaAppConfig,
	VormaClientGlobal,
} from "./types.ts";

export const VORMA_SYMBOL = Symbol.for("__vorma_internal__");

const DEFAULT_APP_CONFIG: VormaAppConfig = {
	actionsRouterMountRoot: "/api/",
	actionsDynamicRune: ":",
	actionsSplatRune: "*",
	loadersDynamicRune: ":",
	loadersSplatRune: "*",
	loadersExplicitIndexSegmentIdentifier: "_index",
};

function create_default_snapshot(): RuntimeRouteSnapshot {
	return {
		outermost_server_error: undefined,
		outermost_server_error_idx: undefined,
		outermost_client_error: undefined,
		outermost_client_error_idx: undefined,
		outermost_error: undefined,
		outermost_error_idx: undefined,
		matched_patterns: [],
		loaders_data: [],
		import_urls: [],
		export_keys: [],
		error_export_keys: [],
		has_root_data: false,
		params: {},
		splat_values: [],
		client_build_id: "1",
		active_components: [],
		active_error_boundary: undefined,
		client_loaders_data: [],
	};
}

function create_default_global(): VormaClientGlobal {
	return {
		is_dev: false,
		public_path_prefix: "",
		is_touch_active: false,
		has_registered_input_modality_listeners: false,
		next_manifest_load_id: 0,
		route_manifest: undefined,
		pattern_to_wait_fn: {},
		default_error_boundary,
		use_view_transitions: false,
		deployment_id: "",
		app_config: DEFAULT_APP_CONFIG,
		route_manifest_url: "",
		pattern_registry: createPatternRegistry({
			dynamicParamPrefixRune: DEFAULT_APP_CONFIG.loadersDynamicRune,
			splatSegmentRune: DEFAULT_APP_CONFIG.loadersSplatRune,
			explicitIndexSegment:
				DEFAULT_APP_CONFIG.loadersExplicitIndexSegmentIdentifier,
		}),
		client_module_map: {},
		snapshot: create_default_snapshot(),
		nav_state_manager: undefined,
		popstate_registered: false,
		last_known_history_key: undefined,
		last_known_history_href: undefined,
		has_registered_beforeunload: false,
		window_listeners: new Map(),
		hard_redirect_for_testing: undefined,
	};
}

export function ensure_global(): VormaClientGlobal {
	const existing = (globalThis as any)[VORMA_SYMBOL];
	if (!existing) {
		const next = create_default_global();
		(globalThis as any)[VORMA_SYMBOL] = next;
		return next;
	}
	if (existing.__initialized) return existing as VormaClientGlobal;
	const merged = {
		...create_default_global(),
		...existing,
		__initialized: true,
	};
	(globalThis as any)[VORMA_SYMBOL] = merged;
	return merged as VormaClientGlobal;
}

export function get_global(): VormaClientGlobal {
	const g = (globalThis as any)[VORMA_SYMBOL] as
		| VormaClientGlobal
		| undefined;
	if (!g) throw new Error("Vorma client runtime is not initialized.");
	return g;
}

export function get_snapshot(): RuntimeRouteSnapshot {
	return ensure_global().snapshot;
}

export function set_snapshot(next: RuntimeRouteSnapshot): RuntimeRouteSnapshot {
	get_global().snapshot = next;
	return next;
}

export function patch_snapshot(
	patch: Partial<RuntimeRouteSnapshot>,
): RuntimeRouteSnapshot {
	return set_snapshot({
		...get_snapshot(),
		...patch,
	} as RuntimeRouteSnapshot);
}
