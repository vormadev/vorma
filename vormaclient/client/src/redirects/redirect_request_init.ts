import { getIsGETRequest } from "vorma/kit/url";

export function buildRedirectRequestInit(
	requestInit: RequestInit | undefined,
	signal: AbortSignal,
): RequestInit {
	const bodyParentObj: RequestInit = {};
	const isGET = getIsGETRequest(requestInit);

	if (requestInit && (requestInit.body !== undefined || !isGET)) {
		if (
			requestInit.body instanceof FormData ||
			typeof requestInit.body === "string"
		) {
			bodyParentObj.body = requestInit.body;
		} else {
			bodyParentObj.body = JSON.stringify(requestInit.body);
		}
	}

	const headers = new Headers(requestInit?.headers);
	// To temporarily test traditional server redirect behavior,
	// you can set this to "0" instead of "1"
	headers.set("X-Accepts-Client-Redirect", "1");
	bodyParentObj.headers = headers;

	return {
		signal,
		...requestInit,
		...bodyParentObj,
	};
}
