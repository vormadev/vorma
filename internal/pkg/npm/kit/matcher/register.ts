import { parseSegments } from "vorma/kit/matcher/utils";

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

export type SegmentNode = {
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

/////////////////////////////////////////////////////////////////////
/////// CONSTANTS
/////////////////////////////////////////////////////////////////////

export const NODE_STATIC = 0;
export const NODE_DYNAMIC = 1;
export const NODE_SPLAT = 2;
export const SCORE_STATIC_MATCH = 2;
export const SCORE_DYNAMIC = 1;

export const SEG_TYPES = {
	splat: "splat" as SegType,
	static: "static" as SegType,
	dynamic: "dynamic" as SegType,
	index: "index" as SegType,
};

/////////////////////////////////////////////////////////////////////
/////// REGISTRATION
/////////////////////////////////////////////////////////////////////

function create_segment_node(): SegmentNode {
	return {
		pattern: "",
		nodeType: NODE_STATIC,
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
		child.nodeType = NODE_STATIC;
		node.children.set("", child);
		return child;
	}

	if (segment[0] === ":") {
		for (const child of node.dynChildren) {
			if (
				child.nodeType === NODE_DYNAMIC &&
				child.paramName === segment.substring(1)
			) {
				return child;
			}
		}
		const child = create_segment_node();
		child.nodeType = NODE_DYNAMIC;
		child.paramName = segment.substring(1);
		node.dynChildren.push(child);
		return child;
	}

	if (segment === "*") {
		for (const child of node.dynChildren) {
			if (child.nodeType === NODE_SPLAT) {
				return child;
			}
		}
		const child = create_segment_node();
		child.nodeType = NODE_SPLAT;
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
	child.nodeType = NODE_STATIC;
	node.children.set(segment, child);
	return child;
}

function classify_segment(
	segment: string,
	dynamic_prefix: string,
	splat_rune: string,
): SegType {
	if (segment === "") {
		return SEG_TYPES.index;
	}
	if (segment.length === 1 && segment === splat_rune) {
		return SEG_TYPES.splat;
	}
	if (segment.length > 0 && segment[0] === dynamic_prefix) {
		return SEG_TYPES.dynamic;
	}
	return SEG_TYPES.static;
}

function is_static(segments: Segment[]): boolean {
	for (const segment of segments) {
		if (
			segment.segType === SEG_TYPES.splat ||
			segment.segType === SEG_TYPES.dynamic
		) {
			return false;
		}
	}
	return true;
}

import { R, type Result } from "vorma/kit/result";

function normalize_pattern(
	original: string,
	config: PatternRegistry["config"],
): Result<RegisteredPattern> {
	let normalized = original;

	if (config.usingExplicitIndexSegment) {
		if (normalized.endsWith("/")) {
			if (normalized !== "/") {
				return R.err(`bad trailing slash: ${original}`);
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

		if (seg_type === SEG_TYPES.dynamic) {
			num_dynamic++;
			val = ":" + seg.substring(1);
		}
		if (seg_type === SEG_TYPES.splat) {
			val = "*";
		}

		segments.push({ normalizedVal: val, segType: seg_type });
	}

	const seg_len = segments.length;
	const last_type: SegType =
		seg_len > 0 ? segments[seg_len - 1]!.segType : SEG_TYPES.static;

	let final_pattern = "/";
	for (let i = 0; i < segments.length; i++) {
		final_pattern += segments[i]!.normalizedVal;
		if (i < seg_len - 1) {
			final_pattern += "/";
		}
	}

	if (final_pattern.endsWith("/") && last_type !== SEG_TYPES.index) {
		final_pattern = final_pattern.replace(/\/+$/, "");
	}

	return R.ok({
		originalPattern: original,
		normalizedPattern: final_pattern,
		normalizedSegments: segments,
		lastSegType: last_type,
		lastSegIsNonRootSplat: last_type === SEG_TYPES.splat && seg_len > 1,
		lastSegIsIndex: last_type === SEG_TYPES.index,
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
		return R.err("explicit index segment cannot contain /");
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
	const rp_res = normalize_pattern(original, registry.config);

	if (!rp_res.ok) {
		return R.err(rp_res.err);
	}

	const rp = rp_res.val;

	// Check for collision or idempotent re-registration.
	const existing =
		registry.staticPatterns.get(rp.normalizedPattern) ||
		registry.dynamicPatterns.get(rp.normalizedPattern);

	if (existing) {
		if (existing.originalPattern === original) {
			return R.ok(existing);
		}
		return R.err(
			`normalized pattern collision: "${original}" and "${existing.originalPattern}" both normalize to "${rp.normalizedPattern}"`,
		);
	}

	if (is_static(rp.normalizedSegments)) {
		registry.staticPatterns.set(rp.normalizedPattern, rp);
		return R.ok(rp);
	}

	registry.dynamicPatterns.set(rp.normalizedPattern, rp);

	let current = registry.rootNode;
	let node_score = 0;

	for (let i = 0; i < rp.normalizedSegments.length; i++) {
		const segment = rp.normalizedSegments[i]!;
		const child = find_or_create_child(current, segment.normalizedVal);

		if (segment.segType === SEG_TYPES.dynamic) {
			node_score += SCORE_DYNAMIC;
		} else if (segment.segType !== SEG_TYPES.splat) {
			node_score += SCORE_STATIC_MATCH;
		}

		if (i === rp.normalizedSegments.length - 1) {
			child.finalScore = node_score;
			child.pattern = rp.normalizedPattern;
		}

		current = child;
	}

	return R.ok(rp);
}
