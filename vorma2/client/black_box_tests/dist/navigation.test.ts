// Assertions covered: 1, 2, 3, 14, 17, 18

import { beforeEach, describe, expect, it, vi } from "vitest";
import {
	clear_all_navigation_state_for_testing,
	register_client_loader_for_testing,
	reset_client_runtime_for_testing,
} from "vorma/testing";
import {
	create_abort_aware_fetch_recorder,
	create_deferred,
	create_deferred_fetch_call,
	create_route_data_response,
	expect_status_idle,
	load_client,
	request_input_to_url,
	shuffled_indices,
	wait_for_request_count,
	with_unhandled_rejection_capture,
} from "./setup.ts";

beforeEach(() => {
	reset_client_runtime_for_testing();
});

describe("navigation", () => {
	// ─── Assertion 1: Concurrent Navigation Authority ────────

	describe("concurrent authority", () => {
		it("latest started navigation remains authoritative when responses resolve out of order", async () => {
			const client = await load_client();
			const { requests } = create_abort_aware_fetch_recorder();

			const first = client.vormaNavigate("/race-first");
			const second = client.vormaNavigate("/race-second");

			await wait_for_request_count({ requests, count: 2 });

			requests[1]!.deferred.resolve(
				create_route_data_response({
					title: { dangerousInnerHTML: "Race Second" },
				}),
			);
			await Promise.resolve();

			requests[0]!.deferred.resolve(
				create_route_data_response({
					title: { dangerousInnerHTML: "Race First" },
				}),
			);

			await Promise.allSettled([first, second]);
			await vi.runAllTimersAsync();

			expect(window.location.pathname).toBe("/race-second");
			expect(document.title).toBe("Race Second");
			expect_status_idle(client.getStatus());
		});

		it("does not require stale responses to settle before winner commits", async () => {
			const client = await load_client();
			const first_deferred = create_deferred<Response>();
			const second_deferred = create_deferred<Response>();
			let fetch_count = 0;
			vi.spyOn(window, "fetch").mockImplementation(() => {
				fetch_count += 1;
				if (fetch_count === 1) {
					return first_deferred.promise;
				}
				if (fetch_count === 2) {
					return second_deferred.promise;
				}
				throw new Error("unexpected fetch");
			});

			const first = client.vormaNavigate("/stale-first");
			const second = client.vormaNavigate("/stale-winner");

			second_deferred.resolve(
				create_route_data_response({
					title: { dangerousInnerHTML: "Winner" },
				}),
			);
			await second;
			await vi.runAllTimersAsync();

			expect(window.location.pathname).toBe("/stale-winner");
			expect(document.title).toBe("Winner");

			first_deferred.resolve(
				create_route_data_response({
					title: { dangerousInnerHTML: "Stale" },
				}),
			);
			await first;
			await vi.runAllTimersAsync();

			expect(window.location.pathname).toBe("/stale-winner");
			expect(document.title).toBe("Winner");
			expect_status_idle(client.getStatus());
		});

		it("reuses in-flight navigation when only hash changes on same data target", async () => {
			const client = await load_client();
			const deferred = create_deferred<Response>();
			const fetch_spy = vi
				.spyOn(window, "fetch")
				.mockImplementation(() => deferred.promise);

			const first = client.vormaNavigate("/hash-only#first");
			await wait_for_request_count({
				requests: fetch_spy.mock.calls,
				count: 1,
			});
			const second = client.vormaNavigate("/hash-only#second");
			await vi.advanceTimersByTimeAsync(8);

			expect(fetch_spy).toHaveBeenCalledTimes(1);

			deferred.resolve(
				create_route_data_response({
					title: { dangerousInnerHTML: "Hash Stable" },
				}),
			);
			await Promise.all([first, second]);
			await vi.runAllTimersAsync();

			expect(window.location.pathname).toBe("/hash-only");
			expect(window.location.hash).toBe("#second");
			expect(document.title).toBe("Hash Stable");
			expect_status_idle(client.getStatus());
		});

		it.each([11, 29, 47, 83, 131])(
			"keeps last-started authoritative across generated resolve permutations (seed=%d)",
			async (seed) => {
				const client = await load_client();
				const { requests } = create_abort_aware_fetch_recorder();
				const nav_count = 6;

				const { result, unhandled_rejections } =
					await with_unhandled_rejection_capture({
						run: async () => {
							const promises = [];
							for (let i = 0; i < nav_count; i += 1) {
								promises.push(
									client.vormaNavigate(
										`/generated-${seed}-${i}`,
									),
								);
							}

							await wait_for_request_count({
								requests,
								count: nav_count,
							});

							const order = shuffled_indices(nav_count, seed);
							for (const idx of order) {
								requests[idx]!.deferred.resolve(
									create_route_data_response({
										title: {
											dangerousInnerHTML: `Generated ${idx}`,
										},
									}),
								);
								await Promise.resolve();
							}

							await Promise.all(promises);
							return {
								expected_pathname: `/generated-${seed}-${nav_count - 1}`,
								expected_title: `Generated ${nav_count - 1}`,
							};
						},
					});

				expect(unhandled_rejections).toEqual([]);
				expect(window.location.pathname).toBe(result.expected_pathname);
				expect(document.title).toBe(result.expected_title);
				expect_status_idle(client.getStatus());
			},
		);
	});

	// ─── Assertion 2: Superseded Request Abort ───────────────

	describe("superseded request abort", () => {
		it("aborts in-flight fetch when new navigation target is requested", async () => {
			const client = await load_client();
			const first_fetch = create_deferred_fetch_call();
			vi.spyOn(window, "fetch")
				.mockImplementationOnce(first_fetch.mock)
				.mockImplementationOnce(() =>
					Promise.resolve(create_route_data_response()),
				);

			const first = client.vormaNavigate("/state-first");
			await Promise.resolve();
			expect(client.getStatus().isNavigating).toBe(true);
			expect(first_fetch.get_signal()).toBeDefined();

			const second = client.vormaNavigate("/state-second");
			expect(first_fetch.get_signal()?.aborted).toBe(true);

			first_fetch.deferred.reject(
				new DOMException("Aborted", "AbortError"),
			);
			await Promise.all([first, second]);
			await vi.runAllTimersAsync();

			expect(window.location.pathname).toBe("/state-second");
			expect_status_idle(client.getStatus());
		});

		it("starts new fetch when search params differ on same pathname", async () => {
			const client = await load_client();
			const first_fetch = create_deferred_fetch_call();
			const second_fetch = create_deferred_fetch_call();
			const fetch_spy = vi
				.spyOn(window, "fetch")
				.mockImplementationOnce(first_fetch.mock)
				.mockImplementationOnce(second_fetch.mock);

			const first = client.vormaNavigate("/search-diff?one=1");
			await wait_for_request_count({
				requests: fetch_spy.mock.calls,
				count: 1,
			});
			const second = client.vormaNavigate("/search-diff?two=2");
			await wait_for_request_count({
				requests: fetch_spy.mock.calls,
				count: 2,
			});

			expect(first_fetch.get_signal()?.aborted).toBe(true);

			first_fetch.deferred.reject(
				new DOMException("Aborted", "AbortError"),
			);
			second_fetch.deferred.resolve(
				create_route_data_response({
					title: { dangerousInnerHTML: "Search Winner" },
				}),
			);

			await Promise.all([first, second]);
			await vi.runAllTimersAsync();

			expect(window.location.pathname).toBe("/search-diff");
			expect(window.location.search).toBe("?two=2");
			expect(document.title).toBe("Search Winner");
			expect_status_idle(client.getStatus());
		});
	});

	// ─── Assertion 3: Stale Client-Loader Suppression ────────

	describe("stale client-loader suppression", () => {
		it("runs speculative client-loader but discards stale commits", async () => {
			const client = await load_client();
			const speculative_effects: string[] = [];
			const abort_states: boolean[] = [];

			register_client_loader_for_testing({
				pattern: "/stale-client-loader",
				client_loader: async (input: unknown) => {
					speculative_effects.push("executed");
					const typed = input as {
						serverDataPromise: Promise<unknown>;
						signal: AbortSignal;
					};
					try {
						await typed.serverDataPromise;
					} catch {
						abort_states.push(typed.signal.aborted);
					}
					return { stale: true };
				},
			});

			const stale_fetch = create_deferred_fetch_call();
			const fresh_fetch = create_deferred_fetch_call();
			vi.spyOn(window, "fetch")
				.mockImplementationOnce(stale_fetch.mock)
				.mockImplementationOnce(fresh_fetch.mock);

			const stale_nav = client.vormaNavigate("/stale-client-loader");
			await Promise.resolve();
			const fresh_nav = client.vormaNavigate("/fresh-target");
			await Promise.resolve();

			expect(stale_fetch.get_signal()?.aborted).toBe(true);

			fresh_fetch.deferred.resolve(
				create_route_data_response({
					title: { dangerousInnerHTML: "Fresh Target" },
				}),
			);
			stale_fetch.deferred.reject(
				new DOMException("Aborted", "AbortError"),
			);

			await Promise.all([stale_nav, fresh_nav]);
			await vi.runAllTimersAsync();

			expect(speculative_effects).toEqual(["executed"]);
			expect(abort_states).toEqual([true]);
			expect(window.location.pathname).toBe("/fresh-target");
			expect(document.title).toBe("Fresh Target");
			expect_status_idle(client.getStatus());
		});
	});

	// ─── Assertion 14: Same-Document Classification ──────────

	describe("same-document classification", () => {
		it("does not fetch for same-document no-op navigation", async () => {
			const client = await load_client();
			window.history.replaceState({}, "", "/same-noop");
			const fetch_spy = vi.spyOn(window, "fetch");

			await client.vormaNavigate("/same-noop");
			await vi.runAllTimersAsync();

			expect(fetch_spy).not.toHaveBeenCalled();
			expect(window.location.pathname).toBe("/same-noop");
			expect_status_idle(client.getStatus());
		});

		it("classifies no-op, hash-change, and full navigation correctly", async () => {
			const client = await load_client();
			window.history.replaceState({}, "", "/classify?a=1&b=2#one");
			const fetch_spy = vi
				.spyOn(window, "fetch")
				.mockResolvedValue(create_route_data_response());

			await client.vormaNavigate("/classify?a=1&b=2#one");
			await vi.runAllTimersAsync();
			expect(fetch_spy).toHaveBeenCalledTimes(0);

			await client.vormaNavigate("/classify?a=1&b=2#two");
			await vi.runAllTimersAsync();
			expect(fetch_spy).toHaveBeenCalledTimes(0);
			expect(window.location.hash).toBe("#two");

			await client.vormaNavigate("/classify?b=2&a=1#two");
			await vi.runAllTimersAsync();
			expect(fetch_spy).toHaveBeenCalledTimes(1);
		});

		it("commits hash-only navigation without fetching route data", async () => {
			const client = await load_client();
			window.history.replaceState({}, "", "/hash-base");
			const fetch_spy = vi.spyOn(window, "fetch");

			await client.vormaNavigate("/hash-base#details");
			await vi.runAllTimersAsync();

			expect(fetch_spy).not.toHaveBeenCalled();
			expect(window.location.pathname).toBe("/hash-base");
			expect(window.location.hash).toBe("#details");
			expect_status_idle(client.getStatus());
		});

		it("preserves metadata across hash-only navigations", async () => {
			const client = await load_client();
			vi.spyOn(window, "fetch").mockResolvedValueOnce(
				create_route_data_response({
					title: { dangerousInnerHTML: "Metadata Title" },
					meta_head_els: [
						{
							tag: "meta",
							attributesKnownSafe: {
								name: "description",
								content: "Metadata description",
							},
							booleanAttributes: [],
						},
					],
				}),
			);

			await client.vormaNavigate("/metadata-parity");
			await vi.runAllTimersAsync();
			expect(document.title).toBe("Metadata Title");

			await client.vormaNavigate("/metadata-parity#details");
			await vi.runAllTimersAsync();
			expect(document.title).toBe("Metadata Title");
			expect(
				document.head
					.querySelector('meta[name="description"]')
					?.getAttribute("content"),
			).toBe("Metadata description");
		});

		it("does not mutate history for encoding-equivalent hash", async () => {
			const client = await load_client();
			window.history.replaceState({}, "", "/equiv-hash#~");
			const push_spy = vi.spyOn(window.history, "pushState");
			const replace_spy = vi.spyOn(window.history, "replaceState");

			await client.vormaNavigate("/equiv-hash#%7E");
			await vi.runAllTimersAsync();

			expect(push_spy).not.toHaveBeenCalled();
			expect(replace_spy).not.toHaveBeenCalled();
		});

		it("pushes history when hash target changes", async () => {
			const client = await load_client();
			window.history.replaceState({}, "", "/hash-change#first");
			const push_spy = vi.spyOn(window.history, "pushState");

			await client.vormaNavigate("/hash-change#second");
			await vi.runAllTimersAsync();

			expect(push_spy).toHaveBeenCalled();
		});

		it("uses history.replace when replace=true", async () => {
			const client = await load_client();
			vi.spyOn(window, "fetch").mockResolvedValue(
				create_route_data_response(),
			);
			const push_spy = vi.spyOn(window.history, "pushState");
			const replace_spy = vi.spyOn(window.history, "replaceState");

			await client.vormaNavigate("/replace-target", {
				replace: true,
			});
			await vi.runAllTimersAsync();

			expect(push_spy).not.toHaveBeenCalled();
			expect(replace_spy).toHaveBeenCalled();
		});

		it("replaces state on same-document no-op when replace=true", async () => {
			const client = await load_client();
			window.history.replaceState({}, "", "/noop-replace");
			const fetch_spy = vi.spyOn(window, "fetch");

			await client.vormaNavigate("/noop-replace", {
				replace: true,
			});
			await vi.runAllTimersAsync();

			expect(fetch_spy).not.toHaveBeenCalled();
			expect(window.location.pathname).toBe("/noop-replace");
		});

		it("applies explicit search and hash overrides", async () => {
			const client = await load_client();
			vi.spyOn(window, "fetch").mockResolvedValue(
				create_route_data_response(),
			);

			await client.vormaNavigate("/target?tab=settings#security");
			await vi.runAllTimersAsync();

			expect(window.location.pathname).toBe("/target");
			expect(window.location.search).toBe("?tab=settings");
			expect(window.location.hash).toBe("#security");
		});

		it("pushes history for navigation to a different URL", async () => {
			const client = await load_client();
			window.history.replaceState({}, "", "/start");
			vi.spyOn(window, "fetch").mockResolvedValue(
				create_route_data_response(),
			);
			const push_spy = vi.spyOn(window.history, "pushState");

			await client.vormaNavigate("/destination");
			await vi.runAllTimersAsync();

			expect(push_spy).toHaveBeenCalled();
		});

		it("does not mutate history when navigating to current URL", async () => {
			const client = await load_client();
			window.history.replaceState({}, "", "/same");
			const push_spy = vi.spyOn(window.history, "pushState");
			const replace_spy = vi.spyOn(window.history, "replaceState");

			await client.vormaNavigate("/same");
			await vi.runAllTimersAsync();

			expect(push_spy).not.toHaveBeenCalled();
			expect(replace_spy).not.toHaveBeenCalled();
		});

		it("throws for cross-origin navigation targets", async () => {
			const client = await load_client();
			const fetch_spy = vi.spyOn(window, "fetch");

			await expect(
				client.vormaNavigate("https://external.example/path"),
			).rejects.toThrow("same-origin");
			expect(fetch_spy).not.toHaveBeenCalled();
		});

		it("does not emit duplicate idle status for same-document no-op", async () => {
			const client = await load_client();
			const statuses: unknown[] = [];
			const cleanup = client.addStatusListener((event) => {
				statuses.push(event.detail);
			});

			vi.spyOn(window, "fetch").mockResolvedValue(
				create_route_data_response(),
			);
			await client.vormaNavigate("/status-dedupe");
			await vi.runAllTimersAsync();
			const count_after_commit = statuses.length;

			await client.vormaNavigate("/status-dedupe");
			await vi.runAllTimersAsync();

			expect(statuses).toHaveLength(count_after_commit);
			cleanup();
		});
	});

	// ─── Assertion 17: clearAll and Terminal Outcomes ─────────

	describe("clearAll and terminal outcomes", () => {
		it("aborts in-flight navigation and submit, then returns idle", async () => {
			const client = await load_client();
			const { requests } = create_abort_aware_fetch_recorder();

			const { result, unhandled_rejections } =
				await with_unhandled_rejection_capture({
					run: async () => {
						const nav = client.vormaNavigate("/clear-all-nav");
						const sub = client.submit(
							"/api/clear-all-submit",
							{ method: "POST" },
							{ revalidate: false },
						);

						await wait_for_request_count({
							requests,
							count: 2,
						});
						expect(client.getStatus()).toEqual({
							isNavigating: true,
							isSubmitting: true,
							isRevalidating: false,
						});

						clear_all_navigation_state_for_testing();

						const submit_result = await sub;
						await nav;
						await vi.runAllTimersAsync();
						return { submit_result };
					},
				});

			expect(result.submit_result).toEqual({
				success: false,
				error: "Aborted",
			});
			expect(unhandled_rejections).toEqual([]);
			expect_status_idle(client.getStatus());
		});

		it("prevents late side effects from aborted work after clearAll", async () => {
			const client = await load_client();
			const fetch_call = create_deferred_fetch_call();
			vi.spyOn(window, "fetch").mockImplementation(fetch_call.mock);

			const nav = client.vormaNavigate("/clear-late");
			await vi.advanceTimersByTimeAsync(8);
			expect(fetch_call.get_signal()?.aborted).toBe(false);

			clear_all_navigation_state_for_testing();
			expect(fetch_call.get_signal()?.aborted).toBe(true);

			fetch_call.deferred.resolve(
				create_route_data_response({
					title: { dangerousInnerHTML: "Late Stale" },
					css_bundles: ["/late-stale.css"],
				}),
			);

			await nav;
			await vi.advanceTimersByTimeAsync(32);
			await vi.runAllTimersAsync();

			expect(window.location.pathname).toBe("/");
			expect(document.title).not.toBe("Late Stale");
			expect(
				document.head.querySelector(
					'link[data-vorma-css-bundle="/late-stale.css"]',
				),
			).toBeNull();
			expect_status_idle(client.getStatus());
		});

		it("remains operable after clearAll for fresh navigation", async () => {
			const client = await load_client();
			const first_fetch = create_deferred_fetch_call();
			vi.spyOn(window, "fetch")
				.mockImplementationOnce(first_fetch.mock)
				.mockResolvedValueOnce(
					create_route_data_response({
						title: { dangerousInnerHTML: "Post-Clear" },
					}),
				);

			const stale = client.vormaNavigate("/clear-stale");
			await vi.advanceTimersByTimeAsync(8);

			clear_all_navigation_state_for_testing();
			first_fetch.deferred.resolve(
				create_route_data_response({
					title: { dangerousInnerHTML: "Should Not Apply" },
				}),
			);
			await stale;
			await vi.runAllTimersAsync();

			await client.vormaNavigate("/clear-fresh");
			await vi.runAllTimersAsync();

			expect(window.location.pathname).toBe("/clear-fresh");
			expect(document.title).toBe("Post-Clear");
			expect_status_idle(client.getStatus());
		});

		it("surfaces explicit terminal outcomes for settled operations", async () => {
			const client = await load_client();
			const { requests } = create_abort_aware_fetch_recorder();

			const { result, unhandled_rejections } =
				await with_unhandled_rejection_capture({
					run: async () => {
						const first = client.submit(
							"/terminal-submit",
							{
								method: "POST",
								body: JSON.stringify({ step: 1 }),
							},
							{
								dedupeKey: "terminal-key",
								revalidate: false,
							},
						);
						await wait_for_request_count({
							requests,
							count: 1,
						});

						const second = client.submit(
							"/terminal-submit",
							{
								method: "POST",
								body: JSON.stringify({ step: 2 }),
							},
							{
								dedupeKey: "terminal-key",
								revalidate: false,
							},
						);
						await wait_for_request_count({
							requests,
							count: 2,
						});

						requests[1]!.deferred.resolve(
							new Response(JSON.stringify({ ok: true }), {
								status: 200,
								headers: {
									"Content-Type": "application/json",
								},
							}),
						);

						const [first_result, second_result] = await Promise.all(
							[first, second],
						);

						const failed = client.submit(
							"/terminal-failure",
							{ method: "POST" },
							{ revalidate: false },
						);
						await wait_for_request_count({
							requests,
							count: 3,
						});
						requests[2]!.deferred.resolve(
							new Response("failed", {
								status: 500,
								statusText: "Failure",
							}),
						);
						const failure_result = await failed;

						return {
							first_result,
							second_result,
							failure_result,
						};
					},
				});

			expect(unhandled_rejections).toEqual([]);
			expect(result.first_result).toEqual({
				success: false,
				error: "Aborted",
			});
			expect(result.second_result).toEqual({
				success: true,
				data: { ok: true },
			});
			expect(result.failure_result.success).toBe(false);
			expect_status_idle(client.getStatus());
		});
	});

	// ─── Assertion 18: Failure Recovery ──────────────────────

	describe("failure recovery", () => {
		it("clears loading state after abort-style failures", async () => {
			const client = await load_client();
			vi.spyOn(window, "fetch").mockRejectedValue(
				new DOMException("Aborted", "AbortError"),
			);

			await expect(client.vormaNavigate("/abort-nav")).resolves.toEqual({
				didNavigate: false,
			});
			expect_status_idle(client.getStatus());
		});

		it("clears loading state after non-abort failures and remains recoverable", async () => {
			const client = await load_client();
			vi.spyOn(window, "fetch")
				.mockRejectedValueOnce(new Error("Network down"))
				.mockResolvedValueOnce(
					create_route_data_response({
						title: { dangerousInnerHTML: "Recovered" },
					}),
				);

			await expect(
				client.vormaNavigate("/non-abort-failure"),
			).rejects.toThrow("Network down");
			expect_status_idle(client.getStatus());

			await client.vormaNavigate("/post-failure-recovery");
			await vi.runAllTimersAsync();

			expect(window.location.pathname).toBe("/post-failure-recovery");
			expect(document.title).toBe("Recovered");
			expect_status_idle(client.getStatus());
		});

		it("recovers from non-ok responses without corrupting router data", async () => {
			const client = await load_client();
			vi.spyOn(window, "fetch")
				.mockResolvedValueOnce(
					create_route_data_response({
						matched_patterns: ["/stable/:id"],
						has_root_data: true,
						loaders_data: [{ id: "A" }],
						params: { id: "A" },
					}),
				)
				.mockResolvedValueOnce(
					new Response("server-boom", {
						status: 500,
						headers: { "Content-Type": "text/plain" },
					}),
				);

			await client.vormaNavigate("/stable/A");
			await vi.runAllTimersAsync();
			const stable_data = client.getRouterData();

			await expect(client.vormaNavigate("/stable/B")).rejects.toThrow(
				"500",
			);
			await vi.runAllTimersAsync();

			expect(client.getRouterData()).toEqual(stable_data);
			expect_status_idle(client.getStatus());
		});

		it("does not apply side effects from stale aborted successes", async () => {
			const client = await load_client();
			const stale_deferred = create_deferred<Response>();
			const build_id_events: {
				oldClientBuildID: string;
				newClientBuildID: string;
			}[] = [];
			const remove_listener = client.addClientBuildIDListener(
				(event: {
					detail: {
						oldClientBuildID: string;
						newClientBuildID: string;
					};
				}) => {
					build_id_events.push(event.detail);
				},
			);

			let fetch_count = 0;
			vi.spyOn(window, "fetch").mockImplementation(() => {
				fetch_count += 1;
				if (fetch_count === 1) {
					return stale_deferred.promise;
				}
				return Promise.resolve(
					create_route_data_response(
						{
							title: { dangerousInnerHTML: "Winner" },
						},
						{
							headers: {
								"X-Wave-Framework-Build-Id": "winner-build",
							},
						},
					),
				);
			});

			const stale = client.vormaNavigate("/stale-effects");
			await Promise.resolve();
			const winner = client.vormaNavigate("/winner-effects");
			await winner;
			await vi.runAllTimersAsync();

			stale_deferred.resolve(
				create_route_data_response(
					{
						title: { dangerousInnerHTML: "Stale" },
						css_bundles: ["/stale.css"],
					},
					{
						headers: {
							"X-Wave-Framework-Build-Id": "stale-build",
						},
					},
				),
			);
			await stale;
			await vi.advanceTimersByTimeAsync(32);
			await vi.runAllTimersAsync();

			expect(window.location.pathname).toBe("/winner-effects");
			expect(document.title).toBe("Winner");
			expect(
				build_id_events.some(
					(e) => e.newClientBuildID === "stale-build",
				),
			).toBe(false);
			expect(
				document.head.querySelector(
					'link[data-vorma-css-bundle="/stale.css"]',
				),
			).toBeNull();
			expect_status_idle(client.getStatus());

			remove_listener();
		});

		it("does not leak unhandled rejections from failed responses", async () => {
			const client = await load_client();
			vi.spyOn(window, "fetch").mockResolvedValue(
				new Response("server-error", { status: 500 }),
			);

			const { unhandled_rejections } =
				await with_unhandled_rejection_capture({
					run: async () => {
						await expect(
							client.vormaNavigate("/failed-response"),
						).rejects.toThrow("500");
						await vi.runAllTimersAsync();
					},
				});

			expect(unhandled_rejections).toEqual([]);
			expect_status_idle(client.getStatus());
		});

		it("treats empty json payloads as failures and recovers", async () => {
			const client = await load_client();
			window.history.replaceState({}, "", "/before-empty");
			document.title = "Before Empty";

			vi.spyOn(window, "fetch")
				.mockResolvedValueOnce(
					new Response("", {
						status: 200,
						headers: {
							"Content-Type": "application/json",
						},
					}),
				)
				.mockResolvedValueOnce(
					create_route_data_response({
						title: { dangerousInnerHTML: "Recovered" },
					}),
				);

			await expect(client.vormaNavigate("/empty-json")).rejects.toThrow();
			expect(window.location.pathname).toBe("/before-empty");
			expect(document.title).toBe("Before Empty");
			expect_status_idle(client.getStatus());

			await client.vormaNavigate("/empty-recovered");
			await vi.runAllTimersAsync();

			expect(window.location.pathname).toBe("/empty-recovered");
			expect(document.title).toBe("Recovered");
			expect_status_idle(client.getStatus());
		});

		it("cleans up completed entries so later navigations still fetch", async () => {
			const client = await load_client();
			const fetch_spy = vi
				.spyOn(window, "fetch")
				.mockImplementation(() =>
					Promise.resolve(create_route_data_response()),
				);

			await client.vormaNavigate("/cleanup-a");
			await vi.runAllTimersAsync();
			await client.vormaNavigate("/cleanup-b");
			await vi.runAllTimersAsync();

			expect(fetch_spy).toHaveBeenCalledTimes(2);
			expect(
				request_input_to_url(
					fetch_spy.mock.calls[0]![0] as RequestInfo | URL,
				).pathname,
			).toBe("/cleanup-a");
			expect(
				request_input_to_url(
					fetch_spy.mock.calls[1]![0] as RequestInfo | URL,
				).pathname,
			).toBe("/cleanup-b");
		});

		it("fails loud for 304 responses", async () => {
			const client = await load_client();
			vi.spyOn(window, "fetch").mockResolvedValue(
				new Response(null, { status: 304 }),
			);

			await expect(client.vormaNavigate("/nav-304")).rejects.toThrow(
				/304/,
			);
			expect_status_idle(client.getStatus());
		});
	});
});
