import type { CoreHost, PublicCallRegistry } from "./host.ts";
import type { IDSource } from "./ids.ts";
import { create_command_interpreter } from "./interpreter.ts";
import type { ClientEvent, ClientState } from "./model.ts";
import { initial_client_state, update } from "./update.ts";

export type CoreController = {
	dispatch: (event: ClientEvent) => void;
	get_state: () => ClientState;
};

export function create_core_controller(
	host: CoreHost,
	public_calls: PublicCallRegistry,
	id_source: IDSource,
	initial_state: ClientState = initial_client_state(),
): CoreController {
	let state = initial_state;

	const interpreter = create_command_interpreter(
		host,
		(event) => {
			dispatch(event);
		},
		id_source,
		public_calls,
	);

	function dispatch(event: ClientEvent): void {
		const result = update(state, event);
		state = result.state;
		interpreter.execute(result.commands);
	}

	function get_state(): ClientState {
		return state;
	}

	return { dispatch, get_state };
}
