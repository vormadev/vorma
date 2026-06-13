// @vitest-environment jsdom

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
	deferred,
	has_route_render_commit,
	mock_fetch,
	register_ccc_lifecycle,
	route_render_commit_at,
	route_render_commit_count,
	route_response,
	seed_payload,
	setup,
	t_opts,
} from "./_test_helpers.ts";
import { create_client_core } from "./create_client_core.ts";

register_ccc_lifecycle(beforeEach, afterEach);

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
		const core_res = create_client_core({}, commit, t_opts());
		if (!core_res.ok) {
			throw new Error(`create_client_core failed with error: ${core_res.err}`);
		}
		const core = core_res.val;

		await core.boot({});

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({
				matched_patterns: ["/hmr"],
				views_data: [{ v: 1 }],
				import_urls: ["/hmr-mod.js"],
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

		await window.__vorma_hmr_view_update?.("/hmr-mod.js", new_mod);

		expect(has_route_render_commit(commit)).toBe(true);
		const state = route_render_commit_at(commit, 0);
		expect(state.entries[0].module).toBe(new_mod);
		expect(state.entries[0].hmr_version).toBe(1);
	});

	it("re-runs client loader only for opted-in patterns", async () => {
		let client_loader_run_count = 0;

		vi.doMock("/hmr-cl-mod.js", () => {
			return {
				default: {
					pattern: "/hmr-cl",
					component: () => {
						return null;
					},
					client_loader: async ({ serverPromise }: any) => {
						client_loader_run_count++;
						await serverPromise;
						return { run: client_loader_run_count };
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

		// defineView with runClientLoaderOnHmr: true
		core.defineView({
			pattern: "/hmr-cl",
			component: () => {
				return null;
			},
			clientLoader: async ({ serverPromise }: any) => {
				client_loader_run_count++;
				await serverPromise;
				return { run: client_loader_run_count };
			},
			runClientLoaderOnHmr: true,
		});

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({
				matched_patterns: ["/hmr-cl"],
				views_data: [{ v: 1 }],
				import_urls: ["/hmr-cl-mod.js"],
			}),
		);

		await core.navigate("/hmr-cl");
		const runs_after_nav = client_loader_run_count;
		commit.mockClear();

		const new_mod = {
			default: {
				pattern: "/hmr-cl",
				component: () => {
					return null;
				},
				client_loader: async ({ serverPromise }: any) => {
					client_loader_run_count++;
					await serverPromise;
					return { run: client_loader_run_count };
				},
			},
		};

		await window.__vorma_hmr_view_update?.("/hmr-cl-mod.js", new_mod);

		expect(client_loader_run_count).toBeGreaterThan(runs_after_nav);
	});

	it("does not publish an HMR update after the inspected route changes", async () => {
		vi.doMock("/hmr-stale.js", () => {
			return {
				default: {
					pattern: "/hmr-stale",
					component: () => {
						return null;
					},
					client_loader: async ({ serverPromise }: any) => {
						await serverPromise;
						return { initial: true };
					},
				},
			};
		});
		vi.doMock("/hmr-other.js", () => {
			return {
				default: {
					pattern: "/other",
					component: () => {
						return null;
					},
				},
			};
		});

		const { core, commit } = await setup();
		core.defineView({
			pattern: "/hmr-stale",
			component: () => {
				return null;
			},
			clientLoader: async () => {
				return {};
			},
			runClientLoaderOnHmr: true,
		});
		const fetcher = mock_fetch();

		const first_nav = core.navigate("/hmr-stale");
		await fetcher.wait_for(1);
		fetcher.call(0).resolve(
			route_response({
				matched_patterns: ["/hmr-stale"],
				views_data: [{ initial: true }],
				import_urls: ["/hmr-stale.js"],
			}),
		);
		await first_nav;
		commit.mockClear();

		const hmr_loader_started = deferred<void>();
		const hmr_loader_release = deferred<void>();
		const hmr_done = window.__vorma_hmr_view_update?.("/hmr-stale.js", {
			default: {
				pattern: "/hmr-stale",
				component: () => {
					return null;
				},
				client_loader: async ({ serverPromise }: any) => {
					await serverPromise;
					hmr_loader_started.resolve();
					await hmr_loader_release.promise;
					return { hmr: true };
				},
			},
		});
		if (!hmr_done) {
			throw new Error("Expected Vorma HMR callback to be registered");
		}
		await hmr_loader_started.promise;

		const other_nav = core.navigate("/other");
		await fetcher.wait_for(2);
		fetcher.call(1).resolve(
			route_response({
				matched_patterns: ["/other"],
				views_data: [{ other: true }],
				import_urls: ["/hmr-other.js"],
			}),
		);
		await other_nav;

		expect(route_render_commit_count(commit)).toBe(1);
		hmr_loader_release.resolve();
		await hmr_done;

		expect(route_render_commit_count(commit)).toBe(1);
		expect(
			route_render_commit_at(commit, 0).entries.map((entry) => {
				return entry.pattern;
			}),
		).toEqual(["/other"]);
	});

	it("does not re-run client loader when not opted in", async () => {
		let client_loader_run_count = 0;

		vi.doMock("/hmr-no-rerun.js", () => {
			return {
				default: {
					pattern: "/hmr-no-rerun",
					component: () => {
						return null;
					},
					client_loader: async ({ serverPromise }: any) => {
						client_loader_run_count++;
						await serverPromise;
						return { run: client_loader_run_count };
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
				matched_patterns: ["/hmr-no-rerun"],
				views_data: [{ v: 1 }],
				import_urls: ["/hmr-no-rerun.js"],
			}),
		);

		await core.navigate("/hmr-no-rerun");
		const runs_after_nav = client_loader_run_count;
		commit.mockClear();

		const new_mod = {
			default: {
				pattern: "/hmr-no-rerun",
				component: () => {
					return null;
				},
				client_loader: async ({ serverPromise }: any) => {
					client_loader_run_count++;
					await serverPromise;
					return { run: client_loader_run_count };
				},
			},
		};

		await window.__vorma_hmr_view_update?.("/hmr-no-rerun.js", new_mod);

		expect(client_loader_run_count).toBe(runs_after_nav);
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
		const core_res = create_client_core({}, commit, t_opts());
		if (!core_res.ok) {
			throw new Error(`create_client_core failed with error: ${core_res.err}`);
		}
		const core = core_res.val;

		await core.boot({});

		vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
			route_response({
				matched_patterns: ["/hmr-match"],
				views_data: [{ v: 1 }],
				import_urls: ["/hmr-match.js"],
			}),
		);

		await core.navigate("/hmr-match");
		commit.mockClear();

		await window.__vorma_hmr_view_update?.("/totally-different.js", {
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
		const core_res = create_client_core({}, commit, t_opts());
		if (!core_res.ok) {
			throw new Error(`create_client_core failed with error: ${core_res.err}`);
		}
		const core = core_res.val;

		const comp = () => {
			return null;
		};
		const boundary = () => {
			return null;
		};
		const client_loader_fn = async () => {
			return { data: true };
		};

		const def = core.defineView({
			pattern: "/test",
			component: comp,
			errorBoundary: boundary,
			clientLoader: client_loader_fn,
		});

		expect(def.pattern).toBe("/test");
		expect(def.component).toBe(comp);
		expect(def.error_boundary).toBe(boundary);
		expect(def.client_loader).toBe(client_loader_fn);
	});

	it("returns definition without optional fields", () => {
		seed_payload();
		const commit = vi.fn();
		const core_res = create_client_core({}, commit, t_opts());
		if (!core_res.ok) {
			throw new Error(`create_client_core failed with error: ${core_res.err}`);
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
