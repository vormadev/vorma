# Rough Status Update

Status: Active  
Last Updated: 2026-02-09  
Purpose: Single rolling snapshot of spec-program reality (done/not-done/next/confidence). This file must be updated after every major audit or ownership-change pass.

## Current Metrics

- Package specs: `65` total (`12` non-placeholder, `53` placeholder).
- Package checklists with open items: `65/65`.
- Open conformance issue rows: `114` total (`60` non-placeholder, `54` placeholder).

## Done

- Package-path spec structure is established under `specs/packages/<package>/`.
- Top-level governance is centralized in `specs/SPEC_GOVERNANCE.md`.
- Redundant `VORMA_*` and `WAVE_*` duplicate file layers were removed in favor of canonical package files.
- `vorma` scope was reduced to wrapper/facade contracts.
- Wave ownership split is now explicit and package-correct:
  - `specs/packages/wave/tooling/*` owns build/dev `WAVE-CLI/DEV/EVT/STATIC/CSS/SCHEMA` contracts.
  - `specs/packages/wave/*` owns runtime `WAVE-RT-*` contracts.
- `wave/tooling` ownership migration now includes rough replay accounting (ledger/checklist/tracker updated; full pass still open).
- `vormabuild` references were redirected to `specs/packages/wave/tooling/*` for build/dev owner behavior.
- `vormaruntime` Wave references were cleaned (removed accidental duplicate runtime link; build/dev link now points at `wave/tooling`).
- `vorma` wrapper package replay was tightened: full structural/boundary/semantic pass items are complete, and coverage gap tracking is explicit (`VORMA-ISSUE-001`).
- Path hygiene rule is enforced in docs (repo-relative paths only).

## Not Done

- No package has reached full closure (full audit + two consecutive no-gap full rounds).
- Most packages are still placeholder-level.
- Open conformance/intent-validation issues remain across key packages.
- `wave` runtime requirements are now explicit but still mostly source-only (coverage gap tracked as `WCI-RT-001`).
- `vorma` still has unresolved wrapper coverage gap (`VORMA-ISSUE-001`), so no-gap rounds are not yet possible.

## Next (Priority Order)

1. Continue full-audit replay for P0 packages in this order:
   - `vorma`
   - `vormaruntime`
   - `vormabuild`
   - `vormaclient/client`
   - `wave/tooling`
   - `wave`
   - `kit/matcher`, `kit/mux`, `kit/response`, `kit/validate`, `kit/headels`
   - `lab/tsgen`, `lab/viteutil`
2. For each package, convert rough pass state to full pass state:
   - detailed requirement reconciliation,
   - traceability reconciliation,
   - conformance issue normalization,
   - two no-gap full rounds.
3. Keep low-priority bootstrap-related scope deferred:
   - `bootstrap`
   - `vormaclient/create`

## Clean vs Dirty

Clean:

- Package folderization and per-package artifact set are in place.
- Canonical governance and audit-definition rules are written.
- Major Wave owner-boundary mismatch is fixed (`wave` vs `wave/tooling`).

Dirty:

- Working tree remains heavily dirty with many uncommitted spec edits/deletions.
- Placeholder volume remains high across non-P0 package specs.
- Open issue volume remains high; most package checklists are still mid-replay.

## Confidence

High confidence:

- Structural status (files/folders/governance) and owner-boundary mapping for Vorma/Wave package splits.

Medium confidence:

- Correctness of moved Wave build/dev catalogs in new `wave/tooling` owner path (structure is correct; full source revalidation still pending).

Low confidence:

- Detailed correctness/completeness of package catalogs that remain placeholder or mid-replay.
