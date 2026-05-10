import { describe, expect, it } from "vitest";
import {
	build_skew_default_behavior,
	build_skew_trigger_kind,
	type CoreEffect,
	type CoreModel,
	type PreparedRoute,
	type RouteSnapshot,
} from "./model.ts";
import { route_facts_to_state } from "./route.ts";
import {
	advance_prepared_route,
	advance_route_hooks,
	advance_route_outcome,
	advance_route_preparation,
	commit_route_publication,
	REVALIDATION_BUILD_SKEW_REASON,
	REVALIDATION_MAX_RETRIES_EXHAUSTED_REASON,
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
	active_route_operation_id: "nav-1",
	browser: current.position,
	client_build_id: "build-1",
	current,
	deployment_id: "",
	focus_revalidation: null,
	last_work_projection: null,
	operations: {
		"nav-1": {
			browser_key: "browser-2",
			href: "https://example.com/next",
			id: "nav-1",
			kind: "navigation",
			public_call_ids: ["nav-call-1"],
			redirect_count: 0,
			replace: false,
			rights: ["own_visible_transition", "publish_route"],
			source: "navigate",
			state: { next: true },
			status: "prepared",
			skip_work_indicator: false,
		},
	},
	phase: "ready",
	prefetch_operation_id: null,
	publication_owner_id: "nav-1",
	refresh: { kind: "idle" },
	submissions: {},
	use_view_transitions: true,
} satisfies CoreModel;

const revalidation_model = {
	...model,
	active_route_operation_id: null,
	operations: {
		"reval-1": {
			attempt: 0,
			href: "https://example.com/current",
			id: "reval-1",
			kind: "route_revalidation",
			public_call_ids: ["reval-call-1"],
			reason: "manual",
			rights: ["publish_route", "satisfy_refresh_demand"],
			status: "started",
		},
	},
	publication_owner_id: null,
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
} satisfies CoreModel;

