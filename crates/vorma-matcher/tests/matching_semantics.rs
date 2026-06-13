use std::cmp::Ordering;
use std::collections::{HashMap, HashSet};

use vorma_matcher::{MatcherBuilder, Options, Params};

const NOT_FOUND: &str = "NOT FOUND";

#[derive(Clone, Copy)]
struct PatternOptionShape<'a> {
	dynamic_param_prefix: char,
	splat_segment_identifier: char,
	explicit_index_segment_identifier: &'a str,
}

fn option_shape_for_rewrites(opts: &Options) -> PatternOptionShape<'_> {
	PatternOptionShape {
		dynamic_param_prefix: opts.dynamic_param_prefix,
		splat_segment_identifier: opts.splat_segment_identifier,
		explicit_index_segment_identifier: opts.explicit_index_segment_identifier.as_str(),
	}
}

fn rewrite_patterns(
	patterns: &[&str],
	source_index: &str,
	shape: PatternOptionShape,
) -> Vec<String> {
	patterns
		.iter()
		.map(|p| rewrite_single_pattern(p, source_index, shape))
		.collect()
}

fn rewrite_single_pattern(
	pattern: &str,
	source_index: &str,
	shape: PatternOptionShape<'_>,
) -> String {
	if pattern.is_empty() {
		return pattern.to_string();
	}
	let mut parts: Vec<String> = pattern.split('/').map(ToString::to_string).collect();
	for (i, part) in parts.iter_mut().enumerate() {
		if !source_index.is_empty() && part == source_index {
			if shape.explicit_index_segment_identifier.is_empty() {
				part.clear();
			} else {
				*part = shape.explicit_index_segment_identifier.to_string();
			}
			continue;
		}
		if source_index.is_empty()
			&& !shape.explicit_index_segment_identifier.is_empty()
			&& part.is_empty()
			&& i > 0
		{
			*part = shape.explicit_index_segment_identifier.to_string();
			continue;
		}
		if part.starts_with(':')
			&& shape.dynamic_param_prefix != '\0'
			&& shape.dynamic_param_prefix != ':'
		{
			*part = format!("{}{}", shape.dynamic_param_prefix, &part[1..]);
			continue;
		}
		if part == "*"
			&& shape.splat_segment_identifier != '\0'
			&& shape.splat_segment_identifier != '*'
		{
			*part = shape.splat_segment_identifier.to_string();
		}
	}
	parts.join("/")
}

fn params(entries: &[(&str, &str)]) -> Params {
	let mut out = Params::default();
	for (k, v) in entries {
		out.insert(k, *v);
	}
	out
}

fn str_vec(values: &[&str]) -> Vec<String> {
	values.iter().map(|v| (*v).to_string()).collect()
}

fn register_all(m: &mut MatcherBuilder, patterns: impl IntoIterator<Item = impl AsRef<str>>) {
	for pattern in patterns {
		m.register_pattern(pattern.as_ref())
			.unwrap_or_else(|err| panic!("RegisterPattern({:?}) error: {err}", pattern.as_ref()));
	}
}

struct MatchCase {
	name: &'static str,
	patterns: &'static [&'static str],
	path: &'static str,
	want_pattern: &'static str,
	want_params: Option<Params>,
	want_splat_values: &'static [&'static str],
}

fn best_match_cases() -> Vec<MatchCase> {
	vec![
		MatchCase {
			name: "home route -- should match empty-str",
			patterns: &[""],
			path: "/",
			want_pattern: "",
			want_params: None,
			want_splat_values: &[],
		},
		MatchCase {
			name: "home route -- idx should beat empty-str",
			patterns: &["", "/"],
			path: "/",
			want_pattern: "/",
			want_params: None,
			want_splat_values: &[],
		},
		MatchCase {
			name: "home route -- empty-str should beat root-splat",
			patterns: &["", "/*"],
			path: "/",
			want_pattern: "",
			want_params: None,
			want_splat_values: &[],
		},
		MatchCase {
			name: "home route -- idx should beat root-splat",
			patterns: &["/", "/*"],
			path: "/",
			want_pattern: "/",
			want_params: None,
			want_splat_values: &[],
		},
		MatchCase {
			name: "home route -- idx should win (empty-str, idx, root-splat registered)",
			patterns: &["", "/", "/*"],
			path: "/",
			want_pattern: "/",
			want_params: None,
			want_splat_values: &[],
		},
		MatchCase {
			name: "home route -- should match root-splat if no idx or empty-str",
			patterns: &["/*"],
			path: "/",
			want_pattern: "/*",
			want_params: None,
			want_splat_values: &[],
		},
		MatchCase {
			name: "trailing slash should not match following dynamic route",
			patterns: &["/users/:user"],
			path: "/users/",
			want_pattern: NOT_FOUND,
			want_params: None,
			want_splat_values: &[],
		},
		MatchCase {
			name: "exact match should win over following splat",
			patterns: &["/", "/users", "/users/*", "/posts"],
			path: "/users",
			want_pattern: "/users",
			want_params: None,
			want_splat_values: &[],
		},
		MatchCase {
			name: "exact match, with trailing slash, should win over catch-all",
			patterns: &["/", "/users", "/users/*", "/posts"],
			path: "/users/",
			want_pattern: "/users",
			want_params: None,
			want_splat_values: &[],
		},
		MatchCase {
			name: "with no trailing slash, should NOT match following catch-all",
			patterns: &["/", "/users/*", "/posts"],
			path: "/users",
			want_pattern: NOT_FOUND,
			want_params: None,
			want_splat_values: &[],
		},
		MatchCase {
			name: "a bare trailing slash does not feed a following catch-all",
			patterns: &["/", "/users/*", "/posts"],
			path: "/users/",
			want_pattern: NOT_FOUND,
			want_params: None,
			want_splat_values: &[],
		},
		MatchCase {
			name: "registered trailing slash -- exact match without trailing wins",
			patterns: &["/", "/users/", "/users", "/users/*", "/posts"],
			path: "/users",
			want_pattern: "/users",
			want_params: None,
			want_splat_values: &[],
		},
		MatchCase {
			name: "registered trailing slash -- trailing slash matches trailing pattern",
			patterns: &["/", "/users/", "/users", "/users/*", "/posts"],
			path: "/users/",
			want_pattern: "/users/",
			want_params: None,
			want_splat_values: &[],
		},
		MatchCase {
			name: "registered trailing slash -- no trailing should NOT match trailing pattern or catch-all",
			patterns: &["/", "/users/", "/users/*", "/posts"],
			path: "/users",
			want_pattern: NOT_FOUND,
			want_params: None,
			want_splat_values: &[],
		},
		MatchCase {
			name: "registered trailing slash -- trailing should match trailing pattern, not catch-all",
			patterns: &["/", "/users/", "/users/*", "/posts"],
			path: "/users/",
			want_pattern: "/users/",
			want_params: None,
			want_splat_values: &[],
		},
		MatchCase {
			name: "dynamic match should win over catch-all",
			patterns: &["/", "/:user", "/:user/*", "/posts"],
			path: "/bob",
			want_pattern: "/:user",
			want_params: Some(params(&[("user", "bob")])),
			want_splat_values: &[],
		},
		MatchCase {
			name: "dynamic match, with trailing slash, should win over catch-all",
			patterns: &["/", "/:user", "/:user/*", "/posts"],
			path: "/bob/",
			want_pattern: "/:user",
			want_params: Some(params(&[("user", "bob")])),
			want_splat_values: &[],
		},
		MatchCase {
			name: "dynamic - no trailing slash should NOT match following catch-all",
			patterns: &["/", "/:user/*", "/posts"],
			path: "/bob",
			want_pattern: NOT_FOUND,
			want_params: None,
			want_splat_values: &[],
		},
		MatchCase {
			name: "dynamic - a bare trailing slash does not feed a following catch-all",
			patterns: &["/", "/:user/*", "/posts"],
			path: "/bob/",
			want_pattern: NOT_FOUND,
			want_params: None,
			want_splat_values: &[],
		},
		MatchCase {
			name: "registered trailing slash -- dynamic without trailing wins over catch-all",
			patterns: &["/", "/:user/", "/:user", "/:user/*", "/posts"],
			path: "/bob",
			want_pattern: "/:user",
			want_params: Some(params(&[("user", "bob")])),
			want_splat_values: &[],
		},
		MatchCase {
			name: "registered trailing slash -- dynamic with trailing matches trailing pattern",
			patterns: &["/", "/:user/", "/:user", "/:user/*", "/posts"],
			path: "/bob/",
			want_pattern: "/:user/",
			want_params: Some(params(&[("user", "bob")])),
			want_splat_values: &[],
		},
		MatchCase {
			name: "registered trailing slash -- dynamic no trailing should NOT match",
			patterns: &["/", "/:user/", "/:user/*", "/posts"],
			path: "/bob",
			want_pattern: NOT_FOUND,
			want_params: None,
			want_splat_values: &[],
		},
		MatchCase {
			name: "registered trailing slash -- dynamic trailing matches trailing pattern",
			patterns: &["/", "/:user/", "/:user/*", "/posts"],
			path: "/bob/",
			want_pattern: "/:user/",
			want_params: Some(params(&[("user", "bob")])),
			want_splat_values: &[],
		},
		MatchCase {
			name: "parameter match",
			patterns: &["/users", "/users/:id", "/users/profile"],
			path: "/users/123",
			want_pattern: "/users/:id",
			want_params: Some(params(&[("id", "123")])),
			want_splat_values: &[],
		},
		MatchCase {
			name: "multiple matches",
			patterns: &["/", "/api", "/api/:version", "/api/v1"],
			path: "/api/v1",
			want_pattern: "/api/v1",
			want_params: None,
			want_splat_values: &[],
		},
		MatchCase {
			name: "splat match",
			patterns: &["/files", "/files/*"],
			path: "/files/documents/report.pdf",
			want_pattern: "/files/*",
			want_params: None,
			want_splat_values: &["documents", "report.pdf"],
		},
		MatchCase {
			name: "no match",
			patterns: &["/users", "/posts", "/settings"],
			path: "/profile",
			want_pattern: NOT_FOUND,
			want_params: None,
			want_splat_values: &[],
		},
		MatchCase {
			name: "complex nested paths",
			patterns: &[
				"/api/v1/users",
				"/api/:version/users",
				"/api/v1/users/:id",
				"/api/:version/users/:id",
				"/api/v1/users/:id/posts",
				"/api/:version/users/:id/posts",
			],
			path: "/api/v2/users/123/posts",
			want_pattern: "/api/:version/users/:id/posts",
			want_params: Some(params(&[("version", "v2"), ("id", "123")])),
			want_splat_values: &[],
		},
		MatchCase {
			name: "no patterns",
			patterns: &[],
			path: "/users",
			want_pattern: NOT_FOUND,
			want_params: None,
			want_splat_values: &[],
		},
		MatchCase {
			name: "many params",
			patterns: &["/api/:p1/:p2/:p3/:p4/:p5"],
			path: "/api/a/b/c/d/e",
			want_pattern: "/api/:p1/:p2/:p3/:p4/:p5",
			want_params: Some(params(&[
				("p1", "a"),
				("p2", "b"),
				("p3", "c"),
				("p4", "d"),
				("p5", "e"),
			])),
			want_splat_values: &[],
		},
		MatchCase {
			name: "nested no match",
			patterns: &["/users/:id", "/users/:id/profile"],
			path: "users/123/settings",
			want_pattern: NOT_FOUND,
			want_params: None,
			want_splat_values: &[],
		},
	]
}

fn best_match_opts_to_test() -> Vec<Options> {
	vec![
		Options::default(),
		Options {
			explicit_index_segment_identifier: "_index".to_string(),
			..Options::default()
		},
		Options {
			dynamic_param_prefix: '$',
			..Options::default()
		},
		Options {
			splat_segment_identifier: '#',
			..Options::default()
		},
		Options {
			dynamic_param_prefix: '<',
			splat_segment_identifier: '>',
			..Options::default()
		},
		Options {
			explicit_index_segment_identifier: "_______".to_string(),
			dynamic_param_prefix: '<',
			splat_segment_identifier: '>',
		},
	]
}

