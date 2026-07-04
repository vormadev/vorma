# P013 — vorma-contract + vorma-macros release-quality pass

Read `docs/maintainer/fable/README.md` (the packet protocol binds you), then
`LEARNINGS.md`, `STATE.md`, repo-root `AGENTS.md`, `docs/maintainer/REMINDERS.md`, and the
review bar: `docs/maintainer/skills/thermo-nuclear-system-review/SKILL.md`. Model packets:
P011 (matcher) and P012 (tasks) — same shape; this packet covers TWO smaller crates in one
pass.

## Context

Same Phase D shape: rust-doc is the user documentation; findings mode for structure.
`vorma-contract` carries the wire/document/tsgen contract types (note the standing ticket
`contract-borrowed-element-construction` — findings touching element construction should
reference it, and the `tsgen-drafter-oxfmt-idempotency` ticket owns the drafter's output
formatting). `vorma-macros` is the `TsGen` derive. Both crates' public surfaces are partly
framework-facing — document honestly for the actual audience (app authors meet `TsGen`,
`TsDrafter`, `TsExtraType`, the document element types via head/document builders; some
contract items are framework-integration surface — say so in their docs rather than
pretending otherwise).

## Scope

1. **Doc sweep, both crates.** Every public item to the teaching bar; deny attributes
   (`missing_docs` + `rustdoc::broken_intra_doc_links`) landed where not present; doctests
   where they genuinely teach (derive-macro doctests compile the derive — keep them
   cheap). For `TsGen`: document the derive's contract fully (what it emits, the supported
   shapes, what it rejects — verified against the macro code and the existing trybuild UI
   tests, not assumed).
2. **Thermo-nuclear review, FINDINGS MODE, both crates.** Directly landable:
   docs/tests/zero-behavior private cleanups. Structural/public-surface findings analyzed
   and reported. Wire-contract types are FROZEN surface (protocol version 3 shipped —
   changes are maintainer territory).
3. **Checklist verdict** per crate, item by item, with evidence.

## Hard constraints

- No public API changes; wire contract frozen; the generated-TS output format is covered
  by `tsgen-drafter-oxfmt-idempotency` (reference, do not fix here unless the fix is
  exactly that ticket's scope — in which case escalate first, since it changes
  generated-file bytes).
- Tests never cheat (trybuild UI expectations are pins — rebless only on genuine compiler
  drift, never to make new code fit); scoped fmt writes only; no git actions; no network
  installs; unexpectedly dirty unowned files are escalations, never cleanup.

## Definition of done

- Deny attributes green in both crates; `RUSTDOCFLAGS="-D warnings" cargo doc` clean;
  doctests pass.
- Findings report + per-crate checklist verdicts.
- Gates green: workspace tests + doctests, clippy `-D warnings`, fmt check,
  `make loom-tasks` untouched-green.
- REPORT.md per template (or in-message on guardrail).
