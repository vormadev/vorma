import { getClientCookie } from "vorma/kit/cookies";

export function getCsrfToken(opts: {
	isDev: boolean;
	cookieName?: string;
}): string | undefined {
	const prefix = opts.isDev ? "__Dev-" : "__Host-";
	const name = opts.cookieName || "csrf_token";
	return getClientCookie(prefix + name);
}
