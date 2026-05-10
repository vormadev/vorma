import { describe, expect, it } from "vitest";
import {
	classify_route_preparation,
	classify_route_response,
} from "./classify.ts";
import type { CoreModel, PreparedRoute, RouteSnapshot } from "./model.ts";
import { begin_route_navigation } from "./navigation.ts";
import { begin_pending_refresh, request_refresh } from "./refresh.ts";
import { route_facts_to_state } from "./route.ts";
import {
	advance_route_outcome,
	advance_route_preparation,
	commit_route_publication,
	settle_route_publication,
} from "./route_operation.ts";

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

const prepared = {
	dom: { title: "Next" },
	render: {
		client_build_id: "build-1",
		entries: [
			{
				client_loader_data: undefined,
				input: undefined,
				loader_data: { next: true },
				module: {},
				module_url: "/next.js",
				pattern: "/next",
			},
		],
		error: null,
		history_state: undefined,
		params: {},
		splat_values: [],
	},
	route: {
		client_build_id: "build-1",
		error: null,
		history_state: undefined,
		href: "https://example.com/server-next",
		matches: [
			{
				client_loader_data: undefined,
				input: undefined,
				loader_data: { next: true },
				module: {},
				module_url: "/next.js",
				pattern: "/next",
			},
		],
		params: {},
		splat_values: [],
	},
} satisfies PreparedRoute;

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

