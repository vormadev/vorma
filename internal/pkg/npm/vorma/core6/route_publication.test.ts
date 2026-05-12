import { describe, expect, it } from "vitest";
import {
	core6_route_error_source,
	type Core6ClientLoaderFn,
	type Core6PreparedRoute,
	type Core6RouteModule,
} from "./route_preparation.ts";
import {
	build_core6_route_publication,
	build_core6_same_document_route_publication,
	core6_route_publish_reason,
	publish_core6_route,
	type Core6RoutePublication,
	type Core6RouteState,
} from "./route_publication.ts";

function make_prepared_route(
	overrides: Partial<Core6PreparedRoute> = {},
): Core6PreparedRoute {
	const root_module = {
		default: {},
		name: "root",
	} satisfies Core6RouteModule;
	const child_module = {
		default: {},
		name: "child",
	} satisfies Core6RouteModule;
	const root_loader: Core6ClientLoaderFn = async () => {
		return { root: true };
	};
	const child_loader: Core6ClientLoaderFn = async () => {
		return { child: true };
	};

	return {
		client_build_id: "build-1",
		client_loaders: [
			{ loader: root_loader, pattern: "/root" },
			{ loader: child_loader, pattern: "/root/child" },
		],
		css_bundles: ["/root.css"],
		deps: ["/dep.js"],
		error: null,
		history_state: { history: "state" },
		href: "https://example.test/root/child?q=Ada",
		matches: [
			{
				client_loader_data: { client: "root" },
				input: { q: "Ada" },
				loader_data: { server: "root" },
				module: root_module,
				module_url: "/root.js",
				pattern: "/root",
			},
			{
				client_loader_data: { client: "child" },
				input: { q: "Ada", child: true },
				loader_data: { server: "child" },
				module: child_module,
				module_url: "/child.js",
				pattern: "/root/child",
			},
		],
		meta_head_els: [{ tag: "meta" }],
		params: { user: "ada" },
		rest_head_els: [{ tag: "link" }],
		splat_values: ["tail"],
		title_html: "Profile",
		...overrides,
	};
}

function same_route_state(
	previous_route: Core6RouteState,
	next_route: Core6RouteState,
): boolean {
	return JSON.stringify(previous_route) === JSON.stringify(next_route);
}

