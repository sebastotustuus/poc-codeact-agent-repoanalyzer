package agent

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"google.golang.org/genai"

	"github.com/user/poc-codeact-repoanalyzer/internal/executor"
	"github.com/user/poc-codeact-repoanalyzer/internal/gemini"
)

// codeBlockRe matches the first fenced bash/sh code block in an LLM response.
// (?s) enables dot-all so the content can span multiple lines.
var codeBlockRe = regexp.MustCompile("(?s)```(?:bash|sh)?\n(.*?)```")

// LLM is the interface required by Agent to call the language model.
// Defined at point of use, not in the gemini package.
type LLM interface {
	GenerateContent(ctx context.Context, contents []*genai.Content, config *genai.GenerateContentConfig) (string, error)
}

// CommandExecutor is the interface required by Agent to run shell commands.
type CommandExecutor interface {
	Run(ctx context.Context, script string) (*executor.Result, error)
	WorkDir() string
	Close() error
}

// Compile-time checks that the concrete types satisfy the interfaces.
var _ LLM = (*gemini.Client)(nil)
var _ CommandExecutor = (*executor.Executor)(nil)

// Agent orchestrates the CodeAct loop and manages multi-turn conversation state.
type Agent struct {
	llm           LLM
	exec          CommandExecutor
	maxIterations int
	verbose       bool
	history       []*genai.Content // persists across REPL turns
	repoDir       string           // always "repo" after Setup
}

// AgentOption configures an Agent.
type AgentOption func(*Agent)

// WithMaxIterations sets the maximum number of CodeAct iterations per turn.
func WithMaxIterations(n int) AgentOption {
	return func(a *Agent) { a.maxIterations = n }
}

// WithVerbose enables printing of intermediate commands and observations.
func WithVerbose(v bool) AgentOption {
	return func(a *Agent) { a.verbose = v }
}

