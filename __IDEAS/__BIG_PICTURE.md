From scratch, I’d design Vorma around one rule:

> There is one canonical router state. Everything else is either an action that
> changes it, a callback that observes it, or a framework adapter view of it.

**1. Canonical State**

```ts
type RouterState = {
	phase: "booting" | "ready";
	browser: {
		href: string;
		historyState: unknown;
	};
	route: null | {
		href: string;
		historyState: unknown;
		clientBuildID: string;
		search: unknown;
		params: Record<string, string>;
		splatValues: string[];
		matches: Array<{
			pattern: string;
			input: unknown;
			data: unknown;
			clientData: unknown;
			error: unknown;
		}>;
	};
	work: {
		navigation: null | {
			href: string;
			replace: boolean;
			source: "navigate" | "popstate" | "redirect";
		};
		revalidation: null | {
			phase: "debouncing" | "running" | "retrying";
			attempt: number;
		};
		prefetch: null | {
			href: string;
		};
		submissions: Array<{
			key: string;
			method: string;
			href: string;
			revalidates: boolean;
		}>;
	};
	status: RouterStatus;
};
```

`status` is part of the `RouterState` snapshot, but it is not a separate source
of truth. It is derived from `work` while creating the snapshot.

```ts
type RouterStatus = {
	isNavigating: boolean;
	isRevalidating: boolean;
	isSubmitting: boolean;
	isPrefetching: boolean;
	isBusy: boolean;
};
```

`RouterState` is point-in-time data. So `state.status` is also point-in-time
data. That is the least surprising behavior: the route, work, and status fields
all describe the same instant.

Avoid a live `app.status` object unless usage proves it is needed. A live status
object sitting next to snapshot APIs creates two time models. Reactive adapters
already solve liveness:

```ts
useRouterState((state) => state.status.isBusy);
```

**2. App Runtime API**

```ts
await app.init({
	render: ({ App, el }) => {},
	progressIndicator: { start, stop, isRunning },
	onRouterUpdate: (state) => {},
	onRouteCommit: (commit) => {},
	onClientBuildIDChange: (prev, next) => {},
});
```

`onRouterUpdate` is state-level: work starts, work ends, route commits, etc.

`onRouteCommit` is edge-level: route data was published.

```ts
type RouteCommit = {
	reason: "initial" | "navigation" | "popstate" | "revalidation" | "hmr";
	previous: RouterState["route"];
	next: NonNullable<RouterState["route"]>;
	diff: {
		urlChanged: boolean;
		patternsChanged: boolean;
		paramsChanged: boolean;
		searchChanged: boolean;
		hashChanged: boolean;
		historyStateChanged: boolean;
		loaderDataChanged: boolean[];
		clientDataChanged: boolean[];
	};
};
```

Vanilla reads:

```ts
app.getRouterState();
```

`app.getRouterStatus()` can exist as a convenience alias for
`app.getRouterState().status`, but it should not be treated as its own model. In
the smallest API, skip it until it has proven value.

No public subscription API unless we decide callbacks-at-init is insufficient.

**3. Navigation API** `Href` means the resolved string only. The object form is
a route destination descriptor, not an href.

```ts
type Href = string;

type RouteDestination = {
	pattern: string;
	params?: Record<string, string>;
	splatValues?: string[];
	search?: unknown;
	hash?: string;
};

type RouteTarget = Href | RouteDestination;
```

The conversion helper accepts only the object form and returns the final href
string:

```ts
const href = app.toHref({
	pattern: "/users/:id",
	params: { id },
});
```

Routing APIs accept either a final href string or a destination descriptor:

```ts
app.navigate(target, { replace, scrollToTop, state, skipProgressIndicator });
app.prefetch(target);
app.stopPrefetch(target);
app.revalidate();
```

Adapters use the same target shape, even though the prop is named `href`:

```tsx
<Link href="/about" />
<Link href={{ pattern: "/users/:id", params: { id } }} />
```

`Link` should set `data-active`, `data-pending`, and likely `aria-current` from
`RouterState`. No extra active API.

**4. URL State**

Vorma should treat the three browser URL/state channels as distinct primitives:

```txt
search        = typed server route input
hash          = bookmarkable client-only URL state
history.state = private client-only navigation state
```

Using search params for client-only state is usually the wrong default. The
browser already gives that role to hash when the state should be visible in the
URL, and to history state when the state should be private to a history entry.

Loaders should mirror GET actions: typed input comes from URL search params and
loader output is route data.

