/// <reference types="vite/client" />

import { get_snapshot } from "./global_state.ts";
import { get_manager } from "./navigation.ts";
import type {
	BaseRouterData,
	ParamsForPattern,
	StatusEventDetail,
	SubmitOptions,
	SubmitResult,
	VormaAppBase,
	VormaLoaderPattern,
	VormaNavigateOptions,
	VormaRoutePropsGeneric,
} from "./types.ts";

// ─── Public Navigation API ───────────────────────────────────────
// Accepts camelCase options (public contract), maps to internal
// snake_case NavigateProps.

export async function vormaNavigate(
	href: string | URL,
	options: VormaNavigateOptions = {},
): Promise<{ didNavigate: boolean }> {
	return get_manager().navigate({
		href,
		replace: options.replace,
		scroll_to_top: options.scrollToTop,
		intent: options.intent ?? "navigate",
		skip_global_loading_indicator: options.skipGlobalLoadingIndicator,
	});
}

export async function revalidate(): Promise<{ didNavigate: boolean }> {
	return get_manager().navigate({
		href: window.location.href,
		intent: "revalidate",
		replace: true,
		scroll_to_top: false,
	});
}

export async function submit<T = unknown>(
	url: string | URL,
	requestInit?: RequestInit,
	options?: SubmitOptions,
): Promise<SubmitResult<T>> {
	return get_manager().submit<T>(url, requestInit, options);
}

export function getStatus(): StatusEventDetail {
	return get_manager().getStatus();
}

export function getClientBuildID(): string {
	return get_snapshot().client_build_id;
}

const ROOT_ELEMENT_ID = "vorma-root";

export function getRootEl(): HTMLElement {
	const el = document.getElementById(ROOT_ELEMENT_ID);
	if (!el) {
		throw new Error(
			`Expected element with id "${ROOT_ELEMENT_ID}" to exist`,
		);
	}
	return el;
}

export function getRouterData<
	App extends VormaAppBase = VormaAppBase,
	P extends VormaLoaderPattern<App> = VormaLoaderPattern<App>,
>(
	_routeProps?: VormaRoutePropsGeneric<unknown, App, P>,
): BaseRouterData<App["rootData"], ParamsForPattern<App, P>> {
	const s = get_snapshot();
	return {
		clientBuildID: s.client_build_id,
		matchedPatterns: s.matched_patterns,
		splatValues: s.splat_values,
		params: s.params as Record<ParamsForPattern<App, P>, string>,
		rootData: (s.has_root_data
			? s.loaders_data[0]
			: null) as App["rootData"],
	};
}
