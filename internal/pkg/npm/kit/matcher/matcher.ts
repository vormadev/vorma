import { R, type Result } from "vorma/kit/result";

/////////////////////////////////////////////////////////////////////
/////// Public Types
/////////////////////////////////////////////////////////////////////

export type Params = Record<string, string>;
export type SegType = "splat" | "static" | "dynamic" | "index";

export type Segment = {
	normalizedVal: string;
	segType: SegType;
};

export type RegisteredPattern = {
	originalPattern: string;
	normalizedPattern: string;
	normalizedSegments: Segment[];
	lastSegType: SegType;
	lastSegIsNonRootSplat: boolean;
	lastSegIsIndex: boolean;
	numberOfDynamicParamSegs: number;
};

type SegmentNode = {
	pattern: string;
	nodeType: number;
	children: Map<string, SegmentNode> | null;
	dynChildren: SegmentNode[];
	paramName: string;
	finalScore: number;
};

export type RegistrationOptions = {
	dynamicParamPrefixRune?: string;
	splatSegmentRune?: string;
	explicitIndexSegment?: string;
};

export type PatternRegistry = {
	staticPatterns: Map<string, RegisteredPattern>;
	dynamicPatterns: Map<string, RegisteredPattern>;
	rootNode: SegmentNode;
	config: {
		dynamicParamPrefixRune: string;
		splatSegmentRune: string;
		explicitIndexSegment: string;
		slashIndexSegment: string;
		usingExplicitIndexSegment: boolean;
	};
};

export type BestMatch = {
	registeredPattern: RegisteredPattern;
	params: Params;
	splatValues: string[];
	score: number;
};

export type Match = {
	registeredPattern: RegisteredPattern;
	params: Params;
	splatValues: string[];
};

export type FindNestedMatchesResult = {
	params: Params;
	splatValues: string[];
	matches: Match[];
};

/////////////////////////////////////////////////////////////////////
/////// Registration
/////////////////////////////////////////////////////////////////////

export function normalizePattern(
	original: string,
	config: PatternRegistry["config"],
): Result<RegisteredPattern> {
	let normalized = original;

	if (config.usingExplicitIndexSegment) {
		if (normalized.endsWith("/")) {
			if (normalized !== "/") {
				return R.err(
					`Error with pattern '${original}'. With the exception of any absolute root pattern ('/'), trailing slashes are not permitted when using an explicit index segment.`,
				);
			}
			normalized = normalized.replace(/\/+$/, "");
		}
		if (normalized.endsWith(config.slashIndexSegment)) {
			normalized = normalized.slice(
				0,
				-config.explicitIndexSegment.length,
			);
		}
	}

	const raw_segments = parseSegments(normalized);
	const segments: Segment[] = [];
	let num_dynamic = 0;

	for (const seg of raw_segments) {
		let val = seg;
		const seg_type = classify_segment(
			seg,
			config.dynamicParamPrefixRune,
			config.splatSegmentRune,
		);

		if (seg_type === seg_types.dynamic) {
			num_dynamic++;
			val = ":" + seg.substring(1);
		}
		if (seg_type === seg_types.splat) {
			val = "*";
		}

		segments.push({ normalizedVal: val, segType: seg_type });
	}

	const seg_len = segments.length;
	const last_type: SegType =
		seg_len > 0 ? segments[seg_len - 1]!.segType : seg_types.static;

	let final_pattern = "/";
	for (let i = 0; i < segments.length; i++) {
		final_pattern += segments[i]!.normalizedVal;
		if (i < seg_len - 1) {
			final_pattern += "/";
		}
	}

	if (final_pattern.endsWith("/") && last_type !== seg_types.index) {
		final_pattern = final_pattern.replace(/\/+$/, "");
	}

	return R.ok({
		originalPattern: original,
		normalizedPattern: final_pattern,
		normalizedSegments: segments,
		lastSegType: last_type,
		lastSegIsNonRootSplat: last_type === seg_types.splat && seg_len > 1,
		lastSegIsIndex: last_type === seg_types.index,
		numberOfDynamicParamSegs: num_dynamic,
	});
}

