import { describe, expect, it } from "vitest";
import {
	BUILD_ID_HEADER,
	CONTENT_TYPE_HEADER,
	JSON_CONTENT_TYPE,
	VERCEL_X_DEPLOYMENT_ID,
	VORMA_PROTOCOL_ENABLED,
	X_ACCEPTS_CLIENT_REDIRECT,
	X_CLIENT_REDIRECT,
} from "../core/constants.ts";
import {
	CORE6_API_SUBMIT_ABORTED_ERROR,
	CORE6_API_SUBMIT_INVALID_REDIRECT_ERROR,
	CORE6_API_SUBMIT_REVALIDATION_OK,
	build_core6_api_submit_request,
	core6_api_submit_build_skew_default_behavior,
	create_core6_api_submit_owner,
	type Core6APISubmitBuildSkewEvent,
	type Core6APISubmitHost,
	type Core6APISubmitOwner,
	type Core6APISubmitRuntime,
} from "./api_submit.ts";
import { create_core6_deferred, type Core6Deferred } from "./deferred.ts";
import type {
	Core6RouteRevalidationRequestInput,
	Core6RouteRevalidationResult,
} from "./route_revalidation.ts";
import { core6_route_revalidation_reason } from "./route_revalidation.ts";
import type { Core6RouteRuntimeRunFetchInput } from "./route_runtime.ts";
import { core6_scope_stale_reason } from "./scope.ts";
import { core6_route_transaction_kind } from "./transaction.ts";

const base_href = "https://example.test/current";
const api_href = "https://example.test/api/action";
const post_method = "POST";
const get_method = "GET";
const deployment_id = "dpl_test_123";

type APIFetchCall = {
	deferred: Core6Deferred<Response>;
	href: string;
	init: RequestInit & { signal: AbortSignal };
};

function json_response(
	data: unknown = { ok: true },
	init: ResponseInit = {},
): Response {
	const headers = new Headers(init.headers);
	headers.set(CONTENT_TYPE_HEADER, JSON_CONTENT_TYPE);
	return new Response(JSON.stringify(data), {
		...init,
		headers,
		status: init.status ?? 200,
	});
}

function redirect_response(href: string, init: ResponseInit = {}): Response {
	const headers = new Headers(init.headers);
	headers.set(X_CLIENT_REDIRECT, href);
	return new Response("", {
		...init,
		headers,
		status: init.status ?? 200,
	});
}

function create_fixture(input: { current_href?: string } = {}): {
	api_fetches: APIFetchCall[];
	build_skews: Core6APISubmitBuildSkewEvent[];
	hard_redirects: string[];
	navigation_fetches: Core6RouteRuntimeRunFetchInput[];
	owner: Core6APISubmitOwner;
	revalidation_requests: Core6RouteRevalidationRequestInput[];
	revalidations: Core6Deferred<Core6RouteRevalidationResult>[];
} {
	const api_fetches: APIFetchCall[] = [];
	const build_skews: Core6APISubmitBuildSkewEvent[] = [];
	const hard_redirects: string[] = [];
	const navigation_fetches: Core6RouteRuntimeRunFetchInput[] = [];
	const revalidation_requests: Core6RouteRevalidationRequestInput[] = [];
	const revalidations: Core6Deferred<Core6RouteRevalidationResult>[] = [];
	const host: Core6APISubmitHost = {
		current_href: () => {
			return input.current_href ?? base_href;
		},
		fetch_api_response: (args) => {
			const deferred = create_core6_deferred<Response>();
			api_fetches.push({ ...args, deferred });
			return deferred.promise;
		},
		hard_redirect: (href) => {
			hard_redirects.push(href);
		},
		notify_api_build_skew: (event) => {
			build_skews.push(event);
		},
	};
	const runtime: Core6APISubmitRuntime = {
		current_route: () => {
			return null;
		},
		publish_same_document: () => {
			throw new Error(
				"api submit should not publish same-document routes",
			);
		},
		run_route_fetch: async (run_input) => {
			navigation_fetches.push(run_input);
			return { ok: false, reason: core6_scope_stale_reason };
		},
		run_route_prepared: () => {
			return { ok: false, reason: core6_scope_stale_reason };
		},
	};
	const owner = create_core6_api_submit_owner({
		active_client_build_id: () => {
			return "build-1";
		},
		deployment_id: () => {
			return deployment_id;
		},
		host,
		revalidation: {
			request: (request_input = {}) => {
				const deferred =
					create_core6_deferred<Core6RouteRevalidationResult>();
				revalidation_requests.push(request_input);
				revalidations.push(deferred);
				return deferred.promise;
			},
		},
		runtime,
	});
	return {
		api_fetches,
		build_skews,
		hard_redirects,
		navigation_fetches,
		owner,
		revalidation_requests,
		revalidations,
	};
}

