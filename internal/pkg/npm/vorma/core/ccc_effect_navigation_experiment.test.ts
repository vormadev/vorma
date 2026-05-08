import { Deferred, Effect, Fiber, TestContext } from "effect";
import { describe, expect, it } from "vitest";
import {
	make_navigation_actor,
	NavigationRedirect,
	type LoadedRoute,
	type NavigationAttempt,
} from "./effect_runtime/navigation_actor.ts";

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

describe("ccc Effect navigation experiment", () => {
	it("shares identical in-flight navigations through one active fiber", async () => {
		const attempts: NavigationAttempt[] = [];
		const published: string[] = [];

		const result = await run_effect(
			Effect.scoped(
				Effect.gen(function* () {
					const release = yield* Deferred.make<void>();
					const actor = yield* make_navigation_actor({
						load: (attempt) => {
							return Effect.gen(function* () {
								yield* Effect.sync(() => {
									attempts.push(attempt);
								});
								yield* Deferred.await(release);
								return {
									href: attempt.href,
									value: attempt.id,
								};
							});
						},
						publish: (loaded) => {
							return Effect.sync(() => {
								published.push(loaded.href);
							});
						},
					});

					const first = yield* Effect.fork(actor.navigate("/same"));
					yield* drain();
					const second = yield* Effect.fork(actor.navigate("/same"));
					yield* drain();
					const snapshot = yield* actor.snapshot;

					yield* Deferred.succeed(release, undefined);
					const first_result = yield* Fiber.join(first);
					const second_result = yield* Fiber.join(second);

					return { first_result, second_result, snapshot };
				}),
			),
		);

		expect(attempts).toHaveLength(1);
		expect(result.snapshot.active?.waiterCount).toBe(2);
		expect(result.first_result).toEqual({
			didNavigate: true,
			href: "/same",
			redirectCount: 0,
		});
		expect(result.second_result).toEqual(result.first_result);
		expect(published).toEqual(["/same"]);
	});

	it("interrupts superseded navigation and resolves its waiters false", async () => {
		const attempts: NavigationAttempt[] = [];
		let interrupted = 0;

		const result = await run_effect(
			Effect.scoped(
				Effect.gen(function* () {
					const actor = yield* make_navigation_actor({
						load: (attempt) => {
							return Effect.sync(() => {
								attempts.push(attempt);
							}).pipe(
								Effect.andThen(
									attempt.href === "/first"
										? Effect.never
										: Effect.succeed({
												href: attempt.href,
												value: attempt.id,
											}),
								),
								Effect.onInterrupt(() => {
									return Effect.sync(() => {
										interrupted++;
									});
								}),
							);
						},
						publish: () => {
							return Effect.void;
						},
					});

					const first = yield* Effect.fork(actor.navigate("/first"));
					yield* drain();
					const second = yield* Effect.fork(
						actor.navigate("/second"),
					);
					yield* drain();

					const first_result = yield* Fiber.join(first);
					const second_result = yield* Fiber.join(second);

					return { first_result, second_result };
				}),
			),
		);

		expect(attempts.map((attempt) => attempt.href)).toEqual([
			"/first",
			"/second",
		]);
		expect(interrupted).toBe(1);
		expect(result.first_result).toEqual({
			didNavigate: false,
			href: null,
			redirectCount: 0,
		});
		expect(result.second_result).toEqual({
			didNavigate: true,
			href: "/second",
			redirectCount: 0,
		});
	});

	it("transfers waiters through redirect chains", async () => {
		const attempts: NavigationAttempt[] = [];
		const published: LoadedRoute[] = [];

		const result = await run_effect(
			Effect.scoped(
				Effect.gen(function* () {
					const actor = yield* make_navigation_actor({
						load: (attempt) => {
							return Effect.gen(function* () {
								yield* Effect.sync(() => {
									attempts.push(attempt);
								});
								if (attempt.href === "/from") {
									return yield* Effect.fail(
										new NavigationRedirect({
											href: "/to",
										}),
									);
								}
								return {
									href: attempt.href,
									value: attempt.id,
								};
							});
						},
						publish: (loaded) => {
							return Effect.sync(() => {
								published.push(loaded);
							});
						},
					});

					return yield* actor.navigate("/from");
				}),
			),
		);

		expect(attempts.map((attempt) => attempt.href)).toEqual([
			"/from",
			"/to",
		]);
		expect(attempts.map((attempt) => attempt.source)).toEqual([
			"navigate",
			"redirect",
		]);
		expect(published.map((loaded) => loaded.href)).toEqual(["/to"]);
		expect(result).toEqual({
			didNavigate: true,
			href: "/to",
			redirectCount: 1,
		});
	});

	it("ignores stale publish commands from superseded fibers", async () => {
		const published: string[] = [];

		const result = await run_effect(
			Effect.scoped(
				Effect.gen(function* () {
					const release_first = yield* Deferred.make<void>();
					const actor = yield* make_navigation_actor({
						load: (attempt) => {
							if (attempt.href === "/first") {
								return Deferred.await(release_first).pipe(
									Effect.as({
										href: attempt.href,
										value: attempt.id,
									}),
									Effect.uninterruptible,
								);
							}
							return Effect.succeed({
								href: attempt.href,
								value: attempt.id,
							});
						},
						publish: (loaded) => {
							return Effect.sync(() => {
								published.push(loaded.href);
							});
						},
					});

					const first = yield* Effect.fork(actor.navigate("/first"));
					yield* drain();
					const second = yield* Effect.fork(
						actor.navigate("/second"),
					);
					yield* drain();

					const first_result = yield* Fiber.join(first);
					const second_result = yield* Fiber.join(second);

					yield* Deferred.succeed(release_first, undefined);
					yield* drain();
					const snapshot = yield* actor.snapshot;

					return { first_result, second_result, snapshot };
				}),
			),
		);

		expect(result.first_result).toEqual({
			didNavigate: false,
			href: null,
			redirectCount: 0,
		});
		expect(result.second_result).toEqual({
			didNavigate: true,
			href: "/second",
			redirectCount: 0,
		});
		expect(published).toEqual(["/second"]);
		expect(result.snapshot.active).toBeNull();
	});
});
