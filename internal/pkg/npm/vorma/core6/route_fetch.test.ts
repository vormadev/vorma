import { describe, expect, it } from "vitest";
import {
	BUILD_ID_HEADER,
	VERCEL_DPL_QUERY_PARAM_KEY,
	VORMA_JSON_KEY,
	VORMA_PROTOCOL_ENABLED,
	X_ACCEPTS_CLIENT_REDIRECT,
	X_CLIENT_REDIRECT,
	X_VORMA_BUILD_SKEW,
} from "../core/constants.ts";
import {
	build_core6_route_fetch_request,
	core6_route_fetch_failure_reason,
	core6_route_fetch_result_kind,
	fetch_core6_route,
	type Core6RouteFetchHost,
	type Core6RouteFetchInput,
} from "./route_fetch.ts";

function make_fetch_input(
	overrides: Partial<Core6RouteFetchInput> = {},
): Core6RouteFetchInput {
	return {
		active_client_build_id: "build-1",
		href: "https://example.test/root?q=Ada",
		host: {
			fetch_route_response: async () => {
				return new Response(JSON.stringify({ ok: true }));
			},
		},
		signal: new AbortController().signal,
		...overrides,
	};
}

describe("core6 route fetch", () => {
	it("builds protocol request URL and headers", () => {
		const controller = new AbortController();

		const request = build_core6_route_fetch_request({
			active_client_build_id: "build-2",
			deployment_id: "deployment-1",
			href: "https://example.test/root?q=Ada",
			is_revalidation: true,
			signal: controller.signal,
		});

		if ("kind" in request) {
			throw new Error("expected request");
		}
		const url = new URL(request.href);
		expect(url.searchParams.get("q")).toBe("Ada");
		expect(url.searchParams.get(VORMA_JSON_KEY)).toBe("build-2");
		expect(url.searchParams.get(VERCEL_DPL_QUERY_PARAM_KEY)).toBe(
			"deployment-1",
		);
		expect(request.headers.get(X_ACCEPTS_CLIENT_REDIRECT)).toBe(
			VORMA_PROTOCOL_ENABLED,
		);
		expect(request.requested_href).toBe("https://example.test/root?q=Ada");
		expect(request.signal).toBe(controller.signal);
	});

	it("does not add deployment id outside revalidation requests", () => {
		const request = build_core6_route_fetch_request({
			active_client_build_id: "build-2",
			deployment_id: "deployment-1",
			href: "https://example.test/root",
			signal: new AbortController().signal,
		});

		if ("kind" in request) {
			throw new Error("expected request");
		}
		expect(
			new URL(request.href).searchParams.get(VERCEL_DPL_QUERY_PARAM_KEY),
		).toBeNull();
	});

	it("returns payload result for successful JSON route response", async () => {
		let captured_href = "";
		let captured_headers = new Headers();
		let captured_signal: AbortSignal | null = null;
		const host: Core6RouteFetchHost = {
			fetch_route_response: async ({ href, init }) => {
				captured_href = href;
				captured_headers = new Headers(init.headers);
				captured_signal = init.signal;
				return new Response(JSON.stringify({ route: "root" }), {
					headers: {
						[BUILD_ID_HEADER]: "build-1",
					},
					status: 200,
				});
			},
		};
		const signal = new AbortController().signal;

		const result = await fetch_core6_route(
			make_fetch_input({ host, signal }),
		);

		expect(result).toMatchObject({
			kind: core6_route_fetch_result_kind.payload,
			payload: { route: "root" },
			response: {
				ok: true,
				requested_href: "https://example.test/root?q=Ada",
				server_build_id: "build-1",
				status: 200,
			},
		});
		expect(new URL(captured_href).searchParams.get(VORMA_JSON_KEY)).toBe(
			"build-1",
		);
		expect(captured_headers.get(X_ACCEPTS_CLIENT_REDIRECT)).toBe(
			VORMA_PROTOCOL_ENABLED,
		);
		expect(captured_signal).toBe(signal);
	});

	it("classifies build skew before redirect or body parsing", async () => {
		const result = await fetch_core6_route(
			make_fetch_input({
				host: {
					fetch_route_response: async () => {
						return new Response("not json", {
							headers: {
								[BUILD_ID_HEADER]: "build-2",
								[X_CLIENT_REDIRECT]: "/login",
								[X_VORMA_BUILD_SKEW]: VORMA_PROTOCOL_ENABLED,
							},
							status: 200,
						});
					},
				},
			}),
		);

		expect(result).toMatchObject({
			kind: core6_route_fetch_result_kind.build_skew,
			redirect: {
				hard: false,
				href: "https://example.test/login",
			},
			response: {
				ok: true,
				server_build_id: "build-2",
				status: 200,
			},
		});
	});

	it("classifies soft route redirect", async () => {
		const result = await fetch_core6_route(
			make_fetch_input({
				host: {
					fetch_route_response: async () => {
						return new Response("not json", {
							headers: {
								[X_CLIENT_REDIRECT]: "../login",
							},
							status: 200,
						});
					},
				},
			}),
		);

		expect(result).toMatchObject({
			kind: core6_route_fetch_result_kind.redirect,
			redirect: {
				hard: false,
				href: "https://example.test/login",
			},
			response: {
				ok: true,
				status: 200,
			},
		});
	});

	it("classifies native route redirect", async () => {
		const response = new Response(null, { status: 200 });
		Object.defineProperty(response, "redirected", { value: true });
		Object.defineProperty(response, "url", {
			value: "https://example.test/native",
		});

		const result = await fetch_core6_route(
			make_fetch_input({
				host: {
					fetch_route_response: async () => {
						return response;
					},
				},
			}),
		);

		expect(result).toMatchObject({
			kind: core6_route_fetch_result_kind.redirect,
			redirect: {
				hard: false,
				href: "https://example.test/native",
			},
		});
	});

	it("classifies non-ok responses as HTTP failures", async () => {
		const result = await fetch_core6_route(
			make_fetch_input({
				host: {
					fetch_route_response: async () => {
						return new Response("failed", {
							status: 503,
							statusText: "Service Unavailable",
						});
					},
				},
			}),
		);

		expect(result).toMatchObject({
			kind: core6_route_fetch_result_kind.failure,
			reason: core6_route_fetch_failure_reason.http_error,
			response: {
				ok: false,
				status: 503,
				status_text: "Service Unavailable",
			},
		});
	});

	it("classifies invalid JSON as payload failure", async () => {
		const result = await fetch_core6_route(
			make_fetch_input({
				host: {
					fetch_route_response: async () => {
						return new Response("not json", { status: 200 });
					},
				},
			}),
		);

		expect(result).toMatchObject({
			kind: core6_route_fetch_result_kind.failure,
			reason: core6_route_fetch_failure_reason.invalid_json,
			response: {
				ok: true,
				status: 200,
			},
		});
	});

	it("classifies invalid request URL before fetching", async () => {
		let did_fetch = false;

		const result = await fetch_core6_route(
			make_fetch_input({
				href: "not a url",
				host: {
					fetch_route_response: async () => {
						did_fetch = true;
						return new Response("{}");
					},
				},
			}),
		);

		expect(result).toMatchObject({
			kind: core6_route_fetch_result_kind.failure,
			reason: core6_route_fetch_failure_reason.invalid_url,
		});
		expect(did_fetch).toBe(false);
	});

	it("classifies fetch rejection as network failure", async () => {
		const error = new Error("network failed");

		const result = await fetch_core6_route(
			make_fetch_input({
				host: {
					fetch_route_response: async () => {
						throw error;
					},
				},
			}),
		);

		expect(result).toMatchObject({
			error,
			kind: core6_route_fetch_result_kind.failure,
			reason: core6_route_fetch_failure_reason.network_error,
		});
	});

	it("classifies aborted fetch rejection as aborted failure", async () => {
		const controller = new AbortController();
		const result_promise = fetch_core6_route(
			make_fetch_input({
				host: {
					fetch_route_response: async () => {
						controller.abort();
						throw new DOMException("Aborted", "AbortError");
					},
				},
				signal: controller.signal,
			}),
		);

		await expect(result_promise).resolves.toMatchObject({
			kind: core6_route_fetch_result_kind.failure,
			reason: core6_route_fetch_failure_reason.aborted,
		});
	});
});
