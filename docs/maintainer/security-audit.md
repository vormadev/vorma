# Security Audit

Purpose: make the Rust framework easy enough to reason about that we can have real
confidence in its security posture before publication. This is not a formal third-party
audit and cannot prove "100% secure", but it should be a serious whole-system pass.

Method: start from first principles. Identify what must be protected, who can influence
the system, every way data/control enters the Rust code, every boundary where trust
changes, and every state transition that could expose or corrupt user/application data.
Then follow those flows through the code. No subsystem or bug class is pre-excluded.

File ledger rule: every Rust source path that is read for this audit must be listed in the
file ledger below. A path is not closed until it has been read with the security model in
mind and either no issue was found or every issue found there has been fixed and verified.

Current file-ledger states:

- Closed: read, security-reviewed, and either clean or fixed and verified.
- In review: read, but not yet closed.
- Pending: not yet read for this audit.

Current ledger state: every Rust source path returned by `rg --files -g '*.rs'` is listed
exactly once below, all listed Rust source paths are closed, and there are no in-review or
pending Rust source paths.

Ledger coverage checks:

```sh
comm -23 <(rg --files -g '*.rs' | sort) <(sed -n '/^### Closed Source Paths$/,/^### Pending Source Paths$/p' docs/maintainer/security-audit.md | rg -o '`[^`]+\.rs`' | tr -d '`' | sort -u)
sed -n '/^### Closed Source Paths$/,/^### Pending Source Paths$/p' docs/maintainer/security-audit.md | rg -o '`[^`]+\.rs`' | tr -d '`' | sort | uniq -d
```

Both commands must print no output. The first command detects Rust source paths missing
from the ledger; the second command detects duplicate ledger entries. Both commands read
only the ledger section so command examples elsewhere in this document cannot affect the
result.

## Done

- [x] Defined the protected assets, potential influence, trust boundaries, and security
      invariants for the Rust framework from first principles.
- [x] Completed an initial runtime request-handling pass from HTTP bytes through body
      limits, request parsing, middleware, handlers, response effects, headers, cookies,
      redirects, HTML, JSON payloads, and static asset serving.
- [x] Completed an initial build/dev pass from configuration through filesystem layout,
      generated output writes, public static publication, Vite manifest promotion, process
      spawning, dev mux/RPC/WebSocket exposure, and cancellation.
- [x] Completed an initial macro/codegen, matcher, task runtime, WASM ABI, dependency
      policy, and package-contents pass.
- [x] `vorma-build` production Vite manifest promotion uses the same no-symlink output
      discipline as the rest of generated output handling. The temp manifest path must be
      a regular file, the retained manifest parent must not traverse symlinks, and a
      hostile symlink in the temporary manifest location must not be promoted into
      retained framework state.
- [x] Public output discovery rejects symlink files instead of placing them in the runtime
      manifest and depending on runtime static serving to reject them later.
- [x] Child process environment scrubbing removes every Vorma-owned internal mode or
      control-channel env key, including Vite plugin port/token keys, before commands add
      back the specific env they intentionally need.
- [x] Redirect validation rejects hostless absolute `http`/`https` forms and
      backslash-containing relative locations instead of relying on downstream browser
      interpretation.
- [x] ETag middleware enforces its maximum body size while collecting, not merely by
      trusting `Body::size_hint()`.
- [x] Physical static file re-hashing re-checks that the source is still a regular file
      and not a symlink.
- [x] Runtime request path decoding rejects invalid percent-decoded UTF-8 with a 400
      instead of routing a lossy replacement-character path.
- [x] Inverse-audited the security changes and removed runtime manifest re-validation that
      duplicated build-time manifest checks and the runtime public-root containment guard.
      Runtime manifest loading still normalizes manifest paths; actual static asset reads
      still require the requested URL to be listed in the manifest and the resolved file
      to stay inside the public output root.
- [x] Build config now rejects `frontend_config.public_static_src_dir` values outside
      `root_dir`, preventing an app config typo or absolute external path from publishing
      files outside the application root as public assets.
- [x] Build config now rejects symlinked `frontend_config.public_static_src_dir` roots,
      preventing public static publication from depending on implicit walker behavior for
      a security boundary.
- [x] Build config now rejects JavaScript package-manager dirs, frontend entry files,
      critical CSS entry files, and Vite config files outside `root_dir`.
- [x] Critical CSS bundling now canonicalizes the entry file and every imported CSS file
      under `root_dir` before Lightning CSS reads it, preventing `@import` from pulling
      outside-root files into generated CSS.
- [x] Build-entry live-state validation now rejects view module `client_file` imports
      outside `root_dir`, backslash-separated imports, duplicate module imports, and
      mismatched view-module map keys before those values can reach Vite manifests.
- [x] Client matcher WASM input is capped at 64 KiB on both the Rust ABI and TypeScript
      wrapper side, so oversized input returns an explicit failure instead of relying on
      allocator behavior.
- [x] Client matcher WASM allocation/deallocation now uses exact allocation layouts and a
      live-allocation registry. Wrong-length or unknown deallocation requests are ignored
      instead of reconstructing a Rust allocation from untrusted ABI arguments.
- [x] `vorma-tasks` cross-execution-context caching is bounded by `TasksOptions`. Existing
      cached or in-flight entries remain usable, but new keys bypass shared caching once
      the configured entry limit is reached instead of growing shared memory without
      bound.
- [x] `vorma-matcher` no longer depends on recursive tree traversal or recursive tree drop
      for deep dynamic paths, and match scoring no longer uses a `u16` accumulator.
- [x] The client-WASM fuzz target checks for null allocation before copying fuzzer bytes,
      so oversized fuzz inputs exercise the ABI error path instead of writing through a
      null pointer.
- [x] The Bombadil framework harness now uses safe process-group setup, rejects zero
      intensity, checks time-limit multiplication for overflow, and creates dev-build temp
      target directories with a fresh non-following `create_dir` path.
- [x] `cargo deny check licenses bans sources advisories` passes.
- [x] `cargo audit` passes.
- [x] Published crate file lists for `vorma-matcher`, `vorma-tasks`, `vorma-macros`,
      `vorma`, and `vorma-build` contain only expected source, tests, README, and Cargo
      metadata.
- [x] Targeted Rust tests/checks passed for `vorma`, `vorma-build`, `vorma-client-wasm`,
      `vorma-matcher`, `vorma-tasks`, `vorma-macros`, `vorma-xtask`,
      `vorma-framework-tests`, `vorma-minimal-example`, and the standalone fuzz package.
- [x] Full gate passed.

## File Read / Closure Ledger

### Closed Source Paths

- [x] `crates/vorma-build/src/build_layout.rs`
- [x] `crates/vorma-build/src/activation.rs`
- [x] `crates/vorma-build/src/browser_sync.rs`
- [x] `crates/vorma-build/src/build_cancel.rs`
- [x] `crates/vorma-build/src/cargo_target.rs`
- [x] `crates/vorma-build/src/command_runner.rs`
- [x] `crates/vorma-build/src/config.rs`
- [x] `crates/vorma-build/src/constants.rs`
- [x] `crates/vorma-build/src/cssbundle.rs`
- [x] `crates/vorma-build/src/dev_lifecycle.rs`
- [x] `crates/vorma-build/src/dev_loop.rs`
- [x] `crates/vorma-build/src/dev_mux.rs`
- [x] `crates/vorma-build/src/dev_watcher.rs`
- [x] `crates/vorma-build/src/document_hash.rs`
- [x] `crates/vorma-build/src/frontend_toolchain.rs`
- [x] `crates/vorma-build/src/fswatcher.rs`
- [x] `crates/vorma-build/src/generation.rs`
- [x] `crates/vorma-build/src/generation_workspace.rs`
- [x] `crates/vorma-build/src/globset.rs`
- [x] `crates/vorma-build/src/lib.rs`
- [x] `crates/vorma-build/src/live_refresh.rs`
- [x] `crates/vorma-build/src/live_state.rs`
- [x] `crates/vorma-build/src/local_cargo.rs`
- [x] `crates/vorma-build/src/manifest.rs`
- [x] `crates/vorma-build/src/manifest/public_output.rs`
- [x] `crates/vorma-build/src/manifest/vite_projection.rs`
- [x] `crates/vorma-build/src/pipeline.rs`
- [x] `crates/vorma-build/src/process_wait.rs`
- [x] `crates/vorma-build/src/public_static_inputs.rs`
- [x] `crates/vorma-build/src/runtime.rs`
- [x] `crates/vorma-build/src/session.rs`
- [x] `crates/vorma-build/src/signals.rs`
- [x] `crates/vorma-build/src/static_build.rs`
- [x] `crates/vorma-build/src/staticproc.rs`
- [x] `crates/vorma-build/src/supervisor.rs`
- [x] `crates/vorma-build/src/test_support.rs`
- [x] `crates/vorma-build/src/ts_gen.rs`
- [x] `crates/vorma-build/src/ts_modules.rs`
- [x] `crates/vorma-build/src/utils.rs`
- [x] `crates/vorma-build/src/vite_plugin.rs`
- [x] `crates/vorma-build/src/viteutil.rs`
- [x] `crates/vorma-build/src/watch_config.rs`
- [x] `crates/vorma-build/src/watch_plan.rs`
- [x] `crates/vorma-build/src/work_queue.rs`
- [x] `crates/vorma-client-wasm/src/abi.rs`
- [x] `crates/vorma-client-wasm/src/encoding.rs`
- [x] `crates/vorma-client-wasm/src/lib.rs`
- [x] `crates/vorma-client-wasm/src/output.rs`
- [x] `crates/vorma-client-wasm/src/registry.rs`
- [x] `crates/vorma-macros/src/app_decl.rs`
- [x] `crates/vorma-macros/src/lib.rs`
- [x] `crates/vorma-macros/src/ts_gen_derive.rs`
- [x] `crates/vorma-matcher/src/builder.rs`
- [x] `crates/vorma-matcher/src/lib.rs`
- [x] `crates/vorma-matcher/src/match_result.rs`
- [x] `crates/vorma-matcher/src/matcher.rs`
- [x] `crates/vorma-matcher/src/options.rs`
- [x] `crates/vorma-matcher/src/parse.rs`
- [x] `crates/vorma-matcher/src/pattern.rs`
- [x] `crates/vorma-matcher/src/segment.rs`
- [x] `crates/vorma-matcher/src/tree.rs`
- [x] `crates/vorma-matcher/tests/grammar.rs`
- [x] `crates/vorma-matcher/tests/matching_semantics.rs`
- [x] `crates/vorma-tasks/src/cancel.rs`
- [x] `crates/vorma-tasks/src/clock.rs`
- [x] `crates/vorma-tasks/src/error.rs`
- [x] `crates/vorma-tasks/src/key.rs`
- [x] `crates/vorma-tasks/src/lib.rs`
- [x] `crates/vorma-tasks/src/observer.rs`
- [x] `crates/vorma-tasks/src/overrides.rs`
- [x] `crates/vorma-tasks/src/store.rs`
- [x] `crates/vorma-tasks/src/task.rs`
- [x] `crates/vorma-tasks/tests/tasks.rs`
- [x] `crates/vorma/src/api.rs`
- [x] `crates/vorma/src/config.rs`
- [x] `crates/vorma/src/constants.rs`
- [x] `crates/vorma/src/core.rs`
- [x] `crates/vorma/src/core/context.rs`
- [x] `crates/vorma/src/core/contract.rs`
- [x] `crates/vorma/src/core/pattern.rs`
- [x] `crates/vorma/src/core/runner.rs`
- [x] `crates/vorma/src/core/runtime.rs`
- [x] `crates/vorma/src/core_tests.rs`
- [x] `crates/vorma/src/document.rs`
- [x] `crates/vorma/src/envutil.rs`
- [x] `crates/vorma/src/error.rs`
- [x] `crates/vorma/src/handler.rs`
- [x] `crates/vorma/src/handler_tests/api_response.rs`
- [x] `crates/vorma/src/handler_tests/dev_assets.rs`
- [x] `crates/vorma/src/handler_tests/mod.rs`
- [x] `crates/vorma/src/handler_tests/view_response.rs`
- [x] `crates/vorma/src/head.rs`
- [x] `crates/vorma/src/htmlutil.rs`
- [x] `crates/vorma/src/init.rs`
- [x] `crates/vorma/src/init/public_asset_service.rs`
- [x] `crates/vorma/src/init/resource_service.rs`
- [x] `crates/vorma/src/init/view_service.rs`
- [x] `crates/vorma/src/init_tests/document_and_view_shell.rs`
- [x] `crates/vorma/src/init_tests/method_and_request_boundaries.rs`
- [x] `crates/vorma/src/init_tests/middleware_and_effects.rs`
- [x] `crates/vorma/src/init_tests/mod.rs`
- [x] `crates/vorma/src/init_tests/public_assets_and_resources.rs`
- [x] `crates/vorma/src/init_tests/service_body_and_context.rs`
- [x] `crates/vorma/src/lib.rs`
- [x] `crates/vorma/src/manifest.rs`
- [x] `crates/vorma/src/middleware.rs`
- [x] `crates/vorma/src/mux.rs`
- [x] `crates/vorma/src/mux/api.rs`
- [x] `crates/vorma/src/mux/context.rs`
- [x] `crates/vorma/src/mux/error.rs`
- [x] `crates/vorma/src/mux/input.rs`
- [x] `crates/vorma/src/mux/middleware.rs`
- [x] `crates/vorma/src/mux/nested.rs`
- [x] `crates/vorma/src/mux/ordered_parallel.rs`
- [x] `crates/vorma/src/mux/request.rs`
- [x] `crates/vorma/src/mux/task.rs`
- [x] `crates/vorma/src/mux_tests/middleware.rs`
- [x] `crates/vorma/src/mux_tests/mod.rs`
- [x] `crates/vorma/src/mux_tests/nested_views.rs`
- [x] `crates/vorma/src/mux_tests/router_and_resources.rs`
- [x] `crates/vorma/src/request.rs`
- [x] `crates/vorma/src/response.rs`
- [x] `crates/vorma/src/searchparams.rs`
- [x] `crates/vorma/src/static.rs`
- [x] `crates/vorma/src/tsgen.rs`
- [x] `crates/vorma/src/tsgen/identifier.rs`
- [x] `crates/vorma/src/tsgen/render.rs`
- [x] `crates/vorma/src/tsgen/resolve.rs`
- [x] `crates/vorma/src/view_payload.rs`
- [x] `crates/vorma/tests/public_api.rs`
- [x] `examples/minimal/src/bin/build.rs`
- [x] `examples/minimal/src/bin/server.rs`
- [x] `examples/minimal/src/lib.rs`
- [x] `fuzz/fuzz_targets/client_wasm_protocol.rs`
- [x] `fuzz/fuzz_targets/matcher_patterns.rs`
- [x] `tests/framework/src/bin/bombadil.rs`
- [x] `tests/framework/src/bin/build.rs`
- [x] `tests/framework/src/bin/serve.rs`
- [x] `tests/framework/src/dev_marker.rs`
- [x] `tests/framework/src/lib.rs`
- [x] `tests/framework/src/scenario.rs`
- [x] `xtask/src/gate.rs`
- [x] `xtask/src/main.rs`
- [x] `xtask/src/rust_fuzz.rs`
- [x] `xtask/src/ts_publish.rs`

### In-Review Source Paths

None.

### Pending Source Paths

None.

## Open

### Findings

No open concrete security bug is currently identified. Every Rust source path in the
ledger is closed and the full gate has passed.

### Security Model

- Protected assets:
    - Application state and handler return values.
    - Application source/build files and generated output directories.
    - Runtime manifests and generated TypeScript contracts.
    - Public static assets that are intentionally published.
    - Dev-only Vite/plugin/mux tokens and local process control channels.
    - HTTP response headers, cookies, redirects, HTML/head output, and JSON payloads.
    - Published crate contents and generated WASM/TypeScript artifacts.
- Potential influence:
    - Remote HTTP clients can control request method/path/query/headers/body.
    - Browser clients can control client-side navigation/fetch behavior and may run
      untrusted same-browser pages during dev.
    - Application code controls Vorma config, route patterns, handlers, middleware,
      document/head definitions, static input paths, and generated TypeScript types.
    - Local machine processes can attempt to talk to dev-only loopback servers.
    - Package consumers receive published crates and npm artifacts.
- Trust boundaries:
    - Network request bytes into Vorma runtime handlers.
    - Manifest/generated file contents into runtime serving and client boot.
    - App config/source paths into build-time filesystem writes and command execution.
    - Dev browser/Vite/plugin messages into build runtime control paths.
    - User macro input into generated Rust/TypeScript-facing code.
    - WASM ABI byte buffers into matcher/protocol decoding.
- Security invariants:
    - Runtime static serving must never escape intentional public outputs.
    - Build output writes must not follow symlinks into user-controlled external paths.
    - Dev-only control channels must not be reachable or usable by ordinary remote
      clients.
    - Request body limits must be enforced before unbounded buffering or parsing.
    - Redirects, headers, cookies, HTML, and JSON must be constructed through
      typed/escaped APIs that reject invalid wire values.
    - Generated artifacts must not leak server-only manifests or internals into clients.
    - Process execution must use explicit argv/env/current-dir boundaries, not shell
      interpolation.
    - Published packages must contain only intended files and metadata.

## Next

None.
