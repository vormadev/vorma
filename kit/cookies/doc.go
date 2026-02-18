// Package cookies provides secure cookie helpers and policy-driven defaults.
//
// The package centers around `Manager`, which encapsulates signing/encryption
// key access and environment-aware defaults (SameSite, partitioning, HttpOnly).
// Callers choose explicit cookie config types for host-only vs non-host-only
// behavior and client-readable vs secure cookies.
//
// This package is intended to make secure defaults easy while keeping each
// cookie's behavior explicit at call sites.
package cookies
