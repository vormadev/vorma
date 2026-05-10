import { describe, expect, it } from "vitest";
import type { CoreModel, RouteSnapshot } from "./model.ts";
import {
	advance_navigation_request,
	begin_route_navigation,
} from "./navigation.ts";

const current = {
	position: {
		href: "https://example.com/current",
		key: "browser-1",
		state: { current: true },
	},
	provisional: false,
	render: {
		client_build_id: "build-1",
		entries: [],
		error: null,
		history_state: { current: true },
		params: {},
		splat_values: [],
	},
	route: {
		client_build_id: "build-1",
		error: null,
		history_state: { current: true },
		href: "https://example.com/current",
		matches: [],
		params: {},
		splat_values: [],
	},
} satisfies RouteSnapshot;

const model = {
	active_route_operation_id: null,
	browser: current.position,
	client_build_id: "build-1",
	current,
	deployment_id: "",
	focus_revalidation: null,
	last_work_projection: null,
	operations: {},
	phase: "ready",
	prefetch_operation_id: null,
	publication_owner_id: null,
	refresh: { kind: "idle" },
	submissions: {},
	use_view_transitions: true,
} satisfies CoreModel;

describe("begin_route_navigation", () => {
	it("starts a visible route navigation with publication ownership", () => {
		const plan = begin_route_navigation({
			browser_key: "browser-2",
			href: "https://example.com/next",
			model,
			operation_id: "nav-1",
			public_call_ids: ["nav-call-1"],
			replace: false,
			scroll_to_top: false,
			skip_work_indicator: true,
			state: { next: true },
		});

		expect(plan).toEqual({
			effects: [
				{
					client_build_id: "build-1",
					href: "https://example.com/next",
					operation_id: "nav-1",
					trigger: "navigation",
					type: "fetch_route",
				},
			],
			model: {
				...model,
				active_route_operation_id: "nav-1",
				operations: {
					"nav-1": {
						browser_key: "browser-2",
						href: "https://example.com/next",
						id: "nav-1",
						kind: "navigation",
						public_call_ids: ["nav-call-1"],
						redirect_count: 0,
						replace: false,
						rights: [
							"abort_effects",
							"own_visible_transition",
							"publish_route",
						],
						scroll_to_top: false,
						source: "navigate",
						state: { next: true },
						status: "started",
						skip_work_indicator: true,
					},
				},
				publication_owner_id: "nav-1",
			},
		});
	});

	it("does not start when another visible route operation owns the lane", () => {
		expect(
			begin_route_navigation({
				browser_key: "browser-3",
				href: "https://example.com/other",
				model: {
					...model,
					active_route_operation_id: "nav-1",
				},
				operation_id: "nav-2",
				public_call_ids: ["nav-call-2"],
				replace: false,
				skip_work_indicator: false,
				state: undefined,
			}),
		).toBeUndefined();
	});
});

