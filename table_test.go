package sqlm_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/w6xian/sqlm"
	"github.com/w6xian/sqlm/store"
)

// ---------------------------------------------------------------------------
// SQL building (no statement executed)
// ---------------------------------------------------------------------------

func TestBuildSelectDefault(t *testing.T) {
	db, _ := newSQLite(t, "build_default")
	tb := db.Table("users")
	assert.Equal(t, "SELECT * FROM mi_users", tb.SQL())
}

func TestBuildSelectJoinWhereOrderLimit(t *testing.T) {
	db, _ := newSQLite(t, "build_full")
	sql := db.Table("users").
		SetProtocol(sqlm.MYSQL).
		Select("id", "name").
		LeftJoin("scores", "scores.user_id = users.id").
		InnerJoin("shops", "shops.id = users.shop_id").
		RightJoin("marks", "marks.user_id = users.id").
		Where("age > %d", 18).
		And("score >= %f", 60.0).
		Or("vip = %d", 1).
		GroupBy("shop_id").
		Order("name").
		Desc("id").
		Limit(0, 10).
		Lock().
		SQL()

	assertSQLEqual(t, "SELECT id,name FROM mi_users "+
		"LEFT JOIN mi_scores ON scores.user_id = users.id "+
		"INNER JOIN mi_shops ON shops.id = users.shop_id "+
		"RIGHT JOIN mi_marks ON marks.user_id = users.id "+
		"WHERE age > 18 AND score >= 60.000000 OR vip = 1 "+
		"GROUP BY shop_id ORDER BY name ASC,id DESC LIMIT 0,10 FOR UPDATE", sql)
}

func TestBuildWhereTrimsLeadingKeyword(t *testing.T) {
	db, _ := newSQLite(t, "build_trim")
	assert.Equal(t, "SELECT * FROM mi_users WHERE id = 1 AND name = 'a'",
		db.Table("users").And("id = 1").And("name = 'a'").SQL())
	// 条件之间是接在末尾的，只有最前面的连接词会被去掉
	assert.Equal(t, "SELECT * FROM mi_users WHERE id = 1 AND name = 'a'",
		db.Table("users").Or("id = 1").And("name = 'a'").SQL())
}

func TestBuildCount(t *testing.T) {
	db, _ := newSQLite(t, "build_count")
	assert.Equal(t, "SELECT count(*) as total FROM mi_users", db.Table("users").Count().SQL())
}

func TestBuildDatabasePrefixedTable(t *testing.T) {
	db, _ := newSQLite(t, "build_dbtable")
	// "db.table" 只取表名部分并补前缀
	assert.Equal(t, "SELECT * FROM mi_users", db.Table("cloud.users").SQL())
}

func TestBuildSelectVariants(t *testing.T) {
	db, _ := newSQLite(t, "build_select")
	assert.Equal(t, "SELECT * FROM mi_users", db.Table("users").Select().SQL())
	assert.Equal(t, "SELECT u.id,u.name FROM mi_users", db.Table("users").SelectWithAlias("u", "id", "name").SQL())
	assert.Equal(t, "SELECT a,b FROM mi_users", db.Table("users").SelectMulti("a", "b").SQL())
	assert.Equal(t, "SELECT id FROM mi_users", db.Table("users").SelectOption(true, "id").SQL())
	assert.Equal(t, "SELECT * FROM mi_users", db.Table("users").SelectOption(false, "id").SQL())
	assert.Equal(t, "SELECT * FROM mi_users", db.Table("users").From("users").SQL())
}

