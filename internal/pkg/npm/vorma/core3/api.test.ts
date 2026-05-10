import { describe, expect, it } from "vitest";
import { api_refresh_demand, complete_api_submit } from "./api.ts";
import type {
	APIOutcome,
	CoreModel,
	RouteSnapshot,
	SubmissionRecord,
} from "./model.ts";

const submission = {
	href: "https://example.com/api/save",
	method: "POST",
	operation_id: "api-1",
	public_call_id: "api-call-1",
	revalidate: true,
	revalidation_operation_id: "reval-1",
	revalidation_public_call_id: "reval-call-1",
	skip_work_indicator: true,
	submission_key: "submit-1",
} satisfies SubmissionRecord;

const success = {
	data: { ok: true },
	kind: "success",
	operation_id: "api-1",
	revalidation_required: true,
	response: {},
	submission_key: "submit-1",
} satisfies APIOutcome;

const current = {
	position: {
		href: "https://example.com/",
		key: "browser-1",
		state: undefined,
	},
	provisional: false,
	render: {
		client_build_id: "build-1",
		entries: [],
		error: null,
		history_state: undefined,
		params: {},
		splat_values: [],
	},
	route: {
		client_build_id: "build-1",
		error: null,
		history_state: undefined,
		href: "https://example.com/",
		matches: [],
		params: {},
		splat_values: [],
	},
} satisfies RouteSnapshot;

const model = {
	active_route_operation_id: null,
	browser: {
		href: "https://example.com/",
		key: "browser-1",
		state: undefined,
	},
	client_build_id: "build-1",
	current: null,
	deployment_id: "",
	focus_revalidation: null,
	last_work_projection: null,
	operations: {},
	phase: "ready",
	prefetch_operation_id: null,
	publication_owner_id: null,
	refresh: { kind: "idle" },
	submissions: {
		"submit-1": submission,
	},
	use_view_transitions: false,
} satisfies CoreModel;

describe("api_refresh_demand", () => {
	it("creates a ready-phase refresh demand with the separate revalidation waiter", () => {
		expect(
			api_refresh_demand({
				outcome: success,
				phase: "ready",
				submission,
			}),
		).toEqual({
			operation_id: "reval-1",
			public_call_ids: ["reval-call-1"],
			reason: "apiRequest",
			skip_work_indicator: true,
		});
	});

	it("keeps boot-phase refresh demand waiterless to avoid loader deadlock", () => {
		expect(
			api_refresh_demand({
				outcome: success,
				phase: "booting",
				submission,
			}),
		).toEqual({
			operation_id: "reval-1",
			public_call_ids: [],
			reason: "apiRequest",
			skip_work_indicator: true,
		});
	});

	it("does not create refresh demand for stale, mismatched, redirected, or non-revalidating outcomes", () => {
		expect(
			api_refresh_demand({
				outcome: {
					kind: "stale",
					operation_id: "api-1",
					submission_key: "submit-1",
				},
				phase: "ready",
				submission,
			}),
		).toBeUndefined();
		expect(
			api_refresh_demand({
				outcome: {
					...success,
					operation_id: "old-api",
				},
				phase: "ready",
				submission,
			}),
		).toBeUndefined();
		expect(
			api_refresh_demand({
				outcome: {
					href: "https://example.com/next",
					kind: "soft_redirect",
					operation_id: "api-1",
					response: {},
					submission_key: "submit-1",
				},
				phase: "ready",
				submission,
			}),
		).toBeUndefined();
		expect(
			api_refresh_demand({
				outcome: {
					...success,
					revalidation_required: false,
				},
				phase: "ready",
				submission,
			}),
		).toBeUndefined();
	});
});

