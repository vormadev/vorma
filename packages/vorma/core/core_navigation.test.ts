// @vitest-environment jsdom

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
	SEARCH_PARAM_SCHEMA_BOOL,
	SEARCH_PARAM_SCHEMA_NUMBER,
	SEARCH_PARAM_SCHEMA_STRING,
} from "../kit/json/search_param_parser.ts";
import {
	deferred,
	has_route_render_commit,
	mock_fetch,
	register_ccc_lifecycle,
	route_render_commit_at,
	route_render_commit_count,
	route_render_scroll_intent_at,
	route_response,
	seed_payload,
	setup,
	t_opts,
	tick,
	wait_until,
} from "./_test_helpers.ts";
import {
	REVALIDATION_DEBOUNCE_MS,
	create_client_core,
	type ClientCommit,
} from "./create_client_core.ts";

register_ccc_lifecycle(beforeEach, afterEach);

describe("navigation flow", () => {
	it("fetches, decodes payload, and commits state on navigate", async () => {
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
				matched_patterns: ["/about"],
				views_data: [{ page: "about" }],
			}),
		);

		await core.navigate("/about");

		expect(has_route_render_commit(commit)).toBe(true);
		const state = route_render_commit_at(commit, 0);
		expect(state.entries).toHaveLength(1);
		expect(state.entries[0].pattern).toBe("/about");
		expect(state.entries[0].view_data).toEqual({ page: "about" });
	});

	it("parses matched input from navigation search schemas", async () => {
		const { core, commit } = await setup();
		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({
				matched_patterns: ["/users"],
				views_data: [{ users: true }],
				search_schemas: [
					{
						active: SEARCH_PARAM_SCHEMA_BOOL,
						page: SEARCH_PARAM_SCHEMA_NUMBER,
						tags: [SEARCH_PARAM_SCHEMA_STRING],
					},
				],
			}),
		);

		await core.navigate("/users?page=3&active=false&tags=c&tags=d");

		const state = route_render_commit_at(commit, 0);
		expect(state.entries[0].input).toEqual({
			active: false,
			page: 3,
			tags: ["c", "d"],
		});
	});

	it("commit receives scroll intent", async () => {
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
			route_response({ matched_patterns: ["/page"] }),
		);

		await core.navigate("/page");

		expect(has_route_render_commit(commit)).toBe(true);
		const scroll_intent = route_render_scroll_intent_at(commit, 0);
		expect(scroll_intent?.scroll).toEqual({ x: 0, y: 0 });
	});

	it("uses view transitions for navigations when enabled", async () => {
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core({}, commit, t_opts());
		if (!core_res.ok) {
			throw new Error(`create_client_core failed with error: ${core_res.err}`);
		}
		const core = core_res.val;

		await core.boot({ useViewTransitions: true });
		commit.mockClear();

		let transition_called = false;
		(document as any).startViewTransition = (cb: () => void) => {
			transition_called = true;
			cb();
			return { finished: Promise.resolve() };
		};

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({ matched_patterns: ["/page"] }),
		);

		await core.navigate("/page");

		expect(transition_called).toBe(true);
	});

	it("clears active navigation after publish before view transition finishes", async () => {
		const { core, commit } = await setup({
			clientOptions: { useViewTransitions: true },
		});

		let finish_transition!: () => void;
		(document as any).startViewTransition = (cb: () => void) => {
			cb();
			return {
				updateCallbackDone: Promise.resolve(),
				finished: new Promise<void>((resolve) => {
					finish_transition = resolve;
				}),
			};
		};

		const { calls, call, wait_for } = mock_fetch();
		const first = core.navigate("/page#one");
		await wait_for(1);
		call(0).resolve(route_response({ matched_patterns: ["/page"] }));

		await wait_until(() => {
			return route_render_commit_count(commit) > 0;
		}, "route did not publish during view transition");

		const second = await core.navigate("/page#two");

		expect(calls).toHaveLength(1);
		expect(second.didNavigate).toBe(true);
		expect(window.location.hash).toBe("#two");

		finish_transition();
		await expect(first).resolves.toEqual({ didNavigate: true });
	});

	it("does not publish a superseded navigation from a delayed view transition callback", async () => {
		const { core, commit } = await setup({
			clientOptions: { useViewTransitions: true },
		});

		const transitions: Array<() => void> = [];
		(document as any).startViewTransition = (cb: () => void) => {
			let resolve_update!: () => void;
			const updateCallbackDone = new Promise<void>((resolve) => {
				resolve_update = resolve;
			});
			transitions.push(() => {
				cb();
				resolve_update();
			});
			return {
				updateCallbackDone,
				finished: Promise.resolve(),
			};
		};

		const { call, wait_for } = mock_fetch();
		const first = core.navigate("/first");
		await wait_for(1);
		call(0).resolve(
			route_response({
				matched_patterns: ["/first"],
				views_data: [{ page: "first" }],
			}),
		);
		await wait_until(() => {
			return transitions.length === 1;
		}, "first view transition did not start");

		const second = core.navigate("/second");
		await wait_for(2);
		call(1).resolve(
			route_response({
				matched_patterns: ["/second"],
				views_data: [{ page: "second" }],
			}),
		);
		await wait_until(() => {
			return transitions.length === 2;
		}, "second view transition did not start");

		transitions[0]!();
		await tick();

		expect(has_route_render_commit(commit)).toBe(false);
		await expect(first).resolves.toEqual({ didNavigate: false });

		transitions[1]!();
		await expect(second).resolves.toEqual({ didNavigate: true });

		expect(route_render_commit_count(commit)).toBe(1);
		const state = route_render_commit_at(commit, 0);
		expect(state.entries[0].view_data).toEqual({ page: "second" });
	});
});

