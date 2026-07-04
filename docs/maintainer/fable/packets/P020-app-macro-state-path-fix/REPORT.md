# P020 Report

Provenance note: delivered by the executor (Sonnet 5 subagent, 2026-07-02) in its final
message per the anti-truncation convention; Fable placed it (transport escaping undone).

## What changed

- `crates/vorma/src/lib.rs` — `macro_rules! app` now emits
  `type __VormaAppState = $state;` beside the generated module (private, `#[doc(hidden)]`,
  `#[allow(dead_code)]`) and the generated module's `pub type State = $state;` became
  `pub type State = super::__VormaAppState;` — the verified anchor-alias fix. The interim
  constraint doc replaced with a plain statement that any in-scope type path works. One
  doctest gained `# fn main() {}` (see Decisions 4 — a real edge case, root-caused).
- `crates/vorma/tests/app_declaration_state_path_forms.rs` (new) — the red-to-green pin:
  three sibling modules exercising bare local, use-imported, and self-qualified state
  names, each asserting the generated `App::from_config` compiles and resolves
  (deliberately-incomplete config so the pin is compile/resolve-only, no filesystem).
- `vorma-release-quality-findings` ticket — Finding 1 marked RESOLVED (P020, 2026-07-02)
  with citation; Findings 2/3 untouched, still held for batch triage.

No other file touched; the concurrent TS executor's tree untouched.

## Decisions made

1. **Location:** `app!` is a `macro_rules!` in `crates/vorma/src/lib.rs`, not a proc-macro
   in vorma-macros — the packet's phrasing was loose shorthand; the finding itself cites
   the real location, and the fix landed there.
2. **Alias visibility: private, no pub** — the generated module is always a descendant of
   the alias's module, and private items are visible to descendants; minimal surface per
   the plumbing framing.
3. **Hygiene/collision verified, not assumed:** the literal identifier resolves under
   definition-site item hygiene; zero pre-existing `__VormaAppState` uses repo-wide.
4. **Real edge case found and root-caused:** the doctest gate surfaced one failure —
   rustdoc wraps main-less doctests in a synthetic `fn main`, and a `mod` declared inside
   a function body attaches to the enclosing module's tree while a function-local sibling
   item is invisible to ALL path resolution (confirmed with an isolated
   `rustc --edition 2024` repro; unconditional Rust property, not fixable in the macro).
   Extracted rustdoc's generated doctest source to confirm; verified by grep that every
   REAL caller invokes `app!` at true module top level (the documented usage). Fixed the
   one doctest with a hidden `# fn main() {}`, matching every sibling `app!` doctest — no
   macro-shape change, no pin weakening.
5. **Test shape:** ordinary integration-test module per the repo's established `app!`-test
   pattern (trybuild here is compile-fail-only by convention; no new harness introduced).
6. **All three forms pinned** (not just one representative) — the DoD names all three.
7. **Unit-struct state types** to avoid dead_code noise without weakening the pin.
8. **Doctests left on `()` state** where they were — simplest for examples not about
   state-path syntax.
9. **Existing `crate::`-anchored callers deliberately untouched** — old code changing zero
   out of the box is the stronger existing-callers pin.

## Gate results

- Pin file in isolation: 3/3 (red-before proven: the identical file failed pre-fix with
  exactly three E0425s, one per form, rustc itself suggesting the crate:: anchor —
  matching Finding 1's repros; also reproduced against the pre-fix macro with a disposable
  scratch repro, removed and verified absent).
- `cargo test --workspace --all-targets`: **591 passed, 0 failed** (588 baseline + 3 new
  pins; every other block matches baseline exactly).
- `cargo test --workspace --doc`: **52 passed, 0 failed** (vorma 11/11 — this gate is what
  caught the Decision-4 edge case pre-fix; clean after).
- clippy `-D warnings` clean; fmt clean (one scoped write on the new file only); doc build
  `-D warnings` clean with `__VormaAppState` confirmed absent from rendered docs and the
  search index (present only in the raw source viewer, as expected); loom 7/7
  untouched-green.
- Extra confidence:
  `cargo check -p vorma-board-example -p vorma-framework-tests --all-targets` clean — the
  real `crate::`-anchored callers compile unmodified.

## Benchmarks

Not applicable — no request-path/concurrency code touched; no recordings.

## Escalations / open questions

The large pre-existing dirty working tree (the session's accumulated uncommitted packet
work) disclosed and untouched per the standing rule; the concurrent TS executor's eight
modified files in packages/ likewise untouched. No judgment calls needing a ruling.

## Discovered out-of-scope work

None filed. Findings 2/3 remain held exactly as P015 left them.
