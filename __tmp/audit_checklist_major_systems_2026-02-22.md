# Open Items

Scope: `wave/*`, `internal/vormaruntime/*`, `kit/mux/*`, `vormabuild/*`,
`vormagogen/*`

## Active TODOs

- [ ] Raise test coverage for requested scope to 100%.
- [ ] Eliminate remaining coverage gaps.
- [ ] Track remaining burdensome one-file packages in scope while splitting:
      `vormabuild`.
- [ ] Bring `vormabuild` into a meaningful one-file-per-package architecture
      through well-scoped subpackages.
- [ ] `vormabuild` split points for remaining subpackages: `routeparse` (route
      discovery/parsing/registration generation), `buildruntime` (build
      lifecycle + watch + runtime state commit/snapshot), `output`
      (atomic/artifact/fs/hash/vite manifest staging).
