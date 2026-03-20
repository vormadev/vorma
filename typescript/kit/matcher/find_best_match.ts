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
} from "vorma/kit/matcher/register";
import { parseSegments } from "vorma/kit/matcher/utils";

type BestMatch = {
	registeredPattern: RegisteredPattern;
	params: Params;
	splatValues: string[];
	score: number;
};

type DfsBestState = {
	best: BestMatch | null;
	best_score: number;
	found: boolean;
};

function dfs_best(
	registry: PatternRegistry,
	node: SegmentNode,
	segments: string[],
	depth: number,
	score: number,
	state: DfsBestState,
	check_trailing: boolean,
): void {
	const at_normal_end = check_trailing && depth === segments.length - 1;

	if (node.pattern.length > 0) {
		const rp = registry.dynamicPatterns.get(node.pattern);
		if (rp) {
			if (
				depth === segments.length ||
				node.nodeType === NODE_SPLAT ||
				at_normal_end
			) {
				if (!state.found || score > state.best_score) {
					state.best = {
						registeredPattern: rp,
						params: {},
						splatValues: [],
						score,
					};
					state.best_score = score;
					state.found = true;
				}
			}
		}
	}

	if (depth >= segments.length) return;

	if (node.children !== null) {
		const child = node.children.get(segments[depth]!);
		if (child) {
			dfs_best(
				registry,
				child,
				segments,
				depth + 1,
				score + SCORE_STATIC_MATCH,
				state,
				check_trailing,
			);

			if (
				state.found &&
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
					dfs_best(
						registry,
						child,
						segments,
						depth + 1,
						score + SCORE_DYNAMIC,
						state,
						check_trailing,
					);
				}
				break;

			case NODE_SPLAT:
				if (child.pattern.length > 0) {
					const rp = registry.dynamicPatterns.get(child.pattern);
					if (rp && !state.found) {
						state.best = {
							registeredPattern: rp,
							params: {},
							splatValues: [],
							score: 0,
						};
						state.found = true;
					}
				}
				break;
		}
	}
}

export function findBestMatch(
	registry: PatternRegistry,
	real_path: string,
): BestMatch | null {
	const rr = registry.staticPatterns.get(real_path);
	if (rr) {
		return {
			registeredPattern: rr,
			params: {},
			splatValues: [],
			score: 0,
		};
	}

	const segments = parseSegments(real_path);
	const has_trailing =
		real_path.length > 0 && real_path[real_path.length - 1] === "/";

	if (has_trailing) {
		const without_slash = real_path.substring(0, real_path.length - 1);
		const rr_no_slash = registry.staticPatterns.get(without_slash);
		if (rr_no_slash) {
			return {
				registeredPattern: rr_no_slash,
				params: {},
				splatValues: [],
				score: 0,
			};
		}
	}

	const state: DfsBestState = {
		best: null,
		best_score: 0,
		found: false,
	};

	dfs_best(registry, registry.rootNode, segments, 0, 0, state, has_trailing);

	if (!state.found || !state.best) {
		return null;
	}

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
