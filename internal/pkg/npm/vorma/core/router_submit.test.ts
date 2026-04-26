// @vitest-environment jsdom

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
	json_response,
	mock_fetch,
	native_redirect_response,
	no_content_response,
	non_ok_redirect_response,
	redirect_response,
	register_ccc_lifecycle,
	route_response,
	setup,
	text_response,
	tick,
} from "./___ccc_test_helpers.ts";
import {
	BUILD_ID_HEADER,
	VERCEL_X_DEPLOYMENT_ID,
	X_CLIENT_REDIRECT,
	X_VORMA_BUILD_SKEW,
} from "./constants.ts";

register_ccc_lifecycle(beforeEach, afterEach);

/////////////////////////////////////////////////////////////////////
/////// Submit
/////////////////////////////////////////////////////////////////////

describe("submit", () => {
	it("returns success with parsed JSON", async () => {
		const { core } = await setup();
		const { call, wait_for } = mock_fetch();

		const sub = core.submit_inner(
			"/api/action",
			{ method: "POST" },
			{
				revalidate: false,
			},
		);
		await wait_for(1);
		call(0).resolve(json_response({ ok: true }));
		const result = await sub;

		expect(result).toMatchObject({ success: true, data: { ok: true } });
	});

	it("sends Vercel deployment ID on action requests when present", async () => {
		const { core } = await setup({
			payload: { DeploymentID: "dpl_test_123" },
		});
		const { call, wait_for } = mock_fetch();

		const sub = core.submit_inner(
			"/api/action",
			{ method: "POST" },
			{
				revalidate: false,
			},
		);
		await wait_for(1);

		const headers = new Headers(call(0).init?.headers);
		expect(headers.get(VERCEL_X_DEPLOYMENT_ID)).toBe("dpl_test_123");

		call(0).resolve(json_response({ ok: true }));
		await sub;
	});

	it("returns success with text for non-JSON responses", async () => {
		const { core } = await setup();
		const { call, wait_for } = mock_fetch();

		const sub = core.submit_inner(
			"/api/action",
			{ method: "POST" },
			{
				revalidate: false,
			},
		);
		await wait_for(1);
		call(0).resolve(text_response("plain-text-ok"));
		const result = await sub;

		expect(result).toMatchObject({ success: true, data: "plain-text-ok" });
	});

	it("returns success with undefined data for 204 responses", async () => {
		const { core } = await setup();
		const { call, wait_for } = mock_fetch();

		const sub = core.submit_inner(
			"/api/action",
			{ method: "POST" },
			{
				revalidate: false,
			},
		);
		await wait_for(1);
		call(0).resolve(no_content_response());
		const result = await sub;

		expect(result).toMatchObject({ success: true, data: undefined });
	});

	it("returns success with undefined for empty non-JSON body", async () => {
		const { core } = await setup();
		const { call, wait_for } = mock_fetch();

		const sub = core.submit_inner(
			"/api/action",
			{ method: "POST" },
			{
				revalidate: false,
			},
		);
		await wait_for(1);
		call(0).resolve(new Response("", { status: 200 }));
		const result = await sub;

		expect(result).toMatchObject({ success: true, data: undefined });
	});

	it("returns failure for non-ok responses", async () => {
		const { core } = await setup();
		const { call, wait_for } = mock_fetch();

		const sub = core.submit_inner(
			"/api/action",
			{ method: "POST" },
			{
				revalidate: false,
			},
		);
		await wait_for(1);
		call(0).resolve(new Response("", { status: 500, statusText: "Err" }));
		const result = await sub;

		expect(result.success).toBe(false);
		if (!result.success) {
			expect(result.error).toBe("Err");
			expect(result.response?.status).toBe(500);
		}
	});

	it("returns Aborted for abort errors", async () => {
		const { core } = await setup();

		vi.spyOn(globalThis, "fetch").mockRejectedValue(
			new DOMException("Aborted", "AbortError"),
		);

		const result = await core.submit_inner(
			"/api/action",
			{ method: "POST" },
			{
				revalidate: false,
			},
		);

		expect(result.success).toBe(false);
		if (!result.success) {
			expect(result.error).toBe("Aborted");
		}
	});

	it("returns failure with preserved error message for network failures", async () => {
		const { core } = await setup();

		vi.spyOn(globalThis, "fetch").mockRejectedValue(
			new Error("network down"),
		);

		const result = await core.submit_inner(
			"/api/action",
			{ method: "POST" },
			{
				revalidate: false,
			},
		);

		expect(result.success).toBe(false);
		if (!result.success) {
			expect(result.error).toBe("network down");
		}
	});

	it("follows redirect even when response is non-ok", async () => {
		const { core, commit } = await setup();
		const { call, wait_for } = mock_fetch();

		const sub = core.submit_inner(
			"/api/action",
			{ method: "POST" },
			{
				revalidate: false,
			},
		);
		await wait_for(1);
		call(0).resolve(
			non_ok_redirect_response({ [X_CLIENT_REDIRECT]: "/target" }),
		);
		const result = await sub;

		expect(result).toMatchObject({ success: true, data: undefined });

		await wait_for(2);
		call(1).resolve(route_response({ MatchedPatterns: ["/target"] }));

		for (let i = 0; i < 50; i++) {
			if (commit.mock.calls.length > 0) {
				break;
			}
			await new Promise((r) => {
				return setTimeout(r, 0);
			});
		}

		expect(commit).toHaveBeenCalled();
	});

	it("dedup aborts previous with same key", async () => {
		const { core } = await setup();
		const { call, wait_for } = mock_fetch();

		const s1 = core.submit_inner(
			"/api/action",
			{ method: "POST" },
			{
				dedupeKey: "k",
				revalidate: false,
			},
		);
		const s2 = core.submit_inner(
			"/api/action",
			{ method: "POST" },
			{
				dedupeKey: "k",
				revalidate: false,
			},
		);

		// s1's fetch was already dispatched (call 0), then aborted; s2 dispatches call 1
		await wait_for(2);
		const r1 = await s1;
		expect(r1.success).toBe(false);
		if (!r1.success) {
			expect(r1.error).toBe("Aborted");
		}

		call(1).resolve(json_response({ ok: true }));
		const r2 = await s2;
		expect(r2).toMatchObject({ success: true, data: { ok: true } });
	});

	it("does not dedup without key", async () => {
		const { core } = await setup();
		const { calls, wait_for } = mock_fetch();

		void core.submit_inner(
			"/api/action",
			{ method: "POST" },
			{
				revalidate: false,
			},
		);
		void core.submit_inner(
			"/api/action",
			{ method: "POST" },
			{
				revalidate: false,
			},
		);

		await wait_for(2);
		expect(calls).toHaveLength(2);
	});

	it("auto-revalidates after settled mutation by default", async () => {
		const { core } = await setup();
		const { calls, call, wait_for } = mock_fetch();

		const sub = core.submit_inner("/api/action", { method: "POST" }, {});
		await wait_for(1);
		call(0).resolve(json_response({ ok: true }));
		const result = await sub;

		await wait_for(2);
		call(1).resolve(route_response({ MatchedPatterns: ["/"] }));
		await expect(result.revalidationPromise).resolves.toEqual({ ok: true });

		expect(result).toMatchObject({ success: true, data: { ok: true } });
		expect(calls).toHaveLength(2);
	});

	it("reports submission as the build skew revalidation reason", async () => {
		const on_build_skew = vi.fn();
		const { core } = await setup({
			init: { onBuildSkewDetected: on_build_skew },
		});
		const { call, wait_for } = mock_fetch();

		const sub = core.submit_inner("/api/action", { method: "POST" }, {});
		await wait_for(1);
		call(0).resolve(json_response({ ok: true }));
		const result = await sub;

		await wait_for(2);
		call(1).resolve(
			redirect_response({
				[BUILD_ID_HEADER]: "build-2",
				[X_VORMA_BUILD_SKEW]: "1",
			}),
		);

		await expect(result.revalidationPromise).resolves.toEqual({
			ok: false,
			reason: "build_skew",
		});
		expect(on_build_skew).toHaveBeenCalledWith(
			expect.objectContaining({
				activeClientBuildID: "build-1",
				serverBuildID: "build-2",
				defaultBehavior: "dropResponse",
				triggeringResponse: expect.objectContaining({
					kind: "route",
					trigger: "revalidation",
					revalidationReason: "submission",
				}),
			}),
		);
	});

	it("auto-revalidates after non-ok mutation response by default", async () => {
		const on_build_skew = vi.fn();
		const { core } = await setup({
			init: { onBuildSkewDetected: on_build_skew },
		});
		const { calls, call, wait_for } = mock_fetch();

		const sub = core.submit_inner("/api/action", { method: "POST" }, {});
		await wait_for(1);
		call(0).resolve(
			new Response("", {
				status: 500,
				statusText: "Err",
				headers: { [BUILD_ID_HEADER]: "build-2" },
			}),
		);
		const result = await sub;

		await wait_for(2);
		call(1).resolve(route_response({ MatchedPatterns: ["/"] }));
		await expect(result.revalidationPromise).resolves.toEqual({ ok: true });

		expect(result.success).toBe(false);
		if (!result.success) {
			expect(result.error).toBe("Err");
		}
		expect(core.getClientBuildID()).toBe("build-1");
		expect(on_build_skew).toHaveBeenCalledWith(
			expect.objectContaining({
				activeClientBuildID: "build-1",
				serverBuildID: "build-2",
				defaultBehavior: "notifyOnly",
				triggeringResponse: expect.objectContaining({
					kind: "action",
					actionKind: "mutation",
					requestedHref: `${window.location.origin}/api/action`,
					method: "POST",
					status: 500,
					ok: false,
				}),
				currentWorkState: expect.objectContaining({
					submissions: [
						expect.objectContaining({
							href: `${window.location.origin}/api/action`,
							method: "POST",
						}),
					],
				}),
			}),
		);
		expect(calls).toHaveLength(2);
	});

	it("reports build skew from successful mutation responses", async () => {
		const on_build_skew = vi.fn();
		const { core } = await setup({
			init: { onBuildSkewDetected: on_build_skew },
		});
		const { call, wait_for } = mock_fetch();

		const sub = core.submit_inner(
			"/api/action",
			{ method: "POST" },
			{ revalidate: false },
		);
		await wait_for(1);
		call(0).resolve(
			new Response(JSON.stringify({ ok: true }), {
				status: 200,
				headers: {
					"Content-Type": "application/json",
					[BUILD_ID_HEADER]: "build-2",
				},
			}),
		);
		const result = await sub;

		expect(result).toMatchObject({ success: true, data: { ok: true } });
		expect(core.getClientBuildID()).toBe("build-1");
		expect(on_build_skew).toHaveBeenCalledWith(
			expect.objectContaining({
				activeClientBuildID: "build-1",
				serverBuildID: "build-2",
				defaultBehavior: "notifyOnly",
				triggeringResponse: expect.objectContaining({
					kind: "action",
					actionKind: "mutation",
					requestedHref: `${window.location.origin}/api/action`,
					method: "POST",
					status: 200,
					ok: true,
				}),
			}),
		);
	});

	it("reports build skew from failed query responses", async () => {
		const on_build_skew = vi.fn();
		const { core } = await setup({
			init: { onBuildSkewDetected: on_build_skew },
		});
		const { call, wait_for } = mock_fetch();

		const sub = core.submit_inner("/api/action", { method: "GET" }, {});
		await wait_for(1);
		call(0).resolve(
			new Response("", {
				status: 500,
				statusText: "Err",
				headers: { [BUILD_ID_HEADER]: "build-2" },
			}),
		);
		const result = await sub;

		expect(result.success).toBe(false);
		expect(core.getClientBuildID()).toBe("build-1");
		expect(on_build_skew).toHaveBeenCalledWith(
			expect.objectContaining({
				activeClientBuildID: "build-1",
				serverBuildID: "build-2",
				defaultBehavior: "notifyOnly",
				triggeringResponse: expect.objectContaining({
					kind: "action",
					actionKind: "query",
					requestedHref: `${window.location.origin}/api/action`,
					method: "GET",
					status: 500,
					ok: false,
				}),
			}),
		);
	});

	it("auto-revalidates after aborted mutation by default", async () => {
		const { core } = await setup();
		const { calls, call, wait_for } = mock_fetch();

		const sub = core.submit_inner("/api/action", { method: "POST" }, {});
		await wait_for(1);
		call(0).reject(new DOMException("Aborted", "AbortError"));
		const result = await sub;

		await wait_for(2);
		call(1).resolve(route_response({ MatchedPatterns: ["/"] }));
		await expect(result.revalidationPromise).resolves.toEqual({ ok: true });

		expect(result.success).toBe(false);
		if (!result.success) {
			expect(result.error).toBe("Aborted");
		}
		expect(calls).toHaveLength(2);
	});

	it("auto-revalidates after network-failed mutation by default", async () => {
		const { core } = await setup();
		const { calls, call, wait_for } = mock_fetch();

		const sub = core.submit_inner("/api/action", { method: "POST" }, {});
		await wait_for(1);
		call(0).reject(new Error("network down"));
		const result = await sub;

		await wait_for(2);
		call(1).resolve(route_response({ MatchedPatterns: ["/"] }));
		await expect(result.revalidationPromise).resolves.toEqual({ ok: true });

		expect(result.success).toBe(false);
		if (!result.success) {
			expect(result.error).toBe("network down");
		}
		expect(calls).toHaveLength(2);
	});

	it("respects revalidate: false", async () => {
		const { core } = await setup();
		const { calls, call, wait_for } = mock_fetch();

		const sub = core.submit_inner(
			"/api/action",
			{ method: "POST" },
			{
				revalidate: false,
			},
		);
		await wait_for(1);
		call(0).resolve(json_response({ ok: true }));
		await sub;

		// No revalidation fetch
		expect(calls).toHaveLength(1);
	});

	it("skips default POST revalidation when actionKind is query", async () => {
		const { core } = await setup();
		const { calls, call, wait_for } = mock_fetch();

		const sub = core.submit_inner(
			"/api/action",
			{ method: "POST" },
			{ actionKind: "query" },
		);
		await wait_for(1);
		call(0).resolve(json_response({ ok: true }));
		const result = await sub;

		expect(result).toMatchObject({ success: true, data: { ok: true } });
		expect(calls).toHaveLength(1);
	});

	it("skips default POST revalidation on non-ok when actionKind is query", async () => {
		const { core } = await setup();
		const { calls, call, wait_for } = mock_fetch();

		const sub = core.submit_inner(
			"/api/action",
			{ method: "POST" },
			{ actionKind: "query" },
		);
		await wait_for(1);
		call(0).resolve(new Response("", { status: 500, statusText: "Err" }));
		const result = await sub;

		expect(result.success).toBe(false);
		if (!result.success) {
			expect(result.error).toBe("Err");
		}
		expect(calls).toHaveLength(1);
	});

	it("lets revalidate: true override query action semantics", async () => {
		const { core } = await setup();
		const { calls, call, wait_for } = mock_fetch();

		const sub = core.submit_inner(
			"/api/action",
			{ method: "POST" },
			{
				actionKind: "query",
				revalidate: true,
			},
		);
		await wait_for(1);
		call(0).resolve(json_response({ ok: true }));
		const result = await sub;

		await wait_for(2);
		call(1).resolve(route_response({ MatchedPatterns: ["/"] }));
		await expect(result.revalidationPromise).resolves.toEqual({ ok: true });

		expect(calls).toHaveLength(2);
	});

	it("lets mutation action semantics override GET default", async () => {
		const { core } = await setup();
		const { calls, call, wait_for } = mock_fetch();

		const sub = core.submit_inner(
			"/api/action",
			{ method: "GET" },
			{ actionKind: "mutation" },
		);
		await wait_for(1);
		call(0).resolve(json_response({ ok: true }));
		const result = await sub;

		await wait_for(2);
		call(1).resolve(route_response({ MatchedPatterns: ["/"] }));
		await expect(result.revalidationPromise).resolves.toEqual({ ok: true });

		expect(calls).toHaveLength(2);
	});

	it("lets mutation action semantics override GET default on non-ok", async () => {
		const { core } = await setup();
		const { calls, call, wait_for } = mock_fetch();

		const sub = core.submit_inner(
			"/api/action",
			{ method: "GET" },
			{ actionKind: "mutation" },
		);
		await wait_for(1);
		call(0).resolve(new Response("", { status: 500, statusText: "Err" }));
		const result = await sub;

		await wait_for(2);
		call(1).resolve(route_response({ MatchedPatterns: ["/"] }));
		await expect(result.revalidationPromise).resolves.toEqual({ ok: true });

		expect(result.success).toBe(false);
		if (!result.success) {
			expect(result.error).toBe("Err");
		}
		expect(calls).toHaveLength(2);
	});

	it("does not unwrap arbitrary JSON payloads with a data field", async () => {
		const { core } = await setup();
		const { call, wait_for } = mock_fetch();

		const sub = core.submit_inner(
			"/api/action",
			{ method: "POST" },
			{
				revalidate: false,
			},
		);
		await wait_for(1);
		call(0).resolve(json_response({ data: { ok: true } }));
		const result = await sub;

		expect(result).toMatchObject({
			success: true,
			data: { data: { ok: true } },
		});
	});

	it("does not auto-revalidate for implicit GET submissions", async () => {
		const { core } = await setup();
		const { calls, call, wait_for } = mock_fetch();

		const sub = core.submit_inner("/api/action", {}, {});
		await wait_for(1);
		call(0).resolve(json_response({ ok: true }));
		await sub;

		expect(calls).toHaveLength(1);
	});

	it("does not auto-revalidate for GET submissions", async () => {
		const { core } = await setup();
		const { calls, call, wait_for } = mock_fetch();

		const sub = core.submit_inner("/api/action", { method: "GET" }, {});
		await wait_for(1);
		call(0).resolve(json_response({ ok: true }));
		await sub;

		expect(calls).toHaveLength(1);
	});

	it("does not auto-revalidate for HEAD submissions", async () => {
		const { core } = await setup();
		const { calls, call, wait_for } = mock_fetch();

		const sub = core.submit_inner("/api/action", { method: "HEAD" }, {});
		await wait_for(1);
		call(0).resolve(json_response({ ok: true }));
		await sub;

		expect(calls).toHaveLength(1);
	});

	it("follows internal soft redirect by navigating", async () => {
		const { core, commit } = await setup();
		const { call, wait_for } = mock_fetch();

		const sub = core.submit_inner(
			"/api/action",
			{ method: "POST" },
			{
				revalidate: false,
			},
		);
		await wait_for(1);
		call(0).resolve(redirect_response({ [X_CLIENT_REDIRECT]: "/target" }));
		const result = await sub;

		expect(result).toMatchObject({ success: true, data: undefined });

		await wait_for(2);
		call(1).resolve(route_response({ MatchedPatterns: ["/target"] }));

		for (let i = 0; i < 50; i++) {
			if (commit.mock.calls.length > 0) {
				break;
			}
			await new Promise((r) => {
				return setTimeout(r, 0);
			});
		}

		expect(commit).toHaveBeenCalled();
	});

	it("hard redirects cross-origin submit redirects", async () => {
		const { core, hard_redirect } = await setup();
		const { call, wait_for } = mock_fetch();

		const sub = core.submit_inner(
			"/api/action",
			{ method: "POST" },
			{
				revalidate: false,
			},
		);
		await wait_for(1);
		call(0).resolve(
			redirect_response({
				[X_CLIENT_REDIRECT]: "https://example.com/hard",
			}),
		);
		await sub;

		expect(hard_redirect).toHaveBeenCalledWith("https://example.com/hard");
	});

	it("does not follow redirect from aborted submit", async () => {
		const { core, hard_redirect } = await setup();
		const { call, wait_for } = mock_fetch();

		const s1 = core.submit_inner(
			"/api/action",
			{ method: "POST" },
			{
				dedupeKey: "k",
				revalidate: false,
			},
		);
		void core.submit_inner(
			"/api/action",
			{ method: "POST" },
			{
				dedupeKey: "k",
				revalidate: false,
			},
		);

		// s1 dispatched call 0 (aborted), s2 dispatched call 1
		await wait_for(2);
		call(1).resolve(json_response({ ok: true }));
		await s1;

		expect(hard_redirect).not.toHaveBeenCalled();
	});

	it("follows native browser redirects on submit", async () => {
		const { core, commit } = await setup();
		const { call, wait_for } = mock_fetch();

		const sub = core.submit_inner(
			"/api/action",
			{ method: "POST" },
			{
				revalidate: false,
			},
		);
		await wait_for(1);
		call(0).resolve(
			native_redirect_response(`${window.location.origin}/native-target`),
		);
		const result = await sub;

		expect(result).toMatchObject({ success: true, data: undefined });

		await wait_for(2);
		call(1).resolve(route_response());

		for (let i = 0; i < 50; i++) {
			if (commit.mock.calls.length > 0) {
				break;
			}
			await new Promise((r) => {
				return setTimeout(r, 0);
			});
		}

		expect(commit).toHaveBeenCalled();
	});

	it("does not fetch for submit redirect to hash-only change", async () => {
		const { core } = await setup();

		// Navigate to /current-page first
		const { call, wait_for } = mock_fetch();
		const nav = core.navigate("/current-page");
		await wait_for(1);
		call(0).resolve(route_response());
		await nav;

		const sub = core.submit_inner(
			"/api/action",
			{ method: "POST" },
			{
				revalidate: false,
			},
		);
		await wait_for(2);
		call(1).resolve(
			redirect_response({
				[X_CLIENT_REDIRECT]: "/current-page#section",
			}),
		);
		await sub;

		// No third fetch for the hash-only redirect
		expect(() => {
			return call(2);
		}).toThrow();
	});

	it("does not re-follow submit redirect to same path", async () => {
		const { core } = await setup();

		// Navigate to /current-page first
		const { call, wait_for } = mock_fetch();
		const nav = core.navigate("/current-page");
		await wait_for(1);
		call(0).resolve(route_response());
		await nav;

		const sub = core.submit_inner(
			"/api/action",
			{ method: "POST" },
			{
				revalidate: false,
			},
		);
		await wait_for(2);
		call(1).resolve(
			redirect_response({ [X_CLIENT_REDIRECT]: "/current-page" }),
		);
		await sub;

		expect(() => {
			return call(2);
		}).toThrow();
		expect(core.getWorkState().navigation !== null).toBe(false);
	});

	it("returns error for non-HTTP redirect schemes on submit", async () => {
		const { core, hard_redirect } = await setup();
		const { call, wait_for } = mock_fetch();

		const sub = core.submit_inner(
			"/api/action",
			{ method: "POST" },
			{
				revalidate: false,
			},
		);
		await wait_for(1);
		call(0).resolve(
			redirect_response({
				[X_CLIENT_REDIRECT]: "javascript:alert(1)",
			}),
		);
		const result = await sub;

		expect(result.success).toBe(false);
		expect(hard_redirect).not.toHaveBeenCalled();
	});
});

