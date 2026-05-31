---
name: thermo-nuclear-system-review
description:
    Run an extremely strict maintainability review for abstraction quality, giant files,
    and spaghetti-condition growth. Use for a thermo-nuclear code quality review,
    thermonuclear review, deep code quality audit, or especially harsh maintainability
    review.
disable-model-invocation: true
---

# Thermo-Nuclear Code Quality Review

Use this skill for an unusually strict review focused on implementation quality,
maintainability, abstraction quality, and codebase health.

Above all, this skill should push the reviewer to be **ambitious** about code structure.
Do not merely identify local cleanup opportunities. Actively search for "code judo" moves:
restructurings that preserve behavior while making the implementation dramatically
simpler, smaller, more direct, and more elegant.

## Core Prompt

Start from this baseline:

> Perform a deep code quality and system design quality audit of the system in question.
> Rethink how to structure / implement the design to meaningfully improve code quality
> without impacting behavior (though you should always correct obvious bugs). Work to
> improve abstractions, modularity, succinctness, and legibility, and to reduce spaghetti
> code. Be ambitious; if there is a clear path to improving the implementation that
> involves restructuring some of the codebase, go for it. Be extremely thorough and
> rigorous. Measure twice, cut once. Also consider code organization and boundaries; if
> anything in the code would be embarrassing if reviewed by someone you respect, then it
> should be fixed and improved until such point that we are proud of it.

## Non-Negotiable Additional Standards

Apply the baseline prompt above, plus these explicit review rules:

1. **Be ambitious about structural simplification.**
    - Do not stop at "this could be a bit cleaner."
    - Look for opportunities to reframe the code/system such that whole branches, helpers,
      modes, conditionals, or layers disappear entirely.
    - Prefer the solution that makes the code/system feel inevitable in hindsight.
    - Assume there is often a "code judo" move available: a re-organization that uses the
      existing architecture more effectively and/or makes the code/system dramatically
      simpler and more elegant.
    - If you see a path to delete complexity rather than rearrange it, push hard for that
      path.

1. **Bias toward cleaning the design, not just accepting working code.**
    - If behavior can stay the same while the structure becomes meaningfully cleaner, push
      for the cleaner version.
    - Do not rubber-stamp "it works" implementations that leave the codebase messier.
    - Strongly prefer simplifications that remove moving pieces altogether over refactors
      that merely spread the same complexity around.

1. **Prefer direct, boring, maintainable code over hacky or magical code.**
    - Treat brittle, ad-hoc, or "magic" behavior as a code-quality problem.
    - Be skeptical of generic mechanisms that hide simple data-shape assumptions.
    - Flag thin abstractions, identity wrappers, or pass-through helpers that add
      indirection without buying clarity.

1. **Push hard on type and boundary cleanliness when they affect maintainability.**
    - Question unnecessary optionality, `unknown`, `any`, or cast-heavy code when a
      clearer type boundary could exist.
    - Prefer explicit typed models or shared contracts over loosely-shaped ad-hoc objects.
    - If a branch relies on silent fallback to paper over an unclear invariant, ask
      whether the boundary should be made explicit instead (the answer is usually "yes").
    - Failing fast with clear contracts is better than being forgiving; the "be liberal
      with your inputs" common "wisdom" is actually stupid, so don't do that. Consumers
      should be forced to interact with the system correctly.

1. **Keep logic in the canonical layer and reuse existing helpers.**
    - Call out feature logic leaking into shared paths or implementation details leaking
      through APIs.
    - Prefer existing canonical utilities/helpers over bespoke one-offs.
    - Push code toward the right package, service, or module instead of normalizing
      architectural drift.
    - Do your best to keep the entire system DRY, within reason.

