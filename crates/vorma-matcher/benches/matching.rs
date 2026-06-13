//! Matcher benchmarks, ported from the Go reference suite so numbers
//! stay comparable across implementations (the Go baselines live in the
//! old repo's results.bench.txt).

use std::hint::black_box;

use criterion::{Criterion, criterion_group, criterion_main};
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

fn bench_parse_segments(c: &mut Criterion) {
	let paths = [
		"/",
		"/api/v1/users",
		"/api/v1/users/123/posts/456/comments",
		"/files/documents/reports/quarterly/q3-2023.pdf",
	];
	let mut group = c.benchmark_group("parse_segments");
	group.bench_function("rotating_paths", |b| {
		let mut i = 0usize;
		b.iter(|| {
			let segments = parse_segments(black_box(paths[i % paths.len()]));
			i += 1;
			black_box(segments)
		});
	});
	group.finish();
}

fn bench_find_best_match_simple(c: &mut Criterion) {
	let matcher = best_match_matcher("medium");
	let mut group = c.benchmark_group("find_best_match_simple");
	for (name, path) in [
		("static_pattern", "/api/v1/users"),
		("dynamic_pattern", "/api/v1/users/123/posts/456"),
		("splat_pattern", "/files/bucket1/deep/path/file.txt"),
	] {
		group.bench_function(name, |b| {
			b.iter(|| black_box(matcher.find_best_match(black_box(path))));
		});
	}
	group.finish();
}

fn bench_find_best_match_at_scale(c: &mut Criterion) {
	let mut group = c.benchmark_group("find_best_match_at_scale");
	for scale in ["small", "medium", "large"] {
		let matcher = best_match_matcher(scale);
		let paths = best_match_paths(scale);
		group.bench_function(scale, |b| {
			let mut i = 0usize;
			b.iter(|| {
				let matched = matcher.find_best_match(black_box(&paths[i % paths.len()]));
				i += 1;
				black_box(matched)
			});
		});
	}
	let matcher = best_match_matcher("large");
	group.bench_function("worst_case_deep_nested", |b| {
		b.iter(|| black_box(matcher.find_best_match(black_box("/api/v9/users/999/posts/999"))));
	});
	group.finish();
}

fn bench_find_nested_matches(c: &mut Criterion) {
	let matcher = nested_matcher();
	let cases: &[(&str, &[&str])] = &[
		(
			"static_patterns",
			&["/", "/dashboard", "/dashboard/customers", "/tiger", "/lion"],
		),
		(
			"dynamic_patterns",
			&[
				"/dashboard/customers/123",
				"/dashboard/customers/456/orders",
				"/tiger/123",
				"/bear/123",
			],
		),
		(
			"deep_nested_patterns",
			&[
				"/dashboard/customers/123/orders/456",
				"/tiger/123/456/789",
				"/bear/123/456/789",
				"/articles/test/articles",
			],
		),
		(
			"splat_patterns",
			&[
				"/does-not-exist",
				"/dashboard/unknown/path",
				"/tiger/123/456/789/extra",
				"/bear/123/456/789/extra",
			],
		),
		(
			"mixed_patterns",
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
	let mut group = c.benchmark_group("find_nested_matches");
	for (name, paths) in cases {
		group.bench_function(*name, |b| {
			let mut i = 0usize;
			b.iter(|| {
				let matched = matcher.find_nested_matches(black_box(paths[i % paths.len()]));
				i += 1;
				black_box(matched)
			});
		});
	}
	group.finish();
}

criterion_group!(
	benches,
	bench_parse_segments,
	bench_find_best_match_simple,
	bench_find_best_match_at_scale,
	bench_find_nested_matches
);
criterion_main!(benches);
