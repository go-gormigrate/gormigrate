package gormigrate

import (
	"os"
	"testing"

	"github.com/jinzhu/gorm"
	_ "github.com/joho/godotenv/autoload"
	"gopkg.in/stretchr/testify.v1/assert"
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

func TestInitSchema(t *testing.T) {
	forEachDatabase(t, func(db *gorm.DB) {
		m := New(db, DefaultOptions, migrations)
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
		assert.Equal(t, 2, tableCount(t, db, "migrations"))
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
