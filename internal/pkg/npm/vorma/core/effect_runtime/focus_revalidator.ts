import { Effect, Ref } from "effect";
import type { RevalidationResult } from "../types.ts";
import type { RevalidationReason, WorkState } from "./client_contract.ts";

export type FocusRevalidator = {
	focus: Effect.Effect<void>;
	mark_activity: Effect.Effect<void>;
};

export type FocusRevalidatorOptions = {
	stale_ms: number;
	get_work_state: Effect.Effect<WorkState>;
	request_revalidation: (
		reason: Exclude<RevalidationReason, "retry">,
		options?: { debounce?: boolean },
	) => Effect.Effect<RevalidationResult>;
	now?: () => number;
};

export function make_focus_revalidator(
	options: FocusRevalidatorOptions,
): Effect.Effect<FocusRevalidator, never> {
	return Effect.gen(function* () {
		const now =
			options.now ??
			((): number => {
				return Date.now();
			});
		const last_activity = yield* Ref.make(now());

		const mark_activity = Ref.set(last_activity, now());
		const focus = Effect.gen(function* () {
			const work = yield* options.get_work_state;
			if (
				work.navigation ||
				work.revalidation ||
				work.apiRequests.length > 0
			) {
				return;
			}
			const last = yield* Ref.get(last_activity);
			if (now() - last < options.stale_ms) {
				return;
			}
			yield* options.request_revalidation("windowFocus", {
				debounce: true,
			});
		}).pipe(
			Effect.catchAll(() => {
				return Effect.void;
			}),
		);

		return { focus, mark_activity };
	});
}
