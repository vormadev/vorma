# P009 Report

Provenance note: the executor (Sonnet 5 subagent, 2026-07-02) was blocked from writing
this file by the recurring harness report-file guardrail and returned the full content in
its final message; Fable placed it (HTML transport escaping undone).

## What changed

Schema (`examples/board/src/schema.sql`) — board owns this, no framework touch:

- `attachments` gained its own `id INTEGER PRIMARY KEY` (was `story_id` as PK, one row per
  story); added `idx_attachments_story` index.
- New `story_tags (story_id, tag)` composite-PK join table for the checkbox tag group.

`examples/board/src/repo.rs`: `Attachment` gained `id`/`story_id`; new lightweight
`AttachmentSummary` (no body) for listing. `ATTACHMENT_FOR_STORY` replaced by
`ATTACHMENTS_FOR_STORY` (by story, summaries only) and `ATTACHMENT_BY_ID` (by attachment
id, with body). New `TAGS_FOR_STORY` task and `save_story_tags` write fn.

`examples/board/src/resources.rs` — `SUBMIT_STORY` rewritten with every multi-value
`FormData` accessor as its own direct call site:

- `fields()` (`resources.rs:151-163`) — blanket per-field byte-cap sweep, closing a real
  pre-existing gap (story `body` had no length cap at all).
- `fields_named("tag")` (`resources.rs:194-198`) — shape check on the tag group (rejects
  more `tag` fields than checkboxes exist), deliberately distinct from `texts` by reading
  the `FormField`s themselves rather than values.
- `texts("tag")` (`resources.rs:207-212`) — extracts checked tag values, filtered against
  `STORY_TAGS`.
- `files()` (`resources.rs:221-238`) — aggregate count/byte-cap validation across the
  whole submission.
- `files_named("attachment")` (`resources.rs:258-279`) — the repeated-file storage loop.
- `FormFile::into_body` (`resources.rs:271`) — owned extraction for the storage write.

New `STORY_TAGS`/`MAX_STORY_ATTACHMENTS`/`MAX_STORY_ATTACHMENT_BYTES` constants
(re-exported via `lib.rs`, exported to TS via `TsDrafter`). `STORY_ATTACHMENT` resource
repointed `/api/stories/:story_id/attachment` to
`/api/stories/:story_id/attachments/:attachment_id`, with a cross-story-id 404 guard.

`examples/board/src/views.rs` — `STORY` view's `ParallelBatch` widened 2 to 4 tasks
(story, comments, attachments, tags, all independent). `StoryPage` gained
`attachments`/`tags`.

`examples/board/src/document.rs` — F-21 reversal: the document shell's `<body>` chain
gained `.boolean_attribute("data-server-rendered")` (name-only, teaches the HTML
boolean-attribute concept) and
`.known_safe_attribute("data-built-with", format!("{APP_NAME} & Rust"))` (teaches the
escaping trust boundary, with the `&` chosen specifically to make the escaped-vs-raw
difference provable).

Client: `submit.view.tsx` gained a multi-file input + tag checkbox fieldset + client-side
cap pre-warning (reading the same generated constants the server validates against).
`story.view.tsx` renders an attachment list with independent per-item load/download state
and the tag list. Minor `main.css` additions. `vorma.gen.ts` regenerated via the real
production build, never hand-edited.

Tests (`examples/board/tests/app.rs`): new `MultipartBody` builder (repeated
`.field`/`.file` calls = repeated wire fields) replacing the old single-purpose helper;
rewrote the attachment test for multi-file + nested route + cross-story 404; added 7 new
tests (tag round-trip, vocabulary filter, tag-count tamper rejection, attachment-count
rejection, attachment-byte-cap rejection, oversized-field rejection, and a document-shell
test asserting the real rendered `<body>` byte-for-byte). Board: 20 to 28 tests.

## Decisions made

- Board's own SQLite schema change (sanctioned by the packet). Deleted the stale
  gitignored `board.db` dev artifact so it regenerates with the new schema (no migration
  framework exists; every table is `CREATE TABLE IF NOT EXISTS`).
