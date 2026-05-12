import { describe, expect, it } from "vitest";
import type { Core6ClientLoaderFn } from "./route_preparation.ts";
import {
	core6_route_publish_reason,
	type Core6RouteCommit,
	type Core6RoutePublication,
	type Core6RouteState,
} from "./route_publication.ts";
import { create_core6_route_publication_store } from "./route_publication_store.ts";

function make_client_loader(name: string): Core6ClientLoaderFn {
	return async () => {
		return { name };
	};
}

function make_route_state(
	overrides: Partial<Core6RouteState> = {},
): Core6RouteState {
	return {
		clientBuildID: "build-1",
		error: null,
		historyState: { from: "history" },
		href: "https://example.test/root",
		matches: [
			{
				clientLoaderData: { client: "root" },
				input: { input: "root" },
				loaderData: { server: "root" },
				pattern: "/root",
			},
		],
		params: { user: "ada" },
		splatValues: ["tail"],
		...overrides,
	};
}

function make_commit(route: Core6RouteState): Core6RouteCommit {
	return {
		route_render: {
			state: {
				client_build_id: route.clientBuildID,
				entries: route.matches.map((match) => {
					return {
						client_loader_data: match.clientLoaderData,
						input: match.input,
						loader_data: match.loaderData,
						module: {},
						module_url: "/root.js",
						pattern: match.pattern,
					};
				}),
				error: route.error,
				history_state: route.historyState,
				params: route.params,
				splat_values: route.splatValues,
			},
		},
		route_update: {
			previous_route: null,
			reason: core6_route_publish_reason.navigation,
			route,
		},
	};
}

function make_publication(input: {
	loaders?: Array<{
		loader: Core6ClientLoaderFn;
		pattern: string;
	}>;
	route?: Core6RouteState;
}): Core6RoutePublication {
	const route = input.route ?? make_route_state();
	return {
		client_loaders: input.loaders ?? [],
		commit: make_commit(route),
		route,
		side_effects: null,
	};
}

function same_route_state(
	previous_route: Core6RouteState,
	next_route: Core6RouteState,
): boolean {
	return JSON.stringify(previous_route) === JSON.stringify(next_route);
}