export function createPatternRegistry(
	opts?: RegistrationOptions,
): Result<PatternRegistry> {
	const config = {
		dynamicParamPrefixRune: opts?.dynamicParamPrefixRune ?? ":",
		splatSegmentRune: opts?.splatSegmentRune ?? "*",
		explicitIndexSegment: opts?.explicitIndexSegment ?? "",
		slashIndexSegment: "/" + (opts?.explicitIndexSegment ?? ""),
		usingExplicitIndexSegment: (opts?.explicitIndexSegment ?? "") !== "",
	};

	if (config.explicitIndexSegment.includes("/")) {
		return R.err("explicit index segment cannot contain a slash");
	}

	return R.ok({
		staticPatterns: new Map(),
		dynamicPatterns: new Map(),
		rootNode: create_segment_node(),
		config,
	});
}

export function registerPattern(
	registry: PatternRegistry,
	original: string,
): Result<RegisteredPattern> {
	const rp_res = normalizePattern(original, registry.config);

	if (!rp_res.ok) {
		return R.err(rp_res.err);
	}

	const rp = rp_res.val;
	const static_pattern = is_static(rp.normalizedSegments);
	const rp_shape_key = static_pattern ? "" : shape_key(rp);

	const stores = [registry.staticPatterns, registry.dynamicPatterns];
	for (const store of stores) {
		const existing = store.get(rp.normalizedPattern);
		if (existing) {
			if (existing.originalPattern === original) {
				return R.ok(existing);
			}
			return R.err(
				`normalized pattern collision: "${original}" and "${existing.originalPattern}" both normalize to "${rp.normalizedPattern}"`,
			);
		}
		if (!static_pattern) {
			for (const existing_pattern of store.values()) {
				if (shape_key(existing_pattern) !== rp_shape_key) {
					continue;
				}
				return R.err(
					`route shape collision: "${original}" and "${existing_pattern.originalPattern}" both match the same paths`,
				);
			}
		}
	}

	if (static_pattern) {
		registry.staticPatterns.set(rp.normalizedPattern, rp);
		return R.ok(rp);
	}

	registry.dynamicPatterns.set(rp.normalizedPattern, rp);

	let current = registry.rootNode;
	let node_score = 0;

	for (let i = 0; i < rp.normalizedSegments.length; i++) {
		const segment = rp.normalizedSegments[i]!;
		const child = find_or_create_child(current, segment.normalizedVal);

		if (segment.segType === seg_types.dynamic) {
			node_score += score_dynamic;
		} else if (segment.segType !== seg_types.splat) {
			node_score += score_static;
		}

		if (i === rp.normalizedSegments.length - 1) {
			child.finalScore = node_score;
			child.pattern = rp.normalizedPattern;
		}

		current = child;
	}

	return R.ok(rp);
}

/////////////////////////////////////////////////////////////////////
/////// Matching
/////////////////////////////////////////////////////////////////////

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
	};

	dfs_best(registry, registry.rootNode, segments, 0, 0, state, has_trailing);

	if (!state.best) {
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
			if (seg.segType === seg_types.dynamic) {
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
				case seg_types.index: {
					longest_index = match;
					break;
				}
				case seg_types.dynamic: {
					longest_dynamic = match;
					break;
				}
				case seg_types.splat: {
					longest_splat = match;
					break;
				}
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
			for (const [pattern, match] of matches) {
				if (
					match.registeredPattern.normalizedSegments.length ===
						longest_len &&
					match.registeredPattern.lastSegType === seg_types.splat
				) {
					matches.delete(pattern);
				}
			}
		}
		if (real_segs_len > longest_len && has_spl && has_dyn) {
			for (const [pattern, match] of matches) {
				if (
					match.registeredPattern.normalizedSegments.length ===
						longest_len &&
					match.registeredPattern.lastSegType === seg_types.dynamic
				) {
					matches.delete(pattern);
				}
			}
		}
	}

	return flatten_and_sort(matches, real_path, real_segs_len);
}

/////////////////////////////////////////////////////////////////////
/////// Public Utility Functions
/////////////////////////////////////////////////////////////////////

export function parseSegments(path: string): string[] {
	if (path === "" || path === "/") {
		return path === "/" ? [""] : [];
	}

	const start_idx = path.startsWith("/") ? 1 : 0;
	const segments: string[] = [];
	let start = start_idx;

	for (let i = start_idx; i < path.length; i++) {
		if (path[i] === "/") {
			if (i > start) {
				segments.push(path.substring(start, i));
			}
			start = i + 1;
		}
	}

	if (start < path.length) {
		segments.push(path.substring(start));
	}

	if (path.endsWith("/")) {
		segments.push("");
	}

	return segments;
}

