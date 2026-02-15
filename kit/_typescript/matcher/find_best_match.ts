import { parseSegments } from "./parse_segments.ts";
import {
	NODE_DYNAMIC,
	NODE_SPLAT,
	SCORE_DYNAMIC,
	SCORE_STATIC_MATCH,
	SEG_TYPES,
	type Params,
	type PatternRegistry,
	type RegisteredPattern,
	type SegmentNode,
} from "./register.ts";

type BestMatch = {
	registeredPattern: RegisteredPattern;
	params: Params;
	splatValues: string[];
	score: number;
};

type DfsBestState = {
	best: BestMatch | null;
	bestScore: number;
	foundMatch: boolean;
};

function dfsBest(props: {
	registry: PatternRegistry;
	node: SegmentNode;
	segments: string[];
	depth: number;
	score: number;
	state: DfsBestState;
	checkTrailingSlash: boolean;
}): void {
	const {
		registry,
		node,
		segments,
		depth,
		score,
		state,
		checkTrailingSlash,
	} = props;
	const atNormalEnd = checkTrailingSlash && depth === segments.length - 1;

	if (node.pattern.length > 0) {
		const rp = registry.dynamicPatterns.get(node.pattern);
		if (rp) {
			if (
				depth === segments.length ||
				node.nodeType === NODE_SPLAT ||
				atNormalEnd
			) {
				if (!state.foundMatch || score > state.bestScore) {
					state.best = {
						registeredPattern: rp,
						params: {},
						splatValues: [],
						score,
					};
					state.bestScore = score;
					state.foundMatch = true;
				}
			}
		}
	}

	if (depth >= segments.length) return;

	if (node.children !== null) {
		const child = node.children.get(segments[depth]!);
		if (child) {
			dfsBest({
				registry,
				node: child,
				segments,
				depth: depth + 1,
				score: score + SCORE_STATIC_MATCH,
				state,
				checkTrailingSlash,
			});

			if (
				state.foundMatch &&
				depth + 1 === segments.length &&
				child.pattern !== ""
			) {
				return;
			}
		}
	}

	for (const child of node.dynChildren) {
		switch (child.nodeType) {
			case NODE_DYNAMIC:
				if (segments[depth] !== "") {
					dfsBest({
						registry,
						node: child,
						segments,
						depth: depth + 1,
						score: score + SCORE_DYNAMIC,
						state,
						checkTrailingSlash,
					});
				}
				break;

			case NODE_SPLAT:
				if (child.pattern.length > 0) {
					const rp = registry.dynamicPatterns.get(child.pattern);
					if (rp && !state.foundMatch) {
						state.best = {
							registeredPattern: rp,
							params: {},
							splatValues: [],
							score: 0,
						};
						state.foundMatch = true;
					}
				}
				break;
		}
	}
}

export function findBestMatch(
	registry: PatternRegistry,
	realPath: string,
): BestMatch | null {
	// Check static patterns first
	const rr = registry.staticPatterns.get(realPath);
	if (rr) {
		return {
			registeredPattern: rr,
			params: {},
			splatValues: [],
			score: 0,
		};
	}

	const segments = parseSegments(realPath);
	const hasTrailingSlash =
		realPath.length > 0 && realPath[realPath.length - 1] === "/";

	// Check static pattern without trailing slash
	if (hasTrailingSlash) {
		const pathWithoutTrailingSlash = realPath.substring(
			0,
			realPath.length - 1,
		);
		const rrWithoutSlash = registry.staticPatterns.get(
			pathWithoutTrailingSlash,
		);
		if (rrWithoutSlash) {
			return {
				registeredPattern: rrWithoutSlash,
				params: {},
				splatValues: [],
				score: 0,
			};
		}
	}

	// Search for dynamic patterns
	const state: DfsBestState = {
		best: null,
		bestScore: 0,
		foundMatch: false,
	};

	dfsBest({
		registry,
		node: registry.rootNode,
		segments,
		depth: 0,
		score: 0,
		state,
		checkTrailingSlash: hasTrailingSlash,
	});

	if (!state.foundMatch || !state.best) {
		return null;
	}

	// Populate params
	if (state.best.registeredPattern.numberOfDynamicParamSegs > 0) {
		const params: Params = {};
		for (
			let i = 0;
			i < state.best.registeredPattern.normalizedSegments.length;
			i++
		) {
			const seg = state.best.registeredPattern.normalizedSegments[i]!;
			if (seg.segType === SEG_TYPES.dynamic) {
				params[seg.normalizedVal.substring(1)] = segments[i]!;
			}
		}
		state.best.params = params;
	}

	// Populate splat values
	if (
		state.best.registeredPattern.normalizedPattern === "/*" ||
		state.best.registeredPattern.lastSegIsNonRootSplat
	) {
		state.best.splatValues = segments.slice(
			state.best.registeredPattern.normalizedSegments.length - 1,
		);
	}

	return state.best;
}
