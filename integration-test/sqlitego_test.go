//go:build sqlitego

package gormigrate_test

import (
	"os"

	"github.com/glebarez/sqlite"
)

func init() {
	dialects = append(dialects, dialect{
		name:              "sqlitego",
		driver:            sqlite.Open(os.Getenv("SQLITE_DSN")),
		supportsAtomicDDL: true,
	})
}
