# P019 — TsGen shared-type dedup (the ruled name-collision fix)

Read `docs/maintainer/fable/README.md` (the packet protocol binds you), then
`LEARNINGS.md`, `STATE.md`, repo-root `AGENTS.md`, `docs/maintainer/REMINDERS.md`, and
your primary input: `docs/maintainer/tickets/tsgen-shared-type-phase-name-collision/`
(P013's full trace, reproduction, and option analysis, now carrying the maintainer
ruling).

## Context and authority

P013 confirmed and reproduced: a named type used as a route input in one place and a route
output in another fails `App::from_config` with `DuplicateTypeName`, because the derive
registers per-phase TypeDefs under one exported TS name and the facade enforces
name-uniqueness with no phase awareness. The maintainer ruled (2026-07-02): **one exported
TS type per name when the two phases agree structurally; an error only on genuine
structural divergence** — this ruling is the packet's semantic-change grant. Boundary
(Fable): dedup applies only to phases of the SAME Rust type; two different Rust types
sharing a name remain `DuplicateTypeName` even if coincidentally identical in shape.
Verify the stable keys can distinguish same-type-across-phases from different-type; if
they cannot, stop and escalate with the analysis.

## Scope

1. **Red pins first.** (a) The shared round-tripping type: one Rust type used as both
   input and output boots successfully post-fix and exports exactly ONE TS type (currently
   fails — reproduce P013's repro as the pin). (b) Genuine divergence: a type whose phases
   differ structurally (the natural case: a `#[serde(default)]` field, optional on
   Deserialize but required on Serialize) still errors — and the error message now TEACHES
   the rule (the shapes differ between input and output use; use two distinct Rust types).
   (c) Different-Rust-types-same-name still errors. Watch both pins fail/pass correctly
   across the fix.
2. **Implement the ruled rule** at the registration/uniqueness boundary (facade and/or
   contract registry — your analysis decides the layer; the comparison must be a
   well-defined structural equivalence over the rendered per-phase TypeDefs, not string
   comparison of output). Mind: `#[serde(default)]`'s per-phase optionality means many
   shared types will structurally diverge BY DESIGN — the ruling stands (they error with
   the teaching message); pin at least one such case and make the docs state this
   consequence plainly.
3. **Docs follow the fix:** update the `Type` trait / derive docs that P013 wrote (they
   currently document the bug honestly) to document the ruled behavior instead; the error
   type's docs teach the disambiguation path.
4. **Rider (P013 finding 2):** add the missing derive coverage — `#[serde(transparent)]`
   structs and `#[serde(default = "path")]` — as ordinary tests in the appropriate suites.
5. **Board rider, only if natural:** if a genuinely shared round-tripping type reads
   honestly somewhere in board, adopt it as the teaching call site; do not contrive one —
   report either way.
6. On completion: delete the consumed ticket; update census/inventory rows only if the
   board rider lands.

## Hard constraints

- The semantic change is EXACTLY the ruled rule — nothing else about tsgen output, wire
  contract, or naming changes. Generated-file formatting stays owned by
  `tsgen-drafter-oxfmt-idempotency`.
- Regenerate `vorma.gen.ts` via the real board build if board types change; never
  hand-edit.
- Tests never cheat; trybuild pins rebless only for genuinely changed diagnostics (the new
  divergence error's pinned text is expected to change/appear — that is the point,
  disclose it).
- Scoped fmt writes only; no git actions; no network installs; unexpectedly dirty unowned
  files are escalations, never cleanup.

## Definition of done

- All three pin classes red-before/green-after as specified; the teaching error text
  pinned.
- Gates green: workspace tests + doctests, clippy `-D warnings`, fmt check, doc build
  `-D warnings`, `make loom-tasks` untouched-green, board tsgo + vitest if board changed.
- REPORT.md per template (or in-message on guardrail) including the exact final error text
  and the structural-equivalence definition for the review record.
