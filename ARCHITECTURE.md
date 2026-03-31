# Architecture

## Overview

The agent is organized around three concerns: **talking to the LLM** (`internal/gemini`), **executing commands safely** (`internal/executor`), and **orchestrating the loop** (`internal/agent`). The CLI (`cmd/repoanalyzer`) wires them together and runs the REPL.

```
cmd/repoanalyzer/
└── main.go              ← REPL loop, CLI flags, signal handling

internal/
├── agent/
│   ├── agent.go         ← CodeAct loop, conversation state, Setup/Turn
│   └── prompt.go        ← System prompt + default first-turn instruction
├── executor/
│   ├── executor.go      ← exec.Command, stdout/stderr capture, temp dir
│   └── sandbox.go       ← Blocked commands, command tokenizer
├── gemini/
│   └── client.go        ← Google GenAI SDK wrapper
└── report/
    └── report.go        ← Terminal output formatting
```

---

## The CodeAct Loop

The core of the agent is the `Turn()` method in `internal/agent/agent.go`. Each call represents one user instruction processed through the CodeAct loop:

```
User instruction appended to history
    ↓
Call Gemini with full conversation history
    ↓
Response contains ```bash block?
    ├── YES → execute script → append observation → loop
    └── NO  → append model response → return text to caller
```

The loop runs up to `maxIterations` times. If the limit is hit, one final forced call asks the model for a plain-text answer.

### Conversation history

History is a `[]*genai.Content` slice on the `Agent` struct. It accumulates across all turns in a session, giving the model full context of what it has already explored. `Setup()` resets it.

### Code extraction

Gemini's response is scanned for the first fenced code block matching `` ```bash `` or `` ```sh `` using a compiled regex. If none is found, the response is treated as the final answer.

---

## Components

### `internal/agent`

**`Agent` struct** — owns the conversation state and orchestrates the loop.

| Method | Description |
|--------|-------------|
| `Setup(ctx, source)` | Clones a URL or symlinks a local path into the temp dir. Resets history. |
| `Turn(ctx, instruction)` | One REPL turn: appends instruction, runs CodeAct loop, returns answer. |
| `Analyze(ctx, repoURL)` | Convenience wrapper: `Setup` + `Turn` with a default exploration instruction. |
| `History()` | Returns the current conversation history (used in tests). |

**Interfaces defined at point of use** — `LLM` and `CommandExecutor` are declared inside `internal/agent`, not in the packages that implement them. This is idiomatic Go and means tests can mock either without importing the real implementations.

```go
type LLM interface {
    GenerateContent(ctx, []*genai.Content, *genai.GenerateContentConfig) (string, error)
}

type CommandExecutor interface {
    Run(ctx, script string) (*executor.Result, error)
    WorkDir() string
    Close() error
}
```

Compile-time checks ensure the concrete types satisfy both interfaces:

```go
var _ LLM = (*gemini.Client)(nil)
var _ CommandExecutor = (*executor.Executor)(nil)
```

---

### `internal/executor`

**`Executor`** owns a temp directory created with `os.MkdirTemp`. Every command runs inside it via `exec.CommandContext("bash", "-c", script)`. `Close()` removes the directory.

**Sandbox constraints:**

| Constraint | Value |
|------------|-------|
| Blocked commands | `rm`, `rmdir`, `mkfs`, `dd`, `chmod`, `chown`, `sudo`, `su` (+ dot variants like `mkfs.ext4`) |
| Timeout per command | 30 seconds |
| Output truncation | 10 KB per stream (stdout / stderr) |

Command validation (`ValidateCommand`) tokenizes the script on shell operators (`&&`, `\|\|`, `;`, `\|`, newlines) and checks the base name of each segment's first token against the blocklist. It is a best-effort heuristic — not a full shell parser or container boundary.

---

### `internal/gemini`

A thin wrapper around `google.golang.org/genai`. Exposes a single method:

```go
func (c *Client) GenerateContent(
    ctx context.Context,
    contents []*genai.Content,
    config *genai.GenerateContentConfig,
) (string, error)
```

The agent passes the full conversation history on every call. The SDK handles authentication and HTTP.

---

### `internal/report`

Two functions:

| Function | Used by |
|----------|---------|
| `Print(w, repoURL, content, iterations)` | Single-shot `Analyze()` mode — framed output with separator lines |
| `PrintResponse(w, content, iterations)` | REPL turns — minimal framing, just the trimmed response text |

Both accept `io.Writer` so they work in tests without `os.Stdout`.

---

### `cmd/repoanalyzer`

`main.go` wires the components and runs the REPL:

1. Parse flags → validate `GEMINI_API_KEY`
2. `signal.NotifyContext` for Ctrl+C
3. Create `gemini.Client` → `executor.Executor` → `agent.Agent`
4. `ag.Setup(ctx, *repo)`
5. `repl(ctx, ag, os.Stdin, os.Stdout)`

The `repl` function accepts a `turnRunner` interface (not `*agent.Agent` directly), which makes it independently testable via a simple mock.

```go
type turnRunner interface {
    Turn(ctx context.Context, instruction string) (string, int, error)
}
```

---

## Data Flow (one REPL turn)

```
stdin → repl() → agent.Turn()
                    │
                    ├─ history += user message
                    │
                    └─ loop:
                         gemini.GenerateContent(history)
                              │
                         extract code block
                              │
                         executor.Run(cd repo && <code>)
                              │
                         buildObservation(stdout, stderr)
                              │
                         history += model + observation
                              │
                         (no code block) → history += model answer
                                              │
                                         report.PrintResponse → stdout
```

---

## Testing Strategy

- **Unit tests** for every package. No network, no filesystem beyond the executor's temp dir.
- **Hand-written mocks** for `LLM` and `CommandExecutor` — no mock frameworks.
- **Table-driven tests** with `t.Parallel()` throughout.
- **`repl()` tested** via `strings.Reader` / `bytes.Buffer` — the `turnRunner` interface makes this straightforward.
- **Integration test** in `internal/gemini` — skipped when `GEMINI_API_KEY` is not set.
- **Race detector** — `go test -race ./...` passes clean.
