package store

// migrations are applied in order; the index+1 is stored in PRAGMA user_version.
// Never edit a released migration: append a new one.
var migrations = []string{
	// 1: initial schema
	`
CREATE TABLE sessions (
    id         INTEGER PRIMARY KEY,
    name       TEXT    NOT NULL UNIQUE,
    goal       TEXT    NOT NULL DEFAULT '',
    state      TEXT    NOT NULL DEFAULT 'active',
    result     TEXT    NOT NULL DEFAULT '',
    shell      TEXT    NOT NULL DEFAULT '',
    host       TEXT    NOT NULL DEFAULT '',
    started_at INTEGER NOT NULL,
    ended_at   INTEGER
);

CREATE TABLE session_tags (
    session_id INTEGER NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    tag        TEXT    NOT NULL,
    PRIMARY KEY (session_id, tag)
);
CREATE INDEX idx_session_tags_tag ON session_tags(tag);

CREATE TABLE steps (
    id          INTEGER PRIMARY KEY,
    session_id  INTEGER NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    seq         INTEGER NOT NULL,
    command     TEXT    NOT NULL,
    cwd         TEXT    NOT NULL DEFAULT '',
    shell       TEXT    NOT NULL DEFAULT '',
    exit_code   INTEGER NOT NULL DEFAULT 0,
    duration_ms INTEGER NOT NULL DEFAULT 0,
    mark        TEXT    NOT NULL DEFAULT '',
    redacted    INTEGER NOT NULL DEFAULT 0,
    created_at  INTEGER NOT NULL,
    UNIQUE (session_id, seq)
);

CREATE TABLE step_techs (
    step_id INTEGER NOT NULL REFERENCES steps(id) ON DELETE CASCADE,
    tech    TEXT    NOT NULL,
    PRIMARY KEY (step_id, tech)
);
CREATE INDEX idx_step_techs_tech ON step_techs(tech);

CREATE TABLE notes (
    id         INTEGER PRIMARY KEY,
    session_id INTEGER NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    step_id    INTEGER REFERENCES steps(id) ON DELETE CASCADE,
    body       TEXT    NOT NULL,
    created_at INTEGER NOT NULL
);
CREATE INDEX idx_notes_session ON notes(session_id);
`,
}
