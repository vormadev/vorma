# Route Transition Test Harness

## Idea

Expose a public-ish test harness for route state transitions.

It should simulate:

- Initial payload
- Navigation payload
- Revalidation payload
- Module URL changes
- Loader data changes
- Client loader result/error

## Why

Apps need a straightforward way to test tricky route behavior without spinning
up an entire browser flow.

## Notes

- Existing internal adapter tests may provide a starting point.
- Keep the harness small and focused on route semantics.
