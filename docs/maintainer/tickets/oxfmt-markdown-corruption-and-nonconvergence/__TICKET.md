# oxfmt markdown defects: content corruption and non-convergent formatting

Found 2026-07-02 during the P006 review's fmt housekeeping. Two distinct oxfmt defects,
both observed on `docs/maintainer/fable/packets/P001b-linux-dev-rebuild-fix/REPORT.md`,
both breaking `make ts-fmt-check` in ways a write pass cannot fix — and one of them
SILENTLY CORRUPTS CONTENT.

## Defect 1 — content corruption: bold spanning code spans with glob asterisks

Original (valid) markdown: a `**bold**` phrase containing an inline code span with glob
asterisks (`` `src/**/*.rs` ``). Successive `oxfmt --write` passes progressively mangled
it: asterisks escaped (`\*`), adjacent code spans glued together (spaces deleted:
`` `...`sources ``), and an underscore inside a code span converted to an asterisk
(`IN_OPEN` became `IN*OPEN` — real API-name corruption). Each pass changed the text again;
check never passed. The corruption was repaired by hand (drop the bold, keep the code
spans; content restored from the executor's original message).

## Defect 2 — non-convergence: indented code block nested in a list item

A four-space-indented code block inside a `- ` list item gained two more spaces of
indentation on EVERY write pass, forever (verified by diffing consecutive passes). Fixed
by converting to a fenced code block, which is stable.

## Defect 3 — content corruption: stray space near a code span at a reflow wrap boundary

Found 2026-07-02 during P009
(`docs/maintainer/tickets/board-api-coverage/PRESSURE_TEST_CENSUS.md`). A single `--write`
pass injected a space INSIDE a code span's contents right where the reflow chose to wrap
that line: `` `.boolean_attribute("data-server-rendered")` `` (no space in the source)
became `` `.boolean_attribute( "data-server-rendered")` `` — a literal-content corruption
of the code span, not just whitespace-around-it. The same pass also injected a stray space
between a slash-separated pair of adjacent code spans at another wrap point, turning
`.text("url")` immediately followed by a bare slash then `.text("body")` (no space around
the slash in the source) into the same pair with a space inserted right before the second
span's opening backtick. Both were caught only because the corrupted text was mine and
freshly written, so a byte-diff against the pre-write source was possible; this repo's own
existing content already carries several older instances of the second
(adjacent-code-span) form pre-dating this ticket
(`PRESSURE_TEST_CENSUS.md:171,173,247,629`, `CENSUS_COMPLETION_P005.md:58-59`) — those
were left alone as out-of-scope repo-wide cleanup, but they are evidence this is a real,
repeatable defect and not a one-off. Both instances found by P009 were hand-repaired
(dropped the injected space) and the surrounding paragraph restructured to stay under a
proven-stable nested-list pattern (trailing prose kept as the last bullet's own
continuation rather than a separate paragraph after the list, matching
`PRESSURE_TEST_CENSUS.md`'s existing F-17 RESOLVED entry) — after which the file converged
to a stable fixed point across two more `--write` passes (identical checksum).

## Mitigations now in effect

- Fenced code blocks, never indented ones, inside maintainer markdown lists.
- No `**bold**` phrase may span an inline code span containing `*` or `_`.
- After any `--write` pass touching maintainer markdown with inline code spans near a
  line-wrap point, diff the touched region against the pre-write source before trusting it
  — do not assume `--write` is content-preserving.
- A paragraph immediately following a nested, doubly-indented list item is a
  nonconvergence risk; keep trailing prose as the last bullet's own continuation instead
  of a separate paragraph directly after the list.
- Repo-wide `make ts-fmt-check` is green as of this ticket after the two hand-fixes; P009
  added two more hand-fixes on top, scoped to the two files it touched.

## Task

- Reduce both to minimal reproductions and report upstream to oxfmt (or verify fixed in a
  newer release and bump the pinned version).
- Until fixed upstream: consider whether the gate should pin the oxfmt version to avoid
  new-version surprises, and whether a grep-based tripwire for the corruption signature
  (`IN\*`-style mangling, `` `\* `` escapes) is worth adding to a maintainer check — weigh
  against the minimal-tooling rule.

## Verification

- Both minimal repros filed upstream or confirmed fixed in a bumped version.
- `make ts-fmt` twice in a row produces zero diff on the second pass, repo-wide.
