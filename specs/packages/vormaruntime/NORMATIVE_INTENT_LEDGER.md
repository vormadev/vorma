# vormaruntime Normative Intent Ledger

Status: Active  
Last Updated: 2026-02-09  
Trust Epoch: `E2`

## Mining Rules

- Intent mining MUST use implementation source plus legacy tests outside `conformance/**`.
- Per-file counters are epoch-scoped.
- `passes`: total pass count in current epoch.
- `clean_passes`: no-new-gap pass count in current epoch.

## Per-File Replay Ledger

| File | Scope | Mining Inputs | Passes | Clean Passes | Epoch State | Notes |
|---|---|---|---:|---:|---|---|
| `vormaruntime/errors.go` | in-scope | source+legacy-tests | 1 | 0 | mined_gaps_open | E2-R1 source replay completed; legacy tests outside `conformance/**` were not found. |
| `vormaruntime/get_deps.go` | in-scope | source+legacy-tests | 1 | 0 | mined_gaps_open | E2-R1 source replay completed; legacy tests outside `conformance/**` were not found. |
| `vormaruntime/get_root_handler.go` | in-scope | source+legacy-tests | 3 | 0 | mined_gaps_open | E2-R1 source replay completed; E2-R2 boundary replay collapsed duplicated proxy-merge internals into owner-reference contracts (`BR-PROXY-001`, removed `BR-PROXY-004/005`) and removed duplicate HTML cache-control requirement (`BR-HTML-005`) already covered by `BR-RESP-003`; legacy tests outside `conformance/**` were not found. |
| `vormaruntime/glue.go` | in-scope | source+legacy-tests | 1 | 0 | mined_gaps_open | E2-R1 source replay completed; legacy tests outside `conformance/**` were not found. |
| `vormaruntime/gmpd.go` | in-scope | source+legacy-tests | 2 | 0 | mined_gaps_open | E2-R1 source replay completed; E2-R2 boundary replay collapsed duplicated matcher internals into owner-reference contracts (`BR-LOAD-002/016/017`); legacy tests outside `conformance/**` were not found. |
| `vormaruntime/paths.go` | in-scope | source+legacy-tests | 2 | 0 | mined_gaps_open | E2-R1 source replay completed; E2-R2 added path-helper requirement `BR-INIT-013`; legacy tests outside `conformance/**` were not found. |
| `vormaruntime/route_registry.go` | in-scope | source+legacy-tests | 1 | 0 | mined_gaps_open | E2-R1 source replay completed; legacy tests outside `conformance/**` were not found. |
| `vormaruntime/route_reload.go` | in-scope | source+legacy-tests | 2 | 0 | mined_gaps_open | E2-R1 source replay completed; E2-R2 added direct reload requirements `BR-DEV-010` and `BR-DEV-011`; legacy tests outside `conformance/**` were not found. |
| `vormaruntime/ssr.go` | in-scope | source+legacy-tests | 1 | 0 | mined_gaps_open | E2-R1 source replay completed; legacy tests outside `conformance/**` were not found. |
| `vormaruntime/types.go` | in-scope | source+legacy-tests | 1 | 0 | mined_gaps_open | E2-R1 source replay completed; legacy tests outside `conformance/**` were not found. |
| `vormaruntime/vite_url.go` | in-scope | source+legacy-tests | 1 | 0 | mined_gaps_open | E2-R1 source replay completed; legacy tests outside `conformance/**` were not found. |
| `vormaruntime/vorma_core.go` | in-scope | source+legacy-tests | 2 | 0 | mined_gaps_open | E2-R1 source replay completed; E2-R2 added mode-toggle requirement `BR-INIT-014`; legacy tests outside `conformance/**` were not found. |
| `vormaruntime/vorma_init.go` | in-scope | source+legacy-tests | 1 | 0 | mined_gaps_open | E2-R1 source replay completed; legacy tests outside `conformance/**` were not found. |

## Round Log

| Round | Status | New Gaps | Notes |
|---|---|---:|---|
| `E2-R1` | completed | 3 | Baseline replay found: (1) traceability-evidence drift (matrix had relied on `conformance/**` outputs despite no legacy tests outside `conformance/**`), (2) non-owner Wave duplication in runtime catalog (`BR-ASSET-*` and `BR-STATIC-002..005`), and (3) stale/non-existent runtime API claim (`Vorma.FaviconRedirect()`). Catalog and matrix were reconciled; `VRI-001` remains open. |
| `E2-R2` | in_progress | 7 | Follow-up replay added source-backed gaps for direct reload/path helper/mode-toggle APIs (`BR-DEV-010`, `BR-DEV-011`, `BR-INIT-013`, `BR-INIT-014`), fixed matcher-ownership duplication by rewriting `BR-LOAD-002/016/017` as owner-reference inheritance contracts, removed duplicated proxy-merge internals (`BR-PROXY-004/005`) in favor of `kit/response` owner references, and removed duplicate HTML cache-control requirement (`BR-HTML-005`) already covered by `BR-RESP-003`; stop criterion remains blocked by open intent-validation issues (including `VRI-001`). |
