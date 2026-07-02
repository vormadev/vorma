# P008 Report

Provenance note: the executor (Sonnet 5 subagent, 2026-07-01/02) was blocked from writing
this file by the recurring harness report-file guardrail and returned the full content in
its final message; Fable placed it (HTML transport escaping undone).

## What changed / exact final public signatures

Granted surface 1 (`crates/vorma/src/exit.rs`):

```rust
impl From<vorma_tasks::Error<crate::Error>> for ViewExit { ... }
impl From<vorma_tasks::Error<crate::Error>> for HttpExit { ... }
```

Concrete impls only, as instructed — no blanket `impl<E>`. `Failed(source)` becomes the
exit's standard server-error form with the application error attached as the source chain
(chain intactness pinned via `std::error::Error::source` traversal); the payload-free
variants use their `Display` text as the server record.

Granted surface 2 (`crates/vorma/src/testing.rs`):

```rust
impl TestApp {
    pub fn session(&self) -> TestSession<'_>;
}

pub trait TestResponseCookies {
    fn set_cookie_headers(&self) -> Vec<Cookie<'static>>; // Cookie = vorma::HttpCookie
}
impl TestResponseCookies for Response<Bytes> { ... }

pub struct TestSession<'a> { ... }
impl<'a> TestSession<'a> {
    pub async fn get(&mut self, path_and_query: &str) -> Response<Bytes>;
    pub async fn get_view_payload(&mut self, path_and_query: &str) -> Response<Bytes>;
    pub async fn request_json<B: serde::Serialize>(&mut self, method: Method, path_and_query: &str, body: &B) -> Response<Bytes>;
    pub fn request(&mut self, method: Method, path_and_query: &str) -> TestSessionRequest<'a, '_>;
}

pub struct TestSessionRequest<'a, 's> { ... }
impl<'a> TestSessionRequest<'a, '_> {
    pub fn header(self, name: impl Into<String>, value: impl Into<String>) -> Self;
    pub fn cookie(self, name: impl AsRef<str>, value: impl AsRef<str>) -> Self;
    pub fn body(self, content_type: impl Into<String>, body: impl Into<Vec<u8>>) -> Self;
    pub async fn send(self) -> Response<Bytes>;
}
```

`TestSession`'s four verbs mirror `TestApp`'s own four exactly; `TestSessionRequest`
mirrors `TestRequest`'s builder verbs plus `send`. Every session verb delegates to
`TestApp::request`/`TestRequest` — no request-construction logic duplicated.

## Pinned Cancelled-conversion behavior

Behavior: `Cancelled` converts to the exit's plain default form — `Display` text ("task
cancelled") as server record, no source, framework's automatic safe-default client
message. Treated identically to the other payload-free runtime variants (`Cycle`,
`MissingOverride`, `TypeMismatch`) — no special case.

Why it is safe (traced, not assumed): `Error::Cancelled` is only produced when a resolving
`ExecCtx`'s cancellation token is already set. In `crates/vorma/src/execution_engine.rs`,
`cancel_execution_contexts` — the only place this engine ever cancels an invocation's
context — runs strictly AFTER `execute_invocation_phase(...).await` has already returned,
meaning every same-phase sibling has already fully run (nothing aborted). A losing
same-phase sibling's context is cancelled only after its output is already computed and
already positionally discarded by `commit_invocation_output`; a next-phase invocation's
context is cancelled before that phase is ever reached, so it never runs.
`ParallelBatch::run`'s own internal cross-sibling cancellation was also traced
(`remember_first_error` in `vorma-tasks`): it always prefers a real error over
`Cancelled`, so a batch only returns `Cancelled` to its caller under the same
ambient-already-cancelled condition. Conclusion: no reachable path exists where a
still-mattering handler observes its own context cancelled — verification showed no leak,
so no escalation was needed per the packet's own instruction. The reachability argument is
recorded in the in-code doctrine comment above the impls.

Tests: `exit.rs::tests::cancelled_task_errors_convert_to_the_plain_default_exit_form`
(pins the conversion output for both exit types);
`execution_engine.rs::tests::cancelled_conversion_loses_the_position_race_to_an_earlier_real_error`
and `..._wins_the_position_race_when_it_runs_first` (prove the discard mechanism is
positional, not identity-based, by running a real `Cancelled`-converting handler via the
actual public `From` impl against a real failing sibling in both orderings).

## Board changes

12 `Task::run`/`ParallelBatch::run` call sites converted from `.map_err(...)?` to bare `?`
(8 in `views.rs`, 4 in `resources.rs`). `LAYOUT`'s stats site kept its explicit
`.with_source(...)` form as the taught contrast, comment updated. Two sites that map a
`vorma::Error` (not a task error — `current_user`/`require_user`) deliberately left
untouched: not `Task::run` sites, already served by the pre-existing `From<crate::Error>`
route, out of the packet's literal grant.

`cookie_pair` deleted from `examples/board/tests/app.rs`. `login`/`submit_story` helpers
and the tests converted to `TestSession` where it teaches better. One test
(`logout_clears_the_session_cookie_and_the_session_row`) deliberately kept the stateless
`TestApp.cookie(name, value)` form because it exists to replay a STALE token after logout
— a session's jar would have honestly forgotten it, silently weakening the test;
`set_cookie_headers()` still replaced the string parsing there.

A real bug was caught and fixed during self-review, before reporting: the first
`TestSessionRequest` draft would have appended the jar as one `Cookie:` header and any
explicit `.cookie(...)` as a second, but `HeaderMap::get` (what real app code reads
through) only returns the first of a repeated header name — silently hiding one or the
other. Fixed by merging jar + explicit additions into exactly one header at `.send()`
time; two tests pin this specifically.

`TestApp::get_view_payload` and `TestSession::get_view_payload` share one private
`view_payload_uri` helper (no duplicated query-marker logic).

F-21 rider: `public_api.rs` extended with a test exercising
`known_safe_attribute`/`boolean_attribute` through the real call site, asserting rendered
HTML forms — no new `vorma` public surface added.

Tickets `task-error-exit-conversions` and `testapp-cookie-continuation` deleted; `NEXT.md`
updated; census rows F-2, F-3, F-21 updated with resolution notes.

## Gate results

- Workspace tests: 567 passed, 0 failed (551 baseline + 16 new). Run twice, clean both
  times.
- Doctests: 2 passed, 0 failed.
- Clippy `-D warnings`: clean, run twice.
- fmt: scoped-clean on every touched file (scoped writes only, because of pre-existing
  unrelated dirty state in the tree).
- `make loom-tasks`: 7/7 pass; `vorma-tasks` confirmed zero-diff against HEAD before
  running (untouched-green).
- Board TS gate: 8 tsgo projects clean; vitest 851/851 across 42 files; `vorma.gen.ts`
  confirmed byte-identical to HEAD (error-mapping-only Rust changes produced zero
  wire-contract drift).

## Escalations

None requiring a decision — the one thing shaped like an escalation (Cancelled possibly
leaking a user-visible 500) was traced to provably unreachable, per the packet's own
instruction to verify before landing.

Observation only, no action taken: the working tree carried substantial pre-existing
uncommitted work not authored by this executor (fable docs, P005-P009 packet dirs, the
board README doctrine restatement dated 2026-07-02, P005's board TS changes). None of it
overlaps this packet; none was touched, reverted, or staged.

## Discovered out-of-scope work

None. No new tickets.
