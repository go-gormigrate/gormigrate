package gormigrate

import (
	"errors"
	"fmt"
	"gorm.io/gorm/clause"
	"reflect"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	initSchemaMigrationID = "SCHEMA_INIT"
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
	// IDColumnSize is the length of the migration id column
	IDColumnSize int
	// UseTransaction makes Gormigrate execute migrations inside a single transaction.
	// Keep in mind that not all databases support DDL commands inside transactions.
	UseTransaction bool
	// ValidateUnknownMigrations will cause migrate to fail if there's unknown migration
	// IDs in the database
	ValidateUnknownMigrations bool
	// DependencyColumnName is the name of column where stores dependency id
	DependencyColumnName string
}

// Migration represents a database migration (a modification to be made on the database).
type Migration struct {
	ID        uint `gorm:"primarykey"`
	CreatedAt time.Time
	UpdatedAt time.Time

	// MigrationID is the migration identifier. Usually a timestamp like "201601021504".
	MigrationID string
	// Migrate is a function that will br executed while running this migration.
	Migrate MigrateFunc `gorm:"-"`
	// Rollback will be executed on rollback. Can be nil.
	Rollback RollbackFunc `gorm:"-"`
	// Dependencies indicates which migrations should be run before
	Dependencies []*Migration `gorm:"foreignKey:DependencyID;references:MigrationID"`
	// DependencyID is the association id
	DependencyID *string
}

// Deprecated: do not use it
func (m *Migration) WithOptions(options *Options) interface{} {
	return m
	// Todo: consider json tag

	t := reflect.TypeOf(Migration{})
	var fieldMigrationID, fieldDependencies reflect.StructField
	if f, ok := t.FieldByName("MigrationID"); ok {
		fieldMigrationID = f
		s := fieldMigrationID.Tag.Get("gorm")
		s += fmt.Sprintf(",column:%s,size:%v", options.IDColumnName, options.IDColumnSize)
		fieldMigrationID.Tag = reflect.StructTag(fmt.Sprintf("gorm:%s", strconv.Quote(s)))
	}

	if f, ok := t.FieldByName("Dependencies"); ok {
		fieldDependencies = f
		s := fieldDependencies.Tag.Get("gorm")
		s += fmt.Sprintf(",column:%s,size:%v", options.DependencyColumnName, options.IDColumnSize)
		fieldDependencies.Tag = reflect.StructTag(fmt.Sprintf("gorm:%s", strconv.Quote(s)))

	}

	// stores table name implicitly on first field so it can be read easily
	field0 := t.Field(0)
	tbName := string(field0.Tag)
	tbName += " tname:" + strings.TrimSpace(options.TableName)
	field0.Tag = reflect.StructTag(tbName)

	fields := []reflect.StructField{field0}

	for i := 1; i < t.NumField(); i++ {
		if i == fieldMigrationID.Index[0] {
			fields = append(fields, fieldMigrationID)
		} else if i == fieldDependencies.Index[0] {
			fields = append(fields, fieldDependencies)
		} else {
			fields = append(fields, t.Field(i))
		}
	}

	v := reflect.New(reflect.StructOf(fields)).Elem()
	for i := 0; i < len(fields); i++ {
		v.Field(i).Set(reflect.ValueOf(*m).Field(i))
	}
	return (&v).Interface()
}

// Gormigrate represents a collection of all migrations of a database schema.
type Gormigrate struct {
	db         *gorm.DB
	tx         *gorm.DB
	options    *Options
	migrations []*Migration
	initSchema InitSchemaFunc
}

// ReservedIDError is returned when a migration is using a reserved ID
type ReservedIDError struct {
	ID string
}

func (e *ReservedIDError) Error() string {
	return fmt.Sprintf(`gormigrate: Reserved migration MigrationID: "%s"`, e.ID)
}

// DuplicatedIDError is returned when more than one migration have the same ID
type DuplicatedIDError struct {
	ID string
}

func (e *DuplicatedIDError) Error() string {
	return fmt.Sprintf(`gormigrate: Duplicated migration MigrationID: "%s"`, e.ID)
}

func DummyMigration(id string) *Migration {
	return &Migration{MigrationID: id, Migrate: dummyMigration}
}

var (
	dummyMigration = func(db *gorm.DB) error {
		return nil
	}
)

