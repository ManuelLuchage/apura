// Package cmdparse extracts the programs invoked by a command line.
//
// It is deliberately a small, forgiving tokenizer rather than a full shell
// parser: it must accept fish, bash, zsh and POSIX syntax alike, and never
// fail. A shell-specific parser (e.g. mvdan.cc/sh for bash/zsh) can be plugged
// in later behind the same functions if more precision is needed.
package cmdparse

import (
	"path/filepath"
	"strings"
)

// Words that start a new command segment or are pure syntax (fish + POSIX).
var segmentKeywords = map[string]bool{
	"and": true, "or": true, "not": true, "!": true,
	"begin": true, "end": true, "if": true, "then": true, "else": true, "elif": true, "fi": true,
	"while": true, "do": true, "done": true, "{": true, "}": true,
}

// Block headers whose words are not a command (`for x in a b`, `switch $v`).
var headerKeywords = map[string]bool{"for": true, "switch": true, "case": true, "function": true}

// Wrappers run another command; we skip them (and their flags) to find it.
var wrappers = map[string]bool{
	"sudo": true, "doas": true, "env": true, "time": true, "nohup": true, "nice": true,
	"ionice": true, "stdbuf": true, "timeout": true, "watch": true, "xargs": true,
	"command": true, "builtin": true, "exec": true, "noglob": true, "caffeinate": true,
	"strace": true, "chronic": true, "unbuffer": true,
}

// Flags of wrappers that consume the following argument.
var wrapperArgFlags = map[string]bool{
	"-u": true, "-g": true, "-n": true, "-c": true, "-s": true, "-C": true, "-D": true,
	"--user": true, "--group": true, "-k": true,
}

// Programs returns the programs invoked in line, in order, possibly repeated.
// Wrappers like sudo/env are skipped; paths are reduced to their base name.
func Programs(line string) []string {
	var out []string
	for _, seg := range Segments(line) {
		if p := programOf(seg); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// FirstProgram returns the first program of the line, or "".
func FirstProgram(line string) string {
	if ps := Programs(line); len(ps) > 0 {
		return ps[0]
	}
	return ""
}

// Segments splits a line into simple commands (lists of words), breaking on
// |, ||, &&, ;, &, newlines, parentheses and segment keywords.
func Segments(line string) [][]string {
	var (
		segs [][]string
		cur  []string
		word strings.Builder
		inW  bool
	)
	flushWord := func() {
		if inW {
			w := word.String()
			// A keyword at the start of a segment (`and`, `if`, `begin`...) is syntax:
			// drop it and keep looking for the program.
			if !(segmentKeywords[w] && len(cur) == 0) {
				cur = append(cur, w)
			}
			word.Reset()
			inW = false
		}
	}
	rs := []rune(line)
	for i := 0; i < len(rs); i++ {
		c := rs[i]
		switch {
		case c == '\\' && i+1 < len(rs):
			i++
			if rs[i] != '\n' {
				word.WriteRune(rs[i])
				inW = true
			}
		case c == '\'' || c == '"':
			inW = true
			q := c
			for i++; i < len(rs) && rs[i] != q; i++ {
				if rs[i] == '\\' && q == '"' && i+1 < len(rs) {
					i++
				}
				word.WriteRune(rs[i])
			}
		case c == '#' && !inW:
			// comment until end of line
			for i < len(rs) && rs[i] != '\n' {
				i++
			}
			flushWord()
			flushSeg(&segs, &cur)
		case c == ' ' || c == '\t':
			flushWord()
		case c == '|' || c == '&' || c == ';' || c == '\n' || c == '(' || c == ')':
			// `$(` / `&>` / `2>&1` are close enough: treated as separators or redirects.
			if c == '&' && i+1 < len(rs) && rs[i+1] == '>' { // &> redirect
				flushWord()
				i++
				skipRedirectTarget(rs, &i)
				continue
			}
			if inW && c == '&' && strings.HasSuffix(word.String(), ">") { // 2>&1
				word.WriteRune(c)
				continue
			}
			if inW && strings.HasSuffix(word.String(), "$") && c == '(' {
				s := word.String()
				word.Reset()
				word.WriteString(s[:len(s)-1])
				inW = word.Len() > 0
			}
			flushWord()
			flushSeg(&segs, &cur)
		default:
			word.WriteRune(c)
			inW = true
		}
	}
	flushWord()
	flushSeg(&segs, &cur)
	return segs
}

func skipRedirectTarget(rs []rune, i *int) {
	for *i+1 < len(rs) && rs[*i+1] == ' ' {
		*i++
	}
	for *i+1 < len(rs) && !strings.ContainsRune(" \t;|&\n", rs[*i+1]) {
		*i++
	}
}

func flushSeg(segs *[][]string, cur *[]string) {
	if len(*cur) > 0 {
		*segs = append(*segs, *cur)
		*cur = nil
	}
}

func isAssignment(w string) bool {
	eq := strings.IndexByte(w, '=')
	if eq <= 0 {
		return false
	}
	for i, r := range w[:eq] {
		if !(r == '_' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || i > 0 && r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}

func isRedirect(w string) bool {
	return strings.ContainsAny(w[:1], "<>") || len(w) > 1 && w[0] >= '0' && w[0] <= '9' && strings.ContainsAny(w[1:2], "<>")
}

func isRedirectOp(w string) bool {
	t := strings.TrimLeft(w, "0123456789&")
	t = strings.TrimRight(t, "&|")
	return t != "" && strings.Trim(t, "<>") == ""
}

func programOf(words []string) string {
	if len(words) > 0 && headerKeywords[words[0]] {
		return ""
	}
	i := 0
	for i < len(words) {
		w := words[i]
		switch {
		case isRedirectOp(w): // `>` `2>>` `&>` alone: the next word is its target
			i += 2
		case isAssignment(w), isRedirect(w):
			i++
		case wrappers[w]:
			i++
			// skip wrapper flags and their arguments, and numeric args (timeout 10, nice -n 5)
			for i < len(words) {
				a := words[i]
				if strings.HasPrefix(a, "-") {
					i++
					if wrapperArgFlags[a] && i < len(words) {
						i++
					}
					continue
				}
				if isAssignment(a) || isNumberish(a) {
					i++
					continue
				}
				break
			}
		default:
			if strings.HasPrefix(w, "$") { // variable used as command
				return ""
			}
			return filepath.Base(w)
		}
	}
	return ""
}

func isNumberish(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !(r >= '0' && r <= '9' || r == '.' || r == 's' || r == 'm' || r == 'h') {
			return false
		}
	}
	return s[0] >= '0' && s[0] <= '9'
}