#[test]
fn find_best_match_matches_table_cases() {
	for opts in best_match_opts_to_test() {
		for case in best_match_cases() {
			let mut m = MatcherBuilder::new(opts.clone()).unwrap();
			register_all(
				&mut m,
				rewrite_patterns(case.patterns, "", option_shape_for_rewrites(&opts)),
			);

			let m = m.finish_flat();
			let found = m.find_best_match(case.path);
			let want_match = case.want_pattern != NOT_FOUND;
			if !want_match {
				assert!(
					found.is_none(),
					"{} {:?}: expected no match, got {:?}",
					case.name,
					opts,
					found.map(|m| m.pattern.normalized_pattern().to_string())
				);
				continue;
			}

			let found = found.unwrap_or_else(|| panic!("{} {:?}: expected match", case.name, opts));
			assert_eq!(
				found.pattern.normalized_pattern(),
				case.want_pattern,
				"{} {:?}",
				case.name,
				opts
			);
			match &case.want_params {
				None => assert!(
					found.params.is_empty(),
					"{} {:?}: params = {:?}, want empty",
					case.name,
					opts,
					found.params
				),
				Some(want) => assert_eq!(&found.params, want, "{} {:?}", case.name, opts),
			}
			assert_eq!(
				found.splat_values,
				str_vec(case.want_splat_values),
				"{} {:?}",
				case.name,
				opts
			);
		}
	}
}

#[test]
fn find_best_match_additional_scenarios() {
	let cases = [
		(
			"default index",
			Options {
				..Options::default()
			},
			vec!["/", "/:slug", "/app"],
			"/settings/account",
		),
		(
			"explicit index",
			Options {
				explicit_index_segment_identifier: "_index".to_string(),
				..Options::default()
			},
			vec!["/", "/:slug", "/_index", "/app"],
			"/settings/account",
		),
	];
	for (name, opts, patterns, path) in cases {
		let mut m = MatcherBuilder::new(opts).unwrap();
		register_all(&mut m, patterns);
		let m = m.finish_flat();
		assert!(m.find_best_match(path).is_none(), "{name}");
	}
}

#[test]
fn best_match_splat_specificity_is_registration_order_independent() {
	for patterns in [vec!["/*", "/:x/*"], vec!["/:x/*", "/*"]] {
		let mut m = MatcherBuilder::new(Options {
			..Options::default()
		})
		.unwrap();
		register_all(&mut m, patterns);
		let m = m.finish_flat();
		let found = m.find_best_match("/a/").expect("expected match");
		assert_eq!(found.pattern.normalized_pattern(), "/*");
		assert!(found.params.is_empty());
		assert_eq!(found.splat_values, str_vec(&["a"]));
	}
}

#[test]
fn best_match_trailing_dynamic_beats_trailing_splat() {
	let pattern_sets = [
		vec!["/:x/*", "/:y"],
		vec!["/:y", "/:x/*"],
		vec!["/*", "/:x/*", "/:y"],
		vec!["/*", "/:y", "/:x/*"],
	];

	for (i, patterns) in pattern_sets.into_iter().enumerate() {
		let mut m = MatcherBuilder::new(Options {
			..Options::default()
		})
		.unwrap();
		register_all(&mut m, patterns);
		let m = m.finish_flat();
		let found = m.find_best_match("/a/").expect("expected match");
		assert_eq!(found.pattern.normalized_pattern(), "/:y", "pattern set {i}");
		assert_eq!(found.params, params(&[("y", "a")]), "pattern set {i}");
		assert!(found.splat_values.is_empty(), "pattern set {i}");
	}
}

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

struct NestedScenario {
	path: &'static str,
	expected_matches: &'static [&'static str],
	splat_values: &'static [&'static str],
	params: Option<Params>,
}

fn nested_scenarios() -> Vec<NestedScenario> {
	vec![
		NestedScenario {
			path: "/does-not-exist",
			splat_values: &["does-not-exist"],
			expected_matches: &["", "/*"],
			params: None,
		},
		NestedScenario {
			path: "/this-should-be-ignored",
			splat_values: &["this-should-be-ignored"],
			expected_matches: &["", "/*"],
			params: None,
		},
		NestedScenario {
			path: "/",
			expected_matches: &["", "/"],
			splat_values: &[],
			params: None,
		},
		NestedScenario {
			path: "/lion",
			expected_matches: &["", "/lion", "/lion/"],
			splat_values: &[],
			params: None,
		},
		NestedScenario {
			path: "/lion/123",
			splat_values: &["123"],
			expected_matches: &["", "/lion", "/lion/*"],
			params: None,
		},
		NestedScenario {
			path: "/lion/123/456",
			splat_values: &["123", "456"],
			expected_matches: &["", "/lion", "/lion/*"],
			params: None,
		},
		NestedScenario {
			path: "/lion/123/456/789",
			splat_values: &["123", "456", "789"],
			expected_matches: &["", "/lion", "/lion/*"],
			params: None,
		},
		NestedScenario {
			path: "/tiger",
			expected_matches: &["", "/tiger", "/tiger/"],
			splat_values: &[],
			params: None,
		},
		NestedScenario {
			path: "/tiger/123",
			params: Some(params(&[("tiger_id", "123")])),
			expected_matches: &["", "/tiger", "/tiger/:tiger_id", "/tiger/:tiger_id/"],
			splat_values: &[],
		},
		NestedScenario {
			path: "/tiger/123/456",
			params: Some(params(&[("tiger_id", "123"), ("tiger_cub_id", "456")])),
			expected_matches: &[
				"",
				"/tiger",
				"/tiger/:tiger_id",
				"/tiger/:tiger_id/:tiger_cub_id",
			],
			splat_values: &[],
		},
		NestedScenario {
			path: "/tiger/123/456/789",
			params: Some(params(&[("tiger_id", "123")])),
			splat_values: &["456", "789"],
			expected_matches: &["", "/tiger", "/tiger/:tiger_id", "/tiger/:tiger_id/*"],
		},
		NestedScenario {
			path: "/bear",
			expected_matches: &["", "/bear", "/bear/"],
			splat_values: &[],
			params: None,
		},
		NestedScenario {
			path: "/bear/123",
			params: Some(params(&[("bear_id", "123")])),
			expected_matches: &["", "/bear", "/bear/:bear_id"],
			splat_values: &[],
		},
		NestedScenario {
			path: "/bear/123/456",
			params: Some(params(&[("bear_id", "123")])),
			splat_values: &["456"],
			expected_matches: &["", "/bear", "/bear/:bear_id", "/bear/:bear_id/*"],
		},
		NestedScenario {
			path: "/bear/123/456/789",
			params: Some(params(&[("bear_id", "123")])),
			splat_values: &["456", "789"],
			expected_matches: &["", "/bear", "/bear/:bear_id", "/bear/:bear_id/*"],
		},
		NestedScenario {
			path: "/dashboard",
			expected_matches: &["", "/dashboard", "/dashboard/"],
			splat_values: &[],
			params: None,
		},
		NestedScenario {
			path: "/dashboard/asdf",
			splat_values: &["asdf"],
			expected_matches: &["", "/dashboard", "/dashboard/*"],
			params: None,
		},
		NestedScenario {
			path: "/dashboard/customers",
			expected_matches: &[
				"",
				"/dashboard",
				"/dashboard/customers",
				"/dashboard/customers/",
			],
			splat_values: &[],
			params: None,
		},
		NestedScenario {
			path: "/dashboard/customers/123",
			params: Some(params(&[("customer_id", "123")])),
			expected_matches: &[
				"",
				"/dashboard",
				"/dashboard/customers",
				"/dashboard/customers/:customer_id",
				"/dashboard/customers/:customer_id/",
			],
			splat_values: &[],
		},
		NestedScenario {
			path: "/dashboard/customers/123/orders",
			params: Some(params(&[("customer_id", "123")])),
			expected_matches: &[
				"",
				"/dashboard",
				"/dashboard/customers",
				"/dashboard/customers/:customer_id",
				"/dashboard/customers/:customer_id/orders",
				"/dashboard/customers/:customer_id/orders/",
			],
			splat_values: &[],
		},
		NestedScenario {
			path: "/dashboard/customers/123/orders/456",
			params: Some(params(&[("customer_id", "123"), ("order_id", "456")])),
			expected_matches: &[
				"",
				"/dashboard",
				"/dashboard/customers",
				"/dashboard/customers/:customer_id",
				"/dashboard/customers/:customer_id/orders",
				"/dashboard/customers/:customer_id/orders/:order_id",
			],
			splat_values: &[],
		},
		NestedScenario {
			path: "/articles",
			expected_matches: &["", "/articles/"],
			splat_values: &[],
			params: None,
		},
		NestedScenario {
			path: "/articles/bob",
			splat_values: &["articles", "bob"],
			expected_matches: &["", "/*"],
			params: None,
		},
		NestedScenario {
			path: "/articles/test",
			splat_values: &["articles", "test"],
			expected_matches: &["", "/*"],
			params: None,
		},
		NestedScenario {
			path: "/articles/test/articles",
			expected_matches: &["", "/articles/test/articles/"],
			splat_values: &[],
			params: None,
		},
		NestedScenario {
			path: "/dynamic-index/index",
			expected_matches: &["", "/dynamic-index/index"],
			splat_values: &[],
			params: None,
		},
		NestedScenario {
			path: "/a/b/hi",
			params: Some(params(&[("value", "hi")])),
			expected_matches: &["", "/a/b/:value"],
			splat_values: &[],
		},
		NestedScenario {
			path: "/c/d/e/hi",
			params: Some(params(&[("_", "hi")])),
			expected_matches: &["", "/c/d/e/:_"],
			splat_values: &[],
		},
		NestedScenario {
			path: "/f/g/h/i/hi/hi2",
			params: Some(params(&[("first", "hi"), ("second", "hi2")])),
			expected_matches: &["", "/f/g/h/i/:first/:second"],
			splat_values: &[],
		},
		NestedScenario {
			path: "/j/k/l/m/n/hi/hi2",
			params: Some(params(&[("left", "hi"), ("right", "hi2")])),
			expected_matches: &["", "/j/k/l/m/n/:left/:right"],
			splat_values: &[],
		},
	]
}

fn nested_opts_to_test() -> Vec<Options> {
	vec![
		Options::default(),
		Options {
			explicit_index_segment_identifier: "_index".to_string(),
			..Options::default()
		},
		Options {
			dynamic_param_prefix: '$',
			..Options::default()
		},
		Options {
			splat_segment_identifier: '#',
			..Options::default()
		},
		Options {
			explicit_index_segment_identifier: "_______".to_string(),
			dynamic_param_prefix: '<',
			splat_segment_identifier: '>',
		},
		Options {
			explicit_index_segment_identifier: String::new(),
			dynamic_param_prefix: '<',
			splat_segment_identifier: '>',
		},
	]
}

fn opts_uses_explicit_index(opts: &Options) -> bool {
	!opts.explicit_index_segment_identifier.is_empty()
}

fn adjust_expected_matches(expected: &[&str], uses_explicit_index: bool) -> Vec<String> {
	expected
		.iter()
		.filter(|e| uses_explicit_index || !e.is_empty())
		.map(|e| (*e).to_string())
		.collect()
}

#[test]
fn find_nested_matches_matches_fixture_cases() {
	for opts in nested_opts_to_test() {
		let uses_explicit = opts_uses_explicit_index(&opts);
		let mut m = MatcherBuilder::new(opts.clone()).unwrap();
		register_all(
			&mut m,
			rewrite_patterns(NESTED_PATTERNS, "_index", option_shape_for_rewrites(&opts)),
		);
		let m = m.finish_nested();

		for tc in nested_scenarios() {
			let results = m.find_nested_matches(tc.path);
			let expected = adjust_expected_matches(tc.expected_matches, uses_explicit);

			if expected.is_empty() {
				assert!(
					results.is_none(),
					"{} {:?}: expected no matches, got {:?}",
					tc.path,
					opts,
					results
				);
				continue;
			}

			let results =
				results.unwrap_or_else(|| panic!("{} {:?}: expected matches", tc.path, opts));
			let want_params = tc.params.unwrap_or_default();
			assert_eq!(results.params, want_params, "{} {:?}", tc.path, opts);
			assert_eq!(
				results.splat_values,
				str_vec(tc.splat_values),
				"{} {:?}",
				tc.path,
				opts
			);
			let actual: Vec<String> = results
				.matches
				.iter()
				.map(|m| m.pattern.normalized_pattern().to_string())
				.collect();
			assert_eq!(actual, expected, "{} {:?}", tc.path, opts);
		}
	}
}

