WAVE TODO

Context:

- This list is rebuilt from a full `/wave/*` audit.
- Items are tracked as candidates until explicitly accepted/completed.

## 1) Hook Timeout Controls

- Add explicit timeout controls for pre/concurrent/post hook command execution
  so a single stuck hook cannot stall watch cycles indefinitely.
- Keep timeout policy configurable and stage-aware without introducing hidden
  defaults that reduce user control.

## 2) Diagnostics (deferred)

- Revisit only after functionality/structure items above are stable.
- If reintroduced, scope must remain minimal and directly actionable.
- Target files: `wave/tooling/cli.go`, `wave/tooling/events*.go`.

## RULES

- No non-JSON authored config path.
- No builder-pattern APIs in Go.
- No duplicate default-path APIs.
- No back-compat adapters while sub-1.0.
- Keep runtime/build boundaries strict.
