# Menu Notes

## Current WIP Contract

`Menu` is a command menu primitive for actions. It is not a select, listbox,
combobox, navigation menubar, context menu, command palette, or arbitrary list
of interactive content.

The public anatomy is:

- `Root`
- `Trigger`
- `Popup`
- `Item`
- `Separator`
- `Group`
- `GroupLabel`

Root owns open state, close-on-select behavior, item collection, roving focus,
and typeahead:

- `open`, `defaultOpen`, and `onOpenChange`
- `closeOnSelect`, defaulting to enabled
- `loopFocus`
- `typeahead`, defaulting to enabled

The trigger is a native button and uses `aria-haspopup="menu"`, `aria-expanded`,
and `aria-controls`.

The popup uses `role="menu"`. Items are native buttons with `role="menuitem"`.
Disabled items use `aria-disabled` instead of the native `disabled` attribute so
they remain focusable, matching the WAI-ARIA menu pattern.

Keyboard behavior currently covers the non-submenu command-menu shape:

- Trigger `ArrowDown`, `Enter`, and `Space` open and focus the first item.
- Trigger `ArrowUp` opens and focuses the last item.
- Popup/item `ArrowDown` and `ArrowUp` move roving focus.
- Popup/item `Home` and `End` move to first and last item.
- Printable characters typeahead to the next matching item.
- Popup/item `Enter` and `Space` activate the focused item.
- Popup/item `Escape` closes and returns focus to the trigger.
- Popup/item `Tab` closes and allows normal tab navigation.

## Current Test Status

Testing is WIP. The current tests cover trigger open, close-on-select, roving
focus, disabled item focusability, disabled item activation guard, typeahead,
Escape focus return, and static anatomy.

Before marking `Fully built: yes`, research and settle the final public API
against mature systems and APG guidance, especially submenu behavior,
checkbox/radio menu items, context-menu entry, outside interaction,
close-complete timing, orientation, and whether menubar belongs in this
component family or a separate one.

Before marking `Fully tested: yes`, add broader coverage for controlled open
state, looping focus, `Tab` close behavior, item consumer `mix` ownership,
outside interaction, browser-level focus behavior, and submenu behavior if it
belongs in the final contract.