#[test]
fn find_nested_matches_additional_scenarios() {
	struct Case {
		name: &'static str,
		patterns: &'static [&'static str],
		path: &'static str,
		expect_match: bool,
		expected_matches: &'static [&'static str],
	}

	let cases = [
		Case {
			name: "Invalid match with unhandled segment",
			patterns: &["/", "/:slug", "/_index", "/app"],
			path: "/settings/account",
			expect_match: false,
			expected_matches: &[],
		},
		Case {
			name: "Deeper Invalid 'Almost' Match",
			patterns: &["/dashboard/customers"],
			path: "/dashboard/customers/reports",
			expect_match: false,
			expected_matches: &[],
		},
		Case {
			name: "Splat as the Only Full Match",
			patterns: &["/files/*", "/files/images"],
			path: "/files/documents/report.pdf",
			expect_match: true,
			expected_matches: &["/files/*"],
		},
		Case {
			name: "Index Segment Edge Case with Extra Segment",
			patterns: &["/articles/_index"],
			path: "/articles/some-topic",
			expect_match: false,
			expected_matches: &[],
		},
		Case {
			name: "No Root Fallback for Multi-Segment Path",
			patterns: &["/"],
			path: "/some/random/path",
			expect_match: false,
			expected_matches: &[],
		},
		Case {
			name: "A",
			patterns: &["/"],
			path: "/",
			expect_match: true,
			expected_matches: &["/"],
		},
		Case {
			name: "B",
			patterns: &["/*"],
			path: "/",
			expect_match: true,
			expected_matches: &["/*"],
		},
		Case {
			name: "C",
			patterns: &["/_index"],
			path: "/",
			expect_match: true,
			expected_matches: &["/_index"],
		},
		Case {
			name: "AB",
			patterns: &["/", "/*"],
			path: "/",
			expect_match: true,
			expected_matches: &["/", "/*"],
		},
		Case {
			name: "AC",
			patterns: &["/", "/_index"],
			path: "/",
			expect_match: true,
			expected_matches: &["/", "/_index"],
		},
		Case {
			name: "BC",
			patterns: &["/*", "/_index"],
			path: "/",
			expect_match: true,
			expected_matches: &["/_index"],
		},
		Case {
			name: "ABC",
			patterns: &["/", "/*", "/_index"],
			path: "/",
			expect_match: true,
			expected_matches: &["/", "/_index"],
		},
		Case {
			name: "A-docs",
			patterns: &["/"],
			path: "/docs",
			expect_match: false,
			expected_matches: &[],
		},
		Case {
			name: "B-docs",
			patterns: &["/*"],
			path: "/docs",
			expect_match: true,
			expected_matches: &["/*"],
		},
		Case {
			name: "C-docs",
			patterns: &["/_index"],
			path: "/docs",
			expect_match: false,
			expected_matches: &[],
		},
		Case {
			name: "AB-docs",
			patterns: &["/", "/*"],
			path: "/docs",
			expect_match: true,
			expected_matches: &["/", "/*"],
		},
		Case {
			name: "AC-docs",
			patterns: &["/", "/_index"],
			path: "/docs",
			expect_match: false,
			expected_matches: &[],
		},
		Case {
			name: "BC-docs",
			patterns: &["/*", "/_index"],
			path: "/docs",
			expect_match: true,
			expected_matches: &["/*"],
		},
		Case {
			name: "ABC-docs",
			patterns: &["/", "/*", "/_index"],
			path: "/docs",
			expect_match: true,
			expected_matches: &["/", "/*"],
		},
	];

	for tc in cases {
		let mut m = MatcherBuilder::new(Options {
			explicit_index_segment_identifier: "_index".to_string(),
			..Options::default()
		})
		.unwrap();
		register_all(&mut m, tc.patterns);
		let m = m.finish_nested();
		let results = m.find_nested_matches(tc.path);
		assert_eq!(results.is_some(), tc.expect_match, "{}", tc.name);
		if tc.expect_match && !tc.expected_matches.is_empty() {
			let actual: Vec<String> = results
				.unwrap()
				.matches
				.iter()
				.map(|m| m.pattern.original_pattern().to_string())
				.collect();
			assert_eq!(actual, str_vec(tc.expected_matches), "{}", tc.name);
		}
	}
}

#[test]
fn nested_trailing_slash_behavior_is_stable() {
	let patterns = [
		"/",
		"/_index",
		"/about",
		"/about/location",
		"/about/hobbies",
		"/about/:id",
		"/about/*",
	];
	let cases = [
		(
			"about with trailing slash",
			"/about/",
			vec!["/", "/about"],
			vec!["/about/:id", "/about/*"],
		),
		(
			"about without trailing slash",
			"/about",
			vec!["/", "/about"],
			vec!["/about/:id", "/about/*"],
		),
		(
			"about with actual id",
			"/about/123",
			vec!["/", "/about/:id"],
			vec!["/about/*"],
		),
		(
			"about location exact match",
			"/about/location",
			vec!["/", "/about", "/about/location"],
			vec!["/_index", "/about/:id", "/about/*"],
		),
		(
			"about with multiple segments",
			"/about/something/else",
			vec!["/", "/about/*"],
			vec!["/about/:id", "/about/location"],
		),
	];

	let mut m = MatcherBuilder::new(Options {
		explicit_index_segment_identifier: "_index".to_string(),
		..Options::default()
	})
	.unwrap();
	register_all(&mut m, patterns);
	let m = m.finish_nested();

	for (name, path, expected, unexpected) in cases {
		let results = m
			.find_nested_matches(path)
			.unwrap_or_else(|| panic!("{name}"));
		let actual: Vec<String> = results
			.matches
			.iter()
			.map(|m| m.pattern.original_pattern().to_string())
			.collect();
		for expected in expected {
			assert!(
				actual.contains(&expected.to_string()),
				"{name}: expected {expected}"
			);
		}
		for unexpected in unexpected {
			assert!(
				!actual.contains(&unexpected.to_string()),
				"{name}: unexpected {unexpected}; actual {actual:?}"
			);
		}
	}
}

#[test]
fn partial_matching_with_gaps_is_stable() {
	let opts = Options {
		explicit_index_segment_identifier: "_index".to_string(),
		..Options::default()
	};

	let mut m = MatcherBuilder::new(opts.clone()).unwrap();
	m.register_pattern("/bob").unwrap();
	m.register_pattern("/bob/larry/susan/jeff").unwrap();
	let m = m.finish_nested();
	let results = m
		.find_nested_matches("/bob/larry/susan/jeff")
		.expect("expected matches");
	let actual: Vec<String> = results
		.matches
		.iter()
		.map(|m| m.pattern.original_pattern().to_string())
		.collect();
	assert_eq!(actual, str_vec(&["/bob", "/bob/larry/susan/jeff"]));

	let mut m = MatcherBuilder::new(opts).unwrap();
	m.register_pattern("/bob").unwrap();
	m.register_pattern("/bob/larry/susan/jeff").unwrap();
	let m = m.finish_nested();
	assert!(m.find_nested_matches("/bob/larry").is_none());
}

#[test]
fn find_nested_matches_smoke() {
	let mut m = MatcherBuilder::new(Options::default()).unwrap();
	register_all(&mut m, ["", "/users", "/users/:id"]);
	let m = m.finish_nested();
	let results = m.find_nested_matches("/users/123").expect("expected match");
	assert_eq!(results.matches.len(), 3);
	assert_eq!(results.params.get("id"), Some("123"));
}

/*
The principle under pin: "/foo/bar" does not match "/foo" unless a
"/foo/<something-that-matches-bar>" completes the chain. A prefix hit
alone is not a match, so it cannot displace the catch-all: with only a
dead prefix registered, the catch-all takes the path.
*/
#[test]
fn catch_all_takes_paths_where_no_chain_completes() {
	let mut m = MatcherBuilder::new(Options::default()).unwrap();
	register_all(&mut m, ["", "/foo", "/*"]);
	let m = m.finish_nested();

	let results = m
		.find_nested_matches("/foo/bar")
		.expect("the catch-all must take a path no chain completes on");
	let actual: Vec<String> = results
		.matches
		.iter()
		.map(|matched| matched.pattern.normalized_pattern().to_string())
		.collect();
	assert_eq!(actual, str_vec(&["", "/*"]));
	assert_eq!(results.splat_values, str_vec(&["foo", "bar"]));
}

// The other half of the same principle: with a completing child, the
// chain through the prefix wins and the catch-all yields.
#[test]
fn chain_completion_through_a_prefix_beats_the_catch_all() {
	let mut m = MatcherBuilder::new(Options::default()).unwrap();
	register_all(&mut m, ["", "/foo", "/foo/:id", "/*"]);
	let m = m.finish_nested();

	let results = m.find_nested_matches("/foo/bar").expect("expected match");
	let actual: Vec<String> = results
		.matches
		.iter()
		.map(|matched| matched.pattern.normalized_pattern().to_string())
		.collect();
	assert_eq!(actual, str_vec(&["", "/foo", "/foo/:id"]));
	assert_eq!(results.params.get("id"), Some("bar"));
}

#[test]
fn match_ordering_is_deterministic() {
	let mut first_params = None;
	let mut first_order = None;
	for i in 0..1000 {
		let mut m = MatcherBuilder::new(Options {
			..Options::default()
		})
		.unwrap();
		register_all(&mut m, ["/api/v1", "/api/:version"]);

		let m = m.finish_nested();
		let results = m.find_nested_matches("/api/v1").expect("expected matches");
		let order: Vec<String> = results
			.matches
			.iter()
			.map(|m| m.pattern.normalized_pattern().to_string())
			.collect();

		if i == 0 {
			first_params = Some(results.params);
			first_order = Some(order);
			continue;
		}
		assert_eq!(
			Some(&results.params),
			first_params.as_ref(),
			"static vs dynamic same depth iteration {i}: params inconsistent"
		);
		assert_eq!(
			Some(&order),
			first_order.as_ref(),
			"static vs dynamic same depth iteration {i}: order inconsistent"
		);
	}

	let mut first_params = None;
	let mut first_splat = None;
	for i in 0..1000 {
		let mut m = MatcherBuilder::new(Options {
			..Options::default()
		})
		.unwrap();
		register_all(&mut m, ["/users/:id", "/users/*"]);

		let m = m.finish_nested();
		let results = m
			.find_nested_matches("/users/123")
			.expect("expected matches");

		if i == 0 {
			assert!(
				!results.params.is_empty(),
				"dynamic and splat same depth: expected params from dynamic match"
			);
			first_params = Some(results.params);
			first_splat = Some(results.splat_values);
			continue;
		}
		assert_eq!(
			Some(&results.params),
			first_params.as_ref(),
			"dynamic and splat same depth iteration {i}: params inconsistent"
		);
		assert_eq!(
			Some(&results.splat_values),
			first_splat.as_ref(),
			"dynamic and splat same depth iteration {i}: splat inconsistent"
		);
	}

	let mut first_params = None;
	let mut first_order = None;
	for i in 0..1000 {
		let mut m = MatcherBuilder::new(Options {
			..Options::default()
		})
		.unwrap();
		register_all(&mut m, ["/a/b", "/a/:p", "/:x/b"]);

		let m = m.finish_nested();
		let results = m.find_nested_matches("/a/b").expect("expected matches");
		let order: Vec<String> = results
			.matches
			.iter()
			.map(|m| m.pattern.normalized_pattern().to_string())
			.collect();

		if i == 0 {
			first_params = Some(results.params);
			first_order = Some(order);
			continue;
		}
		assert_eq!(
			Some(&results.params),
			first_params.as_ref(),
			"three patterns same depth iteration {i}: params inconsistent"
		);
		assert_eq!(
			Some(&order),
			first_order.as_ref(),
			"three patterns same depth iteration {i}: order inconsistent"
		);
	}

	let pattern_sets = [
		vec!["", "/a/:x", "/:x/:y", "/a/*"],
		vec!["", "/a/*", "/:x/:y", "/a/:x"],
		vec!["/:x/:y", "", "/a/:x", "/a/*"],
		vec!["/a/:x", "/a/*", "", "/:x/:y"],
	];

	let mut first_patterns = None;
	let mut first_params = None;
	let mut first_splat = None;
	for (i, patterns) in pattern_sets.into_iter().enumerate() {
		let mut m = MatcherBuilder::new(Options {
			..Options::default()
		})
		.unwrap();
		register_all(&mut m, patterns);

		let m = m.finish_nested();
		let results = m.find_nested_matches("/a/a/a").expect("expected matches");
		let actual_patterns: Vec<String> = results
			.matches
			.iter()
			.map(|m| m.pattern.normalized_pattern().to_string())
			.collect();

		if i == 0 {
			first_patterns = Some(actual_patterns);
			first_params = Some(results.params);
			first_splat = Some(results.splat_values);
			continue;
		}
		assert_eq!(
			Some(&actual_patterns),
			first_patterns.as_ref(),
			"deep splat pattern set {i}: patterns inconsistent"
		);
		assert_eq!(
			Some(&results.params),
			first_params.as_ref(),
			"deep splat pattern set {i}: params inconsistent"
		);
		assert_eq!(
			Some(&results.splat_values),
			first_splat.as_ref(),
			"deep splat pattern set {i}: splat inconsistent"
		);
	}
	assert_eq!(
		first_patterns.unwrap(),
		str_vec(&["", "/a/*"]),
		"deep splat should prune all competing longest dynamics"
	);
	assert!(
		first_params.unwrap().is_empty(),
		"deep splat should not carry params"
	);
	assert_eq!(first_splat.unwrap(), str_vec(&["a", "a"]));

	let pattern_sets = [
		vec!["/b/*", "/:x/:y", "/:x/*"],
		vec!["/:x/*", "/:x/:y", "/b/*"],
		vec!["/:x/:y", "/b/*", "/:x/*"],
		vec!["/:x/*", "/b/*", "/:x/:y"],
	];

	let mut first_patterns = None;
	let mut first_params = None;
	let mut first_splat = None;
	for (i, patterns) in pattern_sets.into_iter().enumerate() {
		let mut m = MatcherBuilder::new(Options {
			..Options::default()
		})
		.unwrap();
		register_all(&mut m, patterns);

		let m = m.finish_nested();
		let results = m.find_nested_matches("/b/a").expect("expected matches");
		let actual_patterns: Vec<String> = results
			.matches
			.iter()
			.map(|m| m.pattern.normalized_pattern().to_string())
			.collect();

		if i == 0 {
			first_patterns = Some(actual_patterns);
			first_params = Some(results.params);
			first_splat = Some(results.splat_values);
			continue;
		}
		assert_eq!(
			Some(&actual_patterns),
			first_patterns.as_ref(),
			"exact dynamic pattern set {i}: patterns inconsistent"
		);
		assert_eq!(
			Some(&results.params),
			first_params.as_ref(),
			"exact dynamic pattern set {i}: params inconsistent"
		);
		assert_eq!(
			Some(&results.splat_values),
			first_splat.as_ref(),
			"exact dynamic pattern set {i}: splat inconsistent"
		);
	}
	assert_eq!(
		first_patterns.unwrap(),
		str_vec(&["/:x/:y"]),
		"exact dynamic should prune all competing longest splats"
	);
	assert_eq!(first_params.unwrap(), params(&[("x", "b"), ("y", "a")]));
	assert!(
		first_splat.unwrap().is_empty(),
		"exact dynamic should not carry splat values"
	);
}

