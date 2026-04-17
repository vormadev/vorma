# Current Route Location Hook

## Idea

Add an adapter hook for reading the current route/location state.

Possible shape:

```ts
const route = app.useCurrentRoute();
```

Possible returned fields:

```ts
{
	url: URL;
	pathname: string;
	search: string;
	hash: string;
	params: Record<string, string>;
	matchedPatterns: string[];
}
```

## Why

Apps often need read-only current route information for UI, analytics, and small
bits of behavior. A first-class hook avoids ad hoc reads from `window.location`.

## Notes

- Keep this descriptive, not a control surface.
- Consider whether this should share machinery with active link state.
