# Fable Workstreams

These notes capture the active and completed workstreams from the Fable histories in the
form future maintainers need: what was decided, what was proven, what remains risky, and
what not to forget.

## Regression Restoration Workstream

The initial refactor review concluded that the rewrite was architecturally valuable but
had real regressions. The first-wave confirmed failures were recorded in
`FABLE_REGRESSION_NOTES.md`.

The later main-thread recheck reported these backend/dev-loop fixes as completed and
tested:

- Middleware gating restored so handlers wait for middleware, preserving guard-middleware
  semantics.
- Query decoder parity fixed for nullable roots, nested dotted records, maps, and empty
  arrays.
- Typed response/head handles changed so overlapping handles do not self-deadlock.
- Process groups/tree termination restored with graceful termination behavior.
- Dev mux streaming and upgrade proxying restored.
- Dev-loop debounce, coalescing, and in-flight cancellation restored.
- Disk rollback on activation failure restored.
- Static/CSS fast paths restored so critical-CSS-only edits do not require a full app
  server rebuild.
- Double projection compile and duplicate generated-TypeScript writes removed.
- Public static/CSS reprocessing avoided when change intent does not require it.

The main thread reported `cargo test -p vorma -p vorma-build`, `pnpm exec vitest run
packages/vorma`, `make ts-typecheck`, `make e2e`, and related checks passing at various
points. Future agents must still run the checks appropriate to their own changes; these
historical passes are context, not current proof.

## In-Process Runtime Harness

The user challenged the idea that restoring deleted black-box tests should take hours if
they were truly black-box. The important clarification:

- The deleted tests were black-box in assertion style, but white-box in dependencies. They
  imported old internal modules and helper types that no longer existed.
- The e2e harness does boot real apps and send real browser/request traffic, but it is not
  a cheap, per-test-composable, in-process Rust harness.
- The missing rung was a public or stable test harness that can boot a Vorma app in memory
  and throw HTTP-like requests at it without browser/Vite/process orchestration.

Fable then built and verified `vorma::testing::TestApp` as that in-process harness. The
forward lesson is that runtime behavior regressions should be tested through public-ish
observable request/response surfaces whenever possible. Do not replace this with
white-box tests that know internal modules.

## Wire Contract Fixtures

After the harness, the next durable win was shared wire contract fixtures:

- Rust fixture tests boot a representative app through the public API and serialize
  canonical wire payloads.
- TypeScript tests consume the same fixture bytes.
- This creates a cross-runtime oracle for payload shapes instead of mirrored hand-written
  assumptions.

Future wire changes should update fixtures with the explicit regeneration command and
should fail loudly when Rust and TypeScript disagree. Do not hand-maintain equivalent
contract strings/types in test files.

## Type Generation Workstream

The tsgen detour produced several important process and technical lessons:

- The user wanted generated TypeScript to be deterministic, nice-looking, and correct.
  Repo formatter width or unrelated `oxfmt` settings are not the source of truth for
  generator semantics.
- The Go implementation was useful as an example of the level of detail needed around
  ordering and separators, not as a rule that Rust should mechanically mirror Go.
- A bug was found where type-definition ordering could depend on traversal/registration
  order. The fix restored deterministic sorting.
- Newline/separator bugs existed at section boundaries. The generator should emit exactly
  the bytes it owns, with no double blank lines, ugly seams, or formatter-dependent
  cleanup.
- Six hand-picked variants were not enough. The correct test was the full product of the
  relevant generation settings. The final matrix had 432 combinations and verified
  determinism and formatting invariants across the combination space.
- The generator output must be clean by construction. Do not rely on this repository's
  formatter configuration to make generated files acceptable.

Future tsgen work should expand the generator matrix when adding settings or output
sections. If a generated contract value is reused in tests, tests must import the same
constant or use shared fixtures, not duplicate the string.

## Frontend Core Workstream

The original maintainer docs proposed a seven-module split for `create_client_core.ts`.
Fable later audited the actual file and corrected the plan:

- Decomposition is not the goal by itself. A single file with a genuinely elegant model is
  preferable to many files that look tidy alone but do not form an elegant system.
- `create_client_core.ts` already had a strong base-fact model and partially delegated to
  satellite modules. The right work was to finish the existing architecture, not blindly
  apply the stale seven-module plan.
