// @vitest-environment jsdom

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
	SEARCH_PARAM_SCHEMA_BOOL,
	SEARCH_PARAM_SCHEMA_NUMBER,
	SEARCH_PARAM_SCHEMA_STRING,
} from "../../kit/json/search_param_parser.ts";
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
	tick,
} from "./___ccc_test_helpers.ts";
import { BUILD_ID_HEADER, X_VORMA_BUILD_SKEW } from "./constants.ts";
import {
	REVALIDATION_DEBOUNCE_MS,
	apply_scroll,
	create_client_core,
	type ClientCommit,
	type ClientCore,
	type WorkIndicatorOptions,
} from "./create_client_core.ts";

register_ccc_lifecycle(beforeEach, afterEach);

const t_opts = () => {
	return {
		hard_redirect: vi.fn(),
		reload: vi.fn(),
		scroll_to: vi.fn(),
	};
};

async function wait_until(
	check: () => boolean,
	message: string,
): Promise<void> {
	for (let i = 0; i < 50; i++) {
		if (check()) {
			return;
		}
		await tick();
		await new Promise((resolve) => {
			return setTimeout(resolve, 0);
		});
	}
	throw new Error(message);
}

/////////////////////////////////////////////////////////////////////
/////// apply_scroll
/////////////////////////////////////////////////////////////////////

describe("apply_scroll", () => {
	it("scrolls to element by decoded hash ID", () => {
		const el = document.createElement("div");
		el.id = "my-section";
		el.scrollIntoView = vi.fn();
		document.body.appendChild(el);

		apply_scroll({ hash: "#my-section" }, t_opts());

		expect(
			// oxlint-disable-next-line unbound-method
			el.scrollIntoView,
		).toHaveBeenCalled();
	});

	it("decodes URI-encoded hash", () => {
		const el = document.createElement("div");
		el.id = "hello world";
		el.scrollIntoView = vi.fn();
		document.body.appendChild(el);

		apply_scroll({ hash: "#hello%20world" }, t_opts());

		expect(
			// oxlint-disable-next-line unbound-method
			el.scrollIntoView,
		).toHaveBeenCalled();
	});

	it("falls back to raw ID when decode fails", () => {
		const el = document.createElement("div");
		el.id = "%E0%A4%A";
		el.scrollIntoView = vi.fn();
		document.body.appendChild(el);

		apply_scroll({ hash: "#%E0%A4%A" }, t_opts());

		expect(
			// oxlint-disable-next-line unbound-method
			el.scrollIntoView,
		).toHaveBeenCalled();
	});

	it("handles hash without leading #", () => {
		const el = document.createElement("div");
		el.id = "no-hash";
		el.scrollIntoView = vi.fn();
		document.body.appendChild(el);

		apply_scroll({ hash: "no-hash" }, t_opts());

		expect(
			// oxlint-disable-next-line unbound-method
			el.scrollIntoView,
		).toHaveBeenCalled();
	});

	it("calls scroll_to for coordinate scroll", () => {
		const opts = t_opts();

		apply_scroll({ x: 100, y: 200 }, opts);

		expect(opts.scroll_to).toHaveBeenCalledWith(100, 200);
	});

	it("does nothing for undefined scroll", () => {
		const opts = t_opts();

		apply_scroll(undefined, opts);

		expect(opts.scroll_to).not.toHaveBeenCalled();
	});
});

/////////////////////////////////////////////////////////////////////
/////// Boot lifecycle
/////////////////////////////////////////////////////////////////////

describe("boot lifecycle", () => {
	it("parses #vorma-data-json script element for initial payload", async () => {
		seed_payload({
			MatchedPatterns: ["/"],
			LoadersData: [{ root: true }],
		});
		const commit = vi.fn();
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.boot({});

		expect(has_route_render_commit(commit)).toBe(true);
		const state = route_render_commit_at(commit, 0);
		expect(state.entries).toHaveLength(1);
		expect(state.entries[0].pattern).toBe("/");
		expect(state.entries[0].loader_data).toEqual({ root: true });
	});

	it("parses matched input from initial search schemas", async () => {
		window.history.replaceState(
			{},
			"",
			"/users?page=2&active=true&tags=a&tags=b",
		);
		seed_payload({
			MatchedPatterns: ["/users"],
			LoadersData: [{ users: true }],
			SearchSchemas: [
				{
					active: SEARCH_PARAM_SCHEMA_BOOL,
					page: SEARCH_PARAM_SCHEMA_NUMBER,
					tags: [SEARCH_PARAM_SCHEMA_STRING],
				},
			],
		});
		const commit = vi.fn();
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.boot({});

		const state = route_render_commit_at(commit, 0);
		expect(state.entries[0].input).toEqual({
			active: true,
			page: 2,
			tags: ["a", "b"],
		});
	});

	it("returns err when data script element is missing", async () => {
		const commit = vi.fn();
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		expect(core_res.ok).toBe(true);
		if (!core_res.ok) {
			throw new Error("unexpected");
		}
		const render_res = await core_res.val.boot({});
		expect(render_res.ok).toBe(false);
	});

	it("calls render callback during boot", async () => {
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;
		const render = vi.fn();

		await core.boot({ render });

		expect(render).toHaveBeenCalledTimes(1);
	});

	it("passes refresh scroll as initial commit scroll intent", async () => {
		sessionStorage.setItem(
			"vorma-scroll-state-reload",
			JSON.stringify({
				x: 55,
				y: 77,
				unix: Date.now(),
				href: window.location.href,
			}),
		);
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.boot({});

		const scroll_intent = route_render_scroll_intent_at(commit, 0);
		expect(scroll_intent?.scroll).toEqual({ x: 55, y: 77 });
		sessionStorage.removeItem("vorma-scroll-state-reload");
	});

	it("passes current hash as initial commit scroll intent", async () => {
		window.history.replaceState({}, "", "/page#section");
		seed_payload({ MatchedPatterns: ["/page"] });
		const commit = vi.fn();
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.boot({});

		const scroll_intent = route_render_scroll_intent_at(commit, 0);
		expect(scroll_intent?.scroll).toEqual({ hash: "#section" });
	});

	it("allows initial client loaders to submit API actions", async () => {
		let core: any;

		vi.doMock("/mod-boot-action.js", () => {
			return {
				default: {
					pattern: "/boot-action",
					component: () => {
						return null;
					},
					client_loader: async () => {
						return core.submit_inner(
							"/api/some-api",
							{ method: "GET" },
							{ revalidate: false },
						);
					},
				},
			};
		});

		seed_payload({
			MatchedPatterns: ["/boot-action"],
			ImportURLs: ["/mod-boot-action.js"],
		});
		const commit = vi.fn();
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		core = core_res.val;

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			new Response(JSON.stringify({ ok: true }), {
				status: 200,
				headers: { "Content-Type": "application/json" },
			}),
		);

		await core.boot({});

		const state = route_render_commit_at(commit, 0);
		expect(state.entries[0].client_loader_data.success).toBe(true);
		expect(state.entries[0].client_loader_data.data).toEqual({ ok: true });
	});

	it("does not deadlock initial client loaders awaiting submit revalidation", async () => {
		let core: any;

		vi.doMock("/mod-boot-submit.js", () => {
			return {
				default: {
					pattern: "/boot-submit",
					component: () => {
						return null;
					},
					client_loader: async () => {
						const result = await core.submit_inner("/api/save", {
							method: "POST",
						});
						return result.revalidationPromise;
					},
				},
			};
		});

		seed_payload({
			MatchedPatterns: ["/boot-submit"],
			ImportURLs: ["/mod-boot-submit.js"],
		});
		const commit = vi.fn();
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		core = core_res.val;

		vi.spyOn(globalThis, "fetch")
			.mockResolvedValueOnce(
				new Response(JSON.stringify({ saved: true }), {
					status: 200,
					headers: { "Content-Type": "application/json" },
				}),
			)
			.mockResolvedValueOnce(route_response());

		await core.boot({});
		await tick();

		const state = route_render_commit_at(commit, 0);
		expect(state.entries[0].client_loader_data).toEqual({ ok: true });
		expect(globalThis.fetch).toHaveBeenCalledTimes(2);
	});

	it("allows initial client loaders to read router data", async () => {
		let core: any;

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

		vi.doMock("/mod-boot-router-data.js", () => {
			return {
				default: {
					pattern: "/boot-router-data",
					component: () => {
						return null;
					},
					client_loader: async () => {
						return core.getRouteState();
					},
				},
			};
		});

		seed_payload({
			MatchedPatterns: ["/", "/boot-router-data"],
			LoadersData: [{ root: true }, { route: true }],
			ImportURLs: ["/mod-root.js", "/mod-boot-router-data.js"],
		});
		const commit = vi.fn();
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		core = core_res.val;

		await core.boot({});

		const state = route_render_commit_at(commit, 0);
		expect(state.entries[1].client_loader_data).toEqual({
			href: window.location.href,
			historyState: undefined,
			clientBuildID: "build-1",
			splatValues: [],
			params: {},
			matches: [
				{
					pattern: "/",
					input: {},
					loaderData: { root: true },
					clientLoaderData: undefined,
				},
				{
					pattern: "/boot-router-data",
					input: {},
					loaderData: { route: true },
					clientLoaderData: undefined,
				},
			],
			error: null,
		});
	});

	it("extracts initial build ID from data script", async () => {
		seed_payload({ ClientBuildID: "initial-build" });
		const commit = vi.fn();
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.boot({});

		expect(core.getClientBuildID()).toBe("initial-build");
	});
});

