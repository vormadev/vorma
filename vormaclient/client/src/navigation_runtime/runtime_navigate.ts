import { navigateWithHandlers, type NavigateHandlers } from "./navigate.ts";
import type { NavigateProps } from "./types.ts";

export type RuntimeNavigate = (
	props: NavigateProps,
) => Promise<{ didNavigate: boolean }>;

export function createRuntimeNavigate(
	handlers: NavigateHandlers,
): RuntimeNavigate {
	return async function navigate(
		props: NavigateProps,
	): Promise<{ didNavigate: boolean }> {
		return navigateWithHandlers(handlers, props);
	};
}
