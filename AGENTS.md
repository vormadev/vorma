## Path Hygiene

- Never, ever commit machine-specific absolute paths (for example, `/Users/...`)
  into repository files.
- Use repository-relative paths in docs and instructions.

## Formatting

- After editing any files formattable by Prettier (including, without
  limitation, `.ts`, `.tsx`, `.json` and `.md` files), always run
  `pnpm prettier` on the files.
