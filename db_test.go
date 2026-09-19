package sqlm_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/w6xian/sqlm"
	"github.com/w6xian/sqlm/store"
)

func TestUseRegistersInstance(t *testing.T) {
	db, _ := newSQLite(t, "use_registry")
	require.NotNil(t, db)
	assert.True(t, sqlm.Has("use_registry"))
	assert.Contains(t, sqlm.Instances(), "use_registry")
}

func TestUseDuplicateNameIsIgnored(t *testing.T) {
	dir := t.TempDir()
	opt := func() *sqlm.Options {
		o, err := sqlm.NewOptionsWithServer(sqlm.Server{
			Protocol: "sqlite",
			DSN:      filepath.Join(dir, "dup.db"),
		}, "dup_name")
		require.NoError(t, err)
		o.SetLogger(&testLog{})
		return o
	}
	d1, err := store.NewDriver(opt())
	require.NoError(t, err)
	d2, err := store.NewDriver(opt())
	require.NoError(t, err)

	require.True(t, sqlm.Use(d1))
	// 重名注册不会覆盖已有实例，避免运行期连接被悄悄替换
	require.False(t, sqlm.Use(d2))

	db := closeOnCleanup(t, sqlm.NewInstance(context.Background(), "dup_name"))
	require.NoError(t, db.Err())
}

func TestNewInstanceUnknownPanics(t *testing.T) {
	assert.Panics(t, func() {
		sqlm.NewInstance(context.Background(), "instance_that_does_not_exist")
	})
}

func TestTryInstanceUnknownReturnsError(t *testing.T) {
	db, err := sqlm.TryInstance(context.Background(), "another_missing_instance")
	assert.Nil(t, db)
	assert.Error(t, err)
}

func TestResetClearsRegistry(t *testing.T) {
	newSQLite(t, "reset_before")
	require.NotEmpty(t, sqlm.Instances())

	sqlm.Reset()
	assert.Empty(t, sqlm.Instances())
	assert.False(t, sqlm.Has("reset_before"))
}

func TestDefaultKeyInstance(t *testing.T) {
	dir := t.TempDir()
	opt, err := sqlm.NewOptionsWithServer(sqlm.Server{
		Protocol: "sqlite",
		DSN:      filepath.Join(dir, "def.db"),
	})
	require.NoError(t, err)
	opt.SetLogger(&testLog{})
	drv, err := store.NewDriver(opt)
	require.NoError(t, err)
	require.True(t, sqlm.Use(drv))

	require.True(t, sqlm.Has(sqlm.DEFAULT_KEY))
	db := closeOnCleanup(t, sqlm.NewDefaultInstance(context.Background()))
	require.NoError(t, db.Err())

	mj := sqlm.Major(context.Background())
	require.NoError(t, mj.Err())
	assert.NotNil(t, sqlm.Master())
}

func TestDbTableNamePrefix(t *testing.T) {
	db, _ := newSQLite(t, "table_prefix")

	assert.Equal(t, "mi_users", db.TableName("users"))
	assert.Equal(t, "users", db.TrimPrefix("mi_users"))
	assert.Equal(t, "mi_users", db.WithPrefix("users"))
	assert.Equal(t, "mi_users", db.WithPrefix("mi_users"))
}

func TestDbExecAndPing(t *testing.T) {
	db, _ := newSQLite(t, "exec_ping")

	require.NoError(t, db.Ping())
	res, err := db.Exec("CREATE TABLE t (id INTEGER PRIMARY KEY, name TEXT)")
	require.NoError(t, err)
	require.NotNil(t, res)

	res, err = db.Exec("INSERT INTO t (name) VALUES (?)", "ok")
	require.NoError(t, err)
	id, err := res.LastInsertId()
	require.NoError(t, err)
	assert.Equal(t, int64(1), id)
}

func TestDbQueryAndQueryMulti(t *testing.T) {
	db, _ := newSQLite(t, "query")
	createUsers(t, db)
	seedUsers(t, db, 5)

	row, err := db.Query("SELECT name FROM mi_users WHERE id = ?", 1)
	require.NoError(t, err)
	assert.Equal(t, "User000", row.Get("name").String())

	rows, err := db.QueryMulti("SELECT id, name FROM mi_users ORDER BY id")
	require.NoError(t, err)
	assert.Equal(t, 5, rows.Length())

	// 无结果时保持稳定，返回 ErrNotFound
	empty, err := db.Query("SELECT name FROM mi_users WHERE id = ?", 9999)
	assert.Nil(t, empty)
	assert.ErrorIs(t, err, sqlm.ErrNotFound)

	emptyMulti, err := db.QueryMulti("SELECT name FROM mi_users WHERE id = ?", 9999)
	assert.Nil(t, emptyMulti)
	assert.ErrorIs(t, err, sqlm.ErrNotFound)
}

func TestDbRowsMustStayOpen(t *testing.T) {
	db, _ := newSQLite(t, "raw_rows")
	createUsers(t, db)
	mustExec(t, db, "INSERT INTO mi_users (name) VALUES (?)", "raw")

	rows, err := db.Rows("SELECT name FROM mi_users")
	require.NoError(t, err)
	require.NotNil(t, rows)
	defer rows.Close()

	// 入口处曾被提前关闭，这里保证调用方还能遍历
	count := 0
	for rows.Next() {
		count++
	}
	assert.Equal(t, 1, count)
}

