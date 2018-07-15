// +build sqlserver

package gormigrate

import (
	_ "github.com/jinzhu/gorm/dialects/mssql"
)

func init() {
	databases = append(databases, database{
		name:    "mssql",
		connEnv: "SQLSERVER_CONN_STRING",
	})
}
