# API Error Narrowing Helpers

## Idea

Add small public helpers for narrowing Vorma API/action result errors.

Possible shape:

```ts
if (apiClient.isActionError(result)) {
	// Handle action error.
}
```

## Why

Typed actions often lead to repeated defensive narrowing code in apps. A tiny
helper can make action error handling less stringly without introducing a larger
error framework.

## Notes

- Keep this small.
- Prefer helpers that clarify existing result shapes over new result semantics.
