package store

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/w6xian/sqlm"
)

func newTestOptions(t *testing.T, protocol, dsn string) *sqlm.Options {
	t.Helper()
	opt, err := sqlm.NewOptionsWithServer(sqlm.Server{
		Protocol:     protocol,
		DSN:          dsn,
		MaxOpenConns: 4,
		MaxIdleConns: 2,
		MaxLifetime:  int(time.Minute),
	}, "store_case")
	require.NoError(t, err)
	opt.SetLogger(sqlm.NewNullLogger())
	return opt
}

// inserter is the batch-write surface exposed by every driver implementation.
type inserter interface {
	Insert(pTable string, columns []string, data []any) (int64, error)
	Inserts(pTable string, columns []string, data [][]any) (int64, error)
}

func sqliteMemory(t *testing.T, name string) *sqlm.Options {
	t.Helper()
	return newTestOptions(t, "sqlite", t.TempDir()+"/"+name+".db")
}

func TestNewDriverUnknownProtocol(t *testing.T) {
	_, err := NewDriver(newTestOptions(t, "oracle", "x"))
	assert.Error(t, err)
}

func TestNewDriverNilOptions(t *testing.T) {
	_, err := NewDriver(nil)
	assert.ErrorIs(t, err, sqlm.ErrNilArgument)
}

func TestNewDriverMissingServer(t *testing.T) {
	_, err := NewMysql(nil)
	assert.Error(t, err)
	_, err = NewSqlite(nil)
	assert.Error(t, err)
	_, err = NewPostgres(nil)
	assert.Error(t, err)

	_, err = NewMysql(&sqlm.Options{})
	assert.Error(t, err)
	_, err = NewSqlite(&sqlm.Options{})
	assert.Error(t, err)
	_, err = NewPostgres(&sqlm.Options{})
	assert.Error(t, err)
}

func TestNewDriverPostgresProtocol(t *testing.T) {
	for _, protocol := range []string{"postgres", "pg"} {
		drv, err := NewDriver(newTestOptions(t, protocol, "postgres://u:p@127.0.0.1:5432/db"))
		require.NoError(t, err, protocol)
		assert.IsType(t, &Postgres{}, drv, protocol)
	}
}

func TestPostgresSource(t *testing.T) {
	dsn, err := postgresSource(&sqlm.Server{
		Host:     "db.local",
		Port:     5432,
		Username: "app",
		Password: "pwd",
		Database: "cloud",
		Charset:  "UTF8",
	})
	require.NoError(t, err)
	assert.Equal(t, "host=db.local port=5432 user=app password=pwd dbname=cloud sslmode=disable client_encoding=UTF8", dsn)

	// unix socket: libpq 需要 socket 所在目录
	dsn, err = postgresSource(&sqlm.Server{Host: "unix:/var/run/postgresql", Database: "cloud"})
	require.NoError(t, err)
	assert.Equal(t, "host=/var/run/postgresql user= password= dbname=cloud sslmode=disable", dsn)

	// 缺省 host/port 回落到本机默认端口
	dsn, err = postgresSource(&sqlm.Server{})
	require.NoError(t, err)
	assert.Equal(t, "host=127.0.0.1 port=5432 user= password= dbname= sslmode=disable", dsn)

	// 显式 DSN 优先
	dsn, err = postgresSource(&sqlm.Server{DSN: "postgres://localhost/db?sslmode=require"})
	require.NoError(t, err)
	assert.Equal(t, "postgres://localhost/db?sslmode=require", dsn)

	_, err = postgresSource(nil)
	assert.Error(t, err)
}

func TestPostgresConnectWithoutDriver(t *testing.T) {
	// 未注册 postgres 驱动时 Connect 必须报错而不是 panic
	drv, err := NewPostgres(newTestOptions(t, "pgx", ""))
	require.NoError(t, err)
	_, err = drv.Connect(context.Background())
	assert.Error(t, err)
}

func TestPostgresHelpersRequireConnection(t *testing.T) {
	drv, err := NewPostgres(newTestOptions(t, "postgres", "postgres://u:p@127.0.0.1:5432/db"))
	require.NoError(t, err)

	assert.Error(t, drv.Ping())
	assert.Error(t, drv.Close())
	_, err = drv.Conn()
	assert.Error(t, err)
	_, err = drv.Query("SELECT 1")
	assert.Error(t, err)
	_, err = drv.Delete("DELETE FROM t")
	assert.Error(t, err)
	_, err = drv.Prepare("SELECT 1")
	assert.Error(t, err)
	_, err = drv.Exec("SELECT 1")
	assert.Error(t, err)
	assert.Error(t, drv.check())

	_, err = drv.Insert("t", []string{"a", "b"}, []any{1})
	assert.Error(t, err)
	_, err = drv.Insert("t", nil, nil)
	assert.Error(t, err)
	_, err = drv.Inserts("t", []string{"a"}, [][]any{{1}})
	assert.Error(t, err)

	assert.NotNil(t, drv.Options())
	assert.Equal(t, "postgres", drv.Conf().Protocol)
	assert.NotPanics(t, func() { drv.WithContext(nil) })
}

