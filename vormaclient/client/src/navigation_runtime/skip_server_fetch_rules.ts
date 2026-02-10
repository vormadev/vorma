import { jsonDeepEquals } from "vorma/kit/json";
import { type Match } from "vorma/kit/matcher/find-nested";
import type {
	SkipCheckContext,
	SkipCheckResult,
} from "./skip_server_fetch_types.ts";

function hasServerLoaderRemoval(ctx: SkipCheckContext): boolean {
	for (const pattern of ctx.currentMatchedPatterns) {
		const hasServerLoader = ctx.routeManifest[pattern] === 1;
		if (hasServerLoader) {
			const stillMatched = ctx.matchResult.matches.some(
				(m: Match) => m.registeredPattern.originalPattern === pattern,
			);
			if (!stillMatched) {
				return true;
			}
		}
	}
	return false;
}

function hasNewClientLoader(ctx: SkipCheckContext): boolean {
	for (const m of ctx.matchResult.matches) {
		const pattern = m.registeredPattern.originalPattern;
		const hasClientLoader = !!ctx.patternToWaitFnMap[pattern];
		const wasAlreadyMatched = ctx.currentMatchedPatterns.includes(pattern);
		if (hasClientLoader && !wasAlreadyMatched) {
			return true;
		}
	}
	return false;
}

function findOutermostLoaderIndex(ctx: SkipCheckContext): number {
	for (let i = ctx.matchResult.matches.length - 1; i >= 0; i--) {
		const match: Match | undefined = ctx.matchResult.matches[i];
		if (!match) continue;

		const pattern = match.registeredPattern.originalPattern;
		const hasServerLoader = ctx.routeManifest[pattern] === 1;
		const hasClientLoader = !!ctx.patternToWaitFnMap[pattern];

		if (hasServerLoader || hasClientLoader) {
			return i;
		}
	}
	return -1;
}

function didSearchParamsChange(ctx: SkipCheckContext): boolean {
	const currentUrlObj = new URL(window.location.href);
	const currentParamsSorted = Array.from(
		currentUrlObj.searchParams.entries(),
	).sort();
	const targetParamsSorted = Array.from(
		ctx.url.searchParams.entries(),
	).sort();
	return !jsonDeepEquals(currentParamsSorted, targetParamsSorted);
}

function didOutermostParamsChange(
	ctx: SkipCheckContext,
	outermostLoaderIndex: number,
): boolean {
	const outermostMatch = ctx.matchResult.matches[outermostLoaderIndex];
	if (!outermostMatch) return false;

	for (const seg of outermostMatch.registeredPattern.normalizedSegments) {
		if (seg.segType === "dynamic") {
			const paramName = seg.normalizedVal.substring(1);
			if (
				ctx.matchResult.params[paramName] !==
				ctx.currentParams[paramName]
			) {
				return true;
			}
		}
	}

	const hasSplat = outermostMatch.registeredPattern.lastSegType === "splat";
	if (hasSplat) {
		if (
			!jsonDeepEquals(ctx.matchResult.splatValues, ctx.currentSplatValues)
		) {
			return true;
		}
	}

	return false;
}

export function isSkipEligibilityViolated(ctx: SkipCheckContext): boolean {
	if (hasServerLoaderRemoval(ctx)) {
		return true;
	}

	if (hasNewClientLoader(ctx)) {
		return true;
	}

	const outermostLoaderIndex = findOutermostLoaderIndex(ctx);

	if (outermostLoaderIndex !== -1 && didSearchParamsChange(ctx)) {
		return true;
	}

	if (
		outermostLoaderIndex !== -1 &&
		didOutermostParamsChange(ctx, outermostLoaderIndex)
	) {
		return true;
	}

	return false;
}

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
		const moduleInfo = ctx.clientModuleMap[pattern];
		if (!moduleInfo) {
			return { canSkip: false };
		}

		importURLs.push(moduleInfo.importURL);
		exportKeys.push(moduleInfo.exportKey);

		const hasServerLoader = ctx.routeManifest[pattern] === 1;
		if (!hasServerLoader) {
			loadersData.push(undefined);
		} else {
			const currentPatternIndex =
				ctx.currentMatchedPatterns.indexOf(pattern);
			if (currentPatternIndex === -1) {
				return { canSkip: false };
			}
			loadersData.push(ctx.currentLoadersData[currentPatternIndex]);
		}
	}

	return {
		canSkip: true,
		matchResult: ctx.matchResult,
		importURLs,
		exportKeys,
		loadersData,
	};
}
