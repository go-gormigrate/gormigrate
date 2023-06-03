//go:build sqlite

package gormigrate_test

import (
	"os"

	"gorm.io/driver/sqlite"
)

func init() {
	dialects = append(dialects, dialect{
		name:              "sqlite",
		driver:            sqlite.Open(os.Getenv("SQLITE_DSN")),
		supportsAtomicDDL: true,
	})
}
