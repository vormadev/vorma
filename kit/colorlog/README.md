# kit/colorlog

`github.com/vormadev/vorma/kit/colorlog`

Color-aware `log/slog` handler for human-readable terminal logs.

## Import

```go
import "github.com/vormadev/vorma/kit/colorlog"
```

## Quick Start

```go
log := colorlog.New("api")

log.Info("server started", "port", 3000)
log.Warn("cache miss", "key", "user:42")
log.Error("db failed", "err", err)
```

## Options

```go
logger := colorlog.New("worker", colorlog.Options{
	Output: os.Stdout,
	Level:  slog.LevelWarn,
	// nil = auto-detect terminal; true/false = force
	UseColor: nil,
})
```

- `Output`: target writer. Defaults to `os.Stdout`.
- `Level`: minimum level emitted by handler.
- `UseColor`:
  - `nil`: color only when `Output` is an `*os.File` tty.
  - `true`: always emit ANSI colors.
  - `false`: never emit ANSI colors.

## Output Shape

Each line is rendered as:

`<timestamp>  (<label>)  <level-prefix><message>  [key = value] ...`

Examples:

- info: `2026/02/07 12:34:56  (api)  started`
- warn: `2026/02/07 12:34:56  (api)  WARNING  cache miss`
- error: `2026/02/07 12:34:56  (api)  ERROR  db failed`
- debug: `2026/02/07 12:34:56  (api)  DEBUG  details`

## Structured Logging

```go
log := colorlog.New("api")

log.With("request_id", "r-123").Info("accepted")
log.WithGroup("http").Info("done", "status", 200, "ms", 12)
// grouped keys render as: http.status http.ms
```

## Concurrency Notes

- Handler clones created by `WithAttrs` and `WithGroup` share a mutex.
- Concurrent writes from cloned loggers stay line-safe (no interleaving within a line).

## API Reference

### Types

- `type ColorLogHandler`
- `type Options`

### Constructors

- `func New(label string, opts ...Options) *slog.Logger`

### `ColorLogHandler` methods

- `func (h *ColorLogHandler) Enabled(_ context.Context, level slog.Level) bool`
- `func (h *ColorLogHandler) Handle(_ context.Context, r slog.Record) error`
- `func (h *ColorLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler`
- `func (h *ColorLogHandler) WithGroup(name string) slog.Handler`
