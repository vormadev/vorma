## Path Hygiene

- Never, ever commit machine-specific absolute paths (for example, `/Users/...`)
  into repository files.
- Use repository-relative paths in docs and instructions.

## Markdown Formatting

- After editing any `.md` files, always run Prettier on the file using
  `pnpm prettier` with the appropriate arguments.

## Scope Guardrail

- During normative intent mining work, edit only `spec/**`.
- While step 1 (normative intent mining) is incomplete, edits outside `spec/**`
  are prohibited.
- No exceptions: do not edit implementation/test/config files until step 1 is
  complete.

## Parallel Mining Dispatch

- In shared-checkout parallel mining, claim work via `spec/MINING_DISPATCH.md`
  before editing package artifacts.
- Claim the lowest-numbered `OPEN` slot and edit only the claimed package path
  under `spec/packages/**`.
