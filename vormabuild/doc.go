// Package vormabuild contains Vorma build and dev entrypoints.
//
// This package is build-time only and is intended for build commands (for
// example, ./cmd/build). Do not import it from runtime request-serving code.
// Keeping build-time orchestration out of production binaries avoids pulling in
// unnecessary tooling dependencies and keeps app binaries smaller.
package vormabuild
