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

## Remix Renderer Rules

Before changing Remix renderer integration, read the installed Remix skill:

```text
internal/pkg/npm/node_modules/@remix-run/cli/bootstrap/.agents/skills/remix/SKILL.md
```

Use `remix/ui` for renderer, JSX, event, ref, `mix`, `css`, and test APIs. Do
not read, copy, wrap, import, or depend on Remix first-party component subpaths
when implementing Vorma components.

## Remix Styling

Vorma Remix components use Remix's `mix` model as the styling and behavior
composition layer. Component recipes should compile to Remix `css(...)` mixins,
and consumer `mix` should compose with those mixins on the relevant host.

Do not add a parallel component style runtime or custom stylesheet manager for
Remix components. If Remix `css(...)` has a behavior that affects Vorma, design
the component/API around that Remix contract instead of bypassing it.

Do not use Remix's fixed `theme` contract as Vorma's design source of truth.
Vorma recipes should use Vorma tokens. Remix first-party themed components and
`theme.*` values are off limits for Vorma component implementation unless the
user explicitly reopens that decision.

Remix `css(...)` emits cascade-layered rules. Global CSS and resets in apps that
consume these components must use deliberate cascade-layer discipline; unlayered
global rules are outside the component styling contract and can override layered
component rules.

## Component Behavior Ownership

Remix is the renderer and style substrate for this package. Vorma owns the
component contracts, rendered anatomy, state model, callback timing,
accessibility behavior, form semantics, and tests.

Do not use Remix's first-party components or behavior components as a foundation
for Vorma components. They are off limits for Vorma component implementation,
API design, behavior, accessibility, and tests unless the user explicitly
reopens this decision.

When implementing interactive components, use ordinary web platform semantics
and WAI-ARIA component patterns as the source of truth. Build small internal
behavior utilities when the same primitive behavior repeats, such as
controlled/uncontrolled state, ID relationships, outside interaction, focus
management, list navigation, roving focus, or scroll locking.

Do not import from component subpaths such as `remix/ui/select`,
`remix/ui/popover`, `remix/ui/menu`, `remix/ui/combobox`, `remix/ui/listbox`,
`remix/ui/accordion`, `remix/ui/button`, `remix/ui/separator`,
`remix/ui/breadcrumbs`, or `remix/ui/theme` when implementing Vorma components.
Use web platform semantics, WAI-ARIA component patterns, Vorma recipes, and
Vorma-owned internal utilities instead.

## Component Semantics

Name and shape components according to widely adopted semantics from mature
component systems. Prefer names and behavior that would feel unsurprising to
users of Radix, Base UI, Ariakit, React Aria, Chakra, MUI, Mantine, or similar
libraries.

Keep `COMPONENT_MATRIX.md` current when adding, removing, renaming, researching,
or deeply testing components. It is the living roadmap and quality-status file
for this package.

The matrix is for component families only. Shared factories and helpers, such as
root-component utilities or style coordinators, do not belong in the matrix
unless they are themselves a public component family.

Use only `yes`, `partial`, and `no` status values in `COMPONENT_MATRIX.md`.
`yes` means complete for the current contract, `partial` means started but
incomplete, and `no` means not started. Every `partial` row must have a matching
`<component-name>.notes.md` file using the component name in kebab-case. Do not
add ambiguous status values or extra matrix columns.

Use Open UI research as the tie-breaker when mature component libraries disagree
with web platform vocabulary. This matters especially for native-ish control
concepts such as select options, popup behavior, dialog behavior, and
customizable form-control anatomy.

Do not create generic Vorma components around one consumer's current layout.
When a proposed component only makes sense for one app, keep it in that app.

Do not promote consumer composition patterns into generic components unless they
are widely established semantics for that component type. For example,
collapsible code should be composed by consumers with `details`/`summary` around
`CodeBlock`; generic `CodeBlock` should stay a static code-display primitive.

Use ordinary platform semantics first:

- `Button` renders an actual button by default and must default to
  `type="button"` unless the consumer passes another type.
- Input-shaped components must apply input props and input event mixins to the
  actual input host.
- Selection components must not inherit hidden or delayed value timing from a
  dependency. Decide public value timing through current component research,
  then implement that Vorma contract directly.
