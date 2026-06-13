// @vitest-environment jsdom

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { SEARCH_PARAM_SCHEMA_NUMBER } from "../kit/json/search_param_parser.ts";
import {
	deferred,
	mock_fetch,
	register_ccc_lifecycle,
	route_render_commit_at,
	route_response,
	seed_payload,
	setup,
	t_opts,
	tick,
	wait_until,
} from "./_test_helpers.ts";
import { create_client_core } from "./create_client_core.ts";

register_ccc_lifecycle(beforeEach, afterEach);

describe("client loaders", () => {
	it("starts all client loaders in parallel before any resolves", async () => {
		const call_order: string[] = [];

		vi.doMock("/mod-a.js", () => {
			return {
				default: {
					pattern: "/parent",
					component: () => {
						return null;
					},
					client_loader: async ({ serverPromise }: any) => {
						call_order.push("a_start");
						await serverPromise;
						call_order.push("a_end");
						return { a: true };
					},
				},
			};
		});

		vi.doMock("/mod-b.js", () => {
			return {
				default: {
					pattern: "/parent/child",
					component: () => {
						return null;
					},
					client_loader: async ({ serverPromise }: any) => {
						call_order.push("b_start");
						await serverPromise;
						call_order.push("b_end");
						return { b: true };
					},
				},
			};
		});

		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core({}, commit, t_opts());
		if (!core_res.ok) {
			throw new Error(`create_client_core failed with error: ${core_res.err}`);
		}
		const core = core_res.val;

		await core.boot({});

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({
				matched_patterns: ["/parent", "/parent/child"],
				views_data: [{ p: 1 }, { c: 2 }],
				import_urls: ["/mod-a.js", "/mod-b.js"],
			}),
		);

		await core.navigate("/parent/child");

		expect(call_order.indexOf("a_start")).toBeLessThan(call_order.indexOf("a_end"));
		expect(call_order.indexOf("b_start")).toBeLessThan(call_order.indexOf("b_end"));
		expect(call_order.indexOf("a_start")).toBeLessThan(call_order.indexOf("b_end"));
		expect(call_order.indexOf("b_start")).toBeLessThan(call_order.indexOf("a_end"));
	});

	it("passes cached search-schema input to prestarted client loaders", async () => {
		let captured_args: any = null;
		const client_loader_started = deferred<void>();

		vi.doMock("/users-module.js", () => {
			return {
				default: {
					pattern: "/users",
					component: () => {
						return null;
					},
					client_loader: async (args: any) => {
						if (args.trigger === "navigation") {
							captured_args = args;
							client_loader_started.resolve();
							await args.serverPromise;
						}
						return { client: true };
					},
				},
			};
		});

		const { core } = await setup({
			payload: {
				matched_patterns: ["/users"],
				views_data: [{ users: "initial" }],
				import_urls: ["/users-module.js"],
				search_schemas: [
					{
						page: SEARCH_PARAM_SCHEMA_NUMBER,
					},
				],
			},
		});
		const fetcher = mock_fetch();

		const nav = core.navigate("/users?page=3");
		await client_loader_started.promise;

		expect(captured_args.input).toEqual({ page: 3 });
		expect(captured_args.knownMatches).toEqual([
			{ pattern: "/users", input: { page: 3 } },
		]);

		fetcher.call(0).resolve(
			route_response({
				matched_patterns: ["/users"],
				views_data: [{ users: "next" }],
				import_urls: ["/users-module.js"],
				search_schemas: [
					{
						page: SEARCH_PARAM_SCHEMA_NUMBER,
					},
				],
			}),
		);
		await nav;
	});

	it("receives correct serverPromise content", async () => {
		let captured_server_data: any = null;

		vi.doMock("/mod-root.js", () => {
			return {
				default: {
					pattern: "/",
					component: () => {
						return null;
					},
				},
			};
		});

		vi.doMock("/mod-users.js", () => {
			return {
				default: {
					pattern: "/users/:id",
					component: () => {
						return null;
					},
					client_loader: async ({ serverPromise }: any) => {
						captured_server_data = await serverPromise;
						return { enhanced: true };
					},
				},
			};
		});

		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core({}, commit, t_opts());
		if (!core_res.ok) {
			throw new Error(`create_client_core failed with error: ${core_res.err}`);
		}
		const core = core_res.val;

		await core.boot({});

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({
				matched_patterns: ["/", "/users/:id"],
				views_data: [{ session: "abc" }, { name: "Ada" }],
				import_urls: ["/mod-root.js", "/mod-users.js"],
				params: { id: "42" },
			}),
		);

		await core.navigate("/users/42");

		expect(captured_server_data).toEqual({
			clientBuildId: "build-1",
			matches: [
				{
					pattern: "/",
					input: {},
					viewData: { session: "abc" },
				},
				{
					pattern: "/users/:id",
					input: {},
					viewData: { name: "Ada" },
				},
			],
			outermostServerError: null,
			viewData: { name: "Ada" },
		});
	});

	it("passes target facts and finalized server state to client loaders", async () => {
		let captured_args: any = null;
		let captured_server_data: any = null;

		vi.doMock("/mod-root.js", () => {
			return {
				default: {
					pattern: "/",
					component: () => {
						return null;
					},
				},
			};
		});

		vi.doMock("/mod-users.js", () => {
			return {
				default: {
					pattern: "/users/:id",
					component: () => {
						return null;
					},
					client_loader: async (args: any) => {
						captured_args = args;
						captured_server_data = await args.serverPromise;
						return { enhanced: true };
					},
				},
			};
		});

		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core({}, commit, t_opts());
		if (!core_res.ok) {
			throw new Error(`create_client_core failed with error: ${core_res.err}`);
		}
		const core = core_res.val;

		await core.boot({});

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({
				matched_patterns: ["/", "/users/:id"],
				views_data: [{ session: "abc" }, { name: "Ada" }],
				import_urls: ["/mod-root.js", "/mod-users.js"],
				params: { id: "42" },
				search_schemas: [
					{},
					{
						page: SEARCH_PARAM_SCHEMA_NUMBER,
					},
				],
			}),
		);

		const href = new URL("/users/42?page=2#bio", window.location.href).href;
		await core.navigate(href, { state: { from: "test" } });

		expect(captured_args).toMatchObject({
			trigger: "navigation",
			href,
			historyState: { from: "test" },
			pattern: "/users/:id",
			params: { id: "42" },
			splatValues: [],
			input: { page: 2 },
			knownMatches: [
				{ pattern: "/", input: {} },
				{ pattern: "/users/:id", input: { page: 2 } },
			],
		});
		expect(captured_args.signal).toBeInstanceOf(AbortSignal);
		expect(captured_server_data).toEqual({
			clientBuildId: "build-1",
			matches: [
				{
					pattern: "/",
					input: {},
					viewData: { session: "abc" },
				},
				{
					pattern: "/users/:id",
					input: { page: 2 },
					viewData: { name: "Ada" },
				},
			],
			outermostServerError: null,
			viewData: { name: "Ada" },
		});
	});

	it("aborts client loader signal when navigation is superseded", async () => {
		let captured_signal: AbortSignal | null = null;

		vi.doMock("/mod-slow.js", () => {
			return {
				default: {
					pattern: "/slow",
					component: () => {
						return null;
					},
					client_loader: async ({ signal, serverPromise }: any) => {
						captured_signal = signal;
						await serverPromise;
						await new Promise((r) => {
							return setTimeout(r, 1000);
						});
						return {};
					},
				},
			};
		});

		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core({}, commit, t_opts());
		if (!core_res.ok) {
			throw new Error(`create_client_core failed with error: ${core_res.err}`);
		}
		const core = core_res.val;

		await core.boot({});

		// First: navigate to /slow fully to register the client loader
		vi.spyOn(globalThis, "fetch")
			.mockResolvedValueOnce(
				route_response({
					matched_patterns: ["/slow"],
					views_data: [{ v: 1 }],
					import_urls: ["/mod-slow.js"],
				}),
			)
			.mockResolvedValueOnce(route_response());

		await core.navigate("/slow");
		await core.navigate("/other");
		captured_signal = null;

		// Second: start navigation to /slow with hanging fetch
		vi.spyOn(globalThis, "fetch").mockImplementation(() => {
			return new Promise(() => {});
		});

		void core.navigate("/slow");
		await tick();

		expect(captured_signal).not.toBeNull();
		expect(captured_signal!.aborted).toBe(false);

		void core.navigate("/other2");

		expect(captured_signal!.aborted).toBe(true);
	});

	it("failure in one client loader cascades abort to subsequent client loaders", async () => {
		let second_signal: AbortSignal | null = null;

		vi.doMock("/mod-fail.js", () => {
			return {
				default: {
					pattern: "/fail-parent",
					component: () => {
						return null;
					},
					client_loader: async () => {
						throw new Error("client loader failed");
					},
				},
			};
		});

		vi.doMock("/mod-child.js", () => {
			return {
				default: {
					pattern: "/fail-parent/child",
					component: () => {
						return null;
					},
					client_loader: async ({ signal, serverPromise }: any) => {
						second_signal = signal;
						await serverPromise;
						return {};
					},
				},
			};
		});

		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core({}, commit, t_opts());
		if (!core_res.ok) {
			throw new Error(`create_client_core failed with error: ${core_res.err}`);
		}
		const core = core_res.val;

		await core.boot({});

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({
				matched_patterns: ["/fail-parent", "/fail-parent/child"],
				views_data: [{ p: 1 }, { c: 2 }],
				import_urls: ["/mod-fail.js", "/mod-child.js"],
			}),
		);

		await core.navigate("/fail-parent/child");

		expect(second_signal).not.toBeNull();
		expect(second_signal!.aborted).toBe(true);
	});

	it("non-abort errors surface as route error state", async () => {
		vi.doMock("/mod-err.js", () => {
			return {
				default: {
					pattern: "/err-route",
					component: () => {
						return null;
					},
					client_loader: async () => {
						throw new Error("client loader boom");
					},
				},
			};
		});

		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core({}, commit, t_opts());
		if (!core_res.ok) {
			throw new Error(`create_client_core failed with error: ${core_res.err}`);
		}
		const core = core_res.val;

		await core.boot({});
		commit.mockClear();

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({
				matched_patterns: ["/err-route"],
				views_data: [{ v: 1 }],
				import_urls: ["/mod-err.js"],
			}),
		);

		await core.navigate("/err-route");

		const state = route_render_commit_at(commit, 0);
		expect(state.error).toEqual({
			idx: 0,
			error: "client loader boom",
			source: "clientLoader",
		});
	});

	it("client loader results stored as client_loader_data on route entries", async () => {
		vi.doMock("/mod-cl.js", () => {
			return {
				default: {
					pattern: "/cl-route",
					component: () => {
						return null;
					},
					client_loader: async ({ serverPromise }: any) => {
						const server = await serverPromise;
						return { enhanced: true, original: server.viewData };
					},
				},
			};
		});

		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core({}, commit, t_opts());
		if (!core_res.ok) {
			throw new Error(`create_client_core failed with error: ${core_res.err}`);
		}
		const core = core_res.val;

		await core.boot({});
		commit.mockClear();

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({
				matched_patterns: ["/cl-route"],
				views_data: [{ raw: "data" }],
				import_urls: ["/mod-cl.js"],
			}),
		);

		await core.navigate("/cl-route");

		const state = route_render_commit_at(commit, 0);
		expect(state.entries[0].client_loader_data).toEqual({
			enhanced: true,
			original: { raw: "data" },
		});
	});

	it("abort errors are silently swallowed", async () => {
		vi.doMock("/mod-abort.js", () => {
			return {
				default: {
					pattern: "/abort-route",
					component: () => {
						return null;
					},
					client_loader: async () => {
						throw new DOMException("Aborted", "AbortError");
					},
				},
			};
		});

		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core({}, commit, t_opts());
		if (!core_res.ok) {
			throw new Error(`create_client_core failed with error: ${core_res.err}`);
		}
		const core = core_res.val;

		await core.boot({});
		commit.mockClear();

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({
				matched_patterns: ["/abort-route"],
				views_data: [{ v: 1 }],
				import_urls: ["/mod-abort.js"],
			}),
		);

		await core.navigate("/abort-route");

		const state = route_render_commit_at(commit, 0);
		expect(state.error).toBeNull();
		expect(state.entries[0].client_loader_data).toBeUndefined();
	});
});

