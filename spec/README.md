# Spec Program Workspace

This directory contains the normative intent mining program used to produce
rebuild-grade specifications for the repository.

Phase 1 (normative intent mining) is currently active. While Phase 1 is
incomplete, edits are restricted to `spec/**`.

Primary entrypoints:

- `spec/WORKER_RUNBOOK.md`: single-file instructions for mining agents.
- `spec/PROGRAM.md`: phase model, quality bar, and completion gates.
- `spec/MINING_DISPATCH.md`: claim board for package-by-package mining.
- `spec/CHECKLIST.md`: operator checklist for each mining cycle.
- `spec/tools/check_all.sh`: guardrail checks.
- `spec/tools/claim_lowest_open_slot.sh`: claim the next eligible slot.
- `spec/tools/mark_slot_done.sh`: mark a claimed slot as done.