const PROPERTY_CASES: usize = 500;

const PROPERTY_BEST_MATCH_PATTERNS: &[&str] = &[
	"", "/", "/*", "/a", "/a/", "/b", "/b/", "/a/b", "/a/b/", "/a/c", "/b/a", "/:x", "/:x/",
	"/a/:x", "/a/:x/", "/:x/a", "/:x/:y", "/a/b/:x", "/a/:x/c", "/a/*", "/b/*", "/:x/*", "/a/b/*",
];

const PROPERTY_NESTED_MATCH_PATTERNS: &[&str] = &[
	"", "/", "/*", "/a", "/a/", "/a/*", "/a/:x", "/a/:x/", "/a/:x/*", "/a/:x/b", "/a/:x/b/",
	"/a/b", "/a/b/", "/a/b/*", "/a/b/:x", "/a/b/:x/", "/b", "/b/", "/b/*", "/b/:x", "/b/:x/*",
	"/b/a", "/b/a/", "/b/a/*", "/:x", "/:x/", "/:x/*", "/:x/a", "/:x/a/", "/:x/a/*", "/:x/a/b",
	"/:x/:y", "/:x/:y/", "/:x/:y/*",
];

const PROPERTY_NESTED_EXPLICIT_INDEX_PATTERNS: &[&str] = &[
	"/",
	"/_index",
	"/a",
	"/a/_index",
	"/a/*",
	"/a/:x",
	"/a/:x/_index",
	"/a/:x/*",
	"/a/b",
	"/a/b/_index",
	"/a/b/*",
	"/b",
	"/b/_index",
	"/b/*",
	"/b/:x",
	"/b/:x/_index",
	"/:x",
	"/:x/_index",
	"/:x/*",
	"/:x/a",
	"/:x/a/_index",
	"/:x/a/*",
	"/:x/:y",
	"/:x/:y/_index",
	"/:x/:y/*",
];

const PROPERTY_UNRELATED_STATIC_PATTERNS: &[&str] = &[
	"/z", "/z/", "/z/a", "/z/a/", "/z/a/b", "/z/b", "/z/b/", "/z/c", "/z/id", "/zz", "/zz/a",
	"/zz/b",
];

const PROPERTY_PATH_SEGMENTS: &[&str] = &["a", "b", "c", "id", "0"];
const PROPERTY_GENERATED_STATIC_SEGMENTS: &[&str] = &["a", "b", "c", "id", "0", "left", "right"];
const PROPERTY_GENERATED_DYNAMIC_NAMES: &[&str] = &["x", "y", "z", "id", "slug", "page"];
const PROPERTY_GENERATED_DYNAMIC_NAME_PAIRS: &[(&str, &str)] = &[
	("x", "y"),
	("id", "slug"),
	("page", "section"),
	("left", "right"),
	("first", "second"),
	("alpha", "beta"),
];

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
enum PropertyRegistrationOrder {
	Original,
	Reversed,
	Sorted,
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
enum PropertySegmentKind {
	Static,
	Dynamic,
	Splat,
	Index,
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
enum PropertyGeneratedSegmentKind {
	Static,
	Dynamic,
	Splat,
	Index,
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
enum PropertyNormalizedCollisionKind {
	ExplicitIndex,
	CustomDynamic,
}

#[derive(Clone, Debug)]
struct PropertyRouteCase {
	catalog: Vec<String>,
	pattern_indexes: Vec<usize>,
	path_segments: Vec<String>,
	trailing_slash: bool,
}

#[derive(Clone, Debug)]
struct PropertyModelPattern {
	normalized: String,
	segments: Vec<PropertyModelSegment>,
	registration_order: usize,
	is_static: bool,
}

#[derive(Clone, Debug)]
struct PropertyModelSegment {
	value: String,
	kind: PropertySegmentKind,
}

#[derive(Clone, Debug, Eq, PartialEq)]
struct PropertyModelMatch {
	normalized_pattern: String,
	params: Params,
	splat_values: Vec<String>,
	score: i32,
	segment_ranks: Vec<i32>,
	registration_order: usize,
	is_static: bool,
	last_is_dynamic: bool,
	last_is_index: bool,
	last_is_splat: bool,
	segment_len: usize,
	dynamic_params: usize,
}

#[derive(Clone, Debug, Eq, PartialEq)]
struct PropertyNestedObservation {
	found: bool,
	patterns: Vec<String>,
	params: Params,
	splat_values: Vec<String>,
	match_params: Vec<Params>,
	match_splats: Vec<Vec<String>>,
}

#[derive(Clone, Debug, Eq, PartialEq)]
struct PropertyMatchObservation {
	found: bool,
	pattern: String,
	params: Params,
	splat_values: Vec<String>,
}

#[derive(Clone, Debug)]
struct PropertyCollisionSegment {
	kind: PropertyGeneratedSegmentKind,
	static_value: String,
	name_pair: (&'static str, &'static str),
}

#[derive(Clone, Debug)]
struct PropertyCollisionCase {
	segments: Vec<PropertyCollisionSegment>,
}

#[derive(Clone, Debug)]
struct PropertyNormalizedCollisionCase {
	kind: PropertyNormalizedCollisionKind,
	prefix: &'static str,
	explicit_index_id: &'static str,
}

#[derive(Clone, Debug)]
struct PropertyRng {
	state: u64,
}

impl PropertyRng {
	fn new(seed: u64) -> Self {
		Self { state: seed }
	}

	fn next(&mut self) -> u64 {
		self.state = self
			.state
			.wrapping_mul(6364136223846793005)
			.wrapping_add(1442695040888963407);
		self.state
	}

	fn bool(&mut self) -> bool {
		self.next() & 1 == 1
	}

	fn usize_inclusive(&mut self, min: usize, max: usize) -> usize {
		assert!(min <= max);
		min + (self.next() as usize % (max - min + 1))
	}

	fn sampled<T: Copy>(&mut self, values: &[T]) -> T {
		values[self.usize_inclusive(0, values.len() - 1)]
	}

