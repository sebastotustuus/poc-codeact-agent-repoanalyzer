package executor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"
)

const (
	DefaultTimeout = 30 * time.Second
	maxOutputBytes = 10 * 1024 // 10 KB
)

type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
	TimedOut bool
}

type Executor struct {
	workDir string
	timeout time.Duration
}

type ExecutorOption func(*Executor)

func WithTimeout(d time.Duration) ExecutorOption {
	return func(e *Executor) { e.timeout = d }
}

func NewExecutor(opts ...ExecutorOption) (*Executor, error) {
	dir, err := os.MkdirTemp("", "repoanalyzer-*")
	if err != nil {
		return nil, fmt.Errorf("executor: create temp dir: %w", err)
	}
	e := &Executor{
		workDir: dir,
		timeout: DefaultTimeout,
	}
	for _, opt := range opts {
		opt(e)
	}
	return e, nil
}

func (e *Executor) Run(ctx context.Context, script string) (*Result, error) {
	if err := ValidateCommand(script); err != nil {
		return nil, err
	}

	runCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, "bash", "-c", script)
	cmd.Dir = e.workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()

	result := &Result{
		Stdout: truncate(stdout.String(), maxOutputBytes),
		Stderr: truncate(stderr.String(), maxOutputBytes),
	}

	if runCtx.Err() == context.DeadlineExceeded {
		result.TimedOut = true
		return result, nil
	}

	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
		return result, nil
	}

	if runErr != nil {
		return result, fmt.Errorf("executor: run: %w", runErr)
	}

	return result, nil
}

func (e *Executor) WorkDir() string {
	return e.workDir
}

func (e *Executor) Close() error {
	if err := os.RemoveAll(e.workDir); err != nil {
		return fmt.Errorf("executor: close: %w", err)
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "\n[...output truncated]"
}
