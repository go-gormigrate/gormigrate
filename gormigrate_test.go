package gormigrate

import (
	"os"
	"testing"

	"github.com/jinzhu/gorm"
	_ "github.com/joho/godotenv/autoload"
	"github.com/stretchr/testify/assert"
)

var databases []database

type database struct {
	name    string
	connEnv string
}

var migrations = []*Migration{
	{
		ID: "201608301400",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&Person{}).Error
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.DropTable("people").Error
		},
	},
	{
		ID: "201608301430",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(&Pet{}).Error
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.DropTable("pets").Error
		},
	},
}

var extendedMigrations = append(migrations, &Migration{
	ID: "201807221927",
	Migrate: func(tx *gorm.DB) error {
		return tx.AutoMigrate(&Book{}).Error
	},
	Rollback: func(tx *gorm.DB) error {
		return tx.DropTable("books").Error
	},
})

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
		assert.True(t, db.HasTable(&Person{}))
		assert.True(t, db.HasTable(&Pet{}))
		assert.Equal(t, 2, tableCount(t, db, "migrations"))

		err = m.RollbackLast()
		assert.NoError(t, err)
		assert.True(t, db.HasTable(&Person{}))
		assert.False(t, db.HasTable(&Pet{}))
		assert.Equal(t, 1, tableCount(t, db, "migrations"))

		err = m.RollbackLast()
		assert.NoError(t, err)
		assert.False(t, db.HasTable(&Person{}))
		assert.False(t, db.HasTable(&Pet{}))
		assert.Equal(t, 0, tableCount(t, db, "migrations"))
	})
}

func TestMigrateTo(t *testing.T) {
	forEachDatabase(t, func(db *gorm.DB) {
		m := New(db, DefaultOptions, extendedMigrations)

		err := m.MigrateTo("201608301430")
		assert.NoError(t, err)
		assert.True(t, db.HasTable(&Person{}))
		assert.True(t, db.HasTable(&Pet{}))
		assert.False(t, db.HasTable(&Book{}))
		assert.Equal(t, 2, tableCount(t, db, "migrations"))
	})
}

func TestRollbackTo(t *testing.T) {
	forEachDatabase(t, func(db *gorm.DB) {
		m := New(db, DefaultOptions, extendedMigrations)

		// First, apply all migrations.
		err := m.Migrate()
		assert.NoError(t, err)
		assert.True(t, db.HasTable(&Person{}))
		assert.True(t, db.HasTable(&Pet{}))
		assert.True(t, db.HasTable(&Book{}))
		assert.Equal(t, 3, tableCount(t, db, "migrations"))

		// Rollback to the first migration: only the last 2 migrations are expected to be rolled back.
		err = m.RollbackTo("201608301400")
		assert.NoError(t, err)
		assert.True(t, db.HasTable(&Person{}))
		assert.False(t, db.HasTable(&Pet{}))
		assert.False(t, db.HasTable(&Book{}))
		assert.Equal(t, 1, tableCount(t, db, "migrations"))
	})
}

// If initSchema is defined, but no migrations are provided,
// then initSchema is executed.
func TestInitSchemaNoMigrations(t *testing.T) {
	forEachDatabase(t, func(db *gorm.DB) {
		m := New(db, DefaultOptions, []*Migration{})
		m.InitSchema(func(tx *gorm.DB) error {
			if err := tx.AutoMigrate(&Person{}).Error; err != nil {
				return err
			}
			if err := tx.AutoMigrate(&Pet{}).Error; err != nil {
				return err
			}
			return nil
		})

		assert.NoError(t, m.Migrate())
		assert.True(t, db.HasTable(&Person{}))
		assert.True(t, db.HasTable(&Pet{}))
		assert.Equal(t, 1, tableCount(t, db, "migrations"))
	})
}

// If initSchema is defined and migrations are provided,
// then initSchema is executed and the migration IDs are stored,
// even though the relevant migrations are not applied.
func TestInitSchemaWithMigrations(t *testing.T) {
	forEachDatabase(t, func(db *gorm.DB) {
		m := New(db, DefaultOptions, migrations)
		m.InitSchema(func(tx *gorm.DB) error {
			if err := tx.AutoMigrate(&Person{}).Error; err != nil {
				return err
			}
			return nil
		})

		assert.NoError(t, m.Migrate())
		assert.True(t, db.HasTable(&Person{}))
		assert.False(t, db.HasTable(&Pet{}))
		assert.Equal(t, 3, tableCount(t, db, "migrations"))
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
			if err := tx.AutoMigrate(&Car{}).Error; err != nil {
				return err
			}
			return nil
		})
		assert.NoError(t, m.Migrate())

		assert.False(t, db.HasTable(&Car{}))
		assert.Equal(t, 1, tableCount(t, db, "migrations"))
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
			if err := tx.AutoMigrate(&Car{}).Error; err != nil {
				return err
			}
			return nil
		})
		assert.NoError(t, m.Migrate())

		assert.False(t, db.HasTable(&Car{}))
		assert.Equal(t, 2, tableCount(t, db, "migrations"))
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

func tableCount(t *testing.T, db *gorm.DB, tableName string) (count int) {
	assert.NoError(t, db.Table(tableName).Count(&count).Error)
	return
}

func forEachDatabase(t *testing.T, fn func(database *gorm.DB)) {
	if len(databases) == 0 {
		panic("No database choosen for testing!")
	}

	for _, database := range databases {
		db, err := gorm.Open(database.name, os.Getenv(database.connEnv))
		assert.NoError(t, err, "Could not connect to database %s, %v", database.name, err)

		defer db.Close()

		// ensure tables do not exists
		assert.NoError(t, db.DropTableIfExists("migrations", "people", "pets").Error)

		fn(db)
	}
}
