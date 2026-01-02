---
title: Zero-Config Vercel Skew Protection With Vorma
description: How Vercel Skew Protection works with Vorma apps
date: Sep 1, 2025
tags: ["Vercel"]
---

When you deploy your Vorma apps to Vercel, you can enable Vercel's
[Skew Protection](https://vercel.com/docs/skew-protection) in one click from
your Vercel dashboard, with zero additional configuration required.

If you're curious how it works under the hood, read on.

## Vorma Build IDs

By default, regardless of where you host your app, Vorma will always assign your
production deployments a deterministic build ID. Vorma build IDs are derived
from three things:

1. Your HTML template
1. Your total routing configuration
1. Your public static assets

### Automatic Safe Hard Reloads

Whenever a Vorma client navigates to a new route (or performs a route data
revalidation) with an outdated build ID, Vorma will automatically hard reload
the page to ensure it has the latest HTML template, application entry module,
and global CSS.

This will feel basically seamless for end users, as it simply temporarily
downgrades to a multi-page app (MPA) experience for the duration of a single
navigation or revalidation. No big deal.

### Event Dispatch In Edge Cases

In other situations (namely, upon any API query response or failed API mutation
response), Vorma can't know whether it's a safe and non-disruptive time to hard
reload the page. So instead of forcing a hard reload, it will just dispatch an
event indicating that a new build is available. Application developers can
listen for this event and handle the situation however they like:

```ts
import { addBuildIDListener } from "vorma/client";

addBuildIDListener(({ oldID, newID }) => {
	// do something, such as:
	// - hard reload
	// - show a toast to the user
	// - save data to localStorage, then hard reload
	// - etc.
});
```

## Vercel Skew Protection

While Vorma handles all this pretty well on its own, Vercel's Skew Protection
makes the situation much better.

Vercel Skew Protection is a clever system that keeps your prior deployments
alive for a certain period of time to reduce the odds of failed requests from
outdated clients.

Vorma supports Vercel Skew Protection natively, with zero configuration
required. All you need to do is turn the setting on in your Vercel dashboard
(available to Vercel Pro and Enterprise teams).

### How It Works

When Vorma generates any fresh HTML page, it will check to see if the
`VERCEL_SKEW_PROTECTION_ENABLED` environment variable has the value `1`. If it
does, Vorma will inject the value of the `VERCEL_DEPLOYMENT_ID` environment
variable into the HTML payload, making it available to the downstream Vorma
client application.

Normal navigations will still hard reload at the first opportunity, as this is a
guaranteed safe and non-disruptive time to load the fresh build.

However, whenever the client makes an API call, it will include the
`VERCEL_DEPLOYMENT_ID` value as a header in the request (with a key of
`x-deployment-id`). This tells Vercel's infrastructure to route the request to a
deployment that matches the current client.

Assuming that (i)&nbsp;you are still within the keep-alive period for Vercel
Skew Protection and (ii)&nbsp;no fundamentally incompatible database or similar
migrations have occurred, user requests from the outdated client should still
succeed, even if your application's latest deployment "broke" the previous
server-client contract.

Further, on route data revalidations, Vorma will include the
`VERCEL_DEPLOYMENT_ID` as a search param to the request URL (with a search param
key of `dpl`), so that the revalidation cycle is also handled by a Vercel
deployment that matches the current client. If a redirect happens to occur at
this point, then that's a great opportunity for a hard reload (and that's
exactly what Vorma does).

This selective use of Vercel's Skew Protection by Vorma prevents the most
disruptive reloads (_e.g._, during background operations), while still allowing
most user requests to succeed (even from outdated clients), all while still
allowing the app to upgrade itself via a hard reload at the earliest non-jarring
opportunity.

In other words, the vast majority of users will experience zero errors and zero
flow-disrupting reloads or notifications while they use your application, even
across incompatible deployments.

---

If you haven't tried Vorma yet, go give it a try, and be sure to select Vercel
for your deployment target if you want to try out Skew Protection:

```sh
npm create vorma@latest
```

## More Resources

- [Vorma Docs](/docs)
- [Vercel Skew Protection Docs](https://vercel.com/docs/skew-protection)