	fn sampled_clone<T: Clone>(&mut self, values: &[T]) -> T {
		values[self.usize_inclusive(0, values.len() - 1)].clone()
	}
}

fn property_default_opts() -> Options {
	Options {
		..Options::default()
	}
}

fn property_custom_marker_opts(rng: &mut PropertyRng) -> Options {
	rng.sampled_clone(&[
		Options {
			dynamic_param_prefix: '$',
			..Options::default()
		},
		Options {
			splat_segment_identifier: '#',
			..Options::default()
		},
		Options {
			dynamic_param_prefix: '<',
			splat_segment_identifier: '>',
			..Options::default()
		},
	])
}

fn property_explicit_index_opts(rng: &mut PropertyRng) -> Options {
	rng.sampled_clone(&[
		Options {
			explicit_index_segment_identifier: "_index".to_string(),
			..Options::default()
		},
		Options {
			explicit_index_segment_identifier: "_______".to_string(),
			dynamic_param_prefix: '<',
			splat_segment_identifier: '>',
		},
	])
}

fn property_catalog(values: &[&str]) -> Vec<String> {
	values.iter().map(|v| (*v).to_string()).collect()
}

fn property_draw_catalog_case(
	rng: &mut PropertyRng,
	catalog: &[&str],
	max_patterns: usize,
	max_path_segments: usize,
) -> PropertyRouteCase {
	let path_segment_count = rng.usize_inclusive(0, max_path_segments);
	let path_segments = (0..path_segment_count)
		.map(|_| rng.sampled(PROPERTY_PATH_SEGMENTS).to_string())
		.collect();

	PropertyRouteCase {
		catalog: property_catalog(catalog),
		pattern_indexes: property_draw_indexes(rng, catalog.len(), max_patterns),
		path_segments,
		trailing_slash: rng.bool(),
	}
}

fn property_draw_indexes(
	rng: &mut PropertyRng,
	catalog_len: usize,
	max_patterns: usize,
) -> Vec<usize> {
	let index_count = rng.usize_inclusive(0, max_patterns);
	(0..index_count)
		.map(|_| rng.usize_inclusive(0, catalog_len - 1))
		.collect()
}

fn property_draw_patterns(
	rng: &mut PropertyRng,
	catalog: &[&str],
	max_patterns: usize,
) -> Vec<String> {
	let indexes = property_draw_indexes(rng, catalog.len(), max_patterns);
	let mut seen = HashSet::new();
	let mut patterns = Vec::new();
	for idx in indexes {
		if seen.insert(idx) {
			patterns.push(catalog[idx].to_string());
		}
	}
	patterns
}

fn property_draw_generated_case(
	rng: &mut PropertyRng,
	max_patterns: usize,
	max_pattern_segments: usize,
	max_path_segments: usize,
) -> PropertyRouteCase {
	let pattern_count = rng.usize_inclusive(0, max_patterns);
	let mut patterns = Vec::new();
	let mut normalized_seen = HashSet::new();
	let mut shape_seen = HashSet::new();

	for _ in 0..pattern_count {
		let pattern = property_draw_generated_pattern(rng, max_pattern_segments);
		if !property_pattern_is_valid(&pattern, &Options::default(), "") {
			continue;
		}
		let model = property_model_pattern(&pattern, patterns.len(), &Options::default(), "");
		if normalized_seen.contains(&model.normalized) || shape_seen.contains(&model.shape_key()) {
			continue;
		}
		normalized_seen.insert(model.normalized.clone());
		shape_seen.insert(model.shape_key());
		patterns.push(pattern);
	}

	property_route_case_from_generated(rng, patterns, max_path_segments)
}

fn property_draw_generated_explicit_index_case(
	rng: &mut PropertyRng,
	max_patterns: usize,
	max_pattern_segments: usize,
	max_path_segments: usize,
) -> PropertyRouteCase {
	let pattern_count = rng.usize_inclusive(0, max_patterns);
	let mut patterns = Vec::new();
	let mut normalized_seen = HashSet::new();
	let mut shape_seen = HashSet::new();
	let opts = Options {
		explicit_index_segment_identifier: "_index".to_string(),
		..Options::default()
	};

	for _ in 0..pattern_count {
		let pattern = property_draw_generated_explicit_index_pattern(rng, max_pattern_segments);
		if !property_pattern_is_valid(&pattern, &opts, "_index") {
			continue;
		}
		let model = property_model_pattern(&pattern, patterns.len(), &opts, "_index");
		if normalized_seen.contains(&model.normalized) || shape_seen.contains(&model.shape_key()) {
			continue;
		}
		normalized_seen.insert(model.normalized.clone());
		shape_seen.insert(model.shape_key());
		patterns.push(pattern);
	}

	property_route_case_from_generated(rng, patterns, max_path_segments)
}

fn property_route_case_from_generated(
	rng: &mut PropertyRng,
	patterns: Vec<String>,
	max_path_segments: usize,
) -> PropertyRouteCase {
	let path_segment_count = rng.usize_inclusive(0, max_path_segments);
	let path_segments = (0..path_segment_count)
		.map(|_| rng.sampled(PROPERTY_PATH_SEGMENTS).to_string())
		.collect();
	let pattern_indexes = (0..patterns.len()).collect();

	PropertyRouteCase {
		catalog: patterns,
		pattern_indexes,
		path_segments,
		trailing_slash: rng.bool(),
	}
}

fn property_pattern_is_valid(pattern: &str, opts: &Options, source_index: &str) -> bool {
	let rewritten = rewrite_single_pattern(pattern, source_index, option_shape_for_rewrites(opts));
	let Ok(matcher) = MatcherBuilder::new(opts.clone()) else {
		return false;
	};
	matcher.normalize_pattern(&rewritten).is_ok()
}

fn property_draw_generated_pattern(rng: &mut PropertyRng, max_pattern_segments: usize) -> String {
	match rng.usize_inclusive(0, 2) {
		0 => return String::new(),
		1 => return "/".to_string(),
		_ => {}
	}

	let segment_count = rng.usize_inclusive(1, max_pattern_segments);
	let segments: Vec<String> = (0..segment_count)
		.map(|i| property_draw_generated_segment(rng, i == segment_count - 1))
		.collect();
	let mut pattern = format!("/{}", segments.join("/"));
	if segments.last().map(String::as_str) != Some("*") && rng.bool() {
		pattern.push('/');
	}
	pattern
}

fn property_draw_generated_explicit_index_pattern(
	rng: &mut PropertyRng,
	max_pattern_segments: usize,
) -> String {
	match rng.usize_inclusive(0, 2) {
		0 => return String::new(),
		1 => return "/".to_string(),
		_ => {}
	}

	let segment_count = rng.usize_inclusive(1, max_pattern_segments);
	let last_kind = rng.sampled(&[
		PropertyGeneratedSegmentKind::Static,
		PropertyGeneratedSegmentKind::Dynamic,
		PropertyGeneratedSegmentKind::Splat,
		PropertyGeneratedSegmentKind::Index,
	]);
	let segments: Vec<String> = (0..segment_count)
		.map(|i| {
			if i != segment_count - 1 {
				return property_draw_generated_segment(rng, false);
			}
			match last_kind {
				PropertyGeneratedSegmentKind::Splat => "*".to_string(),
				PropertyGeneratedSegmentKind::Dynamic => {
					format!(":{}", rng.sampled(PROPERTY_GENERATED_DYNAMIC_NAMES))
				}
				PropertyGeneratedSegmentKind::Static => {
					rng.sampled(PROPERTY_GENERATED_STATIC_SEGMENTS).to_string()
				}
				PropertyGeneratedSegmentKind::Index => "_index".to_string(),
			}
		})
		.collect();
	format!("/{}", segments.join("/"))
}

fn property_draw_generated_segment(rng: &mut PropertyRng, is_last: bool) -> String {
	let kind = if is_last {
		rng.sampled(&[
			PropertyGeneratedSegmentKind::Static,
			PropertyGeneratedSegmentKind::Dynamic,
			PropertyGeneratedSegmentKind::Splat,
		])
	} else {
		rng.sampled(&[
			PropertyGeneratedSegmentKind::Static,
			PropertyGeneratedSegmentKind::Dynamic,
		])
	};
	match kind {
		PropertyGeneratedSegmentKind::Static => {
			rng.sampled(PROPERTY_GENERATED_STATIC_SEGMENTS).to_string()
		}
		PropertyGeneratedSegmentKind::Dynamic => {
			format!(":{}", rng.sampled(PROPERTY_GENERATED_DYNAMIC_NAMES))
		}
		PropertyGeneratedSegmentKind::Splat => "*".to_string(),
		PropertyGeneratedSegmentKind::Index => unreachable!(),
	}
}

fn property_draw_collision_case(
	rng: &mut PropertyRng,
	max_segments: usize,
) -> PropertyCollisionCase {
	let segment_count = rng.usize_inclusive(1, max_segments);
	let mut segments = Vec::with_capacity(segment_count);
	let mut has_dynamic = false;
	let mut dynamic_count = 0usize;

	for i in 0..segment_count {
		let is_last = i == segment_count - 1;
		let kind = if is_last {
			rng.sampled(&[
				PropertyGeneratedSegmentKind::Static,
				PropertyGeneratedSegmentKind::Dynamic,
				PropertyGeneratedSegmentKind::Splat,
			])
		} else {
			rng.sampled(&[
				PropertyGeneratedSegmentKind::Static,
				PropertyGeneratedSegmentKind::Dynamic,
			])
		};
		let mut segment = PropertyCollisionSegment {
			kind,
			static_value: String::new(),
			name_pair: ("x", "y"),
		};
		match kind {
			PropertyGeneratedSegmentKind::Static => {
				segment.static_value = rng.sampled(PROPERTY_GENERATED_STATIC_SEGMENTS).to_string();
			}
			PropertyGeneratedSegmentKind::Dynamic => {
				segment.name_pair = PROPERTY_GENERATED_DYNAMIC_NAME_PAIRS
					[dynamic_count % PROPERTY_GENERATED_DYNAMIC_NAME_PAIRS.len()];
				dynamic_count += 1;
				has_dynamic = true;
			}
			PropertyGeneratedSegmentKind::Splat => {}
			PropertyGeneratedSegmentKind::Index => unreachable!(),
		}
		segments.push(segment);
	}

	if !has_dynamic {
		segments[0].kind = PropertyGeneratedSegmentKind::Dynamic;
		segments[0].static_value.clear();
		segments[0].name_pair = PROPERTY_GENERATED_DYNAMIC_NAME_PAIRS[0];
	}

	PropertyCollisionCase { segments }
}

fn property_draw_normalized_collision_case(
	rng: &mut PropertyRng,
) -> PropertyNormalizedCollisionCase {
	PropertyNormalizedCollisionCase {
		kind: rng.sampled(&[
			PropertyNormalizedCollisionKind::ExplicitIndex,
			PropertyNormalizedCollisionKind::CustomDynamic,
		]),
		prefix: rng.sampled(&["/users", "/users/profile", "/a", "/a/b"]),
		explicit_index_id: rng.sampled(&["_index", "_______", "home"]),
	}
}

fn property_draw_invalid_explicit_index_case(rng: &mut PropertyRng) -> (String, Options) {
	let prefix = rng.sampled(&[
		"/users",
		"/users/profile",
		"/a",
		"/a/b",
		"/dashboard/customers",
	]);
	let explicit_index_id = rng.sampled(&["_index", "_______", "home"]);
	(
		format!("{prefix}/"),
		Options {
			explicit_index_segment_identifier: explicit_index_id.to_string(),
			..Options::default()
		},
	)
}

impl PropertyRouteCase {
	fn patterns(&self) -> Vec<String> {
		let mut seen = HashSet::new();
		let mut patterns = Vec::new();
		for idx in &self.pattern_indexes {
			if seen.insert(*idx) {
				patterns.push(self.catalog[*idx].clone());
			}
		}
		patterns
	}

	fn ordered_patterns(&self, order: PropertyRegistrationOrder) -> Vec<String> {
		let mut patterns = self.patterns();
		match order {
			PropertyRegistrationOrder::Original => {}
			PropertyRegistrationOrder::Reversed => patterns.reverse(),
			PropertyRegistrationOrder::Sorted => patterns.sort(),
		}
		patterns
	}

	fn path(&self) -> String {
		if self.path_segments.is_empty() {
			return "/".to_string();
		}
		let mut path = format!("/{}", self.path_segments.join("/"));
		if self.trailing_slash {
			path.push('/');
		}
		path
	}

	fn expected_best_match(
		&self,
		opts: &Options,
		source_index: &str,
	) -> Option<PropertyModelMatch> {
		let path = self.path();
		if path.contains("//") {
			return None;
		}
		let mut best = None;
		for (i, pattern) in self.patterns().iter().enumerate() {
			let model = property_model_pattern(pattern, i, opts, source_index);
			let Some(candidate) = model.best_match(&path) else {
				continue;
			};
			if best
				.as_ref()
				.is_none_or(|current| candidate.better_than(current))
			{
				best = Some(candidate);
			}
		}
		best
	}

	fn actual_best_match_observation(
		&self,
		opts: &Options,
		source_index: &str,
		order: PropertyRegistrationOrder,
	) -> PropertyMatchObservation {
		self.actual_best_match_observation_with_patterns(
			opts,
			source_index,
			self.ordered_patterns(order),
		)
	}

	fn actual_best_match_observation_with_patterns(
		&self,
		opts: &Options,
		source_index: &str,
		patterns: Vec<String>,
	) -> PropertyMatchObservation {
		let mut m = MatcherBuilder::new(opts.clone()).unwrap();
		let rewritten = rewrite_patterns(
			&patterns.iter().map(String::as_str).collect::<Vec<_>>(),
			source_index,
			option_shape_for_rewrites(opts),
		);
		for pattern in rewritten {
			if m.register_pattern(&pattern).is_err() {
				return PropertyMatchObservation::not_found();
			}
		}
		let m = m.finish_flat();
		let Some(found) = m.find_best_match(&self.path()) else {
			return PropertyMatchObservation::not_found();
		};
		PropertyMatchObservation {
			found: true,
			pattern: found.pattern.normalized_pattern().to_string(),
			params: found.params,
			splat_values: found.splat_values.iter().map(|s| s.to_string()).collect(),
		}
	}

	fn assert_best_match_matches_model(&self, opts: &Options, source_index: &str) {
		let expected = self.expected_best_match(opts, source_index);
		let actual = self.actual_best_match_observation(
			opts,
			source_index,
			PropertyRegistrationOrder::Original,
		);
		let reversed = self.actual_best_match_observation(
			opts,
			source_index,
			PropertyRegistrationOrder::Reversed,
		);
		let sorted = self.actual_best_match_observation(
			opts,
			source_index,
			PropertyRegistrationOrder::Sorted,
		);

		match expected {
			None => {
				assert!(
					!actual.found && !reversed.found && !sorted.found,
					"patterns = {:?}, path = {:?}, opts = {:?}: expected no best match, got original={actual:?}, reversed={reversed:?}, sorted={sorted:?}",
					self.patterns(),
					self.path(),
					opts
				);
			}
			Some(expected) => {
				for (label, observation) in [
					("original", actual),
					("reversed", reversed),
					("sorted", sorted),
				] {
					assert!(observation.found, "{label}: expected best match");
					assert_eq!(
						observation.pattern,
						expected.normalized_pattern,
						"{label}: patterns = {:?}, path = {:?}, opts = {:?}",
						self.patterns(),
						self.path(),
						opts
					);
					assert_eq!(observation.params, expected.params, "{label}: params");
					assert_eq!(
						observation.splat_values, expected.splat_values,
						"{label}: splat"
					);
				}
			}
		}
	}

	fn best_match_observation_with_extra_patterns(
		&self,
		extra_patterns: Vec<String>,
	) -> PropertyMatchObservation {
		let mut patterns = self.ordered_patterns(PropertyRegistrationOrder::Original);
		patterns.extend(extra_patterns);
		self.actual_best_match_observation_with_patterns(&property_default_opts(), "", patterns)
	}

	fn nested_observation(
		&self,
		opts: &Options,
		source_index: &str,
		order: PropertyRegistrationOrder,
	) -> PropertyNestedObservation {
		self.nested_observation_with_patterns(opts, source_index, self.ordered_patterns(order))
	}

