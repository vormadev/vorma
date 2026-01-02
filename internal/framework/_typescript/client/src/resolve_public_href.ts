import { __vormaClientGlobal } from "./vorma_ctx/vorma_ctx.ts";

export function resolvePublicHref(relativeHref: string): string {
	let baseURL = __vormaClientGlobal.get("viteDevURL");
	if (!baseURL) {
		baseURL = __vormaClientGlobal.get("publicPathPrefix");
	}
	if (baseURL.endsWith("/")) {
		baseURL = baseURL.slice(0, -1);
	}
	let final = relativeHref.startsWith("/")
		? baseURL + relativeHref
		: baseURL + "/" + relativeHref;
	return final;
}
