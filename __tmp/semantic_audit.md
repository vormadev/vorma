# Semantic API Decision Workspace

- Date: 2026-02-17
- Scope: Go APIs across the repository.

## Operating Rules

- Discuss exactly one decision at a time in chat.
- Add a decision to this doc only after explicit confirmation in chat.
- Items in the backlog are open proposals until recorded in Confirmed Decisions.

## Confirmed Decisions

- Decision 1:
    - Topic: Panic-wrapper naming for constructors.
    - Rule: Use `MustCreate<Thing>` naming.
    - Constraint: Do not introduce `MustNew*`.
    - Scope: Any panic-wrapper constructor introduced during this audit.
- Decision 2:
    - Topic: Panic policy.
    - Rule: Panics are allowed only for `Must*` APIs or developer invariants.
    - Constraint: Recoverable runtime and input/configuration failures should
      not panic in non-`Must*` APIs.
    - Scope: Repository-wide API surface.
- Decision 3:
    - Topic: Constructor and initializer error-return semantics.
    - Rule: Non-`Must*` constructor/initializer APIs return an `error` only when
      they have a recoverable failure mode that callers can handle.
    - Constraint: Do not add an `error` return solely for naming/semantic
      uniformity when no recoverable failure mode exists.
    - Constraint: If a recoverable failure mode exists, non-`Must*`
      constructor/initializer APIs should return `error` rather than panic.
    - Scope: Repository-wide constructor/initializer surfaces.
- Decision 4:
    - Topic: Mutable-vs-snapshot accessor naming.
    - Rule: If only one variant is exposed, use an unsuffixed name and document
      the returned shape clearly in godoc.
    - Constraint: If both mutable and snapshot variants are realistically
      needed, suffix both explicitly (for example `*Mutable` and `*Snapshot`).
    - Scope: Accessors returning live mutable views or defensive copies.
- Decision 5:
    - Topic: `Set*` vs `Add*` mutation semantics.
    - Rule: `Set*` means create-or-override/replace semantics; additive
      semantics use `Add*` (or `Append*` where appropriate).
    - Constraint: Do not use `Set*` names for append/merge behavior.
    - Scope: Repository-wide mutator APIs, including response proxy cookies.
- Decision 6:
    - Topic: `mux` registration semantics contract.
    - Rule: Additive route and middleware registration APIs use `Add` as the
      verb.
    - Rule: Route naming grammar is `Add` + `<Lane>` + `<Target>`, where
      `<Lane>` is explicit as `HTTP` or `Task` when both lanes exist, and
      `<Target>` is explicit as `Handler` or `HandlerFunc`.
    - Rule: Nested-route APIs use `AddNested...` naming.
    - Rule: Middleware naming grammar is `Add` + `<Scope>` + `<Lane>` +
      `Middleware`, where `<Scope>` is explicit as `Global`, `Method`, or
      `Pattern`, and `<Lane>` is explicit as `HTTP` or `Task` where both lanes
      exist.
    - Rule: If a logical API grouping needs free functions to work around Go
      generic-method limitations, provide free-function variants across the
      related grouping for consistency.
    - Rule: Where a non-generic method form is possible, provide that method
      form as well.
    - Constraint: Do not use `Register*` or `Handle*` names for additive route
      registration behavior.
    - Constraint: Do not use `Set*` for additive route registration behavior.
    - Constraint: Do not use `Set*Middleware` or `Use*Middleware` names for
      additive middleware registration behavior.
    - Scope: `mux` route and middleware registration surfaces (router, nested
      router, and route).
- Decision 7:
    - Topic: Getter naming and lazy-init transparency.
    - Rule: Simple method accessors use idiomatic noun/verb names and do not use
      a `Get` prefix.
    - Rule: If a method accessor performs internal lazy initialization that is
      safe and does not change runtime semantics the caller must reason about,
      keep the same idiomatic accessor naming and treat lazy init as an internal
      detail.
    - Constraint: Do not encode internal lazy-init implementation details in API
      names unless callers must reason about a semantic/runtime consequence.
    - Scope: Repository-wide method accessor naming.
- Decision 8:
    - Topic: `MountRoot` variadic arity contract.
    - Rule: Keep the existing `MountRoot(optionalPatternToAppend ...string)`
      API.
    - Rule: `MountRoot` accepts zero or one argument only.
    - Rule: Passing more than one argument violates a developer invariant and
      panics.
    - Constraint: Do not silently ignore extra variadic arguments.
    - Scope: `(*mux.Router).MountRoot`.
- Decision 9:
    - Topic: Request param/splat absence shape.
    - Rule: `Params(r)` returns `nil` when params are absent.
    - Rule: `SplatValues(r)` returns `nil` when splat values are absent.
    - Constraint: Absence paths do not allocate empty maps or slices.
    - Scope: `mux` request accessor helpers for params and splat values.
