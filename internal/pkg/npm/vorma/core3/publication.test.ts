import { describe, expect, it } from "vitest";
import { route_hrefs_share_document } from "./href.ts";
import type {
	CoreEffect,
	PreparedRoute,
	PublicationTransaction,
	RouteFacts,
	RouteRenderFacts,
	RouteSnapshot,
} from "./model.ts";
import {
	derive_scroll_intent,
	plan_route_publication,
	plan_same_document_publication,
	plan_same_document_route,
	publication_effects,
	route_publication_commit,
	route_update_required,
	type RoutePublicationOperation,
} from "./publication.ts";
import {
	prepared_route_at_position,
	route_facts_at_position,
	route_facts_to_state,
	route_render_facts_at_position,
} from "./route.ts";

const route = {
	matches: [
		{
			client_loader_data: undefined,
			input: undefined,
			loader_data: undefined,
			module: {},
			module_url: "/root.js",
			pattern: "/",
		},
		{
			client_loader_data: undefined,
			input: undefined,
			loader_data: undefined,
			module: {},
			module_url: "/items.js",
			pattern: "/items/:id",
		},
	],
} satisfies Pick<RouteFacts, "matches">;

const target_route_id = `${route.matches.length - 1}:${
	route.matches[route.matches.length - 1]?.pattern ?? ""
}`;

const route_facts = {
	client_build_id: "build-1",
	error: null,
	history_state: { old: true },
	href: "https://example.com/old",
	matches: route.matches,
	params: { id: "1" },
	splat_values: [],
} satisfies RouteFacts;

const render_facts = {
	client_build_id: "build-1",
	entries: route.matches,
	error: null,
	history_state: { old: true },
	params: { id: "1" },
	splat_values: [],
} satisfies RouteRenderFacts;

const previous_snapshot = {
	position: {
		href: "https://example.com/items/1#old",
		key: "browser-1",
		state: { old: true },
	},
	provisional: false,
	render: {
		...render_facts,
		history_state: { old: true },
	},
	route: {
		...route_facts,
		history_state: { old: true },
		href: "https://example.com/items/1#old",
	},
} satisfies RouteSnapshot;

describe("derive_scroll_intent", () => {
	it("keeps reload scroll ahead of boot hash scroll", () => {
		const scroll_intent = derive_scroll_intent(
			{
				browser_key: "browser-1",
				href: "https://example.com/items/1#details",
				id: "boot-1",
				kind: "boot",
				public_call_ids: ["call-1"],
				reload_scroll: { x: 11, y: 22 },
				rights: ["publish_route"],
				state: undefined,
				status: "prepared",
			},
			route,
		);

		expect(scroll_intent).toEqual({
			scroll: { x: 11, y: 22 },
			target_route_id,
		});
	});

	it("targets the deepest route for hash navigations", () => {
		const scroll_intent = derive_scroll_intent(
			{
				browser_key: "browser-2",
				href: "https://example.com/items/1#details",
				id: "nav-1",
				kind: "navigation",
				public_call_ids: ["call-1"],
				redirect_count: 0,
				replace: false,
				rights: ["publish_route"],
				source: "navigate",
				state: undefined,
				status: "prepared",
				skip_work_indicator: false,
			},
			route,
		);

		expect(scroll_intent).toEqual({
			scroll: { hash: "#details" },
			target_route_id,
		});
	});

	it("allows navigation callers to suppress top scroll", () => {
		const scroll_intent = derive_scroll_intent(
			{
				browser_key: "browser-3",
				href: "https://example.com/items/1",
				id: "nav-2",
				kind: "navigation",
				public_call_ids: ["call-1"],
				redirect_count: 0,
				replace: false,
				rights: ["publish_route"],
				scroll_to_top: false,
				source: "navigate",
				state: undefined,
				status: "prepared",
				skip_work_indicator: false,
			},
			route,
		);

		expect(scroll_intent).toBeUndefined();
	});

	it("defaults popstate scroll to the top when no stored position exists", () => {
		const scroll_intent = derive_scroll_intent(
			{
				browser: {
					href: "https://example.com/items/1",
					key: "browser-4",
					state: undefined,
				},
				id: "pop-1",
				kind: "popstate",
				leaving_scroll: { x: 3, y: 4 },
				public_call_ids: [],
				rights: ["publish_route"],
				status: "prepared",
			},
			route,
		);

		expect(scroll_intent).toEqual({
			scroll: { x: 0, y: 0 },
			target_route_id,
		});
	});
});

