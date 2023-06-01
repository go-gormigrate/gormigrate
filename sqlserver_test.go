//go:build sqlserver

package gormigrate_test

import (
	"os"

	"gorm.io/driver/sqlserver"
)

func init() {
	databases = append(databases, database{
		dialect: "sqlserver",
		driver:  sqlserver.Open(os.Getenv("SQLSERVER_DSN")),
	})
}
