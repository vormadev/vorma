# TEMP_HEGEL_PLAN

This is a temporary planning note for where Hegel/property-based tests are
likely to buy real confidence in this repo.

The goal is not to convert ordinary examples into randomized examples, and it is
not to prove that Go, the standard library, or third-party dependencies work as
documented. The goal is to find places where bugs can hide in Vorma-owned
semantics: our route precedence, schema interpretation, reflection rules,
generated TypeScript names, manifest graph interpretation, file inclusion rules,
and other code where this repo makes nontrivial decisions.

## Selection Rule

Use Hegel when at least one of these is true and the behavior being checked is
substantially ours:

- There is an independent oracle or model to compare against.
- The behavior should be invariant under order, repetition, composition, or
  unrelated additions.
- The input space is combinatorial enough that hand-written examples are likely
  to miss interactions.
- Shrinking a failing generated case would materially help debugging.
- The code maintains repo-specific state across a sequence of operations.

Do not use Hegel merely because a package has abstract laws. Sets, LRU caches,
JSON round trips, filesystem copies, HTTP handlers, and cryptographic primitives
all have properties, but those properties often belong to the language, standard
library, or dependency contract rather than to this repo. If a property mostly
proves `map`, `container/list`, `encoding/json`, `net/http`, `fsnotify`, or a
crypto primitive works, it does not belong here.

Prefer normal tests when the behavior is a fixed named case, a small table, a
panic guard, an OS integration detail, or a wrapper over the standard library.

Prefer Hegel when the test can say: "For many generated inputs/configurations,
Vorma's interpretation is equivalent to this simpler model" or "this
Vorma-specific semantic law is preserved."

## Current State

Already added:

- `kit/matcher/matcher_hegel_test.go`

My current judgment:

- `kit/matcher` is a very strong Hegel target and now has the Hegel coverage I
  currently want.
- `kit/mux` is not currently a good first-wave Hegel target. It may have useful
  composition properties later, but any mux property has to be framed around
  mux-owned behavior rather than rechecking `kit/matcher`.

## Highest Priority

### 1. `kit/schema`

Why Hegel fits:

`schema` has reflection, mutation-vs-value semantics, nil/zero/default handling,
object traversal, map/slice recursion, error accumulation, and validator
ordering. Those are exactly the kinds of cross-products that example tests
struggle to cover.

Good property ideas:

- Idempotence: enforcing the same schema twice yields the same final value and
  same validation result as enforcing it once.
- Pointer/value equivalence: value input and pointer input produce equivalent
  `Result.Value`, while only pointer input mutates the caller-owned value.
- Normalization ordering: generated string rules always behave as trim -> case
  transform -> custom transform -> default-if-zero -> validation.
- Error determinism: generated objects/maps/slices produce stable error text and
  stable error classification across repeated runs.
- Schema error precedence: malformed schemas consistently classify as
  `SchemaError`, even when the data would also fail validation.
- Map-key normalization: generated string-keyed maps with normalizing key
  schemas obey a documented collision rule.
- Nil/zero law: `DefaultIfNil`, `MustNotBeNil`, `DefaultIfZero`, and
  `MustNotBeZero` behave consistently across pointer, slice, map, interface, and
  scalar slots.

Notes:

This should use a deliberately small catalog of Go test fixture types rather
than trying to generate Go types dynamically. Generate schemas, values, and
operation modes around those fixtures.

### 2. `kit/searchparams`

Why Hegel fits:

`searchparams` decodes flat query strings into nested structs, slices, maps, and
pointers, and separately derives a schema from Go types. This has a clear
round-trip/model-testing shape and many edge cases around empty values, repeated
keys, prefixes, and pointer materialization.

Good property ideas:

- Query-order invariance: permuting query parameter order does not change the
  parsed struct.
- Repeated scalar rule: generated repeated scalar values obey the documented
  "first value wins" behavior.
- Slice rule: repeated keys and dotted indexed-ish forms that the package
  accepts produce the same slice values when they describe the same data.
- Pointer scalar rule: empty string clears pointer scalars, non-empty values
  materialize them.
- Nested prefix rule: `A.B=x` only affects field `B` under nested field `A`.
- Schema compatibility: generated fixture values parse into shapes compatible
  with `SchemaFromValue`.

Notes:

