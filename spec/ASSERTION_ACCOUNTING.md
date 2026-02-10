# Assertion Accounting Standard

Every legacy test assertion must be recorded exactly once with a disposition.

Allowed dispositions:

- `MEANINGFUL`: normative behavior; must map to one or more `REQ-*` IDs.
- `NON_MEANINGFUL`: excluded from normative spec with explicit rationale.

Required counters per package (`80-assertion-accounting.md`):

- `total_assertions`
- `meaningful_assertions`
- `non_meaningful_assertions`
- `mapped_meaningful_assertions`
- `unclassified_assertions`

Required invariants:

- `total_assertions = meaningful_assertions + non_meaningful_assertions`
- `mapped_meaningful_assertions = meaningful_assertions`
- `unclassified_assertions = 0`

Traceability requirement:

- Every `MEANINGFUL` assertion ledger row must map to at least one requirement
  ID.
- Every requirement in the index must have evidence in `evidence.yaml`.
- Assertion accounting and evidence mapping must stay consistent across mining
  and both required review passes.
