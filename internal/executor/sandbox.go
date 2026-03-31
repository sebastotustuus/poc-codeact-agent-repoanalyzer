package executor

import (
	"fmt"
	"path/filepath"
	"strings"
)

var blockedCommands = map[string]bool{
	"rm":    true,
	"rmdir": true,
	"mkfs":  true,
	"dd":    true,
	"chmod": true,
	"chown": true,
	"sudo":  true,
	"su":    true,
}

func ValidateCommand(script string) error {
	for _, segment := range tokenizeScript(script) {
		cmd := extractCommandName(segment)
		if cmd == "" {
			continue
		}
		if isBlocked(cmd) {
			return fmt.Errorf("executor: blocked command %q", cmd)
		}
	}
	return nil
}

func isBlocked(cmd string) bool {
	if blockedCommands[cmd] {
		return true
	}
	if dot := strings.IndexByte(cmd, '.'); dot > 0 {
		return blockedCommands[cmd[:dot]]
	}
	return false
}

func tokenizeScript(script string) []string {
	r := strings.NewReplacer(
		"&&", "\n",
		"||", "\n",
		";", "\n",
		"|", "\n",
	)
	normalized := r.Replace(script)

	var segments []string
	for _, line := range strings.Split(normalized, "\n") {
		s := strings.TrimSpace(line)
		if s != "" && !strings.HasPrefix(s, "#") {
			segments = append(segments, s)
		}
	}
	return segments
}

func extractCommandName(segment string) string {
	first := strings.Fields(segment)[0]
	return strings.ToLower(filepath.Base(first))
}
