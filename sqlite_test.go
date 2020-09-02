// +build sqlite

package gormigrate

import (
	"gorm.io/driver/sqlite"
	"os"
)

func init() {
	databases = append(databases, database{
		dialect: "sqlite3",
		driver:  sqlite.Open(os.Getenv("SQLITE_CONN_STRING")),
	})
}
