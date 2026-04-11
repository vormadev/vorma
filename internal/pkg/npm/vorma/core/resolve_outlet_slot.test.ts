import { describe, expect, it } from "vitest";
import type { RouteEntry } from "./create_client_core.ts";
import { get_entry_key, resolve_outlet_slot } from "./resolve_outlet_slot.ts";

function make_entry(overrides: Partial<RouteEntry> = {}): RouteEntry {
	return {
		pattern: overrides.pattern ?? "/",
		module_url: overrides.module_url ?? "/mod.js",
		module: overrides.module ?? {},
		data: overrides.data ?? null,
		client_data: overrides.client_data ?? undefined,
		error: overrides.error ?? undefined,
	};
}

function make_component_entry(
	pattern: string,
	component: (props: any) => any,
	error_boundary?: (props: { error: unknown }) => any,
): RouteEntry {
	return make_entry({
		pattern,
		module_url: `/${pattern}.js`,
		module: {
			default: {
				pattern,
				component,
				error_boundary,
			},
		},
	});
}

function make_error_entry(
	pattern: string,
	error: unknown,
	opts?: {
		component?: (props: any) => any;
		error_boundary?: (props: { error: unknown }) => any;
	},
): RouteEntry {
	return make_entry({
		pattern,
		module_url: `/${pattern}.js`,
		module: {
			default: {
				pattern,
				component: opts?.component ?? (() => null),
				error_boundary: opts?.error_boundary,
			},
		},
		error,
	});
}

