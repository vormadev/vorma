/// <reference types="vite/client" />

import { afterEach, beforeEach, describe, expect, it } from "vitest";
import type { PreparedRoute, RouteSnapshot } from "../model.ts";
import { route_facts_to_state } from "../route.ts";
import {
	advance_hmr_preparation,
	begin_hmr_update,
	commit_hmr_publication,
	settle_hmr_publication,
	type HMRDevModel,
} from "./hmr.ts";

let original_dev: boolean;

beforeEach(() => {
	original_dev = import.meta.env.DEV;
	import.meta.env.DEV = true;
});

afterEach(() => {
	import.meta.env.DEV = original_dev;
});

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
				client_loader_data: { before: true },
				input: undefined,
				loader_data: { current: true },
				module: { version: 1 },
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
				client_loader_data: { before: true },
				input: undefined,
				loader_data: { current: true },
				module: { version: 1 },
				module_url: "/current.js",
				pattern: "/current",
			},
		],
		params: {},
		splat_values: [],
	},
} satisfies RouteSnapshot;

const prepared = {
	dom: { title: "HMR" },
	render: {
		client_build_id: "build-1",
		entries: [
			{
				client_loader_data: { after: true },
				input: undefined,
				loader_data: { current: true },
				module: { version: 2 },
				module_url: "/current.js",
				pattern: "/current",
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
		href: "https://example.com/current",
		matches: [
			{
				client_loader_data: { after: true },
				input: undefined,
				loader_data: { current: true },
				module: { version: 2 },
				module_url: "/current.js",
				pattern: "/current",
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
	hmr_operation_id: null,
	last_work_projection: null,
	operations: {},
	phase: "ready",
	prefetch_operation_id: null,
	publication_owner_id: null,
	refresh: { kind: "idle" },
	submissions: {},
	use_view_transitions: true,
} satisfies HMRDevModel;

describe("begin_hmr_update", () => {
	it("starts a matching dev-only HMR update as publication-owned work", () => {
		const plan = begin_hmr_update({
			model,
			module_url: "/current.js",
			operation_id: "hmr-1",
			rerun_client_loader_patterns: ["/current"],
		});

		expect(plan).toEqual({
			effects: [
				{
					module_url: "https://example.com/current.js",
					operation_id: "hmr-1",
					pattern: "/current",
					rerun_client_loader: true,
					type: "prepare_hmr_route",
				},
			],
			model: {
				...model,
				hmr_operation_id: "hmr-1",
				operations: {
					"hmr-1": {
						id: "hmr-1",
						kind: "hmr_update",
						module_url: "https://example.com/current.js",
						pattern: "/current",
						public_call_ids: [],
						rerun_client_loader: true,
						rights: ["abort_effects", "publish_route"],
						status: "started",
					},
				},
				publication_owner_id: "hmr-1",
			},
		});
	});

	it("ignores non-dev, non-current, and visible-route-owned updates", () => {
		import.meta.env.DEV = false;
		expect(
			begin_hmr_update({
				model,
				module_url: "/current.js",
				operation_id: "hmr-1",
				rerun_client_loader_patterns: [],
			}),
		).toBeUndefined();
		import.meta.env.DEV = true;
		expect(
			begin_hmr_update({
				model,
				module_url: "/other.js",
				operation_id: "hmr-1",
				rerun_client_loader_patterns: [],
			}),
		).toBeUndefined();
		expect(
			begin_hmr_update({
				model: {
					...model,
					active_route_operation_id: "nav-1",
				},
				module_url: "/current.js",
				operation_id: "hmr-1",
				rerun_client_loader_patterns: [],
			}),
		).toBeUndefined();
	});
});

describe("advance_hmr_preparation", () => {
	it("publishes a prepared HMR route update without visible navigation rights", () => {
		const started = begin_hmr_update({
			model,
			module_url: "/current.js",
			operation_id: "hmr-1",
			rerun_client_loader_patterns: ["/current"],
		});
		expect(started).toBeDefined();
		if (!started) {
			return;
		}

		const publishing = advance_hmr_preparation({
			model: started.model,
			outcome: {
				kind: "prepared",
				operation_id: "hmr-1",
				prepared,
			},
			view_transition_available: true,
		});

		expect(publishing?.kind).toBe("publishing");
		if (!publishing || publishing.kind !== "publishing") {
			return;
		}
		expect(
			publishing.effects.map((effect) => {
				return effect.type;
			}),
		).toEqual(["apply_publication_dom", "commit", "render"]);

		const commit_effect = publishing.effects.find((effect) => {
			return effect.type === "commit";
		});
		expect(commit_effect).toBeDefined();
		if (!commit_effect || commit_effect.type !== "commit") {
			return;
		}
		expect(commit_effect.commit.route_update?.reason).toBe("revalidation");

		const committed = commit_hmr_publication({
			model: publishing.model,
			next: commit_effect.next,
			operation_id: "hmr-1",
		});
		expect(committed?.publication_owner_id).toBeNull();
		expect(
			committed?.current
				? route_facts_to_state(committed.current.route)
				: undefined,
		).toMatchObject({
			href: "https://example.com/current",
			matches: [
				{
					clientLoaderData: { after: true },
					pattern: "/current",
				},
			],
		});
		if (!committed) {
			return;
		}

		const settled = settle_hmr_publication({
			model: committed,
			operation_id: "hmr-1",
		});
		expect(settled?.hmr_operation_id).toBeNull();
		expect(settled?.operations["hmr-1"]?.status).toBe("settled");
	});
});
