// # sqlm 钩子（hook）样例
//
// 解决的问题：不想在每个调用点手写 span / 慢查询日志 / 计数器，
// 让每条语句执行完之后自己报一遍——包括那些埋在第三方接口深处、
// 你够不到调用点的语句。
//
// 运行：
//
//	go run ./examples/hook
//
// 三个注册入口：
//
//	opt.SetHooks(...) 配置级 —— 之后由这份配置建出来的实例都带上
//	db.SetHooks(...)  实例级 —— 只作用于这个 Db 派生的语句
//	tb.UseHook(...)   语句级 —— 只观测这一条语句
//
// 只依赖标准库：把 spanHook.Emit 里的打印换成你的 APM SDK 即可。
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/w6xian/sqlm"
	"github.com/w6xian/sqlm/store"
)

const usersSchema = `CREATE TABLE IF NOT EXISTS mi_users (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL,
	age INTEGER NOT NULL DEFAULT 0,
	intime INTEGER NOT NULL DEFAULT 0
)`

// User 只是演示 Scan 的载体，与钩子无关。
type User struct {
	Id     int64  `json:"id"`
	Name   string `json:"name"`
	Age    int64  `json:"age"`
	Intime int64  `json:"intime"`
}

// metrics 挂在配置级：由这份配置建出来的实例，所有语句都往这里累计。
var metrics = &metricsHook{}

func main() {
	db := newDemoDB("hook")
	defer db.Close()
	prepare(db) // 建表 + 种子数据，配置级钩子已经在工作

	fmt.Println("\n=== 1. 每条语句一个 span ===")
	spans := &spanHook{system: "sqlite"}
	// 注意：Db.SetHooks 会连同"从配置继承来的钩子"一起覆盖掉——
	// 想留住配置级钩子就把它一起写上（配置级原本的语义是"默认带上"）。
	db.SetHooks(metrics, spans)
	demand(db)

	fmt.Println("\n=== 2. 慢查询日志 ===")
	// 阈值给 0 是为了演示时每条都命中；生产环境用 200*time.Millisecond 这类值。
	db.SetHooks(metrics, slowHook{threshold: 0})
	demand(db)

	fmt.Println("\n=== 3. 语句级：只盯这一条 ===")
	// 上一节的实例级钩子还在——Db 级和语句级会各自被调用一次，
	// 所以下面同时出现 [slow] 和 [once] 两行。
	rows, err := db.Table("users").
		Select("id", "name").
		Where("age > %d", 18).
		LimitOffset(10).
		UseHook(sqlm.HookFunc(func(ctx context.Context, info *sqlm.StmtInfo) {
			fmt.Printf("  [once] op=%s table=%s digest=%s\n", info.Op, info.Table, info.Digest)
		})).
		QueryMulti()
	checkErr("语句级查询", err)
	fmt.Printf("  读到 %d 行\n", rows.Length())

	fmt.Println("\n=== 4. 请求 ctx：span 要挂到调用链上 ===")
	type traceKey struct{}
	ctx := context.WithValue(context.Background(), traceKey{}, "trace-7f3c19")
	db.SetHooks(metrics, sqlm.HookFunc(func(ctx context.Context, info *sqlm.StmtInfo) {
		trace, _ := ctx.Value(traceKey{}).(string)
		fmt.Printf("  [ctx] trace=%s op=%s\n", trace, info.Op)
	}))
	// Db.Table(...) 带的是 Db 自己的进程级 ctx，那上面没有 trace id；
	// 要让 span 挂到请求链路上，就把 ctx 传进来。
	_, err = db.TableWithContext(ctx, "users").QueryMulti()
	checkErr("带 ctx 的查询", err)
	// 池里借出来的实例通常是 Background 建的，也可以先绑一次：
	// db.WithContext(ctx)，之后由它派生的语句都带着这个 ctx。

	fmt.Println("\n=== 5. 事务里的语句同样被观测 ===")
	db.SetHooks(metrics, spans)
	spans.reset()
	_, err = db.Action(func(tx sqlm.ITable, args ...any) (int64, error) {
		n, e := tx.Table("users").Insert(map[string]any{"name": "tx-user", "age": 30, "intime": 1})
		if e != nil {
			return 0, e // 回滚
		}
		return n, nil // 提交
	})
	checkErr("事务写入", err)

	fmt.Println("\n=== 6. 失败的语句更要看得见 ===")
	spans.reset()
	_, err = db.Table("users").Select("no_such_column").QueryMulti()
	fmt.Printf("  业务看到：%v\n", err)
	if last := spans.last(); last != nil {
		fmt.Printf("  钩子看到：op=%s rows=%d err=%v\n", last.Op, last.Rows, last.Err)
	}

	fmt.Println("\n=== 7. 想看完整 SQL：显式声明，并按长度截断 ===")
	// 默认只给 Digest（字面量换成 ?）：sqlm 是拼 SQL 的，
	// 完整语句 = 业务数据，不该默认流进 span / 日志。
	db.SetHooks(metrics, sqlm.NewSQLHook(70, func(ctx context.Context, info *sqlm.StmtInfo) {
		fmt.Printf("  [sql]    %s\n  [digest] %s\n", info.SQL, info.Digest)
	}))
	_, _ = db.Table("users").Where("name = '%s'", "User001").QueryMulti()

	fmt.Println("\n=== 8. 钩子写坏了不能带走业务语句 ===")
	db.SetHooks(metrics, boomHook{})
	rows, err = db.Table("users").QueryMulti()
	checkErr("钩子 panic 后的查询", err)
	fmt.Printf("  业务照样读到 %d 行（钩子里的 panic 被库兜住）\n", rows.Length())

	fmt.Println("\n=== 汇总 ===")
	metrics.dump()
	fmt.Println("  没挂钩子时没有任何观测开销：不计时、不算骨架、不分配——热路径上多一个分支而已。")
}