/////////////////////////////////////////////////////////////////////
/////// beforeRouteYield / beforeRouteCommit
/////////////////////////////////////////////////////////////////////

describe("beforeRouteYield / beforeRouteCommit", () => {
	it("runs current matched yield hooks in parallel before publishing successor route", async () => {
		const root_gate = deferred<void>();
		const current_gate = deferred<void>();
		const calls: string[] = [];
		const hook_args: any[] = [];

		vi.doMock("/root.js", () => {
			return {
				default: {
					pattern: "/",
					component: () => {
						return null;
					},
					before_route_yield: async (args: any) => {
						calls.push("root_start");
						hook_args.push(args);
						await root_gate.promise;
						calls.push("root_end");
					},
				},
			};
		});
		vi.doMock("/current.js", () => {
			return {
				default: {
					pattern: "/current",
					component: () => {
						return null;
					},
					before_route_yield: async (args: any) => {
						calls.push("current_start");
						hook_args.push(args);
						await current_gate.promise;
						calls.push("current_end");
					},
				},
			};
		});
		vi.doMock("/next.js", () => {
			return {
				default: {
					pattern: "/next",
					component: () => {
						return null;
					},
				},
			};
		});

		const { core, commit } = await setup({
			payload: {
				matched_patterns: ["/", "/current"],
				views_data: [{ root: "old" }, { page: "current" }],
				import_urls: ["/root.js", "/current.js"],
			},
		});
		const { call, wait_for } = mock_fetch();

		const nav = core.navigate("/next", { state: { via: "test" } });
		await wait_for(1);
		call(0).resolve(
			route_response({
				matched_patterns: ["/", "/next"],
				views_data: [{ root: "new" }, { page: "next" }],
				import_urls: ["/root.js", "/next.js"],
			}),
		);
		await wait_until(() => {
			return calls.length === 2;
		}, "beforeRouteYield hooks did not start");

		expect(calls).toEqual(["root_start", "current_start"]);
		expect(has_route_render_commit(commit)).toBe(false);
		expect(hook_args).toHaveLength(2);
		expect(hook_args[0].trigger).toBe("navigation");
		expect(hook_args[0].current.href).toBe(window.location.origin + "/");
		expect(hook_args[0].next.href).toBe(window.location.origin + "/next");
		expect(hook_args[0].next.historyState).toEqual({ via: "test" });

		root_gate.resolve();
		await tick();
		expect(has_route_render_commit(commit)).toBe(false);

		current_gate.resolve();
		await expect(nav).resolves.toEqual({ didNavigate: true });
		expect(calls).toEqual(["root_start", "current_start", "root_end", "current_end"]);
		expect(route_render_commit_count(commit)).toBe(1);
		expect(route_render_commit_at(commit, 0).entries[1].view_data).toEqual({
			page: "next",
		});
	});

	it("runs current yield hooks and next commit hooks in parallel", async () => {
		const yield_gate = deferred<void>();
		const commit_gate = deferred<void>();
		const calls: string[] = [];

		vi.doMock("/current.js", () => {
			return {
				default: {
					pattern: "/current",
					component: () => {
						return null;
					},
					before_route_yield: async () => {
						calls.push("yield_start");
						await yield_gate.promise;
						calls.push("yield_end");
					},
				},
			};
		});
		vi.doMock("/next.js", () => {
			return {
				default: {
					pattern: "/next",
					component: () => {
						return null;
					},
					before_route_commit: async () => {
						calls.push("commit_start");
						await commit_gate.promise;
						calls.push("commit_end");
					},
				},
			};
		});

		const { core, commit } = await setup({
			payload: {
				matched_patterns: ["/current"],
				views_data: [{ page: "current" }],
				import_urls: ["/current.js"],
			},
		});
		const { call, wait_for } = mock_fetch();

		const nav = core.navigate("/next");
		await wait_for(1);
		call(0).resolve(
			route_response({
				matched_patterns: ["/next"],
				views_data: [{ page: "next" }],
				import_urls: ["/next.js"],
			}),
		);
		await wait_until(() => {
			return calls.length === 2;
		}, "beforeRouteYield / beforeRouteCommit hooks did not start");

		expect(calls).toEqual(["yield_start", "commit_start"]);
		expect(has_route_render_commit(commit)).toBe(false);

		yield_gate.resolve();
		await tick();
		expect(has_route_render_commit(commit)).toBe(false);

		commit_gate.resolve();
		await expect(nav).resolves.toEqual({ didNavigate: true });
		expect(calls).toEqual(["yield_start", "commit_start", "yield_end", "commit_end"]);
		expect(route_render_commit_count(commit)).toBe(1);
	});

	it("does not run yield hooks from successor-only routes or commit hooks from current-only routes", async () => {
		const current_commit_hook = vi.fn();
		const successor_yield_hook = vi.fn();
		const successor_commit_hook = vi.fn();

		vi.doMock("/current.js", () => {
			return {
				default: {
					pattern: "/current",
					component: () => {
						return null;
					},
					before_route_commit: current_commit_hook,
				},
			};
		});
		vi.doMock("/next.js", () => {
			return {
				default: {
					pattern: "/next",
					component: () => {
						return null;
					},
					before_route_yield: successor_yield_hook,
					before_route_commit: successor_commit_hook,
				},
			};
		});

		const { core, commit } = await setup({
			payload: {
				matched_patterns: ["/current"],
				views_data: [{ page: "current" }],
				import_urls: ["/current.js"],
			},
		});
		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({
				matched_patterns: ["/next"],
				views_data: [{ page: "next" }],
				import_urls: ["/next.js"],
			}),
		);

		await expect(core.navigate("/next")).resolves.toEqual({
			didNavigate: true,
		});

		expect(current_commit_hook).not.toHaveBeenCalled();
		expect(successor_yield_hook).not.toHaveBeenCalled();
		expect(successor_commit_hook).toHaveBeenCalledTimes(1);
		expect(route_render_commit_count(commit)).toBe(1);
	});

	it("aborts superseded yield hooks and does not publish stale route", async () => {
		let aborted = false;
		const hook_started = deferred<void>();
		const first_gate = deferred<void>();

		vi.doMock("/current.js", () => {
			return {
				default: {
					pattern: "/current",
					component: () => {
						return null;
					},
					before_route_yield: ({ signal }: any) => {
						hook_started.resolve();
						signal.addEventListener(
							"abort",
							() => {
								aborted = true;
								first_gate.resolve();
							},
							{ once: true },
						);
						return first_gate.promise;
					},
				},
			};
		});

		const { core, commit } = await setup({
			payload: {
				matched_patterns: ["/current"],
				views_data: [{ page: "current" }],
				import_urls: ["/current.js"],
			},
		});
		const { call, wait_for } = mock_fetch();

		const first = core.navigate("/first");
		await wait_for(1);
		call(0).resolve(
			route_response({
				matched_patterns: ["/first"],
				views_data: [{ page: "first" }],
			}),
		);
		await hook_started.promise;

		const second = core.navigate("/second");
		await wait_for(2);
		call(1).resolve(
			route_response({
				matched_patterns: ["/second"],
				views_data: [{ page: "second" }],
			}),
		);

		await expect(first).resolves.toEqual({ didNavigate: false });
		await expect(second).resolves.toEqual({ didNavigate: true });

		expect(aborted).toBe(true);
		expect(route_render_commit_count(commit)).toBe(1);
		expect(route_render_commit_at(commit, 0).entries[0].view_data).toEqual({
			page: "second",
		});
	});

	it("settles navigation without publishing when a route hook rejects", async () => {
		vi.doMock("/current.js", () => {
			return {
				default: {
					pattern: "/current",
					component: () => {
						return null;
					},
					before_route_yield: async () => {
						throw new Error("yield rejected");
					},
				},
			};
		});

		const { core, commit } = await setup({
			payload: {
				matched_patterns: ["/current"],
				views_data: [{ page: "current" }],
				import_urls: ["/current.js"],
			},
		});
		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({
				matched_patterns: ["/next"],
				views_data: [{ page: "next" }],
			}),
		);

		const pending = Symbol("pending");
		let result: unknown = pending;
		void core.navigate("/next").then((nav_result) => {
			result = nav_result;
		});
		await wait_until(() => {
			return result !== pending;
		}, "navigation did not settle after route hook rejection");

		expect(result).toEqual({ didNavigate: false });
		expect(route_render_commit_count(commit)).toBe(0);
	});
});

