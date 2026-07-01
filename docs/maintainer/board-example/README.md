# Board Example Maintainer Guide

Board is the canonical user-facing teaching example for Vorma and the canonical living API
pressure-test app. Those two roles are not equal: the user-facing teaching role is the
surface area people actually read, and the maintainer pressure-test role must support that
rather than leak internal process into it.

A careful human or agent should be able to read `examples/board` thoroughly and learn how
to build serious Vorma applications. Board source should explain the Vorma APIs it uses,
when those APIs are appropriate, and any subtleties that matter in real applications.
Comments should be user-facing teaching comments, not maintainer notes, census notes,
handoff notes, or private reminders.

Board must cover 100% of public Vorma APIs. Period.

When a public Vorma API is added, removed, renamed, or semantically changed, update Board
in the same work. Active API-specific gaps and work-in-progress coverage lists belong in
tickets under `docs/maintainer/tickets/`, not in this durable guide. If an API has no
existing Board feature, add or invent a Board feature that uses it in a realistic app
flow. The feature may exist primarily to demonstrate an API, but the example code must say
that plainly in user-facing terms and explain when an application would use that API. If
Board usage exposes awkwardness, fix the framework API and keep Board coverage complete.

Do not turn Board into an internal fixture directory. Framework semantic tests belong in
framework-owned suites such as `packages/vorma/core`, adapter tests, `crates/vorma/tests`,
or `tests/framework`. Board may inspire those tests, and a framework integration test may
explicitly choose Board as its fixture if whole-app composition is the behavior under
test, but framework regression coverage should not hide inside `examples/board`.

Board tests, when present under `examples/board`, must be user-facing examples of how to
test a Vorma app. They should test Board behavior through public Vorma testing APIs and
should teach useful application-testing patterns. Do not put tests there merely because
Board happens to exercise a framework primitive.

Do not rely on ad-hoc scripts in `/tmp` for this accounting. If inventory generation or
coverage checking needs code, put that code in checked-in maintainer tooling and document
the command in the relevant maintainer docs or ticket.

## Durable Responsibilities

- Keep `examples/board/README.md` user-facing. Do not put maintainer process, coverage
  ledgers, or internal workstream notes in example READMEs.
- Keep active Board coverage gaps in tickets under `docs/maintainer/tickets/`.
- Keep long-term Board coverage rules here.
- Keep Board comments educational. Every non-obvious Vorma API use should explain why the
  app uses it, when another app would reach for it, and any important edge or lifecycle
  detail.
- Keep framework semantic tests out of Board unless the test is intentionally and
  explicitly using Board as a whole-app fixture from a framework-owned test location.
- Do not treat request-level tests as browser/client tests. Browser, Vite, generated
  client, and dev-loop APIs need tests that exercise those paths directly when semantic
  coverage is missing.
- Mechanical API inventories are inputs only. They do not prove Board coverage by
  themselves.
