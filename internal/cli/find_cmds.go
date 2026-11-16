package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/ManuelLuchage/apura/internal/core"
	"github.com/ManuelLuchage/apura/internal/distill"
	"github.com/ManuelLuchage/apura/internal/shell"
	"github.com/ManuelLuchage/apura/internal/store"
)

func newLsCmd(a *App) *cobra.Command {
	var (
		f            store.ListFilter
		active, done bool
		result       string
		asJSON       bool
	)
	c := &cobra.Command{
		Use:     "ls",
		Aliases: []string{"list"},
		Short:   "List sessions, filtering by tag, technology, state or result",
		Example: `  apura ls --tech docker
  apura ls --tag hetzner --result success
  apura ls --active`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			if f.Result, err = core.ParseResult(result); err != nil {
				return err
			}
			switch {
			case active && done:
				return fmt.Errorf("--active and --ended are mutually exclusive")
			case active:
				f.State = core.StateActive
			case done:
				f.State = core.StateEnded
			}
			return listSessions(cmd, a, f, asJSON, "")
		},
	}
	c.Flags().StringVar(&f.Tag, "tag", "", "only sessions with this tag")
	c.Flags().StringVar(&f.Tech, "tech", "", "only sessions using this technology (inferred or tagged)")
	c.Flags().BoolVar(&active, "active", false, "only sessions still recording")
	c.Flags().BoolVar(&done, "ended", false, "only ended sessions")
	c.Flags().StringVarP(&result, "result", "r", "", "only sessions with this result")
	c.Flags().IntVarP(&f.Limit, "limit", "l", 30, "maximum number of sessions")
	c.Flags().BoolVar(&asJSON, "json", false, "JSON output")
	return c
}

func newSearchCmd(a *App) *cobra.Command {
	var (
		f      store.ListFilter
		asJSON bool
	)
	c := &cobra.Command{
		Use:   "search <text>",
		Short: "Find sessions by name, goal, tag, command or note",
		Example: `  apura search "ufw allow"
  apura search postgres --tech docker`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f.Query = strings.Join(args, " ")
			return listSessions(cmd, a, f, asJSON, f.Query)
		},
	}
	c.Flags().StringVar(&f.Tag, "tag", "", "only sessions with this tag")
	c.Flags().StringVar(&f.Tech, "tech", "", "only sessions using this technology")
	c.Flags().IntVarP(&f.Limit, "limit", "l", 30, "maximum number of sessions")
	c.Flags().BoolVar(&asJSON, "json", false, "JSON output")
	return c
}

