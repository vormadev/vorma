import { isStaleRevalidationEntry } from "./process_successful_navigation_revalidation.ts";
import { renderSuccessfulNavigation } from "./process_successful_navigation_render.ts";
import { waitForSuccessfulNavigationAssets } from "./process_successful_navigation_wait.ts";
import {
	applyResponseArtifactsWhenBuildMatches,
	syncBuildIDFromResponse,
} from "./successful_navigation_effects.ts";
import type {
	NavigationEntry,
	NavigationOutcome,
	NavigationPhase,
} from "./types.ts";

export type ProcessSuccessfulNavigationContext = {
	transitionPhase: (targetUrl: string, phase: NavigationPhase) => void;
	findNavigationEntry: (targetUrl: string) => NavigationEntry | undefined;
	deleteNavigation: (key: string) => boolean;
};

export async function processSuccessfulNavigation(
	context: ProcessSuccessfulNavigationContext,
	outcome: Extract<NavigationOutcome, { type: "success" }>,
	entry: NavigationEntry,
): Promise<void> {
	try {
		const { response, json } = outcome;

		applyResponseArtifactsWhenBuildMatches(response, json);

		// Validate revalidation is still applicable
		if (isStaleRevalidationEntry(entry)) {
			context.deleteNavigation(entry.targetUrl);
			return;
		}

		// Transition to waiting phase
		context.transitionPhase(entry.targetUrl, "waiting");

		// Skip if navigation was aborted
		if (!context.findNavigationEntry(entry.targetUrl)) {
			return;
		}

		syncBuildIDFromResponse(response);

		await waitForSuccessfulNavigationAssets(outcome);

		// Skip rendering for prefetch without intent
		if (entry.intent === "none") {
			context.transitionPhase(entry.targetUrl, "complete");
			return;
		}

		// Skip rendering for revalidation if not on target page
		if (isStaleRevalidationEntry(entry)) {
			return;
		}

		await renderSuccessfulNavigation(context, outcome, entry);
	} finally {
		if (!(entry.type === "prefetch" && entry.intent === "none")) {
			context.deleteNavigation(entry.targetUrl);
		}
	}
}