/////////////////////////////////////////////////////////////////////
/////// Submit body handling
/////////////////////////////////////////////////////////////////////

describe("submit body handling", () => {
	it("serializes object bodies to JSON with content-type", async () => {
		const { core } = await setup();
		const fetch_spy = vi
			.spyOn(globalThis, "fetch")
			.mockResolvedValue(json_response());

		await core.submit_inner(
			"/api/action",
			{ method: "POST", body: { a: 1 } as unknown as BodyInit },
			{ revalidate: false },
		);

		const init = fetch_spy.mock.calls[0]![1] as RequestInit;
		expect(init.body).toBe(JSON.stringify({ a: 1 }));
		expect(
			new Headers(init.headers as HeadersInit).get("Content-Type"),
		).toBe("application/json");
	});

	it("preserves caller-provided content-type for object bodies", async () => {
		const { core } = await setup();
		const fetch_spy = vi
			.spyOn(globalThis, "fetch")
			.mockResolvedValue(json_response());

		await core.submit_inner(
			"/api/action",
			{
				method: "POST",
				body: { a: 1 } as unknown as BodyInit,
				headers: { "Content-Type": "application/merge-patch+json" },
			},
			{ revalidate: false },
		);

		const init = fetch_spy.mock.calls[0]![1] as RequestInit;
		expect(
			new Headers(init.headers as HeadersInit).get("Content-Type"),
		).toBe("application/merge-patch+json");
	});

	it("strips body for GET and HEAD methods", async () => {
		const { core } = await setup();
		const fetch_spy = vi
			.spyOn(globalThis, "fetch")
			.mockResolvedValue(json_response());

		await core.submit_inner(
			"/api/action",
			{
				method: "GET",
				body: "should-strip",
			},
			{},
		);
		await core.submit_inner(
			"/api/action",
			{
				method: "HEAD",
				body: "should-strip",
			},
			{},
		);
		await core.submit_inner(
			"/api/action",
			{
				body: "should-strip",
			},
			{},
		);

		for (const c of fetch_spy.mock.calls) {
			expect((c[1] as RequestInit).body).toBeUndefined();
		}
	});

	it("preserves FormData bodies without JSON serialization", async () => {
		const { core } = await setup();
		const fetch_spy = vi
			.spyOn(globalThis, "fetch")
			.mockResolvedValue(json_response());
		const form = new FormData();
		form.set("key", "value");

		await core.submit_inner(
			"/api/action",
			{ method: "POST", body: form },
			{ revalidate: false },
		);

		const init = fetch_spy.mock.calls[0]![1] as RequestInit;
		expect(init.body).toBe(form);
	});

	it("preserves URLSearchParams bodies", async () => {
		const { core } = await setup();
		const fetch_spy = vi
			.spyOn(globalThis, "fetch")
			.mockResolvedValue(json_response());
		const params = new URLSearchParams({ a: "1" });

		await core.submit_inner(
			"/api/action",
			{ method: "POST", body: params },
			{ revalidate: false },
		);

		const init = fetch_spy.mock.calls[0]![1] as RequestInit;
		expect(init.body).toBe(params);
	});

	it("preserves Blob, ArrayBuffer, and ArrayBufferView bodies", async () => {
		const { core } = await setup();
		const fetch_spy = vi
			.spyOn(globalThis, "fetch")
			.mockResolvedValue(json_response());

		const blob = new Blob(["blob"], { type: "text/plain" });
		const buffer = new ArrayBuffer(16);
		const view = new Uint8Array([1, 2, 3]);

		await core.submit_inner(
			"/api/blob",
			{ method: "POST", body: blob },
			{
				revalidate: false,
			},
		);
		await core.submit_inner(
			"/api/buffer",
			{ method: "POST", body: buffer },
			{
				revalidate: false,
			},
		);
		await core.submit_inner(
			"/api/view",
			{ method: "POST", body: view },
			{
				revalidate: false,
			},
		);

		expect((fetch_spy.mock.calls[0]![1] as RequestInit).body).toBe(blob);
		expect((fetch_spy.mock.calls[1]![1] as RequestInit).body).toBe(buffer);
		expect((fetch_spy.mock.calls[2]![1] as RequestInit).body).toBe(view);
	});

	it("passes through null bodies", async () => {
		const { core } = await setup();
		const fetch_spy = vi
			.spyOn(globalThis, "fetch")
			.mockResolvedValue(json_response());

		await core.submit_inner(
			"/api/action",
			{ method: "POST", body: null },
			{ revalidate: false },
		);

		expect((fetch_spy.mock.calls[0]![1] as RequestInit).body).toBeNull();
	});

	it("returns text data for response without content-type header", async () => {
		const { core } = await setup();
		const { call, wait_for } = mock_fetch();

		const sub = core.submit_inner(
			"/api/action",
			{ method: "POST" },
			{
				revalidate: false,
			},
		);
		await wait_for(1);
		call(0).resolve(new Response("plain-ok", { status: 200 }));
		const result = await sub;

		expect(result).toMatchObject({ success: true, data: "plain-ok" });
	});
});

