# Conda cc shadows system gcc and breaks sanitizer/C++ link steps on the Linux box

Discovered during the P001 gate closeout (2026-07-01) on the Linux machine that now
records the gates.

## Facts

The machine's PATH resolves the C toolchain inconsistently:

- `cc` and `gcc` → `/home/sjc/miniconda3/bin/` → conda-forge cross toolchain
  (`x86_64-conda-linux-gnu-gcc`), which links against conda's bundled OLD-glibc sysroot.
- `c++` and `g++` → `/usr/bin/` → Ubuntu system g++ 15.2, whose headers target the
  system glibc 2.43 and emit C23 symbol redirects (`strtoul` → `__isoc23_strtoul`).

Any build that compiles C++ with the system g++ but lets rustc drive the final link
through `cc` (conda) fails with undefined `__isoc23_*` symbols: the objects reference
glibc-2.38+ symbols that conda's old sysroot glibc does not provide. Observed twice:

1. `make rust-fuzz` — libfuzzer-sys compiles its vendored libFuzzer C++ with system g++;
   rustc links via `cc` (conda) → `undefined symbol: __isoc23_strtoul` from
   `FuzzerDriver.o`. Blocks the fuzz step of `make rust-gate`.
2. `cargo install wasm-opt` — same mechanism through the cxx bridge (worked around by
   building binaryen from source with the system toolchain end to end; `wasm-opt` 130 now
   lives in `~/.local/bin`).

The main workspace gate steps do NOT hit this: their C deps (blake3, mimalloc) are plain
C compiled and linked consistently, and old-glibc-linked binaries run fine on a newer
glibc.

## Current workaround (in use for gate recording)

`RUSTFLAGS='-Clinker=/usr/bin/gcc' make rust-fuzz` — forcing rustc's link driver to the
system gcc makes the library search paths consistent with the system glibc; both fuzz
targets then build and 4096 runs pass. STATE.md's gate record discloses this override.

## Desired outcome

Fix the machine so the unmodified `make rust-gate` passes: remove or demote miniconda's
compiler shims in PATH (e.g. drop the conda compilers package, or stop activating base by
default), so `cc`/`gcc`/`c++`/`g++` all resolve to one consistent toolchain. This is a
machine-environment task for the maintainer, not a repo change — do NOT commit
machine-specific paths or linker overrides into the repo (path-hygiene rule in
`AGENTS.md`).

## Verification

- `which -a cc gcc g++ c++` all resolve within one toolchain family.
- `make rust-fuzz` passes with no RUSTFLAGS override.
- `make rust-gate` exits 0 end to end unmodified.
