package sqlm

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/w6xian/sqlm/utils"
)

type KeyValue map[string]interface{}

type Table struct {
	pTable   string   `sql:"table"`
	pJoin    []string `sql:"join"`
	pWhere   []string `sql:"where"`
	pGroupBy []string `sql:"group by"`
	pOrderD  []string `sql:"order by cols desc"`
	pOrderA  []string `sql:"order by cols asc"`
	pLimit   []int64  `sql:"limit"`
	pColumns []string
	pData    map[string]string
	pOption  string
	pLock    string
	pType    string
	multi    bool
	dbConn   TxConn
	pPre     string
	db       *Db
	log      StdLog
}

func NewTable(tle string) *Table {
	pt := &Table{}
	pt.pColumns = []string{}
	pt.pData = make(map[string]string)
	pt.pOption = "select"
	pt.pType = "array"
	pt.pTable = tle
	pt.pPre = ""
	return pt
}

func (t *Table) PreTable(pre string) *Table {
	t.pPre = pre
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

func (t *Table) LeftJoin(tbl string, onKey string, args ...interface{}) *Table {
	return t.join("LEFT", tbl, onKey, args...)
}

func (t *Table) RightJoin(tbl string, onKey string, args ...interface{}) *Table {
	return t.join("RIGHT", tbl, onKey, args...)
}

func (t *Table) InnerJoin(tbl string, onKey string, args ...interface{}) *Table {
	return t.join("INNER", tbl, onKey, args...)
}

func (t *Table) join(option string, tbl string, onKey string, args ...interface{}) *Table {
	t.pJoin = append(t.pJoin, fmt.Sprintf(" %s JOIN %s ON %s", option, fmt.Sprintf("%s%s", t.pPre, tbl), fmt.Sprintf(onKey, args...)))
	return t
}

func (t *Table) pushConditions(w string) *Table {
	t.pWhere = append(t.pWhere, w)
	return t
}

// func (t *Table) UseSlave(dbName string, server string) *Table {
// 	t.Server = NewServer(server)
// 	t.Database = dbName
// 	return t
// }

// // 满足条件使用
// func (t *Table) UseSlaveOption(ok bool, dbName string, server string) *Table {
// 	if ok {
// 		t.Server = NewServer(server)
// 		t.Database = dbName
// 	}
// 	return t
// }

// func (t *Table) UseMaster(dbName string, server string) *Table {

// 	t.Server = NewServer(server)
// 	t.Database = dbName
// 	return t
// }

func (t *Table) check() error {
	if t.dbConn == nil {
		return errors.New("请调用UseConn方法后再执行")
	}
	return nil
}

func (t *Table) Insert(data map[string]interface{}) (int64, error) {
	if err := t.check(); err != nil {
		return 0, err
	}
	columns := []string{}
	values := []interface{}{}
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
	rst, err := stmt.Exec(values...)
	if err != nil {
		return 0, err
	}
	return rst.LastInsertId()
}

func (t *Table) Inserts(columns []string, data [][]interface{}) (int64, error) {
	if err := t.check(); err != nil {
		return 0, err
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
	for _, v := range data {
		if colLen != len(v) {
			return 0, errors.New("请确保column长度统一")
		}
		qarr = append(qarr, qstr)
	}
	val := []interface{}{}
	for {
		if len(data) > 1 {
			break
		}
		d := data[0]
		val = append(val, d...)
		data = data[1:]
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

func (tx *Table) AndFilters(opts map[string]interface{}, args ...string) *Table {

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
		case []interface{}:
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
		case string:
			tx.pushConditions("AND")
			tx.pushConditions(fmt.Sprintf("%s%s='%s'", alias, k, val))
		case float64:
			tx.pushConditions("AND")
			tx.pushConditions(fmt.Sprintf("%s%s=%f", alias, k, val))
		default:
			fmt.Printf("\r\n%v\r\n", val)
			continue
		}
	}
	return tx
}

func (t *Table) Where(cWhere string, values ...interface{}) *Table {
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
func (t *Table) WhereOption(ok bool, cWhere string, values ...interface{}) *Table {
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

func (t *Table) And(cAnd string, args interface{}) *Table {
	t.pushConditions("AND")
	t.pushConditions(fmt.Sprintf(cAnd, args))
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

func (t *Table) AndOption(ok bool, cAnd string, args ...interface{}) *Table {
	if ok {
		t.pushConditions("AND")
		t.pushConditions(fmt.Sprintf(cAnd, args...))
	}
	return t
}

func (t *Table) Or(cOr string, args ...interface{}) *Table {

	t.pushConditions("OR")
	t.pushConditions(fmt.Sprintf(cOr, args...))
	return t
}

// func (t *Table) Max(column) string{
//     sql = "select max(:column) from :table";
//     return D(t.db, t.host)->GetOne(sql, array(
//         "column" => column,
//         "table" => t.table_prefix()
//     ));
// }

func (t *Table) Query() (*Row, error) {
	if err := t.check(); err != nil {
		return nil, err
	}
	query := t.getSql()
	rows, err := t.dbConn.Query(query)
	if err == nil {
		defer rows.Close()
		return GetRow(rows)
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
	var numb int64 = 0
	if len(num) > 0 {
		numb = num[0]
	}
	if pos+numb == 0 {
		return t
	}
	if pos <= 0 {
		pos = 0
	}
	if numb <= 0 {
		t.pLimit = []int64{pos}
	} else {
		t.pLimit = []int64{pos, numb}
	}
	return t
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
		if numb <= 0 {
			t.pLimit = []int64{pos}
		} else {
			t.pLimit = []int64{pos, numb}
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
	if len(t.pLimit) == 1 {
		sql = sql + " LIMIT " + strconv.FormatInt(t.pLimit[0], 10)
	}
	if len(t.pLimit) == 2 {
		sql = sql + fmt.Sprintf(" LIMIT %d,%d", t.pLimit[0], t.pLimit[1])
	}
	if len(t.pLock) > 0 {
		sql = sql + t.pLock
	}
	t.log.Debug(sql)
	return sql
}

func (t *Table) option(op string) *Table {
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

func (t *Table) Set(value string, args ...interface{}) *Table {
	t.pOption = "update_set"
	t.pData[value] = fmt.Sprintf(value, args...)
	return t
}

func (t *Table) Update(res map[string]interface{}) *Table {
	t.pOption = "update"
	for k, v := range res {
		t.pData[k] = fmt.Sprintf("`%s`='%s'", k, utils.GetString(v))
	}
	return t
}

func (t *Table) SetOption(yep bool, value string, args ...interface{}) *Table {
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
	if option == "update_set" || option == "update" {
		if len(where) <= 0 {
			panic("sql set | Update 中需要设置Where条件")
		}
		sql = fmt.Sprintf("UPDATE %s SET %s WHERE %s", t.table_prefix(), strings.Join(t.getUpdateData(), ","), where)
	} else if option == "delete" {
		if len(where) <= 0 {
			panic("sql Delete 中需要设置Where条件")
		}
		sql = fmt.Sprintf("DELETE FROM %s WHERE %s", t.table_prefix(), strings.Join(t.pWhere, " "))
	}
	if len(sql) <= 0 {
		panic("sql Execute 中需要操作")
	}
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
