//go:build sqlite

package gormigrate_test

import (
	"os"

	"gorm.io/driver/sqlite"
)

func init() {
	databases = append(databases, database{
		dialect: "sqlite",
		driver:  sqlite.Open(os.Getenv("SQLITE_DSN")),
	})
}