/////////////////////////////////////////////////////////////////////
/////// Submit dedupe edge cases
/////////////////////////////////////////////////////////////////////

describe("submit dedupe edge cases", () => {
	it("ignores redirect from late-resolving deduped submit", async () => {
		const { core, hard_redirect } = await setup();
		const { call, wait_for } = mock_fetch();

		const s1 = core.submit_inner(
			"/api/action",
			{ method: "POST" },
			{
				dedupeKey: "k",
				revalidate: false,
			},
		);
		const s2 = core.submit_inner(
			"/api/action",
			{ method: "POST" },
			{
				dedupeKey: "k",
				revalidate: false,
			},
		);

		// s1 dispatched call 0 (aborted), s2 dispatched call 1
		await wait_for(2);
		const r1 = await s1;
		expect(r1.success).toBe(false);

		call(1).resolve(json_response({ winner: true }));
		const r2 = await s2;
		expect(r2).toMatchObject({ success: true, data: { winner: true } });

		expect(hard_redirect).not.toHaveBeenCalled();
	});

	it("clears submitting state when deduped replacement fails", async () => {
		const { core } = await setup();
		const { call, wait_for } = mock_fetch();

		const s1 = core.submit_inner(
			"/api/action",
			{ method: "POST" },
			{
				dedupeKey: "k",
				revalidate: false,
			},
		);
		const s2 = core.submit_inner(
			"/api/action",
			{ method: "POST" },
			{
				dedupeKey: "k",
				revalidate: false,
			},
		);

		// s1 dispatched call 0 (aborted), s2 dispatched call 1
		await wait_for(2);
		call(1).resolve(new Response("failed", { status: 500 }));
		const [r1, r2] = await Promise.all([s1, s2]);

		expect(r1.success).toBe(false);
		expect(r2.success).toBe(false);
		expect(core.getWorkState().submissions.length > 0).toBe(false);
	});

	it("stale deduped mutation still triggers revalidation after abort", async () => {
		const { core } = await setup();
		const { calls, call, wait_for } = mock_fetch();

		const s1 = core.submit_inner(
			"/api/action",
			{ method: "POST" },
			{
				dedupeKey: "k",
			},
		);
		const s2 = core.submit_inner(
			"/api/action",
			{ method: "POST" },
			{
				dedupeKey: "k",
				revalidate: false,
			},
		);

		// s1 dispatched call 0 (aborted), s2 dispatched call 1
		await wait_for(2);
		call(1).resolve(json_response({ ok: true }));
		const [r1, r2] = await Promise.all([s1, s2]);

		await wait_for(3);
		call(2).resolve(route_response({ MatchedPatterns: ["/"] }));
		await expect(r1.revalidationPromise).resolves.toEqual({ ok: true });
		await expect(r2.revalidationPromise).resolves.toEqual({ ok: true });

		expect(r1.success).toBe(false);
		if (!r1.success) {
			expect(r1.error).toBe("Aborted");
		}
		expect(r2).toMatchObject({ success: true, data: { ok: true } });
		expect(calls).toHaveLength(3);
	});
});

