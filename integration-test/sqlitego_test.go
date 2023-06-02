//go:build sqlitego

package gormigrate_test

import (
	"os"

	"github.com/glebarez/sqlite"
)

func init() {
	databases = append(databases, database{
		dialect: "sqlitego",
		driver:  sqlite.Open(os.Getenv("SQLITE_DSN")),
	})
}
