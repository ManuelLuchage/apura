// Package fish implements the fish shell integration.
package fish

import (
	_ "embed"
	"strings"

	"github.com/ManuelLuchage/apura/internal/shell"
)

//go:embed apura.fish
var script string

type fish struct{}

func init() { shell.Register(fish{}) }

func (fish) Name() string { return "fish" }

func (f fish) InitScript(bin string) string {
	return strings.ReplaceAll(script, "@APURA_BIN@", f.Quote(bin))
}

func (f fish) SetEnv(key, value string) string {
	return "set -gx " + key + " " + f.Quote(value)
}

func (fish) UnsetEnv(key string) string { return "set -e " + key }

// Quote uses single quotes; inside them fish only interprets \\ and \'.
func (fish) Quote(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `'`, `\'`)
	return "'" + r.Replace(s) + "'"
}

func (fish) ScriptHeader() string { return "#!/usr/bin/env fish" }

// StepGuard: fish has no `set -e`, so each step is followed by an explicit check.
func (fish) StepGuard() string { return "or exit $status" }

func (fish) FenceLang() string { return "fish" }
