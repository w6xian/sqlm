package sqlm

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/w6xian/sqlm/utils"
)

type KeyValue map[string]any

type Table struct {
	pTable   string   `sql:"table"`
	pJoin    []string `sql:"join"`
	pWhere   []string `sql:"where"`
	pGroupBy []string `sql:"group by"`
	pOrderD  []string `sql:"order by cols desc"`
	pOrderA  []string `sql:"order by cols asc"`
	pLimit   []int64  `sql:"limit"`
	pOffset  []int64  `sql:"offset"`
	pColumns []string
	pData    map[string]string
	pOption  string
	pLock    string
	pType    string
	multi    bool
	dbConn   TxConn
	pPre     string
	protocol string
	db       *Db
	log      StdLog
	ctx      context.Context
}

func NewTable(tle string) *Table {
	return NewTableWithContext(context.Background(), tle)
}

func NewTableWithContext(ctx context.Context, tle string) *Table {
	pt := &Table{}
	pt.pColumns = []string{}
	pt.pData = make(map[string]string)
	pt.pOption = "select"
	pt.pType = "array"
	pt.pTable = tle
	pt.pPre = ""
	pt.ctx = ctx
	pt.protocol = MYSQL
	return pt
}

func (t *Table) PreTable(pre string) *Table {
	t.pPre = pre
	return t
}

func (t *Table) SetProtocol(protocol string) *Table {
	t.protocol = protocol
	return t
}

func (t *Table) Use(db *Db) *Table {
	t.db = db
	t.dbConn = db.conn
	return t
}
func (t *Table) UseLog(log StdLog) *Table {
	t.log = log
	return t
}

func (t *Table) UseConn(conn TxConn) *Table {
	t.dbConn = conn
	return t
}

func (t *Table) From(tle string) *Table {
	t.pTable = tle
	return t
}

// 加前坠 adb.table 表：数据库+表
//
// 只用 strings.Count/IndexByte 判断，不再 strings.Split 出一个切片再 Sprintf：
// 每次拼装都要走这里，拆片的开销是纯粹的浪费。
func (t *Table) table_prefix() string {
	if strings.Count(t.pTable, ".") == 1 {
		if i := strings.IndexByte(t.pTable, '.'); i >= 0 {
			return t.pPre + t.pTable[i+1:]
		}
	}
	return t.pPre + t.pTable
}

func (t *Table) LeftJoin(tbl string, onKey string, args ...any) *Table {
	return t.join("LEFT", tbl, onKey, args...)
}

func (t *Table) RightJoin(tbl string, onKey string, args ...any) *Table {
	return t.join("RIGHT", tbl, onKey, args...)
}

func (t *Table) InnerJoin(tbl string, onKey string, args ...any) *Table {
	return t.join("INNER", tbl, onKey, args...)
}

func (t *Table) join(option string, tbl string, onKey string, args ...any) *Table {
	t.pJoin = append(t.pJoin, fmt.Sprintf(" %s JOIN %s ON %s", option, fmt.Sprintf("%s%s", t.pPre, tbl), fmt.Sprintf(onKey, args...)))
	return t
}

func (t *Table) pushConditions(w string) *Table {
	t.pWhere = append(t.pWhere, w)
	return t
}

// check only validates that a connection holder is bound.
//
// It deliberately does NOT ping the database: doing a round trip before each
// statement doubled the cost of every query. Failures are reported by the
// statement itself, and Db.Ping()/Db.Conn() remain available for an explicit
// health check.
func (t *Table) check() error {
	if t.dbConn == nil {
		if t.db != nil {
			return t.db.checkConn()
		}
		return ErrNoConnection
	}
	if strings.TrimSpace(t.pTable) == "" {
		return ErrEmptyTableName
	}
	return nil
}

