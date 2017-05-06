package gormigrate

import (
	"log"
	"os"
	"testing"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"gopkg.in/stretchr/testify.v1/assert"
)

type Person struct {
	gorm.Model
	Name string
}

type Pet struct {
	gorm.Model
	Name     string
	PersonID int
}

const (
	dbName = "testdb.sqlite3"
)

var (
	migrations = []*Migration{
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
)

func TestMigration(t *testing.T) {
	_ = os.Remove(dbName)

	db, err := gorm.Open("sqlite3", dbName)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err = db.DB().Ping(); err != nil {
		log.Fatal(err)
	}
	// db.LogMode(true)

	m := New(db, DefaultOptions, migrations)

	err = m.Migrate()
	assert.NoError(t, err)
	assert.True(t, db.HasTable(&Person{}))
	assert.True(t, db.HasTable(&Pet{}))
	assert.Equal(t, 2, tableCount(db, "migrations"))

	err = m.RollbackLast()
	assert.NoError(t, err)
	assert.True(t, db.HasTable(&Person{}))
	assert.False(t, db.HasTable(&Pet{}))
	assert.Equal(t, 1, tableCount(db, "migrations"))

	err = m.RollbackLast()
	assert.NoError(t, err)
	assert.False(t, db.HasTable(&Person{}))
	assert.False(t, db.HasTable(&Pet{}))
	assert.Equal(t, 0, tableCount(db, "migrations"))
}

func TestInitSchema(t *testing.T) {
	os.Remove(dbName)

	db, err := gorm.Open("sqlite3", dbName)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err = db.DB().Ping(); err != nil {
		log.Fatal(err)
	}
	// db.LogMode(true)

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

	err = m.Migrate()
	assert.NoError(t, err)
	assert.True(t, db.HasTable(&Person{}))
	assert.True(t, db.HasTable(&Pet{}))
	assert.Equal(t, 2, tableCount(db, "migrations"))
}

func TestMissingID(t *testing.T) {
	os.Remove(dbName)

	db, err := gorm.Open("sqlite3", dbName)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	assert.NoError(t, db.DB().Ping())

	migrationsMissingID := []*Migration{
		{
			Migrate: func(tx *gorm.DB) error {
				return nil
			},
		},
	}

	m := New(db, DefaultOptions, migrationsMissingID)
	assert.Equal(t, ErrMissingID, m.Migrate())
}

func TestDuplicatedID(t *testing.T) {
	os.Remove(dbName)

	db, err := gorm.Open("sqlite3", dbName)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	assert.NoError(t, db.DB().Ping())

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
}

func tableCount(db *gorm.DB, tableName string) (count int) {
	db.Table(tableName).Count(&count)
	return
}
