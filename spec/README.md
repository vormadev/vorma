# Spec Program Workspace

This directory contains the normative intent mining program used to produce
rebuild-grade specifications for the repository.

Phase 1 (normative intent mining) is currently active. While Phase 1 is
incomplete, edits are restricted to `spec/**`.

Primary entrypoints:

- `spec/WORKER_RUNBOOK.md`: single-file instructions for mining agents.
- `spec/PROGRAM.md`: phase model, quality bar, and completion gates.
- `spec/OWNERSHIP_BOUNDARIES.md`: single-owner and inheritance-by-reference
  rules.
- `spec/MINING_DISPATCH.md`: claim board for package-by-package mining.
- `spec/CHECKLIST.md`: operator checklist for each mining cycle.
- `spec/tools/check_all.sh`: guardrail checks.
- `spec/tools/check_worker_allowlist.sh`: enforces mining-edit allowlist.
- `spec/tools/check_append_only_docs.sh`: enforces append-only edits for
  decisions and traceability.
- `spec/tools/check_dispatch_manual_edits.sh`: blocks manual dispatch edits.
- `spec/tools/claim_lowest_open_slot.sh`: claim the next eligible slot.
- `spec/tools/mark_slot_done.sh`: mark a claimed slot as done.
