import { Effect, Ref } from "effect";
import type { EffectClientKernelHandle } from "./client_kernel_assembly.ts";

export type ClientSession = {
	active_handle: Effect.Effect<EffectClientKernelHandle | null>;
	replace_active: (handle: EffectClientKernelHandle) => Effect.Effect<void>;
	shutdown_active: Effect.Effect<void>;
	shutdown_if_active: (
		handle: EffectClientKernelHandle,
	) => Effect.Effect<void>;
};

export function make_client_session(): Effect.Effect<ClientSession, never> {
	return Effect.gen(function* () {
		const active_ref = yield* Ref.make<EffectClientKernelHandle | null>(
			null,
		);
		const shutdown_handle = (
			handle: EffectClientKernelHandle | null,
		): Effect.Effect<void> => {
			if (!handle) {
				return Effect.void;
			}
			return handle.shutdown;
		};
		const shutdown_active = Ref.modify(active_ref, (active) => {
			return [active, null];
		}).pipe(Effect.flatMap(shutdown_handle));
		const shutdown_if_active: ClientSession["shutdown_if_active"] = (
			handle,
		) => {
			return Ref.modify(active_ref, (active) => {
				if (active !== handle) {
					return [null, active];
				}
				return [active, null];
			}).pipe(Effect.flatMap(shutdown_handle));
		};
		const replace_active: ClientSession["replace_active"] = (handle) => {
			return Effect.gen(function* () {
				const previous = yield* Ref.get(active_ref);
				if (previous === handle) {
					return;
				}
				yield* shutdown_handle(previous);
				yield* Ref.set(active_ref, handle);
			});
		};

		return {
			active_handle: Ref.get(active_ref),
			replace_active,
			shutdown_active,
			shutdown_if_active,
		};
	});
}
