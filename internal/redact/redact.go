// Package redact masks secrets in command lines before they are stored.
// It errs on the side of masking: a false positive costs a manual fix in the
// runbook, a false negative leaks a credential into the database.
package redact

import "regexp"

const mask = "***"

type rule struct {
	re   *regexp.Regexp
	repl string
}

const sensitive = `[A-Za-z0-9_]*(?i:secret|token|passw(?:or)?d|passwd|api_?key|access_?key|private_?key|credential|auth_?token)[A-Za-z0-9_]*`
const value = `("[^"]*"|'[^']*'|\S+)`

var rules = []rule{
	// FOO_TOKEN=value  (POSIX assignment, env, export)
	{regexp.MustCompile(`\b(` + sensitive + `)=` + value), "${1}=" + mask},
	// fish: set -gx GITHUB_TOKEN value
	{regexp.MustCompile(`\b(set\s+(?:-[A-Za-z]+\s+)*` + sensitive + `)\s+` + value), "${1} " + mask},
	// --password value / --token=value / -p=value style flags
	{regexp.MustCompile(`(?i)(--?(?:password|passwd|pass|token|secret|api-?key|access-?key|client-secret|auth-token)(?:=|\s+))` + value), "${1}" + mask},
	// HTTP headers
	{regexp.MustCompile(`(?i)((?:authorization|x-api-key|x-auth-token)\s*:\s*(?:bearer\s+|basic\s+|token\s+)?)[^"'\s]+`), "${1}" + mask},
	// credentials in URLs: scheme://user:pass@host
	{regexp.MustCompile(`(://[^/\s:@]+:)[^@\s/]+@`), "${1}" + mask + "@"},
	// well-known token formats
	{regexp.MustCompile(`\b(?:ghp|gho|ghu|ghs|ghr)_[A-Za-z0-9]{20,}`), mask},
	{regexp.MustCompile(`\bgithub_pat_[A-Za-z0-9_]{20,}`), mask},
	{regexp.MustCompile(`\bglpat-[A-Za-z0-9_\-]{20,}`), mask},
	{regexp.MustCompile(`\bxox[abprs]-[A-Za-z0-9\-]{10,}`), mask},
	{regexp.MustCompile(`\bsk-[A-Za-z0-9_\-]{20,}`), mask},
	{regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`), mask},
}

// Line returns the line with secrets masked and whether anything changed.
func Line(s string) (string, bool) {
	out := s
	for _, r := range rules {
		out = r.re.ReplaceAllString(out, r.repl)
	}
	return out, out != s
}
