# kit/matcher Normative Intent Ledger

Status: Active  
Last Updated: 2026-02-09  
Trust Epoch: `E2`

## Mining Rules

- Intent mining MUST use implementation source and incorporate legacy tests
  outside `conformance/**` when present.
- Per-file counters are epoch-scoped.
- `passes`: total pass count in current epoch.
- `clean_passes`: no-new-gap pass count in current epoch.

## Per-File Replay Ledger

| File                                      | Scope        | Mining Inputs       | Passes | Clean Passes | Epoch State     | Notes                                                                                                    |
| ----------------------------------------- | ------------ | ------------------- | -----: | -----------: | --------------- | -------------------------------------------------------------------------------------------------------- |
| `kit/matcher/README.md`                   | out-of-scope | n/a                 |      0 |            0 | out-of-scope    | Documentation/benchmark artifact.                                                                        |
| `kit/matcher/bench.txt`                   | out-of-scope | n/a                 |      0 |            0 | out-of-scope    | Documentation/benchmark artifact.                                                                        |
| `kit/matcher/find_best_match.go`          | in-scope     | source+legacy-tests |      5 |            3 | mined_gaps_open | `E2-R5` clean replay revalidated best-match traversal; no new gap IDs.                                   |
| `kit/matcher/find_best_match_test.go`     | in-scope     | source+legacy-tests |      5 |            3 | mined_gaps_open | `E2-R5` clean replay reconfirmed best-match scenario coverage alignment.                                 |
| `kit/matcher/find_nested_matches.go`      | in-scope     | source+legacy-tests |      5 |            3 | mined_gaps_open | `E2-R5` clean replay reconfirmed nested collision/representative semantics without additional gaps.      |
| `kit/matcher/find_nested_matches_test.go` | in-scope     | source+legacy-tests |      5 |            3 | mined_gaps_open | `E2-R5` clean replay reconfirmed duplicate-key and ordering stress evidence remains aligned.             |
| `kit/matcher/matcher.go`                  | in-scope     | source+legacy-tests |      5 |            3 | mined_gaps_open | `E2-R5` clean replay reconfirmed option/default and getter contract mappings with no new gaps.           |
| `kit/matcher/parse_segments.go`           | in-scope     | source+legacy-tests |      5 |            3 | mined_gaps_open | `E2-R5` clean replay reconfirmed parser contracts and existing coverage mapping.                         |
| `kit/matcher/parse_segments_test.go`      | in-scope     | source+legacy-tests |      5 |            3 | mined_gaps_open | `E2-R5` clean replay reconfirmed parser scenario evidence remains aligned with source behavior.          |
| `kit/matcher/register.go`                 | in-scope     | source+legacy-tests |      5 |            3 | mined_gaps_open | `E2-R5` clean replay reconfirmed normalization/registration behavior with no new requirement-level gaps. |

## Round Log

| Round   | Status    | New Gaps | Notes                                                                                                                                                                                                                 |
| ------- | --------- | -------: | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `E2-R1` | completed |        3 | Full-pass replay and requirement-level reconciliation completed with explicit issue-backed exceptions (`KIT-MATCHER-ISSUE-001..003`).                                                                                 |
| `E2-R2` | completed |        0 | From-scratch replay revalidated source+legacy-test intent; requirements expanded to include exported accessor/getter surface without introducing new issue IDs.                                                       |
| `E2-R3` | completed |        2 | Deep replay found new requirement-level gaps: nested duplicate-key params semantics (`KIT-MATCHER-043`) and same-type longest-depth representative selection semantics (`KIT-MATCHER-044` / `KIT-MATCHER-ISSUE-004`). |
| `E2-R4` | completed |        0 | Clean replay revalidated source+legacy-test intent across updated requirement set with no new gap IDs.                                                                                                                |
| `E2-R5` | completed |        0 | Clean replay revalidated source+legacy-test intent; with `E2-R4`, this satisfies two consecutive no-gap full rounds.                                                                                                  |
