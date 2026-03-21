// Assertions covered: 17 (submit terminal outcomes), 46, 48

import { beforeEach, describe, expect, it, vi } from "vitest";
import {
	reset_client_runtime_for_testing,
	set_hard_redirect_handler_for_testing,
} from "vorma/testing";
import {
	collect_status_snapshots,
	create_abort_aware_fetch_recorder,
	create_deferred,
	create_route_data_response,
	expect_no_loading_gap,
	expect_status_idle,
	load_client,
	request_input_to_url,
	wait_for_request_count,
	with_unhandled_rejection_capture,
} from "./setup.ts";

beforeEach(() => {
	reset_client_runtime_for_testing();
});

describe("submit", () => {
	// ─── Success / Failure Decoding (Assertion 46) ───────────

	describe("result decoding", () => {
		it("returns parsed json for successful json responses", async () => {
			const client = await load_client();
			vi.spyOn(window, "fetch").mockResolvedValue(
				new Response(JSON.stringify({ ok: true }), {
					status: 200,
					headers: { "Content-Type": "application/json" },
				}),
			);

			const result = await client.submit<{ ok: boolean }>(
				"/api/success",
				{ method: "POST", body: JSON.stringify({ value: 1 }) },
				{ revalidate: false },
			);
			await vi.runAllTimersAsync();

			expect(result).toEqual({ success: true, data: { ok: true } });
			expect_status_idle(client.getStatus());
		});

		it("returns text body for successful non-json responses", async () => {
			const client = await load_client();
			vi.spyOn(window, "fetch").mockResolvedValue(
				new Response("plain-text-ok", {
					status: 200,
					headers: { "Content-Type": "text/plain" },
				}),
			);

			const result = await client.submit<string>(
				"/api/text",
				{ method: "POST" },
				{ revalidate: false },
			);
			await vi.runAllTimersAsync();

			expect(result).toEqual({ success: true, data: "plain-text-ok" });
		});

		it("returns text data for responses without content-type", async () => {
			const client = await load_client();
			const response = new Response(
				new Uint8Array([112, 108, 97, 105, 110, 45, 111, 107]),
				{ status: 200 },
			);
			expect(response.headers.get("Content-Type")).toBeNull();
			vi.spyOn(window, "fetch").mockResolvedValue(response);

			const result = await client.submit<string>(
				"/api/no-content-type",
				{ method: "POST" },
				{ revalidate: false },
			);
			await vi.runAllTimersAsync();

			expect(result).toEqual({ success: true, data: "plain-ok" });
		});

		it("returns success with undefined data for 204 responses", async () => {
			const client = await load_client();
			vi.spyOn(window, "fetch").mockResolvedValue(
				new Response(null, { status: 204 }),
			);

			const result = await client.submit(
				"/api/no-content",
				{ method: "POST" },
				{ revalidate: false },
			);
			await vi.runAllTimersAsync();

			expect(result).toEqual({ success: true, data: undefined });
		});

		it("returns success with undefined data for null-body 200 without content-type", async () => {
			const client = await load_client();
			vi.spyOn(window, "fetch").mockResolvedValue(
				new Response(null, { status: 200 }),
			);

			const result = await client.submit(
				"/api/null-body",
				{ method: "POST" },
				{ revalidate: false },
			);
			await vi.runAllTimersAsync();

			expect(result).toEqual({ success: true, data: undefined });
			expect_status_idle(client.getStatus());
		});

		it("returns failure with response text for non-ok responses", async () => {
			const client = await load_client();
			vi.spyOn(window, "fetch").mockResolvedValue(
				new Response("boom", {
					status: 500,
					headers: { "Content-Type": "text/plain" },
				}),
			);

			const result = await client.submit(
				"/api/failure",
				{ method: "POST" },
				{ revalidate: false },
			);
			await vi.runAllTimersAsync();

			expect(result.success).toBe(false);
			if (!result.success) {
				expect(result.error).toContain("boom");
			}
			expect_status_idle(client.getStatus());
		});

		it("returns failure with status code for empty-body non-ok responses", async () => {
			const client = await load_client();
			vi.spyOn(window, "fetch").mockResolvedValue(
				new Response(null, {
					status: 500,
					statusText: "Internal Server Error",
				}),
			);

			const result = await client.submit(
				"/api/status-failure",
				{ method: "POST" },
				{ revalidate: false },
			);
			await vi.runAllTimersAsync();

			expect(result.success).toBe(false);
			if (!result.success) {
				expect(result.error).toContain("500");
			}
		});

		it("returns failure with status code when body matches statusText", async () => {
			const client = await load_client();
			vi.spyOn(window, "fetch").mockResolvedValue(
				new Response("Internal Server Error", {
					status: 500,
					statusText: "Internal Server Error",
				}),
			);

			const result = await client.submit(
				"/api/generic-body",
				{ method: "POST" },
				{ revalidate: false },
			);
			await vi.runAllTimersAsync();

			expect(result).toEqual({ success: false, error: "500" });
		});

		it("returns failure with error message for network errors", async () => {
			const client = await load_client();
			vi.spyOn(window, "fetch").mockRejectedValue(
				new Error("Network failure"),
			);

			const result = await client.submit(
				"/api/network-failure",
				{ method: "POST" },
				{ revalidate: false },
			);
			await vi.runAllTimersAsync();

			expect(result).toEqual({
				success: false,
				error: "Network failure",
			});
		});

		it("returns failure with stringified value for thrown primitives", async () => {
			const client = await load_client();
			vi.spyOn(window, "fetch").mockRejectedValue("not-an-error");

			const result = await client.submit(
				"/api/thrown-primitive",
				{ method: "POST" },
				{ revalidate: false },
			);
			await vi.runAllTimersAsync();

			expect(result).toEqual({
				success: false,
				error: "not-an-error",
			});
		});

		it("returns Aborted for abort errors", async () => {
			const client = await load_client();
			const abort_error = new Error("Aborted");
			abort_error.name = "AbortError";
			vi.spyOn(window, "fetch").mockRejectedValue(abort_error);

			const result = await client.submit(
				"/api/aborted",
				{ method: "POST" },
				{ revalidate: false },
			);
			await vi.runAllTimersAsync();

			expect(result).toEqual({ success: false, error: "Aborted" });
			expect_status_idle(client.getStatus());
		});
	});

	// ─── Body Resolution (Assertion 46) ──────────────────────

	describe("body resolution", () => {
		it("serializes object bodies to json with content-type header", async () => {
			const client = await load_client();
			const fetch_spy = vi.spyOn(window, "fetch").mockResolvedValue(
				new Response(JSON.stringify({ ok: true }), {
					status: 200,
					headers: { "Content-Type": "application/json" },
				}),
			);

			await client.submit(
				"/api/object-body",
				{ method: "POST", body: { a: 1 } as unknown as BodyInit },
				{ revalidate: false },
			);
			await vi.runAllTimersAsync();

			const init = fetch_spy.mock.calls[0]![1] as RequestInit;
			expect(init.body).toBe(JSON.stringify({ a: 1 }));
			expect(new Headers(init.headers).get("content-type")).toContain(
				"application/json",
			);
		});

		it("preserves URLSearchParams bodies", async () => {
			const client = await load_client();
			const fetch_spy = vi.spyOn(window, "fetch").mockResolvedValue(
				new Response(JSON.stringify({ ok: true }), {
					status: 200,
					headers: { "Content-Type": "application/json" },
				}),
			);
			const body = new URLSearchParams({ a: "1" });

			await client.submit(
				"/api/urlsearchparams",
				{ method: "POST", body },
				{ revalidate: false },
			);
			await vi.runAllTimersAsync();

			const init = fetch_spy.mock.calls[0]![1] as RequestInit;
			expect(init.body).toBe(body);
		});

		it("passes through Blob, ArrayBuffer, and ArrayBufferView bodies", async () => {
			const client = await load_client();
			const fetch_spy = vi.spyOn(window, "fetch").mockResolvedValue(
				new Response(JSON.stringify({ ok: true }), {
					status: 200,
					headers: { "Content-Type": "application/json" },
				}),
			);

			const blob_body = new Blob(["blob"], { type: "text/plain" });
			const buffer_body = new ArrayBuffer(16);
			const view_body = new Uint8Array([1, 2, 3]);

			await client.submit(
				"/api/blob",
				{ method: "POST", body: blob_body },
				{ revalidate: false },
			);
			await client.submit(
				"/api/buffer",
				{ method: "POST", body: buffer_body },
				{ revalidate: false },
			);
			await client.submit(
				"/api/view",
				{ method: "POST", body: view_body },
				{ revalidate: false },
			);
			await vi.runAllTimersAsync();

			expect((fetch_spy.mock.calls[0]![1] as RequestInit).body).toBe(
				blob_body,
			);
			expect((fetch_spy.mock.calls[1]![1] as RequestInit).body).toBe(
				buffer_body,
			);
			expect((fetch_spy.mock.calls[2]![1] as RequestInit).body).toBe(
				view_body,
			);
		});

		it("omits body for GET and HEAD submits", async () => {
			const client = await load_client();
			const fetch_spy = vi.spyOn(window, "fetch").mockResolvedValue(
				new Response(JSON.stringify({ ok: true }), {
					status: 200,
					headers: { "Content-Type": "application/json" },
				}),
			);

			await client.submit("/api/get", {
				method: "GET",
				body: "should-be-omitted",
			});
			await client.submit("/api/head", {
				method: "HEAD",
				body: "should-be-omitted",
			});
			await client.submit("/api/implicit-get", {
				body: "should-be-omitted",
			});
			await vi.runAllTimersAsync();

			expect(
				(fetch_spy.mock.calls[0]![1] as RequestInit).body,
			).toBeUndefined();
			expect(
				(fetch_spy.mock.calls[1]![1] as RequestInit).body,
			).toBeUndefined();
			expect(
				(fetch_spy.mock.calls[2]![1] as RequestInit).body,
			).toBeUndefined();
		});

		it("passes through null bodies", async () => {
			const client = await load_client();
			const fetch_spy = vi.spyOn(window, "fetch").mockResolvedValue(
				new Response(JSON.stringify({ ok: true }), {
					status: 200,
					headers: { "Content-Type": "application/json" },
				}),
			);

			await client.submit(
				"/api/null-body",
				{ method: "POST", body: null },
				{ revalidate: false },
			);
			await vi.runAllTimersAsync();

			expect(
				(fetch_spy.mock.calls[0]![1] as RequestInit).body,
			).toBeNull();
		});

		it("preserves caller-provided content-type for object bodies", async () => {
			const client = await load_client();
			const fetch_spy = vi.spyOn(window, "fetch").mockResolvedValue(
				new Response(JSON.stringify({ ok: true }), {
					status: 200,
					headers: { "Content-Type": "application/json" },
				}),
			);

			await client.submit(
				"/api/custom-ct",
				{
					method: "POST",
					body: { a: 1 } as unknown as BodyInit,
					headers: {
						"Content-Type": "application/merge-patch+json",
					},
				},
				{ revalidate: false },
			);
			await vi.runAllTimersAsync();

			const init = fetch_spy.mock.calls[0]![1] as RequestInit;
			expect(new Headers(init.headers).get("content-type")).toBe(
				"application/merge-patch+json",
			);
			expect(init.body).toBe(JSON.stringify({ a: 1 }));
		});
	});

	// ─── Auto-Revalidation ───────────────────────────────────

	describe("auto-revalidation", () => {
		it("auto-revalidates after non-GET submissions by default", async () => {
			const client = await load_client();
			const fetch_spy = vi
				.spyOn(window, "fetch")
				.mockResolvedValueOnce(
					new Response(JSON.stringify({ ok: true }), {
						status: 200,
						headers: { "Content-Type": "application/json" },
					}),
				)
				.mockResolvedValueOnce(create_route_data_response());

			const result = await client.submit<{ ok: boolean }>(
				"/api/auto-revalidate",
				{ method: "POST" },
			);
			await vi.runAllTimersAsync();

			expect(result).toEqual({ success: true, data: { ok: true } });
			expect(fetch_spy).toHaveBeenCalledTimes(2);
		});

		it("does not auto-revalidate after GET submissions", async () => {
			const client = await load_client();
			const fetch_spy = vi.spyOn(window, "fetch").mockResolvedValue(
				new Response(JSON.stringify({ ok: true }), {
					status: 200,
					headers: { "Content-Type": "application/json" },
				}),
			);

			await client.submit<{ ok: boolean }>("/api/get-no-revalidate", {
				method: "GET",
			});
			await vi.runAllTimersAsync();

			expect(fetch_spy).toHaveBeenCalledTimes(1);
		});

		it("keeps loading continuous from submit into auto-revalidate", async () => {
			const client = await load_client();
			const { statuses, cleanup } = collect_status_snapshots(client);
			vi.spyOn(window, "fetch")
				.mockResolvedValueOnce(
					new Response(JSON.stringify({ ok: true }), {
						status: 200,
						headers: { "Content-Type": "application/json" },
					}),
				)
				.mockResolvedValueOnce(
					create_route_data_response({
						title: { dangerousInnerHTML: "After Revalidate" },
					}),
				);

			try {
				await client.submit<{ ok: boolean }>(
					"/api/continuous",
					{ method: "POST" },
					{ revalidate: true },
				);
				await vi.runAllTimersAsync();

				expect(statuses.some((s) => s.isSubmitting)).toBe(true);
				expect(statuses.some((s) => s.isRevalidating)).toBe(true);
				expect_no_loading_gap(statuses);
				expect_status_idle(statuses.at(-1));
			} finally {
				cleanup();
			}
		});
	});

	// ─── Dedupe (Assertion 17) ───────────────────────────────

	describe("deduplication", () => {
		it("aborts the first submission when a second uses the same dedupe key", async () => {
			const client = await load_client();
			const first_deferred = create_deferred<Response>();
			let first_signal: AbortSignal | undefined;
			vi.spyOn(window, "fetch")
				.mockImplementationOnce(
					(_input: RequestInfo | URL, init?: RequestInit) => {
						first_signal = init?.signal ?? undefined;
						return first_deferred.promise;
					},
				)
				.mockResolvedValueOnce(
					new Response(JSON.stringify({ ok: true }), {
						status: 200,
						headers: { "Content-Type": "application/json" },
					}),
				);

			const first = client.submit(
				"/api/dedupe",
				{ method: "POST" },
				{ dedupeKey: "same-key", revalidate: false },
			);
			const second = client.submit(
				"/api/dedupe",
				{ method: "POST" },
				{ dedupeKey: "same-key", revalidate: false },
			);

			expect(first_signal?.aborted).toBe(true);
			first_deferred.reject(new DOMException("Aborted", "AbortError"));

			const [first_result, second_result] = await Promise.all([
				first,
				second,
			]);
			await vi.runAllTimersAsync();

			expect(first_result).toEqual({
				success: false,
				error: "Aborted",
			});
			expect(second_result).toEqual({
				success: true,
				data: { ok: true },
			});
			expect_status_idle(client.getStatus());
		});

		it("keeps submitting state continuous through same-key handoff", async () => {
			const client = await load_client();
			const { statuses, cleanup } = collect_status_snapshots(client);
			const first_deferred = create_deferred<Response>();
			const second_deferred = create_deferred<Response>();
			vi.spyOn(window, "fetch")
				.mockImplementationOnce(
					(_input: RequestInfo | URL, init?: RequestInit) => {
						const signal = init?.signal ?? undefined;
						signal?.addEventListener(
							"abort",
							() => {
								first_deferred.reject(
									new DOMException("Aborted", "AbortError"),
								);
							},
							{ once: true },
						);
						return first_deferred.promise;
					},
				)
				.mockImplementationOnce(() => second_deferred.promise);

			try {
				const first = client.submit(
					"/api/dedupe-status",
					{ method: "POST" },
					{ dedupeKey: "same-key", revalidate: false },
				);
				const second = client.submit(
					"/api/dedupe-status",
					{ method: "POST" },
					{ dedupeKey: "same-key", revalidate: false },
				);

				await first;

				second_deferred.resolve(
					new Response(JSON.stringify({ ok: true }), {
						status: 200,
						headers: { "Content-Type": "application/json" },
					}),
				);
				await second;
				await vi.runAllTimersAsync();

				expect(statuses.some((s) => s.isSubmitting)).toBe(true);
				expect_no_loading_gap(statuses);
				expect_status_idle(client.getStatus());
			} finally {
				cleanup();
			}
		});

		it("does not deduplicate submissions with different dedupe keys", async () => {
			const client = await load_client();
			const first_deferred = create_deferred<Response>();
			let first_signal: AbortSignal | undefined;
			vi.spyOn(window, "fetch")
				.mockImplementationOnce(
					(_input: RequestInfo | URL, init?: RequestInit) => {
						first_signal = init?.signal ?? undefined;
						return first_deferred.promise;
					},
				)
				.mockResolvedValueOnce(
					new Response(JSON.stringify({ ok: "second" }), {
						status: 200,
						headers: { "Content-Type": "application/json" },
					}),
				);

			const first = client.submit(
				"/api/diff-keys",
				{ method: "POST" },
				{ dedupeKey: "A", revalidate: false },
			);
			const second = client.submit(
				"/api/diff-keys",
				{ method: "POST" },
				{ dedupeKey: "B", revalidate: false },
			);

			expect(first_signal?.aborted).toBe(false);
			first_deferred.resolve(
				new Response(JSON.stringify({ ok: "first" }), {
					status: 200,
					headers: { "Content-Type": "application/json" },
				}),
			);

			const [first_result, second_result] = await Promise.all([
				first,
				second,
			]);
			await vi.runAllTimersAsync();

			expect(first_result).toEqual({
				success: true,
				data: { ok: "first" },
			});
			expect(second_result).toEqual({
				success: true,
				data: { ok: "second" },
			});
			expect_status_idle(client.getStatus());
		});

		it("does not deduplicate when no dedupe key is provided", async () => {
			const client = await load_client();
			const first_deferred = create_deferred<Response>();
			let first_signal: AbortSignal | undefined;
			vi.spyOn(window, "fetch")
				.mockImplementationOnce(
					(_input: RequestInfo | URL, init?: RequestInit) => {
						first_signal = init?.signal ?? undefined;
						return first_deferred.promise;
					},
				)
				.mockResolvedValueOnce(
					new Response(JSON.stringify({ ok: "second" }), {
						status: 200,
						headers: { "Content-Type": "application/json" },
					}),
				);

			const first = client.submit(
				"/api/no-dedupe",
				{ method: "POST" },
				{ revalidate: false },
			);
			const second = client.submit(
				"/api/no-dedupe",
				{ method: "POST" },
				{ revalidate: false },
			);

			expect(first_signal?.aborted).toBe(false);
			first_deferred.resolve(
				new Response(JSON.stringify({ ok: "first" }), {
					status: 200,
					headers: { "Content-Type": "application/json" },
				}),
			);

			const [first_result, second_result] = await Promise.all([
				first,
				second,
			]);
			await vi.runAllTimersAsync();

			expect(first_result).toEqual({
				success: true,
				data: { ok: "first" },
			});
			expect(second_result).toEqual({
				success: true,
				data: { ok: "second" },
			});
		});

		it("aborts stale deduped submits during json parsing", async () => {
			const client = await load_client();
			const first_json_deferred = create_deferred<{ ok: string }>();
			const first_response = new Response(
				JSON.stringify({ ok: "stale" }),
				{
					status: 200,
					headers: { "Content-Type": "application/json" },
				},
			);
			vi.spyOn(first_response, "json").mockImplementation(
				() => first_json_deferred.promise,
			);
			vi.spyOn(window, "fetch")
				.mockResolvedValueOnce(first_response)
				.mockResolvedValueOnce(
					new Response(JSON.stringify({ ok: "fresh" }), {
						status: 200,
						headers: { "Content-Type": "application/json" },
					}),
				);

			const first = client.submit(
				"/api/json-parse",
				{ method: "POST" },
				{ dedupeKey: "same-key", revalidate: false },
			);
			await Promise.resolve();
			const second = client.submit(
				"/api/json-parse",
				{ method: "POST" },
				{ dedupeKey: "same-key", revalidate: false },
			);

			const second_result = await second;
			expect(second_result).toEqual({
				success: true,
				data: { ok: "fresh" },
			});

			first_json_deferred.resolve({ ok: "stale" });
			const first_result = await first;
			await vi.runAllTimersAsync();

			expect(first_result).toEqual({
				success: false,
				error: "Aborted",
			});
			expect_status_idle(client.getStatus());
		});

		it("clears submitting state when deduped replacement fails", async () => {
			const client = await load_client();
			const first_deferred = create_deferred<Response>();
			vi.spyOn(window, "fetch")
				.mockImplementationOnce(
					(_input: RequestInfo | URL, init?: RequestInit) => {
						const signal = init?.signal ?? undefined;
						signal?.addEventListener(
							"abort",
							() => {
								first_deferred.reject(
									new DOMException("Aborted", "AbortError"),
								);
							},
							{ once: true },
						);
						return first_deferred.promise;
					},
				)
				.mockResolvedValueOnce(
					new Response("replacement failed", { status: 500 }),
				);

			const first = client.submit(
				"/api/dedupe-fail",
				{ method: "POST" },
				{ dedupeKey: "same-key" },
			);
			const second = client.submit(
				"/api/dedupe-fail",
				{ method: "POST" },
				{ dedupeKey: "same-key" },
			);

			const [first_result, second_result] = await Promise.all([
				first,
				second,
			]);
			await vi.runAllTimersAsync();

			expect(first_result).toEqual({
				success: false,
				error: "Aborted",
			});
			expect(second_result.success).toBe(false);
			expect_status_idle(client.getStatus());
		});
	});

	// ─── Stale Dedupe Side Effects (Assertion 48) ────────────

	describe("stale dedupe side effects", () => {
		it("ignores redirect from late stale deduped submit", async () => {
			const client = await load_client();
			window.history.replaceState({}, "", "/dedupe-stale-base");
			const first_deferred = create_deferred<Response>();
			const fetch_spy = vi
				.spyOn(window, "fetch")
				.mockImplementationOnce(() => first_deferred.promise)
				.mockResolvedValueOnce(
					new Response(JSON.stringify({ ok: "winner" }), {
						status: 200,
						headers: { "Content-Type": "application/json" },
					}),
				);

			const first = client.submit(
				"/api/stale-redirect",
				{ method: "POST" },
				{ dedupeKey: "same-key", revalidate: false },
			);
			const second = client.submit(
				"/api/stale-redirect",
				{ method: "POST" },
				{ dedupeKey: "same-key", revalidate: false },
			);
			await second;

			first_deferred.resolve(
				new Response("", {
					status: 200,
					headers: {
						"X-Client-Redirect": "/dedupe-stale-target",
					},
				}),
			);
			await first;
			await vi.runAllTimersAsync();

			expect(fetch_spy).toHaveBeenCalledTimes(2);
			expect(window.location.pathname).toBe("/dedupe-stale-base");
			expect_status_idle(client.getStatus());
		});

		it("does not allow stale deduped submit to change build id or trigger reload", async () => {
			const client = await load_client();
			const build_id_events: { newClientBuildID: string }[] = [];
			const remove_listener = client.addClientBuildIDListener(
				(
					event: CustomEvent<{
						oldClientBuildID: string;
						newClientBuildID: string;
					}>,
				) => {
					build_id_events.push(event.detail);
				},
			);
			const first_deferred = create_deferred<Response>();
			vi.spyOn(window, "fetch")
				.mockImplementationOnce(() => first_deferred.promise)
				.mockResolvedValueOnce(
					new Response(JSON.stringify({ ok: "winner" }), {
						status: 200,
						headers: { "Content-Type": "application/json" },
					}),
				);
			const hard_redirect_spy = vi.fn();
			set_hard_redirect_handler_for_testing(hard_redirect_spy);

			try {
				const first = client.submit(
					"/api/stale-build",
					{ method: "POST" },
					{ dedupeKey: "same-key", revalidate: false },
				);
				const second = client.submit(
					"/api/stale-build",
					{ method: "POST" },
					{ dedupeKey: "same-key", revalidate: false },
				);
				await second;

				first_deferred.resolve(
					create_route_data_response(
						{},
						{
							headers: {
								"X-Wave-Framework-Reload": "/dedupe-stale-hard",
								"X-Wave-Framework-Build-Id":
									"stale-dedupe-build",
							},
						},
					),
				);
				await first;
				await vi.runAllTimersAsync();

				expect(hard_redirect_spy).not.toHaveBeenCalled();
				expect(client.getClientBuildID()).toBe("1");
				expect(build_id_events).toEqual([]);
				expect_status_idle(client.getStatus());
			} finally {
				set_hard_redirect_handler_for_testing(undefined);
				remove_listener();
			}
		});

		it("does not trigger revalidation from late stale deduped submit", async () => {
			const client = await load_client();
			const first_deferred = create_deferred<Response>();
			const fetch_spy = vi
				.spyOn(window, "fetch")
				.mockImplementationOnce(() => first_deferred.promise)
				.mockResolvedValueOnce(
					new Response(JSON.stringify({ ok: "winner" }), {
						status: 200,
						headers: { "Content-Type": "application/json" },
					}),
				);

			const first = client.submit(
				"/api/stale-revalidate",
				{ method: "POST" },
				{ dedupeKey: "same-key" },
			);
			const second = client.submit(
				"/api/stale-revalidate",
				{ method: "POST" },
				{ dedupeKey: "same-key", revalidate: false },
			);
			await second;

			first_deferred.resolve(
				new Response(JSON.stringify({ ok: "stale" }), {
					status: 200,
					headers: { "Content-Type": "application/json" },
				}),
			);
			await first;
			await vi.runAllTimersAsync();

			expect(fetch_spy).toHaveBeenCalledTimes(2);
			expect_status_idle(client.getStatus());
		});
	});

	// ─── Overlapping Submissions Status ──────────────────────

	describe("overlapping submissions", () => {
		it("keeps submitting state continuous for overlapping submissions", async () => {
			const client = await load_client();
			const { statuses, cleanup } = collect_status_snapshots(client);
			const first_deferred = create_deferred<Response>();
			const second_deferred = create_deferred<Response>();
			vi.spyOn(window, "fetch")
				.mockImplementationOnce(() => first_deferred.promise)
				.mockImplementationOnce(() => second_deferred.promise);

			try {
				const first = client.submit(
					"/api/first",
					{ method: "POST" },
					{ revalidate: false },
				);
				await vi.advanceTimersByTimeAsync(8);
				expect(client.getStatus().isSubmitting).toBe(true);

				const second = client.submit(
					"/api/second",
					{ method: "POST" },
					{ revalidate: false },
				);

				first_deferred.resolve(
					new Response(JSON.stringify({ ok: true }), {
						status: 200,
						headers: { "Content-Type": "application/json" },
					}),
				);
				await first;
				await vi.advanceTimersByTimeAsync(8);
				expect(client.getStatus().isSubmitting).toBe(true);

				second_deferred.resolve(
					new Response(JSON.stringify({ ok: true }), {
						status: 200,
						headers: { "Content-Type": "application/json" },
					}),
				);
				await second;
				await vi.runAllTimersAsync();

				expect_no_loading_gap(statuses);
				expect_status_idle(statuses.at(-1));
			} finally {
				cleanup();
			}
		});
	});

	// ─── Hidden Submissions ──────────────────────────────────

	describe("hidden submissions", () => {
		it("does not expose isSubmitting for hidden submissions", async () => {
			const client = await load_client();
			const { statuses, cleanup } = collect_status_snapshots(client);
			const deferred = create_deferred<Response>();
			vi.spyOn(window, "fetch").mockImplementation(
				() => deferred.promise,
			);

			try {
				const sub = client.submit(
					"/api/hidden",
					{ method: "POST" },
					{
						revalidate: false,
						skipGlobalLoadingIndicator: true,
					},
				);
				await vi.advanceTimersByTimeAsync(8);

				expect(client.getStatus().isSubmitting).toBe(false);
				expect(statuses.every((s) => !s.isSubmitting)).toBe(true);

				deferred.resolve(
					new Response(JSON.stringify({ ok: true }), {
						status: 200,
						headers: { "Content-Type": "application/json" },
					}),
				);
				await sub;
				await vi.runAllTimersAsync();

				expect_status_idle(client.getStatus());
			} finally {
				cleanup();
			}
		});

		it("drops submitting status when only hidden submissions remain", async () => {
			const client = await load_client();
			const hidden_deferred = create_deferred<Response>();
			const visible_deferred = create_deferred<Response>();
			vi.spyOn(window, "fetch")
				.mockImplementationOnce(() => hidden_deferred.promise)
				.mockImplementationOnce(() => visible_deferred.promise);

			const hidden = client.submit(
				"/api/hidden",
				{ method: "POST" },
				{
					revalidate: false,
					skipGlobalLoadingIndicator: true,
				},
			);
			const visible = client.submit(
				"/api/visible",
				{ method: "POST" },
				{ revalidate: false },
			);
			await vi.advanceTimersByTimeAsync(8);

			expect(client.getStatus().isSubmitting).toBe(true);

			visible_deferred.resolve(
				new Response(JSON.stringify({ visible: true }), {
					status: 200,
					headers: { "Content-Type": "application/json" },
				}),
			);
			await visible;
			await vi.runAllTimersAsync();

			expect(client.getStatus().isSubmitting).toBe(false);

			hidden_deferred.resolve(
				new Response(JSON.stringify({ hidden: true }), {
					status: 200,
					headers: { "Content-Type": "application/json" },
				}),
			);
			await hidden;
			await vi.runAllTimersAsync();

			expect_status_idle(client.getStatus());
		});
	});

	// ─── Cross-Origin Rejection ──────────────────────────────

	describe("cross-origin", () => {
		it("throws for cross-origin submit targets", async () => {
			const client = await load_client();
			const fetch_spy = vi.spyOn(window, "fetch");

			await expect(
				client.submit("https://external.example/api", {
					method: "POST",
				}),
			).rejects.toThrow("same-origin");
			expect(fetch_spy).not.toHaveBeenCalled();
		});
	});

	// ─── Submit Status Reporting ─────────────────────────────

	describe("status reporting", () => {
		it("reports isSubmitting during submit without auto-revalidate", async () => {
			const client = await load_client();
			const deferred = create_deferred<Response>();
			vi.spyOn(window, "fetch").mockImplementation(
				() => deferred.promise,
			);

			const sub = client.submit(
				"/api/status",
				{ method: "POST" },
				{ revalidate: false },
			);
			await Promise.resolve();

			expect(client.getStatus()).toEqual({
				isNavigating: false,
				isSubmitting: true,
				isRevalidating: false,
			});

			deferred.resolve(
				new Response(JSON.stringify({ ok: true }), {
					status: 200,
					headers: { "Content-Type": "application/json" },
				}),
			);
			await sub;
			await vi.runAllTimersAsync();

			expect_status_idle(client.getStatus());
		});
	});

	// ─── Stale Submit vs Navigation (Assertion 48) ───────────

	describe("stale submit vs navigation", () => {
		it("ignores stale submit redirect when newer navigation commits first", async () => {
			const client = await load_client();
			const { requests } = create_abort_aware_fetch_recorder();

			const sub = client.submit("/api/submit-redirect", {
				method: "POST",
			});

			await wait_for_request_count({ requests, count: 1 });
			requests[0]!.deferred.resolve(
				create_route_data_response(
					{},
					{
						headers: {
							"X-Client-Redirect": "/submit-redirect-target",
						},
					},
				),
			);
			await wait_for_request_count({ requests, count: 2 });

			const nav = client.vormaNavigate("/submit-redirect-winner");
			await wait_for_request_count({ requests, count: 3 });
			requests[2]!.deferred.resolve(
				create_route_data_response({
					title: { dangerousInnerHTML: "Navigation Winner" },
				}),
			);
			await nav;

			requests[1]!.deferred.resolve(
				create_route_data_response({
					title: { dangerousInnerHTML: "Stale Submit Redirect" },
				}),
			);

			await sub;
			await vi.runAllTimersAsync();

			expect(window.location.pathname).toBe("/submit-redirect-winner");
			expect(document.title).toBe("Navigation Winner");
			expect_status_idle(client.getStatus());
		});

		it("drops stale redirect from different-key submit when newer submit wins", async () => {
			const client = await load_client();
			window.history.replaceState({}, "", "/submit-stale-base");
			const first_deferred = create_deferred<Response>();
			vi.spyOn(window, "fetch")
				.mockImplementationOnce(() => first_deferred.promise)
				.mockResolvedValueOnce(
					new Response(JSON.stringify({ ok: "newer" }), {
						status: 200,
						headers: { "Content-Type": "application/json" },
					}),
				);

			const first = client.submit(
				"/api/stale-first",
				{ method: "POST" },
				{ dedupeKey: "first", revalidate: false },
			);
			const second = client.submit(
				"/api/stale-second",
				{ method: "POST" },
				{ dedupeKey: "second", revalidate: false },
			);
			await second;

			first_deferred.resolve(
				new Response("", {
					status: 200,
					headers: {
						"X-Client-Redirect": "/submit-stale-redirect",
					},
				}),
			);
			await first;
			await vi.runAllTimersAsync();

			expect(window.location.pathname).toBe("/submit-stale-base");
			expect_status_idle(client.getStatus());
		});
	});
});
