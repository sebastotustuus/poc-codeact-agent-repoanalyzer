# Running the CodeAct Repo Analyzer

## Requirements

- Go 1.22+
- `git` available in `PATH`
- A Gemini API key — get one at [Google AI Studio](https://aistudio.google.com/app/apikey)

---

## Install

### Option A — `go install` (recommended, runs from anywhere)

```bash
go install ./cmd/repoanalyzer
```

This compiles and places the binary in `$GOPATH/bin` (usually `~/go/bin`).
Make sure that directory is in your `PATH`:

```bash
# Check
echo $PATH | grep go/bin

# Add it if missing (zsh)
echo 'export PATH="$HOME/go/bin:$PATH"' >> ~/.zshrc && source ~/.zshrc
```

After that, `repoanalyzer` works from any directory.

### Option B — Build binary manually

```bash
go build -o repoanalyzer ./cmd/repoanalyzer
./repoanalyzer --repo https://github.com/user/project
```

### Option C — Run directly without installing

```bash
go run ./cmd/repoanalyzer --repo https://github.com/user/project
```

---

## Configuration

The only required configuration is the Gemini API key, passed via environment variable:

```bash
export GEMINI_API_KEY=your-api-key
```

---

## Usage

```bash
repoanalyzer --repo <url-or-path> [flags]
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--repo` | required | Git repository URL **or** local path |
| `--max-iterations` | `5` | Maximum CodeAct iterations per REPL turn |
| `--verbose` | `false` | Print each bash script executed and its output |

### Analyze a remote repository

```bash
repoanalyzer --repo https://github.com/user/project
```

### Analyze a local repository

```bash
repoanalyzer --repo /path/to/local/project
repoanalyzer --repo ./relative/path
```

### Verbose mode (see every command Gemini runs)

```bash
repoanalyzer --repo https://github.com/user/project --verbose
```

### Increase iteration budget per turn

```bash
repoanalyzer --repo https://github.com/user/project --max-iterations 10
```

---

## Interactive REPL

Once the repository is set up, the agent enters an interactive session:

```
Setting up: https://github.com/user/project
Ready. Ask anything about the repo. Press Enter for a full health report. Type "exit" to quit.
>
```

| Input | Behavior |
|-------|----------|
| Enter (first prompt) | Runs a full health report automatically |
| Any text | Sends that instruction to the agent |
| `exit` or `quit` | Exits and cleans up the temp directory |
| Ctrl+C | Graceful exit |
| Ctrl+D (EOF) | Graceful exit |

The agent retains full conversation history across turns — follow-up questions have context of everything already analyzed.

---

## Running Tests

```bash
# All tests
go test ./...

# With race detector (recommended)
go test -race ./...

# Static analysis
go vet ./...

# Integration test (requires real API key)
GEMINI_API_KEY=your-key go test ./internal/gemini/... -run Integration
```
