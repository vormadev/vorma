# Interaction Primitives Plan

This plan tracks the lower-level behavior primitives that should support hard
Vorma Remix components. Keep this file current as work moves. Do not leave
completed or abandoned steps ambiguous after implementation.

The goal is an intentional internal interaction layer. Select already exposed
enough pressure to know the broad primitive families we need. Do not wait for
every hard component to independently rediscover the same problems.

This layer should still stay small, boring, and component-semantic. It should
make component behavior explicit, testable, and hard to wire incorrectly without
forcing every component through one generic public abstraction.

This is also a browser-bundle concern. Hard interactive components can become
large quickly. Shared behavior primitives should prevent each component from
shipping its own duplicate collection registry, keyboard navigation, typeahead,
popup handling, form mirroring, and state machinery.

## Design Rules

- Vorma owns component behavior. Remix is the renderer, event, ref, `mix`, and
  `css` substrate.
- Web platform semantics and WAI-ARIA patterns are the source of truth for hard
  interactive components.
- Shared primitives should be designed from the known hard-component families:
  `Select`, `Listbox`, `Menu`, `Combobox`, `Tabs`, `RadioGroup`, `Popover`, and
  `Dialog`.
- Component-specific machines are allowed and probably desirable for hard
  widgets.
- Do not force every component through one generic state machine vocabulary, but
  do not avoid a machine-shaped internal design merely because it is more
  architectural than one-off local state.
- Keep public component APIs researched against mature systems before locking
  them in.
- Keep host ownership explicit: the public prop name must match the real DOM
  host receiving props, events, refs, and `mix`.
- Keep tests close to the primitive/component boundary they prove.
- Treat duplicated interaction code as bundle cost, not only maintenance cost.
  Prefer a shared internal primitive when the behavior is already known to span
  multiple hard components.

## Target Shape

The preferred architecture for hard widgets is:

```text
component-specific machine
  -> shared primitives for state, collection, navigation, popup, typeahead, form
  -> Remix component anatomy and host wiring
```

The machine layer should be pure or nearly pure where practical:

```text
state + event -> next state + effects
```

The Remix component layer should own DOM effects: focus, refs, native popover,
scrolling, form hosts, outside interaction listeners, and `mix` composition.

## Candidate Primitives

### Controllable State

- [x] Add a small internal controllable-state primitive now.
- [x] Support `value`, `defaultValue`, and change callback patterns.
- [x] Support explicit change details without hiding the event source.
- [x] Move `Select` value, open, and highlighted state onto it.
- [x] Validate and adjust it while building `Listbox` and `Menu`.

Expected consumers:

- value state: `Select`, `Listbox`, `Combobox`, `RadioGroup`, `CheckboxGroup`,
  `Tabs`
- open state: `Select`, `Menu`, `Popover`, `Dialog`, `Combobox`
- highlighted/current state: `Select`, `Listbox`, `Menu`, `Combobox`

### Ordered Collection

- [x] Add an internal ordered collection registry now.
- [x] Preserve DOM order, not registration order.
- [x] Support stable item IDs, values, disabled state, text values, and host
      nodes.
- [x] Provide lookup by value, first/last enabled item, next/previous enabled
      item, and page jumps.
- [ ] Keep group metadata possible without making every collection grouped.
- [x] Move `Select` option registration/navigation onto it.
- [x] Validate and adjust it while building `Listbox` and `Menu`.

Expected consumers:

- `Select`
- `Listbox`
- `Menu`
- `Combobox`
- `Tabs`
- `RadioGroup`
- `Accordion`
- `CheckboxGroup`

### Typeahead

- [x] Add an internal typeahead primitive now.
- [x] Support buffered matching, repeated-character cycling, timeout reset, and
      disabled item skipping.
- [x] Keep matching over explicit `textValue` first, then DOM text fallback.
- [x] Make typeahead optional for components where it is not always desired.
- [x] Move `Select` typeahead onto it.
- [ ] Validate and adjust it while building `Combobox`; `Listbox` and `Menu` now
      exercise it.

Expected consumers:

- `Select`
- `Listbox`
- `Menu`
- `Combobox`

### Composite Navigation

- [x] Add navigation helpers over ordered collections now.
- [x] Support active-descendant focus for select/listbox/combobox-style widgets.
- [x] Support roving-tabindex focus for menu/tabs/toolbar/radio-style widgets.
- [ ] Support vertical, horizontal, and bidirectional orientation where the
      component pattern needs it.
