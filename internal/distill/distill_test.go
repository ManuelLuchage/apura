package distill

import (
	"strings"
	"testing"

	"github.com/ManuelLuchage/apura/internal/core"
	"github.com/ManuelLuchage/apura/internal/shell"
	_ "github.com/ManuelLuchage/apura/internal/shell/fish"
)

func step(seq int, cmd string, exit int, mark core.Mark) core.Step {
	return core.Step{Seq: seq, Command: cmd, ExitCode: exit, Mark: mark, Cwd: "/srv/app"}
}

func commands(steps []core.Step) []string {
	var out []string
	for _, s := range steps {
		out = append(out, s.Command)
	}
	return out
}

func TestRun(t *testing.T) {
	in := []core.Step{
		step(1, "ls", 0, ""),
		step(2, "apt install nginx", 100, ""),                 // failed: needs sudo
		step(3, "sudo apt install nginx", 0, ""),              // fixed
		step(4, "sudo apt install nginx", 0, ""),              // repeat
		step(5, "cat /etc/nginx/nginx.conf", 0, core.MarkKey), // noise but key
		step(6, "sudo nginx -t", 0, core.MarkFail),            // exit 0 but user says fail
		step(7, "systemctl is-active nginx", 3, core.MarkOK),  // exit 3 but ok
		step(8, "curl localhost", 0, core.MarkSkip),
		step(9, "sudo systemctl restart nginx", 0, ""),
	}
	res := Run(in, Options{})
	want := []string{
		"sudo apt install nginx",
		"cat /etc/nginx/nginx.conf",
		"systemctl is-active nginx",
		"sudo systemctl restart nginx",
	}
	if got := commands(res.Steps); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %q\nwant %q", got, want)
	}
	rules := map[int]string{}
	for _, d := range res.Dropped {
		rules[d.Step.Seq] = d.Rule
	}
	expect := map[int]string{1: "noise", 2: "unsuccessful", 4: "repeat", 6: "unsuccessful", 8: "unsuccessful"}
	for seq, r := range expect {
		if rules[seq] != r {
			t.Errorf("step %d dropped by %q, want %q", seq, rules[seq], r)
		}
	}
}

func TestKeepNoise(t *testing.T) {
	res := Run([]core.Step{step(1, "ls", 0, "")}, Options{KeepNoise: true})
	if len(res.Steps) != 1 {
		t.Fatal("noise should be kept")
	}
}

func TestRenderScriptFish(t *testing.T) {
	sh, _ := shell.Get("fish")
	steps := []core.Step{
		{Seq: 1, Command: "make build", Cwd: "/srv/my app", Notes: []core.Note{{Body: "needs go 1.24"}}},
		{Seq: 2, Command: "make test", Cwd: "/srv/my app"},
	}
	out := RenderScript(Doc{Session: core.Session{Name: "demo"}, Result: Result{Steps: steps}, Shell: sh})
	for _, want := range []string{"#!/usr/bin/env fish", "cd '/srv/my app'", "# [1] needs go 1.24", "make build\nor exit $status"} {
		if !strings.Contains(out, want) {
			t.Errorf("script missing %q:\n%s", want, out)
		}
	}
	if strings.Count(out, "cd ") != 1 {
		t.Errorf("cd should be emitted once:\n%s", out)
	}
}

func TestRenderMarkdown(t *testing.T) {
	sh, _ := shell.Get("fish")
	out := RenderMarkdown(Doc{
		Session: core.Session{Name: "demo", Goal: "ship it", Tags: []string{"go"}},
		Result:  Result{Steps: []core.Step{{Seq: 1, Command: "go build", Cwd: "/x"}}, Dropped: []Dropped{{Rule: "noise"}}},
		Shell:   sh,
	})
	for _, want := range []string{"# demo", "> ship it", "```fish", "go build", "1 of 2 steps removed"} {
		if !strings.Contains(out, want) {
			t.Errorf("markdown missing %q:\n%s", want, out)
		}
	}
}
