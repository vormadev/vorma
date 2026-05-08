import { Effect } from "effect";
import { describe, expect, it } from "vitest";
import type { EffectClientKernelHandle } from "./effect_runtime/client_kernel_assembly.ts";
import { make_client_session } from "./effect_runtime/client_session.ts";

function run_effect<A, E>(program: Effect.Effect<A, E, never>): Promise<A> {
	return Effect.runPromise(program);
}

function kernel_handle(
	name: string,
	events: string[],
): EffectClientKernelHandle {
	return {
		kernel: null as never,
		shutdown: Effect.sync(() => {
			events.push(name);
		}),
	};
}

describe("ccc Effect client session experiment", () => {
	it("replaces and shuts down the active kernel handle", async () => {
		const events: string[] = [];
		const session = Effect.runSync(make_client_session());
		const first = kernel_handle("first", events);
		const second = kernel_handle("second", events);

		await run_effect(session.replace_active(first));
		expect(await run_effect(session.active_handle)).toBe(first);

		await run_effect(session.replace_active(second));
		expect(events).toEqual(["first"]);
		expect(await run_effect(session.active_handle)).toBe(second);

		await run_effect(session.shutdown_active);
		expect(events).toEqual(["first", "second"]);
		expect(await run_effect(session.active_handle)).toBeNull();
	});

	it("only shuts down a matching active handle", async () => {
		const events: string[] = [];
		const session = Effect.runSync(make_client_session());
		const stale = kernel_handle("stale", events);
		const active = kernel_handle("active", events);

		await run_effect(session.replace_active(active));
		await run_effect(session.shutdown_if_active(stale));
		expect(events).toEqual([]);
		expect(await run_effect(session.active_handle)).toBe(active);

		await run_effect(session.shutdown_if_active(active));
		expect(events).toEqual(["active"]);
		expect(await run_effect(session.active_handle)).toBeNull();
	});
});
