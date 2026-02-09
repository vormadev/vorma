# wave/tooling Conformance Issues

Status: Active  
Last Updated: 2026-02-09  
Purpose: Track `wave/tooling` implementation divergences for build/dev
control-plane contracts.

## Open `wave/tooling` Issues

| WCI ID  | Affected Requirement(s)       | Summary                                                                                                                           | Status |
| ------- | ----------------------------- | --------------------------------------------------------------------------------------------------------------------------------- | ------ |
| WCI-001 | WAVE-DEV-012                  | Cycle-vite reload source exclusivity ambiguity in current orchestration path.                                                     | open   |
| WCI-002 | WAVE-DEV-032                  | Vite startup failure currently log-only continuation (`LAB-VITEUTIL-ISSUE-001`).                                                  | open   |
| WCI-003 | WAVE-DEV-033                  | `waitForBuildRetry` drains `restartCh` but drops `recompileGo`/`config_restart` strength bits.                                    | open   |
| WCI-004 | WAVE-DEV-034                  | App startup failure currently non-actionable continuation.                                                                        | open   |
| WCI-005 | WAVE-DEV-035                  | Readiness failures currently warning-only continuation.                                                                           | open   |
| WCI-006 | WAVE-EVT-019                  | Pre/concurrent/post hook and build errors are logged but do not gate success-style browser behavior.                              | open   |
| WCI-007 | WAVE-STATIC-005, WAVE-CSS-003 | Hashed-artifact rotation cleanup failures warning-only.                                                                           | open   |
| WCI-008 | WAVE-EVT-023                  | Concurrent restart arbitration order-sensitive downgrade risk.                                                                    | open   |
| WCI-009 | WAVE-DEV-036                  | Config reload preserves watch patterns only and drops other framework runtime fields (schema extensions + framework build hooks). | open   |
| WCI-010 | WAVE-DEV-030                  | Revalidate script path lacks rejection cleanup for overlay state.                                                                 | open   |
| WCI-011 | WAVE-EVT-001                  | Pattern-level dedupe may drop stronger implicit work.                                                                             | open   |
| WCI-012 | WAVE-STATIC-015               | Granular stale-artifact removal failure silently ignored.                                                                         | open   |
| WCI-013 | WAVE-SCHEMA-007               | Framework schema extensions can override reserved sections.                                                                       | open   |
| WCI-014 | WAVE-EVT-028                  | Directory watch-expansion failure silently ignored.                                                                               | open   |
| WCI-015 | WAVE-EVT-029                  | CSS hot-reload path ignores CSS read failures.                                                                                    | open   |
| WCI-016 | WAVE-EVT-030                  | Callback restart can downgrade implicit Go recompile intent.                                                                      | open   |
| WCI-017 | WAVE-EVT-031                  | Watch-loop reads channels from mutable `s.watcher` pointer under teardown/rebuild races.                                          | open   |
| WCI-018 | WAVE-DEV-041                  | stopApp suppresses kill/wait failures.                                                                                            | open   |
| WCI-019 | WAVE-EVT-032                  | Stale-watch removal errors dropped; tracked state unconditionally deleted.                                                        | open   |
| WCI-020 | WAVE-EVT-033                  | Closed error-channel receive not treated as watcher-loop shutdown.                                                                | open   |

## Source Validation Notes (`E2-R5`)

- `WCI-001`: `wave/tooling/broadcast.go` documents cycle-vite as reload-source
  exclusive, but `broadcastReload` still sends Wave reload payload after
  `cycleVite()`.
- `WCI-002`: `wave/tooling/devserver.go` logs `startVite` failure and continues
  loop/startup instead of entering explicit failure flow.
- `WCI-003`: `wave/tooling/devserver.go` `waitForBuildRetry` consumes one
  `restartCh` request and does not preserve request strength bits.
- `WCI-004`: `wave/tooling/devserver.go` `startApp` logs `cmd.Start` failure and
  returns without propagating actionable error.
- `WCI-005`: `wave/tooling/devserver.go` readiness helpers return `bool`, but
  reload/cycle orchestration does not gate continuation on failure.
- `WCI-006`: `wave/tooling/events.go` pre/concurrent/post hook and build-phase
  failures are often logged and execution continues into success-style browser
  phase decisions.
- `WCI-007`: `wave/tooling/css.go` and `wave/tooling/static.go` old-artifact
  cleanup failures are warning-only in rotation paths.
- `WCI-008`: `wave/tooling/events.go` restart-action processing returns on first
  trigger action, allowing order-sensitive downgrade when stronger restart
  action appears later.
- `WCI-009`: `wave/tooling/devserver.go` `reloadConfig` preserves only watch
  pattern fields and public-file-map out-dir, dropping other framework-injected
  runtime fields.
- `WCI-010`: `wave/refresh.go` revalidate flow removes rebuilding overlay on
  resolve path but lacks reject cleanup path.
- `WCI-011`: `wave/tooling/events.go` pattern-level dedupe (`handledPatterns`)
  can drop stronger implicit work for additional events with the same pattern
  key.
- `WCI-012`: `wave/tooling/static.go` per-file stale artifact removals can warn
  and continue without explicit failure handling.
- `WCI-013`: `wave/tooling/schema.go` copies framework schema extensions into
  top-level properties without reserved-key collision guard.
- `WCI-014`: `wave/tooling/events.go` directory-create watch expansion path
  calls `watcher.AddDir` without handling returned error.
- `WCI-015`: `wave/tooling/events.go` CSS hot-reload path ignores read errors
  from `ReadCriticalCSS`/`ReadNormalCSSURL`.
- `WCI-016`: `wave/tooling/events.go` callback-triggered restart return path can
  bypass stronger implicit rebuild work for the same event.
- `WCI-017`: `wave/tooling/events.go` watch loop reads channels via mutable
  `s.watcher` pointer while teardown/rebuild can swap that pointer.
- `WCI-018`: `wave/tooling/devserver.go` `stopApp` ignores `Kill`/`Wait` errors
  and returns nil.
- `WCI-019`: `wave/tooling/watcher.go` `RemoveStale` deletes tracked paths
  regardless of `fsWatch.Remove` error result.
- `WCI-020`: `wave/tooling/events.go` watch loop error-channel receive does not
  handle closed-channel termination semantics.

## Legacy-Test Input Check (`E2-R5`)

- No legacy tests outside `conformance/**` are currently present under
  `wave/tooling`; this replay slice is source-only for this package path.
