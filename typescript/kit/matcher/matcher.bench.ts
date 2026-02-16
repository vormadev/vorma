// benchmarks.test.ts
import { bench, describe } from "vitest";
import { findBestMatch } from "./find_best_match.ts";
import { findNestedMatches } from "./find_nested_matches.ts";
import { parseSegments } from "./parse_segments.ts";
import {
	createPatternRegistry,
	registerPattern,
	type PatternRegistry,
} from "./register.ts";

// Nested patterns for the nested benchmarks
const NestedPatterns = [
	"/_index",
	"/articles/_index",
	"/articles/test/articles/_index",
	"/bear/_index",
	"/dashboard/_index",
	"/dashboard/customers/_index",
	"/dashboard/customers/:customer_id/_index",
	"/dashboard/customers/:customer_id/orders/_index",
	"/dynamic-index/:pagename/_index",
	"/lion/_index",
	"/tiger/_index",
	"/tiger/:tiger_id/_index",
	"/",
	"/*",
	"/bear",
	"/bear/:bear_id",
	"/bear/:bear_id/*",
	"/dashboard",
	"/dashboard/*",
	"/dashboard/customers",
	"/dashboard/customers/:customer_id",
	"/dashboard/customers/:customer_id/orders",
	"/dashboard/customers/:customer_id/orders/:order_id",
	"/dynamic-index/index",
	"/lion",
	"/lion/*",
	"/tiger",
	"/tiger/:tiger_id",
	"/tiger/:tiger_id/:tiger_cub_id",
	"/tiger/:tiger_id/*",
	"/a/b/:",
	"/c/d/e/:_",
	"/f/g/h/i/:/:",
	"/j/k/l/m/n/:_/:_",
];

// Setup functions - FIXED to avoid duplicates
function setupNonNestedRegistryForBenchmark(scale: string): PatternRegistry {
	const registry = createPatternRegistry();

	switch (scale) {
		case "small":
			// Basic patterns for simple tests (8 patterns like Go)
			registerPattern(registry, "/");
			registerPattern(registry, "/users");
			registerPattern(registry, "/users/:id");
			registerPattern(registry, "/users/:id/profile");
			registerPattern(registry, "/api/v1/users");
			registerPattern(registry, "/api/:version/users");
			registerPattern(registry, "/api/v1/users/:id");
			registerPattern(registry, "/files/*");
			break;

		case "medium":
			// Create ~4000 UNIQUE patterns
			// Mix of static, dynamic, and splat patterns
			for (let i = 0; i < 1000; i++) {
				registerPattern(registry, `/api/v1/users${i}`);
				registerPattern(registry, `/api/v2/users${i}/:id`);
				registerPattern(
					registry,
					`/api/v3/users${i}/:id/posts/:post_id`,
				);
				registerPattern(registry, `/files/bucket${i}/*`);
			}
			break;

		case "large":
			// Create ~60000 UNIQUE patterns
			for (let i = 0; i < 10000; i++) {
				// Static patterns
				registerPattern(registry, `/api/v1/users${i}`);
				registerPattern(registry, `/api/v2/products${i}`);
				registerPattern(registry, `/docs/section${i}`);

				// Dynamic patterns
				registerPattern(
					registry,
					`/api/v3/users${i}/:id/posts/:post_id`,
				);
				registerPattern(registry, `/api/v4/products${i}/:category/:id`);

				// Splat patterns
				registerPattern(registry, `/files/bucket${i}/*`);
			}
			break;
	}

	return registry;
}

function setupNestedRegistryForBenchmark(): PatternRegistry {
	const registry = createPatternRegistry();
	for (const pattern of NestedPatterns) {
		registerPattern(registry, pattern);
	}
	return registry;
}

// Generate test paths
function generateNonNestedPathsForBenchmark(scale: string): string[] {
	switch (scale) {
		case "small":
			return [
				"/",
				"/users",
				"/users/123",
				"/users/123/profile",
				"/api/v1/users",
				"/api/v2/users",
				"/files/document.pdf",
			];
		case "medium":
		case "large":
			return [
				"/api/v1/users500", // Static hit
				"/api/v2/users500/123", // Dynamic hit
				"/api/v3/users500/123/posts/456", // Deep dynamic
				"/files/bucket500/path/to/file.txt", // Splat
				"/api/v1/users999", // Different static
				"/api/v2/products500", // Might not exist in medium
				"/docs/section500", // Another static
			];
	}
	return ["/"];
}

