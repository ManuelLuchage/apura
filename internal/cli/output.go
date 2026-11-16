package cli

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/ManuelLuchage/apura/internal/core"
)

// markSymbol shows the explicit mark, or the exit-code verdict if unmarked.
//
//	★ key   ✔ ok   ✘ fail   ⊘ skip   · exit 0   ! exit≠0
func markSymbol(s core.Step) string {
	switch s.Mark {
	case core.MarkKey:
		return "★"
	case core.MarkOK:
		return "✔"
	case core.MarkFail:
		return "✘"
	case core.MarkSkip:
		return "⊘"
	}
	if s.ExitCode == 0 {
		return "·"
	}
	return "!"
}

func firstLine(s string, max int) string {
	line, _, multi := strings.Cut(s, "\n")
	if multi {
		line += " …"
	}
	if r := []rune(line); len(r) > max {
		line = string(r[:max-1]) + "…"
	}
	return line
}

func humanDuration(d time.Duration) string {
	switch {
	case d < time.Second:
		return fmt.Sprintf("%dms", d.Milliseconds())
	case d < time.Minute:
		return fmt.Sprintf("%.1fs", d.Seconds())
	default:
		return d.Round(time.Second).String()
	}
}

func sessionLabel(s core.Session) string {
	if s.State == core.StateActive {
		return "recording"
	}
	if s.Result != "" {
		return string(s.Result)
	}
	return "ended"
}

func printSessionHeader(w io.Writer, s core.Session) {
	fmt.Fprintf(w, "#%d %s  [%s]  %s  started %s\n", s.ID, s.Name, sessionLabel(s),
		s.Shell, s.StartedAt.Format("2006-01-02 15:04"))
	if s.Goal != "" {
		fmt.Fprintf(w, "   goal:  %s\n", s.Goal)
	}
	if len(s.Tags) > 0 {
		fmt.Fprintf(w, "   tags:  %s\n", strings.Join(s.Tags, ", "))
	}
	if len(s.Techs) > 0 {
		fmt.Fprintf(w, "   techs: %s\n", strings.Join(s.Techs, ", "))
	}
	fmt.Fprintf(w, "   steps: %d\n", s.StepCount)
}
