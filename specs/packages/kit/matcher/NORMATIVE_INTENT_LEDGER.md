# kit/matcher Normative Intent Ledger

Status: Active  
Last Updated: 2026-02-09  
Trust Epoch: `E2`

## Mining Rules

- Intent mining MUST use both implementation source and existing tests.
- Per-file counters are epoch-scoped.
- `passes`: total pass count in current epoch.
- `clean_passes`: no-new-gap pass count in current epoch.

## Per-File Replay Ledger

| File | Scope | Mining Inputs | Passes | Clean Passes | Epoch State | Notes |
|---|---|---|---:|---:|---|---|
| `kit/matcher/README.md` | out-of-scope | n/a | 0 | 0 | out-of-scope | Documentation/benchmark artifact. |
| `kit/matcher/bench.txt` | out-of-scope | n/a | 0 | 0 | out-of-scope | Documentation/benchmark artifact. |
| `kit/matcher/find_best_match.go` | in-scope | source+tests | 1 | 0 | mined_gaps_open | `E2-R1` full-pass reconciled; tied to `KIT-MATCHER-015..024` and open partial/source-backed issue set. |
| `kit/matcher/find_best_match_test.go` | in-scope | source+tests | 1 | 0 | mined_gaps_open | `E2-R1` full-pass reconciled; scenario coverage mapped into requirement-level matrix rows. |
| `kit/matcher/find_nested_matches.go` | in-scope | source+tests | 1 | 0 | mined_gaps_open | `E2-R1` full-pass reconciled; nested pruning/ordering/validity branches mapped with issue-backed partials. |
| `kit/matcher/find_nested_matches_test.go` | in-scope | source+tests | 1 | 0 | mined_gaps_open | `E2-R1` full-pass reconciled; deterministic-order and invalid-match cases mapped to requirements. |
| `kit/matcher/matcher.go` | in-scope | source+tests | 1 | 0 | mined_gaps_open | `E2-R1` full-pass reconciled; option default/validation and API contract issues tracked in `KIT-MATCHER-ISSUE-001`. |
| `kit/matcher/parse_segments.go` | in-scope | source+tests | 1 | 0 | mined_gaps_open | `E2-R1` full-pass reconciled; parser contract mapped to `KIT-MATCHER-001..003`. |
| `kit/matcher/parse_segments_test.go` | in-scope | source+tests | 1 | 0 | mined_gaps_open | `E2-R1` full-pass reconciled; parser scenarios mapped to requirement-level rows. |
| `kit/matcher/register.go` | in-scope | source+tests | 1 | 0 | mined_gaps_open | `E2-R1` full-pass reconciled; dynamic/splat child identity ambiguity tracked via `KIT-MATCHER-ISSUE-002`. |

## Round Log

| Round | Status | New Gaps | Notes |
|---|---|---:|---|
| `E2-R1` | completed | 3 | Full-pass replay and requirement-level reconciliation completed with explicit issue-backed exceptions (`KIT-MATCHER-ISSUE-001..003`). |
| `E2-R2` | pending | `TBD` | No-gap clean pass is blocked until open issues are resolved or confirmed. |
