// @vitest-environment jsdom

import { Effect } from "effect";
import { describe, expect, it } from "vitest";
import { install_window_focus_revalidator } from "./effect_runtime/focus_revalidator.ts";
import {
	WINDOW_EVENT_FOCUS,
	make_runtime_lifecycle,
} from "./effect_runtime/runtime_lifecycle.ts";

function run_effect<A, E>(program: Effect.Effect<A, E, never>): Promise<A> {
	return Effect.runPromise(program);
}

describe("ccc Effect focus revalidator experiment", () => {
	it("owns the browser focus listener through lifecycle shutdown", async () => {
		let revalidations = 0;
		const lifecycle = Effect.runSync(make_runtime_lifecycle());
		await run_effect(
			install_window_focus_revalidator({
				lifecycle,
				stale_ms: 0,
				get_work_state: Effect.succeed({
					navigation: null,
					revalidation: null,
					prefetch: null,
					apiRequests: [],
				}),
				request_revalidation: () => {
					return Effect.sync(() => {
						revalidations += 1;
						return { ok: true };
					});
				},
				now: () => {
					return 0;
				},
			}),
		);

		window.dispatchEvent(new Event(WINDOW_EVENT_FOCUS));
		await Promise.resolve();
		expect(revalidations).toBe(1);

		await run_effect(lifecycle.shutdown);
		window.dispatchEvent(new Event(WINDOW_EVENT_FOCUS));
		await Promise.resolve();
		expect(revalidations).toBe(1);
	});
});
