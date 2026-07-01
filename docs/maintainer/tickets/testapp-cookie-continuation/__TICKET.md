# TestApp Cookie Continuation

Status: open

Improve `vorma::testing::TestApp` ergonomics for auth-flow tests that carry cookies from
one response into later requests.

Current friction:

- `TestRequest::cookie` makes writing request cookies straightforward.
- Reading a `Set-Cookie` response back into a later request still tends to require local
  test helpers that parse raw header strings by hand.
- Board auth tests exposed this with a local `cookie_pair` helper.

Related context:

- `../board-api-coverage/PRESSURE_TEST_CENSUS.md`
- `../board-api-coverage/API_DESIGN.md`

Design constraints:

- This is test ergonomics, not a runtime feature.
- Keep the API direct and boring.
- Do not make the harness stateful in a surprising way. If cookies are carried
  automatically, that must be unmistakable from the type or method name.
- Do not add duplicate cookie parsing logic in tests if implementation code already owns
  the correct behavior.

Possible shape to evaluate:

- A response method that exposes parsed cookies suitable for feeding into a later
  `TestRequest`.
- Or a small explicit jar/continuation object controlled by the test author.

Done means:

- Board auth-flow tests no longer parse `Set-Cookie` by hand.
- The testing API makes continuation explicit at the call site.
- Tests cover name/domain/path behavior if the helper claims to model cookie-jar semantics
  rather than just copying simple cookie pairs.
