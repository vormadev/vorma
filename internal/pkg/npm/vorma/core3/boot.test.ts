import { describe, expect, it } from "vitest";
import {
	begin_boot,
	boot_provisional_snapshot,
	complete_boot_payload,
} from "./boot.ts";
import type { CoreModel, RouteFacts, RouteRenderFacts } from "./model.ts";
import { route_facts_to_state } from "./route.ts";

const route = {
	client_build_id: "build-1",
	error: null,
	history_state: undefined,
	href: "https://example.com/server",
	matches: [
		{
			client_loader_data: undefined,
			input: { from: "server" },
			loader_data: { ok: true },
			module: {},
			module_url: "/boot.js",
			pattern: "/boot",
		},
	],
	params: {},
	splat_values: [],
} satisfies RouteFacts;

const render = {
	client_build_id: "build-1",
	entries: route.matches,
	error: null,
	history_state: undefined,
	params: {},
	splat_values: [],
} satisfies RouteRenderFacts;

const unbooted_model = {
	active_route_operation_id: null,
	browser: null,
	client_build_id: "build-1",
	current: null,
	deployment_id: "",
	focus_revalidation: null,
	last_work_projection: null,
	operations: {},
	phase: "unbooted",
	prefetch_operation_id: null,
	publication_owner_id: null,
	refresh: { kind: "idle" },
	submissions: {},
	use_view_transitions: false,
} satisfies CoreModel;

describe("begin_boot", () => {
	it("starts boot by claiming visible publication and reading payload facts", () => {
		const plan = begin_boot({
			browser: {
				href: "https://example.com/boot",
				key: "browser-1",
				state: { boot: true },
			},
			model: unbooted_model,
			operation_id: "boot-1",
			public_call_ids: ["boot-call-1"],
		});

		expect(plan).toEqual({
			effects: [
				{
					fallback_browser_key: "browser-1",
					operation_id: "boot-1",
					type: "read_boot_payload",
				},
			],
			model: {
				...unbooted_model,
				active_route_operation_id: "boot-1",
				browser: {
					href: "https://example.com/boot",
					key: "browser-1",
					state: { boot: true },
				},
				operations: {
					"boot-1": {
						browser_key: "browser-1",
						href: "https://example.com/boot",
						id: "boot-1",
						kind: "boot",
						public_call_ids: ["boot-call-1"],
						rights: ["publish_route"],
						state: { boot: true },
						status: "started",
					},
				},
				phase: "booting",
				publication_owner_id: "boot-1",
			},
		});
	});

	it("waits for an empty unbooted model", () => {
		expect(
			begin_boot({
				browser: {
					href: "https://example.com/boot",
					key: "browser-1",
					state: undefined,
				},
				model: {
					...unbooted_model,
					phase: "ready",
				},
				operation_id: "boot-1",
				public_call_ids: [],
			}),
		).toBeUndefined();
		expect(
			begin_boot({
				browser: {
					href: "https://example.com/boot",
					key: "browser-1",
					state: undefined,
				},
				model: {
					...unbooted_model,
					active_route_operation_id: "nav-1",
				},
				operation_id: "boot-1",
				public_call_ids: [],
			}),
		).toBeUndefined();
	});
});

describe("complete_boot_payload", () => {
	it("installs provisional route facts and prepares the final boot route", () => {
		const started = begin_boot({
			browser: {
				href: "https://example.com/boot",
				key: "browser-1",
				state: { boot: true },
			},
			model: unbooted_model,
			operation_id: "boot-1",
			public_call_ids: [],
		});
		expect(started).toBeDefined();
		if (!started) {
			return;
		}

		const completed = complete_boot_payload({
			client_build_id: "build-1",
			deployment_id: "",
			model: started.model,
			operation_id: "boot-1",
			payload: { boot: true },
			render,
			route,
		});

		expect(completed?.effects).toEqual([
			{
				client_build_id: "build-1",
				history_state: { boot: true },
				href: "https://example.com/boot",
				operation_id: "boot-1",
				payload: { boot: true },
				trigger: "boot",
				type: "prepare_route",
			},
		]);
		expect(completed?.model.current).toEqual({
			position: {
				href: "https://example.com/boot",
				key: "browser-1",
				state: { boot: true },
			},
			provisional: true,
			render: {
				...render,
				history_state: { boot: true },
			},
			route: {
				...route,
				history_state: { boot: true },
				href: "https://example.com/boot",
			},
		});
		expect(completed?.model.operations["boot-1"]?.status).toBe(
			"classified",
		);
	});
});

describe("boot_provisional_snapshot", () => {
	it("requires boot publication rights", () => {
		const snapshot = boot_provisional_snapshot({
			browser: {
				href: "https://example.com/boot",
				key: "browser-1",
				state: undefined,
			},
			operation: {
				browser_key: "browser-1",
				href: "https://example.com/boot",
				id: "boot-1",
				kind: "boot",
				public_call_ids: [],
				rights: [],
				state: undefined,
				status: "started",
			},
			render,
			route,
		});

		expect(snapshot).toBeUndefined();
	});

	it("installs a browser-positioned provisional route for boot loaders", () => {
		const snapshot = boot_provisional_snapshot({
			browser: {
				href: "https://example.com/boot",
				key: "browser-1",
				state: { boot: true },
			},
			operation: {
				browser_key: "browser-1",
				href: "https://example.com/boot",
				id: "boot-1",
				kind: "boot",
				public_call_ids: [],
				rights: ["publish_route"],
				state: { boot: true },
				status: "started",
			},
			render,
			route,
		});

		expect(snapshot).toEqual({
			position: {
				href: "https://example.com/boot",
				key: "browser-1",
				state: { boot: true },
			},
			provisional: true,
			render: {
				...render,
				history_state: { boot: true },
			},
			route: {
				...route,
				history_state: { boot: true },
				href: "https://example.com/boot",
			},
		});
		expect(route_facts_to_state(snapshot!.route)).toMatchObject({
			historyState: { boot: true },
			href: "https://example.com/boot",
			matches: [
				{
					clientLoaderData: undefined,
					loaderData: { ok: true },
					pattern: "/boot",
				},
			],
		});
	});
});
