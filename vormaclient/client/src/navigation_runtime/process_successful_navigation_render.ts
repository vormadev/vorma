import { __reRenderApp } from "../rendering.ts";
import { isAbortError } from "../utils/errors.ts";
import { logError } from "../utils/logging.ts";
import type {
	NavigationEntry,
	NavigationOutcome,
	NavigationPhase,
} from "./types.ts";

export type ProcessSuccessfulNavigationRenderContext = {
	transitionPhase: (targetUrl: string, phase: NavigationPhase) => void;
};

function buildRunHistoryOptions(
	entry: NavigationEntry,
	props: Extract<NavigationOutcome, { type: "success" }>["props"],
) {
	if (entry.intent !== "navigate") {
		return undefined;
	}

	return {
		href: entry.targetUrl,
		scrollStateToRestore: props.scrollStateToRestore,
		replace: entry.replace || props.replace,
		scrollToTop: entry.scrollToTop,
		state: entry.state,
	};
}

export async function renderSuccessfulNavigation(
	context: ProcessSuccessfulNavigationRenderContext,
	outcome: Extract<NavigationOutcome, { type: "success" }>,
	entry: NavigationEntry,
): Promise<void> {
	context.transitionPhase(entry.targetUrl, "rendering");

	try {
		await __reRenderApp({
			json: outcome.json,
			navigationType: entry.type,
			runHistoryOptions: buildRunHistoryOptions(entry, outcome.props),
			onFinish: () => {
				context.transitionPhase(entry.targetUrl, "complete");
			},
		});
	} catch (error) {
		context.transitionPhase(entry.targetUrl, "complete");
		if (!isAbortError(error)) {
			logError("Error completing navigation", error);
		}
		throw error;
	}
}
