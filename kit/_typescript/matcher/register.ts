import { parseSegments } from "./parse_segments.ts";

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

function createSegmentNode(): SegmentNode {
	return {
		pattern: "",
		nodeType: NODE_STATIC,
		children: null,
		dynChildren: [],
		paramName: "",
		finalScore: 0,
	};
}

function findOrCreateChild(node: SegmentNode, segment: string): SegmentNode {
	if (segment === "*" || (segment.length > 0 && segment[0] === ":")) {
		for (const child of node.dynChildren) {
			if (child.paramName === segment.substring(1)) return child;
		}
		return addDynamicChild(node, segment);
	}

	if (node.children === null) node.children = new Map<string, SegmentNode>();

	let child = node.children.get(segment);
	if (child) return child;

	child = createSegmentNode();
	child.nodeType = NODE_STATIC;
	node.children.set(segment, child);
	return child;
}

function addDynamicChild(node: SegmentNode, segment: string): SegmentNode {
	const child = createSegmentNode();
	if (segment === "*") {
		child.nodeType = NODE_SPLAT;
	} else {
		child.nodeType = NODE_DYNAMIC;
		child.paramName = segment.substring(1);
	}
	node.dynChildren.push(child);
	return child;
}

function getSegmentType(props: {
	segment: string;
	dynamicPrefix: string;
	splatRune: string;
}): SegType {
	const { segment, dynamicPrefix, splatRune } = props;
	if (segment === "") return SEG_TYPES.index;
	if (segment.length === 1 && segment === splatRune) return SEG_TYPES.splat;
	if (segment.length > 0 && segment[0] === dynamicPrefix)
		return SEG_TYPES.dynamic;
	return SEG_TYPES.static;
}

function isStatic(segments: Segment[]): boolean {
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

function normalizePattern(
	originalPattern: string,
	config: PatternRegistry["config"],
): RegisteredPattern {
	let normalizedPattern = originalPattern;

	if (config.usingExplicitIndexSegment) {
		if (normalizedPattern.endsWith("/")) {
			if (normalizedPattern !== "/") {
				throw new Error(`bad trailing slash: ${originalPattern}`);
			}
			normalizedPattern = normalizedPattern.replace(/\/+$/, "");
		}
		if (normalizedPattern.endsWith(config.slashIndexSegment)) {
			normalizedPattern = normalizedPattern.slice(
				0,
				-config.explicitIndexSegment.length,
			);
		}
	}

	const rawSegments = parseSegments(normalizedPattern);
	const segments: Segment[] = [];
	let numberOfDynamicParamSegs = 0;

	for (const seg of rawSegments) {
		let normalizedVal = seg;
		const segType = getSegmentType({
			segment: seg,
			dynamicPrefix: config.dynamicParamPrefixRune,
			splatRune: config.splatSegmentRune,
		});

		if (segType === SEG_TYPES.dynamic) {
			numberOfDynamicParamSegs++;
			normalizedVal = ":" + seg.substring(1);
		}
		if (segType === SEG_TYPES.splat) {
			normalizedVal = "*";
		}

		segments.push({ normalizedVal, segType });
	}

	const segLen = segments.length;
	let lastType: SegType =
		segLen > 0 ? segments[segLen - 1]!.segType : SEG_TYPES.static;

	let finalNormalizedPattern = "/";
	for (let i = 0; i < segments.length; i++) {
		finalNormalizedPattern += segments[i]!.normalizedVal;
		if (i < segLen - 1) finalNormalizedPattern += "/";
	}

	if (finalNormalizedPattern.endsWith("/") && lastType !== SEG_TYPES.index) {
		finalNormalizedPattern = finalNormalizedPattern.replace(/\/+$/, "");
	}

	return {
		originalPattern,
		normalizedPattern: finalNormalizedPattern,
		normalizedSegments: segments,
		lastSegType: lastType,
		lastSegIsNonRootSplat: lastType === SEG_TYPES.splat && segLen > 1,
		lastSegIsIndex: lastType === SEG_TYPES.index,
		numberOfDynamicParamSegs,
	};
}

export function createPatternRegistry(
	opts?: RegistrationOptions,
): PatternRegistry {
	const config = {
		dynamicParamPrefixRune: opts?.dynamicParamPrefixRune ?? ":",
		splatSegmentRune: opts?.splatSegmentRune ?? "*",
		explicitIndexSegment: opts?.explicitIndexSegment ?? "",
		slashIndexSegment: "/" + (opts?.explicitIndexSegment ?? ""),
		usingExplicitIndexSegment: (opts?.explicitIndexSegment ?? "") !== "",
	};

	if (config.explicitIndexSegment.includes("/")) {
		throw new Error("explicit index segment cannot contain /");
	}

	return {
		staticPatterns: new Map(),
		dynamicPatterns: new Map(),
		rootNode: createSegmentNode(),
		config,
	};
}

export function registerPattern(
	registry: PatternRegistry,
	originalPattern: string,
): RegisteredPattern {
	const normalized = normalizePattern(originalPattern, registry.config);

	if (
		!(globalThis.window && globalThis.document) &&
		(registry.staticPatterns.has(normalized.normalizedPattern) ||
			registry.dynamicPatterns.has(normalized.normalizedPattern))
	) {
		console.warn(`already registered: ${originalPattern}`);
	}

	if (isStatic(normalized.normalizedSegments)) {
		registry.staticPatterns.set(normalized.normalizedPattern, normalized);
		return normalized;
	}

	registry.dynamicPatterns.set(normalized.normalizedPattern, normalized);

	let current = registry.rootNode;
	let nodeScore = 0;

	for (let i = 0; i < normalized.normalizedSegments.length; i++) {
		const segment = normalized.normalizedSegments[i]!;
		const child = findOrCreateChild(current, segment.normalizedVal);

		if (segment.segType === SEG_TYPES.dynamic) {
			nodeScore += SCORE_DYNAMIC;
		} else if (segment.segType !== SEG_TYPES.splat) {
			nodeScore += SCORE_STATIC_MATCH;
		}

		if (i === normalized.normalizedSegments.length - 1) {
			child.finalScore = nodeScore;
			child.pattern = normalized.normalizedPattern;
		}

		current = child;
	}

	return normalized;
}