/////////////////////////////////////////////////////////////////////
/////// Build ID
/////////////////////////////////////////////////////////////////////

describe("client loader cancellation", () => {
	it("aborts client loader signal when prefetch is stopped", async () => {
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core({}, commit, t_opts());
		if (!core_res.ok) {
			throw new Error(`create_client_core failed with error: ${core_res.err}`);
		}
		const core = core_res.val;

		let captured_signal: AbortSignal | null = null;
		const client_loader = async ({ signal, serverPromise }: any) => {
			captured_signal = signal;
			await serverPromise;
			return {};
		};

		vi.doMock("/client-loader-module.js", () => {
			return {
				default: {
					pattern: "/client-loader",
					component: () => {
						return null;
					},
					client_loader: client_loader,
				},
			};
		});

		await core.boot({});

		vi.spyOn(globalThis, "fetch")
			.mockResolvedValueOnce(
				route_response({
					matched_patterns: ["/client-loader"],
					views_data: [{ value: 1 }],
					import_urls: ["/client-loader-module.js"],
				}),
			)
			.mockResolvedValueOnce(route_response());

		await core.navigate("/client-loader");
		await core.navigate("/other");
		captured_signal = null;

		vi.spyOn(globalThis, "fetch").mockImplementation(() => {
			return new Promise(() => {});
		});
		core.start_prefetch("/client-loader");
		await tick();

		expect(captured_signal).not.toBeNull();
		expect(captured_signal!.aborted).toBe(false);

		core.stop_prefetch("/client-loader");

		expect(captured_signal!.aborted).toBe(true);
	});

	it("aborts prestarted client loader when prefetch response has server error", async () => {
		let captured_signal: AbortSignal | null = null;
		let server_data_rejected = false;

		vi.doMock("/known-error-module.js", () => {
			return {
				default: {
					pattern: "/known-error",
					component: () => {
						return null;
					},
					client_loader: async ({ signal, serverPromise }: any) => {
						captured_signal = signal;
						try {
							await serverPromise;
						} catch (err) {
							server_data_rejected =
								err instanceof DOMException && err.name === "AbortError";
						}
						return {};
					},
				},
			};
		});

		const { core } = await setup();
		const { call, wait_for } = mock_fetch();

		const first_nav = core.navigate("/known-error");
		await wait_for(1);
		call(0).resolve(
			route_response({
				matched_patterns: ["/known-error"],
				views_data: [{ value: 1 }],
				import_urls: ["/known-error-module.js"],
			}),
		);
		await first_nav;

		const away_nav = core.navigate("/other");
		await wait_for(2);
		call(1).resolve(route_response());
		await away_nav;

		captured_signal = null;
		server_data_rejected = false;

		core.start_prefetch("/known-error");
		await wait_for(3);

		await wait_until(() => {
			return captured_signal !== null;
		}, "expected known prefetch client loader to start");
		expect(captured_signal!.aborted).toBe(false);

		call(2).resolve(
			route_response({
				matched_patterns: ["/known-error"],
				import_urls: ["/known-error-module.js"],
				outermost_server_err_idx: 0,
				outermost_server_err: "server boom",
			}),
		);

		await wait_until(() => {
			return captured_signal!.aborted && server_data_rejected;
		}, "expected server-error prefetch client loader to abort");
	});

	it("aborts prestarted exact and descendant client loaders when navigation response has server error", async () => {
		const signals: Record<string, AbortSignal | null> = {
			parent: null,
			child: null,
		};
		const server_data_rejected = {
			parent: false,
			child: false,
		};

		vi.doMock("/known-parent-module.js", () => {
			return {
				default: {
					pattern: "/known-parent",
					component: () => {
						return null;
					},
					client_loader: async ({ signal, serverPromise }: any) => {
						signals.parent = signal;
						try {
							await serverPromise;
						} catch (err) {
							server_data_rejected.parent =
								err instanceof DOMException && err.name === "AbortError";
						}
						return {};
					},
				},
			};
		});
		vi.doMock("/known-child-module.js", () => {
			return {
				default: {
					pattern: "/known-parent/child",
					component: () => {
						return null;
					},
					client_loader: async ({ signal, serverPromise }: any) => {
						signals.child = signal;
						try {
							await serverPromise;
						} catch (err) {
							server_data_rejected.child =
								err instanceof DOMException && err.name === "AbortError";
						}
						return {};
					},
				},
			};
		});

		const { core, commit } = await setup();
		const { call, wait_for } = mock_fetch();

		const first_nav = core.navigate("/known-parent/child");
		await wait_for(1);
		call(0).resolve(
			route_response({
				matched_patterns: ["/known-parent", "/known-parent/child"],
				views_data: [{ parent: 1 }, { child: 1 }],
				import_urls: ["/known-parent-module.js", "/known-child-module.js"],
			}),
		);
		await first_nav;

		const away_nav = core.navigate("/other");
		await wait_for(2);
		call(1).resolve(route_response());
		await away_nav;

		signals.parent = null;
		signals.child = null;
		server_data_rejected.parent = false;
		server_data_rejected.child = false;
		commit.mockClear();

		const server_error_nav = core.navigate("/known-parent/child");
		await wait_for(3);

		await wait_until(() => {
			return signals.parent !== null && signals.child !== null;
		}, "expected known navigation client loaders to start");
		expect(signals.parent!.aborted).toBe(false);
		expect(signals.child!.aborted).toBe(false);

		call(2).resolve(
			route_response({
				matched_patterns: ["/known-parent", "/known-parent/child"],
				import_urls: ["/known-parent-module.js"],
				outermost_server_err_idx: 0,
				outermost_server_err: "server boom",
			}),
		);
		await server_error_nav;

		await wait_until(() => {
			return (
				signals.parent!.aborted &&
				signals.child!.aborted &&
				server_data_rejected.parent &&
				server_data_rejected.child
			);
		}, "expected server-error navigation client loaders to abort");
		expect(route_render_commit_at(commit, 0).error).toEqual({
			idx: 0,
			error: "server boom",
			source: "server",
		});
	});
});

