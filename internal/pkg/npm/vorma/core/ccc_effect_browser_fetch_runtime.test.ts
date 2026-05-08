import { Effect } from "effect";
import { describe, expect, it } from "vitest";
import {
	BrowserFetchAborted,
	BrowserFetchFailed,
	make_browser_fetch_runtime,
} from "./effect_runtime/browser_fetch_runtime.ts";

const fetch_url = "https://app.example.test/data";

function run_effect<A, E>(program: Effect.Effect<A, E, never>): Promise<A> {
	return Effect.runPromise(program);
}

describe("ccc Effect browser fetch runtime experiment", () => {
	it("merges Effect cancellation with request abort signals", async () => {
		let seen_signal: AbortSignal | undefined;
		const request_controller = new AbortController();
		const runtime = Effect.runSync(
			make_browser_fetch_runtime({
				fetch: async (_url, init) => {
					seen_signal = init.signal ?? undefined;
					request_controller.abort();
					return new Response("ok");
				},
			}),
		);

		await run_effect(
			runtime.fetch({
				url: new URL(fetch_url),
				init: {
					signal: request_controller.signal,
				},
			}),
		);

		expect(seen_signal?.aborted).toBe(true);
	});

	it("classifies abort and non-abort transport failures", async () => {
		const abort_error = { kind: "abort" };
		const abort_runtime = Effect.runSync(
			make_browser_fetch_runtime({
				fetch: async () => {
					throw abort_error;
				},
				is_abort_error: (error) => {
					return error === abort_error;
				},
			}),
		);
		const failed_runtime = Effect.runSync(
			make_browser_fetch_runtime({
				fetch: async () => {
					throw new Error("network broke");
				},
			}),
		);

		const abort_result = await run_effect(
			Effect.either(abort_runtime.fetch({ url: new URL(fetch_url) })),
		);
		const failed_result = await run_effect(
			Effect.either(failed_runtime.fetch({ url: new URL(fetch_url) })),
		);

		expect(abort_result._tag).toBe("Left");
		if (abort_result._tag === "Left") {
			expect(abort_result.left).toBeInstanceOf(BrowserFetchAborted);
			expect(abort_result.left.error).toBe(abort_error);
		}
		expect(failed_result._tag).toBe("Left");
		if (failed_result._tag === "Left") {
			expect(failed_result.left).toBeInstanceOf(BrowserFetchFailed);
			expect(failed_result.left.error).toBeInstanceOf(Error);
		}
	});
});
