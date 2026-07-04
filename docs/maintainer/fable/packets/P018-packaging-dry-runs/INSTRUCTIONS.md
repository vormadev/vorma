# P018 — Packaging and supply-chain dry-runs

Read `docs/maintainer/fable/README.md` (the packet protocol binds you), then
`LEARNINGS.md`, `STATE.md`, repo-root `AGENTS.md`, `docs/maintainer/REMINDERS.md`.

## Context

The release endgame needs proven packaging. `make rust-package` is known green (P001); the
TS publish path has never been dry-run; the dependency policy has run only as the gate's
pass/fail. This packet proves the publish paths and audits the supply chain as an
exhaustive review, not a gate bit.

## Scope

1. **Rust packaging:** `make rust-package` re-verified; then inspect the built packages'
   contents (file lists) for anything that should not ship (maintainer docs, test
   fixtures, local artifacts) or is missing (READMEs, licenses); verify each crate's
   manifest metadata (description, license, repository, keywords) is publish-ready and
   consistent.
2. **TS publish dry-run:** the xtask `ts-publish` path exists — determine its dry-run form
   (or construct the equivalent: `pnpm publish --dry-run` per package with the real .dist
   build) WITHOUT any network publish; inspect the would-be-published file lists the same
   way (nothing missing, nothing extra, package.json metadata publish-ready, the wasm
   artifact included).
3. **Supply-chain review:** `cargo deny` + `cargo audit` as an exhaustive read (every
   dependency: license, source, maintenance signal), plus the TS dependency tree's
   equivalent scan; report the full inventory with anything notable (the known allowed
   RUSTSEC-2026-0190 included for completeness). Findings are recorded, not fixed.
4. Version/metadata coherence: workspace versions consistent; the
   `simple-release-auto-bumper` ticket referenced if versioning friction appears
   (reference, not duplicate).

## Hard constraints

- ABSOLUTELY NO PUBLISHING: dry-run forms only; no network writes of any kind; no registry
  authentication attempted. Network READS needed by cargo/pnpm dry-run machinery are
  permitted; installs are not.
- No source changes except publish-metadata fixes to manifests/package.json IF mechanical
  and obviously correct (missing license field class) — disclosed individually; anything
  judgment-shaped is a finding.
- No git actions; scoped fmt writes only; unexpectedly dirty unowned files are
  escalations, never cleanup.

## Definition of done

- Both publish paths proven in dry-run with contents inspected and verdicts recorded; the
  supply-chain inventory delivered; any metadata fixes disclosed; gates untouched-green
  (workspace tests, clippy, fmt).
- REPORT.md content delivered IN the final message body.
