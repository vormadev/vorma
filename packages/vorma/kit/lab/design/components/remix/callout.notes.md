# Callout Notes

Needs fuller coverage for role semantics, title/body relationships, icon accessibility,
tone variants, and slot `mix`.

## Semantic/API Audit

`Callout` is acceptable as a static message/section-message primitive, but it must stay
distinct from future `Alert`. `Alert` should own alert semantics, live-region behavior,
dismissal, and any status-message behavior if those are introduced.
