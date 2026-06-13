# Fable Architecture Notes

These notes preserve forward-looking architecture context from the Fable history. They are
not a changelog. They record the decisions, warnings, and design pressure that should
shape future work.

For the later main-thread campaign that acted on these findings, read
`FABLE_ARCHITECTURE_CAMPAIGN_NOTES.md`, `FABLE_API_BOARD_NOTES.md`,
`FABLE_MATCHER_NOTES.md`, and `FABLE_TASKS_NOTES.md`. This file is primarily the survey
and pressure-point map; those files record the subsequent decisions and current deltas.

## Architecture Survey Subagents

The `9e3bfc5a-bb96-4b96-ad2c-24995f8ad7d7` architecture campaign spawned four survey
subagents:

- `agent-a053839bde3f7f79a`: core `crates/vorma` survey; completed with a final report.
- `agent-a144e40f16b92520d`: small crates and tooling survey; completed with a final
  report.
- `agent-a9dfc0c2041aff3bf`: `crates/vorma-build` survey; interrupted before a final
  report after gathering partial evidence.
- `agent-a9053e0931cb493b2`: TypeScript side survey; interrupted before a final report
  after gathering partial evidence.

The interrupted subagents still matter as source material, but their conclusions should
be treated as partial until reconciled with the main-thread decisions and current code.

## Core `vorma` Crate Pressure Points

The core-crate survey judged the intended center of gravity sound but found the internal
layering too tangled:

- `crates/vorma/src/execution_engine.rs` mixed request routing, handler execution,
  middleware orchestration, response report construction, and error conversion. The
  recommended mental split was request routing, handler invocation, and response
  building/finalization.
- `crates/vorma/src/runtime_app.rs` mixed HTTP request parsing, route classification,
  view rendering, resource finalization, asset serving, and error responses. The warning
  was that request classification should not be hidden inside the runtime-app outer shell.
- `crates/vorma/src/framework_graph.rs` mixed canonical graph data, graph validation, and
  matcher/builder integration. The survey saw the graph as a data model whose validation
  and matcher integration could be separated if future work needs to reduce entropy.
- The runtime module family was conceptually coherent enough to keep, but the original
  survey worried that `runtime_app`, `runtime_host`, and `runtime_service` blurred app
  execution, Tower service glue, and host assembly. Later main-thread rulings kept
  `runtime_*` naming because it performs useful crate-private namespace work.
- `runtime_snapshot`, `runtime_manifest`, and `runtime_assets` each touched pieces of the
  committed-generation contract. The future pitfall is to avoid moving manifest lifecycle
  decisions into whichever file happens to need them; one layer should own the invariant.
- `runtime_document` was called out as a thin trait facade with little independent weight.
  Do not add similar one-method pass-through modules unless they buy a real boundary.

## Core API Surface Lessons

- Hidden exports are still architecture. The survey treated `__private` and
  `#[doc(hidden)]` exports as signs that `vorma-build` and macro internals may be coupled
  too tightly to runtime internals.
- `ResourceInput` and `ViewInput` were called out as marker traits that users should not
  need to understand. If public or semi-public macro bounds expose them, treat that as API
  pressure, not just harmless implementation detail.
- `ErasedRequestCtx`, erased futures, erased handlers, and static-route internals are
  macro/build implementation details. They should stay behind the smallest possible
  private surface.
- `View` as a declaration name was questioned because it can read like a rendered view
  instance. The survey preferred declaration-language names when the thing is a route
  declaration, not rendered output. Later API work should verify whether the final names
  make declaration-vs-runtime roles impossible to confuse.
- TypeScript-side public API includes object members and generated helper members, not
  just top-level exports. This was repeated later in the main thread and should guide API
  audits.

## Duplication And Contract Strings

- HTML/head tag and attribute constants were duplicated across several runtime modules in
  the surveyed state. The durable lesson is not merely "extract constants"; it is that
  document/head semantics need one canonical owner. Duplicating contract strings across
  handler context, typed context, manifest handling, and view response code makes it too
  easy for one layer to drift.
- Response finalization was duplicated between resource and view responses. Future work
  should keep method-specific and document-vs-JSON differences small, while centralizing
  common HTTP finalization semantics such as HEAD behavior and content-length handling.
- Type resolver patterns appeared in route input, public app declaration, and facade
  lowering. Future changes should make the type-contract generation pipeline explicit
  rather than recreating "invoke resolver, collect definitions, build contract" in
  multiple layers.
