## Path Hygiene

- Never, ever commit machine-specific absolute paths (for example, `/Users/...`)
  into repository files.
- Use repository-relative paths in docs and instructions.

## Markdown Formatting

- After editing any `.md` files, always run Prettier before handoff.
- Format all Markdown files in the repo (excluding `node_modules`) with:
  `rg --files -g '*.md' -g '!node_modules/**' -g '!.git/**' -0 | xargs -0 ./node_modules/.bin/prettier --write`
