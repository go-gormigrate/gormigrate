package gormigrate

import (
	"fmt"
	"gonum.org/v1/gonum/graph/simple"
	"gonum.org/v1/gonum/graph/topo"
	"gorm.io/gorm"
	"hash/fnv"
)

type node struct {
	v *Migration
}

func (n node) ID() (id int64) {
	h := fnv.New64a()
	_, _ = h.Write([]byte(n.v.MigrationID))
	return int64(h.Sum64())
}

// resolveDependency checks the incoming migration request comparing with stored done migrations
// then returns a list in which their dependency can't be satisfied thus would nor run
func (g *Gormigrate) resolveDependency() []*Migration {
	var migrationsToRun []*Migration
	var loadPred func(*Migration)
	loadPred = func(m *Migration) {
		if m == nil {
			return
		}
		// lookup for migrations which had run before
		var lookups []*Migration
		ids := []string{m.MigrationID}
		_ = g.tx.Transaction(func(tx *gorm.DB) error {
			return tx.Table(g.options.TableName).
				Where(fmt.Sprintf("%s = ?", g.options.IDColumnName), ids[0]).
				FirstOrInit(&lookups).Error
		})
		if len(lookups) > 0 && lookups[0].MigrationID == m.MigrationID {
			return
		}
		if len(m.Dependencies) > 0 {
			for _, d := range m.Dependencies {
				ids = append(ids, d.MigrationID)
			}
		}
		_ = g.tx.Transaction(func(tx *gorm.DB) error {
			return tx.Table(g.options.TableName).
				Where(fmt.Sprintf("%s in ?", g.options.IDColumnName), ids).
				Distinct(g.options.IDColumnName).
				Find(&lookups).Error
		})
		for i := range lookups {
			lookups[i].Migrate = dummyMigration
			migrationsToRun = append(migrationsToRun, lookups[i])
		}
		// no more predecessors.
		// Note: There is a trick.
		// Dependencies scanned from storage is always ahead of current migration.
		// When after sorted, the dummyMigration would replace the original one.
		migrationsToRun = append(migrationsToRun, m)
	}
	for i := range g.migrations {
		loadPred(g.migrations[i])
	}
	// add additional dependency to keep order of isolated migrations
	for i := len(migrationsToRun) - 1; i > 0; i-- {
		if len(migrationsToRun[i-1].Dependencies) == 0 {
			migrationsToRun[i].Dependencies = append(migrationsToRun[i].Dependencies, migrationsToRun[i-1])
		}
	}
	s, d := sort(migrationsToRun)
	g.migrations = s
	return d
}

// sort returns a ordered list in which all dependencies were satisfied,
// and a drop list in which some dependencies was missing
func sort(ms []*Migration) (sorted []*Migration, drops []*Migration) {
	sorted = make([]*Migration, 0)
	toSort := make(map[string]*Migration)
	for _, m := range ms {
		if _, ok := toSort[m.MigrationID]; !ok { // only the first
			toSort[m.MigrationID] = m
		} else {
			drops = append(drops, m)
		}
	}
	g := simple.NewDirectedGraph()
Recheck:
	for _, m := range toSort {
		if len(m.Dependencies) > 0 {
			for _, d := range m.Dependencies {
				if _, ok := toSort[d.MigrationID]; !ok {
					drops = append(drops, m)
					delete(toSort, m.MigrationID)
					goto Recheck
				}
			}
		}
	}
	for _, m := range toSort {
		for _, d := range m.Dependencies {
			if f, ok := toSort[d.MigrationID]; ok {
				g.SetEdge(g.NewEdge(node{toSort[f.MigrationID]}, node{m}))
			}
		}
		if g.Node(node{m}.ID()) == nil {
			g.AddNode(node{m})
		}
	}
	gSorted, _ := topo.Sort(g)
	for i := range gSorted {
		n := gSorted[i].(node)
		sorted = append(sorted, n.v)
	}
	return sorted, drops
}
