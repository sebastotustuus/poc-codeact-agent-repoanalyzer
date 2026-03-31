package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"

	"github.com/user/poc-codeact-repoanalyzer/internal/agent"
	"github.com/user/poc-codeact-repoanalyzer/internal/executor"
	"github.com/user/poc-codeact-repoanalyzer/internal/gemini"
	"github.com/user/poc-codeact-repoanalyzer/internal/report"
)

func main() {
	os.Exit(run())
}

// turnRunner is the interface consumed by the REPL loop.
// Defined at point of use so tests can inject a mock without importing agent.
type turnRunner interface {
	Turn(ctx context.Context, instruction string) (string, int, error)
}

func run() int {
	repo := flag.String("repo", "", "Git repository URL or local path to analyze (required)")
	maxIter := flag.Int("max-iterations", 5, "Maximum CodeAct loop iterations per turn")
	verbose := flag.Bool("verbose", false, "Print intermediate commands and observations")
	flag.Parse()

	if *repo == "" {
		fmt.Fprintln(os.Stderr, "error: --repo is required")
		flag.Usage()
		return 1
	}

	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "error: GEMINI_API_KEY environment variable is not set")
		return 1
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	client, err := gemini.NewClient(ctx, apiKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	exec, err := executor.NewExecutor()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	defer exec.Close()

	ag := agent.NewAgent(client, exec,
		agent.WithMaxIterations(*maxIter),
		agent.WithVerbose(*verbose),
	)

	fmt.Fprintf(os.Stderr, "Setting up: %s\n", *repo)
	if err := ag.Setup(ctx, *repo); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	fmt.Fprintln(os.Stderr, `Ready. Ask anything about the repo. Press Enter for a full health report. Type "exit" to quit.`)
	return repl(ctx, ag, os.Stdin, os.Stdout)
}

func repl(ctx context.Context, runner turnRunner, in io.Reader, out io.Writer) int {
	scanner := bufio.NewScanner(in)
	firstTurn := true

	for {
		fmt.Fprint(out, "> ")

		if !scanner.Scan() {
			// EOF (Ctrl+D) — exit cleanly.
			fmt.Fprintln(out)
			return 0
		}

		line := strings.TrimSpace(scanner.Text())

		switch line {
		case "exit", "quit":
			return 0
		case "":
			if firstTurn {
				line = agent.DefaultFirstTurnPrompt
			} else {
				continue
			}
		}

		firstTurn = false

		result, iterations, err := runner.Turn(ctx, line)
		if err != nil {
			if ctx.Err() != nil {
				// Context canceled by Ctrl+C — exit cleanly.
				return 0
			}
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			continue
		}

		report.PrintResponse(out, result, iterations)
	}
}
