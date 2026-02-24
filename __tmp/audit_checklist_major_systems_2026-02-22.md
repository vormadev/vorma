# Open Items

Scope: `wave/*`, `internal/vormaruntime/*`, `kit/mux/*`, `vormabuild/*`,
`vormagogen/*`

## Active TODOs

- [ ] Add any conspicuously missing test coverage.
- [ ] Track remaining burdensome one-file packages in scope while splitting:
      `vormabuild`.
- [ ] Bring `vormabuild` into a meaningful one-file-per-package architecture
      through well-scoped subpackages.
- [ ] `vormabuild` split points for remaining subpackages: `buildlifecycle`
      (watch injection + framework reload action planning), `output`
      (atomic/artifact/fs/hash/vite manifest staging).
