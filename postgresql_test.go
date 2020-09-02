// +build postgresql

package gormigrate

import (
	"gorm.io/driver/postgres"
	"os"
)

func init() {
	databases = append(databases, database{
		dialect: "postgres",
		driver:  postgres.Open(os.Getenv("PG_CONN_STRING")),
	})
}
