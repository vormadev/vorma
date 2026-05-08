// @vitest-environment jsdom

import { Effect } from "effect";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
	WINDOW_EVENT_FOCUS,
	make_runtime_lifecycle,
} from "./effect_runtime/runtime_lifecycle.ts";

function run_effect<A, E>(program: Effect.Effect<A, E, never>): Promise<A> {
	return Effect.runPromise(program);
}

describe("ccc Effect runtime lifecycle experiment", () => {
	beforeEach(() => {
		vi.restoreAllMocks();
	});

	it("runs finalizers in last-in-first-out order once", async () => {
		const events: string[] = [];

		await run_effect(
			Effect.gen(function* () {
				const lifecycle = yield* make_runtime_lifecycle();
				yield* lifecycle.add_finalizer(
					Effect.sync(() => {
						events.push("first");
					}),
				);
				yield* lifecycle.add_finalizer(
					Effect.sync(() => {
						events.push("second");
					}),
				);
				yield* lifecycle.shutdown;
				yield* lifecycle.shutdown;
			}),
		);

		expect(events).toEqual(["second", "first"]);
	});

	it("removes window listeners during shutdown", async () => {
		const events: string[] = [];

		await run_effect(
			Effect.gen(function* () {
				const lifecycle = yield* make_runtime_lifecycle();
				yield* lifecycle.listen_window(WINDOW_EVENT_FOCUS, () => {
					events.push("focus");
				});
				window.dispatchEvent(new Event(WINDOW_EVENT_FOCUS));
				yield* lifecycle.shutdown;
				window.dispatchEvent(new Event(WINDOW_EVENT_FOCUS));
			}),
		);

		expect(events).toEqual(["focus"]);
	});

	it("runs finalizers immediately after shutdown", async () => {
		const events: string[] = [];

		await run_effect(
			Effect.gen(function* () {
				const lifecycle = yield* make_runtime_lifecycle();
				yield* lifecycle.shutdown;
				yield* lifecycle.add_finalizer(
					Effect.sync(() => {
						events.push("late");
					}),
				);
			}),
		);

		expect(events).toEqual(["late"]);
	});
});
