# Listbox Notes

## Fully Built Contract

`Listbox` is a standalone listbox primitive for choosing one or more predefined
options from a visible collection. It is not a popup select, combobox, command
palette, menu, grid, or collection of arbitrary interactive children.

The public anatomy is:

- `Root`
- `Group`
- `GroupLabel`
- `Option`
- `OptionText`
- `OptionIndicator`

Root owns value state, highlighted option state, keyboard navigation, typeahead,
orientation, and active-descendant focus semantics:

- `selectionMode`, defaulting to `single`
- single selection: `value`, `defaultValue`, and `onValueChange`
- multiple selection: `values`, `defaultValues`, and `onValuesChange`
- `highlightedValue`, `defaultHighlightedValue`, and `onHighlightChange`
- `disabled`, `required`, and `invalid`
- `loopFocus`
- `orientation`, defaulting to `vertical`
- `selectionFollowsFocus`, defaulting to disabled
- `typeahead`, defaulting to enabled

The root is the focus target. It uses `role="listbox"`, `tabIndex=0`, and
`aria-activedescendant` to keep DOM focus on the listbox while visually
highlighting options. It sets `aria-multiselectable="true"` in multiple
selection mode and `aria-orientation` for the current orientation.

Options use `role="option"` and expose selected, highlighted, and disabled state
through ARIA and data attributes. Groups use `role="group"` and connect to their
`GroupLabel` with `aria-labelledby`.

Keyboard behavior covers the standalone listbox shape:

- `ArrowDown` and `ArrowUp` move visual focus.
- In horizontal orientation, `ArrowRight` and `ArrowLeft` also move visual
  focus.
- `Home` and `End` move to first and last enabled option.
- `PageDown` and `PageUp` jump by a page-sized amount.
- Printable characters typeahead to the next matching enabled option.
- Single selection: `Enter` and `Space` select the highlighted option.
- Multiple selection: `Enter` and `Space` toggle the highlighted option without
  requiring modifier keys.

## Current Test Status

Testing is WIP. The current tests cover anatomy, focusable root semantics,
selected-option active-descendant focus, keyboard highlight versus selection,
controlled single selection, controlled highlighting, loop focus, page jumps,
multiple selection, controlled multiple selection, horizontal orientation,
selection-following-focus opt-in, disabled root and option guards, typeahead,
and group label relationships.

Before marking `Fully tested: yes`, add broader coverage for controlled
DOM-order registration, typeahead timeout/repeated-character cycling at the
component boundary, rich option-name calculation, and browser-level focus
behavior.
