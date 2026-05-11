# Core4 Law Test Checklist

This checklist tracks the end-state law coverage for `core.test.ts`. These are
core4 internal law tests, not a parallel acceptance suite. The legacy
`vorma/core/*.test.ts` suite remains the public behavioral authority for DOM,
history, adapter commits, public `ClientCore` promises, HMR, and end-to-end
navigation/submission behavior.

## Boundary

- [x] Do not test DOM, `window.history`, fetch timing, adapter commits, browser
      scroll, view-transition callbacks, or public `ClientCore` promises here.
- [x] Do not recreate navigation, prefetch, submit, revalidation, or HMR
      acceptance flows from `vorma/core/*.test.ts`.
- [x] Do not add malformed server-payload tests here unless there is an
      externally observable public-surface bug; the Vorma server protocol is
      trusted by this client code.
- [x] Prefer pure transition inputs, direct model construction, effect arrays,
      and type-level negative cases.
- [x] Keep expected-failure tests out of the passing suite; unimplemented ideas
      stay in `PLAN.md`, not in green test output.

## Model And Type Laws

- [x] Booting models permit nullable `browser` and `current`.
- [x] Ready models require non-null `browser` and `current`.
- [x] Ready init and ready model shapes without `browser` or `current` fail at
      type level.
- [x] Running refresh cannot be manually constructed without an active
      revalidation route.
- [x] Settling refresh cannot be manually constructed without a committed
      publication.
- [x] Publishing publication cannot be manually constructed without an active
      route owner.
- [x] Boot active routes cannot carry public waiters, visible source, non-replace
      policy, or work-indicator state.
- [x] Revalidation active routes cannot carry navigation waiters, source,
      history policy, or navigation scroll policy.
- [x] Popstate active routes cannot carry navigate/redirect source tags or
      non-popstate history policy.
- [x] Core route/public/browser/timer identities are not interchangeable with
      each other or with raw strings.
- [x] Public call settlement effects accept public call IDs, not core tokens.
- [x] Route/API resource effects accept core tokens, not public call IDs.

## Capability And Ownership Laws

- [x] Stale route outcomes are ignored when their token no longer owns route
      work.
- [x] Route outcomes are ignored when the live owner kind differs from the
      outcome owner kind.
- [x] Matching route tokens use current active-route facts, not stale href/state
      facts.
- [x] Stale route preparation outcomes are ignored.
- [x] Aborted live route preparation does not mutate the model.
- [x] Prefetch preparation can only update the matching prefetch token.
- [x] API outcomes carry no submission snapshot.
- [x] Stale API outcomes leave the model unchanged and emit explicit API release
      cleanup.
- [x] Matching API tokens use current submission facts.
- [x] API soft redirect uses current submission policy for settlement and
      revalidation.
- [x] API hard redirect settles the current submission exactly once.
- [x] Dedup replacement removes only the replaced submission token and leaves
      unrelated submissions live.
- [x] Aborted undispatched API outcome does not schedule revalidation.
- [x] Aborted dispatched API outcome schedules revalidation from current
      submission facts.

## Phase And Route Laws

- [x] Boot initialization produces a booting model with browser facts and no
      current route.
- [x] Successful boot commit is the transition from booting to ready.
- [x] Ready-only transition starts are undefined outside ready/current state.
- [x] Retargeting active navigation preserves token and sequence.
- [x] Retargeting changed navigation intent replaces public call IDs and settles
      the old waiters.
- [x] Retargeting identical navigation intent appends public call IDs.
- [x] Superseding active route aborts and settles only the active route token.
- [x] Popstate soft redirect converts route work to navigation ownership.
- [x] Revalidation soft redirect converts route work to navigation ownership.

## Prefetch Laws

- [x] Prefetch cannot start for the current same-document href.
- [x] Prefetch cannot start when active route targets the same document.
- [x] Prefetch cannot start when an existing prefetch already targets the same
      document.
- [x] New prefetch aborts the previous prefetch token and replaces the slot.
- [x] Cancel prefetch only affects a matching same-document href.
- [x] Cancel prefetch ignores a non-matching href.
- [x] Promoting fetching prefetch moves the prefetch token into active route.
- [x] Promoting prepared prefetch mints active route from the navigation request
      token.
- [x] Promoting prepared prefetch publishes without a fetch effect.
- [x] Failed/build-skew prefetch cleanup is represented by route release effects
      and never hard-reloads through the prefetch path.

## Publication Laws

- [x] Publication slot lifecycle is `publishing | committed`.
- [x] Publication stores one immutable plan object shared with the publish
      effect.
