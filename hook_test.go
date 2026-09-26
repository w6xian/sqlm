package sqlm_test

// 钩子的契约：每条语句**恰好**被观测一次，且观测不能反过来影响执行。
//
// 这里覆盖的是"结构性"保证：新增入口忘了打点、钩子 panic 带走业务语句、
// 完整 SQL 被默认吐出去——这三类问题在使用方那边都很难发现，只能在这里钉住。

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/w6xian/sqlm"
	"github.com/w6xian/sqlm/store"
)

// record 收集钩子收到的每一次语句。
type record struct {
	mu   sync.Mutex
	info []sqlm.StmtInfo
	ctxs []context.Context
}

func (r *record) hook() sqlm.Hook {
	return sqlm.HookFunc(func(ctx context.Context, info *sqlm.StmtInfo) {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.info = append(r.info, *info)
		r.ctxs = append(r.ctxs, ctx)
	})
}

func (r *record) last(t testing.TB) sqlm.StmtInfo {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	require.NotEmpty(t, r.info, "钩子一次都没被调用")
	return r.info[len(r.info)-1]
}

func (r *record) len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.info)
}

// 查询会被观测：类型、表名、行数齐全，且默认不吐完整 SQL。
func TestHookObservesSelect(t *testing.T) {
	db, _ := newSQLite(t, "hook_select")
	createUsers(t, db)
	seedUsers(t, db, 3)

	rec := &record{}
	db.SetHooks(rec.hook())

	rows, err := db.Table("users").Select("id", "name").Where("age > %d", 10).LimitOffset(10).QueryMulti()
	require.NoError(t, err)
	require.Equal(t, 3, rows.Length())

	info := rec.last(t)
	require.Equal(t, sqlm.OpSelect, info.Op)
	require.Equal(t, "users", info.Table) // 未加前缀的原始表名
	require.Equal(t, int64(3), info.Rows)
	require.True(t, info.RowsKnown)
	require.NoError(t, info.Err)
	require.GreaterOrEqual(t, int64(info.Duration), int64(0))

	// 默认只给骨架：字面量已换成 ?
	require.Equal(t, "", info.SQL, "默认不得把完整 SQL 交给钩子")
	require.Contains(t, info.Digest, "?")
	require.NotContains(t, info.Digest, "User", "骨架里不该出现真实数据")
	require.Contains(t, info.Digest, "mi_users")
}

// 写操作：insert / update / delete 各走一次，行数取 RowsAffected。
func TestHookObservesWrites(t *testing.T) {
	db, _ := newSQLite(t, "hook_write")
	createUsers(t, db)

	rec := &record{}
	db.SetHooks(rec.hook())

	id, err := db.Table("users").Insert(map[string]any{"name": "bob", "age": 20, "intime": 1})
	require.NoError(t, err)
	info := rec.last(t)
	require.Equal(t, sqlm.OpInsert, info.Op)
	require.Equal(t, "users", info.Table)
	require.True(t, info.RowsKnown)

	n, err := db.Table("users").Update(map[string]any{"age": 30}).Where("id = %d", id).Execute()
	require.NoError(t, err)
	require.Equal(t, int64(1), n)
	info = rec.last(t)
	require.Equal(t, sqlm.OpUpdate, info.Op)
	require.Equal(t, int64(1), info.Rows)

	n, err = db.Table("users").Delete().Where("id = %d", id).Execute()
	require.NoError(t, err)
	require.Equal(t, int64(1), n)
	require.Equal(t, sqlm.OpDelete, rec.last(t).Op)
}

// 失败的语句也要被观测：报错的查询恰恰不该是链路里唯一看不见的那一类。
func TestHookObservesFailure(t *testing.T) {
	db, _ := newSQLite(t, "hook_fail")
	createUsers(t, db)

	rec := &record{}
	db.SetHooks(rec.hook())

	_, err := db.Table("users").QueryMulti()
	require.Error(t, err, "空表查询应返回 ErrNotFound")

	info := rec.last(t)
	require.Error(t, info.Err)
	require.Equal(t, sqlm.OpSelect, info.Op)
	require.Equal(t, int64(0), info.Rows)
}

// Rows() 交出的是原始游标：行数不可知，必须如实标记，不能填 0 冒充。
func TestHookRowsUnknownForCursor(t *testing.T) {
	db, _ := newSQLite(t, "hook_cursor")
	createUsers(t, db)
	seedUsers(t, db, 2)

	rec := &record{}
	db.SetHooks(rec.hook())

	rows, err := db.Table("users").Rows()
	require.NoError(t, err)
	if rows != nil {
		defer rows.Close()
	}
	require.False(t, rec.last(t).RowsKnown, "游标交出去之后行数不可知")
}