	fn nested_observation_with_patterns(
		&self,
		opts: &Options,
		source_index: &str,
		patterns: Vec<String>,
	) -> PropertyNestedObservation {
		let mut m = MatcherBuilder::new(opts.clone()).unwrap();
		let rewritten = rewrite_patterns(
			&patterns.iter().map(String::as_str).collect::<Vec<_>>(),
			source_index,
			option_shape_for_rewrites(opts),
		);
		for pattern in rewritten {
			if m.register_pattern(&pattern).is_err() {
				return PropertyNestedObservation::not_found();
			}
		}
		let m = m.finish_nested();
		let Some(results) = m.find_nested_matches(&self.path()) else {
			return PropertyNestedObservation::not_found();
		};
		PropertyNestedObservation {
			found: true,
			params: results.params,
			splat_values: results.splat_values.iter().map(|s| s.to_string()).collect(),
			patterns: results
				.matches
				.iter()
				.map(|m| m.pattern.normalized_pattern().to_string())
				.collect(),
			match_params: results.matches.iter().map(|m| m.params().clone()).collect(),
			match_splats: results
				.matches
				.iter()
				.map(|m| m.splat_values().iter().map(|s| s.to_string()).collect())
				.collect(),
		}
	}

	fn nested_observation_with_extra_patterns(
		&self,
		extra_patterns: Vec<String>,
	) -> PropertyNestedObservation {
		let mut patterns = self.ordered_patterns(PropertyRegistrationOrder::Original);
		patterns.extend(extra_patterns);
		self.nested_observation_with_patterns(&property_default_opts(), "", patterns)
	}

	fn assert_nested_registration_order_independent(&self, opts: &Options, source_index: &str) {
		let forward =
			self.nested_observation(opts, source_index, PropertyRegistrationOrder::Original);
		let reversed =
			self.nested_observation(opts, source_index, PropertyRegistrationOrder::Reversed);
		let sorted = self.nested_observation(opts, source_index, PropertyRegistrationOrder::Sorted);

		assert_eq!(
			forward,
			reversed,
			"patterns = {:?}, path = {:?}, opts = {:?}: reversed registration changed nested result",
			self.patterns(),
			self.path(),
			opts
		);
		assert_eq!(
			forward,
			sorted,
			"patterns = {:?}, path = {:?}, opts = {:?}: sorted registration changed nested result",
			self.patterns(),
			self.path(),
			opts
		);
	}

	fn assert_nested_matches_model(&self, opts: &Options, source_index: &str) {
		let expected = self.nested_expectation(opts, source_index);
		for (label, order) in [
			("original", PropertyRegistrationOrder::Original),
			("reversed", PropertyRegistrationOrder::Reversed),
			("sorted", PropertyRegistrationOrder::Sorted),
		] {
			let actual = self.nested_observation(opts, source_index, order);
			assert_eq!(
				expected,
				actual,
				"{label}: patterns = {:?}, path = {:?}, opts = {:?}",
				self.patterns(),
				self.path(),
				opts
			);
		}
	}

	fn nested_expectation(&self, opts: &Options, source_index: &str) -> PropertyNestedObservation {
		if self.path().contains("//") {
			return PropertyNestedObservation::not_found();
		}
		let real_path = property_strip_trailing_slash(&self.path());
		let path_segments = property_path_segments(&real_path);
		let patterns = self.patterns();
		let mut matches = HashMap::new();

		if real_path.is_empty() {
			for pattern in &patterns {
				let model = property_model_pattern(pattern, 0, opts, source_index);
				if model.normalized.is_empty() {
					matches.insert(
						"".to_string(),
						model.base_match(Params::default(), Vec::new()),
					);
					continue;
				}
				if model.normalized == "/" {
					matches.insert(
						"/".to_string(),
						model.base_match(Params::default(), Vec::new()),
					);
				}
			}
			if !matches.contains_key("/") {
				for pattern in &patterns {
					let model = property_model_pattern(pattern, 0, opts, source_index);
					if model.normalized == "/*" {
						matches.insert(
							"/*".to_string(),
							model.base_match(Params::default(), Vec::new()),
						);
						break;
					}
				}
			}
			return property_flatten_nested_matches(&real_path, matches, false);
		}

		let mut found_full_static = false;
		for pattern in &patterns {
			let model = property_model_pattern(pattern, 0, opts, source_index);
			let Some(matched) = model.nested_static_match(&path_segments) else {
				continue;
			};
			if matched.segment_len == path_segments.len()
				&& model.is_static
				&& !model.last_segment_is_index()
			{
				found_full_static = true;
			}
			matches.insert(matched.normalized_pattern.clone(), matched);
		}

		if !found_full_static {
			let mut model_patterns = HashMap::new();
			for pattern in &patterns {
				let model = property_model_pattern(pattern, 0, opts, source_index);
				model_patterns.insert(model.normalized.clone(), model.clone());
				if model.normalized == "/*" {
					matches.insert(
						"/*".to_string(),
						model.base_match(Params::default(), path_segments.clone()),
					);
					continue;
				}
				if model.is_static {
					continue;
				}
				if let Some(matched) = model.nested_dynamic_match(&path_segments) {
					matches.insert(matched.normalized_pattern.clone(), matched);
				}
			}
			// An index claims its parent path on its own shape; the
			// parent pattern need not be registered.
			for (normalized, model) in model_patterns {
				if model.is_static || !model.last_segment_is_index() {
					continue;
				}
				let base_len = model.segments.len() - 1;
				if path_segments.len() != base_len || !model.match_prefix(&path_segments) {
					continue;
				}
				matches.insert(
					normalized,
					model.base_match(model.params(&path_segments), Vec::new()),
				);
			}
		}

		property_flatten_nested_matches(&real_path, matches, true)
	}
}

impl PropertyMatchObservation {
	fn not_found() -> Self {
		Self {
			found: false,
			pattern: String::new(),
			params: Params::default(),
			splat_values: Vec::new(),
		}
	}
}

impl PropertyNestedObservation {
	fn not_found() -> Self {
		Self {
			found: false,
			patterns: Vec::new(),
			params: Params::default(),
			splat_values: Vec::new(),
			match_params: Vec::new(),
			match_splats: Vec::new(),
		}
	}
}

impl PropertyCollisionCase {
	fn pattern_pair(&self, opts: &Options) -> (String, String) {
		let mut left_segments = Vec::with_capacity(self.segments.len());
		let mut right_segments = Vec::with_capacity(self.segments.len());
		for segment in &self.segments {
			match segment.kind {
				PropertyGeneratedSegmentKind::Static => {
					left_segments.push(segment.static_value.clone());
					right_segments.push(segment.static_value.clone());
				}
				PropertyGeneratedSegmentKind::Dynamic => {
					left_segments.push(format!(
						"{}{}",
						opts.dynamic_param_prefix, segment.name_pair.0
					));
					right_segments.push(format!(
						"{}{}",
						opts.dynamic_param_prefix, segment.name_pair.1
					));
				}
				PropertyGeneratedSegmentKind::Splat => {
					left_segments.push(opts.splat_segment_identifier.to_string());
					right_segments.push(opts.splat_segment_identifier.to_string());
				}
				PropertyGeneratedSegmentKind::Index => unreachable!(),
			}
		}
		(
			format!("/{}", left_segments.join("/")),
			format!("/{}", right_segments.join("/")),
		)
	}

	fn assert_route_shape_collision(&self, opts: &Options) {
		let (left, right) = self.pattern_pair(opts);
		for (first, second) in [
			(left.as_str(), right.as_str()),
			(right.as_str(), left.as_str()),
		] {
			let error = property_register_patterns_error_message(opts, &[first, second]);
			assert!(
				error.contains("route shape collision:"),
				"left = {left:?}, right = {right:?}, opts = {opts:?}: error = {error:?}, want route shape collision"
			);
		}
	}
}

impl PropertyNormalizedCollisionCase {
	fn collision_inputs(&self) -> (String, String, Options) {
		match self.kind {
			PropertyNormalizedCollisionKind::ExplicitIndex => (
				String::new(),
				"/".to_string(),
				Options {
					explicit_index_segment_identifier: self.explicit_index_id.to_string(),
					..Options::default()
				},
			),
			PropertyNormalizedCollisionKind::CustomDynamic => (
				format!("{}/$id", self.prefix),
				format!("{}/:id", self.prefix),
				Options {
					dynamic_param_prefix: '$',
					..Options::default()
				},
			),
		}
	}
}

impl PropertyModelPattern {
	fn best_match(&self, path: &str) -> Option<PropertyModelMatch> {
		if self.is_static {
			self.match_static(path)
		} else {
			self.match_dynamic(path)
		}
	}

	fn match_static(&self, path: &str) -> Option<PropertyModelMatch> {
		if self.normalized != path {
			if !property_has_trailing_slash(path) {
				return None;
			}
			if self.normalized != property_strip_trailing_slash(path) {
				return None;
			}
		}
		Some(self.base_match(Params::default(), Vec::new()))
	}

	fn match_dynamic(&self, path: &str) -> Option<PropertyModelMatch> {
		let effective = property_strip_trailing_slash(path);
		let had_trailing = effective.len() != path.len();
		let path_segments = property_path_segments(&effective);

		if effective.is_empty() {
			if self.normalized == "/*" {
				return Some(self.base_match(Params::default(), Vec::new()));
			}
			return None;
		}

		if self.last_segment_is_splat() {
			let splat_idx = self.segments.len() - 1;
			if path_segments.len() < self.segments.len() {
				return None;
			}
			if !self.match_prefix(&path_segments[..splat_idx]) {
				return None;
			}
			return Some(self.base_match(
				self.params(&path_segments),
				path_segments[splat_idx..].to_vec(),
			));
		}

		if self.last_segment_is_index() {
			if !had_trailing {
				return None;
			}
			let base_len = self.segments.len() - 1;
			if path_segments.len() != base_len || !self.match_prefix(&path_segments) {
				return None;
			}
			return Some(self.base_match(self.params(&path_segments), Vec::new()));
		}

		if path_segments.len() == self.segments.len() && self.match_segments(&path_segments) {
			return Some(self.base_match(self.params(&path_segments), Vec::new()));
		}

		None
	}

	fn nested_static_match(&self, path_segments: &[String]) -> Option<PropertyModelMatch> {
		if !self.is_static {
			return None;
		}
		if self.last_segment_is_index() {
			let base_segments = &self.segments[..self.segments.len() - 1];
			if base_segments.len() != path_segments.len() {
				return None;
			}
			if !self.match_prefix(path_segments) {
				return None;
			}
			return Some(self.base_match(Params::default(), Vec::new()));
		}
		if self.segments.len() > path_segments.len() {
			return None;
		}
		if !self.match_prefix(&path_segments[..self.segments.len()]) {
			return None;
		}
		Some(self.base_match(Params::default(), Vec::new()))
	}

	fn nested_dynamic_match(&self, path_segments: &[String]) -> Option<PropertyModelMatch> {
		if self.is_static || self.normalized == "/*" || self.last_segment_is_index() {
			return None;
		}
		if self.last_segment_is_splat() {
			let base_segments = &self.segments[..self.segments.len() - 1];
			if path_segments.len() <= base_segments.len() {
				return None;
			}
			if !self.match_prefix(&path_segments[..base_segments.len()]) {
				return None;
			}
			return Some(self.base_match(
				self.params(path_segments),
				path_segments[base_segments.len()..].to_vec(),
			));
		}
		if self.segments.len() > path_segments.len() {
			return None;
		}
		if !self.match_prefix(&path_segments[..self.segments.len()]) {
			return None;
		}
		Some(self.base_match(self.params(path_segments), Vec::new()))
	}

	fn match_prefix(&self, path_segments: &[String]) -> bool {
		if path_segments.len() > self.segments.len() {
			return false;
		}
		path_segments
			.iter()
			.enumerate()
			.all(|(i, path_segment)| self.segments[i].matches(path_segment))
	}

	fn match_segments(&self, path_segments: &[String]) -> bool {
		path_segments.len() == self.segments.len()
			&& self
				.segments
				.iter()
				.zip(path_segments)
				.all(|(segment, path_segment)| segment.matches(path_segment))
	}

	fn params(&self, path_segments: &[String]) -> Params {
		let mut params = Params::default();
		for (i, segment) in self.segments.iter().enumerate() {
			if segment.kind == PropertySegmentKind::Dynamic {
				params.insert(
					segment.value.trim_start_matches(':'),
					path_segments[i].clone(),
				);
			}
		}
		params
	}

	fn last_segment_is_index(&self) -> bool {
		self.segments
			.last()
			.is_some_and(|s| s.kind == PropertySegmentKind::Index)
	}

