# Typed Href Builder

## Idea

Expose the internal helper that turns a typed route descriptor into a string
href.

Possible shape:

```ts
const href = app.href({ pattern: "/users/:id", params: { id } });
```

## Why

Apps sometimes need URLs outside Vorma's `Link` component.

Useful for:

- Normal anchors
- Meta tags
- Redirects
- Copied URLs
- Analytics payloads
- Custom menu/button components

## Notes

- This should use the same descriptor shape as `Link href={{ ... }}`.
- It should preserve the same route type safety.
