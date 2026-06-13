import { X_CLIENT_REDIRECT } from "./constants.ts";

/*
Shared classification ladder for every server-issued redirect target,
regardless of whether a navigation or a submission produced it.
*/

export type ClassifiedRedirect =
	| { kind: "invalid" }
	| { kind: "hard"; href: string }
	| { kind: "soft"; url: URL };

export function is_http(href: string): boolean {
	try {
		const p = new URL(href).protocol;
		return p === "http:" || p === "https:";
	} catch {
		return false;
	}
}

export function detect_redirect(
	res: Response,
	base: URL,
): { href: string; hard: boolean } | null {
	const soft = res.headers.get(X_CLIENT_REDIRECT);
	if (soft) {
		return { href: new URL(soft, base).href, hard: false };
	}
	if (res.redirected && res.url && res.url !== base.href) {
		return { href: new URL(res.url, base).href, hard: false };
	}
	return null;
}

export function classify_redirect_target(
	href: string,
	hard: boolean,
	current_origin: string,
): ClassifiedRedirect {
	if (!is_http(href)) {
		return { kind: "invalid" };
	}
	if (hard || new URL(href).origin !== current_origin) {
		return { kind: "hard", href };
	}
	return { kind: "soft", url: new URL(href) };
}
