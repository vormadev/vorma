import { Effect, Ref } from "effect";
import type { ClientOptions } from "./client_contract.ts";
import type { EffectClientKernel } from "./client_kernel.ts";
import type { FocusRevalidator } from "./focus_revalidator.ts";

export type DefaultErrorBoundary =
	| ((props: { error: unknown }) => any)
	| undefined;

export type ClientCoreState = {
	active_kernel: Effect.Effect<EffectClientKernel | null>;
	client_options: Effect.Effect<ClientOptions>;
	default_error_boundary: Effect.Effect<DefaultErrorBoundary>;
	focus_revalidator: Effect.Effect<FocusRevalidator | null>;
	clear_focus_revalidator: Effect.Effect<void>;
	clear_kernel_if_current: (
		current: EffectClientKernel,
	) => Effect.Effect<void>;
	restore_kernel_if_current: (
		current: EffectClientKernel,
		previous: EffectClientKernel | null,
	) => Effect.Effect<void>;
	set_active_kernel: (
		kernel: EffectClientKernel | null,
	) => Effect.Effect<void>;
	set_boot_options: (options: ClientOptions) => Effect.Effect<void>;
	set_focus_revalidator: (
		focus_revalidator: FocusRevalidator | null,
	) => Effect.Effect<void>;
};

export function make_client_core_state(): Effect.Effect<
	ClientCoreState,
	never
> {
	return Effect.gen(function* () {
		const active_kernel_ref = yield* Ref.make<EffectClientKernel | null>(
			null,
		);
		const client_options_ref = yield* Ref.make<ClientOptions>({});
		const default_error_boundary_ref =
			yield* Ref.make<DefaultErrorBoundary>(undefined);
		const focus_revalidator_ref = yield* Ref.make<FocusRevalidator | null>(
			null,
		);

		return {
			active_kernel: Ref.get(active_kernel_ref),
			client_options: Ref.get(client_options_ref),
			default_error_boundary: Ref.get(default_error_boundary_ref),
			focus_revalidator: Ref.get(focus_revalidator_ref),
			clear_focus_revalidator: Ref.set(focus_revalidator_ref, null),
			clear_kernel_if_current: (current) => {
				return Ref.update(active_kernel_ref, (active) => {
					if (active === current) {
						return null;
					}
					return active;
				});
			},
			restore_kernel_if_current: (current, previous) => {
				return Ref.update(active_kernel_ref, (active) => {
					if (active === current) {
						return previous;
					}
					return active;
				});
			},
			set_active_kernel: (kernel) => {
				return Ref.set(active_kernel_ref, kernel);
			},
			set_boot_options: (options) => {
				return Effect.gen(function* () {
					yield* Ref.set(client_options_ref, options);
					yield* Ref.set(
						default_error_boundary_ref,
						options.defaultErrorBoundary,
					);
				});
			},
			set_focus_revalidator: (focus_revalidator) => {
				return Ref.set(focus_revalidator_ref, focus_revalidator);
			},
		};
	});
}
