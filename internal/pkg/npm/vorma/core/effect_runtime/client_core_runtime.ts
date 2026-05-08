import { Effect, Layer, Ref } from "effect";
import {
	type BootRevalidationGate,
	make_boot_revalidation_gate,
} from "./boot_revalidation_gate.ts";
import {
	type BootRouteState,
	make_boot_route_state,
} from "./boot_route_state.ts";
import {
	type ClientCoreState,
	make_client_core_state,
} from "./client_core_state.ts";
import type { EffectClientKernelHandle } from "./client_kernel_assembly.ts";
import {
	type ClientRuntimeServices,
	type ClientRuntimeServicesLayerContext,
	type ClientRuntimeServicesOptions,
	client_runtime_services_to_layer,
	make_client_runtime_services,
} from "./client_runtime_services.ts";
import { type ClientSession, make_client_session } from "./client_session.ts";

export type ClientCoreRuntime = {
	boot_revalidation_gate: BootRevalidationGate;
	boot_route_state: BootRouteState;
	client_core_state: ClientCoreState;
	client_session: ClientSession;
	runtime_services: ClientRuntimeServices;
	runtime_services_layer: Layer.Layer<
		ClientRuntimeServicesLayerContext,
		never
	>;
};

const active_client_handle_ref =
	Ref.makeUnsafe<EffectClientKernelHandle | null>(null);

export function make_client_core_runtime(
	options: ClientRuntimeServicesOptions = {},
): Effect.Effect<ClientCoreRuntime, never> {
	return Effect.gen(function* () {
		const runtime_services = yield* make_client_runtime_services(options);
		const boot_revalidation_gate = yield* make_boot_revalidation_gate();
		const boot_route_state = yield* make_boot_route_state();
		const client_core_state = yield* make_client_core_state();
		const client_session = yield* make_client_session({
			active_ref: active_client_handle_ref,
		});
		return {
			boot_revalidation_gate,
			boot_route_state,
			client_core_state,
			client_session,
			runtime_services,
			runtime_services_layer:
				client_runtime_services_to_layer(runtime_services),
		};
	});
}
