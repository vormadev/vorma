# DescriptionList Notes

Needs fuller coverage for `dl`, `dt`, and `dd` semantics, item slot ownership, responsive
layout, nested selector styling, and consumer `mix`.

## Semantic/API Audit

The name and underlying `dl`/`dt`/`dd` semantics are appropriate. The main API watch point
is `descriptionTone`: it is clear enough, but it is a slot-specific visual prop. Keep it
only if component recipes repeatedly need separate term and description tones; otherwise
prefer simpler component-level tone.
