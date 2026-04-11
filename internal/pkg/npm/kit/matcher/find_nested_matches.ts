import type {
	Params,
	PatternRegistry,
	RegisteredPattern,
	SegmentNode,
} from "vorma/kit/matcher/register";
import {
	NODE_DYNAMIC,
	NODE_SPLAT,
	SEG_TYPES,
} from "vorma/kit/matcher/register";
import { parseSegments, stripTrailingSlash } from "vorma/kit/matcher/utils";

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

function dfs_nested(
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
			const params_copy = { ...params };
			let splat_values: string[] = [];

			if (node.nodeType === NODE_SPLAT && depth < segments.length) {
				splat_values = segments.slice(depth);
			}

			matches.set(node.pattern, {
				registeredPattern: rp,
				params: params_copy,
				splatValues: splat_values,
			});

			if (depth === segments.length) {
				const idx_pattern = node.pattern + "/";
				const rp_idx = registry.dynamicPatterns.get(idx_pattern);
				if (rp_idx) {
					matches.set(idx_pattern, {
						registeredPattern: rp_idx,
						params: params_copy,
						splatValues: [],
					});
				}
			}
		}
	}

	if (depth >= segments.length) {
		return;
	}

	const seg = segments[depth]!;

	if (node.children !== null) {
		const child = node.children.get(seg);
		if (child) {
			dfs_nested(registry, child, segments, depth + 1, params, matches);
		}
	}

	for (const child of node.dynChildren) {
		switch (child.nodeType) {
			case NODE_DYNAMIC: {
				const old_val = params[child.paramName];
				const had_val = old_val !== undefined;
				params[child.paramName] = seg;

				dfs_nested(
					registry,
					child,
					segments,
					depth + 1,
					params,
					matches,
				);

				if (had_val) {
					params[child.paramName] = old_val!;
				} else {
					delete params[child.paramName];
				}
				break;
			}
			case NODE_SPLAT:
				dfs_nested(registry, child, segments, depth, params, matches);
				break;
		}
	}
}

function flatten_and_sort(
	matches: Map<string, Match>,
	real_path: string,
	real_seg_len: number,
): FindNestedMatchesResult | null {
	const results: Match[] = Array.from(matches.values());

	if (results.length === 0) {
		return null;
	}

	if (results.length > 1) {
		results.sort((a, b) => {
			// Index segments sort last.
			if (
				a.registeredPattern.lastSegIsIndex !==
				b.registeredPattern.lastSegIsIndex
			) {
				return a.registeredPattern.lastSegIsIndex ? 1 : -1;
			}

			// Sort by segment count.
			const len_diff =
				a.registeredPattern.normalizedSegments.length -
				b.registeredPattern.normalizedSegments.length;
			if (len_diff !== 0) {
				return len_diff;
			}

			// Deterministic tiebreaker: lexicographic on normalized pattern.
			if (
				a.registeredPattern.normalizedPattern <
				b.registeredPattern.normalizedPattern
			) {
				return -1;
			}
			if (
				a.registeredPattern.normalizedPattern >
				b.registeredPattern.normalizedPattern
			) {
				return 1;
			}
			return 0;
		});
	}

	const is_not_slash = real_path !== "" && real_path !== "/";
	if (
		is_not_slash &&
		results.length === 1 &&
		results[0]!.registeredPattern.normalizedPattern === ""
	) {
		return null;
	}

	const last = results[results.length - 1]!;

	if (
		!last.registeredPattern.lastSegIsNonRootSplat &&
		last.registeredPattern.normalizedPattern !== "/*"
	) {
		const pat_seg_len = last.registeredPattern.normalizedSegments.length;

		if (pat_seg_len < real_seg_len) {
			return null;
		}

		if (
			pat_seg_len === real_seg_len &&
			last.registeredPattern.numberOfDynamicParamSegs > 0 &&
			Object.keys(last.params).length === 0
		) {
			return null;
		}
	}

	return {
		params: last.params,
		splatValues: last.splatValues,
		matches: results,
	};
}