Use a fixture catalog of struct shapes. This package is a better Hegel target
than ordinary parse examples because most bugs will be in interactions between
field kind, pointer-ness, nesting, and repeated query values.

### 3. `kit/tsgen`

Why Hegel fits:

`tsgen` walks Go type graphs, deduplicates type entries, resolves name
collisions, handles references, and emits TypeScript strings. Bugs are likely to
come from graph shape and registration order, not from one example type.

Good property ideas:

- Registry order independence: registering the same set of root types in any
  order produces the same resolved map.
- Duplicate root idempotence: adding the same root type repeatedly does not
  change output.
- Name collision determinism: generated requested-name collisions resolve to the
  same stable names regardless of input order.
- Reference closure: every emitted reference name points to an emitted type, and
  no sentinel ids remain in final bodies.
- Reachability pruning: unreferenced discovered types are not emitted unless
  they are roots.
- Embedded/anonymous field law: generated fixture structs with embedded fields
  preserve the expected public field shape.

Notes:

Like schema, this needs a type fixture catalog. The generated dimension should
be registration order, requested names, duplicate roots, and combinations of
fixtures.

### 4. `kit/lab/repoconcat`

Why Hegel fits:

`repoconcat` is a glob/gitignore-style pattern engine plus filesystem traversal.
It already has many examples, but the hard part is the interaction of pattern
order, negation, anchoring, trailing slash semantics, default excludes, nested
gitignore files, and user overrides.

Good property ideas:

- Last-match-wins model: for generated small file trees and generated pattern
  sequences, inclusion matches an independent simple model for the supported
  pattern subset.
- Order sensitivity is local: appending an unrelated pattern does not change
  unrelated files.
- Default-exclude override: explicitly including default-excluded roots/files
  follows the same rule as ordinary negation/reinclusion.
- Output-file exclusion: the output file is never included, regardless of
  pattern order or extension.
- Gitignore hierarchy: moving the same gitignore rule from root to a
  subdirectory only changes files under that subdirectory.
- Symlink safety: generated symlink placements are skipped and never traversed.

Notes:

This is high value but more work. It needs a small generated filesystem builder
and a deliberately limited independent model. Do not attempt to model all of
doublestar at once.

### 5. `internal/pkg/viteutil`

Why Hegel fits:

`FindAllDeps` interprets a Vite manifest graph and applies Vorma's dependency
ordering/deduping semantics. The standard library is not the interesting part;
the interesting part is our graph traversal contract and CSS/module collection
rules.

Good property ideas:

- Reachability: result modules are exactly files for chunks reachable from the
  root through `Imports`.
- Deduplication: repeated imports and graph diamonds include each file/CSS
  bundle once.
- Cycle safety: generated cyclic import graphs terminate and include each
  reachable chunk once.
- Order law: output order matches first DFS encounter order.

Notes:

This is a small, high-signal target and a good quick win outside `kit`.

## Conditional High Value

These are plausible Hegel targets, but only if the property is aimed at
Vorma-owned semantics. They should not be treated as "property-test this whole
package."

### 7. `kit/mux`

Why Hegel is only conditional:

`mux` composes Vorma-owned route matching, mount-root stripping, HEAD fallback,
tasks contexts, middleware layers, response proxies, and nested route execution.
The valuable properties would be composition laws, not basic HTTP handler cases
and not "mux returns whatever matcher returns."

Good property ideas:

- Method dispatch law: generated routes for other methods do not affect the
  selected handler for the request method.
- HEAD fallback law: generated HEAD requests use explicit HEAD routes when
  present and otherwise behave like GET without writing a body.
- Mount-root equivalence: serving `/root/x` with mount root `/root` is
  equivalent to serving `/x` without a mount root.
- Nested execution law: generated handler-less ancestors and matching
  descendants produce the expected task execution/result sequence.

Notes:

Do not add mux Hegel until a property clearly names the mux-owned decision it
can falsify. `kit/matcher` can be a helper for setup, but it should not be the
behavior under test.

### 8. `kit/tasks`

Why Hegel fits:

The package has Vorma-owned memoization semantics over task id/input pairs,
shared dependency graphs, cancellation, cache sharing, and TTL. Hegel is useful
only if we model those semantics directly. It should not try to prove goroutine
scheduling works.

Good property ideas:

