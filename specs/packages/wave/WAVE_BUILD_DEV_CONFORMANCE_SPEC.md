# Wave Build and Dev Conformance Specification

Status: Draft  
Last Updated: 2026-02-09  
Applies To: Wave build/dev control-plane behavior (`wave/tooling/*`) independent of framework overlays

## 1. Purpose

This spec is the canonical contract for Wave-owned build/dev behavior.

Framework specs (including Vorma) SHOULD reference these requirements instead of
duplicating Wave internal control-plane semantics.

## 2. Ownership Boundary

Wave owns:

- dev control-plane restart/readiness lifecycle,
- watcher/event-loop semantics,
- static/CSS artifact processing control-plane behavior,
- schema-merge behavior for framework extensions.

Frameworks own:

- framework-specific callbacks injected into Wave,
- framework-specific endpoint interactions and resulting app-visible behavior.

Conflict rule:

- if a framework alias requirement conflicts with this spec for a Wave-owned
  concern, this spec is authoritative.

## 3. Framework Integration Mapping

Framework specs SHOULD reference `WAVE-*` IDs directly at integration
boundaries.

No framework-alias table is maintained in this file; alias rows that were
previously copied from framework docs were removed to prevent stale cross-spec
duplication.

## 4. Requirement Catalog

### 4.0 CLI Entry Contracts

#### WAVE-CLI-001: CLI Logger Defaulting Contract

Given CLI entry helper `BuildWaveWithHook` or wrapper `BuildWave` is invoked
with nil logger  
When entrypoint initializes runtime dependencies  
Then helper MUST construct default color logger named `wave` before invoking
dev/build flows.

#### WAVE-CLI-002: Hook Callback Gate and Precedence Contract

Given CLI flags include `--hook` and a non-nil hook callback is provided  
When entry helper executes  
Then helper MUST invoke callback exactly once with parsed `--dev` value and MUST
return immediately without entering `RunDev` or builder build path.

Given CLI flags include `--hook` but callback is nil  
When entry helper executes  
Then helper MUST ignore hook-only callback path and continue through standard
`--dev`/production flow selection.

#### WAVE-CLI-003: CLI Error Propagation and Builder Lifecycle Contract

Given hook callback, `RunDev`, or production builder path returns error  
When CLI helper handles that failure  
Then helper MUST fail fast via panic (no error return contract).

Given production builder path is selected  
When helper completes (success or panic unwind)  
Then builder cleanup (`Close`) MUST be deferred for execution.

### 4.1 Dev Control Plane

#### WAVE-DEV-012: Cycle-Vite Reload Source Exclusivity

Cycle-vite reload orchestration MUST have one authoritative browser reload
source per cycle and MUST NOT emit duplicate reload triggers.

#### WAVE-DEV-030: Revalidate Overlay-Cleanup Robustness

Refresh-script revalidate flow MUST clear rebuilding overlay for both resolve
and reject outcomes.

#### WAVE-DEV-032: Vite Startup Failure Handling

Vite startup failure in dev loop MUST enter explicit failure handling (not
log-only success continuation).

#### WAVE-DEV-033: Build-Retry Request Strength Preservation

Build-retry wake-up handling MUST preserve restart request strength bits
(`recompileGo`, config-restart intent).

#### WAVE-DEV-034: App Startup Failure Handling

App startup failure MUST propagate as actionable orchestration failure.

#### WAVE-DEV-035: Readiness-Gate Failure Handling

App/Vite readiness timeout/failure outcomes MUST be actionable and MUST gate
reload/cycle success continuation.

#### WAVE-DEV-036: Config Reload Framework-Field Preservation

Config reload replacement MUST preserve framework-injected runtime-only fields
required for control-plane correctness.

#### WAVE-DEV-041: App-Stop Failure Propagation

App stop (`Kill`/`Wait`) failures MUST be surfaced to callers.

### 4.2 Watch and Event Processing

#### WAVE-EVT-001: Batch Dedupe Must Preserve Strongest Effective Work

Event dedupe MUST preserve non-lossy effective work semantics, including
unmatched-event distinctness and strongest-intent aggregation.

#### WAVE-EVT-019: Blocking Event-Phase Failure Gating

Blocking phase failures (hooks/build units) MUST gate success-phase continuation
and success-style browser signaling.

#### WAVE-EVT-023: Concurrent Restart Arbitration Determinism

Concurrent restart actions MUST resolve deterministically to strongest intent.

#### WAVE-EVT-028: Directory Watch-Expansion Failure Handling

Dynamic watch-expansion failure MUST be actionable and MUST NOT silently
continue as success.

#### WAVE-EVT-029: CSS Hot-Reload Artifact Read-Failure Handling

CSS artifact read failures in hot-reload path MUST be actionable and MUST
suppress success-style CSS payload emission.

#### WAVE-EVT-030: Implicit-vs-Callback Restart Strength Preservation

Implicit rebuild work MUST NOT be downgraded by weaker callback restart actions.