describe("core6 route publication store", () => {
	it("starts without a current route or registered client loaders", () => {
		const commits: Core6RouteCommit[] = [];
		const store = create_core6_route_publication_store({
			commit_publication: (publication) => {
				commits.push(publication.commit);
			},
		});

		expect(store.current_route()).toBeNull();
		expect(store.current_render_state()).toBeNull();
		expect(store.client_loader("/root")).toBeUndefined();
		expect(store.client_loader_patterns()).toEqual([]);
		expect(commits).toEqual([]);
	});

	it("publishes route state, registers loaders, and emits the commit", () => {
		const root_loader = make_client_loader("root");
		const commits: Core6RouteCommit[] = [];
		const snapshots: Array<{
			current_route: Core6RouteState | null;
			loader: Core6ClientLoaderFn | undefined;
		}> = [];
		let store = create_core6_route_publication_store({
			commit_publication: (publication) => {
				commits.push(publication.commit);
				snapshots.push({
					current_route: store.current_route(),
					loader: store.client_loader("/root"),
				});
			},
		});
		const publication = make_publication({
			loaders: [{ loader: root_loader, pattern: "/root" }],
		});

		store.publish_route(publication);

		expect(commits).toEqual([publication.commit]);
		expect(snapshots).toEqual([
			{
				current_route: publication.route,
				loader: root_loader,
			},
		]);
		expect(store.current_route()).toEqual(publication.route);
		expect(store.client_loader("/root")).toBe(root_loader);
		expect(store.client_loader_patterns()).toEqual(["/root"]);
	});

	it("returns detached current render state snapshots", () => {
		const store = create_core6_route_publication_store({
			commit_publication: () => {},
		});
		const publication = make_publication({});

		store.publish_route(publication);
		const current_render_state = store.current_render_state();
		if (!current_render_state) {
			throw new Error("expected current render state");
		}
		current_render_state.params.user = "grace";
		current_render_state.splat_values.push("mutated");
		current_render_state.entries[0] = {
			client_loader_data: "mutated",
			input: "mutated",
			loader_data: "mutated",
			module: {},
			module_url: "/mutated.js",
			pattern: "/mutated",
		};

		expect(store.current_render_state()).toEqual(
			publication.commit.route_render.state,
		);
	});

	it("returns detached current route snapshots", () => {
		const store = create_core6_route_publication_store({
			commit_publication: () => {},
		});
		const publication = make_publication({});

		store.publish_route(publication);
		const current_route = store.current_route();
		if (!current_route) {
			throw new Error("expected current route");
		}
		current_route.params.user = "grace";
		current_route.splatValues.push("mutated");
		current_route.matches[0] = {
			clientLoaderData: "mutated",
			input: "mutated",
			loaderData: "mutated",
			pattern: "/mutated",
		};

		expect(store.current_route()).toEqual(publication.route);
	});

	it("replaces loaders for already registered patterns", () => {
		const first_loader = make_client_loader("first");
		const second_loader = make_client_loader("second");
		const store = create_core6_route_publication_store({
			commit_publication: () => {},
		});

		store.publish_route(
			make_publication({
				loaders: [{ loader: first_loader, pattern: "/root" }],
			}),
		);
		store.publish_route(
			make_publication({
				loaders: [{ loader: second_loader, pattern: "/root" }],
				route: make_route_state({
					href: "https://example.test/root?next=true",
				}),
			}),
		);

		expect(store.client_loader("/root")).toBe(second_loader);
		expect(store.client_loader_patterns()).toEqual(["/root"]);
		expect(store.current_route()?.href).toBe(
			"https://example.test/root?next=true",
		);
	});

	it("removes loaders for matched patterns when the published route no longer has one", () => {
		const root_loader = make_client_loader("root");
		const store = create_core6_route_publication_store({
			commit_publication: () => {},
		});

		store.publish_route(
			make_publication({
				loaders: [{ loader: root_loader, pattern: "/root" }],
			}),
		);
		store.publish_route(make_publication({}));

		expect(store.client_loader("/root")).toBeUndefined();
		expect(store.client_loader_patterns()).toEqual([]);
	});

	it("returns null for same-document publication before the first route publication", () => {
		const store = create_core6_route_publication_store({
			commit_publication: () => {},
		});

		const publication = store.publish_same_document({
			history_state: { popped: true },
			href: "https://example.test/root#section",
			reason: core6_route_publish_reason.popstate,
			route_state_equal: same_route_state,
			scroll: { hash: "#section" },
		});

		expect(publication).toBeNull();
		expect(store.current_route()).toBeNull();
	});

	it("publishes same-document routes from stored route render state", () => {
		const commits: Core6RouteCommit[] = [];
		const store = create_core6_route_publication_store({
			commit_publication: (publication) => {
				commits.push(publication.commit);
			},
		});
		const first = make_publication({});

		store.publish_route(first);
		const publication = store.publish_same_document({
			history_state: { popped: true },
			href: "https://example.test/root#section",
			reason: core6_route_publish_reason.popstate,
			route_state_equal: same_route_state,
			scroll: { hash: "#section" },
		});

		expect(publication).toMatchObject({
			client_loaders: [],
			commit: {
				route_render: {
					scroll: { hash: "#section" },
					state: {
						history_state: { popped: true },
					},
				},
				route_update: {
					previous_route: first.route,
					reason: core6_route_publish_reason.popstate,
				},
			},
			route: {
				historyState: { popped: true },
				href: "https://example.test/root#section",
			},
		});
		expect(commits).toEqual([first.commit, publication?.commit]);
		expect(store.current_route()).toMatchObject({
			historyState: { popped: true },
			href: "https://example.test/root#section",
		});
	});

	it("bubbles same-document commit emission failures after advancing route state", () => {
		const error = new Error("emit failed");
		let fail_emit = false;
		const root_loader = make_client_loader("root");
		const store = create_core6_route_publication_store({
			commit_publication: () => {
				if (fail_emit) {
					throw error;
				}
			},
		});
		store.publish_route(
			make_publication({
				loaders: [{ loader: root_loader, pattern: "/root" }],
			}),
		);

		fail_emit = true;
		expect(() => {
			store.publish_same_document({
				history_state: { popped: true },
				href: "https://example.test/root#failed",
				reason: core6_route_publish_reason.popstate,
				route_state_equal: same_route_state,
				scroll: { hash: "#failed" },
			});
		}).toThrow(error);
		expect(store.current_route()).toMatchObject({
			historyState: { popped: true },
			href: "https://example.test/root#failed",
		});
		expect(store.client_loader("/root")).toBe(root_loader);
	});

	it("bubbles commit emission failures after advancing publication state", () => {
		const error = new Error("emit failed");
		const root_loader = make_client_loader("root");
		const store = create_core6_route_publication_store({
			commit_publication: () => {
				throw error;
			},
		});
		const publication = make_publication({
			loaders: [{ loader: root_loader, pattern: "/root" }],
		});

		expect(() => {
			store.publish_route(publication);
		}).toThrow(error);
		expect(store.current_route()).toEqual(publication.route);
		expect(store.client_loader("/root")).toBe(root_loader);
	});
});
