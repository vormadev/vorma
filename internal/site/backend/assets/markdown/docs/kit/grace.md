---
title: grace
description:
    Application lifecycle orchestration with graceful shutdown and signal
    handling.
---

```go
import "github.com/vormadev/vorma/kit/grace"
```

## Orchestrate

Manages startup, shutdown, and OS signal handling:

```go
func Orchestrate(options OrchestrateOptions)
```

Options:

```go
type OrchestrateOptions struct {
    ShutdownTimeout  time.Duration              // default: 30s
    Signals          []os.Signal                // default: SIGHUP, SIGINT, SIGTERM, SIGQUIT (Windows: Interrupt only)
    Logger           *slog.Logger               // default: colorlog to stdout
    StartupCallback  func() error               // blocking main logic (e.g., server.ListenAndServe)
    ShutdownCallback func(context.Context) error // cleanup logic with timeout context
}
```

Example:

```go
grace.Orchestrate(grace.OrchestrateOptions{
    StartupCallback:  func() error { return server.ListenAndServe() },
    ShutdownCallback: func(ctx context.Context) error { return server.Shutdown(ctx) },
})
```

## TerminateProcess

Gracefully terminate a process with timeout, then force kill:

```go
func TerminateProcess(process *os.Process, timeToWait time.Duration, logger *slog.Logger) error
```