/////////////////////////////////////////////////////////////////////
/////// Client loaders
/////////////////////////////////////////////////////////////////////

describe("view transition timing", () => {
	it("does not apply title before view transition snapshot is captured", async () => {
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core({}, commit, t_opts());
		if (!core_res.ok) {
			throw new Error(`create_client_core failed with error: ${core_res.err}`);
		}
		const core = core_res.val;

		await core.boot({ useViewTransitions: true });

		let title_when_transition_started: string | null = null;
		(document as any).startViewTransition = (cb: () => void) => {
			title_when_transition_started = document.title;
			cb();
			return { finished: Promise.resolve() };
		};

		document.title = "Old Title";

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({
				title: { dangerous_inner_html: "New Title" },
			}),
		);

		await core.navigate("/new-page");

		expect(title_when_transition_started).toBe("Old Title");
	});
});

/////////////////////////////////////////////////////////////////////
/////// Stale navigation side effects
/////////////////////////////////////////////////////////////////////

describe("stale navigation side effects", () => {
	it("does not apply title, CSS, or build ID from superseded navigation", async () => {
		seed_payload({ client_build_id: "build-1" });
		const commit = vi.fn();
		const core_res = create_client_core({}, commit, t_opts());
		if (!core_res.ok) {
			throw new Error(`create_client_core failed with error: ${core_res.err}`);
		}
		const core = core_res.val;

		const build_skews: string[] = [];
		await core.boot({
			onBuildSkewDetected: (event) => {
				build_skews.push(event.serverBuildId);
			},
		});
		commit.mockClear();

		let resolve_stale!: (r: Response) => void;
		let call_count = 0;
		vi.spyOn(globalThis, "fetch").mockImplementation(() => {
			call_count++;
			if (call_count === 1) {
				return new Promise((r) => {
					resolve_stale = r;
				});
			}
			return Promise.resolve(
				route_response(
					{ title: { dangerous_inner_html: "Winner" } },
					"winner-build",
				),
			);
		});

		const stale = core.navigate("/stale-page");
		await tick();
		await core.navigate("/winner-page");

		expect(document.title).toBe("Winner");
		expect(core.getClientBuildId()).toBe("build-1");

		resolve_stale(
			route_response(
				{
					title: { dangerous_inner_html: "Stale Title" },
					css_bundles: ["/stale.css"],
				},
				"stale-build",
			),
		);
		await stale;
		await tick();

		expect(document.title).toBe("Winner");
		expect(core.getClientBuildId()).toBe("build-1");
		expect(
			document.head.querySelector('link[data-vorma-css-bundle="/stale.css"]'),
		).toBeNull();
		expect(
			build_skews.some((server_build_id) => {
				return server_build_id === "stale-build";
			}),
		).toBe(false);
	});

	it("does not commit stale revalidation after navigation supersedes it", async () => {
		vi.useFakeTimers();

		try {
			seed_payload({ client_build_id: "build-1" });
			const commit = vi.fn();
			const core_res = create_client_core({}, commit, t_opts());
			if (!core_res.ok) {
				throw new Error(`create_client_core failed with error: ${core_res.err}`);
			}
			const core = core_res.val;

			await core.boot({});
			commit.mockClear();

			let resolve_revalidation!: (r: Response) => void;
			let resolve_navigation!: (r: Response) => void;
			let call_count = 0;
			vi.spyOn(globalThis, "fetch").mockImplementation(() => {
				call_count++;
				if (call_count === 1) {
					return new Promise((r) => {
						resolve_revalidation = r;
					});
				}
				return new Promise((r) => {
					resolve_navigation = r;
				});
			});

			const revalidation = core.revalidate();
			await vi.advanceTimersByTimeAsync(REVALIDATION_DEBOUNCE_MS);
			await tick();

			expect(call_count).toBe(1);

			const navigation = core.navigate("/winner-page");
			await tick();

			expect(call_count).toBe(2);

			resolve_navigation(
				route_response(
					{
						matched_patterns: ["/winner-page"],
						title: { dangerous_inner_html: "Winner" },
					},
					"winner-build",
				),
			);
			await navigation;
			await revalidation;

			expect(document.title).toBe("Winner");
			expect(core.getClientBuildId()).toBe("build-1");
			expect(route_render_commit_count(commit)).toBe(1);

			resolve_revalidation(
				route_response(
					{
						matched_patterns: ["/stale-revalidation"],
						title: { dangerous_inner_html: "Stale" },
					},
					"stale-build",
				),
			);
			await tick();

			expect(document.title).toBe("Winner");
			expect(core.getClientBuildId()).toBe("build-1");
			expect(route_render_commit_count(commit)).toBe(1);
		} finally {
			vi.useRealTimers();
		}
	});
});