- Generated DAG: every reachable op/input pair executes at most once per cache
  while all prepared outputs receive the same result.
- Shared dependency law: two parents depending on the same child observe the
  same child result and child execution count is one.
- Input partition law: same func with different inputs caches independently.
- Error retry law: ordinary errors are cached according to the package contract;
  cancellation/deadline errors are not cached.
- Cache sharing law: `WithContext` shares memoized data but respects the new
  cancellation context.

Notes:

This is a good Hegel target, but concurrency can make failures noisy. Keep the
model deterministic and run with `-race` in focused checks.

### 9. `kit/response`

Why Hegel fits:

`response.Proxy` is a small state machine with composable operations: set/add
headers, cookies, statuses, redirects, head elements, merge, and apply. It is
well-suited to a generated operation-sequence reducer because the merge rules
are ours.

Good property ideas:

- Merge reducer: generated proxy operation sequences merge exactly like an
  independent reducer model.
- Header law: `SetHeader` clears prior values for that key; `AddHeader` appends
  after the latest set.
- Status law: first error wins; otherwise last success wins.
- Redirect law: first redirect wins only when no error wins.
- Apply law: applying a merged proxy to a recorder yields the same
  headers/status as applying the independent reducer's final state.

Notes:

This would probably let us simplify some current example tests over time.

### 10. `kit/head`

Why Hegel fits:

Head rendering has dedupe rules, element hashing, rule initialization, last-wins
semantics, escaping, and prepared output grouping. This is composition-heavy.

Good property ideas:

- Prepare idempotence: preparing rendered-safe elements twice is equivalent to
  preparing once.
- Last unique wins: for generated duplicates matching unique rules, the last
  matching element is retained.
- Non-unique preservation: generated non-unique elements keep relative order
  unless exact hash duplicates replace earlier copies.
- Rule initialization purity: initializing dedupe rules with a builder does not
  mutate the builder's element list.
- Render grouping: prepared title/meta/rest elements always render into the
  expected comment-delimited sections.

Notes:

Good payoff if head behavior is product-critical. Otherwise lower priority than
schema/searchparams/tsgen.

### 11. `kit/lab/fsmarkdown`

Why Hegel fits:

`fsmarkdown` maps a markdown file tree to lookup results, navigation items,
folder/index semantics, parent URLs, sorting, and cache behavior. Generated
small file trees can expose weird navigation interactions.

Good property ideas:

- Path-cleaning equivalence: equivalent dirty paths resolve to the same clean
  result.
- Folder/index law: `/x` resolves to `x.md` or `x/index.md` according to the
  documented precedence.
- Nav sorting law: siblings/children are sorted by order/date/title according to
  the same independent model.
- Parent law: parent URL exists iff the parent directory has an index and the
  page is not root/top-level.
- Dev/prod cache law: prod reuses cached results; dev re-reads changed files.

Notes:

This is a decent target, but it needs a generated in-memory FS plus a small
oracle. Do not start here unless docs navigation bugs are costly.

## Medium Priority

### 12. `kit/cookies`, `kit/securebytes`, `kit/securestring`, `kit/keyset`, `kit/cryptoutil`

Why Hegel partially fits:

There are strong round-trip and tamper-resistance properties, but much of the
behavior is already handled by cryptographic primitives or clear example tests.
Use Hegel sparingly for composition and rotation, not for re-testing AES/HMAC.

Good property ideas:

- Secure value round trip: generated JSON/gob-friendly values survive
  securebytes/securestring serialization and parse with the same keyset.
- Rotation law: values encrypted with any key in an old keyset parse with a
  keyset containing that key later in the list.
- Wrong-key law: generated wrong keysets cannot parse generated ciphertext.
- Cookie config law: generated manager mode/options produce host prefix, secure,
  path, domain, partitioned, and HttpOnly attributes according to policy.
- Keyset attempt law: `Attempt` calls keys in order and stops at the first
  success.

Notes:

These are valuable but should be few. Avoid expensive statistical/randomness
properties in normal gates.

### 13. `kit/csrf`

Why Hegel partially fits:

CSRF is security-sensitive and stateful: GET issues/cycles tokens, POST
validates origin/header/cookie/session, session changes self-heal, dev mode has
host policy. A generated request-flow model could find missed interactions.

Good property ideas:

