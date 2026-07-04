# P013 Report

Provenance note: the executor (Sonnet 5 subagent, 2026-07-02) was blocked from writing
this file by the recurring harness report-file guardrail and returned the content in its
final message; Fable placed it.

## What changed

Eleven files across `crates/vorma-contract/src/` and `crates/vorma-macros/src/`
(+1139/−26, doc-dominated, one behavior-preserving mechanical DRY cleanup):

- `vorma-contract/src/lib.rs` — crate-root doc rewritten: the two-audience split
  (framework-integration vs app-facing tsgen/document surface), the frozen-wire note;
  intra-doc-link deny added (missing_docs deny predated).
- `constants.rs`, `wire.rs` — to the bar; `wire.rs` module doc names it FROZEN
  framework-integration surface with the version-skew rationale.
- `contracts.rs` — every public type to the bar incl. the TypeRefContract key-vs-name
  identity model and the document types' two-axis trust model
  (`contract-borrowed-element-construction` cross-referenced). Three doctests.
- `document_renderer.rs` — module doc lists every enforced trust rule; the clone-cost
  attribution corrected to the caller's construction step.
- `execution_plan.rs` — to the bar; GET/HEAD specificity adjudication verified against the
  runtime call site. DIRECT LANDING: two private matcher-builder helpers that duplicated
  `graph_patterns::matcher_builder` + three constants byte-for-byte were consolidated
  (zero behavior change, private-only).
- `framework_graph.rs` — every public type documented; one doc-accuracy correction
  (input_schema is NOT rendered into generated TS — traced, corrected).
- `live_state.rs`, `runtime_manifest.rs` — to the bar; dev-only protocol confirmed by
  trace; the SHA-256 CSP exception tied to doctrine; ClientCoreAssets traced to the
  WASM-compiled matcher.
- `tsgen.rs` — full module doc with getting-started doctest; four doctests; the `Type`
  trait doc HONESTLY documents the confirmed name-collision bug where a hand-implementer
  meets it.
- `vorma-macros/src/lib.rs` — crate-root framing; `derive_ts_gen`'s doc is the full
  contract (shapes, per-phase optionality, every rejection with exact pinned text) —
  verified against the macro code, all 5 trybuild pins, and a scratch byte-for-byte
  cross-check against real serde case-conversion output (including serde's no-acronym
  quirk). Intra-doc-link deny added.

No public API changes; wire contract untouched; no test files modified. One new ticket:
`tsgen-shared-type-phase-name-collision`.

## Doc-sweep stats

vorma-contract: ~40 structs, 10 enums, 1 trait, 1 alias, 4 fns, ~240 methods, 19 consts,
27 fields — all to the bar; doctests 0 → 7. vorma-macros: 1 public item (`TsGen`)
exhaustively documented; trybuild 5/5 pins are the contract verification (proc-macro
crate, no runnable doctests). Intra-doc-link deny newly landed in both.

## Findings

1. **CONFIRMED BUG (reproduced, ticketed, escalated):** `#[derive(TsGen)]` registers a
   type's Serialize-phase and Deserialize-phase TypeDefs under different stable keys but
   the SAME exported TypeScript name; `vorma`'s facade enforces global name-uniqueness
   with no phase exception. Any named type used as a route input in one place and a route
   output in another fails `App::from_config` with `DuplicateTypeName` — an ordinary
   pattern (a shared `User`). Reproduced empirically against the real facade; zero
   existing tests exercise it. Full trace + design options in the ticket. Maintainer
   design decision required.
2. Test coverage gap: `#[serde(transparent)]` derive support has zero coverage repo-wide;
   secondary: `#[serde(default = "path")]`'s explicit form parsed but untested.
3. Code-judo candidate analyzed, correctly not implemented (public enum change): TypeDef's
   repeated key/name fields — the split relocates rather than removes dispatch.
4. Cleared: framework_graph's Declaration→Node families are a deliberate
   parse-don't-validate typestate boundary (recorded so it isn't re-litigated).
5. Watch-only: framework_graph.rs at 1881 lines; split blast radius disproportionate
   today.
6. Cleared: RenameRule's twin 8-arm matches implement genuinely different pipelines;
   verified byte-for-byte against serde across all 8 rules.
7. Cleared with evidence: escaping/validation security review — strict allowlists,
   escape-by-default, static-str-gated tag names, forbid(unsafe_code) unbroken.

## Checklist verdict

Both crates: all items PASS except **Bug-free: FAIL on Finding 1 (ticketed)**;
Comprehensive Tests carries Finding 2's disclosed gap; full per-item evidence recorded.

## Gate results

- Workspace: 581/581 all-targets, 41/41 doctests (622 total), zero failures.
- contract lib 40/40 unchanged; contract doctests 7/7; macro trybuild pins 5/5
  untouched-green.
- clippy workspace clean; fmt clean; doc build `-D warnings` clean; loom 7/7
  untouched-green; downstream `vorma`/`vorma-build` lib builds clean.
- Methodology note recorded for future executors: `//!` module-doc intra-doc links resolve
  from the PARENT module's scope (unlike `///`), so module docs use fully-qualified
  `crate::` paths.

## Benchmarks

Not applicable (no bench suite in either crate; the one mechanical change is
compile-time-identical).

## Escalations / open questions

1. The name-collision bug (Finding 1) — maintainer design decision, ticket
   `tsgen-shared-type-phase-name-collision`.
2. Findings 2-6 held for the batched Phase D-end triage.

## Discovered out-of-scope work

The new bug ticket; nothing else. Standing contract-area tickets referenced, not
duplicated.
