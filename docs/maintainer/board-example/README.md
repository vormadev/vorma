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

What owes Board coverage is measured by one test (maintainer ruling, 2026-07-01): **is
this a framework-author primitive or an app-useful primitive** — never whether it is
"advanced". `vorma-tasks` is a sovereign crate that Vorma builds on (not the other way
around), so its runtime-lifecycle surface — constructing `Tasks`, opening execution
contexts, cancel tokens, cooperative cancellation, observers — is app-useful: applications
obviously run their own background work, and Board must teach that through real app
features. The only exemptions are genuinely framework-author surfaces, currently:
`TaskOverrides` (its coverage home is the framework test suite, prior maintainer ruling)
and the `Clock` family (determinism-injection tooling). Exempt items are discharged by the
sovereign-crate suites and `crates/vorma/tests/public_api.rs`.

**"Contrived" is never grounds for exemption** (maintainer ruling, restated with force
2026-07-02 after repeated misapplication): Board is a teaching tool and the ENTIRE app is
contrived by design — an invented community site whose reason to exist is exercising
Vorma. A feature invented purely to demonstrate an API is the sanctioned mechanism, not a
defect; the quality bar is only that the resulting code teaches honestly (says plainly
what it demonstrates and when an application reaches for it). If an app-facing API seems
to have no sensible use even in an invented feature, that is evidence of an API-design
problem — escalate it (fix the framework, per the rule above), never record a coverage
waiver. Agents triaging coverage must not use "no honest home," "no real product need," or
any equivalent as a reason to leave app-facing surface uncovered.

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
