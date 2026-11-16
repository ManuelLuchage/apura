// Package distill turns a messy session (attempts, errors, exploration) into
// the shortest ordered list of steps that reproduces the result.
//
// It is deterministic: the same session always produces the same output.
// The pipeline is a list of Rules; each rule sees the output of the previous
// one and reports what it dropped, so users can audit every decision.
package distill

import (
	"strings"

	"github.com/ManuelLuchage/apura/internal/core"
	"github.com/ManuelLuchage/apura/internal/tech"
)

// Options tune the pipeline.
type Options struct {
	KeepNoise bool // keep read-only/navigation commands (ls, cat, cd...)
}

// Dropped records why a step was removed.
type Dropped struct {
	Step core.Step
	Rule string
}

// Result is the distilled run plus the audit apura.
type Result struct {
	Steps   []core.Step
	Dropped []Dropped
}

// Rule is one transformation of the pipeline.
type Rule interface {
	Name() string
	// Apply returns the steps to keep, in order. It must not reorder steps.
	Apply(steps []core.Step) (kept []core.Step, dropped []core.Step)
}

// Pipeline returns the default rules for the given options, in order.
func Pipeline(opts Options) []Rule {
	rules := []Rule{dropUnsuccessful{}}
	if !opts.KeepNoise {
		rules = append(rules, dropNoise{})
	}
	rules = append(rules, collapseRepeats{})
	return rules
}

// Run applies the pipeline to the steps of a session.
func Run(steps []core.Step, opts Options) Result {
	return RunRules(steps, Pipeline(opts))
}

// RunRules applies an explicit list of rules (useful for tests and plugins).
func RunRules(steps []core.Step, rules []Rule) Result {
	res := Result{Steps: steps}
	for _, r := range rules {
		kept, dropped := r.Apply(res.Steps)
		for _, d := range dropped {
			res.Dropped = append(res.Dropped, Dropped{Step: d, Rule: r.Name()})
		}
		res.Steps = kept
	}
	return res
}

// dropUnsuccessful removes failed and skipped steps. A step marked ok/key is
// kept even with a non-zero exit code (e.g. `grep` finding nothing on purpose).
type dropUnsuccessful struct{}

func (dropUnsuccessful) Name() string { return "unsuccessful" }
func (dropUnsuccessful) Apply(in []core.Step) (kept, dropped []core.Step) {
	for _, s := range in {
		if s.Outcome() == core.OutcomeSuccess {
			kept = append(kept, s)
		} else {
			dropped = append(dropped, s)
		}
	}
	return
}

// dropNoise removes read-only/navigation commands unless marked key.
type dropNoise struct{}

func (dropNoise) Name() string { return "noise" }
func (dropNoise) Apply(in []core.Step) (kept, dropped []core.Step) {
	for _, s := range in {
		if s.Mark != core.MarkKey && tech.IsNoise(s.Command) {
			dropped = append(dropped, s)
		} else {
			kept = append(kept, s)
		}
	}
	return
}

// collapseRepeats removes a step identical to the previous kept one (same
// normalized command and directory) — typical of "run it again to be sure".
// Non-consecutive repeats are kept: restarting a service after editing a
// config is meaningful.
type collapseRepeats struct{}

func (collapseRepeats) Name() string { return "repeat" }
func (collapseRepeats) Apply(in []core.Step) (kept, dropped []core.Step) {
	for _, s := range in {
		if n := len(kept); n > 0 && s.Mark != core.MarkKey &&
			normalize(kept[n-1].Command) == normalize(s.Command) && kept[n-1].Cwd == s.Cwd {
			dropped = append(dropped, s)
			continue
		}
		kept = append(kept, s)
	}
	return
}

func normalize(cmd string) string { return strings.Join(strings.Fields(cmd), " ") }
