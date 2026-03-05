## Never Ever Change Commitable Files To Work Around Agent Sandbox Issues

Do not even attempt to run tests or any other commands in your sandbox
environment. Always run things in MY environment. You should never ever ever
make a change to commitable code or other files just to satisfy a sandbox
restriction; that's completely inappropriate and wrong.

Otherwise you hit all kinds of crazy annoying sandbox errors. Don't even try.
Just always ask for elevated permissions. I shouldn't have to change my code to
work around your sandbox issues.

For build/test/dev commands, this is mandatory and non-negotiable:

- Always execute through interactive-login shell: `/bin/zsh -lic '<command>'`
- Never execute build/test/dev commands with `/bin/zsh -lc`
- If `node -v` is not `v24.x` in that shell, stop immediately and do not run the
  command
- If any command output shows Node 20 from `/usr/local/bin/node`, treat that as
  wrong environment usage and stop immediately

## Do Not Use Machine-Absolute Paths Ever Without Explicit Human Permission

Almost nothing in the repo should use machine-absolute paths other than in the
rarest of circumstances. And anywhere that does MUST be labeled with
`MachineAbsolute` or similar in the variable/method name.

## Parse, Don't Validate

Parse, don't validate. You know what this means. Follow it strictly and
religiously.

## Never Generate Non-Temp Test Artifacts

Never generate test artifacts in the repo. Use os temp directories or the repo's
own `__tmp` dir if you must.

## Always, Always, Always Do The Hardest Task First

When you have a checklist of tasks to do, always always always do the #1 hardest
task first, then the second hardest, and so on. No exceptions. If you do the
easy low-hanging-fruit first, you only further embed/ossify whatever structures
are making the hard items hard in the first place. Never ever ever violate this
rule. It's the most important rule.

## Whenever An E2E Test Unearths A Bug, Create a Non-E2E Regression Test That Covers It

Our E2E tests are intended to be a last resort, and they take a long time to
run. For that reason, every time you come across a failing E2E test, make sure
to (if possible) recreate a non-E2E regression test version covering the issue
for faster feedback and regression protection.

## Wave Must Be 100% Independent Of Vorma

Wave is a lower-layer framework and must remain fully Vorma-agnostic.

- Do not add Vorma-specific symbols, defaults, filenames, endpoint names,
  template placeholders, or behavior to `wave/**`.
- Vorma must build on top of Wave through configuration/adapters; Wave must not
  encode Vorma conventions.
- Repository module import paths that happen to include `vorma` are not a
  semantic Wave->Vorma dependency by themselves.

## Keep Runtime Dependency Surfaces Lean (Wave + Vorma)

Wave and Vorma runtime package boundaries must stay clean so production binaries
do not accidentally pull in build/dev-time dependency weight.

- Treat `wave` and `vorma` runtime imports as strict contracts. Do not pull
  build/dev-only helpers into runtime-facing packages.
- Do not introduce build/dev-heavy dependencies (for example file-watching,
  lock/glob orchestration, or build toolchain deps) into runtime-facing packages
  in `wave/**`, `vorma.go`, or `internal/vormaruntime/**`.
- Keep build/dev orchestration in `wave/wavebuild/**`, `wave/wavedev/**`, and
  `vormabuild/**`.
- If shared behavior is needed, split it into runtime-safe and build/dev-only
  packages instead of putting everything into one base package.
- After dependency-boundary refactors, verify runtime deps explicitly (for
  example with `go list -deps github.com/vormadev/vorma/wave` and
  `go list -deps github.com/vormadev/vorma`).

## Git Command Policy

The agent may use Git only for read-only inspection.

Allowed Git commands (and only these):

- `git status`
- `git diff`
- `git show`
- `git log`
- `git blame`
- `git ls-files`
- `git rev-parse --abbrev-ref HEAD`

Forbidden:

- Any Git command not listed above.
- Any Git command that can modify index, worktree, refs, history, stash,
  remotes, submodules, or config.