describe("publication position facts", () => {
	it("retargets route facts to the publication browser position", () => {
		const retargeted = route_facts_at_position(route_facts, {
			href: "https://example.com/items/1",
			state: { next: true },
		});

		expect(retargeted).toEqual({
			...route_facts,
			history_state: { next: true },
			href: "https://example.com/items/1",
		});
	});

	it("retargets render facts without touching route data", () => {
		const retargeted = route_render_facts_at_position(render_facts, {
			state: { next: true },
		});

		expect(retargeted).toEqual({
			...render_facts,
			history_state: { next: true },
		});
	});

	it("retargets prepared routes as inert data", () => {
		const prepared = {
			dom: { title: "old" },
			render: render_facts,
			route: route_facts,
		} satisfies PreparedRoute;
		const retargeted = prepared_route_at_position(prepared, {
			href: "https://example.com/items/1",
			state: { next: true },
		});

		expect(retargeted).toEqual({
			dom: prepared.dom,
			render: {
				...render_facts,
				history_state: { next: true },
			},
			route: {
				...route_facts,
				history_state: { next: true },
				href: "https://example.com/items/1",
			},
		});
	});
});

describe("route publication commits", () => {
	it("converts internal route facts to public route state", () => {
		expect(route_facts_to_state(route_facts)).toEqual({
			clientBuildID: "build-1",
			error: null,
			historyState: { old: true },
			href: "https://example.com/old",
			matches: [
				{
					clientLoaderData: undefined,
					input: undefined,
					loaderData: undefined,
					pattern: "/",
				},
				{
					clientLoaderData: undefined,
					input: undefined,
					loaderData: undefined,
					pattern: "/items/:id",
				},
			],
			params: { id: "1" },
			splatValues: [],
		});
	});

	it("compares the public route state instead of module identity", () => {
		const next_route = {
			...route_facts,
			matches: route_facts.matches.map((match) => {
				return {
					...match,
					module: { changed: true },
					module_url: `${match.module_url}?next`,
				};
			}),
		} satisfies RouteFacts;

		expect(
			route_update_required(
				route_facts_to_state(route_facts),
				route_facts_to_state(next_route),
			),
		).toBe(false);
		expect(
			route_update_required(
				route_facts_to_state(route_facts),
				route_facts_to_state({
					...next_route,
					history_state: { next: true },
				}),
			),
		).toBe(true);
	});

	it("omits route_update when revalidation leaves route state unchanged", () => {
		const commit = route_publication_commit({
			previous: previous_snapshot,
			reason: "revalidation",
			render: previous_snapshot.render,
			route: previous_snapshot.route,
		});

		expect(commit.route_update).toBeUndefined();
		expect(commit.route_render).toEqual({
			scroll_intent: undefined,
			state: previous_snapshot.render,
		});
	});
});

