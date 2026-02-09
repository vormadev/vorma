# Normative Intent Mining Program Ledger

Status: Active  
Last Updated: 2026-02-09  
Trust Epoch: `E2`

## Canonical Sources

- Package index: `specs/packages/PACKAGE_INDEX.md`
- Package ledgers: `specs/packages/<package-path>/NORMATIVE_INTENT_LEDGER.md`

## Global Rules

- Mining input for every package is mandatory: source + existing tests.
- Per-file counters are package-local (`passes`, `clean_passes`).
- Owner behavior is specified by owner package; consumers reference owner specs.

## Stop Criterion

Program closure requires all package paths to satisfy:

1. Every in-scope file row mined in current epoch.
2. Two consecutive full no-gap rounds recorded.
3. Package checklist/tracker/matrix/issues reconciled.