- Examples: `git add`, `git restore`, `git checkout`, `git switch`,
  `git commit`, `git merge`, `git rebase`, `git cherry-pick`, `git revert`,
  `git reset`, `git clean`, `git stash`, `git branch -d/-m/-c`, `git tag`,
  `git fetch`, `git pull`, `git push`, `git submodule update`, `git config`.

If an operation requires forbidden Git usage, the agent must stop and ask the
user to run it manually.

## Go Rules

### One Source Code File Per Package

Every package must have precisely one source code file. If it's getting too long
or burdensome, then it needs to be split into meaningful and well-thought-out
subpackages (and sub-subpackages, if needed). Packages using different build
tags in multiple files to accomplish some legitimate goal are exempted (solely
to the extent necessary to apply such build tags).

When you split packages, do not just create tiny packages with low hanging
fruit. Choose meaningful lines that make sense to be tested together and are
logical from an API design perspective.

When appropriate to split up a large file into organized chunks (while keeping
it a single file), you should add comments using the following shape:

```
/////////////////////////////////////////////////////////////////////
/////// Applicable Topic Or Description
/////////////////////////////////////////////////////////////////////
```

### Never Alias Internal Go Package Names

This rule applies to internal packages in this repository. If you find yourself
needing to alias one of our first-party package names at import, the package
naming is wrong and should be redesigned.

For external packages (stdlib or third-party), aliases are allowed when they
improve clarity or avoid collisions.

### No Builder Patterns

Builder-pattern APIs are prohibited in Go code.

- Do not add fluent/chained configuration methods (for example:
  `NewX(...).WithY(...).WithZ(...)`).
- Do not add `With*`/`MustWith*` mutator methods used primarily for chained
  construction.
- Prefer explicit struct literals, plain functions, and explicit option structs.

### Function Formatting

Functions with many parameters shall format such parameters vertically, like so:

```go
func someFuncWithManyArgsFormattedVertically(
	arg1 any,
	arg2 any,
	arg3 any,
	arg4 any,
)
```

### Do Not Add `doc.go` GoDoc Files.

Just put package-level docs into whatever the main entry file for that Go
package is. I don't want this repo littered with `doc.go` files.

### Kitchen-Sink Utils or Shared Packages Are Fine

If they legitimately help with package boundaries, organization, and
import-cycle-avoidance, then having a shared kitchen-sick types and/or utils
package is perfectly fine, as long as they stay out of the core public
entrypoints for end users. I don't care that this is non-idioamatic Go. It's
fine, and it's far better than creating awkwardly-named, hyper-focused Go
packages just for purity's sake.

## TypeScript Rules

### TypeScript Test Split Is Mandatory

For all TypeScript test execution, keep source and dist test modes separate.

- Source tests: run `make tstest-source` (or the equivalent
  `pnpm vitest run --exclude "typescript/vorma/black_box_tests/dist/**"`).
- Dist tests: run `make tstest-dist` (or the equivalent
  `pnpm vitest --run --config typescript/vorma/black_box_tests/dist/vitest.config.ts`).
- If running targeted subsets, still keep the same split and use the dist config
  for all tests under `typescript/vorma/black_box_tests/dist/**`.

### Special Rule: UI Adapter Tests and Imports Are Dist-Only

For `typescript/vorma/ui-adapters/**`, always operate on and validate behavior
through compiled outputs.

- All `ui-adapters` tests must run in dist mode with
  `typescript/vorma/black_box_tests/dist/vitest.config.ts`.
- All `ui-adapters` imports in those tests must resolve through `npm_dist`
  exports (for example `vorma/react`, `vorma/preact`, `vorma/solid` via the dist
  config), not source-path aliases.
- When auditing or fixing adapter behavior, treat compiled dist behavior as the
  source of truth for test validation.

### Faux Named Params

For internal/private TypeScript implementation code, function signatures that
take multiple parameters of the same type are prohibited because they are
order-fragile and unclear at callsites.

- Do not write signatures like `(a: string, b: string)` or
  `(x: number, y: number, z: number)` unless the shape is imposed by an external
  interface or callback contract you do not control.
- Prefer a single object parameter with explicitly named fields (faux named
  params), for example:
  `someFunc({ sourcePath, destinationPath }: { sourcePath: string; destinationPath: string })`.
