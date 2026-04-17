# Declarative Read-Only POST Actions

## Idea

Keep non-GET actions revalidating by default, but allow semantic read-only POST
actions to opt out declaratively on the server.

Possible Go shape:

```go
Revalidate: vorma.RevalidateNever
```

## Why

JSON-RPC and similar APIs may use POST for reads. `options.revalidate = false`
works at the call site, but a server-side declaration would avoid making every
caller remember the exception.

## Notes

- Do not change the framework default.
- This should be rare and explicit.
