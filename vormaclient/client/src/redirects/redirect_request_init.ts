import {
	isArrayBufferView,
	isInstanceOfGlobal,
} from "../utils/global_constructors.ts";

function shouldSerializeBody(body: unknown): boolean {
	if (body === null || body === undefined) return false;
	if (typeof body === "string") return false;
	if (isInstanceOfGlobal(body, "FormData")) return false;
	if (isInstanceOfGlobal(body, "URLSearchParams")) return false;
	if (isInstanceOfGlobal(body, "Blob")) return false;
	if (isInstanceOfGlobal(body, "ArrayBuffer")) return false;
	if (isArrayBufferView(body)) {
		return false;
	}
	if (isInstanceOfGlobal(body, "ReadableStream")) return false;

	return true;
}

function canIncludeBodyForMethod(method: string | undefined): boolean {
	const normalizedMethod = method?.toUpperCase();
	if (!normalizedMethod) {
		return false;
	}
	return normalizedMethod !== "GET" && normalizedMethod !== "HEAD";
}

export function buildRedirectRequestInit(
	requestInit: RequestInit | undefined,
	signal: AbortSignal,
): RequestInit {
	const {
		body: rawBody,
		headers: rawHeaders,
		signal: _requestSignal,
		...rest
	} = requestInit ?? {};

	const bodyParentObj: Pick<RequestInit, "body"> = {};
	if (rawBody !== undefined && canIncludeBodyForMethod(requestInit?.method)) {
		bodyParentObj.body = shouldSerializeBody(rawBody)
			? JSON.stringify(rawBody)
			: rawBody;
	}

	const headers = new Headers(rawHeaders);
	// To temporarily test traditional server redirect behavior,
	// you can set this to "0" instead of "1"
	headers.set("X-Accepts-Client-Redirect", "1");

	return {
		...rest,
		headers,
		signal,
		...bodyParentObj,
	};
}
