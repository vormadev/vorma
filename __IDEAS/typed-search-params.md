# Typed Search Params

## Idea

Make typed search params feel native.

Possible Go-side shape:

```go
Search: SearchSchema[SwapSearch]{...}
```

## Why

Search params are route state. Typed schemas could provide safer TS link/search
helpers, defaults, normalization, and canonicalization.

## Notes

- This should not become heavy machinery unless real apps need it.
- It should integrate with typed href generation.
