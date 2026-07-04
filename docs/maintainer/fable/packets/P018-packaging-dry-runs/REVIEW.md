# P018 Review

Reviewer: Fable (orchestrator). Date: 2026-07-02. Executor: Sonnet 5 subagent.

## Verdict

**Accepted. Packet closed.** Both publish paths are now proven in dry-run for the first
time, with contents inspected at three independent levels (list, tarball bytes, resolved
manifests) rather than trusted. The tool-choice judgment was excellent: recognizing that
`pnpm publish --dry-run` shows no file list, substituting `pnpm pack --dry-run` (the same
assembly logic), and cross-validating with npm's dry-run specifically because it validates
`bin` targets — which is what turned a plausible-looking package.json into the phase's
most consequential packaging find. The supply-chain review went past the gate bit exactly
as asked: the `r-efi` LGPL finding (invisible to cargo deny's host-target-only graph) is
the kind of narrower-than-it-looks guarantee an exhaustive read exists to catch.
Discipline was correct throughout: zero edits made because zero fell in the authorized
class — the executor proved the restraint the packet demanded even for a defect it had
proven mechanically.

## Fable actions on the findings (decided and landed post-packet)

Three items were mechanical with zero judgment content; Fable landed them directly after
the packet completed (no concurrent executor) and verified each:

1. **`create-vorma` bin fix** — `./.dist/main.js` → `./.dist/main.mjs`
   (`packages/create-vorma/package.json`). The build output is deterministic per tsdown's
   esm format; the old value would have had the registry strip the CLI's entry point on
   publish. Verified: `.dist/main.mjs` exists; JSON valid.
2. **Test-fixture tarball leak closed** — two exact-file negations appended to
   `packages/vorma`'s `files` array (`!core/wire_contract_fixtures.json`,
   `!kit/json/search_param_contract_vectors.json`) — per-file, not patterns, so no
   pattern-breadth judgment was exercised. Verified: `pnpm pack --dry-run` no longer lists
   either file.
3. **P017's queued rustdoc one-liner** — `vite_plugin_contract.rs` doc comment now lists
   all five `VitePluginConfig` fields (added "public static base path"). Verified:
   `RUSTDOCFLAGS="-D warnings" cargo doc -p vorma-build --no-deps` green.

## Routed to the Phase D batch triage (maintainer decisions)

- vorma-contract README authoring; create-vorma README authoring (content).
- Raw-TS-source-shipping design question (deliberate `files` choice — design call).
- `keywords`/`categories` on crates and create-vorma (values are judgment).
- devDependency advisory batch (8, non-shipping; wants a dev-tooling update — network
  installs need maintainer escalation by standing rule anyway).
- `r-efi`/multi-target license visibility (standing-policy question: accept-and-comment vs
  `[graph].targets` in deny.toml).

## Findings

No executor issues found. One observation endorsed for the record: the discovery that
`xtask ts-publish` has NO dry-run form and couples version-bump + git commit + tag +
publish into one unconditional sequence is itself triage-relevant context for the
`simple-release-auto-bumper` ticket.

## Consequence

P018 closes — and with it, EVERY authored Phase D packet (P010–P020). Remaining before
Phase E: the batched surface-polish triage to the maintainer, assembled next.