func TestDbMaxId(t *testing.T) {
	db, _ := newSQLite(t, "max_id")
	createUsers(t, db)
	seedUsers(t, db, 3)

	max := db.MaxId("mi_users")
	require.True(t, max.Valid)
	assert.Equal(t, int64(3), max.Int64)

	// 空表时返回无效的 NullInt64，而不是崩溃
	mustExec(t, db, "DELETE FROM mi_users")
	empty := db.MaxId("mi_users")
	assert.False(t, empty.Valid)
	assert.Equal(t, int64(0), empty.Int64)
}

func TestDbConnAndClose(t *testing.T) {
	db, _ := newSQLite(t, "conn_close")
	conn, err := db.Conn()
	require.NoError(t, err)
	require.NotNil(t, conn)
	require.NoError(t, conn.Ping())

	// 重复关闭不应panic
	db.Close()
	assert.NotPanics(t, db.Close)
}

func TestDbQuerySyntaxError(t *testing.T) {
	db, _ := newSQLite(t, "syntax_error")
	row, err := db.Query("SELECT FROM WHERE NONSENSE")
	assert.Nil(t, row)
	assert.Error(t, err)
}

func TestDbWithoutConnectionReportsError(t *testing.T) {
	// 端口不可达的mysql实例：连接失败时Db仍可用，但所有操作报错
	opt, err := sqlm.NewOptionsWithServer(sqlm.Server{
		Protocol:     "mysql",
		Host:         "127.0.0.1",
		Port:         1,
		Username:     "nobody",
		Password:     "secret",
		Database:     "nothing",
		MaxOpenConns: 1,
	}, "unreachable")
	require.NoError(t, err)
	opt.SetLogger(&testLog{})
	drv, err := store.NewDriver(opt)
	require.NoError(t, err)
	require.True(t, sqlm.Use(drv))

	db := sqlm.NewInstance(context.Background(), "unreachable")
	require.NotNil(t, db)
	t.Cleanup(func() { _ = drv.Close() })
	require.Error(t, db.Err())

	_, err = db.Query("SELECT 1")
	assert.Error(t, err)
	_, err = db.Exec("SELECT 1")
	assert.Error(t, err)
	_, err = db.Conn()
	assert.Error(t, err)
	assert.Error(t, db.Ping())
	assert.NotPanics(t, db.Close)
}

func TestSlaverWithoutReplica(t *testing.T) {
	newDefaultSQLite(t)
	db := sqlm.Slaver(0)
	require.NotNil(t, db)
	assert.Error(t, db.Err())
}

func TestSlaverOutOfRange(t *testing.T) {
	newDefaultSQLite(t)
	db := sqlm.SlaverContext(context.Background(), 3)
	require.NotNil(t, db)
	assert.Error(t, db.Err())
}

func TestSlaverUsesReplicaServer(t *testing.T) {
	dir := t.TempDir()
	master := filepath.Join(dir, "master.db")
	replica := filepath.Join(dir, "replica.db")

	opt, err := sqlm.NewOptionsWithServer(sqlm.Server{
		Protocol:     "sqlite",
		DSN:          master,
		Pretable:     "mi_",
		MaxOpenConns: 4,
		MaxIdleConns: 2,
		MaxLifetime:  int(time.Minute),
	}, "replica_set")
	require.NoError(t, err)
	lg := &testLog{}
	opt.SetLogger(lg)
	opt.AddSlave(&sqlm.Server{
		Protocol:     "sqlite",
		DSN:          replica,
		Pretable:     "mi_",
		MaxOpenConns: 4,
		MaxIdleConns: 2,
		MaxLifetime:  int(time.Minute),
	})

	drv, err := store.NewDriver(opt)
	require.NoError(t, err)
	require.True(t, sqlm.Use(drv))

	db := closeOnCleanup(t, sqlm.NewInstance(context.Background(), "replica_set"))
	require.NoError(t, db.Err())
	createUsers(t, db)
	mustExec(t, db, "INSERT INTO mi_users (name, age) VALUES (?, ?)", "master-only", 1)

	slave := closeOnCleanup(t, sqlm.SlaverOf(context.Background(), "replica_set", 0))
	require.NoError(t, slave.Err())
	createUsers(t, slave)

	// 副本是独立的数据文件：主库写入的行在副本上不存在
	count, err := slave.Table("users").GetCount()
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)

	total, err := db.Table("users").GetCount()
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)

	_, err = os.Stat(replica)
	require.NoError(t, err)
}

func TestSqliteSourceKeepsQueryString(t *testing.T) {
	dir := t.TempDir()
	dsn := filepath.Join(dir, "dsn.db") + "?_pragma=x"
	opt, err := sqlm.NewOptionsWithServer(sqlm.Server{Protocol: "sqlite", DSN: dsn}, "dsn_query")
	require.NoError(t, err)
	opt.SetLogger(&testLog{})
	drv, err := store.NewDriver(opt)
	require.NoError(t, err)
	require.True(t, sqlm.Use(drv))

	db := closeOnCleanup(t, sqlm.NewInstance(context.Background(), "dsn_query"))
	require.NoError(t, db.Err())
	row, err := db.Query("SELECT 1 AS one")
	require.NoError(t, err)
	one, err := row.Get("one").Int()
	require.NoError(t, err)
	assert.Equal(t, 1, one)
}

func TestSqliteInstanceSharesPool(t *testing.T) {
	db, _ := newSQLite(t, "shared_pool")
	c1, err := db.Conn()
	require.NoError(t, err)
	c2, err := sqlm.NewInstance(context.Background(), "shared_pool").Conn()
	require.NoError(t, err)
	assert.Same(t, c1, c2)
}
