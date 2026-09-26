package sqlm

import (
	"context"
	"sync"
	"time"
)

// 语句类型：钩子用它区分操作，通常也直接拿去拼 span / 指标名（db.select、db.insert …）。
const (
	OpSelect = "select"
	OpInsert = "insert"
	OpUpdate = "update"
	OpDelete = "delete"
	// OpExec 调用方自带原始 SQL 的通路（Db.Exec / Tx.Exec），表名无从得知。
	OpExec = "exec"
)

// DefaultSQLLimit 完整语句的默认截断长度。
//
// 一条拼好的 SQL 可以很长（批量 INSERT 尤其），原样交给钩子既浪费内存
// 也可能拖慢 APM 上报，所以默认截断。
const DefaultSQLLimit = 2000

// StmtInfo 一次语句执行的观测快照。
//
// 交给钩子的是**只读快照**：改它不会改变任何执行行为，钩子也没有机会改语句。
type StmtInfo struct {
	// Op 语句类型，见 OpSelect / OpInsert / …常量。
	Op string
	// Table 未加前缀的表名；走原始 SQL 的通路（Db.Exec）为空串。
	Table string
	// Digest 语句骨架：字面量已换成 ?，可安全用于 span 名与指标标签。
	Digest string
	// SQL 完整语句，仅在钩子 Config().SQL 为真时填充（并按 SQLLimit 截断）。
	//
	// 本库是拼 SQL 的，完整语句里全是真实业务数据，默认不给。
	SQL string
	// Rows select 为返回行数，写操作为 RowsAffected。
	Rows int64
	// RowsKnown Rows 是否可信：Rows() 交出的是原始游标，行数无从得知。
	RowsKnown bool
	// Duration 语句耗时。select 含结果读取（结果集已在内部读尽）。
	Duration time.Duration
	// StartAt 起始时刻，供 OpenTelemetry 用 WithTimestamp 回放 span。
	StartAt time.Time
	// Err 执行错误，成功为 nil。
	Err error
}

// HookConfig 钩子声明自己需要什么。
type HookConfig struct {
	// SQL 为真时把完整语句填进 StmtInfo.SQL。
	//
	// 只有在受限环境（本地调试、脱敏后的内网 APM）才该打开：
	// 语句里的字面量就是业务数据本身。
	SQL bool
	// SQLLimit 截断长度，<= 0 时取 DefaultSQLLimit。
	SQLLimit int
}

// Hook 观察一次语句执行。
//
// 实现必须廉价（它在每条语句的关键路径上）且不得 panic
// ——库会 recover，但那是兜底，不是让你省掉错误处理的理由。
// 返回值被忽略：观测面永远不能影响业务面。
type Hook interface {
	// Config 声明需要什么，在注册时与每条语句前各查一次。
	Config() HookConfig
	// Emit 语句执行结束后被调用一次（无论成功失败）。
	Emit(ctx context.Context, info *StmtInfo)
}

// HookFunc 函数式钩子，默认**不要**完整 SQL——安全优先。
type HookFunc func(ctx context.Context, info *StmtInfo)

// Config 函数式钩子只要骨架。要看完整 SQL 请用 NewSQLHook。
func (f HookFunc) Config() HookConfig { return HookConfig{} }

func (f HookFunc) Emit(ctx context.Context, info *StmtInfo) { f(ctx, info) }

// sqlHook 带配置的钩子：目前唯一的配置就是"要不要完整 SQL"。
type sqlHook struct {
	fn  HookFunc
	cfg HookConfig
}

func (h sqlHook) Config() HookConfig                       { return h.cfg }
func (h sqlHook) Emit(ctx context.Context, info *StmtInfo) { h.fn(ctx, info) }

// NewSQLHook 返回一个能看到完整语句的钩子。limit <= 0 时取 DefaultSQLLimit。
//
// 想清楚再开：完整语句 = 业务数据，它会跟着 span / 日志离开进程。
func NewSQLHook(limit int, fn HookFunc) Hook {
	if limit <= 0 {
		limit = DefaultSQLLimit
	}
	return sqlHook{fn: fn, cfg: HookConfig{SQL: true, SQLLimit: limit}}
}

// emitStmt 把一次执行派发给所有钩子。
//
// 每个钩子拿到一份独立副本：要不要完整 SQL、截多长由各自 Config 决定，
// 互不影响；副本是小结构体，比"让钩子共享可变状态"安全得多。
func emitStmt(ctx context.Context, hooks []Hook, base *StmtInfo, query string) {
	for _, h := range hooks {
		info := *base
		if cfg := h.Config(); cfg.SQL {
			info.SQL = truncateSQL(query, cfg.SQLLimit)
		}
		emitOne(ctx, h, &info)
	}
}

// Db 级与语句级钩子分两组派发：既不为了合并而分配新切片，
// 也不会因为两处都配了同一个钩子而重复调用两次。
func emitBoth(ctx context.Context, shared, own []Hook, base *StmtInfo, query string) {
	emitStmt(ctx, shared, base, query)
	emitStmt(ctx, own, base, query)
}