/////////////////////////////////////////////////////////////////////
/////// Client loader prefetch isolation from unrelated work
/////////////////////////////////////////////////////////////////////

describe("client loader prefetch isolation from unrelated work", () => {
	it("does not abort client loader prefetch signal when unrelated submit fires", async () => {
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core({}, commit, t_opts());
		if (!core_res.ok) {
			throw new Error(`create_client_core failed with error: ${core_res.err}`);
		}
		const core = core_res.val;

		let captured_signal: AbortSignal | null = null;
		const client_loader = async ({ signal, serverPromise }: any) => {
			captured_signal = signal;
			await serverPromise;
			return {};
		};

		vi.doMock("/product-module.js", () => {
			return {
				default: {
					pattern: "/products",
					component: () => {
						return null;
					},
					client_loader: client_loader,
				},
			};
		});

		await core.boot({});

		vi.spyOn(globalThis, "fetch")
			.mockResolvedValueOnce(
				route_response({
					matched_patterns: ["/products"],
					views_data: [{ value: 1 }],
					import_urls: ["/product-module.js"],
				}),
			)
			.mockResolvedValueOnce(route_response());

		await core.navigate("/products");
		await core.navigate("/other");
		captured_signal = null;

		vi.spyOn(globalThis, "fetch").mockImplementation(() => {
			return new Promise(() => {});
		});
		core.start_prefetch("/products");
		await tick();

		expect(captured_signal).not.toBeNull();
		expect(captured_signal!.aborted).toBe(false);

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			new Response(JSON.stringify({ ok: true }), {
				status: 200,
				headers: { "Content-Type": "application/json" },
			}),
		);
		await core.submit_inner(
			"/api/save",
			{ method: "POST" },
			{
				revalidate: false,
			},
		);

		expect(captured_signal!.aborted).toBe(false);
	});
});

