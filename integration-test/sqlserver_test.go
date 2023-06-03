//go:build sqlserver

package gormigrate_test

import (
	"os"

	"gorm.io/driver/sqlserver"
)

func init() {
	dialects = append(dialects, dialect{
		name:              "sqlserver",
		driver:            sqlserver.Open(os.Getenv("SQLSERVER_DSN")),
		supportsAtomicDDL: true,
	})
}
