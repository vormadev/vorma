## Consistent Philosophy / Package Composability

All packages shall use a consistent design philosophy and work well together.

## Public APIs

All public APIs shall be intentional, non-leaky, ergonomic, footgun-free, and
user-friendly.

## No Conversational or Changelog Comments

It is prohibited to add conversational or changelog comments to source code
files.

## Function Formatting

Functions with many parameters shall format such parameters vertically, like so:

```go
func someFuncWithManyArgsFormattedVertically(
	arg1 any,
	arg2 any,
	arg3 any,
	arg4 any,
	arg5 any,
	arg6 any,
)
```

## All Issues Are MAJOR/CRITICAL. Never triage.

There is no good reason to triage, prioritize, or categorize the severity of
issues, or to limit any review to only finding "critical" issues. All issues, no
matter how "small" should always be treated as MAJOR/CRITICAL. The goal is
perfection, PERIOD. There is no such thing as a minor issue.

## Tests Shall Never "Cheat"

One example of cheating is hardcoding or computing values to match the current
implementation in order to make a test pass, rather than testing what is in fact
"correct" (either spec-defined or logically or semantically obvious). Another
example of cheating is to add unjustified tolerances to tests to make them pass.
Any type of retro-fitting, mirroring, or scope hacking, or anything spiritually
similar, is prohibited in test suites.

A failing test that exposes a real bug or issue is ALWAYS a HUGE BLESSING, and
we should be CELEBRATING when that happens.

## Long, Clear Function/Method/Variable Names Are Good

When a short function/method/variable name is crystal clear, then that's fine.
But if it's anything less than obvious and abundantly clear to users, then make
the name longer and more descriptive until it is. `BuyInDollarsNoWait` is an
infinitely better name than `Buy` with a documentation comment explaining it's
denominated in dollars and doesn't wait. The classic way of stating this is that
"code should be self-documenting where possible".

In every single case, we need the clearest possible name. There should be zero
room for confusion or potentially forgetting semantics. It should always be
immediately and clearly obvious what a thing does and how it behaves from the
name itself.

Anything dangerous, especially, should have an extremely explicit,
impossible-to-misconstrue name.

## Stay DRY

Don't Repeat Yourself. Following DRY clarifies thinking, keeps building blocks
high-quality, and keeps context windows smaller. Within reason and using common
sense, never repeat complex logic that should be abstracted into a shared
helper.

## Correct, Ideal Code Is The Goal, Not Easy Migrations Or Backwards Compatibility

When analyzing code, do not get caught up in "the easiest way to update it" or
"the way to maintain backwards compatibility". The goal is always "what is the
maximally correct and ideal version of this code", regardless of potential
breakages or level of refactor effort. It's never OK to take the lazy approach
to solving a problem; always strive for the truly correct and ideal approach.

## Pre-Existing Code Comments Are Not Infallible

Do not automatically take code comments at face value. If you are suspicious of
the reasoning, logic, or accuracy of any comment, do not blindly trust it. Do
your own analysis.

## We Should Make No Assumptions On Behalf Of Applications

Anything that could conflict (filenames, symbols, namespaces, etc.) with any
independent choices an application could make shall be over-ridable by user
configuration settings. Further, the defaults should not be overly presumptuous.
In other words, use something like `__vorma_internal_api` instead of `api`.

## Never Rely On Documentation To Smooth Over A Footgun, Bad API, or Bad Name

Never ever ever suggest to "document this clearly" when something is confusing,
a footgun, ambiguous, poorly named, or poorly designed. The answer is not
documentation, it's better names, better design, better internal robustness.

## Do Not Accumulate Cruft

Adding a bunch of helper methods that have no point other than setting a default
for you is bad. This isn't to say we shouldn't have convenience helpers. It's
just to say that each convenience helper should ACTUALLY add true convenience
over other options, not just more options. For example, we don't need a
`Reset()` function that sets a value to `0` when you can easily just do `Set(0)`
(calling `Set(0)` is not actually any harder than calling `Reset()`).
