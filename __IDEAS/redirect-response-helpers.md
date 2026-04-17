# Redirect And Response Helpers

## Idea

Improve server-side redirect and response helpers.

Possible shapes:

```go
return vorma.Redirect("/login")
return vorma.Data(data, vorma.Status(201), vorma.Header("x-thing", "y"))
```

## Why

Redirects, status codes, headers, cookies, and cache directives are common
enough that apps should not need to invent their own response wrapper patterns.

## Notes

- Fit this into the existing Go style.
- Keep simple cases simple.