- [x] Begin publication emits one `publish_route` effect and keeps route work
      abortable for publication hooks.
- [x] Boot publication has no previous route.
- [x] Non-boot publication carries the previous current snapshot.
- [x] Navigation publication has push/replace history action and navigation
      hooks.
- [x] Popstate and revalidation publication have no history action.
- [x] Boot publication has no hooks.
- [x] Same-document and revalidation publications disable view transitions.
- [x] Commit requires the matching active publishing route.
- [x] Commit clears the matching active route and preserves the plan through
      public settlement.
- [x] Commit changes booting phase to ready and keeps ready phase ready.
- [x] Failed publication requires the matching active route.
- [x] Failed publication clears matching publication and route work.

## Freshness Laws

- [x] Refresh demand uses one-past current sequence.
- [x] Coalesced refresh demand preserves waiters.
- [x] Coalesced refresh demand combines skip-work-indicator with logical AND.
- [x] New debounced demand clears the previous debounce timer.
- [x] Firing wrong refresh timer is a no-op.
- [x] Firing matching debounce timer moves demand to pending attempt zero.
- [x] Begin pending revalidation requires ready model, current browser/current
      route, no active route, no publication, and pending refresh.
- [x] Begin pending revalidation moves refresh to running and stores ownership
      in the active route slot.
- [x] Already-running route work cannot satisfy newer freshness demand.
- [x] Later route work can satisfy existing freshness demand.
- [x] Commit of running revalidation moves refresh to settlement state rather
      than leaving orphaned running work.
- [x] `running` and `settling` refresh states do not carry duplicate owner
      tokens.
- [x] `settling` refresh state does not carry retry-attempt accounting.
- [x] Publication settlement resolves settling refresh waiters.
- [x] Retryable revalidation failure schedules exponential retry.
- [x] Non-retryable revalidation failure settles refresh waiters.

## Redirect And Build-Skew Laws

- [x] Active route soft redirect increments redirect count.
- [x] Active route soft redirect preserves token and sequence.
- [x] Redirect fetch effect trigger matches redirected active route kind.
- [x] Redirect to non-HTTP href finishes active route without refetching.
- [x] Redirect to cross-origin HTTP href emits hard redirect and finishes active
      route.
- [x] Redirect to current same-document href finishes active route without
      refetching.
- [x] Redirect over max count fails active route.
- [x] Failed redirect target cannot mutate current route snapshot.
- [x] Build-skew protocol header wins over soft redirect.
- [x] Build-skew route response with reload behavior emits hard redirect.
- [x] Build-skew prefetch and revalidation responses use drop behavior.
- [x] Build-skew notification is suppressed when server build equals client
      build.
- [x] Build-skew notification includes current route and derived work state.
- [x] API build-skew report uses hard reload default only for cross-origin
      redirects.
- [x] Route build-skew report uses hard reload default for cross-origin
      navigation redirects.

## Effect And Work Projection Laws

- [x] Hard redirects do not mutate the model.
- [x] Cross-origin navigation emits hard redirect and settles public calls false.
- [x] Same-document no-op navigation emits scroll/public settlement and no route
      fetch.
- [x] Same-document hash navigation emits publication, not route fetch.
- [x] Work-producing transitions emit the minimal corresponding fetch/prepare
      effect.
- [x] Settling transitions emit public settlement effects exactly once.
- [x] Route work cleanup is represented by explicit `release_route_work` after
      publication settlement, publication failure, stale prefetch/fetch cleanup,
      or supersession.
- [x] API work cleanup is represented by explicit `release_api_submission`.
- [x] Client-loader prefetch cleanup is represented through route release.
- [x] Runtime `run_effect` remains exhaustive over `Core4Effect`.
- [x] Active navigation and popstate appear as navigation work.
- [x] Active boot routes and publishing routes do not appear as navigation work.
- [x] Active/pending/debouncing/retrying refresh appears as revalidation work
      without duplicate projection.
- [x] Running submissions appear as API request work.
- [x] Removed or undefined submissions do not appear as API request work.
- [x] Prefetch fetching/preparing appears only as prefetch projection.
- [x] Prepared prefetch never appears as active work.
- [x] Skip-work-indicator flags propagate from active route, refresh demand, and
      submissions.

## Deliberate Non-Goals

`SubmissionKey` remains unbranded because it is still the public work-state key
and may intentionally equal the caller's `dedupeKey`. If the architecture later
splits public work key from opaque submission identity, add type-level identity
laws for that new identity.

Route payload projection remains a small trusted protocol translation. Do not
add defensive malformed-payload tests unless an externally observable
public-surface bug proves that boundary needs validation.