describe("core6 API submit request", () => {
	it("normalizes method, headers, deployment ID, and JSON object bodies", () => {
		const signal = new AbortController().signal;
		const request = build_core6_api_submit_request({
			base_href,
			deployment_id,
			request_init: {
				body: { a: 1 } as unknown as BodyInit,
				headers: { Accept: "application/json" },
				method: " post ",
			},
			signal,
			url: "/api/action",
		});

		expect(request).toMatchObject({
			href: api_href,
			method: post_method,
			route_kind: "mutation",
			should_revalidate: true,
		});
		if ("kind" in request) {
			throw new Error("expected request");
		}
		const headers = new Headers(request.init.headers);
		expect(headers.get("Accept")).toBe("application/json");
		expect(headers.get(VERCEL_X_DEPLOYMENT_ID)).toBe(deployment_id);
		expect(headers.get(X_ACCEPTS_CLIENT_REDIRECT)).toBe(
			VORMA_PROTOCOL_ENABLED,
		);
		expect(headers.get(CONTENT_TYPE_HEADER)).toBe(JSON_CONTENT_TYPE);
		expect(request.init.body).toBe(JSON.stringify({ a: 1 }));
		expect(request.init.signal).toBe(signal);
	});

	it("strips bodies from GET and HEAD submissions", () => {
		for (const method of [get_method, "HEAD"]) {
			const request = build_core6_api_submit_request({
				base_href,
				request_init: {
					body: "ignored",
					method,
				},
				signal: new AbortController().signal,
				url: api_href,
			});

			if ("kind" in request) {
				throw new Error("expected request");
			}
			expect(request.init.body).toBeUndefined();
			expect(request.route_kind).toBe("query");
			expect(request.should_revalidate).toBe(false);
		}
	});

	it("rejects cross-origin targets before an owner scope starts", async () => {
		const fixture = create_fixture();
		const first = fixture.owner.submit({
			options: { dedupe_key: "save", revalidate: false },
			request_init: { method: post_method },
			url: api_href,
		});
		expect(fixture.api_fetches).toHaveLength(1);

		const second = await fixture.owner.submit({
			options: { dedupe_key: "save", revalidate: false },
			request_init: { method: post_method },
			url: "https://elsewhere.test/api/action",
		});

		expect(second.success).toBe(false);
		if (!second.success) {
			expect(second.error).toContain("https://elsewhere.test/api/action");
			await expect(second.revalidation_promise).resolves.toBe(
				CORE6_API_SUBMIT_REVALIDATION_OK,
			);
		}
		expect(fixture.api_fetches[0]?.init.signal.aborted).toBe(false);
		fixture.api_fetches[0]?.deferred.resolve(json_response());
		await expect(first).resolves.toMatchObject({ success: true });
	});
});