describe("advance_navigation_request", () => {
	it("hard redirects cross-origin navigation without fetching", () => {
		const transition = advance_navigation_request({
			browser_key: "browser-2",
			model,
			operation_id: "nav-1",
			outcome: {
				href: "https://external.example/path",
				kind: "hard_redirect",
			},
			public_call_ids: ["nav-call-1"],
			replace: false,
			skip_work_indicator: false,
			state: undefined,
			view_transition_available: false,
		});

		expect(transition).toEqual({
			effects: [
				{
					href: "https://external.example/path",
					type: "hard_redirect",
				},
				{
					public_call_id: "nav-call-1",
					result: { didNavigate: false },
					type: "resolve_public_call",
				},
			],
			kind: "hard_redirect",
			model,
		});
	});

	it("publishes same-document navigation without fetching", () => {
		const transition = advance_navigation_request({
			browser_key: "browser-2",
			model,
			operation_id: "nav-1",
			outcome: {
				href: "https://example.com/current#details",
				kind: "same_document",
			},
			public_call_ids: ["nav-call-1"],
			replace: false,
			skip_work_indicator: false,
			state: { next: true },
			view_transition_available: false,
		});

		expect(transition?.kind).toBe("publishing");
		expect(
			transition?.effects.map((effect) => {
				return effect.type;
			}),
		).toEqual([
			"save_current_scroll",
			"write_history",
			"commit",
			"render",
			"resolve_public_call",
		]);
		expect(transition?.model.active_route_operation_id).toBe("nav-1");
		expect(transition?.model.publication_owner_id).toBe("nav-1");
		expect(transition?.model.operations["nav-1"]).toMatchObject({
			href: "https://example.com/current#details",
			kind: "navigation",
			status: "publishing",
		});
	});

	it("supersedes incompatible active route work before starting navigation", () => {
		const transition = advance_navigation_request({
			browser_key: "browser-3",
			model: {
				...model,
				active_route_operation_id: "nav-old",
				operations: {
					"nav-old": {
						browser_key: "browser-2",
						href: "https://example.com/old",
						id: "nav-old",
						kind: "navigation",
						public_call_ids: ["old-call"],
						redirect_count: 0,
						replace: false,
						rights: [
							"abort_effects",
							"own_visible_transition",
							"publish_route",
						],
						source: "navigate",
						state: undefined,
						status: "started",
						skip_work_indicator: false,
					},
				},
				publication_owner_id: "nav-old",
			},
			operation_id: "nav-new",
			outcome: {
				href: "https://example.com/new",
				kind: "route_navigation",
			},
			public_call_ids: ["new-call"],
			replace: false,
			skip_work_indicator: false,
			state: undefined,
			view_transition_available: false,
		});

		expect(transition?.effects).toEqual([
			{
				operation_id: "nav-old",
				type: "abort_operation",
			},
			{
				public_call_id: "old-call",
				result: { didNavigate: false },
				type: "resolve_public_call",
			},
			{
				client_build_id: "build-1",
				href: "https://example.com/new",
				operation_id: "nav-new",
				trigger: "navigation",
				type: "fetch_route",
			},
		]);
		expect(transition?.model.operations["nav-old"]?.status).toBe(
			"superseded",
		);
		expect(transition?.model.active_route_operation_id).toBe("nav-new");
	});

	it("promotes prepared prefetch without fetching again", () => {
		const transition = advance_navigation_request({
			browser_key: "browser-2",
			model: {
				...model,
				operations: {
					"prefetch-1": {
						href: "https://example.com/next",
						id: "prefetch-1",
						kind: "route_prefetch",
						prepared_resource_key: "resource-1",
						public_call_ids: [],
						rights: ["abort_effects"],
						status: "prepared",
					},
				},
				prefetch_operation_id: "prefetch-1",
			},
			operation_id: "nav-1",
			outcome: {
				href: "https://example.com/next",
				kind: "route_navigation",
			},
			public_call_ids: ["nav-call-1"],
			replace: false,
			skip_work_indicator: false,
			state: { next: true },
			view_transition_available: false,
		});

		expect(transition).toEqual({
			effects: [
				{
					client_build_id: "build-1",
					history_state: { next: true },
					href: "https://example.com/next",
					operation_id: "prefetch-1",
					resource_key: "resource-1",
					type: "promote_prefetch_route",
				},
			],
			kind: "promoted",
			model: {
				...model,
				active_route_operation_id: "prefetch-1",
				operations: {
					"prefetch-1": {
						browser_key: "browser-2",
						href: "https://example.com/next",
						id: "prefetch-1",
						kind: "navigation",
						public_call_ids: ["nav-call-1"],
						redirect_count: 0,
						replace: false,
						rights: [
							"abort_effects",
							"own_visible_transition",
							"publish_route",
						],
						source: "navigate",
						state: { next: true },
						status: "classified",
						skip_work_indicator: false,
					},
				},
				prefetch_operation_id: null,
				publication_owner_id: "prefetch-1",
			},
		});
	});
});
