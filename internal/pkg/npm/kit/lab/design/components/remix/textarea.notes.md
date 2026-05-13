# Textarea Notes

Fully built. `Textarea` is a direct native textarea host primitive with recipe
`size`/`variant` styling, native form/value props, state data attributes, and
open consumer `mix` ownership on the textarea host.

Current coverage includes native host ownership, native textarea props, input
event `currentTarget`, disabled/read-only/required/invalid state reflection,
anatomy attributes, and recipe condition mapping.

Fully tested is still WIP. Remaining coverage should focus on controlled and
uncontrolled value behavior in rendered Remix updates, native form submission,
browser validation behavior, responsive recipe CSS emission details, consumer
`mix` precedence, and Field composition.

## Semantic/API Audit

`Textarea` is the standard name for the multi-line text-entry primitive. It
should remain a direct textarea host primitive; labels, helper text, errors,
adornments, autoresize behavior, and layout composition belong to `Field`,
recipes, `mix`, or consumer composition.
