import { effectuateRedirectDataResult } from "../redirects/redirects.ts";
import { syncBuildIDFromRedirectData } from "../redirects/redirect_build_id.ts";
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
	const targetUrl = resolveNavigationTargetURL(props.href);

	switch (outcome.type) {
		case "aborted": {
			context.deleteNavigation(targetUrl);
			return { didNavigate: false };
		}

		case "redirect": {
			const entry = context.findNavigationEntry(targetUrl);
			if (!entry) {
				return { didNavigate: false };
			}

			// Skip redirect effectuation for pure prefetches
			if (entry.type === "prefetch" && entry.intent === "none") {
				context.deleteNavigation(targetUrl);
				return { didNavigate: false };
			}

			// Persist/publish redirect build ID before following redirect target.
			syncBuildIDFromRedirectData(outcome.redirectData);

			context.deleteNavigation(targetUrl);
			await effectuateRedirectDataResult(
				outcome.redirectData,
				props.redirectCount || 0,
				props,
			);
			return { didNavigate: false };
		}

		case "success": {
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