describe("advance_route_outcome", () => {
	it("turns current route data into a preparation effect", () => {
		const transition = advance_route_outcome({
			model: {
				...model,
				operations: {
					"nav-1": {
						...model.operations["nav-1"],
						status: "started",
					},
				},
			},
			outcome: {
				kind: "route_data",
				operation_id: "nav-1",
				payload: { route: true },
			},
		});

		expect(transition).toEqual({
			effects: [
				{
					client_build_id: "build-1",
					history_state: { next: true },
					href: "https://example.com/next",
					operation_id: "nav-1",
					payload: { route: true },
					trigger: "navigation",
					type: "prepare_route",
				},
			],
			kind: "preparing",
			model: {
				...model,
				operations: {
					"nav-1": {
						...model.operations["nav-1"],
						status: "classified",
					},
				},
			},
		});
	});

	it("reports route build skew before preparing route data", () => {
		const transition = advance_route_outcome({
			build_skew_report: {
				behavior: "notify",
				ok: true,
				server_build_id: "build-2",
				status: 200,
			},
			model: {
				...model,
				operations: {
					"nav-1": {
						...model.operations["nav-1"],
						status: "started",
					},
				},
			},
			outcome: {
				kind: "route_data",
				operation_id: "nav-1",
				payload: { route: true },
			},
		});

		expect(transition?.effects[0]).toEqual({
			event: {
				activeClientBuildID: "build-1",
				currentRouteState: route_facts_to_state(current.route),
				currentWorkState: {
					apiRequests: [],
					navigation: {
						href: "https://example.com/next",
						replace: false,
						source: "navigate",
					},
					prefetch: null,
					revalidation: null,
				},
				defaultBehavior: build_skew_default_behavior.notify_only,
				serverBuildID: "build-2",
				triggeringResponse: {
					kind: build_skew_trigger_kind.route,
					ok: true,
					requestedHref: "https://example.com/next",
					status: 200,
					trigger: "navigation",
				},
			},
			type: "report_build_skew",
		});
		expect(transition?.effects[1]).toEqual({
			client_build_id: "build-1",
			history_state: { next: true },
			href: "https://example.com/next",
			operation_id: "nav-1",
			payload: { route: true },
			trigger: "navigation",
			type: "prepare_route",
		});
	});

	it("follows soft navigation redirects without replacing operation identity", () => {
		const transition = advance_route_outcome({
			model: {
				...model,
				operations: {
					"nav-1": {
						...model.operations["nav-1"],
						status: "started",
					},
				},
			},
			outcome: {
				hard: false,
				href: "https://example.com/redirected",
				kind: "soft_redirect",
				operation_id: "nav-1",
			},
		});

		expect(transition).toEqual({
			effects: [
				{
					client_build_id: "build-1",
					href: "https://example.com/redirected",
					operation_id: "nav-1",
					trigger: "navigation",
					type: "fetch_route",
				},
			],
			kind: "redirecting",
			model: {
				...model,
				operations: {
					"nav-1": {
						...model.operations["nav-1"],
						href: "https://example.com/redirected",
						redirect_count: 1,
						source: "redirect",
						status: "started",
					},
				},
			},
		});
	});

	it("requires route ownership before preparing visible route data", () => {
		expect(
			advance_route_outcome({
				model: {
					...model,
					active_route_operation_id: null,
				},
				outcome: {
					kind: "route_data",
					operation_id: "nav-1",
					payload: { route: true },
				},
			}),
		).toBeUndefined();
	});

	it("hard redirects current navigation outcomes and settles the navigation call", () => {
		const transition = advance_route_outcome({
			model: {
				...model,
				operations: {
					"nav-1": {
						...model.operations["nav-1"],
						status: "started",
					},
				},
			},
			outcome: {
				hard: true,
				href: "https://external.example/path",
				kind: "hard_redirect",
				operation_id: "nav-1",
			},
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
			kind: "completed",
			model: {
				...model,
				active_route_operation_id: null,
				operations: {
					"nav-1": {
						...model.operations["nav-1"],
						status: "completed",
					},
				},
				publication_owner_id: null,
			},
		});
	});

	it("hard reloads explicit visible build-skew outcomes to the requested route href", () => {
		const transition = advance_route_outcome({
			model: {
				...model,
				operations: {
					"nav-1": {
						...model.operations["nav-1"],
						status: "started",
					},
				},
			},
			outcome: {
				behavior: "reload",
				kind: "build_skew_reload",
				operation_id: "nav-1",
				server_build_id: "build-2",
			},
		});

		expect(transition?.effects[0]).toEqual({
			href: "https://example.com/next",
			type: "hard_redirect",
		});
		expect(transition?.kind).toBe("completed");
		expect(transition?.model.operations["nav-1"]?.status).toBe("completed");
	});

	it("settles route response failures without publishing", () => {
		const transition = advance_route_outcome({
			model: {
				...model,
				operations: {
					"nav-1": {
						...model.operations["nav-1"],
						status: "started",
					},
				},
			},
			outcome: {
				kind: "route_error",
				operation_id: "nav-1",
				status_text: "Err",
			},
		});

		expect(transition).toEqual({
			effects: [
				{
					public_call_id: "nav-call-1",
					result: { didNavigate: false },
					type: "resolve_public_call",
				},
			],
			kind: "failed",
			model: {
				...model,
				active_route_operation_id: null,
				operations: {
					"nav-1": {
						...model.operations["nav-1"],
						status: "failed",
					},
				},
				publication_owner_id: null,
			},
		});
	});

	it("settles background revalidation build skew without hard redirecting", () => {
		const transition = advance_route_outcome({
			build_skew_report: {
				behavior: "drop",
				ok: false,
				server_build_id: "build-2",
				status: 409,
			},
			model: revalidation_model,
			outcome: {
				behavior: "drop",
				kind: "build_skew_drop",
				operation_id: "reval-1",
				server_build_id: "build-2",
			},
		});

		expect(transition).toEqual({
			effects: [
				{
					event: {
						activeClientBuildID: "build-1",
						currentRouteState: route_facts_to_state(current.route),
						currentWorkState: {
							apiRequests: [],
							navigation: null,
							prefetch: null,
							revalidation: {
								attempt: 0,
								status: "running",
							},
						},
						defaultBehavior:
							build_skew_default_behavior.drop_response,
						serverBuildID: "build-2",
						triggeringResponse: {
							kind: build_skew_trigger_kind.route,
							ok: false,
							requestedHref: "https://example.com/current",
							revalidationReason: "manual",
							status: 409,
							trigger: "revalidation",
						},
					},
					type: "report_build_skew",
				},
				{
					public_call_id: "reval-call-1",
					result: {
						ok: false,
						reason: REVALIDATION_BUILD_SKEW_REASON,
					},
					type: "resolve_public_call",
				},
			],
			kind: "completed",
			model: {
				...revalidation_model,
				operations: {
					"reval-1": {
						...revalidation_model.operations["reval-1"],
						status: "completed",
					},
				},
				refresh: { kind: "idle" },
			},
		});
	});

	it("moves retryable background revalidation failures into retry state", () => {
		const transition = advance_route_outcome({
			model: revalidation_model,
			outcome: {
				kind: "retryable_revalidation_failure",
				operation_id: "reval-1",
				status_text: "Err",
			},
			revalidation_retry: {
				backoff_base_ms: 500,
				backoff_cap_ms: 1200,
				max_retries: 3,
				next_operation_id: "reval-2",
				timer_id: "retry-timer",
			},
		});

		expect(transition).toEqual({
			effects: [
				{
					delay_ms: 500,
					operation_id: "reval-2",
					timer_id: "retry-timer",
					type: "start_timer",
				},
			],
			kind: "retrying",
			model: {
				...revalidation_model,
				operations: {
					"reval-1": {
						...revalidation_model.operations["reval-1"],
						status: "failed",
					},
				},
				refresh: {
					attempt: 1,
					demand: {
						operation_id: "reval-2",
						public_call_ids: ["reval-call-1"],
						reason: "retry",
						skip_work_indicator: false,
					},
					kind: "retrying",
					timer_id: "retry-timer",
				},
			},
		});
	});

	it("transfers background revalidation soft redirects to replace navigation", () => {
		const transition = advance_route_outcome({
			model: revalidation_model,
			outcome: {
				hard: false,
				href: "https://example.com/new-page",
				kind: "soft_redirect",
				operation_id: "reval-1",
			},
			revalidation_redirect: {
				browser_key: "browser-2",
				navigation_operation_id: "nav-redirect-1",
				skip_work_indicator: false,
				state: undefined,
			},
		});

		expect(transition).toEqual({
			effects: [
				{
					client_build_id: "build-1",
					href: "https://example.com/new-page",
					operation_id: "nav-redirect-1",
					trigger: "navigation",
					type: "fetch_route",
				},
			],
			kind: "redirecting",
			model: {
				...revalidation_model,
				active_route_operation_id: "nav-redirect-1",
				operations: {
					"reval-1": {
						...revalidation_model.operations["reval-1"],
						status: "completed",
					},
					"nav-redirect-1": {
						browser_key: "browser-2",
						href: "https://example.com/new-page",
						id: "nav-redirect-1",
						kind: "navigation",
						public_call_ids: [],
						redirect_count: 0,
						replace: true,
						rights: [
							"abort_effects",
							"own_visible_transition",
							"publish_route",
							"satisfy_refresh_demand",
						],
						source: "redirect",
						state: undefined,
						status: "started",
						skip_work_indicator: false,
					},
				},
				publication_owner_id: "nav-redirect-1",
				refresh: {
					attempt: 0,
					demand: {
						operation_id: "reval-1",
						public_call_ids: ["reval-call-1"],
						reason: "manual",
						skip_work_indicator: false,
					},
					kind: "running",
					operation_id: "nav-redirect-1",
				},
			},
		});
	});

	it("settles transferred refresh waiters when redirected navigation settles", () => {
		const settled = settle_route_publication({
			model: {
				...model,
				active_route_operation_id: null,
				operations: {
					"nav-redirect-1": {
						browser_key: "browser-2",
						href: "https://example.com/new-page",
						id: "nav-redirect-1",
						kind: "navigation",
						public_call_ids: [],
						redirect_count: 0,
						replace: true,
						rights: [
							"abort_effects",
							"own_visible_transition",
							"publish_route",
							"satisfy_refresh_demand",
						],
						source: "redirect",
						state: undefined,
						status: "published",
						skip_work_indicator: false,
					},
				},
				publication_owner_id: null,
				refresh: {
					attempt: 0,
					demand: {
						operation_id: "reval-1",
						public_call_ids: ["reval-call-1"],
						reason: "manual",
						skip_work_indicator: false,
					},
					kind: "running",
					operation_id: "nav-redirect-1",
				},
			},
			operation_id: "nav-redirect-1",
		});

		expect(settled).toEqual({
			effects: [
				{
					public_call_id: "reval-call-1",
					result: { ok: true },
					type: "resolve_public_call",
				},
			],
			model: {
				...model,
				active_route_operation_id: null,
				operations: {
					"nav-redirect-1": {
						browser_key: "browser-2",
						href: "https://example.com/new-page",
						id: "nav-redirect-1",
						kind: "navigation",
						public_call_ids: [],
						redirect_count: 0,
						replace: true,
						rights: [
							"abort_effects",
							"own_visible_transition",
							"publish_route",
							"satisfy_refresh_demand",
						],
						source: "redirect",
						state: undefined,
						status: "settled",
						skip_work_indicator: false,
					},
				},
				publication_owner_id: null,
				refresh: { kind: "idle" },
			},
		});
	});

	it("settles ignored background refresh outcomes as freshness without publication", () => {
		const blocked_model = {
			...revalidation_model,
			active_route_operation_id: "nav-1",
			operations: {
				...revalidation_model.operations,
				"nav-1": model.operations["nav-1"],
			},
			publication_owner_id: "nav-1",
		} satisfies CoreModel;
		const transition = advance_route_outcome({
			model: blocked_model,
			outcome: {
				kind: "ignored_background_refresh",
				operation_id: "reval-1",
			},
		});

		expect(transition).toEqual({
			effects: [
				{
					public_call_id: "reval-call-1",
					result: { ok: true },
					type: "resolve_public_call",
				},
			],
			kind: "ignored_stale",
			model: {
				...blocked_model,
				operations: {
					...blocked_model.operations,
					"reval-1": {
						...blocked_model.operations["reval-1"],
						status: "ignored_stale",
					},
				},
				refresh: { kind: "idle" },
			},
		});
	});

	it("settles invalid background revalidation redirects as freshness without native redirect", () => {
		const transition = advance_route_outcome({
			model: revalidation_model,
			outcome: {
				href: "mailto:test@example.com",
				kind: "invalid_redirect",
				operation_id: "reval-1",
			},
		});

		expect(transition?.effects).toEqual([
			{
				public_call_id: "reval-call-1",
				result: { ok: true },
				type: "resolve_public_call",
			},
		]);
		expect(transition?.kind).toBe("completed");
		expect(transition?.model.refresh).toEqual({ kind: "idle" });
	});

	it("settles exhausted background revalidation failures", () => {
		const transition = advance_route_outcome({
			model: revalidation_model,
			outcome: {
				kind: "terminal_revalidation_failure",
				operation_id: "reval-1",
				status_text: "Err",
			},
		});

		expect(transition).toEqual({
			effects: [
				{
					public_call_id: "reval-call-1",
					result: {
						ok: false,
						reason: REVALIDATION_MAX_RETRIES_EXHAUSTED_REASON,
					},
					type: "resolve_public_call",
				},
			],
			kind: "failed",
			model: {
				...revalidation_model,
				operations: {
					"reval-1": {
						...revalidation_model.operations["reval-1"],
						status: "failed",
					},
				},
				refresh: { kind: "idle" },
			},
		});
	});
});

