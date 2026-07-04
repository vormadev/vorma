# P018 Report

Provenance note: delivered by the executor (Sonnet 5 subagent, 2026-07-02) in its final
message per the anti-truncation convention; Fable placed it verbatim (transport escaping
undone).

## What changed

No files were changed. Zero manifest edits were made — every candidate gap found (detailed
below) fell outside the packet's narrow authorized-edit class ("mechanical
publish-metadata fixes... missing license/description/repository field," disclosed
individually) and was recorded as a finding instead, per instruction. No source changes,
no test changes, no git actions beyond read-only inspection. The only filesystem writes
were sanctioned build outputs into gitignored paths (`target/package-gate/`,
`packages/vorma/.dist/`, `packages/create-vorma/.dist/`) produced by running
`make rust-package` and `make ts-build` exactly as the packet's scope requires, plus
scratch files under the session's own scratchpad directory (cleaned up).

## Decisions made

- **Ran `make ts-build` to produce a fresh `.dist`** rather than trusting the
  stale/partial `.dist` directories found at session start. Justification: the packet
  explicitly requires inspecting "the would-be-published file lists... with the real .dist
  build," and a stale build would have produced an inaccurate file-list/wasm-freshness
  verdict. `pnpm install --frozen-lockfile` confirmed "Already up to date" first (no
  network install triggered) before running the real build.
- **Used `pnpm pack --dry-run` (not just `pnpm publish --dry-run`) to obtain file lists.**
  `pnpm publish --dry-run` prints only a one-line skip notice with no file listing;
  `pnpm pack --dry-run` performs the identical tarball-assembly logic pnpm's own publish
  path uses and prints the full contents without writing a tarball or touching the network
  beyond registry-metadata reads.
- **Cross-validated with `npm publish --dry-run --tag pre`** as an independent second tool
  against the same `.dist`/`package.json` state, specifically because it surfaces
  `bin`-target-file-existence validation that pnpm's tooling does not surface at all —
  this is what caught the broken `create-vorma` bin path definitively (see findings).
  Read-only from the registry's perspective; not the repo's actual publish path
  (`xtask/src/ts_publish.rs` uses `pnpm`), so presented as corroborating evidence, not the
  primary dry-run proof.
- **Treated every gap found as a finding, never an edit**, including ones that are
  objectively, mechanically wrong (the `create-vorma` bin path) — because the packet's
  edit authorization is scoped narrowly to the "missing license/description/repository
  field" class specifically. Systematically re-checked: every one of
  `description`/`license`/`repository`/`homepage` is present and populated on all six Rust
  crates and both TS packages — there were literally zero instances of the authorized edit
  class available to make anywhere in the repo.
- **Did not run the full re-verification gate** since zero manifest edits were made; per
  the packet's own zero-edit verification branch, `make rust-package` green plus the
  inspections suffices.
- **Did not install `cargo-outdated`** or any other unprovisioned tool (no-new-installs
  constraint); `cargo audit`/`cargo deny` supplied the maintenance-relevant signals,
  supplemented by a manual `Cargo.lock` scan for pre-1.0/alpha-tagged dependencies.
- **Left the pre-existing git-index staleness untouched** (~104 files of stale staged
  content from prior agent-harness checkpoints, matching the exact mechanism documented in
  STATE.md's Open Flags). Never ran `git add`; inspection-only evidence the known hazard
  is still present.

## Gate results

`make rust-package` (full Makefile target, re-run to completion):

```
cargo clean --target-dir target/package-gate
     Removed 418 files, 101.6MiB total
cargo package -p vorma-matcher --allow-dirty --target-dir target/package-gate
   Packaged 22 files, 218.8KiB (49.7KiB compressed)
   Verifying vorma-matcher v0.86.0-pre.3 ... Finished `dev` profile
cargo package -p vorma-tasks --allow-dirty --target-dir target/package-gate
   Packaged 26 files, 178.2KiB (43.2KiB compressed)
   Verifying vorma-tasks v0.86.0-pre.3 ... Finished `dev` profile
cargo package -p vorma-macros --allow-dirty --no-verify --target-dir target/package-gate
   Packaged 8 files, 37.3KiB (9.0KiB compressed)
cargo package -p vorma-contract --allow-dirty --no-verify --target-dir target/package-gate
   Packaged 17 files, 241.7KiB (61.5KiB compressed)
cargo package -p vorma --allow-dirty --no-verify --target-dir target/package-gate <local-patches>
   Packaged 61 files, 790.4KiB (174.1KiB compressed)
cargo package -p vorma-build --allow-dirty --no-verify --target-dir target/package-gate <local-patches>
   Packaged 41 files, 660.8KiB (150.5KiB compressed)
```

All six crates packaged clean. `vorma-matcher`/`vorma-tasks` are additionally `--verify`ed
(full compile from the packaged tarball); the other four use `--no-verify` per the
Makefile's dependency-order design (they depend on unpublished workspace crates resolved
via the `patch.crates-io` local-path config).

