import type { GetRouteDataOutput } from "../app/context.ts";

export type RenderCommitCommand =
	| {
			type: "apply_route_data_to_global_state";
			reason: "render_commit_apply_route_data_to_global_state";
	  }
	| {
			type: "derive_and_set_error_state";
			reason: "render_commit_derive_and_set_error_state";
	  }
	| {
			type: "set_active_components_from_modules";
			reason: "render_commit_set_active_components_from_modules";
	  }
	| {
			type: "set_active_error_boundary_from_modules";
			reason: "render_commit_set_active_error_boundary_from_modules";
	  }
	| {
			type: "run_history_and_capture_scroll_state";
			reason: "render_commit_run_history_and_capture_scroll_state";
	  }
	| {
			type: "apply_route_document_title";
			reason: "render_commit_apply_route_document_title";
	  }
	| {
			type: "apply_css_bundles";
			cssBundles: string[];
			reason: "render_commit_apply_css_bundles";
	  }
	| {
			type: "dispatch_route_change_event";
			reason: "render_commit_dispatch_route_change_event";
	  }
	| {
			type: "apply_route_head_elements";
			reason: "render_commit_apply_route_head_elements";
	  }
	| {
			type: "finish";
			reason: "render_commit_finish";
	  };

export function buildRenderCommitCommands(props: {
	json: GetRouteDataOutput;
}): RenderCommitCommand[] {
	const commands: RenderCommitCommand[] = [
		{
			type: "apply_route_data_to_global_state",
			reason: "render_commit_apply_route_data_to_global_state",
		},
		{
			type: "derive_and_set_error_state",
			reason: "render_commit_derive_and_set_error_state",
		},
		{
			type: "set_active_components_from_modules",
			reason: "render_commit_set_active_components_from_modules",
		},
		{
			type: "set_active_error_boundary_from_modules",
			reason: "render_commit_set_active_error_boundary_from_modules",
		},
		{
			type: "run_history_and_capture_scroll_state",
			reason: "render_commit_run_history_and_capture_scroll_state",
		},
		{
			type: "apply_route_document_title",
			reason: "render_commit_apply_route_document_title",
		},
	];

	if (props.json.cssBundles) {
		commands.push({
			type: "apply_css_bundles",
			cssBundles: props.json.cssBundles,
			reason: "render_commit_apply_css_bundles",
		});
	}

	commands.push(
		{
			type: "dispatch_route_change_event",
			reason: "render_commit_dispatch_route_change_event",
		},
		{
			type: "apply_route_head_elements",
			reason: "render_commit_apply_route_head_elements",
		},
		{
			type: "finish",
			reason: "render_commit_finish",
		},
	);

	return commands;
}