var (
	// DefaultOptions can be used if you don't want to think about options.
	DefaultOptions = &Options{
		TableName:                 "migrations",
		IDColumnName:              "migration_id",
		IDColumnSize:              255,
		UseTransaction:            false,
		ValidateUnknownMigrations: false,
		DependencyColumnName:      "dependency_id",
	}

	// ErrRollbackImpossible is returned when trying to rollback a migration
	// that has no rollback function.
	ErrRollbackImpossible = errors.New("gormigrate: It's impossible to rollback this migration")

	// ErrNoMigrationDefined is returned when no migration is defined.
	ErrNoMigrationDefined = errors.New("gormigrate: No migration defined")

	// ErrMissingID is returned when the MigrationID od migration is equal to ""
	ErrMissingID = errors.New("gormigrate: Missing MigrationID in migration")

	// ErrNoRunMigration is returned when any run migration was found while
	// running RollbackLast
	ErrNoRunMigration = errors.New("gormigrate: Could not find last run migration")

	// ErrMigrationIDDoesNotExist is returned when migrating or rolling back to a migration ID that
	// does not exist in the list of migrations
	ErrMigrationIDDoesNotExist = errors.New("gormigrate: Tried to migrate to an MigrationID that doesn't exist")

	// ErrUnknownPastMigration is returned if a migration exists in the DB that doesn't exist in the code
	ErrUnknownPastMigration = errors.New("gormigrate: Found migration in DB that does not exist in code")
)