- [x] Support non-wrapping defaults and explicit opt-in wrapping.
- [x] Keep disabled-item skipping consistent.
- [x] Move `Select` active-descendant navigation onto it.
- [x] Validate active-descendant behavior with `Listbox`.
- [x] Validate roving-tabindex behavior with `Menu` or `Tabs`.

Expected consumers:

- active descendant: `Select`, `Listbox`, `Combobox`
- roving tab index: `Menu`, `Tabs`, `Toolbar`, `RadioGroup`

### Popup Behavior

- [x] Extract shared native popover sync and target-containment helpers.
- [x] Add a small popup relationship primitive for non-modal popup behavior.
- [x] Support trigger/content refs, outside pointer interaction, and native
      popover sync where appropriate.
- [x] Keep Escape close and open-complete callbacks component-specific because
      their reason semantics differ by component.
- [ ] Keep modality, scroll lock, overlay, and focus trapping separate from
      generic popup behavior.
- [ ] Do not make select/listbox/menu/dialog semantics share one public API just
      because all can open and close.
- [x] Move the non-modal `Select` popup behavior onto it.
- [x] Validate and adjust it while building `Menu` and `Popover`.
- [ ] Validate and adjust it while building `Combobox`.

Expected consumers:

- `Select`
- `Menu`
- `Popover`
- `Combobox`
- `Dialog`
- `AlertDialog`

### Form Mirror

- [x] Add an internal form-mirror primitive now for custom form controls.
- [x] Support visually hidden native `select` for select-like controls.
- [ ] Support hidden inputs for non-select controls where native mirrors are not
      possible.
- [ ] Support `name`, `form`, `required`, `disabled`, `autoComplete`, and
      current value.
- [ ] Keep native validation behavior in mind before marking this settled.
- [x] Move `Select` hidden native select rendering onto it.
- [ ] Validate and adjust it with `RadioGroup`, `CheckboxGroup`, `Slider`, and
      `Switch`. `RadioGroup` and `CheckboxGroup` are native-input backed and do
      not need a hidden mirror.

Expected consumers:

- `Select`
- `Listbox` when form-backed
- `CheckboxGroup`
- `RadioGroup`
- `Slider`
- `Switch`

### Group Labelling

- [x] Add an internal group-label relationship helper now.
- [x] Support generated label IDs and `aria-labelledby` on groups.
- [x] Avoid requiring group labels when an unlabelled group is valid for the
      pattern.
- [x] Move `Select`, `Listbox`, and `Menu` group labelling onto it.
- [ ] Validate and adjust it with grouped form controls. `RadioGroup` and
      `CheckboxGroup` preserve native input form participation; explicit group
      labelling remains a separate `Fieldset`/composition concern.

Expected consumers:

- `Select`
- `Listbox`
- `Menu`
- `CheckboxGroup`
- grouped navigation components

### State And Data Attributes

- [x] Consolidate common state/data attribute helpers now where the vocabulary
      is already clear.
- [x] Keep public data attributes unsurprising: `data-state`, `data-disabled`,
      `data-selected`, `data-highlighted`, `data-invalid`, `data-readonly`, and
      `data-required`.
- [x] Keep ARIA and data attributes aligned but not conflated.
- [x] Move `Select`, `Listbox`, and `Menu` common state/data attribute
      construction onto it where that improves clarity.

Expected consumers:

- most interactive components

## Extraction Sequence

- [x] Build one hard component deeply enough to expose real pressure.
- [x] Create the first internal primitives for controllable state, ordered
      collection, typeahead, and active-descendant navigation, starting from the
      behavior already implemented in `Select`.
- [x] Move `Select` onto those first primitives without changing its public
      contract.
- [x] Build or harden `Listbox` next to validate active-descendant collection
      behavior.
- [x] Build or harden `Menu` next to validate roving focus, typeahead, and popup
      behavior.
- [ ] Refine the primitives after `Listbox` and `Menu` expose real differences.
- [x] Add primitive-level tests for the first state, collection, typeahead, and
      navigation primitives.
- [ ] Keep component-level DOM tests for behavior involving focus, events, refs,
      popover, form hosts, or rendered ARIA.

## Completion Criteria

This plan is complete when hard interactive components can share low-level
behavior without hiding their component-specific semantics.

Do not mark this complete until:

- [x] `Select`, `Listbox`, and `Menu` use the shared collection/navigation
      primitives where appropriate.
- [x] Popup/open behavior has either been extracted or deliberately kept local
      with a written reason.
- [ ] Form mirroring has either been extracted or deliberately kept local with a
      written reason.
- [ ] Tests prove both the shared primitives and the component-specific DOM
      contracts.
- [ ] `COMPONENT_MATRIX.md` and the relevant component notes files reflect the
      actual state.
