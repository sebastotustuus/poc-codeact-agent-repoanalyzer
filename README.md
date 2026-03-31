# CodeAct Git Repo Analyzer

A CLI agent that analyzes Git repositories using the **CodeAct** paradigm. Instead of calling pre-defined tools, the LLM (Gemini) writes executable bash scripts as its actions, observes the output, and iterates until it has enough information to produce a final report.

## What is CodeAct?

CodeAct ([Wang et al., 2024](https://arxiv.org/abs/2402.01030)) is an agent framework where the LLM's actions are executable code rather than structured JSON tool calls. This approach has several advantages:

- **Pre-training alignment** — LLMs have seen massive amounts of code; generating bash is more natural than crafting bespoke JSON schemas.
- **Expressive power** — A single code block can compose multiple tools using loops, conditionals, and pipes.
- **Self-debugging** — When a command fails, the LLM reads its own stderr and corrects itself in the next iteration.
- **Fewer turns** — Research shows ~30% fewer interaction turns and ~20% higher task success versus JSON tool-call baselines.

### The Loop

```
User provides repo URL
    ↓
Agent clones the repo into a temp dir
    ↓
Agent sends initial request to Gemini (with system prompt)
    ↓
Gemini responds with a ```bash``` block
    ↓
Agent extracts and executes the script in the sandbox
    ↓
stdout/stderr are fed back to Gemini as an observation
    ↓
Repeat until Gemini produces a plain-text report (no code block)
    ↓
Report is printed to stdout
```

## Requirements

- Go 1.22+
- `git` available in `PATH`
- A [Gemini API key](https://aistudio.google.com/app/apikey)

## Build

```bash
go build -o repoanalyzer ./cmd/repoanalyzer
```

## Usage

The agent runs as an **interactive REPL**. After cloning (or symlinking) the repo, you can ask questions, follow up, and the model retains full conversation history across turns.

```bash
export GEMINI_API_KEY=your-api-key

# Remote repository
./repoanalyzer --repo https://github.com/user/project

# Local repository
./repoanalyzer --repo /path/to/local/project

# With options
./repoanalyzer --repo https://github.com/user/project --max-iterations 8 --verbose
```

### Example session

```
$ repoanalyzer --repo https://github.com/user/project
Setting up: https://github.com/user/project
Ready. Ask anything about the repo. Press Enter for a full health report. Type "exit" to quit.
>
🔍 (running default analysis...)

## Overview
This is a Go project...

> what are the main dependencies?
It uses the following external packages...

> are there any TODO comments?
Found 3 TODO comments...

> exit
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--repo` | (required) | Git repository URL **or local path** |
| `--max-iterations` | `5` | Maximum CodeAct iterations per turn |
| `--verbose` | `false` | Print intermediate commands and observations |

## Design Choices

### Official Google GenAI SDK

The Gemini client wraps `google.golang.org/genai`, the official Go SDK. The `internal/gemini` package is a thin adapter that exposes a single `GenerateContent` method and keeps the SDK as an implementation detail. The `LLM` interface in `internal/agent` is defined at point of use (not in the gemini package), so tests mock the interface without touching the SDK or the network.

### Sandbox

Each run gets an isolated `os.MkdirTemp` directory that is removed on exit. Command execution is restricted by:

- **Blocked commands**: `rm`, `rmdir`, `mkfs`, `dd`, `chmod`, `chown`, `sudo`, `su` (and dot-suffixed variants like `mkfs.ext4`).
- **30-second timeout** per command.
- **10 KB output truncation** per stream to prevent context overflow.

The sandbox is best-effort (not a container), suitable for a PoC. The system prompt also instructs the LLM never to modify the repository.

### Interfaces at point of use

`LLM` and `CommandExecutor` interfaces are defined inside `internal/agent`, not in the packages that implement them. This follows the Go convention of defining interfaces where they are consumed, enabling straightforward testing with hand-written mocks and zero test dependencies.

### Conversation accumulation

The full conversation (alternating `model`/`user` turns) is sent to Gemini on every iteration. This gives the model complete context of what it has already tried and observed, enabling self-correction across turns.

### Forced final report

If the iteration limit is reached before the model produces a plain-text response, the agent appends a final user message explicitly asking for a report with no code blocks. This guarantees the loop always produces output.

## Project Structure

```
cmd/repoanalyzer/main.go          ← CLI entry point
internal/agent/agent.go            ← CodeAct loop orchestration
internal/agent/prompt.go           ← Gemini system prompt
internal/executor/executor.go      ← Command execution + output capture
internal/executor/sandbox.go       ← Command validation + blocked list
internal/gemini/client.go          ← Gemini REST client
internal/gemini/types.go           ← API request/response types
internal/report/report.go          ← Terminal report formatting
```

## Running Tests

```bash
go test ./...
go test -race ./...
```