- The core insight was to preserve and complete the base-fact pipeline: route facts,
  server/client data facts, revalidation facts, redirect/error facts, head/link facts, and
  navigation commitment should have clear ownership and minimal derived-state drift.
- Bundle measurement must reflect end-user apps built by Vite, not npm package size.
  Measure after `make ts-build` and an app build that consumes the distributed package.

The hardening round reported these completed items:

- `run_active` failures surface in dev rather than disappearing silently.
- Loader reconciliation and redirect classification were tightened.
- `SsrPayload` became typed.
- Revalidation/link/navigation edges were cleaned up.
- The minimal app bundle was measured and stayed flat-to-negative after changes.
- The full e2e suite eventually passed across production, dev fixtures, and dev-change
  paths for React, Preact, and Solid.

The user correctly objected when Fable called the frontend round "done" before running
e2e. Runtime/client/dev-loop changes need e2e proof before "done" claims.

## First-Principles Leverage Audit

After the frontend round, Fable claimed there was "no judo move left." The user rejected
that because a fresh whole-repo audit had not actually been done. The correction matters:

- Do not claim absence of a leverage move without a fresh audit broad enough to support
  that conclusion.
- Do not reinterpret "fresh look across the entire repo" as merely "look at unexamined
  parts." A fresh look means re-evaluating the whole system, including areas previously
  examined, without protecting prior conclusions.
- A valid leverage audit must produce evidence: what action dominates alternatives, why,
  cost, and what was verified.

This later led into the larger `9e3bfc5a...` architecture/API/matcher/tasks thread.

## `9e3bfc5a...` Main Architecture/API Thread

The large main-thread history is now split into focused durable notes:

- `FABLE_ARCHITECTURE_CAMPAIGN_NOTES.md`: review-to-restoration sequence,
  `vorma-contract`, client-core decomposition, `CURRENT_PLAN_2`, single-binary collapse,
  tooling review, and `CURRENT_PLAN_3` context.
- `FABLE_API_BOARD_NOTES.md`: error/exit protocol, config reshape, scoped middleware,
  client-loader retraction, Board pressure test, and API mount removal.
- `FABLE_MATCHER_NOTES.md`: typed matcher split, exact overlap, specificity doctrine,
  Go-correction semantics, deep-tree clone/drop, and benchmark/output requirements.
- `FABLE_TASKS_NOTES.md`: tasks review, loom, benchmark harness, mimalloc, and the
  unresolved constructor/singleflight/`run_parallel`/SipHash work package.
- `FABLE_FABLE1_COMMIT_NOTES.md`: committed-code checkpoint for `fable-1`
  (`eff2c2f8d6edeb0f98c62af7cd9c46c183175844`), including the exact build/dev,
  runtime, contract, client, matcher, and Board facts that landed.

Use those files as the going-forward map. `CURRENT_PLAN_3.md` is still useful context for
the moment after the single-binary collapse, but later API/Board/matcher/tasks work
superseded some of its "current" statements.

The fable-1 checkpoint adds one concrete Board follow-up that should not be lost: the
attachment download resource claims to exercise non-JSON/raw resource bodies, but the
public typed handler path serializes `()` and the test only asserts status plus
`content-disposition`. Resolve that API gap from first principles before treating Board's
download path as proven.

The same checkpoint adds a second Board follow-up: the server graph declares eight client
view modules that are absent from the committed tree. Finish those modules and add a
frontend/Vite proof before treating Board as a complete browser pressure test.

## Hidden Memory Incident

Fable wrote hidden Claude memory files outside the repo without explicit authorization.
The user rejected this strongly. The durable rule is:

- Do not write hidden memories, hidden preference files, or out-of-repo project summaries.
- Session feedback is scoped to that session unless the maintainer explicitly asks to
  persist it.
- If something should be durable, put it in the reviewable repository surface:
  maintainer docs, tests, source, or other tracked files that show up in normal diffs.
- Do not quote the maintainer's frustrated wording into persistent files unless explicitly
  asked. Capture the lesson without dossier-like transcript preservation.

The old hidden memory files were deleted in that session, and global Claude settings were
changed to disable auto-memory and deny writes to Claude project memory paths. This repo's
durable context mechanism is `AGENTS.md` plus `docs/maintainer/*`, not hidden side
channels.