1. **Treat unnecessary sequential orchestration and non-atomic updates as design smells
   when the cleaner structure is obvious.**
    - If independent work is serialized, ask whether the flow should run in parallel
      instead.
    - If related updates can leave state half-applied, push for a more atomic structure
      (atomic in the all-or-nothing db-transaction sense, not the "smallest possible
      module/component" sense).
    - Do not over-index on micro-optimizations, but do flag avoidable orchestration
      complexity that makes the implementation more brittle.

## Primary Review Questions

Always ask:

- Is there a "code judo" move that would make the system or any component dramatically
  simpler?
- Can this system be reframed so fewer concepts, branches, or helper layers are needed?
- Is all logic living in the right file and layer?
- Are there repeated conditionals that signal a missing model or missing helper?
- Is the implementation direct and legible, or does it rely on special cases and
  incidental control flow?
- Are all abstractions actually earning their keep, or are they just low-value wrappers?
- Is all logic living in its natural canonical layer, or are details leaking across a
  boundary?
- Is any orchestration more sequential or less atomic/transactional than it needs to be?

## What to Flag Aggressively

Escalate findings when you see:

- A complicated implementation where a cleaner reframing could delete whole categories of
  complexity.
- Refactors that move code around but fail to reduce the number of concepts a reader must
  hold in their head.
- Suspicious conditionals bolted onto unrelated code paths.
- One-off booleans, nullable modes, or flags that complicate control flow.
- Feature-specific logic leaking into general-purpose modules.
- Generic "magic" handling that hides simple structure and makes the code harder to reason
  about.
- Thin wrappers or identity abstractions that add indirection without simplifying
  anything.
- Unnecessary casts, `any`, `unknown`, or optional params that muddy the real contract.
- Copy-pasted logic instead of extracted helpers.
- Narrow edge-case handling implemented in the middle of an already busy function.
- Refactors that technically pass tests but make the code less modular or less readable.
- "Temporary" branching that is likely to become permanent debt.
- Bespoke helpers where the codebase already has a canonical utility for the job.
- Logic in the wrong layer/package when it should live somewhere more central.
- Sequential async flow where obviously independent work could stay simpler and clearer
  with parallel execution.
- Partial-update logic that leaves state less atomic/transactional than necessary.

## Preferred Remedies

When you identify a code-quality problem, prefer suggestions like:

- Delete a whole layer of indirection rather than polishing it.
- Reframe the state model so conditionals disappear instead of getting centralized.
- Change the ownership boundary so the feature becomes a natural extension of an existing
  abstraction.
- Turn special-case logic into a simpler default flow with fewer exceptions.
- Extract a helper or pure function (but don't create pointless single-use abstractions
  that merely add indirection with no true benefit).
- Split a large file into smaller focused modules.
- Move feature-specific logic behind a dedicated abstraction.
- Replace condition chains with a typed model or explicit dispatcher.
- Separate orchestration from business logic.
- Collapse duplicate branches into a single clearer flow.
- Delete wrappers that do not meaningfully clarify the API.
- Reuse the existing canonical helper instead of introducing a near-duplicate.
- Make type boundaries more explicit so the control flow gets simpler.
- Move the logic to the package/module/layer that already owns the concept.
- Parallelize independent work (although do not do this for already-fast CPU work; this
  advice is for any type of i/o or build system work or similar performance candidates).
- Restructure related updates into a more atomic/transactional flow when partial state
  would be harder to reason about.

Do not be satisfied with "maybe rename this" feedback when the real issue is structural.
Do not be satisfied with a merely cleaner version of the same messy idea if there is a
plausible path to a much simpler idea.

## Output Expectations

Prioritize findings in this order:

1. Structural code-quality regressions
2. Missed opportunities for dramatic simplification / code-judo restructuring
3. spaghetti / branching complexity increases
4. Boundary / abstraction / type-contract problems that make the code harder to reason
   about
5. File-size and decomposition concerns
6. Modularity and abstraction issues
7. Legibility and maintainability concerns

Do not flood the review with low-value nits if there are larger structural issues. Prefer
a smaller number of high-conviction comments over a long list of cosmetic notes.

---

Final checklist:

- [ ] Bug-free and production-ready
- [ ] Performant (no obvious performance issues, no n+1s, no unnecessary work)
- [ ] Clear API (easy to understand and use correctly)
- [ ] Secure
- [ ] Comprehensive Tests (100% of public API surface covered appropriately and robustly,
      without cheating)
- [ ] Comprehensive Docs (all public APIs documented, with clear explanations and examples
      where hepful, but not overly long or verbose)
- [ ] Compliance with AGENTS.md
- [ ] No unnecessary database calls (as few queries as possible, always)
- [ ] Design coherence across all packages (consistent
      philosophy/opinions/method-names/etc)
- [ ] Nothing weird, surprising, or hacky without clear justification
- [ ] No cruft or Rube Goldberg machines
- [ ] Anything that should be a shared util but is defined locally (or anything defined
      locally that duplicates an already-existing shared util)?
- [ ] Nothing overly complicated that could be simpler

Ultrathink
