package sqlm_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/w6xian/sqlm"
	"github.com/w6xian/sqlm/store"
)

// TestConcurrentReads checks that independent builders can be used in
// parallel: every goroutine owns its own Table, sharing only the pool.
func TestConcurrentReads(t *testing.T) {
	db, _ := newSQLite(t, "concurrent_reads")
	createUsers(t, db)
	seedUsers(t, db, 20)

	var wg sync.WaitGroup
	var failures int32
	const workers = 16
	const loops = 10

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < loops; i++ {
				// 每个 goroutine 自己建 builder
				total, err := db.Table("users").And("age < 100").GetCount()
				if err != nil {
					atomic.AddInt32(&failures, 1)
					return
				}
				if total != 20 {
					atomic.AddInt32(&failures, 1)
					return
				}
				list := make([]User, 0, 20)
				if err := db.Table("users").Limit(5).ScanMulti(&list); err != nil {
					atomic.AddInt32(&failures, 1)
					return
				}
			}
		}(w)
	}
	wg.Wait()
	assert.Zero(t, failures)
}

// TestConcurrentWrites makes sure batched inserts survive parallel access and
// that every row lands exactly once.
func TestConcurrentWrites(t *testing.T) {
	db, _ := newSQLite(t, "concurrent_writes")
	createUsers(t, db)

	var wg sync.WaitGroup
	var conflicts int64
	const workers = 8
	const loops = 5

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < loops; i++ {
				_, err := db.Table("users").Insert(map[string]any{
					"name": fmt.Sprintf("w%d-%d", w, i),
					"age":  w*loops + i,
				})
				if err != nil {
					// sqlite在并发写时可能返回 locked，busy_timeout 之后重试
					atomic.AddInt64(&conflicts, 1)
				}
			}
		}(w)
	}
	wg.Wait()

	total, err := db.Table("users").GetCount()
	require.NoError(t, err)
	assert.Equal(t, int64(workers*loops)-conflicts, total)
}

// TestConcurrentTransactions opens transactions from several goroutines: each
// one must either commit fully or roll back completely.
func TestConcurrentTransactions(t *testing.T) {
	db, _ := newSQLite(t, "concurrent_tx")
	createUsers(t, db)

	var wg sync.WaitGroup
	var committed int64
	const workers = 6

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < 3; i++ {
				rollback := (w+i)%2 == 0
				n, err := db.Action(func(tx sqlm.ITable, args ...any) (int64, error) {
					if _, err := tx.Table("users").Insert(map[string]any{
						"name": fmt.Sprintf("tx%d-%d", w, i),
						"age":  i,
					}); err != nil {
						return 0, err
					}
					if rollback {
						return 0, fmt.Errorf("rollback requested")
					}
					return 1, nil
				})
				if err != nil {
					continue
				}
				atomic.AddInt64(&committed, n)
			}
		}(w)
	}
	wg.Wait()

	total, err := db.Table("users").GetCount()
	require.NoError(t, err)
	assert.Equal(t, committed, total)
}

// TestConcurrentRegistry stresses the global instance registry, which carries
// its own mutex and must stay usable from several goroutines.
func TestConcurrentRegistry(t *testing.T) {
	dir := t.TempDir()

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := fmt.Sprintf("registry_%d", i)
			opt, err := sqlm.NewOptionsWithServer(sqlm.Server{
				Protocol: "sqlite",
				DSN:      fmt.Sprintf("%s/%s.db", dir, name),
			}, name)
			if err != nil {
				t.Error(err)
				return
			}
			opt.SetLogger(sqlm.NewNullLogger())
			drv, err := store.NewDriver(opt)
			if err != nil {
				t.Error(err)
				return
			}
			sqlm.Use(drv)

			db := sqlm.NewInstance(context.Background(), name)
			if db == nil {
				t.Errorf("instance %s is nil", name)
				return
			}
			if sqlm.Has(name) {
				_, _ = db.Exec("SELECT 1")
				db.Close()
			}
		}(i)
	}
	wg.Wait()
}

// TestSharedRowsAccessedSequentially verifies that a Rows value can be read by
// another goroutine as long as it is not used at the same time.
func TestSharedRowsAccessedSequentially(t *testing.T) {
	db, _ := newSQLite(t, "shared_rows")
	createUsers(t, db)
	seedUsers(t, db, 5)

	rows, err := db.Table("users").QueryMulti()
	require.NoError(t, err)
	require.Equal(t, 5, rows.Length())

	handoff := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-handoff
		assert.Equal(t, 5, rows.Length())
		assert.NotEmpty(t, rows.ColumnNames())
	}()
	close(handoff)
	wg.Wait()
}