- Decision 10:
    - Topic: `ReqData.Redirect` return contract.
    - Rule: `(*ReqData[I]).Redirect(url string, code ...int)` returns
      `(usedClientRedirect bool, err error)`.
    - Rule: `ReqData.Redirect` preserves the full return semantics of
      `response.Proxy.Redirect`.
    - Constraint: Do not silently discard redirect validation/runtime errors.
    - Scope: `kit/mux` request helper redirect API.
- Decision 11:
    - Topic: Router option vocabulary alignment.
    - Rule: Across router and matcher option surfaces, use `DynamicParamPrefix`
      and `SplatSegmentIdentifier` (not suffixed with `Rune` or `Marker`).
    - Rule: Index-segment vocabulary is used only for nested routing contexts.
    - Rule: When the index segment is optional in a type's contract, use
      `ExplicitIndexSegmentIdentifier`.
    - Rule: When the index segment is always resolved in a type's contract, use
      `IndexSegmentIdentifier`.
    - Scope: `matcher.Options`, `mux.Options`, `mux.NestedOptions`,
      `vormaruntime.LoadersRouterOptions`, and related option types.
- Decision 12:
    - Topic: Discovered-loader helper package and naming.
    - Rule: Build-discovered registration helpers are moved out of the root
      `vorma` package into `vormagogen`.
    - Rule: Helper symbols in `vormagogen` do not use the `Internal__` prefix.
    - Rule: The discovered-loader registration helper is named
      `RegisterLoaderDiscoveredByBuild`.
    - Rule: The discovered-action registration helper is named
      `RegisterActionDiscoveredByBuild`.
    - Scope: Generated loader registration callsites and corresponding exported
      helper API.
- Decision 13:
    - Topic: Release-version helper API surface.
    - Rule: The release-version helper remains in the root `vorma` package.
    - Rule: The helper is public and named `CurrentReleaseVersion`.
    - Rule: Do not use an `Internal__` prefix for this helper.
    - Scope: API currently named `CurrentReleaseVersion`.
- Decision 14:
    - Topic: `Ensure*` semantic contract.
    - Rule: `Ensure*` means "add if not exists" semantically.
    - Rule: `Ensure*` does not imply one specific side-effect class by itself.
    - Scope: Repository-wide `Ensure*` APIs.
- Decision 15:
    - Topic: CSRF dev-mode host mismatch handling.
    - Rule: Keep the current `(*csrf.Protector).Middleware` behavior that panics
      in dev mode when request host is not localhost/loopback.
    - Rule: This behavior is treated as an intentional safety guard.
    - Scope: `kit/csrf` middleware request-path host check.
- Decision 16:
    - Topic: Fluent chaining API pattern.
    - Rule: Fluent/chained APIs are banned across Go package surfaces.
    - Rule: Mutating APIs must not return receiver/collection values solely to
      support call chaining.
    - Rule: Use explicit one-call-per-statement APIs instead.
    - Scope: Repository-wide Go APIs, including internal helper packages.
- Decision 17:
    - Topic: Free-function accessor naming.
    - Rule: Accessor-like free functions use a `Get*` prefix.
    - Rule: For `mux` request accessors, use `GetTasksCtx`, `GetParam`,
      `GetParams`, and `GetSplatValues`.
    - Constraint: Do not force noun-only free-function names when that creates
      awkward type/function naming contortions.
    - Scope: Repository-wide accessor-like package-level functions.
- Decision 18:
    - Topic: Loader/action mark-and-discover naming.
    - Rule: Framework-owned APIs are named `DefineLoaderForRegistration` and
      `DefineActionForRegistration`.
    - Rule: Default bootstrap app-local helper names are `DefineLoader` and
      `DefineAction`.
    - Rule: `DefineLoaderForRegistration` and `DefineActionForRegistration`
      include godoc that explicitly states they mark route declarations for
      build discovery and generated auto-registration, and that the call itself
      does not directly mutate runtime router registration state.
    - Constraint: Do not use `NewLoader` or `NewAction` for these semantics.
    - Scope: Root `vorma` mark APIs and default bootstrap-generated app helper
      wrappers.
- Decision 19:
    - Topic: Loaders/actions handler initialization and side-effect exposure.
    - Rule: Loader-pattern registration/ensure behavior remains internal to
      loader handler initialization.
    - Rule: Do not expose a public `EnsureLoaderPatternsRegistered`-style API.
    - Rule: `LoadersHandler` and `ActionsHandler` use lazy, idempotent handler
      initialization and caching (for example `sync.Once`) so repeated calls do
      not recreate equivalent handlers.
    - Scope: `vormaruntime` handler factory/initializer surfaces for loaders and
      actions.
