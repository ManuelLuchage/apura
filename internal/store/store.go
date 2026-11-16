// Package store persists sessions, steps, notes and tags in SQLite.
// It is the only package that knows SQL.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ManuelLuchage/apura/internal/core"
)

var (
	ErrNotFound  = errors.New("not found")
	ErrNameTaken = errors.New("session name already exists")
)

// Store wraps the SQLite database.
type Store struct {
	db *sql.DB
}

// Open creates the parent directory if needed, opens the database, applies
// connection pragmas and runs pending migrations.
func Open(path string) (*Store, error) {
	if path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return nil, fmt.Errorf("create data dir: %w", err)
		}
	}
	db, err := sql.Open(driverName, path)
	if err != nil {
		return nil, err
	}
	// A CLI needs a single connection; it also keeps per-connection pragmas valid.
	db.SetMaxOpenConns(1)
	for _, p := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000", // several shells may write at the same time
		"PRAGMA foreign_keys=ON",
		"PRAGMA synchronous=NORMAL",
	} {
		if _, err := db.Exec(p); err != nil {
			db.Close()
			return nil, fmt.Errorf("%s: %w", p, err)
		}
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	var version int
	if err := s.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	for i := version; i < len(migrations); i++ {
		tx, err := s.db.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(migrations[i]); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %d: %w", i+1, err)
		}
		if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version=%d", i+1)); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func ms(t time.Time) int64 { return t.UnixMilli() }
func fromMs(v int64) time.Time {
	return time.UnixMilli(v)
}

// ---------------------------------------------------------------- sessions

// CreateSession inserts a new active session.
func (s *Store) CreateSession(ctx context.Context, sess core.Session) (core.Session, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return sess, err
	}
	defer tx.Rollback()

	if sess.StartedAt.IsZero() {
		sess.StartedAt = time.Now()
	}
	sess.State = core.StateActive
	res, err := tx.ExecContext(ctx,
		`INSERT INTO sessions (name, goal, state, shell, host, started_at) VALUES (?,?,?,?,?,?)`,
		sess.Name, sess.Goal, sess.State, sess.Shell, sess.Host, ms(sess.StartedAt))
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return sess, ErrNameTaken
		}
		return sess, err
	}
	sess.ID, _ = res.LastInsertId()
	if err := insertTags(ctx, tx, sess.ID, sess.Tags); err != nil {
		return sess, err
	}
	return sess, tx.Commit()
}

