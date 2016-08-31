// Gormigrate is a migration helper for Gorm (http://jinzhu.me/gorm/).
// Gorm already have useful migrate functions
// (http://jinzhu.me/gorm/database.html#migration), just misses
// proper schema versioning and rollback cababilities.
//
// Example:
//
//     package main
//
//     import (
// 	       "log"
//
// 	       "github.com/go-gormigrate/gormigrate"
// 	       "github.com/jinzhu/gorm"
// 	       _ "github.com/jinzhu/gorm/dialects/sqlite"
//     )
//
//     type Person struct {
// 	        gorm.Model
//          Name string
//     }
//
//     type Pet struct {
// 	        gorm.Model
// 	        Name     string
// 	        PersonID int
//     }
//
//     func main() {
// 	        db, err := gorm.Open("sqlite3", "mydb.sqlite3")
// 	        if err != nil {
// 		        log.Fatal(err)
// 	        }
// 	        if err = db.DB().Ping(); err != nil {
// 	    	    log.Fatal(err)
//         	}
//
// 	        db.LogMode(true)
//
//         	m := gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{
// 	        	{
// 	        		ID: "201608301400",
// 	        		Migrate: func(tx *gorm.DB) error {
// 			        	return tx.AutoMigrate(&Person{}).Error
//         			},
// 		        	Rollback: func(tx *gorm.DB) error {
// 				        return tx.DropTable("people").Error
// 			        },
//         		},
// 	        	{
// 		        	ID: "201608301430",
// 		        	Migrate: func(tx *gorm.DB) error {
// 			        	return tx.AutoMigrate(&Pet{}).Error
// 		        	},
// 			        Rollback: func(tx *gorm.DB) error {
// 				        return tx.DropTable("pets").Error
// 		        	},
// 	        	},
// 	        })
//
// 	        err = m.Migrate()
//         	if err == nil {
//         		log.Printf("Migration did run successfully")
//         	} else {
// 	        	log.Printf("Could not migrate: %v", err)
//         	}
//     }
package gormigrate

import (
	"errors"
	"fmt"

	"github.com/jinzhu/gorm"
)

type MigrateFunc func(*gorm.DB) error
type RollbackFunc func(*gorm.DB) error

// Options define options for all migrations.
type Options struct {
	// TableName is the migration table.
	TableName string
	// IDColumnName is the name of column where the migration id will be stored.
	IDColumnName string
	// UseTransaction makes Gormigrate execute migrations inside a single transaction.
	// Keep in mind that not all databases support DDL commands inside transactions.
	UseTransaction bool
}

// Migrate represents a database migration (a modification to be made on the database).
type Migration struct {
	// ID is the migration identifier. Usually a timestamp like "201601021504".
	ID string
	// Migrate is a function that will br executed while running this migration.
	Migrate MigrateFunc
	// Rollback will be executed on rollback. Can be nil.
	Rollback RollbackFunc
}

type Gormigrate struct {
	db         *gorm.DB
	options    *Options
	migrations []*Migration
}

var (
	// DefaultOptions can be used if you don't want to think about options.
	DefaultOptions = &Options{
		TableName:      "migrations",
		IDColumnName:   "id",
		UseTransaction: false,
	}

	ErrRollbackImpossible = errors.New("It's impossible to rollback this migration")
	ErrNoMigrationDefined = errors.New("No migration defined")
)

// New returns a new Gormigrate.
func New(db *gorm.DB, options *Options, migrations []*Migration) *Gormigrate {
	return &Gormigrate{
		db:         db,
		options:    options,
		migrations: migrations,
	}
}

func (g *Gormigrate) migrationDidRun(m *Migration) bool {
	var count int
	g.db.
		Table(g.options.TableName).
		Where(fmt.Sprintf("%s = ?", g.options.IDColumnName), m.ID).
		Count(&count)
	return count > 0
}

func (g *Gormigrate) createMigrationTableIfNotExists() error {
	if g.db.HasTable(g.options.TableName) {
		return nil
	}

	sql := fmt.Sprintf("CREATE TABLE %s (%s VARCHAR(255) PRIMARY KEY)", g.options.TableName, g.options.IDColumnName)
	err := g.db.Exec(sql).Error
	if err != nil {
		return err
	}
	return nil
}

// Migrate executes all migrations that did not run yet.
func (g *Gormigrate) Migrate() error {
	if err := g.db.DB().Ping(); err != nil {
		return err
	}
	if err := g.createMigrationTableIfNotExists(); err != nil {
		return err
	}

	var tx *gorm.DB
	if g.options.UseTransaction {
		tx = g.db.Begin()
	} else {
		tx = g.db
	}

	for _, migration := range g.migrations {
		if g.migrationDidRun(migration) {
			continue
		}

		if err := migration.Migrate(tx); err != nil {
			tx.Rollback()
			return err
		}
		sql := fmt.Sprintf("INSERT INTO %s (%s) VALUES (?)", g.options.TableName, g.options.IDColumnName)
		if err := g.db.Exec(sql, migration.ID).Error; err != nil {
			tx.Rollback()
			return err
		}
	}
	if g.options.UseTransaction {
		if err := tx.Commit().Error; err != nil {
			return err
		}
	}
	return nil
}

// RollbackMigration undo a migration.
func (g *Gormigrate) RollbackMigration(m *Migration) error {
	if m.Rollback == nil {
		return ErrRollbackImpossible
	}

	var tx *gorm.DB
	if g.options.UseTransaction {
		tx = g.db.Begin()
	} else {
		tx = g.db
	}

	if err := m.Rollback(tx); err != nil {
		return err
	}
	sql := fmt.Sprintf("DELETE FROM %s WHERE %s = ?", g.options.TableName, g.options.IDColumnName)
	if err := g.db.Exec(sql, m.ID).Error; err != nil {
		tx.Rollback()
		return err
	}

	if g.options.UseTransaction {
		if err := tx.Commit().Error; err != nil {
			return err
		}
	}
	return nil
}

// RollbackLast undo the last migration
func (g *Gormigrate) RollbackLast() error {
	if len(g.migrations) == 0 {
		return ErrNoMigrationDefined
	}

	lastMigration := g.migrations[len(g.migrations)-1]
	if err := g.RollbackMigration(lastMigration); err != nil {
		return err
	}
	return nil
}