/////////////////////////////////////////////////////////////////////
/////// Submit response availability
/////////////////////////////////////////////////////////////////////

describe("submit response availability", () => {
	it("includes response on successful result", async () => {
		const { core } = await setup();
		const { call, wait_for } = mock_fetch();

		const sub = core.submit_inner(
			"/api/action",
			{ method: "POST" },
			{
				revalidate: false,
			},
		);
		await wait_for(1);
		call(0).resolve(
			new Response(JSON.stringify({ ok: true }), {
				status: 200,
				headers: {
					"Content-Type": "application/json",
					"X-Custom": "value",
				},
			}),
		);
		const result = await sub;

		expect(result.success).toBe(true);
		expect(result.response).toBeInstanceOf(Response);
		expect(result.response?.headers.get("X-Custom")).toBe("value");
	});

	it("includes response on non-ok result", async () => {
		const { core } = await setup();
		const { call, wait_for } = mock_fetch();

		const sub = core.submit_inner(
			"/api/action",
			{ method: "POST" },
			{
				revalidate: false,
			},
		);
		await wait_for(1);
		call(0).resolve(
			new Response("Validation failed", {
				status: 422,
				statusText: "Unprocessable Entity",
			}),
		);
		const result = await sub;

		expect(result.success).toBe(false);
		expect(result.response).toBeInstanceOf(Response);
		expect(result.response?.status).toBe(422);
	});

	it("has no response on network failure", async () => {
		const { core } = await setup();

		vi.spyOn(globalThis, "fetch").mockRejectedValue(
			new Error("network down"),
		);

		const result = await core.submit_inner(
			"/api/action",
			{ method: "POST" },
			{
				revalidate: false,
			},
		);

		expect(result.success).toBe(false);
		expect(result.response).toBeUndefined();
	});
});