func insertTags(ctx context.Context, tx *sql.Tx, sessionID int64, tags []string) error {
	for _, t := range normalizeList(tags) {
		if _, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO session_tags (session_id, tag) VALUES (?,?)`, sessionID, t); err != nil {
			return err
		}
	}
	return nil
}

// AddTags attaches extra tags to an existing session.
func (s *Store) AddTags(ctx context.Context, sessionID int64, tags []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := insertTags(ctx, tx, sessionID, tags); err != nil {
		return err
	}
	return tx.Commit()
}

// normalizeList lowercases, trims, splits on commas and de-duplicates.
func normalizeList(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, item := range in {
		for _, p := range strings.Split(item, ",") {
			p = strings.ToLower(strings.TrimSpace(p))
			if p != "" && !seen[p] {
				seen[p] = true
				out = append(out, p)
			}
		}
	}
	return out
}

const sessionCols = `s.id, s.name, s.goal, s.state, s.result, s.shell, s.host, s.started_at, s.ended_at,
    (SELECT COUNT(*) FROM steps st WHERE st.session_id = s.id),
    COALESCE((SELECT group_concat(tag) FROM session_tags t WHERE t.session_id = s.id), ''),
    COALESCE((SELECT group_concat(DISTINCT x.tech) FROM step_techs x
              JOIN steps st ON st.id = x.step_id WHERE st.session_id = s.id), '')`

type scanner interface{ Scan(...any) error }

func scanSession(r scanner) (core.Session, error) {
	var (
		sess          core.Session
		started       int64
		ended         sql.NullInt64
		tags, techs   string
		state, result string
	)
	if err := r.Scan(&sess.ID, &sess.Name, &sess.Goal, &state, &result, &sess.Shell, &sess.Host,
		&started, &ended, &sess.StepCount, &tags, &techs); err != nil {
		return sess, err
	}
	sess.State = core.SessionState(state)
	sess.Result = core.Result(result)
	sess.StartedAt = fromMs(started)
	if ended.Valid {
		t := fromMs(ended.Int64)
		sess.EndedAt = &t
	}
	sess.Tags = splitSorted(tags)
	sess.Techs = splitSorted(techs)
	return sess, nil
}

func splitSorted(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	sort.Strings(parts)
	return parts
}

// Session returns a session by id.
func (s *Store) Session(ctx context.Context, id int64) (core.Session, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+sessionCols+` FROM sessions s WHERE s.id = ?`, id)
	sess, err := scanSession(row)
	if errors.Is(err, sql.ErrNoRows) {
		return sess, fmt.Errorf("session %d: %w", id, ErrNotFound)
	}
	return sess, err
}

// SessionByName returns a session by its unique name.
func (s *Store) SessionByName(ctx context.Context, name string) (core.Session, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+sessionCols+` FROM sessions s WHERE s.name = ?`, name)
	sess, err := scanSession(row)
	if errors.Is(err, sql.ErrNoRows) {
		return sess, fmt.Errorf("session %q: %w", name, ErrNotFound)
	}
	return sess, err
}

// Resolve accepts either a numeric id or a name.
func (s *Store) Resolve(ctx context.Context, ref string) (core.Session, error) {
	if id, err := strconv.ParseInt(ref, 10, 64); err == nil {
		if sess, err := s.Session(ctx, id); err == nil {
			return sess, nil
		}
	}
	return s.SessionByName(ctx, ref)
}

// EndSession marks a session as ended with the given result.
func (s *Store) EndSession(ctx context.Context, id int64, result core.Result) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE sessions SET state = ?, result = ?, ended_at = ? WHERE id = ?`,
		core.StateEnded, result, ms(time.Now()), id)
	return err
}

