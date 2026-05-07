# Vorma Remix Components

This package is a generic Vorma design-system component layer for Remix UI. It
must not encode consumer-specific product requirements.

This package is primitive infrastructure for consumer-owned design systems, not
a closed finished UI system. Consumers are expected to wrap these primitives and
narrow their own public APIs at their boundary.

Vorma components must not become a styling boundary. Do not frame styling
support as a positive list of things Vorma permits. Named parts, slots, and
targets are conveniences for common design-system authoring, not permissions. If
CSS can reach something from a rendered host, consumers must have a clean way to
style it without waiting for Vorma to name that surface.

## Remix References

Before changing Remix component behavior, read the installed Remix skill:

```text
internal/pkg/npm/node_modules/@remix-run/cli/bootstrap/.agents/skills/remix/SKILL.md
```

For UI-specific APIs, inspect the installed package source:

```text
internal/pkg/npm/node_modules/@remix-run/ui/src
internal/pkg/npm/node_modules/@remix-run/ui/src/components
internal/pkg/npm/node_modules/remix/src/ui
```

Use `remix/ui` and `remix/ui/*` imports in Vorma source. The `remix/src/ui/*`
files are wrapper exports for the public subpaths.

## Remix UI Surface

The first-class Remix UI subpaths currently include:

- `remix/ui`
- `remix/ui/accordion`
- `remix/ui/anchor`
- `remix/ui/animation`
- `remix/ui/breadcrumbs`
- `remix/ui/button`
- `remix/ui/combobox`
- `remix/ui/glyph`
- `remix/ui/listbox`
- `remix/ui/menu`
- `remix/ui/popover`
- `remix/ui/scroll-lock`
- `remix/ui/select`
- `remix/ui/separator`
- `remix/ui/server`
- `remix/ui/test`
- `remix/ui/theme`

Prefer the lower-level behavior primitives and mixins that Remix exposes, such
as `popover.Context`, `popover.anchor`, `popover.surface`, `select.Context`,
`select.trigger`, `select.list`, `select.option`, `tabs.trigger`, or
`combobox.input`.

Do not wrap Remix's finished themed components as the foundation for this
package. Components such as Remix `Button`, `Breadcrumbs`, `Select`, `Combobox`,
`Tabs`, `Accordion`, and `Menu` may be useful references, but their complete
wrappers and `*Style` exports are tied to Remix's own theme contract. Vorma
components should own structure and Vorma recipe styles while reusing Remix
behavior primitives where those primitives exist.

Some Remix subpaths are utilities rather than reusable behavior substrates:

- `remix/ui/button` mostly provides Remix-theme button styles plus a convenience
  wrapper. Do not use it for Vorma Button unless Remix exposes a behavior-only
  primitive.
- `remix/ui/separator` exposes only Remix-theme separator styling. Vorma
  Separator should remain recipe-owned.
- `remix/ui/glyph` is a named sprite/glyph-sheet system tied to Remix's glyph
  contract. Vorma Icon should remain a generic icon wrapper unless Vorma
  explicitly adds its own glyph-sheet contract.
- `remix/ui/breadcrumbs` is a complete themed component, not a behavior
  primitive.

## Component Semantics

Name and shape components according to widely adopted semantics from mature
component systems. Prefer names and behavior that would feel unsurprising to
users of Radix, Base UI, Ariakit, React Aria, Chakra, MUI, Mantine, or similar
libraries.

Do not create generic Vorma components around one consumer's current layout.
When a proposed component only makes sense for one app, keep it in that app.

Use ordinary platform semantics first:

- `Button` renders an actual button by default and must default to
  `type="button"` unless the consumer passes another type.
- Input-shaped components must apply input props and input event mixins to the
  actual input host.
- Selection components should emit normal value-change callbacks as soon as the
  user selects the value. If an underlying Remix primitive also has meaningful
  value-change-driven close timing, expose that as an explicit value-change
  close callback instead of making the ordinary change callback feel delayed.
- Overlay, menu, select, combobox, popover, tabs, accordion, and listbox
  behavior should reuse Remix's behavior primitives when available.
- Dialog-style overlays/backdrops must remain first-class styling surfaces.
  Native pseudo-elements such as `dialog::backdrop` may back those surfaces, but
  consumers should still author them as ordinary named recipe slots.
- Static layout and typography components should stay simple and recipe-driven.

## Slot Props And Mix Ownership

Every host element that receives Vorma anatomy attributes and recipe mix should
be assembled through `createComponentSlotProps`.

Every real host must preserve a consumer `mix` escape hatch. System mix should
come first and consumer mix should come last so instances can extend or override
primitive styling.

Top-level props belong to the component's semantic primary host. If the primary
host is not the root element, expose explicit slot props for the other hosts.

Examples:

- `RangeField` top-level props target the internal range input.
- `RangeField.rootProps`, `labelProps`, and `valueProps` target those slots.
- `CodeBlock.preProps`, `codeProps`, `summaryProps`, and `captionProps` target
  their named slots.
- Consumer `mix` must run on the host implied by the public prop name.

Do not attach consumer input handlers to a wrapper and rely on event bubbling.
`event.currentTarget` must be the element the public API implies.

Remix provider/context components are not DOM hosts. Never attach consumer or
public API event behavior by passing `mix` to a provider/context component such
as `select.Context` or `popover.Context`. When a no-host root component owns
callbacks such as `onValueChange` or `onOpenChange`, store the callback/state in
the root component context and attach the actual event mixins to the real host
element that receives or dispatches the event.

## Recipes And Conditions

Vorma recipes own renderer-neutral slots, variants, states, and style objects.
Remix components own mapping those renderer-neutral state names to concrete
selectors, attributes, mixins, and DOM behavior.

Keep the factory/recipe styling plane and the end component/instance styling
plane distinct. Factory recipes provide reusable design-system styling through
named targets. Instance host props and `mix` provide open per-use styling for
anything CSS can reach from that host.

Do not add selector knowledge to `design/core`. Do not add consumer-specific
state names to a component just to satisfy one app. State and slot names must be
appropriate for the generic component contract.

When a component has deterministic props, such as `Stack.direction` or
`Grid.align`, route them through the shared component recipe-part coordinator so
base and responsive styles resolve consistently.

## API Quality

Public prop names must describe the semantic behavior, not the implementation.
Use established names such as `placeholder`, `indicator`, `selected`,
`disabled`, `value`, `defaultValue`, `onValueChange`, `open`, `defaultOpen`, and
`onOpenChange` when they fit.

Avoid duplicate ways to do the same thing. There is no back-compat layer in this
lab package unless explicitly approved before implementation.

All public types and functions need explicit return types where the return is
not trivially inferred from a one-line expression.

Tests should cover behavior boundaries, not just style-object snapshots. Add
focused tests for host ownership, event target semantics, responsive structural
props, selector/state mapping, and type ergonomics whenever a change touches
those contracts.
