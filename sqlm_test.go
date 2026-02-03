package sqlm_test

import (
	"context"
	"os"
	"testing"

	"github.com/w6xian/sqlm"
	"github.com/w6xian/sqlm/store"
)

func TestSqlite(t *testing.T) {
	dbFile := "test.db"
	opt, err := sqlm.NewOptionsWithServer(sqlm.Server{
		Protocol:     "sqlite",
		DSN:          dbFile,
		MaxOpenConns: 10,
	}, "sqlite_test")
	if err != nil {
		t.Fatal(err)
	}

	driver, err := store.NewDriver(opt)
	if err != nil {
		t.Fatal(err)
	}
	sqlm.Use(driver)

	ctx := context.Background()
	db := sqlm.NewInstance(ctx, "sqlite_test")

	// Create table
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT,
		age INTEGER,
		score REAL
	)`)
	if err != nil {
		t.Fatal(err)
	}

	// Insert data
	_, err = db.Exec("INSERT INTO users (name, age, score) VALUES (?, ?, ?)", "Alice", 25, 95.5)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec("INSERT INTO users (name, age, score) VALUES (?, ?, ?)", "Bob", 30, 88.0)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec("INSERT INTO users (name, age, score) VALUES (?, ?, ?)", "Charlie", 35, 70.0)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("Query with Filter (Map)", func(t *testing.T) {
		rows, err := db.Table("users").Select("name,age").AndFilters(map[string]any{
			"name": "Alice",
		}).Query()
		if err != nil {
			t.Fatal(err)
		}
		if rows == nil || rows.Length() == 0 {
			t.Error("expected result for Alice")
		}
	})

	t.Run("Query with Where (Literals - Compatibility)", func(t *testing.T) {
		rows, err := db.Table("users").Select("name").Where("age = %d", 30).Query()
		if err != nil {
			t.Fatal(err)
		}
		if rows == nil || rows.Length() == 0 {
			t.Error("expected result for age 30")
		}
	})

	t.Run("Query Mixed Filter and Where", func(t *testing.T) {
		rows, err := db.Table("users").Select("name").Where("score > %f", 80.0).AndFilters(map[string]any{
			"age": 30,
		}).Query()
		if err != nil {
			t.Fatal(err)
		}
		if rows != nil && rows.Length() > 0 {
			// Found
		}
	})

	t.Run("Update with Filter (Map)", func(t *testing.T) {
		_, err := db.Table("users").AndFilters(map[string]any{
			"name": "Charlie",
		}).Update(map[string]any{
			"score": 80,
		}).Execute()
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("Delete with Filter", func(t *testing.T) {
		_, err := db.Table("users").AndFilters(map[string]any{
			"name": "Bob",
		}).Delete().Execute()
		if err != nil {
			t.Fatal(err)
		}
	})

	// Cleanup
	os.Remove(dbFile)
}

func BenchmarkTest(b *testing.B) {
	for i := 0; i < b.N; i++ {
		// do nothing
	}
}