// New returns a new Gormigrate.
func New(db *gorm.DB, options *Options, migrations []*Migration) *Gormigrate {
	if options.TableName == "" {
		options.TableName = DefaultOptions.TableName
	}
	if options.IDColumnName == "" {
		options.IDColumnName = DefaultOptions.IDColumnName
	}
	if options.IDColumnSize == 0 {
		options.IDColumnSize = DefaultOptions.IDColumnSize
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
	if !g.hasMigrations() {
		return ErrNoMigrationDefined
	}

	if err := g.checkReservedID(); err != nil {
		return err
	}

	if err := g.checkDuplicatedID(); err != nil {
		return err
	}

	g.begin()
	defer g.rollback()

	if err := g.createMigrationTableIfNotExists(); err != nil {
		return err
	}

	if g.options.ValidateUnknownMigrations {
		unknownMigrations, err := g.unknownMigrationsHaveHappened()
		if err != nil {
			return err
		}
		if unknownMigrations {
			return ErrUnknownPastMigration
		}
	}

	if g.initSchema != nil {
		canInitializeSchema, err := g.canInitializeSchema()
		if err != nil {
			return err
		}
		if canInitializeSchema {
			if err := g.runInitSchema(); err != nil {
				return err
			}
			return g.commit()
		}
	}

	g.resolveDependency()
	var targetMigrationID string
	if len(g.migrations) > 0 {
		targetMigrationID = g.migrations[len(g.migrations)-1].MigrationID
	}
	return g.migrate(targetMigrationID)
}

// MigrateTo executes all migrations that did not run yet up to the migration that matches `migrationID`.
func (g *Gormigrate) MigrateTo(migrationID string) error {
	if err := g.checkIDExist(migrationID); err != nil {
		return err
	}
	return g.migrate(migrationID)
}

func (g *Gormigrate) migrate(migrationID string) error {
	for _, migration := range g.migrations {
		if err := g.runMigration(migration); err != nil {
			return err
		}
		if migrationID != "" && migration.MigrationID == migrationID {
			break
		}
	}
	return g.commit()
}

// There are migrations to apply if either there's a defined
// initSchema function or if the list of migrations is not empty.
func (g *Gormigrate) hasMigrations() bool {
	return g.initSchema != nil || len(g.migrations) > 0
}

// Check whether any migration is using a reserved ID.
// For now there's only have one reserved ID, but there may be more in the future.
func (g *Gormigrate) checkReservedID() error {
	for _, m := range g.migrations {
		if m.MigrationID == initSchemaMigrationID {
			return &ReservedIDError{ID: m.MigrationID}
		}
	}
	return nil
}

func (g *Gormigrate) checkDuplicatedID() error {
	lookup := make(map[string]struct{}, len(g.migrations))
	for _, m := range g.migrations {
		if _, ok := lookup[m.MigrationID]; ok {
			return &DuplicatedIDError{ID: m.MigrationID}
		}
		lookup[m.MigrationID] = struct{}{}
	}
	return nil
}

func (g *Gormigrate) checkIDExist(migrationID string) error {
	for _, migrate := range g.migrations {
		if migrate.MigrationID == migrationID {
			return nil
		}
	}
	return ErrMigrationIDDoesNotExist
}

// RollbackLast undo the last migration
func (g *Gormigrate) RollbackLast() error {
	if len(g.migrations) == 0 {
		return ErrNoMigrationDefined
	}

	g.begin()
	defer g.rollback()

	lastRunMigration, err := g.getLastRunMigration()
	if err != nil {
		return err
	}

	if err := g.rollbackMigration(lastRunMigration); err != nil {
		return err
	}
	return g.commit()
}

// RollbackTo undoes migrations up to the given migration that matches the `migrationID`.
// Migration with the matching `migrationID` is not rolled back.
func (g *Gormigrate) RollbackTo(migrationID string) error {
	if len(g.migrations) == 0 {
		return ErrNoMigrationDefined
	}

	if err := g.checkIDExist(migrationID); err != nil {
		return err
	}

	g.begin()
	defer g.rollback()

	for i := len(g.migrations) - 1; i >= 0; i-- {
		migration := g.migrations[i]
		if migration.MigrationID == migrationID {
			break
		}
		migrationRan, err := g.migrationRan(migration)
		if err != nil {
			return err
		}
		if migrationRan {
			if err := g.rollbackMigration(migration); err != nil {
				return err
			}
		}
	}
	return g.commit()
}

func (g *Gormigrate) getLastRunMigration() (*Migration, error) {
	for i := len(g.migrations) - 1; i >= 0; i-- {
		migration := g.migrations[i]

		migrationRan, err := g.migrationRan(migration)
		if err != nil {
			return nil, err
		}

		if migrationRan {
			return migration, nil
		}
	}
	return nil, ErrNoRunMigration
}

// RollbackMigration undo a migration.
func (g *Gormigrate) RollbackMigration(m *Migration) error {
	g.begin()
	defer g.rollback()

	if err := g.rollbackMigration(m); err != nil {
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
	return g.tx.Exec(sql, m.MigrationID).Error
}

func (g *Gormigrate) runInitSchema() error {
	if err := g.initSchema(g.tx); err != nil {
		return err
	}
	if err := g.insertMigration(initSchemaMigrationID); err != nil {
		return err
	}

	for _, migration := range g.migrations {
		if err := g.insertMigrationWithDependency(migration); err != nil {
			return err
		}
	}

	return nil
}

func (g *Gormigrate) runMigration(migration *Migration) error {
	if len(migration.MigrationID) == 0 {
		return ErrMissingID
	}

	migrationRan, err := g.migrationRan(migration)
	if err != nil {
		return err
	}
	if !migrationRan {
		if err := migration.Migrate(g.tx); err != nil {
			return err
		}

		if err := g.insertMigrationWithDependency(migration); err != nil {
			return err
		}
	}
	return nil
}

func (g *Gormigrate) createMigrationTableIfNotExists() (err error) {
	g.tx.DisableForeignKeyConstraintWhenMigrating = true
	if g.tx.Migrator().HasTable(DefaultOptions.TableName) {
		if g.options.TableName != DefaultOptions.TableName {
			err = g.tx.Migrator().RenameTable(DefaultOptions.TableName, g.options.TableName)
		}
	} else {
		err = g.tx.Table(g.options.TableName).Migrator().CreateTable(Migration{})
	}
	return err
}

func (g *Gormigrate) migrationRan(m *Migration) (bool, error) {
	var count int64
	err := g.tx.
		Table(g.options.TableName).
		Where(fmt.Sprintf("%s = ?", g.options.IDColumnName), m.MigrationID).
		Count(&count).
		Error
	// ignore dependency since it has been stored
	return count > 0, err
}

// The schema can be initialised only if it hasn't been initialised yet
// and no other migration has been applied already.
func (g *Gormigrate) canInitializeSchema() (bool, error) {
	migrationRan, err := g.migrationRan(&Migration{MigrationID: initSchemaMigrationID})
	if err != nil {
		return false, err
	}
	if migrationRan {
		return false, nil
	}

	// If the MigrationID doesn't exist, we also want the list of migrations to be empty
	var count int64
	err = g.tx.
		Table(g.options.TableName).
		Count(&count).
		Error
	return count == 0, err
}

func (g *Gormigrate) unknownMigrationsHaveHappened() (bool, error) {
	sql := fmt.Sprintf("SELECT %s FROM %s", g.options.IDColumnName, g.options.TableName)
	rows, err := g.tx.Raw(sql).Rows()
	if err != nil {
		return false, err
	}
	defer rows.Close()

	validIDSet := make(map[string]struct{}, len(g.migrations)+1)
	validIDSet[initSchemaMigrationID] = struct{}{}
	for _, migration := range g.migrations {
		validIDSet[migration.MigrationID] = struct{}{}
	}

	for rows.Next() {
		var pastMigrationID string
		if err := rows.Scan(&pastMigrationID); err != nil {
			return false, err
		}
		if _, ok := validIDSet[pastMigrationID]; !ok {
			return true, nil
		}
	}

	return false, nil
}

func (g *Gormigrate) insertMigration(id string) error {
	sql := fmt.Sprintf("INSERT INTO %s (%s) VALUES (?)", g.options.TableName, g.options.IDColumnName)
	return g.tx.Exec(sql, id).Error
}

func (g *Gormigrate) insertMigrationWithDependency(m *Migration) error {
	return g.tx.Table(g.options.TableName).Model(m).
		Clauses(clause.OnConflict{
			UpdateAll: true,
		}).
		Create(m).Error
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
		return g.tx.Commit().Error
	}
	return nil
}

func (g *Gormigrate) rollback() {
	if g.options.UseTransaction {
		g.tx.Rollback()
	}
}
