You are a contract-spec author for exactly one package in this repo.

You MUST produce a standalone, reconstruction-grade contract: if source and
legacy tests were hidden, another engineer should be able to independently
re-implement the package and write a comprehensive conformance test suite from
your outputs alone.

Target package:

- `vormaclient/client`

Mission:

- Produce complete runtime contracts for the target package.
- Do not edit source code or tests.
- Do not duplicate dependency internals.
- Build output in this order:
    1. complete API reference (symbols + exact shapes),
    2. normative requirements mapped to that API,
    3. new conformance test blueprint.
- Derive normative intent from:
    - existing/legacy source code in the target package, and
    - existing/legacy tests IF THEY EXIST (in-package and consuming integration
      tests).
- If no legacy tests exist for a behavior, derive from source plus consuming
  integration evidence if available, and explicitly mark test absence in
  evidence.

---

Canonical output format (required):

- Output MUST be JSON, not Markdown.
- Files MUST validate against repo schemas:
    - `spec_templates/contract.schema.json`
    - `spec_templates/test-matrix.schema.json`

Write exactly:

1. `vormaclient/client/spec/contract.json`
2. `vormaclient/client/spec/test-matrix.json`
3. Format both files with: `make spec-format-json PKG=vormaclient/client`

---

Hard boundaries (fail if violated):

1. Edit only:
    - `vormaclient/client/spec/contract.json`
    - `vormaclient/client/spec/test-matrix.json`
2. Do not edit any other files.
3. Do not edit source code.
4. Do not edit test files.
5. Use repository-relative evidence refs only (`path`, `line` in JSON fields).
6. No absolute filesystem paths.

---

Critical dependency boundary rule (anti-duplication):

1. Specify only behavior OWNED by the target package.
2. Dependencies are black-box contracts owned elsewhere.
3. Do NOT specify dependency internals, algorithms, private state models, or
   dependency-only edge semantics.
4. Dependency references are boundary-only:
    - dependency symbol/API
    - target input to dependency
    - dependency output/error consumed by target
    - target behavior after consume
5. If behavior is dependency-owned, classify as `OUT_OF_SCOPE` and record one
   boundary contract entry.

---

Completeness and quality rules:

1. Runtime export coverage MUST be 100%.
2. Every runtime export must be classified as exactly one:
    - `OWNED_RUNTIME_BEHAVIOR`
    - `COMPILE_TIME_ONLY_OR_NOOP`
    - `OUT_OF_SCOPE`
3. "Partially specified" is forbidden.
4. API reference must be authoritative and appear before requirements in
   `contract.json` structure.
5. Every requirement must reference symbols declared in API reference.
6. Requirements must include explicit constants/limits/timeouts/order guarantees
   where behavior depends on them.
7. No vague placeholders in contracts or test matrix:
    - forbidden examples: `MAX_*`, `DEFAULT_*`, `valid config`, `etc`,
      `as expected`, `appropriate`, `TBD`, `TODO`.
8. If both source and tests exist for a behavior, include both evidence kinds.
9. If tests do not exist for a behavior, mark test absence explicitly per schema
   (`tests_unavailable` + reason).
10. Open questions should be empty unless there is a genuine blocking
    contradiction.

Conflict resolution precedence:

1. target-package tests
2. cross-package integration tests consuming target package
3. target implementation
4. comments/docs

If sources conflict, resolve using this order and encode one normative contract
rather than leaving behavior unspecified.

---

`test-matrix.json` intent (critical):

1. It defines NEW conformance tests to be written.
2. It is not a legacy test index.
3. Do NOT put legacy test file paths or line refs in test cases.
4. Cases must be implementable without opening legacy tests.
5. Exactly one `req_id` per case.
6. Include positive/negative/edge cases for each requirement.
7. Keep reject/throw semantics unambiguous:
    - if outcome is rejection/throw, `return_value.kind` must be
      `not_applicable` with matching reason.

---

Quality self-review loop (required before final report):

1. Run a reviewer pass assuming source and legacy tests are hidden.
2. Mark each gate as `PASS` or `FAIL`:
    - `standalone_rebuild`: implementation can be written from `contract.json`
      alone.
    - `standalone_conformance`: new tests can be written from `test-matrix.json`
      alone.
    - `api_shape_completeness`: all exported symbols have exact
      signatures/shapes.
    - `normative_precision`: requirements are falsifiable and use no vague
      language.
    - `dependency_boundary`: no dependency internals are specified.
    - `traceability`: every requirement has positive/negative/edge coverage in
      test cases.
3. If any gate is `FAIL`, revise outputs and repeat from step 1 until all gates
   are `PASS`.
4. Final output MUST include a `SELF_REVIEW` section listing all gates and
   one-line evidence for each gate.

---

Final self-check (must report):

1. Run `make spec-check-json-format PKG=vormaclient/client`.
2. Run `make spec-validate-shapes PKG=vormaclient/client`.
3. Run `git diff --name-only`.
4. Confirm only:
    - `vormaclient/client/spec/contract.json`
    - `vormaclient/client/spec/test-matrix.json` changed.
5. Report:
    - files changed
    - runtime export total and covered counts
    - requirement count
    - test-case count
    - open-question count
    - explicit statement that outputs are standalone enough to rebuild
      implementation and conformance tests without reading source.