/////////////////////////////////////////////////////////////////////
/////// Submit error message preservation
/////////////////////////////////////////////////////////////////////

describe("submit error message preservation", () => {
	it("preserves network error message", async () => {
		const { core } = await setup();

		vi.spyOn(globalThis, "fetch").mockRejectedValue(
			new Error("ECONNREFUSED"),
		);

		const result = await core.submit_inner(
			"/api/action",
			{ method: "POST" },
			{
				revalidate: false,
			},
		);

		expect(result.success).toBe(false);
		if (!result.success) {
			expect(result.error).toContain("ECONNREFUSED");
		}
	});

	it("preserves thrown string in submit", async () => {
		const { core } = await setup();

		vi.spyOn(globalThis, "fetch").mockRejectedValue("not-an-error");

		const result = await core.submit_inner(
			"/api/action",
			{ method: "POST" },
			{
				revalidate: false,
			},
		);

		expect(result.success).toBe(false);
		if (!result.success) {
			expect(result.error).toBe("not-an-error");
		}
	});

	it("preserves thrown number in submit", async () => {
		const { core } = await setup();

		vi.spyOn(globalThis, "fetch").mockRejectedValue(42);

		const result = await core.submit_inner(
			"/api/action",
			{ method: "POST" },
			{
				revalidate: false,
			},
		);

		expect(result.success).toBe(false);
		if (!result.success) {
			expect(result.error).toBe("42");
		}
	});
});