- Route segment contract strings such as dynamic parameter prefixes, splat identifiers,
  and explicit-index identifiers were flagged as duplicated between graph/planning code
  and matcher/build concerns. Main-thread matcher work later reinforced this: route
  syntax belongs to `vorma-matcher` where possible, and Vorma-specific policy layers on
  top.

## Naming And Concept Boundaries

- Avoid view/page/module/client-file drift. A TypeScript component module, a route
  declaration, and a runtime rendered response are different things.
- Avoid handler/context proliferation. The surveyed state had `HandlerContext`,
  `TypedHandlerContext`, and erased request contexts all representing related pieces of
  one request. If future work adds another context wrapper, first ask whether it can
  collapse into the canonical context boundary.
- Manifest, graph, and snapshot should remain distinct:
  - Graph: canonical application declarations.
  - Manifest: serialized/runtime asset and client contract.
  - Snapshot: immutable compiled runtime state.
  Do not use those terms interchangeably in docs or APIs.
- "Search schema" and "input schema" were called out as near-synonyms. If future public
  API work touches this surface, choose names that make view search params and resource
  body/query input distinctions obvious.

## Error-Handling Lessons

- The survey found no broad panic problem in hot paths, but it did call out generic 500
  conversion and broad error suppression as observability risks. Future runtime error
  work should preserve root-cause information internally while still controlling what
  reaches the client.
- `ViewError` allowing both internal error and client message to be absent was flagged as
  a weak invariant. The later API ruling that bare server-side errors must not leak
  internal messages does not mean the internal error model can be vague; it needs a clear
  invariant for what is logged, what is client-facing, and what is fallback text.
- Single-variant error enums were called out as possible over-modeling when a dedicated
  newtype or simpler error would make the invariant clearer.

## Repo-Wide Hygiene Survey Findings To Reconcile

The small-crate/tooling survey reported several issues that may have been fixed or
superseded later. Reconcile against current code before acting:

- Vestigial Go module files and unused workspace dependencies were reported.
- Both `blake3` and `sha2` were present. The current maintainer reminder says to use
  `blake3` unless a non-owned protocol requires something else, such as CSP hashes
  requiring `sha256`. Therefore the presence of `sha2` is not itself wrong; every use
  must be justified by such an external protocol requirement.
- Root `node_modules` was reported as present. Do not hand-edit ignored/generated
  dependency directories. Fix hygiene through ignore rules and normal package-management
  workflows if this is still true.
- Generated framework TypeScript files under `tests/framework` were reported as committed
  despite generated banners, including `remix` variants not exercised by Bombadil. Later
  generated-contract work established that generated artifacts must have one writer and
  deterministic output. Reconcile whether these files are intended golden fixtures,
  generated build products, or stale cruft before changing them.
- `packages/create-vorma/main.ts` was reported as explicitly out of date. If still true,
  treat it as public-facing breakage, not a harmless backlog note.
- The Bombadil e2e harness was reported as large and multi-concern. Its existence is
  still important because runtime/client/dev-loop claims require e2e proof, but future
  work should keep harness orchestration legible.
- `crates/vorma-macros/src/ts_gen_derive.rs` was reported as lacking compile-fail tests.
  Later trybuild pins may have addressed this; verify before acting.
- `packages/vorma/core/client_wasm/vorma_client_wasm_bg.wasm` was reported as an
  intentionally distributed committed binary whose ABI/versioning story was unclear.
  Future wasm work should make TS-to-wasm compatibility explicit.
- `make gate` was discussed as the real pre-release confidence gate. Later main-thread
  rulings clarified that `make gate` is not a per-push CI job by design.

## Architecture Direction That Survived Later Main-Thread Work

- The canonical graph, immutable execution plan/snapshot, transactionally committed build
  generation, generated contract pins, and decomposed client core remained the right
  center of gravity.
- The answer to architecture entropy is not compatibility shims. If an API or boundary is
  wrong, change it cleanly and update tests/contracts.
- The Go implementation is a comparison oracle, not an authority. Differences must be
  classified as parity requirement, intentional improvement, regression, or Go bug.
- `vorma-matcher` and `vorma-tasks` are sovereign crates. Vorma-specific behavior belongs
  in Vorma layers unless the behavior is truly general-purpose for the sovereign crate.
