package sqlm_test

// UseMaster：语句级把语句打回主库，并可选 schema / 前缀覆盖。
//
// 要钉住的行为：真的路由到了主库（而不是悄悄用从库）、schema 与前缀
// 只影响这一条语句、事务内拒绝、缺主连接报错而不是静默回退。

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/w6xian/sqlm"
	"github.com/w6xian/sqlm/store"
)

// newSQLiteWithSlave 注册一个"主库 + 一个从库"的实例，两个不同的 sqlite 文件。
func newSQLiteWithSlave(t testing.TB, name string) (master, slave *sqlm.Db) {
	t.Helper()
	dir := t.TempDir()
	mk := func(dsn string) sqlm.Server {
		return sqlm.Server{
			Protocol:     "sqlite",
			DSN:          dsn,
			Pretable:     "mi_",
			MaxOpenConns: 4,
			MaxIdleConns: 2,
			MaxLifetime:  int(time.Minute),
		}
	}
	opt, err := sqlm.NewOptionsWithServer(mk(filepath.Join(dir, name+"-master.db")), name)
	require.NoError(t, err)
	slaveSvr := mk(filepath.Join(dir, name+"-slave.db"))
	opt.AddSlave(&slaveSvr)

	drv, err := store.NewDriver(opt)
	require.NoError(t, err)
	require.True(t, sqlm.Use(drv))

	master = sqlm.NewInstance(context.Background(), name)
	require.NoError(t, master.Err())
	slave = sqlm.SlaverOf(context.Background(), name)
	require.NoError(t, slave.Err())
	t.Cleanup(func() {
		master.Close()
		slave.Close()
	})
	return master, slave
}

// 同一条语句，加不加 UseMaster 结果不同：数据只在主库，说明真的切了连接。
func TestUseMasterRoutesToPrimary(t *testing.T) {
	master, slave := newSQLiteWithSlave(t, "um_route")
	createUsers(t, master)
	seedUsers(t, master, 2)

	// 从库文件里没有这张表
	_, err := slave.Table("users").QueryMulti()
	require.Error(t, err, "从库还没有这份数据")

	rows, err := slave.Table("users").UseMaster("").QueryMulti()
	require.NoError(t, err)
	require.Equal(t, 2, rows.Length())

	// 只影响这一条语句：下一条不带 UseMaster 的仍走从库
	_, err = slave.Table("users").QueryMulti()
	require.Error(t, err)
}

// schema 限定 + 前缀覆盖拼出 public.mi_com_regions_2026，且只作用于本条语句。
func TestUseMasterSchemaAndPrefix(t *testing.T) {
	master, _ := newSQLiteWithSlave(t, "um_schema")

	sql := master.Table("com_regions_2026").
		UseMaster("public", sqlm.UseTablePrefix("mi_")).
		Select("*").Where("id = %d", 1).SQL()
	assertSQLEqual(t, "SELECT * FROM public.mi_com_regions_2026 WHERE id = 1", sql)

	// 不传 UseTablePrefix：沿用实例的 Pretable
	sql = master.Table("com_regions_2026").UseMaster("public").SQL()
	assertSQLEqual(t, "SELECT * FROM public.mi_com_regions_2026", sql)

	// 空 schema：只切连接，不改表名
	sql = master.Table("regions").UseMaster("").SQL()
	assertSQLEqual(t, "SELECT * FROM mi_regions", sql)

	// 不带 UseMaster 的语句完全不受影响
	assertSQLEqual(t, "SELECT * FROM mi_regions", master.Table("regions").SQL())
}

// 写路径的表名带 schema 时必须按段加引号：整个 "public.mi_users" 包进一对
// 引号就成了一个不存在的标识符。sqlite 没有 public schema，语句会执行失败，
// 但钩子拿到的完整 SQL 能验证引号是拆开的。
func TestUseMasterWritePathQuotesSchemaByPart(t *testing.T) {
	master, _ := newSQLiteWithSlave(t, "um_quote")
	createUsers(t, master)

	var mu sync.Mutex
	var sqls []string
	master.SetHooks(sqlm.NewSQLHook(0, func(ctx context.Context, info *sqlm.StmtInfo) {
		mu.Lock()
		defer mu.Unlock()
		sqls = append(sqls, info.SQL)
	}))

	// 前缀覆盖走写路径：正常执行成功
	_, err := master.Table("users").
		UseMaster("", sqlm.UseTablePrefix("mi_")).
		Insert(map[string]any{"name": "bob", "age": 20, "intime": 1})
	require.NoError(t, err)

	// schema 限定：sqlite 报错（没有 public），但语句按段引号化
	_, err = master.Table("users").UseMaster("public").
		Insert(map[string]any{"name": "bob", "age": 20, "intime": 1})
	require.Error(t, err)

	mu.Lock()
	defer mu.Unlock()
	require.NotEmpty(t, sqls)
	require.Contains(t, sqls[0], `INSERT INTO "mi_users"`)
	require.Contains(t, sqls[len(sqls)-1], `INSERT INTO "public"."mi_users"`)
}

// 事务内拒绝 UseMaster：连接一切换，语句就逃逸出事务了。
func TestUseMasterRefusedInTransaction(t *testing.T) {
	master, _ := newSQLiteWithSlave(t, "um_tx")
	createUsers(t, master)
	seedUsers(t, master, 1)

	_, err := master.Action(func(tx sqlm.ITable, args ...any) (int64, error) {
		_, e := tx.Table("users").UseMaster("").QueryMulti()
		return 0, e
	})
	require.ErrorIs(t, err, sqlm.ErrInTransaction)

	// 事务本身照常可用
	_, err = master.Action(func(tx sqlm.ITable, args ...any) (int64, error) {
		return tx.Table("users").Insert(map[string]any{"name": "tx", "age": 9, "intime": 1})
	})
	require.NoError(t, err)
}

// 没有实例（Tbx 裸建的语句）解析不到主连接：宁可失败也不静默回退。
func TestUseMasterRequiresInstance(t *testing.T) {
	_, err := sqlm.Tbx(context.Background(), "users").UseMaster("").Query()
	require.ErrorIs(t, err, sqlm.ErrNoMaster)
}