/////////////////////////////////////////////////////////////////////
/////// Client loader prefetch partial matching
/////////////////////////////////////////////////////////////////////

describe("client loader prefetch partial matching", () => {
	it("prestarts parent client loaders when navigating to deeper unregistered path", async () => {
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core({}, commit, t_opts());
		if (!core_res.ok) {
			throw new Error(`create_client_core failed with error: ${core_res.err}`);
		}
		const core = core_res.val;

		let prestart_count = 0;
		const client_loader = async ({ serverPromise }: any) => {
			prestart_count++;
			const data = await serverPromise;
			return data;
		};

		vi.doMock("/parent-module.js", () => {
			return {
				default: {
					pattern: "/parent",
					component: () => {
						return null;
					},
					client_loader: client_loader,
				},
			};
		});

		vi.doMock("/child-module.js", () => {
			return {
				default: {
					pattern: "/parent/child",
					component: () => {
						return null;
					},
				},
			};
		});

		await core.boot({});

		vi.spyOn(globalThis, "fetch")
			.mockResolvedValueOnce(
				route_response({
					matched_patterns: ["/parent"],
					views_data: [{ value: 1 }],
					import_urls: ["/parent-module.js"],
				}),
			)
			.mockResolvedValueOnce(route_response());

		await core.navigate("/parent");
		await core.navigate("/other");
		prestart_count = 0;

		let resolve_fetch!: (r: Response) => void;
		vi.spyOn(globalThis, "fetch").mockImplementation(() => {
			return new Promise((resolve) => {
				resolve_fetch = resolve;
			});
		});

		const nav = core.navigate("/parent/child");
		await tick();

		expect(prestart_count).toBe(1);

		resolve_fetch(
			route_response({
				matched_patterns: ["/parent", "/parent/child"],
				views_data: [{ parent: true }, { child: true }],
				import_urls: ["/parent-module.js", "/child-module.js"],
			}),
		);
		await nav;
	});
});

