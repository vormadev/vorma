# Select Notes

## Fully Built Contract

`Select` is a single-value, select-only combobox/listbox primitive. It is for
choosing one predefined option from a closed set. It is not a text-entry
combobox, autocomplete, multi-select, tag picker, command palette, or
virtualized collection.

The public anatomy is:

- `Root`
- `Trigger`
- `Value`
- `Icon`
- `Popup`
- `List`
- `Group`
- `GroupLabel`
- `Separator`
- `Option`
- `OptionText`
- `OptionIndicator`

Root owns value, open state, highlighted option state, form participation, and
callback timing:

- `value`, `defaultValue`, and `onValueChange`
- `open`, `defaultOpen`, `onOpenChange`, and `onOpenChangeComplete`
- `highlightedValue`, `defaultHighlightedValue`, and `onHighlightChange`
- `name`, `form`, `required`, `disabled`, `readOnly`, `invalid`, and
  `autoComplete`
- `placeholder`
- `loopFocus`, defaulting to no wrapping
- `typeahead`, defaulting to enabled

The trigger is the semantic focus target. It uses combobox semantics,
`aria-controls`, `aria-expanded`, `aria-haspopup="listbox"`, and
`aria-activedescendant` while the popup is open and an option is highlighted.
DOM focus remains on the trigger while navigating the popup.

The popup contains a `listbox`; options use `role="option"` and expose selected,
highlighted, and disabled state through ARIA and data attributes. Groups and
group labels must use ordinary listbox group semantics.

The component renders a visually hidden native `select` when `name` or `form` is
provided, so form submission and native validation use actual select semantics
instead of a fake hidden input.

Keyboard behavior follows the select-only combobox pattern:

- Closed trigger:
    - `ArrowDown`, `Enter`, and `Space` open without changing value.
    - `ArrowUp` opens and highlights the last enabled option.
    - `Home` opens and highlights the first enabled option.
    - `End` opens and highlights the last enabled option.
    - Printable characters open and typeahead to the matching option.
- Open trigger:
    - `ArrowDown` and `ArrowUp` move visual focus.
    - `Home` and `End` move to first and last enabled option.
    - `PageDown` and `PageUp` jump by a page-sized amount.
    - Printable characters typeahead.
    - `Enter` and `Space` commit the highlighted option and close.
    - `Tab` commits the highlighted option, closes, and allows normal tabbing.
    - `Escape` closes without committing.

Pointer selection commits immediately. Pointer movement highlights enabled
options. Disabled options cannot be highlighted or selected.

## Current Test Status

Testing is WIP. The current tests cover ownership/anatomy, controlled and
uncontrolled value, immediate option commit, disabled option guard, outside
close, open completion, keyboard commit/cancel, form mirroring, highlight
control, focus target semantics, non-wrapping navigation, optional wrapping, and
typeahead.

Before marking `Fully tested: yes`, add broader browser-level coverage for
native popover interaction, focus restoration in a real browser, form
validation, and assistive-technology-sensitive ARIA relationships.
