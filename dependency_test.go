package gormigrate

import (
	"errors"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
	"testing"
)

type PersonV2 struct {
	Person
	Age uint
}

func (p *Person) TableName() string {
	return "people"
}

type PetsV2 struct {
	Pet
	Age uint
}

func (p *Pet) TableName() string {
	return "pets"
}

var migrationsV1 = []*Migration{
	{
		MigrationID: "2",
		Migrate: func(tx *gorm.DB) error {
			println("Do migration 2")
			return tx.AutoMigrate(&Pet{})
		},
		Rollback: func(tx *gorm.DB) error {
			println("Undo migration 2")
			return tx.Migrator().DropTable("pets")
		},
		Dependencies: []*Migration{{MigrationID: "1"}},
	},
	{
		MigrationID: "1",
		Migrate: func(tx *gorm.DB) error {
			println("Do migration 1")
			return tx.AutoMigrate(&Person{})
		},
		Rollback: func(tx *gorm.DB) error {
			println("Undo migration 1")
			return tx.Migrator().DropTable("people")
		},
	},
}

var runCount = 0
var migrationsV2 = []*Migration{
	{
		MigrationID: "3",
		Migrate: func(tx *gorm.DB) error {
			println("Do migration 3")
			runCount += 1
			return nil
		},
		Dependencies: []*Migration{DummyMigration("1.1.1"), DummyMigration("2.2")},
	},
	{
		MigrationID: "1.1.1",
		Migrate: func(tx *gorm.DB) error {
			println("Do migration 1.1.1")
			return tx.Model(&Pet{}).AutoMigrate(&PetsV2{})
		},
		Rollback: func(tx *gorm.DB) error {
			println("Undo migration 1.1.1")
			return tx.Migrator().DropColumn(&PetsV2{}, "Age")
		},
		Dependencies: []*Migration{DummyMigration("1.1")},
	},
	{
		MigrationID: "2.2",
		Migrate: func(tx *gorm.DB) error {
			println("Do migration 2.2")
			return nil
		},
		Dependencies: []*Migration{DummyMigration("2")},
	},
	{
		MigrationID: "2.1",
		Migrate: func(tx *gorm.DB) error {
			println("Do migration 2.1")
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			println("Undo migration 2.1")
			return nil
		},
		Dependencies: []*Migration{migrationsV1[0]},
	},
	{
		MigrationID: "1.1",
		Migrate: func(tx *gorm.DB) error {
			println("Do migration 1.1")
			return tx.Model(&Person{}).AutoMigrate(&PersonV2{})
		},
		Rollback: func(tx *gorm.DB) error {
			println("Undo migration 1.1")
			return tx.Migrator().DropColumn(&PersonV2{}, "Age")
		},
		Dependencies: []*Migration{migrationsV1[1]},
	},
	{
		MigrationID: "(will not run)",
		Migrate: func(db *gorm.DB) error {
			return errors.New("should not run")
		},
		Dependencies: []*Migration{DummyMigration("NotSatisfied")},
	},
}

func TestSort(t *testing.T) {
	sorted1, _ := sort(migrationsV1)
	assert.True(t, sorted1[0].MigrationID == "1")
	assert.True(t, sorted1[1].MigrationID == "2")
	assert.True(t, sorted1[0].Migrate != nil)
	assert.True(t, sorted1[1].Migrate != nil)
	sorted2, drops := sort([]*Migration{
		{MigrationID: "3", Dependencies: []*Migration{{MigrationID: "0"}}},
		{MigrationID: "1"},
		{MigrationID: "4", Dependencies: []*Migration{{MigrationID: "2"}}},
		{MigrationID: "3.4", Dependencies: []*Migration{{MigrationID: "3.3"}, {MigrationID: "4"}}},
		{MigrationID: "2", Dependencies: []*Migration{{MigrationID: "1"}}},
		{MigrationID: "3.1", Dependencies: []*Migration{{MigrationID: "3"}, {MigrationID: "4"}}},
		{MigrationID: "3.3", Dependencies: []*Migration{{MigrationID: "3"}, {MigrationID: "4"}}},
		{MigrationID: "3.2", Dependencies: []*Migration{{MigrationID: "3.1"}, {MigrationID: "4"}}},
	})
	find := func(id string, s []*Migration) int {
		for i := range s {
			if id == s[i].MigrationID {
				return i
			}
		}
		return -1
	}
	assert.True(t, sorted2[0].MigrationID == "1")
	assert.True(t, sorted2[1].MigrationID == "2")
	assert.True(t, sorted2[2].MigrationID == "4")
	assert.True(t, find("3", drops) != -1)
	assert.True(t, find("3.1", drops) != -1)
	assert.True(t, find("3.2", drops) != -1)
	assert.True(t, find("3.3", drops) != -1)
	assert.True(t, find("3.4", drops) != -1)
}
func TestDependency(t *testing.T) {
	forEachDatabase(t, func(db *gorm.DB) {
		m := New(db, DefaultOptions, migrationsV1)
		err := m.Migrate()
		assert.NoError(t, err)
		assert.True(t, db.Migrator().HasTable(&Person{}))
		assert.True(t, db.Migrator().HasTable(&Pet{}))
		assert.NoError(t, m.RollbackTo("1"))
		assert.False(t, db.Migrator().HasTable(&Pet{}))
		m2 := New(db, DefaultOptions, migrationsV2)
		assert.NoError(t, m2.Migrate())
		assert.True(t, db.Migrator().HasColumn(&PersonV2{}, "Age"))
		assert.True(t, db.Migrator().HasColumn(&PetsV2{}, "Age"))
		assert.True(t, runCount == 0)
		assert.NoError(t, m.MigrateTo("2"))
		m3 := New(db, DefaultOptions, migrationsV2)
		assert.NoError(t, m3.Migrate())
		assert.True(t, runCount == 1)
		assert.NoError(t, New(db, DefaultOptions, migrationsV2).Migrate()) // re-run00
		assert.True(t, runCount == 1)
		ms := make([]*Migration, 0)
		db.Table(m3.options.TableName).
			Preload("Dependencies").
			Distinct(m3.options.IDColumnName).
			Find(&ms)
		//assert.NoError(t, m2.RollbackTo("1.1"))
		println("done")
	})
}
