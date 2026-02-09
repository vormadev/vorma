# kit/grace

`github.com/vormadev/vorma/kit/grace`

`grace` provides lifecycle orchestration for long-running Go processes:

- startup execution
- OS signal handling
- shutdown callbacks with timeout context
- optional helper to terminate child processes cleanly

## Import

```go
import "github.com/vormadev/vorma/kit/grace"
```

## Quick Start

```go
srv := &http.Server{Addr: ":8080", Handler: router}

grace.Orchestrate(grace.OrchestrateOptions{
	StartupCallback: func() error {
		err := srv.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	},
	ShutdownCallback: func(ctx context.Context) error {
		return srv.Shutdown(ctx)
	},
})
```

## Orchestration Model

- `StartupCallback` runs first. It should block while your app is running.
- On signal (or startup failure), `ShutdownCallback` runs with a timeout
  context.
- Defaults:
- `ShutdownTimeout = 30s`
- signals: `SIGHUP`, `SIGINT`, `SIGTERM`, `SIGQUIT` (Windows: `os.Interrupt`)
- logger: package default logger

Practical guidance:

- return errors from callbacks; do not call `os.Exit`/`log.Fatal` inside
  callbacks
- if startup exits quickly, `Orchestrate` still waits for signal-triggered
  shutdown flow
- for `http.Server`, treat `http.ErrServerClosed` as normal shutdown, not a
  startup failure

## Terminating Child Processes

`TerminateProcess` sends a graceful termination signal and waits up to
`timeToWait`.  
If the process has not exited, it force-kills it.

```go
if err := grace.TerminateProcess(cmd.Process, 5*time.Second, logger); err != nil {
	return err
}
```

## API Coverage

### Types

- `type OrchestrateOptions`

### Exported Struct Fields

- `OrchestrateOptions.Logger *slog.Logger`
- `OrchestrateOptions.ShutdownCallback func(context.Context) error`
- `OrchestrateOptions.ShutdownTimeout time.Duration`
- `OrchestrateOptions.Signals []os.Signal`
- `OrchestrateOptions.StartupCallback func() error`

### Functions

- `func Orchestrate(options OrchestrateOptions)`
- `func TerminateProcess(process *os.Process, timeToWait time.Duration, logger *slog.Logger) error`
