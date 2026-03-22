// Assertions covered: 24

import { beforeEach, describe, expect, it, vi } from "vitest";
import {
	register_client_loader_for_testing,
	reset_client_runtime_for_testing,
	seed_runtime_route_snapshot_for_testing,
	simulate_vite_after_update_for_testing,
} from "vorma/testing";
import {
	create_deferred,
	create_route_data_response,
	expect_status_idle,
	load_client,
	wait_for_request_count,
	with_unhandled_rejection_capture,
} from "./setup.ts";

beforeEach(() => {
	reset_client_runtime_for_testing();
});

describe("client loaders", () => {
	// ─── Parallel Start ──────────────────────────────────────

	describe("parallel start", () => {
		it("starts matched client loaders in parallel before either resolves", async () => {
			const client = await load_client();
			const first_deferred = create_deferred<{ ready: boolean }>();
			const second_deferred = create_deferred<{ ready: boolean }>();
			const started_patterns: string[] = [];

			vi.doMock("/parallel-a.js", () => ({
				default: () => null,
			}));
			vi.doMock("/parallel-b.js", () => ({
				default: () => null,
			}));
			register_client_loader_for_testing({
				pattern: "/parallel-a",
				client_loader: async () => {
					started_patterns.push("/parallel-a");
					return first_deferred.promise;
				},
			});
			register_client_loader_for_testing({
				pattern: "/parallel-b",
				client_loader: async () => {
					started_patterns.push("/parallel-b");
					return second_deferred.promise;
				},
			});

			vi.spyOn(window, "fetch").mockResolvedValue(
				create_route_data_response({
					matched_patterns: ["/parallel-a", "/parallel-b"],
					import_urls: ["/parallel-a.js", "/parallel-b.js"],
					export_keys: ["default", "default"],
					error_export_keys: ["", ""],
					loaders_data: [{ a: 1 }, { b: 2 }],
				}),
			);

			const nav = client.vormaNavigate("/parallel-loaders");
			await wait_for_request_count({
				requests: started_patterns,
				count: 2,
			});

			expect(started_patterns).toEqual(
				expect.arrayContaining(["/parallel-a", "/parallel-b"]),
			);

			first_deferred.resolve({ ready: true });
			await vi.advanceTimersByTimeAsync(1);
			expect(client.getStatus().isNavigating).toBe(true);

			second_deferred.resolve({ ready: true });
			await nav;
			await vi.runAllTimersAsync();

			expect_status_idle(client.getStatus());
		});
	});

	// ─── Running-Promise Reuse ───────────────────────────────

	describe("running-promise reuse", () => {
		it("reuses running client-loader promises without duplicate invocations", async () => {
			const client = await load_client();
			const loader_deferred = create_deferred<{ ready: boolean }>();
			let invocation_count = 0;

			vi.doMock("/reuse-loader.js", () => ({
				default: () => null,
			}));
			register_client_loader_for_testing({
				pattern: "/reuse-loader",
				client_loader: async () => {
					invocation_count += 1;
					return loader_deferred.promise;
				},
			});

			const fetch_deferred = create_deferred<Response>();
			vi.spyOn(window, "fetch").mockImplementation(
				() => fetch_deferred.promise,
			);

			const first = client.vormaNavigate("/reuse-loader#one");
			await vi.advanceTimersByTimeAsync(5);

			fetch_deferred.resolve(
				create_route_data_response({
					matched_patterns: ["/reuse-loader"],
					import_urls: ["/reuse-loader.js"],
					export_keys: ["default"],
					error_export_keys: [""],
					loaders_data: [{ value: 1 }],
				}),
			);
			await vi.advanceTimersByTimeAsync(10);
			expect(invocation_count).toBe(1);

			const second = client.vormaNavigate("/reuse-loader#two");
			await vi.advanceTimersByTimeAsync(10);
			expect(invocation_count).toBe(1);

			loader_deferred.resolve({ ready: true });
			await Promise.all([first, second]);
			await vi.runAllTimersAsync();

			expect_status_idle(client.getStatus());
			expect(window.location.pathname).toBe("/reuse-loader");
			expect(window.location.hash).toBe("#two");
		});
	});

	// ─── Matched Server Data ─────────────────────────────────

	describe("matched server data", () => {
		it("passes matched server loader data into client loader functions", async () => {
			const client = await load_client();
			let observed_server_data: unknown = undefined;

			register_client_loader_for_testing({
				pattern: "/matched-data",
				client_loader: async (input: unknown) => {
					const typed = input as {
						serverDataPromise: Promise<unknown>;
					};
					observed_server_data = await typed.serverDataPromise;
					return { from_client: true };
				},
			});

			vi.spyOn(window, "fetch").mockResolvedValue(
				create_route_data_response({
					matched_patterns: ["/matched-data"],
					loaders_data: [{ server: "data" }],
				}),
			);

			await client.vormaNavigate("/matched-data");
			await vi.runAllTimersAsync();

			expect(observed_server_data).toEqual({
				clientBuildID: "1",
				matchedPatterns: ["/matched-data"],
				rootData: null,
				loaderData: { server: "data" },
			});
			expect_status_idle(client.getStatus());
		});
	});

	// ─── Route-Change Blocking ───────────────────────────────

	describe("route-change blocking", () => {
		it("blocks route-change until all matched client loaders settle", async () => {
			const client = await load_client();
			const loader_deferred = create_deferred<{ ready: boolean }>();
			vi.doMock("/blocking-loader.js", () => ({
				default: () => null,
			}));
			register_client_loader_for_testing({
				pattern: "/blocking-loader",
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

				const nav = client.vormaNavigate("/blocking-loader");
				fetch_deferred.resolve(
					create_route_data_response({
						matched_patterns: ["/blocking-loader"],
						import_urls: ["/blocking-loader.js"],
						export_keys: ["default"],
						error_export_keys: [""],
						loaders_data: [{}],
					}),
				);

				await vi.advanceTimersByTimeAsync(10);
				expect(route_change_listener).toHaveBeenCalledTimes(0);
				expect(client.getStatus().isNavigating).toBe(true);

				loader_deferred.resolve({ ready: true });
				await nav;
				await vi.runAllTimersAsync();

				expect(route_change_listener).toHaveBeenCalledTimes(1);
				expect_status_idle(client.getStatus());
			} finally {
				remove_listener();
			}
		});
	});

	// ─── Client-Loader Failure Containment ───────────────────

	describe("failure containment", () => {
		it("contains client-loader failures without leaking unhandled rejections", async () => {
			const client = await load_client();
			register_client_loader_for_testing({
				pattern: "/loader-failure",
				client_loader: async () => {
					throw new Error("Client loader failed");
				},
			});

			vi.spyOn(window, "fetch").mockResolvedValue(
				create_route_data_response({
					matched_patterns: ["/loader-failure"],
					loaders_data: [{ value: "server" }],
				}),
			);

			const { unhandled_rejections } =
				await with_unhandled_rejection_capture({
					run: async () => {
						await client.vormaNavigate("/loader-failure");
						await vi.runAllTimersAsync();
					},
				});

			expect(unhandled_rejections).toEqual([]);
			expect_status_idle(client.getStatus());
		});

		it("ignores stale client-loader failures after newer navigation wins", async () => {
			const client = await load_client();
			register_client_loader_for_testing({
				pattern: "/stale-loader-failure",
				client_loader: async () => {
					throw new Error("Stale client-loader failure");
				},
			});

			const stale_deferred = create_deferred<Response>();
			const fresh_deferred = create_deferred<Response>();
			let fetch_count = 0;
			vi.spyOn(window, "fetch").mockImplementation(() => {
				fetch_count += 1;
				if (fetch_count === 1) {
					return stale_deferred.promise;
				}
				return fresh_deferred.promise;
			});

			const { unhandled_rejections } =
				await with_unhandled_rejection_capture({
					run: async () => {
						const stale = client.vormaNavigate(
							"/stale-loader-failure",
						);
						await Promise.resolve();
						const fresh = client.vormaNavigate("/fresh-wins");
						await Promise.resolve();

						fresh_deferred.resolve(
							create_route_data_response({
								title: {
									dangerousInnerHTML: "Fresh Wins",
								},
							}),
						);
						await fresh;
						await vi.runAllTimersAsync();

						stale_deferred.resolve(
							create_route_data_response({
								matched_patterns: ["/stale-loader-failure"],
								loaders_data: [{ value: "stale" }],
								title: {
									dangerousInnerHTML:
										"Stale Should Not Apply",
								},
							}),
						);
						await stale;
						await vi.runAllTimersAsync();
					},
				});

			expect(unhandled_rejections).toEqual([]);
			expect(window.location.pathname).toBe("/fresh-wins");
			expect(document.title).toBe("Fresh Wins");
			expect_status_idle(client.getStatus());
		});
	});

	// ─── HMR Rerun Behavior ──────────────────────────────────

	describe("HMR rerun", () => {
		it("reruns only opted-in matched client loaders after JS HMR updates", async () => {
			const client = await load_client();
			const opted_in_loader = vi.fn(async ({ serverDataPromise }) => {
				await serverDataPromise;
				return "opted-in";
			});
			const non_opted_loader = vi.fn(async ({ serverDataPromise }) => {
				await serverDataPromise;
				return "non-opted";
			});

			register_client_loader_for_testing({
				pattern: "/hmr-opted-in",
				client_loader: opted_in_loader,
				re_run_on_module_change: {
					url: "http://localhost:3000/src/routes/hmr-opted-in.tsx?t=1",
				} as ImportMeta,
			});
			register_client_loader_for_testing({
				pattern: "/hmr-non-opted",
				client_loader: non_opted_loader,
			});

			vi.doMock("/hmr-opted-in.js", () => ({
				default: () => null,
			}));
			vi.doMock("/hmr-non-opted.js", () => ({
				default: () => null,
			}));

			seed_runtime_route_snapshot_for_testing({
				matched_patterns: ["/hmr-opted-in", "/hmr-non-opted"],
				loaders_data: [{ id: "a" }, { id: "b" }],
				import_urls: ["/hmr-opted-in.js", "/hmr-non-opted.js"],
				export_keys: ["default", "default"],
				error_export_keys: ["", ""],
			});

			await client.initClient({});
			expect(opted_in_loader).toHaveBeenCalledTimes(1);
			expect(non_opted_loader).toHaveBeenCalledTimes(1);

			await simulate_vite_after_update_for_testing([
				{
					type: "js-update",
					path: "/src/routes/hmr-opted-in.tsx?t=2",
				},
			]);
			await vi.runAllTimersAsync();

			expect(opted_in_loader).toHaveBeenCalledTimes(2);
			expect(non_opted_loader).toHaveBeenCalledTimes(1);
		});

		it("does not rerun opted-in client loaders for CSS-only HMR updates", async () => {
			const client = await load_client();
			const opted_in_loader = vi.fn(async ({ serverDataPromise }) => {
				await serverDataPromise;
				return "opted-in";
			});

			register_client_loader_for_testing({
				pattern: "/hmr-css",
				client_loader: opted_in_loader,
				re_run_on_module_change: {
					url: "http://localhost:3000/src/routes/hmr-css.tsx?t=1",
				} as ImportMeta,
			});

			vi.doMock("/hmr-css.js", () => ({
				default: () => null,
			}));

			seed_runtime_route_snapshot_for_testing({
				matched_patterns: ["/hmr-css"],
				loaders_data: [{ id: "css" }],
				import_urls: ["/hmr-css.js"],
				export_keys: ["default"],
				error_export_keys: [""],
			});

			await client.initClient({});
			expect(opted_in_loader).toHaveBeenCalledTimes(1);

			await simulate_vite_after_update_for_testing([
				{
					type: "css-update",
					path: "/src/routes/hmr-css.tsx?t=2",
				},
			]);
			await vi.runAllTimersAsync();

			expect(opted_in_loader).toHaveBeenCalledTimes(1);
		});

		it("does not trigger server-loader revalidation from HMR updates", async () => {
			const client = await load_client();
			const opted_in_loader = vi.fn(async ({ serverDataPromise }) => {
				await serverDataPromise;
				return "opted-in";
			});

			register_client_loader_for_testing({
				pattern: "/hmr-no-revalidate",
				client_loader: opted_in_loader,
				re_run_on_module_change: {
					url: "http://localhost:3000/src/routes/hmr-no-revalidate.tsx?t=1",
				} as ImportMeta,
			});

			vi.doMock("/hmr-no-revalidate.js", () => ({
				default: () => null,
			}));

			seed_runtime_route_snapshot_for_testing({
				matched_patterns: ["/hmr-no-revalidate"],
				loaders_data: [{ id: "a" }],
				import_urls: ["/hmr-no-revalidate.js"],
				export_keys: ["default"],
				error_export_keys: [""],
			});

			await client.initClient({});
			const fetch_spy = vi.spyOn(window, "fetch");

			await simulate_vite_after_update_for_testing([
				{
					type: "js-update",
					path: "/src/routes/hmr-no-revalidate.tsx?t=2",
				},
			]);
			await vi.runAllTimersAsync();

			expect(fetch_spy).not.toHaveBeenCalled();
			expect(opted_in_loader).toHaveBeenCalledTimes(2);
		});

		it("does not rerun loaders for non-matching HMR JS updates", async () => {
			const client = await load_client();
			const loader = vi.fn(async ({ serverDataPromise }) => {
				await serverDataPromise;
				return "result";
			});

			register_client_loader_for_testing({
				pattern: "/hmr-non-matching",
				client_loader: loader,
				re_run_on_module_change: {
					url: "http://localhost:3000/src/routes/hmr-non-matching.tsx?t=1",
				} as ImportMeta,
			});

			vi.doMock("/hmr-non-matching.js", () => ({
				default: () => null,
			}));

			seed_runtime_route_snapshot_for_testing({
				matched_patterns: ["/hmr-non-matching"],
				loaders_data: [{ id: "a" }],
				import_urls: ["/hmr-non-matching.js"],
				export_keys: ["default"],
				error_export_keys: [""],
			});

			await client.initClient({});
			expect(loader).toHaveBeenCalledTimes(1);
			const fetch_spy = vi.spyOn(window, "fetch");

			await simulate_vite_after_update_for_testing([
				{
					type: "js-update",
					path: "/src/routes/other.tsx?t=2",
				},
			]);
			await vi.runAllTimersAsync();

			expect(loader).toHaveBeenCalledTimes(1);
			expect(fetch_spy).not.toHaveBeenCalled();
		});

		it("ignores malformed HMR update payloads without throwing", async () => {
			const client = await load_client();
			vi.doMock("/hmr-invalid.js", () => ({
				default: () => null,
			}));

			seed_runtime_route_snapshot_for_testing({
				matched_patterns: ["/hmr-invalid"],
				loaders_data: [{ from: "initial" }],
				import_urls: ["/hmr-invalid.js"],
				export_keys: ["default"],
				error_export_keys: [""],
			});

			await client.initClient({});
			const fetch_spy = vi.spyOn(window, "fetch");

			await expect(
				simulate_vite_after_update_for_testing([
					{
						type: "not-supported",
						path: "/src/routes/hmr-invalid.tsx?t=99",
					},
				]),
			).resolves.toBeUndefined();
			await vi.runAllTimersAsync();

			expect(fetch_spy).not.toHaveBeenCalled();
		});
	});

	// ─── Duplicate Registration ──────────────────────────────

	describe("duplicate registration", () => {
		it("keeps the latest registration for duplicate patterns", async () => {
			const client = await load_client();
			const first_loader = vi.fn(async ({ serverDataPromise }) => {
				await serverDataPromise;
				return { source: "first" };
			});
			const second_loader = vi.fn(async ({ serverDataPromise }) => {
				await serverDataPromise;
				return { source: "second" };
			});

			register_client_loader_for_testing({
				pattern: "/dup-pattern",
				client_loader: first_loader,
			});
			register_client_loader_for_testing({
				pattern: "/dup-pattern",
				client_loader: second_loader,
			});

			vi.spyOn(window, "fetch").mockResolvedValue(
				create_route_data_response({
					matched_patterns: ["/dup-pattern"],
					loaders_data: [{ id: "42" }],
					import_urls: ["/dup.js"],
					export_keys: ["default"],
					error_export_keys: [""],
				}),
			);
			vi.doMock("/dup.js", () => ({ default: () => null }));

			await client.vormaNavigate("/dup-pattern/42");
			await vi.runAllTimersAsync();

			expect(first_loader).not.toHaveBeenCalled();
			expect(second_loader).toHaveBeenCalledTimes(1);
		});
	});
});