// 完整 SQL 必须"声明了才给"，并按 limit 截断。
func TestHookSQLIsOptIn(t *testing.T) {
	db, _ := newSQLite(t, "hook_sql")
	createUsers(t, db)
	seedUsers(t, db, 1)

	var got string
	db.SetHooks(sqlm.NewSQLHook(0, func(ctx context.Context, info *sqlm.StmtInfo) {
		got = info.SQL
	}))
	_, _ = db.Table("users").Where("name = '%s'", "User000").QueryMulti()
	require.Contains(t, got, "User000", "声明要看 SQL 的钩子应拿到完整语句")

	var short string
	db.SetHooks(sqlm.NewSQLHook(20, func(ctx context.Context, info *sqlm.StmtInfo) {
		short = info.SQL
	}))
	_, _ = db.Table("users").QueryMulti()
	require.LessOrEqual(t, len(short), 20, "完整 SQL 要按 limit 截断")
}

// 钩子 panic 不能带走业务语句：观测面的故障必须留在观测面。
func TestHookPanicDoesNotBreakStatement(t *testing.T) {
	db, _ := newSQLite(t, "hook_panic")
	createUsers(t, db)
	seedUsers(t, db, 2)

	db.SetHooks(sqlm.HookFunc(func(ctx context.Context, info *sqlm.StmtInfo) {
		panic("钩子写坏了")
	}))

	rows, err := db.Table("users").QueryMulti()
	require.NoError(t, err)
	require.Equal(t, 2, rows.Length())
}

// span 要挂得到请求链路上：Db 自带的 ctx 是实例级的，
// 只有 TableWithContext 带来的 ctx 上才有 trace id 一类的请求级数据。
func TestHookReceivesRequestContext(t *testing.T) {
	db, _ := newSQLite(t, "hook_ctx")
	createUsers(t, db)
	seedUsers(t, db, 1)

	type ctxKey struct{}
	rec := &record{}
	db.SetHooks(rec.hook())

	ctx := context.WithValue(context.Background(), ctxKey{}, "trace-1")
	_, err := db.TableWithContext(ctx, "users").QueryMulti()
	require.NoError(t, err)

	rec.mu.Lock()
	defer rec.mu.Unlock()
	require.NotEmpty(t, rec.ctxs)
	require.Equal(t, "trace-1", rec.ctxs[len(rec.ctxs)-1].Value(ctxKey{}))
}

// 池里借出来的实例通常是 Background 建的：绑一次请求 ctx，
// 之后由它派生的语句就都挂在这条链路上。
func TestDbWithContextReachesHook(t *testing.T) {
	db, _ := newSQLite(t, "hook_dbctx")
	createUsers(t, db)
	seedUsers(t, db, 1)

	type ctxKey struct{}
	rec := &record{}
	db.SetHooks(rec.hook())

	ctx := context.WithValue(context.Background(), ctxKey{}, "trace-2")
	db.WithContext(ctx)
	_, err := db.Table("users").QueryMulti() // 注意：不带 ctx 的 Table
	require.NoError(t, err)

	rec.mu.Lock()
	defer rec.mu.Unlock()
	require.NotEmpty(t, rec.ctxs)
	require.Equal(t, "trace-2", rec.ctxs[len(rec.ctxs)-1].Value(ctxKey{}))
}

// 语句级钩子只作用于这一条语句；Db 级的照旧生效且不重复调用。
func TestHookPerStatement(t *testing.T) {
	db, _ := newSQLite(t, "hook_stmt")
	createUsers(t, db)
	seedUsers(t, db, 1)

	dbRec := &record{}
	oneRec := &record{}
	db.SetHooks(dbRec.hook())

	_, err := db.Table("users").UseHook(oneRec.hook()).QueryMulti()
	require.NoError(t, err)
	require.Equal(t, 1, dbRec.len(), "Db 级钩子仍要调用，且只调用一次")
	require.Equal(t, 1, oneRec.len(), "语句级钩子要生效")

	_, err = db.Table("users").QueryMulti()
	require.NoError(t, err)
	require.Equal(t, 2, dbRec.len())
	require.Equal(t, 1, oneRec.len(), "语句级钩子不该粘到下一条语句上")
}

