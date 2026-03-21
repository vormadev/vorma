/// <reference types="vite/client" />

import { panic } from "./abort.ts";
import { get_global, get_snapshot } from "./global_state.ts";
import type {
	NavigationArtifacts,
	RouteDataFetchResult,
	RouteDataPayload,
	RuntimeRouteSnapshot,
} from "./types.ts";

// ─── Wire Format Decode ──────────────────────────────────────────

function decode_html_to_text(html: string | undefined): string {
	if (!html) {
		return "";
	}
	const el = document.createElement("textarea");
	el.innerHTML = html;
	return el.value;
}

type DecodedRouteData = {
	snapshot: RuntimeRouteSnapshot;
	artifacts: NavigationArtifacts;
	deps: string[];
};

function decode_payload(payload: RouteDataPayload): DecodedRouteData {
	const prev = get_snapshot();

	const decoded_title = decode_html_to_text(
		payload.title?.dangerousInnerHTML,
	);

	return {
		snapshot: {
			outermost_server_error: payload.outermostServerError,
			outermost_server_error_idx: payload.outermostServerErrorIdx,
			outermost_client_error: undefined,
			outermost_client_error_idx: undefined,
			outermost_error: payload.outermostServerError,
			outermost_error_idx: payload.outermostServerErrorIdx,
			matched_patterns: payload.matchedPatterns ?? [],
			loaders_data: payload.loadersData ?? [],
			import_urls: payload.importURLs ?? [],
			export_keys: payload.exportKeys ?? [],
			error_export_keys: payload.errorExportKeys ?? [],
			has_root_data: payload.hasRootData ?? false,
			params: payload.params ?? {},
			splat_values: payload.splatValues ?? [],
			client_build_id: prev.client_build_id,
			active_components: [],
			active_error_boundary: undefined,
			client_loaders_data: [],
		},
		artifacts: {
			title: decoded_title,
			meta_head_els: payload.metaHeadEls ?? [],
			rest_head_els: payload.restHeadEls ?? [],
			css_bundles: payload.cssBundles ?? [],
		},
		deps: payload.deps ?? [],
	};
}

// ─── Asset Preloading ────────────────────────────────────────────

function preload_modules(deps: string[]): void {
	if (import.meta.env.DEV) {
		return;
	}
	for (const dep of new Set(deps)) {
		if (
			document.head.querySelector(
				`link[rel="modulepreload"][href="${dep}"]`,
			)
		) {
			continue;
		}
		const link = document.createElement("link");
		link.rel = "modulepreload";
		link.href = dep;
		document.head.appendChild(link);
	}
}

function preload_css(css_bundles: string[]): void {
	for (const path of new Set(css_bundles)) {
		if (
			document.head.querySelector(
				`link[data-vorma-css-preload-bundle="${path}"]`,
			)
		) {
			continue;
		}
		const link = document.createElement("link");
		link.rel = "preload";
		link.setAttribute("as", "style");
		link.setAttribute("data-vorma-css-preload-bundle", path);
		link.setAttribute("data-vorma-css-preload-managed", "1");
		link.setAttribute("data-vorma-css-preload-settled", "0");
		const mark = () =>
			link.setAttribute("data-vorma-css-preload-settled", "1");
		link.addEventListener("load", mark);
		link.addEventListener("error", mark);
		link.href = path;
		document.head.appendChild(link);
	}
}

export async function wait_for_css(
	artifacts: NavigationArtifacts | undefined,
	signal: AbortSignal,
): Promise<void> {
	if (!artifacts) {
		return;
	}
	const promises: Promise<void>[] = [];
	for (const path of new Set(artifacts.css_bundles)) {
		const node = document.head.querySelector<HTMLLinkElement>(
			`link[data-vorma-css-preload-bundle="${path}"][data-vorma-css-preload-managed="1"]`,
		);
		if (
			!node ||
			node.getAttribute("data-vorma-css-preload-settled") === "1"
		) {
			continue;
		}
		promises.push(
			new Promise<void>((resolve) => {
				if (signal.aborted) {
					resolve();
					return;
				}
				const done = () => {
					node.setAttribute("data-vorma-css-preload-settled", "1");
					cleanup();
					resolve();
				};
				const on_abort = () => {
					cleanup();
					resolve();
				};
				const cleanup = () => {
					node.removeEventListener("load", done);
					node.removeEventListener("error", done);
					signal.removeEventListener("abort", on_abort);
				};
				node.addEventListener("load", done, { once: true });
				node.addEventListener("error", done, { once: true });
				signal.addEventListener("abort", on_abort, { once: true });
			}),
		);
	}
	if (promises.length > 0) {
		await Promise.all(promises);
	}
}

export function apply_css_bundles(
	artifacts: NavigationArtifacts | undefined,
): void {
	if (!artifacts) {
		return;
	}
	for (const path of new Set(artifacts.css_bundles)) {
		if (
			document.head.querySelector(`link[data-vorma-css-bundle="${path}"]`)
		) {
			continue;
		}
		const link = document.createElement("link");
		link.rel = "stylesheet";
		link.setAttribute("data-vorma-css-bundle", path);
		link.href = path;
		document.head.appendChild(link);
	}
}

// ─── Fetch Route Data ────────────────────────────────────────────

export async function fetch_route_data(props: {
	target_url: string;
	signal: AbortSignal;
}): Promise<RouteDataFetchResult> {
	const g = get_global();
	const request_url = new URL(props.target_url);
	request_url.searchParams.set("vorma_json", get_snapshot().client_build_id);
	const headers = new Headers();
	headers.set("X-Accepts-Client-Redirect", "1");
	if (g.deployment_id) {
		request_url.searchParams.set("dpl", g.deployment_id);
	}

	const response = await window.fetch(request_url, {
		method: "GET",
		headers,
		signal: props.signal,
	});
	const client_build_id =
		response.headers.get("X-Vorma-Client-Build-Id") ?? "";

	const hard_reload = response.headers.get("X-Vorma-Reload");
	if (hard_reload) {
		return {
			status: "redirect",
			href: new URL(hard_reload, request_url.href).href,
			client_build_id,
			is_hard_reload: true,
		};
	}

	const client_redirect = response.headers.get("X-Client-Redirect");
	if (client_redirect) {
		return {
			status: "redirect",
			href: new URL(client_redirect, request_url.href).href,
			client_build_id,
			is_hard_reload: false,
		};
	}

	if (response.redirected && response.url) {
		const r = new URL(response.url, request_url.href).href;
		if (r !== request_url.href) {
			return {
				status: "redirect",
				href: r,
				client_build_id,
				is_hard_reload: false,
			};
		}
	}

	if (!response.ok) {
		const body = await response.text();
		panic(`Navigation request failed (${response.status}): ${body}.`);
	}
	const json: unknown = await response.json();
	if (json === null || typeof json !== "object" || Array.isArray(json)) {
		panic(
			"Route data JSON contract violated: response must be a JSON object.",
		);
	}

	const { snapshot, artifacts, deps } = decode_payload(
		json as RouteDataPayload,
	);

	preload_modules(deps);
	preload_css(artifacts.css_bundles);

	return {
		status: "success",
		route_snapshot: { ...snapshot, client_build_id },
		artifacts,
		client_build_id,
	};
}

export function make_hard_reload_href(
	target: string,
	client_build_id: string,
): string {
	const url = new URL(target, window.location.href);
	url.searchParams.set("vorma_reload", client_build_id);
	return url.href;
}
