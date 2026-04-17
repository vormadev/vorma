# Optional React Query Helpers

## Idea

Consider optional React Query helpers.

Possible package or helper surface:

```ts
api.queryOptions(...)
api.useQuery(...)
```

## Why

Many Vorma apps may use React Query and write the same wrapper logic: stable
key, signal wiring, typed output, and consistent action error handling.

## Notes

- Keep this optional.
- The smaller `apiClient.queryKey` helper may deliver much of the value without
  depending on React Query.
