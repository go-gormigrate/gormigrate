// +build postgresql

package gormigrate

import (
	_ "github.com/jinzhu/gorm/dialects/postgres"
)

func init() {
	databases = append(databases, database{
		name:    "postgres",
		connEnv: "PG_CONN_STRING",
	})
}