func TestPostgresInsertSQL(t *testing.T) {
	sql := postgresInsertSQL("mi_users", []string{`"name"`, "age"}, 2)
	assert.Equal(t, `INSERT INTO "mi_users" ("name","age") VALUES ($1,$2),($3,$4)`, sql)

	assert.Equal(t, `INSERT INTO "a" ("b") VALUES ($1)`, postgresInsertSQL(`"a"`, []string{"b"}, 1))
}

func TestDriverConnectAndReuse(t *testing.T) {
	opt := sqliteMemory(t, "reuse")
	drv, err := NewDriver(opt)
	require.NoError(t, err)

	conn, err := drv.Connect(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	assert.True(t, strings.Contains(drv.Conf().DSN, "reuse.db"))
	assert.NotNil(t, drv.Options())

	// 第二次Connect复用连接池
	again, err := drv.Connect(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, again)

	require.NoError(t, again.Ping())
	c, err := again.Conn()
	require.NoError(t, err)
	require.NotNil(t, c)
}

func TestDriverHelpersRequireConnection(t *testing.T) {
	drv, err := NewSqlite(newTestOptions(t, "sqlite", "unused.db"))
	require.NoError(t, err)

	assert.Error(t, drv.Ping())
	_, err = drv.Conn()
	assert.Error(t, err)
	assert.Error(t, drv.Close())
	_, err = drv.Query("SELECT 1")
	assert.Error(t, err)
	_, err = drv.Delete("DELETE FROM t")
	assert.Error(t, err)
	_, err = drv.Prepare("SELECT 1")
	assert.Error(t, err)
	_, err = drv.Exec("SELECT 1")
	assert.Error(t, err)
	_, err = drv.Insert("t", []string{"a"}, []any{1})
	assert.Error(t, err)
	_, err = drv.Inserts("t", []string{"a"}, [][]any{{1}})
	assert.Error(t, err)

	// 未连接时 check() 必须报错，driver 实例本身也不可用
	assert.Error(t, drv.check())
}

func TestSqliteCRUD(t *testing.T) {
	opt := sqliteMemory(t, "crud")
	drv, err := NewDriver(opt)
	require.NoError(t, err)
	conn, err := drv.Connect(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	_, err = conn.Exec("CREATE TABLE IF NOT EXISTS mi_users (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT, age INTEGER)")
	require.NoError(t, err)

	batch, ok := conn.(inserter)
	require.True(t, ok, "driver must expose batch insert helpers")

	id, err := batch.Insert("mi_users", []string{"name", "age"}, []any{"A", 1})
	require.NoError(t, err)
	assert.Equal(t, int64(1), id)

	lastID, err := batch.Inserts("mi_users", []string{"name", "age"}, [][]any{{"B", 2}, {"C", 3}})
	require.NoError(t, err)
	assert.Equal(t, int64(3), lastID)

	rows, err := conn.Query("SELECT COUNT(*) AS total FROM mi_users")
	require.NoError(t, err)
	require.True(t, rows.Next())
	var total int
	require.NoError(t, rows.Scan(&total))
	require.NoError(t, rows.Close())
	assert.Equal(t, 3, total)

	res, err := conn.Exec("UPDATE mi_users SET age = ? WHERE name = ?", 10, "A")
	require.NoError(t, err)
	affected, err := res.RowsAffected()
	require.NoError(t, err)
	assert.Equal(t, int64(1), affected)
}

func TestSqliteInsertValidation(t *testing.T) {
	opt := sqliteMemory(t, "validation")
	drv, err := NewDriver(opt)
	require.NoError(t, err)
	conn, err := drv.Connect(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	_, err = conn.Exec("CREATE TABLE IF NOT EXISTS mi_users (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT)")
	require.NoError(t, err)

	batch := conn.(inserter)

	_, err = batch.Insert("mi_users", []string{"name", "age"}, []any{"A"})
	assert.Error(t, err)
	_, err = batch.Inserts("mi_users", []string{"name"}, [][]any{{"A", 1}})
	assert.Error(t, err)
	_, err = batch.Inserts("mi_users", []string{"name"}, nil)
	assert.Error(t, err)
	_, err = batch.Inserts("mi_users", nil, [][]any{{"A"}})
	assert.Error(t, err)
}

func TestNewServerConn(t *testing.T) {
	opt := sqliteMemory(t, "switcher")
	drv, err := NewDriver(opt)
	require.NoError(t, err)
	master, err := drv.Connect(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() { _ = master.Close() })

	sw, ok := drv.(sqlm.ServerSwitcher)
	require.True(t, ok, "driver must implement ServerSwitcher")

	_, err = sw.NewServerConn(context.Background(), nil)
	assert.Error(t, err)

	replica, err := sw.NewServerConn(context.Background(), &sqlm.Server{
		Protocol: "sqlite",
		DSN:      t.TempDir() + "/replica.db",
	})
	require.NoError(t, err)
	require.NotNil(t, replica)
	t.Cleanup(func() { _ = replica.Close() })
}

func TestSqliteSource(t *testing.T) {
	assert.Contains(t, sqliteSource("/tmp/a.db"), "?_pragma=foreign_keys(0)")
	assert.Contains(t, sqliteSource("file:memdb1?mode=memory"), "&_pragma=foreign_keys(0)")
	assert.Equal(t, 1, strings.Count(sqliteSource("file:x?y=1"), "?"))
}

func TestMysqlSource(t *testing.T) {
	tcp := mysqlSource(&sqlm.Server{Host: "db.local", Port: 3306, Username: "root", Password: "pwd", Database: "cloud", Charset: "utf8mb4"})
	assert.Equal(t, "root:pwd@tcp(db.local:3306)/cloud?charset=utf8mb4", tcp)

	sock := mysqlSource(&sqlm.Server{Host: "unix:/var/run/mysqld/mysqld.sock", Username: "root", Password: "pwd", Database: "cloud", Charset: "utf8"})
	assert.Equal(t, "root:pwd@unix(/var/run/mysqld/mysqld.sock)/cloud?charset=utf8", sock)
}

// newOpenedConn opens a throwaway sqlite pool used to inspect pool settings.
func newOpenedConn(t *testing.T) *sql.DB {
	t.Helper()
	conn, err := sql.Open("sqlite", sqliteSource(t.TempDir()+"/pool.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func TestApplyPoolDefaults(t *testing.T) {
	conn := newOpenedConn(t)
	// 未配置时使用库内置默认值，避免无上限的连接池
	applyPool(conn, &sqlm.Server{})
	assert.Greater(t, defaultMaxOpenConns, 0)
	assert.Greater(t, defaultMaxIdleConns, 0)

	applyPool(conn, &sqlm.Server{MaxOpenConns: 10, MaxIdleConns: 3, MaxLifetime: int(time.Minute)})
	assert.Equal(t, 10, conn.Stats().MaxOpenConnections)

	// 空闲数大于最大连接数时收敛，否则database/sql会报错
	applyPool(conn, &sqlm.Server{MaxOpenConns: 2, MaxIdleConns: 50})
	assert.Equal(t, 2, conn.Stats().MaxOpenConnections)

	assert.NotPanics(t, func() { applyPool(conn, nil) })
	assert.NotPanics(t, func() { applyPool(nil, &sqlm.Server{}) })
}

func TestInsertSQL(t *testing.T) {
	sql := insertSQL("mi_users", []string{"`name`", "age"}, 2)
	assert.Equal(t, "INSERT INTO `mi_users` (`name`,`age`) VALUES (?,?),(?,?)", sql)

	assert.Equal(t, "INSERT INTO `a` (`b`) VALUES (?)", insertSQL("`a`", []string{"b"}, 1))
}

func TestFlattenValues(t *testing.T) {
	val, err := flattenValues(2, [][]any{{1, 2}, {3, 4}})
	require.NoError(t, err)
	assert.Equal(t, []any{1, 2, 3, 4}, val)

	_, err = flattenValues(0, [][]any{{1}})
	assert.Error(t, err)
	_, err = flattenValues(1, nil)
	assert.Error(t, err)
	_, err = flattenValues(2, [][]any{{1}})
	assert.Error(t, err)
}

func TestQuoteColumns(t *testing.T) {
	assert.Equal(t, []string{"`a`", "`b`"}, quoteColumns([]string{"a", "`b`"}))
}

func TestCheckConnectionAndContext(t *testing.T) {
	assert.Error(t, checkConnection(nil))

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	assert.NotPanics(t, func() { ctxOrBackground(cancelled) })
	assert.NotNil(t, ctxOrBackground(nil))
	assert.NotNil(t, ctxOrBackground(context.Background()))
}

func TestMysqlConnectFailure(t *testing.T) {
	drv, err := NewMysql(newTestOptions(t, "mysql", ""))
	require.NoError(t, err)
	drv.Conf().Host = "127.0.0.1"
	drv.Conf().Port = 1

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	_, err = drv.Connect(ctx)
	assert.Error(t, err)
}
