# Handler Outcome & Response Protocol Design

Status: SUPERSEDED IN PART — being folded into the full API design pass (user ruling
2026-06-11): the error/response protocol is one section of a whole-surface design
document, so cross-cutting conventions get decided once. Rulings already settled by the
user that the API design pass MUST honor:

- Views and resources get DIFFERENT error types (different semantics → different types;
  unrepresentability beats one-type-to-learn, per AGENTS.md philosophy).
- Nothing reaches the client unless explicitly marked client-facing.
- Views have NO status surface (rendering protocol, not HTTP documents); a 200 with a
  failing segment is CORRECT (parents served, error message served).
- "msg" naming for the server-side record is wrong (implies transmission); the server
  record needs an unambiguous name.
- No sugar: if we are tempted to add sugar, the normal API is wrong.
- Wire details must be coherent and non-misleading; otherwise they are under-the-hood.
- Redirects: handle-based leaning, BUT the fabricated-return wart (handler must still
  produce a typed Ok after redirecting) was acknowledged; the channel-riding
  `return ctx.redirect(...)` shape and exit-type naming are open for the design pass.

INTERIM STATE LANDED (green, all suites): the E generic is fully removed (View<S>,
AppConfig<S>, all ctx types single-param; ViewErrorClientMsg / ViewError / the BoxError
downcast deleted), handlers return `Result<O, vorma::Error>` directly (no `.into()`), and
the single interim `vorma::Error` carries status/client_msg consulted engine-side. This is
the mechanical foundation; the per-protocol type split happens with the design pass.

Driving ruling (user, 2026-06-11): views are NESTED segments in a framework-owned
rendering protocol, not HTTP documents — error status codes are meaningless there; nothing
reaches the client unless explicitly marked client-facing; the goal is the simplest API
that is impossible to misuse, respecting the SPIRIT of the Go version while beating its
APIs.

## Receipts: what Go actually did (origin/refactor-2026-13)

- **Views** (`vormarun/handler.go`): loaders returned `LoaderError { ClientMsg, Err }`. On
  loader error: page still rendered, HTTP 200, payload carried `OutermostServerErr`
  (ClientMsg or the generic) and segments truncated after the failing index. The proxy
  escape hatch existed (a loader COULD `SetStatus(4xx)`), and then
  `ApplyToResponseWriter` + `IsError → return` ABANDONED the page and wrote `http.Error`
  text — same wart we have; the new design deletes the hatch rather than porting it.
- **API routes** (`kit/mux/mux.go` task handler): returned error → log + blind
  `res.InternalServerError()` — NO client message channel on returned errors at all. The
  official rejection idiom was proxy `SetStatus(status, text)` + `return zero_value, nil`;
  apply wrote `http.Error(text)` (text/plain) and the data was discarded. Success → bare
  `res.JSON(data)`.
- **Middlewares** (`kit/mux` pipeline): ran as parallel tasks BEFORE the route; merged
  proxies; `IsError || IsRedirect` → respond and stop (the route never ran). Task failure
  → blind 500.
- **The Go-era TS client** read `res.statusText` on `!res.ok` — so even in Go, the proxy's
  carefully-set text (which landed in the BODY) never reached `result.error`. The two
  halves never connected there either; current Rust+TS faithfully ports the disconnect.

Spirit extracted: ONE vocabulary for rejection (status + optional client text), client
text only ever explicit, views render through errors, resources are HTTP. Mechanics to
discard: proxy-status-as-control-flow, discarded `Ok` data, text/plain rejection bodies,
statusText reading, the view escape hatch.

## The design

### Vocabulary (already partially landed)

One public error type for everything:

```rust
vorma::Error::msg("server-side reason")        // required: server record
	.with_status(StatusCode)                    // resource/middleware response status
	.with_client_msg("client-visible text")     // ONLY path to client-visible text
	.with_source(any_error)                     // diagnostics chain
```

Invariant: the client sees `client_msg` or a framework generic. The server message and
source NEVER travel. There is no other client-text channel anywhere (status_text dies;
set_client_error dies; ViewError and the ViewErrorClientMsg trait die; the E generic dies
— all previously ratified).

### View protocol (framework-owned rendering; HTTP status is not a concept)

