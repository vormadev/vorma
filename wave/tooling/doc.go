// Package tooling contains Wave build-time and development-time orchestration.
//
// It is intentionally separate from package wave runtime APIs so production
// binaries can depend on runtime functionality without pulling in build/dev
// tool dependencies.
//
// Major responsibilities include:
// - static asset processing and file mapping
// - CSS/Vite build integration
// - devserver lifecycle, watch pipelines, and restart orchestration
// - config validation and schema generation
package tooling
