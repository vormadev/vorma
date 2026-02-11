# Client Contract Tests

This folder holds high-level, public-behavior tests for `vormaclient/client`.

## Scope

- Assert stable behavior through the exported client API.
- Assert strict first-principles correctness, not implementation quirks.
- Avoid asserting private state, internal map sizes, or implementation-specific
  call ordering.
- Keep scenario setup small and readable via `contract_test_harness.ts`.

## File Naming

- Use `*.contract.test.ts` for contract suites.
- Group by behavior surface, not by internal file boundaries.

## Migration Rule

When adding coverage here, map overlapping legacy tests in
`CLIENT_REFACTOR_PROGRESS.md` and only mark legacy cases
`READY_TO_DELETE_AFTER_SIGNOFF` once contract parity is confirmed. Do not add
relaxed interim assertions; if behavior is ambiguous, ask the user immediately.