/////////////////////////////////////////////////////////////////////
/////// Title entity decoding
/////////////////////////////////////////////////////////////////////

describe("title entity decoding", () => {
	it("decodes HTML entities in title during boot", async () => {
		seed_payload({
			Title: { dangerousInnerHTML: "A &amp; B" },
		});
		const commit = vi.fn();
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.boot({});

		expect(document.title).toBe("A & B");
	});

	it("decodes multiple HTML entities in title", async () => {
		seed_payload({
			Title: {
				dangerousInnerHTML: "&lt;Title&gt; &amp; &quot;More&quot;",
			},
		});
		const commit = vi.fn();
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.boot({});

		expect(document.title).toBe('<Title> & "More"');
	});

	it("handles empty title dangerousInnerHTML", async () => {
		seed_payload({
			Title: { dangerousInnerHTML: "" },
		});
		const commit = vi.fn();
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.boot({});

		expect(document.title).toBe("");
	});
});

/////////////////////////////////////////////////////////////////////
/////// Navigation flow
/////////////////////////////////////////////////////////////////////

describe("navigation flow", () => {
	it("fetches, decodes payload, and commits state on navigate", async () => {
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.boot({});
		commit.mockClear();

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({
				MatchedPatterns: ["/about"],
				LoadersData: [{ page: "about" }],
			}),
		);

		await core.navigate("/about");

		expect(has_route_render_commit(commit)).toBe(true);
		const state = route_render_commit_at(commit, 0);
		expect(state.entries).toHaveLength(1);
		expect(state.entries[0].pattern).toBe("/about");
		expect(state.entries[0].loader_data).toEqual({ page: "about" });
	});

	it("parses matched input from navigation search schemas", async () => {
		const { core, commit } = await setup();
		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({
				MatchedPatterns: ["/users"],
				LoadersData: [{ users: true }],
				SearchSchemas: [
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
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.boot({});
		commit.mockClear();

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({ MatchedPatterns: ["/page"] }),
		);

		await core.navigate("/page");

		expect(has_route_render_commit(commit)).toBe(true);
		const scroll_intent = route_render_scroll_intent_at(commit, 0);
		expect(scroll_intent?.scroll).toEqual({ x: 0, y: 0 });
	});

	it("uses view transitions for navigations when enabled", async () => {
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
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
			route_response({ MatchedPatterns: ["/page"] }),
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
		call(0).resolve(route_response({ MatchedPatterns: ["/page"] }));

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
				MatchedPatterns: ["/first"],
				LoadersData: [{ page: "first" }],
			}),
		);
		await wait_until(() => {
			return transitions.length === 1;
		}, "first view transition did not start");

		const second = core.navigate("/second");
		await wait_for(2);
		call(1).resolve(
			route_response({
				MatchedPatterns: ["/second"],
				LoadersData: [{ page: "second" }],
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
		expect(state.entries[0].loader_data).toEqual({ page: "second" });
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
				MatchedPatterns: ["/", "/current"],
				LoadersData: [{ root: "old" }, { page: "current" }],
				ImportURLs: ["/root.js", "/current.js"],
			},
		});
		const { call, wait_for } = mock_fetch();

		const nav = core.navigate("/next", { state: { via: "test" } });
		await wait_for(1);
		call(0).resolve(
			route_response({
				MatchedPatterns: ["/", "/next"],
				LoadersData: [{ root: "new" }, { page: "next" }],
				ImportURLs: ["/root.js", "/next.js"],
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
		expect(calls).toEqual([
			"root_start",
			"current_start",
			"root_end",
			"current_end",
		]);
		expect(route_render_commit_count(commit)).toBe(1);
		expect(
			route_render_commit_at(commit, 0).entries[1].loader_data,
		).toEqual({
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
				MatchedPatterns: ["/current"],
				LoadersData: [{ page: "current" }],
				ImportURLs: ["/current.js"],
			},
		});
		const { call, wait_for } = mock_fetch();

		const nav = core.navigate("/next");
		await wait_for(1);
		call(0).resolve(
			route_response({
				MatchedPatterns: ["/next"],
				LoadersData: [{ page: "next" }],
				ImportURLs: ["/next.js"],
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
		expect(calls).toEqual([
			"yield_start",
			"commit_start",
			"yield_end",
			"commit_end",
		]);
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
				MatchedPatterns: ["/current"],
				LoadersData: [{ page: "current" }],
				ImportURLs: ["/current.js"],
			},
		});
		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({
				MatchedPatterns: ["/next"],
				LoadersData: [{ page: "next" }],
				ImportURLs: ["/next.js"],
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
				MatchedPatterns: ["/current"],
				LoadersData: [{ page: "current" }],
				ImportURLs: ["/current.js"],
			},
		});
		const { call, wait_for } = mock_fetch();

		const first = core.navigate("/first");
		await wait_for(1);
		call(0).resolve(
			route_response({
				MatchedPatterns: ["/first"],
				LoadersData: [{ page: "first" }],
			}),
		);
		await hook_started.promise;

		const second = core.navigate("/second");
		await wait_for(2);
		call(1).resolve(
			route_response({
				MatchedPatterns: ["/second"],
				LoadersData: [{ page: "second" }],
			}),
		);

		await expect(first).resolves.toEqual({ didNavigate: false });
		await expect(second).resolves.toEqual({ didNavigate: true });

		expect(aborted).toBe(true);
		expect(route_render_commit_count(commit)).toBe(1);
		expect(
			route_render_commit_at(commit, 0).entries[0].loader_data,
		).toEqual({
			page: "second",
		});
	});
});