function generateNestedPathsForBenchmark(): string[] {
	return [
		"/",
		"/dashboard",
		"/dashboard/customers",
		"/dashboard/customers/123",
		"/dashboard/customers/123/orders",
		"/dashboard/customers/123/orders/456",
		"/tiger",
		"/tiger/123",
		"/tiger/123/456",
		"/tiger/123/456/789",
		"/bear/123/456/789",
		"/articles/test/articles",
		"/does-not-exist",
		"/dashboard/unknown/path",
	];
}

describe("FindBestMatch Benchmarks", () => {
	// Pre-setup registries
	const mediumRegistry = setupNonNestedRegistryForBenchmark("medium");
	const smallRegistry = setupNonNestedRegistryForBenchmark("small");
	const largeRegistry = setupNonNestedRegistryForBenchmark("large");

	// Simple scenarios - matching Go's test cases
	const scenarios = [
		{
			name: "StaticPattern",
			path: "/api/v1/users",
			registry: mediumRegistry,
		},
		{
			name: "DynamicPattern",
			path: "/api/v2/users100/123/posts/456",
			registry: mediumRegistry,
		},
		{
			name: "SplatPattern",
			path: "/files/bucket100/deep/path/file.txt",
			registry: mediumRegistry,
		},
	];

	describe("FindBestMatchSimple", () => {
		for (const s of scenarios) {
			bench(
				s.name,
				() => {
					findBestMatch(s.registry, s.path);
				},
				{ time: 1000 },
			);
		}
	});

	describe("FindBestMatchAtScale", () => {
		bench(
			"Scale_small",
			() => {
				const paths = generateNonNestedPathsForBenchmark("small");
				findBestMatch(
					smallRegistry,
					paths[Math.floor(Math.random() * paths.length)]!,
				);
			},
			{ time: 1000 },
		);

		bench(
			"Scale_medium",
			() => {
				const paths = generateNonNestedPathsForBenchmark("medium");
				findBestMatch(
					mediumRegistry,
					paths[Math.floor(Math.random() * paths.length)]!,
				);
			},
			{ time: 1000 },
		);

		bench(
			"Scale_large",
			() => {
				const paths = generateNonNestedPathsForBenchmark("large");
				findBestMatch(
					largeRegistry,
					paths[Math.floor(Math.random() * paths.length)]!,
				);
			},
			{ time: 1000 },
		);

		bench(
			"WorstCase_DeepNested",
			() => {
				// This path exists in large registry
				findBestMatch(largeRegistry, "/api/v3/users9999/999/posts/999");
			},
			{ time: 1000 },
		);
	});
});

describe("FindNestedMatches Benchmarks", () => {
	const nestedRegistry = setupNestedRegistryForBenchmark();

	const cases = [
		{
			name: "StaticPatterns",
			paths: [
				"/",
				"/dashboard",
				"/dashboard/customers",
				"/tiger",
				"/lion",
			],
		},
		{
			name: "DynamicPatterns",
			paths: [
				"/dashboard/customers/123",
				"/dashboard/customers/456/orders",
				"/tiger/123",
				"/bear/123",
			],
		},
		{
			name: "DeepNestedPatterns",
			paths: [
				"/dashboard/customers/123/orders/456",
				"/tiger/123/456/789",
				"/bear/123/456/789",
				"/articles/test/articles",
			],
		},
		{
			name: "SplatPatterns",
			paths: [
				"/does-not-exist",
				"/dashboard/unknown/path",
				"/tiger/123/456/789/extra",
				"/bear/123/456/789/extra",
			],
		},
		{
			name: "MixedPatterns",
			paths: generateNestedPathsForBenchmark(),
		},
	];

	for (const tc of cases) {
		bench(
			`FindNestedMatches/${tc.name}`,
			() => {
				const path =
					tc.paths[Math.floor(Math.random() * tc.paths.length)];
				findNestedMatches(nestedRegistry, path!);
			},
			{ time: 1000 },
		);
	}
});

describe("ParseSegments Benchmarks", () => {
	const paths = [
		"/",
		"/api/v1/users",
		"/api/v1/users/123/posts/456/comments",
		"/files/documents/reports/quarterly/q3-2023.pdf",
	];

	bench(
		"ParseSegments/ParseSegments",
		() => {
			const path = paths[Math.floor(Math.random() * paths.length)];
			parseSegments(path!);
		},
		{ time: 1000 },
	);
});
