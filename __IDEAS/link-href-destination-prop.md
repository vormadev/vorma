# Link Href Destination Prop

## Idea

Make `href` the single public destination prop for `Link`.

Clean target API:

```tsx
<Link href="/bob" />
<Link href={{ pattern: "/users/:id", params: { id } }} />
```

## Why

This lets Vorma support normal links and typed route links through the same
mental model: `href` is where the anchor points.

## Notes

- Drop top-level `pattern` from the ideal public API.
- `href` should accept either a string or a typed route descriptor.
- The typed descriptor should preserve route pattern, params, splat, search,
  hash, and state type safety.
- TypeScript inference should be tested carefully, especially nested
  `href={{ pattern }}`.
- Overloaded Link call signatures may be cleaner than one large union.