- Flow model: generated sequences of GET, POST, session change, login cycle,
  logout cycle, and token tamper produce expected allow/deny/self-heal behavior.
- Origin model: generated Origin/Referer/Host combinations obey the origin
  validation contract.
- Session binding law: tokens bound to one session do not validate for another
  session, and self-heal occurs only for the documented cases.
- Header-name law: custom header names are consistently honored.

Notes:

This could be high value, but the existing test file is already large. Add Hegel
only if it replaces a meaningful matrix or models whole request flows.

### 14. `internal/pkg/vormarun`

Why Hegel partially fits:

The loaders handler combines nested route matches, task results, response
proxies, manifest route modules, deps/CSS dedupe, head preparation, JSON vs
HTML, and build-id reload behavior. The high-value target is integration
invariants, not small helpers.

Good property ideas:

- Loader payload model: generated matched patterns plus manifest modules produce
  payload import URLs, deps, CSS bundles, params, splats, schemas, and data in
  the expected order.
- Error boundary law: the outermost loader error index/message is the first
  failing matched loader, and later loaders do not contribute data.
- Reload law: mismatched client build id returns reload headers and does not run
  loaders.
- Dependency dedupe law: entry deps precede route deps and each dep/CSS appears
  once.

Notes:

This likely needs test seams or fixture builders. Do not start here until the
lower-level route/task/head/manifest properties are stable.

### 15. `internal/pkg/vormabuild`

Why Hegel partially fits:

Manifest generation maps config, TS route modules, Vite manifest graphs, public
file maps, CSS, and search schemas into runtime manifests. Properties could
catch path normalization and dependency graph bugs.

Good property ideas:

- Dev/prod manifest law: generated route module maps produce dev URLs or prod
  public URLs according to mode.
- Client module law: `to_client_module` agrees with `viteutil.FindAllDeps` for
  generated Vite graphs.
- Public file law: final public file paths are exactly public filemap values
  plus client entry/route deps and CSS bundles.

Notes:

Useful later, but it may require more setup than the first wave should spend.

### 16. `internal/pkg/lockfile`

Why Hegel partially fits:

`lockfile` is a lease-based state machine with acquire, heartbeat, stale-owner
reclamation, release, invalid lock records, and ownership loss callbacks.
Generated operation sequences could help, but real time and filesystem behavior
make it more integration-heavy than the pure state-machine packages.

Good property ideas:

- Single-owner law: across generated acquire/release sequences, at most one lock
  instance believes it owns a path.
- Reentrant acquire law: acquiring an already-held lock by the same instance is
  idempotent.
- Stale takeover law: generated stale lease records can be reclaimed, while
  fresh lease records are treated as held.
- Invalid-record law: malformed records are either recovered or rejected
  according to the documented stale threshold.
- Release law: release removes ownership only for the current owner.

Notes:

This may need shorter injected timings or a clock seam before it becomes a good
normal-gate Hegel target. Until then, keep mostly normal/integration tests.

### 17. `internal/pkg/staticproc`

Why Hegel partially fits:

Static processing maps physical or injected files to hashed output names, then
reconciles an output directory to match the desired file map. There are useful
filesystem invariants here, but the code is less central than routing/schema.

Good property ideas:

- Reconcile law: after reconciling generated files, the output directory
  contains exactly the expected output names and no stale files.
- Hash stability law: unchanged physical file path/modtime/size/content yields a
  stable output name; changed bytes or source path changes the output name.
- Prehashed passthrough law: generated prehashed directories preserve source
  names according to the configured passthrough rules.
- Inject law: injected in-memory files appear in `Files.Map()` and reconcile as
  physical output bytes.

Notes:

Worth doing if asset pipeline bugs are recurring. Otherwise lower priority than
`viteutil` and `vormabuild` manifest tests.

## Low Priority / Probably Normal Tests

These packages mostly have small deterministic behavior, wrappers around
standard library functions, or hand-picked security examples where normal tests
are clearer:

- `kit/bytesutil`: base64/gob round trips are simple; Hegel could test gob round
  trips but payoff is low.
- `kit/jsonutil`: mostly `encoding/json` wrappers.
- `kit/set`: algebraic laws are easy to property-test, but the package is small
  and unlikely to hide expensive bugs.
