package cli

import (
	"bytes"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ManuelLuchage/apura/internal/core"
)

// run executes the CLI in-process with an isolated database.
func run(t *testing.T, db string, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	root, a := NewRoot("test")
	defer a.Close()
	var out, errb bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errb)
	root.SetArgs(append([]string{"--db", db}, args...))
	err = root.Execute()
	return out.String(), errb.String(), err
}

func TestParseSeqs(t *testing.T) {
	got, err := parseSeqs([]string{"1", "3-5", "7,9", "#10"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []int{1, 3, 4, 5, 7, 9, 10}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
	if _, err := parseSeqs([]string{"5-3"}); err == nil {
		t.Fatal("want error for inverted range")
	}
}

func TestShouldRecord(t *testing.T) {
	for line, want := range map[string]bool{
		"ls":              true,
		" secret command": false,
		"apura note x":    false,
		"":                false,
	} {
		if shouldRecord(line) != want {
			t.Errorf("shouldRecord(%q) != %v", line, want)
		}
	}
}

// TestFlow simulates what the fish integration does, without fish.
func TestFlow(t *testing.T) {
	db := filepath.Join(t.TempDir(), "t.db")
	t.Setenv(core.EnvEmit, "fish")
	t.Setenv(core.EnvShell, "fish")
	t.Setenv(core.EnvSession, "")

	out, _, err := run(t, db, "start", "demo", "-g", "test flow", "-t", "go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "set -gx APURA_SESSION '1'") {
		t.Fatalf("start must emit env code on stdout, got %q", out)
	}
	t.Setenv(core.EnvSession, "1")

	rec := func(exit, cmd string) {
		if _, _, err := run(t, db, "_record", "--session", "1", "--exit", exit, "--duration", "1200",
			"--cwd", "/tmp", "--shell", "fish", "--", cmd); err != nil {
			t.Fatal(err)
		}
	}
	rec("1", "go biuld ./...")
	rec("0", "go build ./...")
	rec("0", "ls")
	rec("0", "apura note ignored")
	rec("0", "export API_TOKEN=abc")

	if _, _, err := run(t, db, "note", "--step", "2", "builds in 1s"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := run(t, db, "mark", "skip"); err != nil { // last step (the export)
		t.Fatal(err)
	}

	out, _, err = run(t, db, "show", "--json")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"step_count": 4`, `API_TOKEN=***`, `"builds in 1s"`, `"mark": "skip"`, `"go"`} {
		if !strings.Contains(out, want) {
			t.Errorf("show --json missing %s:\n%s", want, out)
		}
	}

	out, _, err = run(t, db, "distill", "--format", "sh")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "go build ./...") || strings.Contains(out, "biuld") || strings.Contains(out, "\nls\n") || strings.Contains(out, "API_TOKEN") {
		t.Fatalf("distill output wrong:\n%s", out)
	}

	out, _, err = run(t, db, "search", "build", "--tech", "go")
	if err != nil || !strings.Contains(out, "demo") {
		t.Fatalf("search: %v\n%s", err, out)
	}

	out, _, err = run(t, db, "end", "-r", "success")
	if err != nil || !strings.Contains(out, "set -e APURA_SESSION") {
		t.Fatalf("end must unset env: %v %q", err, out)
	}
	t.Setenv(core.EnvSession, "")

	out, _, _ = run(t, db, "ls", "--result", "success")
	if !strings.Contains(out, "demo") {
		t.Fatalf("ls: %s", out)
	}
	if _, _, err := run(t, db, "start", "demo"); err == nil {
		t.Fatal("duplicate name must fail")
	}
}