`cargo audit`: one allowed warning — RUSTSEC-2026-0190 (anyhow 1.0.102, unsound
`Error::downcast_mut()`), matching STATE.md's recorded gate status exactly; already
ticketed (`anyhow-rustsec-2026-0190-upgrade`); nothing new across 321 scanned crate
dependencies.

`cargo deny check licenses bans sources advisories`:
`advisories ok, bans ok, licenses ok, sources ok`.

`make ts-build`: exit 0, fresh real `.dist` for both packages (the `[UNRESOLVED_IMPORT]`
warnings are expected/cosmetic — tsdown correctly treats the package's own
self-referential subpath imports as externals resolved through the published `exports`
map).

`pnpm publish --dry-run --no-git-checks` (both packages): both printed
`vorma@0.86.0-pre.3 → https://registry.npmjs.org/` then
`[WARN] Skip publishing ... (dry run)` — no network write attempted.

## Package contents verdicts — Rust (scope item 1)

Verified at three independent levels for all six crates: `cargo package --list`, direct
`tar tzf` inspection of the produced `.crate` archives, and the resolved
(post-substitution) `Cargo.toml` inside each archive.

**vorma-matcher** (22 files, 49.7KiB compressed) — clean. Standard metadata files,
`README.md`, `benches/matching.rs`, 9 `src/*.rs`, 6 `tests/*.rs`. Nothing extraneous,
nothing missing. Resolved manifest: `description`/`homepage`/`license`/`repository`/
`readme` all populated; `rustc-hash`/`smallvec` correctly pinned to registry version
ranges (no leaked path deps).

**vorma-tasks** (26 files, 43.2KiB compressed) — clean. Standard set plus
`tests/ui/*.rs`/`*.stderr` (genuine trybuild fixtures). Resolved manifest complete;
`[target.'cfg(loom)'.dependencies]` and the `unexpected_cfgs` lint allowance correctly
carried through.

**vorma-macros** (8 files, 9.0KiB compressed) — clean, minimal. Resolved manifest
complete; `vorma-matcher` dependency correctly resolved to `version = "0.86.0-pre.3"`
(registry form).

**vorma-contract** (17 files, 61.5KiB compressed) — clean of extraneous content, but
**missing a README**: no `README.md` exists anywhere in `crates/vorma-contract/`
(confirmed via direct filesystem check) and the resolved publish-time manifest shows
`readme = false`. The crate's only metadata gap; not an instance of the authorized edit
class (setting `readme = "README.md"` without the file existing would fail
`cargo package`; authoring content is judgment-shaped). Recorded as a finding.

**vorma** (61 files, 174.1KiB compressed) — clean. Ships `src/refresh_script.js`
(correctly — genuine runtime asset: the dev-refresh WebSocket script served to browsers),
the full `compile_fail`/`in_memory_test_app`/`public_api`/ `wire_contract_fixtures` test
suite, `benches/engine.rs`. Resolved manifest complete including the `mimalloc` feature
gate; all four internal `vorma-*` path-deps correctly resolved to registry-version form.

**vorma-build** (41 files, 150.5KiB compressed) — clean. All 38 `src/*.rs` plus
`tests/public_api.rs`. Resolved manifest complete; `[target.'cfg(unix)'.dependencies]`
(nix) correctly carried through.

**Consistent across all six**: no `keywords` or `categories` field on any crate
(grep-confirmed zero matches) — crates.io accepts this but they aid discoverability; not
added since keyword selection is the packet's own named judgment-shaped exclusion. No
per-crate `LICENSE` file (relying on the workspace-inherited SPDX `license = "MIT"` field,
which is sufficient and standard). Every `.cargo_vcs_info.json` correctly records
`"dirty": true` given the genuinely dirty tree at packaging time — accurate, not a defect
(reads `false` on a real publish from a clean, committed tree).