- When a function currently violates this rule, refactor both the function
  signature and all callsites to the object-parameter form.
- Do not break established public API signatures for this rule unless explicitly
  requested. New APIs should follow this rule by default.
- Some exceptions may be appropriate, such as extremely simple key-value setters
  that are near-impossible to get wrong such as `setItem(key, value)`.
  Additionally, no need to follow this for compare functions where order doesn't
  actually matter.

## Frontend Runtime and Contract-Test Rules

### Escalate Suspicious or Ambiguous Normative Intent

If a test expectation appears suspicious, ambiguous, bug-memorializing, or
first-principles-wrong:

- Escalate to the user immediately upon noticing it.
- Do not continue triage or implementation work on that expectation until the
  user responds.
- Do not change normative contract expectations without explicit user sign-off.

### Backend-Contract Trust for Backend-Owned Fields

- Frontend runtime must trust backend-owned contract fields.
- Do not add frontend fallback or recovery logic for backend-owned contract
  violations.
- Do not add frontend panic/assert validation branches for backend-owned
  contract violations.
- Backend contract enforcement belongs in backend tests and backend build-time
  invariants, not frontend runtime checks.
- Keep frontend runtime checks only for non-backend-owned surfaces (e.g., user
  input or client-side storage).

### Do Not Spend Bundle Size Protecting Type-System Violations

- Do not add runtime guards whose primary purpose is tolerating consumer
  type-system bypasses (for example `as any`, unsafe casts, or deliberately
  incorrect generic arguments).
- Assume typed public APIs are used according to their type contracts.
- If a caller bypasses TypeScript contracts, resulting runtime failures are an
  application bug, not framework runtime responsibility.
- Keep runtime validation only for truly untyped/tamperable boundaries (for
  example browser persistence or raw network payload boundaries that are not
  backend-owned invariants).
- If any tests tries to require us to be resilient against a type system
  violation, that test assertion is wrong and should be changed (but always
  double check with me first).

### Client-Owned Persistence Validation Must Be Minimal

- Browser-owned persisted state (`sessionStorage`, `localStorage`, IndexedDB,
  Cache API, cookies) is mutable/tamperable and not backend-owned.
- Parse safely and validate only the fields required for current behavior.
- Ignore/drop malformed entries instead of adding broad fallback systems.
- Add black-box regression tests for client-owned persistence parsing paths.

### Browser-Only Frontend Runtime Rule

- Frontend TypeScript runtime and adapter code in this repository is
  browser-only code.
- Do not add server/SSR fallback guards like `typeof window === "undefined"` or
  `typeof document === "undefined"` to frontend runtime paths.
- Assume browser APIs are present on supported frontend runtime paths.
- If logic must be portable across environments, move it to a separate
  non-browser module instead of bloating frontend runtime with server guards.
- Cross-realm resilience is a non-goal; do not ever try to add resilience for
  missing browser APIs or other "cross-realm" concerns.

### Contract Tests Must Stay Strictly Black-Box

- Contract tests must assert only public observable behavior: public API return
  values, DOM/head/title effects, history/location changes, event payload/order,
  and network-visible behavior.
- Contract tests must not assert internal state shape, internal helper
  names/symbols, reducer/state-machine internals, event-plan internals, or
  private implementation sequencing.
- UI adapter behavior validation remains dist-only via `npm_dist` imports and
  dist configuration.

### Never Add Random Hardening To Frontend Code Without Clear Justification

It's easy to forget that every line of code inflates the user bundle, but it's
true, and we want to avoid that bloat. Do not add random hardening "for good
measure" to frontend code the way you might on the backend. The tradeoffs are
different. Some hardening may be appropriate, but you should be able to
articulate clearly why it's necessary and worth the bloat.

## Path Hygiene

- Never, ever commit machine-specific absolute paths (for example, `/Users/...`)
  into repository files.
- Use repository-relative paths in docs and instructions.

## Formatting

- After editing any files formattable by oxfmt (including, without limitation,
  `.ts`, `.tsx`, `.json` and `.md` files), always run `make tsfmt` on the files.

## No Conversational or Changelog Comments