func (t *Table) Insert(data map[string]any) (int64, error) {
	if err := t.check(); err != nil {
		return 0, err
	}
	if len(data) == 0 {
		return 0, ErrMissingValues
	}
	// 稳定输出：按列名字典序生成，便于与SQL缓存/慢日志对齐
	keys := make([]string, 0, len(data))
	for c := range data {
		keys = append(keys, c)
	}
	sort.Strings(keys)

	// 直接写进一个 builder：旧实现要先生成列名切片、占位符切片，再 Join 两次、
	// Sprintf 一次，每一步都是一次分配。
	values := make([]any, 0, len(keys))
	var sb strings.Builder
	sb.Grow(32 + len(t.pTable) + joinLen(keys)*3)
	sb.WriteString("INSERT INTO ")
	sb.WriteString(t.quoteTable())
	sb.WriteString(" (")
	for i, c := range keys {
		if i > 0 {
			sb.WriteByte(',')
		}
		t.writeIdent(&sb, c)
		values = append(values, data[c])
	}
	sb.WriteString(") VALUES (")
	t.writePlaceholders(&sb, len(values), 0)
	sb.WriteByte(')')
	sql := sb.String()
	t.logger().Debug(sql)

	stmt, err := t.dbConn.Prepare(sql)
	defer func() {
		if stmt != nil {
			stmt.Close()
		}
	}()
	if err != nil {
		return 0, err
	}
	rst, err := stmt.ExecContext(t.context(), values...)
	if err != nil {
		return 0, err
	}
	return lastInsertID(rst)
}

// context never returns nil: a missing context would panic in database/sql.
func (t *Table) context() context.Context {
	if t.ctx != nil {
		return t.ctx
	}
	return context.Background()
}

// quoteTable quotes the (possibly prefixed) table name for the active protocol.
func (t *Table) quoteTable() string {
	return quoteIdentFor(t.protocol, t.table_prefix())
}

// quoteIdent wraps a column name with the quoting used by the active protocol.
func (t *Table) quoteIdent(name string) string {
	return quoteIdentFor(t.protocol, name)
}

