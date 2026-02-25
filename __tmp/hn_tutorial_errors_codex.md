# HN Tutorial Corrections Checklist (Codex)

Scope: `__tmp/vorma-hn-clone-react-tutorial.draft.md`

## Fixes Required

- [ ] **Prerequisite Node version is too loose.** Change `Node.js 22+` to
      `Node.js 22.11+`. Refs: draft line 8;
      `typescript/vorma/create/main.ts:75-83`.

- [ ] **Hardcoded `pnpm` commands conflict with scaffold package-manager
      choice.** Replace hardcoded `pnpm dev` / `pnpm build` usage with
      package-manager-agnostic wording. Refs: draft lines 59, 1271, 2950,
      3037-3038; `typescript/vorma/create/main.ts:269-277`;
      `bootstrap/bootstrap.go:92-132`.

- [ ] **Project-creation prompt list omits package-manager selection.** Add the
      package manager prompt to the “When prompted” section. Refs: draft lines
      47-52; `typescript/vorma/create/main.ts:269-277`.

- [ ] **`MustInitWithDefaultRouter()` behavior is misstated.** Remove claim that
      it runs code generation; it initializes runtime and mounts handlers. Refs:
      draft lines 210-212; `internal/vormaruntime/vormaruntime.go:1587-1609`.

- [ ] **Section 11.10 refetch behavior is incorrect.** Do not claim parent/root
      loaders are skipped due “already has data.” Matched loaders are executed
      for route-data requests. Refs: draft lines 1591-1597;
      `internal/vormaruntime/vormaruntime.go:612-616`;
      `kit/nestedmux/nestedmux.go:356-432`.

- [ ] **Section 11.10 overstates “zero caching.”** If you keep an implementation
      note, clarify there is route-data metadata caching, but not selective
      skipping of matched loader execution. Refs:
      `internal/vormaruntime/runtimehttp/runtimehttp.go:369-383`;
      `internal/vormaruntime/routepipeline/routepipeline.go:326-361`.

- [ ] **Mutation examples perform redundant refresh navigations.** Current
      submit runtime auto-revalidates non-GET by default; examples that mutate
      then navigate just to refresh should be rewritten (or pass
      `options: { revalidate: false }`). Refs: draft lines 905-910, 1042-1046;
      `typescript/vorma/client/src/core/navigation/runtime_submit.ts:194-203, 382-386`.

- [ ] **Validation-error handling example is inaccurate for current client
      runtime.** `result.error` is currently the status string for non-OK
      responses (e.g. `"400"`), not guaranteed validator message text. Refs:
      draft lines 2241-2254;
      `typescript/vorma/client/src/core/navigation/runtime_submit.ts:367-369`;
      `typescript/vorma/client/src/tests/contracts/client.submit_and_redirect.contract.test.ts:1350-1368`.

- [ ] **Client loader import path is wrong.** `addClientLoader` is not exported
      from `vorma/client`; use typed adapter helper wiring (typically via
      `vorma.app.tsx`). Refs: draft line 2708;
      `typescript/vorma/client/index.ts:6-62`;
      `bootstrap/tmpls/frontend_app_tsx_tmpl.txt:10-15,52`.

- [ ] **Client loader semantics are misstated.** Client-loader output does not
      replace `useLoaderData`; it is accessed through the hook returned by
      `addClientLoader`. Refs: draft lines 2728-2730;
      `typescript/vorma/ui-adapters/react/src/helpers.ts:38-44,83-99`.

- [ ] **Production build section is not aligned with scaffold’s blessed flow.**
      Present `build` script / `go run ./backend/cmd/build` as primary flow
      instead of a mandatory two-step manual sequence. Refs: draft lines
      2945-2953; `bootstrap/tmpls/package_json_tmpl.txt:5-6`;
      `vormabuild/buildflow/buildflow.go:403-417`;
      `wave/wavebuild/builder/builder.go:324-326`.

- [ ] **Docker example is not scaffold-aligned.** Update to
      package-manager-aware scaffold pattern and build pipeline behavior. Refs:
      draft lines 3026-3040; `bootstrap/tmpls/dockerfile_tmpl.txt:1-10`;
      `bootstrap/bootstrap.go:92-132,252-261`.

- [ ] **Task dedup-key mechanism wording is inaccurate.** Do not describe
      context as a field in the task key. The key is `(taskPtr, input)` and
      dedup scope is per `tasks.Ctx` map. Refs: draft line 1868;
      `kit/tasks/tasks.go:59-67`.

- [ ] **`setupGlobalLoadingIndicator.include` typing should be explicit.**
      Clarify that `include` accepts `"all"` or an array of specific kinds.
      Refs: draft lines 2407-2409;
      `typescript/vorma/client/src/core/extras.ts:258-290`.

- [ ] **Critical/non-critical CSS statement needs precision.** `Critical` is
      inlined (style element), but `NonCritical` currently renders as a normal
      stylesheet link (no explicit async-load attributes in generated element).
      Refs: draft lines 3203-3205;
      `internal/vormaruntime/rendering/rendering.go:290-291`;
      `wave/internal/waveruntime/waveruntime.go:80-109`.

- [ ] **Response-control coverage should include blessed ReqData helper
      methods.** Add `c.Redirect`, `c.SetResponseStatus`, `c.SetResponseHeader`,
      `c.AddResponseHeader`, `c.SetResponseCookie` alongside proxy discussion.
      Refs: draft lines 2262-2323; `kit/mux/mux.go:1082-1117`.

## Verified Accurate (No Change Needed)

- [x] **Proxy merge rule summary is correct** (`first error wins`, otherwise
      `last success`, `first redirect wins unless error`). Refs: draft lines
      2347-2353; `kit/response/response.go:600-634`.

- [x] **Middleware-order statement is correct.** First registered middleware is
      outermost, so it runs first on request and last on response. Refs: draft
      lines 2908-2909; `kit/mux/mux.go:633-642`.
