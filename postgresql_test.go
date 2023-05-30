//go:build postgresql

package gormigrate_test

import (
	"os"

	"gorm.io/driver/postgres"
)

func init() {
	databases = append(databases, database{
		dialect: "postgres",
		driver:  postgres.Open(os.Getenv("PG_CONN_STRING")),
	})
}
