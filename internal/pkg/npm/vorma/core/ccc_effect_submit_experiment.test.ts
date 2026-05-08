import { Effect, Fiber, TestContext } from "effect";
import { describe, expect, it } from "vitest";
import { X_CLIENT_REDIRECT } from "./constants.ts";
import {
	SubmitAborted,
	type SubmitDispatch,
	make_submit_manager,
} from "./effect_runtime/submit_manager.ts";

function run_effect<A>(program: Effect.Effect<A, never, never>): Promise<A> {
	return Effect.runPromise(
		program.pipe(Effect.provide(TestContext.TestContext)),
	);
}

function drain(): Effect.Effect<void> {
	return Effect.gen(function* () {
		for (let i = 0; i < 10; i++) {
			yield* Effect.yieldNow();
		}
	});
}

function json_response(data: unknown): Response {
	return new Response(JSON.stringify(data), {
		status: 200,
		headers: { "Content-Type": "application/json" },
	});
}

describe("ccc Effect submit experiment", () => {
	it("dedupes keyed submissions by interrupting the old request fiber", async () => {
		const dispatches: SubmitDispatch[] = [];
		let interrupted = 0;

		const result = await run_effect(
			Effect.scoped(
				Effect.gen(function* () {
					const manager = yield* make_submit_manager({
						dispatch: (request) => {
							return Effect.sync(() => {
								dispatches.push(request);
							}).pipe(
								Effect.andThen(
									request.url.pathname === "/slow"
										? Effect.never
										: Effect.succeed(
												json_response({ ok: true }),
											),
								),
								Effect.onInterrupt(() => {
									return Effect.sync(() => {
										interrupted++;
									});
								}),
							);
						},
					});

					const first = yield* Effect.fork(
						manager.submit({
							href: "/slow",
							init: { method: "POST" },
							options: { dedupeKey: "save", revalidate: false },
						}),
					);
					yield* drain();
					const second = yield* Effect.fork(
						manager.submit({
							href: "/fast",
							init: { method: "POST" },
							options: { dedupeKey: "save", revalidate: false },
						}),
					);
					yield* drain();

					return {
						first: yield* Fiber.join(first),
						second: yield* Fiber.join(second),
					};
				}),
			),
		);

		expect(dispatches.map((dispatch) => dispatch.url.pathname)).toEqual([
			"/slow",
			"/fast",
		]);
		expect(interrupted).toBe(1);
		expect(result.first).toMatchObject({
			success: false,
			error: "Aborted",
		});
		expect(result.second).toMatchObject({
			success: true,
			data: { ok: true },
		});
	});

	it("tracks concurrent unkeyed submissions independently", async () => {
		const result = await run_effect(
			Effect.scoped(
				Effect.gen(function* () {
					const manager = yield* make_submit_manager({
						dispatch: () => {
							return Effect.never;
						},
					});

					const first = yield* Effect.fork(
						manager.submit({
							href: "/a",
							init: { method: "POST" },
							options: { revalidate: false },
						}),
					);
					const second = yield* Effect.fork(
						manager.submit({
							href: "/b",
							init: { method: "GET" },
						}),
					);
					yield* drain();
					const snapshot = yield* manager.snapshot;
					yield* manager.shutdown;

					return {
						snapshot,
						first: yield* Fiber.join(first),
						second: yield* Fiber.join(second),
					};
				}),
			),
		);

		expect(result.snapshot.active.map((item) => item.href)).toEqual([
			"http://localhost/a",
			"http://localhost/b",
		]);
		expect(result.snapshot.active.map((item) => item.key)).toEqual([
			"submit:1",
			"submit:2",
		]);
		expect(result.first).toMatchObject({
			success: false,
			error: "Aborted",
		});
		expect(result.second).toMatchObject({
			success: false,
			error: "Aborted",
		});
	});

	it("parses JSON and returns a revalidation Effect for mutations", async () => {
		let revalidation_count = 0;

		const result = await run_effect(
			Effect.scoped(
				Effect.gen(function* () {
					const manager = yield* make_submit_manager({
						dispatch: () => {
							return Effect.succeed(json_response({ ok: true }));
						},
						revalidate: () => {
							return Effect.sync(() => {
								revalidation_count++;
								return { ok: true as const };
							});
						},
					});

					const fiber = yield* Effect.fork(
						manager.submit<{ ok: boolean }>({
							href: "/save",
							init: { method: "POST" },
						}),
					);
					yield* drain();
					const submit_result = yield* Fiber.join(fiber);
					const revalidation = yield* submit_result.revalidation;

					return { submit_result, revalidation };
				}),
			),
		);

		expect(result.submit_result).toMatchObject({
			success: true,
			data: { ok: true },
		});
		expect(result.revalidation).toEqual({ ok: true });
		expect(revalidation_count).toBe(1);
	});

	it("hands same-origin redirects to the injected redirect effect", async () => {
		const redirects: Array<{ href: string; kind: "client" | "hard" }> = [];

		const result = await run_effect(
			Effect.scoped(
				Effect.gen(function* () {
					const manager = yield* make_submit_manager({
						dispatch: () => {
							return Effect.succeed(
								new Response("", {
									status: 200,
									headers: {
										[X_CLIENT_REDIRECT]: "/target",
									},
								}),
							);
						},
						redirect: (href, kind) => {
							return Effect.sync(() => {
								redirects.push({ href, kind });
							});
						},
					});

					const fiber = yield* Effect.fork(
						manager.submit({
							href: "/action",
							init: { method: "POST" },
						}),
					);
					yield* drain();
					return yield* Fiber.join(fiber);
				}),
			),
		);

		expect(result).toMatchObject({
			success: true,
			data: undefined,
		});
		expect(redirects).toEqual([
			{ href: "http://localhost/target", kind: "client" },
		]);
	});

	it("normalizes dispatch abort errors into submit failures", async () => {
		const result = await run_effect(
			Effect.scoped(
				Effect.gen(function* () {
					const manager = yield* make_submit_manager({
						dispatch: () => {
							return Effect.fail(new SubmitAborted());
						},
					});

					const fiber = yield* Effect.fork(
						manager.submit({
							href: "/action",
							init: { method: "POST" },
						}),
					);
					yield* drain();
					return yield* Fiber.join(fiber);
				}),
			),
		);

		expect(result).toMatchObject({
			success: false,
			error: "Aborted",
		});
	});
});
