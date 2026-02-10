import { navigationStateManager } from "./client.ts";
import type { NavigationOutcome } from "./navigation_runtime/types.ts";
import { effectuateRedirectDataResult } from "./redirects/redirects.ts";

type LinkLifecycleCallbacks<E extends Event> = {
	beforeRender?: (event: E) => void | Promise<void>;
	afterRender?: (event: E) => void | Promise<void>;
};

export type HandleLinkNavigationOutcomeInput<E extends Event> = {
	event: E;
	outcome: NavigationOutcome;
	targetUrl: string;
	callbacks: LinkLifecycleCallbacks<E>;
};

export async function handleLinkNavigationOutcome<E extends Event>(
	input: HandleLinkNavigationOutcomeInput<E>,
): Promise<void> {
	const { event, outcome, targetUrl, callbacks } = input;

	switch (outcome.type) {
		case "aborted":
			// Navigation was aborted - clean up to prevent stuck loading indicator
			navigationStateManager.removeNavigation(targetUrl);
			return;

		case "redirect":
			// Call beforeRender while entry still exists (consistent with success case)
			await callbacks.beforeRender?.(event);

			// Clean up before redirect to prevent race conditions
			navigationStateManager.removeNavigation(targetUrl);

			// Effectuate the redirect
			await effectuateRedirectDataResult(
				outcome.redirectData,
				outcome.props.redirectCount || 0,
				outcome.props,
			);

			// Call afterRender after redirect effectuation
			await callbacks.afterRender?.(event);
			return;

		case "success": {
			// Call beforeRender before processing (matches original behavior)
			await callbacks.beforeRender?.(event);

			// Process the successful navigation if entry still exists
			const entry = navigationStateManager.getNavigation(targetUrl);
			if (entry) {
				await navigationStateManager.processSuccessfulNavigation(
					outcome,
					entry,
				);
			}

			// Call afterRender after processing (matches original behavior)
			await callbacks.afterRender?.(event);
			return;
		}

		default: {
			// Exhaustiveness check
			const _exhaustive: never = outcome;
			throw new Error(
				`Unexpected outcome type: ${(_exhaustive as any).type}`,
			);
		}
	}
}
