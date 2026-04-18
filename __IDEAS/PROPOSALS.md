# Vorma Proposals

This file is a discussion queue, not a decision record and not an implementation
checklist. Each proposal should be revisited from first principles before code
changes.

Implemented or superseded ideas have been removed rather than preserved here.

////////////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////////

## Link State Attributes

Let users style active and pending links with plain CSS, without class-name APIs
or render props.

Preferred shape:

```txt
data-vorma-active-link
data-vorma-pending-link
aria-current="page"
```

Semantics:

- `data-vorma-active-link` means the link target is the current committed route.
- `data-vorma-pending-link` means navigation is pending to that link target.
- `aria-current` should be added only when appropriate and should not override a
  user-provided value.

Implementation concerns:

- Pending state needs target-aware work state, not a global loading boolean.
- Links should not rerender for unrelated route data changes.
- Active comparison should use the same normalized href construction as
  navigation.

////////////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////////

## Client Loader Invocation Args

Client loaders should receive facts about the invocation target directly on the
args object. They should not receive global router state just in case, and the
target facts should not be nested under a `target` key.

Possible shape:

```ts
clientLoader: async ({
	href,
	params,
	splatValues,
	matchedPatterns,
	matches,
	serverDataPromise,
	signal,
	trigger,
}) => {};
```

Already-settled vocabulary:

- Use `trigger`, not `reason`.
- Keep the args object flat.

Open questions:

- What is the exact `matches` element shape?
- Should `matches` include parsed route input?
- Should `matches` include loader data promises, static match facts only, or
  both?
- During prefetch, how much of the not-yet-committed target route should be
  available before `serverDataPromise` resolves?

////////////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////////

## Dev Debug Event Stream

Consider a dev-only route/debug event stream.

Useful events:

- Route fetch start/finish.
- Trigger: init, navigation, popstate, revalidation, prefetch.
- Matched patterns.
- Import URLs.
- Hard reload / build mismatch events.
- Client loader timings.
- Server loader timings when available.
- Route state diffs per commit.

Potential value:

- Debugging navigation, revalidation, prefetch, and stale client builds becomes
  much easier.
- Route state diffs could explain exactly what changed on a commit.

Constraints:

- Keep production cost near zero.
- Prefer one coherent debug stream over scattered debug APIs.

////////////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////////

## Route Transition Test Harness

Consider a small test harness for route state transitions.

It could simulate:

- Initial payload.
- Navigation payload.
- Revalidation payload.
- Module URL changes.
- Loader data changes.
- Client loader result/error.

Potential value:

- Apps can test tricky route behavior without spinning up full browser flows.
- Internal adapter tests may provide a starting point.

Constraint:

- This should stay focused on route semantics, not become a second router.

////////////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////////

## Server Response Helpers

Consider small Go helpers for redirects, status codes, headers, cookies, and
data responses.

Possible shapes:

```go
return vorma.Redirect("/login")
return vorma.Data(data, vorma.Status(201), vorma.Header("x-thing", "y"))
```

Potential value:

- Common response patterns become boring and consistent.
- Apps avoid inventing local response wrapper conventions.

Open questions:

- How does this fit with the existing Go handler style?
- Can simple cases stay simple?
- Should cache helpers be part of this or separate?

////////////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////////

## Cache Policy Helpers

Consider explicit server-side cache policy helpers.

Possible shape:

```go
ctx.Cache().Public(time.Second)
ctx.Cache().Private(time.Second)
ctx.Cache().NoStore()
```

Potential value:

- Cache policy is important enough not to feel ad hoc.
- Public versus private cache behavior becomes visually obvious.

Open questions:

- Is this just a nicer layer over existing response/header APIs?
- Should it apply to loaders, actions, task DAG results, or all of them?
- How should browser cache, server memory cache, and wallet/user-private data be
  distinguished?

////////////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////////

## API Result / Error Helpers

Consider small helpers for working with `SubmitResult`.

Possible shape:

```ts
if (apiClient.isError(result)) {
	// result is narrowed
}
```

Potential value:

- Less repeated narrowing code.
- Clearer action error handling.

Constraint:

- Do not introduce a new error framework unless the current result shape proves
  insufficient.

////////////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////////

## Client Cache Key Helpers

Consider a small typed helper for canonical client-cache keys for Vorma API
submissions.

Possible shape:

```ts
const key = apiClient.key({
	method: "GET",
	pattern: "/quote",
	input,
});
```

Potential value:

- Apps using React Query, SWR, or custom caches get one stable key convention.
- Vorma can reuse its typed action input model.
- This may provide most of the value of a larger React Query integration with
  much less framework commitment.

Open questions:

- Should this live on `apiClient`, a separate helper, or not exist at all?
- Should it support only GET-like submissions or every action?
- Should it include method and normalized href, or method/pattern/input?

////////////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////////

## Optional React Query Helpers

Consider optional helpers for apps using TanStack Query.

Possible shapes:

```ts
apiClient.queryOptions(...);
apiClient.useQuery(...);
```

Potential value:

- Typed output.
- Signal wiring.
- Stable keys.
- Consistent error handling for action results.

Risks:

- Vorma should not make React Query feel required.
- React Query naming and semantics may not map perfectly onto method-based Vorma
  actions.
- A cache-key helper may be enough.

////////////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////////

## Generated Route Helpers

Consider generated route constants or helper objects so app code does not need
to repeat string route patterns.

Possible shape:

```ts
routes.users.detail.pattern;
routes.users.detail.href({ id });
```

Potential value:

- Fewer raw route strings in app code.
- Better discoverability for large apps.
- A natural companion to `buildHref`, `Link`, and `navigate`.

Risks:

- This can become syntactic sugar quickly.
- Generated helper trees can add API bulk.
- Pattern strings are already type-checked in many important places.

////////////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////////

## Ideal App Architecture Docs

Document the intended Vorma app architecture with practical examples.

Core guidance:

- Server loaders for stable route data.
- Client loaders for browser-only route prework.
- React Query or similar for live client data.
- Task DAG for server composition/cache.
- Actions for typed API calls.
- Route revalidation only for route data invalidation.

Potential value:

- Users understand which primitive owns which job.
- Apps are less likely to build unnecessary local routing/data machinery.

Related docs:

- React Query integration.
- Wallet provider placement.
- SSR-safe browser state library settings.
- Client loaders and prefetch.
- Route-level code splitting.
