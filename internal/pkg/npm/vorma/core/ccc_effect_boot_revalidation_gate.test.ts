import { Effect } from "effect";
import { describe, expect, it } from "vitest";
import {
	BOOT_REVALIDATION_OK,
	make_boot_revalidation_gate,
} from "./effect_runtime/boot_revalidation_gate.ts";
import type { RevalidationResult } from "./types.ts";

const REQUEST_REVALIDATION_OK: RevalidationResult = { ok: true };

function run_effect<A, E>(program: Effect.Effect<A, E, never>): Promise<A> {
	return Effect.runPromise(program);
}

describe("ccc Effect boot revalidation gate experiment", () => {
	it("defers API revalidation during boot and reports it once", async () => {
		let requests = 0;
		const gate = Effect.runSync(make_boot_revalidation_gate());
		const request = Effect.sync(() => {
			requests += 1;
			return REQUEST_REVALIDATION_OK;
		});

		const before_boot = await run_effect(gate.request_or_defer(request));
		await run_effect(gate.start_boot);
		const during_boot = await run_effect(gate.request_or_defer(request));
		const should_revalidate = await run_effect(gate.finish_boot);
		const second_finish = await run_effect(gate.finish_boot);
		const after_boot = await run_effect(gate.request_or_defer(request));

		expect(before_boot).toBe(REQUEST_REVALIDATION_OK);
		expect(during_boot).toBe(BOOT_REVALIDATION_OK);
		expect(should_revalidate).toBe(true);
		expect(second_finish).toBe(false);
		expect(after_boot).toBe(REQUEST_REVALIDATION_OK);
		expect(requests).toBe(2);
	});

	it("drops deferred API revalidation when boot is cancelled", async () => {
		const gate = Effect.runSync(make_boot_revalidation_gate());

		await run_effect(gate.start_boot);
		await run_effect(
			gate.request_or_defer(Effect.succeed(REQUEST_REVALIDATION_OK)),
		);
		await run_effect(gate.cancel_boot);

		expect(await run_effect(gate.finish_boot)).toBe(false);
	});
});
