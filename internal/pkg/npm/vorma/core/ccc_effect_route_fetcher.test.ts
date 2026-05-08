import { Effect, Result as EffectResult } from "effect";
import { TestClock } from "effect/testing";
import { describe, expect, it } from "vitest";
import {
	VERCEL_DPL_QUERY_PARAM_KEY,
	VORMA_JSON_KEY,
	X_ACCEPTS_CLIENT_REDIRECT,
	X_CLIENT_REDIRECT,
	X_VORMA_BUILD_SKEW,
} from "./constants.ts";
import {
	BrowserFetchFailed,
	make_browser_fetch_runtime,
} from "./effect_runtime/browser_fetch_runtime.ts";
import {
	RouteFetchFailed,
	make_route_fetcher,
} from "./effect_runtime/route_fetcher.ts";

const CLIENT_BUILD_ID = "build-1";
const DEPLOYMENT_ID = "deploy-1";
const ROUTE_HREF = "http://localhost/dashboard?tab=home";

type FetchCall = {
	url: URL;
	init: RequestInit;
};

function run_effect<A, E>(program: Effect.Effect<A, E, never>): Promise<A> {
	return Effect.runPromise(program.pipe(Effect.provide(TestClock.layer())));
}

function json_response(data: unknown, init?: ResponseInit): Response {
	const headers = new Headers({
		"Content-Type": "application/json",
	});
	if (init?.headers) {
		new Headers(init.headers).forEach((value, key) => {
			headers.set(key, value);
		});
	}
	return new Response(JSON.stringify(data), {
		status: init?.status ?? 200,
		statusText: init?.statusText,
		headers,
	});
}

function browser_fetch(
	fetch_impl: (url: URL, init: RequestInit) => Promise<Response>,
) {
	return Effect.runSync(
		make_browser_fetch_runtime({
			fetch: fetch_impl,
		}),
	).fetch;
}

describe("ccc Effect route fetcher experiment", () => {
	it("decorates route requests and classifies JSON data responses", async () => {
		const calls: FetchCall[] = [];
		const fetcher = make_route_fetcher({
			client_build_id: CLIENT_BUILD_ID,
			fetch: browser_fetch(async (url, init) => {
				calls.push({ url, init });
				return json_response({ ok: true });
			}),
		});

		const result = await run_effect(
			fetcher.fetch_route({
				url: new URL(ROUTE_HREF),
			}),
		);

		expect(result.kind).toBe("data");
		expect(result.requested_url.searchParams.get(VORMA_JSON_KEY)).toBe(
			CLIENT_BUILD_ID,
		);
		expect(calls).toHaveLength(1);
		expect(calls[0]?.url.href).toBe(result.requested_url.href);
		expect(
			new Headers(calls[0]?.init.headers).get(X_ACCEPTS_CLIENT_REDIRECT),
		).toBe("1");
		if (result.kind !== "data") {
			throw new Error("expected data route fetch result");
		}
		expect(result.data).toEqual({ ok: true });
	});

	it("adds deployment query only for revalidation fetches", async () => {
		const calls: FetchCall[] = [];
		const fetcher = make_route_fetcher({
			client_build_id: CLIENT_BUILD_ID,
			deployment_id: DEPLOYMENT_ID,
			fetch: browser_fetch(async (url, init) => {
				calls.push({ url, init });
				return json_response({ ok: true });
			}),
		});

		await run_effect(
			fetcher.fetch_route({
				url: new URL(ROUTE_HREF),
			}),
		);
		await run_effect(
			fetcher.fetch_route({
				url: new URL(ROUTE_HREF),
				revalidation: true,
			}),
		);

		expect(calls[0]?.url.searchParams.get(VERCEL_DPL_QUERY_PARAM_KEY)).toBe(
			null,
		);
		expect(calls[1]?.url.searchParams.get(VERCEL_DPL_QUERY_PARAM_KEY)).toBe(
			DEPLOYMENT_ID,
		);
	});

	it("prioritizes build skew classification over redirects", async () => {
		const fetcher = make_route_fetcher({
			client_build_id: CLIENT_BUILD_ID,
			fetch: browser_fetch(async () => {
				return json_response(
					{ ignored: true },
					{
						headers: {
							[X_VORMA_BUILD_SKEW]: "1",
							[X_CLIENT_REDIRECT]: "/target",
						},
					},
				);
			}),
		});

		const result = await run_effect(
			fetcher.fetch_route({
				url: new URL(ROUTE_HREF),
			}),
		);

		expect(result.kind).toBe("build_skew");
	});

	it("classifies X-Client-Redirect before HTTP errors", async () => {
		const fetcher = make_route_fetcher({
			client_build_id: CLIENT_BUILD_ID,
			fetch: browser_fetch(async () => {
				return new Response("", {
					status: 500,
					statusText: "Server Error",
					headers: {
						[X_CLIENT_REDIRECT]: "child/final",
					},
				});
			}),
		});

		const result = await run_effect(
			fetcher.fetch_route({
				url: new URL("http://localhost/parent/page"),
			}),
		);

		expect(result).toMatchObject({
			kind: "redirect",
			href: "http://localhost/parent/child/final",
			hard: false,
		});
	});

	it("classifies non-OK and invalid JSON responses as route errors", async () => {
		const fetcher = make_route_fetcher({
			client_build_id: CLIENT_BUILD_ID,
			fetch: browser_fetch(async (url) => {
				if (url.pathname === "/bad-json") {
					return new Response("not-json", {
						status: 200,
						statusText: "OK",
					});
				}
				return new Response("unavailable", {
					status: 503,
					statusText: "Service Unavailable",
				});
			}),
		});

		const non_ok = await run_effect(
			fetcher.fetch_route({
				url: new URL("http://localhost/unavailable"),
			}),
		);
		const bad_json = await run_effect(
			fetcher.fetch_route({
				url: new URL("http://localhost/bad-json"),
			}),
		);

		expect(non_ok).toMatchObject({
			kind: "error",
			status: 503,
			status_text: "Service Unavailable",
		});
		expect(bad_json).toMatchObject({
			kind: "error",
			status: 200,
			status_text: "OK",
		});
	});

	it("fails the Effect when the fetch operation itself fails", async () => {
		const fetcher = make_route_fetcher({
			client_build_id: CLIENT_BUILD_ID,
			fetch: browser_fetch(async () => {
				throw new Error("network broke");
			}),
		});

		const result = await run_effect(
			Effect.result(
				fetcher.fetch_route({
					url: new URL(ROUTE_HREF),
				}),
			),
		);

		expect(EffectResult.isFailure(result)).toBe(true);
		if (!EffectResult.isFailure(result)) {
			throw new Error("expected route fetch failure");
		}
		expect(result.failure).toBeInstanceOf(RouteFetchFailed);
		expect(result.failure.error).toBeInstanceOf(BrowserFetchFailed);
		const transport_error = result.failure.error as BrowserFetchFailed;
		expect(transport_error.error).toBeInstanceOf(Error);
		expect((transport_error.error as Error).message).toBe("network broke");
	});
});