// ---------------------------------------------------------------------------
// 四个演示钩子
// ---------------------------------------------------------------------------

// spanHook 语句级 span，对应你们在调用点手写的那段 tracing 代码。
//
// OpenTelemetry 版（示意，放到 Emit 里即可）：
//
//	_, sp := tracer.Start(ctx, "db."+info.Op, trace.WithTimestamp(info.StartAt))
//	defer sp.End(trace.WithTimestamp(info.StartAt.Add(info.Duration)))
//	sp.SetAttributes(
//	    attribute.String("db.system", h.system),
//	    attribute.String("db.operation", info.Op),
//	    attribute.String("db.table", info.Table),
//	    attribute.String("db.statement", info.Digest), // 骨架，不是完整 SQL
//	)
//	if info.RowsKnown {
//	    sp.SetAttributes(attribute.Int64("db.rows", info.Rows))
//	}
//	if info.Err != nil {
//	    sp.RecordError(info.Err)
//	}
type spanHook struct {
	system string

	mu    sync.Mutex
	items []sqlm.StmtInfo
}

// Config 声明"只要骨架"：不看完整 SQL，数据就不会跟着 span 离开进程。
func (h *spanHook) Config() sqlm.HookConfig { return sqlm.HookConfig{} }

func (h *spanHook) Emit(ctx context.Context, info *sqlm.StmtInfo) {
	h.mu.Lock()
	h.items = append(h.items, *info)
	h.mu.Unlock()

	rows := "?"
	if info.RowsKnown {
		rows = strconv.FormatInt(info.Rows, 10)
	}
	status := "OK"
	if info.Err != nil {
		status = "ERR:" + info.Err.Error()
	}
	fmt.Printf("  [span] db.%-6s system=%s table=%-8s rows=%-3s %-8s %s\n",
		info.Op, h.system, info.Table, rows,
		info.Duration.Round(time.Microsecond), status)
}

func (h *spanHook) last() *sqlm.StmtInfo {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.items) == 0 {
		return nil
	}
	return &h.items[len(h.items)-1]
}

func (h *spanHook) reset() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.items = h.items[:0]
}

// slowHook 超过阈值才吱声：不建议用完整 SQL 打日志，用骨架。
type slowHook struct{ threshold time.Duration }

func (slowHook) Config() sqlm.HookConfig { return sqlm.HookConfig{} }

func (h slowHook) Emit(ctx context.Context, info *sqlm.StmtInfo) {
	if info.Duration < h.threshold {
		return
	}
	fmt.Printf("  [slow] %-6s %-8s %-8s %s\n", info.Op, info.Table,
		info.Duration.Round(time.Microsecond), cut(info.Digest, 62))
}

