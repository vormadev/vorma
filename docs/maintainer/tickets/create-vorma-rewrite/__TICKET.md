# Restore create-vorma For Rust Vorma

Status: open, sequenced after docs/API settle

Restore the old Go-era `create-vorma` new-app system as a current Rust/Vorma scaffold.
This is public onboarding surface, not harmless maintainer tooling.

The current package at `packages/create-vorma` is stale. It still checks for Go, creates
or reuses a `go.mod`, fetches `github.com/vormadev/vorma`, and runs a Go bootstrapper.
That behavior was correct for the old Go framework, but it is wrong for the Rust-era
framework unless a fresh design explicitly proves otherwise.

Do not treat this as a template touch-up. The work is to understand what the old Go
`create-vorma` system usefully guaranteed, then port the concept to the current Rust Vorma
package/API shape.

Source context:

- Old Go scaffold reference: `main` at commit `0a94922d52668205b2fdcb35410302b2bcea5502`
  contains the last known main-branch Go version. Inspect it with:
    - `git show main:packages/create-vorma/main.ts`
    - `git show main:bootstrap/bootstrap.go`
    - `git ls-tree -r --name-only main bootstrap`
- `../board-api-coverage/API_DESIGN.md`
- `../board-api-coverage/PRESSURE_TEST_CENSUS.md`
- `../user-facing-docs/__TICKET.md`
- `docs/maintainer/board-example/README.md`
- `examples/board/README.md`
- `docs/maintainer/ARCHITECTURE.md`

Source-of-truth audit:

- Read the old Go `create-vorma` / bootstrap implementation before replacing it.
- Preserve useful UX guarantees from the old system: correct project creation, dependency
  setup, package-manager wiring, runnable first app, and deploy-target shape where still
  applicable.
- Do not preserve Go-specific mechanics unless they still make sense for Rust Vorma.
- Treat the current TypeScript launcher as stale evidence, not authoritative design.

Known Rust-era scaffold responsibilities:

- Generate a current Rust Vorma app crate with the current exhaustive `AppConfig` shape.
- Include the client `vite.d.ts` needed for CSS side-effect imports.
- Set up workspace/package wiring for examples/apps without hand-made symlinks.
- Generate the current client entry/module shape for the selected UI adapter.
- Produce a first app that runs through the normal Vorma dev/build path without local hand
  fixes.
- Teach current Board/docs-era patterns rather than old Notes-era patterns.
- Avoid adding speculative options or helper surfaces that are not needed by the real
  scaffold.

Done means:

- A new app starts with the current public API shape.
- The scaffold no longer creates a Go app, requires a Go module, or delegates to the old
  Go bootstrap path.
- The scaffold does not need local hand fixes for TypeScript, workspace wiring, or Vite.
- Generated files are deterministic and do not fight repo formatters.
- The generated app has at least one automated smoke test proving it can be created and
  run through the relevant Rust and TypeScript checks.
