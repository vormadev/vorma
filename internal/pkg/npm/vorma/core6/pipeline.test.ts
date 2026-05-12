import { describe, expect, it } from "vitest";
import { run_core6_route_pipeline } from "./pipeline.ts";
import { core6_scope_stale_reason } from "./scope.ts";
import {
	core6_route_transaction_kind,
	create_core6_route_transaction_manager,
} from "./transaction.ts";

type RouteIntent = {
	href: string;
	replace: boolean;
};

function make_intent(href: string): RouteIntent {
	return {
		href,
		replace: false,
	};
}

describe("core6 route pipeline", () => {
	it("runs fetch, prepare, and publish through one transaction", async () => {
		const manager = create_core6_route_transaction_manager<
			typeof core6_route_transaction_kind.navigation,
			RouteIntent
		>();
		const transaction = manager.start({
			kind: core6_route_transaction_kind.navigation,
			intent: make_intent("/next"),
		});
		const calls: string[] = [];

		const result = await run_core6_route_pipeline(transaction, {
			fetch: async ({ intent, signal }) => {
				calls.push(`fetch:${intent.href}:${signal.aborted}`);
				return { body: "payload" };
			},
			prepare: async ({ fetched, intent, signal }) => {
				calls.push(
					`prepare:${intent.href}:${fetched.body}:${signal.aborted}`,
				);
				return { route: fetched.body.toUpperCase() };
			},
			publish: ({ intent, prepared }) => {
				calls.push(`publish:${intent.href}:${prepared.route}`);
				return prepared.route;
			},
		});

		expect(result).toEqual({ ok: true, value: "PAYLOAD" });
		expect(calls).toEqual([
			"fetch:/next:false",
			"prepare:/next:payload:false",
			"publish:/next:PAYLOAD",
		]);
		expect(transaction.live()).toBe(false);
		expect(manager.current()).toBeNull();
	});

	it("does not prepare or publish stale fetch results", async () => {
		const manager = create_core6_route_transaction_manager<
			typeof core6_route_transaction_kind.navigation,
			RouteIntent
		>();
		const transaction = manager.start({
			kind: core6_route_transaction_kind.navigation,
			intent: make_intent("/first"),
		});
		const calls: string[] = [];
		let resolve_fetch!: (value: string) => void;

		const result = run_core6_route_pipeline(transaction, {
			fetch: async () => {
				calls.push("fetch");
				return await new Promise<string>((resolve) => {
					resolve_fetch = resolve;
				});
			},
			prepare: async () => {
				calls.push("prepare");
				return "prepared";
			},
			publish: () => {
				calls.push("publish");
				return "published";
			},
		});

		manager.start({
			kind: core6_route_transaction_kind.navigation,
			intent: make_intent("/second"),
		});
		resolve_fetch("late");

		await expect(result).resolves.toEqual({
			ok: false,
			reason: core6_scope_stale_reason,
		});
		expect(calls).toEqual(["fetch"]);
	});

	it("does not publish stale preparation results", async () => {
		const manager = create_core6_route_transaction_manager<
			typeof core6_route_transaction_kind.navigation,
			RouteIntent
		>();
		const transaction = manager.start({
			kind: core6_route_transaction_kind.navigation,
			intent: make_intent("/first"),
		});
		const calls: string[] = [];
		let resolve_prepare!: (value: string) => void;
		let mark_prepare_started!: () => void;
		const prepare_started = new Promise<void>((resolve) => {
			mark_prepare_started = resolve;
		});

		const result = run_core6_route_pipeline(transaction, {
			fetch: async () => {
				calls.push("fetch");
				return "payload";
			},
			prepare: async () => {
				calls.push("prepare");
				mark_prepare_started();
				return await new Promise<string>((resolve) => {
					resolve_prepare = resolve;
				});
			},
			publish: () => {
				calls.push("publish");
				return "published";
			},
		});

		await prepare_started;
		manager.start({
			kind: core6_route_transaction_kind.navigation,
			intent: make_intent("/second"),
		});
		resolve_prepare("late");

		await expect(result).resolves.toEqual({
			ok: false,
			reason: core6_scope_stale_reason,
		});
		expect(calls).toEqual(["fetch", "prepare"]);
	});

	it("cancels ownership when fetch rejects while current", async () => {
		const manager = create_core6_route_transaction_manager<
			typeof core6_route_transaction_kind.navigation,
			RouteIntent
		>();
		const transaction = manager.start({
			kind: core6_route_transaction_kind.navigation,
			intent: make_intent("/next"),
		});
		const error = new Error("fetch failed");

		await expect(
			run_core6_route_pipeline(transaction, {
				fetch: async () => {
					throw error;
				},
				prepare: async () => {
					return "prepared";
				},
				publish: () => {
					return "published";
				},
			}),
		).rejects.toBe(error);
		expect(transaction.live()).toBe(false);
		expect(transaction.signal.aborted).toBe(true);
		expect(manager.current()).toBeNull();
	});

	it("cancels ownership when prepare rejects while current", async () => {
		const manager = create_core6_route_transaction_manager<
			typeof core6_route_transaction_kind.navigation,
			RouteIntent
		>();
		const transaction = manager.start({
			kind: core6_route_transaction_kind.navigation,
			intent: make_intent("/next"),
		});
		const error = new Error("prepare failed");

		await expect(
			run_core6_route_pipeline(transaction, {
				fetch: async () => {
					return "payload";
				},
				prepare: async () => {
					throw error;
				},
				publish: () => {
					return "published";
				},
			}),
		).rejects.toBe(error);
		expect(transaction.live()).toBe(false);
		expect(transaction.signal.aborted).toBe(true);
		expect(manager.current()).toBeNull();
	});

	it("releases ownership when publish throws", async () => {
		const manager = create_core6_route_transaction_manager<
			typeof core6_route_transaction_kind.navigation,
			RouteIntent
		>();
		const transaction = manager.start({
			kind: core6_route_transaction_kind.navigation,
			intent: make_intent("/next"),
		});
		const error = new Error("publish failed");

		await expect(
			run_core6_route_pipeline(transaction, {
				fetch: async () => {
					return "payload";
				},
				prepare: async () => {
					return "prepared";
				},
				publish: () => {
					throw error;
				},
			}),
		).rejects.toBe(error);
		expect(transaction.live()).toBe(false);
		expect(transaction.signal.aborted).toBe(false);
		expect(manager.current()).toBeNull();
	});
});