// NewAgent creates an Agent wired to the given LLM and executor.
func NewAgent(llm LLM, exec CommandExecutor, opts ...AgentOption) *Agent {
	a := &Agent{
		llm:           llm,
		exec:          exec,
		maxIterations: 5,
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// genConfig returns the generation configuration for LLM calls.
func (a *Agent) genConfig() *genai.GenerateContentConfig {
	return &genai.GenerateContentConfig{
		Temperature:     genai.Ptr[float32](0.2),
		MaxOutputTokens: 4096,
		SystemInstruction: &genai.Content{
			Parts: []*genai.Part{{Text: InteractiveSystemPrompt}},
		},
	}
}

// Setup prepares the working environment for a repository. If source starts
// with http:// or https://, the repo is cloned via git. Otherwise source is
// treated as a local path and symlinked into the executor's temp directory.
// Setup resets the conversation history.
func (a *Agent) Setup(ctx context.Context, source string) error {
	if isURL(source) {
		cloneScript := fmt.Sprintf("git clone --depth=1 %q repo", source)
		result, err := a.exec.Run(ctx, cloneScript)
		if err != nil {
			return fmt.Errorf("agent: clone repo: %w", err)
		}
		if result.ExitCode != 0 {
			return fmt.Errorf("agent: clone failed (exit %d): %s", result.ExitCode, result.Stderr)
		}
	} else {
		absPath, err := filepath.Abs(source)
		if err != nil {
			return fmt.Errorf("agent: resolve path %q: %w", source, err)
		}
		checkResult, err := a.exec.Run(ctx, fmt.Sprintf("test -d %q", absPath))
		if err != nil || checkResult.ExitCode != 0 {
			return fmt.Errorf("agent: local path %q is not a valid directory", absPath)
		}
		linkResult, err := a.exec.Run(ctx, fmt.Sprintf("ln -s %q repo", absPath))
		if err != nil {
			return fmt.Errorf("agent: symlink local path: %w", err)
		}
		if linkResult.ExitCode != 0 {
			return fmt.Errorf("agent: symlink failed (exit %d): %s", linkResult.ExitCode, linkResult.Stderr)
		}
	}

	a.repoDir = "repo"
	a.history = nil
	return nil
}

// Turn sends a single user instruction through the CodeAct loop and returns
// the model's final text response, the number of CodeAct iterations used, and
// any error. Conversation history accumulates across multiple Turn calls.
func (a *Agent) Turn(ctx context.Context, instruction string) (string, int, error) {
	a.history = append(a.history, &genai.Content{
		Role:  genai.RoleUser,
		Parts: []*genai.Part{{Text: instruction}},
	})

	config := a.genConfig()

	for i := range a.maxIterations {
		text, err := a.llm.GenerateContent(ctx, a.history, config)
		if err != nil {
			return "", i, fmt.Errorf("agent: llm call iteration %d: %w", i+1, err)
		}

		code := ExtractCodeBlock(text)
		if code == "" {
			// No code block — the model has answered.
			a.history = append(a.history, &genai.Content{
				Role:  genai.RoleModel,
				Parts: []*genai.Part{{Text: text}},
			})
			return text, i + 1, nil
		}

		if a.verbose {
			fmt.Printf("\n[iteration %d] executing:\n%s\n", i+1, code)
		}

		a.history = append(a.history, &genai.Content{
			Role:  genai.RoleModel,
			Parts: []*genai.Part{{Text: text}},
		})

		fullScript := fmt.Sprintf("cd %s/%s && %s", a.exec.WorkDir(), a.repoDir, code)
		result, runErr := a.exec.Run(ctx, fullScript)
		obs := buildObservation(result, runErr)

		if a.verbose {
			fmt.Printf("[iteration %d] observation:\n%s\n", i+1, obs)
		}

		a.history = append(a.history, &genai.Content{
			Role:  genai.RoleUser,
			Parts: []*genai.Part{{Text: obs}},
		})
	}

	// Max iterations reached — force the model to answer.
	a.history = append(a.history, &genai.Content{
		Role: genai.RoleUser,
		Parts: []*genai.Part{{
			Text: "You have reached the maximum number of iterations for this turn. " +
				"Please provide your answer now. Write plain text only — do NOT include any code blocks.",
		}},
	})

	text, err := a.llm.GenerateContent(ctx, a.history, config)
	if err != nil {
		return "", a.maxIterations, fmt.Errorf("agent: forced response call: %w", err)
	}

	a.history = append(a.history, &genai.Content{
		Role:  genai.RoleModel,
		Parts: []*genai.Part{{Text: text}},
	})

	return text, a.maxIterations, nil
}

// History returns the current conversation history.
func (a *Agent) History() []*genai.Content {
	return a.history
}

// Analyze is a convenience wrapper for single-shot analysis. It calls Setup
// then Turn with an initial exploration instruction.
func (a *Agent) Analyze(ctx context.Context, repoURL string) (string, int, error) {
	if err := a.Setup(ctx, repoURL); err != nil {
		return "", 0, err
	}
	instruction := fmt.Sprintf(
		"Please analyze the repository at %s. "+
			"It has been cloned to the ./repo subdirectory. "+
			"Start by exploring its top-level structure.",
		repoURL,
	)
	return a.Turn(ctx, instruction)
}

// buildObservation formats command output into a user-readable observation message.
func buildObservation(result *executor.Result, runErr error) string {
	var b strings.Builder
	b.WriteString("**Observation:**\n")

	if runErr != nil {
		fmt.Fprintf(&b, "Execution error: %v\n", runErr)
	}

	if result == nil {
		return b.String()
	}

	if result.TimedOut {
		b.WriteString("**The command timed out after 30 seconds.**\n")
	}

	if result.Stdout != "" {
		b.WriteString("```\n")
		b.WriteString(result.Stdout)
		b.WriteString("\n```\n")
	}

	if result.Stderr != "" {
		b.WriteString("**stderr:**\n```\n")
		b.WriteString(result.Stderr)
		b.WriteString("\n```\n")
	}

	if result.ExitCode != 0 {
		fmt.Fprintf(&b, "**Exit code: %d**\n", result.ExitCode)
	}

	return b.String()
}

// ExtractCodeBlock returns the content of the first ```bash or ```sh block
// found in text, or an empty string if none is present.
func ExtractCodeBlock(text string) string {
	matches := codeBlockRe.FindStringSubmatch(text)
	if len(matches) < 2 {
		return ""
	}
	return strings.TrimSpace(matches[1])
}

// isURL reports whether source looks like a remote Git URL.
func isURL(source string) bool {
	return strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://")
}