func TestBuildOrderHelpers(t *testing.T) {
	db, _ := newSQLite(t, "build_order")
	// ASC 列总是先输出
	assert.Equal(t, "SELECT * FROM mi_users ORDER BY name ASC,id DESC",
		db.Table("users").Desc("id").Asc("name").SQL())
	assert.Equal(t, "SELECT * FROM mi_users ORDER BY id DESC",
		db.Table("users").DescOption(true, "id").SQL())
	assert.Equal(t, "SELECT * FROM mi_users ORDER BY name ASC",
		db.Table("users").AscOption(true, "name").SQL())
	assert.Equal(t, "SELECT * FROM mi_users ORDER BY id ASC",
		db.Table("users").OrderOption(true, "id", "asc").SQL())
	assert.Equal(t, "SELECT * FROM mi_users ORDER BY id DESC",
		db.Table("users").OrderOption(true, "id", "DESC").SQL())
	assert.Equal(t, "SELECT * FROM mi_users",
		db.Table("users").OrderOption(true, "", "desc").SQL())
	assert.Equal(t, "SELECT * FROM mi_users",
		db.Table("users").OrderOption(false, "id", "desc").SQL())
}

func TestBuildBetweenHelpers(t *testing.T) {
	db, _ := newSQLite(t, "build_between")
	assert.Equal(t, "SELECT * FROM mi_users WHERE intime BETWEEN 1 AND 2",
		db.Table("users").AndBetween(1, 2).SQL())
	assert.Equal(t, "SELECT * FROM mi_users WHERE age BETWEEN 1 AND 2",
		db.Table("users").AndBetweenOption(true, 1, 2, "age").SQL())
	assert.Equal(t, "SELECT * FROM mi_users",
		db.Table("users").AndBetweenOption(false, 1, 2, "age").SQL())
	assert.Equal(t, "SELECT * FROM mi_users WHERE a = 1 AND b = 2",
		db.Table("users").Ands([]string{"a = 1", "b = 2"}).SQL())
	assert.Equal(t, "SELECT * FROM mi_users WHERE name = 'a'",
		db.Table("users").AndOption(true, "name = '%s'", "a").SQL())
	assert.Equal(t, "SELECT * FROM mi_users WHERE x = 1",
		db.Table("users").WhereOption(true, "x = %d", 1).SQL())
}

func TestBuildLimitMySQLSemantics(t *testing.T) {
	db, _ := newSQLite(t, "build_limit")
	limit := func() *sqlm.Table { return db.Table("users").SetProtocol(sqlm.MYSQL) }

	assert.Equal(t, "SELECT * FROM mi_users LIMIT 10", limit().Limit(10).SQL())
	assert.Equal(t, "SELECT * FROM mi_users LIMIT 20,10", limit().Limit(20, 10).SQL())
	assert.Equal(t, "SELECT * FROM mi_users LIMIT 5", limit().LimitOption(true, 5).SQL())
	assert.Equal(t, "SELECT * FROM mi_users", limit().LimitOption(false, 5).SQL())
	assert.Equal(t, "SELECT * FROM mi_users LIMIT 10", limit().LimitOffset(10).SQL())
	// MySQLLimitOffset：第二个参数是偏移量，内部换算成页码
	assert.Equal(t, "SELECT * FROM mi_users LIMIT 2,10", limit().LimitOffset(10, 20).SQL())
	assert.Equal(t, "SELECT * FROM mi_users", limit().LimitOffsetOption(false, 10).SQL())
}

func TestBuildLimitSQLiteSemantics(t *testing.T) {
	db, _ := newSQLite(t, "build_limit_sqlite")
	limit := func() *sqlm.Table { return db.Table("users").SetProtocol(sqlm.SQLITE) }

	assert.Equal(t, "SELECT * FROM mi_users LIMIT 10", limit().Limit(10).SQL())
	assert.Equal(t, "SELECT * FROM mi_users LIMIT 10 OFFSET 200", limit().Limit(20, 10).SQL())
	assert.Equal(t, "SELECT * FROM mi_users LIMIT 10", limit().LimitOffset(10).SQL())
	assert.Equal(t, "SELECT * FROM mi_users LIMIT 10 OFFSET 20", limit().LimitOffset(10, 20).SQL())
	assert.Equal(t, "SELECT * FROM mi_users", limit().LimitOption(false, 10).SQL())
}