	fn last_segment_is_dynamic(&self) -> bool {
		self.segments
			.last()
			.is_some_and(|s| s.kind == PropertySegmentKind::Dynamic)
	}

	fn last_segment_is_splat(&self) -> bool {
		self.segments
			.last()
			.is_some_and(|s| s.kind == PropertySegmentKind::Splat)
	}

	fn shape_key(&self) -> String {
		self.segments
			.iter()
			.map(|segment| match segment.kind {
				PropertySegmentKind::Static => format!("s:{}", segment.value),
				PropertySegmentKind::Dynamic => ":".to_string(),
				PropertySegmentKind::Splat => "*".to_string(),
				PropertySegmentKind::Index => "i".to_string(),
			})
			.collect::<Vec<_>>()
			.join("/")
	}

	fn base_match(&self, params: Params, splat_values: Vec<String>) -> PropertyModelMatch {
		let segment_ranks: Vec<i32> = self
			.segments
			.iter()
			.map(PropertyModelSegment::rank)
			.collect();
		PropertyModelMatch {
			normalized_pattern: self.normalized.clone(),
			params,
			splat_values,
			score: segment_ranks.iter().sum(),
			segment_ranks,
			registration_order: self.registration_order,
			is_static: self.is_static,
			last_is_dynamic: self.last_segment_is_dynamic(),
			last_is_index: self.last_segment_is_index(),
			last_is_splat: self.last_segment_is_splat(),
			segment_len: self.segments.len(),
			dynamic_params: self
				.segments
				.iter()
				.filter(|s| s.kind == PropertySegmentKind::Dynamic)
				.count(),
		}
	}
}

impl PropertyModelSegment {
	fn matches(&self, path_segment: &str) -> bool {
		match self.kind {
			PropertySegmentKind::Static => self.value == path_segment,
			PropertySegmentKind::Index => path_segment.is_empty(),
			PropertySegmentKind::Dynamic => !path_segment.is_empty(),
			PropertySegmentKind::Splat => false,
		}
	}

	fn rank(&self) -> i32 {
		match self.kind {
			PropertySegmentKind::Static | PropertySegmentKind::Index => 2,
			PropertySegmentKind::Dynamic => 1,
			PropertySegmentKind::Splat => 0,
		}
	}
}

impl PropertyModelMatch {
	fn better_than(&self, other: &Self) -> bool {
		if self.is_static != other.is_static {
			return self.is_static;
		}
		if self.score != other.score {
			return self.score > other.score;
		}
		for i in 0..self.segment_ranks.len().min(other.segment_ranks.len()) {
			if self.segment_ranks[i] != other.segment_ranks[i] {
				return self.segment_ranks[i] > other.segment_ranks[i];
			}
		}
		if self.last_is_splat != other.last_is_splat {
			return !self.last_is_splat;
		}
		if self.segment_ranks.len() != other.segment_ranks.len() {
			return self.segment_ranks.len() > other.segment_ranks.len();
		}
		self.registration_order < other.registration_order
	}