describe("publication_effects", () => {
	const transaction = {
		after_transition: [
			{
				public_call_id: "call-1",
				result: { didNavigate: true },
				type: "resolve_public_call",
			},
		],
		before_transition: [{ type: "save_current_scroll" }],
		inside_transition: [{ type: "render" }],
		operation_id: "nav-1",
		reason: "navigation",
	} satisfies PublicationTransaction;

	const operation = {
		browser_key: "browser-2",
		href: "https://example.com/items/1",
		id: "nav-1",
		kind: "navigation",
		public_call_ids: ["call-1"],
		redirect_count: 0,
		replace: false,
		rights: ["own_visible_transition", "publish_route"],
		source: "navigate",
		state: undefined,
		status: "publishing",
		skip_work_indicator: false,
	} satisfies RoutePublicationOperation;

	it("flattens publication phases when view transitions are unavailable", () => {
		expect(
			publication_effects({
				operation,
				transaction,
				use_view_transitions: true,
				view_transition_available: false,
			}),
		).toEqual([
			{ type: "save_current_scroll" },
			{ type: "render" },
			{
				public_call_id: "call-1",
				result: { didNavigate: true },
				type: "resolve_public_call",
			},
		]);
	});

	it("wraps publication phases when the operation owns a visible transition", () => {
		expect(
			publication_effects({
				operation,
				transaction,
				use_view_transitions: true,
				view_transition_available: true,
			}),
		).toEqual([
			{
				transaction,
				type: "run_view_transition",
			},
		]);
	});

	it("does not wrap operations without visible transition rights", () => {
		expect(
			publication_effects({
				operation: {
					...operation,
					rights: ["publish_route"],
				},
				transaction,
				use_view_transitions: true,
				view_transition_available: true,
			}),
		).toEqual([
			{ type: "save_current_scroll" },
			{ type: "render" },
			{
				public_call_id: "call-1",
				result: { didNavigate: true },
				type: "resolve_public_call",
			},
		]);
	});
});