/////////////////////////////////////////////////////////////////////
/////// Client loaders
/////////////////////////////////////////////////////////////////////

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
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.boot({});

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({
				MatchedPatterns: ["/parent", "/parent/child"],
				LoadersData: [{ p: 1 }, { c: 2 }],
				ImportURLs: ["/mod-a.js", "/mod-b.js"],
			}),
		);

		await core.navigate("/parent/child");

		expect(call_order.indexOf("a_start")).toBeLessThan(
			call_order.indexOf("a_end"),
		);
		expect(call_order.indexOf("b_start")).toBeLessThan(
			call_order.indexOf("b_end"),
		);
		expect(call_order.indexOf("a_start")).toBeLessThan(
			call_order.indexOf("b_end"),
		);
		expect(call_order.indexOf("b_start")).toBeLessThan(
			call_order.indexOf("a_end"),
		);
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
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.boot({});

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({
				MatchedPatterns: ["/", "/users/:id"],
				LoadersData: [{ session: "abc" }, { name: "Ada" }],
				ImportURLs: ["/mod-root.js", "/mod-users.js"],
				Params: { id: "42" },
			}),
		);

		await core.navigate("/users/42");

		expect(captured_server_data).toEqual({
			clientBuildID: "build-1",
			matches: [
				{
					pattern: "/",
					input: {},
					loaderData: { session: "abc" },
				},
				{
					pattern: "/users/:id",
					input: {},
					loaderData: { name: "Ada" },
				},
			],
			outermostServerError: null,
			loaderData: { name: "Ada" },
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
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.boot({});

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({
				MatchedPatterns: ["/", "/users/:id"],
				LoadersData: [{ session: "abc" }, { name: "Ada" }],
				ImportURLs: ["/mod-root.js", "/mod-users.js"],
				Params: { id: "42" },
				SearchSchemas: [
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
			clientBuildID: "build-1",
			matches: [
				{
					pattern: "/",
					input: {},
					loaderData: { session: "abc" },
				},
				{
					pattern: "/users/:id",
					input: { page: 2 },
					loaderData: { name: "Ada" },
				},
			],
			outermostServerError: null,
			loaderData: { name: "Ada" },
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
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.boot({});

		// First: navigate to /slow fully to register the client loader
		vi.spyOn(globalThis, "fetch")
			.mockResolvedValueOnce(
				route_response({
					MatchedPatterns: ["/slow"],
					LoadersData: [{ v: 1 }],
					ImportURLs: ["/mod-slow.js"],
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

	it("failure in one loader cascades abort to subsequent loaders", async () => {
		let second_signal: AbortSignal | null = null;

		vi.doMock("/mod-fail.js", () => {
			return {
				default: {
					pattern: "/fail-parent",
					component: () => {
						return null;
					},
					client_loader: async () => {
						throw new Error("loader failed");
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
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.boot({});

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({
				MatchedPatterns: ["/fail-parent", "/fail-parent/child"],
				LoadersData: [{ p: 1 }, { c: 2 }],
				ImportURLs: ["/mod-fail.js", "/mod-child.js"],
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
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.boot({});
		commit.mockClear();

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({
				MatchedPatterns: ["/err-route"],
				LoadersData: [{ v: 1 }],
				ImportURLs: ["/mod-err.js"],
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
						return { enhanced: true, original: server.loaderData };
					},
				},
			};
		});

		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.boot({});
		commit.mockClear();

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({
				MatchedPatterns: ["/cl-route"],
				LoadersData: [{ raw: "data" }],
				ImportURLs: ["/mod-cl.js"],
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
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.boot({});
		commit.mockClear();

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({
				MatchedPatterns: ["/abort-route"],
				LoadersData: [{ v: 1 }],
				ImportURLs: ["/mod-abort.js"],
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

describe("build ID", () => {
	it("keeps active build ID stable when a route response has a newer build", async () => {
		seed_payload({ ClientBuildID: "build-1" });
		const commit = vi.fn();
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.boot({});

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({}, "build-2"),
		);

		await core.navigate("/page");

		expect(core.getClientBuildID()).toBe("build-1");
	});

	it("fires onBuildSkewDetected when a route response has a newer build", async () => {
		seed_payload({ ClientBuildID: "build-1" });
		const commit = vi.fn();
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		const on_build_skew = vi.fn();
		await core.boot({ onBuildSkewDetected: on_build_skew });

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({}, "build-2"),
		);

		await core.navigate("/page");

		expect(on_build_skew).toHaveBeenCalledWith(
			expect.objectContaining({
				activeClientBuildID: "build-1",
				serverBuildID: "build-2",
				defaultBehavior: "notifyOnly",
				triggeringResponse: expect.objectContaining({
					kind: "route",
					trigger: "navigation",
					requestedHref: `${window.location.origin}/page`,
					status: 200,
					ok: true,
				}),
				currentRouteState: expect.objectContaining({
					clientBuildID: "build-1",
					href: `${window.location.origin}/`,
				}),
				currentWorkState: expect.objectContaining({
					navigation: expect.objectContaining({
						href: `${window.location.origin}/page`,
					}),
				}),
			}),
		);
	});

	it("does not fire notification when build ID is same", async () => {
		seed_payload({ ClientBuildID: "build-1" });
		const commit = vi.fn();
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		const on_build_skew = vi.fn();
		await core.boot({ onBuildSkewDetected: on_build_skew });

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({}, "build-1"),
		);

		await core.navigate("/page");

		expect(on_build_skew).not.toHaveBeenCalled();
	});
});

/////////////////////////////////////////////////////////////////////
/////// Work integration
/////////////////////////////////////////////////////////////////////

describe("work integration", () => {
	it("onWorkUpdate receives current work state", async () => {
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		const work_updates: any[] = [];
		await core.boot({
			onWorkUpdate: (work) => {
				return work_updates.push({ ...work });
			},
		});

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(route_response());

		await core.navigate("/page");

		expect(work_updates.length).toBeGreaterThan(0);
		const navigating = work_updates.find((work) => {
			return work.navigation !== null;
		});
		expect(navigating).toBeDefined();
		expect(navigating).toHaveProperty("revalidation");
		expect(navigating).toHaveProperty("apiRequests");
	});

	it("getWorkState returns current snapshot", async () => {
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.boot({});

		const idle = core.getWorkState();
		expect(idle).toEqual({
			navigation: null,
			revalidation: null,
			prefetch: null,
			apiRequests: [],
		});
	});
});

/////////////////////////////////////////////////////////////////////
/////// Work indicators
/////////////////////////////////////////////////////////////////////

describe("work indicators", () => {
	afterEach(() => {
		vi.useRealTimers();
	});

	type WorkIndicatorRenderEvent = {
		kind: "start" | "stop";
		was_visible: boolean;
	};

	type WorkIndicatorRenderer = WorkIndicatorOptions & {
		events: WorkIndicatorRenderEvent[];
		force_visible: () => void;
		is_visible: () => boolean;
		reset_events: () => void;
		start: ReturnType<typeof vi.fn>;
		stop: ReturnType<typeof vi.fn>;
	};

	function make_work_indicator_renderer(
		initial_visible = false,
	): WorkIndicatorRenderer {
		let visible = initial_visible;
		const events: WorkIndicatorRenderEvent[] = [];
		const start = vi.fn(() => {
			events.push({ kind: "start", was_visible: visible });
			visible = true;
		});
		const stop = vi.fn(() => {
			events.push({ kind: "stop", was_visible: visible });
			visible = false;
		});
		return {
			events,
			force_visible: () => {
				visible = true;
			},
			is_visible: () => {
				return visible;
			},
			reset_events: () => {
				events.length = 0;
				start.mockClear();
				stop.mockClear();
			},
			start,
			startDelayMS: 1,
			stop,
			stopDelayMS: 1,
		};
	}

	function expect_work_indicator_idle(
		core: ClientCore,
		config: WorkIndicatorRenderer,
	): void {
		expect(core.workIndicator.isActive()).toBe(false);
		expect(core.getWorkState()).toEqual({
			apiRequests: [],
			navigation: null,
			prefetch: null,
			revalidation: null,
		});
		expect(config.is_visible()).toBe(false);
	}

	function expect_stop_after_last_start(config: WorkIndicatorRenderer): void {
		let last_start_index = -1;
		let last_stop_index = -1;
		for (let i = 0; i < config.events.length; i++) {
			const event = config.events[i];
			if (event?.kind === "start") {
				last_start_index = i;
			}
			if (event?.kind === "stop") {
				last_stop_index = i;
			}
		}
		expect(last_start_index).toBeGreaterThanOrEqual(0);
		expect(last_stop_index).toBeGreaterThan(last_start_index);
	}

	async function setup_core(
		workIndicator: WorkIndicatorOptions,
	): Promise<ClientCore> {
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		await core_res.val.boot({ workIndicator });
		return core_res.val;
	}

	it("starts and stops around navigation with delays", async () => {
		vi.useFakeTimers();
		const config = {
			start: vi.fn(),
			stop: vi.fn(),
			startDelayMS: 10,
			stopDelayMS: 10,
		};
		const core = await setup_core(config);

		let resolve_fetch!: (r: Response) => void;
		vi.spyOn(globalThis, "fetch").mockImplementation(() => {
			return new Promise((r) => {
				resolve_fetch = r;
			});
		});

		void core.navigate("/page");
		await vi.advanceTimersByTimeAsync(10);

		expect(config.start).toHaveBeenCalled();

		resolve_fetch(route_response());
		await vi.advanceTimersByTimeAsync(10);
		await tick();

		expect(config.stop).toHaveBeenCalled();
	});

	it("respects category skips", async () => {
		vi.useFakeTimers();
		const config = {
			start: vi.fn(),
			stop: vi.fn(),
			skipAPIRequests: true,
			startDelayMS: 1,
			stopDelayMS: 1,
		};
		const core = await setup_core(config);

		vi.spyOn(globalThis, "fetch").mockResolvedValue(
			new Response(JSON.stringify({ ok: true }), {
				status: 200,
				headers: { "Content-Type": "application/json" },
			}),
		);

		await core.submit_inner(
			"/api/action",
			{ method: "POST" },
			{
				revalidate: false,
			},
		);
		await vi.advanceTimersByTimeAsync(10);

		expect(config.start).not.toHaveBeenCalled();
	});

	it("skips work indicator for opted-out submissions", async () => {
		vi.useFakeTimers();
		const config = {
			start: vi.fn(),
			stop: vi.fn(),
			startDelayMS: 1,
			stopDelayMS: 1,
		};
		const core = await setup_core(config);

		vi.spyOn(globalThis, "fetch").mockResolvedValue(
			new Response(JSON.stringify({ ok: true }), {
				status: 200,
				headers: { "Content-Type": "application/json" },
			}),
		);

		await core.submit_inner(
			"/api/action",
			{ method: "POST" },
			{
				revalidate: false,
				skipWorkIndicator: true,
			},
		);
		await vi.advanceTimersByTimeAsync(10);

		expect(config.start).not.toHaveBeenCalled();
	});

	it("skips work indicator for opted-out GET query submissions", async () => {
		vi.useFakeTimers();
		const config = make_work_indicator_renderer();
		const core = await setup_core(config);
		const fetcher = mock_fetch();

		const submit = core.submit_inner("/api/search?q=ada", undefined, {
			apiRouteKind: "query",
			dedupeKey: "search:debug",
			skipWorkIndicator: true,
		});
		await fetcher.wait_for(1);
		await vi.advanceTimersByTimeAsync(1);

		expect(core.getWorkState().apiRequests).toEqual([
			{
				key: "search:debug",
				method: "GET",
				href: "http://localhost:3000/api/search?q=ada",
			},
		]);
		expect(config.start).not.toHaveBeenCalled();
		expect(core.workIndicator.isActive()).toBe(false);

		fetcher.call(0).resolve(
			new Response(JSON.stringify({ ok: true }), {
				status: 200,
				headers: { "Content-Type": "application/json" },
			}),
		);
		const result = await submit;
		await tick();
		await vi.advanceTimersByTimeAsync(1);

		expect(result.success).toBe(true);
		expect(fetcher.calls).toHaveLength(1);
		expect(config.start).not.toHaveBeenCalled();
		expect(config.stop).not.toHaveBeenCalled();
		expect_work_indicator_idle(core, config);
	});

	it("skips work indicator for opted-out mutation revalidation", async () => {
		vi.useFakeTimers();
		const config = {
			start: vi.fn(),
			stop: vi.fn(),
			startDelayMS: 1,
			stopDelayMS: 1,
		};
		const core = await setup_core(config);
		const fetcher = mock_fetch();

		const submit = core.submit_inner(
			"/api/action",
			{ method: "POST" },
			{
				skipWorkIndicator: true,
			},
		);
		await fetcher.wait_for(1);
		await vi.advanceTimersByTimeAsync(1);

		expect(config.start).not.toHaveBeenCalled();

		fetcher.call(0).resolve(
			new Response(JSON.stringify({ ok: true }), {
				status: 200,
				headers: { "Content-Type": "application/json" },
			}),
		);
		const result = await submit;
		await fetcher.wait_for(2);
		await vi.advanceTimersByTimeAsync(1);

		expect(config.start).not.toHaveBeenCalled();

		fetcher.call(1).resolve(route_response());
		await result.revalidationPromise;
		await tick();
		await vi.advanceTimersByTimeAsync(1);

		expect(config.start).not.toHaveBeenCalled();
	});

	it("skips work indicator for opted-out navigations", async () => {
		vi.useFakeTimers();
		const config = {
			start: vi.fn(),
			stop: vi.fn(),
			startDelayMS: 1,
			stopDelayMS: 1,
		};
		const core = await setup_core(config);

		let resolve_fetch!: (r: Response) => void;
		vi.spyOn(globalThis, "fetch").mockImplementation(() => {
			return new Promise((r) => {
				resolve_fetch = r;
			});
		});

		void core.navigate("/quiet-page", {
			skipWorkIndicator: true,
		});
		await vi.advanceTimersByTimeAsync(10);

		expect(config.start).not.toHaveBeenCalled();

		resolve_fetch(route_response());
		await vi.advanceTimersByTimeAsync(10);
		await tick();
	});

	it("overlapping work does not cause start-stop thrash", async () => {
		vi.useFakeTimers();
		const config = {
			start: vi.fn(),
			stop: vi.fn(),
			startDelayMS: 1,
			stopDelayMS: 1,
		};
		const core = await setup_core(config);

		let resolve_first!: (r: Response) => void;
		let resolve_second!: (r: Response) => void;
		let call_count = 0;
		vi.spyOn(globalThis, "fetch").mockImplementation(() => {
			call_count++;
			if (call_count === 1) {
				return new Promise((r) => {
					resolve_first = r;
				});
			}
			return new Promise((r) => {
				resolve_second = r;
			});
		});

		void core.submit_inner(
			"/api/a",
			{ method: "POST" },
			{ revalidate: false },
		);
		void core.submit_inner(
			"/api/b",
			{ method: "POST" },
			{ revalidate: false },
		);
		await vi.advanceTimersByTimeAsync(1);

		expect(config.start).toHaveBeenCalledTimes(1);

		resolve_first(
			new Response(JSON.stringify({}), {
				status: 200,
				headers: { "Content-Type": "application/json" },
			}),
		);
		await vi.advanceTimersByTimeAsync(0);

		// Still one submit in flight, should not stop
		expect(config.stop).not.toHaveBeenCalled();

		resolve_second(
			new Response(JSON.stringify({}), {
				status: 200,
				headers: { "Content-Type": "application/json" },
			}),
		);
		await vi.advanceTimersByTimeAsync(1);
		await tick();

		expect(config.stop).toHaveBeenCalledTimes(1);
	});

	it("clears pending start timer when work finishes before delay", async () => {
		vi.useFakeTimers();
		const config = {
			start: vi.fn(),
			stop: vi.fn(),
			startDelayMS: 100,
			stopDelayMS: 10,
		};
		const core = await setup_core(config);

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(route_response());

		await core.navigate("/fast-page");
		await vi.advanceTimersByTimeAsync(200);

		expect(config.start).not.toHaveBeenCalled();
		expect(config.stop).not.toHaveBeenCalled();
	});

	it("cancels pending stop timer when new work begins", async () => {
		vi.useFakeTimers();
		const config = {
			start: vi.fn(),
			stop: vi.fn(),
			startDelayMS: 1,
			stopDelayMS: 100,
		};
		const core = await setup_core(config);

		let resolve_first!: (r: Response) => void;
		let resolve_second!: (r: Response) => void;
		let call_count = 0;
		vi.spyOn(globalThis, "fetch").mockImplementation(() => {
			call_count++;
			if (call_count === 1) {
				return new Promise((r) => {
					resolve_first = r;
				});
			}
			return new Promise((r) => {
				resolve_second = r;
			});
		});

		void core.navigate("/first");
		await vi.advanceTimersByTimeAsync(1);
		expect(config.start).toHaveBeenCalledTimes(1);

		resolve_first(route_response());
		await vi.advanceTimersByTimeAsync(0);
		await tick();

		void core.navigate("/second");
		await vi.advanceTimersByTimeAsync(100);

		expect(config.stop).not.toHaveBeenCalled();
		expect(config.start).toHaveBeenCalledTimes(1);

		resolve_second(route_response());
		await vi.advanceTimersByTimeAsync(100);
		await tick();

		expect(config.stop).toHaveBeenCalledTimes(1);
	});

	it("tracks app-owned promise work", async () => {
		vi.useFakeTimers();
		const config = {
			start: vi.fn(),
			stop: vi.fn(),
			startDelayMS: 1,
			stopDelayMS: 1,
		};
		const core = await setup_core(config);
		const external_work = deferred<number>();

		const tracked = core.workIndicator.track(external_work.promise);

		expect(core.workIndicator.isActive()).toBe(true);
		await vi.advanceTimersByTimeAsync(1);
		expect(config.start).toHaveBeenCalledTimes(1);

		external_work.resolve(42);
		await expect(tracked).resolves.toBe(42);

		expect(core.workIndicator.isActive()).toBe(false);
		await vi.advanceTimersByTimeAsync(1);
		expect(config.stop).toHaveBeenCalledTimes(1);
	});

	it("keeps app-owned work active after Vorma work settles", async () => {
		vi.useFakeTimers();
		const config = {
			start: vi.fn(),
			stop: vi.fn(),
			startDelayMS: 1,
			stopDelayMS: 1,
		};
		const core = await setup_core(config);
		const external_work = deferred<void>();
		const tracked = core.workIndicator.track(external_work.promise);

		await vi.advanceTimersByTimeAsync(1);
		expect(config.start).toHaveBeenCalledTimes(1);

		let resolve_fetch!: (r: Response) => void;
		vi.spyOn(globalThis, "fetch").mockImplementation(() => {
			return new Promise((r) => {
				resolve_fetch = r;
			});
		});

		const nav = core.navigate("/page");
		await vi.advanceTimersByTimeAsync(1);
		resolve_fetch(route_response());
		await nav;
		await tick();
		await vi.advanceTimersByTimeAsync(1);

		expect(config.stop).not.toHaveBeenCalled();

		external_work.resolve();
		await tracked;
		await vi.advanceTimersByTimeAsync(1);

		expect(config.stop).toHaveBeenCalledTimes(1);
	});

	it("does not stop a visible renderer when boot starts idle", async () => {
		vi.useFakeTimers();
		const config = make_work_indicator_renderer(true);

		await setup_core(config);
		await vi.advanceTimersByTimeAsync(1);

		expect(config.stop).not.toHaveBeenCalled();
		expect(config.is_visible()).toBe(true);
	});

	it("does not stop a visible renderer after skipped Vorma work settles", async () => {
		vi.useFakeTimers();
		const config = make_work_indicator_renderer();
		const core = await setup_core(config);
		await vi.advanceTimersByTimeAsync(1);
		config.reset_events();
		config.force_visible();

		let resolve_fetch!: (r: Response) => void;
		vi.spyOn(globalThis, "fetch").mockImplementation(() => {
			return new Promise((r) => {
				resolve_fetch = r;
			});
		});

		void core.navigate("/quiet-page", {
			skipWorkIndicator: true,
		});
		await vi.advanceTimersByTimeAsync(1);
		resolve_fetch(route_response());
		await vi.advanceTimersByTimeAsync(1);
		await tick();

		expect(config.stop).not.toHaveBeenCalled();
		expect(config.is_visible()).toBe(true);
	});

	it("stops after Vorma-owned navigation aborts", async () => {
		vi.useFakeTimers();
		const config = make_work_indicator_renderer();
		const core = await setup_core(config);
		vi.spyOn(globalThis, "fetch").mockRejectedValueOnce(
			new DOMException("Aborted", "AbortError"),
		);

		await core.navigate("/aborted");
		await vi.advanceTimersByTimeAsync(1);
		await tick();

		expect(config.start).not.toHaveBeenCalled();
		expect_work_indicator_idle(core, config);

		const slow_fetch = mock_fetch();
		const nav = core.navigate("/slow-abort");
		await slow_fetch.wait_for(1);
		await vi.advanceTimersByTimeAsync(1);

		expect(config.start).toHaveBeenCalledTimes(1);
		slow_fetch.call(0).reject(new DOMException("Aborted", "AbortError"));
		await nav;
		await tick();
		await vi.advanceTimersByTimeAsync(1);

		expect_work_indicator_idle(core, config);
		expect_stop_after_last_start(config);
	});

	it("stops after Vorma-owned API request rejects", async () => {
		vi.useFakeTimers();
		const config = make_work_indicator_renderer();
		const core = await setup_core(config);
		const fetcher = mock_fetch();

		const submit = core.submit_inner(
			"/api/failing-action",
			{ method: "POST" },
			{ revalidate: false },
		);
		await fetcher.wait_for(1);
		await vi.advanceTimersByTimeAsync(1);

		expect(config.start).toHaveBeenCalledTimes(1);
		fetcher.call(0).reject(new Error("network down"));
		const result = await submit;
		await tick();
		await vi.advanceTimersByTimeAsync(1);

		expect(result.success).toBe(false);
		expect_work_indicator_idle(core, config);
		expect_stop_after_last_start(config);
	});

	it("stops after revalidation debounce and fetch settle", async () => {
		vi.useFakeTimers();
		const config = make_work_indicator_renderer();
		const core = await setup_core(config);
		const fetcher = mock_fetch();

		const revalidation = core.revalidate();
		await vi.advanceTimersByTimeAsync(1);

		expect(config.start).toHaveBeenCalledTimes(1);
		await vi.advanceTimersByTimeAsync(REVALIDATION_DEBOUNCE_MS);
		await fetcher.wait_for(1);
		fetcher.call(0).resolve(route_response());
		await revalidation;
		await tick();
		await vi.advanceTimersByTimeAsync(1);

		expect_work_indicator_idle(core, config);
		expect_stop_after_last_start(config);
	});

	it("moves a visible Vorma-owned indicator across option replacement", async () => {
		vi.useFakeTimers();
		const first_config = make_work_indicator_renderer();
		const core = await setup_core(first_config);
		const external_work = deferred<void>();
		const tracked = core.workIndicator.track(external_work.promise);

		await vi.advanceTimersByTimeAsync(1);

		expect(first_config.start).toHaveBeenCalledTimes(1);
		expect(first_config.is_visible()).toBe(true);

		const second_config = make_work_indicator_renderer();
		await core.boot({ workIndicator: second_config });
		await vi.advanceTimersByTimeAsync(1);

		expect(first_config.is_visible()).toBe(false);
		expect(first_config.stop).toHaveBeenCalledTimes(1);
		expect(second_config.start).toHaveBeenCalledTimes(1);
		expect(second_config.is_visible()).toBe(true);

		external_work.resolve();
		await tracked;
		await vi.advanceTimersByTimeAsync(1);

		expect_work_indicator_idle(core, second_config);
		expect_stop_after_last_start(first_config);
		expect_stop_after_last_start(second_config);
	});

	it("keeps the renderer reconciled after varied work ordering", async () => {
		vi.useFakeTimers();
		const config = make_work_indicator_renderer();
		const core = await setup_core(config);
		const tracked_work = [
			deferred<void>(),
			deferred<void>(),
			deferred<void>(),
		];

		const tracked = tracked_work.map((work) => {
			return core.workIndicator.track(work.promise);
		});

		let resolve_first_fetch!: (r: Response) => void;
		let resolve_second_fetch!: (r: Response) => void;
		let fetch_count = 0;
		vi.spyOn(globalThis, "fetch").mockImplementation(() => {
			fetch_count++;
			if (fetch_count === 1) {
				return new Promise((r) => {
					resolve_first_fetch = r;
				});
			}
			if (fetch_count === 2) {
				return new Promise((r) => {
					resolve_second_fetch = r;
				});
			}
			return Promise.resolve(
				new Response(JSON.stringify({ ok: true }), {
					status: 200,
					headers: { "Content-Type": "application/json" },
				}),
			);
		});

		const first_nav = core.navigate("/first");
		await vi.advanceTimersByTimeAsync(1);
		void core.submit_inner(
			"/api/action",
			{ method: "POST" },
			{ revalidate: false },
		);
		tracked_work[1]!.resolve();
		await tracked[1];
		resolve_first_fetch(route_response());
		await first_nav;
		await tick();

		const second_nav = core.navigate("/second");
		await vi.advanceTimersByTimeAsync(1);
		tracked_work[0]!.resolve();
		await tracked[0];
		resolve_second_fetch(route_response());
		await second_nav;
		await tick();

		tracked_work[2]!.resolve();
		await tracked[2];
		await vi.advanceTimersByTimeAsync(1);
		await tick();

		expect_work_indicator_idle(core, config);
		expect_stop_after_last_start(config);
	});

	it("does not orphan after mixed Vorma and app work settle", async () => {
		vi.useFakeTimers();
		const config = make_work_indicator_renderer();
		const core = await setup_core(config);
		const fetcher = mock_fetch();
		const external_work = deferred<void>();
		const tracked = core.workIndicator.track(external_work.promise);

		const nav = core.navigate("/chaos-nav");
		await fetcher.wait_for(1);
		await vi.advanceTimersByTimeAsync(1);

		expect(config.start).toHaveBeenCalledTimes(1);

		const submit = core.submit_inner(
			"/api/chaos-action",
			{ method: "POST" },
			{ revalidate: false },
		);
		await fetcher.wait_for(2);
		const revalidation = core.revalidate();

		fetcher.call(0).resolve(route_response());
		await nav;
		await tick();
		await vi.advanceTimersByTimeAsync(1);

		expect(config.is_visible()).toBe(true);

		external_work.resolve();
		await tracked;
		fetcher.call(1).resolve(
			new Response(JSON.stringify({ ok: true }), {
				status: 200,
				headers: { "Content-Type": "application/json" },
			}),
		);
		await submit;
		await vi.advanceTimersByTimeAsync(REVALIDATION_DEBOUNCE_MS);
		await fetcher.wait_for(3);

		expect(config.is_visible()).toBe(true);

		fetcher.call(2).resolve(route_response());
		await revalidation;
		await tick();
		await vi.advanceTimersByTimeAsync(1);

		expect_work_indicator_idle(core, config);
		expect_stop_after_last_start(config);
	});
});

/////////////////////////////////////////////////////////////////////
/////// Focus-triggered revalidation
/////////////////////////////////////////////////////////////////////

describe("focus-triggered revalidation", () => {
	afterEach(() => {
		vi.useRealTimers();
	});

	it("fires revalidate after stale time elapsed", async () => {
		vi.useFakeTimers();
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.boot({
			revalidateOnWindowFocus: { staleTimeMS: 100 },
		});

		vi.spyOn(globalThis, "fetch").mockResolvedValue(route_response());

		await vi.advanceTimersByTimeAsync(100);
		window.dispatchEvent(new Event("focus"));
		await vi.advanceTimersByTimeAsync(100);

		expect(globalThis.fetch).toHaveBeenCalled();
	});

	it("reports window focus as the build skew revalidation reason", async () => {
		vi.useFakeTimers();
		seed_payload();
		const commit = vi.fn();
		const on_build_skew = vi.fn();
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.boot({
			onBuildSkewDetected: on_build_skew,
			revalidateOnWindowFocus: { staleTimeMS: 100 },
		});

		vi.spyOn(globalThis, "fetch").mockResolvedValue(
			new Response("", {
				headers: {
					[BUILD_ID_HEADER]: "build-2",
					[X_VORMA_BUILD_SKEW]: "1",
				},
			}),
		);

		await vi.advanceTimersByTimeAsync(100);
		window.dispatchEvent(new Event("focus"));
		await vi.advanceTimersByTimeAsync(100 + REVALIDATION_DEBOUNCE_MS);
		await vi.advanceTimersByTimeAsync(0);

		expect(on_build_skew).toHaveBeenCalledWith(
			expect.objectContaining({
				activeClientBuildID: "build-1",
				serverBuildID: "build-2",
				defaultBehavior: "dropResponse",
				triggeringResponse: expect.objectContaining({
					kind: "route",
					trigger: "revalidation",
					revalidationReason: "windowFocus",
				}),
			}),
		);
	});

	it("does not fire when stale time has not elapsed", async () => {
		vi.useFakeTimers();
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.boot({
			revalidateOnWindowFocus: { staleTimeMS: 5000 },
		});

		vi.spyOn(globalThis, "fetch").mockResolvedValue(route_response());

		await vi.advanceTimersByTimeAsync(100);
		window.dispatchEvent(new Event("focus"));
		await vi.advanceTimersByTimeAsync(0);

		expect(globalThis.fetch).not.toHaveBeenCalled();
	});

	it("does not listen when disabled", async () => {
		vi.useFakeTimers();
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.boot({ revalidateOnWindowFocus: false });

		vi.spyOn(globalThis, "fetch").mockResolvedValue(route_response());

		await vi.advanceTimersByTimeAsync(200);
		window.dispatchEvent(new Event("focus"));
		await vi.advanceTimersByTimeAsync(100);

		expect(globalThis.fetch).not.toHaveBeenCalled();
	});

	it("stale time resets after successful navigation", async () => {
		vi.useFakeTimers();
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.boot({
			revalidateOnWindowFocus: { staleTimeMS: 1000 },
		});

		vi.spyOn(globalThis, "fetch").mockResolvedValue(route_response());

		// Navigate resets last_activity_ts
		await core.navigate("/page");

		// Advance less than stale time after navigation
		await vi.advanceTimersByTimeAsync(500);
		const fetch_count_before = (globalThis.fetch as any).mock.calls.length;
		window.dispatchEvent(new Event("focus"));
		await vi.advanceTimersByTimeAsync(100);

		// No additional fetch for revalidation (only the navigate fetch)
		expect((globalThis.fetch as any).mock.calls.length).toBe(
			fetch_count_before,
		);
	});

	it("does not fire during active navigation", async () => {
		vi.useFakeTimers();
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.boot({
			revalidateOnWindowFocus: { staleTimeMS: 0 },
		});

		let resolve_nav!: (r: Response) => void;
		vi.spyOn(globalThis, "fetch").mockImplementation(() => {
			return new Promise((r) => {
				resolve_nav = r;
			});
		});

		void core.navigate("/in-flight");
		await vi.advanceTimersByTimeAsync(200);

		const fetch_count = (globalThis.fetch as any).mock.calls.length;
		window.dispatchEvent(new Event("focus"));
		await vi.advanceTimersByTimeAsync(100);

		expect((globalThis.fetch as any).mock.calls.length).toBe(fetch_count);

		resolve_nav(route_response());
		await vi.advanceTimersByTimeAsync(100);
	});

	it("does not fire during active submission", async () => {
		vi.useFakeTimers();
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.boot({
			revalidateOnWindowFocus: { staleTimeMS: 0 },
		});

		let resolve_submit!: (r: Response) => void;
		vi.spyOn(globalThis, "fetch").mockImplementation(() => {
			return new Promise((r) => {
				resolve_submit = r;
			});
		});

		void core.submit_inner(
			"/api/action",
			{ method: "POST" },
			{ revalidate: false },
		);
		await vi.advanceTimersByTimeAsync(200);

		const fetch_count = (globalThis.fetch as any).mock.calls.length;
		window.dispatchEvent(new Event("focus"));
		await vi.advanceTimersByTimeAsync(100);

		expect((globalThis.fetch as any).mock.calls.length).toBe(fetch_count);

		resolve_submit(
			new Response(JSON.stringify({ ok: true }), {
				status: 200,
				headers: { "Content-Type": "application/json" },
			}),
		);
		await vi.advanceTimersByTimeAsync(100);
	});

	it("does not fire during active revalidation", async () => {
		vi.useFakeTimers();
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.boot({
			revalidateOnWindowFocus: { staleTimeMS: 0 },
		});

		let resolve_rev!: (r: Response) => void;
		vi.spyOn(globalThis, "fetch").mockImplementation(() => {
			return new Promise((r) => {
				resolve_rev = r;
			});
		});

		void core.revalidate();
		await vi.advanceTimersByTimeAsync(200);

		const fetch_count = (globalThis.fetch as any).mock.calls.length;
		window.dispatchEvent(new Event("focus"));
		await vi.advanceTimersByTimeAsync(100);

		expect((globalThis.fetch as any).mock.calls.length).toBe(fetch_count);

		resolve_rev(route_response());
		await vi.advanceTimersByTimeAsync(100);
	});

	it("does not advance stale-time timestamp for aborted navigations", async () => {
		vi.useFakeTimers();
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.boot({
			revalidateOnWindowFocus: { staleTimeMS: 100 },
		});

		vi.spyOn(globalThis, "fetch")
			.mockRejectedValueOnce(new DOMException("Aborted", "AbortError"))
			.mockResolvedValue(route_response());

		await vi.advanceTimersByTimeAsync(90);
		await core.navigate("/aborted");

		await vi.advanceTimersByTimeAsync(20);
		window.dispatchEvent(new Event("focus"));
		await vi.advanceTimersByTimeAsync(100);

		expect((globalThis.fetch as any).mock.calls.length).toBe(2);
	});

	it("does not reset stale-time for hash-only navigations", async () => {
		vi.useFakeTimers();
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.boot({
			revalidateOnWindowFocus: { staleTimeMS: 100 },
		});

		vi.spyOn(globalThis, "fetch").mockResolvedValue(route_response());

		await vi.advanceTimersByTimeAsync(90);
		await core.navigate("/#section");
		await vi.advanceTimersByTimeAsync(0);

		await vi.advanceTimersByTimeAsync(20);
		window.dispatchEvent(new Event("focus"));
		await vi.advanceTimersByTimeAsync(100);

		expect((globalThis.fetch as any).mock.calls.length).toBe(1);
	});
});

/////////////////////////////////////////////////////////////////////
/////// HMR
/////////////////////////////////////////////////////////////////////

describe("HMR", () => {
	it("updates module cache on HMR callback", async () => {
		vi.doMock("/hmr-mod.js", () => {
			return {
				default: {
					pattern: "/hmr",
					component: () => {
						return "v1";
					},
				},
			};
		});

		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.boot({});

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({
				MatchedPatterns: ["/hmr"],
				LoadersData: [{ v: 1 }],
				ImportURLs: ["/hmr-mod.js"],
			}),
		);

		await core.navigate("/hmr");
		commit.mockClear();

		const new_mod = {
			default: {
				pattern: "/hmr",
				component: () => {
					return "v2";
				},
			},
		};

		await window.__vorma_hmr_route_update?.("/hmr-mod.js", new_mod);

		expect(has_route_render_commit(commit)).toBe(true);
		const state = route_render_commit_at(commit, 0);
		expect(state.entries[0].module).toBe(new_mod);
	});

	it("re-runs client loader only for opted-in patterns", async () => {
		let loader_run_count = 0;

		vi.doMock("/hmr-cl-mod.js", () => {
			return {
				default: {
					pattern: "/hmr-cl",
					component: () => {
						return null;
					},
					client_loader: async ({ serverPromise }: any) => {
						loader_run_count++;
						await serverPromise;
						return { run: loader_run_count };
					},
				},
			};
		});

		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.boot({});

		// defineView with runClientLoaderOnHMR: true
		core.defineView({
			pattern: "/hmr-cl",
			component: () => {
				return null;
			},
			clientLoader: async ({ serverPromise }: any) => {
				loader_run_count++;
				await serverPromise;
				return { run: loader_run_count };
			},
			runClientLoaderOnHMR: true,
		});

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({
				MatchedPatterns: ["/hmr-cl"],
				LoadersData: [{ v: 1 }],
				ImportURLs: ["/hmr-cl-mod.js"],
			}),
		);

		await core.navigate("/hmr-cl");
		const runs_after_nav = loader_run_count;
		commit.mockClear();

		const new_mod = {
			default: {
				pattern: "/hmr-cl",
				component: () => {
					return null;
				},
				client_loader: async ({ serverPromise }: any) => {
					loader_run_count++;
					await serverPromise;
					return { run: loader_run_count };
				},
			},
		};

		await window.__vorma_hmr_route_update?.("/hmr-cl-mod.js", new_mod);

		expect(loader_run_count).toBeGreaterThan(runs_after_nav);
	});

	it("does not re-run client loader when not opted in", async () => {
		let loader_run_count = 0;

		vi.doMock("/hmr-no-rerun.js", () => {
			return {
				default: {
					pattern: "/hmr-no-rerun",
					component: () => {
						return null;
					},
					client_loader: async ({ serverPromise }: any) => {
						loader_run_count++;
						await serverPromise;
						return { run: loader_run_count };
					},
				},
			};
		});

		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.boot({});

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({
				MatchedPatterns: ["/hmr-no-rerun"],
				LoadersData: [{ v: 1 }],
				ImportURLs: ["/hmr-no-rerun.js"],
			}),
		);

		await core.navigate("/hmr-no-rerun");
		const runs_after_nav = loader_run_count;
		commit.mockClear();

		const new_mod = {
			default: {
				pattern: "/hmr-no-rerun",
				component: () => {
					return null;
				},
				client_loader: async ({ serverPromise }: any) => {
					loader_run_count++;
					await serverPromise;
					return { run: loader_run_count };
				},
			},
		};

		await window.__vorma_hmr_route_update?.("/hmr-no-rerun.js", new_mod);

		expect(loader_run_count).toBe(runs_after_nav);
	});

	it("ignores non-matching module paths", async () => {
		vi.doMock("/hmr-match.js", () => {
			return {
				default: {
					pattern: "/hmr-match",
					component: () => {
						return null;
					},
				},
			};
		});

		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.boot({});

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({
				MatchedPatterns: ["/hmr-match"],
				LoadersData: [{ v: 1 }],
				ImportURLs: ["/hmr-match.js"],
			}),
		);

		await core.navigate("/hmr-match");
		commit.mockClear();

		await window.__vorma_hmr_route_update?.("/totally-different.js", {
			default: {
				pattern: "/other",
				component: () => {
					return null;
				},
			},
		});

		expect(has_route_render_commit(commit)).toBe(false);
	});
});

/////////////////////////////////////////////////////////////////////
/////// defineView
/////////////////////////////////////////////////////////////////////

describe("defineView", () => {
	it("returns ViewDefinition with correct fields", () => {
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		const comp = () => {
			return null;
		};
		const boundary = () => {
			return null;
		};
		const loader = async () => {
			return { data: true };
		};

		const def = core.defineView({
			pattern: "/test",
			component: comp,
			errorBoundary: boundary,
			clientLoader: loader,
		});

		expect(def.pattern).toBe("/test");
		expect(def.component).toBe(comp);
		expect(def.error_boundary).toBe(boundary);
		expect(def.client_loader).toBe(loader);
	});

	it("returns definition without optional fields", () => {
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		const def = core.defineView({
			pattern: "/minimal",
			component: () => {
				return null;
			},
		});

		expect(def.pattern).toBe("/minimal");
		expect(def.error_boundary).toBeUndefined();
		expect(def.client_loader).toBeUndefined();
	});
});

/////////////////////////////////////////////////////////////////////
/////// Prefetch integration
/////////////////////////////////////////////////////////////////////

describe("prefetch integration", () => {
	it("start_prefetch forwards to router with correct URL", async () => {
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.boot({});

		vi.spyOn(globalThis, "fetch").mockImplementation(() => {
			return new Promise(() => {});
		});

		core.start_prefetch("/prefetch-target");
		await tick();

		expect(globalThis.fetch).toHaveBeenCalled();
		const fetched_url = (globalThis.fetch as any).mock.calls[0][0];
		expect(fetched_url.toString()).toContain("/prefetch-target");
	});

	it("stop_prefetch cancels in-flight prefetch", async () => {
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.boot({});

		let signal: AbortSignal | undefined;
		vi.spyOn(globalThis, "fetch").mockImplementation(
			(_url: any, init?: any) => {
				signal = init?.signal;
				return new Promise(() => {});
			},
		);

		core.start_prefetch("/prefetch-target");
		await tick();

		expect(signal).toBeDefined();
		expect(signal!.aborted).toBe(false);

		core.stop_prefetch("/prefetch-target");

		expect(signal!.aborted).toBe(true);
	});

	it("resolves prestarted client loader server data during prefetch", async () => {
		let loader_call_count = 0;
		let seen_server_data: unknown;

		vi.doMock("/known-prefetch-module.js", () => {
			return {
				default: {
					pattern: "/known-prefetch",
					component: () => {
						return null;
					},
					client_loader: async ({ serverPromise }: any) => {
						loader_call_count++;
						seen_server_data = await serverPromise;
						return { client: true };
					},
				},
			};
		});

		const { core } = await setup();
		const { calls, call, wait_for } = mock_fetch();

		const first_nav = core.navigate("/known-prefetch");
		await wait_for(1);
		call(0).resolve(
			route_response({
				MatchedPatterns: ["/known-prefetch"],
				LoadersData: [{ initial: true }],
				ImportURLs: ["/known-prefetch-module.js"],
			}),
		);
		await first_nav;

		const away_nav = core.navigate("/other");
		await wait_for(2);
		call(1).resolve(route_response());
		await away_nav;

		loader_call_count = 0;
		seen_server_data = undefined;

		core.start_prefetch("/known-prefetch");
		await wait_for(3);

		expect(loader_call_count).toBe(1);
		expect(seen_server_data).toBeUndefined();

		call(2).resolve(
			route_response({
				MatchedPatterns: ["/known-prefetch"],
				LoadersData: [{ prefetched: true }],
				ImportURLs: ["/known-prefetch-module.js"],
			}),
		);

		await wait_until(() => {
			return seen_server_data !== undefined;
		}, "expected prefetch server data to resolve");

		expect(calls).toHaveLength(3);
		expect(seen_server_data).toEqual({
			clientBuildID: "build-1",
			matches: [
				{
					pattern: "/known-prefetch",
					input: {},
					loaderData: { prefetched: true },
				},
			],
			outermostServerError: null,
			loaderData: { prefetched: true },
		});
	});

	it("runs first-time route client loader during prefetch and reuses it on navigation", async () => {
		let loader_call_count = 0;
		let seen_server_data: unknown;

		vi.doMock("/first-prefetch-module.js", () => {
			return {
				default: {
					pattern: "/first-prefetch",
					component: () => {
						return null;
					},
					client_loader: async ({ serverPromise }: any) => {
						loader_call_count++;
						seen_server_data = await serverPromise;
						return { from_client: true };
					},
				},
			};
		});

		const { core, commit } = await setup();
		const { calls, call, wait_for } = mock_fetch();

		core.start_prefetch("/first-prefetch");
		await wait_for(1);

		call(0).resolve(
			route_response({
				MatchedPatterns: ["/first-prefetch"],
				LoadersData: [{ from_server: true }],
				ImportURLs: ["/first-prefetch-module.js"],
			}),
		);

		await wait_until(() => {
			return seen_server_data !== undefined;
		}, "expected first-time prefetch client loader to run");

		expect(has_route_render_commit(commit)).toBe(false);
		expect(loader_call_count).toBe(1);
		expect(seen_server_data).toEqual({
			clientBuildID: "build-1",
			matches: [
				{
					pattern: "/first-prefetch",
					input: {},
					loaderData: { from_server: true },
				},
			],
			outermostServerError: null,
			loaderData: { from_server: true },
		});

		const result = await core.navigate("/first-prefetch");

		expect(result.didNavigate).toBe(true);
		expect(calls).toHaveLength(1);
		expect(loader_call_count).toBe(1);
		expect(route_render_commit_count(commit)).toBe(1);

		const state = route_render_commit_at(commit, 0);
		expect(state.entries[0].client_loader_data).toEqual({
			from_client: true,
		});
	});

	it("clears prepared prefetch from work state while retaining it for navigation", async () => {
		vi.doMock("/work-prefetch-module.js", () => {
			return {
				default: {
					pattern: "/work-prefetch",
					component: () => {
						return null;
					},
				},
			};
		});

		const { core, commit } = await setup();
		const { calls, call, wait_for } = mock_fetch();

		core.start_prefetch("/work-prefetch");
		await wait_for(1);

		expect(core.getWorkState().prefetch?.href).toContain("/work-prefetch");

		call(0).resolve(
			route_response({
				MatchedPatterns: ["/work-prefetch"],
				LoadersData: [{ from_server: true }],
				ImportURLs: ["/work-prefetch-module.js"],
			}),
		);

		await wait_until(() => {
			return core.getWorkState().prefetch === null;
		}, "expected prepared prefetch to leave work state");

		const result = await core.navigate("/work-prefetch");

		expect(result.didNavigate).toBe(true);
		expect(calls).toHaveLength(1);
		expect(route_render_commit_count(commit)).toBe(1);
	});

	it("aborts first-time route client loader when prefetch is stopped", async () => {
		let captured_signal: AbortSignal | null = null;

		vi.doMock("/abort-prefetch-module.js", () => {
			return {
				default: {
					pattern: "/abort-prefetch",
					component: () => {
						return null;
					},
					client_loader: async ({ signal }: any) => {
						captured_signal = signal;
						return new Promise(() => {});
					},
				},
			};
		});

		const { core } = await setup();
		const { call, wait_for } = mock_fetch();

		core.start_prefetch("/abort-prefetch");
		await wait_for(1);

		call(0).resolve(
			route_response({
				MatchedPatterns: ["/abort-prefetch"],
				LoadersData: [{ value: 1 }],
				ImportURLs: ["/abort-prefetch-module.js"],
			}),
		);

		await wait_until(() => {
			return captured_signal !== null;
		}, "expected first-time prefetch client loader signal");

		expect(captured_signal!.aborted).toBe(false);

		core.stop_prefetch("/abort-prefetch");

		expect(captured_signal!.aborted).toBe(true);
	});
});

/////////////////////////////////////////////////////////////////////
/////// Client loader cancellation
/////////////////////////////////////////////////////////////////////

describe("client loader cancellation", () => {
	it("aborts client loader signal when prefetch is stopped", async () => {
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		let captured_signal: AbortSignal | null = null;
		const client_loader = async ({ signal, serverPromise }: any) => {
			captured_signal = signal;
			await serverPromise;
			return {};
		};

		vi.doMock("/loader-module.js", () => {
			return {
				default: {
					pattern: "/loader",
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
					MatchedPatterns: ["/loader"],
					LoadersData: [{ value: 1 }],
					ImportURLs: ["/loader-module.js"],
				}),
			)
			.mockResolvedValueOnce(route_response());

		await core.navigate("/loader");
		await core.navigate("/other");
		captured_signal = null;

		vi.spyOn(globalThis, "fetch").mockImplementation(() => {
			return new Promise(() => {});
		});
		core.start_prefetch("/loader");
		await tick();

		expect(captured_signal).not.toBeNull();
		expect(captured_signal!.aborted).toBe(false);

		core.stop_prefetch("/loader");

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
								err instanceof DOMException &&
								err.name === "AbortError";
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
				MatchedPatterns: ["/known-error"],
				LoadersData: [{ value: 1 }],
				ImportURLs: ["/known-error-module.js"],
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
				MatchedPatterns: ["/known-error"],
				ImportURLs: ["/known-error-module.js"],
				OutermostServerErrIdx: 0,
				OutermostServerErr: "server boom",
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
								err instanceof DOMException &&
								err.name === "AbortError";
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
								err instanceof DOMException &&
								err.name === "AbortError";
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
				MatchedPatterns: ["/known-parent", "/known-parent/child"],
				LoadersData: [{ parent: 1 }, { child: 1 }],
				ImportURLs: [
					"/known-parent-module.js",
					"/known-child-module.js",
				],
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
				MatchedPatterns: ["/known-parent", "/known-parent/child"],
				ImportURLs: ["/known-parent-module.js"],
				OutermostServerErrIdx: 0,
				OutermostServerErr: "server boom",
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
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
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
					MatchedPatterns: ["/products"],
					LoadersData: [{ value: 1 }],
					ImportURLs: ["/product-module.js"],
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
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
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
					MatchedPatterns: ["/parent"],
					LoadersData: [{ value: 1 }],
					ImportURLs: ["/parent-module.js"],
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
				MatchedPatterns: ["/parent", "/parent/child"],
				LoadersData: [{ parent: true }, { child: true }],
				ImportURLs: ["/parent-module.js", "/child-module.js"],
			}),
		);
		await nav;
	});
});

/////////////////////////////////////////////////////////////////////
/////// View transition timing
/////////////////////////////////////////////////////////////////////

describe("view transition timing", () => {
	it("does not apply title before view transition snapshot is captured", async () => {
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
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
				Title: { dangerousInnerHTML: "New Title" },
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
		seed_payload({ ClientBuildID: "build-1" });
		const commit = vi.fn();
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		const build_skews: string[] = [];
		await core.boot({
			onBuildSkewDetected: (event) => {
				build_skews.push(event.serverBuildID);
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
					{ Title: { dangerousInnerHTML: "Winner" } },
					"winner-build",
				),
			);
		});

		const stale = core.navigate("/stale-page");
		await tick();
		await core.navigate("/winner-page");

		expect(document.title).toBe("Winner");
		expect(core.getClientBuildID()).toBe("build-1");

		resolve_stale(
			route_response(
				{
					Title: { dangerousInnerHTML: "Stale Title" },
					CSSBundles: ["/stale.css"],
				},
				"stale-build",
			),
		);
		await stale;
		await tick();

		expect(document.title).toBe("Winner");
		expect(core.getClientBuildID()).toBe("build-1");
		expect(
			document.head.querySelector(
				'link[data-vorma-css-bundle="/stale.css"]',
			),
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
			seed_payload({ ClientBuildID: "build-1" });
			const commit = vi.fn();
			const core_res = create_client_core(
				{ apiMountRoot: "/api/" },
				commit,
				t_opts(),
			);
			if (!core_res.ok) {
				throw new Error(
					`create_client_core failed with error: ${core_res.err}`,
				);
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
						MatchedPatterns: ["/winner-page"],
						Title: { dangerousInnerHTML: "Winner" },
					},
					"winner-build",
				),
			);
			await navigation;
			await revalidation;

			expect(document.title).toBe("Winner");
			expect(core.getClientBuildID()).toBe("build-1");
			expect(route_render_commit_count(commit)).toBe(1);

			resolve_revalidation(
				route_response(
					{
						MatchedPatterns: ["/stale-revalidation"],
						Title: { dangerousInnerHTML: "Stale" },
					},
					"stale-build",
				),
			);
			await tick();

			expect(document.title).toBe("Winner");
			expect(core.getClientBuildID()).toBe("build-1");
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
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
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
				MatchedPatterns: ["/coherence"],
				LoadersData: [{ coherent: true }],
			}),
		);

		await core.navigate("/coherence");

		expect(state_during_callback).not.toBeNull();
		const state = state_during_callback as any;
		expect(state.matches.map((m: any) => m.pattern)).toEqual([
			"/coherence",
		]);
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
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.boot({
			onRouteUpdate: () => {
				return order.push("onRouteUpdate");
			},
		});
		order.length = 0;

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({ MatchedPatterns: ["/order"] }),
		);

		await core.navigate("/order");

		expect(order).toEqual(["commit", "onRouteUpdate"]);
	});

	it("fires an initial route update", async () => {
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
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

describe("route update blocked by client loaders", () => {
	it("does not fire onRouteUpdate until client loaders settle", async () => {
		let loader_resolve: ((v: unknown) => void) | undefined;

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
							loader_resolve = r;
						});
					},
				},
			};
		});

		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
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
				MatchedPatterns: ["/blocking"],
				LoadersData: [{ v: 1 }],
				ImportURLs: ["/mod-blocking.js"],
			}),
		);

		const nav = core.navigate("/blocking");

		for (let i = 0; i < 200; i++) {
			if (loader_resolve) {
				break;
			}
			await new Promise((r) => {
				return setTimeout(r, 0);
			});
		}

		expect(loader_resolve).toBeDefined();
		expect(route_changed).toBe(false);

		loader_resolve!({ loaded: true });
		await nav;

		expect(route_changed).toBe(true);
	});
});

