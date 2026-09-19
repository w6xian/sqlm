package sqlm_test

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/w6xian/sqlm"
	"github.com/w6xian/sqlm/store"
)

// ---------------------------------------------------------------------------
// testing harness
// ---------------------------------------------------------------------------

// testLog records everything emitted by sqlm so tests can assert on the
// generated SQL without touching stdout.
type testLog struct {
	mu    sync.Mutex
	debug []string
	info  []string
	warn  []string
	error []string
}

func (l *testLog) Debug(s string) { l.record(&l.debug, s) }
func (l *testLog) Info(s string)  { l.record(&l.info, s) }
func (l *testLog) Warn(s string)  { l.record(&l.warn, s) }
func (l *testLog) Error(s string) { l.record(&l.error, s) }
func (l *testLog) Panic(s string) { l.record(&l.error, s) }
func (l *testLog) Fatal(s string) { l.record(&l.error, s) }

func (l *testLog) record(dst *[]string, s string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	*dst = append(*dst, s)
}

// LastSQL returns the last SELECT/UPDATE statement traced at DEBUG level.
func (l *testLog) LastSQL() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.debug == nil {
		return ""
	}
	return l.debug[len(l.debug)-1]
}

func (l *testLog) Errors() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.error...)
}

// newSQLite registers a private sqlite instance for a single test.
//
// Every test gets its own database file inside t.TempDir() and its own
// instance name, so tests stay independent and can run in any order.
func newSQLite(t testing.TB, name string) (*sqlm.Db, *testLog) {
	t.Helper()
	dir := t.TempDir()
	dsn := filepath.Join(dir, name+".db")
	opt, err := sqlm.NewOptionsWithServer(sqlm.Server{
		Protocol:     "sqlite",
		DSN:          dsn,
		Pretable:     "mi_",
		MaxOpenConns: 10,
		MaxIdleConns: 5,
		MaxLifetime:  int(time.Minute),
	}, name)
	require.NoError(t, err)

	lg := &testLog{}
	opt.SetLogger(lg)

	drv, err := store.NewDriver(opt)
	require.NoError(t, err)
	require.True(t, sqlm.Use(drv))

	db := sqlm.NewInstance(context.Background(), name)
	require.NotNil(t, db)
	require.NoError(t, db.Err())
	t.Cleanup(func() {
		db.Close()
	})
	return db, lg
}

// usersSchema is the table used by most of the tests (pretable aware).
const usersSchema = `CREATE TABLE IF NOT EXISTS mi_users (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL,
	age INTEGER NOT NULL DEFAULT 0,
	score REAL NOT NULL DEFAULT 0,
	mobile TEXT,
	avatar BLOB,
	intime INTEGER NOT NULL DEFAULT 0
)`

func createUsers(t testing.TB, db *sqlm.Db) {
	t.Helper()
	_, err := db.Exec(usersSchema)
	require.NoError(t, err)
}

// seedUsers inserts n users starting at intime base.
func seedUsers(t testing.TB, db *sqlm.Db, n int) {
	t.Helper()
	rows := make([][]any, 0, n)
	for i := 0; i < n; i++ {
		rows = append(rows, []any{
			fmt.Sprintf("User%03d", i),
			18 + (i % 40),
			float64(50 + (i % 50)),
			fmt.Sprintf("138%08d", i),
			[]byte("avatar"),
			int64(1700000000 + i),
		})
	}
	// Inserts returns LastInsertId (not RowsAffected) on purpose: it mirrors the
	// behaviour of every supported driver.
	id, err := db.Table("users").
		Inserts([]string{"name", "age", "score", "mobile", "avatar", "intime"}, rows)
	require.NoError(t, err)
	require.GreaterOrEqual(t, id, int64(1))
}

// mustExec runs a statement that must succeed.
func mustExec(t testing.TB, db *sqlm.Db, query string, args ...any) {
	t.Helper()
	_, err := db.Exec(query, args...)
	require.NoError(t, err)
}

// User is the entity used by the Scan() tests.
type User struct {
	Id     int64   `json:"id"`
	Name   string  `json:"name"`
	Age    int64   `json:"age"`
	Score  float64 `json:"score"`
	Mobile string  `json:"mobile"`
	Intime int64   `json:"intime"`
}

// hidden mirrors a field that must never be written by Scan().
type hidden struct {
	Id       int64  `json:"id"`
	Name     string `json:"name"`
	internal string // 未导出字段：Scan 必须跳过而不是 panic
}

// ptrUser exercises pointer fields.
type ptrUser struct {
	Id     *int64  `json:"id"`
	Name   *string `json:"name"`
	Avatar []byte  `json:"avatar"`
}

// optUser uses json tag options, they must be stripped before matching.
type optUser struct {
	Id   int64  `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
	Skip string `json:"-"`
}

// noTagUser relies on the struct field name fallback.
type noTagUser struct {
	Id   int64
	Name string
}

// ignores exercises the ignore:"io" tag.
type ignores struct {
	Id     int64  `json:"id"`
	Name   string `json:"name"`
	Secret string `json:"secret" ignore:"io"`
	Public string `json:"public" ignore:"api"`
}

// closeOnCleanup makes sure the instance is closed when the test ends, which
// also releases the sqlite file handle on Windows.
func closeOnCleanup(t testing.TB, db *sqlm.Db) *sqlm.Db {
	t.Helper()
	if db != nil {
		t.Cleanup(func() { db.Close() })
	}
	return db
}

// newDefaultSQLite registers a sqlite instance under the default key so the
// Slaver()/Major() helpers always have something to work with.
func newDefaultSQLite(t testing.TB) *sqlm.Db {
	t.Helper()
	if sqlm.Has(sqlm.DEFAULT_KEY) {
		return sqlm.NewInstance(context.Background(), sqlm.DEFAULT_KEY)
	}
	dir := t.TempDir()
	opt, err := sqlm.NewOptionsWithServer(sqlm.Server{
		Protocol:     "sqlite",
		DSN:          filepath.Join(dir, "def.db"),
		MaxOpenConns: 4,
		MaxIdleConns: 2,
		MaxLifetime:  int(time.Minute),
	})
	require.NoError(t, err)
	opt.SetLogger(&testLog{})
	drv, err := store.NewDriver(opt)
	require.NoError(t, err)
	sqlm.Use(drv)
	db := sqlm.NewInstance(context.Background(), sqlm.DEFAULT_KEY)
	require.NoError(t, db.Err())
	return db
}

// assertSQL is a small helper comparing generated SQL.
func assertSQLEqual(t testing.TB, want, got string) {
	t.Helper()
	require.Equal(t, want, normalizeSpace(got))
}

func normalizeSpace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
