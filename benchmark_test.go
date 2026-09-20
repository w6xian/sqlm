package sqlm_test

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/w6xian/sqlm"
)

var benchSeq int64 // atomic counter keeping benchmark instance names unique

func benchName(b *testing.B) string {
	b.Helper()
	n := atomic.AddInt64(&benchSeq, 1)
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' {
			return r
		}
		return '_'
	}, fmt.Sprintf("%s_%d", b.Name(), n))
}

// benchDB prepares a sqlite instance seeded with n users. Cleanup (including
// the database file) is handled by the harness through b.TempDir().
//
// The instance name is derived from the benchmark name plus a unique suffix so
// every run owns its own registry entry and database file (a benchmark may be
// executed more than once by the testing binary).
func benchDB(b *testing.B, n int) *sqlm.Db {
	b.Helper()
	db, _ := newSQLite(b, benchName(b))
	createUsers(b, db)
	if n > 0 {
		rows := make([][]any, 0, n)
		for i := 0; i < n; i++ {
			rows = append(rows, []any{
				fmt.Sprintf("Bench%04d", i),
				18 + (i % 40),
				float64(50 + (i % 50)),
				"13800000000",
				[]byte("avatar"),
				int64(1700000000 + i),
			})
		}
		_, err := db.Table("users").Inserts(
			[]string{"name", "age", "score", "mobile", "avatar", "intime"}, rows)
		if err != nil {
			b.Fatal(err)
		}
	}
	return db
}

// BenchmarkInstanceLookup measures NewInstance, the hottest call of the
// library: it must stay a map lookup without any connection round trip.
func BenchmarkInstanceLookup(b *testing.B) {
	name := benchName(b)
	db, _ := newSQLite(b, name)
	createUsers(b, db)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if got := sqlm.NewInstance(context.Background(), name); got == nil {
			b.Fatal("instance is nil")
		}
	}
}

// BenchmarkBuildSQL measures the statement builder alone.
func BenchmarkBuildSQL(b *testing.B) {
	db := benchDB(b, 0)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = db.Table("users").
			Select("id", "name", "age").
			And("age > 20").
			OrderDESC("id").
			LimitOffset(10, 0).
			SQL()
	}
}

func BenchmarkQueryRow(b *testing.B) {
	db := benchDB(b, 100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		row, err := db.Table("users").AndFilters(map[string]any{"id": 1 + (i % 100)}).Query()
		if err != nil {
			b.Fatal(err)
		}
		if row.Length() == 0 {
			b.Fatal("empty row")
		}
	}
}

func BenchmarkQueryMulti(b *testing.B) {
	db := benchDB(b, 200)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rows, err := db.Table("users").Limit(20).QueryMulti()
		if err != nil {
			b.Fatal(err)
		}
		if rows.Length() != 20 {
			b.Fatalf("expected 20 rows, got %d", rows.Length())
		}
	}
}

func BenchmarkScanStruct(b *testing.B) {
	db := benchDB(b, 200)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		list := make([]User, 0, 20)
		if err := db.Table("users").Limit(20).ScanMulti(&list); err != nil {
			b.Fatal(err)
		}
		if len(list) != 20 {
			b.Fatalf("expected 20 users, got %d", len(list))
		}
	}
}

func BenchmarkGetCount(b *testing.B) {
	db := benchDB(b, 100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := db.Table("users").GetCount(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkInsertSingle(b *testing.B) {
	db := benchDB(b, 0)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := db.Table("users").Insert(map[string]any{
			"name": "BenchInsert",
			"age":  i % 60,
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkInsertBatch(b *testing.B) {
	db := benchDB(b, 0)
	cols := []string{"name", "age", "score", "mobile", "avatar", "intime"}
	const batch = 50
	rows := make([][]any, batch)
	for i := range rows {
		rows[i] = []any{"BenchBatch", 18, 50.0, "13800000000", []byte("avatar"), time.Now().Unix()}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := db.Table("users").Inserts(cols, rows); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkConnectReuse measures NewInstance, which internally asks the driver
// to reuse the already established pool instead of dialling again.
func BenchmarkConnectReuse(b *testing.B) {
	db := benchDB(b, 0)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		conn, err := db.Conn()
		if err != nil {
			b.Fatal(err)
		}
		if conn == nil {
			b.Fatal("nil connection")
		}
	}
}

// BenchmarkQueryMapFilters builds the WHERE clause from a map (AndFilters).
func BenchmarkQueryMapFilters(b *testing.B) {
	db := benchDB(b, 100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		row, err := db.Table("users").Select("name,age").
			AndFilters(map[string]any{"age": 18 + (i % 40)}).
			Query()
		if err != nil {
			b.Fatal(err)
		}
		if row.Length() == 0 {
			b.Fatal("empty row")
		}
	}
}

// BenchmarkQuerySprintfWhere builds the same statement through the legacy
// fmt.Sprintf style helpers.
func BenchmarkQuerySprintfWhere(b *testing.B) {
	db := benchDB(b, 100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		row, err := db.Table("users").Select("name,age").
			Where("age = %d", 18+(i%40)).
			Query()
		if err != nil {
			b.Fatal(err)
		}
		if row.Length() == 0 {
			b.Fatal("empty row")
		}
	}
}

func BenchmarkUpdateRow(b *testing.B) {
	db := benchDB(b, 100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := db.Table("users").
			AndFilters(map[string]any{"id": 1 + (i % 100)}).
			Update(map[string]any{"age": i % 60}).
			Execute()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTransaction(b *testing.B) {
	db := benchDB(b, 0)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := db.Action(func(tx sqlm.ITable, args ...any) (int64, error) {
			return tx.Table("users").Insert(map[string]any{"name": "TxBench", "age": 1})
		}); err != nil {
			b.Fatal(err)
		}
	}
}
