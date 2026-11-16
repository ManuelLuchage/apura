package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ManuelLuchage/apura/internal/core"
)

// parseSeqs accepts "3", "3-5" and "3,4,7" (and mixes of them).
func parseSeqs(args []string) ([]int, error) {
	var out []int
	for _, a := range args {
		for _, p := range strings.Split(a, ",") {
			p = strings.TrimPrefix(strings.TrimSpace(p), "#")
			if p == "" {
				continue
			}
			if lo, hi, ok := strings.Cut(p, "-"); ok {
				l, err1 := strconv.Atoi(lo)
				h, err2 := strconv.Atoi(hi)
				if err1 != nil || err2 != nil || l > h {
					return nil, fmt.Errorf("invalid range %q", p)
				}
				for i := l; i <= h; i++ {
					out = append(out, i)
				}
				continue
			}
			n, err := strconv.Atoi(p)
			if err != nil {
				return nil, fmt.Errorf("invalid step number %q", p)
			}
			out = append(out, n)
		}
	}
	return out, nil
}

func newNoteCmd(a *App) *cobra.Command {
	var (
		ref       string
		step      int
		onSession bool
	)
	c := &cobra.Command{
		Use:   "note <text>...",
		Short: "Comment the last step (or a given step, or the whole session)",
		Example: `  apura note "port 6443 was blocked by ufw"
  apura note --step 4 "needs sudo on Ubuntu 24.04"
  apura note --session "works only with k3s >= 1.30"`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			s, err := a.session(ctx, ref)
			if err != nil {
				return err
			}
			st, _ := a.Store()
			body := strings.Join(args, " ")
			var target *core.Step
			if !onSession {
				var x core.Step
				if step > 0 {
					x, err = st.StepBySeq(ctx, s.ID, step)
				} else {
					x, err = st.LastStep(ctx, s.ID)
				}
				if err == nil {
					target = &x
				} else if step > 0 {
					return fmt.Errorf("step %d not found in %q", step, s.Name)
				}
			}
			if target == nil {
				_, err = st.AddNote(ctx, s.ID, nil, body)
				if err == nil {
					fmt.Fprintf(cmd.OutOrStdout(), "noted on session %q\n", s.Name)
				}
				return err
			}
			if _, err := st.AddNote(ctx, s.ID, &target.ID, body); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "noted on #%d %s\n", target.Seq, firstLine(target.Command, 60))
			return nil
		},
	}
	c.Flags().StringVarP(&ref, "in", "i", "", "session name or id (default: active)")
	c.Flags().IntVar(&step, "step", 0, "step number (default: last)")
	c.Flags().BoolVar(&onSession, "session", false, "attach the note to the session instead of a step")
	return c
}

func newMarkCmd(a *App) *cobra.Command {
	var ref string
	c := &cobra.Command{
		Use:   "mark <ok|fail|skip|key|none> [step...]",
		Short: "Judge steps: ok, fail, skip (irrelevant), key (essential) or none (clear)",
		Long: `Mark the last step, or the given steps, with your verdict.

  ok    worked as intended (kept even if the exit code was non-zero)
  fail  did not achieve the goal (dropped even if the exit code was 0)
  skip  exploratory / irrelevant (always dropped when distilling)
  key   essential (always kept, even if it looks like noise)
  none  clear the mark; the exit code decides again`,
		Example: `  apura mark fail          # last step
  apura mark ok 3 5 8-10
  apura mark key 2`,
		Args:      cobra.MinimumNArgs(1),
		ValidArgs: []string{"ok", "fail", "skip", "key", "none"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			mark, err := core.ParseMark(args[0])
			if err != nil {
				return err
			}
			s, err := a.session(ctx, ref)
			if err != nil {
				return err
			}
			st, _ := a.Store()
			seqs, err := parseSeqs(args[1:])
			if err != nil {
				return err
			}
			if len(seqs) == 0 {
				last, err := st.LastStep(ctx, s.ID)
				if err != nil {
					return fmt.Errorf("session %q has no steps yet", s.Name)
				}
				seqs = []int{last.Seq}
			}
			n, err := st.SetMark(ctx, s.ID, seqs, mark)
			if err != nil {
				return err
			}
			label := string(mark)
			if label == "" {
				label = "none"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s %d step(s) as %s\n", markSymbol(core.Step{Mark: mark}), n, label)
			return nil
		},
	}
	c.Flags().StringVarP(&ref, "session", "s", "", "session name or id (default: active)")
	return c
}

func newRmCmd(a *App) *cobra.Command {
	var (
		ref    string
		failed bool
		dryRun bool
	)
	c := &cobra.Command{
		Use:   "rm [step...]",
		Short: "Remove steps from a session (e.g. all failed ones)",
		Example: `  apura rm 4 6-7
  apura rm --failed            # every failed or skipped step
  apura rm --failed --dry-run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			s, err := a.session(ctx, ref)
			if err != nil {
				return err
			}
			st, _ := a.Store()
			seqs, err := parseSeqs(args)
			if err != nil {
				return err
			}
			if failed {
				steps, err := st.Steps(ctx, s.ID)
				if err != nil {
					return err
				}
				for _, x := range steps {
					if x.Outcome() != core.OutcomeSuccess {
						seqs = append(seqs, x.Seq)
					}
				}
			}
			if len(seqs) == 0 {
				return fmt.Errorf("nothing to remove: give step numbers or --failed")
			}
			if dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "would remove steps %v from %q\n", seqs, s.Name)
				return nil
			}
			n, err := st.DeleteSteps(ctx, s.ID, seqs)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed %d step(s) from %q\n", n, s.Name)
			return nil
		},
	}
	c.Flags().StringVarP(&ref, "session", "s", "", "session name or id (default: active)")
	c.Flags().BoolVar(&failed, "failed", false, "remove every failed or skipped step")
	c.Flags().BoolVarP(&dryRun, "dry-run", "n", false, "only show what would be removed")
	return c
}
