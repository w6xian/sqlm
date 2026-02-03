package sqlm_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/w6xian/sqlm"
	"github.com/w6xian/sqlm/store"
)

type NoopLogger struct{}

func (l *NoopLogger) Debug(s string) {}
func (l *NoopLogger) Info(s string)  {}
func (l *NoopLogger) Warn(s string)  {}
func (l *NoopLogger) Error(s string) {}
func (l *NoopLogger) Panic(s string) {}
func (l *NoopLogger) Fatal(s string) {}

func TestBenchFilePickedUp(t *testing.T) {
	t.Log("Benchmark file is picked up")
}

func setupBenchDb(b *testing.B) (*sqlm.Db, string) {
	dbFile := fmt.Sprintf("bench_%d.db", time.Now().UnixNano())
	// Create options
	opt, err := sqlm.NewOptionsWithServer(sqlm.Server{
		Protocol:     "sqlite",
		DSN:          dbFile,
		MaxOpenConns: 10,
		MaxIdleConns: 5,
		MaxLifetime:  int(time.Minute),
	}, "sqlite_bench")
	if err != nil {
		b.Fatal(err)
	}
	opt.SetLogger(&NoopLogger{})

	// Create driver
	driver, err := store.NewDriver(opt)
	if err != nil {
		b.Fatal(err)
	}

	// Register driver
	sqlm.Use(driver)

	// Get instance
	db := sqlm.NewInstance(context.Background(), "sqlite_bench")

	// Create table
	createSQL := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT,
		age INTEGER,
		score REAL
	);
	`
	_, err = db.Exec(createSQL)
	if err != nil {
		b.Fatal(err)
	}

	// Insert some data
	for i := 0; i < 100; i++ {
		_, err = db.Exec("INSERT INTO users (name, age, score) VALUES (?, ?, ?)", fmt.Sprintf("User%d", i), 20+(i%50), 50.0+float64(i%50))
		if err != nil {
			b.Fatal(err)
		}
	}

	return db, dbFile
}

func BenchmarkConnectReuse(b *testing.B) {
	dbFile := "bench_connect.db"
	opt, err := sqlm.NewOptionsWithServer(sqlm.Server{
		Protocol:     "sqlite",
		DSN:          dbFile,
		MaxOpenConns: 10,
	}, "sqlite_bench_conn")
	if err != nil {
		b.Fatal(err)
	}
	opt.SetLogger(&NoopLogger{})
	driver, err := store.NewDriver(opt)
	if err != nil {
		b.Fatal(err)
	}
	sqlm.Use(driver)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sqlm.NewInstance(context.Background(), "sqlite_bench_conn")
	}
	os.Remove(dbFile)
}

func BenchmarkQueryMapFilters(b *testing.B) {
	db, dbFile := setupBenchDb(b)
	defer os.Remove(dbFile)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rows, err := db.Table("users").Select("name,age").AndFilters(map[string]any{
			"age": 20 + (i % 50),
		}).Query()
		if err != nil {
			b.Fatal(err)
		}
		if rows.Length() == 0 {
			// b.Fatal("expected result")
		}
	}
}

func BenchmarkQuerySprintfWhere(b *testing.B) {
	db, dbFile := setupBenchDb(b)
	defer os.Remove(dbFile)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rows, err := db.Table("users").Select("name,age").Where("age = %d", 20+(i%50)).Query()
		if err != nil {
			b.Fatal(err)
		}
		if rows.Length() == 0 {
			// b.Fatal("expected result")
		}
	}
}
