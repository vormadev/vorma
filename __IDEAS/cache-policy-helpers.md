# Cache Policy Helpers

## Idea

Make HTTP cache policy easy to attach from server loaders/actions.

Possible Go shape:

```go
c.Cache().Public(time.Second)
c.Cache().Private(time.Second)
c.Cache().NoStore()
```

## Why

Apps often need to distinguish browser-cacheable, server-memory-cacheable,
user-private, and no-store data. Cache behavior is important enough that it
should not feel ad hoc.

## Notes

- If response proxy APIs already support this, this can still be a nicer layer
  over them.
- The API should make private versus public caching very obvious.