/////////////////////////////////////////////////////////////////////
/////// Cross-origin submit rejection
/////////////////////////////////////////////////////////////////////

describe("cross-origin submit rejection", () => {
	it("returns failure for cross-origin submit without fetching", async () => {
		const { core } = await setup();
		const { calls } = mock_fetch();

		const result = await core.submit_inner(
			"https://external.example/api",
			{ method: "POST" },
			{ revalidate: false },
		);

		expect(calls).toHaveLength(0);
		expect(result.success).toBe(false);
	});
});

/////////////////////////////////////////////////////////////////////
/////// Status continuity
/////////////////////////////////////////////////////////////////////

describe("status continuity", () => {
	it("keeps submitting continuous through overlapping non-deduped submissions", async () => {
		const { core } = await setup();
		const { call, wait_for } = mock_fetch();

		void core.submit_inner(
			"/api/a",
			{ method: "POST" },
			{
				revalidate: false,
			},
		);
		void core.submit_inner(
			"/api/b",
			{ method: "POST" },
			{
				revalidate: false,
			},
		);

		expect(core.getWorkState().submissions.length > 0).toBe(true);

		await wait_for(2);
		call(0).resolve(json_response());
		await tick();
		expect(core.getWorkState().submissions.length > 0).toBe(true);

		call(1).resolve(json_response());
		await tick();

		for (let i = 0; i < 10; i++) {
			if (core.getWorkState().submissions.length === 0) {
				break;
			}
			await new Promise((r) => {
				return setTimeout(r, 0);
			});
		}

		expect(core.getWorkState().submissions.length > 0).toBe(false);
	});

	it("keeps submitting continuous through same-key dedupe handoff", async () => {
		const { core } = await setup();
		const { call, wait_for } = mock_fetch();

		const s1 = core.submit_inner(
			"/api/action",
			{ method: "POST" },
			{
				dedupeKey: "k",
				revalidate: false,
			},
		);
		const s2 = core.submit_inner(
			"/api/action",
			{ method: "POST" },
			{
				dedupeKey: "k",
				revalidate: false,
			},
		);

		expect(core.getWorkState().submissions.length > 0).toBe(true);

		await s1;
		expect(core.getWorkState().submissions.length > 0).toBe(true);

		// s1 dispatched call 0 (aborted), s2 dispatched call 1
		await wait_for(2);
		call(1).resolve(json_response());
		await s2;

		for (let i = 0; i < 10; i++) {
			if (core.getWorkState().submissions.length === 0) {
				break;
			}
			await new Promise((r) => {
				return setTimeout(r, 0);
			});
		}

		expect(core.getWorkState().submissions.length > 0).toBe(false);
	});

	it("keeps loading continuous from submit into auto-revalidate", async () => {
		const { core } = await setup();
		const { call, wait_for } = mock_fetch();

		const sub = core.submit_inner("/api/action", { method: "POST" }, {});
		await wait_for(1);
		call(0).resolve(json_response({ ok: true }));
		const result = await sub;

		expect(core.getWorkState().revalidation !== null).toBe(true);

		await wait_for(2);
		call(1).resolve(route_response());
		await result.revalidationPromise;

		expect(core.getWorkState().revalidation !== null).toBe(false);
	});
});

