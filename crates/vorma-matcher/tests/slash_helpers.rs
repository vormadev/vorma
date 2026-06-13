//! Pins for the public slash helpers, ported from the Go reference's
//! helper-function conformance cases.

use vorma_matcher::{
	ensure_leading_and_trailing_slash, ensure_leading_slash, ensure_trailing_slash,
	strip_leading_slash, strip_trailing_slash,
};

#[test]
fn slash_helpers_handle_absolute_relative_and_trailing_forms() {
	assert_eq!(ensure_leading_slash("/users"), "/users");
	assert_eq!(ensure_leading_slash("users"), "/users");
	assert_eq!(ensure_trailing_slash("/users"), "/users/");
	assert_eq!(ensure_trailing_slash("/users/"), "/users/");
	assert_eq!(strip_leading_slash("/users"), "users");
	assert_eq!(strip_leading_slash("users"), "users");
	assert_eq!(strip_trailing_slash("/users/"), "/users");
	assert_eq!(strip_trailing_slash("/users"), "/users");
	assert_eq!(ensure_leading_and_trailing_slash("users"), "/users/");
	assert_eq!(ensure_leading_and_trailing_slash("/users/"), "/users/");
}

// Each helper strips or guarantees exactly one slash; doubled separators
// are preserved, never collapsed.
#[test]
fn slash_helpers_touch_exactly_one_slash() {
	assert_eq!(strip_leading_slash("//users"), "/users");
	assert_eq!(strip_trailing_slash("/users//"), "/users/");
	assert_eq!(ensure_trailing_slash("/users//"), "/users//");
	assert_eq!(ensure_leading_slash("//users"), "//users");
}

#[test]
fn slash_helpers_handle_empty_and_root() {
	assert_eq!(ensure_leading_slash(""), "/");
	assert_eq!(ensure_trailing_slash(""), "/");
	assert_eq!(ensure_leading_and_trailing_slash(""), "/");
	assert_eq!(ensure_leading_and_trailing_slash("/"), "/");
	assert_eq!(strip_leading_slash("/"), "");
	assert_eq!(strip_trailing_slash("/"), "");
	assert_eq!(strip_leading_slash(""), "");
	assert_eq!(strip_trailing_slash(""), "");
}
