// Assertions covered: 25, 26, 44, 45, 46

import { beforeEach, describe, expect, it, vi } from "vitest";
import {
	read_is_touch_input_modality_active_for_testing,
	read_route_manifest_for_testing,
	register_client_loader_for_testing,
	replace_pattern_registry_for_testing,
	reset_client_runtime_for_testing,
	seed_runtime_route_snapshot_for_testing,
	set_deployment_id_for_testing,
	set_route_manifest_for_testing,
} from "vorma/testing";
import {
	create_deferred,
	create_route_data_response,
	expect_status_idle,
	load_client,
	request_input_to_url,
} from "./setup.ts";

beforeEach(() => {
	reset_client_runtime_for_testing();
});

const TEST_APP_CONFIG = {
	actionsRouterMountRoot: "/api/",
	actionsDynamicRune: ":",
	actionsSplatRune: "*",
	loadersDynamicRune: ":",
	loadersSplatRune: "*",
	loadersExplicitIndexSegmentIdentifier: "_index",
};

describe("api surface", () => {
	// ─── Assertion 25: Safe Defaults and Accessors ───────────

	describe("safe defaults", () => {
		it("returns safe default router data before any commit", async () => {
			const client = await load_client();
			expect(client.getRouterData()).toEqual({
				clientBuildID: "1",
				matchedPatterns: [],
				splatValues: [],
				params: {},
				rootData: null,
			});
		});

		it("returns current build ID from getClientBuildID", async () => {
			const client = await load_client();
			expect(client.getClientBuildID()).toBe("1");

			vi.spyOn(window, "fetch").mockResolvedValue(
				create_route_data_response(
					{},
					{
						headers: {
							"X-Vorma-Client-Build-Id": "build-check",
						},
					},
				),
			);

			await client.vormaNavigate("/build-id-check");
			await vi.runAllTimersAsync();

			expect(client.getClientBuildID()).toBe("build-check");
		});

		it("returns synchronous status snapshots from getStatus", async () => {
			const client = await load_client();
			const deferred = create_deferred<Response>();
			vi.spyOn(window, "fetch").mockImplementation(
				() => deferred.promise,
			);

			const nav = client.vormaNavigate("/status-sync");
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
	});

	describe("root element", () => {
		it("returns #vorma-root from getRootEl and throws when missing", async () => {
			const client = await load_client();
			void client.getStatus();
			const root = document.createElement("div");
			root.id = "vorma-root";
			document.body.appendChild(root);

			expect(client.getRootEl()).toBe(root);

			root.remove();
			expect(() => client.getRootEl()).toThrow("vorma-root");
		});

		it("returns non-div root elements", async () => {
			const client = await load_client();
			void client.getStatus();
			const root = document.createElement("main");
			root.id = "vorma-root";
			document.body.appendChild(root);

			expect(client.getRootEl()).toBe(root);
		});
	});

	describe("no internal leaks", () => {
		it("does not expose buildtime-only route registration from runtime client entry", async () => {
			const runtime = await import("vorma/client");
			expect("route" in runtime).toBe(false);
			expect((runtime as Record<string, unknown>).route).toBeUndefined();
		});

		it("does not expose unstable internal __* helpers from runtime client entry", async () => {
			const runtime = await import("vorma/client");
			const record = runtime as Record<string, unknown>;
			expect(record.__registerClientLoaderPattern).toBeUndefined();
			expect(record.__runClientLoadersAfterHMRUpdate).toBeUndefined();
			expect(record.__registerClientLoaderForAdapter).toBeUndefined();
			expect(record.__applyScrollState).toBeUndefined();
			expect(record.__makeFinalLinkProps).toBeUndefined();
			expect(record.__resolvePath).toBeUndefined();
			expect(record.__getClientRuntimeRenderState).toBeUndefined();
			expect(record.__setClientLoaderWaitFn).toBeUndefined();
		});

		it("exposes route registration from the buildtime entry", async () => {
			const buildtime = await import("vorma/buildtime");
			expect(typeof buildtime.route).toBe("function");
			expect(() =>
				buildtime.route(
					"/",
					Promise.resolve({ default: () => null }),
					"default",
				),
			).not.toThrow();
		});
	});

	// ─── Assertion 25: Router Data ───────────────────────────

	describe("router data", () => {
		it("updates getRouterData after successful navigation", async () => {
			const client = await load_client();
			vi.spyOn(window, "fetch").mockResolvedValue(
				create_route_data_response({
					matched_patterns: ["/router-data/:id"],
					has_root_data: true,
					loaders_data: [{ id: "123", label: "root" }],
					params: { id: "123" },
					splat_values: [],
				}),
			);

			await client.vormaNavigate("/router-data/123");
			await vi.runAllTimersAsync();

			expect(client.getRouterData()).toEqual({
				clientBuildID: "1",
				matchedPatterns: ["/router-data/:id"],
				splatValues: [],
				params: { id: "123" },
				rootData: { id: "123", label: "root" },
			});
		});

		it("deterministically overwrites router data on later commits", async () => {
			const client = await load_client();
			vi.spyOn(window, "fetch")
				.mockResolvedValueOnce(
					create_route_data_response({
						matched_patterns: ["/ctx/:id"],
						has_root_data: true,
						loaders_data: [{ id: "A" }],
						params: { id: "A" },
					}),
				)
				.mockResolvedValueOnce(
					create_route_data_response({
						matched_patterns: ["/ctx/:id"],
						has_root_data: true,
						loaders_data: [{ id: "B" }],
						params: { id: "B" },
					}),
				);

			await client.vormaNavigate("/ctx/A");
			await vi.runAllTimersAsync();
			await client.vormaNavigate("/ctx/B");
			await vi.runAllTimersAsync();

			expect(client.getRouterData()).toEqual({
				clientBuildID: "1",
				matchedPatterns: ["/ctx/:id"],
				splatValues: [],
				params: { id: "B" },
				rootData: { id: "B" },
			});
		});
	});

	// ─── Assertion 25: Build ID Lifecycle ────────────────────

	describe("build ID lifecycle", () => {
		it("dispatches build-id updates with old/new values", async () => {
			const client = await load_client();
			const events: {
				oldClientBuildID: string;
				newClientBuildID: string;
			}[] = [];
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

			try {
				vi.spyOn(window, "fetch").mockResolvedValue(
					create_route_data_response(
						{},
						{
							headers: {
								"X-Vorma-Client-Build-Id": "build-2",
							},
						},
					),
				);

				await client.vormaNavigate("/build-id-next");
				await vi.runAllTimersAsync();

				expect(client.getClientBuildID()).toBe("build-2");
				expect(events).toEqual([
					{
						oldClientBuildID: "1",
						newClientBuildID: "build-2",
					},
				]);
			} finally {
				cleanup();
			}
		});

		it("syncs build ID only when value changes", async () => {
			const client = await load_client();
			const events: {
				oldClientBuildID: string;
				newClientBuildID: string;
			}[] = [];
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

			try {
				vi.spyOn(window, "fetch")
					.mockResolvedValueOnce(
						create_route_data_response(
							{},
							{
								headers: {
									"X-Vorma-Client-Build-Id": "1",
								},
							},
						),
					)
					.mockResolvedValueOnce(
						create_route_data_response(
							{},
							{
								headers: {
									"X-Vorma-Client-Build-Id": "build-2",
								},
							},
						),
					);

				await client.vormaNavigate("/build-id-noop");
				await vi.runAllTimersAsync();
				await client.vormaNavigate("/build-id-change");
				await vi.runAllTimersAsync();

				expect(events).toEqual([
					{
						oldClientBuildID: "1",
						newClientBuildID: "build-2",
					},
				]);
				expect(client.getClientBuildID()).toBe("build-2");
			} finally {
				cleanup();
			}
		});

		it("updates getClientBuildID before listeners observe the event", async () => {
			const client = await load_client();
			const observed: {
				current: string;
				event: string;
			}[] = [];
			const cleanup = client.addClientBuildIDListener(
				(
					event: CustomEvent<{
						oldClientBuildID: string;
						newClientBuildID: string;
					}>,
				) => {
					observed.push({
						current: client.getClientBuildID(),
						event: event.detail.newClientBuildID,
					});
				},
			);

			try {
				vi.spyOn(window, "fetch").mockResolvedValue(
					create_route_data_response(
						{},
						{
							headers: {
								"X-Vorma-Client-Build-Id": "build-9",
							},
						},
					),
				);

				await client.vormaNavigate("/build-id-order");
				await vi.runAllTimersAsync();

				expect(observed).toEqual([
					{ current: "build-9", event: "build-9" },
				]);
			} finally {
				cleanup();
			}
		});

		it("uses build-id fallback 1 when response header is missing", async () => {
			const client = await load_client();
			const fetch_spy = vi
				.spyOn(window, "fetch")
				.mockResolvedValueOnce(
					new Response(
						JSON.stringify({
							matchedPatterns: [],
							loadersData: [],
							importURLs: [],
							exportKeys: [],
							errorExportKeys: [],
							hasRootData: false,
							params: {},
							splatValues: [],
							deps: [],
							cssBundles: [],
							metaHeadEls: [],
							restHeadEls: [],
						}),
						{
							status: 200,
							headers: {
								"Content-Type": "application/json",
							},
						},
					),
				)
				.mockResolvedValueOnce(create_route_data_response());

			await client.vormaNavigate("/build-id-missing-header");
			await vi.runAllTimersAsync();
			await client.vormaNavigate("/build-id-follow-up");
			await vi.runAllTimersAsync();

			expect(client.getClientBuildID()).toBe("1");
			expect(fetch_spy).toHaveBeenCalledTimes(2);
		});
	});

	// ─── Assertion 26: Typed Navigate ────────────────────────

	describe("typed navigate", () => {
		it("resolves route params and forwards navigation options", async () => {
			const client = await load_client();
			const fetch_spy = vi
				.spyOn(window, "fetch")
				.mockResolvedValue(create_route_data_response());
			const typed_navigate = client.makeTypedNavigate(
				TEST_APP_CONFIG as any,
			);

			await typed_navigate({
				pattern: "/docs/*",
				splatValues: ["guides", "intro"],
				search: "?mode=full",
				hash: "#overview",
				replace: true,
				scrollToTop: false,
			} as any);
			await vi.runAllTimersAsync();

			const fetch_url = request_input_to_url(
				fetch_spy.mock.calls[0]![0] as RequestInfo | URL,
			);
			expect(fetch_url.pathname).toBe("/docs/guides/intro");
			expect(fetch_url.searchParams.get("mode")).toBe("full");
			expect(fetch_url.searchParams.get("vorma_json")).toBe("1");
			expect(window.location.pathname).toBe("/docs/guides/intro");
			expect(window.location.search).toBe("?mode=full");
			expect(window.location.hash).toBe("#overview");
		});
	});

	describe("compiled path resolution", () => {
		it("resolves loader paths from compiled internal exports", async () => {
			vi.resetModules();
			const internal = await import("vorma/client/__internal");
			const path = internal.resolve_path({
				app_config: TEST_APP_CONFIG as any,
				type: "loader",
				pattern: "/products/:id/_index",
				params: { id: "42" },
			});

			expect(path).toBe("/products/42");
		});
	});

	// ─── Assertion 26: Typed API Client ──────────────────────

	describe("typed API client", () => {
		it("query/mutate apply decorator and preserve submit result contract", async () => {
			const client = await load_client();
			const decorator = vi
				.fn()
				.mockResolvedValueOnce(undefined)
				.mockResolvedValueOnce({
					headers: { "X-Decorator": "1" },
				});
			const typed_client = client.makeTypedAPIClient(
				TEST_APP_CONFIG as any,
				decorator as any,
			);
			const fetch_spy = vi
				.spyOn(window, "fetch")
				.mockResolvedValueOnce(
					new Response(JSON.stringify({ users: [1, 2] }), {
						status: 200,
						headers: { "Content-Type": "application/json" },
					}),
				)
				.mockResolvedValueOnce(
					new Response(JSON.stringify({ ok: true }), {
						status: 200,
						headers: { "Content-Type": "application/json" },
					}),
				);

			const query_result = await typed_client.query({
				pattern: "/users/:id",
				params: { id: "42" },
				input: { include: "posts" },
				options: { revalidate: false },
			} as any);
			const mutate_result = await typed_client.mutate({
				pattern: "/users/:id",
				params: { id: "42" },
				input: { name: "Ada" },
				options: { revalidate: false },
			} as any);
			await vi.runAllTimersAsync();

			expect(query_result).toEqual({
				success: true,
				data: { users: [1, 2] },
			});
			expect(mutate_result).toEqual({
				success: true,
				data: { ok: true },
			});
			expect(decorator).toHaveBeenCalledTimes(2);

			const query_init = fetch_spy.mock.calls[0]![1] as RequestInit;
			expect(query_init.method).toBe("GET");
			const query_url = request_input_to_url(
				fetch_spy.mock.calls[0]![0] as RequestInfo | URL,
			);
			expect(query_url.pathname).toBe("/api/users/42");
			expect(query_url.searchParams.get("include")).toBe("posts");

			const mutate_url = request_input_to_url(
				fetch_spy.mock.calls[1]![0] as RequestInfo | URL,
			);
			expect(mutate_url.pathname).toBe("/api/users/42");
			const mutate_init = fetch_spy.mock.calls[1]![1] as RequestInit;
			expect(mutate_init.method).toBe("POST");
			expect(new Headers(mutate_init.headers).get("X-Decorator")).toBe(
				"1",
			);
		});

		it("merges resolver defaults with per-call requestInit headers", async () => {
			const client = await load_client();
			const decorator = vi.fn().mockResolvedValue({
				credentials: "include",
				headers: {
					Authorization: "Bearer token",
					"X-Defaults": "1",
				},
			});
			const typed_client = client.makeTypedAPIClient(
				TEST_APP_CONFIG as any,
				decorator as any,
			);
			const fetch_spy = vi.spyOn(window, "fetch").mockResolvedValue(
				new Response(JSON.stringify({ ids: ["1"] }), {
					status: 200,
					headers: { "Content-Type": "application/json" },
				}),
			);

			const result = await typed_client.query({
				pattern: "/users/:id",
				params: { id: "42" },
				input: { include: "posts" },
				requestInit: {
					headers: { "X-Trace-ID": "trace-1" },
				},
				options: { revalidate: false },
			} as any);
			await vi.runAllTimersAsync();

			expect(result).toEqual({
				success: true,
				data: { ids: ["1"] },
			});
			const init = fetch_spy.mock.calls[0]![1] as RequestInit;
			const headers = new Headers(init.headers ?? undefined);
			expect(init.method).toBe("GET");
			expect(init.credentials).toBe("include");
			expect(headers.get("Authorization")).toBe("Bearer token");
			expect(headers.get("X-Defaults")).toBe("1");
			expect(headers.get("X-Trace-ID")).toBe("trace-1");
		});

		it("supports mutation calls without request-init decoration", async () => {
			const client = await load_client();
			const typed_client = client.makeTypedAPIClient(
				TEST_APP_CONFIG as any,
			);
			const fetch_spy = vi.spyOn(window, "fetch").mockResolvedValue(
				new Response(JSON.stringify({ ok: true }), {
					status: 200,
					headers: { "Content-Type": "application/json" },
				}),
			);

			const result = await typed_client.mutate({
				pattern: "/users/:id",
				params: { id: "42" },
				input: { username: "alice" },
				requestInit: {
					headers: { "X-Request-ID": "req-1" },
				},
				options: { revalidate: false },
			} as any);
			await vi.runAllTimersAsync();

			expect(result).toEqual({
				success: true,
				data: { ok: true },
			});
			const init = fetch_spy.mock.calls[0]![1] as RequestInit;
			expect(init.method).toBe("POST");
			expect(init.body).toBe(JSON.stringify({ username: "alice" }));
			expect(new Headers(init.headers).get("X-Request-ID")).toBe("req-1");
		});
	});

	// ─── Assertion 44: URL Construction ──────────────────────

	describe("URL construction", () => {
		it("URL-encodes dynamic params in query and mutation URLs", async () => {
			const client = await load_client();
			const typed_client = client.makeTypedAPIClient(
				TEST_APP_CONFIG as any,
			);
			const fetch_spy = vi.spyOn(window, "fetch").mockResolvedValue(
				new Response(JSON.stringify({ ok: true }), {
					status: 200,
					headers: { "Content-Type": "application/json" },
				}),
			);

			await typed_client.query({
				pattern: "/users/:id",
				params: { id: "a/b" },
				input: { tab: "activity", page: 2 },
				options: { revalidate: false },
			} as any);
			await vi.runAllTimersAsync();

			const query_url = request_input_to_url(
				fetch_spy.mock.calls[0]![0] as RequestInfo | URL,
			);
			expect(query_url.pathname).toBe("/api/users/a%2Fb");
			expect(query_url.searchParams.get("tab")).toBe("activity");
			expect(query_url.searchParams.get("page")).toBe("2");
		});

		it("does not encode mutation input into query params", async () => {
			const client = await load_client();
			const typed_client = client.makeTypedAPIClient(
				TEST_APP_CONFIG as any,
			);
			const fetch_spy = vi.spyOn(window, "fetch").mockResolvedValue(
				new Response(JSON.stringify({ ok: true }), {
					status: 200,
					headers: { "Content-Type": "application/json" },
				}),
			);

			await typed_client.mutate({
				pattern: "/users/:id",
				params: { id: "42" },
				input: { should_not_appear: true },
				options: { revalidate: false },
			} as any);
			await vi.runAllTimersAsync();

			const mutate_url = request_input_to_url(
				fetch_spy.mock.calls[0]![0] as RequestInfo | URL,
			);
			expect(mutate_url.pathname).toBe("/api/users/42");
			expect(mutate_url.search).toBe("");
		});
	});

	// ─── Assertion 45: Request Markers ───────────────────────

	describe("request markers", () => {
		it("includes vorma_json marker on navigation route-data requests", async () => {
			const client = await load_client();
			const fetch_spy = vi
				.spyOn(window, "fetch")
				.mockResolvedValue(create_route_data_response());

			await client.vormaNavigate("/marker-check?x=1");
			await vi.runAllTimersAsync();

			const url = request_input_to_url(
				fetch_spy.mock.calls[0]![0] as RequestInfo | URL,
			);
			expect(url.searchParams.get("vorma_json")).toBe("1");
			expect(url.searchParams.get("x")).toBe("1");
		});

		it("includes current build id in navigation route-data requests", async () => {
			const client = await load_client();
			const fetch_spy = vi
				.spyOn(window, "fetch")
				.mockResolvedValueOnce(
					create_route_data_response(
						{},
						{
							headers: {
								"X-Vorma-Client-Build-Id": "build-2",
							},
						},
					),
				)
				.mockResolvedValueOnce(
					create_route_data_response(
						{},
						{
							headers: {
								"X-Vorma-Client-Build-Id": "build-3",
							},
						},
					),
				);

			await client.vormaNavigate("/build-id-first");
			await vi.runAllTimersAsync();
			await client.vormaNavigate("/build-id-second");
			await vi.runAllTimersAsync();

			const first_url = request_input_to_url(
				fetch_spy.mock.calls[0]![0] as RequestInfo | URL,
			);
			const second_url = request_input_to_url(
				fetch_spy.mock.calls[1]![0] as RequestInfo | URL,
			);
			expect(first_url.searchParams.get("vorma_json")).toBe("1");
			expect(second_url.searchParams.get("vorma_json")).toBe("build-2");
		});

		it("includes redirect-accept header on navigation and submit fetches", async () => {
			const client = await load_client();
			const fetch_spy = vi
				.spyOn(window, "fetch")
				.mockResolvedValue(create_route_data_response());

			await client.vormaNavigate("/header-check");
			await vi.runAllTimersAsync();
			await client.submit(
				"/api/header-check",
				{ method: "POST" },
				{ revalidate: false },
			);
			await vi.runAllTimersAsync();

			const nav_headers = new Headers(
				(fetch_spy.mock.calls[0]![1] as RequestInit).headers,
			);
			const submit_headers = new Headers(
				(fetch_spy.mock.calls[1]![1] as RequestInit).headers,
			);
			expect(nav_headers.get("X-Accepts-Client-Redirect")).toBe("1");
			expect(submit_headers.get("X-Accepts-Client-Redirect")).toBe("1");
		});

		it("includes deployment query param on revalidation when configured", async () => {
			const client = await load_client();
			await client.initClient({});
			set_deployment_id_for_testing("deploy-abc");
			window.history.replaceState(
				{},
				"",
				"/revalidate-target?tab=details",
			);
			const fetch_spy = vi
				.spyOn(window, "fetch")
				.mockResolvedValue(create_route_data_response());

			await client.revalidate();
			await vi.runAllTimersAsync();

			const url = request_input_to_url(
				fetch_spy.mock.calls[0]![0] as RequestInfo | URL,
			);
			expect(url.pathname).toBe("/revalidate-target");
			expect(url.searchParams.get("tab")).toBe("details");
			expect(url.searchParams.get("vorma_json")).toBe("1");
			expect(url.searchParams.get("dpl")).toBe("deploy-abc");
		});

		it("includes x-deployment-id on submit requests when configured", async () => {
			const client = await load_client();
			void client.getStatus();
			set_deployment_id_for_testing("deploy-42");
			const fetch_spy = vi
				.spyOn(window, "fetch")
				.mockResolvedValue(create_route_data_response());

			await client.submit(
				"/api/with-deployment",
				{
					method: "POST",
					headers: { "X-Test-Header": "kept" },
					body: JSON.stringify({ ok: true }),
				},
				{ revalidate: false },
			);
			await vi.runAllTimersAsync();

			const headers = new Headers(
				(fetch_spy.mock.calls[0]![1] as RequestInit).headers,
			);
			expect(headers.get("x-deployment-id")).toBe("deploy-42");
			expect(headers.get("X-Test-Header")).toBe("kept");
		});
	});

	// ─── Assertion 46: Same-Origin Acceptance ────────────────

	describe("same-origin acceptance", () => {
		it("allows relative and same-origin absolute targets for navigation and submit", async () => {
			const client = await load_client();
			const fetch_spy = vi
				.spyOn(window, "fetch")
				.mockResolvedValueOnce(create_route_data_response())
				.mockResolvedValueOnce(create_route_data_response())
				.mockResolvedValueOnce(
					new Response(JSON.stringify({ ok: true }), {
						status: 200,
						headers: {
							"Content-Type": "application/json",
						},
					}),
				);

			await client.vormaNavigate("/relative-target");
			await vi.runAllTimersAsync();
			await client.vormaNavigate(
				`${window.location.origin}/absolute-target`,
			);
			await vi.runAllTimersAsync();
			await client.submit(
				`${window.location.origin}/api/absolute-submit`,
				{ method: "POST" },
				{ revalidate: false },
			);
			await vi.runAllTimersAsync();

			expect(fetch_spy).toHaveBeenCalledTimes(3);
			expect(
				request_input_to_url(
					fetch_spy.mock.calls[0]![0] as RequestInfo | URL,
				).pathname,
			).toBe("/relative-target");
			expect(
				request_input_to_url(
					fetch_spy.mock.calls[1]![0] as RequestInfo | URL,
				).pathname,
			).toBe("/absolute-target");
			expect(
				request_input_to_url(
					fetch_spy.mock.calls[2]![0] as RequestInfo | URL,
				).pathname,
			).toBe("/api/absolute-submit");
		});
	});

	// ─── initClient Lifecycle ────────────────────────────────

	describe("initClient lifecycle", () => {
		it("calls render function during init", async () => {
			const client = await load_client();
			const render_fn = vi.fn();

			await client.initClient({ renderFn: render_fn });

			expect(render_fn).toHaveBeenCalledTimes(1);
		});

		it("loads initial components from current importURLs during init", async () => {
			const client = await load_client();
			let did_load = false;
			vi.doMock("/initial.js", () => {
				did_load = true;
				return { default: () => null };
			});
			seed_runtime_route_snapshot_for_testing({
				matched_patterns: ["/"],
				loaders_data: [{ initial: "data" }],
				import_urls: ["/initial.js"],
				export_keys: ["default"],
				error_export_keys: [""],
				has_root_data: true,
			});

			await client.initClient({});

			expect(did_load).toBe(true);
		});

		it("runs initial client loaders during init with current loader data", async () => {
			const client = await load_client();
			vi.doMock("/initial-loader.js", () => ({
				default: () => null,
			}));
			const observed_server_data: unknown[] = [];
			register_client_loader_for_testing({
				pattern: "/",
				client_loader: async (input: unknown) => {
					const typed = input as {
						serverDataPromise: Promise<unknown>;
					};
					observed_server_data.push(await typed.serverDataPromise);
					return { initialized: true };
				},
			});
			seed_runtime_route_snapshot_for_testing({
				matched_patterns: ["/"],
				loaders_data: [{ initial: "data" }],
				import_urls: ["/initial-loader.js"],
				export_keys: ["default"],
				error_export_keys: [""],
				has_root_data: true,
			});

			await client.initClient({});

			expect(observed_server_data).toEqual([
				{
					matchedPatterns: ["/"],
					rootData: { initial: "data" },
					loaderData: { initial: "data" },
					clientBuildID: "1",
				},
			]);
		});

		it("cleans vorma_reload from URL during init", async () => {
			const client = await load_client();
			window.history.replaceState(
				{},
				"",
				"/init-clean?vorma_reload=abc123&foo=bar#frag",
			);

			await client.initClient({});
			await vi.runAllTimersAsync();

			const url = new URL(window.location.href);
			expect(url.searchParams.has("vorma_reload")).toBe(false);
			expect(url.searchParams.get("foo")).toBe("bar");
			expect(url.pathname).toBe("/init-clean");
			expect(url.hash).toBe("#frag");
		});

		it("exposes a dev revalidate handle on window during init", async () => {
			const client = await load_client();
			delete (window as unknown as Record<string, unknown>)
				.__waveRevalidate;

			await client.initClient({ renderFn: () => {} });

			expect(
				(window as unknown as Record<string, unknown>).__waveRevalidate,
			).toBe(client.revalidate);
		});

		it("applies useViewTransitions option on every init call", async () => {
			const client = await load_client();
			const start_spy = vi.fn((run_transition: () => void) => {
				run_transition();
				return { finished: Promise.resolve() };
			});
			Object.defineProperty(document, "startViewTransition", {
				configurable: true,
				value: start_spy,
			});
			vi.spyOn(window, "fetch").mockImplementation(() =>
				Promise.resolve(create_route_data_response()),
			);

			await client.initClient({ useViewTransitions: true });
			await client.vormaNavigate("/view-transition-on");
			await vi.runAllTimersAsync();
			expect(start_spy).toHaveBeenCalledTimes(1);

			await client.initClient({ useViewTransitions: false });
			await client.vormaNavigate("/view-transition-off");
			await vi.runAllTimersAsync();
			expect(start_spy).toHaveBeenCalledTimes(1);
		});

		it("skips view transitions for prefetch and revalidation", async () => {
			const client = await load_client();
			const start_spy = vi.fn((run_transition: () => void) => {
				run_transition();
				return { finished: Promise.resolve() };
			});
			Object.defineProperty(document, "startViewTransition", {
				configurable: true,
				value: start_spy,
			});
			vi.spyOn(window, "fetch").mockImplementation(() =>
				Promise.resolve(create_route_data_response()),
			);

			await client.initClient({ useViewTransitions: true });

			await client.vormaNavigate("/view-transition-user-nav");
			await vi.runAllTimersAsync();
			expect(start_spy).toHaveBeenCalledTimes(1);

			await client.vormaNavigate("/view-transition-prefetch", {
				intent: "prefetch",
			} as any);
			await vi.runAllTimersAsync();
			expect(start_spy).toHaveBeenCalledTimes(1);

			await client.revalidate();
			await vi.runAllTimersAsync();
			expect(start_spy).toHaveBeenCalledTimes(1);
		});

		it("switches pointer modality between touch and fine pointers", async () => {
			const client = await load_client();
			await client.initClient({});

			expect(read_is_touch_input_modality_active_for_testing()).toBe(
				false,
			);

			window.dispatchEvent(new Event("touchstart"));
			expect(read_is_touch_input_modality_active_for_testing()).toBe(
				true,
			);

			const mouse_move = new Event("pointermove");
			Object.defineProperty(mouse_move, "pointerType", {
				value: "mouse",
			});
			window.dispatchEvent(mouse_move);
			expect(read_is_touch_input_modality_active_for_testing()).toBe(
				false,
			);

			const touch_down = new Event("pointerdown");
			Object.defineProperty(touch_down, "pointerType", {
				value: "touch",
			});
			window.dispatchEvent(touch_down);
			expect(read_is_touch_input_modality_active_for_testing()).toBe(
				true,
			);
		});

		it("accepts null outermostServerErrorIdx during init bootstrap", async () => {
			const client = await load_client();
			vi.doMock("/null-error-idx.js", () => ({
				default: () => null,
			}));
			seed_runtime_route_snapshot_for_testing({
				matched_patterns: ["/"],
				loaders_data: [{ initial: "data" }],
				import_urls: ["/null-error-idx.js"],
				export_keys: ["default"],
				error_export_keys: [""],
				has_root_data: true,
				outermost_server_error_idx: null,
			});

			await expect(client.initClient({})).resolves.toBeUndefined();
		});
	});

	// ─── Route Manifest ──────────────────────────────────────

	describe("route manifest", () => {
		it("uses precompiled route manifest during init without progressive fetch", async () => {
			const client = await load_client();
			set_route_manifest_for_testing({ "/precompiled/:id": 1 });
			const fetch_spy = vi.spyOn(window, "fetch").mockResolvedValue(
				new Response("unused", {
					status: 200,
					headers: {
						"Content-Type": "application/json",
					},
				}),
			);

			await client.initClient({
				routeManifestURL: "http://localhost:3000/manifest.json",
			});

			expect(fetch_spy).not.toHaveBeenCalled();
			expect(read_route_manifest_for_testing()).toEqual({
				"/precompiled/:id": 1,
			});
		});

		it("falls back to progressive manifest loading when precompiled is absent", async () => {
			const client = await load_client();
			set_route_manifest_for_testing(undefined);
			const manifest = { "/progressive/:id": 1 };
			const fetch_spy = vi.spyOn(window, "fetch").mockResolvedValue(
				new Response(JSON.stringify(manifest), {
					status: 200,
					headers: {
						"Content-Type": "application/json",
					},
				}),
			);

			await client.initClient({
				routeManifestURL: "http://localhost:3000/manifest.json",
			});

			expect(fetch_spy).toHaveBeenCalledWith(
				"http://localhost:3000/manifest.json",
				{ method: "GET" },
			);
			expect(read_route_manifest_for_testing()).toEqual(manifest);
		});

		it("treats progressive loading failures as non-fatal", async () => {
			const client = await load_client();
			set_route_manifest_for_testing(undefined);
			const fetch_error = new Error("manifest network failed");
			vi.spyOn(window, "fetch").mockRejectedValue(fetch_error);
			const warn_spy = vi.spyOn(console, "warn");

			await expect(
				client.initClient({
					routeManifestURL: "http://localhost:3000/manifest.json",
				}),
			).resolves.toBeUndefined();

			expect(read_route_manifest_for_testing()).toBeUndefined();
			expect(warn_spy).toHaveBeenCalledWith(
				"Failed to load route manifest:",
				fetch_error,
			);
		});

		it("treats non-OK manifest responses as non-fatal", async () => {
			const client = await load_client();
			set_route_manifest_for_testing(undefined);
			vi.spyOn(window, "fetch").mockResolvedValue(
				new Response("manifest unavailable", {
					status: 503,
					headers: {
						"Content-Type": "text/plain",
					},
				}),
			);
			const warn_spy = vi.spyOn(console, "warn");

			await expect(
				client.initClient({
					routeManifestURL: "http://localhost:3000/manifest.json",
				}),
			).resolves.toBeUndefined();

			expect(read_route_manifest_for_testing()).toBeUndefined();
			const manifest_warning = warn_spy.mock.calls.find(
				(call) => call[0] === "Failed to load route manifest:",
			);
			expect(manifest_warning).toBeDefined();
			expect(manifest_warning![0]).toBe("Failed to load route manifest:");
			expect(manifest_warning![1]).toBe(503);
		});

		it("ignores stale progressive manifest after newer init", async () => {
			const client = await load_client();
			set_route_manifest_for_testing(undefined);
			const older_manifest = { "/stale-old/:id": 1 };
			const newer_manifest = { "/stale-new/:id": 1 };
			const older_deferred = create_deferred<Response>();
			const newer_deferred = create_deferred<Response>();
			vi.spyOn(window, "fetch").mockImplementation(
				(input: RequestInfo | URL) => {
					const href =
						typeof input === "string"
							? input
							: input instanceof URL
								? input.href
								: input.url;
					if (href === "http://localhost:3000/manifest-old.json") {
						return older_deferred.promise;
					}
					if (href === "http://localhost:3000/manifest-new.json") {
						return newer_deferred.promise;
					}
					throw new Error(`Unexpected fetch: ${href}`);
				},
			);

			const older_init = client.initClient({
				routeManifestURL: "http://localhost:3000/manifest-old.json",
			});
			const newer_init = client.initClient({
				routeManifestURL: "http://localhost:3000/manifest-new.json",
			});

			newer_deferred.resolve(
				new Response(JSON.stringify(newer_manifest), {
					status: 200,
					headers: {
						"Content-Type": "application/json",
					},
				}),
			);
			await newer_init;
			expect(read_route_manifest_for_testing()).toEqual(newer_manifest);

			older_deferred.resolve(
				new Response(JSON.stringify(older_manifest), {
					status: 200,
					headers: {
						"Content-Type": "application/json",
					},
				}),
			);
			await older_init;

			expect(read_route_manifest_for_testing()).toEqual(newer_manifest);
		});

		it("ignores manifest payload when pattern registry is replaced mid-flight", async () => {
			const client = await load_client();
			set_route_manifest_for_testing(undefined);
			const deferred = create_deferred<Response>();
			vi.spyOn(window, "fetch").mockImplementation(
				() => deferred.promise,
			);

			const init_promise = client.initClient({
				routeManifestURL: "http://localhost:3000/manifest.json",
			});
			replace_pattern_registry_for_testing();

			deferred.resolve(
				new Response(
					JSON.stringify({
						"/should-be-ignored/:id": 1,
					}),
					{
						status: 200,
						headers: {
							"Content-Type": "application/json",
						},
					},
				),
			);
			await init_promise;

			expect(read_route_manifest_for_testing()).toBeUndefined();
		});
	});
});
