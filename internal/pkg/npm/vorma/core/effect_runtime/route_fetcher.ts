import { Data, Effect } from "effect";
import {
	VERCEL_DPL_QUERY_PARAM_KEY,
	VORMA_JSON_KEY,
	X_ACCEPTS_CLIENT_REDIRECT,
	X_CLIENT_REDIRECT,
	X_VORMA_BUILD_SKEW,
} from "../constants.ts";
import type { BrowserFetchRuntime } from "./browser_fetch_runtime.ts";

export type RouteFetchInput = {
	url: URL;
	revalidation?: boolean;
	signal?: AbortSignal;
};

export type RouteFetchResult =
	| {
			kind: "data";
			requested_url: URL;
			response: Response;
			data: unknown;
	  }
	| {
			kind: "build_skew";
			requested_url: URL;
			response: Response;
	  }
	| {
			kind: "redirect";
			requested_url: URL;
			response: Response;
			href: string;
			hard: boolean;
	  }
	| {
			kind: "error";
			requested_url: URL;
			response?: Response;
			status: number;
			status_text: string;
			error?: unknown;
	  };

export type RouteFetcherOptions = {
	client_build_id: string;
	deployment_id?: string;
	fetch: BrowserFetchRuntime["fetch"];
};

export type RouteFetcher = {
	fetch_route: (
		input: RouteFetchInput,
	) => Effect.Effect<RouteFetchResult, RouteFetchFailed>;
};

export class RouteFetchFailed extends Data.TaggedError("RouteFetchFailed")<{
	readonly requested_url: string;
	readonly error: unknown;
}> {}

export function make_route_fetcher(options: RouteFetcherOptions): RouteFetcher {
	return {
		fetch_route: (input) => {
			return Effect.gen(function* () {
				const requested_url = route_request_url(input, options);
				const response = yield* options
					.fetch({
						url: requested_url,
						init: {
							headers: {
								[X_ACCEPTS_CLIENT_REDIRECT]: "1",
							},
						},
						signals: [input.signal],
					})
					.pipe(
						Effect.mapError((error) => {
							return new RouteFetchFailed({
								requested_url: requested_url.href,
								error,
							});
						}),
					);
				return yield* classify_route_response(requested_url, response);
			});
		},
	};
}

function route_request_url(
	input: RouteFetchInput,
	options: RouteFetcherOptions,
): URL {
	const requested_url = new URL(input.url.href);
	requested_url.searchParams.set(VORMA_JSON_KEY, options.client_build_id);
	if (input.revalidation && options.deployment_id) {
		requested_url.searchParams.set(
			VERCEL_DPL_QUERY_PARAM_KEY,
			options.deployment_id,
		);
	}
	return requested_url;
}

function classify_route_response(
	requested_url: URL,
	response: Response,
): Effect.Effect<RouteFetchResult> {
	if (response.headers.get(X_VORMA_BUILD_SKEW) === "1") {
		return Effect.succeed({
			kind: "build_skew",
			requested_url,
			response,
		});
	}
	const redirect = detect_redirect(response, requested_url);
	if (redirect) {
		return Effect.succeed({
			kind: "redirect",
			requested_url,
			response,
			...redirect,
		});
	}
	if (!response.ok) {
		return Effect.succeed({
			kind: "error",
			requested_url,
			response,
			status: response.status,
			status_text: response.statusText,
		});
	}
	return Effect.either(
		Effect.tryPromise({
			try: () => {
				return response.json();
			},
			catch: (error) => {
				return error;
			},
		}),
	).pipe(
		Effect.map((result) => {
			if (result._tag === "Left") {
				return {
					kind: "error" as const,
					requested_url,
					response,
					status: response.status,
					status_text: response.statusText,
					error: result.left,
				};
			}
			return {
				kind: "data" as const,
				requested_url,
				response,
				data: result.right,
			};
		}),
	);
}

function detect_redirect(
	response: Response,
	base: URL,
): { href: string; hard: boolean } | null {
	const soft = response.headers.get(X_CLIENT_REDIRECT);
	if (soft) {
		return { href: new URL(soft, base).href, hard: false };
	}
	if (response.redirected && response.url && response.url !== base.href) {
		return { href: new URL(response.url, base).href, hard: false };
	}
	return null;
}
