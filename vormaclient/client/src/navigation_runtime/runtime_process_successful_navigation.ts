import {
	processSuccessfulNavigation as executeProcessSuccessfulNavigation,
	type ProcessSuccessfulNavigationContext,
} from "./process_successful_navigation.ts";
import type { NavigationEntry, NavigationOutcome } from "./types.ts";

export type RuntimeProcessSuccessfulNavigation = (
	outcome: Extract<NavigationOutcome, { type: "success" }>,
	entry: NavigationEntry,
) => Promise<void>;

export function createRuntimeProcessSuccessfulNavigation(
	context: ProcessSuccessfulNavigationContext,
): RuntimeProcessSuccessfulNavigation {
	return async function processSuccessfulNavigation(
		outcome: Extract<NavigationOutcome, { type: "success" }>,
		entry: NavigationEntry,
	): Promise<void> {
		return executeProcessSuccessfulNavigation(context, outcome, entry);
	};
}
