import { Effect, Ref } from "effect";
import type { RevalidationResult } from "../types.ts";

export const BOOT_REVALIDATION_OK: RevalidationResult = { ok: true };

export type BootRevalidationDecision = {
	requested: boolean;
	skip_work_indicator: boolean;
};

export type BootRevalidationGate = {
	cancel_boot: Effect.Effect<void>;
	finish_boot: Effect.Effect<BootRevalidationDecision>;
	request_or_defer: <E, R>(
		request: Effect.Effect<RevalidationResult, E, R>,
		options?: { skipWorkIndicator?: boolean },
	) => Effect.Effect<RevalidationResult, E, R>;
	start_boot: Effect.Effect<void>;
};

type BootRevalidationGateState = {
	booting: boolean;
	requested: boolean;
	skip_work_indicator: boolean;
};

export function make_boot_revalidation_gate(): Effect.Effect<
	BootRevalidationGate,
	never
> {
	return Effect.gen(function* () {
		const state_ref = yield* Ref.make<BootRevalidationGateState>({
			booting: false,
			requested: false,
			skip_work_indicator: true,
		});

		const start_boot = Ref.set(state_ref, {
			booting: true,
			requested: false,
			skip_work_indicator: true,
		});
		const cancel_boot = Ref.set(state_ref, {
			booting: false,
			requested: false,
			skip_work_indicator: true,
		});
		const finish_boot = Ref.modify(state_ref, (state) => {
			return [
				{
					requested: state.booting && state.requested,
					skip_work_indicator:
						state.booting &&
						state.requested &&
						state.skip_work_indicator,
				},
				{
					booting: false,
					requested: false,
					skip_work_indicator: true,
				},
			];
		});
		const request_or_defer: BootRevalidationGate["request_or_defer"] = (
			request,
			options,
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
							skip_work_indicator:
								state.skip_work_indicator &&
								options?.skipWorkIndicator === true,
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