/////////////////////////////////////////////////////////////////////
/////// revalidationPromise
/////////////////////////////////////////////////////////////////////

describe("revalidationPromise", () => {
	it("resolves immediately for non-mutating submissions", async () => {
		const { core } = await setup();
		const { call, wait_for } = mock_fetch();

		const sub = core.submit_inner("/api/action", { method: "GET" }, {});
		await wait_for(1);
		call(0).resolve(json_response({ ok: true }));
		const result = await sub;

		await expect(result.revalidationPromise).resolves.toEqual({ ok: true });
	});

	it("resolves immediately when revalidate is false", async () => {
		const { core } = await setup();
		const { call, wait_for } = mock_fetch();

		const sub = core.submit_inner(
			"/api/action",
			{ method: "POST" },
			{
				revalidate: false,
			},
		);
		await wait_for(1);
		call(0).resolve(json_response({ ok: true }));
		const result = await sub;

		await expect(result.revalidationPromise).resolves.toEqual({ ok: true });
	});

	it("resolves after system revalidation completes", async () => {
		const { core } = await setup();
		const { call, wait_for } = mock_fetch();

		const sub = core.submit_inner("/api/action", { method: "POST" }, {});
		await wait_for(1);
		call(0).resolve(json_response({ ok: true }));
		const result = await sub;

		let resolved = false;
		let revalidation_result: unknown;
		void result.revalidationPromise.then((value) => {
			resolved = true;
			revalidation_result = value;
		});

		await wait_for(2);
		expect(resolved).toBe(false);

		call(1).resolve(route_response());

		for (let i = 0; i < 50; i++) {
			if (resolved) {
				break;
			}
			await new Promise((r) => {
				return setTimeout(r, 0);
			});
		}

		expect(resolved).toBe(true);
		expect(revalidation_result).toEqual({ ok: true });
	});

	it("resolves when a user navigation satisfies freshness", async () => {
		const { core } = await setup();
		const { call, wait_for } = mock_fetch();

		const sub = core.submit_inner("/api/action", { method: "POST" }, {});
		await wait_for(1);
		call(0).resolve(json_response({ ok: true }));
		const result = await sub;

		// System revalidation fires
		await wait_for(2);

		let resolved = false;
		let revalidation_result: unknown;
		void result.revalidationPromise.then((value) => {
			resolved = true;
			revalidation_result = value;
		});

		// User navigates — aborts system revalidation, starts own fetch
		const nav = core.navigate("/other-page");
		await wait_for(3);

		expect(resolved).toBe(false);

		call(2).resolve(route_response());
		await nav;

		for (let i = 0; i < 50; i++) {
			if (resolved) {
				break;
			}
			await new Promise((r) => {
				return setTimeout(r, 0);
			});
		}

		expect(resolved).toBe(true);
		expect(revalidation_result).toEqual({ ok: true });
	});
});
