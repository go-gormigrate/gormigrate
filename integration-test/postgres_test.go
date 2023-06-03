//go:build postgres

package gormigrate_test

import (
	"os"

	"gorm.io/driver/postgres"
)

func init() {
	dialects = append(dialects, dialect{
		name:              "postgres",
		driver:            postgres.Open(os.Getenv("POSTGRES_DSN")),
		supportsAtomicDDL: true,
	})
}