describe("plan_same_document_publication", () => {
	it("recognizes hrefs that share a document but not a search", () => {
		expect(
			route_hrefs_share_document(
				"https://example.com/items/1#old",
				"https://example.com/items/1#new",
			),
		).toBe(true);
		expect(
			route_hrefs_share_document(
				"https://example.com/items/1?tab=a#old",
				"https://example.com/items/1?tab=b#old",
			),
		).toBe(false);
	});

	it("plans hash navigation as publication without prepared route data", () => {
		const plan = plan_same_document_publication({
			operation: {
				browser_key: "browser-2",
				href: "https://example.com/items/1#details",
				id: "nav-1",
				kind: "navigation",
				public_call_ids: ["call-1"],
				redirect_count: 0,
				replace: false,
				rights: ["publish_route"],
				source: "navigate",
				state: { next: true },
				status: "publishable",
				skip_work_indicator: false,
			},
			previous: previous_snapshot,
		});
		const commit_effect = plan?.transaction.inside_transition[1] as
			| Extract<CoreEffect, { type: "commit" }>
			| undefined;

		expect(plan?.did_navigate).toBe(true);
		expect(plan?.transaction.before_transition).toEqual([
			{ type: "save_current_scroll" },
		]);
		expect(plan?.transaction.inside_transition[0]).toEqual({
			position: {
				href: "https://example.com/items/1#details",
				key: "browser-2",
				state: { next: true },
			},
			replace: false,
			type: "write_history",
		});
		expect(commit_effect?.commit.route_render.scroll_intent).toEqual({
			scroll: { hash: "#details" },
			target_route_id,
		});
		expect(commit_effect?.commit.route_update).toEqual({
			previous_route: route_facts_to_state(previous_snapshot.route),
			reason: "navigation",
			route: route_facts_to_state({
				...previous_snapshot.route,
				history_state: { next: true },
				href: "https://example.com/items/1#details",
			}),
		});
		expect(plan?.transaction.after_transition).toEqual([
			{
				public_call_id: "call-1",
				result: { didNavigate: true },
				type: "resolve_public_call",
			},
		]);
	});

	it("keeps replace-with-same-hash as a publication without navigation", () => {
		const plan = plan_same_document_publication({
			operation: {
				browser_key: "browser-2",
				href: "https://example.com/items/1#old",
				id: "nav-2",
				kind: "navigation",
				public_call_ids: ["call-2"],
				redirect_count: 0,
				replace: true,
				rights: ["publish_route"],
				source: "navigate",
				state: { old: true },
				status: "publishable",
				skip_work_indicator: false,
			},
			previous: previous_snapshot,
		});
		const commit_effect = plan?.transaction.inside_transition[1] as
			| Extract<CoreEffect, { type: "commit" }>
			| undefined;

		expect(plan?.did_navigate).toBe(false);
		expect(plan?.transaction.before_transition).toEqual([]);
		expect(commit_effect?.commit.route_render.scroll_intent).toEqual({
			scroll: { x: 0, y: 0 },
			target_route_id,
		});
		expect(commit_effect?.commit.route_update).toBeUndefined();
		expect(plan?.transaction.after_transition).toEqual([
			{
				public_call_id: "call-2",
				result: { didNavigate: false },
				type: "resolve_public_call",
			},
		]);
	});

	it("models same-hash navigation as scroll-only completion", () => {
		const plan = plan_same_document_route({
			operation: {
				browser_key: "browser-2",
				href: "https://example.com/items/1#old",
				id: "nav-3",
				kind: "navigation",
				public_call_ids: ["call-3"],
				redirect_count: 0,
				replace: false,
				rights: [],
				source: "navigate",
				state: { old: true },
				status: "publishable",
				skip_work_indicator: false,
			},
			previous: previous_snapshot,
		});

		expect(plan).toEqual({
			did_navigate: false,
			effects: [
				{
					scroll: { x: 0, y: 0 },
					type: "apply_scroll",
				},
				{
					public_call_id: "call-3",
					result: { didNavigate: false },
					type: "resolve_public_call",
				},
			],
			kind: "scroll_only",
			next: previous_snapshot,
		});
	});

	it("plans same-document popstate without history writes", () => {
		const plan = plan_same_document_publication({
			operation: {
				browser: {
					href: "https://example.com/items/1#details",
					key: "browser-2",
					state: { next: true },
				},
				id: "pop-1",
				kind: "popstate",
				leaving_scroll: { x: 10, y: 20 },
				public_call_ids: [],
				rights: ["publish_route"],
				status: "publishable",
			},
			previous: previous_snapshot,
		});

		expect(plan?.transaction.before_transition).toEqual([
			{
				key: "browser-1",
				scroll: { x: 10, y: 20 },
				type: "save_scroll_position",
			},
		]);
		expect(plan?.transaction.inside_transition[0]).toEqual({
			commit: {
				route_render: {
					scroll_intent: {
						scroll: { hash: "#details" },
						target_route_id,
					},
					state: {
						...previous_snapshot.render,
						history_state: { next: true },
					},
				},
				route_update: {
					previous_route: route_facts_to_state(
						previous_snapshot.route,
					),
					reason: "popstate",
					route: route_facts_to_state({
						...previous_snapshot.route,
						history_state: { next: true },
						href: "https://example.com/items/1#details",
					}),
				},
			},
			next: plan?.next,
			operation_id: "pop-1",
			type: "commit",
		});
	});
});

