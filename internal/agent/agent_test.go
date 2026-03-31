package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"google.golang.org/genai"

	"github.com/user/poc-codeact-repoanalyzer/internal/executor"
)

type mockLLM struct {
	responses []string
	calls     int
	err       error
}

func (m *mockLLM) GenerateContent(_ context.Context, _ []*genai.Content, _ *genai.GenerateContentConfig) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	if m.calls >= len(m.responses) {
		return "", fmt.Errorf("mockLLM: no response configured for call %d", m.calls+1)
	}
	text := m.responses[m.calls]
	m.calls++
	return text, nil
}

type mockExecutor struct {
	workDir string
	results []*executor.Result
	runErr  error
	calls   int
}

func (m *mockExecutor) Run(_ context.Context, _ string) (*executor.Result, error) {
	if m.runErr != nil {
		return nil, m.runErr
	}
	if m.calls >= len(m.results) {
		return &executor.Result{Stdout: "ok", ExitCode: 0}, nil
	}
	r := m.results[m.calls]
	m.calls++
	return r, nil
}

func (m *mockExecutor) WorkDir() string {
	if m.workDir == "" {
		return "/tmp/test-workdir"
	}
	return m.workDir
}

func (m *mockExecutor) Close() error { return nil }

func okResult(stdout string) *executor.Result {
	return &executor.Result{Stdout: stdout, ExitCode: 0}
}

func failResult(stderr string, exitCode int) *executor.Result {
	return &executor.Result{Stderr: stderr, ExitCode: exitCode}
}

func TestIsURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		source string
		want   bool
	}{
		{name: "https", source: "https://github.com/user/repo", want: true},
		{name: "http", source: "http://github.com/user/repo", want: true},
		{name: "absolute path", source: "/home/user/myrepo", want: false},
		{name: "relative path", source: "./myrepo", want: false},
		{name: "just name", source: "myrepo", want: false},
		{name: "git protocol", source: "git@github.com:user/repo.git", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := isURL(tt.source)
			if got != tt.want {
				t.Errorf("isURL(%q) = %v, want %v", tt.source, got, tt.want)
			}
		})
	}
}

func TestSetup_URL(t *testing.T) {
	t.Parallel()

	exec := &mockExecutor{
		results: []*executor.Result{okResult("Cloning into 'repo'...")},
	}
	ag := NewAgent(&mockLLM{}, exec)

	err := ag.Setup(context.Background(), "https://github.com/user/repo")
	if err != nil {
		t.Fatalf("Setup() error = %v", err)
	}
	if ag.repoDir != "repo" {
		t.Errorf("repoDir = %q, want %q", ag.repoDir, "repo")
	}
	if ag.history != nil {
		t.Error("Setup() should reset history to nil")
	}
}

func TestSetup_URL_CloneFails(t *testing.T) {
	t.Parallel()

	exec := &mockExecutor{
		results: []*executor.Result{failResult("repository not found", 128)},
	}
	ag := NewAgent(&mockLLM{}, exec)

	err := ag.Setup(context.Background(), "https://github.com/nonexistent/repo")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "clone failed") {
		t.Errorf("error = %q, want 'clone failed'", err.Error())
	}
}

func TestSetup_LocalPath(t *testing.T) {
	t.Parallel()

	exec := &mockExecutor{
		results: []*executor.Result{okResult(""), okResult("")},
	}
	ag := NewAgent(&mockLLM{}, exec)

	err := ag.Setup(context.Background(), "/some/local/repo")
	if err != nil {
		t.Fatalf("Setup() error = %v", err)
	}
	if ag.repoDir != "repo" {
		t.Errorf("repoDir = %q, want %q", ag.repoDir, "repo")
	}
}

func TestSetup_InvalidLocalPath(t *testing.T) {
	t.Parallel()

	exec := &mockExecutor{
		results: []*executor.Result{failResult("", 1)},
	}
	ag := NewAgent(&mockLLM{}, exec)

	err := ag.Setup(context.Background(), "/nonexistent/path")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not a valid directory") {
		t.Errorf("error = %q, want 'not a valid directory'", err.Error())
	}
}

func TestSetup_ResetsHistory(t *testing.T) {
	t.Parallel()

	exec := &mockExecutor{}
	ag := NewAgent(&mockLLM{}, exec)
	ag.history = []*genai.Content{{Role: genai.RoleUser}}

	ag.Setup(context.Background(), "https://github.com/user/repo")

	if ag.history != nil {
		t.Error("Setup() should reset history to nil")
	}
}

// --- Turn ---

func TestTurn_ImmediateAnswer(t *testing.T) {
	t.Parallel()

	llm := &mockLLM{responses: []string{"The project is written in Go."}}
	ag := NewAgent(llm, &mockExecutor{})
	ag.repoDir = "repo"

	result, iterations, err := ag.Turn(context.Background(), "what language is this?")
	if err != nil {
		t.Fatalf("Turn() error = %v", err)
	}
	if result != "The project is written in Go." {
		t.Errorf("result = %q", result)
	}
	if iterations != 1 {
		t.Errorf("iterations = %d, want 1", iterations)
	}
	if len(ag.history) != 2 {
		t.Errorf("history len = %d, want 2", len(ag.history))
	}
}

