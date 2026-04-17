# Client Loader Reason

## Idea

Pass the load reason to client loaders.

Possible shape:

```ts
clientLoader({ signal, reason: "navigation" | "revalidation" | "prefetch" });
```

## Why

Some client loaders should behave differently during navigation, revalidation,
and prefetch. For example, an app might want to prefetch client-side cache data
on navigation and hover, but skip that work during route revalidation.

## Notes

- This should reinforce the route lifecycle vocabulary.
- It should remain compatible with abort handling.
