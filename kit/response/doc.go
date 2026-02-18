// Package response provides thin, explicit HTTP response helpers.
//
// The `Response` type wraps `http.ResponseWriter` and centralizes common
// response patterns (JSON, text, HTML, redirects, status/error helpers) while
// still exposing straightforward HTTP semantics.
//
// Package response intentionally avoids hidden policy and keeps behavior
// predictable so higher-level runtime layers can compose it safely.
package response
