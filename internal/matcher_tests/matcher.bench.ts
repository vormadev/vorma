import { bench, describe } from "vitest";
import {
	createPatternRegistry,
	findBestMatch,
	findNestedMatches,
	parseSegments,
	registerPattern,
	type PatternRegistry,
} from "vorma/kit/matcher";

const nested_patterns = [
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

function setup_non_nested_registry_for_benchmark(
	scale: string,
): PatternRegistry {
	const registry_res = createPatternRegistry();
	if (!registry_res.ok) {
		console.error(`createPatternRegistry() error: ${registry_res.err}`);
		process.exit(1);
	}
	const registry = registry_res.val;

	switch (scale) {
		case "small":
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
			for (let i = 0; i < 10000; i++) {
				registerPattern(registry, `/api/v1/users${i}`);
				registerPattern(registry, `/api/v2/products${i}`);
				registerPattern(registry, `/docs/section${i}`);

				registerPattern(
					registry,
					`/api/v3/users${i}/:id/posts/:post_id`,
				);
				registerPattern(registry, `/api/v4/products${i}/:category/:id`);

				registerPattern(registry, `/files/bucket${i}/*`);
			}
			break;
	}

	return registry;
}

function setup_nested_registry_for_benchmark(): PatternRegistry {
	const registry_res = createPatternRegistry();
	if (!registry_res.ok) {
		console.error(`createPatternRegistry() error: ${registry_res.err}`);
		process.exit(1);
	}
	const registry = registry_res.val;
	for (const pattern of nested_patterns) {
		registerPattern(registry, pattern);
	}
	return registry;
}

function generate_non_nested_paths_for_benchmark(scale: string): string[] {
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
				"/api/v1/users500",
				"/api/v2/users500/123",
				"/api/v3/users500/123/posts/456",
				"/files/bucket500/path/to/file.txt",
				"/api/v1/users999",
				"/api/v2/products500",
				"/docs/section500",
			];
	}
	return ["/"];
}

function generate_nested_paths_for_benchmark(): string[] {
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
	const medium_registry = setup_non_nested_registry_for_benchmark("medium");
	const small_registry = setup_non_nested_registry_for_benchmark("small");
	const large_registry = setup_non_nested_registry_for_benchmark("large");

	const scenarios = [
		{
			name: "StaticPattern",
			path: "/api/v1/users",
			registry: medium_registry,
		},
		{
			name: "DynamicPattern",
			path: "/api/v2/users100/123/posts/456",
			registry: medium_registry,
		},
		{
			name: "SplatPattern",
			path: "/files/bucket100/deep/path/file.txt",
			registry: medium_registry,
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
				const paths = generate_non_nested_paths_for_benchmark("small");
				findBestMatch(
					small_registry,
					paths[Math.floor(Math.random() * paths.length)]!,
				);
			},
			{ time: 1000 },
		);

		bench(
			"Scale_medium",
			() => {
				const paths = generate_non_nested_paths_for_benchmark("medium");
				findBestMatch(
					medium_registry,
					paths[Math.floor(Math.random() * paths.length)]!,
				);
			},
			{ time: 1000 },
		);

		bench(
			"Scale_large",
			() => {
				const paths = generate_non_nested_paths_for_benchmark("large");
				findBestMatch(
					large_registry,
					paths[Math.floor(Math.random() * paths.length)]!,
				);
			},
			{ time: 1000 },
		);

		bench(
			"WorstCase_DeepNested",
			() => {
				findBestMatch(
					large_registry,
					"/api/v3/users9999/999/posts/999",
				);
			},
			{ time: 1000 },
		);
	});
});

describe("FindNestedMatches Benchmarks", () => {
	const nested_registry = setup_nested_registry_for_benchmark();

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
			paths: generate_nested_paths_for_benchmark(),
		},
	];

	for (const tc of cases) {
		bench(
			`FindNestedMatches/${tc.name}`,
			() => {
				const path =
					tc.paths[Math.floor(Math.random() * tc.paths.length)];
				findNestedMatches(nested_registry, path!);
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
