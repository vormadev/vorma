# vorma-matcher thermo-nuclear findings (P011) — held for batched Phase D-end triage

P011's findings-mode review (2026-07-02) surfaced five structural findings, none
implemented (public-surface or hot-path-adjacent). Fable's triage plan: hold ALL per-crate
surface-shape findings from the Phase D passes and present them to the maintainer ONCE,
together, at Phase D's end — pre-1.0 breaking changes are sanctioned ("correct over
compat"), but the maintainer should rule on the whole surface-polish picture in one
sitting, not five times per crate. Full analyses in
`docs/maintainer/fable/packets/P011-matcher-release-quality/REPORT.md`.

1. Namespace the flat root exports (a `path` submodule for the five slash helpers) —
   AGENTS.md alignment; breaking.
2. `NestedMatch` field-visibility inconsistency vs `Match`/`NestedMatches` (pass-through
   getters hiding identical data) — convert to plain fields; breaking.
3. `prune_nested_matches` two-pass where the oracle does one — behavior-preserving but
   inside the matching hot path; would need a measured micro-packet (P004 discipline).
4. `register_pattern` shape-key collision loop covers a store it provably cannot fire in —
   registration-time simplification; test-pinned behavior nearby.
5. `matcher.rs` at 616 lines (doc-growth-driven) — watch only, no action recommended.

Disposition when triaged: accepted rows become granted packets; rejected rows get the
ruling recorded here and this ticket closes.

## Fable recommendations (2026-07-02, Phase D triage — maintainer ruling pending)

Finding 3 (one-pass prune): recommend REJECT — hot-path churn with nothing to solve;
re-open only if matcher perf ever needs headroom. Finding 5 (file size): watch-only stands
(no action proposed). Findings 1, 2, 4 remain pending the maintainer's breaking-surface
ruling (triage Q1); on acceptance they become one granted packet and this ticket closes.
