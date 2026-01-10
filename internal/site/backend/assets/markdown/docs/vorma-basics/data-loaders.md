---
title: Data Loaders
description: How Vorma data loaders work, and how to use them
order: 4
---

## Nested Routing

Similar to React Router, Vorma uses _nested_ data loaders, tied to URL segments,
to preload the data needed for your routes in parallel.

In Vorma, this is always done in a single request, and all response headers get
merged according to a safe and predictable formula. This makes data
drift/inconsistency/staleness across route segments impossible. It's also an
incredibly nice mental model, as it's extremely similar to how you reason about
a traditional MPA.

If this is problematic for certain use cases, Vorma also provides a client
loaders primitive (more on this later) that you can use to break down your data
fetching more granularly.

Another benefit of the single-request model is that you can coalesce/memoize
your data fetching across your route segments to avoid duplicate work. Vorma is
built on a fundamental primitive it calls `tasks` (from the `vorma/kit/tasks`
package) that makes this possible. Here's the
[documentation on tasks](/docs/kit/tasks) if you want to read more. In short,
the `tasks` primitive allows you to do things like check user authentication in
every relevant function across your entire app while knowing it's guaranteed to
only actually run once per request, side-stepping the Go equivalent of the
"prop-drilling" problem entirely. To be clear, you don't have to use `tasks` --
but it's there if you want it.

### Segment Types

In Vorma, there are four URL segment types:

1. Static
2. Dynamic
3. Index
4. Splat

#### Static Segments

A static segment is just a static string of URL-safe characters (other than your
explicit index segment, which we'll get into below), such as "posts". In the URL
pattern `"/posts"`, there is one URL segment: `["posts"]`.

##### Base Slash Route ("/")

The base slash route (`"/"`) is a special kind of static segment. You don't have
to register routes at the `"/"` route, but if you do, it will act as an outer
layout for your whole app.

From the server perspective, this can be useful for passing user data, feature
flags, or environment variables into all pages of your app. From the client
perspective, this can be useful for providing an outer UI layout for your entire
app.

What pattern do you register for the home/landing page then? Well, you would use
an index segment, such as `"/_index"` (more on index segments below).

<lightbulb>
The loader data for your base slash route is also known in Vorma as
your app's `RootData`.
</lightbulb>

#### Dynamic Segments

A dynamic segment is a URL pattern segment that starts with a non-URL-safe
character designated as your dynamic params prefix. Be default, this is the
character/rune `':'`, but this is configurable if you want.

#### Index Segments

An index segment represents the default child of any parent route (for our
purposes here, a parent route is simply any route that renders a child). It is
signified by an explicit marker (by default, that explicit marker is `_index`,
but you can change this if you want).

Let's take a quick detour to explain how a parent route can actually render a
child. It's very simple. It simply renders `<props.Outlet />` anywhere in its
TSX. Unlike in React Router, where `Outlet` is a component you import from the
npm package, in Vorma, Outlet is just a prop passed to all Vorma routes.

It looks something like this:

```tsx
import type { RouteProps } from "../vorma.gen/index.ts";
import { useRouterData } from "../vorma.utils.tsx";

export function Posts(props: RouteProps<"/posts">) {
	return (
		<PostsLayout>
			<props.Outlet />
		</PostsLayout>
	);
}

export function PostsIndex(props: RouteProps<"/posts/_index">) {
	return <AllPosts />;
}

export function Post(props: RouteProps<"/posts/:post_id">) {
	const routerData = useRouterData(props); // type-safe!

	return <Post id={routerData.params.postID} />;
}
```

In the above example, if the `/posts` pattern has a child route that is matched
in the current URL, then that child route will be rendered wherever
`props.Outlet` is.

So if the URL is `/posts`, then the matched patterns would be
`["/posts", "/posts/_index"]`, and the effective rendered output would be this:

```tsx
<PostsLayout>
	<PostsIndex />
</PostsLayout>
```

Whereas if the URL is `/posts/post-1`, then the matched patterns would be
`["/posts", "/posts/:post_id"]`, and the effective rendered output would be
this:

```tsx
<PostsLayout>
	<Post id="post-1" />
</PostsLayout>
```

---

[TO BE CONTINUED]
