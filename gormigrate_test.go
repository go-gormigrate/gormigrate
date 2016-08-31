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
	db         *gorm.DB
	gormigrate *Gormigrate
)

func TestMain(m *testing.M) {
	os.Remove(dbName)

	var err error
	db, err = gorm.Open("sqlite3", dbName)
	if err != nil {
		log.Fatal(err)
	}
	if err = db.DB().Ping(); err != nil {
		log.Fatal(err)
	}

	db.LogMode(true)

	gormigrate = New(db, DefaultOptions, []*Migration{
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
	})

	os.Exit(m.Run())
}

func TestMigration(t *testing.T) {
	err := gormigrate.Migrate()
	assert.Nil(t, err)
	assert.True(t, db.HasTable(&Person{}))
	assert.True(t, db.HasTable(&Pet{}))
	assert.Equal(t, 2, tableCount("migrations"))

	err = gormigrate.RollbackLast()
	assert.Nil(t, err)
	assert.True(t, db.HasTable(&Person{}))
	assert.False(t, db.HasTable(&Pet{}))
	assert.Equal(t, 1, tableCount("migrations"))
}

func tableCount(tableName string) (count int) {
	db.Table(tableName).Count(&count)
	return
}
