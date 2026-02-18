// Package vorma exposes the public application-facing API for building Vorma
// apps.
//
// Runtime internals live under internal/vormaruntime and are surfaced here via
// a stable facade (`Vorma`) plus typed loader/action registration helpers.
//
// Build-time orchestration and code generation are intentionally separated into
// package vormabuild.
package vorma
