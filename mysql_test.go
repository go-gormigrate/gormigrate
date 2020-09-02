// +build mysql

package gormigrate

import (
	"gorm.io/driver/mysql"
	"os"
)

func init() {
	databases = append(databases, database{
		dialect: "mysql",
		driver:  mysql.Open(os.Getenv("MYSQL_CONN_STRING")),
	})
}
