// Assertions covered: 47, 48, 49

import { beforeEach, describe, expect, it, vi } from "vitest";
import {
	pop_back_for_testing,
	push_history_for_testing,
	reset_client_runtime_for_testing,
} from "vorma/testing";
import {
	collect_status_snapshots,
	create_deferred,
	create_route_data_response,
	expect_no_loading_gap,
	expect_status_idle,
	load_client,
	request_input_to_url,
	stub_window_location_href,
	wait_for_request_count,
} from "./setup.ts";

beforeEach(() => {
	reset_client_runtime_for_testing();
});

describe("redirects", () => {
	// ─── Assertion 47: Navigation Redirects ──────────────────

	describe("navigation soft redirects", () => {
		it("follows X-Client-Redirect through final destination commit", async () => {
			const client = await load_client();
			vi.spyOn(window, "fetch")
				.mockResolvedValueOnce(
					new Response("", {
						status: 200,
						headers: {
							"X-Client-Redirect": "/redirect-final",
							"X-Vorma-Client-Build-Id": "redirect-1",
						},
					}),
				)
				.mockResolvedValueOnce(
					create_route_data_response(
						{
							title: {
								dangerousInnerHTML: "Redirect Final",
							},
						},
						{
							headers: {
								"X-Vorma-Client-Build-Id": "redirect-2",
							},
						},
					),
				);

			const result = await client.vormaNavigate("/redirect-source");
			await vi.runAllTimersAsync();

			expect(result).toEqual({ didNavigate: true });
			expect(window.location.pathname).toBe("/redirect-final");
			expect(document.title).toBe("Redirect Final");
			expect(client.getClientBuildID()).toBe("redirect-2");
			expect_status_idle(client.getStatus());
		});

		it("follows redirect even when first response is non-ok", async () => {
			const client = await load_client();
			vi.spyOn(window, "fetch")
				.mockResolvedValueOnce(
					new Response("ignored", {
						status: 500,
						headers: {
							"X-Client-Redirect": "/redirect-non-ok",
							"X-Vorma-Client-Build-Id": "non-ok-1",
						},
					}),
				)
				.mockResolvedValueOnce(
					create_route_data_response({
						title: {
							dangerousInnerHTML: "Redirect Non-OK Final",
						},
					}),
				);

			await client.vormaNavigate("/redirect-non-ok-start");
			await vi.runAllTimersAsync();

			expect(window.location.pathname).toBe("/redirect-non-ok");
			expect(document.title).toBe("Redirect Non-OK Final");
			expect_status_idle(client.getStatus());
		});

		it("resolves relative redirect targets against the redirecting request URL path", async () => {
			const client = await load_client();
			const fetch_spy = vi
				.spyOn(window, "fetch")
				.mockResolvedValueOnce(
					new Response("", {
						status: 200,
						headers: {
							"X-Client-Redirect": "child/final",
							"X-Vorma-Client-Build-Id": "relative-1",
						},
					}),
				)
				.mockResolvedValueOnce(
					create_route_data_response({
						title: {
							dangerousInnerHTML: "Relative Redirect Final",
						},
					}),
				);

			await client.vormaNavigate("/relative/base");
			await vi.runAllTimersAsync();

			const second_url = request_input_to_url(
				fetch_spy.mock.calls[1]![0] as RequestInfo | URL,
			);
			expect(second_url.pathname).toBe("/relative/child/final");
			expect(window.location.pathname).toBe("/relative/child/final");
			expect(document.title).toBe("Relative Redirect Final");
		});

		it("updates build ID before following redirect targets", async () => {
			const client = await load_client();
			const observed: {
				currentBuildID: string;
				newClientBuildID: string;
			}[] = [];
			const remove_listener = client.addClientBuildIDListener(
				(
					event: CustomEvent<{
						oldClientBuildID: string;
						newClientBuildID: string;
					}>,
				) => {
					observed.push({
						currentBuildID: client.getClientBuildID(),
						newClientBuildID: event.detail.newClientBuildID,
					});
				},
			);

			try {
				vi.spyOn(window, "fetch")
					.mockResolvedValueOnce(
						new Response("", {
							status: 200,
							headers: {
								"X-Client-Redirect": "/redirect-build-target",
								"X-Vorma-Client-Build-Id": "redirect-build-1",
							},
						}),
					)
					.mockResolvedValueOnce(
						create_route_data_response(
							{},
							{
								headers: {
									"X-Vorma-Client-Build-Id":
										"redirect-build-1",
								},
							},
						),
					);

				await client.vormaNavigate("/redirect-build-start");
				await vi.runAllTimersAsync();

				expect(observed).toEqual([
					{
						currentBuildID: "redirect-build-1",
						newClientBuildID: "redirect-build-1",
					},
				]);
				expect(window.location.pathname).toBe("/redirect-build-target");
			} finally {
				remove_listener();
			}
		});

		it("does not re-follow redirect to encoding-equivalent current location", async () => {
			const client = await load_client();
			window.history.replaceState({}, "", "/already-here#same");
			const fetch_spy = vi.spyOn(window, "fetch").mockResolvedValueOnce(
				new Response("", {
					status: 200,
					headers: {
						"X-Client-Redirect": "/already-here#same",
					},
				}),
			);

			await client.vormaNavigate("/redirect-to-current");
			await vi.runAllTimersAsync();

			expect(fetch_spy).toHaveBeenCalledTimes(1);
			expect(window.location.pathname).toBe("/already-here");
			expect(window.location.hash).toBe("#same");
			expect_status_idle(client.getStatus());
		});

		it("rejects non-http redirect schemes", async () => {
			const client = await load_client();
			window.history.replaceState({}, "", "/before-non-http");
			document.title = "Before Non-HTTP";
			vi.spyOn(window, "fetch").mockResolvedValue(
				create_route_data_response(
					{
						title: { dangerousInnerHTML: "Should Not Commit" },
					},
					{
						headers: {
							"X-Client-Redirect": "mailto:test@example.com",
						},
					},
				),
			);

			await expect(
				client.vormaNavigate("/non-http-start"),
			).rejects.toThrow(/http\(s\)/i);
			await vi.runAllTimersAsync();

			expect(window.location.pathname).toBe("/before-non-http");
			expect(document.title).toBe("Before Non-HTTP");
			expect_status_idle(client.getStatus());
		});
	});

	describe("navigation hard redirects", () => {
		it("performs hard redirect for external targets", async () => {
			const client = await load_client();
			const location_stub = stub_window_location_href();

			try {
				vi.spyOn(window, "fetch").mockResolvedValueOnce(
					create_route_data_response(
						{},
						{
							headers: {
								"X-Client-Redirect": "https://external.example",
							},
						},
					),
				);

				await client.vormaNavigate("/external-nav-start");
				await vi.runAllTimersAsync();

				expect(location_stub.get_href()).toContain("external.example");
				expect_status_idle(client.getStatus());
			} finally {
				location_stub.restore();
			}
		});

		it("performs hard reload for X-Wave-Framework-Reload responses", async () => {
			const client = await load_client();
			const location_stub = stub_window_location_href();

			try {
				vi.spyOn(window, "fetch").mockResolvedValueOnce(
					create_route_data_response(
						{},
						{
							headers: {
								"X-Vorma-Reload": "/force-reload",
								"X-Vorma-Client-Build-Id": "reload-build-1",
							},
						},
					),
				);

				await client.vormaNavigate("/hard-reload-start");
				await vi.runAllTimersAsync();

				expect(location_stub.get_href()).toContain("/force-reload");
				expect(location_stub.get_href()).toContain(
					"vorma_reload=reload-build-1",
				);
				expect_status_idle(client.getStatus());
			} finally {
				location_stub.restore();
			}
		});

		it("resolves relative X-Wave-Framework-Reload against request URL path", async () => {
			const client = await load_client();
			const location_stub = stub_window_location_href();

			try {
				vi.spyOn(window, "fetch").mockResolvedValueOnce(
					create_route_data_response(
						{},
						{
							headers: {
								"X-Vorma-Reload": "child-reload",
								"X-Vorma-Client-Build-Id": "relative-reload-1",
							},
						},
					),
				);

				await client.vormaNavigate("/server/base/start");
				await vi.runAllTimersAsync();

				expect(location_stub.get_href()).toContain(
					"/server/base/child-reload",
				);
				expect(location_stub.get_href()).toContain(
					"vorma_reload=relative-reload-1",
				);
			} finally {
				location_stub.restore();
			}
		});

		it("prioritizes X-Vorma-Reload over X-Client-Redirect", async () => {
			const client = await load_client();
			const location_stub = stub_window_location_href();

			try {
				vi.spyOn(window, "fetch").mockResolvedValueOnce(
					create_route_data_response(
						{},
						{
							headers: {
								"X-Vorma-Reload": "/force-reload-priority",
								"X-Client-Redirect": "/ignored-soft",
								"X-Vorma-Client-Build-Id": "priority-1",
							},
						},
					),
				);

				await client.vormaNavigate("/priority-start");
				await vi.runAllTimersAsync();

				expect(location_stub.get_href()).toContain(
					"/force-reload-priority",
				);
				expect(location_stub.get_href()).not.toContain("/ignored-soft");
			} finally {
				location_stub.restore();
			}
		});
	});

	describe("navigation native redirects", () => {
		it("follows native fetch redirects for GET navigation requests", async () => {
			const client = await load_client();
			const native_redirect = create_route_data_response();
			Object.defineProperty(native_redirect, "redirected", {
				value: true,
				configurable: true,
			});
			Object.defineProperty(native_redirect, "url", {
				value: `${window.location.origin}/native-get-redirect`,
				configurable: true,
			});

			const fetch_spy = vi
				.spyOn(window, "fetch")
				.mockResolvedValueOnce(native_redirect)
				.mockResolvedValueOnce(
					create_route_data_response({
						title: {
							dangerousInnerHTML: "Native GET Redirected",
						},
					}),
				);

			await client.vormaNavigate("/native-get-start");
			await vi.runAllTimersAsync();

			expect(fetch_spy).toHaveBeenCalledTimes(2);
			const second_url = request_input_to_url(
				fetch_spy.mock.calls[1]![0] as RequestInfo | URL,
			);
			expect(second_url.pathname).toBe("/native-get-redirect");
			expect(window.location.pathname).toBe("/native-get-redirect");
			expect(document.title).toBe("Native GET Redirected");
		});

		it("does not re-follow native redirects to encoding-equivalent current hash", async () => {
			const client = await load_client();
			window.history.replaceState({}, "", "/native-current#~");
			const native_redirect = create_route_data_response();
			Object.defineProperty(native_redirect, "redirected", {
				value: true,
				configurable: true,
			});
			Object.defineProperty(native_redirect, "url", {
				value: `${window.location.origin}/native-current#%7E`,
				configurable: true,
			});

			const fetch_spy = vi
				.spyOn(window, "fetch")
				.mockResolvedValueOnce(native_redirect);

			await client.vormaNavigate("/native-start");
			await vi.runAllTimersAsync();

			expect(fetch_spy).toHaveBeenCalledTimes(1);
			expect(window.location.pathname).toBe("/native-current");
			expect(window.location.hash).toBe("#~");
		});

		it("does not follow stale native redirects from aborted navigations", async () => {
			const client = await load_client();
			const stale_deferred = create_deferred<Response>();
			const native_redirect = create_route_data_response(
				{},
				{
					headers: {
						"X-Vorma-Client-Build-Id": "stale-native-build",
					},
				},
			);
			Object.defineProperty(native_redirect, "redirected", {
				value: true,
				configurable: true,
			});
			Object.defineProperty(native_redirect, "url", {
				value: `${window.location.origin}/stale-native-redirect`,
				configurable: true,
			});

			let fetch_count = 0;
			vi.spyOn(window, "fetch").mockImplementation(() => {
				fetch_count += 1;
				if (fetch_count === 1) {
					return stale_deferred.promise;
				}
				return Promise.resolve(
					create_route_data_response({
						title: {
							dangerousInnerHTML: "Navigation Winner",
						},
					}),
				);
			});

			const stale = client.vormaNavigate("/stale-native-start");
			await vi.advanceTimersByTimeAsync(8);

			await client.vormaNavigate("/winner-page");
			await vi.runAllTimersAsync();

			stale_deferred.resolve(native_redirect);
			await stale;
			await vi.runAllTimersAsync();

			expect(fetch_count).toBe(2);
			expect(window.location.pathname).toBe("/winner-page");
			expect(document.title).toBe("Navigation Winner");
			expect_status_idle(client.getStatus());
		});
	});

	describe("stale navigation redirect suppression", () => {
		it("does not follow stale client-redirect from aborted navigation", async () => {
			const client = await load_client();
			const stale_deferred = create_deferred<Response>();
			let fetch_count = 0;
			vi.spyOn(window, "fetch").mockImplementation(() => {
				fetch_count += 1;
				if (fetch_count === 1) {
					return stale_deferred.promise;
				}
				if (fetch_count === 2) {
					return Promise.resolve(
						create_route_data_response({
							title: {
								dangerousInnerHTML: "Soft Redirect Winner",
							},
						}),
					);
				}
				return Promise.resolve(
					create_route_data_response({
						title: {
							dangerousInnerHTML: "Stale Was Followed",
						},
					}),
				);
			});

			const stale = client.vormaNavigate("/stale-soft-start");
			await vi.advanceTimersByTimeAsync(8);

			await client.vormaNavigate("/soft-winner");
			await vi.runAllTimersAsync();

			stale_deferred.resolve(
				create_route_data_response(
					{},
					{
						headers: {
							"X-Client-Redirect": "/stale-soft-target",
							"X-Vorma-Client-Build-Id": "stale-soft-build",
						},
					},
				),
			);
			await stale;
			await vi.runAllTimersAsync();

			expect(fetch_count).toBe(2);
			expect(window.location.pathname).toBe("/soft-winner");
			expect(document.title).toBe("Soft Redirect Winner");
			expect_status_idle(client.getStatus());
		});

		it("does not hard-reload from stale aborted navigation responses", async () => {
			const client = await load_client();
			const stale_deferred = create_deferred<Response>();

			let fetch_count = 0;
			vi.spyOn(window, "fetch").mockImplementation(() => {
				fetch_count += 1;
				if (fetch_count === 1) {
					return stale_deferred.promise;
				}
				return Promise.resolve(
					create_route_data_response({
						title: { dangerousInnerHTML: "Hard Reload Winner" },
					}),
				);
			});

			const stale = client.vormaNavigate("/stale-hard-start");
			await vi.advanceTimersByTimeAsync(8);

			await client.vormaNavigate("/hard-winner");
			await vi.runAllTimersAsync();

			const location_stub = stub_window_location_href(
				window.location.href,
			);

			try {
				stale_deferred.resolve(
					create_route_data_response(
						{},
						{
							headers: {
								"X-Vorma-Reload": "/stale-hard-redirect",
								"X-Vorma-Client-Build-Id": "stale-hard-build",
							},
						},
					),
				);
				await stale;
				await vi.runAllTimersAsync();

				expect(location_stub.get_href()).toContain("/hard-winner");
				expect(location_stub.get_href()).not.toContain(
					"/stale-hard-redirect",
				);
				expect_status_idle(client.getStatus());
			} finally {
				location_stub.restore();
			}
		});

		it("does not let stale redirect follow-up override newer navigation", async () => {
			const client = await load_client();
			const stale_followup_deferred = create_deferred<Response>();
			const fetch_spy = vi
				.spyOn(window, "fetch")
				.mockImplementation((input: RequestInfo | URL) => {
					const url = request_input_to_url(input);
					if (url.pathname === "/redirect-race-start") {
						return Promise.resolve(
							new Response("", {
								status: 200,
								headers: {
									"X-Client-Redirect":
										"/redirect-race-target",
									"X-Vorma-Client-Build-Id":
										"redirect-race-1",
								},
							}),
						);
					}
					if (url.pathname === "/redirect-race-target") {
						return stale_followup_deferred.promise;
					}
					if (url.pathname === "/redirect-race-winner") {
						return Promise.resolve(
							create_route_data_response({
								title: {
									dangerousInnerHTML: "Redirect Race Winner",
								},
							}),
						);
					}
					throw new Error(`Unexpected: ${url.pathname}`);
				});

			const stale = client.vormaNavigate("/redirect-race-start");
			await wait_for_request_count({
				requests: fetch_spy.mock.calls,
				count: 2,
			});

			const winner = client.vormaNavigate("/redirect-race-winner");
			await winner;
			await vi.runAllTimersAsync();

			stale_followup_deferred.resolve(
				create_route_data_response({
					title: { dangerousInnerHTML: "Stale Follow-Up" },
				}),
			);
			await stale;
			await vi.runAllTimersAsync();

			expect(window.location.pathname).toBe("/redirect-race-winner");
			expect(document.title).toBe("Redirect Race Winner");
			expect_status_idle(client.getStatus());
		});

		it("does not let stale POP completion override newer navigation", async () => {
			const client = await load_client();
			await client.initClient({});
			const stale_pop_deferred = create_deferred<Response>();
			const build_id_events: {
				newClientBuildID: string;
			}[] = [];
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

			let fetch_count = 0;
			const fetch_spy = vi
				.spyOn(window, "fetch")
				.mockImplementation(() => {
					fetch_count += 1;
					if (fetch_count === 1) {
						return stale_pop_deferred.promise;
					}
					if (fetch_count === 2) {
						return Promise.resolve(
							create_route_data_response({
								title: {
									dangerousInnerHTML: "POP Winner",
								},
							}),
						);
					}
					throw new Error("Unexpected fetch call");
				});

			try {
				push_history_for_testing("/stale-pop-target");
				await vi.runAllTimersAsync();
				pop_back_for_testing();
				await wait_for_request_count({
					requests: fetch_spy.mock.calls,
					count: 1,
				});

				await client.vormaNavigate("/pop-winner");
				await vi.runAllTimersAsync();

				stale_pop_deferred.resolve(
					create_route_data_response(
						{
							title: {
								dangerousInnerHTML: "Stale POP Applied",
							},
						},
						{
							headers: {
								"X-Vorma-Client-Build-Id": "stale-pop-build",
							},
						},
					),
				);
				await vi.runAllTimersAsync();

				expect(fetch_count).toBe(2);
				expect(window.location.pathname).toBe("/pop-winner");
				expect(document.title).toBe("POP Winner");
				expect(client.getClientBuildID()).toBe("1");
				expect(build_id_events).toEqual([]);
				expect_status_idle(client.getStatus());
			} finally {
				remove_listener();
			}
		});
	});

	// ─── Assertion 48: Submit Redirects ──────────────────────

	describe("submit soft redirects", () => {
		it("follows internal redirect from submit", async () => {
			const client = await load_client();
			const fetch_spy = vi
				.spyOn(window, "fetch")
				.mockResolvedValueOnce(
					new Response("", {
						status: 200,
						headers: {
							"X-Client-Redirect": "/submit-redirect-final",
						},
					}),
				)
				.mockResolvedValueOnce(
					create_route_data_response({
						title: {
							dangerousInnerHTML: "Submit Redirect Final",
						},
					}),
				);

			const result = await client.submit("/api/submit-redirect", {
				method: "POST",
			});
			await vi.runAllTimersAsync();

			expect(result).toEqual({ success: true, data: undefined });
			expect(fetch_spy).toHaveBeenCalledTimes(2);
			expect(window.location.pathname).toBe("/submit-redirect-final");
			expect(document.title).toBe("Submit Redirect Final");
		});

		it("follows submit redirect even when response is non-ok", async () => {
			const client = await load_client();
			vi.spyOn(window, "fetch")
				.mockResolvedValueOnce(
					new Response("ignored", {
						status: 500,
						headers: {
							"X-Client-Redirect":
								"/submit-redirect-non-ok-final",
						},
					}),
				)
				.mockResolvedValueOnce(
					create_route_data_response({
						title: {
							dangerousInnerHTML: "Submit Redirect Non-OK Final",
						},
					}),
				);

			const result = await client.submit(
				"/api/submit-redirect-non-ok",
				{ method: "POST" },
				{ revalidate: false },
			);
			await vi.runAllTimersAsync();

			expect(result).toEqual({ success: true, data: undefined });
			expect(window.location.pathname).toBe(
				"/submit-redirect-non-ok-final",
			);
		});

		it("does not fetch route data for hash-only submit redirects", async () => {
			const client = await load_client();
			window.history.replaceState({}, "", "/submit-hash-redirect");
			const fetch_spy = vi.spyOn(window, "fetch").mockResolvedValue(
				new Response("", {
					status: 200,
					headers: {
						"X-Client-Redirect": "/submit-hash-redirect#frag",
					},
				}),
			);

			const result = await client.submit(
				"/api/submit-hash",
				{ method: "POST" },
				{ revalidate: false },
			);
			await vi.runAllTimersAsync();

			expect(result).toEqual({ success: true, data: undefined });
			expect(fetch_spy).toHaveBeenCalledTimes(1);
			expect(window.location.hash).toBe("#frag");
		});

		it("does not re-follow submit redirect to same path", async () => {
			const client = await load_client();
			window.history.replaceState({}, "", "/submit-same-path");
			const fetch_spy = vi.spyOn(window, "fetch").mockResolvedValue(
				new Response("", {
					status: 200,
					headers: {
						"X-Client-Redirect": "/submit-same-path",
					},
				}),
			);

			const result = await client.submit(
				"/api/submit-same",
				{ method: "POST" },
				{ revalidate: false },
			);
			await vi.runAllTimersAsync();

			expect(result).toEqual({ success: true, data: undefined });
			expect(fetch_spy).toHaveBeenCalledTimes(1);
			expect_status_idle(client.getStatus());
		});

		it("updates build ID before following submit redirect", async () => {
			const client = await load_client();
			const observed: {
				currentBuildID: string;
				newClientBuildID: string;
			}[] = [];
			const remove_listener = client.addClientBuildIDListener(
				(
					event: CustomEvent<{
						oldClientBuildID: string;
						newClientBuildID: string;
					}>,
				) => {
					observed.push({
						currentBuildID: client.getClientBuildID(),
						newClientBuildID: event.detail.newClientBuildID,
					});
				},
			);

			try {
				vi.spyOn(window, "fetch")
					.mockResolvedValueOnce(
						new Response("", {
							status: 200,
							headers: {
								"X-Client-Redirect": "/submit-build-target",
								"X-Vorma-Client-Build-Id": "submit-build-1",
							},
						}),
					)
					.mockResolvedValueOnce(
						create_route_data_response(
							{},
							{
								headers: {
									"X-Vorma-Client-Build-Id": "submit-build-1",
								},
							},
						),
					);

				await client.submit("/api/submit-build", {
					method: "POST",
				});
				await vi.runAllTimersAsync();

				expect(observed).toEqual([
					{
						currentBuildID: "submit-build-1",
						newClientBuildID: "submit-build-1",
					},
				]);
				expect(window.location.pathname).toBe("/submit-build-target");
			} finally {
				remove_listener();
			}
		});

		it("returns explicit failure when submit redirect follow fails", async () => {
			const client = await load_client();
			vi.spyOn(window, "fetch")
				.mockResolvedValueOnce(
					new Response("", {
						status: 200,
						headers: {
							"X-Client-Redirect": "/submit-follow-fail",
						},
					}),
				)
				.mockRejectedValueOnce(new Error("Redirect failed"));

			const result = await client.submit("/api/submit-follow-fail", {
				method: "POST",
			});
			await vi.runAllTimersAsync();

			expect(result).toEqual({
				success: false,
				error: "Redirect failed",
			});
			expect_status_idle(client.getStatus());
		});
	});

	describe("submit hard redirects", () => {
		it("performs hard reload for X-Vorma-Reload on submit", async () => {
			const client = await load_client();
			const location_stub = stub_window_location_href();

			try {
				vi.spyOn(window, "fetch").mockResolvedValueOnce(
					create_route_data_response(
						{},
						{
							headers: {
								"X-Vorma-Reload": "/force-reload-submit",
								"X-Vorma-Client-Build-Id": "reload-submit-1",
							},
						},
					),
				);

				const result = await client.submit("/api/action", {
					method: "POST",
				});
				await vi.runAllTimersAsync();

				expect(result).toEqual({
					success: true,
					data: undefined,
				});
				expect(location_stub.get_href()).toContain(
					"/force-reload-submit",
				);
				expect(location_stub.get_href()).toContain(
					"vorma_reload=reload-submit-1",
				);
			} finally {
				location_stub.restore();
			}
		});

		it("performs hard redirect for external submit targets", async () => {
			const client = await load_client();
			const location_stub = stub_window_location_href();

			try {
				vi.spyOn(window, "fetch").mockResolvedValueOnce(
					create_route_data_response(
						{},
						{
							headers: {
								"X-Client-Redirect": "https://external.com",
							},
						},
					),
				);

				const result = await client.submit("/api/action", {
					method: "POST",
				});
				await vi.runAllTimersAsync();

				expect(result).toEqual({
					success: true,
					data: undefined,
				});
				expect(location_stub.get_href()).toMatch(
					/^https:\/\/external\.com/,
				);
			} finally {
				location_stub.restore();
			}
		});

		it("prioritizes X-Vorma-Reload over X-Client-Redirect for submit", async () => {
			const client = await load_client();
			const location_stub = stub_window_location_href();

			try {
				vi.spyOn(window, "fetch").mockResolvedValueOnce(
					create_route_data_response(
						{},
						{
							headers: {
								"X-Vorma-Reload": "/force-reload-priority",
								"X-Client-Redirect": "/ignored-soft",
								"X-Vorma-Client-Build-Id": "priority-1",
							},
						},
					),
				);

				await client.submit("/api/priority", { method: "POST" });
				await vi.runAllTimersAsync();

				expect(location_stub.get_href()).toContain(
					"/force-reload-priority",
				);
				expect(location_stub.get_href()).not.toContain("/ignored-soft");
			} finally {
				location_stub.restore();
			}
		});
	});

	describe("submit native redirects", () => {
		it("follows native fetch redirects for non-GET submit requests", async () => {
			const client = await load_client();
			const native_redirect = create_route_data_response();
			Object.defineProperty(native_redirect, "redirected", {
				value: true,
				configurable: true,
			});
			Object.defineProperty(native_redirect, "url", {
				value: `${window.location.origin}/native-submit-redirect`,
				configurable: true,
			});

			const fetch_spy = vi
				.spyOn(window, "fetch")
				.mockResolvedValueOnce(native_redirect)
				.mockResolvedValueOnce(
					create_route_data_response({
						title: {
							dangerousInnerHTML: "Native Submit Redirected",
						},
					}),
				);

			const result = await client.submit("/api/native-submit-redirect", {
				method: "POST",
			});
			await vi.runAllTimersAsync();

			expect(result).toEqual({ success: true, data: undefined });
			expect(fetch_spy).toHaveBeenCalledTimes(2);
			const second_url = request_input_to_url(
				fetch_spy.mock.calls[1]![0] as RequestInfo | URL,
			);
			expect(second_url.pathname).toBe("/native-submit-redirect");
			expect(window.location.pathname).toBe("/native-submit-redirect");
		});
	});

	// ─── Assertion 49: Redirect Chains ───────────────────────

	describe("redirect chains", () => {
		it("keeps loading continuous through redirect chains", async () => {
			const client = await load_client();
			const { statuses, cleanup } = collect_status_snapshots(client);

			try {
				vi.spyOn(window, "fetch")
					.mockResolvedValueOnce(
						new Response("", {
							status: 200,
							headers: {
								"X-Client-Redirect": "/redirect-mid",
								"X-Vorma-Client-Build-Id": "chain-1",
							},
						}),
					)
					.mockResolvedValueOnce(
						new Response("", {
							status: 200,
							headers: {
								"X-Client-Redirect": "/redirect-final-chain",
								"X-Vorma-Client-Build-Id": "chain-2",
							},
						}),
					)
					.mockResolvedValueOnce(
						create_route_data_response({
							title: {
								dangerousInnerHTML: "Chain Final",
							},
						}),
					);

				await client.vormaNavigate("/redirect-chain-start");
				await vi.runAllTimersAsync();

				expect(window.location.pathname).toBe("/redirect-final-chain");
				expect(document.title).toBe("Chain Final");
				expect(statuses.some((s) => s.isNavigating)).toBe(true);
				expect_no_loading_gap(statuses);
				expect_status_idle(statuses.at(-1));
			} finally {
				cleanup();
			}
		});

		it("caps redirect chains at ten follows and exits loading", async () => {
			const client = await load_client();
			const fetch_spy = vi.spyOn(window, "fetch");
			const console_error_spy = vi
				.spyOn(console, "error")
				.mockImplementation(() => {});

			for (let i = 0; i < 15; i += 1) {
				fetch_spy.mockResolvedValueOnce(
					new Response("", {
						status: 200,
						headers: {
							"X-Client-Redirect": `/redirect-loop-${i}`,
						},
					}),
				);
			}

			const result = await client.vormaNavigate("/redirect-loop-start");
			await vi.runAllTimersAsync();

			expect(result).toEqual({ didNavigate: false });
			expect(fetch_spy).toHaveBeenCalledTimes(10);
			expect(window.location.pathname).toBe("/");
			expect(
				console_error_spy.mock.calls.some((args) =>
					args.some(
						(v) =>
							typeof v === "string" &&
							v.includes("Too many redirects"),
					),
				),
			).toBe(true);
			expect_status_idle(client.getStatus());
		});
	});
});