describe("resolve_outlet_slot", () => {
	it("returns empty for out-of-bounds index", () => {
		const entries = [make_component_entry("/", () => "root")];
		const slot = resolve_outlet_slot(entries, 5, undefined);
		expect(slot.kind).toBe("empty");
	});

	it("returns empty for empty entries array", () => {
		const slot = resolve_outlet_slot([], 0, undefined);
		expect(slot.kind).toBe("empty");
	});

	it("returns component with stable wrapper for valid index", () => {
		const comp = () => "root";
		const entries = [make_component_entry("/", comp)];
		const slot = resolve_outlet_slot(entries, 0, undefined);
		expect(slot.kind).toBe("component");
		if (slot.kind === "component") {
			expect(slot.component({})).toBe("root");
		}
	});

	it("returns pass_through when entry has no component and deeper entries exist", () => {
		const entries = [
			make_entry({ pattern: "/", module: {} }),
			make_component_entry("/child", () => "child"),
		];
		const slot = resolve_outlet_slot(entries, 0, undefined);
		expect(slot.kind).toBe("pass_through");
	});

	it("returns empty when entry has no component and is last", () => {
		const entries = [make_entry({ pattern: "/", module: {} })];
		const slot = resolve_outlet_slot(entries, 0, undefined);
		expect(slot.kind).toBe("empty");
	});

	it("returns error slot with route error boundary", () => {
		const boundary = (props: { error: unknown }) =>
			`handled:${props.error as any}`;
		const entries = [
			make_error_entry("/", "boom", { error_boundary: boundary }),
		];
		const slot = resolve_outlet_slot(entries, 0, undefined);
		expect(slot.kind).toBe("error");
		if (slot.kind === "error") {
			expect(slot.error).toBe("boom");
			expect(slot.boundary({ error: "boom" })).toBe("handled:boom");
		}
	});

	it("falls back to default error boundary when route has none", () => {
		const default_boundary = (props: { error: unknown }) =>
			`default:${props.error as any}`;
		const entries = [make_error_entry("/", "boom")];
		const slot = resolve_outlet_slot(entries, 0, default_boundary);
		expect(slot.kind).toBe("error");
		if (slot.kind === "error") {
			expect(slot.boundary({ error: "boom" })).toBe("default:boom");
		}
	});

	it("falls back to built-in error boundary when no boundaries provided", () => {
		const entries = [make_error_entry("/", "boom")];
		const slot = resolve_outlet_slot(entries, 0, undefined);
		expect(slot.kind).toBe("error");
		if (slot.kind === "error") {
			expect(slot.boundary({ error: "boom" })).toBe("Error: boom");
		}
	});

	it("built-in boundary handles Error instances", () => {
		const entries = [make_error_entry("/", new Error("test message"))];
		const slot = resolve_outlet_slot(entries, 0, undefined);
		if (slot.kind === "error") {
			expect(slot.boundary({ error: new Error("test message") })).toBe(
				"Error: test message",
			);
		}
	});

	it("built-in boundary handles non-string non-Error values", () => {
		const entries = [make_error_entry("/", 42)];
		const slot = resolve_outlet_slot(entries, 0, undefined);
		if (slot.kind === "error") {
			expect(slot.boundary({ error: 42 })).toBe(
				"An unexpected error occurred.",
			);
		}
	});

	it("outermost error causes all indices at or beyond to return error slot", () => {
		const boundary = (props: { error: unknown }) =>
			`handled:${props.error as any}`;
		const entries = [
			make_component_entry("/parent", () => "parent"),
			make_error_entry("/child", "child-boom", {
				error_boundary: boundary,
			}),
			make_component_entry("/grandchild", () => "grandchild"),
		];

		const slot0 = resolve_outlet_slot(entries, 0, undefined);
		expect(slot0.kind).toBe("component");

		const slot1 = resolve_outlet_slot(entries, 1, undefined);
		expect(slot1.kind).toBe("error");
		if (slot1.kind === "error") {
			expect(slot1.error).toBe("child-boom");
		}

		const slot2 = resolve_outlet_slot(entries, 2, undefined);
		expect(slot2.kind).toBe("error");
		if (slot2.kind === "error") {
			expect(slot2.error).toBe("child-boom");
		}
	});

	it("error at root causes all indices to return error slot", () => {
		const entries = [
			make_error_entry("/root", "root-boom"),
			make_component_entry("/child", () => "child"),
		];

		const slot0 = resolve_outlet_slot(entries, 0, undefined);
		expect(slot0.kind).toBe("error");

		const slot1 = resolve_outlet_slot(entries, 1, undefined);
		expect(slot1.kind).toBe("error");
	});

	it("returns stable wrapper identity for same pattern across calls", () => {
		const comp_a = () => "version-a";
		const comp_b = () => "version-b";

		const entries_a = [make_component_entry("/stable-test", comp_a)];
		const slot_a = resolve_outlet_slot(entries_a, 0, undefined);

		const entries_b = [make_component_entry("/stable-test", comp_b)];
		const slot_b = resolve_outlet_slot(entries_b, 0, undefined);

		expect(slot_a.kind).toBe("component");
		expect(slot_b.kind).toBe("component");
		if (slot_a.kind === "component" && slot_b.kind === "component") {
			expect(slot_a.component).toBe(slot_b.component);
			expect(slot_b.component({})).toBe("version-b");
		}
	});

	it("returns stable error boundary wrapper identity for same pattern", () => {
		const boundary_a = () => "boundary-a";
		const boundary_b = () => "boundary-b";

		const entries_a = [
			make_error_entry("/stable-error-test", "err", {
				error_boundary: boundary_a,
			}),
		];
		const slot_a = resolve_outlet_slot(entries_a, 0, undefined);

		const entries_b = [
			make_error_entry("/stable-error-test", "err", {
				error_boundary: boundary_b,
			}),
		];
		const slot_b = resolve_outlet_slot(entries_b, 0, undefined);

		expect(slot_a.kind).toBe("error");
		expect(slot_b.kind).toBe("error");
		if (slot_a.kind === "error" && slot_b.kind === "error") {
			expect(slot_a.boundary).toBe(slot_b.boundary);
			expect(slot_b.boundary({ error: "err" })).toBe("boundary-b");
		}
	});
});

describe("get_entry_key", () => {
	it("returns pattern::module_url", () => {
		const entry = make_entry({
			pattern: "/users/:id",
			module_url: "/users.js",
		});
		expect(get_entry_key(entry)).toBe("/users/:id::/users.js");
	});
});
