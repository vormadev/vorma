// Assertions covered: 4, 22, 23

import { beforeEach, describe, expect, it, vi } from "vitest";
import {
	register_client_loader_for_testing,
	reset_client_runtime_for_testing,
} from "vorma/testing";
import {
	collect_status_snapshots,
	create_deferred,
	create_route_data_response,
	expect_no_loading_gap,
	expect_status_idle,
	load_client,
} from "./setup.ts";

beforeEach(() => {
	reset_client_runtime_for_testing();
});

describe("status and loading", () => {
	// ─── Assertion 4: Simultaneous Status ────────────────────

	describe("simultaneous status", () => {
		it("reports navigating status while navigation is in flight", async () => {
			const client = await load_client();
			const deferred = create_deferred<Response>();
			vi.spyOn(window, "fetch").mockImplementation(
				() => deferred.promise,
			);

			const nav = client.vormaNavigate("/status-navigating");
			await Promise.resolve();

			expect(client.getStatus()).toEqual({
				isNavigating: true,
				isSubmitting: false,
				isRevalidating: false,
			});

			deferred.resolve(create_route_data_response());
			await nav;
			await vi.runAllTimersAsync();

			expect_status_idle(client.getStatus());
		});

		it("does not dispatch duplicate status events for identical status", async () => {
			const client = await load_client();
			const listener = vi.fn();
			const cleanup = client.addStatusListener(listener);
			vi.spyOn(window, "fetch").mockImplementation(
				() => new Promise(() => {}),
			);

			try {
				void client.vormaNavigate("/same-a");
				await vi.advanceTimersByTimeAsync(8);
				const count = listener.mock.calls.length;

				void client.vormaNavigate("/same-b");
				await vi.advanceTimersByTimeAsync(8);

				expect(listener).toHaveBeenCalledTimes(count);
			} finally {
				cleanup();
			}
		});

		it("debounces rapid status transitions into one event", async () => {
			const client = await load_client();
			const listener = vi.fn();
			const cleanup = client.addStatusListener(listener);
			vi.spyOn(window, "fetch").mockImplementation(
				() => new Promise(() => {}),
			);

			try {
				void client.vormaNavigate("/debounce-a");
				void client.vormaNavigate("/debounce-b");

				await vi.advanceTimersByTimeAsync(8);
				expect(listener).toHaveBeenCalledTimes(1);
			} finally {
				cleanup();
			}
		});
	});

	// ─── Assertion 22: Global Loading Indicators ─────────────

	describe("global loading indicators", () => {
		it("starts and stops indicator around navigation", async () => {
			const client = await load_client();
			let is_running = false;
			const start = vi.fn(() => {
				is_running = true;
			});
			const stop = vi.fn(() => {
				is_running = false;
			});
			const cleanup = client.setupGlobalLoadingIndicator({
				start,
				stop,
				isRunning: () => is_running,
				include: ["navigations"],
				startDelayMS: 0,
				stopDelayMS: 0,
			});

			try {
				const deferred = create_deferred<Response>();
				vi.spyOn(window, "fetch").mockImplementation(
					() => deferred.promise,
				);

				const nav = client.vormaNavigate("/indicator-nav");
				await Promise.resolve();
				await vi.advanceTimersByTimeAsync(1);
				expect(start).toHaveBeenCalledTimes(1);

				deferred.resolve(create_route_data_response());
				await nav;
				await vi.runAllTimersAsync();

				expect(stop).toHaveBeenCalledTimes(1);
			} finally {
				cleanup();
			}
		});

		it("does not start indicator for instant navigation with default delay", async () => {
			const client = await load_client();
			let is_running = false;
			const start = vi.fn(() => {
				is_running = true;
			});
			const stop = vi.fn(() => {
				is_running = false;
			});
			const cleanup = client.setupGlobalLoadingIndicator({
				start,
				stop,
				isRunning: () => is_running,
				include: ["navigations"],
			});

			try {
				vi.spyOn(window, "fetch").mockResolvedValue(
					create_route_data_response(),
				);

				await client.vormaNavigate("/indicator-instant");
				await vi.runAllTimersAsync();

				expect(start).not.toHaveBeenCalled();
				expect(stop).not.toHaveBeenCalled();
				expect(is_running).toBe(false);
			} finally {
				cleanup();
			}
		});

		it("clears pending start timer when work finishes before delay elapses", async () => {
			const client = await load_client();
			let is_running = false;
			const start = vi.fn(() => {
				is_running = true;
			});
			const stop = vi.fn(() => {
				is_running = false;
			});
			const cleanup = client.setupGlobalLoadingIndicator({
				start,
				stop,
				isRunning: () => is_running,
				include: ["navigations"],
				startDelayMS: 100,
				stopDelayMS: 0,
			});

			try {
				const deferred = create_deferred<Response>();
				vi.spyOn(window, "fetch").mockImplementation(
					() => deferred.promise,
				);

				const nav = client.vormaNavigate("/indicator-cancel-start");
				await vi.advanceTimersByTimeAsync(8);

				deferred.resolve(create_route_data_response());
				await nav;
				await vi.runAllTimersAsync();

				expect(start).not.toHaveBeenCalled();
				expect(stop).not.toHaveBeenCalled();
				expect(is_running).toBe(false);
			} finally {
				cleanup();
			}
		});

		it("clears indicator timers when timer id is zero", async () => {
			const client = await load_client();
			const original_set_timeout = globalThis.setTimeout.bind(globalThis);
			const set_timeout_spy = vi
				.spyOn(window, "setTimeout")
				.mockImplementationOnce(
					() =>
						0 as unknown as ReturnType<
							typeof globalThis.setTimeout
						>,
				)
				.mockImplementation((handler, timeout, ...args) =>
					original_set_timeout(handler, timeout, ...args),
				);
			const clear_timeout_spy = vi.spyOn(window, "clearTimeout");
			const cleanup = client.setupGlobalLoadingIndicator({
				start: vi.fn(),
				stop: vi.fn(),
				isRunning: () => false,
				include: ["navigations"],
				startDelayMS: 5,
				stopDelayMS: 5,
			});
			const deferred = create_deferred<Response>();
			vi.spyOn(window, "fetch").mockImplementation(
				() => deferred.promise,
			);
			const nav = client.vormaNavigate("/indicator-zero-timer");

			try {
				await Promise.resolve();
				cleanup();
				expect(set_timeout_spy).toHaveBeenCalled();
				expect(clear_timeout_spy).toHaveBeenCalledWith(0);
				deferred.resolve(create_route_data_response());
				await nav;
				await vi.runAllTimersAsync();
			} finally {
				set_timeout_spy.mockRestore();
			}
		});

		it("stops and detaches behavior after cleanup", async () => {
			const client = await load_client();
			let is_running = false;
			const start = vi.fn(() => {
				is_running = true;
			});
			const stop = vi.fn(() => {
				is_running = false;
			});
			const deferred = create_deferred<Response>();
			vi.spyOn(window, "fetch").mockImplementation(
				() => deferred.promise,
			);
			const cleanup = client.setupGlobalLoadingIndicator({
				start,
				stop,
				isRunning: () => is_running,
				include: ["navigations"],
				startDelayMS: 0,
				stopDelayMS: 0,
			});

			const nav = client.vormaNavigate("/indicator-cleanup");
			await vi.advanceTimersByTimeAsync(8);
			await vi.runAllTimersAsync();
			expect(start).toHaveBeenCalledTimes(1);
			expect(is_running).toBe(true);

			cleanup();
			expect(stop).toHaveBeenCalledTimes(1);
			expect(is_running).toBe(false);

			deferred.resolve(create_route_data_response());
			await nav;
			await vi.runAllTimersAsync();

			expect(start).toHaveBeenCalledTimes(1);
			expect(stop).toHaveBeenCalledTimes(1);
		});

		it("cancels pending stop timer when new work begins", async () => {
			const client = await load_client();
			let is_running = false;
			const start = vi.fn(() => {
				is_running = true;
			});
			const stop = vi.fn(() => {
				is_running = false;
			});
			const first_deferred = create_deferred<Response>();
			const second_deferred = create_deferred<Response>();
			vi.spyOn(window, "fetch")
				.mockImplementationOnce(() => first_deferred.promise)
				.mockImplementationOnce(() => second_deferred.promise);
			const cleanup = client.setupGlobalLoadingIndicator({
				start,
				stop,
				isRunning: () => is_running,
				include: ["navigations"],
				startDelayMS: 0,
				stopDelayMS: 100,
			});

			try {
				const first = client.vormaNavigate("/stop-cancel-first");
				await vi.advanceTimersByTimeAsync(8);
				await vi.runAllTimersAsync();
				expect(start).toHaveBeenCalledTimes(1);
				expect(is_running).toBe(true);

				first_deferred.resolve(create_route_data_response());
				await first;
				await vi.advanceTimersByTimeAsync(8);

				const second = client.vormaNavigate("/stop-cancel-second");
				await vi.advanceTimersByTimeAsync(8);
				await vi.advanceTimersByTimeAsync(100);

				expect(stop).not.toHaveBeenCalled();
				expect(start).toHaveBeenCalledTimes(1);
				expect(is_running).toBe(true);

				second_deferred.resolve(create_route_data_response());
				await second;
				await vi.runAllTimersAsync();

				expect(stop).toHaveBeenCalledTimes(1);
				expect(is_running).toBe(false);
			} finally {
				cleanup();
			}
		});

		it("avoids start-stop thrash during overlapping work", async () => {
			const client = await load_client();
			let is_running = false;
			const start = vi.fn(() => {
				is_running = true;
			});
			const stop = vi.fn(() => {
				is_running = false;
			});
			const first_deferred = create_deferred<Response>();
			const second_deferred = create_deferred<Response>();
			vi.spyOn(window, "fetch")
				.mockImplementationOnce(() => first_deferred.promise)
				.mockImplementationOnce(() => second_deferred.promise);
			const cleanup = client.setupGlobalLoadingIndicator({
				start,
				stop,
				isRunning: () => is_running,
				include: ["navigations"],
				startDelayMS: 5,
				stopDelayMS: 5,
			});

			try {
				const first = client.vormaNavigate("/overlap-first");
				await vi.advanceTimersByTimeAsync(13);
				expect(start).toHaveBeenCalledTimes(1);
				expect(is_running).toBe(true);

				const second = client.vormaNavigate("/overlap-second");

				first_deferred.resolve(create_route_data_response());
				await first;
				await vi.advanceTimersByTimeAsync(20);

				expect(stop).not.toHaveBeenCalled();
				expect(start).toHaveBeenCalledTimes(1);
				expect(is_running).toBe(true);

				second_deferred.resolve(create_route_data_response());
				await second;
				await vi.runAllTimersAsync();

				expect(start).toHaveBeenCalledTimes(1);
				expect(stop).toHaveBeenCalledTimes(1);
				expect(is_running).toBe(false);
			} finally {
				cleanup();
			}
		});

		it("keeps simultaneous registrations isolated", async () => {
			const client = await load_client();
			let first_running = false;
			let second_running = false;
			const first_start = vi.fn(() => {
				first_running = true;
			});
			const first_stop = vi.fn(() => {
				first_running = false;
			});
			const second_start = vi.fn(() => {
				second_running = true;
			});
			const second_stop = vi.fn(() => {
				second_running = false;
			});
			const nav_deferred = create_deferred<Response>();
			vi.spyOn(window, "fetch").mockImplementation(
				() => nav_deferred.promise,
			);

			const cleanup_first = client.setupGlobalLoadingIndicator({
				start: first_start,
				stop: first_stop,
				isRunning: () => first_running,
				include: ["navigations"],
				startDelayMS: 0,
				stopDelayMS: 0,
			});
			const cleanup_second = client.setupGlobalLoadingIndicator({
				start: second_start,
				stop: second_stop,
				isRunning: () => second_running,
				include: ["navigations"],
				startDelayMS: 50,
				stopDelayMS: 0,
			});

			try {
				const nav = client.vormaNavigate("/two-instances");
				await vi.advanceTimersByTimeAsync(8);
				await vi.advanceTimersByTimeAsync(1);

				expect(first_start).toHaveBeenCalledTimes(1);
				expect(second_start).not.toHaveBeenCalled();
				expect(first_running).toBe(true);
				expect(second_running).toBe(false);

				cleanup_first();
				expect(first_stop).toHaveBeenCalledTimes(1);
				expect(first_running).toBe(false);

				await vi.advanceTimersByTimeAsync(50);
				expect(second_start).toHaveBeenCalledTimes(1);
				expect(second_running).toBe(true);

				nav_deferred.resolve(create_route_data_response());
				await nav;
				await vi.runAllTimersAsync();

				expect(first_start).toHaveBeenCalledTimes(1);
				expect(first_stop).toHaveBeenCalledTimes(1);
				expect(second_start).toHaveBeenCalledTimes(1);
				expect(second_stop).toHaveBeenCalledTimes(1);
				expect(second_running).toBe(false);
			} finally {
				cleanup_second();
			}
		});
	});

	describe("inclusion filters", () => {
		it("respects navigation-only filter", async () => {
			const client = await load_client();
			let is_running = false;
			const start = vi.fn(() => {
				is_running = true;
			});
			const stop = vi.fn(() => {
				is_running = false;
			});
			const cleanup = client.setupGlobalLoadingIndicator({
				start,
				stop,
				isRunning: () => is_running,
				include: ["navigations"],
				startDelayMS: 0,
				stopDelayMS: 0,
			});

			try {
				const nav_deferred = create_deferred<Response>();
				vi.spyOn(window, "fetch")
					.mockImplementationOnce(() => nav_deferred.promise)
					.mockImplementationOnce(() =>
						Promise.resolve(
							new Response(JSON.stringify({ ok: true }), {
								status: 200,
								headers: {
									"Content-Type": "application/json",
								},
							}),
						),
					);

				const nav = client.vormaNavigate("/indicator-nav-only");
				await Promise.resolve();
				await vi.advanceTimersByTimeAsync(1);
				expect(start).toHaveBeenCalledTimes(1);

				nav_deferred.resolve(create_route_data_response());
				await nav;
				await vi.runAllTimersAsync();
				expect(stop).toHaveBeenCalledTimes(1);

				await client.submit(
					"/api/ignored",
					{ method: "POST" },
					{ revalidate: false },
				);
				await vi.runAllTimersAsync();
				expect(start).toHaveBeenCalledTimes(1);
			} finally {
				cleanup();
			}
		});

		it("respects revalidation-only filter", async () => {
			const client = await load_client();
			let is_running = false;
			const start = vi.fn(() => {
				is_running = true;
			});
			const stop = vi.fn(() => {
				is_running = false;
			});
			const cleanup = client.setupGlobalLoadingIndicator({
				start,
				stop,
				isRunning: () => is_running,
				include: ["revalidations"],
				startDelayMS: 0,
				stopDelayMS: 0,
			});

			try {
				const rev_deferred = create_deferred<Response>();
				vi.spyOn(window, "fetch")
					.mockImplementationOnce(() => rev_deferred.promise)
					.mockImplementationOnce(() =>
						Promise.resolve(create_route_data_response()),
					);

				const rev = client.revalidate();
				await Promise.resolve();
				await vi.advanceTimersByTimeAsync(1);
				expect(start).toHaveBeenCalledTimes(1);

				rev_deferred.resolve(create_route_data_response());
				await rev;
				await vi.runAllTimersAsync();
				expect(stop).toHaveBeenCalledTimes(1);

				await client.vormaNavigate("/indicator-nav-ignored");
				await vi.runAllTimersAsync();
				expect(start).toHaveBeenCalledTimes(1);
			} finally {
				cleanup();
			}
		});

		it("respects submissions-only filter", async () => {
			const client = await load_client();
			let is_running = false;
			const start = vi.fn(() => {
				is_running = true;
			});
			const stop = vi.fn(() => {
				is_running = false;
			});
			const cleanup = client.setupGlobalLoadingIndicator({
				start,
				stop,
				isRunning: () => is_running,
				include: ["submissions"],
				startDelayMS: 0,
				stopDelayMS: 0,
			});

			try {
				const sub_deferred = create_deferred<Response>();
				vi.spyOn(window, "fetch")
					.mockImplementationOnce(() => sub_deferred.promise)
					.mockResolvedValueOnce(create_route_data_response());

				const sub = client.submit(
					"/api/submit-only",
					{ method: "POST" },
					{ revalidate: false },
				);
				await Promise.resolve();
				await vi.advanceTimersByTimeAsync(1);
				expect(start).toHaveBeenCalledTimes(1);

				sub_deferred.resolve(
					new Response(JSON.stringify({ ok: true }), {
						status: 200,
						headers: { "Content-Type": "application/json" },
					}),
				);
				await sub;
				await vi.runAllTimersAsync();
				expect(stop).toHaveBeenCalledTimes(1);

				await client.vormaNavigate("/indicator-nav-ignored");
				await vi.runAllTimersAsync();
				expect(start).toHaveBeenCalledTimes(1);
			} finally {
				cleanup();
			}
		});
	});

	describe("per-operation opt-outs", () => {
		it("skips indicator when navigate opts out", async () => {
			const client = await load_client();
			let is_running = false;
			const start = vi.fn(() => {
				is_running = true;
			});
			const stop = vi.fn(() => {
				is_running = false;
			});
			const cleanup = client.setupGlobalLoadingIndicator({
				start,
				stop,
				isRunning: () => is_running,
				include: "all",
				startDelayMS: 0,
				stopDelayMS: 0,
			});

			try {
				const deferred = create_deferred<Response>();
				vi.spyOn(window, "fetch").mockImplementation(
					() => deferred.promise,
				);

				const nav = client.vormaNavigate("/indicator-skipped", {
					skipGlobalLoadingIndicator: true,
				});
				await Promise.resolve();
				await vi.advanceTimersByTimeAsync(1);

				expect(start).not.toHaveBeenCalled();

				deferred.resolve(create_route_data_response());
				await nav;
				await vi.runAllTimersAsync();

				expect(start).not.toHaveBeenCalled();
				expect(stop).not.toHaveBeenCalled();
			} finally {
				cleanup();
			}
		});

		it("skips indicator when submit opts out", async () => {
			const client = await load_client();
			let is_running = false;
			const start = vi.fn(() => {
				is_running = true;
			});
			const stop = vi.fn(() => {
				is_running = false;
			});
			const cleanup = client.setupGlobalLoadingIndicator({
				start,
				stop,
				isRunning: () => is_running,
				include: "all",
				startDelayMS: 0,
				stopDelayMS: 0,
			});

			try {
				const deferred = create_deferred<Response>();
				vi.spyOn(window, "fetch").mockImplementation(
					() => deferred.promise,
				);

				const sub = client.submit(
					"/api/indicator-submit-skipped",
					{ method: "POST" },
					{
						skipGlobalLoadingIndicator: true,
						revalidate: false,
					},
				);
				await Promise.resolve();
				await vi.advanceTimersByTimeAsync(1);

				expect(start).not.toHaveBeenCalled();

				deferred.resolve(
					new Response(JSON.stringify({ ok: true }), {
						status: 200,
						headers: { "Content-Type": "application/json" },
					}),
				);
				await sub;
				await vi.runAllTimersAsync();

				expect(start).not.toHaveBeenCalled();
				expect(stop).not.toHaveBeenCalled();
			} finally {
				cleanup();
			}
		});
	});

	// ─── Assertion 23: Full Render Pipeline ──────────────────

	describe("full render pipeline", () => {
		it("keeps loading continuous through complete navigation render cycle", async () => {
			const client = await load_client();
			const { statuses, cleanup } = collect_status_snapshots(client);
			let route_change_count = 0;
			const remove_listener = client.addRouteChangeListener(() => {
				route_change_count += 1;
			});
			vi.spyOn(window, "fetch").mockResolvedValue(
				create_route_data_response({
					matched_patterns: ["/complex-page"],
					title: { dangerousInnerHTML: "Complex Page" },
				}),
			);

			try {
				expect(client.getStatus().isNavigating).toBe(false);
				const nav = client.vormaNavigate("/complex-page");
				expect(client.getStatus().isNavigating).toBe(true);

				await nav;
				await vi.runAllTimersAsync();

				expect(route_change_count).toBe(1);
				expect_no_loading_gap(statuses);
				expect_status_idle(client.getStatus());
			} finally {
				cleanup();
				remove_listener();
			}
		});

		it("keeps navigating while waiting for css preload completion", async () => {
			const client = await load_client();
			const { statuses, cleanup } = collect_status_snapshots(client);

			try {
				const fetch_deferred = create_deferred<Response>();
				vi.spyOn(window, "fetch").mockImplementation(
					() => fetch_deferred.promise,
				);

				const nav = client.vormaNavigate("/styled-page");
				fetch_deferred.resolve(
					create_route_data_response({
						css_bundles: ["/style1.css"],
					}),
				);

				await vi.advanceTimersByTimeAsync(10);
				expect(client.getStatus().isNavigating).toBe(true);

				const preload_link = Array.from(
					document.head.querySelectorAll<HTMLLinkElement>(
						'link[rel="preload"][as="style"][data-vorma-css-preload-bundle]',
					),
				).find(
					(node) =>
						node.getAttribute("data-vorma-css-preload-bundle") ===
						"/style1.css",
				);
				expect(preload_link).toBeDefined();

				preload_link?.dispatchEvent(new Event("load"));
				await nav;
				await vi.runAllTimersAsync();

				expect_no_loading_gap(statuses);
				expect(client.getStatus().isNavigating).toBe(false);
			} finally {
				cleanup();
			}
		});

		it("continues navigation when css preload settles via error", async () => {
			const client = await load_client();
			const { statuses, cleanup } = collect_status_snapshots(client);

			try {
				const fetch_deferred = create_deferred<Response>();
				vi.spyOn(window, "fetch").mockImplementation(
					() => fetch_deferred.promise,
				);

				const nav = client.vormaNavigate("/styled-page-error");
				fetch_deferred.resolve(
					create_route_data_response({
						css_bundles: ["/style-error.css"],
						title: {
							dangerousInnerHTML: "Styled Error Settle",
						},
					}),
				);

				await vi.advanceTimersByTimeAsync(10);
				expect(client.getStatus().isNavigating).toBe(true);

				const preload_link = Array.from(
					document.head.querySelectorAll<HTMLLinkElement>(
						'link[rel="preload"][as="style"][data-vorma-css-preload-bundle]',
					),
				).find(
					(node) =>
						node.getAttribute("data-vorma-css-preload-bundle") ===
						"/style-error.css",
				);
				expect(preload_link).toBeDefined();

				preload_link?.dispatchEvent(new Event("error"));
				await nav;
				await vi.runAllTimersAsync();

				expect_no_loading_gap(statuses);
				expect(client.getStatus().isNavigating).toBe(false);
				expect(document.title).toBe("Styled Error Settle");
			} finally {
				cleanup();
			}
		});

		it("deduplicates stylesheet application for repeated css bundles", async () => {
			const client = await load_client();
			const append_child = document.head.appendChild.bind(document.head);
			vi.spyOn(document.head, "appendChild").mockImplementation(
				(node) => {
					if (
						node instanceof HTMLLinkElement &&
						node.rel === "preload" &&
						node.getAttribute("as") === "style"
					) {
						void Promise.resolve().then(() =>
							node.dispatchEvent(new Event("load")),
						);
					}
					return append_child(node);
				},
			);

			vi.spyOn(window, "fetch")
				.mockResolvedValueOnce(
					create_route_data_response({
						css_bundles: ["/dup.css", "/dup.css"],
					}),
				)
				.mockResolvedValueOnce(
					create_route_data_response({
						css_bundles: ["/dup.css"],
					}),
				);

			await client.vormaNavigate("/dup-css-a");
			await vi.runAllTimersAsync();
			await client.vormaNavigate("/dup-css-b");
			await vi.runAllTimersAsync();

			const stylesheets = document.querySelectorAll(
				'link[rel="stylesheet"][data-vorma-css-bundle="/dup.css"]',
			);
			expect(stylesheets).toHaveLength(1);
		});

		it("preloads css bundle assets during fetch phase", async () => {
			const client = await load_client();
			const append_child = document.head.appendChild.bind(document.head);
			vi.spyOn(document.head, "appendChild").mockImplementation(
				(node) => {
					if (
						node instanceof HTMLLinkElement &&
						node.rel === "preload" &&
						node.getAttribute("as") === "style"
					) {
						void Promise.resolve().then(() =>
							node.dispatchEvent(new Event("load")),
						);
					}
					return append_child(node);
				},
			);

			vi.spyOn(window, "fetch").mockResolvedValue(
				create_route_data_response({
					css_bundles: ["/a.css", "/b.css"],
				}),
			);

			await client.vormaNavigate("/with-css-bundles");
			await vi.runAllTimersAsync();

			const preload_links = Array.from(
				document.querySelectorAll<HTMLLinkElement>(
					'link[rel="preload"][as="style"]',
				),
			);
			expect(preload_links).toHaveLength(2);
			const hrefs = preload_links.map((l) => l.getAttribute("href"));
			expect(hrefs).toEqual(expect.arrayContaining(["/a.css", "/b.css"]));
		});

		it("preloads unique deps as modulepreload links in production", async () => {
			const original_dev = import.meta.env.DEV;
			(import.meta.env as Record<string, unknown>).DEV = false;

			try {
				const client = await load_client();
				vi.spyOn(window, "fetch").mockResolvedValue(
					create_route_data_response({
						deps: ["/dep-a.js", "/dep-b.js", "/dep-a.js"],
					}),
				);

				await client.vormaNavigate("/with-deps");
				await vi.runAllTimersAsync();

				const module_preloads = Array.from(
					document.querySelectorAll<HTMLLinkElement>(
						'link[rel="modulepreload"]',
					),
				);
				expect(module_preloads).toHaveLength(2);
				const hrefs = module_preloads.map((l) =>
					l.getAttribute("href"),
				);
				expect(hrefs).toEqual(
					expect.arrayContaining(["/dep-a.js", "/dep-b.js"]),
				);
			} finally {
				(import.meta.env as Record<string, unknown>).DEV = original_dev;
			}
		});

		it("keeps navigating while waiting for client loader completion", async () => {
			const client = await load_client();
			const { statuses, cleanup } = collect_status_snapshots(client);
			const loader_deferred = create_deferred<{ loaded: boolean }>();
			vi.doMock("/data-page.js", () => ({
				default: () => null,
			}));
			register_client_loader_for_testing({
				pattern: "/data-page",
				client_loader: () => loader_deferred.promise,
			});

			try {
				const fetch_deferred = create_deferred<Response>();
				vi.spyOn(window, "fetch").mockImplementation(
					() => fetch_deferred.promise,
				);

				const nav = client.vormaNavigate("/data-page");
				fetch_deferred.resolve(
					create_route_data_response({
						matched_patterns: ["/data-page"],
						import_urls: ["/data-page.js"],
						export_keys: ["default"],
						error_export_keys: [""],
					}),
				);

				await vi.advanceTimersByTimeAsync(10);
				expect(client.getStatus().isNavigating).toBe(true);

				loader_deferred.resolve({ loaded: true });
				await nav;
				await vi.runAllTimersAsync();

				expect_no_loading_gap(statuses);
				expect(client.getStatus().isNavigating).toBe(false);
			} finally {
				cleanup();
			}
		});

		it("waits for client loader before dispatching route-change", async () => {
			const client = await load_client();
			const loader_deferred = create_deferred<{ ready: boolean }>();
			vi.doMock("/route-change-loader.js", () => ({
				default: () => null,
			}));
			register_client_loader_for_testing({
				pattern: "/route-change-loader",
				client_loader: () => loader_deferred.promise,
			});
			const route_change_listener = vi.fn();
			const remove_listener = client.addRouteChangeListener(
				route_change_listener,
			);

			try {
				const fetch_deferred = create_deferred<Response>();
				vi.spyOn(window, "fetch").mockImplementation(
					() => fetch_deferred.promise,
				);

				const nav = client.vormaNavigate("/route-change-loader");
				fetch_deferred.resolve(
					create_route_data_response({
						matched_patterns: ["/route-change-loader"],
						import_urls: ["/route-change-loader.js"],
						export_keys: ["default"],
						error_export_keys: [""],
					}),
				);

				await vi.advanceTimersByTimeAsync(10);
				expect(route_change_listener).toHaveBeenCalledTimes(0);

				loader_deferred.resolve({ ready: true });
				await nav;
				await vi.runAllTimersAsync();

				expect(route_change_listener).toHaveBeenCalledTimes(1);
			} finally {
				remove_listener();
			}
		});
	});
});
