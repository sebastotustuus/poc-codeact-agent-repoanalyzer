# CLAUDE.md — CodeAct Git Repo Analyzer

## Go conventions

Follow `.claude/skills/senior-go-idiomatic/SKILL.md` for all Go conventions and code quality standards.

## Objective
 
Build a CLI agent that analyzes any Git repository autonomously. The user points it at a repo URL (or local path) and the agent produces a comprehensive health report: structure, languages, dependencies, code smells, exposed secrets, test coverage, documentation quality, and git history insights.
 
The agent must figure out **what to analyze and how** on its own — each repo is different (Go, Python, monorepo, etc.), so the analysis scripts cannot be predefined.

## Project

CLI agent in Go using the **CodeAct paradigm**: the LLM generates executable bash scripts (not JSON tool calls). The agent extracts the code, executes it in a sandbox, captures stdout/stderr, and feeds it back as observations. Loops until the LLM emits a final text report (no code block).

**Proof of concept** — clean code over feature completeness.

## Architecture

```
cmd/repoanalyzer/main.go          ← CLI (stdlib flag)
internal/agent/agent.go            ← CodeAct loop
internal/agent/prompt.go           ← System prompt for Gemini
internal/executor/executor.go      ← exec.Command + output capture
internal/executor/sandbox.go       ← Timeout, blocked commands, temp dir
internal/gemini/client.go          ← Gemini client wrapper
internal/report/report.go          ← Terminal report formatting
```

## References CodeAct

- https://arxiv.org/html/2402.01030v4
- https://www.emergentmind.com/topics/codeact-agent-framework
- https://www.linkedin.com/pulse/when-research-turns-real-building-working-agent-from-codeact-sharma-nf1oc

## CodeAct loop

```
Repo URL → prompt Gemini → receives ```bash``` block →
extract & execute → stdout/stderr back to Gemini →
more code? loop (max 10). text only? final report.
```

## Gemini

- SDK: `google.golang.org/genai` (official Go SDK)
- Model: `gemini-3.1-flash-lite-preview`
- API key from env var `GEMINI_API_KEY` (SDK reads it automatically)

## Sandbox

Blocked: `rm, rmdir, mkfs, dd, chmod, chown, sudo, su`. Timeout: 30s. Working dir: temp per session.

## CLI

```bash
repoanalyzer --repo https://github.com/user/project [--max-iterations 5] [--verbose]
```

## Rules

- Prefer stdlib, minimize deps (Gemini SDK is the only required external dep)
- No API keys in code
- No company names in repo/code
- README: what CodeAct is, how to build/run, design choices