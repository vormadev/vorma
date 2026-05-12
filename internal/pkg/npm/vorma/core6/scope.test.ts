import { describe, expect, it } from "vitest";
import {
	core6_scope_cancelled_reason,
	core6_scope_stale_reason,
	create_core6_scope_manager,
	run_core6_scope_stage,
} from "./scope.ts";

describe("core6 ownership scopes", () => {
	it("allows the current scope to commit", () => {
		const manager = create_core6_scope_manager<"route">();
		const scope = manager.start("route");

		const result = scope.commit(() => {
			return "committed";
		});

		expect(result).toEqual({ ok: true, value: "committed" });
		expect(scope.live()).toBe(true);
		expect(manager.current()).toBe(scope);
	});

	it("prevents a superseded scope from committing", () => {
		const manager = create_core6_scope_manager<"route">();
		const first = manager.start("route");
		const cleanup_calls: string[] = [];
		first.on_cleanup(() => {
			cleanup_calls.push("first");
		});

		const second = manager.start("route");
		const result = first.commit(() => {
			return "stale";
		});

		expect(first.live()).toBe(false);
		expect(first.signal.aborted).toBe(true);
		expect(second.live()).toBe(true);
		expect(result).toEqual({
			ok: false,
			reason: core6_scope_stale_reason,
		});
		expect(cleanup_calls).toEqual(["first"]);
	});

	it("cancels only the current scope", () => {
		const manager = create_core6_scope_manager<"route">();
		const first = manager.start("route");
		const second = manager.start("route");

		expect(first.cancel()).toBe(false);
		expect(second.cancel()).toBe(true);
		expect(manager.current()).toBeNull();
		expect(second.signal.aborted).toBe(true);
		expect(second.commit(() => "late")).toEqual({
			ok: false,
			reason: core6_scope_cancelled_reason,
		});
	});

	it("runs cleanup at most once", () => {
		const manager = create_core6_scope_manager<"route">();
		const scope = manager.start("route");
		let calls = 0;
		scope.on_cleanup(() => {
			calls++;
		});

		expect(scope.cancel()).toBe(true);
		expect(scope.cancel()).toBe(false);
		expect(calls).toBe(1);
	});

	it("runs every cleanup even when one cleanup throws", () => {
		const manager = create_core6_scope_manager<"route">();
		const scope = manager.start("route");
		const error = new Error("cleanup failed");
		const calls: string[] = [];
		scope.on_cleanup(() => {
			calls.push("first");
			throw error;
		});
		scope.on_cleanup(() => {
			calls.push("second");
		});

		expect(() => {
			scope.cancel();
		}).toThrow(error);

		expect(calls).toEqual(["first", "second"]);
		expect(scope.live()).toBe(false);
		expect(manager.current()).toBeNull();
	});

	it("runs cleanup even when a cancel listener throws", () => {
		const manager = create_core6_scope_manager<"route">();
		const scope = manager.start("route");
		const error = new Error("cancel listener failed");
		let cleaned = false;
		scope.on_cancel(() => {
			throw error;
		});
		scope.on_cleanup(() => {
			cleaned = true;
		});

		expect(() => {
			scope.cancel();
		}).toThrow(error);

		expect(cleaned).toBe(true);
		expect(scope.live()).toBe(false);
		expect(manager.current()).toBeNull();
	});

	it("completes without aborting the signal", () => {
		const manager = create_core6_scope_manager<"route">();
		const scope = manager.start("route");
		let cleaned = false;
		scope.on_cleanup(() => {
			cleaned = true;
		});

		const result = scope.complete(() => {
			return "done";
		});

		expect(result).toEqual({ ok: true, value: "done" });
		expect(scope.signal.aborted).toBe(false);
		expect(scope.live()).toBe(false);
		expect(cleaned).toBe(true);
		expect(manager.current()).toBeNull();
	});

	it("cleans up and releases ownership when completion throws", () => {
		const manager = create_core6_scope_manager<"route">();
		const scope = manager.start("route");
		const error = new Error("publish failed");
		let cleaned = false;
		scope.on_cleanup(() => {
			cleaned = true;
		});

		expect(() => {
			scope.complete(() => {
				throw error;
			});
		}).toThrow(error);

		expect(cleaned).toBe(true);
		expect(scope.live()).toBe(false);
		expect(scope.signal.aborted).toBe(false);
		expect(manager.current()).toBeNull();
		expect(scope.commit(() => "late")).toEqual({
			ok: false,
			reason: core6_scope_stale_reason,
		});
	});

	it("runs late cleanup immediately for stale scopes", () => {
		const manager = create_core6_scope_manager<"route">();
		const first = manager.start("route");
		manager.start("route");
		let cleaned = false;

		first.on_cleanup(() => {
			cleaned = true;
		});

		expect(cleaned).toBe(true);
	});

	it("runs cancel listeners only on cancellation", () => {
		const manager = create_core6_scope_manager<"route">();
		const cancelled = manager.start("route");
		let cancel_calls = 0;
		cancelled.on_cancel(() => {
			cancel_calls++;
		});

		manager.start("route");
		expect(cancel_calls).toBe(1);

		const completed = manager.start("route");
		completed.on_cancel(() => {
			cancel_calls++;
		});
		completed.complete(() => {
			return undefined;
		});
		expect(cancel_calls).toBe(1);
	});

	it("guards async stage results with the current scope", async () => {
		const manager = create_core6_scope_manager<"route">();
		const first = manager.start("route");
		let resolve_stage!: (value: string) => void;
		const stage = run_core6_scope_stage(first, async () => {
			return await new Promise<string>((resolve) => {
				resolve_stage = resolve;
			});
		});

		manager.start("route");
		resolve_stage("late");

		await expect(stage).resolves.toEqual({
			ok: false,
			reason: core6_scope_stale_reason,
		});
	});

	it("maps rejected stale async stages to stale results", async () => {
		const manager = create_core6_scope_manager<"route">();
		const first = manager.start("route");
		const error = new Error("aborted by newer owner");
		let reject_stage!: (reason: unknown) => void;
		const stage = run_core6_scope_stage(first, async () => {
			return await new Promise<string>((_resolve, reject) => {
				reject_stage = reject;
			});
		});

		manager.start("route");
		reject_stage(error);

		await expect(stage).resolves.toEqual({
			ok: false,
			reason: core6_scope_stale_reason,
		});
	});

	it("maps rejected cancelled async stages to cancelled results", async () => {
		const manager = create_core6_scope_manager<"route">();
		const scope = manager.start("route");
		const error = new Error("aborted by cancellation");
		let reject_stage!: (reason: unknown) => void;
		const stage = run_core6_scope_stage(scope, async () => {
			return await new Promise<string>((_resolve, reject) => {
				reject_stage = reject;
			});
		});

		scope.cancel();
		reject_stage(error);

		await expect(stage).resolves.toEqual({
			ok: false,
			reason: core6_scope_cancelled_reason,
		});
	});

	it("lets current async stage errors propagate", async () => {
		const manager = create_core6_scope_manager<"route">();
		const scope = manager.start("route");
		const error = new Error("network failed");

		await expect(
			run_core6_scope_stage(scope, async () => {
				throw error;
			}),
		).rejects.toBe(error);
		expect(scope.live()).toBe(true);
		expect(manager.current()).toBe(scope);
	});

	it("passes the scope abort signal into async stages", async () => {
		const manager = create_core6_scope_manager<"route">();
		const scope = manager.start("route");

		const result = await run_core6_scope_stage(scope, async (signal) => {
			return signal.aborted;
		});

		expect(result).toEqual({ ok: true, value: false });
	});
});
