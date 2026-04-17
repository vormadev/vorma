// @vitest-environment jsdom

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
	mock_fetch,
	register_ccc_lifecycle,
	route_response,
	seed_payload,
	setup,
	tick,
} from "./___ccc_test_helpers.ts";
import {
	REVALIDATION_DEBOUNCE_MS,
	apply_scroll,
	create_client_core,
	type ClientCore,
	type ProgressIndicatorConfig,
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
/////// Init lifecycle
/////////////////////////////////////////////////////////////////////

describe("init lifecycle", () => {
	it("parses #vorma-data-json script element for initial payload", async () => {
		seed_payload({
			MatchedPatterns: ["/"],
			LoadersData: [{ root: true }],
		});
		const commit = vi.fn();
		const core_res = create_client_core(
			{ actionsMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.init({});

		expect(commit).toHaveBeenCalled();
		const state = commit.mock.calls[0]![0];
		expect(state.entries).toHaveLength(1);
		expect(state.entries[0].pattern).toBe("/");
		expect(state.entries[0].data).toEqual({ root: true });
	});

	it("returns err when data script element is missing", async () => {
		const commit = vi.fn();
		const core_res = create_client_core(
			{ actionsMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		expect(core_res.ok).toBe(true);
		if (!core_res.ok) {
			throw new Error("unexpected");
		}
		const init_res = await core_res.val.init({});
		expect(init_res.ok).toBe(false);
	});

	it("calls render during init", async () => {
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ actionsMountRoot: "/api/" },
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

		await core.init({ render });

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
			{ actionsMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.init({});

		const scroll_intent = commit.mock.calls[0]![1];
		expect(scroll_intent?.scroll).toEqual({ x: 55, y: 77 });
		sessionStorage.removeItem("vorma-scroll-state-reload");
	});

	it("passes current hash as initial commit scroll intent", async () => {
		window.history.replaceState({}, "", "/page#section");
		seed_payload({ MatchedPatterns: ["/page"] });
		const commit = vi.fn();
		const core_res = create_client_core(
			{ actionsMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.init({});

		const scroll_intent = commit.mock.calls[0]![1];
		expect(scroll_intent?.scroll).toEqual({ hash: "#section" });
	});

	it("allows initial client loaders to submit API queries", async () => {
		let core: any;

		vi.doMock("/mod-init-query.js", () => {
			return {
				default: {
					pattern: "/init-query",
					component: () => {
						return null;
					},
					client_loader: async () => {
						return core.submit(
							"/api/some-api",
							{ method: "GET" },
							{ revalidate: false },
						);
					},
				},
			};
		});

		seed_payload({
			MatchedPatterns: ["/init-query"],
			ImportURLs: ["/mod-init-query.js"],
		});
		const commit = vi.fn();
		const core_res = create_client_core(
			{ actionsMountRoot: "/api/" },
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

		await core.init({});

		const state = commit.mock.calls[0]![0];
		expect(state.entries[0].client_data.success).toBe(true);
		expect(state.entries[0].client_data.data).toEqual({ ok: true });
	});

	it("does not deadlock initial client loaders awaiting submit revalidation", async () => {
		let core: any;

		vi.doMock("/mod-init-mutate.js", () => {
			return {
				default: {
					pattern: "/init-mutate",
					component: () => {
						return null;
					},
					client_loader: async () => {
						const result = await core.submit("/api/save", {
							method: "POST",
						});
						return result.revalidationPromise;
					},
				},
			};
		});

		seed_payload({
			MatchedPatterns: ["/init-mutate"],
			ImportURLs: ["/mod-init-mutate.js"],
		});
		const commit = vi.fn();
		const core_res = create_client_core(
			{ actionsMountRoot: "/api/" },
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

		await core.init({});
		await tick();

		const state = commit.mock.calls[0]![0];
		expect(state.entries[0].client_data).toEqual({ ok: true });
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

		vi.doMock("/mod-init-router-data.js", () => {
			return {
				default: {
					pattern: "/init-router-data",
					component: () => {
						return null;
					},
					client_loader: async () => {
						return core.getRouterData();
					},
				},
			};
		});

		seed_payload({
			MatchedPatterns: ["/", "/init-router-data"],
			LoadersData: [{ root: true }, { route: true }],
			ImportURLs: ["/mod-root.js", "/mod-init-router-data.js"],
		});
		const commit = vi.fn();
		const core_res = create_client_core(
			{ actionsMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		core = core_res.val;

		await core.init({});

		const state = commit.mock.calls[0]![0];
		expect(state.entries[1].client_data).toEqual({
			clientBuildID: "build-1",
			matchedPatterns: ["/", "/init-router-data"],
			splatValues: [],
			params: {},
			historyState: undefined,
			rootData: { root: true },
		});
	});

	it("extracts initial build ID from data script", async () => {
		seed_payload({ ClientBuildID: "initial-build" });
		const commit = vi.fn();
		const core_res = create_client_core(
			{ actionsMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.init({});

		expect(core.getClientBuildID()).toBe("initial-build");
	});
});

/////////////////////////////////////////////////////////////////////
/////// Title entity decoding
/////////////////////////////////////////////////////////////////////

describe("title entity decoding", () => {
	it("decodes HTML entities in title during init", async () => {
		seed_payload({
			Title: { dangerousInnerHTML: "A &amp; B" },
		});
		const commit = vi.fn();
		const core_res = create_client_core(
			{ actionsMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.init({});

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
			{ actionsMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.init({});

		expect(document.title).toBe('<Title> & "More"');
	});

	it("handles empty title dangerousInnerHTML", async () => {
		seed_payload({
			Title: { dangerousInnerHTML: "" },
		});
		const commit = vi.fn();
		const core_res = create_client_core(
			{ actionsMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.init({});

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
			{ actionsMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.init({});
		commit.mockClear();

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({
				MatchedPatterns: ["/about"],
				LoadersData: [{ page: "about" }],
			}),
		);

		await core.navigate("/about");

		expect(commit).toHaveBeenCalled();
		const state = commit.mock.calls[0]![0];
		expect(state.entries).toHaveLength(1);
		expect(state.entries[0].pattern).toBe("/about");
		expect(state.entries[0].data).toEqual({ page: "about" });
	});

	it("commit receives scroll intent", async () => {
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ actionsMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.init({});
		commit.mockClear();

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({ MatchedPatterns: ["/page"] }),
		);

		await core.navigate("/page");

		expect(commit).toHaveBeenCalled();
		const scroll_intent = commit.mock.calls[0]![1];
		expect(scroll_intent?.scroll).toEqual({ x: 0, y: 0 });
	});

	it("uses view transitions for navigations when enabled", async () => {
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ actionsMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.init({ useViewTransitions: true });
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
			init: { useViewTransitions: true },
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
			return commit.mock.calls.length > 0;
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
			init: { useViewTransitions: true },
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

		expect(commit).not.toHaveBeenCalled();
		await expect(first).resolves.toEqual({ didNavigate: false });

		transitions[1]!();
		await expect(second).resolves.toEqual({ didNavigate: true });

		expect(commit).toHaveBeenCalledTimes(1);
		const state = commit.mock.calls[0]![0];
		expect(state.entries[0].data).toEqual({ page: "second" });
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
					client_loader: async ({ serverDataPromise }: any) => {
						call_order.push("a_start");
						await serverDataPromise;
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
					client_loader: async ({ serverDataPromise }: any) => {
						call_order.push("b_start");
						await serverDataPromise;
						call_order.push("b_end");
						return { b: true };
					},
				},
			};
		});

		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ actionsMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.init({});

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

	it("receives correct serverDataPromise content", async () => {
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
					client_loader: async ({ serverDataPromise }: any) => {
						captured_server_data = await serverDataPromise;
						return { enhanced: true };
					},
				},
			};
		});

		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ actionsMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.init({});

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
			matchedPatterns: ["/", "/users/:id"],
			rootData: { session: "abc" },
			loaderData: { name: "Ada" },
			clientBuildID: "build-1",
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
					client_loader: async ({
						signal,
						serverDataPromise,
					}: any) => {
						captured_signal = signal;
						await serverDataPromise;
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
			{ actionsMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.init({});

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
					client_loader: async ({
						signal,
						serverDataPromise,
					}: any) => {
						second_signal = signal;
						await serverDataPromise;
						return {};
					},
				},
			};
		});

		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ actionsMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.init({});

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

	it("non-abort errors surface as route entry errors", async () => {
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
			{ actionsMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.init({});
		commit.mockClear();

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({
				MatchedPatterns: ["/err-route"],
				LoadersData: [{ v: 1 }],
				ImportURLs: ["/mod-err.js"],
			}),
		);

		await core.navigate("/err-route");

		const state = commit.mock.calls[0]![0];
		expect(state.entries[0].error).toBe("client loader boom");
	});

	it("client loader results stored as client_data on route entries", async () => {
		vi.doMock("/mod-cl.js", () => {
			return {
				default: {
					pattern: "/cl-route",
					component: () => {
						return null;
					},
					client_loader: async ({ serverDataPromise }: any) => {
						const server = await serverDataPromise;
						return { enhanced: true, original: server.loaderData };
					},
				},
			};
		});

		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ actionsMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.init({});
		commit.mockClear();

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({
				MatchedPatterns: ["/cl-route"],
				LoadersData: [{ raw: "data" }],
				ImportURLs: ["/mod-cl.js"],
			}),
		);

		await core.navigate("/cl-route");

		const state = commit.mock.calls[0]![0];
		expect(state.entries[0].client_data).toEqual({
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
			{ actionsMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.init({});
		commit.mockClear();

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({
				MatchedPatterns: ["/abort-route"],
				LoadersData: [{ v: 1 }],
				ImportURLs: ["/mod-abort.js"],
			}),
		);

		await core.navigate("/abort-route");

		const state = commit.mock.calls[0]![0];
		expect(state.entries[0].error).toBeUndefined();
		expect(state.entries[0].client_data).toBeUndefined();
	});
});

/////////////////////////////////////////////////////////////////////
/////// Build ID
/////////////////////////////////////////////////////////////////////

describe("build ID", () => {
	it("extracts build ID from response header", async () => {
		seed_payload({ ClientBuildID: "build-1" });
		const commit = vi.fn();
		const core_res = create_client_core(
			{ actionsMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.init({});

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({}, "build-2"),
		);

		await core.navigate("/page");

		expect(core.getClientBuildID()).toBe("build-2");
	});

	it("fires onClientBuildIDChange on change", async () => {
		seed_payload({ ClientBuildID: "build-1" });
		const commit = vi.fn();
		const core_res = create_client_core(
			{ actionsMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		const on_change = vi.fn();
		await core.init({ onClientBuildIDChange: on_change });

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({}, "build-2"),
		);

		await core.navigate("/page");

		expect(on_change).toHaveBeenCalledWith("build-1", "build-2");
	});

	it("does not fire notification when build ID is same", async () => {
		seed_payload({ ClientBuildID: "build-1" });
		const commit = vi.fn();
		const core_res = create_client_core(
			{ actionsMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		const on_change = vi.fn();
		await core.init({ onClientBuildIDChange: on_change });

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({}, "build-1"),
		);

		await core.navigate("/page");

		expect(on_change).not.toHaveBeenCalled();
	});
});

/////////////////////////////////////////////////////////////////////
/////// Status integration
/////////////////////////////////////////////////////////////////////

describe("status integration", () => {
	it("onStatusChange receives mapped StatusInfo", async () => {
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ actionsMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		const statuses: any[] = [];
		await core.init({
			onStatusChange: (s) => {
				return statuses.push({ ...s });
			},
		});

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(route_response());

		await core.navigate("/page");

		expect(statuses.length).toBeGreaterThan(0);
		const navigating = statuses.find((s) => {
			return s.isNavigating;
		});
		expect(navigating).toBeDefined();
		expect(navigating).toHaveProperty("isRevalidating");
		expect(navigating).toHaveProperty("isSubmitting");
	});

	it("getStatus returns current snapshot", async () => {
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ actionsMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.init({});

		const idle = core.getStatus();
		expect(idle).toEqual({
			isNavigating: false,
			isRevalidating: false,
			isSubmitting: false,
		});
	});
});

/////////////////////////////////////////////////////////////////////
/////// Progress indicators
/////////////////////////////////////////////////////////////////////

describe("progress indicators", () => {
	afterEach(() => {
		vi.useRealTimers();
	});

	async function setup_core(
		progressIndicator: ProgressIndicatorConfig,
	): Promise<ClientCore> {
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ actionsMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		await core_res.val.init({ progressIndicator });
		return core_res.val;
	}

	it("start/stop around navigation with delays", async () => {
		vi.useFakeTimers();
		let running = false;
		const config = {
			start: vi.fn(() => {
				running = true;
			}),
			stop: vi.fn(() => {
				running = false;
			}),
			isRunning: () => {
				return running;
			},
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

	it("respects inclusion filter for navigations only", async () => {
		vi.useFakeTimers();
		let running = false;
		const config = {
			start: vi.fn(() => {
				running = true;
			}),
			stop: vi.fn(() => {
				running = false;
			}),
			isRunning: () => {
				return running;
			},
			include: ["navigations"] as Array<
				"navigations" | "submissions" | "revalidations"
			>,
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

		await core.submit(
			"/api/action",
			{ method: "POST" },
			{
				revalidate: false,
			},
		);
		await vi.advanceTimersByTimeAsync(10);

		expect(config.start).not.toHaveBeenCalled();
	});

	it("skips progress indicator for opted-out submissions", async () => {
		vi.useFakeTimers();
		let running = false;
		const config = {
			start: vi.fn(() => {
				running = true;
			}),
			stop: vi.fn(() => {
				running = false;
			}),
			isRunning: () => {
				return running;
			},
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

		await core.submit(
			"/api/action",
			{ method: "POST" },
			{
				revalidate: false,
				skipProgressIndicator: true,
			},
		);
		await vi.advanceTimersByTimeAsync(10);

		expect(config.start).not.toHaveBeenCalled();
		expect(config.stop).not.toHaveBeenCalled();
	});

	it("skips progress indicator for opted-out navigations", async () => {
		vi.useFakeTimers();
		let running = false;
		const config = {
			start: vi.fn(() => {
				running = true;
			}),
			stop: vi.fn(() => {
				running = false;
			}),
			isRunning: () => {
				return running;
			},
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
			skipProgressIndicator: true,
		});
		await vi.advanceTimersByTimeAsync(10);

		expect(config.start).not.toHaveBeenCalled();

		resolve_fetch(route_response());
		await vi.advanceTimersByTimeAsync(10);
		await tick();

		expect(config.stop).not.toHaveBeenCalled();
	});

	it("overlapping work does not cause start-stop thrash", async () => {
		vi.useFakeTimers();
		let running = false;
		const config = {
			start: vi.fn(() => {
				running = true;
			}),
			stop: vi.fn(() => {
				running = false;
			}),
			isRunning: () => {
				return running;
			},
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

		void core.submit("/api/a", { method: "POST" }, { revalidate: false });
		void core.submit("/api/b", { method: "POST" }, { revalidate: false });
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
		let running = false;
		const config = {
			start: vi.fn(() => {
				running = true;
			}),
			stop: vi.fn(() => {
				running = false;
			}),
			isRunning: () => {
				return running;
			},
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
		let running = false;
		const config = {
			start: vi.fn(() => {
				running = true;
			}),
			stop: vi.fn(() => {
				running = false;
			}),
			isRunning: () => {
				return running;
			},
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
			{ actionsMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.init({});

		const cleanup = core.revalidateOnWindowFocus({ staleTimeMS: 100 });

		vi.spyOn(globalThis, "fetch").mockResolvedValue(route_response());

		await vi.advanceTimersByTimeAsync(100);
		window.dispatchEvent(new Event("focus"));
		await vi.advanceTimersByTimeAsync(100);

		expect(globalThis.fetch).toHaveBeenCalled();
		cleanup();
	});

	it("does not fire when stale time has not elapsed", async () => {
		vi.useFakeTimers();
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ actionsMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.init({});

		const cleanup = core.revalidateOnWindowFocus({ staleTimeMS: 5000 });

		vi.spyOn(globalThis, "fetch").mockResolvedValue(route_response());

		await vi.advanceTimersByTimeAsync(100);
		window.dispatchEvent(new Event("focus"));
		await vi.advanceTimersByTimeAsync(0);

		expect(globalThis.fetch).not.toHaveBeenCalled();
		cleanup();
	});

	it("cleanup stops listening", async () => {
		vi.useFakeTimers();
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ actionsMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.init({});

		const cleanup = core.revalidateOnWindowFocus({ staleTimeMS: 100 });
		cleanup();

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
			{ actionsMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.init({});

		const cleanup = core.revalidateOnWindowFocus({ staleTimeMS: 1000 });

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

		cleanup();
	});

	it("does not fire during active navigation", async () => {
		vi.useFakeTimers();
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ actionsMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.init({});

		const cleanup = core.revalidateOnWindowFocus({ staleTimeMS: 0 });

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
		cleanup();
	});

	it("does not fire during active submission", async () => {
		vi.useFakeTimers();
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ actionsMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.init({});

		const cleanup = core.revalidateOnWindowFocus({ staleTimeMS: 0 });

		let resolve_submit!: (r: Response) => void;
		vi.spyOn(globalThis, "fetch").mockImplementation(() => {
			return new Promise((r) => {
				resolve_submit = r;
			});
		});

		void core.submit(
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
		cleanup();
	});

	it("does not fire during active revalidation", async () => {
		vi.useFakeTimers();
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ actionsMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.init({});

		const cleanup = core.revalidateOnWindowFocus({ staleTimeMS: 0 });

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
		cleanup();
	});

	it("does not advance stale-time timestamp for aborted navigations", async () => {
		vi.useFakeTimers();
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ actionsMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.init({});

		const cleanup = core.revalidateOnWindowFocus({ staleTimeMS: 100 });

		vi.spyOn(globalThis, "fetch")
			.mockRejectedValueOnce(new DOMException("Aborted", "AbortError"))
			.mockResolvedValue(route_response());

		await vi.advanceTimersByTimeAsync(90);
		await core.navigate("/aborted");

		await vi.advanceTimersByTimeAsync(20);
		window.dispatchEvent(new Event("focus"));
		await vi.advanceTimersByTimeAsync(100);

		expect((globalThis.fetch as any).mock.calls.length).toBe(2);
		cleanup();
	});

	it("does not reset stale-time for hash-only navigations", async () => {
		vi.useFakeTimers();
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ actionsMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.init({});

		const cleanup = core.revalidateOnWindowFocus({ staleTimeMS: 100 });

		vi.spyOn(globalThis, "fetch").mockResolvedValue(route_response());

		await vi.advanceTimersByTimeAsync(90);
		await core.navigate("/#section");
		await vi.advanceTimersByTimeAsync(0);

		await vi.advanceTimersByTimeAsync(20);
		window.dispatchEvent(new Event("focus"));
		await vi.advanceTimersByTimeAsync(100);

		expect((globalThis.fetch as any).mock.calls.length).toBe(1);
		cleanup();
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
			{ actionsMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.init({});

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

		expect(commit).toHaveBeenCalled();
		const state = commit.mock.calls[0]![0];
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
					client_loader: async ({ serverDataPromise }: any) => {
						loader_run_count++;
						await serverDataPromise;
						return { run: loader_run_count };
					},
				},
			};
		});

		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ actionsMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.init({});

		// defineRoute with runClientLoaderOnHMR: true
		core.defineRoute({
			pattern: "/hmr-cl",
			component: () => {
				return null;
			},
			clientLoader: async ({ serverDataPromise }: any) => {
				loader_run_count++;
				await serverDataPromise;
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
				client_loader: async ({ serverDataPromise }: any) => {
					loader_run_count++;
					await serverDataPromise;
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
					client_loader: async ({ serverDataPromise }: any) => {
						loader_run_count++;
						await serverDataPromise;
						return { run: loader_run_count };
					},
				},
			};
		});

		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ actionsMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.init({});

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
				client_loader: async ({ serverDataPromise }: any) => {
					loader_run_count++;
					await serverDataPromise;
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
			{ actionsMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.init({});

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

		expect(commit).not.toHaveBeenCalled();
	});
});

/////////////////////////////////////////////////////////////////////
/////// defineRoute
/////////////////////////////////////////////////////////////////////

describe("defineRoute", () => {
	it("returns RouteDefinition with correct fields", () => {
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ actionsMountRoot: "/api/" },
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

		const def = core.defineRoute({
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
			{ actionsMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		const def = core.defineRoute({
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
			{ actionsMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.init({});

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
			{ actionsMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.init({});

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
					client_loader: async ({ serverDataPromise }: any) => {
						loader_call_count++;
						seen_server_data = await serverDataPromise;
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
			matchedPatterns: ["/known-prefetch"],
			rootData: undefined,
			loaderData: { prefetched: true },
			clientBuildID: "build-1",
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
					client_loader: async ({ serverDataPromise }: any) => {
						loader_call_count++;
						seen_server_data = await serverDataPromise;
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

		expect(commit).not.toHaveBeenCalled();
		expect(loader_call_count).toBe(1);
		expect(seen_server_data).toEqual({
			matchedPatterns: ["/first-prefetch"],
			rootData: undefined,
			loaderData: { from_server: true },
			clientBuildID: "build-1",
		});

		const result = await core.navigate("/first-prefetch");

		expect(result.didNavigate).toBe(true);
		expect(calls).toHaveLength(1);
		expect(loader_call_count).toBe(1);
		expect(commit).toHaveBeenCalledTimes(1);

		const state = commit.mock.calls[0]![0];
		expect(state.entries[0].client_data).toEqual({ from_client: true });
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
/////// Client loader prefetch abort on stop
/////////////////////////////////////////////////////////////////////

describe("client loader prefetch abort on stop", () => {
	it("aborts client loader signal when prefetch is stopped", async () => {
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ actionsMountRoot: "/api/" },
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
		const client_loader = async ({ signal, serverDataPromise }: any) => {
			captured_signal = signal;
			await serverDataPromise;
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

		await core.init({});

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
});

/////////////////////////////////////////////////////////////////////
/////// Client loader prefetch isolation from unrelated work
/////////////////////////////////////////////////////////////////////

describe("client loader prefetch isolation from unrelated work", () => {
	it("does not abort client loader prefetch signal when unrelated submit fires", async () => {
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ actionsMountRoot: "/api/" },
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
		const client_loader = async ({ signal, serverDataPromise }: any) => {
			captured_signal = signal;
			await serverDataPromise;
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

		await core.init({});

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
		await core.submit(
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
			{ actionsMountRoot: "/api/" },
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
		const client_loader = async ({ serverDataPromise }: any) => {
			prestart_count++;
			const data = await serverDataPromise;
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

		await core.init({});

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
			{ actionsMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.init({ useViewTransitions: true });

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
			{ actionsMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		const build_changes: Array<{ prev: string; next: string }> = [];
		await core.init({
			onClientBuildIDChange: (prev, next) => {
				build_changes.push({ prev, next });
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
		expect(core.getClientBuildID()).toBe("winner-build");

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
		expect(core.getClientBuildID()).toBe("winner-build");
		expect(
			document.head.querySelector(
				'link[data-vorma-css-bundle="/stale.css"]',
			),
		).toBeNull();
		expect(
			build_changes.some((c) => {
				return c.next === "stale-build";
			}),
		).toBe(false);
	});

	it("does not commit stale revalidation after navigation supersedes it", async () => {
		vi.useFakeTimers();

		try {
			seed_payload({ ClientBuildID: "build-1" });
			const commit = vi.fn();
			const core_res = create_client_core(
				{ actionsMountRoot: "/api/" },
				commit,
				t_opts(),
			);
			if (!core_res.ok) {
				throw new Error(
					`create_client_core failed with error: ${core_res.err}`,
				);
			}
			const core = core_res.val;

			await core.init({});
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
			expect(core.getClientBuildID()).toBe("winner-build");
			expect(commit).toHaveBeenCalledTimes(1);

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
			expect(core.getClientBuildID()).toBe("winner-build");
			expect(commit).toHaveBeenCalledTimes(1);
		} finally {
			vi.useRealTimers();
		}
	});
});

/////////////////////////////////////////////////////////////////////
/////// Route commit coherence
/////////////////////////////////////////////////////////////////////

describe("route commit coherence", () => {
	it("state is updated before onRouteCommit fires", async () => {
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ actionsMountRoot: "/api/" },
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
		let route_commit_info: unknown = null;
		await core.init({
			onRouteCommit: (info) => {
				state_during_callback = core.getRouterData();
				route_commit_info = info;
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
		expect(state.matchedPatterns).toEqual(["/coherence"]);
		expect(route_commit_info).toMatchObject({
			reason: "navigation",
			url: `${window.location.origin}/coherence`,
			previousUrl: `${window.location.origin}/`,
			urlChanged: true,
			patternsChanged: true,
			paramsChanged: false,
			searchChanged: false,
			hashChanged: false,
			historyStateChanged: false,
		});
	});

	it("commit is called before onRouteCommit fires", async () => {
		seed_payload();
		const order: string[] = [];
		const commit = vi.fn(() => {
			return order.push("commit");
		});
		const core_res = create_client_core(
			{ actionsMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.init({
			onRouteCommit: () => {
				return order.push("onRouteCommit");
			},
		});
		order.length = 0;

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({ MatchedPatterns: ["/order"] }),
		);

		await core.navigate("/order");

		expect(order).toEqual(["commit", "onRouteCommit"]);
	});

	it("fires an initial route commit", async () => {
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ actionsMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		let route_commit_info: unknown = null;
		await core.init({
			onRouteCommit: (info) => {
				route_commit_info = info;
			},
		});

		expect(route_commit_info).toMatchObject({
			reason: "initial",
			url: `${window.location.origin}/`,
			previousUrl: null,
			urlChanged: true,
			patternsChanged: true,
			paramsChanged: true,
			searchChanged: true,
			hashChanged: true,
			historyStateChanged: true,
		});
	});
});

/////////////////////////////////////////////////////////////////////
/////// Route commit blocked by client loaders
/////////////////////////////////////////////////////////////////////

describe("route commit blocked by client loaders", () => {
	it("does not fire onRouteCommit until client loaders settle", async () => {
		let loader_resolve: ((v: unknown) => void) | undefined;

		vi.doMock("/mod-blocking.js", () => {
			return {
				default: {
					pattern: "/blocking",
					component: () => {
						return null;
					},
					client_loader: async ({ serverDataPromise }: any) => {
						await serverDataPromise;
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
			{ actionsMountRoot: "/api/" },
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
		await core.init({
			onRouteCommit: (info) => {
				if (info.reason === "initial") {
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
			{ actionsMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.init({});
		const init_commit_count = commit.mock.calls.length;

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
		expect(commit.mock.calls.length).toBe(init_commit_count);

		preload!.dispatchEvent(new Event("load"));
		await nav;

		expect(commit.mock.calls.length).toBe(init_commit_count + 1);
	});

	it("unblocks navigation when CSS preload errors", async () => {
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ actionsMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.init({});
		const init_commit_count = commit.mock.calls.length;

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

		expect(commit.mock.calls.length).toBe(init_commit_count + 1);
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
					client_loader: async ({ serverDataPromise }: any) => {
						invocation_count++;
						await serverDataPromise;
						return { count: invocation_count };
					},
				},
			};
		});

		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ actionsMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.init({});

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
			{ actionsMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.init({});
		commit.mockClear();

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({ MatchedPatterns: ["/about"] }),
		);

		await core.navigate("/about", { state: { from: "search" } });

		expect(commit).toHaveBeenCalled();
		const state = commit.mock.calls[0]![0];
		expect(state.history_state).toEqual({ from: "search" });
	});

	it("history_state is undefined when no state is provided", async () => {
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ actionsMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.init({});
		commit.mockClear();

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({ MatchedPatterns: ["/page"] }),
		);

		await core.navigate("/page");

		const state = commit.mock.calls[0]![0];
		expect(state.history_state).toBeUndefined();
	});

	it("history_state is undefined on initial load", async () => {
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ actionsMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.init({});

		expect(commit).toHaveBeenCalled();
		const state = commit.mock.calls[0]![0];
		expect(state.history_state).toBeUndefined();
	});

	it("history_state is preserved through revalidation", async () => {
		seed_payload();
		const commit = vi.fn();
		const opts = t_opts();
		const core_res = create_client_core(
			{ actionsMountRoot: "/api/" },
			commit,
			opts,
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.init({});
		commit.mockClear();

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({ MatchedPatterns: ["/stateful"] }),
		);

		await core.navigate("/stateful", {
			state: { modal: "confirm" },
		});

		const nav_state = commit.mock.calls[0]![0];
		expect(nav_state.history_state).toEqual({ modal: "confirm" });
		commit.mockClear();

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({ MatchedPatterns: ["/stateful"] }),
		);

		await core.revalidate();
		await tick();

		expect(commit).toHaveBeenCalled();
		const reval_state = commit.mock.calls[0]![0];
		expect(reval_state.history_state).toEqual({ modal: "confirm" });
	});

	it("getRouterData exposes historyState", async () => {
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ actionsMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.init({});

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({ MatchedPatterns: ["/detail"] }),
		);

		await core.navigate("/detail", {
			state: { from: "favorites" },
		});

		const router_data = core.getRouterData();
		expect(router_data.historyState).toEqual({ from: "favorites" });
	});

	it("hash-only navigation commits state to history", async () => {
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ actionsMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.init({});

		await core.navigate("/#section", {
			state: { tab: "overview" },
		});

		// State should be written to history even for hash-only nav
		const router_data = core.getRouterData();
		expect(router_data.historyState).toEqual({ tab: "overview" });
	});

	it("state from replace navigation overwrites previous state", async () => {
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ actionsMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.init({});
		commit.mockClear();

		vi.spyOn(globalThis, "fetch")
			.mockResolvedValueOnce(
				route_response({ MatchedPatterns: ["/page"] }),
			)
			.mockResolvedValueOnce(
				route_response({ MatchedPatterns: ["/page"] }),
			);

		await core.navigate("/page", { state: { v: 1 } });

		const first_state = commit.mock.calls[0]![0];
		expect(first_state.history_state).toEqual({ v: 1 });
		commit.mockClear();

		await core.navigate("/page", {
			replace: true,
			state: { v: 2 },
		});

		const second_state = commit.mock.calls[0]![0];
		expect(second_state.history_state).toEqual({ v: 2 });
	});

	it("complex state objects are preserved", async () => {
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core(
			{ actionsMountRoot: "/api/" },
			commit,
			t_opts(),
		);
		if (!core_res.ok) {
			throw new Error(
				`create_client_core failed with error: ${core_res.err}`,
			);
		}
		const core = core_res.val;

		await core.init({});
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

		const state = commit.mock.calls[0]![0];
		expect(state.history_state).toEqual(complex_state);
	});
});
