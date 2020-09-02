package gormigrate

import (
	"errors"
	"fmt"
	"testing"

	_ "github.com/joho/godotenv/autoload"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var databases []database

type database struct {
	dialect string
	driver  gorm.Dialector
}

var migrations = []*Migration{
	{
		ID: "201608301400",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&Person{})
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.Migrator().DropTable("people")
		},
	},
	{
		ID: "201608301430",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&Pet{})
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.Migrator().DropTable("pets")
		},
	},
}

var extendedMigrations = append(migrations, &Migration{
	ID: "201807221927",
	Migrate: func(tx *gorm.DB) error {
		return tx.AutoMigrate(&Book{})
	},
	Rollback: func(tx *gorm.DB) error {
		return tx.Migrator().DropTable("books")
	},
})

var failingMigration = []*Migration{
	{
		ID: "201904231300",
		Migrate: func(tx *gorm.DB) error {
			if err := tx.AutoMigrate(&Book{}); err != nil {
				return err
			}
			return errors.New("this transaction should be rolled back")
		},
		Rollback: func(tx *gorm.DB) error {
			return nil
		},
	},
}

type Person struct {
	gorm.Model
	Name string
}

type Pet struct {
	gorm.Model
	Name     string
	PersonID int
}

type Book struct {
	gorm.Model
	Name     string
	PersonID int
}

func TestMigration(t *testing.T) {
	forEachDatabase(t, func(db *gorm.DB) {
		m := New(db, DefaultOptions, migrations)

		err := m.Migrate()
		assert.NoError(t, err)
		assert.True(t, db.Migrator().HasTable(&Person{}))
		assert.True(t, db.Migrator().HasTable(&Pet{}))
		assert.Equal(t, int64(2), tableCount(t, db, "migrations"))

		err = m.RollbackLast()
		assert.NoError(t, err)
		assert.True(t, db.Migrator().HasTable(&Person{}))
		assert.False(t, db.Migrator().HasTable(&Pet{}))
		assert.Equal(t, int64(1), tableCount(t, db, "migrations"))

		err = m.RollbackLast()
		assert.NoError(t, err)
		assert.False(t, db.Migrator().HasTable(&Person{}))
		assert.False(t, db.Migrator().HasTable(&Pet{}))
		assert.Equal(t, int64(0), tableCount(t, db, "migrations"))
	})
}

func TestMigrateTo(t *testing.T) {
	forEachDatabase(t, func(db *gorm.DB) {
		m := New(db, DefaultOptions, extendedMigrations)

		err := m.MigrateTo("201608301430")
		assert.NoError(t, err)
		assert.True(t, db.Migrator().HasTable(&Person{}))
		assert.True(t, db.Migrator().HasTable(&Pet{}))
		assert.False(t, db.Migrator().HasTable(&Book{}))
		assert.Equal(t, int64(2), tableCount(t, db, "migrations"))
	})
}

func TestRollbackTo(t *testing.T) {
	forEachDatabase(t, func(db *gorm.DB) {
		m := New(db, DefaultOptions, extendedMigrations)

		// First, apply all migrations.
		err := m.Migrate()
		assert.NoError(t, err)
		assert.True(t, db.Migrator().HasTable(&Person{}))
		assert.True(t, db.Migrator().HasTable(&Pet{}))
		assert.True(t, db.Migrator().HasTable(&Book{}))
		assert.Equal(t, int64(3), tableCount(t, db, "migrations"))

		// Rollback to the first migration: only the last 2 migrations are expected to be rolled back.
		err = m.RollbackTo("201608301400")
		assert.NoError(t, err)
		assert.True(t, db.Migrator().HasTable(&Person{}))
		assert.False(t, db.Migrator().HasTable(&Pet{}))
		assert.False(t, db.Migrator().HasTable(&Book{}))
		assert.Equal(t, int64(1), tableCount(t, db, "migrations"))
	})
}

// If initSchema is defined, but no migrations are provided,
// then initSchema is executed.
func TestInitSchemaNoMigrations(t *testing.T) {
	forEachDatabase(t, func(db *gorm.DB) {
		m := New(db, DefaultOptions, []*Migration{})
		m.InitSchema(func(tx *gorm.DB) error {
			if err := tx.AutoMigrate(&Person{}); err != nil {
				return err
			}
			if err := tx.AutoMigrate(&Pet{}); err != nil {
				return err
			}
			return nil
		})

		assert.NoError(t, m.Migrate())
		assert.True(t, db.Migrator().HasTable(&Person{}))
		assert.True(t, db.Migrator().HasTable(&Pet{}))
		assert.Equal(t, int64(1), tableCount(t, db, "migrations"))
	})
}

