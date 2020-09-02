// +build sqlserver

package gormigrate

import (
	"gorm.io/driver/sqlserver"
	"os"
)

func init() {
	databases = append(databases, database{
		dialect: "mssql",
		driver:  sqlserver.Open(os.Getenv("SQLSERVER_CONN_STRING")),
	})
}
