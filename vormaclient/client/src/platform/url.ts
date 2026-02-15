import { __vormaClientGlobal } from "../app/context.ts";

export const VORMA_HARD_RELOAD_QUERY_PARAM = "vorma_reload";

function stripHashPrefix(hash: string): string {
	return hash.startsWith("#") ? hash.slice(1) : hash;
}

export function hrefWithoutHash(props: {
	href: string;
	baseHref?: string;
}): string {
	const { href, baseHref = window.location.href } = props;
	const url = new URL(href, baseHref);
	url.hash = "";
	return url.href;
}

export function hasSameDataTarget(props: {
	firstHref: string;
	secondHref: string;
	baseHref?: string;
}): boolean {
	const { firstHref, secondHref, baseHref = window.location.href } = props;
	return (
		hrefWithoutHash({ href: firstHref, baseHref }) ===
		hrefWithoutHash({ href: secondHref, baseHref })
	);
}

export function hasSameNavigationTarget(props: {
	firstHref: string;
	secondHref: string;
	baseHref?: string;
}): boolean {
	const { firstHref, secondHref, baseHref = window.location.href } = props;
	return (
		firstHref === secondHref ||
		hasSameDataTarget({ firstHref, secondHref, baseHref })
	);
}

export function findMapEntryByNavigationTarget<T>(props: {
	map: ReadonlyMap<string, T>;
	targetHref: string;
	baseHref?: string;
}): [string, T] | undefined {
	const { map, targetHref, baseHref = window.location.href } = props;
	if (map.has(targetHref)) {
		return [targetHref, map.get(targetHref)!];
	}

	for (const [key, value] of map.entries()) {
		if (
			hasSameDataTarget({
				firstHref: key,
				secondHref: targetHref,
				baseHref,
			})
		) {
			return [key, value];
		}
	}

	return undefined;
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

export function isSameDocumentHashChange(props: {
	targetHref: string;
	currentHref?: string;
}): boolean {
	const { targetHref, currentHref = window.location.href } = props;
	return (
		hasSameDataTarget({
			firstHref: targetHref,
			secondHref: currentHref,
			baseHref: currentHref,
		}) &&
		normalizedHashFragmentFromHref(targetHref) !==
			normalizedHashFragmentFromHref(currentHref)
	);
}

export function isSameDocumentLocation(props: {
	targetHref: string;
	currentHref?: string;
}): boolean {
	const { targetHref, currentHref = window.location.href } = props;
	return (
		hasSameDataTarget({
			firstHref: targetHref,
			secondHref: currentHref,
			baseHref: currentHref,
		}) &&
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
