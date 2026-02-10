import { REVALIDATION_COALESCE_MS } from "./constants.ts";
import { createRuntimeBeginContextSetup } from "./runtime_begin_context_setup.ts";
import { createNavigationRuntimeAPI } from "./runtime_api.ts";
import { createRuntimeBookkeepingStatusSetup } from "./runtime_bookkeeping_status_setup.ts";
import type { NavigationStateManager, SubmissionEntry } from "./types.ts";

export type CreateNavigationRuntimeOptions = {
	onNavigationIntentResolved?: () => void;
};

export function createNavigationRuntime(
	options: CreateNavigationRuntimeOptions = {},
): NavigationStateManager {
	const { onNavigationIntentResolved } = options;

	// Submissions tracked separately
	const submissions = new Map<string | symbol, SubmissionEntry>();
	const { navigationBookkeeping, getStatus, scheduleStatusUpdate } =
		createRuntimeBookkeepingStatusSetup(submissions);

	const { beginNavigationContext, createActiveNavigation } =
		createRuntimeBeginContextSetup({
			navigationBookkeeping,
			scheduleStatusUpdate,
			revalidationCoalesceMS: REVALIDATION_COALESCE_MS,
		});

	return createNavigationRuntimeAPI({
		submissions,
		getStatus,
		scheduleStatusUpdate,
		navigationBookkeeping,
		beginNavigationContext,
		createActiveNavigation,
		onNavigationIntentResolved,
	});
}
