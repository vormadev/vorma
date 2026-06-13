//! Matcher benchmarks, ported from the Go reference suite so numbers
//! stay comparable across implementations (the Go baselines live in the
//! old repo's results.bench.txt).

use std::hint::black_box;

// This crate does not link the vorma server crate, so the bench
// binary declares the allocator vorma apps run by default.
#[global_allocator]
static GLOBAL_ALLOCATOR: mimalloc::MiMalloc = mimalloc::MiMalloc;

use vorma_bench::Bench;
use vorma_matcher::{FlatMatcher, MatcherBuilder, NestedMatcher, Options, parse_segments};

// The nested fixture from the test suite, registered raw under default
// options exactly as the Go benchmark registered its list. The three
// unnamed-param entries from Go are spelled with the named params the
// tightened grammar requires.
const NESTED_PATTERNS: &[&str] = &[
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
	"/a/b/:value",
	"/c/d/e/:_",
	"/f/g/h/i/:first/:second",
	"/j/k/l/m/n/:left/:right",
];

fn nested_matcher() -> NestedMatcher {
	let mut builder = MatcherBuilder::new(Options::default()).unwrap();
	for pattern in NESTED_PATTERNS {
		builder.register_pattern(pattern).unwrap();
	}
	builder.finish_nested()
}

fn best_match_matcher(scale: &str) -> FlatMatcher {
	let mut builder = MatcherBuilder::new(Options::default()).unwrap();
	match scale {
		"small" => {
			for pattern in [
				"/",
				"/users",
				"/users/:id",
				"/users/:id/profile",
				"/api/v1/users",
				"/api/:version/users",
				"/api/v1/users/:id",
				"/files/*",
			] {
				builder.register_pattern(pattern).unwrap();
			}
		}
		"medium" => {
			for i in 0..1_000usize {
				builder
					.register_pattern(&format!("/api/v{}/users", i % 5))
					.unwrap();
				builder
					.register_pattern(&format!("/api/v{}/users/:id", i % 5))
					.unwrap();
				builder
					.register_pattern(&format!("/api/v{}/users/:id/posts/:post_id", i % 5))
					.unwrap();
				builder
					.register_pattern(&format!("/files/bucket{}/*", i % 10))
					.unwrap();
			}
		}
		"large" => {
			for i in 0..10_000usize {
				builder
					.register_pattern(&format!("/api/v{}/users", i % 10))
					.unwrap();
				builder
					.register_pattern(&format!("/api/v{}/products", i % 10))
					.unwrap();
				builder
					.register_pattern(&format!("/docs/section{}", i % 100))
					.unwrap();
				builder
					.register_pattern(&format!("/api/v{}/users/:id/posts/:post_id", i % 10))
					.unwrap();
				builder
					.register_pattern(&format!("/api/v{}/products/:category/:id", i % 10))
					.unwrap();
				builder
					.register_pattern(&format!("/files/bucket{}/*", i % 20))
					.unwrap();
			}
		}
		other => panic!("unknown scale {other:?}"),
	}
	builder.finish_flat()
}

fn best_match_paths(scale: &str) -> Vec<String> {
	match scale {
		"small" => [
			"/",
			"/users",
			"/users/123",
			"/users/123/profile",
			"/api/v1/users",
			"/api/v2/users",
			"/files/document.pdf",
		]
		.iter()
		.map(|p| (*p).to_owned())
		.collect(),
		_ => {
			let mut paths = Vec::with_capacity(1_000);
			for i in 0..400usize {
				paths.push(format!("/api/v{}/users", i % 5));
			}
			for i in 0..400usize {
				paths.push(format!("/api/v{}/users/{}/posts/{}", i % 5, i, i % 100));
			}
			for i in 0..200usize {
				paths.push(format!("/files/bucket{}/path/to/file{}.txt", i % 10, i));
			}
			paths
		}
	}
}

fn main() {
	let mut bench = Bench::new(env!("CARGO_PKG_NAME"));

	let parse_paths = [
		"/",
		"/api/v1/users",
		"/api/v1/users/123/posts/456/comments",
		"/files/documents/reports/quarterly/q3-2023.pdf",
	];
	let mut i = 0usize;
	bench.bench("parse_segments/rotating_paths", || {
		let segments = parse_segments(black_box(parse_paths[i % parse_paths.len()]));
		i += 1;
		black_box(segments);
	});

	let matcher = best_match_matcher("medium");
	for (name, path) in [
		("find_best_match_simple/static_pattern", "/api/v1/users"),
		(
			"find_best_match_simple/dynamic_pattern",
			"/api/v1/users/123/posts/456",
		),
		(
			"find_best_match_simple/splat_pattern",
			"/files/bucket1/deep/path/file.txt",
		),
	] {
		bench.bench(name, || {
			black_box(matcher.find_best_match(black_box(path)));
		});
	}

	for scale in ["small", "medium", "large"] {
		let matcher = best_match_matcher(scale);
		let paths = best_match_paths(scale);
		let mut i = 0usize;
		bench.bench(&format!("find_best_match_at_scale/{scale}"), || {
			let matched = matcher.find_best_match(black_box(&paths[i % paths.len()]));
			i += 1;
			black_box(matched);
		});
	}
	let matcher = best_match_matcher("large");
	bench.bench("find_best_match_at_scale/worst_case_deep_nested", || {
		black_box(matcher.find_best_match(black_box("/api/v9/users/999/posts/999")));
	});

	let matcher = nested_matcher();
	let cases: &[(&str, &[&str])] = &[
		(
			"find_nested_matches/static_patterns",
			&["/", "/dashboard", "/dashboard/customers", "/tiger", "/lion"],
		),
		(
			"find_nested_matches/dynamic_patterns",
			&[
				"/dashboard/customers/123",
				"/dashboard/customers/456/orders",
				"/tiger/123",
				"/bear/123",
			],
		),
		(
			"find_nested_matches/deep_nested_patterns",
			&[
				"/dashboard/customers/123/orders/456",
				"/tiger/123/456/789",
				"/bear/123/456/789",
				"/articles/test/articles",
			],
		),
		(
			"find_nested_matches/splat_patterns",
			&[
				"/does-not-exist",
				"/dashboard/unknown/path",
				"/tiger/123/456/789/extra",
				"/bear/123/456/789/extra",
			],
		),
		(
			"find_nested_matches/mixed_patterns",
			&[
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
			],
		),
	];
	for (name, paths) in cases {
		let mut i = 0usize;
		bench.bench(name, || {
			let matched = matcher.find_nested_matches(black_box(paths[i % paths.len()]));
			i += 1;
			black_box(matched);
		});
	}
}