// quoteIdentFor quotes an identifier exactly once: MySQL uses backticks, every
// other engine (PostgreSQL, SQLite) understands the standard double quotes.
func quoteIdentFor(protocol, name string) string {
	if protocol == MYSQL {
		name = strings.Trim(name, "`")
		return "`" + strings.ReplaceAll(name, "`", "``") + "`"
	}
	name = strings.Trim(name, `"`)
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// writeIdent writes a quoted identifier into sb.
//
// The common case - a name without any quote character - is written straight
// into the builder, which saves one allocation per column compared to building
// the quoted string first.
func (t *Table) writeIdent(sb *strings.Builder, name string) {
	quote := byte('"')
	if t.protocol == MYSQL {
		quote = '`'
	}
	if strings.IndexByte(name, quote) < 0 {
		sb.WriteByte(quote)
		sb.WriteString(name)
		sb.WriteByte(quote)
		return
	}
	sb.WriteString(quoteIdentFor(t.protocol, name))
}

// writePlaceholders writes count placeholders into sb. PostgreSQL numbers its
// markers, so start lets a multi row statement continue the numbering of the
// previous row.
func (t *Table) writePlaceholders(sb *strings.Builder, count, start int) {
	var buf [20]byte
	for i := 0; i < count; i++ {
		if i > 0 {
			sb.WriteByte(',')
		}
		if !isPostgres(t.protocol) {
			sb.WriteByte('?')
			continue
		}
		sb.WriteByte('$')
		// 直接写进 builder，避免 strconv.Itoa 为三位以上的编号分配字符串
		sb.Write(strconv.AppendInt(buf[:0], int64(start+i+1), 10))
	}
}

// escape quotes a value so it can be safely inlined in a statement.
func (t *Table) escape(value string) string {
	// PostgreSQL runs with standard_conforming_strings: a backslash is a
	// regular character, escaping it would store duplicated backslashes.
	if t.protocol == SQLITE || isPostgres(t.protocol) {
		if !strings.ContainsAny(value, "'\x00") {
			return value
		}
		out := strings.ReplaceAll(value, "'", "''")
		if strings.IndexByte(value, 0) >= 0 {
			// 标准协议的字符串里放不下 NUL：sqlite 会截断语句，postgres 会报
			// invalid byte sequence，直接丢掉比把整条语句写坏要好。
			out = strings.ReplaceAll(out, "\x00", "")
		}
		return out
	}
	// 没有需要转义的字符时原样返回，省掉一次分配
	if !strings.ContainsAny(value, "'\\\x00") {
		return value
	}
	var sb strings.Builder
	sb.Grow(len(value) + 4)
	for _, r := range value {
		switch r {
		case '\'':
			sb.WriteString("''")
		case '\\':
			sb.WriteString("\\\\")
		case 0:
			// 截断NULL字节，避免语句被意外终止
			sb.WriteString("\\0")
		default:
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

func (t *Table) Inserts(columns []string, data [][]any) (int64, error) {
	if err := t.check(); err != nil {
		return 0, err
	}

	if len(data) <= 0 {
		return 0, ErrMissingValues
	}
	colLen := len(columns)
	if colLen <= 0 {
		return 0, ErrMissingColumns
	}
	// 注意：绝不就地改写调用方传入的 columns（旧实现会这么干，同一个切片换协议
	// 复用一次就会把反引号留在双引号里面，语句直接写坏）。
	val := make([]any, 0, len(data)*colLen)
	var sb strings.Builder
	sb.Grow(32 + len(t.pTable) + joinLen(columns)*3 + len(data)*(colLen*5+3))
	sb.WriteString("INSERT INTO ")
	sb.WriteString(t.quoteTable())
	sb.WriteString(" (")
	for i, c := range columns {
		if i > 0 {
			sb.WriteByte(',')
		}
		t.writeIdent(&sb, c)
	}
	sb.WriteString(") VALUES ")
	for i, v := range data {
		if colLen != len(v) {
			return 0, ErrColumnsNotMatched
		}
		if i > 0 {
			sb.WriteByte(',')
		}
		// PostgreSQL numbers its placeholders across the whole statement, so
		// every row continues where the previous one stopped.
		sb.WriteByte('(')
		t.writePlaceholders(&sb, colLen, len(val))
		sb.WriteByte(')')
		val = append(val, v...)
	}
	sql := sb.String()
	t.logger().Debug(sql)
	stmt, err := t.dbConn.Prepare(sql)
	defer func() {
		if stmt != nil {
			stmt.Close()
		}
	}()
	if err != nil {
		return 0, err
	}
	rst, err := stmt.ExecContext(t.context(), val...)
	if err != nil {
		return 0, err
	}
	return lastInsertID(rst)
}

func (tx *Table) AndSearchOption(ok bool, col string, value string, args ...string) *Table {
	if ok {
		if len(args) == 0 {
			args = append(args, "")
		}
		alias := args[0]
		if col == "" {
			return tx
		}
		if strings.HasPrefix(col, "$") {
			return tx
		}
		tx.pushConditions("AND")
		if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
			// 区间查询，时间和价格区间
			vs := strings.Split(strings.TrimSuffix(strings.TrimPrefix(value, "["), "]"), ",")
			if len(vs) != 2 {
				tx.pushConditions(fmt.Sprintf("%s%s LIKE '%s'", alias, col, tx.escape("%"+value+"%")))
				return tx
			}
			start, err1 := utils.ParseInt64(strings.TrimSpace(vs[0]))
			end, err2 := utils.ParseInt64(strings.TrimSpace(vs[1]))
			if err1 != nil && err2 != nil {
				tx.pushConditions(fmt.Sprintf("%s%s LIKE '%s'", alias, col, tx.escape("%"+value+"%")))
			} else if err1 != nil {
				tx.pushConditions(fmt.Sprintf("%s%s<=%d", alias, col, end))
			} else if err2 != nil {
				tx.pushConditions(fmt.Sprintf("%s%s>=%d", alias, col, start))
			} else {
				tx.pushConditions(fmt.Sprintf("%s%s BETWEEN %d AND %d", alias, col, start, end))
			}
		} else {
			tx.pushConditions(fmt.Sprintf("%s%s LIKE '%s'", alias, col, tx.escape("%"+value+"%")))
		}

	}
	return tx
}

// AndFilters turns a map into AND conditions.
//
// Keys are sorted before generation so the same map always produces the same
// SQL (amicable with statement caches and reproducible in slow logs), and every
// value is escaped so a quote cannot break out of the literal.
//
// Keys prefixed with "$" and nil values are ignored; unsupported value types
// are silently skipped.
func (tx *Table) AndFilters(opts map[string]any, args ...string) *Table {
	if len(opts) == 0 {
		return tx
	}
	alias := ""
	if len(args) > 0 {
		alias = args[0]
	}
	// 有值的情况下，表示有多表
	if len(alias) > 0 {
		alias = fmt.Sprintf("%s.", alias)
	}
	keys := make([]string, 0, len(opts))
	for k := range opts {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		v := opts[k]
		if strings.HasPrefix(k, "$") {
			continue
		}
		if v == nil {
			continue
		}
		values, ok := flattenFilterValues(v)
		if !ok || len(values) == 0 {
			continue
		}
		if len(values) == 1 {
			lit, ok := sqlLiteral(tx, values[0])
			if !ok {
				continue
			}
			tx.pushConditions("AND")
			tx.pushConditions(fmt.Sprintf("%s%s = %s", alias, k, lit))
			continue
		}
		strs := make([]string, 0, len(values))
		for _, item := range values {
			lit, ok := sqlLiteral(tx, item)
			if !ok {
				continue
			}
			strs = append(strs, lit)
		}
		if len(strs) == 0 {
			continue
		}
		tx.pushConditions("AND")
		tx.pushConditions(fmt.Sprintf("%s%s IN (%s)", alias, k, strings.Join(strs, ",")))
	}
	return tx
}

// flattenFilterValues normalizes a filter value into a list of scalar values.
// []any and typed slices ([]int, []string, ...) are accepted.
func flattenFilterValues(v any) ([]any, bool) {
	switch val := v.(type) {
	case []any:
		return val, true
	case nil:
		return nil, false
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Slice, reflect.Array:
		if rv.Type().Elem().Kind() == reflect.Uint8 {
			// []byte 是单个值，不是集合
			return []any{v}, true
		}
		if rv.Kind() == reflect.Slice && rv.IsNil() {
			return nil, false
		}
		out := make([]any, 0, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			out = append(out, rv.Index(i).Interface())
		}
		return out, true
	default:
		return []any{v}, true
	}
}

// sqlLiteral renders a value as a safe SQL literal.
func sqlLiteral(t *Table, v any) (string, bool) {
	switch val := v.(type) {
	case nil:
		return "NULL", true
	case bool:
		if val {
			return "1", true
		}
		return "0", true
	case string:
		return "'" + t.escape(val) + "'", true
	case []byte:
		return "'" + t.escape(string(val)) + "'", true
	case int:
		return strconv.Itoa(val), true
	case int8:
		return strconv.FormatInt(int64(val), 10), true
	case int16:
		return strconv.FormatInt(int64(val), 10), true
	case int32:
		return strconv.FormatInt(int64(val), 10), true
	case int64:
		return strconv.FormatInt(val, 10), true
	case uint:
		return strconv.FormatUint(uint64(val), 10), true
	case uint8:
		return strconv.FormatUint(uint64(val), 10), true
	case uint16:
		return strconv.FormatUint(uint64(val), 10), true
	case uint32:
		return strconv.FormatUint(uint64(val), 10), true
	case uint64:
		return strconv.FormatUint(val, 10), true
	case float32:
		return strconv.FormatFloat(float64(val), 'f', -1, 32), true
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64), true
	case time.Time:
		return "'" + t.escape(val.Format("2006-01-02 15:04:05")) + "'", true
	default:
		return "", false
	}
}

func (t *Table) Where(cWhere string, values ...any) *Table {
	t.pWhere = append(t.pWhere, fmt.Sprintf(cWhere, values...))
	return t
}

/**
 * 满足条件使用
 * @param ok bool
 * @param string cWhere
 * @param mixed ...values
 * @return *Table
 */
func (t *Table) WhereOption(ok bool, cWhere string, values ...any) *Table {
	if ok {
		t.pWhere = append(t.pWhere, fmt.Sprintf(cWhere, values...))
	}
	return t
}

func (t *Table) GroupBy(col ...string) *Table {
	t.pGroupBy = append(t.pGroupBy, col...)
	return t
}

func (t *Table) OrderDESC(cols ...string) *Table {
	t.pOrderD = append(t.pOrderD, cols...)
	return t
}

func (t *Table) Desc(cols ...string) *Table {
	return t.OrderDESC(cols...)
}

func (t *Table) Asc(cols ...string) *Table {
	return t.Order(cols...)
}

func (t *Table) OrderASC(cols ...string) *Table {
	return t.Order(cols...)
}

func (t *Table) Order(cols ...string) *Table {
	t.pOrderA = append(t.pOrderA, cols...)
	return t
}

func (t *Table) OrderOption(ok bool, col string, ad string) *Table {
	if ok {
		if col == "" || ad == "" {
			return t
		}
		ad = strings.ToLower(ad)
		if strings.HasPrefix(ad, "a") {
			t.pOrderA = append(t.pOrderA, col)
		} else if strings.HasPrefix(ad, "d") {
			t.pOrderD = append(t.pOrderD, col)
		}
	}
	return t
}

/**
 * 按需排序
 */
func (t *Table) DescOption(ok bool, cols ...string) *Table {
	if ok {
		t.pOrderD = append(t.pOrderD, cols...)
	}
	return t
}

/**
 * 按需排序
 */
func (t *Table) AscOption(ok bool, cols ...string) *Table {
	if ok {
		t.pOrderA = append(t.pOrderA, cols...)
	}
	return t
}

func (t *Table) And(cAnd string, args ...any) *Table {
	t.pushConditions("AND")
	t.pushConditions(fmt.Sprintf(cAnd, args...))
	return t
}

func (tx *Table) Ands(strs []string) *Table {
	for _, str := range strs {
		tx.pushConditions("AND")
		tx.pushConditions(str)
	}
	return tx
}

func (t *Table) AndBetween(sta int, end int, column ...string) *Table {
	if len(column) == 0 {
		column = append(column, "intime")
	}
	t.pushConditions("AND")
	t.pushConditions(fmt.Sprintf("%s BETWEEN %d AND %d", column[0], sta, end))
	return t
}

func (t *Table) AndBetweenOption(ok bool, sta int, end int, column ...string) *Table {

	if len(column) == 0 {
		column = append(column, "intime")
	}
	if ok {
		t.pushConditions("AND")
		t.pushConditions(fmt.Sprintf("%s BETWEEN %d AND %d", column[0], sta, end))
	}
	return t
}

func (t *Table) AndOption(ok bool, cAnd string, args ...any) *Table {
	if ok {
		t.pushConditions("AND")
		t.pushConditions(fmt.Sprintf(cAnd, args...))
	}
	return t
}

func (t *Table) Or(cOr string, args ...any) *Table {

	t.pushConditions("OR")
	t.pushConditions(fmt.Sprintf(cOr, args...))
	return t
}

func (t *Table) Query() (*Row, error) {
	if err := t.check(); err != nil {
		return nil, err
	}
	query := t.getSql()
	rows, err := t.dbConn.Query(query)
	if err != nil {
		return nil, err
	}
	// GetRow owns rows from here and always closes them.
	return GetRow(rows)
}

// Rows exposes the underlying *sql.Rows. The caller owns them and MUST close
// them once done.
func (t *Table) Rows() (*sql.Rows, error) {
	if err := t.check(); err != nil {
		return nil, err
	}
	query := t.getSql()
	return t.dbConn.Query(query)
}

func (t *Table) QueryMulti() (*Rows, error) {
	if err := t.check(); err != nil {
		return nil, err
	}
	query := t.getSql()
	rows, err := t.dbConn.Query(query)
	if err != nil {
		return nil, err
	}
	// GetRows owns rows from here and always closes them.
	return GetRows(rows)
}

func (t *Table) Scan(target any) error {
	if target == nil {
		return ErrNilArgument
	}
	// 判断target是否为切片
	ty := reflect.TypeOf(target)
	if ty.Kind() != reflect.Pointer {
		return ErrUnsupportedType
	}
	if reflect.ValueOf(target).IsNil() {
		return ErrNilArgument
	}
	if ty.Elem().Kind() == reflect.Slice {
		rows, err := t.QueryMulti()
		if err != nil {
			return err
		}
		return rows.ScanMulti(target)
	}
	row, err := t.Query()
	if err != nil {
		return err
	}
	return row.ScanMulti(target)
}

// ScanMulti 扫描多行数据到切片
func (t *Table) ScanMulti(target any) error {
	if target == nil {
		return ErrNilArgument
	}
	// 判断target是否为切片
	ty := reflect.TypeOf(target)
	if ty.Kind() != reflect.Pointer {
		return ErrUnsupportedType
	}
	if reflect.ValueOf(target).IsNil() {
		return ErrNilArgument
	}
	if ty.Elem().Kind() != reflect.Slice {
		return ErrUnsupportedType
	}
	rows, err := t.QueryMulti()
	if err != nil {
		return err
	}
	return rows.ScanMulti(target)
}

func (t *Table) Lock() *Table {
	t.pLock = " FOR UPDATE "
	return t
}

func (t *Table) LockOption(lock bool) *Table {
	if lock {
		t.pLock = " FOR UPDATE "
	}
	return t
}

func (t *Table) Limit(pos int64, num ...int64) *Table {
	return t.LimitOption(true, pos, num...)
}

func (t *Table) LimitOption(ok bool, pos int64, num ...int64) *Table {
	if ok {
		var numb int64 = 0
		if len(num) > 0 {
			numb = num[0]
		}
		if pos <= 0 {
			pos = 0
		}
		if t.protocol == MYSQL {
			if numb <= 0 {
				t.pLimit = []int64{pos}
			} else {
				t.pLimit = []int64{pos, numb}
			}
			return t
		}
		// PostgreSQL 与 SQLite 使用标准的 LIMIT .. OFFSET ..
		if numb <= 0 {
			// 只给了一个参数时它就是行数
			t.pOffset = []int64{pos}
		} else {
			t.pOffset = []int64{numb, pos * numb}
		}
	}
	return t
}

func (t *Table) LimitOffset(num int64, offset ...int64) *Table {
	return t.LimitOffsetOption(true, num, offset...)
}

func (t *Table) LimitOffsetOption(ok bool, num int64, offset ...int64) *Table {
	if ok {
		var numb int64 = 0
		if len(offset) > 0 {
			numb = offset[0]
		}
		if num <= 0 {
			num = 0
		}
		if t.protocol == MYSQL {
			if numb <= 0 {
				t.pLimit = []int64{num}
			} else {
				t.pLimit = []int64{numb / num, num}
			}
			return t
		}
		if numb <= 0 {
			t.pOffset = []int64{num}
		} else {
			t.pOffset = []int64{num, numb}
		}
	}
	return t
}

// SQL returns the SELECT statement the builder would run. Useful for tests and
// for logging slow queries.
func (t *Table) SQL() string {
	return t.getSql()
}

func (t *Table) getSql() string {
	cols := t.pColumns
	if len(cols) <= 0 {
		cols = []string{"*"}
	}
	// 先估一次长度，省掉 builder 的反复扩容；下面所有列表都直接写进 builder，
	// 不再 strings.Join 出中间字符串。
	var sb strings.Builder
	sb.Grow(64 + len(t.pTable) + joinLen(cols) + joinLen(t.pWhere) +
		joinLen(t.pJoin) + joinLen(t.pGroupBy) + joinLen(t.pOrderA) + joinLen(t.pOrderD))
	sb.WriteString("SELECT ")
	for i, c := range cols {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(c)
	}
	sb.WriteString(" FROM ")
	sb.WriteString(t.table_prefix())
	for _, j := range t.pJoin {
		sb.WriteString(j) // join 自带前导空格
	}
	if hasConditions(t.pWhere) {
		sb.WriteString(" WHERE ")
		writeConditions(&sb, t.pWhere)
	}
	if len(t.pGroupBy) > 0 {
		sb.WriteString(" GROUP BY ")
		for i, c := range t.pGroupBy {
			if i > 0 {
				sb.WriteByte(',')
			}
			sb.WriteString(c)
		}
	}
	if len(t.pOrderA) > 0 {
		sb.WriteString(" ORDER BY ")
		for i, c := range t.pOrderA {
			if i > 0 {
				sb.WriteByte(',')
			}
			sb.WriteString(c)
		}
		sb.WriteString(" ASC")
		if len(t.pOrderD) > 0 {
			sb.WriteByte(',')
			for i, c := range t.pOrderD {
				if i > 0 {
					sb.WriteByte(',')
				}
				sb.WriteString(c)
			}
			sb.WriteString(" DESC")
		}
	} else if len(t.pOrderD) > 0 {
		sb.WriteString(" ORDER BY ")
		for i, c := range t.pOrderD {
			if i > 0 {
				sb.WriteByte(',')
			}
			sb.WriteString(c)
		}
		sb.WriteString(" DESC")
	}
	if len(t.pLimit) > 0 {
		sb.WriteString(" LIMIT ")
		if len(t.pLimit) == 1 {
			sb.WriteString(strconv.FormatInt(t.pLimit[0], 10))
		} else if len(t.pLimit) == 2 {
			sb.WriteString(strconv.FormatInt(t.pLimit[0], 10))
			sb.WriteByte(',')
			sb.WriteString(strconv.FormatInt(t.pLimit[1], 10))
		}
	} else if len(t.pOffset) > 0 {
		sb.WriteString(" LIMIT ")
		sb.WriteString(strconv.FormatInt(t.pOffset[0], 10))
		if len(t.pOffset) == 2 {
			sb.WriteString(" OFFSET ")
			sb.WriteString(strconv.FormatInt(t.pOffset[1], 10))
		}
	}

	if len(t.pLock) > 0 {
		sb.WriteString(t.pLock)
	}
	sql := sb.String()
	t.logger().Debug(sql)
	return sql
}

// joinLen estimates the length of a list once it is joined with separators.
// Only lengths are read, so sizing a builder with it costs no allocation.
func joinLen(list []string) int {
	n := 0
	for _, s := range list {
		n += len(s) + 1
	}
	return n
}

// writeConditions writes the WHERE chain into sb: entries are joined by a
// single space, empty entries are dropped and the leading AND/OR produced by
// And()/Or() is removed exactly once - the same rule the old trimCondition had.
// It reports whether anything was written.
func writeConditions(sb *strings.Builder, conds []string) bool {
	written := 0
	trimmed := false
	for _, c := range conds {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		if written == 0 && !trimmed && (c == "AND" || c == "OR") {
			trimmed = true
			continue
		}
		if written > 0 {
			sb.WriteByte(' ')
		}
		sb.WriteString(c)
		written++
	}
	return written > 0
}

// hasConditions reports whether the chain holds at least one real condition.
// It gates the " WHERE " keyword so it is never written on its own.
func hasConditions(conds []string) bool {
	for _, c := range conds {
		c = strings.TrimSpace(c)
		if c == "" || c == "AND" || c == "OR" {
			continue
		}
		return true
	}
	return false
}

// logger never returns nil so getSql can always trace.
func (t *Table) logger() StdLog {
	if t.log != nil {
		return t.log
	}
	return NewNoopLogger()
}

func (t *Table) Option(op string) *Table {
	t.pOption = op
	return t
}

func (t *Table) Select(columns ...string) *Table {
	if len(columns) == 0 {
		columns = []string{"*"}
	}
	t.pColumns = columns
	return t
}

func (t *Table) SelectWithAlias(alias string, cols ...string) *Table {
	columns := []string{}
	for _, v := range cols {
		columns = append(columns, fmt.Sprintf("%s.%s", alias, v))
	}
	t.pColumns = append(t.pColumns, columns...)
	return t
}

func (t *Table) SelectOption(ok bool, cols ...string) *Table {
	if ok {
		t.pColumns = append(t.pColumns, cols...)
	}
	return t
}

func (t *Table) Count() *Table {
	t.pOption = "select"
	t.pType = "data"
	t.pColumns = []string{"count(*) as total"}
	return t
}

// GetCount runs the count(*) built by Count() and returns its value.
func (t *Table) GetCount() (int64, error) {
	row, err := t.Count().Query()
	if err != nil {
		return 0, err
	}
	return row.Get("total").Int64()
}

func (t *Table) SelectMulti(columns ...string) *Table {
	t.pColumns = columns
	t.multi = true
	return t
}

func (t *Table) Delete(args ...string) *Table {
	t.pOption = "delete"
	return t
}

func (t *Table) Update(res map[string]any) *Table {
	t.pOption = "update"
	for k, v := range res {
		// 拼接代替 Sprintf：省掉一次格式化开销
		t.pData[k] = t.quoteIdent(k) + "='" + t.escape(utils.GetString(v)) + "'"
	}
	return t
}

// Set stores a raw assignment expression. The expression is only run through
// fmt.Sprintf when arguments are given, so a plain value containing a percent
// sign (think of "discount = '50%'") is kept as written instead of being
// mangled into %!(NOVERB).
func (t *Table) Set(value string, args ...any) *Table {
	t.pOption = "update_set"
	if len(args) == 0 {
		t.pData[value] = value
		return t
	}
	t.pData[value] = fmt.Sprintf(value, args...)
	return t
}

func (t *Table) SetOption(yep bool, value string, args ...any) *Table {
	if yep {
		return t.Set(value, args...)
	}
	return t
}

// writeUpdateData writes the SET assignments into sb in a stable order, so the
// same data always produces the same statement.
func (t *Table) writeUpdateData(sb *strings.Builder) {
	keys := make([]string, 0, len(t.pData))
	for k := range t.pData {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for i, k := range keys {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(t.pData[k])
	}
}

func (t *Table) Execute() (int64, error) {
	if err := t.check(); err != nil {
		return 0, err
	}
	var sb strings.Builder
	sb.Grow(32 + len(t.pTable) + joinLen(t.pWhere))
	switch t.pOption {
	case "update_set", "update":
		if !hasConditions(t.pWhere) {
			// 危险操作：无条件的UPDATE会影响整张表
			return 0, ErrMissingWhere
		}
		if len(t.pData) == 0 {
			return 0, ErrMissingValues
		}
		sb.WriteString("UPDATE ")
		sb.WriteString(t.table_prefix())
		sb.WriteString(" SET ")
		t.writeUpdateData(&sb)
		sb.WriteString(" WHERE ")
		writeConditions(&sb, t.pWhere)
	case "delete":
		if !hasConditions(t.pWhere) {
			// 危险操作：无条件的DELETE会清空整张表
			return 0, ErrMissingWhere
		}
		sb.WriteString("DELETE FROM ")
		sb.WriteString(t.table_prefix())
		sb.WriteString(" WHERE ")
		writeConditions(&sb, t.pWhere)
	default:
		return 0, ErrMissingOperation
	}
	sql := sb.String()
	t.logger().Debug(sql)
	stmt, err := t.dbConn.Prepare(sql)
	defer func() {
		if stmt != nil {
			stmt.Close()
		}
	}()
	if err != nil {
		return 0, err
	}
	rst, err := stmt.ExecContext(t.context())
	if err != nil {
		return 0, err
	}
	return rst.RowsAffected()
}

// isPostgres reports whether the protocol targets PostgreSQL.
func isPostgres(protocol string) bool {
	return protocol == POSTGRES || protocol == PG
}

// lastInsertID returns the key generated by the last INSERT. PostgreSQL has no
// LastInsertId, its drivers answer with an error, so the number of written rows
// is returned instead of failing the call.
func lastInsertID(rst sql.Result) (int64, error) {
	id, err := rst.LastInsertId()
	if err == nil {
		return id, nil
	}
	affected, aerr := rst.RowsAffected()
	if aerr != nil {
		return 0, err
	}
	return affected, nil
}
