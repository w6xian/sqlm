package sqlm

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

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
func (t *Table) table_prefix() string {
	dt := strings.Split(t.pTable, ".")
	if len(dt) == 2 {
		// t.dbConn.Conf().Database = dt[0]
		return fmt.Sprintf("%s%s", t.pPre, dt[1])
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

func (t *Table) check() error {
	if t.dbConn == nil {
		return errors.New("请调用UseConn方法后再执行")
	}
	db, err := t.db.conn.Conn()
	if err != nil {
		return err
	}
	if err = db.Ping(); err != nil {
		return err
	}

	return nil
}

func (t *Table) Insert(data map[string]any) (int64, error) {
	if err := t.check(); err != nil {
		return 0, err
	}
	columns := []string{}
	values := []any{}
	for c, v := range data {
		columns = append(columns, c)
		values = append(values, v)
	}
	for k, v := range columns {
		columns[k] = "`" + strings.Trim(v, "`") + "`"
	}
	sql := fmt.Sprintf("INSERT INTO `%s` (%s) VALUES (%s)", t.table_prefix(), strings.Join(columns, ","), strings.Join(t.buildSqlQ(len(values)), ","))

	stmt, err := t.dbConn.Prepare(sql)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()
	rst, err := stmt.ExecContext(t.ctx, values...)
	if err != nil {
		return 0, err
	}
	return rst.LastInsertId()
}

func (t *Table) Inserts(columns []string, data [][]any) (int64, error) {
	if err := t.check(); err != nil {
		return 0, err
	}

	if len(data) <= 0 {
		return 0, errors.New("请提供数据")
	}
	colLen := len(columns)
	if colLen <= 0 {
		return 0, errors.New("请提供字段")
	}
	for k, v := range columns {
		columns[k] = "`" + strings.Trim(v, "`") + "`"
	}
	qstr := "(" + strings.Join(t.buildSqlQ(len(columns)), ",") + ")"
	qarr := []string{}
	val := []any{}
	for _, v := range data {
		if colLen != len(v) {
			return 0, errors.New("请确保column长度统一")
		}
		qarr = append(qarr, qstr)
		val = append(val, v...)
	}

	sql := fmt.Sprintf("INSERT INTO `%s` (%s) VALUES %s", t.table_prefix(), strings.Join(columns, ","), strings.Join(qarr, ","))
	stmt, err := t.dbConn.Prepare(sql)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()
	rst, err := stmt.Exec(val...)
	if err != nil {
		return 0, err
	}
	return rst.LastInsertId()
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
		if strings.HasPrefix(value, "[") && strings.HasPrefix(value, "]") {
			// 区间查询，时间和价格区间
			vs := strings.Split(value, ",")
			start, err1 := utils.ParseInt64(vs[0])
			end, err2 := utils.ParseInt64(vs[1])
			if err1 != nil {
				tx.pushConditions(fmt.Sprintf("%s%s<=%d", alias, col, end))
			} else if err2 != nil {
				tx.pushConditions(fmt.Sprintf("%s%s>=%d", alias, col, start))
			} else {
				tx.pushConditions(fmt.Sprintf("%s%s BETWEEN %d AND %d", alias, col, start, end))
			}
		} else {
			str := fmt.Sprintf("%s%s like '%s'", alias, col, "%"+value+"%")
			tx.pushConditions(str)
		}

	}
	return tx
}

func (tx *Table) AndFilters(opts map[string]any, args ...string) *Table {

	if len(args) == 0 {
		args = append(args, "")
	}
	alias := args[0]
	// 有值的情况下，表示有多表
	if len(alias) > 0 {
		alias = fmt.Sprintf("%s.", alias)
	}
	for k, v := range opts {
		if strings.HasPrefix(k, "$") {
			continue
		}
		if v == nil {
			continue
		}

		switch val := v.(type) {
		case []any:
			l := len(val)
			if l <= 0 {
				continue
			}
			tx.pushConditions("AND")
			if l == 1 {
				tx.pushConditions(fmt.Sprintf("%s%s='%v'", alias, k, val[0]))
			} else {
				strs := []string{}
				for _, v := range val {
					strs = append(strs, fmt.Sprintf("'%v'", v))
				}
				tx.pushConditions(fmt.Sprintf("%s%s in (%s)", alias, k, strings.Join(strs, ",")))
			}
		case float64, float32:
			tx.pushConditions("AND")
			tx.pushConditions(fmt.Sprintf("%s%s=%f", alias, k, val))
		case int, int8, int16, int32, int64, uint, uint16, uint32, uint64, byte:
			tx.pushConditions("AND")
			tx.pushConditions(fmt.Sprintf("%s%s=%d", alias, k, val))
		default:
			fmt.Printf("\r\n%v\r\n", val)
			continue
		}
	}
	return tx
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
		fmt.Println(err.Error())
		return nil, err
	}
	query := t.getSql()
	rows, err := t.dbConn.Query(query)
	if err == nil {
		defer rows.Close()
		return GetRow(rows)
	}
	t.db.Close()
	return nil, err
}
func (t *Table) Rows() (*sql.Rows, error) {
	if err := t.check(); err != nil {
		return nil, err
	}
	query := t.getSql()
	rows, err := t.dbConn.Query(query)
	if err == nil {
		defer rows.Close()
		return rows, err
	}
	return nil, err
}

