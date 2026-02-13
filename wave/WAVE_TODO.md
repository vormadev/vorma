WAVE TODO

Context:

- This list is rebuilt from a full `/wave/*` audit.
- Items are tracked as candidates until explicitly accepted/completed.

## 1) Watcher Intake Decomposition

- Split watcher event intake/debounce/filter concerns into focused modules so
  path matching and dedup rules stay pure and independently testable.
- Keep watcher side-effect boundaries (`fsnotify` IO and channel fanout) thin.

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