	fn last_is_non_root_splat(&self) -> bool {
		self.last_is_splat && self.segment_len > 1
	}
}

fn property_model_pattern(
	pattern: &str,
	registration_order: usize,
	opts: &Options,
	source_index: &str,
) -> PropertyModelPattern {
	let input = rewrite_single_pattern(pattern, source_index, option_shape_for_rewrites(opts));
	let normalized = property_normalized_pattern(&input, opts);
	let segments: Vec<PropertyModelSegment> = property_path_segments(&normalized)
		.into_iter()
		.map(|segment| {
			let kind = if segment.is_empty() {
				PropertySegmentKind::Index
			} else if segment == "*" {
				PropertySegmentKind::Splat
			} else if segment.starts_with(':') {
				PropertySegmentKind::Dynamic
			} else {
				PropertySegmentKind::Static
			};
			PropertyModelSegment {
				value: segment,
				kind,
			}
		})
		.collect();
	let is_static = segments.iter().all(|s| {
		!matches!(
			s.kind,
			PropertySegmentKind::Dynamic | PropertySegmentKind::Splat
		)
	});

	PropertyModelPattern {
		normalized,
		segments,
		registration_order,
		is_static,
	}
}

fn property_normalized_pattern(pattern: &str, opts: &Options) -> String {
	if pattern.is_empty() {
		return String::new();
	}

	let mut normalized = pattern.to_string();
	if !opts.explicit_index_segment_identifier.is_empty() {
		if normalized.ends_with('/') {
			if normalized != "/" {
				return String::new();
			}
			normalized = normalized.trim_end_matches('/').to_string();
		}
		let slash_index = format!("/{}", opts.explicit_index_segment_identifier);
		if normalized.ends_with(&slash_index) {
			let len = normalized.len() - opts.explicit_index_segment_identifier.len();
			normalized.truncate(len);
		}
	}

	let raw_segments = property_path_segments(&normalized);
	let mut segments = Vec::with_capacity(raw_segments.len());
	for raw_segment in raw_segments {
		let (value, kind) = if raw_segment.is_empty() {
			(raw_segment, PropertySegmentKind::Index)
		} else if raw_segment.chars().count() == 1
			&& raw_segment.starts_with(opts.splat_segment_identifier)
		{
			("*".to_string(), PropertySegmentKind::Splat)
		} else if raw_segment.starts_with(opts.dynamic_param_prefix) {
			(
				format!(":{}", &raw_segment[opts.dynamic_param_prefix.len_utf8()..]),
				PropertySegmentKind::Dynamic,
			)
		} else {
			(raw_segment, PropertySegmentKind::Static)
		};
		segments.push(PropertyModelSegment { value, kind });
	}

	let last_type = segments
		.last()
		.map(|s| s.kind)
		.unwrap_or(PropertySegmentKind::Static);
	let mut final_pattern = format!(
		"/{}",
		segments
			.iter()
			.map(|s| s.value.as_str())
			.collect::<Vec<_>>()
			.join("/")
	);
	if final_pattern.ends_with('/') && last_type != PropertySegmentKind::Index {
		while final_pattern.ends_with('/') {
			final_pattern.pop();
		}
	}
	final_pattern
}

fn property_has_trailing_slash(path: &str) -> bool {
	path.ends_with('/') && !path.is_empty()
}

fn property_strip_trailing_slash(path: &str) -> String {
	path.strip_suffix('/').unwrap_or(path).to_string()
}

fn property_path_segments(path: &str) -> Vec<String> {
	if path.is_empty() {
		return Vec::new();
	}
	if path == "/" {
		return vec![String::new()];
	}
	let path = path.strip_prefix('/').unwrap_or(path);
	if let Some(trimmed) = path.strip_suffix('/') {
		let mut parts: Vec<String> = trimmed.split('/').map(ToString::to_string).collect();
		parts.push(String::new());
		return parts;
	}
	path.split('/').map(ToString::to_string).collect()
}

fn property_non_index_segments(path: &str) -> Vec<String> {
	let segments = property_path_segments(path);
	if segments.len() == 1 && segments[0].is_empty() {
		return Vec::new();
	}
	if segments.last().is_some_and(String::is_empty) {
		return segments[..segments.len() - 1].to_vec();
	}
	segments
}

fn property_has_non_root_trailing_slash(path: &str) -> bool {
	path != "/" && property_has_trailing_slash(path)
}

fn property_flatten_nested_matches(
	real_path: &str,
	mut matches: HashMap<String, PropertyModelMatch>,
	prune: bool,
) -> PropertyNestedObservation {
	if prune {
		if matches.contains_key("/*") {
			let real_segment_len = property_path_segments(real_path).len();
			let any_other_covers = matches.iter().any(|(key, matched)| {
				key != "/*"
					&& (matched.last_is_non_root_splat() || matched.segment_len >= real_segment_len)
			});
			if any_other_covers {
				matches.remove("/*");
			} else {
				matches.retain(|key, _| key.is_empty() || key == "/*");
			}
		}
		if matches.len() >= 2 {
			property_prune_nested_matches(real_path, &mut matches);
		}
	}

	let mut results: Vec<PropertyModelMatch> = matches.into_values().collect();
	if results.is_empty() {
		return PropertyNestedObservation::not_found();
	}
	if results.len() > 1 {
		results.sort_by(|a, b| {
			if a.last_is_index != b.last_is_index {
				return if a.last_is_index {
					Ordering::Greater
				} else {
					Ordering::Less
				};
			}
			match a.segment_len.cmp(&b.segment_len) {
				Ordering::Equal => a.normalized_pattern.cmp(&b.normalized_pattern),
				other => other,
			}
		});
	}

	let real_segment_len = property_path_segments(real_path).len();
	if !real_path.is_empty()
		&& real_path != "/"
		&& results.len() == 1
		&& results[0].normalized_pattern.is_empty()
	{
		return PropertyNestedObservation::not_found();
	}

	let last = results.last().unwrap();
	if !last.last_is_non_root_splat()
		&& last.normalized_pattern != "/*"
		&& last.segment_len < real_segment_len
	{
		return PropertyNestedObservation::not_found();
	}

	PropertyNestedObservation {
		found: true,
		params: last.params.clone(),
		splat_values: last.splat_values.clone(),
		patterns: results
			.iter()
			.map(|m| m.normalized_pattern.clone())
			.collect(),
		match_params: results.iter().map(|m| m.params.clone()).collect(),
		match_splats: results.iter().map(|m| m.splat_values.clone()).collect(),
	}
}

fn property_prune_nested_matches(
	real_path: &str,
	matches: &mut HashMap<String, PropertyModelMatch>,
) {
	let mut longest_len = 0usize;
	let mut has_longest_index = false;
	let mut has_longest_dynamic = false;
	let mut has_longest_splat = false;
	for matched in matches.values() {
		if matched.segment_len > longest_len {
			longest_len = matched.segment_len;
			has_longest_index = false;
			has_longest_dynamic = false;
			has_longest_splat = false;
		}
		if matched.segment_len != longest_len {
			continue;
		}
		if matched.last_is_index {
			has_longest_index = true;
		} else if matched.last_is_splat {
			has_longest_splat = true;
		} else if matched.last_is_dynamic {
			has_longest_dynamic = true;
		}
	}

	let shorter_to_remove: Vec<String> = matches
		.iter()
		.filter_map(|(pattern, matched)| {
			if matched.segment_len < longest_len
				&& (matched.last_is_non_root_splat() || matched.last_is_index)
			{
				Some(pattern.clone())
			} else {
				None
			}
		})
		.collect();
	for pattern in shorter_to_remove {
		matches.remove(&pattern);
	}

	let type_count =
		has_longest_index as usize + has_longest_dynamic as usize + has_longest_splat as usize;
	if type_count <= 1 {
		return;
	}

	let real_segment_len = property_path_segments(real_path).len();
	let to_remove: Vec<String> = matches
		.iter()
		.filter_map(|(pattern, matched)| {
			if matched.segment_len != longest_len {
				return None;
			}
			if matched.last_is_index {
				return Some(pattern.clone());
			}
			if real_segment_len == longest_len
				&& has_longest_dynamic
				&& has_longest_splat
				&& matched.last_is_splat
			{
				return Some(pattern.clone());
			}
			if real_segment_len > longest_len
				&& has_longest_dynamic
				&& has_longest_splat
				&& matched.last_is_dynamic
			{
				return Some(pattern.clone());
			}
			None
		})
		.collect();
	for pattern in to_remove {
		matches.remove(&pattern);
	}
}

fn property_register_patterns_error_message(opts: &Options, patterns: &[&str]) -> String {
	let mut m = MatcherBuilder::new(opts.clone()).unwrap();
	for pattern in patterns {
		if let Err(err) = m.register_pattern(pattern) {
			return err;
		}
	}
	String::new()
}

fn property_catalog_contains_all(catalog: &[&str], needles: &[&str]) -> bool {
	needles.iter().all(|needle| catalog.contains(needle))
}

fn property_catalog_indexes(catalog: &[&str], patterns: &[&str]) -> Vec<usize> {
	patterns
		.iter()
		.map(|pattern| {
			catalog
				.iter()
				.position(|candidate| candidate == pattern)
				.expect("pattern exists in catalog")
		})
		.collect()
}

#[test]
fn best_match_model_pattern_normalization_is_unique() {
	let mut seen = HashMap::new();
	for pattern in PROPERTY_BEST_MATCH_PATTERNS {
		let model = property_model_pattern(pattern, 0, &Options::default(), "");
		if let Some(existing) = seen.insert(model.normalized.clone(), *pattern) {
			panic!(
				"patterns {existing:?} and {pattern:?} both normalize to {:?}",
				model.normalized
			);
		}
	}
}

#[test]
fn best_match_model_agrees_with_existing_table_cases() {
	for tc in best_match_cases() {
		if tc.want_pattern != NOT_FOUND && !PROPERTY_BEST_MATCH_PATTERNS.contains(&tc.want_pattern)
		{
			continue;
		}
		if !property_catalog_contains_all(PROPERTY_BEST_MATCH_PATTERNS, tc.patterns) {
			continue;
		}

		let model_tc = PropertyRouteCase {
			catalog: property_catalog(PROPERTY_BEST_MATCH_PATTERNS),
			pattern_indexes: property_catalog_indexes(PROPERTY_BEST_MATCH_PATTERNS, tc.patterns),
			path_segments: property_non_index_segments(tc.path),
			trailing_slash: property_has_non_root_trailing_slash(tc.path),
		};
		let got = model_tc.expected_best_match(&Options::default(), "");
		let want_found = tc.want_pattern != NOT_FOUND;
		assert_eq!(
			got.is_some(),
			want_found,
			"{}: model found mismatch",
			tc.name
		);
		if !want_found {
			continue;
		}
		let got = got.unwrap();
		assert_eq!(got.normalized_pattern, tc.want_pattern, "{}", tc.name);
		assert_eq!(
			got.params,
			tc.want_params.clone().unwrap_or_default(),
			"{}",
			tc.name
		);
		assert_eq!(
			got.splat_values,
			str_vec(tc.want_splat_values),
			"{}",
			tc.name
		);
	}
}

#[test]
fn nested_semantic_model_agrees_with_existing_table_cases() {
	let catalog = NESTED_PATTERNS;
	let pattern_indexes: Vec<usize> = (0..catalog.len()).collect();
	for opts in nested_opts_to_test() {
		let uses_explicit = opts_uses_explicit_index(&opts);
		for tc in nested_scenarios() {
			let model_tc = PropertyRouteCase {
				catalog: property_catalog(catalog),
				pattern_indexes: pattern_indexes.clone(),
				path_segments: property_non_index_segments(tc.path),
				trailing_slash: property_has_non_root_trailing_slash(tc.path),
			};
			let got = model_tc.nested_expectation(&opts, "_index");
			let want_patterns = adjust_expected_matches(tc.expected_matches, uses_explicit);
			let want_found = !want_patterns.is_empty();
			assert_eq!(got.found, want_found, "{} {:?}: model found", tc.path, opts);
			if !want_found {
				continue;
			}
			assert_eq!(got.patterns, want_patterns, "{} {:?}", tc.path, opts);
			assert_eq!(
				got.params,
				tc.params.clone().unwrap_or_default(),
				"{} {:?}: params",
				tc.path,
				opts
			);
			assert_eq!(
				got.splat_values,
				str_vec(tc.splat_values),
				"{} {:?}: splat",
				tc.path,
				opts
			);
		}
	}
}

#[test]
fn find_best_match_matches_property_model() {
	let mut rng = PropertyRng::new(0x2026_0513_0001);
	for _ in 0..PROPERTY_CASES {
		let tc = property_draw_catalog_case(&mut rng, PROPERTY_BEST_MATCH_PATTERNS, 14, 4);
		tc.assert_best_match_matches_model(&property_default_opts(), "");
	}

	let mut rng = PropertyRng::new(0x2026_0513_0002);
	for _ in 0..PROPERTY_CASES {
		let tc = property_draw_catalog_case(&mut rng, PROPERTY_BEST_MATCH_PATTERNS, 14, 4);
		let opts = property_custom_marker_opts(&mut rng);
		tc.assert_best_match_matches_model(&opts, "");
	}

	let mut rng = PropertyRng::new(0x2026_0513_0003);
	for _ in 0..PROPERTY_CASES {
		let tc = property_draw_generated_explicit_index_case(&mut rng, 18, 5, 5);
		let opts = property_explicit_index_opts(&mut rng);
		tc.assert_best_match_matches_model(&opts, "_index");
	}
}

#[test]
fn find_best_match_ignores_unrelated_static_routes() {
	let mut rng = PropertyRng::new(0x2026_0513_0004);
	for _ in 0..PROPERTY_CASES {
		let tc = property_draw_catalog_case(&mut rng, PROPERTY_BEST_MATCH_PATTERNS, 14, 4);
		let extra_patterns =
			property_draw_patterns(&mut rng, PROPERTY_UNRELATED_STATIC_PATTERNS, 8);
		let base = tc.actual_best_match_observation(
			&property_default_opts(),
			"",
			PropertyRegistrationOrder::Original,
		);
		let expanded = tc.best_match_observation_with_extra_patterns(extra_patterns);
		assert_eq!(
			base,
			expanded,
			"patterns = {:?}, path = {:?}",
			tc.patterns(),
			tc.path()
		);
	}
}

#[test]
fn find_best_match_generated_patterns_match_property_model() {
	let mut rng = PropertyRng::new(0x2026_0513_0005);
	for _ in 0..PROPERTY_CASES {
		let tc = property_draw_generated_case(&mut rng, 18, 5, 5);
		tc.assert_best_match_matches_model(&property_default_opts(), "");
	}

	let mut rng = PropertyRng::new(0x2026_0513_0006);
	for _ in 0..PROPERTY_CASES {
		let tc = property_draw_generated_case(&mut rng, 18, 5, 5);
		let opts = property_custom_marker_opts(&mut rng);
		tc.assert_best_match_matches_model(&opts, "");
	}
}

#[test]
fn register_pattern_rejects_generated_route_shape_collisions() {
	let mut rng = PropertyRng::new(0x2026_0513_0007);
	for _ in 0..PROPERTY_CASES {
		let tc = property_draw_collision_case(&mut rng, 5);
		tc.assert_route_shape_collision(&property_default_opts());
	}

	let mut rng = PropertyRng::new(0x2026_0513_0008);
	for _ in 0..PROPERTY_CASES {
		let tc = property_draw_collision_case(&mut rng, 5);
		let opts = rng.sampled_clone(&[
			Options {
				dynamic_param_prefix: '$',
				..Options::default()
			},
			Options {
				dynamic_param_prefix: '<',
				splat_segment_identifier: '>',
				..Options::default()
			},
		]);
		tc.assert_route_shape_collision(&opts);
	}
}

#[test]
fn register_pattern_rejects_generated_normalized_collisions() {
	let mut rng = PropertyRng::new(0x2026_0513_0009);
	for _ in 0..PROPERTY_CASES {
		let tc = property_draw_normalized_collision_case(&mut rng);
		let (first, second, opts) = tc.collision_inputs();
		let error = property_register_patterns_error_message(&opts, &[&first, &second]);
		assert!(
			error.contains("normalized pattern collision:"),
			"first = {first:?}, second = {second:?}, opts = {opts:?}: error = {error:?}"
		);
	}
}

#[test]
fn register_pattern_rejects_invalid_explicit_index_trailing_slash_patterns() {
	let mut rng = PropertyRng::new(0x2026_0513_0010);
	for _ in 0..PROPERTY_CASES {
		let (pattern, opts) = property_draw_invalid_explicit_index_case(&mut rng);
		let error = property_register_patterns_error_message(&opts, &[&pattern]);
		assert!(
			error.contains(
				"trailing slashes are not permitted when using an explicit index segment"
			),
			"pattern = {pattern:?}, opts = {opts:?}: error = {error:?}"
		);
	}
}

#[test]
fn find_nested_matches_registration_order_independent() {
	let mut rng = PropertyRng::new(0x2026_0513_0011);
	for _ in 0..PROPERTY_CASES {
		let tc = property_draw_catalog_case(&mut rng, PROPERTY_BEST_MATCH_PATTERNS, 14, 4);
		tc.assert_nested_registration_order_independent(&property_default_opts(), "");
	}

	let mut rng = PropertyRng::new(0x2026_0513_0012);
	for _ in 0..PROPERTY_CASES {
		let tc = property_draw_catalog_case(&mut rng, PROPERTY_NESTED_MATCH_PATTERNS, 18, 5);
		tc.assert_nested_registration_order_independent(&property_default_opts(), "");
	}

	let mut rng = PropertyRng::new(0x2026_0513_0013);
	for _ in 0..PROPERTY_CASES {
		let tc = property_draw_catalog_case(&mut rng, PROPERTY_NESTED_MATCH_PATTERNS, 18, 5);
		let opts = property_custom_marker_opts(&mut rng);
		tc.assert_nested_registration_order_independent(&opts, "");
	}

	let mut rng = PropertyRng::new(0x2026_0513_0014);
	for _ in 0..PROPERTY_CASES {
		let tc =
			property_draw_catalog_case(&mut rng, PROPERTY_NESTED_EXPLICIT_INDEX_PATTERNS, 18, 5);
		let opts = property_explicit_index_opts(&mut rng);
		tc.assert_nested_registration_order_independent(&opts, "_index");
	}

	let mut rng = PropertyRng::new(0x2026_0513_0015);
	for _ in 0..PROPERTY_CASES {
		let tc = property_draw_generated_explicit_index_case(&mut rng, 18, 5, 5);
		let opts = property_explicit_index_opts(&mut rng);
		tc.assert_nested_registration_order_independent(&opts, "_index");
	}
}

#[test]
fn find_nested_matches_ignores_unrelated_static_routes() {
	let mut rng = PropertyRng::new(0x2026_0513_0016);
	for _ in 0..PROPERTY_CASES {
		let tc = property_draw_catalog_case(&mut rng, PROPERTY_NESTED_MATCH_PATTERNS, 18, 5);
		let extra_patterns =
			property_draw_patterns(&mut rng, PROPERTY_UNRELATED_STATIC_PATTERNS, 8);
		let base = tc.nested_observation(
			&property_default_opts(),
			"",
			PropertyRegistrationOrder::Original,
		);
		let expanded = tc.nested_observation_with_extra_patterns(extra_patterns);
		assert_eq!(
			base,
			expanded,
			"patterns = {:?}, path = {:?}",
			tc.patterns(),
			tc.path()
		);
	}
}

#[test]
fn find_nested_matches_matches_semantic_model() {
	let mut rng = PropertyRng::new(0x2026_0513_0017);
	for _ in 0..PROPERTY_CASES {
		let tc = property_draw_catalog_case(&mut rng, PROPERTY_BEST_MATCH_PATTERNS, 14, 4);
		tc.assert_nested_matches_model(&property_default_opts(), "");
	}

	let mut rng = PropertyRng::new(0x2026_0513_0018);
	for _ in 0..PROPERTY_CASES {
		let tc = property_draw_catalog_case(&mut rng, PROPERTY_NESTED_MATCH_PATTERNS, 18, 5);
		tc.assert_nested_matches_model(&property_default_opts(), "");
	}

	let mut rng = PropertyRng::new(0x2026_0513_0019);
	for _ in 0..PROPERTY_CASES {
		let tc = property_draw_generated_case(&mut rng, 18, 5, 5);
		tc.assert_nested_matches_model(&property_default_opts(), "");
	}

	let mut rng = PropertyRng::new(0x2026_0513_0020);
	for _ in 0..PROPERTY_CASES {
		let tc = property_draw_catalog_case(&mut rng, PROPERTY_NESTED_MATCH_PATTERNS, 18, 5);
		let opts = property_custom_marker_opts(&mut rng);
		tc.assert_nested_matches_model(&opts, "");
	}

	let mut rng = PropertyRng::new(0x2026_0513_0021);
	for _ in 0..PROPERTY_CASES {
		let tc = property_draw_generated_case(&mut rng, 18, 5, 5);
		let opts = property_custom_marker_opts(&mut rng);
		tc.assert_nested_matches_model(&opts, "");
	}

	let mut rng = PropertyRng::new(0x2026_0513_0022);
	for _ in 0..PROPERTY_CASES {
		let tc =
			property_draw_catalog_case(&mut rng, PROPERTY_NESTED_EXPLICIT_INDEX_PATTERNS, 18, 5);
		let opts = property_explicit_index_opts(&mut rng);
		tc.assert_nested_matches_model(&opts, "_index");
	}

	let mut rng = PropertyRng::new(0x2026_0513_0023);
	for _ in 0..PROPERTY_CASES {
		let tc = property_draw_generated_explicit_index_case(&mut rng, 18, 5, 5);
		let opts = property_explicit_index_opts(&mut rng);
		tc.assert_nested_matches_model(&opts, "_index");
	}
}
