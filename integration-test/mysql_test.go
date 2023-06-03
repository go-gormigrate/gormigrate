//go:build mysql

package gormigrate_test

import (
	"os"

	"gorm.io/driver/mysql"
)

func init() {
	dialects = append(dialects, dialect{
		name:   "mysql",
		driver: mysql.Open(os.Getenv("MYSQL_DSN")),
		// mysql/mariadb causes implicit commits in transactional DDL statements, see for details:
		//   https://mariadb.com/kb/en/sql-statements-that-cause-an-implicit-commit
		//   https://dev.mysql.com/doc/refman/8.0/en/atomic-ddl.html
		supportsAtomicDDL: false,
	})
}
