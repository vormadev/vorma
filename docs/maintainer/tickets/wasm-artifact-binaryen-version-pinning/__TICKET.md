# Pin the binaryen/wasm-opt version for the shipped client wasm artifact

Observed during the P001 gate closeout (2026-07-01): `make rust-gate`'s
`rust-build-client-wasm` step regenerates the TRACKED artifact
`packages/vorma/core/client_wasm/vorma_client_wasm_bg.wasm` through whatever `wasm-opt`
the machine has. The Linux gate run (binaryen 130, built from source) produced different
bytes than the committed artifact (81228 → 81070 bytes), which was built on macOS with a
different binaryen.

Consequence: every gate run on a machine with a different binaryen version flip-flops a
tracked shipped artifact, producing diff noise and making "which bytes ship in the TS
package" a function of who last ran the gate.

## Task

Decide and implement a single source of truth for the artifact bytes. Options include
pinning a binaryen version that all gate machines must have (checked by the build step —
fail loudly on mismatch, per the no-silent-skew doctrine), or making a specific machine
or CI job the only artifact producer. Whatever the ruling, the wasm-opt invocation should
verify its version matches the pin so drift cannot land silently.

## Verification

- Two consecutive `make rust-build-client-wasm` runs on machines with the pinned setup
  produce byte-identical artifacts.
- A machine with the wrong wasm-opt version fails the step loudly instead of writing
  different bytes.
