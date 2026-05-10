import { describe, expect, it } from "vitest";
import type { CoreModel, RouteSnapshot } from "./model.ts";
import {
	begin_pending_refresh,
	consume_refresh_timer,
	plan_refresh_retry,
	request_focus_revalidation,
	request_manual_revalidation,
	request_refresh,
} from "./refresh.ts";

const demand = {
	operation_id: "reval-1",
	public_call_ids: ["call-1"],
	reason: "manual",
	skip_work_indicator: true,
} as const;

const current = {
	position: {
		href: "https://example.com/current",
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
	refresh: {
		attempt: 0,
		demand,
		kind: "pending",
	},
	submissions: {},
	use_view_transitions: false,
} satisfies CoreModel;

describe("request_refresh", () => {
	it("plans debounced refresh and clears the replaced timer", () => {
		const plan = request_refresh({
			debounce: true,
			debounce_ms: 12,
			demand: {
				operation_id: "reval-2",
				public_call_ids: ["call-2"],
				reason: "manual",
				skip_work_indicator: true,
			},
			refresh: {
				demand,
				kind: "debouncing",
				timer_id: "timer-1",
			},
			timer_id: "timer-2",
		});

		expect(plan).toEqual({
			effects: [
				{
					timer_id: "timer-1",
					type: "clear_timer",
				},
				{
					delay_ms: 12,
					operation_id: "reval-2",
					timer_id: "timer-2",
					type: "start_timer",
				},
			],
			refresh: {
				demand: {
					operation_id: "reval-2",
					public_call_ids: ["call-1", "call-2"],
					reason: "manual",
					skip_work_indicator: true,
				},
				kind: "debouncing",
				timer_id: "timer-2",
			},
		});
	});

	it("plans immediate refresh without timer effects", () => {
		const plan = request_refresh({
			debounce: false,
			debounce_ms: 12,
			demand,
			refresh: { kind: "idle" },
		});

		expect(plan).toEqual({
			effects: [],
			refresh: {
				attempt: 0,
				demand,
				kind: "pending",
			},
		});
	});

	it("keeps work indicator skipped only when every merged demand skips it", () => {
		const plan = request_refresh({
			debounce: false,
			debounce_ms: 12,
			demand: {
				operation_id: "reval-2",
				public_call_ids: ["call-2"],
				reason: "apiRequest",
				skip_work_indicator: false,
			},
			refresh: {
				attempt: 0,
				demand,
				kind: "pending",
			},
		});

		expect(plan.refresh).toEqual({
			attempt: 0,
			demand: {
				operation_id: "reval-2",
				public_call_ids: ["call-1", "call-2"],
				reason: "apiRequest",
				skip_work_indicator: false,
			},
			kind: "pending",
		});
	});
});

describe("request_manual_revalidation", () => {
	it("queues and immediately starts manual revalidation when the route lane is idle", () => {
		const plan = request_manual_revalidation({
			debounce: false,
			debounce_ms: 0,
			model: {
				...model,
				refresh: { kind: "idle" },
			},
			operation_id: "reval-manual-1",
			public_call_id: "manual-call-1",
			skip_work_indicator: true,
		});

		expect(plan).toEqual({
			effects: [
				{
					client_build_id: "build-1",
					href: "https://example.com/current",
					operation_id: "reval-manual-1",
					trigger: "revalidation",
					type: "fetch_route",
				},
			],
			model: {
				...model,
				operations: {
					"reval-manual-1": {
						attempt: 0,
						href: "https://example.com/current",
						id: "reval-manual-1",
						kind: "route_revalidation",
						public_call_ids: ["manual-call-1"],
						reason: "manual",
						rights: ["publish_route", "satisfy_refresh_demand"],
						status: "started",
					},
				},
				refresh: {
					attempt: 0,
					demand: {
						operation_id: "reval-manual-1",
						public_call_ids: ["manual-call-1"],
						reason: "manual",
						skip_work_indicator: true,
					},
					kind: "running",
					operation_id: "reval-manual-1",
				},
			},
		});
	});
});

describe("request_focus_revalidation", () => {
	it("debounces stale window-focus revalidation", () => {
		const plan = request_focus_revalidation({
			debounce_ms: 8,
			model: {
				...model,
				focus_revalidation: {
					last_activity_ms: 1000,
					stale_ms: 100,
				},
				refresh: { kind: "idle" },
			},
			now_ms: 1200,
			operation_id: "reval-focus-1",
			timer_id: "focus-timer-1",
		});

		expect(plan).toEqual({
			effects: [
				{
					delay_ms: 8,
					operation_id: "reval-focus-1",
					timer_id: "focus-timer-1",
					type: "start_timer",
				},
			],
			model: {
				...model,
				focus_revalidation: {
					last_activity_ms: 1000,
					stale_ms: 100,
				},
				refresh: {
					demand: {
						operation_id: "reval-focus-1",
						public_call_ids: [],
						reason: "windowFocus",
						skip_work_indicator: true,
					},
					kind: "debouncing",
					timer_id: "focus-timer-1",
				},
			},
		});
	});

	it("waits for stale focus and an idle model", () => {
		expect(
			request_focus_revalidation({
				debounce_ms: 8,
				model: {
					...model,
					focus_revalidation: {
						last_activity_ms: 1000,
						stale_ms: 500,
					},
					refresh: { kind: "idle" },
				},
				now_ms: 1200,
				operation_id: "reval-focus-1",
			}),
		).toBeUndefined();
		expect(
			request_focus_revalidation({
				debounce_ms: 8,
				model: {
					...model,
					active_route_operation_id: "nav-1",
					focus_revalidation: {
						last_activity_ms: 1000,
						stale_ms: 100,
					},
					refresh: { kind: "idle" },
				},
				now_ms: 1200,
				operation_id: "reval-focus-1",
			}),
		).toBeUndefined();
	});
});

describe("begin_pending_refresh", () => {
	it("starts pending refresh as a route revalidation operation", () => {
		const plan = begin_pending_refresh({ model });

		expect(plan).toEqual({
			effects: [
				{
					client_build_id: "build-1",
					href: "https://example.com/current",
					operation_id: "reval-1",
					trigger: "revalidation",
					type: "fetch_route",
				},
			],
			model: {
				...model,
				operations: {
					"reval-1": {
						attempt: 0,
						href: "https://example.com/current",
						id: "reval-1",
						kind: "route_revalidation",
						public_call_ids: ["call-1"],
						reason: "manual",
						rights: ["publish_route", "satisfy_refresh_demand"],
						status: "started",
					},
				},
				refresh: {
					attempt: 0,
					demand,
					kind: "running",
					operation_id: "reval-1",
				},
			},
		});
	});

	it("waits for ready phase, browser position, current route, idle route lane, and refresh predecessor", () => {
		expect(
			begin_pending_refresh({
				model: {
					...model,
					phase: "booting",
				},
			}),
		).toBeUndefined();
		expect(
			begin_pending_refresh({
				model: {
					...model,
					browser: null,
				},
			}),
		).toBeUndefined();
		expect(
			begin_pending_refresh({
				model: {
					...model,
					current: null,
				},
			}),
		).toBeUndefined();
		expect(
			begin_pending_refresh({
				model: {
					...model,
					active_route_operation_id: "nav-1",
				},
			}),
		).toBeUndefined();
		expect(
			begin_pending_refresh({
				model: {
					...model,
					operations: {
						"nav-1": {
							browser_key: "browser-2",
							href: "https://example.com/next",
							id: "nav-1",
							kind: "navigation",
							public_call_ids: [],
							redirect_count: 0,
							replace: false,
							rights: ["publish_route"],
							source: "navigate",
							state: undefined,
							status: "published",
							skip_work_indicator: false,
						},
					},
					refresh: {
						attempt: 0,
						demand: {
							...demand,
							after_operation_id: "nav-1",
						},
						kind: "pending",
					},
				},
			}),
		).toBeUndefined();
	});
});

describe("consume_refresh_timer", () => {
	it("turns a debounced refresh into running route revalidation", () => {
		const plan = consume_refresh_timer({
			model: {
				...model,
				refresh: {
					demand,
					kind: "debouncing",
					timer_id: "timer-1",
				},
			},
			operation_id: "reval-1",
			timer_id: "timer-1",
		});

		expect(plan).toEqual({
			effects: [
				{
					client_build_id: "build-1",
					href: "https://example.com/current",
					operation_id: "reval-1",
					trigger: "revalidation",
					type: "fetch_route",
				},
			],
			model: {
				...model,
				operations: {
					"reval-1": {
						attempt: 0,
						href: "https://example.com/current",
						id: "reval-1",
						kind: "route_revalidation",
						public_call_ids: ["call-1"],
						reason: "manual",
						rights: ["publish_route", "satisfy_refresh_demand"],
						status: "started",
					},
				},
				refresh: {
					attempt: 0,
					demand,
					kind: "running",
					operation_id: "reval-1",
				},
			},
		});
	});

	it("ignores stale timer facts", () => {
		expect(
			consume_refresh_timer({
				model: {
					...model,
					refresh: {
						demand,
						kind: "debouncing",
						timer_id: "timer-1",
					},
				},
				operation_id: "reval-2",
				timer_id: "timer-1",
			}),
		).toBeUndefined();
	});
});

describe("plan_refresh_retry", () => {
	it("plans capped exponential retry timing with a new operation identity", () => {
		const plan = plan_refresh_retry({
			backoff_base_ms: 500,
			backoff_cap_ms: 1200,
			demand,
			max_retries: 8,
			next_operation_id: "reval-retry-3",
			previous_attempt: 2,
			timer_id: "retry-timer",
		});

		expect(plan).toEqual({
			effects: [
				{
					delay_ms: 1200,
					operation_id: "reval-retry-3",
					timer_id: "retry-timer",
					type: "start_timer",
				},
			],
			kind: "retrying",
			refresh: {
				attempt: 3,
				demand: {
					...demand,
					operation_id: "reval-retry-3",
					reason: "retry",
				},
				kind: "retrying",
				timer_id: "retry-timer",
			},
		});
	});

	it("reports exhausted refresh without scheduling another timer", () => {
		const plan = plan_refresh_retry({
			backoff_base_ms: 500,
			backoff_cap_ms: 1200,
			demand,
			max_retries: 2,
			next_operation_id: "reval-retry-3",
			previous_attempt: 2,
			timer_id: "retry-timer",
		});

		expect(plan).toEqual({
			kind: "exhausted",
			public_call_ids: ["call-1"],
			refresh: { kind: "idle" },
		});
	});
});
