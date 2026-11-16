package redact

import (
	"strings"
	"testing"
)

func TestLine(t *testing.T) {
	cases := []struct{ in, want string }{
		{"export GITHUB_TOKEN=abc123", "export GITHUB_TOKEN=***"},
		{"DB_PASSWORD='s3 cret' ./run", "DB_PASSWORD=*** ./run"},
		{"set -gx OPENAI_API_KEY sk-xyz", "set -gx OPENAI_API_KEY ***"},
		{"mysql --password=hunter2 -u root", "mysql --password=*** -u root"},
		{"docker login --password hunter2", "docker login --password ***"},
		{`curl -H "Authorization: Bearer eyJhbGci.x.y" https://api`, `curl -H "Authorization: Bearer ***" https://api`},
		{"git clone https://user:pa55@github.com/x/y", "git clone https://user:***@github.com/x/y"},
		{"echo ghp_abcdefghijklmnopqrstuvwxyz0123", "echo ***"},
		{"aws configure set aws_access_key_id AKIAABCDEFGHIJKLMNOP", "aws configure set aws_access_key_id ***"},
	}
	for _, c := range cases {
		got, changed := Line(c.in)
		if got != c.want || !changed {
			t.Errorf("Line(%q) = %q (changed=%v), want %q", c.in, got, changed, c.want)
		}
	}
}

func TestLineLeavesHarmlessCommands(t *testing.T) {
	for _, in := range []string{
		"git commit --author='Ana <a@b.c>' -m 'x'",
		"ls -la /etc/ssh",
		"kubectl get secrets -n default",
		"docker run -p 8080:80 nginx",
		"ssh user@host",
	} {
		if got, changed := Line(in); changed {
			t.Errorf("Line(%q) changed to %q", in, got)
		}
	}
	// sanity: the mask never leaks the original
	if got, _ := Line("TOKEN=supersecret"); strings.Contains(got, "supersecret") {
		t.Fatal("secret leaked")
	}
}
