# Framework Papercuts Noticed During E2E Work

- `App.ServerAddr()` is not reliable until after
  `MustInit()`/`MustInitWithDefaultRouter()` runs. Reading it earlier silently
  yields an empty address and can lead to wrong/implicit listen behavior.
- `DefineActionForRegistration` requires the method argument to resolve to a
  compile-time string literal for build-time discovery. Passing a wrapper
  parameter or `http.MethodPost` fails discovery in a way that is not obvious at
  callsite.
- `MustInitWithDefaultRouter()` does not automatically include `/healthz`. That
  makes framework-level dev/test harness boot checks easy to misconfigure unless
  health middleware is explicitly added.
- The generated first-party `api.mutate` helper returns `SubmitResult`
  (`{ success, data|error }`), not raw payload data. This is easy to misuse and
  the handling pattern needs to be explicit in scaffold/docs/examples.
- Loader failures surface in route error boundaries as a generic message
  (`An error occurred`) instead of backend error text. Debugging requires
  checking logs/traces, so this behavior needs clearer developer guidance.
- Actions are mounted under `/api` via generated config, but direct action paths
  can appear to work in some contexts. This can create confusion in custom
  clients/harnesses unless URL construction rules are followed strictly.
- Failed/interrupted dev runs can leave wave/vite/runtime processes alive,
  causing `another wave dev process is already running` on subsequent boots.
  Cleanup semantics for interrupted test/dev sessions could be more robust.
