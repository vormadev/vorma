# Open Items

Scope: `wave/*`, `internal/vormaruntime/*`, `kit/mux/*`, `vormabuild/*`,
`vormagogen/*`

## Active TODOs

- [ ] `internal/vormaruntime`: final meaningful deconstruction pass on
      `vormaruntime.go` for remaining burdensome root-owned concerns.
- [ ] `internal/vormaruntime`: re-audit root test ownership after final split;
      keep integration/orchestration tests in root and move pure logic tests to
      owning packages.
- [ ] `internal/vormaruntime`: identify any additional package boundary issues
      that still violate the meaningful split spirit and correct them.
- [ ] `vormabuild`: gate before further deconstruction: audit current tests for
      ownership/location correctness (root vs subpackages) and move tests to
      owning packages first.
- [ ] `vormabuild`: produce explicit ownership audit outputs: move list, keep
      list, and missing-package test list.
- [ ] `vormabuild`: continue meaningful one-file-per-package deconstruction from
      audited ownership baseline.
