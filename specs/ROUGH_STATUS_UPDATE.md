# Rough Status Update

Status: Active  
Last Updated: 2026-02-09
Update Style: Full overwrite each update. No cumulative timeline.

## Current Truth

- Package specs: `65` total (`14` active/non-placeholder, `51` placeholder).
- Checklists with open items: `65/65`.
- Open conformance issue rows: `121` total (`70` in active packages, `51` in placeholder packages).
- Working tree is intentionally dirty (`78` modified files), almost entirely spec-doc edits.

## High-Confidence State

- Package-first spec layout is stable under `specs/packages/<package>/`.
- Vorma wrapper boundary and Wave runtime/tooling ownership split are in place.
- `kit/response` and `kit/headels` are now non-placeholder and mined from source + legacy tests.
- `kit/matcher` replay `E2-R2` is complete with no new issue IDs; exported API surface gaps were reconciled into requirements/matrix.
- `kit/mux` replay `E2-R2` is complete with no new issue IDs; exported API surface gaps were reconciled into requirements/matrix.
- `kit/validate` replay `E2-R2` is now completed from scratch and corrected one prior overclaim: `KIT-VALIDATE-001` is partial (direct `ValidationError.Unwrap()` branch remains source-backed).
- Mining-rule wording is normalized across package ledgers to source + legacy tests outside `conformance/**`.

## Low-Confidence / Risk Areas

- Most packages remain placeholder-level; broad repository trust is still low outside audited packages.
- No package has reached final closure (two consecutive no-gap full rounds).
- Active package issue backlogs are still open and blocking no-gap closure.
- The single largest quality risk right now is false confidence from partially covered behaviors being marked covered; `kit/validate` replay just surfaced one such case.

## Immediate Next Steps

1. Run `kit/matcher` `E2-R3` full replay and verify whether it is truly no-new-gap.
2. Run `kit/mux` `E2-R3` full replay and verify whether it is truly no-new-gap.
3. Continue P0 audit order: `vorma`, `vormaruntime`, `vormabuild`, `vormaclient/client`, `wave/tooling`, `wave`.
4. Keep enforcing delete/replace behavior for incorrect spec text rather than preserving historical wording.

## Handoff Notes

- Treat conformance tests as outputs, not mining inputs.
- Keep owner boundaries strict: owner package defines semantics, consumers reference owner specs.
- If a requirement is only indirectly covered, mark it partial and attach an issue rather than claiming covered.
