import { dispatchBuildIDEvent } from "../events.ts";
import { effectuateRedirectDataResult } from "../redirects/redirects.ts";
import { __vormaClientGlobal } from "../vorma_ctx/vorma_ctx.ts";
import { resolveNavigationTargetURL } from "./target_url.ts";
import type {
	NavigateProps,
	NavigationEntry,
	NavigationOutcome,
} from "./types.ts";

export type HandleNavigationOutcomeContext = {
	findNavigationEntry: (targetUrl: string) => NavigationEntry | undefined;
	deleteNavigation: (key: string) => boolean;
	processSuccessfulNavigation: (
		outcome: Extract<NavigationOutcome, { type: "success" }>,
		entry: NavigationEntry,
	) => Promise<void>;
	onNavigationIntentResolved?: () => void;
};

export async function handleNavigationOutcome(
	context: HandleNavigationOutcomeContext,
	props: NavigateProps,
	outcome: NavigationOutcome,
): Promise<{ didNavigate: boolean }> {
	switch (outcome.type) {
		case "aborted": {
			const targetUrl = resolveNavigationTargetURL(props.href);
			context.deleteNavigation(targetUrl);
			return { didNavigate: false };
		}

		case "redirect": {
			const targetUrl = resolveNavigationTargetURL(props.href);
			const entry = context.findNavigationEntry(targetUrl);
			if (!entry) {
				return { didNavigate: false };
			}

			// Skip redirect effectuation for pure prefetches
			if (entry.type === "prefetch" && entry.intent === "none") {
				context.deleteNavigation(targetUrl);
				return { didNavigate: false };
			}

			// Redirect responses can carry a newer build ID. Persist and
			// publish it before following the redirect target so subsequent
			// fetches use the fresh build marker.
			if (outcome.redirectData.status === "should") {
				const oldID = __vormaClientGlobal.get("buildID");
				const newID = outcome.redirectData.latestBuildID;
				if (newID && newID !== oldID) {
					__vormaClientGlobal.set("buildID", newID);
					dispatchBuildIDEvent({ newID, oldID });
				}
			}

			context.deleteNavigation(targetUrl);
			await effectuateRedirectDataResult(
				outcome.redirectData,
				props.redirectCount || 0,
				props,
			);
			return { didNavigate: false };
		}

		case "success": {
			const targetUrl = resolveNavigationTargetURL(props.href);
			const entry = context.findNavigationEntry(targetUrl);
			if (!entry) {
				return { didNavigate: false };
			}

			if (entry.intent === "navigate" || entry.intent === "revalidate") {
				context.onNavigationIntentResolved?.();
			}

			await context.processSuccessfulNavigation(outcome, entry);

			if (entry.intent === "none" && entry.type === "prefetch") {
				return { didNavigate: false };
			}

			return { didNavigate: true };
		}

		default: {
			// Exhaustiveness check - TypeScript will error if a case is missing
			const exhaustive: never = outcome;
			throw new Error(
				`Unexpected navigation outcome type: ${(exhaustive as any).type}`,
			);
		}
	}
}
