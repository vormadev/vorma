# Vorma Proposals

This file is a discussion queue, not a decision record and not an implementation
checklist. Each proposal should be revisited from first principles before code
changes.

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