/////////////////////////////////////////////////////////////////////
/////// CSS preload gating
/////////////////////////////////////////////////////////////////////

describe("CSS preload gating", () => {
	it("blocks navigation commit until CSS preloads settle", async () => {
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.boot({});
		const boot_commit_count = route_render_commit_count(commit);

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({ CSSBundles: ["/blocking.css"] }),
		);

		const nav = core.navigate("/css-blocked");

		// Poll until the preload link appears
		let preload: Element | null = null;
		for (let i = 0; i < 200; i++) {
			preload = document.head.querySelector(
				'link[data-vorma-css-preload="/blocking.css"]',
			);
			if (preload) {
				break;
			}
			await Promise.resolve();
		}

		expect(preload).not.toBeNull();
		expect(route_render_commit_count(commit)).toBe(boot_commit_count);

		preload!.dispatchEvent(new Event("load"));
		await nav;

		expect(route_render_commit_count(commit)).toBe(boot_commit_count + 1);
	});

	it("unblocks navigation when CSS preload errors", async () => {
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.boot({});
		const boot_commit_count = route_render_commit_count(commit);

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({ CSSBundles: ["/error.css"] }),
		);

		const nav = core.navigate("/css-error");

		let preload: Element | null = null;
		for (let i = 0; i < 200; i++) {
			preload = document.head.querySelector(
				'link[data-vorma-css-preload="/error.css"]',
			);
			if (preload) {
				break;
			}
			await Promise.resolve();
		}

		expect(preload).not.toBeNull();
		preload!.dispatchEvent(new Event("error"));
		await nav;

		expect(route_render_commit_count(commit)).toBe(boot_commit_count + 1);
	});
});

