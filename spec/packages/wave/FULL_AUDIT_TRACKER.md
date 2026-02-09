# wave Full Audit Tracker

Status: In Progress  
Last Updated: 2026-02-09  
Purpose: From-scratch package-owned audit for Wave runtime specs and ledger.

## Boundary Statement

- `wave` package specs define runtime-owner behavior implemented in `wave/*.go`.
- Build/dev control-plane behavior is owned by `wave/tooling`.
- Consumer packages reference owner requirements and do not duplicate owner
  internals.

## Artifact Pass Board

| Artifact                                        | Structural  | Boundary    | Semantic    | Last Updated | Notes                                                                                                                       |
| ----------------------------------------------- | ----------- | ----------- | ----------- | ------------ | --------------------------------------------------------------------------------------------------------------------------- |
| `spec/packages/wave/SPEC.md`                    | in_progress | in_progress | in_progress | 2026-02-09   | Rewritten to runtime-owner `WAVE-RT-*` catalog; full replay reconciliation pending.                                         |
| `spec/packages/wave/TRACEABILITY_MATRIX.md`     | in_progress | in_progress | n/a         | 2026-02-09   | Runtime-only rows authored; evidence is currently source-only where no legacy tests outside `conformance/**` exist in-repo. |
| `spec/packages/wave/CONFORMANCE_ISSUES.md`      | in_progress | in_progress | n/a         | 2026-02-09   | No active package-level issues; legacy-test absence is documented as source-only evidence state in matrix/ledger artifacts. |
| `spec/packages/wave/SPEC_CHECKLIST.md`          | in_progress | n/a         | n/a         | 2026-02-09   | Rough pass complete; full pass items open.                                                                                  |
| `spec/packages/wave/NORMATIVE_INTENT_LEDGER.md` | in_progress | n/a         | n/a         | 2026-02-09   | Epoch E2 per-file counters updated for runtime-owner sweep.                                                                 |

## Round Status

- Active round: `E2-R1`.
- Stop criterion: not met.