// emitOne 单个钩子：panic 就地吞掉。
//
// 一个写坏的钩子不该带走一条业务语句——观测面的故障必须留在观测面。
func emitOne(ctx context.Context, h Hook, info *StmtInfo) {
	defer func() {
		_ = recover()
	}()
	h.Emit(ctx, info)
}

// truncateSQL 按 rune 截断，避免把多字节字符切成半个。
func truncateSQL(query string, limit int) string {
	if limit <= 0 {
		limit = DefaultSQLLimit
	}
	if len(query) <= limit {
		return query
	}
	n := 0
	for i := range query {
		if n >= limit {
			return query[:i]
		}
		n++
	}
	return query
}

// ===== SQL 骨架（digest）=====

// digestCacheMax 骨架缓存的上限。
//
// 同一条骨架会被反复计算，缓存能省下每次扫描字符串的开销；
// 但入参不同的语句骨架也不同（骨架里仍有 IN 的项数、LIMIT 的数值差异…），
// 无界缓存就是内存泄漏，超出上限直接整体清空——比 LRU 简单，也够用。
const digestCacheMax = 1024

var digestCache = struct {
	mu sync.Mutex
	m  map[string]string
}{m: make(map[string]string, 64)}

// DigestSQL 把语句里的字面量换成 ?，得到可安全暴露的骨架。
//
// 为什么必须有它：完整 SQL 带真实数据，而"每个不同入参一条时间序列"
// 会把 APM / 指标的基数打爆。骨架才是能长期留在指标里的形态。
func DigestSQL(query string) string {
	if query == "" {
		return ""
	}
	digestCache.mu.Lock()
	d, ok := digestCache.m[query]
	digestCache.mu.Unlock()
	if ok {
		return d
	}
	d = digestOf(query)
	digestCache.mu.Lock()
	if len(digestCache.m) >= digestCacheMax {
		digestCache.m = make(map[string]string, 64)
	}
	digestCache.m[query] = d
	digestCache.mu.Unlock()
	return d
}

// digestOf 单趟扫描：单引号字符串与数字字面量换成 ?，空白折叠成一个空格。
//
// 标识符（表名、列名、别名）里的数字与点必须保留，否则 mi_users 会变成 mi_?s，
// 骨架照样基数爆炸——所以数字只在"前一个字节不是标识符字符"时才替换。
func digestOf(query string) string {
	var sb []byte
	sb = make([]byte, 0, len(query))
	space := false
	for i := 0; i < len(query); {
		c := query[i]
		switch {
		case c == '\'':
			sb = flushSpace(sb, &space)
			i = skipLiteral(query, i+1)
			sb = append(sb, '?')
			continue
		case isSQLSpace(c):
			space = true
			i++
			continue
		case numStart(query, i):
			sb = flushSpace(sb, &space)
			i = skipNumber(query, i)
			sb = append(sb, '?')
			continue
		}
		if space && len(sb) > 0 {
			sb = append(sb, ' ')
		}
		space = false
		sb = append(sb, c)
		i++
	}
	return string(sb)
}

// flushSpace 补上待写的空格。
//
// 空格是"遇到下一个非空内容时才写"：折叠连续空白与去掉首尾空格
// 用的是同一段逻辑，字面量分支也要先补空格再写占位符。
func flushSpace(sb []byte, space *bool) []byte {
	if *space && len(sb) > 0 {
		sb = append(sb, ' ')
	}
	*space = false
	return sb
}

// skipLiteral 跳过单引号字符串，返回结束引号后的位置；” 视为转义。
func skipLiteral(query string, i int) int {
	for i < len(query) {
		if query[i] != '\'' {
			i++
			continue
		}
		// 两个连续引号是转义，不是一个字符串的结束
		if i+1 < len(query) && query[i+1] == '\'' {
			i += 2
			continue
		}
		return i + 1
	}
	return i
}

// numStart 判断 i 处是不是一个数字字面量的开头（而不是标识符的一部分）。
func numStart(query string, i int) bool {
	c := query[i]
	if c >= '0' && c <= '9' {
		return i == 0 || !identTail(query[i-1])
	}
	if (c == '-' || c == '+') && i+1 < len(query) && query[i+1] >= '0' && query[i+1] <= '9' {
		return i == 0 || (!identTail(query[i-1]) && query[i-1] != ')')
	}
	return false
}

// skipNumber 跳过一个数字字面量（含小数与指数）。
func skipNumber(query string, i int) int {
	if query[i] == '-' || query[i] == '+' {
		i++
	}
	seenDot := false
	for i < len(query) {
		c := query[i]
		switch {
		case c >= '0' && c <= '9':
			i++
		case c == '.' && !seenDot:
			seenDot = true
			i++
		case (c == 'e' || c == 'E') && i+1 < len(query) &&
			(query[i+1] >= '0' && query[i+1] <= '9' || query[i+1] == '-' || query[i+1] == '+'):
			i += 2
		default:
			return i
		}
	}
	return i
}

func isSQLSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

// identTail 标识符里允许出现的字符：数字不能紧跟在它们后面被当成字面量。
func identTail(c byte) bool {
	return c == '_' || c == '.' || c == '`' || c == '"' ||
		(c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}
