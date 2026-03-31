# CodeAct Git Repo Analyzer

A CLI agent that analyzes Git repositories using the **CodeAct** paradigm — the LLM writes executable bash scripts as actions, observes the output, and iterates until it can answer your question. Supports interactive multi-turn sessions with persistent conversation history.

---

## Documentation

| Document | Description |
|----------|-------------|
| [RUNNING.md](RUNNING.md) | How to install, configure, and run the agent |
| [ARCHITECTURE.md](ARCHITECTURE.md) | Technical design, components, and internals |
| [DECISIONS.md](DECISIONS.md) | Vision, tradeoffs, and development experience |

---

## Quick Start

### Set environment variables
```bash
export GEMINI_API_KEY=your-api-key
```

### Install the agent

```bash
go install ./cmd/repoanalyzer
```

### Run the agent
```bash
repoanalyzer --repo https://github.com/user/project
```

```
> (Enter)            → full health report
> what language?     → targeted question
> any SQL risks?     → follow-up with context
> exit
```

**Requirements:** Go 1.22+ · `git` in PATH · Gemini API key from [Google AI Studio](https://aistudio.google.com/app/apikey)
