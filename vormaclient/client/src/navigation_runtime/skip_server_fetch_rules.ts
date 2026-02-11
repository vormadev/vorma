import { type Match } from "vorma/kit/matcher/find-nested";
import { buildSkipResultItem } from "./skip_server_fetch_result_item.ts";
import type {
	SkipCheckContext,
	SkipCheckResult,
} from "./skip_server_fetch_types.ts";
export { isSkipEligibilityViolated } from "./skip_server_fetch_eligibility.ts";

export function buildSkipResultFromContext(
	ctx: SkipCheckContext,
): SkipCheckResult {
	const importURLs: string[] = [];
	const exportKeys: string[] = [];
	const loadersData: any[] = [];

	for (let i = 0; i < ctx.matchResult.matches.length; i++) {
		const match: Match | undefined = ctx.matchResult.matches[i];
		if (!match) continue;

		const pattern = match.registeredPattern.originalPattern;
		const item = buildSkipResultItem({ ctx, pattern });
		if (!item) {
			return { canSkip: false };
		}

		importURLs.push(item.importURL);
		exportKeys.push(item.exportKey);
		loadersData.push(item.loaderData);
	}

	return {
		canSkip: true,
		matchResult: ctx.matchResult,
		importURLs,
		exportKeys,
		loadersData,
	};
}
