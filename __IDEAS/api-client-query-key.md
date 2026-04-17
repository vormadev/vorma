# API Client Query Keys

## Idea

Add a typed helper for producing canonical client-cache keys for Vorma queries.

Possible shape:

```ts
const key = apiClient.queryKey({ pattern: "/quote", input });
```

## Why

Apps using React Query or other client caches need stable keys. Vorma already
has the route/action type information, so it can provide the boring, correct
default instead of each app inventing its own shape.

## Notes

- This does not need to commit Vorma to React Query.
- The main value is canonical key shape and type safety.
- The helper should match the exact input accepted by `apiClient.query`.
