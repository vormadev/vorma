/// <reference types="vite/client" />

import { serializeToSearchParams } from "vorma/kit/json";
import { stripTrailingSlash } from "vorma/kit/matcher/utils";
import {
	getHrefDetails,
	resolveAbsoluteHref,
	resolveAbsoluteHrefWithOptionalSearchAndHash,
} from "vorma/kit/url";
import { normalize_hash } from "./scroll.ts";
import type { VormaAppConfig } from "./types.ts";

// ─── Path Resolution ─────────────────────────────────────────────

function strip_trailing_preserve_root(path: string): string {
	return path === "/" ? path : stripTrailingSlash(path);
}

function resolve_pattern_path(props: {
	pattern: string;
	params?: Record<string, string>;
	splat_values?: string[];
	dynamic_rune: string;
	splat_rune: string;
	index_segment: string;
}): string {
	const params = props.params ?? {};
	const normalized =
		props.pattern === `/${props.index_segment}` ? "/" : props.pattern;
	const segments = normalized
		.split("/")
		.filter((s) => s.length > 0)
		.filter((s) => s !== props.index_segment);
	const splat_encoded = (props.splat_values ?? [])
		.map((v) => encodeURIComponent(v))
		.join("/");
	const path_segments = segments.flatMap((seg) => {
		if (seg === props.splat_rune)
			return splat_encoded.split("/").filter((p) => p.length > 0);
		if (seg.startsWith(props.dynamic_rune))
			return [
				encodeURIComponent(
					params[seg.slice(props.dynamic_rune.length)] as string,
				),
			];
		return [seg];
	});
	const result = "/" + path_segments.join("/");
	return strip_trailing_preserve_root(result.length === 0 ? "/" : result);
}

export type ResolvePathInput = {
	type: "query" | "mutation" | "loader";
	app_config: VormaAppConfig;
	pattern: string;
	params?: Record<string, string>;
	splat_values?: string[];
};

export function resolve_path(input: ResolvePathInput): string {
	const is_loader = input.type === "loader";
	return resolve_pattern_path({
		pattern: input.pattern,
		params: input.params,
		splat_values: input.splat_values,
		dynamic_rune: is_loader
			? input.app_config.loadersDynamicRune
			: input.app_config.actionsDynamicRune,
		splat_rune: is_loader
			? input.app_config.loadersSplatRune
			: input.app_config.actionsSplatRune,
		index_segment: input.app_config.loadersExplicitIndexSegmentIdentifier,
	});
}

// ─── Typed Link Href Builder (shared by all UI adapters) ─────────

export function build_typed_link_href(props: {
	app_config: VormaAppConfig;
	pattern: string;
	params?: Record<string, string>;
	splat_values?: string[];
	search?: string;
	hash?: string;
}): string {
	const base = resolve_path({
		type: "loader",
		app_config: props.app_config,
		pattern: props.pattern,
		params: props.params,
		splat_values: props.splat_values,
	});
	const url = new URL(base, window.location.origin);
	if (props.search !== undefined) url.search = props.search;
	if (props.hash !== undefined) url.hash = props.hash;
	return url.href;
}

// ─── URL Building ────────────────────────────────────────────────

function build_url(
	input: ResolvePathInput & { search?: string; hash?: string },
): URL {
	const pathname = resolve_path(input);
	const full =
		input.type === "loader"
			? pathname
			: strip_trailing_preserve_root(
					input.app_config.actionsRouterMountRoot,
				) + (pathname === "/" ? "" : pathname);
	const url = new URL(full, window.location.origin);
	if (input.search !== undefined) url.search = input.search;
	if (input.hash !== undefined) url.hash = input.hash;
	return url;
}

export function build_query_url(
	cfg: VormaAppConfig,
	props: {
		pattern: string;
		params?: Record<string, string>;
		splat_values?: string[];
		input?: unknown;
		search?: string;
		hash?: string;
	},
): URL {
	const url = build_url({
		type: "query",
		app_config: cfg,
		pattern: props.pattern,
		params: props.params,
		splat_values: props.splat_values,
	});
	if (props.input && typeof props.input === "object")
		url.search = serializeToSearchParams(props.input).toString();
	return url;
}

export function build_mutation_url(
	cfg: VormaAppConfig,
	props: {
		pattern: string;
		params?: Record<string, string>;
		splat_values?: string[];
		search?: string;
		hash?: string;
	},
): URL {
	return build_url({
		type: "mutation",
		app_config: cfg,
		pattern: props.pattern,
		params: props.params,
		splat_values: props.splat_values,
	});
}

export function resolve_body(props: {
	input?: unknown;
}): BodyInit | null | undefined {
	const i = props.input;
	if (
		i === undefined ||
		i === null ||
		typeof i === "string" ||
		i instanceof ReadableStream ||
		i instanceof FormData ||
		i instanceof URLSearchParams ||
		i instanceof Blob ||
		i instanceof ArrayBuffer
	)
		return i;
	if (ArrayBuffer.isView(i)) return i as ArrayBufferView<ArrayBuffer>;
	return JSON.stringify(i);
}

// ─── Navigation Classification ──────────────────────────────────

export type NavigationClassification =
	| "same-document-noop"
	| "same-document-hash-change"
	| "requires-fetch";

function href_without_hash(href: string): string {
	const u = new URL(href, window.location.href);
	u.hash = "";
	return u.href;
}

export function classify_target(
	target_href: string,
	current_href: string,
): NavigationClassification {
	const t = new URL(target_href, current_href);
	const c = new URL(current_href);
	if (href_without_hash(t.href) !== href_without_hash(c.href))
		return "requires-fetch";
	return normalize_hash(t.hash) === normalize_hash(c.hash)
		? "same-document-noop"
		: "same-document-hash-change";
}

export function is_same_origin(href: string): boolean {
	return (
		new URL(href, window.location.href).origin === window.location.origin
	);
}

export function assert_same_origin(href: string, api: string): void {
	if (!is_same_origin(href))
		throw new Error(
			`${api} only supports same-origin targets. Received: "${href}".`,
		);
}

export function make_absolute(href: string | URL): string {
	return resolveAbsoluteHref({ href });
}

export function make_absolute_with_search_hash(props: {
	href: string;
	search?: string;
	hash?: string;
}): string {
	return resolveAbsoluteHrefWithOptionalSearchAndHash(props);
}

export function get_target_data_key(href: string): string {
	const u = new URL(href, window.location.href);
	return `${u.pathname}${u.search}`;
}

export { getHrefDetails };