describe("core6 API submit owner", () => {
	it("settles successful mutations and requests immediate revalidation", async () => {
		const fixture = create_fixture();
		const result_promise = fixture.owner.submit<{ ok: true }>({
			request_init: { method: post_method },
			url: "/api/action",
		});

		expect(fixture.owner.current_statuses()).toMatchObject([
			{
				href: api_href,
				method: post_method,
				route_kind: "mutation",
				should_revalidate: true,
			},
		]);
		fixture.api_fetches[0]?.deferred.resolve(json_response({ ok: true }));

		const result = await result_promise;
		expect(result).toMatchObject({
			data: { ok: true },
			success: true,
		});
		expect(fixture.revalidation_requests).toEqual([
			{
				debounce: false,
				reason: core6_route_revalidation_reason.mutation,
				skip_work_indicator: false,
			},
		]);
		fixture.revalidations[0]?.resolve({ ok: true });
		await expect(result.revalidation_promise).resolves.toEqual({
			ok: true,
		});
		expect(fixture.owner.current_statuses()).toEqual([]);
	});

	it("parses text and empty bodies without requesting query revalidation", async () => {
		const fixture = create_fixture();
		const text = fixture.owner.submit<string>({
			request_init: { method: get_method },
			url: "/api/action",
		});
		fixture.api_fetches[0]?.deferred.resolve(
			new Response("plain-ok", { status: 200 }),
		);

		await expect(text).resolves.toMatchObject({
			data: "plain-ok",
			success: true,
		});

		const empty = fixture.owner.submit<undefined>({
			options: { revalidate: false },
			request_init: { method: post_method },
			url: "/api/action",
		});
		fixture.api_fetches[1]?.deferred.resolve(
			new Response(null, { status: 204 }),
		);

		await expect(empty).resolves.toMatchObject({
			data: undefined,
			success: true,
		});
		expect(fixture.revalidation_requests).toEqual([]);
	});

	it("dedupes by key, aborts the replaced owner, and ignores its late redirect", async () => {
		const fixture = create_fixture();
		const first = fixture.owner.submit({
			options: { dedupe_key: "save", revalidate: false },
			request_init: { method: post_method },
			url: "/api/action",
		});
		const second = fixture.owner.submit({
			options: { dedupe_key: "save", revalidate: false },
			request_init: { method: post_method },
			url: "/api/action",
		});

		expect(fixture.api_fetches).toHaveLength(2);
		expect(fixture.api_fetches[0]?.init.signal.aborted).toBe(true);

		await expect(first).resolves.toMatchObject({
			error: CORE6_API_SUBMIT_ABORTED_ERROR,
			success: false,
		});

		fixture.api_fetches[0]?.deferred.resolve(
			redirect_response("https://elsewhere.test/late"),
		);
		fixture.api_fetches[1]?.deferred.resolve(json_response({ ok: true }));

		await expect(second).resolves.toMatchObject({
			data: { ok: true },
			success: true,
		});
		expect(fixture.hard_redirects).toEqual([]);
	});

	it("allows different identities to run concurrently", async () => {
		const fixture = create_fixture();
		const first = fixture.owner.submit({
			options: { revalidate: false },
			request_init: { method: post_method },
			url: "/api/first",
		});
		const second = fixture.owner.submit({
			options: { revalidate: false },
			request_init: { method: post_method },
			url: "/api/second",
		});

		expect(fixture.api_fetches).toHaveLength(2);
		expect(fixture.owner.current_statuses()).toHaveLength(2);

		fixture.api_fetches[1]?.deferred.resolve(json_response({ n: 2 }));
		fixture.api_fetches[0]?.deferred.resolve(json_response({ n: 1 }));

		await expect(first).resolves.toMatchObject({
			data: { n: 1 },
			success: true,
		});
		await expect(second).resolves.toMatchObject({
			data: { n: 2 },
			success: true,
		});
		expect(fixture.owner.current_statuses()).toEqual([]);
	});

	it("converts same-origin submit redirects into navigation work", async () => {
		const fixture = create_fixture();
		const result_promise = fixture.owner.submit({
			options: { revalidate: false },
			request_init: { method: post_method },
			url: "/api/action",
		});

		fixture.api_fetches[0]?.deferred.resolve(
			redirect_response("/target?q=1"),
		);

		await expect(result_promise).resolves.toMatchObject({
			data: undefined,
			success: true,
		});
		await Promise.resolve();
		await Promise.resolve();
		expect(fixture.navigation_fetches).toHaveLength(1);
		expect(fixture.navigation_fetches[0]).toMatchObject({
			intent: {
				href: "https://example.test/target?q=1",
			},
			kind: core6_route_transaction_kind.navigation,
		});
	});

	it("hard redirects cross-origin submit redirects and reports API build skew", async () => {
		const fixture = create_fixture();
		const result_promise = fixture.owner.submit({
			options: { revalidate: false },
			request_init: { method: post_method },
			url: "/api/action",
		});

		fixture.api_fetches[0]?.deferred.resolve(
			redirect_response("https://elsewhere.test/hard", {
				headers: { [BUILD_ID_HEADER]: "build-2" },
			}),
		);

		await expect(result_promise).resolves.toMatchObject({
			data: undefined,
			success: true,
		});
		expect(fixture.hard_redirects).toEqual(["https://elsewhere.test/hard"]);
		expect(fixture.build_skews).toMatchObject([
			{
				active_client_build_id: "build-1",
				default_behavior:
					core6_api_submit_build_skew_default_behavior.hard_reload,
				method: post_method,
				requested_href: api_href,
				route_kind: "mutation",
				server_build_id: "build-2",
				status: 200,
			},
		]);
	});

	it("rejects non-HTTP submit redirects without revalidation", async () => {
		const fixture = create_fixture();
		const result_promise = fixture.owner.submit({
			options: { revalidate: true },
			request_init: { method: post_method },
			url: "/api/action",
		});

		fixture.api_fetches[0]?.deferred.resolve(
			redirect_response("javascript:alert(1)"),
		);

		const result = await result_promise;
		expect(result.success).toBe(false);
		if (!result.success) {
			expect(result.error).toBe(
				`${CORE6_API_SUBMIT_INVALID_REDIRECT_ERROR} "javascript:alert(1)".`,
			);
			await expect(result.revalidation_promise).resolves.toBe(
				CORE6_API_SUBMIT_REVALIDATION_OK,
			);
		}
		expect(fixture.revalidation_requests).toEqual([]);
		expect(fixture.hard_redirects).toEqual([]);
	});

	it("revalidates dispatched mutation failures and aborts", async () => {
		const fixture = create_fixture();
		const failed = fixture.owner.submit({
			request_init: { method: post_method },
			url: "/api/action",
		});
		fixture.api_fetches[0]?.deferred.resolve(
			new Response("", { status: 500, statusText: "Err" }),
		);

		const failure_result = await failed;
		expect(failure_result.success).toBe(false);
		if (!failure_result.success) {
			expect(failure_result.error).toBe("Err");
		}

		const aborted = fixture.owner.submit({
			request_init: { method: post_method },
			url: "/api/action",
		});
		fixture.api_fetches[1]?.deferred.reject(
			new DOMException(CORE6_API_SUBMIT_ABORTED_ERROR, "AbortError"),
		);

		const abort_result = await aborted;
		expect(abort_result.success).toBe(false);
		if (!abort_result.success) {
			expect(abort_result.error).toBe(CORE6_API_SUBMIT_ABORTED_ERROR);
		}

		expect(fixture.revalidation_requests).toHaveLength(2);
		for (const revalidation of fixture.revalidations) {
			revalidation.resolve({ ok: true });
		}
		if (!failure_result.success) {
			await expect(failure_result.revalidation_promise).resolves.toEqual({
				ok: true,
			});
		}
		if (!abort_result.success) {
			await expect(abort_result.revalidation_promise).resolves.toEqual({
				ok: true,
			});
		}
	});
});
