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

## Current Rule

- Legacy test migration is complete.
- Do not reintroduce mirrored legacy-style assertions.
- Keep first-principles assertions strict; if behavior is ambiguous, ask the
  user immediately.
- Keep `contract_test_harness.ts` minimal and prune dead helpers as suites
  evolve.