describe("visible navigation route flow", () => {
	it("runs start, response, preparation, publication, commit, and settlement", () => {
		const started = begin_route_navigation({
			browser_key: "browser-2",
			href: "https://example.com/next",
			model,
			operation_id: "nav-1",
			public_call_ids: ["nav-call-1"],
			replace: false,
			skip_work_indicator: false,
			state: { next: true },
		});
		expect(started).toBeDefined();
		if (!started) {
			return;
		}
		expect(started.effects).toEqual([
			{
				client_build_id: "build-1",
				href: "https://example.com/next",
				operation_id: "nav-1",
				trigger: "navigation",
				type: "fetch_route",
			},
		]);

		const route_classification = classify_route_response({
			active_client_build_id: "build-1",
			is_current_operation: true,
			max_redirects: 10,
			max_revalidation_retries: 3,
			operation_id: "nav-1",
			operation_kind: "navigation",
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
		const prepared_route = advance_route_outcome({
			model: started.model,
			outcome: route_classification.outcome,
		});
		expect(prepared_route?.effects).toEqual([
			{
				client_build_id: "build-1",
				history_state: { next: true },
				href: "https://example.com/next",
				operation_id: "nav-1",
				payload: { route: true },
				trigger: "navigation",
				type: "prepare_route",
			},
		]);
		expect(prepared_route?.kind).toBe("preparing");
		if (!prepared_route) {
			return;
		}

		const preparation = classify_route_preparation({
			is_current_operation: true,
			operation_id: "nav-1",
			result: {
				kind: "prepared",
				prepared,
			},
		});
		const publishing = advance_route_preparation({
			hooks_required: false,
			model: prepared_route.model,
			outcome: preparation,
			view_transition_available: false,
		});
		expect(publishing?.kind).toBe("publishing");
		if (!publishing || publishing.kind !== "publishing") {
			return;
		}
		expect(
			publishing.effects.map((effect) => {
				return effect.type;
			}),
		).toEqual([
			"save_current_scroll",
			"write_history",
			"apply_publication_dom",
			"commit",
			"render",
			"resolve_public_call",
		]);

		const commit_effect = publishing.effects.find((effect) => {
			return effect.type === "commit";
		});
		expect(commit_effect).toBeDefined();
		if (!commit_effect || commit_effect.type !== "commit") {
			return;
		}
		const committed = commit_route_publication({
			model: publishing.model,
			next: commit_effect.next,
			operation_id: "nav-1",
		});
		expect(committed?.active_route_operation_id).toBeNull();
		expect(committed?.browser).toEqual({
			href: "https://example.com/next",
			key: "browser-2",
			state: { next: true },
		});
		expect(
			committed?.current
				? route_facts_to_state(committed.current.route)
				: undefined,
		).toMatchObject({
			historyState: { next: true },
			href: "https://example.com/next",
			matches: [
				{
					loaderData: { next: true },
					pattern: "/next",
				},
			],
		});
		if (!committed) {
			return;
		}

		const settled = settle_route_publication({
			model: committed,
			operation_id: "nav-1",
		});
		expect(settled).toEqual({
			effects: [],
			model: {
				...committed,
				operations: {
					...committed.operations,
					"nav-1": {
						...committed.operations["nav-1"],
						status: "settled",
					},
				},
			},
		});
	});
});

describe("background revalidation route flow", () => {
	it("runs demand, start, response, preparation, publication, commit, and settlement", () => {
		const refresh = request_refresh({
			debounce: false,
			debounce_ms: 0,
			demand: {
				operation_id: "reval-1",
				public_call_ids: ["reval-call-1"],
				reason: "manual",
				skip_work_indicator: false,
			},
			refresh: model.refresh,
		});
		expect(refresh.effects).toEqual([]);

		const started = begin_pending_refresh({
			model: {
				...model,
				refresh: refresh.refresh,
			},
		});
		expect(started).toBeDefined();
		if (!started) {
			return;
		}
		expect(started.effects).toEqual([
			{
				client_build_id: "build-1",
				href: "https://example.com/current",
				operation_id: "reval-1",
				trigger: "revalidation",
				type: "fetch_route",
			},
		]);

		const route_classification = classify_route_response({
			active_client_build_id: "build-1",
			is_current_operation: true,
			max_redirects: 10,
			max_revalidation_retries: 3,
			operation_id: "reval-1",
			operation_kind: "route_revalidation",
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
		const prepared_route = advance_route_outcome({
			model: started.model,
			outcome: route_classification.outcome,
		});
		expect(prepared_route?.effects).toEqual([
			{
				client_build_id: "build-1",
				history_state: { current: true },
				href: "https://example.com/current",
				operation_id: "reval-1",
				payload: { route: true },
				trigger: "revalidation",
				type: "prepare_route",
			},
		]);
		expect(prepared_route?.kind).toBe("preparing");
		if (!prepared_route) {
			return;
		}

		const preparation = classify_route_preparation({
			is_current_operation: true,
			operation_id: "reval-1",
			result: {
				kind: "prepared",
				prepared,
			},
		});
		const publishing = advance_route_preparation({
			hooks_required: false,
			model: prepared_route.model,
			outcome: preparation,
			view_transition_available: false,
		});
		expect(publishing?.kind).toBe("publishing");
		if (!publishing || publishing.kind !== "publishing") {
			return;
		}
		expect(
			publishing.effects.map((effect) => {
				return effect.type;
			}),
		).toEqual([
			"apply_publication_dom",
			"commit",
			"render",
			"resolve_public_call",
		]);

		const commit_effect = publishing.effects.find((effect) => {
			return effect.type === "commit";
		});
		expect(commit_effect).toBeDefined();
		if (!commit_effect || commit_effect.type !== "commit") {
			return;
		}
		const committed = commit_route_publication({
			model: publishing.model,
			next: commit_effect.next,
			operation_id: "reval-1",
		});
		expect(committed?.browser).toEqual(current.position);
		expect(committed?.publication_owner_id).toBeNull();
		expect(
			committed?.current
				? route_facts_to_state(committed.current.route)
				: undefined,
		).toMatchObject({
			historyState: { current: true },
			href: "https://example.com/current",
			matches: [
				{
					loaderData: { next: true },
					pattern: "/next",
				},
			],
		});
		if (!committed) {
			return;
		}

		const settled = settle_route_publication({
			model: committed,
			operation_id: "reval-1",
		});
		expect(settled).toEqual({
			effects: [],
			model: {
				...committed,
				operations: {
					...committed.operations,
					"reval-1": {
						...committed.operations["reval-1"],
						status: "settled",
					},
				},
				refresh: { kind: "idle" },
			},
		});
	});
});