```go
type UsersSearch struct {
	Page int `json:"page"`
	Sort string `json:"sort"`
}

vorma.Loader[UsersSearch, UsersPage]{
	Pattern: "/users",
	Handler: func(ctx *app.LoaderCtx[UsersSearch]) (UsersPage, error) {
		search := ctx.Input()
		return load_users(search)
	},
}
```

Routes without search input can use `struct{}` as the input type.

Runtime behavior should reuse the existing GET action mechanics:

```txt
RouteDestination.search object
→ serializeToSearchParams
→ URL query string
→ validate.URLSearchParamsInto
→ ctx.Input()
```

Typegen should also mirror actions. Loader metadata should carry both `__I` and
`__O`, and destination/link/navigate types should use the loader input type for
`search`.

```ts
type MakeTypedLoaderInput<App, Pattern> = ...;

type RouteDestination<P> = {
	pattern: P;
	search?: MakeTypedLoaderInput<App, P>;
};
```

**5. Route Data API** Vanilla:

```ts
app.getRouteData();
app.getLoaderData(pattern);
app.getClientLoaderData(pattern);
app.getRouteParams();
```

Adapter equivalents:

```ts
useRouterState(selector?);
useRouteData();
useLoaderData(pattern?);
useClientLoaderData(pattern?);
useRouteParams();
```

I would keep the hook set small. `useRouterState(selector?)` is the escape
hatch, including for status:

```ts
useRouterState((state) => state.status);
useRouterState((state) => state.status.isBusy);
```

`useRouterStatus()` can exist as a convenience alias, but it should be
documented as a view of `useRouterState((state) => state.status)`, not as a
separate reactive model. The data hooks exist because pattern-keyed loader
access is not sugar; it is core typed route data access.

**6. Client Loader API**

Client loader args should describe the load invocation, not pass global router
state "just in case." If a loader needs the current committed router state, it
can read that from the app binding source.

```ts
type ClientLoaderArgs = {
	signal: AbortSignal;
	reason: "initial" | "navigation" | "revalidation" | "prefetch" | "hmr";
	target: {
		href: string;
		params: Record<string, string>;
		splatValues: string[];
		matchedPatterns: string[];
	};
	serverDataPromise: Promise<{
		matchedPatterns: string[];
		rootData: unknown;
		loaderData: unknown;
		clientBuildID: string;
	}>;
};
```

`target` is intentionally separate from `RouterState.route`. During prefetch,
`RouterState.route` describes the currently committed route, while `target`
describes the route being prepared.

```txt
ClientLoaderArgs = invocation-local facts
RouterState = current committed router snapshot plus current work
serverDataPromise = server data for the invocation target
```

**7. Actions / API Calls** I’d separate semantic reads from writes at the API
level:

```ts
api.query({ pattern, input }, options);
api.mutate({ pattern, input }, options);
```

Defaults:

```txt
query  -> does not revalidate by default
mutate -> revalidates by default
```

Server metadata can override defaults for weird cases like POST reads:

```go
Revalidate: vorma.RevalidateNever
```

Low-level escape hatch:

```ts
app.submit(href, requestInit, { revalidate, dedupeKey, skipProgressIndicator });
```

But generated clients should make most users never touch `submit`.

**8. Server API** From scratch, I’d want server declarations to make intent
explicit:

```go
vorma.Loader[In, Out]{
	Pattern: "/users/:id",
	Handler: func(ctx *app.LoaderCtx[In]) (Out, error) {
		in := ctx.Input()
		return load_user(ctx, in)
	},
}

vorma.Action[In, Out]{
	Method: http.MethodPost,
	Pattern: "/api/thing",
	Revalidate: vorma.RevalidateDefault,
	Handle: func(ctx, in) (out, error) {},
}
```

Cache policy belongs directly on request/loader/action context:

```go
ctx.Cache().Public(time.Second)
ctx.Cache().Private(time.Second)
ctx.Cache().NoStore()
```

**The Shape** So the whole API stack is:

```txt
RouterState
→ app.getRouterState / onRouterUpdate
→ useRouterState
→ status selectors / Link data attrs / route data hooks
```

And:

```txt
RouteDestination
→ app.toHref
→ Href string
```

And:

```txt
RouteTarget = Href | RouteDestination
→ app.navigate / app.prefetch
→ Link href
```

And:

```txt
Action metadata
→ api.query / api.mutate
→ default revalidation behavior
```

And:

```txt
ClientLoaderArgs
→ reason / target / serverDataPromise
→ no duplicated global router state in loader args
```

That gives Vorma a small number of real primitives without making every later
feature invent its own model.
