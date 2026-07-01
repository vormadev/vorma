# Build / Live-State Language Cleanup

Status: open

Audit maintainer-facing comments, error messages, and docs that still say "build entry"
when they mean the app server executable running in live-state mode.

Why this exists:

- The current dev loop has one app-linked binary per iteration: the app server. It is
  built once, run once with `__VORMA_LIVE_BUILD_STATE=1` to emit live-state JSON, and run
  again as the serving app.
- The per-app build entry still exists, but it is the long-lived orchestrator shim that
  calls `vorma_build::run(app_config)`. It is not the per-iteration live-state emitter.
- Historical notes found stale wording after the single-binary collapse. Some current
  "build entry" language is legitimate, so this is an audit rather than a blind rename.

Known places to inspect:

- `crates/vorma-build/src/live_state_command.rs`: comments and errors that refer to a
  prebuilt build-entry executable or build-entry live-state command.
- `crates/vorma-contract/src/live_state.rs`: comments around the live-state env key, env
  value, and payload emitter.
- `crates/vorma-build/src/output_lock.rs`, `crates/vorma-build/src/entrypoint.rs`,
  `crates/vorma-build/src/lib.rs`, and `crates/vorma-build/README.md`: these may be
  correct when they refer to the real build entry/orchestrator surface.
- `docs/maintainer/ARCHITECTURE.md` and `docs/maintainer/REMINDERS.md`: these already
  describe the current model and should be preserved.

Done means:

- "Build entry" is used only for the actual build/orchestrator entrypoint.
- The live-state executable is described as the app server binary wherever that is the
  current truth.
- Error strings remain clear to app authors and maintainers.
- Tests or snapshots that assert these strings are updated deliberately.