func (t *Table) QueryMulti() (*Rows, error) {
	if err := t.check(); err != nil {
		return nil, err
	}
	query := t.getSql()
	rows, err := t.dbConn.Query(query)
	if err == nil {
		defer rows.Close()
		return GetRows(rows)
	}
	return nil, err
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
		if t.protocol == SQLITE {
			if numb <= 0 {
				t.pOffset = []int64{numb}
			} else {
				t.pOffset = []int64{numb, pos * numb}
			}
			return t
		}
		if numb <= 0 {
			t.pLimit = []int64{pos}
		} else {
			t.pLimit = []int64{pos, numb}
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

func (t *Table) getSql() string {
	if len(t.pColumns) <= 0 {
		t.pColumns = []string{"*"}
	}
	columns := strings.Join(t.pColumns, ",")
	sql := "SELECT " + columns + " FROM " + t.table_prefix()
	if len(t.pJoin) > 0 {
		sql = sql + strings.Join(t.pJoin, " ")
	}
	if len(t.pWhere) > 0 {
		sql = sql + " WHERE " + strings.Join(t.pWhere, " ")
	}
	if len(t.pGroupBy) > 0 {
		sql = sql + " GROUP BY " + strings.Join(t.pGroupBy, ",")
	}
	if len(t.pOrderA) > 0 {
		sql = sql + " ORDER BY " + strings.Join(t.pOrderA, ",") + " ASC"
		if len(t.pOrderD) > 0 {
			sql = sql + "," + strings.Join(t.pOrderD, ",") + " DESC"
		}
	} else if len(t.pOrderD) > 0 {
		sql = sql + " ORDER BY " + strings.Join(t.pOrderD, ",") + " DESC"
	}
	if len(t.pLimit) > 0 {
		if len(t.pLimit) == 1 {
			sql = sql + " LIMIT " + strconv.FormatInt(t.pLimit[0], 10)
		}
		if len(t.pLimit) == 2 {
			sql = sql + fmt.Sprintf(" LIMIT %d,%d", t.pLimit[0], t.pLimit[1])
		}
	} else if len(t.pOffset) > 0 {
		if len(t.pOffset) == 1 {
			sql = sql + fmt.Sprintf(" LIMIT %d", t.pOffset[0])
		}
		if len(t.pOffset) == 2 {
			sql = sql + fmt.Sprintf(" LIMIT %d OFFSET %d", t.pOffset[0], t.pOffset[1])
		}
	}

	if len(t.pLock) > 0 {
		sql = sql + t.pLock
	}
	t.log.Debug(sql)
	return sql
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

func (t *Table) SelectMulti(columns ...string) *Table {
	t.pColumns = columns
	t.multi = true
	return t
}

func (t *Table) Delete(args ...string) *Table {
	t.pOption = "delete"
	return t
}

func (t *Table) Set(value string, args ...any) *Table {
	t.pOption = "update_set"
	t.pData[value] = fmt.Sprintf(value, args...)
	return t
}

func (t *Table) Update(res map[string]any) *Table {
	t.pOption = "update"
	for k, v := range res {
		t.pData[k] = fmt.Sprintf("`%s`='%s'", k, utils.GetString(v))
	}
	return t
}

func (t *Table) SetOption(yep bool, value string, args ...any) *Table {
	if yep {
		t.pOption = "update_set"
		t.pData[value] = fmt.Sprintf(value, args...)
	}
	return t
}

func (t *Table) getUpdateData() []string {
	data := []string{}
	for _, v := range t.pData {
		data = append(data, v)
	}
	return data
}

func (t *Table) Execute() (int64, error) {
	if err := t.check(); err != nil {
		return 0, err
	}
	option := t.pOption
	sql := ""
	where := strings.Join(t.pWhere, " ")
	switch option {
	case "update_set", "update":
		if len(where) <= 0 {
			panic("sql set | Update 中需要设置Where条件")
		}
		sql = fmt.Sprintf("UPDATE %s SET %s WHERE %s", t.table_prefix(), strings.Join(t.getUpdateData(), ","), where)
	case "delete":
		if len(where) <= 0 {
			panic("sql Delete 中需要设置Where条件")
		}
		sql = fmt.Sprintf("DELETE FROM %s WHERE %s", t.table_prefix(), strings.Join(t.pWhere, " "))
	}
	if len(sql) <= 0 {
		panic("sql Execute 中需要操作")
	}
	t.log.Debug(sql)
	stmt, err := t.dbConn.Prepare(sql)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()
	rst, err := stmt.Exec()
	if err != nil {
		return 0, err
	}
	return rst.RowsAffected()
}

func (t *Table) buildSqlQ(num int) []string {
	rst := []string{}
	for i := 0; i < num; i++ {
		rst = append(rst, "?")
	}
	return rst
}