## Package contents verdicts — TypeScript (scope item 2)

The repo's actual TS publish path (`xtask/src/ts_publish.rs`) has **no dry-run form**: it
unconditionally runs `pnpm version --recursive`, `git add .`, `git commit --no-verify`,
`git tag`, then `pnpm publish --access public --recursive` (optionally `--tag pre`) —
every step a real git/network write with no `--dry-run` branch anywhere. Per the packet's
guidance, the equivalent was constructed manually per-package: verify-only frozen-lockfile
install, `make ts-build` (real `.dist`), then `pnpm publish --dry-run --no-git-checks`
plus `pnpm pack --dry-run` for file lists, cross-validated with
`npm publish --dry-run --tag pre`. No network writes occurred.

**packages/vorma** (120 files, 347.3kB tarball / 1.2MB unpacked) —

- Every one of the 14 subpath entries × 2 conditions (28 targets) in the `exports` map
  resolves to a real file after the build — programmatically cross-checked, zero misses.
- **The wasm artifact is present and correct**: `.dist/core/vorma_client_wasm_bg.wasm`
  (81092 bytes) is byte-identical (md5-verified) to the freshly rebuilt
  `core/client_wasm/vorma_client_wasm_bg.wasm` from this same build run.
- `AGENTS.md`, `*.test.*`, `*.bench.*`, `.DS_Store`, `*.notes.md`, `*PLAN.md`,
  `test-setup.ts`, `.dist/kit/url/**`, `tests/`, `core/_test_helpers.ts` — all correctly
  excluded per the `files` negation globs; none in the actual tarball.
- A `LICENSE` file is auto-included by pnpm (pulled from the repo root; correct MIT text).
  npm's dry-run does NOT show this auto-inclusion for this monorepo layout — since the
  real publish path uses pnpm, pnpm's behavior governs and is correct; discrepancy flagged
  for awareness only.
- **Genuinely, deliberately, ships raw TS/TSX source** (`core/`, `kit/`, `ui/`, `vite/`
  whole directories) alongside compiled `.dist/` — an explicit `files`-array design
  choice, not an accident. Recorded as a finding for the maintainer to weigh (install
  footprint, source exposure) per the packet's whether-a-file-list-is-right exclusion.
- **Two test-fixture data files leak into the published tarball**:
  `core/wire_contract_fixtures.json` (8.4kB, header literally reads "Captured
  wire-contract fixtures... consumed by the TypeScript suite") and
  `kit/json/search_param_contract_vectors.json` (5.4kB, "Golden vectors for the
  search-parameter wire contract"). Grep-confirmed the ONLY importer of each is its
  sibling `*.test.ts` file — zero non-test consumers. The `!**/*.test.*` exclusion drops
  the `.test.ts` importers but not these sibling data files, so they slip through the
  whole-directory includes and ship with zero runtime purpose. A genuine gap in
  exclude-pattern coverage, distinct from the deliberate raw-source design above.
- The `.gen.ts` files in the listing (`core/wire_contracts.gen.ts`,
  `vite/plugin_contract.gen.ts`) are, by contrast, confirmed genuine runtime source
  consumed by seven non-test production files — correctly included, not a finding.

**packages/create-vorma** (4 files, 50.2kB tarball / 195.5kB unpacked) —

- Minimal and clean: `.dist/main.d.mts`, `.dist/main.mjs`, `.dist/main.mjs.map`,
  `package.json`, plus the auto-included root `LICENSE`. No source leakage, no test
  fixtures — `files: [".dist/"]` works as intended.
- **No `README.md` exists anywhere under `packages/create-vorma/`** (its sibling has one)
  — the registry page would have no content beyond the one-line `description`. Recorded as
  a finding (content-authoring, outside the edit class).
- **`package.json`'s `bin` field is objectively broken**:
  `"bin": { "create-vorma": "./.dist/main.js" }`, but the actual build output —
  deterministically, per `tsdown.config.ts`'s shared `base` config (`format: "esm"`,
  inherited by every entry) — is `main.mjs`, never `main.js`. Confirmed two ways: (1)
  direct filesystem check — `.dist/main.js` does not exist; (2) independently corroborated
  by `npm publish --dry-run`, which warned three times
  `"No bin file found at .dist/main.js"` and stated it would auto-correct by **stripping
  the `bin[create-vorma]` entry entirely** on a real publish. **Practical consequence if
  published as-is: `npx create-vorma` / `pnpm dlx create-vorma` would resolve to no
  executable at all** — the CLI's primary invocation path silently erased from the
  published metadata by the registry's own normalization. Objectively, mechanically wrong,
  but outside the packet's specific edit-authorization class (wrong-value defect in a
  different field, not an absent field of the three enumerated), so reported rather than
  fixed — the single most consequential finding in this report.

