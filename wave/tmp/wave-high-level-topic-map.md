Wave High-Level Topic Map

Purpose

- Define complete broad-topic coverage for the wave behavioral audit before
  line-level specification extraction.
- Establish the authoritative topic index that the normative spec and regression
  diff will reference.

Guiding Principle

- Across build orchestration, watch/reload behavior, static/CSS processing, and
  framework integration, the design objective is to do only the minimum amount
  of work required to ensure runtime outputs and data are not stale.

Documentation Discipline

- This document remains strictly declarative and present-tense.
- Historical narration, backward-looking commentary, conversational notes, and
  changelog-style accumulation are prohibited.
- Superseded statements are replaced in place rather than retained as history.

Scope Definition

- In scope: `/wave/*` behavior that is externally observable to developers and
  users.
- Out of scope: non-`/wave/*` implementation details unless required to explain
  wave behavior.

Branch Root Coverage

- Baseline branch (`main`) wave roots:
    - `wave/wave.go`
    - `wave/internal/ki`
- Working branch (`refactor-2026-8`) wave roots:
    - `wave/*.go`
    - `wave/internal/pathnorm`
    - `wave/tooling`

Broad Behavior Topics

- `HL-01` Config parsing, defaults, and environment override rules
    - Core/vite/watch block validation, required fields, default values, runtime
      mode and env key semantics.
- `HL-02` Runtime state model and cache access surfaces
    - Runtime cache initialization, accessor contracts, symbol/state ownership,
      and non-stale read expectations.
- `HL-03` Dist layout and filesystem abstraction semantics
    - Dist directory shape contracts, embedded/non-embedded FS behavior, and
      path/layout assumptions.
- `HL-04` URL/filemap/public-path generation contracts
    - Public URL resolution, hashed filename mapping, prefix behavior, and
      fallback semantics.
- `HL-05` CSS build graph and output semantics
    - CSS entry processing, resolver behavior, critical/non-critical outputs,
      hash references, and import dependency tracking.
- `HL-06` Static asset pipeline semantics
    - Public/private/static resolution, hashing/no-hash handling, incremental
      reuse/delete behavior, and atomic output guarantees.
- `HL-07` Build orchestration ordering and hook behavior
    - Pre/post file processing, build hook ordering, schema/output emission, and
      go build coupling semantics.
- `HL-08` Dev bootstrap and lifecycle initialization
    - Dev startup ordering, rebuild handling, initial process launch, and
      teardown guarantees.
- `HL-09` Watcher setup and directory-plan behavior
    - Watch-root normalization, include/exclude expansion, default watch rules,
      and recursive directory registration.
- `HL-10` Watch event intake and debounce behavior
    - Debounce windows, batch coalescing, rename/create/delete handling, and
      ignored event classes.
- `HL-11` Event classification and matching semantics
    - Per-event detail derivation, go/css/other categorization, watched-file
      matching, and ignore/full-reset markers.
- `HL-12` On-change hook planning and execution semantics
    - Timing classes (`pre/concurrent/post/no-wait`), exclusion filtering,
      command resolution, and callback ordering guarantees.
- `HL-13` Rebuild/restart decision matrix
    - Go recompile paths, app restart vs no-restart, revalidate-only flow,
      hard-reload gating, and batch-vs-single-event differences.
- `HL-14` App process lifecycle behavior
    - Binary compile/run/terminate semantics, graceful shutdown handling, and
      restart sequencing guarantees.
- `HL-15` Vite integration lifecycle behavior
    - Dev/prod vite invocation, manifest/outdir contracts, readiness coupling,
      and invalidation behavior.
- `HL-16` Refresh server and websocket protocol semantics
    - Sidecar refresh server lifecycle, websocket registration/broadcast rules,
      and client channel behavior.
- `HL-17` Browser reload/revalidate/hot-css behavior
    - `rebuilding/other/normal/critical/revalidate` payload semantics,
      hard-reload vs CSS hot swap behavior, and client script invariants.
- `HL-18` Readiness/wait/timeout policy behavior
    - App/vite readiness probes, wait bounds, retry schedules, and failure
      handling semantics.
- `HL-19` Static serving and middleware behavior
    - Public asset serve decisions, cache headers, prefix rules, and favicon
      redirect behavior.
- `HL-20` Lock/single-runner and command serialization behavior
    - Lock file semantics, concurrent-run prevention, and command ordering
      constraints.
- `HL-21` CLI and run-mode contracts
    - CLI argument/config behavior, dev/prod run paths, and user-visible command
      lifecycle outputs.
- `HL-22` Error handling and non-mutation safety guarantees
    - Validation panics/errors, fallback/no-op behavior, and state protection
      under failed operations.
- `HL-23` Cross-platform path and OS behavior
    - Path normalization contracts, unix/windows lock/platform deltas, and
      runtime portability guarantees.
- `HL-24` Framework integration runtime contracts
    - Wave runtime integration surfaces consumed by frameworks and required
      observable parity across integrations.
- `HL-25` Development vs production behavioral deltas
    - Explicit mode-driven differences in builds, serving, processing, and
      runtime mutation behavior.
- `HL-26` Public API behavior surface
    - Exported wave API contracts and user-observable behavior guarantees.

Cross-Branch Mapping Intention

- Each `HL-*` topic maps to a detailed normative spec section with stable
  requirement IDs.
- Branch comparison evaluates behavior parity by requirement ID, not by API
  naming or file layout.

Next-Step Boundary

- This document is the checkpoint output for broad-topic coverage only.
- Next step is line-level extraction of observable behavior stories under each
  `HL-*` topic for baseline branch first.
