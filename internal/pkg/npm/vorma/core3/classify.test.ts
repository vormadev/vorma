import { describe, expect, it } from "vitest";
import {
	classify_api_response,
	classify_navigation_request,
	classify_route_hooks,
	classify_route_preparation,
	classify_route_response,
} from "./classify.ts";
import type { PreparedRoute } from "./model.ts";

const base_route_input = {
	active_client_build_id: "build-1",
	is_current_operation: true,
	max_redirects: 10,
	max_revalidation_retries: 8,
	operation_id: "op-1",
	operation_kind: "navigation" as const,
	redirect_count: 0,
	revalidation_attempt: 0,
};

const base_api_input = {
	active_client_build_id: "build-1",
	is_current_submission: true,
	operation_id: "api-1",
	revalidate: true,
	submission_key: "submit-1",
};

const prepared_route = {
	dom: { title: "Ready" },
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

describe("classify_navigation_request", () => {
	it("resolves relative hrefs before route navigation classification", () => {
		const classified = classify_navigation_request({
			current_browser_href: "https://example.com/accounts/current",
			current_route_href: "https://example.com/accounts/current",
			href: "../settings#security",
		});

		expect(classified).toEqual({
			outcome: {
				href: "https://example.com/settings#security",
				kind: "route_navigation",
			},
		});
	});

	it("classifies off-origin navigation requests as hard redirects", () => {
		const classified = classify_navigation_request({
			current_browser_href: "https://example.com/current",
			current_route_href: "https://example.com/current",
			href: "https://example.net/elsewhere",
		});

		expect(classified).toEqual({
			outcome: {
				href: "https://example.net/elsewhere",
				kind: "hard_redirect",
			},
		});
	});

	it("classifies same-document navigation without fetch", () => {
		const classified = classify_navigation_request({
			current_browser_href: "https://example.com/items/1#old",
			current_route_href: "https://example.com/items/1#old",
			href: "#details",
		});

		expect(classified).toEqual({
			outcome: {
				href: "https://example.com/items/1#details",
				kind: "same_document",
			},
		});
	});

	it("does not collapse search changes into same-document navigation", () => {
		const classified = classify_navigation_request({
			current_browser_href: "https://example.com/items/1?tab=a",
			current_route_href: "https://example.com/items/1?tab=a",
			href: "?tab=b",
		});

		expect(classified).toEqual({
			outcome: {
				href: "https://example.com/items/1?tab=b",
				kind: "route_navigation",
			},
		});
	});
});

describe("classify_route_response", () => {
	it("ignores stale route responses before reporting build skew", () => {
		const classified = classify_route_response({
			...base_route_input,
			is_current_operation: false,
			response: {
				kind: "data",
				ok: true,
				payload: { route: true },
				server_build_id: "build-2",
				status: 200,
			},
		});

		expect(classified).toEqual({
			outcome: {
				kind: "stale",
				operation_id: "op-1",
			},
		});
	});

	it("keeps build-skew data publishable while reporting notify policy", () => {
		const classified = classify_route_response({
			...base_route_input,
			response: {
				kind: "data",
				ok: true,
				payload: { route: true },
				server_build_id: "build-2",
				status: 200,
			},
		});

		expect(classified).toEqual({
			build_skew_report: {
				behavior: "notify",
				ok: true,
				server_build_id: "build-2",
				status: 200,
			},
			outcome: {
				kind: "route_data",
				operation_id: "op-1",
				payload: { route: true },
			},
		});
	});

	it("drops explicit build-skew revalidation responses", () => {
		const classified = classify_route_response({
			...base_route_input,
			operation_kind: "route_revalidation",
			response: {
				kind: "build_skew",
				ok: false,
				server_build_id: "build-2",
				status: 409,
			},
		});

		expect(classified).toEqual({
			build_skew_report: {
				behavior: "drop",
				ok: false,
				server_build_id: "build-2",
				status: 409,
			},
			outcome: {
				behavior: "drop",
				kind: "build_skew_drop",
				operation_id: "op-1",
				server_build_id: "build-2",
			},
		});
	});

	it("classifies hard route redirects as reload-skew candidates", () => {
		const classified = classify_route_response({
			...base_route_input,
			response: {
				hard: true,
				href: "https://example.com/next",
				http: true,
				kind: "redirect",
				ok: true,
				server_build_id: "build-2",
				status: 200,
			},
		});

		expect(classified).toEqual({
			build_skew_report: {
				behavior: "reload",
				ok: true,
				server_build_id: "build-2",
				status: 200,
			},
			outcome: {
				hard: true,
				href: "https://example.com/next",
				kind: "hard_redirect",
				operation_id: "op-1",
			},
		});
	});
});

describe("classify_api_response", () => {
	it("preserves the response object on successful API outcomes", () => {
		const response = { status: 200 };
		const classified = classify_api_response({
			...base_api_input,
			response: {
				data: { ok: true },
				kind: "success",
				ok: true,
				response,
				server_build_id: "build-1",
				status: 200,
			},
		});

		expect(classified.outcome).toEqual({
			data: { ok: true },
			kind: "success",
			operation_id: "api-1",
			revalidation_required: true,
			response,
			submission_key: "submit-1",
		});
	});

	it("does not schedule revalidation for failures before dispatch", () => {
		const classified = classify_api_response({
			...base_api_input,
			response: {
				dispatched: false,
				error: "submit only supports same-origin targets.",
				kind: "failure",
			},
		});

		expect(classified).toEqual({
			outcome: {
				error: "submit only supports same-origin targets.",
				kind: "failure",
				operation_id: "api-1",
				revalidation_required: false,
				submission_key: "submit-1",
			},
		});
	});

	it("preserves the response object on dispatched API failures", () => {
		const response = { status: 500 };
		const classified = classify_api_response({
			...base_api_input,
			response: {
				dispatched: true,
				error: "Err",
				kind: "failure",
				ok: false,
				response,
				server_build_id: "build-1",
				status: 500,
			},
		});

		expect(classified.outcome).toEqual({
			error: "Err",
			kind: "failure",
			operation_id: "api-1",
			revalidation_required: true,
			response,
			submission_key: "submit-1",
		});
	});

	it("settles API hard redirects separately from revalidation", () => {
		const response = { status: 200 };
		const classified = classify_api_response({
			...base_api_input,
			response: {
				hard: true,
				href: "https://example.com/next",
				http: true,
				kind: "redirect",
				ok: true,
				response,
				server_build_id: "build-2",
				status: 200,
			},
		});

		expect(classified).toEqual({
			build_skew_report: {
				behavior: "reload",
				ok: true,
				server_build_id: "build-2",
				status: 200,
			},
			outcome: {
				href: "https://example.com/next",
				kind: "hard_redirect",
				operation_id: "api-1",
				response,
				submission_key: "submit-1",
			},
		});
	});
});

describe("classify_route_preparation", () => {
	it("ignores stale preparation failures before exposing causes", () => {
		const classified = classify_route_preparation({
			is_current_operation: false,
			operation_id: "op-1",
			result: {
				cause: new Error("late failure"),
				kind: "failed",
			},
		});

		expect(classified).toEqual({
			kind: "stale",
			operation_id: "op-1",
		});
	});

	it("keeps prepared route data inert after preparation", () => {
		const classified = classify_route_preparation({
			is_current_operation: true,
			operation_id: "op-1",
			result: {
				kind: "prepared",
				prepared: prepared_route,
			},
		});

		expect(classified).toEqual({
			kind: "prepared",
			operation_id: "op-1",
			prepared: prepared_route,
		});
	});
});

describe("classify_route_hooks", () => {
	it("ignores stale hook failures before exposing causes", () => {
		const classified = classify_route_hooks({
			is_current_operation: false,
			operation_id: "op-1",
			result: {
				cause: new Error("late hook"),
				kind: "failed",
			},
		});

		expect(classified).toEqual({
			kind: "stale",
			operation_id: "op-1",
		});
	});

	it("marks hook-completed prepared data as publishable", () => {
		const classified = classify_route_hooks({
			is_current_operation: true,
			operation_id: "op-1",
			result: {
				kind: "completed",
				prepared: prepared_route,
			},
		});

		expect(classified).toEqual({
			kind: "publishable",
			operation_id: "op-1",
			prepared: prepared_route,
		});
	});
});
