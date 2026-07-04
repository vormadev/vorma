# P011 — vorma-matcher release-quality pass (docs + thermo-nuclear findings)

Read `docs/maintainer/fable/README.md` (the packet protocol binds you), then
`LEARNINGS.md` (the matcher doctrine section is load-bearing — the semantics it records
are maintainer-ratified and frozen), `STATE.md`, repo-root `AGENTS.md`,
`docs/maintainer/REMINDERS.md`, and the review bar itself:
`docs/maintainer/skills/thermo-nuclear-system-review/SKILL.md`.

## Context

Phase D raises every crate to release quality. Per the repo's documentation strategy
(AGENTS.md), rust-doc comments ARE the user documentation — there is no separate manual.
`vorma-matcher` is a sovereign crate, post-first-principles-review: its semantics are
corrected, pinned by the property-model and oracle suites, and its benches beat Go on
every row. Nothing here needs rescuing; this pass documents it to the bar and audits it
against the thermo-nuclear standard.

## Scope

1. **Doc sweep.** Every public item (crate root doc, modules, types, methods, consts) gets
   rust-doc at the teaching bar: what it does, when an application/framework author
   reaches for it, the subtleties that matter (the LEARNINGS doctrine — specificity as the
   single ordering, the dirty-path rule, the catch-all cover rule, index-pattern semantics
   — belongs IN the public docs where users meet those behaviors, restated in user-facing
   terms, never as maintainer-process references). Examples where they genuinely help;
   concise elsewhere. Then land `#![deny(missing_docs)]` (and
   `#![deny(rustdoc::broken_intra_doc_links)]` if not already denied via the gate) at the
   crate root as durable enforcement — the sweep is done when the crate compiles under it
   with `RUSTDOCFLAGS="-D warnings" cargo doc`.
2. **Thermo-nuclear review, FINDINGS MODE.** Run the full skill against the crate. You may
   land directly: doc fixes, test additions that strengthen coverage without weakening
   anything, and mechanical cleanups with zero observable-behavior change and zero
   public-signature change (naming of private items, dead private code, DRY consolidation
   of private helpers). Everything else — structural restructurings, public-surface
   changes, anything touching matching semantics or the recorded performance techniques —
   is a FINDING: report it with the skill's expected depth (the code-judo analysis, what
   would get simpler, what the risks are), do not implement it. Findings are a
   deliverable, not a failure; an empty findings list from this skill is suspicious, so
   dig.
3. **Checklist verdict.** The skill's final checklist, item by item, with evidence per
   item (tests coverage claims cite the suites; performance cites the recorded benches).

## Hard constraints

- Matching semantics are FROZEN (LEARNINGS: any semantic change must first go red in the
  property-model/oracle suites and be escalated — but this packet grants none).
- No public API changes of any kind. No bench recordings (`make bench-*` untouched); if a
  mechanical cleanup could plausibly affect the hot path, run the bench DIRECTLY (no tee,
  no recording) before/after and revert on any regression — the recorded baselines are not
  yours to move.
- Tests never cheat; doc examples must compile (doctests are part of the gate).
- Scoped fmt writes only; no git actions; no network installs; unexpectedly dirty unowned
  files are escalations, never cleanup.

## Definition of done

- `#![deny(missing_docs)]` landed and green;
  `RUSTDOCFLAGS="-D warnings" cargo doc --workspace --no-deps` clean; doctests pass.
- The findings report with the checklist verdict.
- Gates green: workspace tests + doctests, clippy `-D warnings`, fmt check;
  `make loom-tasks` untouched-green; a direct (unrecorded) matcher bench run showing no
  regression if any code was touched.
- REPORT.md per template (or full content returned in-message if the report-file guardrail
  fires), including the findings list and checklist verdict.
