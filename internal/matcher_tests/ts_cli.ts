import {
	type BestMatch,
	type Match,
	type Params,
	type PatternRegistry,
	type RegisteredPattern,
	type RegistrationOptions,
	createPatternRegistry,
	ensureLeadingAndTrailingSlash,
	ensureLeadingSlash,
	ensureTrailingSlash,
	findBestMatch,
	findNestedMatches,
	hasLeadingSlash,
	hasTrailingSlash,
	joinPatterns,
	normalizePattern,
	parseSegments,
	registerPattern,
	stripLeadingSlash,
	stripTrailingSlash,
} from "vorma/kit/matcher";
import { R, type Result } from "vorma/kit/result";

const op_parse_segments = "parseSegments";
const op_matcher_config = "matcherConfig";
const op_normalize_pattern = "normalizePattern";
const op_register_pattern = "registerPattern";
const op_find_best_match = "findBestMatch";
const op_find_nested_matches = "findNestedMatches";
const op_path_helpers = "pathHelpers";
const op_join_patterns = "joinPatterns";

type Request = {
	operation?: unknown;
	options?: unknown;
	patterns?: unknown;
	pattern?: unknown;
	path?: unknown;
	suffix?: unknown;
};

function string_field(value: unknown): string {
	if (typeof value === "string") {
		return value;
	}
	return "";
}

function string_array_field(value: unknown): string[] {
	if (!Array.isArray(value)) {
		return [];
	}
	const out: string[] = [];
	for (const item of value) {
		if (typeof item === "string") {
			out.push(item);
		}
	}
	return out;
}

function snapshot_pattern(pattern: RegisteredPattern | null): unknown {
	if (pattern === null) {
		return null;
	}
	return {
		normalizedPattern: pattern.normalizedPattern,
		normalizedSegments: pattern.normalizedSegments,
		originalPattern: pattern.originalPattern,
	};
}

function register_patterns(
	registry: PatternRegistry,
	patterns: string[],
): Result<unknown> {
	let last: RegisteredPattern | null = null;
	for (const pattern of patterns) {
		const res = registerPattern(registry, pattern);
		if (!res.ok) {
			return R.err(res.err);
		}
		last = res.val;
	}
	return R.ok(snapshot_pattern(last));
}

function normalize_params(params: Params): Params | null {
	if (Object.keys(params).length === 0) {
		return null;
	}
	return params;
}

function normalize_splat(splat_values: string[]): string[] | null {
	if (splat_values.length === 0) {
		return null;
	}
	return splat_values;
}

function snapshot_match(match: BestMatch | Match) {
	return {
		params: normalize_params(match.params),
		registeredPattern: snapshot_pattern(match.registeredPattern),
		splatValues: normalize_splat(match.splatValues),
	};
}

function handle(req: Request): Result<unknown> {
	const operation = string_field(req.operation);
	const path = string_field(req.path);

	if (operation === op_parse_segments) {
		return R.ok(parseSegments(path));
	}

	if (operation === op_path_helpers) {
		return R.ok({
			ensureLeadingAndTrailingSlash: ensureLeadingAndTrailingSlash(path),
			ensureLeadingSlash: ensureLeadingSlash(path),
			ensureTrailingSlash: ensureTrailingSlash(path),
			hasLeadingSlash: hasLeadingSlash(path),
			hasTrailingSlash: hasTrailingSlash(path),
			stripLeadingSlash: stripLeadingSlash(path),
			stripTrailingSlash: stripTrailingSlash(path),
		});
	}

	const registration_options: RegistrationOptions = {};
	if (
		req.options !== null &&
		typeof req.options === "object" &&
		!Array.isArray(req.options)
	) {
		const opts = req.options as Record<string, unknown>;
		const dynamic_prefix = string_field(opts.dynamicParamPrefix);
		if (dynamic_prefix !== "") {
			registration_options.dynamicParamPrefixRune = dynamic_prefix;
		}
		const explicit_index = string_field(
			opts.explicitIndexSegmentIdentifier,
		);
		if (explicit_index !== "") {
			registration_options.explicitIndexSegment = explicit_index;
		}
		const splat = string_field(opts.splatSegmentIdentifier);
		if (splat !== "") {
			registration_options.splatSegmentRune = splat;
		}
	}

	const registry_res = createPatternRegistry(registration_options);
	if (!registry_res.ok) {
		return R.err(registry_res.err);
	}
	const registry = registry_res.val;

	switch (operation) {
		case op_matcher_config:
			return R.ok({
				dynamicParamPrefix: registry.config.dynamicParamPrefixRune,
				explicitIndexSegmentIdentifier:
					registry.config.explicitIndexSegment,
				splatSegmentIdentifier: registry.config.splatSegmentRune,
			});
		case op_normalize_pattern: {
			const res = normalizePattern(
				string_field(req.pattern),
				registry.config,
			);
			if (!res.ok) {
				return R.err(res.err);
			}
			return R.ok(snapshot_pattern(res.val));
		}
		case op_join_patterns: {
			const res = normalizePattern(
				string_field(req.pattern),
				registry.config,
			);
			if (!res.ok) {
				return R.err(res.err);
			}
			return R.ok(joinPatterns(res.val, string_field(req.suffix)));
		}
		case op_register_pattern: {
			const patterns = string_array_field(req.patterns);
			const pattern = string_field(req.pattern);
			if (patterns.length > 0 || pattern === "") {
				return register_patterns(registry, patterns);
			}
			return register_patterns(registry, [pattern]);
		}
		case op_find_best_match: {
			const register_response = register_patterns(
				registry,
				string_array_field(req.patterns),
			);
			if (!register_response.ok) {
				return register_response;
			}
			const match = findBestMatch(registry, path);
			if (match === null) {
				return R.ok({ found: false });
			}
			return R.ok({ found: true, ...snapshot_match(match) });
		}
		case op_find_nested_matches: {
			const register_response = register_patterns(
				registry,
				string_array_field(req.patterns),
			);
			if (!register_response.ok) {
				return register_response;
			}
			const results = findNestedMatches(registry, path);
			if (results === null) {
				return R.ok({ found: false });
			}
			return R.ok({
				found: true,
				matches: results.matches.map((match) => {
					return snapshot_match(match);
				}),
				params: normalize_params(results.params),
				splatValues: normalize_splat(results.splatValues),
			});
		}
		default:
			return R.err(`unknown matcher conformance operation ${operation}`);
	}
}

function process_request(input: string): void {
	let response: Result<unknown>;
	try {
		response = handle(JSON.parse(input) as Request);
	} catch (caught) {
		response = R.err(
			caught instanceof Error ? caught.message : String(caught),
		);
	}
	process.stdout.write(JSON.stringify(response) + "\n");
}

let input = "";
process.stdin.setEncoding("utf8");
for await (const chunk of process.stdin) {
	input += chunk;
	const lines = input.split("\n");
	input = lines.pop() ?? "";
	for (const line of lines) {
		if (line.trim() !== "") {
			process_request(line);
		}
	}
}
if (input.trim() !== "") {
	process_request(input);
}