// If initSchema is defined and migrations are provided,
// then initSchema is executed and the migration IDs are stored,
// even though the relevant migrations are not applied.
func TestInitSchemaWithMigrations(t *testing.T) {
	forEachDatabase(t, func(db *gorm.DB) {
		m := New(db, DefaultOptions, migrations)
		m.InitSchema(func(tx *gorm.DB) error {
			if err := tx.AutoMigrate(&Person{}); err != nil {
				return err
			}
			return nil
		})

		assert.NoError(t, m.Migrate())
		assert.True(t, db.Migrator().HasTable(&Person{}))
		assert.False(t, db.Migrator().HasTable(&Pet{}))
		assert.Equal(t, int64(3), tableCount(t, db, "migrations"))
	})
}

// If the schema has already been initialised,
// then initSchema() is not executed, even if defined.
func TestInitSchemaAlreadyInitialised(t *testing.T) {
	type Car struct {
		gorm.Model
	}

	forEachDatabase(t, func(db *gorm.DB) {
		m := New(db, DefaultOptions, []*Migration{})

		// Migrate with empty initialisation
		m.InitSchema(func(tx *gorm.DB) error {
			return nil
		})
		assert.NoError(t, m.Migrate())

		// Then migrate again, this time with a non empty initialisation
		// This second initialisation should not happen!
		m.InitSchema(func(tx *gorm.DB) error {
			if err := tx.AutoMigrate(&Car{}); err != nil {
				return err
			}
			return nil
		})
		assert.NoError(t, m.Migrate())

		assert.False(t, db.Migrator().HasTable(&Car{}))
		assert.Equal(t, int64(1), tableCount(t, db, "migrations"))
	})
}

// If the schema has not already been initialised,
// but any other migration has already been applied,
// then initSchema() is not executed, even if defined.
func TestInitSchemaExistingMigrations(t *testing.T) {
	type Car struct {
		gorm.Model
	}

	forEachDatabase(t, func(db *gorm.DB) {
		m := New(db, DefaultOptions, migrations)

		// Migrate without initialisation
		assert.NoError(t, m.Migrate())

		// Then migrate again, this time with a non empty initialisation
		// This initialisation should not happen!
		m.InitSchema(func(tx *gorm.DB) error {
			if err := tx.AutoMigrate(&Car{}); err != nil {
				return err
			}
			return nil
		})
		assert.NoError(t, m.Migrate())

		assert.False(t, db.Migrator().HasTable(&Car{}))
		assert.Equal(t, int64(2), tableCount(t, db, "migrations"))
	})
}

func TestMigrationIDDoesNotExist(t *testing.T) {
	forEachDatabase(t, func(db *gorm.DB) {
		m := New(db, DefaultOptions, migrations)
		assert.Equal(t, ErrMigrationIDDoesNotExist, m.MigrateTo("1234"))
		assert.Equal(t, ErrMigrationIDDoesNotExist, m.RollbackTo("1234"))
		assert.Equal(t, ErrMigrationIDDoesNotExist, m.MigrateTo(""))
		assert.Equal(t, ErrMigrationIDDoesNotExist, m.RollbackTo(""))
	})
}

func TestMissingID(t *testing.T) {
	forEachDatabase(t, func(db *gorm.DB) {
		migrationsMissingID := []*Migration{
			{
				Migrate: func(tx *gorm.DB) error {
					return nil
				},
			},
		}

		m := New(db, DefaultOptions, migrationsMissingID)
		assert.Equal(t, ErrMissingID, m.Migrate())
	})
}

func TestReservedID(t *testing.T) {
	forEachDatabase(t, func(db *gorm.DB) {
		migrationsReservedID := []*Migration{
			{
				ID: "SCHEMA_INIT",
				Migrate: func(tx *gorm.DB) error {
					return nil
				},
			},
		}

		m := New(db, DefaultOptions, migrationsReservedID)
		_, isReservedIDError := m.Migrate().(*ReservedIDError)
		assert.True(t, isReservedIDError)
	})
}

func TestDuplicatedID(t *testing.T) {
	forEachDatabase(t, func(db *gorm.DB) {
		migrationsDuplicatedID := []*Migration{
			{
				ID: "201705061500",
				Migrate: func(tx *gorm.DB) error {
					return nil
				},
			},
			{
				ID: "201705061500",
				Migrate: func(tx *gorm.DB) error {
					return nil
				},
			},
		}

		m := New(db, DefaultOptions, migrationsDuplicatedID)
		_, isDuplicatedIDError := m.Migrate().(*DuplicatedIDError)
		assert.True(t, isDuplicatedIDError)
	})
}