func TestBuildTracesSQLToLogger(t *testing.T) {
	db, lg := newSQLite(t, "build_trace")
	_ = db.Table("users").Where("id = %d", 7).SQL()
	assert.Equal(t, "SELECT * FROM mi_users WHERE id = 7", lg.LastSQL())
}

// ---------------------------------------------------------------------------
// AndFilters
// ---------------------------------------------------------------------------

func TestAndFiltersSortsKeysAndEscapesValues(t *testing.T) {
	db, _ := newSQLite(t, "filters_order")
	sql := db.Table("users").AndFilters(map[string]any{
		"name":  "Bob",
		"age":   30,
		"score": 9.5,
		"vip":   true,
	}).SQL()
	assertSQLEqual(t, "SELECT * FROM mi_users WHERE age = 30 AND name = 'Bob' AND score = 9.5 AND vip = 1", sql)

	// 同一份map必须生成同样的SQL
	for i := 0; i < 20; i++ {
		assert.Equal(t, sql, db.Table("users").AndFilters(map[string]any{
			"name":  "Bob",
			"age":   30,
			"score": 9.5,
			"vip":   true,
		}).SQL())
	}
}

func TestAndFiltersInAndAlias(t *testing.T) {
	db, _ := newSQLite(t, "filters_in")
	assertSQLEqual(t, "SELECT * FROM mi_users WHERE u.age IN (18,20,30)",
		db.Table("users").AndFilters(map[string]any{"age": []int{18, 20, 30}}, "u").SQL())
	assertSQLEqual(t, "SELECT * FROM mi_users WHERE name IN ('a','b')",
		db.Table("users").AndFilters(map[string]any{"name": []any{"a", "b"}}).SQL())
	assertSQLEqual(t, "SELECT * FROM mi_users WHERE name = 'a'",
		db.Table("users").AndFilters(map[string]any{"name": []string{"a"}}).SQL())
	assertSQLEqual(t, "SELECT * FROM mi_users WHERE name = NULL",
		db.Table("users").AndFilters(map[string]any{"name": []any{nil}}).SQL())
}

func TestAndFiltersSkipsUnsupportedValues(t *testing.T) {
	db, _ := newSQLite(t, "filters_skip")
	assertSQLEqual(t, "SELECT * FROM mi_users WHERE age = 1",
		db.Table("users").AndFilters(map[string]any{
			"age":     1,
			"$skip":   "ignored",
			"nothing": nil,
			"weird":   struct{ A int }{A: 1},
			"empty":   []int{},
		}).SQL())
}

func TestAndFiltersEscapesInjection(t *testing.T) {
	db, _ := newSQLite(t, "filters_injection")
	createUsers(t, db)
	id, err := db.Table("users").Insert(map[string]any{"name": "Alice", "age": 20})
	require.NoError(t, err)
	assert.Equal(t, int64(1), id)

	// 注入串只是普通字符串，不会改写条件
	rows, err := db.Table("users").AndFilters(map[string]any{"name": "' OR '1'='1"}).QueryMulti()
	assert.ErrorIs(t, err, sqlm.ErrNotFound)
	assert.Nil(t, rows)

	rows, err = db.Table("users").AndFilters(map[string]any{"name": "Alice"}).QueryMulti()
	require.NoError(t, err)
	assert.Equal(t, 1, rows.Length())
}

func TestAndFiltersTimeAndBytes(t *testing.T) {
	db, _ := newSQLite(t, "filters_time")
	ts := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	assertSQLEqual(t, "SELECT * FROM mi_users WHERE intime = '2024-01-02 03:04:05'",
		db.Table("users").AndFilters(map[string]any{"intime": ts}).SQL())
	assertSQLEqual(t, "SELECT * FROM mi_users WHERE avatar = 'abc'",
		db.Table("users").AndFilters(map[string]any{"avatar": []byte("abc")}).SQL())
}

