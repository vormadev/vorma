# Spec Program Workspace

This directory contains the normative intent mining program used to produce
rebuild-grade specifications for the repository.

Phase 1 (normative intent mining) is currently active. While Phase 1 is
incomplete, edits are restricted to `spec/**`.

For parallel mining, use isolated git worktrees (one per agent). Shared checkout
is unsupported by guard checks.

Primary entrypoints:

- `spec/WORKER_RUNBOOK.md`: single-file instructions for mining agents.
- `spec/PROGRAM.md`: phase model, quality bar, review gates, and completion
  rules.
- `spec/OWNERSHIP_BOUNDARIES.md`: single-owner and inheritance-by-reference
  rules.
- `spec/MINING_DISPATCH.json`: claim board for package-by-package mining.
- `spec/CHECKLIST.md`: operator checklist for each mining cycle.
- `spec/DECISIONS.json`: append-only decision log for unresolved/resolved
  cross-package questions.
- `spec/TRACEABILITY.json`: append-only goal-to-capability-to-requirement map.
- `spec/PACKAGE_CATALOG.json`: generated slot/catalog mapping.
- `spec/PHASE_STATUS.json`: phase status state file.
- `go run ./spec/tools/cmd/check_all`: guardrail checks.
- `go run ./spec/tools/cmd/check_worker_allowlist`: enforces mining-edit
  allowlist.
- `go run ./spec/tools/cmd/check_append_only_docs`: enforces append-only edits
  for decisions and traceability.
- `go run ./spec/tools/cmd/check_dispatch_manual_edits`: blocks manual dispatch
  edits.
- `go run ./spec/tools/cmd/check_decisions`: validates decisions log structure
  and status discipline.
- `go run ./spec/tools/cmd/check_traceability`: validates traceability JSON
  structure and identifier/path formats.
- `go run ./spec/tools/cmd/claim_lowest_open_slot`: claim the next eligible
  slot.
- `go run ./spec/tools/cmd/generate_catalog_and_dispatch`: regenerate
  `spec/PACKAGE_CATALOG.json` and `spec/MINING_DISPATCH.json`.
- `go run ./spec/tools/cmd/scaffold_packages`: re-render package `spec.json`
  files from the template.
- `go run ./spec/tools/cmd/record_review_pass`: record required independent
  review outcomes.
- `go run ./spec/tools/cmd/mark_slot_done`: mark a claimed slot as done after
  all review gates.
- `go run ./spec/tools/cmd/validate_slot_spec`: JSON artifact validator used by
  guard and done gates.
- `go run ./spec/tools/cmd/check_phase_completion`: enforces completion-only
  gates when `phase1_status: COMPLETE`.

Package artifact format:

- One file per package: `spec/packages/<package>/spec.json`.
- No split requirement files.
- No package markdown table artifacts.