type sessionJSON struct {
	ID        int64      `json:"id"`
	Name      string     `json:"name"`
	Goal      string     `json:"goal,omitempty"`
	State     string     `json:"state"`
	Result    string     `json:"result,omitempty"`
	Shell     string     `json:"shell,omitempty"`
	Host      string     `json:"host,omitempty"`
	StartedAt time.Time  `json:"started_at"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
	Tags      []string   `json:"tags,omitempty"`
	Techs     []string   `json:"techs,omitempty"`
	StepCount int        `json:"step_count"`
	Steps     []stepJSON `json:"steps,omitempty"`
	Notes     []string   `json:"notes,omitempty"`
}

type stepJSON struct {
	Seq        int      `json:"seq"`
	Command    string   `json:"command"`
	Cwd        string   `json:"cwd,omitempty"`
	ExitCode   int      `json:"exit_code"`
	DurationMs int64    `json:"duration_ms"`
	Mark       string   `json:"mark,omitempty"`
	Outcome    string   `json:"outcome"`
	Techs      []string `json:"techs,omitempty"`
	Notes      []string `json:"notes,omitempty"`
}

func toSessionJSON(s core.Session) sessionJSON {
	return sessionJSON{ID: s.ID, Name: s.Name, Goal: s.Goal, State: string(s.State), Result: string(s.Result),
		Shell: s.Shell, Host: s.Host, StartedAt: s.StartedAt, EndedAt: s.EndedAt, Tags: s.Tags,
		Techs: s.Techs, StepCount: s.StepCount}
}

func toStepJSON(x core.Step) stepJSON {
	j := stepJSON{Seq: x.Seq, Command: x.Command, Cwd: x.Cwd, ExitCode: x.ExitCode,
		DurationMs: x.Duration.Milliseconds(), Mark: string(x.Mark), Outcome: string(x.Outcome()), Techs: x.Techs}
	for _, n := range x.Notes {
		j.Notes = append(j.Notes, n.Body)
	}
	return j
}

func listSessions(cmd *cobra.Command, a *App, f store.ListFilter, asJSON bool, query string) error {
	ctx := cmd.Context()
	st, err := a.Store()
	if err != nil {
		return err
	}
	sessions, err := st.ListSessions(ctx, f)
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	if asJSON {
		list := make([]sessionJSON, 0, len(sessions))
		for _, s := range sessions {
			list = append(list, toSessionJSON(s))
		}
		return writeJSON(out, list)
	}
	if len(sessions) == 0 {
		fmt.Fprintln(out, "no sessions found")
		return nil
	}
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tNAME\tSTATUS\tSTEPS\tSTARTED\tTAGS / TECHS")
	for _, s := range sessions {
		fmt.Fprintf(tw, "%d\t%s\t%s\t%d\t%s\t%s\n", s.ID, s.Name, sessionLabel(s), s.StepCount,
			s.StartedAt.Format("2006-01-02 15:04"), strings.Join(mergeUnique(s.Tags, s.Techs), ","))
		if query != "" {
			steps, err := st.Steps(ctx, s.ID)
			if err != nil {
				return err
			}
			shown := 0
			q := strings.ToLower(query)
			for _, x := range steps {
				if shown == 3 {
					break
				}
				hit := strings.Contains(strings.ToLower(x.Command), q)
				for _, n := range x.Notes {
					hit = hit || strings.Contains(strings.ToLower(n.Body), q)
				}
				if hit {
					fmt.Fprintf(tw, "\t  %s #%d %s\t\t\t\t\n", markSymbol(x), x.Seq, firstLine(x.Command, 70))
					shown++
				}
			}
		}
	}
	return tw.Flush()
}

func mergeUnique(a, b []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, x := range append(append([]string{}, a...), b...) {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func newShowCmd(a *App) *cobra.Command {
	var (
		only   []string
		asJSON bool
	)
	c := &cobra.Command{
		Use:   "show [name|id]",
		Short: "Show a session's steps in execution order (default: active session)",
		Long: `Show the steps of a session in the order they ran.

Symbols: ★ key  ✔ ok  ✘ fail  ⊘ skip  · unmarked, exit 0  ! unmarked, exit≠0`,
		Example: `  apura show
  apura show setup-k3s --only success
  apura show setup-k3s --only fail,skip
  apura show 12 --json`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			ref := ""
			if len(args) == 1 {
				ref = args[0]
			}
			s, err := a.session(ctx, ref)
			if err != nil {
				return err
			}
			st, _ := a.Store()
			steps, err := st.Steps(ctx, s.ID)
			if err != nil {
				return err
			}
			notes, err := st.Notes(ctx, s.ID)
			if err != nil {
				return err
			}
			keep, err := stepFilter(only)
			if err != nil {
				return err
			}
			var filtered []core.Step
			for _, x := range steps {
				if keep(x) {
					filtered = append(filtered, x)
				}
			}
			out := cmd.OutOrStdout()
			if asJSON {
				j := toSessionJSON(s)
				for _, x := range filtered {
					j.Steps = append(j.Steps, toStepJSON(x))
				}
				for _, n := range notes {
					if n.StepID == nil {
						j.Notes = append(j.Notes, n.Body)
					}
				}
				return writeJSON(out, j)
			}
			printSessionHeader(out, s)
			for _, n := range notes {
				if n.StepID == nil {
					fmt.Fprintf(out, "   note:  %s\n", n.Body)
				}
			}
			fmt.Fprintln(out)
			tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "  #\t\tEXIT\tTIME\tCOMMAND")
			for _, x := range filtered {
				fmt.Fprintf(tw, "  %d\t%s\t%d\t%s\t%s\n", x.Seq, markSymbol(x), x.ExitCode,
					humanDuration(x.Duration), firstLine(x.Command, 100))
				for _, n := range x.Notes {
					fmt.Fprintf(tw, "\t\t\t\t  ↳ %s\n", n.Body)
				}
			}
			return tw.Flush()
		},
	}
	c.Flags().StringSliceVar(&only, "only", nil, "filter: success, failure, skipped, or marks ok, fail, skip, key, none")
	c.Flags().BoolVar(&asJSON, "json", false, "JSON output")
	return c
}

// stepFilter builds a predicate from --only values (OR between values).
func stepFilter(only []string) (func(core.Step) bool, error) {
	if len(only) == 0 {
		return func(core.Step) bool { return true }, nil
	}
	var preds []func(core.Step) bool
	for _, o := range only {
		switch strings.ToLower(strings.TrimSpace(o)) {
		case "success", "succeeded":
			preds = append(preds, func(s core.Step) bool { return s.Outcome() == core.OutcomeSuccess })
		case "failure", "failed":
			preds = append(preds, func(s core.Step) bool { return s.Outcome() == core.OutcomeFailure })
		case "skipped":
			preds = append(preds, func(s core.Step) bool { return s.Outcome() == core.OutcomeSkipped })
		default:
			m, err := core.ParseMark(o)
			if err != nil {
				return nil, err
			}
			preds = append(preds, func(s core.Step) bool { return s.Mark == m })
		}
	}
	return func(s core.Step) bool {
		for _, p := range preds {
			if p(s) {
				return true
			}
		}
		return false
	}, nil
}

func newDistillCmd(a *App) *cobra.Command {
	var (
		format      string
		output      string
		keepNoise   bool
		showDropped bool
	)
	c := &cobra.Command{
		Use:   "distill [name|id]",
		Short: "Produce a clean runbook (Markdown) or script from a messy session",
		Long: `Distill keeps only what is needed to reproduce the result:

  1. drops failed and skipped steps (your marks win over exit codes)
  2. drops read-only/navigation noise (ls, cat, cd...) unless marked key
  3. collapses consecutive repeats of the same command

Directory changes are re-created from each step's working directory.`,
		Example: `  apura distill setup-k3s -o setup-k3s.md
  apura distill setup-k3s --format sh -o setup-k3s.fish
  apura distill --show-dropped`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			ref := ""
			if len(args) == 1 {
				ref = args[0]
			}
			s, err := a.session(ctx, ref)
			if err != nil {
				return err
			}
			st, _ := a.Store()
			steps, err := st.Steps(ctx, s.ID)
			if err != nil {
				return err
			}
			notes, err := st.Notes(ctx, s.ID)
			if err != nil {
				return err
			}
			sh, err := shellFor(s.Shell)
			if err != nil {
				return err
			}
			res := distill.Run(steps, distill.Options{KeepNoise: keepNoise})
			doc := distill.Doc{Session: s, SessionNotes: notes, Result: res, Shell: sh}

			var text string
			switch format {
			case "md", "markdown":
				text = distill.RenderMarkdown(doc)
			case "sh", "script":
				text = distill.RenderScript(doc)
			default:
				return fmt.Errorf("unknown format %q (md or sh)", format)
			}

			if showDropped {
				for _, d := range res.Dropped {
					fmt.Fprintf(cmd.ErrOrStderr(), "dropped #%d [%s] %s\n", d.Step.Seq, d.Rule, firstLine(d.Step.Command, 80))
				}
			}
			if output == "" || output == "-" {
				_, err = io.WriteString(cmd.OutOrStdout(), text)
				return err
			}
			mode := os.FileMode(0o644)
			if format == "sh" || format == "script" {
				mode = 0o755
			}
			if err := os.WriteFile(output, []byte(text), mode); err != nil {
				return err
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "wrote %s: %d steps kept, %d dropped\n", output, len(res.Steps), len(res.Dropped))
			return nil
		},
	}
	c.Flags().StringVarP(&format, "format", "f", "md", "output format: md or sh")
	c.Flags().StringVarP(&output, "output", "o", "", "write to file instead of stdout")
	c.Flags().BoolVar(&keepNoise, "keep-noise", false, "keep read-only/navigation commands")
	c.Flags().BoolVar(&showDropped, "show-dropped", false, "list dropped steps and why (stderr)")
	return c
}

// shellFor returns the integration a session was recorded with, falling back
// to the current shell and then to fish.
func shellFor(name string) (shell.Integration, error) {
	for _, n := range []string{name, os.Getenv("APURA_SHELL"), "fish"} {
		if n == "" {
			continue
		}
		if sh, err := shell.Get(n); err == nil {
			return sh, nil
		}
	}
	return nil, fmt.Errorf("no shell integration available")
}