func TestAndSearchOption(t *testing.T) {
	db, _ := newSQLite(t, "search_option")
	assertSQLEqual(t, "SELECT * FROM mi_users WHERE name LIKE '%bob%'",
		db.Table("users").AndSearchOption(true, "name", "bob").SQL())
	assertSQLEqual(t, "SELECT * FROM mi_users WHERE u.intime BETWEEN 1 AND 9",
		db.Table("users").AndSearchOption(true, "intime", "[1,9]", "u.").SQL())
	assertSQLEqual(t, "SELECT * FROM mi_users WHERE intime>=1",
		db.Table("users").AndSearchOption(true, "intime", "[1,]").SQL())
	assertSQLEqual(t, "SELECT * FROM mi_users WHERE intime<=9",
		db.Table("users").AndSearchOption(true, "intime", "[,9]").SQL())
	assertSQLEqual(t, "SELECT * FROM mi_users WHERE intime LIKE '%[bad]%'",
		db.Table("users").AndSearchOption(true, "intime", "[bad]").SQL())
	assertSQLEqual(t, "SELECT * FROM mi_users",
		db.Table("users").AndSearchOption(false, "name", "bob").SQL())
	assertSQLEqual(t, "SELECT * FROM mi_users",
		db.Table("users").AndSearchOption(true, "", "bob").SQL())
	assertSQLEqual(t, "SELECT * FROM mi_users",
		db.Table("users").AndSearchOption(true, "$ignore", "bob").SQL())
}

// ---------------------------------------------------------------------------
// write operations
// ---------------------------------------------------------------------------

func TestInsertAndQueryBack(t *testing.T) {
	db, _ := newSQLite(t, "crud_insert")
	createUsers(t, db)

	id, err := db.Table("users").Insert(map[string]any{
		"name": "Alice", "age": 30, "score": 95.5, "intime": 1,
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), id)

	row, err := db.Table("users").Select("*").AndFilters(map[string]any{"id": id}).Query()
	require.NoError(t, err)
	assert.Equal(t, "Alice", row.Get("name").String())
	age, err := row.Get("age").Int()
	require.NoError(t, err)
	assert.Equal(t, 30, age)
	score, err := row.Get("score").Float64()
	require.NoError(t, err)
	assert.InDelta(t, 95.5, score, 0.0001)
}

func TestInsertEmptyMap(t *testing.T) {
	db, _ := newSQLite(t, "crud_insert_empty")
	createUsers(t, db)
	_, err := db.Table("users").Insert(map[string]any{})
	assert.ErrorIs(t, err, sqlm.ErrMissingValues)
}

func TestInsertsBatchAndValidation(t *testing.T) {
	db, _ := newSQLite(t, "crud_inserts")
	createUsers(t, db)

	id, err := db.Table("users").Inserts(
		[]string{"name", "age"},
		[][]any{{"A", 1}, {"B", 2}, {"C", 3}},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(3), id)

	count, err := db.Table("users").GetCount()
	require.NoError(t, err)
	assert.Equal(t, int64(3), count)

	// 多行数据必须全部写入，而不是只有第一行
	rows, err := db.Table("users").Select("name").Order("id").QueryMulti()
	require.NoError(t, err)
	names := []string{}
	for rows.Next() != nil {
		names = append(names, rows.Get("name").String())
	}
	sort.Strings(names)
	assert.Equal(t, []string{"A", "B", "C"}, names)

	_, err = db.Table("users").Inserts([]string{"name"}, nil)
	assert.ErrorIs(t, err, sqlm.ErrMissingValues)
	_, err = db.Table("users").Inserts(nil, [][]any{{"A"}})
	assert.Error(t, err)
	_, err = db.Table("users").Inserts([]string{"name", "age"}, [][]any{{"A"}})
	assert.ErrorIs(t, err, sqlm.ErrColumnsNotMatched)
}

