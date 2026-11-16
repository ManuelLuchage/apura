package cmdparse

import (
	"reflect"
	"testing"
)

func TestPrograms(t *testing.T) {
	cases := []struct {
		line string
		want []string
	}{
		{"ls -la", []string{"ls"}},
		{"sudo apt install -y nginx", []string{"apt"}},
		{"sudo -u postgres psql -c 'select 1'", []string{"psql"}},
		{"FOO=1 BAR=2 go test ./...", []string{"go"}},
		{"env GOOS=linux go build", []string{"go"}},
		{"timeout 10s curl -s https://x | jq .a", []string{"curl", "jq"}},
		{"make build && ./bin/app --flag", []string{"make", "app"}},
		{"cd /tmp; ls", []string{"cd", "ls"}},
		// fish syntax
		{"docker ps; and docker logs web", []string{"docker", "docker"}},
		{"set -gx PATH $PATH ~/bin", []string{"set"}},
		{"kubectl get pods (kubectl config current-context)", []string{"kubectl", "kubectl"}},
		{"for f in *.go; gofmt -l $f; end", []string{"gofmt"}},
		{"if test -f x; echo yes; end", []string{"test", "echo"}},
		{"begin; git add .; git commit -m 'a; b'; end", []string{"git", "git"}},
		// bash syntax
		{"echo $(date) > out.txt 2>&1", []string{"echo", "date"}},
		{"cmd &> /dev/null", []string{"cmd"}},
		{"/usr/local/bin/terraform plan", []string{"terraform"}},
		{"git commit -m \"fix: \\\"quoted\\\"\"", []string{"git"}},
		{"   ", nil},
		{"# just a comment", nil},
		{"$EDITOR file.txt", nil},
	}
	for _, c := range cases {
		got := Programs(c.line)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("Programs(%q) = %v, want %v", c.line, got, c.want)
		}
	}
}

func TestFirstProgram(t *testing.T) {
	for line, want := range map[string]string{
		"apura note hi":                "apura",
		"command apura mark ok":        "apura",
		"sudo systemctl restart nginx": "systemctl",
		"":                             "",
	} {
		if got := FirstProgram(line); got != want {
			t.Errorf("FirstProgram(%q) = %q, want %q", line, got, want)
		}
	}
}