- Overlay, menu, select, combobox, popover, tabs, accordion, and listbox
  behavior must follow web platform and WAI-ARIA expectations. Vorma should own
  the contract.
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

- `Slider` top-level props target the native range input.
- `CodeBlock.preProps`, `codeProps`, `headerProps`, and `captionProps` target
  their named slots.
- Consumer `mix` must run on the host implied by the public prop name.

Do not attach consumer input handlers to a wrapper and rely on event bubbling.
`event.currentTarget` must be the element the public API implies.

Provider/context components are not DOM hosts. Never attach consumer or public
API event behavior by passing `mix` to a provider/context component. When a
no-host root component owns callbacks such as `onValueChange` or `onOpenChange`,
store the callback/state in the root component context and attach the actual
event mixins to the real host element that receives or dispatches the event.

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

Core may carry nested style objects as opaque recipe data. The Remix component
layer decides whether keys such as nested selectors, pseudo-elements, and
at-rules are valid Remix CSS and how they are emitted.

When a component has deterministic props, such as `Stack.direction` or
`Grid.align`, route them through the shared component recipe-part coordinator so
base and responsive styles resolve consistently.

Recipe `defaultVariants` are the only place for visual defaults. Do not add
factory options such as `defaultSize`, `defaultTone`, or `defaultVariant`.
Factory options are for behavior, condition mappings, target extensions,
variable names, and similar non-visual-default configuration.

Selector-backed style targets must compose. If multiple logical targets map into
the same host selector, the coordinator should deep-merge the styles rather than
letting target order erase earlier selector styles.

## API Quality

Public prop names must describe the semantic behavior, not the implementation.
Use established names such as `placeholder`, `indicator`, `selected`,
`disabled`, `value`, `defaultValue`, `onValueChange`, `open`, `defaultOpen`, and
`onOpenChange` when they fit.

Field-like components must own accessible relationships. If a component
coordinates label, help text, error text, and a control, it should provide the
ID/ARIA wiring or a control mixin/helper so consumers do not manually stitch
`htmlFor`, `aria-describedby`, and `aria-invalid`.

Form and settings primitives should support the controlled and uncontrolled
shapes that mature component systems would expect. For value-based controls,
`value` is the controlled source of truth and `defaultValue` is the uncontrolled
initial value.

Avoid duplicate ways to do the same thing. There is no back-compat layer in this
lab package unless explicitly approved before implementation.

All public types and functions need explicit return types where the return is
not trivially inferred from a one-line expression.

Tests should cover behavior boundaries, not just style-object snapshots. Add
focused tests for host ownership, event target semantics, responsive structural
props, selector/state mapping, and type ergonomics whenever a change touches
those contracts.

Mount real Vorma component behavior when the contract is runtime behavior. VNode
or style-object inspection is fine for style compilation, but controlled
updates, mix ownership, progress updates, keyboard behavior, focus behavior,
outside interaction, and event timing need DOM tests.

## Test Organization

Do not put component tests into giant catch-all files. The default is one test
file per component, named `<component>.test.ts`. Keep exported component
families in one family test file when the public API is a family, such as
`table.test.ts`, `description-list.test.ts`, or `list.test.ts`.

Current test groups:

- `component-style.test.ts` covers recipe, condition, selector, responsive, and
  `mix` compilation.
- One file per static or platform primitive covers that component's rendered
  anatomy, semantic host, prop ownership, and accessibility defaults.
- `field.test.ts` covers field relationship wiring.
- `slider.test.ts` covers range input host ownership and range pseudo-target
  styling.
- `layout-coordination.test.ts` covers deterministic layout/style-system
  coordination and generic type preservation.
- `dialog.test.ts`, `popover.test.ts`, and `select.test.ts` cover interactive
  composite behavior.

Interactive component tests should be organized around user-observable
contracts: controlled and uncontrolled state, keyboard behavior, focus behavior,
outside interaction, callback timing, ARIA relationships, form integration, and
consumer `mix` ownership.

Use `test-setup.ts` for shared DOM environment shims only. Do not hide component
fixtures or assertions in broad helpers. A helper belongs in a test file until
multiple files genuinely need the same setup.
