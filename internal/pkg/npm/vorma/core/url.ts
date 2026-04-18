import { serializeToSearchParams } from "vorma/kit/json";
import type {
	AppConfig,
	MakeTypedLoaderPattern,
	MakeTypedNavProps,
	MakeTypedNavTarget,
	MakeTypedRouteDestination,
} from "./types.ts";

const DYNAMIC_RUNE = ":";
const SPLAT_RUNE = "*";
const INDEX_SEGMENT = "_index";

function strip_trailing_slash(path: string): string {
	if (path === "/") {
		return path;
	}
	return path.endsWith("/") ? path.slice(0, -1) : path;
}

function resolve_pattern_path(
	pattern: string,
	params?: Record<string, string>,
	splat_values?: string[],
): string {
	const p = params ?? {};
	const segments = pattern.split("/").filter((s) => {
		return s.length > 0;
	});
	const splat_encoded = (splat_values ?? [])
		.map((v) => {
			return encodeURIComponent(v);
		})
		.join("/");

	const path_segments = segments.flatMap((seg) => {
		if (seg === SPLAT_RUNE) {
			return splat_encoded.split("/").filter((s) => {
				return s.length > 0;
			});
		}
		if (seg.startsWith(DYNAMIC_RUNE)) {
			return [
				encodeURIComponent(p[seg.slice(DYNAMIC_RUNE.length)] as string),
			];
		}
		return [seg];
	});

	const result = "/" + path_segments.join("/");
	return strip_trailing_slash(result.length === 0 ? "/" : result);
}

export function resolve_path(
	type: "loader" | "action",
	pattern: string,
	params?: Record<string, string>,
	splat_values?: string[],
): string {
	let path = resolve_pattern_path(pattern, params, splat_values);

	if (type === "loader") {
		const suffix = `/${INDEX_SEGMENT}`;
		if (path.endsWith(suffix)) {
			path = path.slice(0, -suffix.length) || "/";
		}
	}

	return path;
}

export function to_typed_href(
	pattern: string,
	params?: Record<string, string>,
	splat_values?: string[],
	search?: unknown,
	hash?: string,
): string {
	const base = resolve_path("loader", pattern, params, splat_values);
	const url = new URL(base, window.location.origin);
	if (search !== undefined) {
		url.search = serializeToSearchParams(search).toString();
	}
	if (hash !== undefined) {
		url.hash = hash;
	}
	return url.href;
}

export function build_action_url(
	actions_mount_root: string,
	pattern: string,
	params?: Record<string, string>,
	splat_values?: string[],
	input?: unknown,
): URL {
	const pathname = resolve_path("action", pattern, params, splat_values);
	const full =
		strip_trailing_slash(actions_mount_root) +
		(pathname === "/" ? "" : pathname);
	const url = new URL(full, window.location.origin);
	if (input && typeof input === "object") {
		url.search = serializeToSearchParams(input).toString();
	}
	return url;
}

export function create_typed_to_href<A extends AppConfig>() {
	return <P extends MakeTypedLoaderPattern<A>>(
		destination: MakeTypedRouteDestination<A, P>,
	): string => {
		const d = destination as any;
		return to_typed_href(
			d.pattern,
			d.params,
			d.splatValues,
			d.search,
			d.hash,
		);
	};
}

export function resolve_body(input: unknown): BodyInit | null | undefined {
	if (
		input === undefined ||
		input === null ||
		typeof input === "string" ||
		input instanceof ReadableStream ||
		input instanceof FormData ||
		input instanceof URLSearchParams ||
		input instanceof Blob ||
		input instanceof ArrayBuffer
	) {
		return input;
	}
	if (ArrayBuffer.isView(input)) {
		return input as ArrayBufferView<ArrayBuffer>;
	}
	return JSON.stringify(input);
}

export function create_typed_navigate<A extends AppConfig>(
	navigate_fn: (
		href: string | URL,
		options?: {
			replace?: boolean;
			scrollToTop?: boolean;
			state?: unknown;
			skipProgressIndicator?: boolean;
		},
	) => Promise<{ didNavigate: boolean }>,
) {
	const to_href = create_typed_to_href<A>();
	return async <P extends MakeTypedLoaderPattern<A>>(
		props: MakeTypedNavProps<A, P>,
	): Promise<{ didNavigate: boolean }> => {
		const href = props.href ?? to_href(props as any);
		return navigate_fn(href, {
			replace: props.replace,
			scrollToTop: props.scrollToTop,
			state: props.state,
			skipProgressIndicator: props.skipProgressIndicator,
		});
	};
}

export function create_typed_prefetch<A extends AppConfig>(
	prefetch_fn: (href: string) => void,
) {
	const to_href = create_typed_to_href<A>();
	return <P extends MakeTypedLoaderPattern<A>>(
		target: MakeTypedNavTarget<A, P>,
	): void => {
		prefetch_fn(target.href ?? to_href(target as any));
	};
}
