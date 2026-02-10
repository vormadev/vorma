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
