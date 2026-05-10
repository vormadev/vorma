import { describe, expect, it } from "vitest";
import type { CoreModel, RouteSnapshot } from "./model.ts";
import { begin_route_popstate } from "./popstate.ts";

const current = {
	position: {
		href: "https://example.com/current",
		key: "browser-2",
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
	use_view_transitions: false,
} satisfies CoreModel;

describe("begin_route_popstate", () => {
	it("starts browser-owned route work with publication ownership", () => {
		const plan = begin_route_popstate({
			browser: {
				href: "https://example.com/previous",
				key: "browser-1",
				state: { previous: true },
			},
			leaving_scroll: { x: 0, y: 120 },
			model,
			operation_id: "pop-1",
			popstate_scroll: { x: 0, y: 24 },
		});

		expect(plan).toEqual({
			effects: [
				{
					client_build_id: "build-1",
					href: "https://example.com/previous",
					operation_id: "pop-1",
					trigger: "popstate",
					type: "fetch_route",
				},
			],
			model: {
				...model,
				active_route_operation_id: "pop-1",
				operations: {
					"pop-1": {
						browser: {
							href: "https://example.com/previous",
							key: "browser-1",
							state: { previous: true },
						},
						id: "pop-1",
						kind: "popstate",
						leaving_scroll: { x: 0, y: 120 },
						popstate_scroll: { x: 0, y: 24 },
						public_call_ids: [],
						rights: [
							"abort_effects",
							"own_visible_transition",
							"publish_route",
						],
						status: "started",
					},
				},
				publication_owner_id: "pop-1",
			},
		});
	});

	it("ignores duplicate browser positions and occupied visible route work", () => {
		expect(
			begin_route_popstate({
				browser: current.position,
				leaving_scroll: { x: 0, y: 0 },
				model,
				operation_id: "pop-1",
			}),
		).toBeUndefined();
		expect(
			begin_route_popstate({
				browser: {
					href: "https://example.com/previous",
					key: "browser-1",
					state: undefined,
				},
				leaving_scroll: { x: 0, y: 0 },
				model: {
					...model,
					active_route_operation_id: "nav-1",
				},
				operation_id: "pop-1",
			}),
		).toBeUndefined();
	});
});
