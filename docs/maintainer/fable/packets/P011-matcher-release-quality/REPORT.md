# P011 Report

Provenance note: the executor (Sonnet 5 subagent, 2026-07-02) was blocked from writing
this file by the recurring harness report-file guardrail and returned the content in its
final message; Fable placed it.

## What changed

Nine files, all in `crates/vorma-matcher/src/` (+780/−88, doc-dominated):

- `lib.rs` — crate-root doc rewritten to a full teaching entry point (getting-started
  example, the flat/nested split, specificity ordering, dirty-path rule, catch-all cover
  rule, errors-as-strings rationale — LEARNINGS doctrine restated user-facing).
  `#![deny(rustdoc::broken_intra_doc_links)]` added. Five slash-helper functions got
  teaching docs with doctests.
- `builder.rs` — `MatcherBuilder` and all methods to the teaching bar with doctests; one
  dead-code removal in `validate_param_name` (unreachable let-else fallback, traced
  provably-unreachable, single private call site, identical test counts before/after).
- `matcher.rs` — `FlatMatcher`/`NestedMatcher` and match methods to the bar, restating the
  dirty-path and catch-all cover rules with doctests; one clarifying comment at the
  private `flatten_and_sort` implementing the cover rule.
- `match_result.rs` — `Params`/`SplatValues`/`Match`/`NestedMatch`/`NestedMatches` to the
  bar; corrected an inaccurate nested-capture claim and an inherited allocation-count
  comment (both verified empirically).
- `options.rs`, `overlap.rs`, `parse.rs` (corrected a doubled-trailing-slash claim,
  verified empirically), `pattern.rs`, `segment.rs` — all public items to the bar with
  doctests. `tree.rs` untouched (fully private).

No public API changes; no test files modified (existing coverage already exercised
everything documented); two scratch verification probes created and deleted.

## Doc-sweep stats

67 public items total (14 types, 41 fns/methods, 12 fields) — all documented.
`deny(missing_docs)` predated this packet (confirmed via git log) and was already green;
this packet's work was doc QUALITY plus the newly-added intra-doc-link deny. Doctests: 0 →
17, all passing.

## Findings (FINDINGS MODE — analyzed, not implemented)

1. Flat root-export surface should be namespaced (a `path` submodule for the slash
   helpers) — AGENTS.md alignment; breaking.
2. `NestedMatch` field-visibility inconsistency vs `Match`/`NestedMatches` — pass-through
   getters over identical data; convert to plain fields; breaking.
3. `prune_nested_matches` two-pass where the oracle's equivalent is one pass —
   behavior-preserving by trace but on the matching hot path; needs a measured
   micro-packet if accepted.
4. `register_pattern`'s shape-key collision loop covers a store it structurally cannot
   fire in (traced; cross-checked against the grammar suite's collision cases).
5. `matcher.rs` at 616 lines, growth doc-driven — watch only.

Explicitly considered and REJECTED as code-judo: unifying the flat/nested DFS walks — they
diverge in three doctrine-driven ways (best-vs-accumulate, inline-vs-deferred splat, the
flat-only `had_trailing` index gate); unification would thread ~3 modes through one body,
the skill's own anti-pattern.

## Checklist verdict

All 13 items PASS with evidence; three carry attached findings (design coherence → F2;
AGENTS compliance → F1; nothing-overly-complicated → F3/F4). "No unnecessary database
calls" N/A (zero I/O by design).

## Gate results

- matcher tests 57/57 (unchanged); doctests 17/17; workspace 580/0 + 19 doctests.
- clippy `-D warnings` clean (crate + workspace); fmt clean (crate + workspace).
- `RUSTDOCFLAGS="-D warnings" cargo doc --workspace --no-deps` clean.
- loom 7/7 untouched-green. `cargo package -p vorma-matcher` clean (22 files).
- Bench discipline: direct unrecorded before/after runs, all 13 rows within ±3.4% of the
  recorded Linux baseline (documented noise band); recordings untouched.

## Escalations / open questions

None. The five findings are held in ticket `matcher-release-quality-findings` for the
batched Phase D-end surface-polish triage (Fable's disposition at review).

## Discovered out-of-scope work

None beyond the findings ticket.