func TestUpdateSetAndDelete(t *testing.T) {
	db, _ := newSQLite(t, "crud_update")
	createUsers(t, db)
	for _, u := range []struct {
		name string
		age  int
	}{{"U1", 10}, {"U2", 20}, {"U3", 30}} {
		_, err := db.Table("users").Insert(map[string]any{"name": u.name, "age": u.age})
		require.NoError(t, err)
	}

	affected, err := db.Table("users").
		Update(map[string]any{"age": 99, "name": "O'Reilly"}).
		AndFilters(map[string]any{"id": 1}).
		Execute()
	require.NoError(t, err)
	assert.Equal(t, int64(1), affected)

	row, err := db.Table("users").AndFilters(map[string]any{"id": 1}).Query()
	require.NoError(t, err)
	assert.Equal(t, "O'Reilly", row.Get("name").String())

	// Set 走原生表达式
	affected, err = db.Table("users").Set("age = age + 1").AndFilters(map[string]any{"id": 2}).Execute()
	require.NoError(t, err)
	assert.Equal(t, int64(1), affected)
	row, err = db.Table("users").AndFilters(map[string]any{"id": 2}).Query()
	require.NoError(t, err)
	assert.Equal(t, int64(21), row.Get("age").NullInt64().Int64)

	affected, err = db.Table("users").Delete().AndFilters(map[string]any{"id": 3}).Execute()
	require.NoError(t, err)
	assert.Equal(t, int64(1), affected)

	count, err := db.Table("users").GetCount()
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)
}

func TestSetOptionAndExecuteErrors(t *testing.T) {
	db, _ := newSQLite(t, "crud_errors")
	createUsers(t, db)
	_, err := db.Table("users").Insert(map[string]any{"name": "A", "age": 1})
	require.NoError(t, err)

	assertSQLEqual(t, "SELECT * FROM mi_users WHERE id = 1",
		db.Table("users").SetOption(false, "age = 5").Where("id = %d", 1).SQL())

	// 没有Where的写操作必须被拒绝，而不是panic
	_, err = db.Table("users").Update(map[string]any{"age": 1}).Execute()
	assert.ErrorIs(t, err, sqlm.ErrMissingWhere)

	_, err = db.Table("users").Delete().Execute()
	assert.ErrorIs(t, err, sqlm.ErrMissingWhere)

	// 没有可执行的操作
	_, err = db.Table("users").Execute()
	assert.ErrorIs(t, err, sqlm.ErrMissingOperation)

	// Set 之后没有值
	_, err = db.Table("users").Where("id = 1").Execute()
	assert.ErrorIs(t, err, sqlm.ErrMissingOperation)
}

func TestQueryWithoutTableOrConnection(t *testing.T) {
	// 未绑定连接的builder：报错而不是panic
	tb := sqlm.Tb("users")
	_, err := tb.Query()
	assert.ErrorIs(t, err, sqlm.ErrNoConnection)
	_, err = tb.QueryMulti()
	assert.ErrorIs(t, err, sqlm.ErrNoConnection)
	_, err = tb.Rows()
	assert.ErrorIs(t, err, sqlm.ErrNoConnection)
	_, err = tb.Insert(map[string]any{"a": 1})
	assert.ErrorIs(t, err, sqlm.ErrNoConnection)
	_, err = tb.Inserts([]string{"a"}, [][]any{{1}})
	assert.ErrorIs(t, err, sqlm.ErrNoConnection)
	_, err = tb.Update(map[string]any{"a": 1}).Where("id = 1").Execute()
	assert.ErrorIs(t, err, sqlm.ErrNoConnection)
	err = tb.Scan(&User{})
	assert.ErrorIs(t, err, sqlm.ErrNoConnection)
	err = tb.ScanMulti(&[]User{})
	assert.ErrorIs(t, err, sqlm.ErrNoConnection)
	_, err = tb.GetCount()
	assert.ErrorIs(t, err, sqlm.ErrNoConnection)

	// 空表名
	db, _ := newSQLite(t, "empty_table")
	_, err = db.Table("").Query()
	assert.ErrorIs(t, err, sqlm.ErrEmptyTableName)
}

