import { describe, expect, it } from "vitest";
import type { PreparedRoute, RouteSnapshot } from "./model.ts";
import { plan_route_readiness } from "./readiness.ts";
import { route_facts_to_state } from "./route.ts";

const prepared = {
	dom: { title: "ready" },
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
		href: "https://example.com/ready",
		matches: [],
		params: {},
		splat_values: [],
	},
} satisfies PreparedRoute;

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

describe("plan_route_readiness", () => {
	it("publishes boot preparation without running route hooks", () => {
		const plan = plan_route_readiness({
			browser: null,
			current,
			hooks_required: true,
			operation: {
				browser_key: "browser-1",
				href: "https://example.com/ready",
				id: "boot-1",
				kind: "boot",
				public_call_ids: [],
				rights: ["publish_route"],
				state: undefined,
				status: "prepared",
			},
			prepared,
		});

		expect(plan).toEqual({
			kind: "publishable",
			operation_id: "boot-1",
			prepared,
		});
	});

	it("publishes prepared routes when no route hooks are present", () => {
		const plan = plan_route_readiness({
			browser: null,
			current,
			hooks_required: false,
			operation: {
				browser_key: "browser-2",
				href: "https://example.com/ready",
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
			prepared,
		});

		expect(plan).toEqual({
			kind: "publishable",
			operation_id: "nav-1",
			prepared,
		});
	});

	it("runs route hooks before non-boot publication when hooks are present", () => {
		const plan = plan_route_readiness({
			browser: null,
			current,
			hooks_required: true,
			operation: {
				browser_key: "browser-3",
				href: "https://example.com/ready",
				id: "nav-2",
				kind: "navigation",
				public_call_ids: ["call-2"],
				redirect_count: 0,
				replace: false,
				rights: ["publish_route"],
				source: "navigate",
				state: { next: true },
				status: "prepared",
				skip_work_indicator: false,
			},
			prepared,
		});

		expect(plan).toEqual({
			effects: [
				{
					operation_id: "nav-2",
					request: {
						current_route: route_facts_to_state(current.route),
						next_route: route_facts_to_state({
							...prepared.route,
							history_state: { next: true },
						}),
						prepared: {
							...prepared,
							render: {
								...prepared.render,
								history_state: { next: true },
							},
							route: {
								...prepared.route,
								history_state: { next: true },
							},
						},
						trigger: "navigation",
					},
					type: "run_route_hooks",
				},
			],
			kind: "hooks_running",
			operation_id: "nav-2",
		});
	});

	it("does not treat a provisional boot snapshot as a previous public route", () => {
		const plan = plan_route_readiness({
			browser: null,
			current: {
				...current,
				provisional: true,
			},
			hooks_required: true,
			operation: {
				attempt: 0,
				href: "https://example.com/ready",
				id: "reval-1",
				kind: "route_revalidation",
				public_call_ids: [],
				reason: "manual",
				rights: ["publish_route"],
				status: "prepared",
			},
			prepared,
		});

		expect(plan).toEqual({
			kind: "publishable",
			operation_id: "reval-1",
			prepared,
		});
	});

	it("requires browser facts before planning revalidation route hooks", () => {
		const plan = plan_route_readiness({
			browser: null,
			current,
			hooks_required: true,
			operation: {
				attempt: 0,
				href: "https://example.com/ready",
				id: "reval-2",
				kind: "route_revalidation",
				public_call_ids: [],
				reason: "manual",
				rights: ["publish_route"],
				status: "prepared",
			},
			prepared,
		});

		expect(plan).toBeUndefined();
	});
});
