# Specs Program Checklist

Status: Active  
Last Updated: 2026-02-09

## Canonical Index

- [x] Package-path canonical index exists: `specs/packages/PACKAGE_INDEX.md`

## Program Rules

- [x] Package-path ownership model is active (`specs/packages/<package-path>/`).
- [x] Every package has independent spec/checklist/audit/ledger/matrix/issues files.
- [x] Intent mining requires both source and tests, with per-file `passes`/`clean_passes`.
- [x] Old shared out-of-scope ledger removed.

## Priority Bands

- [ ] P0 closure: `vorma`, `vormabuild`, `vormaruntime`, `vormaclient/client`, `wave`, `wave/tooling`, `kit/matcher`, `kit/mux`, `kit/response`, `kit/validate`, `kit/headels`, `lab/tsgen`, `lab/viteutil`.
- [ ] P1 closure: remaining `kit/*`, `lab/*`, and `vormaclient/*` package paths.
- [ ] P2 closure: `bootstrap` package path (explicitly lowest priority).

## Program Stop Condition

- [ ] Every package in `specs/packages/PACKAGE_INDEX.md` has two consecutive no-gap full rounds in its own ledger.
