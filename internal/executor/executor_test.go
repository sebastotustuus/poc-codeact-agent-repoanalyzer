package executor

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestNewExecutor(t *testing.T) {
	t.Parallel()

	exec, err := NewExecutor()
	if err != nil {
		t.Fatalf("NewExecutor() error = %v", err)
	}
	defer exec.Close()

	if exec.WorkDir() == "" {
		t.Error("WorkDir() returned empty string")
	}
}

func TestExecutorRun(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		script     string
		wantStdout string
		wantExit   int
		wantErr    bool
	}{
		{
			name:       "simple echo",
			script:     "echo hello",
			wantStdout: "hello\n",
			wantExit:   0,
		},
		{
			name:     "non-zero exit",
			script:   "exit 42",
			wantExit: 42,
		},
		{
			name:    "blocked command",
			script:  "rm -rf /",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			exec, err := NewExecutor()
			if err != nil {
				t.Fatalf("NewExecutor() error = %v", err)
			}
			defer exec.Close()

			result, err := exec.Run(context.Background(), tt.script)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Run() unexpected error = %v", err)
			}
			if result.Stdout != tt.wantStdout {
				t.Errorf("Stdout = %q, want %q", result.Stdout, tt.wantStdout)
			}
			if result.ExitCode != tt.wantExit {
				t.Errorf("ExitCode = %d, want %d", result.ExitCode, tt.wantExit)
			}
		})
	}
}

func TestExecutorTimeout(t *testing.T) {
	t.Parallel()

	exec, err := NewExecutor(WithTimeout(100 * time.Millisecond))
	if err != nil {
		t.Fatalf("NewExecutor() error = %v", err)
	}
	defer exec.Close()

	result, err := exec.Run(context.Background(), "sleep 60")
	if err != nil {
		t.Fatalf("Run() unexpected error = %v", err)
	}
	if !result.TimedOut {
		t.Error("expected TimedOut = true")
	}
}

func TestTruncate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		limit int
		want  string
	}{
		{name: "no truncation", input: "hello", limit: 10, want: "hello"},
		{name: "exact limit", input: "hello", limit: 5, want: "hello"},
		{name: "truncated", input: "hello world", limit: 5, want: "hello\n[...output truncated]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := truncate(tt.input, tt.limit)
			if !strings.HasPrefix(got, tt.want) && got != tt.want {
				t.Errorf("truncate() = %q, want %q", got, tt.want)
			}
		})
	}
}
