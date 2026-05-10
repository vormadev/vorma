# Core3 Architecture Ideas

These notes define the forward architecture pressure for core3. The goal is to
start from better primitives and keep every implementation decision accountable
to the existing full gate.

## Operation Algebra First

Model route and API work as operations with one lifecycle vocabulary:

- requested
- started
- superseded
- aborted
- completed
- ignored as stale
- published
- settled
- failed

Navigation, popstate, prefetch, foreground revalidation, background
revalidation, API submit, boot, and HMR should all be explainable with that
vocabulary. Each operation should declare whether it owns visible transition,
background refresh, public settlement, abortable IO, or publication rights.

## Outcome Classification Before Mutation

Fetch and preparation results should be classified into semantic outcomes before
state is mutated.

Route outcomes should include at least:

- publishable route data
- soft redirect
- hard redirect
- build skew reload
- build skew drop
- stale response
- retryable revalidation failure
- terminal revalidation failure
- route error
- ignored background refresh

API outcomes should likewise be classified before they can mutate submission,
redirect, revalidation, or public promise state.

The classification layer should be small, explicit, and testable without DOM,
history, timers, or rendering.

## Publication Transaction

Publication should be a first-class transaction, not a series of commands whose
correctness depends on incidental array order.

A publication plan should describe:

- history write
- current browser position
- route snapshot
- head and CSS changes
- route render commit
- route update notification
- scroll intent
- view transition scope
- pending refresh continuation
- public call settlement
- work-state settlement

The transaction should make it obvious which effects are inside a view
transition and which must run after it.

## State Modes Without Statechart Theatre

Core3 should consider explicit router modes, but only if they prevent illegal
states rather than adding ceremony.

Useful modes may include:

- unbooted
- booting
- ready
- operating
- publishing
- disposed

The model should also represent overlapping work directly. A mode cannot be an
excuse to hide concurrent navigation, prefetch, submit, HMR, and revalidation
facts in side channels.

## Resource Graph Exploration

Core3 should seriously evaluate whether routes, modules, loaders, CSS, head
data, API mutations, and build IDs are resources with dependencies.

Navigation then becomes a graph update. Revalidation becomes invalidation and
refresh. Prefetch becomes warming graph nodes. HMR becomes invalidating module
and loader-derived resources. This may reduce special cases more than another
large route reducer can.

This idea is large enough that it should be proved with a small model before
core3 commits to it.

## Background Revalidation As A Real Operation

Background revalidation should not be a side path. It should be an operation
that can fetch and classify outcomes without owning the visible transition until
it has a publishable result.

It needs explicit rules for:

- when foreground navigation blocks it
- when it can run during boot provisional state
- when it can publish
- when it resolves waiters without publication
- when retries keep or replace operation identity
- when build skew is reported, dropped, or escalated

## API And Route Interaction

API submit should not be glued onto route logic as an afterthought.

Core3 should model:

- submission identity separately from fetch operation identity
- dedupe replacement as an explicit operation outcome
- redirect as an API outcome that may request a route operation
- mutation revalidation as completion policy, not success-only policy
- public API result settlement separately from follow-up revalidation settlement

## Publication Rights

Only one operation should be able to publish a visible route at a time, but more
than one operation may be running. Publication rights should be represented
directly instead of inferred from whichever field currently holds an operation
ID.

This should make stale completion handling local and readable.

## Prepared Data Is Not Publication

Prepared route data should remain inert until a publication transaction consumes
it. This matters for prefetch promotion, route hooks, view transitions,
provisional boot routes, and background revalidation.

Core3 should avoid making "prepared" mean "almost current".

## Work State As Projection

Work state and work-indicator activity should be derived from operation facts,
not pushed by individual branches.

The model should keep public work state and private indicator activity close
enough that they cannot drift, while avoiding JSON-stringify comparison as the
main change detector.

## Browser History And Scroll As Domain Facts

History key, user state, current URL, previous scroll, popstate scroll, reload
scroll, and hash scroll should be modeled as facts at the browser edge.

Core3 should make two edges explicit:

- leaving a browser position
- publishing or restoring a browser position

That should keep hash-only navigation, popstate, replace, push, and reload
restoration from becoming scattered cases.

## Build Skew Policy

Build skew should be classified as a policy outcome, not reported
opportunistically inside response handling.

Core3 should define the matrix of:

- route navigation
- popstate
- prefetch
- foreground revalidation
- background revalidation
- API success
- API failure
- API redirect

Each row should state whether the default behavior is notify, drop, reload, or
settle without visible change.

## View Transitions Are Publication Decorators

View transitions should wrap publication transactions. They should not create a
second publication path.

Core3 should represent what is captured before the transition, what mutates
inside it, and what settles after it. Whether a transaction is wrapped should be
decided from model facts plus browser capability facts; the host should not
invent a separate publication sequence.

## Route Hooks As Gated Effects

Route hooks are effects that gate publication. They should receive immutable
facts about current and next route state, and their completion should produce an
operation-correlated outcome.

They should not be allowed to publish directly, resolve public calls directly,
or mutate router state through hidden closures.

## Host Edge Discipline

The browser host should do IO and return facts. It should not decide route
policy.

Allowed host responsibilities:

- fetch
- import modules
- run client loaders
- wait for CSS
- read and write browser history
- read and write scroll storage
- mutate DOM when given a publication plan
- run user callbacks when commanded

Forbidden host responsibilities:

- stale response policy
- redirect policy
- build skew policy
- refresh retry policy
- public promise settlement policy
- publication ownership

## Size And Simplicity Checkpoint

Core3 ships to client browsers, so bundle size is an architectural constraint.
Core3 should aim to shrink. It may spend bytes only when those bytes buy clear
correctness, remove whole bug classes, or collapse duplicated behavior.

The operation/outcome/publication model should reduce code, not become a second
framework. If the model is larger but still requires branch-local command-order
reasoning, it has failed.

Measure size after the model is coherent, not while it is half-built.

Avoid:

- runtime vocabulary objects that exist only for aesthetics
- abstraction layers that do not erase or inline well
- generic command constructors that add indirection without deleting logic
- parallel data structures for public work and private activity
- broad event buses when direct typed transitions are smaller and clearer

## Testing Discipline

The existing full gate is the acceptance suite. Core3 must pass it.

Do not create a parallel test suite, and do not rewrite the existing suite to
fit core3. This is not public API evolution. This is an architecture replacement
under the same product contract.

Targeted new tests are useful only when they cover new internal primitives that
the existing gate cannot directly isolate. They should supplement the full gate,
not replace it.

Useful targeted tests:

- stale completions cannot publish
- outcome classification has no side effects
- publication plans preserve command ordering locally
- background revalidation cannot steal visible transition ownership
- prefetch data cannot publish until promoted
- API redirect and API revalidation settle independently
- hash-only navigation does not refetch
- build skew policy matrix is explicit

Every targeted model test should make it easier to satisfy the existing full
gate, not easier to argue around it.

## Failure Signals

Restart or redesign if core3 develops any of these smells:

- a port of core2's `update.ts`
- command arrays whose ordering is only understandable from surrounding code
- response handling that mutates route state before classification
- publication spread across unrelated branches
- background revalidation with a special side-channel lifecycle
- public shell inspecting router state to decide route policy
- host code deciding policy because the model lacks vocabulary
- generic helpers that only make dense branches shorter
- a parallel test suite that lets core3 pass while the existing full gate fails
