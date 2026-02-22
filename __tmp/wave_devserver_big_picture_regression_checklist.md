# Wave Devserver Big-Picture Regression Matrix

Goal: lock down the full watcher -> planner -> build/runtime -> browser feedback
loop so refactors cannot silently regress behavior.

## 1) Config authority and short-circuit behavior

- [x] semantic config mutation (write/create/remove/rename + path aliases)
      triggers config restart and skips non-config execution
- [x] no-op config mutation (same bytes/semantics) is true no-op (no restart, no
      broadcast) and logs explicit no-op message
- [x] mixed batch with semantic config mutation + non-config events still
      short-circuits to config restart only
- [x] mixed batch with no-op config mutation + non-config events ignores config
      event and processes non-config events

## 2) Event classification and watched-file semantics

- [x] css entry and css import edits classify as css work (critical/normal) and
      never trigger unnecessary restart
- [x] framework watch patterns (route registry/template) are recognized across
      write/create/remove/rename
- [x] markdown inside private static defaults to private-static behavior without
      markdown include override
- [x] markdown include override switches markdown behavior to revalidate flow
      (no hard reload)
- [x] path alias/symlink-equivalent paths classify identically to canonical
      paths

## 3) Mixed-batch precedence and minimum-work guarantees

- [x] public-static + private-static in same batch uses correct browser action
      precedence and avoids wasted work
- [x] revalidate-eligible markdown + hard-reload framework event in same batch
      resolves to hard reload
- [x] css hot-reload candidate + go compile/restart event in same batch resolves
      to compile/restart path, not css-only path
- [x] run-on-change-only + implicit-build event mixes preserve minimal build
      work and hook execution ordering

## 4) Build phase correctness and granularity

- [x] changed-path processing is used when possible; full scans only when
      required
- [x] public/private file map updates stay correct across create/remove/rename
      and directory-level rename bursts
- [x] css hot-reload payloads are emitted only when rebuild output is valid and
      fresh
- [x] build failure short-circuits downstream restart/browser actions
      appropriately and recovers on next valid edit

## 5) Browser update semantics

- [x] rebuilding overlay appears for operations that require it and is skipped
      when explicitly suppressed
- [x] hard reload / revalidate / css hot reload / vite invalidate are chosen
      from first-principles precedence
- [x] invalidate-vite fallback path is deterministic when endpoint fails
- [x] wait flags (WaitForApp / WaitForVite) match the selected browser action
      contract

## 6) Runtime lifecycle and resiliency

- [x] restart intents (recompile-go / no-go / config restart) are queued and
      consumed correctly under bursty edits
- [x] no-wait hooks never block main pipeline and obey concurrency limits
- [x] app stop/start behavior is correct for single hard-reload vs batch
      hard-reload vs no-stop paths

## 7) Observability contracts

- [x] each externally meaningful no-op/fallback path has a deterministic log
      message
- [x] watcher event logs include cycle/batch trace fields for debounced batches

## 8) Consumer-level integration confidence

- [x] site-style integration harness covers all core file categories and
      operation types
- [x] mixed-batch site-level tests exist for cross-component precedence
      scenarios
- [x] regression tests avoid coupling to internal/site specifics while
      preserving behavior contracts