describe("advance_prepared_route", () => {
	it("consumes prepared outcomes through readiness and publication", () => {
		const transition = advance_route_preparation({
			hooks_required: false,
			model: {
				...model,
				operations: {
					"nav-1": {
						...model.operations["nav-1"],
						status: "classified",
					},
				},
			},
			outcome: {
				kind: "prepared",
				operation_id: "nav-1",
				prepared,
			},
			view_transition_available: true,
		});

		expect(transition?.kind).toBe("publishing");
		expect(transition?.model.operations["nav-1"]?.status).toBe(
			"publishing",
		);
	});

	it("ignores stale preparation outcomes", () => {
		expect(
			advance_route_preparation({
				hooks_required: false,
				model,
				outcome: {
					kind: "stale",
					operation_id: "nav-1",
				},
				view_transition_available: true,
			}),
		).toBeUndefined();
	});

	it("settles aborted navigation preparation without publishing", () => {
		const transition = advance_route_preparation({
			hooks_required: false,
			model: {
				...model,
				operations: {
					"nav-1": {
						...model.operations["nav-1"],
						status: "classified",
					},
				},
			},
			outcome: {
				kind: "aborted",
				operation_id: "nav-1",
			},
			view_transition_available: true,
		});

		expect(transition).toEqual({
			effects: [
				{
					public_call_id: "nav-call-1",
					result: { didNavigate: false },
					type: "resolve_public_call",
				},
			],
			kind: "aborted",
			model: {
				...model,
				active_route_operation_id: null,
				operations: {
					"nav-1": {
						...model.operations["nav-1"],
						status: "aborted",
					},
				},
				publication_owner_id: null,
			},
		});
	});

	it("rejects failed boot preparation public calls", () => {
		const cause = new Error("boot failed");
		const transition = advance_route_preparation({
			hooks_required: false,
			model: {
				...model,
				active_route_operation_id: "boot-1",
				operations: {
					"boot-1": {
						browser_key: "browser-1",
						href: "https://example.com/current",
						id: "boot-1",
						kind: "boot",
						public_call_ids: ["boot-call-1"],
						rights: ["publish_route"],
						state: undefined,
						status: "classified",
					},
				},
				phase: "booting",
				publication_owner_id: "boot-1",
			},
			outcome: {
				cause,
				kind: "failed",
				operation_id: "boot-1",
			},
			view_transition_available: true,
		});

		expect(transition).toEqual({
			effects: [
				{
					cause,
					public_call_id: "boot-call-1",
					type: "reject_public_call",
				},
			],
			kind: "failed",
			model: {
				...model,
				active_route_operation_id: null,
				operations: {
					"boot-1": {
						browser_key: "browser-1",
						href: "https://example.com/current",
						id: "boot-1",
						kind: "boot",
						public_call_ids: ["boot-call-1"],
						rights: ["publish_route"],
						state: undefined,
						status: "failed",
					},
				},
				phase: "booting",
				publication_owner_id: null,
			},
		});
	});

	it("grants publication ownership to current prepared background revalidation", () => {
		const transition = advance_route_preparation({
			hooks_required: false,
			model: {
				...model,
				active_route_operation_id: null,
				operations: {
					"reval-1": {
						attempt: 0,
						href: "https://example.com/current",
						id: "reval-1",
						kind: "route_revalidation",
						public_call_ids: ["reval-call-1"],
						reason: "manual",
						rights: ["publish_route", "satisfy_refresh_demand"],
						status: "classified",
					},
				},
				publication_owner_id: null,
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
			},
			outcome: {
				kind: "prepared",
				operation_id: "reval-1",
				prepared,
			},
			view_transition_available: true,
		});

		if (transition?.kind !== "publishing") {
			throw new Error("expected publishing transition");
		}
		expect(transition.model.publication_owner_id).toBe("reval-1");
		expect(transition.model.operations["reval-1"]?.status).toBe(
			"publishing",
		);
		const commit_effect = transition.effects.find(
			(effect): effect is Extract<CoreEffect, { type: "commit" }> => {
				return effect.type === "commit";
			},
		);
		expect(commit_effect?.next.position).toEqual(current.position);
		expect(transition.effects.at(-1)).toEqual({
			public_call_id: "reval-call-1",
			result: { ok: true },
			type: "resolve_public_call",
		});
	});

	it("does not grant background revalidation publication ownership while visible route work is active", () => {
		expect(
			advance_route_preparation({
				hooks_required: false,
				model: {
					...model,
					active_route_operation_id: "nav-2",
					operations: {
						"nav-2": {
							browser_key: "browser-2",
							href: "https://example.com/next",
							id: "nav-2",
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
						"reval-1": {
							attempt: 0,
							href: "https://example.com/current",
							id: "reval-1",
							kind: "route_revalidation",
							public_call_ids: ["reval-call-1"],
							reason: "manual",
							rights: ["publish_route", "satisfy_refresh_demand"],
							status: "classified",
						},
					},
					publication_owner_id: "nav-2",
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
				},
				outcome: {
					kind: "prepared",
					operation_id: "reval-1",
					prepared,
				},
				view_transition_available: true,
			}),
		).toBeUndefined();
	});

	it("does not grant background revalidation publication ownership after the browser leaves the document", () => {
		expect(
			advance_route_preparation({
				hooks_required: false,
				model: {
					...model,
					active_route_operation_id: null,
					browser: {
						...current.position,
						href: "https://example.com/other",
					},
					operations: {
						"reval-1": {
							attempt: 0,
							href: "https://example.com/current",
							id: "reval-1",
							kind: "route_revalidation",
							public_call_ids: ["reval-call-1"],
							reason: "manual",
							rights: ["publish_route", "satisfy_refresh_demand"],
							status: "classified",
						},
					},
					publication_owner_id: null,
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
				},
				outcome: {
					kind: "prepared",
					operation_id: "reval-1",
					prepared,
				},
				view_transition_available: true,
			}),
		).toBeUndefined();
	});

	it("runs route hooks with public current and retargeted next route state", () => {
		const transition = advance_prepared_route({
			hooks_required: true,
			model,
			operation_id: "nav-1",
			prepared,
			view_transition_available: true,
		});

		expect(transition?.kind).toBe("hooks_running");
		expect(transition?.model.current).toBe(current);
		expect(transition?.model.operations["nav-1"]?.status).toBe(
			"hooks_running",
		);
		expect(transition?.effects).toEqual([
			{
				operation_id: "nav-1",
				request: {
					current_route: route_facts_to_state(current.route),
					next_route: route_facts_to_state({
						...prepared.route,
						history_state: { next: true },
						href: "https://example.com/next",
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
							href: "https://example.com/next",
						},
					},
					trigger: "navigation",
				},
				type: "run_route_hooks",
			},
		]);
	});

	it("publishes hook-completed route data through the operation model", () => {
		const transition = advance_route_hooks({
			model: {
				...model,
				operations: {
					"nav-1": {
						...model.operations["nav-1"],
						status: "hooks_running",
					},
				},
			},
			outcome: {
				kind: "publishable",
				operation_id: "nav-1",
				prepared,
			},
			view_transition_available: true,
		});

		expect(transition?.kind).toBe("publishing");
		expect(transition?.model.operations["nav-1"]?.status).toBe(
			"publishing",
		);
	});

	it("ignores stale route hook outcomes", () => {
		expect(
			advance_route_hooks({
				model,
				outcome: {
					kind: "stale",
					operation_id: "nav-1",
				},
				view_transition_available: true,
			}),
		).toBeUndefined();
	});

	it("settles aborted navigation hooks without publishing", () => {
		const transition = advance_route_hooks({
			model: {
				...model,
				operations: {
					"nav-1": {
						...model.operations["nav-1"],
						status: "hooks_running",
					},
				},
			},
			outcome: {
				kind: "aborted",
				operation_id: "nav-1",
			},
			view_transition_available: true,
		});

		expect(transition).toEqual({
			effects: [
				{
					public_call_id: "nav-call-1",
					result: { didNavigate: false },
					type: "resolve_public_call",
				},
			],
			kind: "aborted",
			model: {
				...model,
				active_route_operation_id: null,
				operations: {
					"nav-1": {
						...model.operations["nav-1"],
						status: "aborted",
					},
				},
				publication_owner_id: null,
			},
		});
	});

	it("publishes prepared route data through a view-transition-decorated transaction", () => {
		const transition = advance_prepared_route({
			hooks_required: false,
			model,
			operation_id: "nav-1",
			prepared,
			view_transition_available: true,
		});

		if (transition?.kind !== "publishing") {
			throw new Error("expected publishing transition");
		}

		expect(transition.model.current).toBe(current);
		expect(transition.model.operations["nav-1"]?.status).toBe("publishing");
		const view_transition_effect = transition.effects[0];
		if (view_transition_effect?.type !== "run_view_transition") {
			throw new Error("expected view transition effect");
		}
		expect(view_transition_effect.transaction.before_transition).toEqual([
			{ type: "save_current_scroll" },
		]);
		expect(view_transition_effect.transaction.inside_transition[0]).toEqual(
			{
				position: {
					href: "https://example.com/next",
					key: "browser-2",
					state: { next: true },
				},
				replace: false,
				type: "write_history",
			},
		);
		const commit_effect =
			view_transition_effect.transaction.inside_transition.find(
				(effect): effect is Extract<CoreEffect, { type: "commit" }> => {
					return effect.type === "commit";
				},
			);
		expect(commit_effect?.next.position).toEqual({
			href: "https://example.com/next",
			key: "browser-2",
			state: { next: true },
		});
	});

	it("installs the published snapshot at the commit boundary", () => {
		const transition = advance_prepared_route({
			hooks_required: false,
			model,
			operation_id: "nav-1",
			prepared,
			view_transition_available: true,
		});

		if (transition?.kind !== "publishing") {
			throw new Error("expected publishing transition");
		}
		const view_transition_effect = transition.effects[0];
		if (view_transition_effect?.type !== "run_view_transition") {
			throw new Error("expected view transition effect");
		}
		const commit_effect =
			view_transition_effect.transaction.inside_transition.find(
				(effect): effect is Extract<CoreEffect, { type: "commit" }> => {
					return effect.type === "commit";
				},
			);
		if (!commit_effect) {
			throw new Error("expected commit effect");
		}
		const committed = commit_route_publication({
			model: transition.model,
			next: commit_effect.next,
			operation_id: commit_effect.operation_id,
		});

		expect(committed?.current).toEqual(commit_effect.next);
		expect(committed?.browser).toEqual(commit_effect.next.position);
		expect(committed?.active_route_operation_id).toBeNull();
		expect(committed?.publication_owner_id).toBeNull();
		expect(committed?.operations["nav-1"]?.status).toBe("published");
	});

	it("does not publish without publication ownership", () => {
		expect(
			advance_prepared_route({
				hooks_required: false,
				model: {
					...model,
					publication_owner_id: null,
				},
				operation_id: "nav-1",
				prepared,
				view_transition_available: true,
			}),
		).toBeUndefined();
	});

	it("does not commit a route publication without publication ownership", () => {
		const transition = advance_prepared_route({
			hooks_required: false,
			model,
			operation_id: "nav-1",
			prepared,
			view_transition_available: true,
		});

		if (transition?.kind !== "publishing") {
			throw new Error("expected publishing transition");
		}
		const view_transition_effect = transition.effects[0];
		if (view_transition_effect?.type !== "run_view_transition") {
			throw new Error("expected view transition effect");
		}
		const commit_effect =
			view_transition_effect.transaction.inside_transition.find(
				(effect): effect is Extract<CoreEffect, { type: "commit" }> => {
					return effect.type === "commit";
				},
			);
		if (!commit_effect) {
			throw new Error("expected commit effect");
		}

		expect(
			commit_route_publication({
				model: {
					...transition.model,
					publication_owner_id: null,
				},
				next: commit_effect.next,
				operation_id: "nav-1",
			}),
		).toBeUndefined();
	});

	it("settles boot publication and starts deferred pending refresh", () => {
		const settled = settle_route_publication({
			model: {
				...model,
				active_route_operation_id: null,
				operations: {
					"boot-1": {
						browser_key: "browser-1",
						href: "https://example.com/current",
						id: "boot-1",
						kind: "boot",
						public_call_ids: [],
						rights: ["publish_route"],
						state: undefined,
						status: "published",
					},
				},
				phase: "booting",
				publication_owner_id: null,
				refresh: {
					attempt: 0,
					demand: {
						after_operation_id: "boot-1",
						operation_id: "reval-1",
						public_call_ids: ["reval-call-1"],
						reason: "apiRequest",
						skip_work_indicator: true,
					},
					kind: "pending",
				},
			},
			operation_id: "boot-1",
		});

		expect(settled?.model.phase).toBe("ready");
		expect(settled?.model.operations["boot-1"]?.status).toBe("settled");
		expect(settled?.model.operations["reval-1"]).toEqual({
			attempt: 0,
			href: "https://example.com/current",
			id: "reval-1",
			kind: "route_revalidation",
			public_call_ids: ["reval-call-1"],
			reason: "apiRequest",
			rights: ["publish_route", "satisfy_refresh_demand"],
			status: "started",
		});
		expect(settled?.model.refresh).toEqual({
			attempt: 0,
			demand: {
				after_operation_id: "boot-1",
				operation_id: "reval-1",
				public_call_ids: ["reval-call-1"],
				reason: "apiRequest",
				skip_work_indicator: true,
			},
			kind: "running",
			operation_id: "reval-1",
		});
		expect(settled?.effects).toEqual([
			{
				client_build_id: "build-1",
				href: "https://example.com/current",
				operation_id: "reval-1",
				trigger: "revalidation",
				type: "fetch_route",
			},
		]);
	});

	it("settles published route revalidation by satisfying running refresh", () => {
		const settled = settle_route_publication({
			model: {
				...model,
				active_route_operation_id: null,
				operations: {
					"reval-1": {
						attempt: 0,
						href: "https://example.com/current",
						id: "reval-1",
						kind: "route_revalidation",
						public_call_ids: ["reval-call-1"],
						reason: "manual",
						rights: ["publish_route", "satisfy_refresh_demand"],
						status: "published",
					},
				},
				publication_owner_id: null,
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
			},
			operation_id: "reval-1",
		});

		expect(settled).toEqual({
			effects: [],
			model: {
				...model,
				active_route_operation_id: null,
				operations: {
					"reval-1": {
						attempt: 0,
						href: "https://example.com/current",
						id: "reval-1",
						kind: "route_revalidation",
						public_call_ids: ["reval-call-1"],
						reason: "manual",
						rights: ["publish_route", "satisfy_refresh_demand"],
						status: "settled",
					},
				},
				publication_owner_id: null,
				refresh: { kind: "idle" },
			},
		});
	});
});
