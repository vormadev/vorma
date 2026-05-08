import { Effect } from "effect";
import { describe, expect, it } from "vitest";
import { make_submit_dispatcher } from "./effect_runtime/submit_dispatcher.ts";
import type { SubmitDispatch } from "./effect_runtime/submit_manager.ts";
import { SubmitAborted } from "./effect_runtime/submit_manager.ts";

const submit_url = "https://app.example.test/api/action";

function run_effect<A, E>(program: Effect.Effect<A, E, never>): Promise<A> {
	return Effect.runPromise(program);
}

function submit_dispatch(init: RequestInit = {}): SubmitDispatch {
	return {
		id: 1,
		key: "submit:1",
		url: new URL(submit_url),
		method: "POST",
		apiRouteKind: "mutation",
		init,
	};
}

describe("ccc Effect submit dispatcher experiment", () => {
	it("dispatches with a merged abort signal", async () => {
		let seen_signal: AbortSignal | undefined;
		const request_controller = new AbortController();
		const dispatcher = Effect.runSync(
			make_submit_dispatcher({
				fetch: async (_url, init) => {
					seen_signal = init.signal ?? undefined;
					request_controller.abort();
					return new Response("ok");
				},
			}),
		);

		const result = await run_effect(
			dispatcher.dispatch(
				submit_dispatch({ signal: request_controller.signal }),
			),
		);

		expect(result.ok).toBe(true);
		expect(seen_signal?.aborted).toBe(true);
	});

	it("maps DOM abort failures into SubmitAborted", async () => {
		const dispatcher = Effect.runSync(
			make_submit_dispatcher({
				fetch: async () => {
					throw new DOMException("Aborted", "AbortError");
				},
			}),
		);

		const result = await run_effect(
			Effect.either(dispatcher.dispatch(submit_dispatch())),
		);

		expect(result._tag).toBe("Left");
		if (result._tag === "Left") {
			expect(result.left).toBeInstanceOf(SubmitAborted);
		}
	});
});
