# Dev Route State Diffs

## Idea

Expose route state diffs in development.

Possible shape:

```ts
{
	reason: "revalidation";
	urlChanged: false;
	patternsChanged: false;
	paramsChanged: false;
	loaderDataChanged: [false, true, false];
	clientLoaderDataChanged: [false, false, false];
	moduleURLsChanged: false;
}
```

## Why

Framework behavior feels more trustworthy when users can see exactly what
changed on each commit.

## Notes

- This likely belongs with broader dev/debug events.
- Keep it out of hot production paths unless there is a zero-cost shape.
