// +build sqlserver

package gormigrate

import (
	"os"

	"gorm.io/driver/sqlserver"
)

func init() {
	databases = append(databases, database{
		dialect: "mssql",
		driver:  sqlserver.Open(os.Getenv("SQLSERVER_CONN_STRING")),
	})
}
