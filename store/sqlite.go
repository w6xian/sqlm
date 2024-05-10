package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"

	// Import the SQLite driver.
	"github.com/w6xian/sqlm"
	"github.com/w6xian/sqlm/utils"
	_ "modernc.org/sqlite"
)

type Sqlite struct {
	conf        *sqlm.Server
	connection  *sql.DB
	isConnected bool
	log         sqlm.StdLog
	ctx         context.Context
}

func NewSqlite(opt *sqlm.Options) (*Sqlite, error) {
	return &Sqlite{conf: &opt.Server, log: opt.GetLogger(), isConnected: false}, nil

}

func (m *Sqlite) NewConn(conn *sql.DB, isConnected bool) (sqlm.DbConn, error) {
	return &Sqlite{conf: m.conf, connection: conn, isConnected: false}, nil
}

func (m *Sqlite) Conf() *sqlm.Server {
	return m.conf
}

func (m *Sqlite) Ping() error {
	if err := m.check(); err != nil {
		return err
	}
	return m.connection.PingContext(m.ctx)
}
func (m *Sqlite) Conn() (*sql.DB, error) {
	return m.connection, nil
}
func (m *Sqlite) Close() error {
	if err := m.check(); err != nil {
		return err
	}
	return m.connection.Close()
}

func (m *Sqlite) check() error {
	if m.connection == nil {
		return errors.New("请设置数据库链接")
	}
	if err := m.connection.PingContext(m.ctx); err != nil {
		return err
	}
	return nil
}

func (m *Sqlite) Connect(ctx context.Context) (sqlm.DbConn, error) {
	// Connect to the database with some sane settings:
	// - No shared-cache: it's obsolete; WAL journal mode is a better solution.
	// - No foreign key constraints: it's currently disabled by default, but it's a
	// good practice to be explicit and prevent future surprises on SQLite upgrades.
	// - Journal mode set to WAL: it's the recommended journal mode for most applications
	// as it prevents locking issues.
	//
	// Notes:
	// - When using the `modernc.org/sqlite` driver, each pragma must be prefixed with `_pragma=`.
	//
	// References:
	// - https://pkg.go.dev/modernc.org/sqlite#Driver.Open
	// - https://www.sqlite.org/sharedcache.html
	// - https://www.sqlite.org/pragma.html

	source := fmt.Sprintf("%s?%s", m.conf.DSN, "_pragma=foreign_keys(0)&_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)")

	conn, err := sql.Open(
		m.conf.Protocol,
		source,
	)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open db with dsn: %s", m.conf.DSN)
	}
	err = conn.Ping()
	if err != nil {
		return nil, err
	}
	m.isConnected = true
	m.connection = conn
	conn.SetMaxOpenConns(m.conf.MaxOpenConns)
	conn.SetMaxIdleConns(m.conf.MaxIdleConns)
	conn.SetConnMaxLifetime(time.Duration(m.conf.MaxLifetime))
	newconn, _ := m.NewConn(conn, true)
	newconn.WithContext(ctx)
	return newconn, nil
}

func (m *Sqlite) WithContext(ctx context.Context) {
	m.ctx = ctx
}

func (m *Sqlite) Delete(query string, args ...interface{}) (*sql.Rows, error) {
	if err := m.check(); err != nil {
		return nil, errors.New("does not connected")
	}
	return m.connection.QueryContext(m.ctx, query, args...)
}

func (m *Sqlite) Prepare(query string) (*sql.Stmt, error) {
	if err := m.check(); err != nil {
		return nil, err
	}
	return m.connection.PrepareContext(m.ctx, query)
}

func (m *Sqlite) Query(query string, args ...interface{}) (*sql.Rows, error) {

	if err := m.check(); err != nil {
		return nil, err
	}
	return m.connection.QueryContext(m.ctx, query, args...)
}

func (m *Sqlite) Exec(query string, args ...interface{}) (sql.Result, error) {
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

func (m *Sqlite) Insert(pTable string, columns []string, data []interface{}) (int64, error) {
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
func (m *Sqlite) Inserts(pTable string, columns []string, data [][]interface{}) (int64, error) {

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
