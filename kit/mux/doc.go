// Package mux provides typed HTTP task routing with pattern matching,
// middleware, and nested route execution.
//
// It is optimized for Vorma/Wave runtime use-cases where handlers are modeled
// as typed tasks instead of untyped `http.HandlerFunc` chains. The package
// supports:
// - request input decoding into typed structs
// - nested pattern routing with params/splats
// - task middleware and HTTP middleware composition
// - deterministic task execution and result aggregation
//
// Package mux is framework-agnostic and can be used directly outside Vorma.
package mux
