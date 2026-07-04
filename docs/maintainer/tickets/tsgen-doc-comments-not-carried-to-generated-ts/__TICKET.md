# TsGen does not carry Rust doc comments into generated vorma.gen.ts

Found 2026-07-02 during P016 (TS package jsdoc/release-quality pass). The framework's
documentation strategy (`AGENTS.md`, "User-Facing Documentation Strategy") states that
in-code rust-doc and jsdoc are the user documentation, and that "any generated/reference
docs derived from source comments, exported types, and generated contracts" is part of
that story. For an app's `TsGen`-derived types, that promise is currently false: a Rust
struct's `///` doc comments never survive into its generated `vorma.gen.ts` TypeScript
declaration. `vorma.gen.ts` is many users' first contact with their own request/response
types (view/resource input/output structs) — a struct genuinely documented on the Rust
side currently produces an undocumented `export type` in the file a frontend developer
actually reads while wiring up a view or `apiClient` call.

## Verification (exhaustive, traced end to end, not inferred)

Three-layer trace confirming the gap is structural, not incidental:

1. **The derive macro's attribute allowlist excludes `doc`.**
   `#[proc_macro_derive(TsGen, attributes(serde))]` (`crates/vorma-macros/src/lib.rs:133`)
   registers only `serde` as an inert helper attribute the derive can see. `doc`
   attributes (what `///` comments desugar to) are never in that allowlist, so the
   derive's `syn`-based parser never reads or forwards them — confirmed by reading every
   attribute-handling branch in `crates/vorma-macros/src/ts_gen_derive.rs`, none of which
   mention `doc`.
2. **The data model the derive builds has no doc field.** `vorma_contract::tsgen::Type`,
   `TypeDef`, `FieldDef`, `TypeRef` (`crates/vorma-contract/src/tsgen.rs`,
   `crates/vorma-contract/src/contracts.rs`) — the entire structure `TsGen` populates and
   that later drives TS emission — carry no documentation-string field anywhere in their
   definitions. There is structurally nowhere to put a doc comment even if the derive
   parsed one.
3. **The renderer has exactly one hand-templated jsdoc emission, unrelated to any Rust doc
   comment.** `render_type_def` (`crates/vorma-build/src/typescript_contracts.rs:201`, the
   function that actually writes `export type ...`/`export const ...` text) contains one
   `/** ... */` emission at line 167 — the boilerplate
   `/** Argument must be a static string literal. */` comment above the generated
   `vormaPublicUrlKeys` type/`vormaPublicUrl` global declaration, unconditionally
   hand-written into every generated file regardless of any app type's doc comments.
   Confirmed empirically against `examples/board/src/client/vorma.gen.ts`: not one of its
   many `export type`/`export const` declarations (`Story`, `User`, `LayoutData`,
   `SiteStats`, etc., all `#[derive(... vorma::TsGen)]` structs in
   `examples/board/src/repo.rs`) carries a doc comment, and the one line searched for
   above is the sole `/** */` occurrence in the whole file.

## Task

Give `TsGen`-derived types' generated declarations real jsdoc, sourced from the Rust
struct/field's own `///` comments, end to end:

- Extend the `TsGen` derive (`crates/vorma-macros/src/ts_gen_derive.rs`) to read `doc`
  attributes on the struct itself and on each field (both the container-level summary and
  per-field docs matter — a generated `export type Story = { title: string; ... }` should
  ideally carry both a leading type-level comment and per-field comments, matching how
  rust-doc itself renders struct docs).
- Extend `vorma_contract::tsgen`'s data model (`TypeDef`/`FieldDef` at minimum) with an
  optional doc-comment field, threaded through
  `collect_type_defs`/`collect_type_defs_for`.
- Extend `render_type_def` (`crates/vorma-build/src/typescript_contracts.rs`) to emit a
  real `/** ... */` block above each documented declaration and per-field comments inside
  record literals, when present — matching the jsdoc conventions already established
  repo-wide in `packages/vorma` (P016's own sweep) so generated and hand-written jsdoc
  read consistently.
- Decide and document the manual-`Type`-implementation story: a hand-written
  `impl Type for X` (the escape hatch for shapes `TsGen` cannot derive — data-carrying
  enums, `serde(flatten)`, etc., per `crates/vorma-contract/src/tsgen.rs`'s own module
  docs) has no `///` comment for the derive to read in the first place; decide whether
  `TsExtraType`/manual implementations get any doc-comment story at all, or whether this
  ticket is `TsGen`-derive-only by design (a defensible scope boundary, since a manual
  impl already requires the author to write the `Type` trait methods by hand and could
  reasonably be expected to also hand-write a doc comment through a different, still
  undesigned mechanism).

## Verification

- A `TsGen`-derived struct with `///` comments on the struct and at least one field
  produces a generated `vorma.gen.ts` declaration with a real jsdoc block matching those
  comments (both container- and field-level).
- `examples/board`'s existing structs remain undocumented at the Rust source (a Board
  content choice, not evidence either way) unless a follow-up packet adds doc comments
  there to exercise the new behavior end to end — filing that follow-up, if wanted, is a
  separate call for whoever picks this ticket up.
- No change to the wire protocol / runtime serialization shape — this is purely additive
  metadata for TypeScript generation; `cargo test -p vorma-build` (the golden-pinned
  contract tests in `crates/vorma-build/src/typescript_contracts.rs`) stays green apart
  from tests that intentionally pin the new doc-emission behavior.
