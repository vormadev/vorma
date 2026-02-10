# Assertion Accounting Standard

Every legacy test assertion must be recorded exactly once with a disposition.

Allowed dispositions:

- `MEANINGFUL`: normative behavior; maps to one or more `REQ-*` IDs.
- `NON_MEANINGFUL`: excluded from normative spec with explicit rationale.

Required ledger fields per package (in `spec.json` under
`assertion_accounting.ledger`):

- `assertion_id`
- `source_test_ref` (`path`, `line`)
- `disposition` (`MEANINGFUL | NON_MEANINGFUL`)
- `requirement_ids`
- `rationale`

Required derived invariants:

- every assertion is classified (`MEANINGFUL` or `NON_MEANINGFUL`)
- `MEANINGFUL` rows map to at least one local requirement
- `NON_MEANINGFUL` rows map to zero requirements
- derived unclassified assertion count is `0`

Traceability requirements:

- Every `MEANINGFUL` assertion ledger row maps to at least one requirement ID.
- Every requirement has both test and implementation evidence mappings.
- Assertion accounting and evidence mapping remain consistent across mining and
  both required review passes.
