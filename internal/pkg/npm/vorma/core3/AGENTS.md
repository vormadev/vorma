# Core3 Clean-Room Notes

Core3 is a new clean-room rewrite of Vorma's client core.

It may learn from baseline-core and core2 as evidence, but neither existing
implementation is architectural truth. Core3 is not a public API evolution and
not a test-suite rewrite. It is an internal foundation replacement that must
pass the existing full gate.

Baseline-core is the current behavioral baseline. Core2 is unproven
architectural evidence. Existing internal structure is not an input.

## Non-Negotiables

- Do not copy code directly from baseline-core into core3.
- Do not copy code directly from core2 into core3.
- Do not port core2's reducer branch-by-branch.
- Do not split core2 files into core3 files and call that a rewrite.
- Do not preserve a concept just because either previous core needed it.
- Do not introduce a central imperative runtime that owns route policy through
  closure state.
- Do not create manager-per-concern ceremony to hide unchanged assumptions.
- Do not use generic helpers to disguise missing domain vocabulary.
- Do not renegotiate public behavior.
- Do not create a parallel acceptance test suite.
- Do not treat core2 behavior as authoritative when it differs from the existing
  full-gate baseline.
- Do not treat bundle size as a secondary concern. More bytes must buy clear
  correctness, and the preferred outcome is smaller generated client code.

## What Can Be Learned

Core3 can learn from existing evidence:

- baseline-core behavior is the required behavior target
- existing tests describe required product contracts
- which race conditions previous cores had to survive
- which host effects exist at the browser edge
- which concepts were valuable but poorly placed

Core3 should not reuse implementation shape by default. If a core2 idea still
belongs, restate it in core3 vocabulary first, then implement it from that new
model.

## Starting Bet

Core3 should use explicit domain transitions instead of a mutable closure
router. It should also avoid making a single dense reducer the only place where
domain meaning can live.

The new center should be:

```ts
operation + classified outcome + publication transaction -> next model + effects
```

The reducer is not automatically the product this time. The product is the
domain algebra that makes route work, response handling, invalidation,
publication, and settlement explicit.

## Truth Sources

Behavior truth:

- the existing full gate
- the current public API contract
- user-observable route, history, DOM, scroll, revalidation, API, and work
  behavior
- baseline-core behavior for the current product contract

Architecture truth:

- explicit core3 model notes, only where they do not conflict with behavior
  truth
- bundle-size discipline, especially avoiding runtime vocabulary or abstraction
  layers that exist only to make code read nicely

Treat these as evidence only:

- baseline-core implementation structure
- core2 behavior and implementation structure
- incidental command ordering
- helper names
- file boundaries
- temporary shims

## First Implementation Rule

The first implementation artifact should describe core3's domain model before
any browser shell, host, or compatibility adapter exists.

If the first meaningful file looks like `create_client_core`, `runtime`,
`kernel`, or a port of `update.ts`, core3 has already drifted.

## Core Documents

- [MODEL.md](./MODEL.md) defines the forward domain model.
- [ROADMAP.md](./ROADMAP.md) defines the implementation sequence.
- [IDEAS.md](./IDEAS.md) defines the big picture philosophy.