| Handler outcome          | Result                                                                                                                                                                              |
| ------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `Ok(data)`               | Segment commits with data.                                                                                                                                                          |
| `Err(error)`             | Segment error: page still renders, HTTP 200, wire payload carries the explicit `client_msg` (else generic) at the failing segment; deeper segments truncate. Server message → logs. |
| redirect (see ruling R3) | Terminal redirect, page abandoned (legitimately — there is nothing to render).                                                                                                      |

- The view ctx has NO status surface: no `set_status`, nothing that can make a view
  response "an error" at the HTTP layer. The page-abandoning terminal path for views
  shrinks to: redirects, and framework faults.
- `with_status` on an error returned from a VIEW: see ruling R2.
- Head, headers, and cookies remain available (decoration, not outcome).

### Resource protocol (actual HTTP)

| Handler outcome | Result                                                                                                                            |
| --------------- | --------------------------------------------------------------------------------------------------------------------------------- |
| `Ok(data)`      | Status 200 (or the handler's explicit SUCCESS status, e.g. 201 — see R5), body = bare JSON of `data` (Go parity).                 |
| `Err(error)`    | Status = `error.status` (default 500), body = JSON error envelope (see R4), content-type application/json. Server message → logs. |

- Returning `Err` is the ONLY way to produce an error response. The
  `Ok(fabricated_data) + error status` idiom becomes unwritable.
- The TS client parses the envelope on `!res.ok` and sets `result.error` from it — the
  server and client halves of the channel finally meet. `res.statusText` reading dies.

### Middleware protocol (the one place both kinds meet)

`Ok(())` → continue. `Err(error)` → short-circuit the whole request (route handlers never
run), honoring `status` (this IS an HTTP boundary: 401/403/429 are real here):

- Wire/JSON requests (resource calls, view JSON navigations): the same JSON error
  envelope.
- Document requests (browser GET of a page): a framework-owned minimal error response with
  the status and the explicit client text (else generic) — no app payload exists to
  render. (Go wrote text/plain here; we keep that, swappable later for a minimal HTML
  shell without protocol change.)
- Middleware redirects (auth → login) remain first-class and terminal.

### Framework faults (the only true 500s on the view path)

Input decode failures (400), manifest/boot errors (500), panics (500): framework-owned,
generic client text always, envelope on wire requests, plain on document requests.

### What this deletes

- `set_client_error` / `set_status_with_text` / `status_text` field and the text/plain
  rejection bodies built from it.
- `is_terminal()`'s "any status ≥ 400" clause — terminal becomes
  redirect-or-framework-fault only. Statuses stop being control flow.
- The view ctx's entire status surface.
- `res.statusText` consumption in the TS client.
- The ECHO-fixture idiom (rewritten as
  `Err(Error::msg(...) .with_status(CONFLICT).with_client_msg(...))`).

### Misuse-resistance audit (the "impossible to misuse" check)

- Send internals to the client by accident → impossible (one explicit channel; defaults
  generic).
- Error response with fabricated success data → unwritable (no status setters; errors only
  via `Err`).
- View pretending to be an HTTP error document → inexpressible (no status surface on
  views).
- Two ways to express a rejection → no (the handle lost its error vocabulary; the returned
  error IS the rejection).
- Resource success with a non-2xx status → prevented by R5's assert.

## Open rulings

- **R1 — Ratify the protocol matrix above** (views / resources / middlewares / framework
  faults).
- **R2 — `with_status` on a view-returned error**: (a) ignored + loud dev-mode log
  [RECOMMENDED: keeps one Error type; views simply don't consult status; dev log catches
  confusion], (b) ignored silently, (c) separate per-kind error types (maximum strictness,
  two types to learn).
- **R3 — Redirects stay handle-based** (`ctx.response().redirect(...)`, terminal,
  available to views/resources/middlewares) rather than becoming a returned-outcome enum
  [RECOMMENDED: Go spirit; outcome enums tax every happy path with ceremony;
  redirect-after-data reads naturally unlike error-after-data].
- **R4 — Resource error envelope shape**: minimal `{ "error": string }` [RECOMMENDED],
  extensible later (e.g. optional `code`) without breaking parsers; success stays bare
  JSON (Go parity).
- **R5 — `set_status` survives ONLY on the resource ctx, asserted `< 400`** (success
  statuses like 201 are a real need; error statuses ride errors) [RECOMMENDED]; removed
  from the view ctx entirely.