describe("core6 route publication", () => {
	it("builds route state and render state from prepared route data", () => {
		const prepared = make_prepared_route({
			error: {
				error: "child boom",
				idx: 1,
				source: core6_route_error_source.server,
			},
		});

		const publication = build_core6_route_publication({
			prepared,
			previous_route: null,
			reason: core6_route_publish_reason.boot,
			route_state_equal: same_route_state,
		});

		expect(publication.route).toEqual({
			clientBuildID: "build-1",
			error: {
				error: "child boom",
				idx: 1,
				source: core6_route_error_source.server,
			},
			historyState: { history: "state" },
			href: "https://example.test/root/child?q=Ada",
			matches: [
				{
					clientLoaderData: { client: "root" },
					input: { q: "Ada" },
					loaderData: { server: "root" },
					pattern: "/root",
				},
				{
					clientLoaderData: { client: "child" },
					input: { q: "Ada", child: true },
					loaderData: { server: "child" },
					pattern: "/root/child",
				},
			],
			params: { user: "ada" },
			splatValues: ["tail"],
		});
		expect(publication.commit.route_render.state).toEqual({
			client_build_id: "build-1",
			entries: [
				{
					client_loader_data: { client: "root" },
					input: { q: "Ada" },
					loader_data: { server: "root" },
					module: prepared.matches[0]!.module,
					module_url: "/root.js",
					pattern: "/root",
				},
				{
					client_loader_data: { client: "child" },
					input: { q: "Ada", child: true },
					loader_data: { server: "child" },
					module: prepared.matches[1]!.module,
					module_url: "/child.js",
					pattern: "/root/child",
				},
			],
			error: {
				error: "child boom",
				idx: 1,
				source: core6_route_error_source.server,
			},
			history_state: { history: "state" },
			params: { user: "ada" },
			splat_values: ["tail"],
		});
		expect(publication.commit.route_update).toEqual({
			previous_route: null,
			reason: core6_route_publish_reason.boot,
			route: publication.route,
		});
		expect(publication.history).toBeUndefined();
		expect(publication.side_effects).toEqual({
			css_bundles: ["/root.css"],
			deps: ["/dep.js"],
			meta_head_els: [{ tag: "meta" }],
			rest_head_els: [{ tag: "link" }],
			title_html: "Profile",
		});
		expect(publication.client_loaders).toEqual(prepared.client_loaders);
	});

	it("omits route update when the next route equals the previous route", () => {
		const prepared = make_prepared_route();
		const first = build_core6_route_publication({
			prepared,
			previous_route: null,
			reason: core6_route_publish_reason.boot,
			route_state_equal: same_route_state,
		});
		const equality_calls: Array<{
			next_route: Core6RouteState;
			previous_route: Core6RouteState;
		}> = [];

		const second = build_core6_route_publication({
			prepared,
			previous_route: first.route,
			reason: core6_route_publish_reason.revalidation,
			route_state_equal: (previous_route, next_route) => {
				equality_calls.push({ next_route, previous_route });
				return true;
			},
		});

		expect(equality_calls).toEqual([
			{ next_route: second.route, previous_route: first.route },
		]);
		expect(second.commit.route_update).toBeUndefined();
		expect(second.commit.route_render.state.entries).toHaveLength(2);
	});

	it("threads history mutation intent through the publication boundary", () => {
		const publication = build_core6_route_publication({
			history: {
				href: "https://example.test/input",
				replace: true,
				state: { stale: true },
			},
			prepared: make_prepared_route({
				history_state: { current: true },
				href: "https://example.test/final",
			}),
			previous_route: null,
			reason: core6_route_publish_reason.navigation,
			route_state_equal: same_route_state,
		});

		expect(publication.history).toEqual({
			href: "https://example.test/final",
			replace: true,
			state: { current: true },
		});
	});

	it("can suppress prepared side effects for internal republish paths", () => {
		const publication = build_core6_route_publication({
			apply_side_effects: false,
			prepared: make_prepared_route(),
			previous_route: null,
			reason: core6_route_publish_reason.revalidation,
			route_state_equal: same_route_state,
		});

		expect(publication.side_effects).toBeNull();
	});

	it("builds same-document publications from the current route render snapshot", () => {
		const prepared = make_prepared_route();
		const current = build_core6_route_publication({
			prepared,
			previous_route: null,
			reason: core6_route_publish_reason.boot,
			route_state_equal: same_route_state,
		});

		const publication = build_core6_same_document_route_publication({
			current_render_state: current.commit.route_render.state,
			current_route: current.route,
			href: "https://example.test/root/child?q=Ada#section",
			history_state: { popped: true },
			reason: core6_route_publish_reason.popstate,
			route_state_equal: same_route_state,
			scroll: { hash: "#section" },
		});

		expect(publication.client_loaders).toEqual([]);
		expect(publication.route).toEqual({
			...current.route,
			historyState: { popped: true },
			href: "https://example.test/root/child?q=Ada#section",
		});
		expect(publication.route).not.toBe(current.route);
		expect(publication.route.matches).not.toBe(current.route.matches);
		expect(publication.commit.route_render).toMatchObject({
			scroll: { hash: "#section" },
			state: {
				history_state: { popped: true },
			},
		});
		expect(publication.commit.route_render.state).not.toBe(
			current.commit.route_render.state,
		);
		expect(publication.commit.route_render.state.entries[0]?.module).toBe(
			prepared.matches[0]!.module,
		);
		expect(publication.commit.route_update).toEqual({
			previous_route: current.route,
			reason: core6_route_publish_reason.popstate,
			route: publication.route,
		});
		expect(publication.history).toBeUndefined();
		expect(publication.side_effects).toBeNull();
	});

	it("omits same-document route updates when the retargeted route compares equal", () => {
		const current = build_core6_route_publication({
			prepared: make_prepared_route(),
			previous_route: null,
			reason: core6_route_publish_reason.boot,
			route_state_equal: same_route_state,
		});

		const publication = build_core6_same_document_route_publication({
			current_render_state: current.commit.route_render.state,
			current_route: current.route,
			href: "https://example.test/root/child?q=Ada#ignored",
			history_state: { ignored: true },
			reason: core6_route_publish_reason.navigation,
			route_state_equal: () => {
				return true;
			},
			scroll: { x: 3, y: 5 },
		});

		expect(publication.commit.route_update).toBeUndefined();
		expect(publication.commit.route_render.scroll).toEqual({ x: 3, y: 5 });
	});

	it("keeps route and render containers detached from prepared route containers", () => {
		const prepared = make_prepared_route();
		const publication = build_core6_route_publication({
			prepared,
			previous_route: null,
			reason: core6_route_publish_reason.navigation,
			route_state_equal: same_route_state,
		});

		publication.route.params.user = "grace";
		publication.route.splatValues.push("mutated");
		publication.commit.route_render.state.params.user = "katherine";
		publication.commit.route_render.state.splat_values.push("rendered");

		expect(prepared.params).toEqual({ user: "ada" });
		expect(prepared.splat_values).toEqual(["tail"]);
	});

	it("publishes a single synchronous publication object for host-owned durable effects", () => {
		const prepared = make_prepared_route();
		const publications: Core6RoutePublication[] = [];

		const publication = publish_core6_route({
			host: {
				publish_route: (next_publication) => {
					publications.push(next_publication);
				},
			},
			prepared,
			previous_route: null,
			reason: core6_route_publish_reason.navigation,
			route_state_equal: same_route_state,
		});

		expect(publications).toEqual([publication]);
		expect(publications[0]!.client_loaders).toEqual(
			prepared.client_loaders,
		);
		expect(publications[0]!.commit.route_update?.reason).toBe(
			core6_route_publish_reason.navigation,
		);
	});

	it("lets host publish errors bubble to the transaction boundary", () => {
		const error = new Error("publish failed");

		expect(() => {
			publish_core6_route({
				host: {
					publish_route: () => {
						throw error;
					},
				},
				prepared: make_prepared_route(),
				previous_route: null,
				reason: core6_route_publish_reason.navigation,
				route_state_equal: same_route_state,
			});
		}).toThrow(error);
	});
});
