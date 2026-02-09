# kit/matcher Normative Intent Ledger

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
| `kit/matcher/README.md` | out-of-scope | n/a | 0 | 0 | out-of-scope | Documentation/benchmark artifact. |
| `kit/matcher/bench.txt` | out-of-scope | n/a | 0 | 0 | out-of-scope | Documentation/benchmark artifact. |
| `kit/matcher/find_best_match.go` | in-scope | source+legacy-tests | 2 | 1 | mined_gaps_open | `E2-R1` and `E2-R2` replays reconciled; no new gap IDs in `E2-R2`. |
| `kit/matcher/find_best_match_test.go` | in-scope | source+legacy-tests | 2 | 1 | mined_gaps_open | `E2-R2` revalidated scenario coverage mapping; no new gap IDs. |
| `kit/matcher/find_nested_matches.go` | in-scope | source+legacy-tests | 2 | 1 | mined_gaps_open | `E2-R2` revalidated nested branch mappings and ordering behavior. |
| `kit/matcher/find_nested_matches_test.go` | in-scope | source+legacy-tests | 2 | 1 | mined_gaps_open | `E2-R2` revalidated deterministic-order and invalid-match scenarios. |
| `kit/matcher/matcher.go` | in-scope | source+legacy-tests | 2 | 1 | mined_gaps_open | `E2-R2` revalidated options/defaults and added explicit getter API requirements. |
| `kit/matcher/parse_segments.go` | in-scope | source+legacy-tests | 2 | 1 | mined_gaps_open | `E2-R2` confirmed parser contract set without introducing new gaps. |
| `kit/matcher/parse_segments_test.go` | in-scope | source+legacy-tests | 2 | 1 | mined_gaps_open | `E2-R2` confirmed parser scenario coverage remains aligned. |
| `kit/matcher/register.go` | in-scope | source+legacy-tests | 2 | 1 | mined_gaps_open | `E2-R2` revalidated normalization/registration semantics; added accessor API requirement coverage rows. |

## Round Log

| Round | Status | New Gaps | Notes |
|---|---|---:|---|
| `E2-R1` | completed | 3 | Full-pass replay and requirement-level reconciliation completed with explicit issue-backed exceptions (`KIT-MATCHER-ISSUE-001..003`). |
| `E2-R2` | completed | 0 | From-scratch replay revalidated source+legacy-test intent; requirements expanded to include exported accessor/getter surface without introducing new issue IDs. |
| `E2-R3` | pending | `TBD` | Second consecutive no-new-gap full round pending. |
