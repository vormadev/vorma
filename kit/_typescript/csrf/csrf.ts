import { getClientCookie } from "vorma/kit/cookies";

/////////////////////////////////////////////////////////////////////
/////// Must stay aligned with corresponding server code in
/////// kit/csrf/csrf.go (inheriting from kit/cookies/cookies.go)
/////////////////////////////////////////////////////////////////////

export function getCSRFToken(opts: {
	isDev: boolean;
	cookieName?: string;
}): string | undefined {
	const prefix = opts.isDev ? "__Dev-" : "__Host-";
	const name = opts.cookieName || "csrf_token";
	return getClientCookie(prefix + name);
}