export function stripTrailingSlash(pattern: string): string {
	if (pattern.length > 0 && pattern[pattern.length - 1] === "/") {
		return pattern.substring(0, pattern.length - 1);
	}
	return pattern;
}

export function hasLeadingSlash(pattern: string): boolean {
	return pattern.length > 0 && pattern[0] === "/";
}

export function hasTrailingSlash(pattern: string): boolean {
	return pattern.length > 0 && pattern[pattern.length - 1] === "/";
}

export function ensureLeadingSlash(pattern: string): string {
	if (!hasLeadingSlash(pattern)) {
		return "/" + pattern;
	}
	return pattern;
}

export function ensureTrailingSlash(pattern: string): string {
	if (!hasTrailingSlash(pattern)) {
		return pattern + "/";
	}
	return pattern;
}

export function stripLeadingSlash(pattern: string): string {
	if (hasLeadingSlash(pattern)) {
		return pattern.substring(1);
	}
	return pattern;
}

export function ensureLeadingAndTrailingSlash(pattern: string): string {
	return ensureLeadingSlash(ensureTrailingSlash(pattern));
}

export function joinPatterns(
	registered_pattern: { normalizedPattern: string },
	pattern: string,
): string {
	let suffix = pattern;
	let out = registered_pattern.normalizedPattern;
	const has_lead = hasLeadingSlash(suffix);
	if (hasTrailingSlash(out) && has_lead) {
		suffix = suffix.substring(1);
	} else if (!has_lead) {
		out += "/";
	}
	return out + suffix;
}

/////////////////////////////////////////////////////////////////////
/////// Private Types And Constants
/////////////////////////////////////////////////////////////////////

type DfsBestState = {
	best: BestMatch | null;
};

const node_static = 0;
const node_dynamic = 1;
const node_splat = 2;
const score_static = 2;
const score_dynamic = 1;

const seg_types = {
	splat: "splat" as SegType,
	static: "static" as SegType,
	dynamic: "dynamic" as SegType,
	index: "index" as SegType,
};

/////////////////////////////////////////////////////////////////////
/////// Private Helpers
/////////////////////////////////////////////////////////////////////

function classify_segment(
	segment: string,
	dynamic_prefix: string,
	splat_rune: string,
): SegType {
	if (segment === "") {
		return seg_types.index;
	}
	if (segment.length === 1 && segment === splat_rune) {
		return seg_types.splat;
	}
	if (segment.length > 0 && segment[0] === dynamic_prefix) {
		return seg_types.dynamic;
	}
	return seg_types.static;
}

function best_match_rank(segment_type: string): number {
	if (segment_type === seg_types.dynamic) {
		return score_dynamic;
	}
	if (segment_type === seg_types.splat) {
		return 0;
	}
	return score_static;
}

function candidate_better_than(
	candidate: BestMatch,
	current: BestMatch,
): boolean {
	if (candidate.score !== current.score) {
		return candidate.score > current.score;
	}
	const candidate_segments = candidate.registeredPattern.normalizedSegments;
	const current_segments = current.registeredPattern.normalizedSegments;
	const len = Math.min(candidate_segments.length, current_segments.length);
	for (let i = 0; i < len; i++) {
		const left = best_match_rank(candidate_segments[i]!.segType);
		const right = best_match_rank(current_segments[i]!.segType);
		if (left !== right) {
			return left > right;
		}
	}
	const candidate_last = candidate.registeredPattern.lastSegType;
	const current_last = current.registeredPattern.lastSegType;
	if (candidate_last !== current_last) {
		if (candidate_last === seg_types.splat) {
			return false;
		}
		if (current_last === seg_types.splat) {
			return true;
		}
	}
	if (candidate_segments.length !== current_segments.length) {
		return candidate_segments.length > current_segments.length;
	}
	return false;
}

function is_static(segments: Segment[]): boolean {
	for (const segment of segments) {
		if (
			segment.segType === seg_types.splat ||
			segment.segType === seg_types.dynamic
		) {
			return false;
		}
	}
	return true;
}

