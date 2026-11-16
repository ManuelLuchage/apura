// Package cli wires the cobra commands. Commands stay thin: parse flags,
// call store/distill/shell, print. Business rules live in other packages.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/ManuelLuchage/apura/internal/app"
	"github.com/ManuelLuchage/apura/internal/core"
	"github.com/ManuelLuchage/apura/internal/shell"
	_ "github.com/ManuelLuchage/apura/internal/shell/all" // register shells
	"github.com/ManuelLuchage/apura/internal/store"
)

// App carries shared state for all commands.
type App struct {
	dbPath string
	st     *store.Store
}

// Store opens the database on first use.
func (a *App) Store() (*store.Store, error) {
	if a.st != nil {
		return a.st, nil
	}
	path := a.dbPath
	if path == "" {
		var err error
		if path, err = app.DBPath(); err != nil {
			return nil, err
		}
	}
	st, err := store.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open database %s: %w", path, err)
	}
	a.st = st
	return st, nil
}

func (a *App) Close() {
	if a.st != nil {
		a.st.Close()
	}
}

// NewRoot builds the command tree.
func NewRoot(version string) (*cobra.Command, *App) {
	a := &App{}
	root := &cobra.Command{
		Use:   "apura",
		Short: "Refine messy terminal sessions into the steps that worked",
		Long: `apura records the commands you run inside a named session, lets you annotate
and mark each one (ok, fail, skip, key), and distills the session into the
shortest sequence of steps that worked.

Set up your shell once:
    fish:  echo 'status is-interactive; and apura init fish | source' >> ~/.config/fish/config.fish`,
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().StringVar(&a.dbPath, "db", "", "database path (default: $APURA_DB or $XDG_DATA_HOME/apura/apura.db)")

	root.AddGroup(
		&cobra.Group{ID: "session", Title: "Session:"},
		&cobra.Group{ID: "steps", Title: "Steps:"},
		&cobra.Group{ID: "find", Title: "Find & export:"},
	)
	add := func(group string, cmds ...*cobra.Command) {
		for _, c := range cmds {
			c.GroupID = group
			root.AddCommand(c)
		}
	}
	add("session", newStartCmd(a), newResumeCmd(a), newEndCmd(a), newStatusCmd(a), newTagCmd(a), newDeleteCmd(a))
	add("steps", newNoteCmd(a), newMarkCmd(a), newRmCmd(a))
	add("find", newLsCmd(a), newShowCmd(a), newSearchCmd(a), newDistillCmd(a))
	root.AddCommand(newInitCmd(a, root), newRecordCmd(a))
	return root, a
}

// Execute runs the CLI and returns the process exit code.
func Execute(version string) int {
	root, a := NewRoot(version)
	defer a.Close()
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "apura:", err)
		return 1
	}
	return 0
}

var errNoActive = errors.New("no active session in this shell (start one with `apura start <name>` or pass a session name)")

// session resolves an explicit reference, or the active session of this shell.
func (a *App) session(ctx context.Context, ref string) (core.Session, error) {
	st, err := a.Store()
	if err != nil {
		return core.Session{}, err
	}
	if ref != "" {
		return st.Resolve(ctx, ref)
	}
	env := os.Getenv(core.EnvSession)
	if env == "" {
		return core.Session{}, errNoActive
	}
	id, err := strconv.ParseInt(env, 10, 64)
	if err != nil {
		return core.Session{}, fmt.Errorf("invalid %s=%q", core.EnvSession, env)
	}
	return st.Session(ctx, id)
}

// emitEnv prints shell code that sets/unsets variables in the parent shell.
// It only does so when invoked through the shell wrapper (APURA_EMIT_ENV set);
// otherwise it prints a hint, because a child process cannot change its parent.
func emitEnv(stdout, stderr io.Writer, set [][2]string, unset []string) error {
	name := os.Getenv(core.EnvEmit)
	if name == "" {
		fmt.Fprintln(stderr, "note: shell integration not loaded, so this shell's variables were not changed.")
		fmt.Fprintln(stderr, "      load it with e.g. `apura init fish | source` (see `apura init --help`).")
		return nil
	}
	sh, err := shell.Get(name)
	if err != nil {
		return err
	}
	for _, kv := range set {
		fmt.Fprintln(stdout, sh.SetEnv(kv[0], kv[1]))
	}
	for _, k := range unset {
		fmt.Fprintln(stdout, sh.UnsetEnv(k))
	}
	return nil
}