describe("plan_route_publication", () => {
	it("does not plan publication without publication rights", () => {
		const plan = plan_route_publication({
			browser: null,
			operation: {
				browser_key: "browser-2",
				href: "https://example.com/items/1",
				id: "nav-1",
				kind: "navigation",
				public_call_ids: ["call-1"],
				redirect_count: 0,
				replace: false,
				rights: [],
				source: "navigate",
				state: undefined,
				status: "publishable",
				skip_work_indicator: false,
			},
			prepared: {
				dom: { title: "next" },
				render: render_facts,
				route: route_facts,
			},
			previous: null,
		});

		expect(plan).toBeUndefined();
	});

	it("plans navigation publication as ordered transaction data", () => {
		const plan = plan_route_publication({
			browser: null,
			operation: {
				browser_key: "browser-2",
				href: "https://example.com/items/1#details",
				id: "nav-1",
				kind: "navigation",
				public_call_ids: ["call-1"],
				redirect_count: 0,
				replace: false,
				rights: ["publish_route"],
				source: "navigate",
				state: { next: true },
				status: "publishable",
				skip_work_indicator: false,
			},
			prepared: {
				dom: { title: "next" },
				render: render_facts,
				route: route_facts,
			},
			previous: null,
		});

		expect(plan?.next.position).toEqual({
			href: "https://example.com/items/1#details",
			key: "browser-2",
			state: { next: true },
		});
		expect(plan?.transaction.before_transition).toEqual([
			{ type: "save_current_scroll" },
		]);
		expect(plan?.transaction.inside_transition[0]).toEqual({
			position: {
				href: "https://example.com/items/1#details",
				key: "browser-2",
				state: { next: true },
			},
			replace: false,
			type: "write_history",
		});
		expect(plan?.transaction.after_transition).toEqual([
			{
				public_call_id: "call-1",
				result: { didNavigate: true },
				type: "resolve_public_call",
			},
		]);
	});

	it("plans boot listener installation after publication", () => {
		const plan = plan_route_publication({
			browser: null,
			operation: {
				browser_key: "browser-1",
				href: "https://example.com/items/1",
				id: "boot-1",
				kind: "boot",
				public_call_ids: ["boot-call"],
				rights: ["publish_route"],
				state: undefined,
				status: "publishable",
			},
			prepared: {
				dom: { title: "boot" },
				render: render_facts,
				route: route_facts,
			},
			previous: null,
		});

		expect(plan?.transaction.before_transition).toEqual([]);
		expect(plan?.transaction.inside_transition[0]).toEqual({
			prepared: {
				dom: { title: "boot" },
				render: {
					...render_facts,
					history_state: undefined,
				},
				route: {
					...route_facts,
					history_state: undefined,
					href: "https://example.com/items/1",
				},
			},
			type: "apply_publication_dom",
		});
		expect(plan?.transaction.after_transition).toEqual([
			{ type: "install_browser_listeners" },
			{
				public_call_id: "boot-call",
				result: undefined,
				type: "resolve_public_call",
			},
		]);
	});

	it("requires browser position for route revalidation publication", () => {
		const missing_browser_plan = plan_route_publication({
			browser: null,
			operation: {
				attempt: 0,
				href: "https://example.com/items/1",
				id: "reval-1",
				kind: "route_revalidation",
				public_call_ids: ["reval-call"],
				reason: "manual",
				rights: ["publish_route", "satisfy_refresh_demand"],
				status: "publishable",
			},
			prepared: {
				dom: { title: "refresh" },
				render: render_facts,
				route: route_facts,
			},
			previous: null,
		});
		const plan = plan_route_publication({
			browser: {
				href: "https://example.com/items/1",
				key: "browser-1",
				state: { current: true },
			},
			operation: {
				attempt: 0,
				href: "https://example.com/items/1",
				id: "reval-1",
				kind: "route_revalidation",
				public_call_ids: ["reval-call"],
				reason: "manual",
				rights: ["publish_route", "satisfy_refresh_demand"],
				status: "publishable",
			},
			prepared: {
				dom: { title: "refresh" },
				render: render_facts,
				route: route_facts,
			},
			previous: null,
		});

		expect(missing_browser_plan).toBeUndefined();
		expect(plan?.transaction.reason).toBe("revalidation");
		expect(plan?.transaction.before_transition).toEqual([]);
		expect(plan?.transaction.inside_transition[0]).toEqual({
			prepared: {
				dom: { title: "refresh" },
				render: {
					...render_facts,
					history_state: { current: true },
				},
				route: {
					...route_facts,
					history_state: { current: true },
					href: "https://example.com/items/1",
				},
			},
			type: "apply_publication_dom",
		});
		expect(plan?.transaction.after_transition).toEqual([
			{
				public_call_id: "reval-call",
				result: { ok: true },
				type: "resolve_public_call",
			},
		]);
	});
});
