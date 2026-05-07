# Vorma Remix Components API Plan

## Primitive Boundary

This package is a low-level component foundation for consumer-owned design
systems. It is not a closed, finished UI system.

Vorma components must not become a styling boundary. Consumer design systems
should be able to build their own wrapped components on top of these primitives
and narrow the API there. This package should preserve platform reach.

Do not frame styling support as a positive list of things Vorma allows. Named
parts and targets are conveniences for common design-system authoring, not
permissions. If CSS can reach something from a rendered host, consumers must
have a clean way to style it without waiting for Vorma to name that surface.

## Two Styling Planes

There are two separate styling planes and both matter.

1. Factory and recipe plane.

This plane is for consumer design-system authors. It defines reusable component
styling once when calling factories like `createButton`, `createRangeField`, or
`createSelect`.

Recipes should support named style targets for component anatomy and useful
virtual surfaces. These targets make common design-system styling reusable and
consistent.

2. End component and instance plane.

This plane is for each actual component usage. Every real host element must
expose props that include consumer `mix`. Instance-level `mix` is the open
escape hatch and must not be limited by named recipe targets.

Consumer `mix` must be merged after system mix so the instance can override or
extend the primitive's default styling.

## Style Targets

The current helper model treats component parts mostly as rendered recipe slots.
That is too narrow.

The Remix component layer should model style targets. A style target can be:

- A real rendered host, such as `root`, `input`, `trigger`, or `surface`.
- A selector target rooted at a real host, such as `&:focus-visible`.
- A native pseudo-element target, such as `&::-webkit-slider-thumb`.
- A native browser part, pseudo-class, descendant, or future selector surface.
- A component-generated selector surface backed by data attributes, ARIA, or
  Remix behavior primitives.

Core recipes remain selector-free. The Remix component layer maps recipe target
names to actual CSS selectors and host mixes.

Example target model:

```ts
targets: {
	root: { host: "root" },
	input: { host: "input" },
	thumb: {
		host: "input",
		selectors: [
			"&::-webkit-slider-thumb",
			"&::-moz-range-thumb",
		],
	},
}
```

The recipe author can target `thumb` once for reusable design-system styling.
The component user can still use `inputProps.mix` with any selector CSS
supports, whether or not Vorma has a named `thumb` target.

## Host Props

Every rendered host that receives Vorma anatomy attributes or system mix should
have an explicit prop surface unless the component's top-level props already
target that host.

Examples:

- If top-level props target an internal `input`, expose `rootProps`,
  `labelProps`, and other host props separately.
- If top-level props target a `button`, expose named props for any additional
  rendered hosts.
- Do not attach consumer event handlers or mix to a wrapper when the public API
  implies a different host.

The invariant is: the host implied by the prop name is the host that receives
the props and `mix`.

## Conditions

Recipe conditions are renderer-neutral names. Component factories provide
default selector mappings for known generic conditions, but the system must not
be artificially closed over those names.

Factories should allow consumer design systems to add condition mappings per
target or host. If a recipe uses a condition that has no selector mapping in the
component layer, fail clearly.

Do not add selector knowledge to `design/core`.

## Implementation Direction

Use `createComponentStyleTargets` as the target compiler that:

- Accepts real-host targets and selector-backed virtual targets.
- Resolves base, variant, condition, and responsive recipe styles for every
  target.
- Groups virtual target styles into the mix for their real host.
- Preserves consumer host `mix` after generated system mix.
- Supports component-specific deterministic styles in the same target pipeline.
- Supports component-factory extensions for additional targets and condition
  mappings.

The output should be host-oriented because virtual targets compile into real
hosts:

```ts
const hosts = createComponentStyleTargets(...);

createComponentSlotProps({
	attrs: ...,
	mix: hosts.input.mix,
	props: input_props,
});
```

## Tests

Tests for this architecture should prove behavior, not just snapshots.

Cover:

- Real host target styles compile to the host mix.
- Selector-backed virtual target styles compile into their host mix.
- Responsive styles work for selector-backed targets.
- Condition styles work for selector-backed targets.
- Consumer `mix` is preserved after system mix.
- Consumer custom condition mappings work.
- Missing condition mappings fail clearly.
- Top-level and slot props land on the host implied by the public API.
