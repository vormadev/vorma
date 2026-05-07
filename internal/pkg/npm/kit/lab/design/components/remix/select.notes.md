# Select Notes

Needs fuller coverage for WAI-ARIA select/listbox behavior, keyboard navigation,
typeahead, focus restore, controlled and uncontrolled value, form integration,
callback timing, and popup host styling.

## Semantic/API Audit

The component family shape is directionally right: Root, Trigger, Value, Popup,
List, Option, Group, GroupLabel, Separator, OptionText, OptionIndicator, and
Icon are unsurprising for a headless select.

Before marking this stable, verify the keyboard/focus behavior against the
current WAI-ARIA listbox/select pattern and keep callback timing consistent with
Dialog and Popover open-change semantics.
