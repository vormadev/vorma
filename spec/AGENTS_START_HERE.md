# Agents Start Here

Purpose: router/entrypoint for Step 1 work.

Program state: **Step 1 (normative intent mining) is active**.

## New Chat Bootstrap (copy exactly)

`Read AGENTS_START_HERE.md, then follow it exactly.`

## Required Flow (in order)

1. Read `spec/SPEC_GOVERNANCE.md` (authoritative rules +
   rebuild-from-scratch-from-spec-alone definition).
2. Read `spec/packages/PACKAGE_INDEX.md` (scope + priorities).
3. If working in shared checkout, claim the next `OPEN` slot in
   `spec/MINING_DISPATCH.md`.
4. Work only the claimed package path under `spec/packages/<package>/**`.
5. At handoff, release your slot in `spec/MINING_DISPATCH.md`.

## Non-Negotiable Gate

- During Step 1, edit only `spec/**`.
- Do not edit source/tests/config/build files outside `spec/**`.
- If any instruction seems ambiguous or conflicting, stop and ask the user.

No light rounds: baseline full replay, then strict verification full replays.

Backward compatibility is not a priority in this pre-1.0 phase.