// 配置级钩子：之后建出来的实例自动带上。
func TestHookFromOptions(t *testing.T) {
	dir := t.TempDir()
	opt, err := sqlm.NewOptionsWithServer(sqlm.Server{
		Protocol:     "sqlite",
		DSN:          dir + "/opt.db",
		Pretable:     "mi_",
		MaxOpenConns: 4,
		MaxIdleConns: 2,
	}, "hook_opt")
	require.NoError(t, err)

	rec := &record{}
	sqlm.WithHooks(rec.hook())(opt)
	require.Len(t, opt.Hooks(), 1)

	drv, err := store.NewDriver(opt)
	require.NoError(t, err)
	require.True(t, sqlm.Use(drv))
	db := sqlm.NewInstance(context.Background(), "hook_opt")
	require.NoError(t, db.Err())
	t.Cleanup(db.Close)

	// 钩子在建实例时就带上了，所以建表语句（Db.Exec）也会被观测
	createUsers(t, db)
	_, err = db.Table("users").QueryMulti()
	require.Error(t, err)
	require.Equal(t, 2, rec.len(), "建表 + 查询各一次")
	require.Equal(t, sqlm.OpSelect, rec.last(t).Op)
}

// 骨架化：字面量与数字换 ?，标识符里的数字必须留下。
func TestDigestSQL(t *testing.T) {
	cases := []struct{ in, want string }{
		{"SELECT * FROM mi_users WHERE name = 'bob'", "SELECT * FROM mi_users WHERE name = ?"},
		{"SELECT * FROM mi_users WHERE id = 12", "SELECT * FROM mi_users WHERE id = ?"},
		{"SELECT * FROM mi_users WHERE score > -1.5", "SELECT * FROM mi_users WHERE score > ?"},
		{"INSERT INTO mi_users (id, name) VALUES (1, 'a''b')", "INSERT INTO mi_users (id, name) VALUES (?, ?)"},
		{"SELECT * FROM mi_users WHERE id IN (1, 2, 3)", "SELECT * FROM mi_users WHERE id IN (?, ?, ?)"},
		{"SELECT *  FROM   mi_users", "SELECT * FROM mi_users"},
		{"SELECT * FROM mi_2024_logs WHERE id = 7", "SELECT * FROM mi_2024_logs WHERE id = ?"},
	}
	for _, c := range cases {
		require.Equal(t, c.want, sqlm.DigestSQL(c.in))
		// 第二次走缓存，结果必须一致
		require.Equal(t, c.want, sqlm.DigestSQL(c.in))
	}
	require.Equal(t, "", sqlm.DigestSQL(""))
}

// 原始 SQL 通路（Db.Exec / Db.Query）也要被观测，
// 否则"绕过构造器的语句"就成了链路上的黑洞。
func TestHookObservesRawSQL(t *testing.T) {
	db, _ := newSQLite(t, "hook_raw")
	createUsers(t, db)

	rec := &record{}
	db.SetHooks(rec.hook())

	mustExec(t, db, "INSERT INTO mi_users (name, age, intime) VALUES ('bob', 20, 1)")
	require.Equal(t, sqlm.OpExec, rec.last(t).Op)
	require.Equal(t, "", rec.last(t).Table, "原始 SQL 无从得知表名")
	require.True(t, rec.last(t).RowsKnown)

	_, err := db.Query("SELECT count(*) as c FROM mi_users")
	require.NoError(t, err)
	require.Equal(t, sqlm.OpSelect, rec.last(t).Op)
	require.Equal(t, int64(1), rec.last(t).Rows)
}

// 事务里的语句同样要被观测（Db.Action 把钩子带进 Tx）。
func TestHookObservesTransaction(t *testing.T) {
	db, _ := newSQLite(t, "hook_tx")
	createUsers(t, db)

	rec := &record{}
	db.SetHooks(rec.hook())

	_, err := db.Action(func(tx sqlm.ITable, args ...any) (int64, error) {
		return tx.Table("users").Insert(map[string]any{"name": "tx", "age": 9, "intime": 1})
	})
	require.NoError(t, err)
	require.Equal(t, 1, rec.len())
	require.Equal(t, sqlm.OpInsert, rec.last(t).Op)
}

// 没挂钩子时行为不变，且实例上没有钩子。
func TestNoHookChangesNothing(t *testing.T) {
	db, _ := newSQLite(t, "hook_none")
	createUsers(t, db)
	seedUsers(t, db, 2)

	rows, err := db.Table("users").QueryMulti()
	require.NoError(t, err)
	require.Equal(t, 2, rows.Length())
	require.Empty(t, db.Hooks())
}
