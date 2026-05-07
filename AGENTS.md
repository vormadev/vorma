## Go rules

Prefer methods when behavior belongs to a long-lived domain type with state,
identity, invariants, or an existing method surface.

Do not create free-floating helper functions that merely operate on such a type
from the outside. For example, prefer `router.register(...)` over
`register(router, ...)` when `router` is the object whose behavior is being
extended.

This rule does not apply to short-lived parameter/config structs, conversion
records, test cases, option bags, or small value objects whose purpose is just
to carry inputs into a function.

---

## TypeScript rules

Always use curlies in control flow branches.

---

Always use curlies and explicit return statement for (1) object literals and (2)
anything that doesn't fit on one line.

---

## Universal rules (applicable to both Go and TypeScript)

Never ever perform or attempt to perform git actions other than purely read-only
activities. You are not allowed to stage, unstage, commit, push, revert, stash,
or take any other potentially destructive git actions. Never ever hand-edit a
gitignored file. Running normal build, test, format, or tooling commands that
write ignored generated outputs or caches is allowed when those commands are
part of the requested workflow.

---

Before writing tests or running commands, make sure to read both
`TEST_README.md` and `Makefile` in the repo root.

---

Single-use helpers are strictly prohibited unless they dramatically and
objectively simplify the code.

---

All internal symbols shall be `snake_case`, and all public-facing symbols shall
be either: (1) `SCREAMING_CASE`, (2) `camelCase`, or (3) `PascalCase`, as is
appropriate or idiomatic contextually. TypeScript types, however, should still
always be `PascalCase`, regardless of public exposure.

---

For repo-wide consistency, when naming a `camelCase` or `PascalCase` variable
containing an acronym-ish component, always use Go-style where the acronym-ish
component is all caps, like `getID` or `formatOKLCH` (rather than `getId` or
`formatOklch`), even in TypeScript or other non-Go files.

---

Never duplicate contract strings. Define constants for values that are reused or
that form part of an external/internal contract: environment keys, route paths,
storage keys, generated field names, file names, protocol markers, event names,
CSS/test selectors, and similar values.

This absolutely includes tests. Tests should import the same constants as the
implementation whenever they are asserting contract values. DO NOT UNDER ANY
CIRCUMSTANCES re-define contract constants in test files.

Single-use string literals are fine when they are local, self-explanatory, and
not part of a contract. Do not extract one-off labels, prose, or obvious local
values into constants just to avoid a literal.

---

Do not make tool calls that need to make network calls (e.g., pnpm installations
or similar), either directly or indirectly. They will fail from sandbox
restrictions. Instead, escalate immediately instead of even trying the
non-escalated call. If you accidentally do this and notice it failing (e.g.,
`ENOTFOUND` or similar), then kill it immediately and escalate; do not wait for
natural failure, which is a pure waste of everyone's time.

---

Comments shall be in one of the following formats only:

```
/*
Multi-line comment
*/

/////////////////////////////////////////////////////////////////////
/////// Topic
/////////////////////////////////////////////////////////////////////

/////// Sub-Topic

/// Sub-Sub-Topic

// Normal old single-line comment
```
