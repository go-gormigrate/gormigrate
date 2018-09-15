package gormigrate

import (
	"errors"
	"fmt"
	"sort"

	"github.com/jinzhu/gorm"
)

// MigrateFunc is the func signature for migrating.
type MigrateFunc func(*gorm.DB) error

// RollbackFunc is the func signature for rollbacking.
type RollbackFunc func(*gorm.DB) error

// InitSchemaFunc is the func signature for initializing the schema.
type InitSchemaFunc func(*gorm.DB) error

// Options define options for all migrations.
type Options struct {
	// TableName is the migration table.
	TableName string
	// IDColumnName is the name of column where the migration id will be stored.
	IDColumnName string
	// UseTransaction makes Gormigrate execute migrations inside a single transaction.
	// Keep in mind that not all databases support DDL commands inside transactions.
	UseTransaction bool
	// SortMigration is the flag is responsible for the sorting of migrations on the ID,
	// the migrations is sorted before running
	SortMigration bool
}

// Migration represents a database migration (a modification to be made on the database).
type Migration struct {
	// ID is the migration identifier. Usually a timestamp like "201601021504".
	ID uint64
	// Migrate is a function that will br executed while running this migration.
	Migrate MigrateFunc
	// Rollback will be executed on rollback. Can be nil.
	Rollback RollbackFunc
}

// Gormigrate represents a collection of all migrations of a database schema.
type Gormigrate struct {
	db         *gorm.DB
	tx         *gorm.DB
	options    *Options
	migrations []*Migration
	initSchema InitSchemaFunc
}

// DuplicatedIDError is returned when more than one migration have the same ID
type DuplicatedIDError struct {
	ID uint64
}

func (e *DuplicatedIDError) Error() string {
	return fmt.Sprintf(`gormigrate: Duplicated migration ID: "%d"`, e.ID)
}

var (
	// DefaultOptions can be used if you don't want to think about options.
	DefaultOptions = &Options{
		TableName:      "migrations",
		IDColumnName:   "id",
		UseTransaction: false,
		SortMigration:  false,
	}

	// ErrRollbackImpossible is returned when trying to rollback a migration
	// that has no rollback function.
	ErrRollbackImpossible = errors.New("gormigrate: It's impossible to rollback this migration")

	// ErrNoMigrationDefined is returned when no migration is defined.
	ErrNoMigrationDefined = errors.New("gormigrate: No migration defined")

	// ErrMissingID is returned when the ID od migration is equal to ""
	ErrMissingID = errors.New("gormigrate: Missing ID in migration")

	// ErrNoRunnedMigration is returned when any runned migration was found while
	// running RollbackLast
	ErrNoRunnedMigration = errors.New("gormigrate: Could not find last runned migration")
)

// New returns a new Gormigrate.
func New(db *gorm.DB, options *Options, migrations []*Migration) *Gormigrate {
	if options.TableName == "" {
		options.TableName = DefaultOptions.TableName
	}
	if options.IDColumnName == "" {
		options.IDColumnName = DefaultOptions.IDColumnName
	}
	if options.SortMigration {
		sort.Slice(migrations, func(i, j int) bool {
			return migrations[i].ID < migrations[j].ID
		})
	}

	return &Gormigrate{
		db:         db,
		options:    options,
		migrations: migrations,
	}
}

// InitSchema sets a function that is run if no migration is found.
// The idea is preventing to run all migrations when a new clean database
// is being migrating. In this function you should create all tables and
// foreign key necessary to your application.
func (g *Gormigrate) InitSchema(initSchema InitSchemaFunc) {
	g.initSchema = initSchema
}

// Migrate executes all migrations that did not run yet.
func (g *Gormigrate) Migrate() error {
	return g.migrate(0)
}

// MigrateTo executes all migrations that did not run yet up to the migration that matches `migrationID`.
func (g *Gormigrate) MigrateTo(migrationID uint64) error {
	return g.migrate(migrationID)
}

func (g *Gormigrate) migrate(migrationID uint64) error {
	if len(g.migrations) == 0 {
		return ErrNoMigrationDefined
	}

	if err := g.checkDuplicatedID(); err != nil {
		return err
	}

	if err := g.createMigrationTableIfNotExists(); err != nil {
		return err
	}

	g.begin()

	if g.initSchema != nil && g.isFirstRun() {
		if err := g.runInitSchema(); err != nil {
			g.rollback()
			return err
		}
		return g.commit()
	}

	for _, migration := range g.migrations {
		if err := g.runMigration(migration); err != nil {
			g.rollback()
			return err
		}
		if migrationID != 0 && migration.ID == migrationID {
			break
		}
	}

	return g.commit()
}

