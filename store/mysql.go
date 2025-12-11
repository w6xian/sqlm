package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"errors"

	_ "github.com/go-sql-driver/mysql"
	"github.com/w6xian/sqlm"
	"github.com/w6xian/sqlm/utils"
)

type Mysql struct {
	options     *sqlm.Options
	conf        *sqlm.Server
	connection  *sql.DB
	isConnected bool
	log         sqlm.StdLog
	ctx         context.Context
}

func NewMysql(opt *sqlm.Options) (Driver, error) {
	return &Mysql{options: opt, conf: opt.Server, log: opt.GetLogger(), isConnected: false}, nil

}

func (m *Mysql) NewConn(conn *sql.DB, isConnected bool) (sqlm.DbConn, error) {
	return &Mysql{conf: m.conf, connection: conn, isConnected: isConnected}, nil
}

func (m *Mysql) Conf() *sqlm.Server {
	return m.conf
}

func (m *Mysql) Options() *sqlm.Options {
	return m.options
}

func (m *Mysql) Ping() error {
	if err := m.check(); err != nil {
		return err
	}
	return m.connection.PingContext(m.ctx)
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

func (m *Mysql) Connect(ctx context.Context) (sqlm.DbConn, error) {
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
		return nil, err
	}
	err = conn.Ping()
	if err != nil {
		return nil, err
	}
	conn.SetMaxOpenConns(m.conf.MaxOpenConns)
	conn.SetMaxIdleConns(m.conf.MaxIdleConns)
	conn.SetConnMaxLifetime(time.Duration(m.conf.MaxLifetime))
	newConn, err := m.NewConn(conn, true)
	newConn.WithContext(ctx)
	if err != nil {
		return nil, err
	}
	return newConn, nil
}
func (m *Mysql) WithContext(ctx context.Context) {
	m.ctx = ctx
}

func (m *Mysql) Delete(query string, args ...any) (*sql.Rows, error) {
	if err := m.check(); err != nil {
		return nil, errors.New("does not connected")
	}
	return m.connection.QueryContext(m.ctx, query, args...)
}

func (m *Mysql) Prepare(query string) (*sql.Stmt, error) {
	if err := m.check(); err != nil {
		return nil, err
	}
	return m.connection.PrepareContext(m.ctx, query)
}
func (m *Mysql) Query(query string, args ...any) (*sql.Rows, error) {
	if err := m.check(); err != nil {
		return nil, err
	}
	return m.connection.QueryContext(m.ctx, query, args...)
}

func (m *Mysql) Exec(query string, args ...any) (sql.Result, error) {
	if err := m.check(); err != nil {
		return nil, err
	}
	stmt, err := m.connection.PrepareContext(m.ctx, query)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()
	rst, err := stmt.ExecContext(m.ctx, args...)
	if err != nil {
		return nil, err
	}
	return rst, nil
}

func (m *Mysql) Insert(pTable string, columns []string, data []any) (int64, error) {
	if len(columns) != len(data) {
		return 0, errors.New("请确保column长度统一")
	}
	if err := m.check(); err != nil {
		return 0, err
	}
	for k, v := range columns {
		columns[k] = "`" + strings.Trim(v, "`") + "`"
	}
	sql := fmt.Sprintf("INSERT INTO `%s` (%s) VALUES (%s)", pTable, strings.Join(columns, ","), strings.Join(utils.BuildSqlQ(len(data)), ","))

	stmt, err := m.connection.PrepareContext(m.ctx, sql)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()
	rst, err := stmt.ExecContext(m.ctx, data...)
	if err != nil {
		return 0, err
	}
	return rst.LastInsertId()
}

/**
 * 为了执行效率，请自行保证query中需要的参数个数与后面的参数中数组长度相对应
 */
func (m *Mysql) Inserts(pTable string, columns []string, data [][]any) (int64, error) {

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
	qstr := "(" + strings.Join(utils.BuildSqlQ(len(columns)), ",") + ")"
	qarr := []string{}
	for _, v := range data {
		if colLen != len(v) {
			return 0, errors.New("请确保column长度统一")
		}
		qarr = append(qarr, qstr)
	}
	val := []any{}
	for {
		if len(data) > 1 {
			break
		}
		d := data[0]
		val = append(val, d...)
		data = data[1:]
	}

	sql := fmt.Sprintf("INSERT INTO `%s` (%s) VALUES %s", pTable, strings.Join(columns, ","), strings.Join(qarr, ","))
	stmt, err := m.connection.PrepareContext(m.ctx, sql)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()
	rst, err := stmt.ExecContext(m.ctx, val...)
	if err != nil {
		return 0, err
	}
	return rst.LastInsertId()
}
