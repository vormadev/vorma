# Fable Transcript Notes

These notes are the durable, forward-looking context extracted from the raw Fable
conversation histories. They are intentionally over-inclusive. They should capture the
specific decisions, rejected designs, pitfalls, unresolved work, and active threads that a
future maintainer needs without reopening the raw histories.

This is not a changelog. Dates and transcript order matter only when they explain why a
decision exists, which ideas were rejected, or what still needs correction.

## Raw-History Processing Batches

### `6b4ce1cd-2179-466e-be7f-027821c2e834.jsonl` Lines 1-25

This session only checked the agent shell's Node version twice. The first run reported
`v20.12.1`; a later "try again now please" reported `v26.3.0`. The useful forward fact is
not the exact versions themselves. The useful fact is that an agent session can inherit a
stale shell environment and then later report the maintainer's refreshed toolchain after
restart or environment refresh. When debugging Node, pnpm, oxfmt, or generated-TypeScript
behavior, do not conclude that the maintainer's environment is stale merely because the
agent shell saw an old version once.

## Environment And Tooling Context

- Agent-observed tool versions are evidence about that agent process, not automatically
  evidence about the maintainer's login shell. If a tool-version discrepancy matters,
  re-check in the current process and tie the claim to the exact shell that produced it.
- Under this repository's sandbox rules, `pnpm install` must be escalated immediately.
  Do not spend time on a doomed non-escalated install just to rediscover network
  restrictions.

### `f941344a-12be-4bf4-823c-0b3b17b19f28.jsonl` Lines 1-178

This session has two durable lessons: permission boundaries and inherited toolchain
state.

For permissions, the maintainer wanted this exact shape:

- The agent can read files in the current repo, write files in the current repo, run
  non-destructive local repo commands, and read git.
- The agent must never write git: no staging, unstaging, committing, pushing, pulling,
  fetching, checking out, switching, stashing, resetting, restoring, merging, rebasing,
  cherry-picking, reverting, cleaning, worktree manipulation, git config changes, git rm,
  or git mv.
- The agent must not have out-of-repo edit/write permissions.
- The agent must not edit untracked files unless the maintainer explicitly asks.

The important correction is that Claude global settings cannot express "the repo in which
the model is currently operating" as a dynamic path. Global settings match static command
and path patterns. A global `Edit` or `Write` allow scoped to a literal path only covers
that one path; a home-directory allow is much broader than the maintainer asked for and is
wrong. The correct split is:

- Use global settings for global deny rules: git write/destructive commands and hidden
  memory paths.
- Use per-project/per-repo configuration, the harness sandbox, and `AGENTS.md` for
  additive repo-local permissions and behavior rules.
- Treat "only tracked files" as a behavioral rule unless the current harness enforces it
  at runtime; path-pattern permissions alone do not know git tracked/untracked state.

This session also records a naming and coordination rule: do not invent a harness-specific
project-instruction filename. This repository's cross-agent instruction file is
`AGENTS.md`; the maintainer works with multiple harnesses and does not want tool-specific
instruction docs treated as privileged project memory.

For hidden memory, the global settings visible in the transcript had `autoMemoryEnabled:
false` and deny rules against writing or editing Claude project `memory` paths. The
forward rule is stricter than the literal settings: durable project context belongs in
repo-visible maintainer docs, tests, or source files that the maintainer can review.
Hidden out-of-repo summaries are not acceptable unless explicitly requested.

For Node/toolchain drift, the agent saw this PATH order in the old Claude process:
`/usr/bin`, `/bin`, `/usr/sbin`, `/sbin`, then multiple `~/.nvm/versions/node/.../bin`
entries with `v20.12.1` before later Node versions. That made `node` resolve to
`v20.12.1` even though the maintainer's default was newer. The practical diagnostic is:
after restarting the agent/harness from a fresh environment, ask the agent process itself
to run `node --version`. Checking only the maintainer's terminal can be irrelevant because
the stuck state lives in the already-launched agent process.

The persisted sidecar for that command was read in full. It adds no Vorma design facts:
after the initial `which node` and `node --version` evidence, it is the expanded `nvm`
shell function body plus `no fnm` and `no volta`.

## Permissions And Git Boundaries

- Read-only git commands are the only git operations an agent may use in this repo. Do not
  stage, unstage, stash, commit, push, pull, fetch, checkout, switch, reset, restore,
  merge, rebase, cherry-pick, revert, clean, edit git config, create worktrees, remove git
  files, or move git files.
- Do not treat `git checkout` or `git fetch` as harmless read operations. Both mutate git
  state and were explicitly rejected.
- If a permission request asks for "the repo in question" or "current repo," do not widen
  that to the user's home directory and do not hard-code one repository into global
  settings as though it solves the general request.
- When a tool or harness limitation means the requested hard guarantee is impossible, say
  that before making edits. Do not make the maintainer discover the limitation through
  failed or over-broad changes.

## Subagent Corpus Map

The `36126369-0da4-45a2-83f9-65bd07a35805` session spawned review and verification
subagents around regressions, lifecycle behavior, dev-build/watcher behavior, execution
engine behavior, TS/Vite/HMR behavior, input decoding, runtime fixes, build/dev-loop
fixes, performance/cleanup fixes, and cross-file tracing. Those subagent transcripts are
part of the raw-history scope and must be read with the main session because they contain
the audit findings that drove `REGRESSIONS_AUDIT.md` and later fixes.

The `9e3bfc5a-bb96-4b96-ad2c-24995f8ad7d7` session spawned architecture survey
subagents for `vorma-build`, the core `vorma` crate, small crates/tooling, and the
TypeScript package side. Those surveys are part of the architecture-campaign source
material and should not be treated as secondary if the main transcript references them.
