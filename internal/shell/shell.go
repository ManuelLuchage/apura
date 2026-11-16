// Package shell defines the contract every shell integration implements and a
// registry of available integrations. The rest of apura only talks to shells
// through this interface, so adding zsh/bash/nushell means adding one package
// under internal/shell/<name> plus one import in internal/shell/all.
//
// See docs/SHELL_INTEGRATION.md for the full checklist.
package shell

import (
	"fmt"
	"sort"
	"sync"
)

// Integration is everything apura needs to know about a shell.
type Integration interface {
	// Name is the identifier used on the CLI and stored with each step ("fish").
	Name() string

	// InitScript returns the code users load in their shell config, e.g.
	// `apura init fish | source`. bin is the absolute path of the apura binary.
	// It must:
	//   1. export APURA_SHELL=<name>;
	//   2. define a `apura` wrapper that, for start/resume/end, runs the binary
	//      with APURA_EMIT_ENV=<name> and evaluates its stdout in the current shell;
	//   3. after every interactive command, if APURA_SESSION is set, call
	//      `<bin> _record --session $APURA_SESSION --exit <code> --duration <ms>
	//       --cwd <dir where the command started> -- <command line>`
	//      without blocking the prompt.
	InitScript(bin string) string

	// SetEnv / UnsetEnv return code that sets/unsets a global exported variable
	// in this shell. Used by start/resume/end to modify the parent shell.
	SetEnv(key, value string) string
	UnsetEnv(key string) string

	// Quote makes s a single literal argument in this shell's syntax.
	Quote(s string) string

	// ScriptHeader is placed at the top of exported scripts (shebang + strict mode).
	ScriptHeader() string

	// StepGuard is emitted on the line after each step of an exported script to
	// abort on failure. Return "" if ScriptHeader already enables that (set -e).
	StepGuard() string

	// FenceLang is the Markdown code-fence language for this shell's syntax.
	FenceLang() string
}

var (
	mu       sync.RWMutex
	registry = map[string]Integration{}
)

// Register makes an integration available. Called from each package's init().
func Register(i Integration) {
	mu.Lock()
	defer mu.Unlock()
	if _, dup := registry[i.Name()]; dup {
		panic("shell: duplicate integration " + i.Name())
	}
	registry[i.Name()] = i
}

// Get returns the integration for name.
func Get(name string) (Integration, error) {
	mu.RLock()
	defer mu.RUnlock()
	if i, ok := registry[name]; ok {
		return i, nil
	}
	return nil, fmt.Errorf("unsupported shell %q (available: %v)", name, Names())
}

// Names lists registered shells, sorted.
func Names() []string {
	var out []string
	for n := range registry {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}
