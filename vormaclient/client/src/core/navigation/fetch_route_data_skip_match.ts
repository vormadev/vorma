import { jsonDeepEquals } from "vorma/kit/json";
import { findNestedMatches } from "vorma/kit/matcher/find-nested";
import type { PatternRegistry } from "vorma/kit/matcher/register";
import { __vormaClientGlobal } from "../../app/context.ts";
import type { VormaClientGlobal } from "../../app/context.ts";

export type SkipMatch = {
	registeredPattern: {
		originalPattern: string;
		normalizedSegments: Array<{
			segType: string;
			normalizedVal: string;
		}>;
		lastSegType: string;
	};
};

export type SkipMatchResult = {
	params: Record<string, string>;
	splatValues: string[];
	matches: SkipMatch[];
};

export type SkipCheckContext = {
	routeManifest: Record<string, number>;
	patternRegistry: PatternRegistry;
	patternToWaitFnMap: VormaClientGlobal["patternToWaitFnMap"];
	clientModuleMap: VormaClientGlobal["clientModuleMap"];
	currentMatchedPatterns: string[];
	currentParams: Record<string, string>;
	currentSplatValues: string[];
	currentLoadersData: unknown[];
	url: URL;
	matchResult: SkipMatchResult;
};

type SkipCheckContextGlobalSnapshot = {
	routeManifest: Record<string, number>;
	patternRegistry: PatternRegistry;
	patternToWaitFnMap: VormaClientGlobal["patternToWaitFnMap"];
	clientModuleMap: VormaClientGlobal["clientModuleMap"];
	currentMatchedPatterns: string[];
	currentParams: Record<string, string>;
	currentSplatValues: string[];
	currentLoadersData: unknown[];
};

function getSkipCheckContextGlobalSnapshot():
	| SkipCheckContextGlobalSnapshot
	| undefined {
	const routeManifest = __vormaClientGlobal.get("routeManifest");
	if (!routeManifest) {
		return undefined;
	}

	const patternRegistry = __vormaClientGlobal.get("patternRegistry");
	if (!patternRegistry) {
		return undefined;
	}

	return {
		routeManifest,
		patternRegistry,
		patternToWaitFnMap: __vormaClientGlobal.get("patternToWaitFnMap") || {},
		clientModuleMap: __vormaClientGlobal.get("clientModuleMap") || {},
		currentMatchedPatterns:
			__vormaClientGlobal.get("matchedPatterns") || [],
		currentParams: __vormaClientGlobal.get("params") || {},
		currentSplatValues: __vormaClientGlobal.get("splatValues") || [],
		currentLoadersData: __vormaClientGlobal.get("loadersData") || [],
	};
}

export function getMatchedPatternOrThrow(
	match: SkipMatch | undefined,
	index: number,
	context: string,
): string {
	if (!match) {
		throw new Error(
			`${context} returned a sparse matches array at index ${index}.`,
		);
	}

	const pattern = match.registeredPattern.originalPattern;
	if (!pattern) {
		throw new Error(
			`${context} returned an empty route pattern at index ${index}.`,
		);
	}

	return pattern;
}

export function getMatchedPatternsOrThrow(props: {
	matches: Array<SkipMatch | undefined>;
	context: string;
}): string[] {
	const { matches, context } = props;
	const matchedPatterns: string[] = [];

	for (let i = 0; i < matches.length; i++) {
		matchedPatterns.push(getMatchedPatternOrThrow(matches[i], i, context));
	}

	return matchedPatterns;
}

function doesMatchResultContainPattern(
	matchResult: SkipMatchResult,
	pattern: string,
	context: string,
): boolean {
	for (let i = 0; i < matchResult.matches.length; i++) {
		if (
			getMatchedPatternOrThrow(matchResult.matches[i], i, context) ===
			pattern
		) {
			return true;
		}
	}

	return false;
}

export function buildSkipCheckContext(
	targetUrl: string,
): SkipCheckContext | undefined {
	const globalSnapshot = getSkipCheckContextGlobalSnapshot();
	if (!globalSnapshot) {
		return undefined;
	}

	const url = new URL(targetUrl);
	const nestedMatchResult = findNestedMatches(
		globalSnapshot.patternRegistry,
		url.pathname,
	);
	if (!nestedMatchResult) {
		return undefined;
	}
	const matchResult: SkipMatchResult = nestedMatchResult;

	return {
		...globalSnapshot,
		url,
		matchResult,
	};
}

function hasServerLoaderRemoval(ctx: SkipCheckContext): boolean {
	for (const pattern of ctx.currentMatchedPatterns) {
		const hasServerLoader = ctx.routeManifest[pattern] === 1;
		if (hasServerLoader) {
			const stillMatched = doesMatchResultContainPattern(
				ctx.matchResult,
				pattern,
				"Route matcher",
			);
			if (!stillMatched) {
				return true;
			}
		}
	}
	return false;
}

function hasNewClientLoader(ctx: SkipCheckContext): boolean {
	for (let i = 0; i < ctx.matchResult.matches.length; i++) {
		const pattern = getMatchedPatternOrThrow(
			ctx.matchResult.matches[i],
			i,
			"Route matcher",
		);
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
		const pattern = getMatchedPatternOrThrow(
			ctx.matchResult.matches[i],
			i,
			"Route matcher",
		);
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
	return currentUrlObj.search !== ctx.url.search;
}

function didOutermostParamsChange(
	ctx: SkipCheckContext,
	outermostLoaderIndex: number,
): boolean {
	const outermostMatch = ctx.matchResult.matches[outermostLoaderIndex]!;

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