## Supply-chain inventory (scope item 3)

**Rust — `cargo deny check` (all four categories):** all ok. `deny.toml` is minimal with
no exemption/skip lists hiding anything: `yanked = "deny"`, `wildcards = "deny"`,
`unknown-git`/`unknown-registry` both `"deny"` (every dependency from crates.io proper),
license allow-list a standard permissive set (Apache-2.0 [+LLVM exception],
BSD-2/3-Clause, BSL-1.0, CC0-1.0, ISC, MIT/MIT-0, MPL-2.0, Unicode-3.0, Unlicense, Zlib —
no copyleft admitted), `multiple-versions = "allow"` (pragmatic for a workspace this
size).

**Rust — `cargo audit`:** one allowed warning, RUSTSEC-2026-0190 (anyhow 1.0.102), already
ticketed. Nothing else across 321 dependencies.

**Rust — full license inventory (`cargo deny list`, exhaustive):** 0BSD (1), Apache-2.0
(156), Apache-2.0 WITH LLVM-exception (6), BSD-2-Clause (2), BSD-3-Clause (6), BSL-1.0,
CC0-1.0, ISC, **LGPL-2.1-or-later (2)**, MIT, MIT-0, MPL-2.0, Unicode-3.0, Unlicense,
Zlib.

**A genuinely notable finding**: `LGPL-2.1-or-later` appears in the full listing
(`r-efi@5.3.0`/`@6.0.0`) but is NOT in `deny.toml`'s allow-list, yet
`cargo deny check licenses` still reports ok. Root cause traced precisely: `r-efi` is a
UEFI-target-only dependency of `getrandom` — surfacing in `cargo tree` only with
`--target all`, exactly mirroring why cargo deny's default host-target-only graph
resolution never evaluates its license. Currently benign — no target this project builds
for (Linux/macOS/wasm32-unknown-unknown) ever compiles it — but the allow-list's apparent
completeness is narrower than `licenses ok` suggests: a hypothetical `x86_64-unknown-uefi`
build would fail the license check on this exact crate, and `deny.toml` has no
`[graph]`/`[targets]` section making the full multi-target graph evaluated proactively.
Recorded per the exhaustive-review framing.

**Rust — maintenance-signal scan** (manual, `Cargo.lock`-derived): two non-vorma-owned
dependencies carry pre-1.0/pre-release markers worth naming:
`lightningcss`/`lightningcss-derive` (pinned exactly to `1.0.0-alpha.71`/`.43` in the
workspace manifest — a deliberate pre-1.0-alpha pin, a vorma-build dependency) and
`paranoid` (workspace declares `0.0.0-pre.2`, lock resolves `0.0.0-pre.7` — several
pre-releases ahead of the declared floor; resolves and compiles cleanly, so a
maintenance-signal observation only, the `0.0.0-pre.x` numbering signaling very
early-stage upstream development).

