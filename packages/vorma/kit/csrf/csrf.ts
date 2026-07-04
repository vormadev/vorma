import { getClientCookie } from "vorma/kit/cookies";

/**
 * Read a CSRF-style token from a cookie following the `__Host-` browser
 * cookie-prefix convention (the strictest cookie-prefix protection
 * browsers support: the server must set `Secure`, no `Domain` attribute,
 * and `Path=/` for the prefix to be honored). `opts.isDev` swaps in a
 * `__Dev-` prefix instead — a plain naming convention, not a
 * browser-enforced one — for local development over plain HTTP, where
 * `__Host-`'s `Secure` requirement cannot be met; the server side must
 * name its cookie to match.
 *
 * This is a generic cookie-reading helper — kit does not generate,
 * validate, or otherwise own CSRF tokens; an app pairs this with its own
 * server-side token issuance and an `apiDecorator` that attaches the read
 * value as a request header (see `ToApiDecorator`).
 *
 * `opts.cookieName` (default `"csrf_token"`) is the name after the prefix.
 */
export function getCsrfToken(opts: {
	isDev: boolean;
	cookieName?: string;
}): string | undefined {
	const prefix = opts.isDev ? "__Dev-" : "__Host-";
	const name = opts.cookieName || "csrf_token";
	return getClientCookie(prefix + name);
}
