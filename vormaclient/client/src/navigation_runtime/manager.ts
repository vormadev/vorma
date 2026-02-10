import {
	createNavigationRuntime,
	type CreateNavigationRuntimeOptions,
} from "./runtime.ts";
import type { NavigationStateManager } from "./types.ts";

export type CreateNavigationStateManagerOptions =
	CreateNavigationRuntimeOptions;

export function createNavigationStateManager(
	options: CreateNavigationStateManagerOptions = {},
): NavigationStateManager {
	return createNavigationRuntime(options);
}
