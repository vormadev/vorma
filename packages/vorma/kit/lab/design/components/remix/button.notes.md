# Button Notes

Fully built. `Button` renders a native button host, defaults to `type="button"`, preserves
explicit form button types, supports disabled and loading state, exposes loading anatomy
and custom loading indicators, preserves consumer `mix` on the button host, and keeps
recipe styling on named anatomy slots.

Current coverage includes native host semantics, default and explicit button types,
loading state, custom loading indicator rendering, accessible loading label behavior,
anatomy attributes, responsive recipe prop plumbing, and consumer `mix` event target
ownership.

Fully tested is still WIP. Remaining coverage should focus on form submission behavior in
a real form, disabled browser behavior, responsive CSS emission details, consumer `mix`
precedence, and keyboard activation behavior across browsers.

## Semantic/API Audit

`Button` is an action trigger built on the native `button` element. Links that look like
buttons should be styled links rather than `Button` instances. Loading state disables the
native button and sets `aria-busy`; `loadingLabel` only overrides the accessible name when
a consumer explicitly provides it.
