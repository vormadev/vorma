# Wave/Vite Strict Regression Checklist (2026-03-01)

Context:
- Goal is strict behavioral parity/stability vs `main` for dev runtime.
- A/B bind test result: `rolldown-vite@7.1.14` and `7.3.1` behaved the same under Node `v24.13.1` when started without `--host` (IPv6 loopback bind, `127.0.0.1` refused).

Checklist (ordered hardest/systemic first):

- [x] Restore fail-fast lifecycle semantics for runtime startup (do not continue in degraded state when Vite or app startup fails).
  - Current new behavior to remove:
    - `wave/wavedev/devserver/devserver.go` (`startRunCycleRuntime`, `StartApp`, `StartVite`) logs and continues.
  - Old behavior reference:
    - `main:wave/internal/ki/dev_core.go`
    - `main:wave/internal/ki/dumb_little_helpers.go`
  - Validation:
    - inject startup failure and confirm run loop exits loudly instead of continuing.

- [x] Fix refresh sidecar port propagation so runtime always uses the actual started sidecar port.
  - Current issue:
    - refresh server assigns `server.RefreshPort`, but env writer `setRefreshServerPort` is not called in runtime path.
    - runtime script can fall back to `10000`.
  - Affected files:
    - `wave/wavedev/devserver/devserver.go`
    - `wave/wave.go`
  - Validation:
    - inspect rendered runtime script and confirm port matches started sidecar every time.

- [x] Fix refresh host/bind consistency to eliminate IPv4/IPv6 loopback mismatch flakes.
  - Current issue:
    - sidecar bind is `127.0.0.1:<port>`, while browser URL host may resolve as `localhost`/`::1`.
  - Affected files:
    - `wave/wavedev/devserver/internal/reloadwait/reloadwait.go`
    - `wave/internal/waveruntime/waveruntime.go`
  - Validation:
    - deterministic success for `localhost`, `127.0.0.1`, and `[::1]` page access modes.

- [x] Reconcile readiness-failure handling with strict behavior (no silent skip/broadcast suppression on readiness errors).
  - Current issue:
    - readiness errors are logged as warning and broadcast is skipped.
  - Affected files:
    - `wave/wavedev/devserver/internal/reloadwait/reloadwait.go`
  - Old behavior reference:
    - `main:wave/internal/ki/dumb_little_helpers.go`
  - Validation:
    - force readiness failure and confirm deterministic failure path, not silent degraded behavior.

- [x] Reconcile readiness wait/backoff policy drift from old short budget to current long budget.
  - Current default:
    - `maxWait=25s`, exponential backoff.
  - Old behavior:
    - tighter `~3s` effective budget.
  - Affected files:
    - `wave/wavedev/devserver/internal/appsupervisor/appsupervisor.go`
    - old reference in `main:wave/internal/ki/dumb_little_helpers.go`
  - Validation:
    - cold/warm rebuild latency and failure timing match intended strict policy.

- [x] Resolve Vite port policy drift (`InitPort` semantics and fallback behavior) to avoid new collision/flaky profiles.
  - Current new behavior:
    - `InitPort(defaultPort)` honors caller default.
    - script fallback can use `5173` when env parse fails.
  - Old behavior:
    - initialization started from `5199` regardless of caller default.
  - Affected files:
    - `lab/viteutil/viteutil.go`
    - old reference in `main:kit/viteutil/viteutil.go`
  - Validation:
    - repeatable multi-run/multi-process startup without accidental cross-project collisions.

- [x] Investigate and fix `.well-known/appspecific` request bleed into markdown sitemap/page-detail pathing.
  - Symptom:
    - request for `/.well-known/appspecific/com.chrome.devtools.json` triggers markdown dir/sitemap errors and loader error path.
  - Validation:
    - devtools probe requests no longer trigger page-detail generation errors.

- [x] Decide and enforce deterministic Vite client URL host policy (even if not a new-vs-old delta) to remove resolver-dependent behavior.
  - Context:
    - both old and new rely on `localhost` script URLs with no explicit Vite host.
  - Validation:
    - no `@vite/client` connection refusal under local resolver/address-family variance.

Guardrail tests to add while fixing:

- [x] Add non-E2E regression tests for each fixed behavioral bug path (startup fail-fast, refresh port propagation, host/bind consistency, readiness error semantics, and `.well-known` isolation).
- [x] Keep tests black-box where possible and avoid implementation mirroring.
