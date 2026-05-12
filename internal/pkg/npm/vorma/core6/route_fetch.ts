import {
	BUILD_ID_HEADER,
	VERCEL_DPL_QUERY_PARAM_KEY,
	VORMA_JSON_KEY,
	VORMA_PROTOCOL_ENABLED,
	X_ACCEPTS_CLIENT_REDIRECT,
	X_CLIENT_REDIRECT,
	X_VORMA_BUILD_SKEW,
} from "../core/constants.ts";

export const core6_route_fetch_result_kind = {
	build_skew: "build_skew",
	failure: "failure",
	payload: "payload",
	redirect: "redirect",
} as const;

export const core6_route_fetch_failure_reason = {
	aborted: "aborted",
	http_error: "http_error",
	invalid_json: "invalid_json",
	invalid_redirect: "invalid_redirect",
	invalid_url: "invalid_url",
	network_error: "network_error",
} as const;

export type Core6RouteFetchResultKind =
	(typeof core6_route_fetch_result_kind)[keyof typeof core6_route_fetch_result_kind];

export type Core6RouteFetchFailureReason =
	(typeof core6_route_fetch_failure_reason)[keyof typeof core6_route_fetch_failure_reason];

export type Core6RouteFetchHost = {
	fetch_route_response: (args: {
		href: string;
		init: RequestInit & { signal: AbortSignal };
	}) => Promise<Response>;
};

export type Core6RouteFetchInput = {
	active_client_build_id: string;
	deployment_id?: string | null;
	href: string;
	host: Core6RouteFetchHost;
	is_revalidation?: boolean;
	signal: AbortSignal;
};

export type Core6RouteFetchRequest = {
	active_client_build_id: string;
	headers: Headers;
	href: string;
	requested_href: string;
	signal: AbortSignal;
};

export type Core6RouteFetchResponseMeta = {
	final_href: string;
	ok: boolean;
	requested_href: string;
	server_build_id: string;
	status: number;
	status_text: string;
};

export type Core6RouteFetchRedirect = {
	hard: boolean;
	href: string;
};

export type Core6RouteFetchPayloadResult = {
	kind: typeof core6_route_fetch_result_kind.payload;
	payload: unknown;
	response: Core6RouteFetchResponseMeta;
};

export type Core6RouteFetchRedirectResult = {
	kind: typeof core6_route_fetch_result_kind.redirect;
	redirect: Core6RouteFetchRedirect;
	response: Core6RouteFetchResponseMeta;
};

export type Core6RouteFetchBuildSkewResult = {
	kind: typeof core6_route_fetch_result_kind.build_skew;
	redirect: Core6RouteFetchRedirect | null;
	response: Core6RouteFetchResponseMeta;
};

export type Core6RouteFetchFailureResult = {
	error?: unknown;
	kind: typeof core6_route_fetch_result_kind.failure;
	reason: Core6RouteFetchFailureReason;
	response?: Core6RouteFetchResponseMeta;
};

export type Core6RouteFetchResult =
	| Core6RouteFetchPayloadResult
	| Core6RouteFetchRedirectResult
	| Core6RouteFetchBuildSkewResult
	| Core6RouteFetchFailureResult;

export function build_core6_route_fetch_request(
	input: Omit<Core6RouteFetchInput, "host">,
): Core6RouteFetchRequest | Core6RouteFetchFailureResult {
	let url: URL;
	try {
		url = new URL(input.href);
	} catch (error) {
		return {
			error,
			kind: core6_route_fetch_result_kind.failure,
			reason: core6_route_fetch_failure_reason.invalid_url,
		};
	}
	url.searchParams.set(VORMA_JSON_KEY, input.active_client_build_id);
	if (input.is_revalidation && input.deployment_id) {
		url.searchParams.set(VERCEL_DPL_QUERY_PARAM_KEY, input.deployment_id);
	}
	const headers = new Headers();
	headers.set(X_ACCEPTS_CLIENT_REDIRECT, VORMA_PROTOCOL_ENABLED);
	return {
		active_client_build_id: input.active_client_build_id,
		headers,
		href: url.href,
		requested_href: input.href,
		signal: input.signal,
	};
}