/////////////////////////////////////////////////////////////////////
/////// Route update coherence
/////////////////////////////////////////////////////////////////////

describe("route update coherence", () => {
	it("state is updated before onRouteUpdate fires", async () => {
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core({}, commit, t_opts());
		if (!core_res.ok) {
			throw new Error(`create_client_core failed with error: ${core_res.err}`);
		}
		const core = core_res.val;

		let state_during_callback: unknown = null;
		let route_update: unknown = null;
		await core.boot({
			onRouteUpdate: (route, previous_route, reason) => {
				state_during_callback = core.getRouteState();
				route_update = { route, previous_route, reason };
			},
		});

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({
				matched_patterns: ["/coherence"],
				views_data: [{ coherent: true }],
			}),
		);

		await core.navigate("/coherence");

		expect(state_during_callback).not.toBeNull();
		const state = state_during_callback as any;
		expect(state.matches.map((m: any) => m.pattern)).toEqual(["/coherence"]);
		expect(route_update).toMatchObject({
			reason: "navigation",
			route: {
				href: `${window.location.origin}/coherence`,
			},
			previous_route: {
				href: `${window.location.origin}/`,
			},
		});
	});

	it("commit is called before onRouteUpdate fires", async () => {
		seed_payload();
		const order: string[] = [];
		const commit = vi.fn((client_commit: ClientCommit): void => {
			if (client_commit.route_render) {
				order.push("commit");
			}
		});
		const core_res = create_client_core({}, commit, t_opts());
		if (!core_res.ok) {
			throw new Error(`create_client_core failed with error: ${core_res.err}`);
		}
		const core = core_res.val;

		await core.boot({
			onRouteUpdate: () => {
				return order.push("onRouteUpdate");
			},
		});
		order.length = 0;

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({ matched_patterns: ["/order"] }),
		);

		await core.navigate("/order");

		expect(order).toEqual(["commit", "onRouteUpdate"]);
	});

	it("fires an initial route update", async () => {
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core({}, commit, t_opts());
		if (!core_res.ok) {
			throw new Error(`create_client_core failed with error: ${core_res.err}`);
		}
		const core = core_res.val;

		let route_update: unknown = null;
		await core.boot({
			onRouteUpdate: (route, previous_route, reason) => {
				route_update = { route, previous_route, reason };
			},
		});

		expect(route_update).toMatchObject({
			reason: "boot",
			route: {
				href: `${window.location.origin}/`,
			},
			previous_route: null,
		});
	});
});

/////////////////////////////////////////////////////////////////////
/////// Route update blocked by client loaders
/////////////////////////////////////////////////////////////////////
