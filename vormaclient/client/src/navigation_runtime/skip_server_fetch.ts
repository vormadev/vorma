import { buildSkipCheckContext } from "./skip_server_fetch_context.ts";
import {
	buildSkipResultFromContext,
	isSkipEligibilityViolated,
} from "./skip_server_fetch_rules.ts";
import type { SkipCheckResult } from "./skip_server_fetch_types.ts";

export type { SkipCheckResult } from "./skip_server_fetch_types.ts";

export function canSkipServerFetch(targetUrl: string): SkipCheckResult {
	const ctx = buildSkipCheckContext(targetUrl);
	if (!ctx) {
		return { canSkip: false };
	}

	if (isSkipEligibilityViolated(ctx)) {
		return { canSkip: false };
	}

	return buildSkipResultFromContext(ctx);
}
