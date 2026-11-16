package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/ManuelLuchage/apura/internal/cmdparse"
	"github.com/ManuelLuchage/apura/internal/core"
	"github.com/ManuelLuchage/apura/internal/redact"
	"github.com/ManuelLuchage/apura/internal/shell"
	"github.com/ManuelLuchage/apura/internal/tech"
)

func newInitCmd(a *App, root *cobra.Command) *cobra.Command {
	var noCompletions bool
	c := &cobra.Command{
		Use:   "init [shell]",
		Short: "Print the shell integration code (hooks, wrapper, completions)",
		Long: `Print the code that integrates apura with your shell. Load it from your
shell config so every new interactive shell can record sessions.

  fish   ~/.config/fish/config.fish:
         status is-interactive; and apura init fish | source

Supported shells: ` + strings.Join(shell.Names(), ", "),
		Args:      cobra.MaximumNArgs(1),
		ValidArgs: shell.Names(),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := ""
			if len(args) == 1 {
				name = args[0]
			} else if s := os.Getenv("SHELL"); s != "" {
				name = filepath.Base(s)
			}
			sh, err := shell.Get(name)
			if err != nil {
				return err
			}
			bin, err := os.Executable()
			if err != nil {
				return err
			}
			if resolved, err := filepath.EvalSymlinks(bin); err == nil {
				bin = resolved
			}
			out := cmd.OutOrStdout()
			io.WriteString(out, sh.InitScript(bin))
			if noCompletions {
				return nil
			}
			fmt.Fprintln(out)
			return writeCompletions(root, sh.Name(), out)
		},
	}
	c.Flags().BoolVar(&noCompletions, "no-completions", false, "do not include tab completions")
	return c
}

// writeCompletions uses cobra's generators; new shells usually get them free.
func writeCompletions(root *cobra.Command, shellName string, w io.Writer) error {
	switch shellName {
	case "fish":
		return root.GenFishCompletion(w, true)
	case "zsh":
		return root.GenZshCompletion(w)
	case "bash":
		return root.GenBashCompletionV2(w, true)
	}
	return nil
}

// newRecordCmd is the hidden endpoint every shell hook calls after a command.
// It must be fast, silent and never fail loudly: the hook discards its output.
func newRecordCmd(a *App) *cobra.Command {
	var (
		sessionID  int64
		exitCode   int
		durationMs int64
		cwd        string
		shellName  string
	)
	c := &cobra.Command{
		Use:    "_record --session <id> --exit <code> --duration <ms> --cwd <dir> --shell <name> -- <command>",
		Short:  "Record one executed command (called by shell hooks)",
		Hidden: true,
		Args:   cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			line := strings.Join(args, " ")
			if !shouldRecord(line) {
				return nil
			}
			ctx := cmd.Context()
			st, err := a.Store()
			if err != nil {
				return err
			}
			s, err := st.Session(ctx, sessionID)
			if err != nil || s.State != core.StateActive {
				return err // ended in another terminal: silently ignore
			}
			clean, redacted := redact.Line(line)
			_, err = st.AddStep(ctx, core.Step{
				SessionID: sessionID,
				Command:   strings.TrimRight(clean, " \n"),
				Cwd:       cwd,
				Shell:     shellName,
				ExitCode:  exitCode,
				Duration:  time.Duration(durationMs) * time.Millisecond,
				Redacted:  redacted,
				Techs:     tech.Infer(clean),
			})
			return err
		},
	}
	c.Flags().Int64Var(&sessionID, "session", 0, "session id")
	c.Flags().IntVar(&exitCode, "exit", 0, "exit status")
	c.Flags().Int64Var(&durationMs, "duration", 0, "duration in milliseconds")
	c.Flags().StringVar(&cwd, "cwd", "", "directory where the command started")
	c.Flags().StringVar(&shellName, "shell", "", "shell name")
	c.MarkFlagRequired("session")
	return c
}

// shouldRecord filters what never belongs in a session.
func shouldRecord(line string) bool {
	if strings.TrimSpace(line) == "" {
		return false
	}
	// Leading space = "private" command in fish/zsh/bash (HISTCONTROL=ignorespace).
	if strings.HasPrefix(line, " ") {
		return false
	}
	// apura's own commands (note, mark...) are metadata, not steps.
	return cmdparse.FirstProgram(line) != "apura"
}