/////////////////////////////////////////////////////////////////////
/////// View transition timing
/////////////////////////////////////////////////////////////////////

describe("route update blocked by client loaders", () => {
	it("does not fire onRouteUpdate until client loaders settle", async () => {
		let client_loader_resolve: ((v: unknown) => void) | undefined;

		vi.doMock("/mod-blocking.js", () => {
			return {
				default: {
					pattern: "/blocking",
					component: () => {
						return null;
					},
					client_loader: async ({ serverPromise }: any) => {
						await serverPromise;
						return new Promise((r) => {
							client_loader_resolve = r;
						});
					},
				},
			};
		});

		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core({}, commit, t_opts());
		if (!core_res.ok) {
			throw new Error(`create_client_core failed with error: ${core_res.err}`);
		}
		const core = core_res.val;

		let route_changed = false;
		await core.boot({
			onRouteUpdate: (_route, _previous_route, reason) => {
				if (reason === "boot") {
					return;
				}
				route_changed = true;
			},
		});

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({
				matched_patterns: ["/blocking"],
				views_data: [{ v: 1 }],
				import_urls: ["/mod-blocking.js"],
			}),
		);

		const nav = core.navigate("/blocking");

		for (let i = 0; i < 200; i++) {
			if (client_loader_resolve) {
				break;
			}
			await new Promise((r) => {
				return setTimeout(r, 0);
			});
		}

		expect(client_loader_resolve).toBeDefined();
		expect(route_changed).toBe(false);

		client_loader_resolve!({ loaded: true });
		await nav;

		expect(route_changed).toBe(true);
	});
});