- Decision 20:
    - Topic: Handler surface naming and signatures.
    - Rule: Handler-producing surfaces use no-verb `*Handler` names.
    - Rule: `(*Wave).StaticHandler` remains the non-panicking constructor
      surface and returns `(http.Handler, error)`.
    - Rule: Panic-wrapper static surfaces use `Must*` names:
      `(*Wave).MustStaticHandler`, `(*Wave).MustStaticMiddleware`, and
      `(*Vorma).MustStaticMiddleware`.
    - Rule: `(*Vorma).GetLoadersHandler` is renamed to
      `(*Vorma).LoadersHandler`.
    - Rule: `(*Vorma).GetActionsHandler` is renamed to
      `(*Vorma).ActionsHandler`.
    - Rule: `(*Vorma).LoadersHandler` and `(*Vorma).ActionsHandler` take no
      router parameters and resolve their routers internally from the owning
      `Vorma` instance.
    - Constraint: Do not keep `Get*Handler` naming for these surfaces.
    - Scope: `wave` and `vormaruntime` handler factory/initializer APIs.
- Decision 21:
    - Topic: `MustGetPort` fail-fast behavior.
    - Rule: `MustGetPort` APIs remain `Must*` and fail fast on unresolved
      port-resolution failure paths.
    - Rule: Dev-mode port resolution does not silently fall back to a possibly
      unavailable port when free-port resolution fails.
    - Rule: Failure paths panic with explicit error context.
    - Constraint: Do not preserve silent fallback behavior that masks resolution
      failures.
    - Scope: `wave.MustGetPort`, `(*PortResolver).MustGetPort`,
      `(*Wave).MustGetPort`, and `vorma.MustGetPort`.
- Decision 22:
    - Topic: `kit/contextutil` method naming.
    - Rule: `GetContextWithValue` is renamed to `ContextWithValue`.
    - Rule: `GetValueFromContext` is renamed to `Value`.
    - Rule: `GetRequestWithContext` is renamed to `RequestWithContextValue`.
    - Constraint: Do not keep `Get*` prefixes on these methods.
    - Scope: `kit/contextutil.Store[T]` method surface.
- Decision 23:
    - Topic: `Must*` naming grammar.
    - Rule: `Must*` APIs are named `Must<Verb><Object>`.
    - Constraint: Never use `Must<Noun>` names.
    - Constraint: The verb must be explicit and grammatical.
    - Scope: Repository-wide `Must*` APIs.

### Locked `mux` Canonicalization Map

- Route registration:
    - `RegisterTaskHandler` -> `AddTaskHandler`
    - `RegisterHandlerFunc` -> `AddHTTPHandlerFunc`
    - `(*Router).RegisterHandlerFunc` -> `(*Router).AddHTTPHandlerFunc`
    - `RegisterHandler` -> `AddHTTPHandler`
    - `(*Router).RegisterHandler` -> `(*Router).AddHTTPHandler`
    - `RegisterNestedTaskHandler` -> `AddNestedTaskHandler`
    - `RegisterNestedPatternWithoutHandler` -> `AddNestedPatternWithoutHandler`
    - `(*NestedRouter).RegisterPatternWithoutHandlerIfMissing` ->
      `(*NestedRouter).AddNestedPatternWithoutHandlerIfMissing`
- Middleware registration:
    - `SetGlobalTaskMiddleware` -> `AddGlobalTaskMiddleware`
    - `SetGlobalHTTPMiddleware` -> `AddGlobalHTTPMiddleware`
    - `(*Router).SetGlobalHTTPMiddleware` -> `(*Router).AddGlobalHTTPMiddleware`
    - `SetMethodLevelTaskMiddleware` -> `AddMethodLevelTaskMiddleware`
    - `SetMethodLevelHTTPMiddleware` -> `AddMethodLevelHTTPMiddleware`
    - `(*Router).SetMethodLevelHTTPMiddleware` ->
      `(*Router).AddMethodLevelHTTPMiddleware`
    - `SetPatternLevelTaskMiddleware` -> `AddPatternLevelTaskMiddleware`
    - `SetPatternLevelHTTPMiddleware` -> `AddPatternLevelHTTPMiddleware`
    - `(*Route[I, O]).SetPatternLevelHTTPMiddleware` ->
      `(*Route[I, O]).AddPatternLevelHTTPMiddleware`
- Request accessors:
    - `TasksCtx` -> `GetTasksCtx`
    - `Param` -> `GetParam`
    - `Params` -> `GetParams`
    - `SplatValues` -> `GetSplatValues`

## Implementation Re-Audit Findings (2026-02-17)

Only non-conforming locked decisions are listed.

- Decision 2:
    - Non-`Must*` APIs still panic on recoverable config/input/runtime failures.
    - Examples: `wave.New` (`wave/wave.go`), `bootstrap.Init`
      (`bootstrap/bootstrap.go`), `vormaruntime.NewVormaApp`
      (`internal/vormaruntime/glue.go`), `csrf.NewProtector`
      (`kit/csrf/csrf.go`), `cookies.NewManager` (`kit/cookies/cookies.go`).

## Open Decision Backlog

None. All currently tracked semantic decisions are locked.

## Decision Log Template

Use this format when a decision is confirmed:

- Decision N:
    - Topic:
    - Rule:
    - Constraint:
    - Scope:
