package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// mockRunner satisfies the turnRunner interface for testing repl().
type mockRunner struct {
	responses []string
	calls     int
}

func (m *mockRunner) Turn(_ context.Context, _ string) (string, int, error) {
	if m.calls >= len(m.responses) {
		return "default response", 1, nil
	}
	r := m.responses[m.calls]
	m.calls++
	return r, 1, nil
}

func TestRepl_ExitCommand(t *testing.T) {
	t.Parallel()

	in := strings.NewReader("exit\n")
	var out bytes.Buffer
	code := repl(context.Background(), &mockRunner{}, in, &out)

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if got := out.String(); !strings.HasPrefix(got, "> ") {
		t.Errorf("expected prompt '> ', got %q", got)
	}
}

func TestRepl_QuitCommand(t *testing.T) {
	t.Parallel()

	in := strings.NewReader("quit\n")
	var out bytes.Buffer
	code := repl(context.Background(), &mockRunner{}, in, &out)

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
}

func TestRepl_EOF(t *testing.T) {
	t.Parallel()

	in := strings.NewReader("") // immediate EOF
	var out bytes.Buffer
	code := repl(context.Background(), &mockRunner{}, in, &out)

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
}

func TestRepl_EmptyFirstTurn_UsesDefault(t *testing.T) {
	t.Parallel()

	runner := &mockRunner{responses: []string{"health report result"}}
	in := strings.NewReader("\nexit\n") // empty Enter, then exit
	var out bytes.Buffer
	repl(context.Background(), runner, in, &out)

	if runner.calls != 1 {
		t.Errorf("Turn() called %d times, want 1", runner.calls)
	}
	if !strings.Contains(out.String(), "health report result") {
		t.Errorf("output missing expected response: %q", out.String())
	}
}

func TestRepl_EmptySubsequentTurnsSkipped(t *testing.T) {
	t.Parallel()

	runner := &mockRunner{responses: []string{"first answer"}}
	// First real turn, then two empty lines, then exit.
	in := strings.NewReader("what is this?\n\n\nexit\n")
	var out bytes.Buffer
	repl(context.Background(), runner, in, &out)

	if runner.calls != 1 {
		t.Errorf("Turn() called %d times, want 1 (empty lines should be skipped)", runner.calls)
	}
}

func TestRepl_MultipleTurns(t *testing.T) {
	t.Parallel()

	runner := &mockRunner{responses: []string{"Go project", "main.go"}}
	in := strings.NewReader("what language?\nentry point?\nexit\n")
	var out bytes.Buffer
	repl(context.Background(), runner, in, &out)

	if runner.calls != 2 {
		t.Errorf("Turn() called %d times, want 2", runner.calls)
	}
	output := out.String()
	if !strings.Contains(output, "Go project") {
		t.Errorf("output missing 'Go project': %q", output)
	}
	if !strings.Contains(output, "main.go") {
		t.Errorf("output missing 'main.go': %q", output)
	}
}
