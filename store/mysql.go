package store

import (
	"database/sql"
	"fmt"
	"strings"

	"errors"

	_ "github.com/go-sql-driver/mysql"
	"github.com/w6xian/sqlm"
)

type Mysql struct {
	conf        *sqlm.Server
	connection  *sql.DB
	isConnected bool
	log         sqlm.StdLog
}

func NewMysql(opt *sqlm.Options) (*Mysql, error) {
	return &Mysql{conf: &opt.Server, log: opt.GetLogger(), isConnected: false}, nil

}

func (m *Mysql) NewConn(opt sqlm.Server) (sqlm.DbConn, error) {
	return &Mysql{conf: &opt, isConnected: false}, nil
}

func (m *Mysql) Conf() *sqlm.Server {
	return m.conf
}

func (m *Mysql) Ping() error {
	if err := m.check(); err != nil {
		return err
	}
	return m.connection.Ping()
}
func (m *Mysql) Conn() (*sql.DB, error) {
	return m.connection, nil
}
func (m *Mysql) Close() error {
	if err := m.check(); err != nil {
		return err
	}
	return m.connection.Close()
}

func (m *Mysql) check() error {
	if m.connection == nil {
		return errors.New("请设置数据库链接")
	}
	if err := m.connection.Ping(); err != nil {
		return err
	}
	return nil
}

func (m *Mysql) Connect() error {

	source := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s", m.conf.Username, m.conf.Password, m.conf.Host, m.conf.Port, m.conf.Database, m.conf.Charset)

	if strings.HasPrefix(m.conf.Host, "unix:") {
		parts := strings.SplitN(m.conf.Host, ":", 2)
		socketPath := parts[1]
		// mariadbUser+":"+mariadbPassword+"@unix("+socketPath+")"+"/"+mariadbDatabase+"?charset=utf8&parseTime=True"
		source = fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s", m.conf.Username, m.conf.Password, socketPath, m.conf.Port, m.conf.Database, m.conf.Charset)
	}

	conn, err := sql.Open(
		m.conf.Protocol,
		source,
	)
	if err != nil {
		return err
	}
	err = conn.Ping()
	if err != nil {
		return err
	}
	m.isConnected = true
	m.connection = conn
	conn.SetMaxOpenConns(m.conf.Maxconnetion)
	return nil
}

func (m *Mysql) Delete(query string, args ...interface{}) (*sql.Rows, error) {
	if err := m.check(); err != nil {
		return nil, errors.New("does not connected")
	}
	return m.connection.Query(query, args...)
}

func (m *Mysql) Prepare(query string) (*sql.Stmt, error) {
	if err := m.check(); err != nil {
		return nil, err
	}
	return m.connection.Prepare(query)
}
func (m *Mysql) Query(query string, args ...interface{}) (*sql.Rows, error) {

	if err := m.check(); err != nil {
		return nil, err
	}
	return m.connection.Query(query, args...)
}

func (m *Mysql) Exec(query string, args ...interface{}) (sql.Result, error) {
	if err := m.check(); err != nil {
		return nil, err
	}
	stmt, err := m.connection.Prepare(query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()
	rst, err := stmt.Exec(args...)
	if err != nil {
		return nil, err
	}
	return rst, nil
}

func (m *Mysql) Insert(pTable string, columns []string, data []interface{}) (int64, error) {
	if len(columns) != len(data) {
		return 0, errors.New("请确保column长度统一")
	}
	if err := m.check(); err != nil {
		return 0, err
	}
	for k, v := range columns {
		columns[k] = "`" + strings.Trim(v, "`") + "`"
	}
	sql := fmt.Sprintf("INSERT INTO `%s` (%s) VALUES (%s)", pTable, strings.Join(columns, ","), strings.Join(m.buildSqlQ(len(data)), ","))

	stmt, err := m.connection.Prepare(sql)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()
	rst, err := stmt.Exec(data...)
	if err != nil {
		return 0, err
	}
	return rst.LastInsertId()
}

/**
 * 为了执行效率，请自行保证query中需要的参数个数与后面的参数中数组长度相对应
 */
func (m *Mysql) Inserts(pTable string, columns []string, data [][]interface{}) (int64, error) {

	if err := m.check(); err != nil {
		return 0, err
	}

	colLen := len(columns)
	if colLen <= 0 {
		return 0, errors.New("请提供字段")
	}
	for k, v := range columns {
		columns[k] = "`" + strings.Trim(v, "`") + "`"
	}
	qstr := "(" + strings.Join(m.buildSqlQ(len(columns)), ",") + ")"
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

	sql := fmt.Sprintf("INSERT INTO `%s` (%s) VALUES %s", pTable, strings.Join(columns, ","), strings.Join(qarr, ","))
	stmt, err := m.connection.Prepare(sql)
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

func (m *Mysql) buildSqlQ(num int) []string {
	rst := []string{}
	for i := 0; i < num; i++ {
		rst = append(rst, "?")
	}
	return rst
}