It is prohibited to add conversational or changelog comments to source code
files.

## Report All Issues. No Severity Labels. Never Triage.

When doing reviews or audits, the goal is to find and report **all** issues, not
just a subset and not just "critical" issues.

It is prohibited to triage, prioritize, rank, or categorize issues by severity
or priority.

It is also prohibited to use severity or priority language in review outputs
(for example: "critical", "major", "minor", "P0/P1/P2", "high/medium/low", or
similar labels).

Findings shall be presented as a plain, exhaustive list of issues. If no issues
are found, state that explicitly.

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

## Correct, Ideal Code Is The Goal, Not Quick Unblocks, Easy Migrations, Or Backwards Compatibility

When analyzing code, do not get caught up in "the easiest way to update it" or
"the way to maintain backwards compatibility". The goal is always "what is the
maximally correct and ideal version of this code", regardless of potential
breakages or level of refactor effort. It's never OK to take the lazy approach
to solving a problem; always strive for the truly correct and ideal approach.

## As Long As We Are Sub-1.0, Do Not Add Back-Compat Adapters

For a sub-1.0 library/framework, it's far better for consumers to be forced to
update to latest APIs immediately when they upgrade. Otherwise we risk absorbing
too much cruft before we even get to full stability.

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

## Follow the Semantic Rules in `repodocs/SEMANTIC_RULES.md`

Follow the semantic rules in `repodocs/SEMANTIC_RULES.md`, and any time we agree
on new semantic rules, add them to that rules doc in short, simple terms.

## Generally Speaking, Avoid "Graceful Degradation" -- Fail Loud And Fast Instead

With some exceptions (like the `kit/grace` package, where graceful shutdown is
the whole point), it's generally an anti-pattern to try to be too generous with
input parsing, or to try to always fallback when errors happen, or to try to
support users who are abusing your APIs "just in case". This, ironically, leads
to more brittle code, and it's impossible to know where to "draw the line".
Instead, the goal should simply be to be 100% robust for the intended,
explicitly supported use case, and to fail loudly and quickly in all other
cases. Use common sense here according to context. Don't follow this rule
religiously if it in fact is wrong from first principles for the specific code
you're working on--just use it as a general guiding light.

## Never, Ever, Ever Keep Around "Compat Layers" or "Compat Wrappers"

It's wrong every single time. Zero exceptions. Never, ever do this. Every time
you find yourself tempted to do an incremental "compat layer" strategy, stop and
just go directly to the desired end state instead. Just finish the job and do it
right. Incrementalism is riskier than large refactors, because incrementalism
has a way of sticking around forever and killing codebases little by little.

## Write Clear Comments For All Internal and External Symbols

Don't go overboard, but always include the amount of comments appropriate to
help a human understand the context of what is going on in the code, and write
them the way a human would. Make sure they are declarative and up-to-date, and
never backward-looking, temporal, changelog style, or conversational. Humans
can't read 10 documents instantly to quickly accumulate context the way you can.
Help us out.

## GoDocs / Package Comments Should Be Explanatory, Not Just Descriptive

Don't just state what a package does when writing package-level docs or
comments. Explain why it is needed at all and what it's useful for, and what
problems you would have if it didn't exist, and who the intended consumers are.
Assume the reader is new to the codebase and doesn't have a deep understanding
of neighboring code.

## Do Not Write Stuffy, Enterprise-Style Code

Code should feel light, clear, and punchy, not stuffy and enterprise-y. If it
starts feeling like Java, you're doing something wrong.

## No Rube Goldberg Machines

Every piece of code shall have a clear, articulable reason for existing. Never
add extra layers of indirection unless absolutely required to make a module
reasonably testable.

## Prefer Pure Functions, State Machines, and Tight Engines/Orchestrators To Messy Imperative Styles

Try to write highly testable pure functions and state machines, and
well-designed engines/orchestrators, rather than overly messy spaghetti
imperative code.

## Never Call A "Regression Test" A "Regression"

It's not a "regression"; it's a "regression test". A "regression" is a newly
introduced bug, not a test preventing such bugs. Please avoid this extremely
annoying and counterproductive communication failure mode at all costs.
