# P019 Report

Provenance note: the executor (Sonnet 5 subagent, 2026-07-02) was blocked from writing
this file by the recurring harness report-file guardrail; content re-emitted on request
and placed by Fable (HTML transport escaping undone).

## What changed

Six files across `crates/vorma-contract/src/`, `crates/vorma-macros/src/`, and
`crates/vorma/src/` (no other crate touched):

- `contracts.rs` — the structural-equivalence layer: `SharedTypeNameRelation`
  (`SamePhaseShape` / `DivergentPhaseShape` / `UnrelatedTypes`),
  `TypeDef::classify_shared_name_with` (the public decision function, one doctest), and
  private helpers (`phase_independent_key`, `is_serialize_phase_key`,
  `type_defs_have_equivalent_shape` + `type_refs_have_equivalent_shape` for the recursive
  nested-key-erased comparison). `TypeDef` doc updated with the new exception to the
  name-uniqueness invariant.
- `graph_validation.rs` — `validate_type_contracts` rewritten: returns
  `Result<Vec<TypeDef>, GraphError>` (the collapsed list); a name collision classifies via
  `classify_shared_name_with` before deciding collapse / teaching-error /
  `DuplicateTypeName`; the `declared` reference-resolution set gets BOTH of a collapsed
  pair's original keys so `Named` refs from either phase still resolve. New
  `order_phase_keys` helper.
- `framework_graph.rs` — `compile` consumes the collapsed list (so `type_defs()` — what
  vorma-build's TS renderer iterates — carries it). New
  `GraphError::DivergentTypePhaseShapes` + Display text; docs and module invariant list
  updated; 4 new tests (pin classes a/b/c plus a defensive N>2-name-collision boundary
  test).
- `tsgen.rs` — `TypePhase`/`Type` docs rewritten from documenting-the-bug to
  documenting-the-rule; coverage rider: 2 new tests (`#[serde(transparent)]` shape,
  `#[serde(default = "path")]` phase divergence).
- `vorma-macros/src/lib.rs` — the shared-type doc section rewritten to the rule, including
  the exact key-suffix format and the teaching-error trigger.
- `vorma/src/facade.rs` — `register_type_def` now classifies before rejecting: only
  `UnrelatedTypes` fails fast with `DuplicateTypeName`; both phase cases flow to graph
  compile (the single authoritative boundary). 3 new tests through the REAL `TsGen` derive
  and REAL public route-registration entry points.

Ticket `tsgen-shared-type-phase-name-collision` deleted (consumed). No board changes; no
`vorma.gen.ts` regeneration needed.

## Decisions made

1. **Graph-compile is authoritative; the facade is a fail-fast pre-filter, not a second
   source of truth.** `FrameworkDeclarations` is publicly constructible with a naive
   `add_type_def` and every test in the repo builds one directly — so
   `validate_type_contracts` is the one path all construction routes pass through. The
   facade's fix does the minimum (only `UnrelatedTypes` still fails fast, matching the
   pre-existing conflict test exactly); collapse/divergence decisions and the teaching
   text are computed in exactly one place.
2. **The equivalence, precisely.** Two `TypeDef`s under different keys and one name are:
   `SamePhaseShape` iff (a) stripping a recognized `::serialize`/`::deserialize` suffix
   yields the same remaining key (the pairing test — confines the rule to phases of the
   SAME Rust type, since the derive's key is `{module_path}::{TypeIdent}::{phase}` and
   Rust forbids two same-name items at one path) AND (b) recursive shape identity where
   nested `TypeRefContract::Named` refs compare by `name`, never `key` (nested derived
   fields are always phase-suffixed even when genuinely shared — literal key comparison
   would make collapse never fire for most real shared types; proven with a nested
   `SharedUser.address: Address` test). `DivergentPhaseShape` iff (a) without (b) — the
   anticipated `serde(default)`/`skip_serializing_if` case. `UnrelatedTypes` iff not (a).
   The suffix literals are a documented `pub(crate)` constant contract (Rust's `concat!`
   cannot take const paths — confirmed by compile check — so a single shared symbol with
   the macro output is impossible); drift is caught by tests exercising the real derive
   output against the real stripping logic.