func TestTableRowsStaysOpen(t *testing.T) {
	db, _ := newSQLite(t, "table_rows_open")
	createUsers(t, db)
	seedUsers(t, db, 2)

	rows, err := db.Table("users").Rows()
	require.NoError(t, err)
	require.NotNil(t, rows)
	defer rows.Close()

	cols, err := rows.Columns()
	require.NoError(t, err)
	assert.NotEmpty(t, cols)

	n := 0
	for rows.Next() {
		n++
	}
	assert.Equal(t, 2, n)
}

func TestQueryMultiEmptyResult(t *testing.T) {
	db, _ := newSQLite(t, "table_empty")
	createUsers(t, db)

	rows, err := db.Table("users").QueryMulti()
	assert.Nil(t, rows)
	assert.ErrorIs(t, err, sqlm.ErrNotFound)

	row, err := db.Table("users").Query()
	assert.Nil(t, row)
	assert.ErrorIs(t, err, sqlm.ErrNotFound)
}

func TestTableUsesCallerContext(t *testing.T) {
	dir := t.TempDir()
	opt, err := sqlm.NewOptionsWithServer(sqlm.Server{
		Protocol: "sqlite",
		DSN:      filepath.Join(dir, "ctx.db"),
	}, "ctx_case")
	require.NoError(t, err)
	opt.SetLogger(&testLog{})
	drv, err := store.NewDriver(opt)
	require.NoError(t, err)
	require.True(t, sqlm.Use(drv))

	conn, err := drv.Connect(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	createUsers(t, sqlm.NewInstance(context.Background(), "ctx_case"))

	// 已取消的context必须让语句立刻失败，而不是被静默忽略
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	conn.WithContext(cancelled)

	tb := sqlm.Tbx(cancelled, "users").UseConn(conn).PreTable("mi_").SetProtocol(sqlm.SQLITE)
	_, err = tb.QueryMulti()
	assert.ErrorIs(t, err, context.Canceled)
	_, err = tb.Query()
	assert.ErrorIs(t, err, context.Canceled)
	_, err = tb.Insert(map[string]any{"name": "nope"})
	assert.ErrorIs(t, err, context.Canceled)
}

func TestLockOptionAndOptionString(t *testing.T) {
	db, _ := newSQLite(t, "table_lock")
	assertSQLEqual(t, "SELECT * FROM mi_users FOR UPDATE", db.Table("users").LockOption(true).SQL())
	assertSQLEqual(t, "SELECT * FROM mi_users", db.Table("users").LockOption(false).SQL())

	tb := db.Table("users").Option("delete")
	_, err := tb.Execute()
	assert.ErrorIs(t, err, sqlm.ErrMissingWhere)
}

func TestNewTableDefaults(t *testing.T) {
	tb := sqlm.NewTable("demo")
	require.NotNil(t, tb)
	assert.Equal(t, "SELECT * FROM demo", tb.SQL())
	assert.Equal(t, "SELECT * FROM demo", sqlm.NewTableWithContext(nil, "demo").SQL())

	// 多列/多条件组合不应产生重复的空格
	sql := sqlm.NewTable("demo").Select("a", "b").Where("x = 1").And("y = 2").
		GroupBy("x").Order("x").Limit(1).SQL()
	assert.False(t, strings.Contains(sql, "  "))
	assert.Equal(t, "SELECT a,b FROM demo WHERE x = 1 AND y = 2 GROUP BY x ORDER BY x ASC LIMIT 1", sql)
}

// ---------------------------------------------------------------------------
// PostgreSQL dialect (no server needed, only the generated text is checked)
// ---------------------------------------------------------------------------

// recConn records the last statement and always fails so nothing is executed.
type recConn struct {
	last string
	err  error
}

func (c *recConn) Exec(query string, _ ...any) (sql.Result, error) {
	c.last = query
	return nil, c.err
}

func (c *recConn) Prepare(query string) (*sql.Stmt, error) {
	c.last = query
	return nil, c.err
}

func (c *recConn) Query(query string, _ ...any) (*sql.Rows, error) {
	c.last = query
	return nil, c.err
}

func pgTable(t *testing.T, protocol string) (*sqlm.Table, *recConn) {
	t.Helper()
	conn := &recConn{err: errors.New("statement not executed")}
	tb := sqlm.Tbx(context.Background(), "users").
		PreTable("mi_").
		SetProtocol(protocol).
		UseConn(conn)
	return tb, conn
}

func TestInsertDialectPerProtocol(t *testing.T) {
	cases := []struct {
		protocol string
		single   string
		batch    string
	}{
		{
			sqlm.MYSQL,
			"INSERT INTO `mi_users` (`age`,`name`) VALUES (?,?)",
			"INSERT INTO `mi_users` (`age`,`name`) VALUES (?,?),(?,?)",
		},
		{
			sqlm.SQLITE,
			`INSERT INTO "mi_users" ("age","name") VALUES (?,?)`,
			`INSERT INTO "mi_users" ("age","name") VALUES (?,?),(?,?)`,
		},
		// PostgreSQL 用双引号标识符，占位符必须跨行连续编号
		{
			sqlm.POSTGRES,
			`INSERT INTO "mi_users" ("age","name") VALUES ($1,$2)`,
			`INSERT INTO "mi_users" ("age","name") VALUES ($1,$2),($3,$4)`,
		},
	}
	for _, c := range cases {
		tb, conn := pgTable(t, c.protocol)
		_, err := tb.Insert(map[string]any{"name": "a", "age": 1})
		assert.Error(t, err, c.protocol)
		assert.Equal(t, c.single, conn.last, c.protocol)

		tb, conn = pgTable(t, c.protocol)
		_, err = tb.Inserts([]string{"age", "name"}, [][]any{{1, "a"}, {2, "b"}})
		assert.Error(t, err, c.protocol)
		assert.Equal(t, c.batch, conn.last, c.protocol)
	}
}

func TestUpdateAndLimitUsePostgresDialect(t *testing.T) {
	tb, conn := pgTable(t, sqlm.POSTGRES)
	_, err := tb.Update(map[string]any{"name": "a"}).Where("id = 1").Execute()
	assert.Error(t, err)
	// 列名被引号包起来，表名保持原样（与 mysql 一致）
	assert.Equal(t, `UPDATE mi_users SET "name"='a' WHERE id = 1`, conn.last)

	// PostgreSQL 只认 LIMIT .. OFFSET ..，不接受 MySQL 的 LIMIT m,n
	assertSQLEqual(t, "SELECT id FROM mi_users LIMIT 10",
		pgBuilder(t, sqlm.POSTGRES).Select("id").Limit(10).SQL())
	assertSQLEqual(t, "SELECT id FROM mi_users LIMIT 10 OFFSET 20",
		pgBuilder(t, sqlm.POSTGRES).Select("id").LimitOffset(10, 20).SQL())
	// Limit(页码, 每页条数) 与 sqlite 一致：OFFSET = 页码 * 每页条数
	assertSQLEqual(t, "SELECT id FROM mi_users LIMIT 2 OFFSET 20",
		pgBuilder(t, sqlm.POSTGRES).Select("id").Limit(10, 2).SQL())
	// MySQL 仍然保留逗号写法
	assertSQLEqual(t, "SELECT id FROM mi_users LIMIT 10,2",
		pgBuilder(t, sqlm.MYSQL).Select("id").Limit(10, 2).SQL())
}

func TestEscapeKeepsBackslashOnPostgres(t *testing.T) {
	tb, conn := pgTable(t, sqlm.POSTGRES)
	_, err := tb.Update(map[string]any{"name": `a\b'c`}).Where("id = 1").Execute()
	assert.Error(t, err)
	// standard_conforming_strings 下反斜杠是普通字符，只转义单引号
	assert.Equal(t, `UPDATE mi_users SET "name"='a\b''c' WHERE id = 1`, conn.last)
}

func pgBuilder(t *testing.T, protocol string) *sqlm.Table {
	t.Helper()
	return sqlm.Tbx(context.Background(), "users").PreTable("mi_").SetProtocol(protocol)
}
