---
title: Using Vorma With A Third-Party Router (Like Chi)
description: Using Vorma with a third-party router like Chi
---

The Vorma framework's HTTP layer is built on top of its own lower-level
`vorma/kit/mux` package, and for most projects, we recommend just using that for
your core router. It's fast, simple, flexible, and works out of the box. This is
what the Vorma bootstrapper CLI will set up for you.

However, third-party routers (such as [Chi](https://go-chi.io/)) are fully
supported as well. At the end of the day, Vorma boils down to standard
`http.Handler` instances that you can mount anywhere. All you need to do is add
`vorma.EnableThirdPartyRouter` in your middleware stack.

For example, here is how you might use Vorma with Chi:

```go
// backend/src/router/router.go

package router

import (
    "your-app/backend"

    "github.com/go-chi/chi/v5"
    "github.com/vormadev/vorma"
    "github.com/vormadev/vorma/kit/middleware/healthcheck"
)

var App = vorma.NewVormaApp(vorma.VormaAppConfig{
    Wave: backend.Wave,
    // ... your config
})

func Init() (addr string, handler http.Handler) {
    App.Init()

    r := chi.NewRouter()
    loaders, actions := App.Loaders(), App.Actions()

    // Apply global middlewares
    r.Use(App.ServeStatic())
    r.Use(healthcheck.Healthz)
    r.Use(vorma.EnableThirdPartyRouter) // <-- KEY PIECE

    // Register GET handler for loaders
    r.Method("GET", loaders.HandlerMountPattern(), loaders.Handler())

    // Register handlers for API methods
    for method := range actions.SupportedMethods() {
        r.Method(
            method,
            actions.HandlerMountPattern(),
            actions.Handler(),
        )
    }

    return App.ServerAddr(), r
}
```
