# Generated Route Helpers

## Idea

Generate route constants or route helper objects.

Possible shape:

```ts
routes.users.detail.pattern;
routes.users.detail.href({ id });
```

## Why

Vorma route patterns are identity-bearing, so reducing raw strings in app code
can improve safety and readability.

## Notes

- This could start with constants before growing into richer builders.
- It should compose with the public typed href helper.