/////////////////////////////////////////////////////////////////////
/////// CSS preload gating
/////////////////////////////////////////////////////////////////////

describe("client loader promise reuse on hash change", () => {
	it("retargets in-flight same-href navigation when options differ", async () => {
		const { core } = await setup();
		let resolve_fetch!: (r: Response) => void;
		vi.spyOn(globalThis, "fetch").mockImplementation(() => {
			return new Promise((resolve) => {
				resolve_fetch = resolve;
			});
		});

		const first = core.navigate("/reuse", {
			state: { request: "first" },
		});
		await tick();

		const second = core.navigate("/reuse", {
			state: { request: "second" },
		});
		const pending = Symbol("pending");
		let first_result: unknown = pending;
		void first.then((result) => {
			first_result = result;
		});
		await tick();
		expect(first_result).toEqual({ didNavigate: false });

		resolve_fetch(
			route_response({
				matched_patterns: ["/reuse"],
				views_data: [{ v: 1 }],
			}),
		);

		await expect(second).resolves.toEqual({ didNavigate: true });
		expect(core.getRouteState().historyState).toEqual({
			request: "second",
		});
	});

	it("does not re-invoke client loader when only hash changes on in-flight navigation", async () => {
		let invocation_count = 0;

		vi.doMock("/mod-reuse.js", () => {
			return {
				default: {
					pattern: "/reuse",
					component: () => {
						return null;
					},
					client_loader: async ({ serverPromise }: any) => {
						invocation_count++;
						await serverPromise;
						return { count: invocation_count };
					},
				},
			};
		});

		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core({}, commit, t_opts());
		if (!core_res.ok) {
			throw new Error(`create_client_core failed with error: ${core_res.err}`);
		}
		const core = core_res.val;

		await core.boot({});

		let resolve_fetch!: (r: Response) => void;
		vi.spyOn(globalThis, "fetch").mockImplementation(() => {
			return new Promise((r) => {
				resolve_fetch = r;
			});
		});

		// First navigate registers the client loader
		const first_nav = core.navigate("/reuse");
		await tick();
		resolve_fetch(
			route_response({
				matched_patterns: ["/reuse"],
				views_data: [{ v: 1 }],
				import_urls: ["/mod-reuse.js"],
			}),
		);
		await first_nav;
		expect(invocation_count).toBe(1);

		// Navigate away
		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(route_response());
		await core.navigate("/other");
		invocation_count = 0;

		// Navigate to /reuse#one — starts client loader prefetch + fetch
		vi.spyOn(globalThis, "fetch").mockImplementation(() => {
			return new Promise((r) => {
				resolve_fetch = r;
			});
		});
		const hash_nav_1 = core.navigate("/reuse#one");
		await tick();

		expect(invocation_count).toBe(1);

		// Navigate to /reuse#two — same data URL, router reuses in-flight
		const hash_nav_2 = core.navigate("/reuse#two");
		await tick();

		// Client loader should NOT have been called again
		expect(invocation_count).toBe(1);

		resolve_fetch(
			route_response({
				matched_patterns: ["/reuse"],
				views_data: [{ v: 2 }],
				import_urls: ["/mod-reuse.js"],
			}),
		);
		await Promise.all([hash_nav_1, hash_nav_2]);
	});
});

/////////////////////////////////////////////////////////////////////
/////// History state
/////////////////////////////////////////////////////////////////////
