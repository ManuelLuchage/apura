package cli

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/ManuelLuchage/apura/internal/core"
	"github.com/ManuelLuchage/apura/internal/store"
)

func newStartCmd(a *App) *cobra.Command {
	var goal string
	var tags []string
	c := &cobra.Command{
		Use:   "start <name>",
		Short: "Start recording a named session in this shell",
		Example: `  apura start setup-k3s --goal "single-node k3s on Hetzner" --tags k3s,hetzner
  apura start fix-nginx-502 -t nginx`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			errw := cmd.ErrOrStderr()
			if cur := os.Getenv(core.EnvSession); cur != "" {
				if s, err := a.session(ctx, ""); err == nil && s.State == core.StateActive {
					return fmt.Errorf("session %q is already recording in this shell; run `apura end` first", s.Name)
				}
			}
			st, err := a.Store()
			if err != nil {
				return err
			}
			host, _ := os.Hostname()
			s, err := st.CreateSession(ctx, core.Session{
				Name: args[0], Goal: goal, Tags: tags, Shell: os.Getenv(core.EnvShell), Host: host,
			})
			if errors.Is(err, store.ErrNameTaken) {
				return fmt.Errorf("a session named %q already exists; use `apura resume %s` or pick another name", args[0], args[0])
			}
			if err != nil {
				return err
			}
			if err := emitEnv(cmd.OutOrStdout(), errw, [][2]string{
				{core.EnvSession, strconv.FormatInt(s.ID, 10)}, {core.EnvSessionName, s.Name},
			}, nil); err != nil {
				return err
			}
			fmt.Fprintf(errw, "⏺ recording %q (#%d). Annotate with `apura note`, judge with `apura mark`, finish with `apura end`.\n", s.Name, s.ID)
			return nil
		},
	}
	c.Flags().StringVarP(&goal, "goal", "g", "", "what this session should achieve")
	c.Flags().StringSliceVarP(&tags, "tags", "t", nil, "comma-separated tags (technologies, project...)")
	return c
}

func newResumeCmd(a *App) *cobra.Command {
	return &cobra.Command{
		Use:   "resume <name|id>",
		Short: "Continue recording an existing session in this shell (also attaches a 2nd terminal)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			s, err := a.session(ctx, args[0])
			if err != nil {
				return err
			}
			st, _ := a.Store()
			if s.State == core.StateEnded {
				if err := st.ReopenSession(ctx, s.ID); err != nil {
					return err
				}
			}
			if err := emitEnv(cmd.OutOrStdout(), cmd.ErrOrStderr(), [][2]string{
				{core.EnvSession, strconv.FormatInt(s.ID, 10)}, {core.EnvSessionName, s.Name},
			}, nil); err != nil {
				return err
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "⏺ recording %q (#%d) again, %d steps so far.\n", s.Name, s.ID, s.StepCount)
			return nil
		},
	}
}

func newEndCmd(a *App) *cobra.Command {
	var result string
	c := &cobra.Command{
		Use:   "end [name|id]",
		Short: "Stop recording; optionally record the session's result",
		Example: `  apura end -r success
  apura end -r failure
  apura end other-session -r abandoned`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			res, err := core.ParseResult(result)
			if err != nil {
				return err
			}
			ref := ""
			if len(args) == 1 {
				ref = args[0]
			}
			s, err := a.session(ctx, ref)
			if err != nil {
				return err
			}
			st, _ := a.Store()
			if err := st.EndSession(ctx, s.ID, res); err != nil {
				return err
			}
			// Only clear this shell's variables if we ended *its* session.
			if os.Getenv(core.EnvSession) == strconv.FormatInt(s.ID, 10) {
				if err := emitEnv(cmd.OutOrStdout(), cmd.ErrOrStderr(), nil,
					[]string{core.EnvSession, core.EnvSessionName}); err != nil {
					return err
				}
			}
			steps, err := st.Steps(ctx, s.ID)
			if err != nil {
				return err
			}
			var ok, fail int
			for _, x := range steps {
				switch x.Outcome() {
				case core.OutcomeSuccess:
					ok++
				case core.OutcomeFailure:
					fail++
				}
			}
			label := string(res)
			if label == "" {
				label = "no result (set one with `apura end -r success|failure|abandoned`)"
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "■ ended %q: %d steps (%d ok, %d failed) — %s\n", s.Name, len(steps), ok, fail, label)
			return nil
		},
	}
	c.Flags().StringVarP(&result, "result", "r", "", "session result: success, failure or abandoned")
	return c
}

func newStatusCmd(a *App) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the session active in this shell",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := a.session(cmd.Context(), "")
			if errors.Is(err, errNoActive) {
				fmt.Fprintln(cmd.OutOrStdout(), "no active session")
				return nil
			}
			if err != nil {
				return err
			}
			printSessionHeader(cmd.OutOrStdout(), s)
			return nil
		},
	}
}

func newTagCmd(a *App) *cobra.Command {
	var ref string
	c := &cobra.Command{
		Use:     "tag <tag>...",
		Short:   "Add tags to the active (or given) session",
		Example: "  apura tag postgres backup",
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := a.session(cmd.Context(), ref)
			if err != nil {
				return err
			}
			st, _ := a.Store()
			return st.AddTags(cmd.Context(), s.ID, args)
		},
	}
	c.Flags().StringVarP(&ref, "session", "s", "", "session name or id (default: active)")
	return c
}

func newDeleteCmd(a *App) *cobra.Command {
	var yes bool
	c := &cobra.Command{
		Use:   "delete <name|id>",
		Short: "Delete a session and all its steps and notes (irreversible)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := a.session(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if !yes {
				return fmt.Errorf("this deletes %q and its %d steps permanently; re-run with --yes", s.Name, s.StepCount)
			}
			st, _ := a.Store()
			if err := st.DeleteSession(cmd.Context(), s.ID); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "deleted %q\n", s.Name)
			return nil
		},
	}
	c.Flags().BoolVar(&yes, "yes", false, "confirm deletion")
	return c
}
