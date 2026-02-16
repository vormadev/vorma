import { parseSegments } from "./parse_segments.ts";
import type {
	Params,
	PatternRegistry,
	RegisteredPattern,
	SegmentNode,
} from "./register.ts";
import { NODE_DYNAMIC, NODE_SPLAT, SEG_TYPES } from "./register.ts";

export type Match = {
	registeredPattern: RegisteredPattern;
	params: Params;
	splatValues: string[];
};

type FindNestedMatchesResult = {
	params: Params;
	splatValues: string[];
	matches: Match[];
};

function stripTrailingSlash(pattern: string): string {
	return pattern.length > 0 && pattern[pattern.length - 1] === "/"
		? pattern.substring(0, pattern.length - 1)
		: pattern;
}

function dfsNestedMatches(
	registry: PatternRegistry,
	node: SegmentNode,
	segments: string[],
	depth: number,
	params: Params,
	matches: Map<string, Match>,
): void {
	if (node.pattern.length > 0) {
		const rp = registry.dynamicPatterns.get(node.pattern);
		if (rp && node.pattern !== "/*") {
			const paramsCopy = { ...params };
			let splatValues: string[] = [];

			if (node.nodeType === NODE_SPLAT && depth < segments.length) {
				splatValues = segments.slice(depth);
			}

			matches.set(node.pattern, {
				registeredPattern: rp,
				params: paramsCopy,
				splatValues,
			});

			if (depth === segments.length) {
				const indexPattern = node.pattern + "/";
				const rpIndex = registry.dynamicPatterns.get(indexPattern);
				if (rpIndex) {
					matches.set(indexPattern, {
						registeredPattern: rpIndex,
						params: paramsCopy,
						splatValues: [],
					});
				}
			}
		}
	}

	if (depth >= segments.length) return;

	const seg = segments[depth]!;

	if (node.children !== null) {
		const child = node.children.get(seg);
		if (child) {
			dfsNestedMatches(
				registry,
				child,
				segments,
				depth + 1,
				params,
				matches,
			);
		}
	}

	for (const child of node.dynChildren) {
		switch (child.nodeType) {
			case NODE_DYNAMIC: {
				const oldVal = params[child.paramName];
				const hadVal = oldVal !== undefined;
				params[child.paramName] = seg;

				dfsNestedMatches(
					registry,
					child,
					segments,
					depth + 1,
					params,
					matches,
				);

				if (hadVal) {
					params[child.paramName] = oldVal!;
				} else {
					delete params[child.paramName];
				}
				break;
			}
			case NODE_SPLAT:
				dfsNestedMatches(
					registry,
					child,
					segments,
					depth,
					params,
					matches,
				);
				break;
		}
	}
}

function flattenAndSortMatches(
	matches: Map<string, Match>,
	realPath: string,
	realSegmentLen: number,
): FindNestedMatchesResult | null {
	const results: Match[] = Array.from(matches.values());

	results.sort((i, j) => {
		if (i.registeredPattern.lastSegIsIndex) return 1;
		if (j.registeredPattern.lastSegIsIndex) return -1;
		return (
			i.registeredPattern.normalizedSegments.length -
			j.registeredPattern.normalizedSegments.length
		);
	});

	if (results.length === 0) return null;

	const isNotSlashRoute = realPath !== "" && realPath !== "/";
	if (
		isNotSlashRoute &&
		results.length === 1 &&
		results[0]!.registeredPattern.normalizedPattern === ""
	) {
		return null;
	}

	const lastMatch = results[results.length - 1]!;

	if (
		!lastMatch.registeredPattern.lastSegIsNonRootSplat &&
		lastMatch.registeredPattern.normalizedPattern !== "/*"
	) {
		const patternSegmentsLen =
			lastMatch.registeredPattern.normalizedSegments.length;

		if (patternSegmentsLen < realSegmentLen) return null;

		if (
			patternSegmentsLen === realSegmentLen &&
			lastMatch.registeredPattern.numberOfDynamicParamSegs > 0 &&
			Object.keys(lastMatch.params).length === 0
		) {
			return null;
		}
	}

	return {
		params: lastMatch.params,
		splatValues: lastMatch.splatValues,
		matches: results,
	};
}

