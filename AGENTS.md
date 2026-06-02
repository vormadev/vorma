## Maintainer Docs

Maintainer docs live at `docs/maintainer/*`.

Always read `docs/maintainer/REMINDERS.md` at least once after each context compaction.

Everything you work on should be written such that it would pass the standards set forth
in `docs/maintainer/skills/thermo-nuclear-system-review/SKILL.md`.

## Maintainer Tooling Should Be As Simple and Minimal As Possible

Do not add unnecessary garbage like "help" args to maintainer-facing tooling. All
maintainer-facing tooling (scripts, xtasks, makefiles, etc.) should be as simple and
minimal as possible. There should be no extra or speculative surface area, period.

## Git Usage

Never ever perform or attempt to perform git actions other than purely read-only
activities. You are not allowed to stage, unstage, commit, push, revert, stash, or take
any other potentially destructive git actions. Never ever hand-edit a gitignored file.
Running normal build, test, format, or tooling commands that write ignored generated
outputs or caches is allowed when those commands are part of the requested workflow.

## TypeScript Style

- Always use curlies in control flow branches.
- Always use curlies and explicit return statement for (1) object literals and (2)
  anything that doesn't fit on one line.
- All internal symbols shall be `snake_case`, and all public-facing symbols shall be
  either: (1) `SCREAMING_CASE`, (2) `camelCase`, or (3) `PascalCase`, as is appropriate or
  idiomatic contextually. TypeScript type symbols should always be `PascalCase` regardless
  of exposure, but any individual fields on the type should be `snake_case` if the type is
  non-public (or the field is otherwise not intended for public consumption, in which case
  it should also be prefixed with `__`).

## No Single-Use Helpers

Single-use helpers are strictly prohibited unless they dramatically and objectively
simplify the code.

## Symbol Casing

For repo-wide consistency, when naming a `camelCase` or `PascalCase` variable containing
an acronym-ish component, always use the style where the acronym-ish component is NOT all
caps, like `getId` or `formatOklch` (rather than `getID` or `formatOKLCH`), even in
TypeScript or other non-Rust files. This rule makes casing more predictable and improves
mechanical translatation to/from `snake_case`.

## No Multi-Use Magic Strings

Never duplicate contract strings. Define constants for values that are reused or that form
part of an external/internal contract: environment keys, route paths, storage keys,
generated field names, file names, protocol markers, event names, CSS/test selectors, and
similar values.

This absolutely includes tests. Tests should import the same constants as the
implementation whenever they are asserting contract values. DO NOT UNDER ANY CIRCUMSTANCES
re-define contract constants in test files.

Single-use string literals are fine when they are local, self-explanatory, and not part of
a contract. Do not extract one-off labels, prose, or obvious local values into constants
just to avoid a literal.

## Be Mindful of Sandbox Restrictions

Do not directly or indirectly (via other commands) run pnpm installations without
escalation, as they will fail from sandbox restrictions. Escalate immediately instead of
even trying the non-escalated call. If you accidentally do this and notice it failing
(evidenced by `ENOTFOUND` or similar), then kill it immediately and escalate; do not wait
for natural failure, which is a pure waste of everyone's time.

## Comment Style

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

## Consistent Philosophy / Package Composability

All packages shall use a consistent design philosophy and work well together.

## Public APIs

All public APIs shall be intentional, non-leaky, ergonomic, footgun-free, and
user-friendly.

Prefer namespaced, intention-revealing Rust modules over flat root exports. Do not expose
lower-level implementation primitives merely because tests or internals use them. If a
lower-level primitive is easy to misuse, keep it private and expose a higher-level
composition.

## No Conversational or Changelog Comments

It is prohibited to add conversational or changelog comments to source code files.

## Report All Issues. No Severity Labels. Never Triage.

When doing reviews or audits, the goal is to find and report **all** issues, not just a
subset and not just "critical" issues.

It is prohibited to triage, prioritize, rank, or categorize issues by severity or
priority.

It is also prohibited to use severity or priority language in review outputs (for example:
"critical", "major", "minor", "P0/P1/P2", "high/medium/low", or similar labels).

Findings shall be presented as a plain, exhaustive list of issues. If no issues are found,
state that explicitly.

## Tests Shall Never "Cheat"

