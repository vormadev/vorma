package wave

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const (
	defaultConfigProviderTimeout      = 10 * time.Second
	defaultConfigProviderMaxStdoutBts = 4 * 1024 * 1024
)

type ConfigProviderInvocation struct {
	Command      string
	Args         []string
	WorkingDir   string
	Env          []string
	Timeout      time.Duration
	MaxStdoutBts int
}

type ConfigProviderRunner struct {
	execCommandContext func(context.Context, string, ...string) *exec.Cmd
}

func NewConfigProviderRunner() *ConfigProviderRunner {
	return &ConfigProviderRunner{
		execCommandContext: exec.CommandContext,
	}
}

func (runner *ConfigProviderRunner) Run(
	ctx context.Context,
	invocation ConfigProviderInvocation,
) (*ConfigProviderPayload, error) {
	if runner == nil {
		return nil, fmt.Errorf("provider runner is nil")
	}
	if strings.TrimSpace(invocation.Command) == "" {
		return nil, fmt.Errorf("provider command is required")
	}

	timeout := invocation.Timeout
	if timeout <= 0 {
		timeout = defaultConfigProviderTimeout
	}
	maxStdoutBts := invocation.MaxStdoutBts
	if maxStdoutBts <= 0 {
		maxStdoutBts = defaultConfigProviderMaxStdoutBts
	}

	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := runner.execCommandContext(runCtx, invocation.Command, invocation.Args...)
	if invocation.WorkingDir != "" {
		cmd.Dir = invocation.WorkingDir
	}
	if len(invocation.Env) > 0 {
		cmd.Env = append(cmd.Environ(), invocation.Env...)
	}

	limitedStdoutBuffer := &limitedBuffer{maxBytes: maxStdoutBts}
	var stderrBuffer bytes.Buffer
	cmd.Stdout = limitedStdoutBuffer
	cmd.Stderr = &stderrBuffer

	if err := cmd.Run(); err != nil {
		if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
			return nil, fmt.Errorf("provider command timed out after %s", timeout)
		}
		if errors.Is(err, errLimitedBufferOverflow) {
			return nil, fmt.Errorf("provider stdout exceeded %d bytes", maxStdoutBts)
		}
		if stderrText := strings.TrimSpace(stderrBuffer.String()); stderrText != "" {
			return nil, fmt.Errorf("provider command failed: %w: %s", err, stderrText)
		}
		return nil, fmt.Errorf("provider command failed: %w", err)
	}

	if limitedStdoutBuffer.overflowed {
		return nil, fmt.Errorf("provider stdout exceeded %d bytes", maxStdoutBts)
	}

	payload, err := ParseConfigProviderPayload(limitedStdoutBuffer.Bytes())
	if err != nil {
		return nil, fmt.Errorf("provider payload invalid: %w", err)
	}

	return payload, nil
}

var errLimitedBufferOverflow = errors.New("limited buffer overflow")

type limitedBuffer struct {
	buffer     bytes.Buffer
	maxBytes   int
	overflowed bool
}

func (limitedBuffer *limitedBuffer) Write(data []byte) (int, error) {
	if limitedBuffer.overflowed {
		return 0, errLimitedBufferOverflow
	}
	if limitedBuffer.buffer.Len()+len(data) > limitedBuffer.maxBytes {
		limitedBuffer.overflowed = true
		return 0, errLimitedBufferOverflow
	}
	return limitedBuffer.buffer.Write(data)
}

func (limitedBuffer *limitedBuffer) Bytes() []byte {
	return limitedBuffer.buffer.Bytes()
}
