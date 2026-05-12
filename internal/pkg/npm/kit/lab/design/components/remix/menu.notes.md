# Menu Notes

## Fully Built Contract

`Menu` is a dropdown command-menu primitive for actions and action-like choices.
It is not a select, listbox, combobox, navigation menubar, context menu, command
palette, submenu system, or arbitrary list of interactive content.

`Menubar`, `ContextMenu`, and nested submenu behavior are separate component
families. Do not widen `Menu` to absorb them.

The public anatomy is:

- `Root`
- `Trigger`
- `Popup`
- `Item`
- `CheckboxItem`
- `RadioGroup`
- `RadioItem`
- `ItemIndicator`
- `Separator`
- `Group`
- `GroupLabel`

Root owns open state, close-on-select behavior, item collection, roving focus,
and typeahead:

- `open`, `defaultOpen`, and `onOpenChange`
- `onOpenChangeComplete`
- `closeOnSelect`, defaulting to enabled
- `loopFocus`
- `typeahead`, defaulting to enabled

The trigger is a native button and uses `aria-haspopup="menu"`, `aria-expanded`,
and `aria-controls`.

The popup uses `role="menu"`. Items are native buttons with `role="menuitem"`.
Disabled items use `aria-disabled` instead of the native `disabled` attribute so
they remain focusable, matching the WAI-ARIA menu pattern.

Checkbox and radio items use `role="menuitemcheckbox"` and
`role="menuitemradio"`, expose `aria-checked`, and share the item slot so recipe
authors can style all menu item kinds consistently. `ItemIndicator` provides a
styled inline indicator slot for checkbox and radio items.

Keyboard behavior covers the non-submenu command-menu shape:

- Trigger `ArrowDown`, `Enter`, and `Space` open and focus the first item.
- Trigger `ArrowUp` opens and focuses the last item.
- Popup/item `ArrowDown` and `ArrowUp` move roving focus.
- Popup/item `Home` and `End` move to first and last item.
- Printable characters typeahead to the next matching item.
- Popup/item `Enter` and `Space` activate the focused item.
- Popup/item `Escape` closes and returns focus to the trigger.
- Popup/item `Tab` closes and allows normal tab navigation.
- Outside pointer interaction closes the popup.

## Current Test Status

Testing is WIP. The current tests cover trigger open, close-on-select, roving
focus, disabled item focusability, disabled item activation guard,
checkbox/radio item behavior, typeahead, Escape focus return, outside
interaction, controlled open state, completed open changes, `Tab` close
behavior, and static anatomy.

Before marking `Fully tested: yes`, add broader coverage for looping focus, item
consumer `mix` ownership, browser-level focus behavior, custom trigger IDs,
controlled checkbox/radio state, and recipe condition output for highlighted and
checkable states.
