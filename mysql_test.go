// +build mysql

package gormigrate

import (
	_ "github.com/jinzhu/gorm/dialects/mysql"
)

func init() {
	databases = append(databases, database{
		name:    "mysql",
		connEnv: "MYSQL_CONN_STRING",
	})
}
