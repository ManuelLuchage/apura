package fish

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/ManuelLuchage/apura/internal/shell"
)

func TestRegistered(t *testing.T) {
	if _, err := shell.Get("fish"); err != nil {
		t.Fatal(err)
	}
}

func TestQuote(t *testing.T) {
	f := fish{}
	cases := map[string]string{
		"plain":       "'plain'",
		"it's":        `'it\'s'`,
		`back\slash`:  `'back\\slash'`,
		"$HOME (x) *": "'$HOME (x) *'",
	}
	for in, want := range cases {
		if got := f.Quote(in); got != want {
			t.Errorf("Quote(%q) = %s, want %s", in, got, want)
		}
	}
}

func TestInitScriptContract(t *testing.T) {
	s := fish{}.InitScript("/opt/my tools/apura")
	for _, want := range []string{
		"set -gx APURA_SHELL fish",
		"set -g __apura_bin '/opt/my tools/apura'",
		"--on-event fish_postexec",
		"--on-event fish_preexec",
		"APURA_EMIT_ENV=fish",
		"_record --session $APURA_SESSION",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("init script missing %q", want)
		}
	}
	if strings.Contains(s, "@APURA_BIN@") {
		t.Error("placeholder not replaced")
	}
}

// If fish is installed, make sure the generated code at least parses.
func TestInitScriptParses(t *testing.T) {
	path, err := exec.LookPath("fish")
	if err != nil {
		t.Skip("fish not installed")
	}
	cmd := exec.Command(path, "--no-execute")
	cmd.Stdin = strings.NewReader(fish{}.InitScript("/usr/bin/apura"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("fish --no-execute: %v\n%s", err, out)
	}
	// Quoting round-trip through real fish.
	f := fish{}
	tricky := `it's a \ "test" $HOME (x)`
	out, err := exec.Command(path, "-c", "printf %s "+f.Quote(tricky)).Output()
	if err != nil || string(out) != tricky {
		t.Fatalf("round-trip: got %q, err %v", out, err)
	}
}
