# vorma-contract: borrowed construction path for DocumentElementContract rendering

Deferred from P004 Part 2 (2026-07-01) as a design question — recorded per the
tickets-are-the-default doctrine.

## Facts

`render_document_element` (vorma-contract) accepts only owned `DocumentElementContract`s,
whose builder API (`with_attributes`, `with_known_safe_attributes`, ...) is
owned-by-design and used at both declaration time and runtime across the crate.
Consequently the per-request dynamic head elements (title, request-varying meta) are
rendered by cloning each `HeadElement`'s attribute collections into a fresh owned contract
per element per request (`view_response.rs::append_payload_head_element`).

P004 Part 2 step 1 already precomputes all manifest-derived (snapshot-constant) fragments,
so what remains on this path is small — a title and at most a couple of meta elements per
request — and was proven below the bench's noise floor at current sizes.

## Design question for the maintainer

Whether a borrowed rendering path (e.g. a `render_document_element_ref` over borrowed
attribute views, or a by-ref contract facade) is worth adding to `vorma-contract`'s public
surface. This is a crate-wide API design call, not a mechanical change; the per-request
win is currently marginal, so this may be a deliberate "no" — record the ruling either way
so the question stops recurring.
