# Fable Architecture Campaign Notes

These notes cover the main-thread architecture campaign in the `9e3bfc5a...` history:
the regression-restoration closeout, `CURRENT_PLAN.md`, contract extraction,
client-core decomposition, `CURRENT_PLAN_2`, single-binary collapse, tooling review, and
the fresh `CURRENT_PLAN_3` review.

## Source Coverage

- Main transcript ranges: `9e3bfc5a-bb96-4b96-ad2c-24995f8ad7d7.jsonl` lines 1-2,776
  cover the full-repo review, regression audit/restoration, Phase A/B setup, contract
  extraction, and early client-core decomposition.
- Lines 2,777-4,801 cover client-core tranches, `CURRENT_PLAN_2`, the single-binary
  collapse, Go audit tranches, architecture prose, and fresh review into
  `CURRENT_PLAN_3.md`.
- Lines 4,810-5,080 cover the tooling review and public API scrutiny setup.
- Current reconciliation: `CURRENT_PLAN.md`, `CURRENT_PLAN_2.md`, `CURRENT_PLAN_3.md`,
  `ARCHITECTURE.md`, `GO_PARITY_AUDIT.md`, `Makefile`, and current config/build code.

## Full-Repo Review Thesis

The first architecture review found a strong post-rewrite skeleton but too many implicit
contracts. The most durable thesis was:

- Entropy concentrated where real internal contracts were implicit instead of owned.
- The TypeScript client core was the largest browser-side entropy point.
- The `__private` surfaces were the real framework architecture pretending to be
  implementation detail.
- Cross-language contract handling had both the right pattern and the wrong pattern:
  generated/golden-pinned wire fixtures were good; hand-mirrored constants/types were bad.

Later work validated the thesis by extracting `vorma-contract`, shrinking `__private`,
adding generated contract pins, and decomposing the client core around owned state units.

## Regression Audit Before Improvements

After the `paranoid` file-lock regression was discovered, the user explicitly paused
first-principles cleanup and required a meticulous regression audit first. The durable
process rule:

- Once one silent regression is confirmed, the diff cannot be trusted by vibes.
- Build a regression audit ledger in `docs/maintainer`.
- Classify removed tests, capabilities, and behavior against the baseline.
- Restore confirmed regressions before architecture polish.
- Do not treat repo reminders as optional or as "questions" needing user confirmation.

Important user corrections from that phase:

- Sequential cargo builds were not an open question; they violated maintainer reminders.
- Watch roots are resolution anchors, not security boundaries. Explicit `../` watch
  patterns must work for monorepos.
- Serve-time symlink checks and public-asset byte caching were large regressions unless
  proved carried.
- The Vercel current-dir manifest fallback was a regression.
- Prehashed-directory support had been intentionally dropped.

The regression audit/restoration arc closed only after the user ran `make gate` and
confirmed green.

## Combined Cargo Build Restoration

Before the single-binary collapse, a restoration brought back the old dual-bin build
parallelism:

- One cargo invocation built both build-entry and app-server binaries with
  `--message-format=json-render-diagnostics`.
- Artifact paths were parsed from cargo JSON messages.
- The live-state binary and app server were run directly from built executable paths.
- Two concurrent cargo commands would serialize on Cargo's target-dir lock; one cargo
  invocation was the correct shape for dual-bin parallelism.

This was later superseded by the single-binary collapse. Do not resurrect dual-bin
machinery unless the collapse is intentionally undone.

## JSON and Formatter Lessons

The user wanted all framework-written JSON tab-indented, not just one golden file.

Key settled points:

- Generator output should be written in the shape the generator owns.
- A generated JSON fixture with a byte-exact Rust golden must not be co-owned by `oxfmt`.
- `packages/vorma/core/wire_contract_fixtures.json` became generator-owned and ignored by
  oxfmt.
- Hand-authored JSON such as search-param vectors stayed under formatter control.
- The repo gate was changed to run formatters in write mode before checks, per maintainer
  ruling.