export async function fetch_core6_route(
	input: Core6RouteFetchInput,
): Promise<Core6RouteFetchResult> {
	const request = build_core6_route_fetch_request(input);
	if (is_core6_route_fetch_failure_result(request)) {
		return request;
	}
	try {
		const response = await input.host.fetch_route_response({
			href: request.href,
			init: {
				headers: request.headers,
				signal: request.signal,
			},
		});
		return await classify_core6_route_fetch_response({
			request,
			response,
		});
	} catch (error) {
		return {
			error,
			kind: core6_route_fetch_result_kind.failure,
			reason:
				input.signal.aborted || is_core6_route_fetch_abort_error(error)
					? core6_route_fetch_failure_reason.aborted
					: core6_route_fetch_failure_reason.network_error,
		};
	}
}

export async function classify_core6_route_fetch_response(input: {
	request: Core6RouteFetchRequest;
	response: Response;
}): Promise<Core6RouteFetchResult> {
	const response = core6_route_fetch_response_meta(
		input.request,
		input.response,
	);
	const redirect = core6_route_fetch_redirect(input.request, input.response);
	if (is_core6_route_fetch_failure_result(redirect)) {
		return {
			...redirect,
			response,
		};
	}
	if (
		input.response.headers.get(X_VORMA_BUILD_SKEW) ===
		VORMA_PROTOCOL_ENABLED
	) {
		return {
			kind: core6_route_fetch_result_kind.build_skew,
			redirect: redirect.redirect,
			response,
		};
	}
	if (redirect.redirect) {
		return {
			kind: core6_route_fetch_result_kind.redirect,
			redirect: redirect.redirect,
			response,
		};
	}
	if (!input.response.ok) {
		return {
			kind: core6_route_fetch_result_kind.failure,
			reason: core6_route_fetch_failure_reason.http_error,
			response,
		};
	}
	try {
		return {
			kind: core6_route_fetch_result_kind.payload,
			payload: await input.response.json(),
			response,
		};
	} catch (error) {
		return {
			error,
			kind: core6_route_fetch_result_kind.failure,
			reason: core6_route_fetch_failure_reason.invalid_json,
			response,
		};
	}
}

function core6_route_fetch_redirect(
	request: Core6RouteFetchRequest,
	response: Response,
): { redirect: Core6RouteFetchRedirect | null } | Core6RouteFetchFailureResult {
	const soft_redirect = response.headers.get(X_CLIENT_REDIRECT);
	if (soft_redirect) {
		try {
			return {
				redirect: {
					hard: false,
					href: new URL(soft_redirect, request.href).href,
				},
			};
		} catch (error) {
			return {
				error,
				kind: core6_route_fetch_result_kind.failure,
				reason: core6_route_fetch_failure_reason.invalid_redirect,
			};
		}
	}
	if (response.redirected && response.url && response.url !== request.href) {
		return {
			redirect: {
				hard: false,
				href: response.url,
			},
		};
	}
	return { redirect: null };
}

function core6_route_fetch_response_meta(
	request: Core6RouteFetchRequest,
	response: Response,
): Core6RouteFetchResponseMeta {
	return {
		final_href: response.url || request.href,
		ok: response.ok,
		requested_href: request.requested_href,
		server_build_id: response.headers.get(BUILD_ID_HEADER) ?? "",
		status: response.status,
		status_text: response.statusText,
	};
}

function is_core6_route_fetch_abort_error(error: unknown): boolean {
	return (
		error instanceof DOMException &&
		(error.name === "AbortError" || error.message === "Aborted")
	);
}

function is_core6_route_fetch_failure_result(
	result:
		| Core6RouteFetchFailureResult
		| Core6RouteFetchRequest
		| { redirect: Core6RouteFetchRedirect | null },
): result is Core6RouteFetchFailureResult {
	return "kind" in result;
}