export function findNestedMatches(
	registry: PatternRegistry,
	real_path: string,
): FindNestedMatchesResult | null {
	real_path = stripTrailingSlash(real_path);

	const real_segs = parseSegments(real_path);
	const real_segs_len = real_segs.length;
	const matches: Map<string, Match> = new Map();

	const empty_rr = registry.staticPatterns.get("");
	const has_empty = empty_rr !== undefined;
	if (has_empty) {
		matches.set(empty_rr.normalizedPattern, {
			registeredPattern: empty_rr,
			params: {},
			splatValues: [],
		});
	}

	if (real_path === "") {
		const rr = registry.staticPatterns.get("/");
		if (rr) {
			matches.set(rr.normalizedPattern, {
				registeredPattern: rr,
				params: {},
				splatValues: [],
			});
		} else {
			const catch_all = registry.dynamicPatterns.get("/*");
			if (catch_all) {
				matches.set("/*", {
					registeredPattern: catch_all,
					params: {},
					splatValues: [],
				});
			}
		}
		return flatten_and_sort(matches, real_path, real_segs_len);
	}

	// Progressive static pattern matching.
	let pb = "";
	let found_full_static = false;

	for (let i = 0; i < real_segs.length; i++) {
		pb += "/" + real_segs[i];
		const rr = registry.staticPatterns.get(pb);
		if (rr) {
			matches.set(rr.normalizedPattern, {
				registeredPattern: rr,
				params: {},
				splatValues: [],
			});
			if (i === real_segs_len - 1) {
				found_full_static = true;
			}
		}
		if (i === real_segs_len - 1) {
			pb += "/";
			const rr_slash = registry.staticPatterns.get(pb);
			if (rr_slash) {
				matches.set(rr_slash.normalizedPattern, {
					registeredPattern: rr_slash,
					params: {},
					splatValues: [],
				});
			}
		}
	}

	if (!found_full_static) {
		const rr = registry.dynamicPatterns.get("/*");
		if (rr) {
			matches.set("/*", {
				registeredPattern: rr,
				params: {},
				splatValues: real_segs,
			});
		}

		const params: Params = {};
		dfs_nested(registry, registry.rootNode, real_segs, 0, params, matches);
	}

	// Prune catch-all when better matches exist.
	if (matches.has("/*")) {
		if (has_empty) {
			if (matches.size > 2) {
				matches.delete("/*");
			}
		} else if (matches.size > 1) {
			matches.delete("/*");
		}
	}

	if (matches.size < 2) {
		return flatten_and_sort(matches, real_path, real_segs_len);
	}

	// Track longest-segment match types.
	let longest_len = 0;
	let longest_index: Match | null = null;
	let longest_dynamic: Match | null = null;
	let longest_splat: Match | null = null;

	for (const match of matches.values()) {
		const seg_len = match.registeredPattern.normalizedSegments.length;
		if (seg_len > longest_len) {
			longest_len = seg_len;
			longest_index = null;
			longest_dynamic = null;
			longest_splat = null;
		}
		if (seg_len === longest_len) {
			switch (match.registeredPattern.lastSegType) {
				case SEG_TYPES.index:
					longest_index = match;
					break;
				case SEG_TYPES.dynamic:
					longest_dynamic = match;
					break;
				case SEG_TYPES.splat:
					longest_splat = match;
					break;
			}
		}
	}

	// Remove shorter splats and indexes.
	for (const [pattern, match] of matches) {
		if (match.registeredPattern.normalizedSegments.length < longest_len) {
			if (
				match.registeredPattern.lastSegIsNonRootSplat ||
				match.registeredPattern.lastSegIsIndex
			) {
				matches.delete(pattern);
			}
		}
	}

	if (matches.size < 2) {
		return flatten_and_sort(matches, real_path, real_segs_len);
	}

	// Disambiguate longest-segment matches.
	let type_count = 0;
	if (longest_index !== null) {
		type_count++;
	}
	if (longest_dynamic !== null) {
		type_count++;
	}
	if (longest_splat !== null) {
		type_count++;
	}

	if (type_count > 1) {
		if (longest_index !== null) {
			matches.delete(longest_index.registeredPattern.normalizedPattern);
		}

		const has_dyn = longest_dynamic !== null;
		const has_spl = longest_splat !== null;

		if (real_segs_len === longest_len && has_dyn && has_spl) {
			matches.delete(longest_splat!.registeredPattern.normalizedPattern);
		}
		if (real_segs_len > longest_len && has_spl && has_dyn) {
			matches.delete(
				longest_dynamic!.registeredPattern.normalizedPattern,
			);
		}
	}

	return flatten_and_sort(matches, real_path, real_segs_len);
}
