import type { Core6RouteScroll } from "./route_publication.ts";

export function core6_is_http_href(href: string): boolean {
	try {
		const protocol = new URL(href).protocol;
		return protocol === "http:" || protocol === "https:";
	} catch {
		return false;
	}
}

export function core6_same_origin_href(
	left_href: string,
	right_href: string,
): boolean {
	try {
		return new URL(left_href).origin === new URL(right_href).origin;
	} catch {
		return false;
	}
}

export function core6_same_document_href(
	left_href: string,
	right_href: string,
): boolean {
	const left = new URL(left_href);
	const right = new URL(right_href);
	return (
		left.origin === right.origin &&
		left.pathname === right.pathname &&
		left.search === right.search
	);
}

export function core6_scroll_for_href(
	href: string,
	fallback?: Core6RouteScroll,
): Core6RouteScroll {
	const hash = new URL(href).hash;
	if (hash.length > 0) {
		return { hash };
	}
	return fallback ?? { x: 0, y: 0 };
}

export function core6_normalized_hash_from_href(href: string): string {
	try {
		return core6_normalize_hash(new URL(href).hash);
	} catch {
		return "";
	}
}

export function core6_normalize_hash(hash: string): string {
	const without_prefix = hash.startsWith("#") ? hash.slice(1) : hash;
	if (without_prefix.length === 0) {
		return without_prefix;
	}
	try {
		return decodeURIComponent(without_prefix);
	} catch {
		return without_prefix;
	}
}
