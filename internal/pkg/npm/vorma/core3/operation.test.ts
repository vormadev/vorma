import { describe, expect, it } from "vitest";
import type { CoreOperation } from "./model.ts";
import {
	can_publish_route,
	is_route_operation,
	owns_route_completion,
	satisfiable_refresh_for_operation,
	type OperationGuardModel,
	type RouteOperation,
	type RouteOwnershipModel,
} from "./operation.ts";

const navigation_operation = {
	browser_key: "browser-1",
	href: "https://example.com/next",
	id: "nav-1",
	kind: "navigation",
	public_call_ids: ["call-1"],
	redirect_count: 0,
	replace: false,
	rights: ["abort_effects", "publish_route"],
	source: "navigate",
	state: undefined,
	status: "publishable",
	skip_work_indicator: false,
} satisfies RouteOperation;

const prefetch_operation = {
	href: "https://example.com/warm",
	id: "prefetch-1",
	kind: "route_prefetch",
	public_call_ids: [],
	rights: ["abort_effects"],
	status: "started",
} satisfies CoreOperation;

const model = {
	operations: {
		"nav-1": navigation_operation,
		"prefetch-1": prefetch_operation,
	},
	publication_owner_id: "nav-1",
} satisfies OperationGuardModel;

const browser = {
	href: "https://example.com/current",
	key: "browser-1",
	state: undefined,
};

const revalidation_operation = {
	attempt: 0,
	href: "https://example.com/current",
	id: "reval-1",
	kind: "route_revalidation",
	public_call_ids: ["reval-call-1"],
	reason: "manual",
	rights: ["publish_route", "satisfy_refresh_demand"],
	status: "started",
} satisfies RouteOperation;

const running_revalidation_model = {
	active_route_operation_id: null,
	browser,
	refresh: {
		attempt: 0,
		demand: {
			operation_id: "reval-1",
			public_call_ids: ["reval-call-1"],
			reason: "manual",
			skip_work_indicator: false,
		},
		kind: "running",
		operation_id: "reval-1",
	},
} satisfies RouteOwnershipModel;

describe("operation guards", () => {
	it("requires both publication ownership and publication rights", () => {
		const missing_owner = {
			...model,
			publication_owner_id: null,
		} satisfies OperationGuardModel;
		const missing_right = {
			...model,
			operations: {
				...model.operations,
				"nav-1": {
					...model.operations["nav-1"],
					rights: [],
				},
			},
		} satisfies OperationGuardModel;

		expect(can_publish_route(model, "nav-1")).toBe(true);
		expect(can_publish_route(missing_owner, "nav-1")).toBe(false);
		expect(can_publish_route(missing_right, "nav-1")).toBe(false);
		expect(can_publish_route(model, "prefetch-1")).toBe(false);
		expect(can_publish_route(model, "missing")).toBe(false);
	});

	it("recognizes route operations", () => {
		expect(is_route_operation(model.operations["nav-1"])).toBe(true);
		expect(is_route_operation(model.operations["prefetch-1"])).toBe(false);
		expect(is_route_operation(undefined)).toBe(false);
	});

	it("requires explicit visible or refresh ownership to accept route completions", () => {
		const navigation_model = {
			active_route_operation_id: "nav-1",
			browser,
			refresh: { kind: "idle" },
		} satisfies RouteOwnershipModel;
		expect(
			owns_route_completion(navigation_model, model.operations["nav-1"]),
		).toBe(true);
		expect(
			owns_route_completion(
				{
					...navigation_model,
					active_route_operation_id: null,
				},
				model.operations["nav-1"],
			),
		).toBe(false);
		expect(
			owns_route_completion(
				running_revalidation_model,
				revalidation_operation,
			),
		).toBe(true);
		expect(
			owns_route_completion(
				{
					...running_revalidation_model,
					active_route_operation_id: "nav-1",
				},
				revalidation_operation,
			),
		).toBe(false);
		expect(
			owns_route_completion(
				{
					...running_revalidation_model,
					browser: {
						...browser,
						href: "https://example.com/other",
					},
				},
				revalidation_operation,
			),
		).toBe(false);
	});

	it("finds a refresh only when the operation may satisfy it", () => {
		expect(
			satisfiable_refresh_for_operation(
				running_revalidation_model,
				revalidation_operation,
			),
		).toBe(running_revalidation_model.refresh);
		expect(
			satisfiable_refresh_for_operation(running_revalidation_model, {
				...revalidation_operation,
				rights: ["publish_route"],
			}),
		).toBeUndefined();
	});
});