// metricsHook 按 op+table 聚合。
//
// 标签用 Digest 而不是完整 SQL：每个不同入参一条时间序列，基数会打爆。
type metricsHook struct {
	mu     sync.Mutex
	counts map[string]int
	total  time.Duration
}

func (h *metricsHook) Config() sqlm.HookConfig { return sqlm.HookConfig{} }

func (h *metricsHook) Emit(ctx context.Context, info *sqlm.StmtInfo) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.counts == nil {
		h.counts = map[string]int{}
	}
	if info.Table == "" {
		h.counts["db."+info.Op]++
	} else {
		h.counts["db."+info.Op+"."+info.Table]++
	}
	h.total += info.Duration
}

func (h *metricsHook) dump() {
	h.mu.Lock()
	defer h.mu.Unlock()
	keys := make([]string, 0, len(h.counts))
	for k := range h.counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	fmt.Printf("  配置级钩子累计 %d 类语句，总耗时 %s：\n", len(keys), h.total.Round(time.Microsecond))
	for _, k := range keys {
		fmt.Printf("    %-24s %d 次\n", k, h.counts[k])
	}
}

// boomHook 故意 panic，演示库会兜住、业务语句照常执行。
type boomHook struct{}

func (boomHook) Config() sqlm.HookConfig { return sqlm.HookConfig{} }

func (boomHook) Emit(ctx context.Context, info *sqlm.StmtInfo) {
	panic("钩子写坏了")
}

// ---------------------------------------------------------------------------
// 样板：建库、演示读写
// ---------------------------------------------------------------------------

func newDemoDB(name string) *sqlm.Db {
	dsn := filepath.Join(os.TempDir(), "sqlm_hook_demo.db")
	opt, err := sqlm.NewOptionsWithServer(sqlm.Server{
		Protocol:     "sqlite",
		DSN:          dsn,
		Pretable:     "mi_",
		MaxOpenConns: 4,
		MaxIdleConns: 2,
		MaxLifetime:  int(time.Minute),
	}, name)
	checkErr("构造配置", err)

	// 静音：sqlm 自带的 Debug 日志也会吐 SQL，这里只想看钩子的输出。
	opt.SetLogger(sqlm.NewNoopLogger())

	// 配置级钩子：注册在这份配置上，之后由它建出来的实例自动带上。
	sqlm.WithHooks(metrics)(opt)

	drv, err := store.NewDriver(opt)
	checkErr("建驱动", err)
	if !sqlm.Use(drv) {
		fmt.Println("注册实例失败")
		os.Exit(1)
	}
	db := sqlm.NewInstance(context.Background(), name)
	checkErr("取实例", db.Err())
	fmt.Printf("已连接 sqlite：%s\n", dsn)
	return db
}

func prepare(db *sqlm.Db) {
	if _, err := db.Exec("DROP TABLE IF EXISTS mi_users"); err != nil {
		checkErr("清表", err)
	}
	if _, err := db.Exec(usersSchema); err != nil {
		checkErr("建表", err)
	}
	data := make([][]any, 0, 3)
	for i := 0; i < 3; i++ {
		data = append(data, []any{fmt.Sprintf("User%03d", i), 18 + i, int64(time.Now().Unix())})
	}
	if _, err := db.Table("users").Inserts([]string{"name", "age", "intime"}, data); err != nil {
		checkErr("写种子数据", err)
	}
}

// demand 一条普通业务读写路径：钩子要覆盖的就是这种代码。
func demand(db *sqlm.Db) {
	if _, err := db.Table("users").QueryMulti(); err != nil {
		checkErr("列表查询", err)
	}
	var u User
	row, err := db.Table("users").Select("id", "name", "age").Where("age > %d", 18).Query()
	if err == nil && row != nil {
		_ = row.Scan(&u)
	}
	id, err := db.Table("users").Insert(map[string]any{"name": "hook-user", "age": 22, "intime": 1})
	if err != nil {
		checkErr("写入", err)
	}
	if _, err = db.Table("users").Update(map[string]any{"age": 33}).Where("id = %d", id).Execute(); err != nil {
		checkErr("更新", err)
	}
	if _, err = db.Table("users").Delete().Where("id = %d", id).Execute(); err != nil {
		checkErr("删除", err)
	}
}

func checkErr(what string, err error) {
	if err != nil {
		fmt.Printf("%s失败：%v\n", what, err)
		os.Exit(1)
	}
}

func cut(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
