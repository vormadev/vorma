## Path Hygiene

- Never, ever commit machine-specific absolute paths (for example, `/Users/...`)
  into repository files.
- Use repository-relative paths in docs and instructions.

## Markdown Formatting

- After editing any `.md` files, always run Prettier on the file using
  `pnpm prettier` with the appropriate arguments.