export function findNestedMatches(
	registry: PatternRegistry,
	realPath: string,
): FindNestedMatchesResult | null {
	realPath = stripTrailingSlash(realPath);

	const realSegments = parseSegments(realPath);
	const matches: Map<string, Match> = new Map();

	// Check for empty pattern
	const emptyRR = registry.staticPatterns.get("");
	if (emptyRR) {
		matches.set(emptyRR.normalizedPattern, {
			registeredPattern: emptyRR,
			params: {},
			splatValues: [],
		});
	}

	const realSegmentsLen = realSegments.length;

	// Handle root path
	if (realPath === "") {
		const rr = registry.staticPatterns.get("/");
		if (rr) {
			matches.set(rr.normalizedPattern, {
				registeredPattern: rr,
				params: {},
				splatValues: [],
			});
		} else {
			const catchAllRR = registry.dynamicPatterns.get("/*");
			if (catchAllRR) {
				matches.set("/*", {
					registeredPattern: catchAllRR,
					params: {},
					splatValues: [],
				});
			}
		}
		return flattenAndSortMatches(matches, realPath, realSegmentsLen);
	}

	// Check static patterns progressively
	let pb = "";
	let foundFullStatic = false;

	for (let i = 0; i < realSegments.length; i++) {
		pb += "/" + realSegments[i];
		const rr = registry.staticPatterns.get(pb);
		if (rr) {
			matches.set(rr.normalizedPattern, {
				registeredPattern: rr,
				params: {},
				splatValues: [],
			});
			if (i === realSegmentsLen - 1) foundFullStatic = true;
		}
		if (i === realSegmentsLen - 1) {
			pb += "/";
			const rrWithSlash = registry.staticPatterns.get(pb);
			if (rrWithSlash) {
				matches.set(rrWithSlash.normalizedPattern, {
					registeredPattern: rrWithSlash,
					params: {},
					splatValues: [],
				});
			}
		}
	}

	// Check dynamic patterns if no full static match
	if (!foundFullStatic) {
		const rr = registry.dynamicPatterns.get("/*");
		if (rr) {
			matches.set("/*", {
				registeredPattern: rr,
				params: {},
				splatValues: realSegments,
			});
		}

		const params: Params = {};
		dfsNestedMatches(
			registry,
			registry.rootNode,
			realSegments,
			0,
			params,
			matches,
		);
	}

	// Clean up catch-all pattern if necessary
	const hasEmptyRR = emptyRR !== undefined;
	if (matches.has("/*")) {
		if (hasEmptyRR) {
			if (matches.size > 2) matches.delete("/*");
		} else if (matches.size > 1) {
			matches.delete("/*");
		}
	}

	if (matches.size < 2) {
		return flattenAndSortMatches(matches, realPath, realSegmentsLen);
	}

	// Find longest segment matches
	let longestSegmentLen = 0;
	const longestSegmentMatches: Map<string, Match> = new Map();

	for (const match of matches.values()) {
		if (
			match.registeredPattern.normalizedSegments.length >
			longestSegmentLen
		) {
			longestSegmentLen =
				match.registeredPattern.normalizedSegments.length;
		}
	}

	for (const match of matches.values()) {
		if (
			match.registeredPattern.normalizedSegments.length ===
			longestSegmentLen
		) {
			longestSegmentMatches.set(
				match.registeredPattern.lastSegType,
				match,
			);
		}
	}

	// Remove shorter matches
	for (const [pattern, match] of matches) {
		if (
			match.registeredPattern.normalizedSegments.length <
			longestSegmentLen
		) {
			if (
				match.registeredPattern.lastSegIsNonRootSplat ||
				match.registeredPattern.lastSegIsIndex
			) {
				matches.delete(pattern);
			}
		}
	}

	if (matches.size < 2) {
		return flattenAndSortMatches(matches, realPath, realSegmentsLen);
	}

	// Handle conflicts between longest matches
	if (longestSegmentMatches.size > 1) {
		const indexMatch = longestSegmentMatches.get(SEG_TYPES.index);
		if (indexMatch) {
			matches.delete(indexMatch.registeredPattern.normalizedPattern);
		}

		const dynamicExists = longestSegmentMatches.has(SEG_TYPES.dynamic);
		const splatExists = longestSegmentMatches.has(SEG_TYPES.splat);

		if (
			realSegmentsLen === longestSegmentLen &&
			dynamicExists &&
			splatExists
		) {
			const splatMatch = longestSegmentMatches.get(SEG_TYPES.splat)!;
			matches.delete(splatMatch.registeredPattern.normalizedPattern);
		}

		if (
			realSegmentsLen > longestSegmentLen &&
			splatExists &&
			dynamicExists
		) {
			const dynamicMatch = longestSegmentMatches.get(SEG_TYPES.dynamic)!;
			matches.delete(dynamicMatch.registeredPattern.normalizedPattern);
		}
	}

	return flattenAndSortMatches(matches, realPath, realSegmentsLen);
}
