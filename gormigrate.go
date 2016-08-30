package gormigrate

import (
	"errors"
	"fmt"

	"github.com/jinzhu/gorm"
)

type MigrateFunc func(*gorm.DB) error
type RollbackFunc func(*gorm.DB) error

type Options struct {
	TableName      string
	IDColumnName   string
	UseTransaction bool
}

type Migration struct {
	ID       string
	Migrate  MigrateFunc
	Rollback RollbackFunc
}

type Gormigrate struct {
	db         *gorm.DB
	options    *Options
	migrations []*Migration
}

var (
	DefaultOptions = &Options{
		TableName:    "migration",
		IDColumnName: "id",
	}

	ErrRollbackImpossible = errors.New("It's impossible to rollback this migration")
	ErrNoMigrationDefined = errors.New("No migration defined")
)

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
