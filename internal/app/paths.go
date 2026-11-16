// Package app holds process-level configuration (paths, environment).
package app

import (
	"os"
	"path/filepath"

	"github.com/ManuelLuchage/apura/internal/core"
)

// DBPath returns the database location: $APURA_DB, else
// $XDG_DATA_HOME/apura/apura.db, else ~/.local/share/apura/apura.db.
func DBPath() (string, error) {
	if p := os.Getenv(core.EnvDB); p != "" {
		return p, nil
	}
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, "apura", "apura.db"), nil
}
