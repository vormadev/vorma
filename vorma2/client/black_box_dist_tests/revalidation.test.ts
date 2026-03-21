// Assertions covered: 5, 6, 7, 8, 19, 20, 21

import { beforeEach, describe, expect, it, vi } from "vitest";
import {
	reset_client_runtime_for_testing,
	set_hard_redirect_handler_for_testing,
} from "vorma/testing";
import {
	create_deferred,
	create_deferred_fetch_call,
	create_route_data_response,
	expect_status_idle,
	load_client,
	request_input_to_url,
} from "./setup.ts";

beforeEach(() => {
	reset_client_runtime_for_testing();
});

describe("revalidation", () => {
	// ─── Assertion 6: Status Lifecycle ───────────────────────

	describe("status lifecycle", () => {
		it("sets isRevalidating while in flight and clears on completion", async () => {
			const client = await load_client();
			const deferred = create_deferred<Response>();
			vi.spyOn(window, "fetch").mockImplementation(
				() => deferred.promise,
			);

			const revalidate_promise = client.revalidate();
			await Promise.resolve();

			expect(client.getStatus()).toEqual({
				isNavigating: false,
				isSubmitting: false,
				isRevalidating: true,
			});

			deferred.resolve(
				create_route_data_response({
					title: { dangerousInnerHTML: "Revalidated" },
				}),
			);

			await revalidate_promise;
			await vi.runAllTimersAsync();

			expect(document.title).toBe("Revalidated");
			expect_status_idle(client.getStatus());
		});

		it("does not mutate history during revalidation", async () => {
			const client = await load_client();
			window.history.replaceState({}, "", "/revalidate-history?x=1");
			const push_spy = vi.spyOn(window.history, "pushState");
			const replace_spy = vi.spyOn(window.history, "replaceState");
			vi.spyOn(window, "fetch").mockResolvedValue(
				create_route_data_response(),
			);

			await client.revalidate();
			await vi.runAllTimersAsync();

			expect(push_spy).not.toHaveBeenCalled();
			expect(replace_spy).not.toHaveBeenCalled();
			expect(window.location.pathname).toBe("/revalidate-history");
			expect(window.location.search).toBe("?x=1");
		});

		it("revalidates against the current url including search params", async () => {
			const client = await load_client();
			window.history.replaceState(
				{},
				"",
				"/revalidate-query?foo=bar&n=1",
			);
			const fetch_spy = vi
				.spyOn(window, "fetch")
				.mockResolvedValue(create_route_data_response());

			await client.revalidate();
			await vi.runAllTimersAsync();

			expect(fetch_spy).toHaveBeenCalledTimes(1);
			const url = request_input_to_url(
				fetch_spy.mock.calls[0]![0] as RequestInfo | URL,
			);
			expect(url.pathname).toBe("/revalidate-query");
			expect(url.searchParams.get("foo")).toBe("bar");
			expect(url.searchParams.get("n")).toBe("1");
		});
	});

	// ─── Assertion 5: No-op Navigation During Revalidation ──

	describe("no-op navigation during revalidation", () => {
		it("does not clear in-flight revalidation state on same-document no-op", async () => {
			const client = await load_client();
			window.history.replaceState({}, "", "/noop-while-revalidating");
			const deferred = create_deferred<Response>();
			vi.spyOn(window, "fetch").mockImplementation(
				() => deferred.promise,
			);

			const revalidate_promise = client.revalidate();
			await Promise.resolve();

			await client.vormaNavigate("/noop-while-revalidating");
			await vi.runAllTimersAsync();

			expect(client.getStatus()).toEqual({
				isNavigating: false,
				isSubmitting: false,
				isRevalidating: true,
			});

			deferred.resolve(create_route_data_response());
			await revalidate_promise;
			await vi.runAllTimersAsync();

			expect_status_idle(client.getStatus());
		});

		it("can report navigating and revalidating simultaneously", async () => {
			const client = await load_client();
			const nav_deferred = create_deferred<Response>();
			const rev_deferred = create_deferred<Response>();
			vi.spyOn(window, "fetch")
				.mockImplementationOnce(() => nav_deferred.promise)
				.mockImplementationOnce(() => rev_deferred.promise);

			const nav_promise = client.vormaNavigate("/dual-lane");
			await Promise.resolve();
			const rev_promise = client.revalidate();
			await Promise.resolve();

			expect(client.getStatus()).toEqual({
				isNavigating: true,
				isSubmitting: false,
				isRevalidating: true,
			});

			nav_deferred.resolve(create_route_data_response());
			rev_deferred.resolve(create_route_data_response());

			await Promise.all([nav_promise, rev_promise]);
			await vi.runAllTimersAsync();

			expect_status_idle(client.getStatus());
		});
	});

	// ─── Assertion 7: Redirect Ownership ─────────────────────

	describe("redirect ownership", () => {
		it("follows redirects while current location still owns the result", async () => {
			const client = await load_client();
			window.history.replaceState({}, "", "/revalidate-start");
			vi.spyOn(window, "fetch")
				.mockResolvedValueOnce(
					new Response("", {
						status: 302,
						headers: {
							"X-Client-Redirect": "/revalidate-target",
							"X-Vorma-Client-Build-Id": "2",
						},
					}),
				)
				.mockResolvedValueOnce(
					create_route_data_response(
						{},
						{
							headers: {
								"X-Vorma-Client-Build-Id": "2",
							},
						},
					),
				);

			await client.revalidate();
			await vi.runAllTimersAsync();

			expect(window.location.pathname).toBe("/revalidate-target");
			expect_status_idle(client.getStatus());
		});

		it("does not follow stale redirects after target ownership changes", async () => {
			const client = await load_client();
			window.history.replaceState({}, "", "/revalidate-stale-a");
			const first_deferred = create_deferred<Response>();
			const fetch_spy = vi
				.spyOn(window, "fetch")
				.mockImplementationOnce(() => first_deferred.promise)
				.mockResolvedValueOnce(
					create_route_data_response({
						title: { dangerousInnerHTML: "Stale Winner" },
					}),
				);

			const first = client.revalidate();
			await vi.advanceTimersByTimeAsync(8);

			window.history.replaceState({}, "", "/revalidate-stale-b");
			const second = client.revalidate();
			await second;

			first_deferred.resolve(
				new Response("", {
					status: 200,
					headers: {
						"X-Client-Redirect": "/revalidate-stale-redirect",
					},
				}),
			);
			await first;
			await vi.runAllTimersAsync();

			expect(window.location.pathname).toBe("/revalidate-stale-b");
			expect(document.title).toBe("Stale Winner");
			expect(fetch_spy).toHaveBeenCalledTimes(2);
			expect_status_idle(client.getStatus());
		});

		it("does not follow stale native redirects after external location change", async () => {
			const client = await load_client();
			window.history.replaceState(
				{},
				"",
				"/revalidate-native-redirect-a",
			);
			const deferred = create_deferred<Response>();
			const native_redirect = create_route_data_response();
			Object.defineProperty(native_redirect, "redirected", {
				value: true,
				configurable: true,
			});
			Object.defineProperty(native_redirect, "url", {
				value: `${window.location.origin}/revalidate-native-stale`,
				configurable: true,
			});
			const fetch_spy = vi
				.spyOn(window, "fetch")
				.mockImplementationOnce(() => deferred.promise);

			const rev_promise = client.revalidate();
			await vi.advanceTimersByTimeAsync(8);

			window.history.pushState({}, "", "/revalidate-native-redirect-b");
			deferred.resolve(native_redirect);
			await rev_promise;
			await vi.runAllTimersAsync();

			expect(fetch_spy).toHaveBeenCalledTimes(1);
			expect(window.location.pathname).toBe(
				"/revalidate-native-redirect-b",
			);
			expect_status_idle(client.getStatus());
		});
	});

	// ─── Assertion 8: Search-Param Stale Boundary ────────────

	describe("search-param stale boundary", () => {
		it("discards revalidation results after search-param change", async () => {
			const client = await load_client();
			window.history.replaceState({}, "", "/revalidate-search?x=1");
			const deferred = create_deferred<Response>();
			vi.spyOn(window, "fetch").mockImplementation(
				() => deferred.promise,
			);

			const rev_promise = client.revalidate();
			await vi.advanceTimersByTimeAsync(8);

			window.history.pushState({}, "", "/revalidate-search?x=2");
			deferred.resolve(
				create_route_data_response({
					title: { dangerousInnerHTML: "Stale Search" },
				}),
			);
			await rev_promise;
			await vi.runAllTimersAsync();

			expect(window.location.pathname).toBe("/revalidate-search");
			expect(window.location.search).toBe("?x=2");
			expect(document.title).not.toBe("Stale Search");
			expect_status_idle(client.getStatus());
		});

		it("does not follow stale redirects after search-param change", async () => {
			const client = await load_client();
			window.history.replaceState({}, "", "/revalidate-redirect-a");
			const deferred = create_deferred<Response>();
			const fetch_spy = vi
				.spyOn(window, "fetch")
				.mockImplementationOnce(() => deferred.promise);

			const rev_promise = client.revalidate();
			await vi.advanceTimersByTimeAsync(8);

			window.history.pushState({}, "", "/revalidate-redirect-b");
			deferred.resolve(
				new Response("", {
					status: 200,
					headers: {
						"X-Client-Redirect": "/revalidate-redirect-stale",
					},
				}),
			);
			await rev_promise;
			await vi.runAllTimersAsync();

			expect(fetch_spy).toHaveBeenCalledTimes(1);
			expect(window.location.pathname).toBe("/revalidate-redirect-b");
			expect_status_idle(client.getStatus());
		});
	});

	// ─── Assertion 19: Coalescing and Trailing ───────────────

	describe("coalescing and trailing", () => {
		it("coalesces rapid same-target revalidate calls into one fetch", async () => {
			const client = await load_client();
			let resolve_fetch: ((value: Response) => void) | undefined;
			const fetch_spy = vi.spyOn(window, "fetch").mockImplementation(
				() =>
					new Promise<Response>((resolve) => {
						resolve_fetch = resolve;
					}),
			);

			const first = client.revalidate();
			const second = client.revalidate();
			const third = client.revalidate();

			expect(fetch_spy).toHaveBeenCalledTimes(1);

			resolve_fetch!(create_route_data_response());
			await Promise.all([first, second, third]);
			await vi.runAllTimersAsync();

			expect_status_idle(client.getStatus());
		});

		it("keeps one in-flight and runs at most one trailing pass", async () => {
			const client = await load_client();
			const first_deferred = create_deferred<Response>();
			const trailing_deferred = create_deferred<Response>();
			const fetch_spy = vi
				.spyOn(window, "fetch")
				.mockImplementationOnce(() => first_deferred.promise)
				.mockImplementationOnce(() => trailing_deferred.promise);

			const first = client.revalidate();
			await vi.advanceTimersByTimeAsync(8);

			const second = client.revalidate();
			const third = client.revalidate();
			await vi.advanceTimersByTimeAsync(16);

			expect(fetch_spy).toHaveBeenCalledTimes(1);

			first_deferred.resolve(
				create_route_data_response({
					title: { dangerousInnerHTML: "First Revalidation" },
				}),
			);
			await first;
			await vi.advanceTimersByTimeAsync(8);

			expect(fetch_spy).toHaveBeenCalledTimes(2);

			trailing_deferred.resolve(
				create_route_data_response({
					title: { dangerousInnerHTML: "Trailing Revalidation" },
				}),
			);
			await Promise.all([second, third]);
			await vi.runAllTimersAsync();

			expect(document.title).toBe("Trailing Revalidation");
			expect_status_idle(client.getStatus());
		});

		it("does not coalesce across data-target changes", async () => {
			const client = await load_client();
			window.history.replaceState({}, "", "/revalidate-a");
			const first_fetch = create_deferred_fetch_call();
			const fetch_spy = vi
				.spyOn(window, "fetch")
				.mockImplementationOnce(
					(input: RequestInfo | URL, init?: RequestInit) => {
						const promise = first_fetch.mock(input, init);
						first_fetch.get_signal()?.addEventListener(
							"abort",
							() => {
								first_fetch.deferred.reject(
									new DOMException("Aborted", "AbortError"),
								);
							},
							{ once: true },
						);
						return promise;
					},
				)
				.mockImplementationOnce(() =>
					Promise.resolve(create_route_data_response()),
				);

			const first = client.revalidate();
			await vi.advanceTimersByTimeAsync(8);

			window.history.replaceState({}, "", "/revalidate-b");
			const second = client.revalidate();

			expect(first_fetch.get_signal()?.aborted).toBe(true);

			await Promise.all([first, second]);
			await vi.runAllTimersAsync();

			expect(fetch_spy).toHaveBeenCalledTimes(2);
			const second_url = request_input_to_url(
				fetch_spy.mock.calls[1]![0] as RequestInfo | URL,
			);
			expect(second_url.pathname).toBe("/revalidate-b");
			expect_status_idle(client.getStatus());
		});
	});

	// ─── Assertion 20: Side-Effect Ownership ─────────────────

	describe("side-effect ownership", () => {
		it("applies results when hash changes but pathname stays the same", async () => {
			const client = await load_client();
			window.history.replaceState({}, "", "/revalidate-hash#initial");
			const deferred = create_deferred<Response>();
			vi.spyOn(window, "fetch").mockImplementation(
				() => deferred.promise,
			);

			const rev_promise = client.revalidate();
			await vi.advanceTimersByTimeAsync(8);

			window.history.replaceState({}, "", "/revalidate-hash#next");
			deferred.resolve(
				create_route_data_response({
					title: { dangerousInnerHTML: "Hash Applied" },
				}),
			);
			await rev_promise;
			await vi.runAllTimersAsync();

			expect(window.location.hash).toBe("#next");
			expect(document.title).toBe("Hash Applied");
			expect_status_idle(client.getStatus());
		});

		it("ignores stale results after external location change", async () => {
			const client = await load_client();
			window.history.replaceState({}, "", "/revalidate-external-a");
			const deferred = create_deferred<Response>();
			vi.spyOn(window, "fetch").mockImplementation(
				() => deferred.promise,
			);

			const rev_promise = client.revalidate();
			await vi.advanceTimersByTimeAsync(8);

			window.history.pushState({}, "", "/revalidate-external-b");
			deferred.resolve(
				create_route_data_response({
					title: { dangerousInnerHTML: "Stale External" },
				}),
			);
			await rev_promise;
			await vi.runAllTimersAsync();

			expect(window.location.pathname).toBe("/revalidate-external-b");
			expect(document.title).not.toBe("Stale External");
			expect_status_idle(client.getStatus());
		});

		it("does not let stale results override later navigation state or css", async () => {
			const client = await load_client();
			window.history.replaceState({}, "", "/");
			const rev_deferred = create_deferred<Response>();
			vi.spyOn(window, "fetch").mockImplementation(
				(input: RequestInfo | URL) => {
					const url = request_input_to_url(input);
					if (url.pathname === "/about") {
						return Promise.resolve(
							create_route_data_response({
								title: { dangerousInnerHTML: "About Page" },
							}),
						);
					}
					return rev_deferred.promise;
				},
			);

			const rev_promise = client.revalidate();
			await vi.advanceTimersByTimeAsync(8);

			await client.vormaNavigate("/about");
			await vi.runAllTimersAsync();
			expect(window.location.pathname).toBe("/about");
			expect(document.title).toBe("About Page");

			rev_deferred.resolve(
				create_route_data_response({
					title: { dangerousInnerHTML: "Stale Revalidation" },
					css_bundles: ["/stale-revalidation.css"],
				}),
			);
			await rev_promise;
			await vi.advanceTimersByTimeAsync(32);
			await vi.runAllTimersAsync();

			expect(window.location.pathname).toBe("/about");
			expect(document.title).toBe("About Page");
			expect(
				document.head.querySelector(
					'link[data-vorma-css-bundle="/stale-revalidation.css"]',
				),
			).toBeNull();
			expect_status_idle(client.getStatus());
		});

		it("does not update build id from stale results after external location change", async () => {
			const client = await load_client();
			window.history.replaceState({}, "", "/revalidate-build-id-a");
			const deferred = create_deferred<Response>();
			vi.spyOn(window, "fetch").mockImplementation(
				() => deferred.promise,
			);

			const rev_promise = client.revalidate();
			await vi.advanceTimersByTimeAsync(8);

			window.history.pushState({}, "", "/revalidate-build-id-b");
			deferred.resolve(
				create_route_data_response(
					{},
					{
						headers: {
							"X-Vorma-Client-Build-Id": "stale-build-id",
						},
					},
				),
			);
			await rev_promise;
			await vi.runAllTimersAsync();

			expect(client.getClientBuildID()).toBe("1");
			expect_status_idle(client.getStatus());
		});

		it("does not perform hard reload from stale results after external location change", async () => {
			const client = await load_client();
			window.history.replaceState({}, "", "/revalidate-hard-reload-a");
			const deferred = create_deferred<Response>();
			vi.spyOn(window, "fetch").mockImplementation(
				() => deferred.promise,
			);
			const hard_redirect_spy = vi.fn();
			set_hard_redirect_handler_for_testing(hard_redirect_spy);

			try {
				const rev_promise = client.revalidate();
				await vi.advanceTimersByTimeAsync(8);

				window.history.pushState({}, "", "/revalidate-hard-reload-b");
				deferred.resolve(
					create_route_data_response(
						{},
						{
							headers: {
								"X-Vorma-Reload": "/revalidate-stale-hard",
								"X-Vorma-Client-Build-Id": "stale-hard-build",
							},
						},
					),
				);
				await rev_promise;
				await vi.runAllTimersAsync();

				expect(hard_redirect_spy).not.toHaveBeenCalled();
				expect(window.location.pathname).toBe(
					"/revalidate-hard-reload-b",
				);
				expect(client.getClientBuildID()).toBe("1");
				expect_status_idle(client.getStatus());
			} finally {
				set_hard_redirect_handler_for_testing(undefined);
			}
		});

		it("does not apply stale hard-reload or build-id after ownership changes via revalidation", async () => {
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
			const hard_redirect_spy = vi.fn();
			set_hard_redirect_handler_for_testing(hard_redirect_spy);

			try {
				window.history.replaceState({}, "", "/revalidate-stale-hard-a");
				const first_deferred = create_deferred<Response>();
				vi.spyOn(window, "fetch")
					.mockImplementationOnce(() => first_deferred.promise)
					.mockResolvedValueOnce(
						create_route_data_response(
							{
								title: {
									dangerousInnerHTML: "Hard Winner",
								},
							},
							{
								headers: {
									"X-Vorma-Client-Build-Id":
										"winner-build-id",
								},
							},
						),
					);

				const first = client.revalidate();
				await vi.advanceTimersByTimeAsync(8);

				window.history.replaceState({}, "", "/revalidate-stale-hard-b");
				const second = client.revalidate();
				await second;

				first_deferred.resolve(
					create_route_data_response(
						{},
						{
							headers: {
								"X-Vorma-Reload":
									"/revalidate-stale-hard-reload",
								"X-Vorma-Client-Build-Id": "stale-build-id",
							},
						},
					),
				);
				await first;
				await vi.runAllTimersAsync();

				expect(hard_redirect_spy).not.toHaveBeenCalled();
				expect(window.location.pathname).toBe(
					"/revalidate-stale-hard-b",
				);
				expect(
					build_id_events.every(
						(e) => e.newClientBuildID !== "stale-build-id",
					),
				).toBe(true);
				expect_status_idle(client.getStatus());
			} finally {
				set_hard_redirect_handler_for_testing(undefined);
				remove_listener();
			}
		});
	});

	// ─── Assertion 21: Focus-Triggered Revalidation ──────────

	describe("focus-triggered revalidation", () => {
		it("revalidates on focus only after staleTime has elapsed", async () => {
			const client = await load_client();
			const fetch_spy = vi
				.spyOn(window, "fetch")
				.mockResolvedValue(create_route_data_response());
			const cleanup = client.revalidateOnWindowFocus({
				staleTimeMS: 1000,
			});

			try {
				window.dispatchEvent(new Event("focus"));
				await Promise.resolve();
				await vi.runAllTimersAsync();
				expect(fetch_spy).toHaveBeenCalledTimes(0);

				await vi.advanceTimersByTimeAsync(1001);
				window.dispatchEvent(new Event("focus"));
				await Promise.resolve();
				await vi.runAllTimersAsync();

				expect(fetch_spy).toHaveBeenCalledTimes(1);
			} finally {
				cleanup();
			}
		});

		it("resets stale-time window after successful navigation", async () => {
			const client = await load_client();
			const fetch_spy = vi
				.spyOn(window, "fetch")
				.mockImplementation(() =>
					Promise.resolve(create_route_data_response()),
				);

			await vi.advanceTimersByTimeAsync(10);
			await client.vormaNavigate("/focus-stale-gate");
			await vi.runAllTimersAsync();

			const cleanup = client.revalidateOnWindowFocus({
				staleTimeMS: 100,
			});
			try {
				window.dispatchEvent(new Event("focus"));
				await Promise.resolve();
				await vi.runAllTimersAsync();
				expect(fetch_spy).toHaveBeenCalledTimes(1);

				await vi.advanceTimersByTimeAsync(101);
				window.dispatchEvent(new Event("focus"));
				await Promise.resolve();
				await vi.runAllTimersAsync();
				expect(fetch_spy).toHaveBeenCalledTimes(2);
			} finally {
				cleanup();
			}
		});

		it("resets stale-time window after successful revalidation", async () => {
			const client = await load_client();
			void client.getStatus();
			const fetch_spy = vi
				.spyOn(window, "fetch")
				.mockImplementation(() =>
					Promise.resolve(create_route_data_response()),
				);
			const cleanup = client.revalidateOnWindowFocus({
				staleTimeMS: 100,
			});
			try {
				await vi.advanceTimersByTimeAsync(101);
				window.dispatchEvent(new Event("focus"));
				await Promise.resolve();
				await vi.runAllTimersAsync();
				expect(fetch_spy).toHaveBeenCalledTimes(1);

				window.dispatchEvent(new Event("focus"));
				await Promise.resolve();
				await vi.runAllTimersAsync();
				expect(fetch_spy).toHaveBeenCalledTimes(1);

				await vi.advanceTimersByTimeAsync(101);
				window.dispatchEvent(new Event("focus"));
				await Promise.resolve();
				await vi.runAllTimersAsync();
				expect(fetch_spy).toHaveBeenCalledTimes(2);
			} finally {
				cleanup();
			}
		});

		it("does not reset stale-time window for hash-only navigations", async () => {
			const client = await load_client();
			void client.getStatus();
			const fetch_spy = vi
				.spyOn(window, "fetch")
				.mockImplementation(() =>
					Promise.resolve(create_route_data_response()),
				);
			const cleanup = client.revalidateOnWindowFocus({
				staleTimeMS: 100,
			});
			try {
				await vi.advanceTimersByTimeAsync(90);
				await client.vormaNavigate("/#focus-hash-no-reset");
				await vi.runAllTimersAsync();

				await vi.advanceTimersByTimeAsync(20);
				window.dispatchEvent(new Event("focus"));
				await Promise.resolve();
				await vi.runAllTimersAsync();

				expect(fetch_spy).toHaveBeenCalledTimes(1);
			} finally {
				cleanup();
			}
		});

		it("does not advance stale-time timestamp for aborted navigations", async () => {
			const client = await load_client();
			void client.getStatus();
			const fetch_spy = vi
				.spyOn(window, "fetch")
				.mockRejectedValueOnce(
					new DOMException("Aborted", "AbortError"),
				)
				.mockResolvedValue(create_route_data_response());
			const cleanup = client.revalidateOnWindowFocus({
				staleTimeMS: 100,
			});
			try {
				await vi.advanceTimersByTimeAsync(90);
				await expect(
					client.vormaNavigate("/focus-aborted"),
				).resolves.toEqual({ didNavigate: false });
				await vi.runAllTimersAsync();

				await vi.advanceTimersByTimeAsync(20);
				window.dispatchEvent(new Event("focus"));
				await Promise.resolve();
				await vi.runAllTimersAsync();

				expect(fetch_spy).toHaveBeenCalledTimes(2);
			} finally {
				cleanup();
			}
		});

		it("does not advance stale-time timestamp for aborted revalidations", async () => {
			const client = await load_client();
			void client.getStatus();
			const fetch_spy = vi
				.spyOn(window, "fetch")
				.mockRejectedValueOnce(
					new DOMException("Aborted", "AbortError"),
				)
				.mockResolvedValue(create_route_data_response());
			const cleanup = client.revalidateOnWindowFocus({
				staleTimeMS: 100,
			});
			try {
				await vi.advanceTimersByTimeAsync(101);
				window.dispatchEvent(new Event("focus"));
				await Promise.resolve();
				await vi.runAllTimersAsync();
				expect(fetch_spy).toHaveBeenCalledTimes(1);

				window.dispatchEvent(new Event("focus"));
				await Promise.resolve();
				await vi.runAllTimersAsync();
				expect(fetch_spy).toHaveBeenCalledTimes(2);
			} finally {
				cleanup();
			}
		});

		it("does not revalidate on focus during active navigation", async () => {
			const client = await load_client();
			const nav_deferred = create_deferred<Response>();
			let fetch_call_count = 0;
			vi.spyOn(window, "fetch").mockImplementation(() => {
				fetch_call_count += 1;
				if (fetch_call_count === 1) {
					return nav_deferred.promise;
				}
				return Promise.resolve(create_route_data_response());
			});
			const cleanup = client.revalidateOnWindowFocus({
				staleTimeMS: 0,
			});
			try {
				const nav_promise = client.vormaNavigate("/focus-during-nav");
				await vi.advanceTimersByTimeAsync(8);

				window.dispatchEvent(new Event("focus"));
				await Promise.resolve();
				await vi.advanceTimersByTimeAsync(16);

				expect(fetch_call_count).toBe(1);

				nav_deferred.resolve(create_route_data_response());
				await nav_promise;
				await vi.runAllTimersAsync();
				expect_status_idle(client.getStatus());
			} finally {
				cleanup();
			}
		});

		it("does not revalidate on focus during active submission", async () => {
			const client = await load_client();
			const submit_deferred = create_deferred<Response>();
			const fetch_spy = vi
				.spyOn(window, "fetch")
				.mockImplementation(() => {
					if (fetch_spy.mock.calls.length <= 1) {
						return submit_deferred.promise;
					}
					return Promise.resolve(create_route_data_response());
				});
			const cleanup = client.revalidateOnWindowFocus({
				staleTimeMS: 0,
			});
			try {
				const submit_promise = client.submit(
					"/focus-while-submit",
					{ method: "POST" },
					{ revalidate: false },
				);
				await vi.advanceTimersByTimeAsync(8);

				window.dispatchEvent(new Event("focus"));
				await Promise.resolve();
				await vi.advanceTimersByTimeAsync(16);
				expect(fetch_spy).toHaveBeenCalledTimes(1);

				submit_deferred.resolve(
					new Response(JSON.stringify({ ok: true }), {
						status: 200,
						headers: { "Content-Type": "application/json" },
					}),
				);
				await submit_promise;
				await vi.runAllTimersAsync();
			} finally {
				cleanup();
			}
		});

		it("does not revalidate on focus during active revalidation", async () => {
			const client = await load_client();
			const rev_deferred = create_deferred<Response>();
			const fetch_spy = vi
				.spyOn(window, "fetch")
				.mockImplementation(() => {
					if (fetch_spy.mock.calls.length <= 1) {
						return rev_deferred.promise;
					}
					return Promise.resolve(create_route_data_response());
				});
			const cleanup = client.revalidateOnWindowFocus({
				staleTimeMS: 0,
			});
			try {
				const rev_promise = client.revalidate();
				await vi.advanceTimersByTimeAsync(8);

				window.dispatchEvent(new Event("focus"));
				await Promise.resolve();
				await vi.advanceTimersByTimeAsync(16);
				expect(fetch_spy).toHaveBeenCalledTimes(1);

				rev_deferred.resolve(create_route_data_response());
				await rev_promise;
				await vi.runAllTimersAsync();
			} finally {
				cleanup();
			}
		});

		it("stops revalidate-on-focus after cleanup", async () => {
			const client = await load_client();
			const fetch_spy = vi
				.spyOn(window, "fetch")
				.mockResolvedValue(create_route_data_response());
			const cleanup = client.revalidateOnWindowFocus({
				staleTimeMS: 0,
			});
			cleanup();

			window.dispatchEvent(new Event("focus"));
			await Promise.resolve();
			await vi.runAllTimersAsync();

			expect(fetch_spy).not.toHaveBeenCalled();
		});
	});
});
