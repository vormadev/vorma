# P008 — Ruled API additions: task-error exit conversions + TestApp cookie continuation

Read `docs/maintainer/fable/README.md` (the packet protocol binds you), then
`LEARNINGS.md`, `STATE.md`, repo-root `AGENTS.md`, and `docs/maintainer/REMINDERS.md`.
Inputs: tickets `docs/maintainer/tickets/task-error-exit-conversions/` and
`docs/maintainer/tickets/testapp-cookie-continuation/`, census rows F-2/F-3 in
`docs/maintainer/tickets/board-api-coverage/PRESSURE_TEST_CENSUS.md`.

## Context and authority

Two public-API additions were maintainer-approved 2026-07-01 (public API shape is
maintainer territory; the approval is this packet's grant — nothing beyond the two granted
surfaces may change). Both came from census friction: F-2 (every `Task::run` call site
hand-maps errors and the easy form flattens the source chain the exit types were designed
to carry) and F-3 (reading a `Set-Cookie` back in tests means hand-parsing header strings;
board grew a local `cookie_pair` helper every auth-testing app would re-write).

## Granted surface 1 — `From` conversions for task errors into exits

- `impl From<vorma::tasks::Error<vorma::Error>> for ViewExit` and the same for `HttpExit`.
  Concrete impls only (no blanket generic over `E` — conservative start; widening later is
  a new ruling).
- Behavior: the conversion produces the exit's standard server-error form with the
  framework's safe default client message, and the ENTIRE task error preserved as the
  source chain (nothing flattened). Apps wanting a custom client message keep writing
  explicit `.map_err(... .with_client_msg(...) .with_source(...))` — the From makes the
  safe default frictionless, it does not replace the expressive path.
- The `Cancelled` variant must convert to something deliberate and pinned (a From cannot
  refuse). Expected reality: a task cancelled mid-request means the request is being torn
  down and upstream cancellation handling supersedes whatever the handler returns — verify
  that expectation against the engine's behavior and pin the observable outcome in a test
  rather than leaving it an accident. If verification shows a converted Cancelled could
  actually leak a user-visible 500 on a live request path, stop and escalate with the
  evidence before landing.
- Tests first: unit pins for both impls (message, status class, source-chain intactness —
  assert the chain via `std::error::Error::source` traversal, not string matching) plus
  the Cancelled pin.
- Board call sites: convert `Task::run` error mappings to `?` where the default message is
  the right teaching (most sites). Keep exactly one explicit
  `.map_err(...with_client_msg/with_source...)` site as the taught "custom client message"
  form — the LAYOUT stats site in `views.rs` already has a bespoke message and is the
  natural keeper; its teaching comment should now contrast the two forms.

## Granted surface 2 — test cookie continuation

- Response side: a typed accessor for `Set-Cookie` pairs on the testing response surface
  (align naming with the existing testing types; long, crystal-clear names per AGENTS.md).
- Continuation: an EXPLICIT session object — `TestApp::session()` returning a
  `TestSession` (or clearer name if the surface suggests one) whose requests carry cookies
  accumulated from its prior responses' `Set-Cookie` headers (standard overwrite-by-name
  semantics; expired/cleared cookies honored). `TestApp` itself stays stateless — no
  hidden jar on the app, no cross-session sharing. The session exposes the same
  request-building verbs the app does (delegating, not duplicating logic — DRY).
- Tests first for the new surface (jar accumulation, overwrite, clearing, and the
  login→authorized-request loop).
- Board tests: convert the auth flows to the session where it teaches better; delete the
  hand-rolled `cookie_pair` helper. Board tests must read as exemplary app testing.

## Rider (Fable-ruled at the P005 review, census F-21)

`DocumentAttributes::known_safe_attribute` and `::boolean_attribute` have no honest board
call site; the ruling is that they are discharged by `crates/vorma/tests/public_api.rs`.
Extend that file's existing document-helpers usability test to exercise both (assert the
rendered attribute forms). Small, mechanical, same test-suite this packet already touches.

## Hard constraints

- NOTHING public changes beyond the two granted surfaces. No other crate touched
  (`vorma-tasks` is not modified — the From impls live in `vorma`, on vorma's types).
- No new dependencies (cookie parsing at test-grade is string handling; if you believe a
  crate is genuinely warranted, escalate first).
- Tests never cheat; contract strings imported, never re-defined.
- No git actions; no network installs; scoped fmt writes only; unexpectedly dirty unowned
  files are escalations, never cleanup.
- Both tickets are consumed by this packet: on completion, delete both ticket directories
  and update census rows F-2/F-3 with the resolution (per `tickets/README.md`).

## Definition of done

- Both surfaces landed with their pins; board call sites/tests converted as specified;
  `cookie_pair` gone; tickets deleted; census updated.
- Gates green: workspace tests + doctests, clippy `-D warnings`, fmt check,
  `make loom-tasks` untouched-green, board tsgo + vitest (board client is untouched —
  confirm no TS surface changed).
- REPORT.md per template (or full content returned in-message if the file-write guardrail
  fires, for the orchestrator to place), including the exact final public signatures for
  the review record.
