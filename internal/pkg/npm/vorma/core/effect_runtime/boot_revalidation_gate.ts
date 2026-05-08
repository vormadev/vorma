import { Effect, Ref } from "effect";
import type { RevalidationResult } from "../types.ts";

export const BOOT_REVALIDATION_OK: RevalidationResult = { ok: true };

export type BootRevalidationGate = {
	cancel_boot: Effect.Effect<void>;
	finish_boot: Effect.Effect<boolean>;
	request_or_defer: <E, R>(
		request: Effect.Effect<RevalidationResult, E, R>,
	) => Effect.Effect<RevalidationResult, E, R>;
	start_boot: Effect.Effect<void>;
};

type BootRevalidationGateState = {
	booting: boolean;
	requested: boolean;
};

export function make_boot_revalidation_gate(): Effect.Effect<
	BootRevalidationGate,
	never
> {
	return Effect.gen(function* () {
		const state_ref = yield* Ref.make<BootRevalidationGateState>({
			booting: false,
			requested: false,
		});

		const start_boot = Ref.set(state_ref, {
			booting: true,
			requested: false,
		});
		const cancel_boot = Ref.set(state_ref, {
			booting: false,
			requested: false,
		});
		const finish_boot = Ref.modify(state_ref, (state) => {
			return [
				state.booting && state.requested,
				{
					booting: false,
					requested: false,
				},
			];
		});
		const request_or_defer: BootRevalidationGate["request_or_defer"] = (
			request,
		) => {
			return Effect.gen(function* () {
				const deferred = yield* Ref.modify(state_ref, (state) => {
					if (!state.booting) {
						return [false, state];
					}
					return [
						true,
						{
							booting: true,
							requested: true,
						},
					];
				});
				if (deferred) {
					return BOOT_REVALIDATION_OK;
				}
				return yield* request;
			});
		};

		return {
			cancel_boot,
			finish_boot,
			request_or_defer,
			start_boot,
		};
	});
}
