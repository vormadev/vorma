# Core4 Architecture Plan

This plan captures the read-only architecture review from May 11, 2026. It is
about code shape, invariants, and impossible-state pressure, not file
organization.

## Current Direction

Core4's main architectural bet is sound: keep concrete router slots in a pure
model, emit declarative effects, and leave browser resources to the runtime. The
strongest next step is making more of the router's math visible in the type
system and in transition boundaries.

The target is not a generic operation framework. The target is a model where
route, prefetch, refresh, publication, and API submission states cannot drift
apart.

## Implemented Architecture

The implementation now covers these architectural moves:

- `Core4BootingModel | Core4ReadyModel` makes a ready model without `browser` or
  `current` unrepresentable.
- `ActiveRouteSlot` is kind-indexed so boot and revalidation routes cannot
  accidentally carry navigation-only policy.
- `PublicationSlot` stores one immutable `PublicationPlan` with an explicit
  `publishing | committed` lifecycle, and commit requires the matching active
  publishing route transaction.
- route-work cleanup is emitted as an explicit `release_route_work` effect when
  a route token leaves the route/publication lifecycle, instead of being
  inferred from transition names in the runtime.
- API fetch cleanup is emitted as an explicit `release_api_submission` effect
  instead of being owned by the async runner's `finally` block.
- `Core4Token`, `PublicCallID`, `BrowserKey`, and `TimerID` are branded string
  identities, which prevents those identity classes from being interchanged at
  compile time with no runtime object cost.
- `RouteResponseOutcome` and `APISubmissionOutcome` no longer carry stale owner
  or submission snapshots across async boundaries. They carry only the token
  plus the minimal outcome facts; accept boundaries re-read the live owner.

The orphan-running-refresh transition gap is now modeled explicitly: committing
a revalidation publication moves the refresh from `running` to `settling`, and
public settlement then resolves the refresh waiters. `running` and `settling`
refresh states no longer carry duplicate owner tokens; running ownership is
derived from the active revalidation route, and settlement ownership is derived
from the committed publication. `settling` also drops retry-attempt accounting,
because retry decisions are finished once the revalidation publication commits.

The stronger type-level end state is represented by a cross-slot model union:
`running` cannot be manually constructed without an active revalidation route,
`settling` cannot be manually constructed without a committed publication, and a
`publishing` publication cannot be manually constructed without an active route
owner. The union deliberately avoids value-level token equality because
TypeScript cannot express that relationship without runtime wrappers.

## Improvements

### Re-check Ownership At Accept Boundaries

Route response classification still receives a live owner snapshot because
classification needs route-kind facts to decide retry/build-skew policy.
Accepted route outcomes only keep the owner kind. API outcomes keep no
submission snapshot at all.

The accept layer treats outcome facts as capability hints at most. It re-reads
the current slot by token from the model, ignores stale outcomes, and uses the
current slot facts for policy.

This is an architectural hardening point, not a public regression claim unless
an externally observable failure is demonstrated through the client/router
surface.

### Add Core4 Law Tests

Core4 should have its own tests, but their purpose is different from the legacy
client/router acceptance tests. The legacy suite remains the behavioral
authority for externally observable navigation, prefetch, revalidation,
publication, client-loader, and submission behavior.

The `core4` suite should test internal mathematical laws:

- completion outcomes act like capabilities and only affect the live owner of
  their token.
- owner snapshots are hints; accept boundaries re-read current model facts.
- publication is a two-step transaction: commit before public settlement.
- freshness demands are sequence-based, so already-running route work cannot
  satisfy newer demand.
- derived work state comes from slots, and completed or publishing slots do not
  appear as active work.

These tests are not regression tests unless paired with a public-surface
failure. They are proof scaffolding for the architecture.

### Encode Model Phase As A Union

`Core4Model` started as a flat record with nullable fields. The phase union now
excludes a ready model without `browser` or `current`. The next layer is a
cross-slot union that also excludes manually constructed owner drift, such as a
running refresh without the matching revalidation route or a publishing
publication without the active publishing route.

Introduce `Core4BootingModel | Core4ReadyModel` first. Then narrow slot shapes
inside the ready model. The runtime should not need repeated `phase`, `browser`,
and `current` guard logic for states the type system can exclude.

### Split Active Route By Kind

`ActiveRouteSlot` uses one flat shape for boot, navigation, popstate, and
revalidation. Some fields are meaningful for only one or two kinds. A
kind-indexed union would make impossible combinations invalid:

- boot has no public navigation waiters and no visible navigation source.
- revalidation cannot own history push/replace policy.
- popstate does not write browser history.
- navigation owns user-facing settlement, history, scroll, and redirect intent.

The practical payoff is fewer branchy helper functions and less accidental
policy reuse between route kinds.

### Make Publication A Transaction State

`PublicationSlot` and `PublicationPlan` duplicate most of the same facts.
Publication should be a transaction record with an explicit lifecycle:
`publishing` before the host commits, `committed` before public settlement, and
then absent.

Store one immutable plan in the slot and use the slot lifecycle to make
`settle_publication` impossible before `commit_publication`.

### Make Resource Release An Effect

The runtime currently releases abort controllers and client-loader prefetches
based partly on transition-kind checks. That couples host resource cleanup to
transition naming.

Prefer explicit effects for release/cleanup points, and make `run_effect`
exhaustive over `Core4Effect`. New effects should fail typecheck until the host
knows how to run them.

Route publication is still route work: `beforeRouteYield` and
`beforeRouteCommit` hooks use the route token's abort signal. Therefore entering
`publishing` must not release the route resource; cleanup belongs at publication
settlement, publication failure, stale prefetch/fetch cleanup, or supersession.

### Brand Identity Types

`Core4Token`, `PublicCallID`, `BrowserKey`, and `TimerID` are branded string
types. This prevents passing one identity class where another is expected while
erasing to plain strings at runtime.

`SubmissionKey` is deliberately not branded in the current architecture because
it is also the public work-state key and may intentionally be the caller's
`dedupeKey`. If that key needs identity protection later, first split the
concept into an opaque submission identity and a public work key.

This is type-only architecture; it should not add runtime weight.

### Keep The Server Payload Boundary Thin

The route payload comes from the Vorma server protocol, which this codebase
owns. Core4 should not grow a defensive runtime validator for malformed or
hostile route payload JSON unless a public-surface behavior issue proves that
need.

The architectural goal at this boundary is the opposite: keep the trusted
protocol projection small, obvious, and low-weight. Any improvement here should
be about reducing accidental coupling or making the projection's local
assumptions clearer without adding bulky validation to the hot path.

## Order Of Work

1. [x] Add core4 law tests for the primitive machinery without duplicating the
       legacy acceptance suite.
2. [x] Harden stale owner/submission acceptance at the pure boundary.
3. [x] Introduce phase-specific model unions.
4. [x] Split active route into a kind-indexed union.
5. [x] Collapse publication slot/plan duplication into a transaction record.
6. [x] Move route-work release into explicit effects.
7. [x] Brand the identity strings that are actually opaque in the current model.
8. [x] Close the transition-level orphan-running-refresh gap with an explicit
       committed-publication settlement state.
9. [x] Close the type-level cross-slot owner correlations for running refresh,
       settling refresh, and publishing publication.
10. [x] Make effect execution exhaustive so future effect additions are checked.
11. [x] Leave `SubmissionKey` unbranded unless the public work-state key is
        separated from opaque submission identity.
12. [x] Revisit route payload projection only for small trusted-protocol
        clarity; do not add defensive malformed-payload validation without an
        externally observable bug.

After each meaningful change, run focused tests first and then the normal
TypeScript gate for the package.
