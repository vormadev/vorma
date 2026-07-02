# oxlint no-redundant-type-constituents warnings in the react adapter

Observed during the P001 gate closeout run (2026-07-01): `make ts-lint` exits green but
prints five type-aware warnings, all in `packages/vorma/ui/react/react.tsx`:

```
react.tsx:110:19 warning typescript(no-redundant-type-constituents):
  'RouteState' is an 'error' type that acts as 'any' and overrides all other types in this union type.
react.tsx:208:29 (same, 'ScrollIntent')
react.tsx:251:66 (same, 'RouteState')
react.tsx:252:40 (same, 'RouteState')
react.tsx:264:63 (same, 'WorkState')
react.tsx:265:38 (same, 'WorkState')
react.tsx:362:5  (same, 'ToViewOutput<A, P>')
react.tsx:529:45 (same, 'ToLinkProps<A, P>', intersection type)
react.tsx:551:46 (same, 'ToLinkProps<A, P>', intersection type)
```

Plus one warning outside the react adapter:

```
packages/vorma/core/ownership.test.ts:187:21 warning
  typescript(no-redundant-type-constituents): 'any' overrides all other types in this union type.
```

"Is an 'error' type" means oxlint's type resolution could not resolve those symbols at the
lint site and treated them as `any`. Two possible causes, with different fixes:

1. A lint-configuration gap (oxlint type-aware mode not resolving project references /
   paths the way tsgo does). Then the fix is lint config, and the types are fine —
   `make ts-typecheck` (tsgo) is the type-truth gate and passes.
2. A real type hole where `WorkState` / `ToViewOutput` / `ToLinkProps` genuinely resolve
   to error types in those positions, silently widening unions/intersections to `any` for
   consumers.

## Task

- Determine which cause applies (check whether tsgo resolves the same symbols cleanly at
  those sites; inspect oxlint type-aware configuration in `oxlint.config.ts`).
- Fix the true cause. Do not suppress the rule.
- Consider whether `make ts-lint` should deny warnings so the gate cannot accumulate
  warning debt silently (maintainer call; today the gate bar is exit code only).

## Verification

- `make ts-lint` output is warning-free for `react.tsx`.
- `make ts-typecheck` remains green.
