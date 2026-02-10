import type { StatusEventDetail } from "../events.ts";
import type { NavigationEntry, SubmissionEntry } from "./types.ts";

export type ComputeNavigationStatusInput = {
	activeNavigation: NavigationEntry | null;
	pendingRevalidation: NavigationEntry | null;
	submissions: Map<string | symbol, SubmissionEntry>;
};

export function computeNavigationStatus(
	input: ComputeNavigationStatusInput,
): StatusEventDetail {
	const { activeNavigation, pendingRevalidation, submissions } = input;

	const isNavigating =
		activeNavigation !== null &&
		activeNavigation.intent === "navigate" &&
		activeNavigation.phase !== "complete";

	const isRevalidating =
		pendingRevalidation !== null &&
		pendingRevalidation.phase !== "complete";

	const isSubmitting = Array.from(submissions.values()).some(
		(x) => !x.skipGlobalLoadingIndicator,
	);

	return { isNavigating, isSubmitting, isRevalidating };
}
