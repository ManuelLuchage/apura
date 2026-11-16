//go:build sqlite_cgo

package store

// Alternative driver (go build -tags sqlite_cgo): CGO-based mattn/go-sqlite3.
// Useful where modernc is unavailable or for benchmarking.
import _ "github.com/mattn/go-sqlite3"

const driverName = "sqlite3"