Pitfall: write-mode formatting during pre-commit can leave formatted changes unstaged if
the hook formats after staging. That is an inherent tradeoff of format-on-hook.

## Phase A: Quick Wins and Dead Surface

Phase A originally targeted dead `vorma_build::__private`, stale docs, a dangling test
README link, and an ignored doctest. Deleting the hidden build surface revealed the real
value:

- Hidden glob re-exports had blinded dead-code analysis.
- Removing them surfaced dozens of unused/internal-only items.
- The correct response was item-by-item adjudication, not bulk deletion.
- Ownership fields that keep servers/processes alive should use the `_field` idiom rather
  than be deleted.
- Test-only observers/helpers should be `#[cfg(test)]`.
- Real cruft should be deleted.

The durable lesson is that `__private` is not harmless. If it makes internals publicly
reachable, it can conceal dead code and weaken compiler assistance.

## `vorma-contract` Extraction

The main architecture move was a full `vorma-contract` crate, not a minimal hidden-export
shrink.

Moved/owned concepts included:

- framework graph model and validation inputs,
- execution plan model,
- runtime manifest,
- contract constants,
- TypeScript generation model,
- document renderer pieces,
- wire types such as `HeadElement`, `ViewPayload`, and `SsrPayload`.

Important details:

- `vorma-build` imports `vorma-contract` directly rather than reaching through
  `vorma::__private`.
- `vorma` keeps a small internal re-export line so its own modules can stay stable while
  the true owner moved.
- `impl Type for FormData` stayed beside `FormData` in `vorma` because orphan rules
  require the impl where a local type exists.
- `extern crate self as vorma` in `vorma-contract` is an intentional macro-ABI trick so
  derive output can use `::vorma::tsgen::*` paths. It is exotic but documented and pinned.
- Flat root `tsgen` exports were removed; the public home is the `vorma::tsgen` module.

The golden wire fixture tests passing unchanged were the proof that cross-runtime bytes
did not drift during the move.

## Remaining `__private` Doctrine

After contract extraction, remaining hidden surfaces should be narrow and honest:

- Macro ABI internals can be hidden if no user or build crate should call them directly.
- Build/runtime contract surfaces should be named, documented modules, not broad
  `__private` bags.
- Hidden exports must not be used to avoid designing an ownership boundary.

If a hidden export becomes a de-facto cross-crate contract, it should either move to
`vorma-contract` or another named public-internal crate/module.

## Client-Core Decomposition

The original plan's "split the big file" framing was corrected. The goal was not smaller
files; it was owned state and owned behavior.

Tranches that landed:

- `revalidation_scheduler.ts`: owns refresh demand/backoff/debounce state and launch
  requests through an explicit dependency interface.
- `work_projection.ts`: derives work state from an explicit `WorkSources` model instead
  of ad-hoc reads.
- `redirects.ts`: owns redirect target classification and redirect detection.
- `wire_payload.ts`: owns pure wire payload decoding.
- `client_loaders.ts`: owns client loader registry, search-schema registry, matcher-ready
  queueing, prefetch reconcile algebra, server-state building, and loader runs.
- `abort_error.ts`: shared abort/stringification helpers.
- `route_modules.ts`: owns view-module materialization, dev module cache, HMR version map,
  loader registration from modules, route-record building, and module HMR updates.

Important test lesson: a new unit test caught a mistaken assumption about retry behavior.
That is the point of decomposition: previously invisible invariants become directly
testable.

The final assembler in `create_client_core.ts` can still be large if it is truly only the
composition root for browser runtime transactions. Do not split it again solely by line
count; split when a piece has a real state model or behavior contract to own.

## `CURRENT_PLAN_2` and the Single-Binary Collapse

`CURRENT_PLAN_2` unified the next set of improvements:

- Vite targeted invalidation.
- Go dev-loop audit tranches.
- Single-binary collapse.
- Layering residue.
- Compile-fail pins and architecture prose.
- Fresh review into `CURRENT_PLAN_3`.

