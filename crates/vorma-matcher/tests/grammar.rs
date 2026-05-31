use std::collections::HashMap;

use vorma_matcher::{MatcherBuilder, Options, Params, parse_segments};

fn params(entries: &[(&str, &str)]) -> Params {
	let mut out = HashMap::new();
	for (k, v) in entries {
		out.insert((*k).to_string(), (*v).to_string());
	}
	out
}

fn str_vec(values: &[&str]) -> Vec<String> {
	values.iter().map(|v| (*v).to_string()).collect()
}

#[test]
fn parse_segments_has_stable_path_splitting_semantics() {
	let cases = [
		("empty path", "", vec![]),
		("root path", "/", vec![""]),
		("simple path", "/users", vec!["users"]),
		(
			"multi-segment path",
			"/api/v1/users",
			vec!["api", "v1", "users"],
		),
		("trailing slash", "/users/", vec!["users", ""]),
		(
			"path with parameters",
			"/users/:id/posts",
			vec!["users", ":id", "posts"],
		),
		(
			"path with parameters, implicit index segment",
			"/users/:id/posts/",
			vec!["users", ":id", "posts", ""],
		),
		(
			"path with parameters, explicit index segment",
			"/users/:id/posts/_index",
			vec!["users", ":id", "posts", "_index"],
		),
		("path with splat", "/files/*", vec!["files", "*"]),
		("multiple slashes", "//api///users", vec!["api", "users"]),
		(
			"complex path",
			"/api/v1/users/:user_id/posts/:post_id/comments",
			vec![
				"api", "v1", "users", ":user_id", "posts", ":post_id", "comments",
			],
		),
		(
			"unicode path",
			"/café/über/resumé",
			vec!["café", "über", "resumé"],
		),
	];

	for (name, path, expected) in cases {
		assert_eq!(parse_segments(path), str_vec(&expected), "{name}");
	}
}

#[test]
fn builder_exposes_normalized_pattern_metadata() {
	let default = MatcherBuilder::new(Options::default()).unwrap();
	assert_eq!(default.dynamic_param_prefix(), ':');
	assert_eq!(default.splat_segment_identifier(), '*');
	assert_eq!(default.explicit_index_segment_identifier(), "");

	let custom = MatcherBuilder::new(Options {
		dynamic_param_prefix: '$',
		splat_segment_identifier: '#',
		..Options::default()
	})
	.unwrap();
	assert_eq!(custom.dynamic_param_prefix(), '$');
	assert_eq!(custom.splat_segment_identifier(), '#');
	let rp = custom.normalize_pattern("/users/$id/#").unwrap();
	assert_eq!(rp.normalized_pattern(), "/users/:id/*");

	let explicit = MatcherBuilder::new(Options {
		dynamic_param_prefix: '@',
		splat_segment_identifier: '#',
		explicit_index_segment_identifier: "_index".to_string(),
	})
	.unwrap();
	assert_eq!(explicit.dynamic_param_prefix(), '@');
	assert_eq!(explicit.splat_segment_identifier(), '#');
	assert_eq!(explicit.explicit_index_segment_identifier(), "_index");
}

#[test]
fn builder_rejects_invalid_route_pattern_grammar() {
	for (pattern, expected) in [
		("relative/:id", "pattern must be absolute"),
		("/:", "param name cannot be empty"),
		("/:123_bad", "must start with an ASCII letter or underscore"),
		(
			"/:user-id",
			"must contain only ASCII letters, digits, or underscores",
		),
		("/:id/:id", "duplicate param name \"id\""),
		("/files/*/edit", "splat segment must be the final segment"),
		("/files/*/", "splat segment must be the final segment"),
		("/a//b", "empty middle segments are not permitted"),
	] {
		let mut builder = MatcherBuilder::new(Options::default()).unwrap();
		let error = builder.register_pattern(pattern).unwrap_err();
		assert!(
			error.contains(expected),
			"pattern = {pattern:?}: error = {error:?}, expected substring = {expected:?}"
		);
	}
}

#[test]
fn builder_rejects_ambiguous_segment_markers() {
	for (opts, expected) in [
		(
			Options {
				dynamic_param_prefix: '/',
				..Options::default()
			},
			"dynamic param prefix cannot be a slash",
		),
		(
			Options {
				splat_segment_identifier: '/',
				..Options::default()
			},
			"splat segment identifier cannot be a slash",
		),
		(
			Options {
				dynamic_param_prefix: '*',
				..Options::default()
			},
			"dynamic param prefix and splat segment identifier must differ",
		),
		(
			Options {
				explicit_index_segment_identifier: ":index".to_owned(),
				..Options::default()
			},
			"explicit index segment cannot be classified as dynamic or splat",
		),
		(
			Options {
				explicit_index_segment_identifier: "*".to_owned(),
				..Options::default()
			},
			"explicit index segment cannot be classified as dynamic or splat",
		),
	] {
		let error = MatcherBuilder::new(opts).unwrap_err();
		assert!(
			error.contains(expected),
			"error = {error:?}, expected substring = {expected:?}"
		);
	}
}

#[test]
fn builder_rejects_cross_pattern_collisions() {
	let mut builder = MatcherBuilder::new(Options {
		dynamic_param_prefix: '$',
		..Options::default()
	})
	.unwrap();
	builder.register_pattern("/users/$id").unwrap();
	assert_eq!(
		builder.register_pattern("/users/:id").unwrap_err(),
		r#"normalized pattern collision: "/users/:id" and "/users/$id" both normalize to "/users/:id""#
	);

	let mut builder = MatcherBuilder::new(Options::default()).unwrap();
	let first = builder.register_pattern("/users/:id").unwrap();
	let second = builder.register_pattern("/users/:id").unwrap();
	assert_eq!(second.original_pattern(), first.original_pattern());
	assert_eq!(second.normalized_pattern(), first.normalized_pattern());

	let matcher = builder.finish();
	let found = matcher.find_best_match("/users/42").unwrap();
	assert_eq!(found.params, params(&[("id", "42")]));

	let mut builder = MatcherBuilder::new(Options {
		splat_segment_identifier: '#',
		..Options::default()
	})
	.unwrap();
	builder.register_pattern("/files/#").unwrap();
	assert_eq!(
		builder.register_pattern("/files/*").unwrap_err(),
		r#"normalized pattern collision: "/files/*" and "/files/#" both normalize to "/files/*""#
	);

	for (name, first, second, expected) in [
		(
			"single dynamic segment",
			"/users/:id",
			"/users/:slug",
			r#"route shape collision: "/users/:slug" and "/users/:id" both match the same paths"#,
		),
		(
			"dynamic plus splat",
			"/:section/*",
			"/:page/*",
			r#"route shape collision: "/:page/*" and "/:section/*" both match the same paths"#,
		),
	] {
		let mut builder = MatcherBuilder::new(Options::default()).unwrap();
		builder.register_pattern(first).unwrap();
		assert_eq!(
			builder.register_pattern(second).unwrap_err(),
			expected,
			"{name}"
		);
	}
}