// ReopenSession makes an ended session active again.
func (s *Store) ReopenSession(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE sessions SET state = ?, ended_at = NULL WHERE id = ?`, core.StateActive, id)
	return err
}

// DeleteSession removes a session and everything attached to it.
func (s *Store) DeleteSession(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, id)
	return err
}

// ListFilter narrows ListSessions. Zero values mean "no filter".
type ListFilter struct {
	Tag    string
	Tech   string // matches inferred techs or tags
	State  core.SessionState
	Result core.Result
	Query  string // free text over name, goal, tags, commands and notes
	Limit  int
}

// ListSessions returns sessions newest first.
func (s *Store) ListSessions(ctx context.Context, f ListFilter) ([]core.Session, error) {
	var (
		where []string
		args  []any
	)
	if f.Tag != "" {
		where = append(where, `EXISTS (SELECT 1 FROM session_tags t WHERE t.session_id = s.id AND t.tag = ?)`)
		args = append(args, strings.ToLower(f.Tag))
	}
	if f.Tech != "" {
		tech := strings.ToLower(f.Tech)
		where = append(where, `(EXISTS (SELECT 1 FROM step_techs x JOIN steps st ON st.id = x.step_id
                                 WHERE st.session_id = s.id AND x.tech = ?)
                 OR EXISTS (SELECT 1 FROM session_tags t WHERE t.session_id = s.id AND t.tag = ?))`)
		args = append(args, tech, tech)
	}
	if f.State != "" {
		where = append(where, `s.state = ?`)
		args = append(args, f.State)
	}
	if f.Result != "" {
		where = append(where, `s.result = ?`)
		args = append(args, f.Result)
	}
	if f.Query != "" {
		like := "%" + escapeLike(f.Query) + "%"
		where = append(where, `(s.name LIKE ? ESCAPE '\' OR s.goal LIKE ? ESCAPE '\'
            OR EXISTS (SELECT 1 FROM session_tags t WHERE t.session_id = s.id AND t.tag LIKE ? ESCAPE '\')
            OR EXISTS (SELECT 1 FROM steps st WHERE st.session_id = s.id AND st.command LIKE ? ESCAPE '\')
            OR EXISTS (SELECT 1 FROM notes n WHERE n.session_id = s.id AND n.body LIKE ? ESCAPE '\'))`)
		args = append(args, like, like, like, like, like)
	}
	q := `SELECT ` + sessionCols + ` FROM sessions s`
	if len(where) > 0 {
		q += ` WHERE ` + strings.Join(where, " AND ")
	}
	q += ` ORDER BY s.started_at DESC`
	if f.Limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", f.Limit)
	}
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []core.Session
	for rows.Next() {
		sess, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sess)
	}
	return out, rows.Err()
}

func escapeLike(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}

// ------------------------------------------------------------------- steps

// AddStep appends a step at the end of the session. The sequence number is
// computed inside the INSERT so concurrent writers never collide.
func (s *Store) AddStep(ctx context.Context, st core.Step) (core.Step, error) {
	if st.CreatedAt.IsZero() {
		st.CreatedAt = time.Now()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return st, err
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx, `
        INSERT INTO steps (session_id, seq, command, cwd, shell, exit_code, duration_ms, mark, redacted, created_at)
        SELECT ?, COALESCE(MAX(seq), 0) + 1, ?, ?, ?, ?, ?, ?, ?, ? FROM steps WHERE session_id = ?`,
		st.SessionID, st.Command, st.Cwd, st.Shell, st.ExitCode, st.Duration.Milliseconds(),
		st.Mark, boolInt(st.Redacted), ms(st.CreatedAt), st.SessionID)
	if err != nil {
		return st, err
	}
	st.ID, _ = res.LastInsertId()
	if err := tx.QueryRowContext(ctx, `SELECT seq FROM steps WHERE id = ?`, st.ID).Scan(&st.Seq); err != nil {
		return st, err
	}
	for _, t := range normalizeList(st.Techs) {
		if _, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO step_techs (step_id, tech) VALUES (?,?)`, st.ID, t); err != nil {
			return st, err
		}
	}
	return st, tx.Commit()
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// Steps returns all steps of a session in order, with techs and notes.
func (s *Store) Steps(ctx context.Context, sessionID int64) ([]core.Step, error) {
	rows, err := s.db.QueryContext(ctx, `
        SELECT id, session_id, seq, command, cwd, shell, exit_code, duration_ms, mark, redacted, created_at
        FROM steps WHERE session_id = ? ORDER BY seq`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var steps []core.Step
	index := map[int64]int{}
	for rows.Next() {
		var (
			st       core.Step
			dur      int64
			created  int64
			mark     string
			redacted int
		)
		if err := rows.Scan(&st.ID, &st.SessionID, &st.Seq, &st.Command, &st.Cwd, &st.Shell,
			&st.ExitCode, &dur, &mark, &redacted, &created); err != nil {
			return nil, err
		}
		st.Duration = time.Duration(dur) * time.Millisecond
		st.Mark = core.Mark(mark)
		st.Redacted = redacted == 1
		st.CreatedAt = fromMs(created)
		index[st.ID] = len(steps)
		steps = append(steps, st)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()

	techRows, err := s.db.QueryContext(ctx, `
        SELECT x.step_id, x.tech FROM step_techs x JOIN steps st ON st.id = x.step_id
        WHERE st.session_id = ? ORDER BY x.tech`, sessionID)
	if err != nil {
		return nil, err
	}
	for techRows.Next() {
		var id int64
		var tech string
		if err := techRows.Scan(&id, &tech); err != nil {
			techRows.Close()
			return nil, err
		}
		if i, ok := index[id]; ok {
			steps[i].Techs = append(steps[i].Techs, tech)
		}
	}
	techRows.Close()

	notes, err := s.Notes(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	for _, n := range notes {
		if n.StepID == nil {
			continue
		}
		if i, ok := index[*n.StepID]; ok {
			steps[i].Notes = append(steps[i].Notes, n)
		}
	}
	return steps, nil
}

// LastStep returns the most recent step of a session.
func (s *Store) LastStep(ctx context.Context, sessionID int64) (core.Step, error) {
	return s.stepWhere(ctx, `session_id = ? ORDER BY seq DESC LIMIT 1`, sessionID)
}

// StepBySeq returns a step by its position in the session.
func (s *Store) StepBySeq(ctx context.Context, sessionID int64, seq int) (core.Step, error) {
	return s.stepWhere(ctx, `session_id = ? AND seq = ?`, sessionID, seq)
}

func (s *Store) stepWhere(ctx context.Context, where string, args ...any) (core.Step, error) {
	var (
		st      core.Step
		dur     int64
		created int64
		mark    string
		red     int
	)
	err := s.db.QueryRowContext(ctx, `
        SELECT id, session_id, seq, command, cwd, shell, exit_code, duration_ms, mark, redacted, created_at
        FROM steps WHERE `+where, args...).
		Scan(&st.ID, &st.SessionID, &st.Seq, &st.Command, &st.Cwd, &st.Shell, &st.ExitCode, &dur, &mark, &red, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return st, fmt.Errorf("step: %w", ErrNotFound)
	}
	st.Duration = time.Duration(dur) * time.Millisecond
	st.Mark = core.Mark(mark)
	st.Redacted = red == 1
	st.CreatedAt = fromMs(created)
	return st, err
}

// SetMark sets the mark of the given steps (by seq). Returns how many changed.
func (s *Store) SetMark(ctx context.Context, sessionID int64, seqs []int, mark core.Mark) (int64, error) {
	if len(seqs) == 0 {
		return 0, nil
	}
	q, args := inClause(`UPDATE steps SET mark = ? WHERE session_id = ? AND seq IN `, seqs)
	res, err := s.db.ExecContext(ctx, q, append([]any{mark, sessionID}, args...)...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// DeleteSteps removes steps (by seq). Sequence numbers of the others are kept.
func (s *Store) DeleteSteps(ctx context.Context, sessionID int64, seqs []int) (int64, error) {
	if len(seqs) == 0 {
		return 0, nil
	}
	q, args := inClause(`DELETE FROM steps WHERE session_id = ? AND seq IN `, seqs)
	res, err := s.db.ExecContext(ctx, q, append([]any{sessionID}, args...)...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func inClause(prefix string, seqs []int) (string, []any) {
	ph := make([]string, len(seqs))
	args := make([]any, len(seqs))
	for i, v := range seqs {
		ph[i] = "?"
		args[i] = v
	}
	return prefix + "(" + strings.Join(ph, ",") + ")", args
}

// ------------------------------------------------------------------- notes

// AddNote attaches a note to a session, or to one of its steps if stepID != nil.
func (s *Store) AddNote(ctx context.Context, sessionID int64, stepID *int64, body string) (core.Note, error) {
	n := core.Note{SessionID: sessionID, StepID: stepID, Body: body, CreatedAt: time.Now()}
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO notes (session_id, step_id, body, created_at) VALUES (?,?,?,?)`,
		sessionID, stepID, body, ms(n.CreatedAt))
	if err != nil {
		return n, err
	}
	n.ID, _ = res.LastInsertId()
	return n, nil
}

// Notes returns every note of a session (session-level and step-level).
func (s *Store) Notes(ctx context.Context, sessionID int64) ([]core.Note, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, session_id, step_id, body, created_at FROM notes WHERE session_id = ? ORDER BY id`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []core.Note
	for rows.Next() {
		var (
			n       core.Note
			stepID  sql.NullInt64
			created int64
		)
		if err := rows.Scan(&n.ID, &n.SessionID, &stepID, &n.Body, &created); err != nil {
			return nil, err
		}
		if stepID.Valid {
			v := stepID.Int64
			n.StepID = &v
		}
		n.CreatedAt = fromMs(created)
		out = append(out, n)
	}
	return out, rows.Err()
}