One example of cheating is hardcoding or computing values to match the current
implementation in order to make a test pass, rather than testing what is in fact "correct"
(either spec-defined or logically or semantically obvious). Another example of cheating is
to add unjustified tolerances to tests to make them pass. Any type of retro-fitting,
mirroring, or scope hacking, or anything spiritually similar, is prohibited in test
suites.

A failing test that exposes a real bug or issue is ALWAYS a HUGE BLESSING, and we should
be CELEBRATING when that happens.

## Tests Must Never "Skip" When A Resource Is Missing

It is strictly prohibited to skip tests, ever. If something a test needs to run is missing
(e.g., a live Docker container or whatever), then the test must instantly and loudly fail.
Zero exceptions. We cannot risk ever having only a subset of tests run; it would
dangerously lead to false confidence from a suite that didn't even run fully. Printing a
warning is NOT sufficient. The tests must fail. Similarly, the repo's full gate
(`make gate`) must include all tests in the repo (otherwise it's not a proper full gate).

## Long, Clear Function/Method/Variable Names Are Good

When a short function/method/variable name is crystal clear, then that's fine. But if it's
anything less than obvious and abundantly clear to users, then make the name longer and
more descriptive until it is. `BuyInDollarsNoWait` is an infinitely better name than `Buy`
with a documentation comment explaining it's denominated in dollars and doesn't wait. The
classic way of stating this is that "code should be self-documenting where possible".

In every single case, we need the clearest possible name. There should be zero room for
confusion or potentially forgetting semantics. It should always be immediately and clearly
obvious what a thing does and how it behaves from the name itself.

Anything dangerous, especially, should have an extremely explicit,
impossible-to-misconstrue name.

## Stay DRY

Don't Repeat Yourself. Following DRY clarifies thinking, keeps building blocks
high-quality, and keeps context windows smaller. Within reason and using common sense,
never repeat complex logic that should be abstracted into a shared helper.

## Performance Is Always A Concern, And It's Non-Optional

So long as it doesn't undermine security or correctness, always write code such that it
results in the highest performance "bang for the buck", even if it's more difficult to
write. Wastefulness is a form of incorrectness.

## Correct, Ideal Code Is The Goal, Not Easy Migrations Or Backwards Compatibility

When analyzing code, do not get caught up in "the easiest way to update it" or "the way to
maintain backwards compatibility". The goal is always "what is the maximally correct and
ideal version of this code", regardless of potential breakages or level of refactor
effort. It's never OK to take the lazy approach to solving a problem; always strive for
the truly correct and ideal approach.

## Pre-Existing Code Comments Are Not Infallible

Do not automatically take code comments at face value. If you are suspicious of the
reasoning, logic, or accuracy of any comment, do not blindly trust it. Do your own
analysis.

## We Should Make No Assumptions On Behalf Of Applications

Anything that could conflict (filenames, table names, key prefixes, etc.) with any
independent choices an application could make shall be over-ridable by user configuration
settings. Further, the defaults should not be overly presumptuous. For example, prefer
something like `__vorma_` instead of `vorma` for something that might conflict with
userland.

## Never Rely On Documentation To Smooth Over A Footgun, Bad API, or Bad Name

Never ever ever suggest to "document this clearly" when something is confusing, a footgun,
ambiguous, poorly named, or poorly designed. The answer is not documentation, it's better
names, better design, better internal robustness.

## Do Not Accumulate Cruft

Adding a bunch of helper methods that have no point other than setting a default for you
is bad. This isn't to say we shouldn't have convenience helpers. It's just to say that
each convenience helper should ACTUALLY add true convenience over other options, not just
more options. For example, we don't need a `reset()` function that sets a value to `0`
when you can easily just do `set(0)` (because calling `set(0)` is not actually any harder
than calling `reset()`).

## Untracked Files and `*.human.*` Files

- Never edit or delete files matching `*.human.*` or `*.local.*` unless the user
  explicitly asks.
- For other untracked files:
    - If the file appears user-authored or pre-existing, do not edit/delete it unless the
      user explicitly asks.
    - If the file was created by you (the agent) as part of the current task, you (the
      agent) may edit/delete it as needed to complete the task.

## Don't Add Non-Conflicted Makefile Targets to .PHONY

Unless there's an actual conflict with a file or directory on disk, it's just cruft.

## Path Hygiene

- Never, ever commit machine-specific absolute paths (for example, `/Users/...`) into
  repository files.
- Use repository-root-relative paths in docs and instructions.