func TestEmptyMigrationList(t *testing.T) {
	forEachDatabase(t, func(db *gorm.DB) {
		t.Run("with empty list", func(t *testing.T) {
			m := New(db, DefaultOptions, []*Migration{})
			err := m.Migrate()
			assert.Equal(t, ErrNoMigrationDefined, err)
		})

		t.Run("with nil list", func(t *testing.T) {
			m := New(db, DefaultOptions, nil)
			err := m.Migrate()
			assert.Equal(t, ErrNoMigrationDefined, err)
		})
	})
}

func TestMigration_WithUseTransactions(t *testing.T) {
	options := DefaultOptions
	options.UseTransaction = true

	forEachDatabase(t, func(db *gorm.DB) {
		m := New(db, options, migrations)

		err := m.Migrate()
		require.NoError(t, err)
		assert.True(t, db.Migrator().HasTable(&Person{}))
		assert.True(t, db.Migrator().HasTable(&Pet{}))
		assert.Equal(t, int64(2), tableCount(t, db, "migrations"))

		err = m.RollbackLast()
		require.NoError(t, err)
		assert.True(t, db.Migrator().HasTable(&Person{}))
		assert.False(t, db.Migrator().HasTable(&Pet{}))
		assert.Equal(t, int64(1), tableCount(t, db, "migrations"))

		err = m.RollbackLast()
		require.NoError(t, err)
		assert.False(t, db.Migrator().HasTable(&Person{}))
		assert.False(t, db.Migrator().HasTable(&Pet{}))
		assert.Equal(t, int64(0), tableCount(t, db, "migrations"))
	}, "postgres", "sqlite3", "mssql")
}

func TestMigration_WithUseTransactionsShouldRollback(t *testing.T) {
	options := DefaultOptions
	options.UseTransaction = true

	forEachDatabase(t, func(db *gorm.DB) {
		assert.True(t, true)
		m := New(db, options, failingMigration)

		// Migration should return an error and not leave around a Book table
		err := m.Migrate()
		assert.Error(t, err)
		assert.False(t, db.Migrator().HasTable(&Book{}))
	}, "postgres", "sqlite3", "mssql")
}

func TestUnexpectedMigrationEnabled(t *testing.T) {
	forEachDatabase(t, func(db *gorm.DB) {
		options := DefaultOptions
		options.ValidateUnknownMigrations = true
		m := New(db, options, migrations)

		// Migrate without initialisation
		assert.NoError(t, m.Migrate())

		// Try with fewer migrations. Should fail as we see a migration in the db that
		// we don't recognise any more
		n := New(db, DefaultOptions, migrations[:1])
		assert.Equal(t, ErrUnknownPastMigration, n.Migrate())
	})
}

func TestUnexpectedMigrationDisabled(t *testing.T) {
	forEachDatabase(t, func(db *gorm.DB) {
		options := DefaultOptions
		options.ValidateUnknownMigrations = false
		m := New(db, options, migrations)

		// Migrate without initialisation
		assert.NoError(t, m.Migrate())

		// Try with fewer migrations. Should pass as we see a migration in the db that
		// we don't recognise any more, but the validation defaults off
		n := New(db, DefaultOptions, migrations[:1])
		assert.NoError(t, n.Migrate())
	})
}

func tableCount(t *testing.T, db *gorm.DB, tableName string) (count int64) {
	assert.NoError(t, db.Table(tableName).Count(&count).Error)
	return
}

func forEachDatabase(t *testing.T, fn func(database *gorm.DB), dialects ...string) {
	if len(databases) == 0 {
		panic("No database choosen for testing!")
	}

	for _, database := range databases {
		if len(dialects) > 0 && !contains(dialects, database.dialect) {
			t.Skip(fmt.Sprintf("test is not supported by [%s] dialect", database.dialect))
		}

		// Ensure defers are not stacked up for each DB
		func() {
			db, err := gorm.Open(database.driver, &gorm.Config{})
			require.NoError(t, err, "Could not connect to database %s, %v", database.dialect, err)

			// ensure tables do not exists
			assert.NoError(t, db.Migrator().DropTable("migrations", "people", "pets"))

			fn(db)
		}()
	}
}

func contains(haystack []string, needle string) bool {
	for _, straw := range haystack {
		if straw == needle {
			return true
		}
	}
	return false
}
