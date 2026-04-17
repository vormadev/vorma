# Status Hook

## Idea

Add an adapter hook for live status.

Possible shape:

```ts
const status = app.useStatus();
```

## Why

Apps may want custom progress UI, disabled states, navigation indicators, and
app-specific loading affordances.

## Notes

- Core already has status concepts.
- The hook should expose a stable live view without forcing users to wire
  subscriptions manually.
