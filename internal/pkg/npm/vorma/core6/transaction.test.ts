import { describe, expect, it } from "vitest";
import {
	core6_scope_cancelled_reason,
	core6_scope_stale_reason,
} from "./scope.ts";
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

describe("core6 route transactions", () => {
	it("runs stages with the transaction intent and signal", async () => {
		const manager = create_core6_route_transaction_manager<
			typeof core6_route_transaction_kind.navigation,
			RouteIntent
		>();
		const transaction = manager.start({
			kind: core6_route_transaction_kind.navigation,
			intent: make_intent("/next"),
		});

		const result = await transaction.stage(async ({ intent, signal }) => {
			return {
				aborted: signal.aborted,
				href: intent.href,
			};
		});

		expect(result).toEqual({
			ok: true,
			value: {
				aborted: false,
				href: "/next",
			},
		});
	});

	it("prevents superseded transaction stages from publishing", async () => {
		const manager = create_core6_route_transaction_manager<
			typeof core6_route_transaction_kind.navigation,
			RouteIntent
		>();
		const first = manager.start({
			kind: core6_route_transaction_kind.navigation,
			intent: make_intent("/first"),
		});
		let resolve_stage!: (value: string) => void;
		const stage = first.stage(async () => {
			return await new Promise<string>((resolve) => {
				resolve_stage = resolve;
			});
		});

		const second = manager.start({
			kind: core6_route_transaction_kind.navigation,
			intent: make_intent("/second"),
		});
		resolve_stage("late");

		await expect(stage).resolves.toEqual({
			ok: false,
			reason: core6_scope_stale_reason,
		});
		expect(first.signal.aborted).toBe(true);
		expect(second.live()).toBe(true);
		expect(manager.current()).toBe(second);
	});

	it("makes completion the final publish boundary", () => {
		const manager = create_core6_route_transaction_manager<
			typeof core6_route_transaction_kind.navigation,
			RouteIntent
		>();
		const transaction = manager.start({
			kind: core6_route_transaction_kind.navigation,
			intent: make_intent("/done"),
		});
		const commits: string[] = [];

		const result = transaction.complete((intent) => {
			commits.push(intent.href);
			return true;
		});

		expect(result).toEqual({ ok: true, value: true });
		expect(commits).toEqual(["/done"]);
		expect(transaction.live()).toBe(false);
		expect(transaction.signal.aborted).toBe(false);
		expect(manager.current()).toBeNull();
		expect(manager.is_current(transaction)).toBe(false);
	});

	it("releases ownership when completion throws", () => {
		const manager = create_core6_route_transaction_manager<
			typeof core6_route_transaction_kind.navigation,
			RouteIntent
		>();
		const transaction = manager.start({
			kind: core6_route_transaction_kind.navigation,
			intent: make_intent("/done"),
		});
		const error = new Error("publish failed");

		expect(() => {
			transaction.complete(() => {
				throw error;
			});
		}).toThrow(error);

		expect(transaction.live()).toBe(false);
		expect(transaction.signal.aborted).toBe(false);
		expect(manager.current()).toBeNull();
		expect(manager.is_current(transaction)).toBe(false);
	});

	it("refuses commits from stale transactions", () => {
		const manager = create_core6_route_transaction_manager<
			typeof core6_route_transaction_kind.navigation,
			RouteIntent
		>();
		const first = manager.start({
			kind: core6_route_transaction_kind.navigation,
			intent: make_intent("/first"),
		});
		manager.start({
			kind: core6_route_transaction_kind.navigation,
			intent: make_intent("/second"),
		});

		const result = first.commit((intent) => {
			return intent.href;
		});

		expect(result).toEqual({
			ok: false,
			reason: core6_scope_stale_reason,
		});
	});

	it("cancels the current transaction through the manager", () => {
		const manager = create_core6_route_transaction_manager<
			typeof core6_route_transaction_kind.navigation,
			RouteIntent
		>();
		const transaction = manager.start({
			kind: core6_route_transaction_kind.navigation,
			intent: make_intent("/next"),
		});

		expect(manager.cancel_current()).toBe(true);
		expect(transaction.signal.aborted).toBe(true);
		expect(transaction.live()).toBe(false);
		expect(manager.current()).toBeNull();
		expect(manager.cancel_current()).toBe(false);
	});

	it("cancels the current transaction through the transaction", () => {
		const manager = create_core6_route_transaction_manager<
			typeof core6_route_transaction_kind.navigation,
			RouteIntent
		>();
		const transaction = manager.start({
			kind: core6_route_transaction_kind.navigation,
			intent: make_intent("/next"),
		});

		expect(transaction.cancel()).toBe(true);
		expect(manager.current()).toBeNull();
		expect(manager.is_current(transaction)).toBe(false);
	});

	it("maps rejected stages after cancellation to cancelled results", async () => {
		const manager = create_core6_route_transaction_manager<
			typeof core6_route_transaction_kind.navigation,
			RouteIntent
		>();
		const transaction = manager.start({
			kind: core6_route_transaction_kind.navigation,
			intent: make_intent("/next"),
		});
		const error = new Error("aborted");
		let reject_stage!: (reason: unknown) => void;
		const stage = transaction.stage(async () => {
			return await new Promise<string>((_resolve, reject) => {
				reject_stage = reject;
			});
		});

		transaction.cancel();
		reject_stage(error);

		await expect(stage).resolves.toEqual({
			ok: false,
			reason: core6_scope_cancelled_reason,
		});
	});
});
