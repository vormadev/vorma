# Input Notes

Fully built. `Input` is a direct native input host primitive with recipe `size`/`variant`
styling, `htmlSize` for the native input `size` attribute, native form/value props, state
data attributes, and open consumer `mix` ownership on the input host.

Current coverage includes native host ownership, native input props, `htmlSize`, input
event `currentTarget`, disabled/read-only/required/invalid state reflection, anatomy
attributes, and recipe condition mapping.

Fully tested is still WIP. Remaining coverage should focus on controlled and uncontrolled
value behavior in rendered Remix updates, native form submission, browser validation
behavior, responsive recipe CSS emission details, consumer `mix` precedence, and Field
composition.

## Semantic/API Audit

`Input` stays a plain single-line text-entry primitive. It does not own labels, helper
text, errors, adornments, or grouped layout; those belong to `Field` and consumer
composition. The visual recipe `size` prop intentionally takes the normal
component-library meaning, so native input size is exposed as `htmlSize`.
