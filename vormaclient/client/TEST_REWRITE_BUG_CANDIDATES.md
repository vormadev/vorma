# vormaclient/client Strict Decision Ledger

This file records strict first-principles decisions made during migration. There
is no deferred "open" queue in this process.

## Status Keys

- `accepted`: behavior is explicitly chosen as intended contract.
- `fixed`: implementation and tests were updated to satisfy strict contract.

## Decisions

| ID   | Area                        | Status   | Strict Decision                                                                                       | Resolution                                                                                                                                               |
| ---- | --------------------------- | -------- | ----------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------- |
| BC1  | redirect handling           | accepted | Native browser redirect responses (`response.redirected === true`) for GET must be followed.          | Covered by strict contract in `src/contracts/client.navigation_modes.contract.test.ts` ("follows native fetch redirects for GET").                       |
| BC2  | redirect handling           | fixed    | Native browser redirect responses for non-GET must also be followed.                                  | Fixed in `src/redirects/redirects.ts` by removing non-GET suppression; covered by `src/contracts/client.submit_and_redirect.contract.test.ts`.           |
| BC3  | redirect loop handling      | fixed    | Redirect-limit failures must always unwind loading state and clean navigation entries.                | Fixed in `src/client.ts` by deleting aborted navigation entries in `navigate`; covered by strict redirect-cap test.                                      |
| BC4  | mixed loading state policy  | accepted | `isNavigating` and `isRevalidating` may both be true when operations overlap.                         | Existing strict contract retained in `src/contracts/client.state_and_revalidation.contract.test.ts`.                                                     |
| BC5  | navigation error behavior   | fixed    | Failed navigation must keep URL/title stable, clear loading state, and keep client operable.          | Enforced by strict assertions in `src/contracts/client.error_and_edge.contract.test.ts`; duplicate top-level log removed in `src/client.ts`.             |
| BC6  | submit error model          | accepted | Submit failures use deterministic exact error values (`"500"`, `"Network failure"`, `"Aborted"`).     | Enforced by strict assertions in `src/contracts/client.submit_and_redirect.contract.test.ts`.                                                            |
| BC7  | redirect limit semantics    | accepted | Redirect chains cap at exactly 10 fetches and emit explicit "Too many redirects" logging.             | Enforced by strict assertions in `src/contracts/client.submit_and_redirect.contract.test.ts`.                                                            |
| BC8  | submit dedupe handoff       | fixed    | Same-key dedupe handoff must not create a false `isSubmitting=false` gap while replacement is active. | Fixed in `src/client.ts` by guarding submission-key cleanup to owner entry; enforced by submit dedupe-handoff strict contract.                           |
| BC9  | redirect buildID ordering   | fixed    | Navigation redirects must update/dispatch build ID before following redirect target fetches.          | Fixed in `src/client.ts` redirect outcome path; enforced by strict contract in `src/contracts/client.submit_and_redirect.contract.test.ts`.              |
| BC10 | head section reconciliation | fixed    | Managed head sections must remove interleaved non-element nodes during reconciliation.                | Fixed in `src/head_elements/head_elements.ts` by removing non-element nodes between markers; enforced by `src/contracts/head_elements.contract.test.ts`. |

## Rule

If a new ambiguity appears and first principles cannot resolve it, ask the user
immediately before adding or relaxing assertions.