- `fields_named` given a genuinely distinct role from `texts` (group-shape check vs value
  extraction) rather than left as dead weight underneath `texts`'s own implementation.
- `fields()`'s validation closes a real gap (verified `body` had no prior length cap), not
  theater.
- `into_body()` reached via an owned clone because `ctx.input()` only ever returns
  `&FormData` in this framework (confirmed by reading every `input()` signature) — the
  comment states the honest reason plainly rather than overclaiming a performance win.
- New nested download route + cross-story guard, required once attachment ids stopped
  being 1:1 with story ids.
- `boolean_attribute`/`known_safe_attribute` demo values chosen to be genuinely true (the
  document builder only runs for real server responses) and provably different when
  escaped (the `&`), not arbitrary/functionless.
- Front page NOT given attachment/tag display — before this packet only the story page
  showed attachment info at all; extending to the front page would be a new feature, not
  the packet's stated "wherever stories already show their attachment."
- Test constants re-exported from the crate root so tests import the same constants the
  implementation validates against (anti-magic-string rule).
- `.claude/launch.json` created for live manual verification, then deleted — not part of
  the deliverable.
- Three oxfmt markdown corruption instances (two genuine tool bugs, one an authoring
  mistake) found and hand-fixed while formatting the census docs; verified to a genuine
  fixed point (identical checksum across repeated `--write` passes); documented as "Defect
  3" in the existing `oxfmt-markdown-corruption-and-nonconvergence` ticket.

## Gate results

- `cargo test --workspace --all-targets`: 575 passed, 0 failed (board 28/28, up from 20/20
  pre-packet).
- `cargo test --workspace --doc`: 2 passed, 0 failed.
- `cargo clippy --workspace --all-targets -- -D warnings`: clean, exit 0.
- `cargo fmt --all --check`: clean, exit 0.
- `pnpm exec tsgo -p examples/board --pretty false`: clean.
- `pnpm exec oxlint --config=oxlint.config.ts .`: clean.
- `pnpm exec vitest run --reporter=dot`: 851/851, 42/42 files (board has no vitest files
  of its own, per the board README's testing-surface guidance).
- Board-scoped `oxfmt --check` (board + the two census docs + the corruption ticket):
  clean, verified as a true fixed point.
- Production build (`cargo run -p vorma-board-example`): clean, all 19 client chunks.
- Manual live-browser verification (temporary `.claude/launch.json`, removed after):
  signed in, checked two tags, attached two real files, submitted — story page listed both
  attachments independently, tags rendered, front-page counter incremented, per-attachment
  Load/Download state worked independently, and `document.body.outerHTML` confirmed the
  exact rendered attribute forms live.

## Benchmarks

Not applicable — app-code only.

## Escalations / open questions

None requiring a maintainer decision.

## Discovered out-of-scope work

1. `vorma.gen.ts` regeneration is not oxfmt-idempotent — `TsDrafter::add_const`
   (`crates/vorma-contract/src/tsgen.rs:99-105`) always renders multi-line quoted-key
   JSON; oxfmt wants single-line short arrays and unquoted identifier keys. Verified
   pre-existing-but-latent. Fix belongs in `vorma-contract`; flagged for a ticket (Fable
   filed `tsgen-drafter-oxfmt-idempotency` at review).
2. Ticket updated (not new): `oxfmt-markdown-corruption-and-nonconvergence` gained "Defect
   3" (space injected inside/beside code spans at reflow wrap boundaries; two fresh
   instances hand-repaired; several older pre-existing instances located in the census
   files at `PRESSURE_TEST_CENSUS.md:171,173,247,629` and
   `CENSUS_COMPLETION_P005.md:58-59`, left as repo-wide cleanup).

## Census updates

- `PRESSURE_TEST_CENSUS.md`: appended "F-21 LANDED (P009, 2026-07-02)" and "F-22 LANDED
  (P009, 2026-07-02)" with full citations, teaching rationale per accessor, and covering
  tests.
- `CENSUS_COMPLETION_P005.md`: appended a "P009 update" section flagging its own stale
  rows, pointing to the census as the durable record.
