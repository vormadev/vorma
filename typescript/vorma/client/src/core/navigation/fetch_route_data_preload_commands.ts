import type { ServerSuccessPreloadExecutionPlan } from "./fetch_route_data_preload_state_machine.ts";

export type ServerSuccessPreloadCommand =
	| {
			type: "preload_module_dependency";
			dependency: string;
			reason: "server_success_preload_allowed";
	  }
	| {
			type: "preload_css_bundle";
			bundle: string;
			reason: "server_success_preload_allowed";
	  };

export function buildServerSuccessPreloadCommands(props: {
	executionPlan: ServerSuccessPreloadExecutionPlan;
}): ServerSuccessPreloadCommand[] {
	if (props.executionPlan.type === "skip") {
		return [];
	}

	const commands: ServerSuccessPreloadCommand[] = [];
	for (const dependency of props.executionPlan.moduleDependenciesToPreload) {
		if (typeof dependency !== "string" || dependency.length === 0) {
			continue;
		}
		commands.push({
			type: "preload_module_dependency",
			dependency,
			reason: props.executionPlan.reason,
		});
	}
	for (const bundle of props.executionPlan.cssBundlesToPreload) {
		if (typeof bundle !== "string" || bundle.length === 0) {
			continue;
		}
		commands.push({
			type: "preload_css_bundle",
			bundle,
			reason: props.executionPlan.reason,
		});
	}

	return commands;
}