/////////////////////////////////////////////////////////////////////
/////// Client loader promise reuse on hash change
/////////////////////////////////////////////////////////////////////

describe("client loader promise reuse on hash change", () => {
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
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
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
				MatchedPatterns: ["/reuse"],
				LoadersData: [{ v: 1 }],
				ImportURLs: ["/mod-reuse.js"],
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
				MatchedPatterns: ["/reuse"],
				LoadersData: [{ v: 2 }],
				ImportURLs: ["/mod-reuse.js"],
			}),
		);
		await Promise.all([hash_nav_1, hash_nav_2]);
	});
});

/////////////////////////////////////////////////////////////////////
/////// History state
/////////////////////////////////////////////////////////////////////

describe("history state", () => {
	it("committed RouteState includes history_state from navigate", async () => {
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.boot({});
		commit.mockClear();

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({ MatchedPatterns: ["/about"] }),
		);

		await core.navigate("/about", { state: { from: "search" } });

		expect(has_route_render_commit(commit)).toBe(true);
		const state = route_render_commit_at(commit, 0);
		expect(state.history_state).toEqual({ from: "search" });
	});

	it("history_state is undefined when no state is provided", async () => {
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.boot({});
		commit.mockClear();

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({ MatchedPatterns: ["/page"] }),
		);

		await core.navigate("/page");

		const state = route_render_commit_at(commit, 0);
		expect(state.history_state).toBeUndefined();
	});

	it("history_state is undefined on initial load", async () => {
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.boot({});

		expect(has_route_render_commit(commit)).toBe(true);
		const state = route_render_commit_at(commit, 0);
		expect(state.history_state).toBeUndefined();
	});

	it("history_state is preserved through revalidation", async () => {
		seed_payload();
		const commit = vi.fn();
		const opts = t_opts();
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			opts,
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.boot({});
		commit.mockClear();

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({ MatchedPatterns: ["/stateful"] }),
		);

		await core.navigate("/stateful", {
			state: { modal: "confirm" },
		});

		const nav_state = route_render_commit_at(commit, 0);
		expect(nav_state.history_state).toEqual({ modal: "confirm" });
		commit.mockClear();

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({ MatchedPatterns: ["/stateful"] }),
		);

		await core.revalidate();
		await tick();

		expect(has_route_render_commit(commit)).toBe(true);
		const reval_state = route_render_commit_at(commit, 0);
		expect(reval_state.history_state).toEqual({ modal: "confirm" });
	});

	it("getRouteState exposes historyState", async () => {
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.boot({});

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({ MatchedPatterns: ["/detail"] }),
		);

		await core.navigate("/detail", {
			state: { from: "favorites" },
		});

		const route_state = core.getRouteState();
		expect(route_state.historyState).toEqual({ from: "favorites" });
	});

	it("hash-only navigation commits state to history", async () => {
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.boot({});

		await core.navigate("/#section", {
			state: { tab: "overview" },
		});

		// State should be written to history even for hash-only nav
		const route_state = core.getRouteState();
		expect(route_state.historyState).toEqual({ tab: "overview" });
	});

	it("state from replace navigation overwrites previous state", async () => {
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.boot({});
		commit.mockClear();

		vi.spyOn(globalThis, "fetch")
			.mockResolvedValueOnce(
				route_response({ MatchedPatterns: ["/page"] }),
			)
			.mockResolvedValueOnce(
				route_response({ MatchedPatterns: ["/page"] }),
			);

		await core.navigate("/page", { state: { v: 1 } });

		const first_state = route_render_commit_at(commit, 0);
		expect(first_state.history_state).toEqual({ v: 1 });
		commit.mockClear();

		await core.navigate("/page", {
			replace: true,
			state: { v: 2 },
		});

		const second_state = route_render_commit_at(commit, 0);
		expect(second_state.history_state).toEqual({ v: 2 });
	});

	it("complex state objects are preserved", async () => {
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ apiMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.boot({});
		commit.mockClear();

		const complex_state = {
			selectedContact: { id: "c1", name: "Alice" },
			filters: ["active", "recent"],
			page: 3,
		};

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({ MatchedPatterns: ["/compose"] }),
		);

		await core.navigate("/compose", { state: complex_state });

		const state = route_render_commit_at(commit, 0);
		expect(state.history_state).toEqual(complex_state);
	});
});