#### WAVE-EVT-031: Stable Watcher Channel Source Under Teardown

Watch-loop channel consumption MUST use a stable watcher source under teardown
races.

#### WAVE-EVT-032: Stale-Watch Removal Failure Handling

Stale watch removal failures MUST be explicit and tracked-state updates MUST stay
consistent with actual removal outcome.

#### WAVE-EVT-033: Closed Error-Channel Watch-Loop Termination

Closed watcher error-channel receive MUST terminate watcher loop (not nil-error
spin/log behavior).

### 4.3 Static, CSS, and Schema

#### WAVE-STATIC-005: Filemap Artifact Rotation Cleanup Guarantees

Filemap artifact rotation MUST obey explicit cleanup-failure policy (actionable
failure or explicitly bounded tolerated-stale policy).

#### WAVE-STATIC-015: Granular Stale-Artifact Removal Failure Handling

Granular stale artifact removal failures MUST be explicit and policy-aligned.

#### WAVE-CSS-003: Normal CSS Rotation Cleanup Guarantees

Normal CSS hashed artifact rotation MUST obey explicit cleanup-failure policy.

#### WAVE-SCHEMA-007: Reserved Schema-Key Collision Guard

Framework schema extensions MUST NOT silently override reserved top-level Wave
schema sections.

## 5. Scenario Catalog (Initial)

### WDC-CLI-001 (covers WAVE-CLI-001)

CLI helper invocation variants with nil logger input MUST verify default `wave`
color logger initialization occurs before delegated work.

### WDC-CLI-002 (covers WAVE-CLI-002)

`--hook` callback-present and callback-nil variants MUST verify callback
gating, dev-flag forwarding, and early-return vs standard mode-selection
semantics.

### WDC-CLI-003 (covers WAVE-CLI-003)

Hook/dev/prod delegated-path failure fixtures MUST verify panic-based failure
propagation and production builder deferred-close lifecycle.

### WDC-DEV-012 (covers WAVE-DEV-012)

Cycle-vite orchestration fixtures MUST verify single authoritative reload source
per cycle.

### WDC-DEV-030 (covers WAVE-DEV-030)

Revalidate helper resolve/reject fixtures MUST both clear rebuilding overlay.

### WDC-DEV-032 (covers WAVE-DEV-032)

Vite startup failure fixture MUST verify explicit failure-state handling.

### WDC-DEV-033 (covers WAVE-DEV-033)

Build-retry restart-strength fixtures MUST verify strength bits are preserved.

### WDC-DEV-034 (covers WAVE-DEV-034)

App startup failure fixture MUST verify actionable propagation.

### WDC-DEV-035 (covers WAVE-DEV-035)

Readiness timeout fixtures MUST verify failure-state gating.

### WDC-DEV-036 (covers WAVE-DEV-036)

Config-reload fixtures MUST verify full framework-injected field preservation.

### WDC-DEV-041 (covers WAVE-DEV-041)

App stop failure fixtures MUST verify caller-observable error propagation.

### WDC-EVT-001 (covers WAVE-EVT-001)

Mixed-strength/mixed-match dedupe fixtures MUST verify non-lossy strongest-work
aggregation.

### WDC-EVT-019 (covers WAVE-EVT-019)

Blocking phase failure fixtures MUST verify success-signal suppression.

### WDC-EVT-023 (covers WAVE-EVT-023)

Concurrent restart fixtures with completion-order variance MUST verify
deterministic strongest-intent outcome.

### WDC-EVT-028 (covers WAVE-EVT-028)

Dynamic directory watch-expansion failure fixtures MUST verify actionable
failure.

### WDC-EVT-029 (covers WAVE-EVT-029)

CSS artifact read-failure fixtures MUST verify no success-style CSS payload
emission.

### WDC-EVT-030 (covers WAVE-EVT-030)

Implicit-work + callback-restart fixtures MUST verify no downgrade of
recompile-required outcome.

### WDC-EVT-031 (covers WAVE-EVT-031)

Watcher teardown-race fixtures MUST verify stable watcher-channel source.

### WDC-EVT-032 (covers WAVE-EVT-032)

Stale-watch removal-failure fixtures MUST verify explicit failure handling and
state consistency.

### WDC-EVT-033 (covers WAVE-EVT-033)

Closed error-channel fixtures MUST verify bounded watch-loop termination without
nil-error spin.

### WDC-STATIC-005 (covers WAVE-STATIC-005)

Filemap rotation cleanup-failure fixtures MUST verify explicit policy
conformance.

### WDC-STATIC-015 (covers WAVE-STATIC-015)

Granular stale-artifact removal-failure fixtures MUST verify explicit policy
conformance.

### WDC-CSS-003 (covers WAVE-CSS-003)

Normal CSS rotation cleanup-failure fixtures MUST verify explicit policy
conformance.

### WDC-SCHEMA-007 (covers WAVE-SCHEMA-007)

Reserved schema-key collision fixtures MUST fail instead of silently overriding
reserved sections.
