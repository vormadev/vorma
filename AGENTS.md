## Path Hygiene

- Never, ever commit machine-specific absolute paths (for example, `/Users/...`)
  into repository files.
- Use repository-relative paths in docs and instructions.

## Markdown Formatting

- After editing any `.md` files, always run Prettier before handoff.
- Format all Markdown files in the repo (excluding `node_modules`) with:
  `rg --files -g '*.md' -g '!node_modules/**' -g '!.git/**' -0 | xargs -0 ./node_modules/.bin/prettier --write`

## Scope Guardrail

- During normative intent mining work, edit only `spec/**`.
- While step 1 (normative intent mining) is incomplete, edits outside `spec/**`
  are prohibited.
- No exceptions: do not edit implementation/test/config files until step 1 is
  complete.