3. **Collapse mechanics.** Final `type_defs` deduped by first-occurrence-per-name
   (vorma-build re-sorts by name before render, so input order is unobservable
   downstream); the `declared` set is built from the RAW pre-collapse iteration so a route
   reaching the shared type via the discarded phase's key still resolves — caught by the
   test failing `UnknownNamedType` before the mechanism was right, which also proved the
   collapse must (and does) work recursively for nested shared types.
4. **New `GraphError` variant, not reuse** — callers can mechanically distinguish "fix
   your naming" from "fix your shared type's shape."
5. **No trybuild rebless needed** — this is a runtime registration failure, not a
   macro-expansion rejection; all 5 pins untouched and confirmed by the workspace run.
6. **Coverage rider split into two types** after serde itself rejected
   transparent+default-path on one field (a serde constraint, not a Vorma gap) — a more
   faithful test of each concern anyway.
7. **Board rider: investigated, not adopted.** All 30 board derive sites enumerated: 26
   Serialize-only; the 5 Deserialize types all name-distinct from every Serialize type;
   three input/output pairs spot-checked and confirmed intentional domain-shape splits,
   not accidental. No board file touched.

## Gate results

- `cargo test --workspace --all-targets`: **588 tests, 0 failed** (full log captured;
  includes vorma-build — concurrently worked by another executor, untouched by this packet
  — and board).
- Workspace doctests: 0 failed (10 doc-test binaries; contract now 8/8, +1 for the new
  `classify_shared_name_with` doctest).
- clippy workspace `-D warnings` clean; fmt clean; doc build `-D warnings` clean (one real
  private-intra-doc-link catch fixed en route); `make loom-tasks` 7/7 untouched-green;
  board tsgo/vitest correctly skipped (board unchanged; its one dirty file predates this
  session). No AddrInUse flake in this session's runs.
- Scoped dev-loop re-runs (superseded by the full runs): contract lib 45/45, vorma lib
  228/228, targeted clippy clean.

## Pins — red-before / green-after evidence

Red-before proven by reverting the just-written logic in place at BOTH layers:

- Graph layer: (a) `graph_collapses_shared_type_used_as_both_route_input_and_output` — red
  `DuplicateTypeName { name: "SharedUser" }` (P013's exact ticketed error) → green with
  exactly one `SharedUser` and one nested `Address` TypeDef. (b)
  `graph_rejects_structurally_divergent_shared_type_phases_with_teaching_error` — red
  `expected DivergentTypePhaseShapes, got DuplicateTypeName` → green with the teaching
  Display text. (c) the pre-existing duplicate-key/name test plus a new defensive
  third-unrelated-claimant test — green throughout (never red; that behavior must never
  regress; the boundary test also proves collapse never chains past one recognized pairing
  per name).
- Facade layer: `facade_collapses_shared_tsgen_type_used_as_both_route_input_and_output` —
  red `DuplicateTypeName { name: "SharedProfile" }` with the naive pre-check reinstated →
  green with exactly one TypeDef; the divergent-phase facade test surfaces
  `FacadeError::Graph(DivergentTypePhaseShapes { .. })` end to end (decision logic
  red-before'd at the graph layer it defers to). All 7 pre-existing facade tests green
  throughout.

## The exact final error text

type "SharedUser" is used as both a route input and a route output, but its serialized
(output) and deserialized (input) shapes differ (for example, a #[serde(default)] field is
optional on input but required on output) — use two distinct Rust types, one for each
shape, instead of sharing "SharedUser" across both (serialize key
"app::model::SharedUser::serialize", deserialize key
"app::model::SharedUser::deserialize")

(Captured from the real `Display` output via a scratch binary, not reconstructed.)

## Benchmarks

Not applicable — compile-time app-declaration validation, not a request hot path.

## Escalations / open questions

None. The key-distinguishability precondition passed cleanly: phase-suffix-stripped key
equality is necessary and sufficient for same-Rust-type-opposite-phase, with no
false-pairing risk and no false negatives for the recursive case.

## Discovered out-of-scope work

None filed. The serde transparent+default-path constraint is recorded for context, not
ticket-worthy.