**TypeScript — runtime dependency surface (the only dependencies that ship):**
`packages/vorma` has **zero runtime dependencies** — `dependencies`/
`peerDependencies`/`optionalDependencies` all empty; every framework-adjacent library is a
devDependency never installed transitively by consumers. `packages/create-vorma` has
exactly one runtime dependency, `@clack/prompts@^1.2.0` (bundled into the tarball per
tsdown's explicit `deps: {}` override). Full transitive closure traced:
`@clack/prompts@1.5.1` → `@clack/core@1.4.1`, `fast-string-width@3.0.2` →
`fast-string-truncated-width@3.0.3`, `fast-wrap-ansi@0.2.2` (shares `fast-string-width`),
`sisteransi@1.0.5`. **All six MIT-licensed** — zero surprises.

**TypeScript — `pnpm audit` (whole-workspace, exhaustive):** 8 advisories — 1 critical, 3
high, 2 moderate, 2 low. Full list:

- critical: `@vitest/browser` (resolved 4.1.6, patched ≥4.1.8) — "Exposed Browser Mode API
  Can Proxy CDP and Overwrite Config Files, Leading to RCE" (GHSA-g8mr-85jm-7xhm)
- high: `undici` (resolved 7.27.2, patched ≥7.28.0) — TLS certificate validation bypass
  via dropped requestTls in SOCKS5 ProxyAgent (GHSA-vmh5-mc38-953g)
- high: `undici` — WebSocket client denial of service via fragment count bypass
  (GHSA-vxpw-j846-p89q)
- high: `undici` — cross-origin request routing vulnerability
- moderate: `undici` — HTTP header injection via Set-Cookie percent-decoding
- moderate: `undici` — cross-user information disclosure via shared cache whitespace
  bypass
- low: `undici` — denial of service (GHSA-35p6-xmwp-9g52)
- low: `undici` — Set-Cookie SameSite attribute downgrade via permissive substring
  matching (GHSA-g8m3-5g58-fq7m)

Every path traced (`pnpm why`): all resolve exclusively through root `devDependencies`
(`@vitest/browser-playwright`→`vitest`→`jsdom`→`undici`;
`@vitest/browser-playwright`→`@vitest/browser`) — **none appear anywhere in either
published tarball's actual dependency chain**. Reported in full per the
exhaustive-inventory demand and the no-severity-triage policy, with the
shipping-vs-devtooling distinction stated plainly.

## Version/metadata coherence (scope item 4)

**Rust:** all 11 workspace-member crates inherit `version.workspace = true` uniformly —
zero crates declare their own version — and every one resolves to `0.86.0-pre.3`, matching
the workspace declaration exactly; the five internal `vorma-*` workspace-level dependency
pins also read `0.86.0-pre.3`.

**TypeScript:** both publishable `package.json` files declare `0.86.0-pre.3` — matching
the Rust workspace version exactly. Root `package.json` correctly `"private": true`.

**Cross-language:** Rust and TS in lockstep at `0.86.0-pre.3` throughout — no drift found
anywhere.

**On the auto-bumper ticket** (`simple-release-auto-bumper`, referenced not duplicated):
no versioning friction was actually encountered — every version everywhere already
coherent; the ticket's premise confirmed still accurate (bumping is a fully manual
single-source-of-truth edit today).

## Escalations / open questions

1. **`create-vorma`'s `bin` points at a file that will never exist** (`./.dist/main.js` vs
   the deterministic build output `./.dist/main.mjs`) — demonstrated by direct filesystem
   inspection and corroborated by npm's dry-run, which would silently strip the broken
   entry on a real publish, erasing the CLI's primary invocation path. Outside the edit
   authorization; needs the maintainer's go-ahead (correct the path, or reconsider the
   build config).
2. **`crates/vorma-contract` has no README.md** (unlike its five packaged siblings) and no
   `readme` manifest field. Content authoring is outside the edit authorization.
3. **`packages/create-vorma` has no README.md** either. Same disposition.
4. **Whether `packages/vorma`'s deliberate raw-TS-source-shipping design is the desired
   shape** is a maintainer call — a real, deliberate `files`-array choice, not an
   oversight; no position taken beyond surfacing it.
5. **Two test-fixture `.json` files leak into the published tarball** (details above) — a
   genuine exclude-pattern coverage gap, unlike finding 4; a mechanical exclude-pattern
   addition would close it, but pattern selection is still a content/scope judgment not
   authorized unilaterally.
6. **`create-vorma` has no `keywords` field** while its sibling does — worth adding for
   discoverability, but keyword values are the packet's own named judgment-shaped
   exclusion.
7. **8 TS-side advisories in devDependencies** (1 critical RCE-class in `@vitest/browser`,
   7 in `undici` via the jsdom/vitest chain) — confirmed non-shipping, reported in full; a
   dev-tooling update would likely resolve all 8 (single upstream chain), though version
   bumps are outside this packet's remit.
8. **`r-efi` (LGPL-2.1-or-later) exists in the full multi-target dependency graph** but is
   invisible to cargo deny's default host-target-only license check — currently benign;
   recorded for awareness of the allow-list's real coverage.
9. **Pre-existing git-index staleness observed** (~104 files staged, matching STATE.md's
   documented harness mechanism exactly) — not new; no action taken.

## Discovered out-of-scope work

No new tickets filed. Every finding is either already covered by an existing referenced
ticket (`anyhow-rustsec-2026-0190-upgrade`, `simple-release-auto-bumper`) or disclosed in
the Escalations section for triage-time disposition, consistent with how the other Phase-D
`*-release-quality-findings` were handled (batched, not filed piecemeal).
