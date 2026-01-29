---
title: colorlog
description:
    Colored slog.Handler for terminal output with level filtering and
    auto-detection.
---

```go
import "github.com/vormadev/vorma/kit/colorlog"
```

## Constructor

Returns `*slog.Logger` using the colored handler:

```go
func New(label string, opts ...Options) *slog.Logger
```

Examples:

```go
log := colorlog.New("app")
log := colorlog.New("app", colorlog.Options{Level: slog.LevelWarn})
```

## Options

Optional configuration (all fields have sensible defaults):

```go
type Options struct {
    Output   io.Writer   // default: os.Stdout
    Level    slog.Level  // minimum level; default: LevelInfo (0)
    UseColor *bool       // nil = auto-detect from terminal
}
```

## Handler

`ColorLogHandler` implements `slog.Handler`. Supports `WithAttrs` and
`WithGroup`.
