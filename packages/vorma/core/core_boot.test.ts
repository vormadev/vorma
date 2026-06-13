// @vitest-environment jsdom

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
	SEARCH_PARAM_SCHEMA_BOOL,
	SEARCH_PARAM_SCHEMA_NUMBER,
	SEARCH_PARAM_SCHEMA_STRING,
} from "../kit/json/search_param_parser.ts";
import {
	has_route_render_commit,
	redirect_response,
	register_ccc_lifecycle,
	route_render_commit_at,
	route_render_scroll_intent_at,
	route_response,
	seed_payload,
	t_opts,
	tick,
} from "./_test_helpers.ts";
import { BUILD_ID_HEADER, X_CLIENT_REDIRECT } from "./constants.ts";
import { create_client_core } from "./create_client_core.ts";

register_ccc_lifecycle(beforeEach, afterEach);

describe("boot lifecycle", () => {
	it("parses #vorma-data-json script element for initial payload", async () => {
		seed_payload({
			matched_patterns: ["/"],
			views_data: [{ root: true }],
		});
		const commit = vi.fn();
		const core_res = create_client_core({}, commit, t_opts());
		if (!core_res.ok) {
			throw new Error(`create_client_core failed with error: ${core_res.err}`);
		}
		const core = core_res.val;

		await core.boot({});

		expect(has_route_render_commit(commit)).toBe(true);
		const state = route_render_commit_at(commit, 0);
		expect(state.entries).toHaveLength(1);
		expect(state.entries[0].pattern).toBe("/");
		expect(state.entries[0].view_data).toEqual({ root: true });
	});

	it("parses matched input from initial search schemas", async () => {
		window.history.replaceState({}, "", "/users?page=2&active=true&tags=a&tags=b");
		seed_payload({
			matched_patterns: ["/users"],
			views_data: [{ users: true }],
			search_schemas: [
				{
					active: SEARCH_PARAM_SCHEMA_BOOL,
					page: SEARCH_PARAM_SCHEMA_NUMBER,
					tags: [SEARCH_PARAM_SCHEMA_STRING],
				},
			],
		});
		const commit = vi.fn();
		const core_res = create_client_core({}, commit, t_opts());
		if (!core_res.ok) {
			throw new Error(`create_client_core failed with error: ${core_res.err}`);
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
		const core_res = create_client_core({}, commit, t_opts());
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
		const core_res = create_client_core({}, commit, t_opts());
		if (!core_res.ok) {
			throw new Error(`create_client_core failed with error: ${core_res.err}`);
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
		const core_res = create_client_core({}, commit, t_opts());
		if (!core_res.ok) {
			throw new Error(`create_client_core failed with error: ${core_res.err}`);
		}
		const core = core_res.val;

		await core.boot({});

		const scroll_intent = route_render_scroll_intent_at(commit, 0);
		expect(scroll_intent?.scroll).toEqual({ x: 55, y: 77 });
		sessionStorage.removeItem("vorma-scroll-state-reload");
	});

	it("passes current hash as initial commit scroll intent", async () => {
		window.history.replaceState({}, "", "/page#section");
		seed_payload({ matched_patterns: ["/page"] });
		const commit = vi.fn();
		const core_res = create_client_core({}, commit, t_opts());
		if (!core_res.ok) {
			throw new Error(`create_client_core failed with error: ${core_res.err}`);
		}
		const core = core_res.val;

		await core.boot({});

		const scroll_intent = route_render_scroll_intent_at(commit, 0);
		expect(scroll_intent?.scroll).toEqual({ hash: "#section" });
	});

	it("allows initial client loaders to submit API requests", async () => {
		let core: any;

		vi.doMock("/mod-boot-resource.js", () => {
			return {
				default: {
					pattern: "/boot-resource",
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
			matched_patterns: ["/boot-resource"],
			import_urls: ["/mod-boot-resource.js"],
		});
		const commit = vi.fn();
		const core_res = create_client_core({}, commit, t_opts());
		if (!core_res.ok) {
			throw new Error(`create_client_core failed with error: ${core_res.err}`);
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
			matched_patterns: ["/boot-submit"],
			import_urls: ["/mod-boot-submit.js"],
		});
		const commit = vi.fn();
		const core_res = create_client_core({}, commit, t_opts());
		if (!core_res.ok) {
			throw new Error(`create_client_core failed with error: ${core_res.err}`);
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
			matched_patterns: ["/", "/boot-router-data"],
			views_data: [{ root: true }, { route: true }],
			import_urls: ["/mod-root.js", "/mod-boot-router-data.js"],
		});
		const commit = vi.fn();
		const core_res = create_client_core({}, commit, t_opts());
		if (!core_res.ok) {
			throw new Error(`create_client_core failed with error: ${core_res.err}`);
		}
		core = core_res.val;

		await core.boot({});

		const state = route_render_commit_at(commit, 0);
		expect(state.entries[1].client_loader_data).toEqual({
			href: window.location.href,
			historyState: undefined,
			clientBuildId: "build-1",
			splatValues: [],
			params: {},
			matches: [
				{
					pattern: "/",
					input: {},
					viewData: { root: true },
					clientLoaderData: undefined,
				},
				{
					pattern: "/boot-router-data",
					input: {},
					viewData: { route: true },
					clientLoaderData: undefined,
				},
			],
			error: null,
		});
	});

	it("extracts initial build ID from data script", async () => {
		seed_payload({ client_build_id: "initial-build" });
		const commit = vi.fn();
		const core_res = create_client_core({}, commit, t_opts());
		if (!core_res.ok) {
			throw new Error(`create_client_core failed with error: ${core_res.err}`);
		}
		const core = core_res.val;

		await core.boot({});

		expect(core.getClientBuildId()).toBe("initial-build");
	});
});

/////////////////////////////////////////////////////////////////////
/////// Title entity decoding
/////////////////////////////////////////////////////////////////////

describe("title entity decoding", () => {
	it("decodes HTML entities in title during boot", async () => {
		seed_payload({
			title: { dangerous_inner_html: "A &amp; B" },
		});
		const commit = vi.fn();
		const core_res = create_client_core({}, commit, t_opts());
		if (!core_res.ok) {
			throw new Error(`create_client_core failed with error: ${core_res.err}`);
		}
		const core = core_res.val;

		await core.boot({});

		expect(document.title).toBe("A & B");
	});

	it("decodes multiple HTML entities in title", async () => {
		seed_payload({
			title: {
				dangerous_inner_html: "&lt;Title&gt; &amp; &quot;More&quot;",
			},
		});
		const commit = vi.fn();
		const core_res = create_client_core({}, commit, t_opts());
		if (!core_res.ok) {
			throw new Error(`create_client_core failed with error: ${core_res.err}`);
		}
		const core = core_res.val;

		await core.boot({});

		expect(document.title).toBe('<Title> & "More"');
	});

	it("handles empty title dangerous_inner_html", async () => {
		seed_payload({
			title: { dangerous_inner_html: "" },
		});
		const commit = vi.fn();
		const core_res = create_client_core({}, commit, t_opts());
		if (!core_res.ok) {
			throw new Error(`create_client_core failed with error: ${core_res.err}`);
		}
		const core = core_res.val;

		await core.boot({});

		expect(document.title).toBe("");
	});
});

/////////////////////////////////////////////////////////////////////
/////// Navigation flow
/////////////////////////////////////////////////////////////////////

describe("build ID", () => {
	it("keeps active build ID stable when a route response has a newer build", async () => {
		seed_payload({ client_build_id: "build-1" });
		const commit = vi.fn();
		const core_res = create_client_core({}, commit, t_opts());
		if (!core_res.ok) {
			throw new Error(`create_client_core failed with error: ${core_res.err}`);
		}
		const core = core_res.val;

		await core.boot({});

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({}, "build-2"),
		);

		await core.navigate("/page");

		expect(core.getClientBuildId()).toBe("build-1");
	});

	it("fires onBuildSkewDetected when a route response has a newer build", async () => {
		seed_payload({ client_build_id: "build-1" });
		const commit = vi.fn();
		const core_res = create_client_core({}, commit, t_opts());
		if (!core_res.ok) {
			throw new Error(`create_client_core failed with error: ${core_res.err}`);
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
				activeClientBuildId: "build-1",
				serverBuildId: "build-2",
				triggeringResponse: expect.objectContaining({
					kind: "route",
					trigger: "navigation",
					requestedHref: `${window.location.origin}/page`,
					status: 200,
					ok: true,
				}),
				currentRouteState: expect.objectContaining({
					clientBuildId: "build-1",
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
		seed_payload({ client_build_id: "build-1" });
		const commit = vi.fn();
		const core_res = create_client_core({}, commit, t_opts());
		if (!core_res.ok) {
			throw new Error(`create_client_core failed with error: ${core_res.err}`);
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

	it("reports build skew context for route hard redirects with a newer build", async () => {
		seed_payload({ client_build_id: "build-1" });
		const commit = vi.fn();
		const core_res = create_client_core({}, commit, t_opts());
		if (!core_res.ok) {
			throw new Error(`create_client_core failed with error: ${core_res.err}`);
		}
		const core = core_res.val;

		const on_build_skew = vi.fn();
		await core.boot({ onBuildSkewDetected: on_build_skew });

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			redirect_response({
				[BUILD_ID_HEADER]: "build-2",
				[X_CLIENT_REDIRECT]: "https://example.com/page",
			}),
		);

		await core.navigate("/page");

		expect(on_build_skew).toHaveBeenCalledWith(
			expect.objectContaining({
				activeClientBuildId: "build-1",
				serverBuildId: "build-2",
				triggeringResponse: expect.objectContaining({
					kind: "route",
					ok: true,
					requestedHref: `${window.location.origin}/page`,
					status: 200,
					trigger: "navigation",
				}),
			}),
		);
	});
});

/////////////////////////////////////////////////////////////////////
/////// Work integration
/////////////////////////////////////////////////////////////////////
