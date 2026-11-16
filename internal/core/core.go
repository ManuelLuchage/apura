// Package core holds the shell-agnostic domain model of apura: sessions,
// steps, marks and notes. It must not import any other internal package.
package core

import (
	"fmt"
	"strings"
	"time"
)

// Environment variables shared between the binary and every shell integration.
// They are the contract that lets new shells plug in without touching the core.
const (
	EnvSession     = "APURA_SESSION"      // id of the active session in this shell
	EnvSessionName = "APURA_SESSION_NAME" // name of the active session (for prompts)
	EnvShell       = "APURA_SHELL"        // set by the init script: "fish", "zsh", ...
	EnvEmit        = "APURA_EMIT_ENV"     // set by the shell wrapper: print env code on stdout
	EnvDB          = "APURA_DB"           // override database path
)

// Mark is the user's verdict on a step.
type Mark string

const (
	MarkNone Mark = ""     // no verdict; outcome derived from exit code
	MarkOK   Mark = "ok"   // worked as intended
	MarkFail Mark = "fail" // did not achieve the goal (even if exit code was 0)
	MarkSkip Mark = "skip" // irrelevant / exploratory; never part of a distilled run
	MarkKey  Mark = "key"  // essential: always kept, even if it looks like noise
)

// ParseMark accepts the user-facing names (and "none"/"clear" to reset).
func ParseMark(s string) (Mark, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "ok", "success", "good":
		return MarkOK, nil
	case "fail", "failed", "bad":
		return MarkFail, nil
	case "skip", "ignore":
		return MarkSkip, nil
	case "key", "important", "star":
		return MarkKey, nil
	case "none", "clear", "":
		return MarkNone, nil
	}
	return MarkNone, fmt.Errorf("unknown mark %q (use ok, fail, skip, key or none)", s)
}

// Outcome is the effective result of a step, combining mark and exit code.
type Outcome string

const (
	OutcomeSuccess Outcome = "success"
	OutcomeFailure Outcome = "failure"
	OutcomeSkipped Outcome = "skipped"
)

// SessionState tells whether a session is still recording.
type SessionState string

const (
	StateActive SessionState = "active"
	StateEnded  SessionState = "ended"
)

// Result is the user's verdict on a whole session.
type Result string

const (
	ResultNone      Result = ""
	ResultSuccess   Result = "success"
	ResultFailure   Result = "failure"
	ResultAbandoned Result = "abandoned"
)

// ParseResult accepts the user-facing names for a session result.
func ParseResult(s string) (Result, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "success", "ok", "done":
		return ResultSuccess, nil
	case "failure", "fail", "failed":
		return ResultFailure, nil
	case "abandoned", "abandon", "dropped":
		return ResultAbandoned, nil
	case "", "none":
		return ResultNone, nil
	}
	return ResultNone, fmt.Errorf("unknown result %q (use success, failure or abandoned)", s)
}

// Session is a named, goal-oriented recording of terminal work.
type Session struct {
	ID        int64
	Name      string
	Goal      string
	State     SessionState
	Result    Result
	Shell     string
	Host      string
	StartedAt time.Time
	EndedAt   *time.Time
	Tags      []string

	// Aggregates filled by listing queries.
	StepCount int
	Techs     []string
}

// Step is one command executed inside a session.
type Step struct {
	ID        int64
	SessionID int64
	Seq       int // position inside the session, 1-based, stable (gaps allowed after rm)
	Command   string
	Cwd       string
	Shell     string
	ExitCode  int
	Duration  time.Duration
	Mark      Mark
	Redacted  bool // secrets were masked before storing
	CreatedAt time.Time
	Techs     []string
	Notes     []Note
}

// Outcome combines the explicit mark with the exit code.
func (s Step) Outcome() Outcome {
	switch s.Mark {
	case MarkOK, MarkKey:
		return OutcomeSuccess
	case MarkFail:
		return OutcomeFailure
	case MarkSkip:
		return OutcomeSkipped
	}
	if s.ExitCode == 0 {
		return OutcomeSuccess
	}
	return OutcomeFailure
}

// Note is a free-text comment on a session or on one of its steps.
type Note struct {
	ID        int64
	SessionID int64
	StepID    *int64 // nil = note about the whole session
	Body      string
	CreatedAt time.Time
}
