import { describe, expect, it } from "vitest";
import { classify_route_response } from "./classify.ts";
import type { CoreModel, RouteSnapshot } from "./model.ts";
import {
	advance_route_prefetch_outcome,
	advance_route_prefetch_preparation,
	begin_route_prefetch,
	cancel_route_prefetch,
} from "./prefetch.ts";
import { derive_work } from "./work.ts";

const current = {
	position: {
		href: "https://example.com/current",
		key: "browser-1",
		state: { current: true },
	},
	provisional: false,
	render: {
		client_build_id: "build-1",
		entries: [
			{
				client_loader_data: undefined,
				input: undefined,
				loader_data: { current: true },
				module: {},
				module_url: "/current.js",
				pattern: "/current",
			},
		],
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
		matches: [
			{
				client_loader_data: undefined,
				input: undefined,
				loader_data: { current: true },
				module: {},
				module_url: "/current.js",
				pattern: "/current",
			},
		],
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

describe("begin_route_prefetch", () => {
	it("starts prefetch route work without publication rights", () => {
		const plan = begin_route_prefetch({
			href: "/next",
			model,
			operation_id: "prefetch-1",
		});

		expect(plan).toEqual({
			effects: [
				{
					client_build_id: "build-1",
					href: "https://example.com/next",
					operation_id: "prefetch-1",
					trigger: "prefetch",
					type: "fetch_route",
				},
			],
			model: {
				...model,
				operations: {
					"prefetch-1": {
						href: "https://example.com/next",
						id: "prefetch-1",
						kind: "route_prefetch",
						public_call_ids: [],
						rights: ["abort_effects"],
						status: "started",
					},
				},
				prefetch_operation_id: "prefetch-1",
			},
		});
	});

	it("cancels stale prefetch work when a different target starts", () => {
		const first = begin_route_prefetch({
			href: "/first",
			model,
			operation_id: "prefetch-1",
		});
		expect(first).toBeDefined();
		if (!first) {
			return;
		}

		const second = begin_route_prefetch({
			href: "/second",
			model: first.model,
			operation_id: "prefetch-2",
		});

		expect(second?.effects).toEqual([
			{
				operation_id: "prefetch-1",
				type: "abort_operation",
			},
			{
				client_build_id: "build-1",
				href: "https://example.com/second",
				operation_id: "prefetch-2",
				trigger: "prefetch",
				type: "fetch_route",
			},
		]);
		expect(second?.model.operations["prefetch-1"]?.status).toBe("aborted");
		expect(second?.model.prefetch_operation_id).toBe("prefetch-2");
	});

	it("ignores off-origin, current-route, and matching active navigation targets", () => {
		expect(
			begin_route_prefetch({
				href: "https://elsewhere.example/next",
				model,
				operation_id: "prefetch-1",
			}),
		).toBeUndefined();
		expect(
			begin_route_prefetch({
				href: "/current#details",
				model,
				operation_id: "prefetch-1",
			}),
		).toBeUndefined();
		expect(
			begin_route_prefetch({
				href: "/next",
				model: {
					...model,
					active_route_operation_id: "nav-1",
					operations: {
						"nav-1": {
							browser_key: "browser-2",
							href: "https://example.com/next",
							id: "nav-1",
							kind: "navigation",
							public_call_ids: [],
							redirect_count: 0,
							replace: false,
							rights: ["own_visible_transition", "publish_route"],
							source: "navigate",
							state: undefined,
							status: "started",
							skip_work_indicator: false,
						},
					},
				},
				operation_id: "prefetch-1",
			}),
		).toBeUndefined();
	});
});

describe("cancel_route_prefetch", () => {
	it("aborts the matching current prefetch", () => {
		const started = begin_route_prefetch({
			href: "/next",
			model,
			operation_id: "prefetch-1",
		});
		expect(started).toBeDefined();
		if (!started) {
			return;
		}

		const canceled = cancel_route_prefetch({
			href: "/next#hash",
			model: started.model,
		});

		expect(canceled?.effects).toEqual([
			{
				operation_id: "prefetch-1",
				type: "abort_operation",
			},
		]);
		expect(canceled?.model.operations["prefetch-1"]?.status).toBe(
			"aborted",
		);
		expect(canceled?.model.prefetch_operation_id).toBeNull();
	});
});

describe("advance_route_prefetch_outcome", () => {
	it("prepares data without granting publication rights", () => {
		const started = begin_route_prefetch({
			href: "/next",
			model,
			operation_id: "prefetch-1",
		});
		expect(started).toBeDefined();
		if (!started) {
			return;
		}

		const classified = classify_route_response({
			active_client_build_id: "build-1",
			is_current_operation: true,
			max_redirects: 10,
			max_revalidation_retries: 3,
			operation_id: "prefetch-1",
			operation_kind: "route_prefetch",
			redirect_count: 0,
			revalidation_attempt: 0,
			response: {
				kind: "data",
				ok: true,
				payload: { route: true },
				server_build_id: "build-1",
				status: 200,
			},
		});
		const preparing = advance_route_prefetch_outcome({
			build_skew_report: classified.build_skew_report,
			model: started.model,
			outcome: classified.outcome,
		});

		expect(preparing?.effects).toEqual([
			{
				client_build_id: "build-1",
				history_state: undefined,
				href: "https://example.com/next",
				operation_id: "prefetch-1",
				payload: { route: true },
				trigger: "prefetch",
				type: "prepare_route",
			},
		]);
		expect(preparing?.model.operations["prefetch-1"]).toMatchObject({
			rights: ["abort_effects"],
			status: "classified",
		});
	});

	it("reports and drops prefetch build skew without publishing", () => {
		const started = begin_route_prefetch({
			href: "/next",
			model,
			operation_id: "prefetch-1",
		});
		expect(started).toBeDefined();
		if (!started) {
			return;
		}

		const classified = classify_route_response({
			active_client_build_id: "build-1",
			is_current_operation: true,
			max_redirects: 10,
			max_revalidation_retries: 3,
			operation_id: "prefetch-1",
			operation_kind: "route_prefetch",
			redirect_count: 0,
			revalidation_attempt: 0,
			response: {
				kind: "build_skew",
				ok: true,
				server_build_id: "build-2",
				status: 200,
			},
		});
		const transition = advance_route_prefetch_outcome({
			build_skew_report: classified.build_skew_report,
			model: started.model,
			outcome: classified.outcome,
		});

		expect(transition?.effects).toEqual([
			{
				event: expect.objectContaining({
					triggeringResponse: expect.objectContaining({
						trigger: "prefetch",
					}),
				}),
				type: "report_build_skew",
			},
		]);
		expect(transition?.kind).toBe("completed");
		expect(transition?.model.operations["prefetch-1"]?.status).toBe(
			"completed",
		);
		expect(transition?.model.prefetch_operation_id).toBeNull();
	});

	it("records prepared resources inertly and removes public prefetch work", () => {
		const started = begin_route_prefetch({
			href: "/next",
			model,
			operation_id: "prefetch-1",
		});
		expect(started).toBeDefined();
		if (!started) {
			return;
		}
		const preparing = advance_route_prefetch_outcome({
			model: started.model,
			outcome: {
				kind: "route_data",
				operation_id: "prefetch-1",
				payload: { route: true },
			},
		});
		expect(preparing).toBeDefined();
		if (!preparing) {
			return;
		}

		const prepared = advance_route_prefetch_preparation({
			model: preparing.model,
			outcome: {
				kind: "prepared",
				operation_id: "prefetch-1",
				prepared_resource_key: "resource-1",
			},
		});

		expect(prepared?.model.operations["prefetch-1"]).toMatchObject({
			prepared_resource_key: "resource-1",
			status: "prepared",
		});
		if (!prepared) {
			return;
		}
		expect(derive_work(prepared.model).projection.prefetch).toBeNull();
	});
});