func TestTurn_WithCodeIteration(t *testing.T) {
	t.Parallel()

	llm := &mockLLM{
		responses: []string{
			"```bash\nls repo/\n```",
			"It has main.go and go.mod.",
		},
	}
	exec := &mockExecutor{
		results: []*executor.Result{okResult("main.go\ngo.mod")},
	}
	ag := NewAgent(llm, exec)
	ag.repoDir = "repo"

	result, iterations, err := ag.Turn(context.Background(), "list files")
	if err != nil {
		t.Fatalf("Turn() error = %v", err)
	}
	if !strings.Contains(result, "main.go") {
		t.Errorf("result = %q, want 'main.go'", result)
	}
	if iterations != 2 {
		t.Errorf("iterations = %d, want 2", iterations)
	}
	if len(ag.history) != 4 {
		t.Errorf("history len = %d, want 4", len(ag.history))
	}
}

func TestTurn_HistoryPersistsAcrossTurns(t *testing.T) {
	t.Parallel()

	llm := &mockLLM{
		responses: []string{
			"It is Go.",
			"The entry is main.",
		},
	}
	ag := NewAgent(llm, &mockExecutor{})
	ag.repoDir = "repo"

	_, _, err := ag.Turn(context.Background(), "what language?")
	if err != nil {
		t.Fatalf("first Turn() error = %v", err)
	}
	histAfterFirst := len(ag.history)

	_, _, err = ag.Turn(context.Background(), "what is the entry point?")
	if err != nil {
		t.Fatalf("second Turn() error = %v", err)
	}

	if len(ag.history) != histAfterFirst+2 {
		t.Errorf("history len = %d, want %d", len(ag.history), histAfterFirst+2)
	}
}

func TestTurn_MaxIterationsForced(t *testing.T) {
	t.Parallel()

	bashBlock := "```bash\nls\n```"
	llm := &mockLLM{
		responses: []string{
			bashBlock,
			bashBlock,
			"Forced answer.",
		},
	}
	ag := NewAgent(llm, &mockExecutor{}, WithMaxIterations(2))
	ag.repoDir = "repo"

	result, iterations, err := ag.Turn(context.Background(), "what is this?")
	if err != nil {
		t.Fatalf("Turn() error = %v", err)
	}
	if iterations != 2 {
		t.Errorf("iterations = %d, want 2", iterations)
	}
	if result != "Forced answer." {
		t.Errorf("result = %q", result)
	}
}

func TestTurn_LLMError(t *testing.T) {
	t.Parallel()

	llm := &mockLLM{err: errors.New("quota exceeded")}
	ag := NewAgent(llm, &mockExecutor{})
	ag.repoDir = "repo"

	_, _, err := ag.Turn(context.Background(), "anything")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestAnalyze_HappyPath(t *testing.T) {
	t.Parallel()

	llm := &mockLLM{
		responses: []string{
			"Let me explore.\n```bash\nls repo/\n```",
			"## Overview\nGo project.\n## Summary\nLooks good.",
		},
	}
	exec := &mockExecutor{
		results: []*executor.Result{
			okResult("cloned"),
			okResult("main.go"),
		},
	}

	ag := NewAgent(llm, exec)
	report, iterations, err := ag.Analyze(context.Background(), "https://github.com/test/repo")

	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if iterations != 2 {
		t.Errorf("iterations = %d, want 2", iterations)
	}
	if !strings.Contains(report, "Overview") {
		t.Errorf("report missing 'Overview': %q", report)
	}
}

func TestAnalyze_CloneFailure(t *testing.T) {
	t.Parallel()

	exec := &mockExecutor{
		results: []*executor.Result{failResult("repository not found", 128)},
	}
	ag := NewAgent(&mockLLM{}, exec)

	_, _, err := ag.Analyze(context.Background(), "https://github.com/nonexistent/repo")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "clone failed") {
		t.Errorf("error = %q, want 'clone failed'", err.Error())
	}
}

func TestAnalyze_LLMError(t *testing.T) {
	t.Parallel()

	llm := &mockLLM{err: errors.New("quota exceeded")}
	exec := &mockExecutor{
		results: []*executor.Result{okResult("cloned")},
	}

	ag := NewAgent(llm, exec)
	_, _, err := ag.Analyze(context.Background(), "https://github.com/test/repo")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- ExtractCodeBlock ---

func TestExtractCodeBlock(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "bash block",
			input: "some text\n```bash\nls -la\n```\nmore text",
			want:  "ls -la",
		},
		{
			name:  "sh block",
			input: "```sh\nfind . -name '*.go'\n```",
			want:  "find . -name '*.go'",
		},
		{
			name:  "no language tag",
			input: "```\necho hello\n```",
			want:  "echo hello",
		},
		{
			name:  "no code block",
			input: "This is just a plain text answer.",
			want:  "",
		},
		{
			name:  "multiline script",
			input: "```bash\nls -la\nfind . -name '*.go'\nwc -l *.go\n```",
			want:  "ls -la\nfind . -name '*.go'\nwc -l *.go",
		},
		{
			name:  "takes only first block",
			input: "```bash\nls\n```\n\n```bash\ncat file\n```",
			want:  "ls",
		},
		{
			name:  "empty block",
			input: "```bash\n\n```",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ExtractCodeBlock(tt.input)
			if got != tt.want {
				t.Errorf("ExtractCodeBlock() = %q, want %q", got, tt.want)
			}
		})
	}
}