- `kit/lru`: state-machine testing is possible, especially around the custom
  spam/TTL interaction, but most properties would be generic cache behavior
  backed by `map`, `container/list`, and `time`. Keep this normal-test driven
  unless a concrete cache bug appears or the spam/TTL policy becomes more
  product-critical.
- `kit/envutil`: environment parsing is a normal table test target.
- `kit/netutil`: host parsing examples are clearer than generated strings unless
  a bug appears.
- `kit/fsutil`: copy behavior is filesystem integration with a few important
  examples; generated filesystem trees are possible but lower value than
  `repoconcat` or `fsmarkdown`.
- `kit/htmlutil`: escaping/rendering has some property shape, but examples
  probably suffice unless generated HTML element combinations have caused bugs.
- `kit/reflectutil`: some behavior is complex, but Go cannot generate new struct
  types at runtime; use fixture catalogs and normal tests unless a specific
  invariant emerges.
- `kit/k9` and `kit/id`: round-trip/range properties are natural but already
  well-covered and partly random/statistical.
- `kit/colorlog`, `kit/executil`, `kit/grace`, `kit/contextutil`,
  `kit/lazyonce`, `kit/middleware/*`, `kit/theme`, `kit/lab/mailutil`,
  `kit/lab/cliutil`: keep these mostly normal-test driven unless a concrete
  state-machine or parser invariant appears.
- `internal/pkg/stringutil`: small builder/line helpers; normal tests are
  enough.
- `internal/pkg/mailbox`: tiny concurrent map/notification helper; race tests or
  focused normal tests are more useful than Hegel unless it grows.
- `internal/pkg/fswatcher`: event timing and OS watcher behavior make this a
  better integration-test target than normal-gate Hegel.
- `internal/pkg/cssbundle`: mostly esbuild/plugin integration. Use fixture
  integration tests unless URL rewriting grows a pure seam.
- `internal/pkg/npm`, `internal/cmd/*`, `internal/apps/docs`,
  `internal/vorma_fw_tests`, `bootstrap`, and `build`: these are
  entrypoint/app/integration layers. Hegel only makes sense here after
  extracting a pure model or when testing generated end-to-end scenarios outside
  the normal unit gate.

## Suggested Punch List

1. Add `kit/schema` Hegel next.
    - Start with idempotence and pointer/value equivalence. Those are broad,
      Vorma-owned semantics and do not need giant fixtures.

2. Add `kit/searchparams` after schema.
    - Start with query-order invariance and pointer/slice/map fixture parsing.
    - The target is Vorma's decoding semantics, not URL parsing itself.

3. Add `kit/tsgen`.
    - Start with registry order independence, duplicate roots, and no unresolved
      sentinel ids.
    - The target is Vorma's type graph/name resolution, not reflection in
      isolation.

4. Add small, high-signal graph property in `internal/pkg/viteutil`.
    - This is cheap and directly checks our Vite manifest graph interpretation.

5. Consider `kit/lab/repoconcat` only with a bounded independent model.
    - High value, but do not start without constraining the pattern subset.

6. Add `kit/response` proxy reducer property if mux/loader proxy behavior keeps
   being important.

7. Consider `kit/tasks` only with a deterministic model of memoization
   semantics.
    - Do not write properties that merely create many goroutines and hope the
      scheduler explores something interesting.

8. Consider CSRF/request-flow Hegel only if it can replace a large matrix of
   hand-written tests or model actual user/session flows.

9. Leave `kit/lru` and other generic utilities alone for now.
    - Revisit only if there is a concrete Vorma-specific policy bug to hunt.

10. Leave app/cmd/bootstrap layers alone for now.
    - They should benefit indirectly from lower-level properties plus ordinary
      integration tests.

## Practical Guidelines

- Prefer one strong property over five randomized table tests.
- Every Hegel test should name its oracle: independent model, algebraic law,
  state-machine reducer, or equivalence relation.
- Every proposed property should answer: "What Vorma-owned decision could this
  falsify?"
- Do not add Hegel tests whose main oracle is "the standard library behaves as
  promised."
- Keep checked-in case counts low enough for `go test -race ./...`.
- Use focused local stress with `-count=N` when touching the target package.
- If a property needs a lot of scaffolding, ask whether a smaller normal table
  would be clearer.
- If a property is only checking a fixed scenario with random labels, it
  probably does not belong in Hegel.
