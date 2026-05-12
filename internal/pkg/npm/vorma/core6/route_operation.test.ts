import { describe, expect, it } from "vitest";
import { run_core6_route_operation } from "./route_operation.ts";
import { core6_scope_stale_reason } from "./scope.ts";
import {
	core6_route_transaction_kind,
	create_core6_route_transaction_manager,
} from "./transaction.ts";

type RouteIntent = {
	href: string;
	replace: boolean;
};

type FetchedRoute = {
	body: string;
};

type PreparedRoute = {
	route: string;
};

function make_intent(href: string): RouteIntent {
	return {
		href,
		replace: false,
	};
}

describe("core6 route operation", () => {
	it("starts a transaction and runs fetch, prepare, and publish through the route pipeline", async () => {
		const transaction_manager = create_core6_route_transaction_manager<
			typeof core6_route_transaction_kind.navigation,
			RouteIntent
		>();
		const calls: string[] = [];

		const result = await run_core6_route_operation({
			host: {
				fetch_route: async ({ intent, signal }) => {
					calls.push(`fetch:${intent.href}:${signal.aborted}`);
					return { body: "payload" };
				},
				prepare_route: async ({ fetched, intent, signal }) => {
					calls.push(
						`prepare:${intent.href}:${fetched.body}:${signal.aborted}`,
					);
					return { route: fetched.body.toUpperCase() };
				},
				publish_route: ({ intent, prepared }) => {
					calls.push(`publish:${intent.href}:${prepared.route}`);
					return prepared.route;
				},
			},
			intent: make_intent("/next"),
			kind: core6_route_transaction_kind.navigation,
			transaction_manager,
		});

		expect(result).toEqual({ ok: true, value: "PAYLOAD" });
		expect(calls).toEqual([
			"fetch:/next:false",
			"prepare:/next:payload:false",
			"publish:/next:PAYLOAD",
		]);
		expect(transaction_manager.current()).toBeNull();
	});

	it("does not prepare or publish stale fetch results", async () => {
		const transaction_manager = create_core6_route_transaction_manager<
			typeof core6_route_transaction_kind.navigation,
			RouteIntent
		>();
		const calls: string[] = [];
		let resolve_fetch!: (route: FetchedRoute) => void;

		const result = run_core6_route_operation({
			host: {
				fetch_route: async () => {
					calls.push("fetch");
					return await new Promise<FetchedRoute>((resolve) => {
						resolve_fetch = resolve;
					});
				},
				prepare_route: async () => {
					calls.push("prepare");
					return { route: "prepared" };
				},
				publish_route: () => {
					calls.push("publish");
					return "published";
				},
			},
			intent: make_intent("/first"),
			kind: core6_route_transaction_kind.navigation,
			transaction_manager,
		});

		transaction_manager.start({
			intent: make_intent("/second"),
			kind: core6_route_transaction_kind.navigation,
		});
		resolve_fetch({ body: "late" });

		await expect(result).resolves.toEqual({
			ok: false,
			reason: core6_scope_stale_reason,
		});
		expect(calls).toEqual(["fetch"]);
	});

	it("does not publish stale preparation results", async () => {
		const transaction_manager = create_core6_route_transaction_manager<
			typeof core6_route_transaction_kind.navigation,
			RouteIntent
		>();
		const calls: string[] = [];
		let resolve_prepare!: (route: PreparedRoute) => void;
		let mark_prepare_started!: () => void;
		const prepare_started = new Promise<void>((resolve) => {
			mark_prepare_started = resolve;
		});

		const result = run_core6_route_operation({
			host: {
				fetch_route: async () => {
					calls.push("fetch");
					return { body: "payload" };
				},
				prepare_route: async () => {
					calls.push("prepare");
					mark_prepare_started();
					return await new Promise<PreparedRoute>((resolve) => {
						resolve_prepare = resolve;
					});
				},
				publish_route: () => {
					calls.push("publish");
					return "published";
				},
			},
			intent: make_intent("/first"),
			kind: core6_route_transaction_kind.navigation,
			transaction_manager,
		});

		await prepare_started;
		transaction_manager.start({
			intent: make_intent("/second"),
			kind: core6_route_transaction_kind.navigation,
		});
		resolve_prepare({ route: "late" });

		await expect(result).resolves.toEqual({
			ok: false,
			reason: core6_scope_stale_reason,
		});
		expect(calls).toEqual(["fetch", "prepare"]);
	});

	it("uses publish as the only visible commit boundary", async () => {
		const transaction_manager = create_core6_route_transaction_manager<
			typeof core6_route_transaction_kind.navigation,
			RouteIntent
		>();
		let visible_route = "initial";

		const result = await run_core6_route_operation({
			host: {
				fetch_route: async () => {
					expect(visible_route).toBe("initial");
					return { body: "payload" };
				},
				prepare_route: async () => {
					expect(visible_route).toBe("initial");
					return { route: "prepared" };
				},
				publish_route: ({ prepared }) => {
					expect(visible_route).toBe("initial");
					visible_route = prepared.route;
					return visible_route;
				},
			},
			intent: make_intent("/next"),
			kind: core6_route_transaction_kind.navigation,
			transaction_manager,
		});

		expect(result).toEqual({ ok: true, value: "prepared" });
		expect(visible_route).toBe("prepared");
		expect(transaction_manager.current()).toBeNull();
	});

	it("cancels ownership when fetch rejects while current", async () => {
		const transaction_manager = create_core6_route_transaction_manager<
			typeof core6_route_transaction_kind.navigation,
			RouteIntent
		>();
		const error = new Error("fetch failed");
		let fetch_signal!: AbortSignal;

		await expect(
			run_core6_route_operation({
				host: {
					fetch_route: async ({ signal }) => {
						fetch_signal = signal;
						throw error;
					},
					prepare_route: async () => {
						return { route: "prepared" };
					},
					publish_route: () => {
						return "published";
					},
				},
				intent: make_intent("/next"),
				kind: core6_route_transaction_kind.navigation,
				transaction_manager,
			}),
		).rejects.toBe(error);

		expect(fetch_signal.aborted).toBe(true);
		expect(transaction_manager.current()).toBeNull();
	});

	it("cancels ownership when prepare rejects while current", async () => {
		const transaction_manager = create_core6_route_transaction_manager<
			typeof core6_route_transaction_kind.navigation,
			RouteIntent
		>();
		const error = new Error("prepare failed");
		let prepare_signal!: AbortSignal;

		await expect(
			run_core6_route_operation({
				host: {
					fetch_route: async () => {
						return { body: "payload" };
					},
					prepare_route: async ({ signal }) => {
						prepare_signal = signal;
						throw error;
					},
					publish_route: () => {
						return "published";
					},
				},
				intent: make_intent("/next"),
				kind: core6_route_transaction_kind.navigation,
				transaction_manager,
			}),
		).rejects.toBe(error);

		expect(prepare_signal.aborted).toBe(true);
		expect(transaction_manager.current()).toBeNull();
	});

	it("releases ownership without aborting when publish throws", async () => {
		const transaction_manager = create_core6_route_transaction_manager<
			typeof core6_route_transaction_kind.navigation,
			RouteIntent
		>();
		const error = new Error("publish failed");
		let fetch_signal!: AbortSignal;

		await expect(
			run_core6_route_operation({
				host: {
					fetch_route: async ({ signal }) => {
						fetch_signal = signal;
						return { body: "payload" };
					},
					prepare_route: async () => {
						return { route: "prepared" };
					},
					publish_route: () => {
						throw error;
					},
				},
				intent: make_intent("/next"),
				kind: core6_route_transaction_kind.navigation,
				transaction_manager,
			}),
		).rejects.toBe(error);

		expect(fetch_signal.aborted).toBe(false);
		expect(transaction_manager.current()).toBeNull();
	});
});
