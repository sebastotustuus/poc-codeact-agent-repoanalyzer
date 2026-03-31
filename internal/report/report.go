// Package report formats the agent's final analysis for terminal output.
package report

import (
	"fmt"
	"io"
	"strings"
)

const separator = "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

// Print writes a framed report to w.
func Print(w io.Writer, repoURL string, content string, iterations int) {
	fmt.Fprintln(w, separator)
	fmt.Fprintf(w, "  Repository Analysis: %s\n", repoURL)
	fmt.Fprintf(w, "  Completed in %s\n", iterationLabel(iterations))
	fmt.Fprintln(w, separator)
	fmt.Fprintln(w)
	fmt.Fprintln(w, strings.TrimSpace(content))
	fmt.Fprintln(w)
	fmt.Fprintln(w, separator)
}

// PrintResponse writes a single REPL turn response to w with minimal framing.
func PrintResponse(w io.Writer, content string, _ int) {
	fmt.Fprintln(w)
	fmt.Fprintln(w, strings.TrimSpace(content))
	fmt.Fprintln(w)
}

func iterationLabel(n int) string {
	if n == 1 {
		return "1 iteration"
	}
	return fmt.Sprintf("%d iterations", n)
}
