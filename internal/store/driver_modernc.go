//go:build !sqlite_cgo

package store

// Default driver: pure Go, no CGO, trivial cross-compilation.
import _ "modernc.org/sqlite"

const driverName = "sqlite"
