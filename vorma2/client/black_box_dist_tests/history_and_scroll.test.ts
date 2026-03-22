// Assertions covered: 13, 15, 16

import { beforeEach, describe, expect, it, vi } from "vitest";
import {
	is_page_refresh_scroll_state_storage_key_for_testing,
	is_scroll_state_storage_key_for_testing,
	pop_back_for_testing,
	push_history_for_testing,
	read_current_history_key_for_testing,
	read_page_refresh_scroll_state_for_testing,
	read_scroll_state_for_testing,
	reset_client_runtime_for_testing,
	seed_page_refresh_scroll_state_for_testing,
	seed_scroll_state_for_testing,
	set_hard_redirect_handler_for_testing,
	write_raw_scroll_state_storage_for_testing,
} from "vorma/testing";
import type { StatusSnapshot } from "./setup.ts";
import {
	create_deferred,
	create_route_data_response,
	expect_status_idle,
	load_client,
	request_input_to_url,
	stub_window_location_href,
	wait_for_request_count,
} from "./setup.ts";

beforeEach(() => {
	reset_client_runtime_for_testing();
});

describe("history and scroll", () => {
	// ─── Assertion 13: Committed Events and Listeners ────────

	describe("committed events and listeners", () => {
		it("dispatches route-change only after committed location and router snapshot are coherent", async () => {
			const client = await load_client();
			const observed: {
				pathname: string;
				router_data: ReturnType<typeof client.getRouterData>;
			}[] = [];
			const cleanup = client.addRouteChangeListener(() => {
				observed.push({
					pathname: window.location.pathname,
					router_data: client.getRouterData(),
				});
			});

			try {
				vi.spyOn(window, "fetch").mockResolvedValue(
					create_route_data_response(),
				);

				await client.vormaNavigate("/snapshot-coherence");
				await vi.runAllTimersAsync();

				expect(observed).toHaveLength(1);
				expect(observed[0]!.pathname).toBe("/snapshot-coherence");
				expect(observed[0]!.router_data).toEqual(
					client.getRouterData(),
				);
			} finally {
				cleanup();
			}
		});

		it("emits hash scroll state in route-change events for hash-only navigation", async () => {
			const client = await load_client();
			await client.initClient({});
			window.history.replaceState({}, "", "/hash-scroll-base");
			const details: unknown[] = [];
			const cleanup = client.addRouteChangeListener(
				(event: CustomEvent) => {
					details.push(event.detail);
				},
			);

			try {
				await client.vormaNavigate("/hash-scroll-base#chapter-1");
				await vi.runAllTimersAsync();

				expect(window.location.hash).toBe("#chapter-1");
				expect(details.at(-1)).toEqual({});
			} finally {
				cleanup();
			}
		});

		it("emits top scroll state in route-change events for standard navigation", async () => {
			const client = await load_client();
			const details: unknown[] = [];
			const cleanup = client.addRouteChangeListener(
				(event: CustomEvent) => {
					details.push(event.detail);
				},
			);

			try {
				vi.spyOn(window, "fetch").mockResolvedValue(
					create_route_data_response(),
				);

				await client.vormaNavigate("/top-scroll-target");
				await vi.runAllTimersAsync();

				expect(details.at(-1)).toEqual({
					__scrollState: { x: 0, y: 0 },
				});
			} finally {
				cleanup();
			}
		});

		it("emits undefined scroll state when scrollToTop is disabled", async () => {
			const client = await load_client();
			const details: unknown[] = [];
			const cleanup = client.addRouteChangeListener(
				(event: CustomEvent) => {
					details.push(event.detail);
				},
			);

			try {
				vi.spyOn(window, "fetch").mockResolvedValue(
					create_route_data_response(),
				);

				await client.vormaNavigate("/no-top-scroll", {
					scrollToTop: false,
				});
				await vi.runAllTimersAsync();

				expect(details.at(-1)).toEqual({
					__scrollState: undefined,
				});
			} finally {
				cleanup();
			}
		});

		it("includes restored scroll state in browser-history route-change events", async () => {
			const client = await load_client();
			await client.initClient({});
			const details: unknown[] = [];
			const cleanup = client.addRouteChangeListener(
				(event: CustomEvent) => {
					details.push(event.detail);
				},
			);

			try {
				vi.spyOn(window, "fetch").mockResolvedValue(
					create_route_data_response(),
				);

				push_history_for_testing("/history-pop-scroll-target");
				await vi.runAllTimersAsync();
				const target_key = read_current_history_key_for_testing();
				push_history_for_testing("/history-pop-scroll-source");
				await vi.runAllTimersAsync();

				seed_scroll_state_for_testing([
					{ history_key: target_key, x: 120, y: 240 },
				]);
				pop_back_for_testing();
				await vi.runAllTimersAsync();

				expect(window.location.pathname).toBe(
					"/history-pop-scroll-target",
				);
				expect(details.at(-1)).toEqual(
					expect.objectContaining({
						__scrollState: { x: 120, y: 240 },
					}),
				);
			} finally {
				cleanup();
			}
		});

		it("stops status event delivery after listener cleanup", async () => {
			const client = await load_client();
			const statuses: StatusSnapshot[] = [];
			const cleanup = client.addStatusListener(
				(event: CustomEvent<StatusSnapshot>) => {
					statuses.push(event.detail);
				},
			);

			vi.spyOn(window, "fetch").mockImplementation(() =>
				Promise.resolve(create_route_data_response()),
			);

			await client.vormaNavigate("/event-listener-a");
			await vi.runAllTimersAsync();
			expect(statuses.length).toBeGreaterThan(0);

			cleanup();
			const count_after_cleanup = statuses.length;

			await client.vormaNavigate("/event-listener-b");
			await vi.runAllTimersAsync();

			expect(statuses).toHaveLength(count_after_cleanup);
		});

		it("stops route-change event delivery after listener cleanup", async () => {
			const client = await load_client();
			const events: unknown[] = [];
			const cleanup = client.addRouteChangeListener(
				(event: CustomEvent) => {
					events.push(event.detail);
				},
			);

			vi.spyOn(window, "fetch")
				.mockResolvedValueOnce(create_route_data_response())
				.mockResolvedValueOnce(create_route_data_response());

			await client.vormaNavigate("/route-change-a");
			await vi.runAllTimersAsync();
			expect(events.length).toBeGreaterThan(0);

			cleanup();
			const count_after_cleanup = events.length;

			await client.vormaNavigate("/route-change-b");
			await vi.runAllTimersAsync();

			expect(events).toHaveLength(count_after_cleanup);
		});

		it("stops build-id event delivery after listener cleanup", async () => {
			const client = await load_client();
			const events: unknown[] = [];
			const cleanup = client.addClientBuildIDListener(
				(
					event: CustomEvent<{
						oldClientBuildID: string;
						newClientBuildID: string;
					}>,
				) => {
					events.push(event.detail);
				},
			);

			vi.spyOn(window, "fetch")
				.mockResolvedValueOnce(
					create_route_data_response(
						{},
						{
							headers: {
								"X-Vorma-Client-Build-Id": "build-a",
							},
						},
					),
				)
				.mockResolvedValueOnce(
					create_route_data_response(
						{},
						{
							headers: {
								"X-Vorma-Client-Build-Id": "build-b",
							},
						},
					),
				);

			await client.vormaNavigate("/build-id-listener-a");
			await vi.runAllTimersAsync();
			expect(events).toHaveLength(1);

			cleanup();
			const count_after_cleanup = events.length;

			await client.vormaNavigate("/build-id-listener-b");
			await vi.runAllTimersAsync();

			expect(events).toHaveLength(count_after_cleanup);
		});

		it("registers listeners on window for all event types", async () => {
			const client = await load_client();
			const add_spy = vi.spyOn(window, "addEventListener");

			const remove_status = client.addStatusListener(() => {});
			const remove_route = client.addRouteChangeListener(() => {});
			const remove_build = client.addClientBuildIDListener(() => {});

			expect(add_spy).toHaveBeenCalledWith(
				"vorma:status",
				expect.any(Function),
			);
			expect(add_spy).toHaveBeenCalledWith(
				"vorma:route-change",
				expect.any(Function),
			);
			expect(add_spy).toHaveBeenCalledWith(
				"vorma:client-build-id",
				expect.any(Function),
			);

			remove_status();
			remove_route();
			remove_build();
		});

		it("sets history scrollRestoration to manual during init", async () => {
			const client = await load_client();
			const setter_spy = vi.fn();
			Object.defineProperty(window.history, "scrollRestoration", {
				configurable: true,
				get: () => "auto",
				set: setter_spy,
			});

			await client.initClient({});

			expect(setter_spy).toHaveBeenCalledWith("manual");
		});

		it("registers beforeunload scroll persistence once across repeated init calls", async () => {
			const client = await load_client();
			window.scrollX = 200;
			window.scrollY = 400;
			const set_item_spy = vi.spyOn(Storage.prototype, "setItem");
			const count_page_refresh_writes = () =>
				set_item_spy.mock.calls.filter(
					([key]) =>
						typeof key === "string" &&
						is_page_refresh_scroll_state_storage_key_for_testing(
							key,
						),
				).length;

			await client.initClient({});
			const before_first = count_page_refresh_writes();
			window.dispatchEvent(new Event("beforeunload"));
			const first_delta = count_page_refresh_writes() - before_first;

			await client.initClient({});
			const before_second = count_page_refresh_writes();
			window.dispatchEvent(new Event("beforeunload"));
			const second_delta = count_page_refresh_writes() - before_second;

			expect(second_delta).toBe(first_delta);
			const saved = read_page_refresh_scroll_state_for_testing();
			expect(saved).toBeTruthy();
			expect(saved).toMatchObject({
				x: 200,
				y: 400,
				href: window.location.href,
			});
			expect(typeof saved?.unix).toBe("number");
		});
	});

	// ─── Assertion 15: POP Navigation and Scroll ─────────────

	describe("same-document POP scroll restoration", () => {
		it("uses one decode step when resolving same-document POP hash targets", async () => {
			const client = await load_client();
			const percent_literal = document.createElement("div");
			percent_literal.id = "%20-literal";
			const scroll_spy = vi.fn();
			Object.defineProperty(percent_literal, "scrollIntoView", {
				value: scroll_spy,
				configurable: true,
			});
			document.body.appendChild(percent_literal);

			try {
				push_history_for_testing("/single-decode#%2520-literal");
				await vi.runAllTimersAsync();
				push_history_for_testing("/single-decode");
				await vi.runAllTimersAsync();

				pop_back_for_testing();
				await vi.runAllTimersAsync();

				expect(window.location.pathname).toBe("/single-decode");
				expect(window.location.hash).toBe("#%2520-literal");
				expect(scroll_spy).toHaveBeenCalledTimes(1);
			} finally {
				percent_literal.remove();
			}
		});

		it("resolves encoded unicode hash targets on same-document POP transitions", async () => {
			const client = await load_client();
			const checkmark = document.createElement("div");
			checkmark.id = "✓";
			const scroll_spy = vi.fn();
			Object.defineProperty(checkmark, "scrollIntoView", {
				value: scroll_spy,
				configurable: true,
			});
			document.body.appendChild(checkmark);

			try {
				push_history_for_testing("/decoded-unicode#%E2%9C%93");
				await vi.runAllTimersAsync();
				push_history_for_testing("/decoded-unicode");
				await vi.runAllTimersAsync();

				pop_back_for_testing();
				await vi.runAllTimersAsync();

				expect(window.location.pathname).toBe("/decoded-unicode");
				expect(window.location.hash).toBe("#%E2%9C%93");
				expect(scroll_spy).toHaveBeenCalledTimes(1);
			} finally {
				checkmark.remove();
			}
		});

		it("falls back to raw hash fragments when decode fails during same-document POP", async () => {
			const client = await load_client();
			const invalid_fragment = "%E0%A4%A";
			const raw_element = document.createElement("div");
			raw_element.id = invalid_fragment;
			const scroll_spy = vi.fn();
			Object.defineProperty(raw_element, "scrollIntoView", {
				value: scroll_spy,
				configurable: true,
			});
			document.body.appendChild(raw_element);

			try {
				push_history_for_testing(
					`/decode-fallback#${invalid_fragment}`,
				);
				await vi.runAllTimersAsync();
				push_history_for_testing("/decode-fallback");
				await vi.runAllTimersAsync();

				pop_back_for_testing();
				await vi.runAllTimersAsync();

				expect(window.location.pathname).toBe("/decode-fallback");
				expect(window.location.hash).toBe(`#${invalid_fragment}`);
				expect(scroll_spy).toHaveBeenCalledTimes(1);
			} finally {
				raw_element.remove();
			}
		});

		it("does not re-scroll when same-document POP hash targets are encoding-equivalent", async () => {
			const client = await load_client();
			const tilde = document.createElement("div");
			tilde.id = "~";
			const scroll_spy = vi.fn();
			Object.defineProperty(tilde, "scrollIntoView", {
				value: scroll_spy,
				configurable: true,
			});
			document.body.appendChild(tilde);

			try {
				push_history_for_testing("/equivalent-hash#~");
				await vi.runAllTimersAsync();
				push_history_for_testing("/equivalent-hash#%7E");
				await vi.runAllTimersAsync();

				pop_back_for_testing();
				await vi.runAllTimersAsync();

				expect(window.location.pathname).toBe("/equivalent-hash");
				expect(scroll_spy).toHaveBeenCalledTimes(0);
			} finally {
				tilde.remove();
			}
		});

		it("restores saved scroll coordinates when same-document POP removes a hash", async () => {
			const client = await load_client();
			push_history_for_testing("/hash-remove-target");
			await vi.runAllTimersAsync();
			const target_key = read_current_history_key_for_testing();
			seed_scroll_state_for_testing([
				{ history_key: target_key, x: 75, y: 150 },
			]);
			push_history_for_testing("/hash-remove-target#section");
			await vi.runAllTimersAsync();

			pop_back_for_testing();
			await vi.runAllTimersAsync();

			expect(window.location.pathname).toBe("/hash-remove-target");
			expect(window.location.hash).toBe("");
			expect(window.scrollTo).toHaveBeenCalledWith(75, 150);
		});

		it("falls back to origin scroll when same-document POP removes hash without stored state", async () => {
			const client = await load_client();
			push_history_for_testing("/hash-remove-origin-target");
			await vi.runAllTimersAsync();
			push_history_for_testing("/hash-remove-origin-target#section");
			await vi.runAllTimersAsync();

			pop_back_for_testing();
			await vi.runAllTimersAsync();

			expect(window.location.pathname).toBe("/hash-remove-origin-target");
			expect(window.location.hash).toBe("");
			expect(window.scrollTo).toHaveBeenCalledWith(0, 0);
		});

		it("restores saved scroll coordinates when same-document POP targets empty fragment '#'", async () => {
			const client = await load_client();
			push_history_for_testing("/hash-empty-fragment-target#");
			await vi.runAllTimersAsync();
			const target_key = read_current_history_key_for_testing();
			seed_scroll_state_for_testing([
				{ history_key: target_key, x: 88, y: 166 },
			]);
			push_history_for_testing("/hash-empty-fragment-target#section");
			await vi.runAllTimersAsync();

			pop_back_for_testing();
			await vi.runAllTimersAsync();

			expect(window.location.pathname).toBe(
				"/hash-empty-fragment-target",
			);
			expect(window.location.hash).toBe("");
			expect(window.scrollTo).toHaveBeenCalledWith(88, 166);
		});
	});

	describe("cross-document POP transitions", () => {
		it("fetches and commits route data for cross-document POP transitions", async () => {
			const client = await load_client();
			await client.initClient({});
			const fetch_spy = vi.spyOn(window, "fetch").mockResolvedValue(
				create_route_data_response({
					title: {
						dangerousInnerHTML: "POP Commit Title",
					},
				}),
			);

			push_history_for_testing("/cross-pop-target");
			await vi.runAllTimersAsync();
			push_history_for_testing("/cross-pop-source");
			await vi.runAllTimersAsync();

			pop_back_for_testing();
			await vi.runAllTimersAsync();

			expect(fetch_spy).toHaveBeenCalledTimes(1);
			const pop_url = request_input_to_url(
				fetch_spy.mock.calls[0]![0] as RequestInfo | URL,
			);
			expect(pop_url.pathname).toBe("/cross-pop-target");
			expect(pop_url.searchParams.has("vorma_json")).toBe(true);
			expect(window.location.pathname).toBe("/cross-pop-target");
			expect(document.title).toBe("POP Commit Title");
			expect_status_idle(client.getStatus());
		});

		it("follows cross-document POP redirects and commits redirected destination", async () => {
			const client = await load_client();
			await client.initClient({});
			const fetch_spy = vi
				.spyOn(window, "fetch")
				.mockResolvedValueOnce(
					new Response("", {
						status: 302,
						headers: {
							"X-Client-Redirect": "/pop-redirect-destination",
							"X-Vorma-Client-Build-Id": "2",
						},
					}),
				)
				.mockResolvedValueOnce(
					create_route_data_response(
						{
							title: {
								dangerousInnerHTML: "POP Redirect Destination",
							},
						},
						{
							headers: {
								"X-Vorma-Client-Build-Id": "2",
							},
						},
					),
				);

			push_history_for_testing("/pop-redirect-target");
			await vi.runAllTimersAsync();
			push_history_for_testing("/pop-redirect-source");
			await vi.runAllTimersAsync();

			pop_back_for_testing();
			await vi.runAllTimersAsync();

			expect(fetch_spy).toHaveBeenCalledTimes(2);
			expect(
				request_input_to_url(
					fetch_spy.mock.calls[0]![0] as RequestInfo | URL,
				).pathname,
			).toBe("/pop-redirect-target");
			expect(
				request_input_to_url(
					fetch_spy.mock.calls[1]![0] as RequestInfo | URL,
				).pathname,
			).toBe("/pop-redirect-destination");
			expect(window.location.pathname).toBe("/pop-redirect-destination");
			expect(document.title).toBe("POP Redirect Destination");
			expect_status_idle(client.getStatus());
		});

		it("uses POP listener payload URL as fetch source-of-truth", async () => {
			const client = await load_client();
			await client.initClient({});
			const fetch_spy = vi
				.spyOn(window, "fetch")
				.mockResolvedValue(create_route_data_response());

			push_history_for_testing("/listener-source?q=1#a");
			await vi.runAllTimersAsync();
			push_history_for_testing("/listener-target?q=2#details");
			await vi.runAllTimersAsync();

			window.history.replaceState(
				window.history.state,
				"",
				"/window-location-only?q=99#ignored",
			);
			pop_back_for_testing();
			await vi.runAllTimersAsync();

			expect(fetch_spy).toHaveBeenCalledTimes(1);
			const pop_url = request_input_to_url(
				fetch_spy.mock.calls[0]![0] as RequestInfo | URL,
			);
			expect(pop_url.pathname).toBe("/listener-source");
			expect(pop_url.searchParams.get("q")).toBe("1");
		});

		it("treats query-order changes as different POP targets", async () => {
			const client = await load_client();
			await client.initClient({});
			const fetch_spy = vi
				.spyOn(window, "fetch")
				.mockResolvedValue(create_route_data_response());

			push_history_for_testing("/pop-query-order?a=1&b=2");
			await vi.runAllTimersAsync();
			push_history_for_testing("/pop-query-order?b=2&a=1");
			await vi.runAllTimersAsync();

			pop_back_for_testing();
			await vi.runAllTimersAsync();

			expect(fetch_spy).toHaveBeenCalledTimes(1);
			const pop_url = request_input_to_url(
				fetch_spy.mock.calls[0]![0] as RequestInfo | URL,
			);
			expect(pop_url.pathname).toBe("/pop-query-order");
			expect(pop_url.searchParams.get("a")).toBe("1");
			expect(pop_url.searchParams.get("b")).toBe("2");
		});

		it("reloads browser when cross-document POP navigation fails", async () => {
			const client = await load_client();
			await client.initClient({});
			const hard_redirect_spy = vi.fn();
			set_hard_redirect_handler_for_testing(hard_redirect_spy);
			const console_error_spy = vi
				.spyOn(console, "error")
				.mockImplementation(() => {});
			vi.spyOn(window, "fetch").mockRejectedValue(
				new Error("pop-fetch-failure"),
			);

			try {
				push_history_for_testing("/pop-reload-target");
				await vi.runAllTimersAsync();
				push_history_for_testing("/pop-reload-source");
				await vi.runAllTimersAsync();

				pop_back_for_testing();
				await vi.runAllTimersAsync();

				expect(hard_redirect_spy).toHaveBeenCalledTimes(1);
				expect(String(hard_redirect_spy.mock.calls[0]![0])).toContain(
					"/pop-reload-target",
				);
				expect(
					console_error_spy.mock.calls.some((args) =>
						args.some(
							(v) =>
								typeof v === "string" &&
								v.includes("POP navigation failed"),
						),
					),
				).toBe(true);
			} finally {
				set_hard_redirect_handler_for_testing(undefined);
			}
		});

		it("logs when hard-reload fallback fails after cross-document POP errors", async () => {
			const client = await load_client();
			await client.initClient({});
			set_hard_redirect_handler_for_testing(() => {
				throw new Error("reload-failed");
			});
			const console_error_spy = vi
				.spyOn(console, "error")
				.mockImplementation(() => {});
			vi.spyOn(window, "fetch").mockRejectedValue(
				new Error("pop-fetch-failure"),
			);

			try {
				push_history_for_testing("/pop-reload-error-target");
				await vi.runAllTimersAsync();
				push_history_for_testing("/pop-reload-error-source");
				await vi.runAllTimersAsync();

				pop_back_for_testing();
				await vi.runAllTimersAsync();

				expect(
					console_error_spy.mock.calls.some((args) =>
						args.some(
							(v) =>
								typeof v === "string" &&
								v.includes("Hard redirect fallback failed"),
						),
					),
				).toBe(true);
			} finally {
				set_hard_redirect_handler_for_testing(undefined);
			}
		});

		it("saves outgoing scroll state before cross-document POP commits", async () => {
			const client = await load_client();
			vi.spyOn(window, "fetch").mockImplementation(() =>
				Promise.resolve(create_route_data_response()),
			);

			push_history_for_testing("/scroll-pop-target");
			await vi.runAllTimersAsync();
			push_history_for_testing("/scroll-pop-source");
			await vi.runAllTimersAsync();
			const source_key = read_current_history_key_for_testing();
			window.scrollX = 50;
			window.scrollY = 100;

			pop_back_for_testing();
			await vi.runAllTimersAsync();

			expect(read_scroll_state_for_testing()).toContainEqual({
				history_key: source_key,
				x: 50,
				y: 100,
			});
		});
	});

	// ─── Assertion 16: Scroll State Persistence ──────────────

	describe("scroll state persistence", () => {
		it("saves outgoing scroll state before programmatic navigation", async () => {
			const client = await load_client();
			vi.spyOn(window, "fetch").mockImplementation(() =>
				Promise.resolve(create_route_data_response()),
			);

			await client.vormaNavigate("/scroll-source");
			await vi.runAllTimersAsync();

			window.scrollX = 150;
			window.scrollY = 300;

			await client.vormaNavigate("/scroll-target");
			await vi.runAllTimersAsync();

			const entries = read_scroll_state_for_testing();
			expect(entries.some((e) => e.x === 150 && e.y === 300)).toBe(true);
		});

		it("saves outgoing scroll state under the correct history key", async () => {
			const client = await load_client();
			vi.spyOn(window, "fetch").mockImplementation(() =>
				Promise.resolve(create_route_data_response()),
			);

			await client.vormaNavigate("/scroll-key-source");
			await vi.runAllTimersAsync();
			const source_key = read_current_history_key_for_testing();

			window.scrollX = 150;
			window.scrollY = 300;

			await client.vormaNavigate("/scroll-key-target");
			await vi.runAllTimersAsync();

			expect(read_scroll_state_for_testing()).toContainEqual({
				history_key: source_key,
				x: 150,
				y: 300,
			});
		});

		it("ignores malformed session-stored scroll entries and persists fresh state", async () => {
			const client = await load_client();
			write_raw_scroll_state_storage_for_testing(
				JSON.stringify([
					["valid-old", { x: 10, y: 20 }],
					["missing-y", { x: 1 }],
					["bad-x", { x: "10", y: 20 }],
					[123, { x: 1, y: 2 }],
					["extra-shape", { x: 1, y: 2, z: 3 }],
					["nan-shape", { x: null, y: 2 }],
					"not-an-entry",
				]),
			);

			vi.spyOn(window, "fetch").mockImplementation(() =>
				Promise.resolve(create_route_data_response()),
			);

			await client.vormaNavigate("/malformed-scroll-source");
			await vi.runAllTimersAsync();

			window.scrollX = 150;
			window.scrollY = 300;

			await client.vormaNavigate("/malformed-scroll-target");
			await vi.runAllTimersAsync();

			const entries = read_scroll_state_for_testing();
			expect(
				entries.some(
					(e) =>
						e.history_key === "valid-old" &&
						e.x === 10 &&
						e.y === 20,
				),
			).toBe(true);
			for (const entry of entries) {
				expect(typeof entry.history_key).toBe("string");
				expect(Number.isFinite(entry.x)).toBe(true);
				expect(Number.isFinite(entry.y)).toBe(true);
			}
		});

		it("keeps scroll-state storage at 50 entries with oldest-first eviction", async () => {
			const client = await load_client();
			seed_scroll_state_for_testing([]);
			vi.spyOn(window, "fetch").mockImplementation(() =>
				Promise.resolve(create_route_data_response()),
			);

			push_history_for_testing("/scroll-fifo-start");
			await vi.runAllTimersAsync();
			const oldest_key = read_current_history_key_for_testing();

			for (let i = 0; i <= 50; i += 1) {
				window.scrollX = i;
				window.scrollY = i;
				await client.vormaNavigate(`/scroll-fifo-${i}`);
				await vi.runAllTimersAsync();
			}

			const entries = read_scroll_state_for_testing();
			expect(entries).toHaveLength(50);
			expect(entries.some((e) => e.history_key === oldest_key)).toBe(
				false,
			);
			expect(entries.some((e) => e.x === 50 && e.y === 50)).toBe(true);
		});

		it("does not throw when sessionStorage getItem fails for scroll reads", async () => {
			const client = await load_client();
			const original_get = Storage.prototype.getItem;
			vi.spyOn(Storage.prototype, "getItem").mockImplementation(function (
				this: Storage,
				key: string,
			): string | null {
				if (is_scroll_state_storage_key_for_testing(key)) {
					throw new Error("getItem failed");
				}
				return original_get.call(this, key);
			});
			vi.spyOn(window, "fetch").mockResolvedValue(
				create_route_data_response(),
			);

			await expect(
				client.vormaNavigate("/storage-read-failure"),
			).resolves.toEqual({ didNavigate: true });
			await vi.runAllTimersAsync();
		});

		it("does not throw when sessionStorage set/remove fails for scroll persistence", async () => {
			const client = await load_client();
			const original_set = Storage.prototype.setItem;
			const original_remove = Storage.prototype.removeItem;
			vi.spyOn(Storage.prototype, "setItem").mockImplementation(function (
				this: Storage,
				key: string,
				value: string,
			): void {
				if (
					is_scroll_state_storage_key_for_testing(key) ||
					is_page_refresh_scroll_state_storage_key_for_testing(key)
				) {
					throw new Error("setItem failed");
				}
				return original_set.call(this, key, value);
			});
			vi.spyOn(Storage.prototype, "removeItem").mockImplementation(
				function (this: Storage, key: string): void {
					if (
						is_page_refresh_scroll_state_storage_key_for_testing(
							key,
						)
					) {
						throw new Error("removeItem failed");
					}
					return original_remove.call(this, key);
				},
			);
			vi.spyOn(window, "fetch").mockResolvedValue(
				create_route_data_response(),
			);

			await expect(
				client.vormaNavigate("/storage-write-failure"),
			).resolves.toEqual({ didNavigate: true });
			await expect(client.initClient({})).resolves.toBeUndefined();
			await vi.runAllTimersAsync();
		});
	});

	describe("page-refresh scroll restoration", () => {
		it("restores recent page-refresh scroll state during init", async () => {
			const client = await load_client();
			seed_page_refresh_scroll_state_for_testing({
				x: 300,
				y: 600,
				unix: Date.now() - 1000,
				href: window.location.href,
			});
			vi.spyOn(window, "requestAnimationFrame").mockImplementation(
				(cb) => {
					cb(0);
					return 0;
				},
			);

			await client.initClient({});
			await vi.runAllTimersAsync();

			expect(window.scrollTo).toHaveBeenCalledWith(300, 600);
			expect(read_page_refresh_scroll_state_for_testing()).toBeNull();
		});

		it("does not restore page-refresh scroll when stored href differs", async () => {
			const client = await load_client();
			seed_page_refresh_scroll_state_for_testing({
				x: 250,
				y: 500,
				unix: Date.now() - 1000,
				href: `${window.location.origin}/different-page`,
			});

			await client.initClient({});
			await vi.runAllTimersAsync();

			expect(window.scrollTo).not.toHaveBeenCalledWith(250, 500);
			expect(read_page_refresh_scroll_state_for_testing()).toBeNull();
		});

		it("does not restore page-refresh scroll when snapshot is stale", async () => {
			const client = await load_client();
			seed_page_refresh_scroll_state_for_testing({
				x: 250,
				y: 500,
				unix: Date.now() - 6000,
				href: window.location.href,
			});

			await client.initClient({});
			await vi.runAllTimersAsync();

			expect(window.scrollTo).not.toHaveBeenCalledWith(250, 500);
			expect(read_page_refresh_scroll_state_for_testing()).toBeNull();
		});

		it("restores scroll for encoding-equivalent hash URLs", async () => {
			const client = await load_client();
			window.history.replaceState({}, "", "/refresh-hash#%E2%9C%93");
			seed_page_refresh_scroll_state_for_testing({
				x: 42,
				y: 24,
				unix: Date.now() - 1000,
				href: `${window.location.origin}/refresh-hash#✓`,
			});
			vi.spyOn(window, "requestAnimationFrame").mockImplementation(
				(cb) => {
					cb(0);
					return 0;
				},
			);

			await client.initClient({});
			await vi.runAllTimersAsync();

			expect(window.scrollTo).toHaveBeenCalledWith(42, 24);
			expect(read_page_refresh_scroll_state_for_testing()).toBeNull();
		});

		it("drops malformed page-refresh snapshots during init", async () => {
			const client = await load_client();
			seed_page_refresh_scroll_state_for_testing({
				x: 1,
				y: 2,
				unix: Date.now(),
				href: window.location.href,
			});

			// Find the storage key and corrupt it
			let storage_key: string | undefined;
			for (let i = 0; i < window.sessionStorage.length; i += 1) {
				const key = window.sessionStorage.key(i);
				if (
					key &&
					is_page_refresh_scroll_state_storage_key_for_testing(key)
				) {
					storage_key = key;
					break;
				}
			}
			expect(storage_key).toBeDefined();
			window.sessionStorage.setItem(storage_key!, "{ malformed-json");

			await client.initClient({});
			await vi.runAllTimersAsync();

			expect(window.scrollTo).not.toHaveBeenCalled();
			expect(window.sessionStorage.getItem(storage_key!)).toBeNull();
		});

		it("drops parseable non-object and invalid-shape snapshots during init", async () => {
			const client = await load_client();
			seed_page_refresh_scroll_state_for_testing({
				x: 10,
				y: 20,
				unix: Date.now(),
				href: window.location.href,
			});

			let storage_key: string | undefined;
			for (let i = 0; i < window.sessionStorage.length; i += 1) {
				const key = window.sessionStorage.key(i);
				if (
					key &&
					is_page_refresh_scroll_state_storage_key_for_testing(key)
				) {
					storage_key = key;
					break;
				}
			}
			expect(storage_key).toBeDefined();

			// Array instead of object
			window.sessionStorage.setItem(
				storage_key!,
				JSON.stringify(["bad"]),
			);
			await client.initClient({});
			await vi.runAllTimersAsync();
			expect(window.scrollTo).not.toHaveBeenCalled();
			expect(window.sessionStorage.getItem(storage_key!)).toBeNull();

			// Wrong field type
			seed_page_refresh_scroll_state_for_testing({
				x: 10,
				y: 20,
				unix: Date.now(),
				href: window.location.href,
			});
			window.sessionStorage.setItem(
				storage_key!,
				JSON.stringify({
					x: 10,
					y: "20",
					unix: Date.now(),
					href: window.location.href,
				}),
			);
			await client.initClient({});
			await vi.runAllTimersAsync();
			expect(window.scrollTo).not.toHaveBeenCalled();
			expect(window.sessionStorage.getItem(storage_key!)).toBeNull();
		});
	});
});
