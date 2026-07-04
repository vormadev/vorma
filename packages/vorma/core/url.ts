import { serializeToSearchParams } from "vorma/kit/json";
import type {
	AppConfig,
	ToNavigateArgs,
	ToNavigationTarget,
	ToRouteDestination,
	ToViewPattern,
} from "./types.ts";

const dynamic_rune = ":";
const splat_rune = "*";
const index_segment = "_index";

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
		if (seg === splat_rune) {
			return splat_encoded.split("/").filter((s) => {
				return s.length > 0;
			});
		}
		if (seg.startsWith(dynamic_rune)) {
			return [encodeURIComponent(p[seg.slice(dynamic_rune.length)] as string)];
		}
		return [seg];
	});

	const result = "/" + path_segments.join("/");
	return strip_trailing_slash(result.length === 0 ? "/" : result);
}

/**
 * Resolve a route pattern (with `:param`/`*` placeholders) plus its params
 * and splat values into a concrete pathname. `type: "view"` additionally
 * collapses a trailing `/_index` segment to its parent path (an index
 * pattern's own URL); `type: "resource"` never does that collapsing, since
 * resource patterns have no index-route concept.
 */
export function resolve_path(
	type: "view" | "resource",
	pattern: string,
	params?: Record<string, string>,
	splat_values?: string[],
): string {
	let path = resolve_pattern_path(pattern, params, splat_values);

	if (type === "view") {
		const suffix = `/${index_segment}`;
		if (path.endsWith(suffix)) {
			path = path.slice(0, -suffix.length) || "/";
		}
	}

	return path;
}

/**
 * Resolve a view pattern plus params/splat values/search/hash into a
 * complete, absolute `href`. The typed adapter surface (`toHref`, `Link`,
 * `navigate`) builds on this — reach for it directly only when building a
 * href from raw, pre-validated pieces outside that typed surface.
 */
export function to_typed_href(
	pattern: string,
	params?: Record<string, string>,
	splat_values?: string[],
	search?: unknown,
	hash?: string,
): string {
	const base = resolve_path("view", pattern, params, splat_values);
	const url = new URL(base, window.location.origin);
	if (search !== undefined) {
		url.search = serializeToSearchParams(search).toString();
	}
	if (hash !== undefined) {
		url.hash = hash;
	}
	return url.href;
}

/**
 * Resolve a resource pattern plus params/splat values into a URL, with
 * `input` (a GET-style query object, not a request body) serialized onto
 * the query string when it is a plain object. This is what `apiClient`
 * builds every GET/HEAD request URL through — a mutation's `input` goes
 * through {@link resolve_body} onto the request body instead.
 */
export function build_resource_url(
	pattern: string,
	params?: Record<string, string>,
	splat_values?: string[],
	input?: unknown,
): URL {
	const pathname = resolve_path("resource", pattern, params, splat_values);
	const url = new URL(pathname, window.location.origin);
	if (input && typeof input === "object") {
		url.search = serializeToSearchParams(input).toString();
	}
	return url;
}

/** Build a typed `toHref` closure bound to app config `A` — what every adapter's `passthrough.toHref` is. */
export function create_typed_to_href<A extends AppConfig>() {
	return <P extends ToViewPattern<A>>(
		destination: ToRouteDestination<A, P>,
	): string => {
		const d = destination as any;
		return to_typed_href(d.pattern, d.params, d.splatValues, d.search, d.hash);
	};
}

/**
 * Convert a mutation's `input` into a `fetch` `BodyInit`. Values `fetch`
 * already understands as bodies (a string, `FormData`, `Blob`, a typed
 * array, etc.) pass through unchanged; anything else is JSON-stringified.
 * `apiClient` uses this for every non-GET/HEAD request body.
 */
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

/** Build a typed `navigate` closure bound to app config `A`, over a raw untyped navigate function — what every adapter's `passthrough.navigate` is. */
export function create_typed_navigate<A extends AppConfig>(
	navigate_fn: (
		href: string | URL,
		options?: {
			replace?: boolean;
			scrollToTop?: boolean;
			state?: unknown;
			skipWorkIndicator?: boolean;
		},
	) => Promise<{ didNavigate: boolean }>,
) {
	const to_href = create_typed_to_href<A>();
	return async <P extends ToViewPattern<A>>(
		args: ToNavigateArgs<A, P>,
	): Promise<{ didNavigate: boolean }> => {
		const href = args.href ?? to_href(args as any);
		return navigate_fn(href, {
			replace: args.replace,
			scrollToTop: args.scrollToTop,
			state: args.state,
			skipWorkIndicator: args.skipWorkIndicator,
		});
	};
}

/** Build a typed `prefetch`/`cancelPrefetch` closure bound to app config `A` — what every adapter's `passthrough.prefetch`/`cancelPrefetch` are (the underlying `prefetch_fn` doubles as both start and cancel, since Vorma coalesces to at most one in-flight prefetch). */
export function create_typed_prefetch<A extends AppConfig>(
	prefetch_fn: (href: string) => void,
) {
	const to_href = create_typed_to_href<A>();
	return <P extends ToViewPattern<A>>(target: ToNavigationTarget<A, P>): void => {
		prefetch_fn(target.href ?? to_href(target as any));
	};
}
