# Target-Aware Navigation Status

## Idea

Enrich status beyond global booleans so route and link UI can know which URL is
current or pending.

Possible shape:

```ts
{
	isNavigating: boolean;
	isRevalidating: boolean;
	isSubmitting: boolean;
	currentHref: string;
	pendingHref?: string;
}
```

## Why

Global status is good for app-wide progress UI, but per-link pending state needs
to know the active navigation target. A boolean like `isNavigating` cannot tell
one link whether it is the pending link.

## Notes

- Keep the global status API cheap and stable.
- Consider a smaller location/navigation channel if that is better than
  broadening status.
- This would support active/pending link data attributes.