describe("complete_api_submit", () => {
	it("settles the API call, removes API work, and queues revalidation", () => {
		const plan = complete_api_submit({
			debounce_refresh: false,
			debounce_refresh_ms: 0,
			model,
			outcome: success,
		});

		expect(plan).toEqual({
			effects: [
				{
					public_call_id: "api-call-1",
					result: {
						data: { ok: true },
						response: {},
						revalidation_public_call_id: "reval-call-1",
						success: true,
					},
					type: "resolve_api_call",
				},
			],
			model: {
				...model,
				refresh: {
					attempt: 0,
					demand: {
						operation_id: "reval-1",
						public_call_ids: ["reval-call-1"],
						reason: "apiRequest",
						skip_work_indicator: true,
					},
					kind: "pending",
				},
				submissions: {},
			},
		});
	});

	it("starts API-requested revalidation when the route lane is idle", () => {
		const plan = complete_api_submit({
			debounce_refresh: false,
			debounce_refresh_ms: 0,
			model: {
				...model,
				current,
			},
			outcome: success,
		});

		expect(plan?.effects).toEqual([
			{
				client_build_id: "build-1",
				href: "https://example.com/",
				operation_id: "reval-1",
				trigger: "revalidation",
				type: "fetch_route",
			},
			{
				public_call_id: "api-call-1",
				result: {
					data: { ok: true },
					response: {},
					revalidation_public_call_id: "reval-call-1",
					success: true,
				},
				type: "resolve_api_call",
			},
		]);
		expect(plan?.model.operations["reval-1"]).toEqual({
			attempt: 0,
			href: "https://example.com/",
			id: "reval-1",
			kind: "route_revalidation",
			public_call_ids: ["reval-call-1"],
			reason: "apiRequest",
			rights: ["publish_route", "satisfy_refresh_demand"],
			status: "started",
		});
		expect(plan?.model.refresh).toEqual({
			attempt: 0,
			demand: {
				operation_id: "reval-1",
				public_call_ids: ["reval-call-1"],
				reason: "apiRequest",
				skip_work_indicator: true,
			},
			kind: "running",
			operation_id: "reval-1",
		});
		expect(plan?.model.submissions).toEqual({});
	});

	it("debounces API revalidation without keeping API request work alive", () => {
		const plan = complete_api_submit({
			debounce_refresh: true,
			debounce_refresh_ms: 25,
			model,
			outcome: success,
			refresh_timer_id: "timer-1",
		});

		expect(plan?.effects).toEqual([
			{
				delay_ms: 25,
				operation_id: "reval-1",
				timer_id: "timer-1",
				type: "start_timer",
			},
			{
				public_call_id: "api-call-1",
				result: {
					data: { ok: true },
					response: {},
					revalidation_public_call_id: "reval-call-1",
					success: true,
				},
				type: "resolve_api_call",
			},
		]);
		expect(plan?.model.submissions).toEqual({});
		expect(plan?.model.refresh).toEqual({
			demand: {
				operation_id: "reval-1",
				public_call_ids: ["reval-call-1"],
				reason: "apiRequest",
				skip_work_indicator: true,
			},
			kind: "debouncing",
			timer_id: "timer-1",
		});
	});

	it("settles dispatched failures with the response object", () => {
		const response = { status: 500 };
		const plan = complete_api_submit({
			debounce_refresh: false,
			debounce_refresh_ms: 0,
			model,
			outcome: {
				error: "Err",
				kind: "failure",
				operation_id: "api-1",
				revalidation_required: true,
				response,
				submission_key: "submit-1",
			},
		});

		expect(plan?.effects[0]).toEqual({
			public_call_id: "api-call-1",
			result: {
				error: "Err",
				response,
				revalidation_public_call_id: "reval-call-1",
				success: false,
			},
			type: "resolve_api_call",
		});
		expect(plan?.model.submissions).toEqual({});
	});

	it("ignores stale or non-current API completions", () => {
		expect(
			complete_api_submit({
				debounce_refresh: false,
				debounce_refresh_ms: 0,
				model,
				outcome: {
					kind: "stale",
					operation_id: "api-1",
					submission_key: "submit-1",
				},
			}),
		).toBeUndefined();
		expect(
			complete_api_submit({
				debounce_refresh: false,
				debounce_refresh_ms: 0,
				model,
				outcome: {
					...success,
					operation_id: "old-api",
				},
			}),
		).toBeUndefined();
	});
});
