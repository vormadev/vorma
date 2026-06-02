# Maintainer Specs

This directory contains maintainer-intent specs.

These specs should describe the system at the level of features, invariants, and semantic
boundaries. They should not read like API reference docs, implementation walkthroughs,
source maps, test maps, changelogs, or exhaustive catalogs of observable behavior.

The goal is to put the maintainer's model of the system into durable written form: what
matters, why it matters, and what kinds of changes would break the spirit of the system
even if the local implementation details changed.

## How To Read These Specs

Read these specs for intent. They are meant to answer questions like:

- What is this area responsible for?
- What behavior is load-bearing?
- What would count as a conceptually wrong change?
- What distinctions matter even if they are not obvious from one local file?
- What should remain true after a refactor?

Do not read these specs as a list of current function names, file names, data structures,
or implementation steps. Stable public names can appear when they are truly part of the
contract, but private symbols should not carry the explanation.

## How To Write These Specs

Write sections as feature-level invariants with enough detail to guide a real change.

A useful section usually does this:

1. Names the invariant.
2. Describes the scenario where the invariant matters.
3. States what must remain true.
4. Explains why the invariant matters.
5. Describes what would violate it.

Use whatever format makes the meaning clearest. Bullets are fine. Paragraphs are fine. The
point is not prose style. The point is whether the section makes real claims about the
system.

Avoid empty inventories. A paragraph can be just as empty as a bullet list if it only
names nearby concerns. A reader should come away knowing what the system cares about, not
merely which nouns are involved.

Avoid overfitting a section to the current implementation. If it would become misleading
after a behavior-preserving refactor, it is probably too tied to implementation shape.

## Empty Example

### Shipment Flow

- Allocation.
- Packing.
- Labels.
- Tracking.
- Returns.

This is too shallow because it only names topics. It does not say what must remain true,
why it matters, or what would count as breaking the shipment model.

## Useful Example

### Shipment Ownership

- Only the shipment that owns packed inventory may buy a label for that inventory.
- Re-rating may change carrier or service before label purchase, but it must not silently
  split inventory ownership.
- Canceling a label releases shipment ownership only after carrier cancellation is known
  or the label is marked unrecoverable.

## Empty Example

### Preview And Export

Preview uses the browser renderer. Export uses the PDF renderer. They should mostly match.

This gestures at a relationship, but "mostly match" does not state a usable invariant.

## Useful Example

### Approved Preview Defines Export Semantics

- Export must preserve the document semantics shown in the approved preview.
- Delivery-specific details may differ, but page breaks, content visibility, numbering,
  fonts, and image placement must not.
- Editor-only overlays may appear in preview and file packaging may differ in export, but
  neither mode may change the document being represented.