func (g *Gormigrate) checkDuplicatedID() error {
	lookup := make(map[uint64]struct{}, len(g.migrations))
	for _, m := range g.migrations {
		if _, ok := lookup[m.ID]; ok {
			return &DuplicatedIDError{ID: m.ID}
		}
		lookup[m.ID] = struct{}{}
	}
	return nil
}

// RollbackLast undo the last migration
func (g *Gormigrate) RollbackLast() error {
	if len(g.migrations) == 0 {
		return ErrNoMigrationDefined
	}

	lastRunnedMigration, err := g.getLastRunnedMigration()
	if err != nil {
		return err
	}

	if err := g.RollbackMigration(lastRunnedMigration); err != nil {
		return err
	}
	return nil
}

// RollbackTo undoes migrations up to the given migration that matches the `migrationID`.
// Migration with the matching `migrationID` is not rolled back.
func (g *Gormigrate) RollbackTo(migrationID uint64) error {
	if len(g.migrations) == 0 {
		return ErrNoMigrationDefined
	}

	g.begin()

	for i := len(g.migrations) - 1; i >= 0; i-- {
		migration := g.migrations[i]
		if migration.ID == migrationID {
			break
		}
		if g.migrationDidRun(migration) {
			if err := g.rollbackMigration(migration); err != nil {
				g.rollback()
				return err
			}
		}
	}

	return g.commit()
}

func (g *Gormigrate) getLastRunnedMigration() (*Migration, error) {
	for i := len(g.migrations) - 1; i >= 0; i-- {
		migration := g.migrations[i]
		if g.migrationDidRun(migration) {
			return migration, nil
		}
	}
	return nil, ErrNoRunnedMigration
}

// RollbackMigration undo a migration.
func (g *Gormigrate) RollbackMigration(m *Migration) error {
	g.begin()
	if err := g.rollbackMigration(m); err != nil {
		g.rollback()
		return err
	}
	return g.commit()
}

func (g *Gormigrate) rollbackMigration(m *Migration) error {
	if m.Rollback == nil {
		return ErrRollbackImpossible
	}

	if err := m.Rollback(g.tx); err != nil {
		return err
	}

	sql := fmt.Sprintf("DELETE FROM %s WHERE %s = ?", g.options.TableName, g.options.IDColumnName)
	if err := g.db.Exec(sql, m.ID).Error; err != nil {
		return err
	}
	return nil
}

func (g *Gormigrate) runInitSchema() error {
	if err := g.initSchema(g.tx); err != nil {
		return err
	}

	for _, migration := range g.migrations {
		if err := g.insertMigration(migration.ID); err != nil {
			return err
		}
	}

	return nil
}

func (g *Gormigrate) runMigration(migration *Migration) error {
	if migration.ID == 0 {
		return ErrMissingID
	}

	if !g.migrationDidRun(migration) {
		if err := migration.Migrate(g.tx); err != nil {
			return err
		}

		if err := g.insertMigration(migration.ID); err != nil {
			return err
		}
	}
	return nil
}

func (g *Gormigrate) createMigrationTableIfNotExists() error {
	if g.db.HasTable(g.options.TableName) {
		return nil
	}

	sql := fmt.Sprintf("CREATE TABLE %s (%s BIGINT PRIMARY KEY)", g.options.TableName, g.options.IDColumnName)
	if err := g.db.Exec(sql).Error; err != nil {
		return err
	}
	return nil
}

func (g *Gormigrate) migrationDidRun(m *Migration) bool {
	var count int
	g.db.
		Table(g.options.TableName).
		Where(fmt.Sprintf("%s = ?", g.options.IDColumnName), m.ID).
		Count(&count)
	return count > 0
}

func (g *Gormigrate) isFirstRun() bool {
	var count int
	g.db.
		Table(g.options.TableName).
		Count(&count)
	return count == 0
}

func (g *Gormigrate) insertMigration(id uint64) error {
	sql := fmt.Sprintf("INSERT INTO %s (%s) VALUES (?)", g.options.TableName, g.options.IDColumnName)
	return g.tx.Exec(sql, id).Error
}

func (g *Gormigrate) begin() {
	if g.options.UseTransaction {
		g.tx = g.db.Begin()
	} else {
		g.tx = g.db
	}
}

func (g *Gormigrate) commit() error {
	if g.options.UseTransaction {
		if err := g.tx.Commit().Error; err != nil {
			return err
		}
	}
	return nil
}

func (g *Gormigrate) rollback() {
	if g.options.UseTransaction {
		g.tx.Rollback()
	}
}
