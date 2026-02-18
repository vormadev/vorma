import { describe, expect, it } from "vitest";
import { buildRenderCommitCommands } from "../../core/render_commit_runtime.ts";

function createRouteDataJSON(
	overrides: Record<string, unknown> = {},
): Record<string, unknown> {
	return {
		matchedPatterns: [],
		loadersData: [],
		importURLs: [],
		exportKeys: [],
		errorExportKeys: [],
		hasRootData: false,
		params: {},
		splatValues: [],
		deps: [],
		cssBundles: [],
		outermostServerError: undefined,
		outermostServerErrorIdx: undefined,
		title: undefined,
		metaHeadEls: undefined,
		restHeadEls: undefined,
		...overrides,
	};
}

describe("render runtime commit command builder", () => {
	it("builds commit command sequence with css apply command when cssBundles are defined", () => {
		const commands = buildRenderCommitCommands({
			json: createRouteDataJSON({
				cssBundles: ["/a.css", "/b.css"],
			}) as any,
		});

		expect(commands).toEqual([
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
			{
				type: "apply_css_bundles",
				cssBundles: ["/a.css", "/b.css"],
				reason: "render_commit_apply_css_bundles",
			},
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
		]);
	});

	it("omits css apply command when cssBundles are undefined", () => {
		const commands = buildRenderCommitCommands({
			json: createRouteDataJSON({
				cssBundles: undefined,
			}) as any,
		});

		expect(commands).toEqual([
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
		]);
	});
});