The single-binary collapse is the biggest structural result:

- Live-state protocol moved to `vorma-contract`.
- Runtime app server can emit live state and exit under env keys.
- `app_binaries_build.rs` and dual-bin build machinery were removed.
- `BuildOptions` was deleted.
- `vorma_build::run(app_config)` became the whole public build surface.
- Child unexpected-exit monitoring was restored as part of the collapse.
- Windows child-exit monitoring remains a documented stub by explicit ruling until there
  is a real Windows dev target to test.

The user explicitly rejected a CLI. Do not suggest `vorma dev`/`vorma build` as a near-term
path unless that ruling is reopened.

## Go Parity Audit Doctrine

Go is a bell-ringer, not gospel:

- If Rust loses a behavior that Go had and the behavior was intended, it is a regression.
- If Go's behavior was an accident/bug, Rust should fix it and record the correction.
- If Rust intentionally redesigns a concept, record why and pin the new contract.

Important Go audit findings:

- Vite public-asset restarts were a regression; targeted invalidation is the corrected
  design.
- Critical-CSS import watching had regressed and was fixed with per-generation watch plan
  classification.
- `dist_dir` transition cleanup had regressed and was fixed at commit time to preserve
  zero-downtime rollback semantics.
- Runtime manifest and generated TS were broadly parity/intentionally redesigned.
- Modern Go lived on the `refactor-2026-*` branches; older first-parent history can be a
  red herring with deprecated trees.

When using Go as a source, read the correct branch/snapshot. Do not rely on one grep or
one memory-shaped path.

## Architecture Prose

`ARCHITECTURE.md` was written after the collapse and should be treated as the system map
for that generation:

- app config and declaration graph,
- build/runtime contract boundary,
- live-state protocol,
- dev generation epochs,
- Vite/plugin control planes,
- runtime execution,
- error/exit model.

If future architecture changes outdate it, update it in the same change. Stale
architecture prose is worse than absent prose because it teaches the next agent the wrong
center of gravity.

## Tooling Review

The tooling review judged the shape mostly right but found important fixes:

- `e2e` and `e2e-smoke` depend on `ts-build` so stale `.dist` cannot break e2e silently.
- `rust-package` and release instructions were missing `vorma-contract`; fixed.
- Make targets were declared phony after an actual conflict risk review.
- `cargo run -p xtask` replaced manifest-path detours.
- `ts-gate` had redundant build-client-wasm prereqs.
- A `clean` aggregate was added.

Kept deliberately:

- xtask gate sandwich because make alone does not handle per-step logs/env scrubbing well.
- Small hand-rolled version parsing in xtask instead of adding a dependency.
- Bombadil's polling as test-harness observation of external processes, not framework dev
  loop polling.
- `make gate` as the real gate; CI job deferred by maintainer ruling.

## API Review Before Docs

The user ruled that user-facing docs should wait until public APIs are loved. This
created the API design pass and later Board pressure test.

Do not rush docs for an API that is still under active scrutiny. The docs should teach
the final coherent surface, not smooth over a footgun.

Create-vorma remains out of date and is sequenced last after the framework/API surface is
settled. Treat create-vorma as public-facing scaffolding debt, not harmless tooling.

## Fresh Review and Current Follow-On

`CURRENT_PLAN_3.md` closed the `CURRENT_PLAN_2` initiative and recorded that the
architecture was broadly in the shape a from-scratch design would choose. It then queued
public API scrutiny and the realistic pressure-test app.

After `CURRENT_PLAN_3`, significant newer work happened:

- Error/exit protocol redesign.
- Config reshape.
- Notes workout.
- Scoped middleware.
- API mount removal.
- Matcher review/performance.
- Tasks review, with unresolved items.
- Board pressure-test census and implementation.

Therefore `CURRENT_PLAN_3.md` is a milestone, not the latest universal truth. Use it for
pre-Board architecture context, then check `API_DESIGN.md`,
`PRESSURE_TEST_CENSUS.md`, and the Fable notes for later deltas.