function shape_key(pattern: RegisteredPattern): string {
	let out = "";
	for (let i = 0; i < pattern.normalizedSegments.length; i++) {
		const segment = pattern.normalizedSegments[i]!;
		if (i > 0) {
			out += "/";
		}
		switch (segment.segType) {
			case seg_types.dynamic: {
				out += "D:";
				break;
			}
			case seg_types.splat: {
				out += "P:";
				break;
			}
			case seg_types.index: {
				out += "I:";
				break;
			}
			default: {
				out += "S:" + segment.normalizedVal;
				break;
			}
		}
	}
	return out;
}

/////////////////////////////////////////////////////////////////////
/////// Segment Node Tree
/////////////////////////////////////////////////////////////////////

function create_segment_node(): SegmentNode {
	return {
		pattern: "",
		nodeType: node_static,
		children: null,
		dynChildren: [],
		paramName: "",
		finalScore: 0,
	};
}

function find_or_create_child(node: SegmentNode, segment: string): SegmentNode {
	// Empty-string segments (index) go into static children map.
	if (segment.length === 0) {
		if (node.children === null) {
			node.children = new Map<string, SegmentNode>();
		}
		let child = node.children.get("");
		if (child) {
			return child;
		}
		child = create_segment_node();
		child.nodeType = node_static;
		node.children.set("", child);
		return child;
	}

	if (segment[0] === ":") {
		for (const child of node.dynChildren) {
			if (
				child.nodeType === node_dynamic &&
				child.paramName === segment.substring(1)
			) {
				return child;
			}
		}
		const child = create_segment_node();
		child.nodeType = node_dynamic;
		child.paramName = segment.substring(1);
		node.dynChildren.push(child);
		return child;
	}

	if (segment === "*") {
		for (const child of node.dynChildren) {
			if (child.nodeType === node_splat) {
				return child;
			}
		}
		const child = create_segment_node();
		child.nodeType = node_splat;
		node.dynChildren.push(child);
		return child;
	}

	if (node.children === null) {
		node.children = new Map<string, SegmentNode>();
	}
	let child = node.children.get(segment);
	if (child) {
		return child;
	}
	child = create_segment_node();
	child.nodeType = node_static;
	node.children.set(segment, child);
	return child;
}

/////////////////////////////////////////////////////////////////////
/////// DFS: Best Match
/////////////////////////////////////////////////////////////////////

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
				node.nodeType === node_splat ||
				at_normal_end
			) {
				const candidate = {
					registeredPattern: rp,
					params: {},
					splatValues: [],
					score,
				};
				if (
					state.best === null ||
					candidate_better_than(candidate, state.best)
				) {
					state.best = candidate;
				}
			}
		}
	}

	if (depth >= segments.length) {
		return;
	}

	if (node.children !== null) {
		const child = node.children.get(segments[depth]!);
		if (child) {
			dfs_best(
				registry,
				child,
				segments,
				depth + 1,
				score + score_static,
				state,
				check_trailing,
			);

			if (
				state.best &&
				depth + 1 === segments.length &&
				child.pattern !== ""
			) {
				return;
			}
		}
	}

	for (const child of node.dynChildren) {
		switch (child.nodeType) {
			case node_dynamic: {
				if (segments[depth] !== "") {
					dfs_best(
						registry,
						child,
						segments,
						depth + 1,
						score + score_dynamic,
						state,
						check_trailing,
					);
				}
				break;
			}
			case node_splat: {
				if (child.pattern.length > 0) {
					const rp = registry.dynamicPatterns.get(child.pattern);
					if (rp) {
						const candidate = {
							registeredPattern: rp,
							params: {},
							splatValues: [],
							score,
						};
						if (
							state.best === null ||
							candidate_better_than(candidate, state.best)
						) {
							state.best = candidate;
						}
					}
				}
				break;
			}
		}
	}
}

/////////////////////////////////////////////////////////////////////
/////// DFS: Nested Matches
/////////////////////////////////////////////////////////////////////

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

			if (node.nodeType === node_splat && depth < segments.length) {
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
			case node_dynamic: {
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
			case node_splat: {
				dfs_nested(registry, child, segments, depth, params, matches);
				break;
			}
		}
	}
}

/////////////////////////////////////////////////////////////////////
/////// Flatten And Sort Nested Matches
/////////////////////////////////////////////////////////////////////

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
