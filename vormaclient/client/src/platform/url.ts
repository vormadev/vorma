import { __vormaClientGlobal } from "../app/context.ts";

export const VORMA_HARD_RELOAD_QUERY_PARAM = "vorma_reload";

function stripHashPrefix(hash: string): string {
	return hash.startsWith("#") ? hash.slice(1) : hash;
}

export function hrefWithoutHash(
	href: string,
	baseHref = window.location.href,
): string {
	const url = new URL(href, baseHref);
	url.hash = "";
	return url.href;
}

export function hasSameDataTarget(
	a: string,
	b: string,
	baseHref = window.location.href,
): boolean {
	return hrefWithoutHash(a, baseHref) === hrefWithoutHash(b, baseHref);
}

export function decodeHashFragment(hashFragment: string): string {
	try {
		return decodeURIComponent(hashFragment);
	} catch {
		return hashFragment;
	}
}

export function normalizedHashFragmentFromHash(hash: string): string {
	return decodeHashFragment(hashFragmentFromHash(hash));
}

export function hashFragmentFromHash(hash: string): string {
	return stripHashPrefix(hash);
}

export function normalizedHashFragmentFromHref(href: string): string {
	return decodeHashFragment(hashFragmentFromHref(href));
}

export function hashFragmentFromHref(href: string): string {
	const url = new URL(href, window.location.href);
	return hashFragmentFromHash(url.hash);
}

export function isSameDocumentHashChange(
	targetHref: string,
	currentHref = window.location.href,
): boolean {
	return (
		hasSameDataTarget(targetHref, currentHref, currentHref) &&
		normalizedHashFragmentFromHref(targetHref) !==
			normalizedHashFragmentFromHref(currentHref)
	);
}

export function isSameDocumentLocation(
	targetHref: string,
	currentHref = window.location.href,
): boolean {
	return (
		hasSameDataTarget(targetHref, currentHref, currentHref) &&
		normalizedHashFragmentFromHref(targetHref) ===
			normalizedHashFragmentFromHref(currentHref)
	);
}

export function resolvePublicHref(relativeHref: string): string {
	let baseURL = __vormaClientGlobal.get("viteDevURL");
	if (!baseURL) {
		baseURL = __vormaClientGlobal.get("publicPathPrefix");
	}
	if (baseURL.endsWith("/")) {
		baseURL = baseURL.slice(0, -1);
	}
	const final = relativeHref.startsWith("/")
		? baseURL + relativeHref
		: baseURL + "/" + relativeHref;
	return final;
}
