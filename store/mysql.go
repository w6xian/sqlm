package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/pkg/errors"

	_ "github.com/go-sql-driver/mysql"
	"github.com/w6xian/sqlm"
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
	if opt == nil || opt.Server == nil {
		return nil, errors.New("mysql: options is required")
	}
	return &Mysql{options: opt, conf: opt.Server, log: opt.Logger(), isConnected: false}, nil
}

func (m *Mysql) NewConn(conn *sql.DB, isConnected bool) (sqlm.DbConn, error) {
	return &Mysql{options: m.options, conf: m.conf, log: m.log, connection: conn, isConnected: isConnected}, nil
}

// NewServerConn opens an independent connection to another server (replica)
// reusing the driver options. It implements sqlm.ServerSwitcher.
func (m *Mysql) NewServerConn(ctx context.Context, svr *sqlm.Server) (sqlm.DbConn, error) {
	if svr == nil {
		return nil, errors.New("mysql: server config is nil")
	}
	opts := cloneOptions(m.options, svr)
	n := &Mysql{options: opts, conf: svr, log: opts.Logger(), isConnected: false}
	return n.Connect(ctx)
}

func (m *Mysql) Conf() *sqlm.Server {
	return m.conf
}

func (m *Mysql) Options() *sqlm.Options {
	return m.options
}

func (m *Mysql) Ping() error {
	if err := checkConnection(m.connection); err != nil {
		return err
	}
	return m.connection.PingContext(ctxOrBackground(m.ctx))
}
func (m *Mysql) Conn() (*sql.DB, error) {
	if err := checkConnection(m.connection); err != nil {
		return nil, err
	}
	return m.connection, nil
}
func (m *Mysql) Close() error {
	if err := checkConnection(m.connection); err != nil {
		return err
	}
	return m.connection.Close()
}

func (m *Mysql) check() error {
	return checkConnection(m.connection)
}

func (m *Mysql) Connect(ctx context.Context) (sqlm.DbConn, error) {
	ctx = ctxOrBackground(ctx)
	if m.connection != nil {
		if m.isConnected {
			// 已建立的连接池直接复用，避免每次取实例都多一次 Ping 往返
			newConn, _ := m.NewConn(m.connection, true)
			newConn.WithContext(ctx)
			return newConn, nil
		}
		if err := m.connection.PingContext(ctx); err == nil {
			newConn, _ := m.NewConn(m.connection, true)
			newConn.WithContext(ctx)
			return newConn, nil
		}
	}

	source := mysqlSource(m.conf)

	conn, err := sql.Open(
		m.conf.Protocol,
		source,
	)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open db with protocol: %s", m.conf.Protocol)
	}
	applyPool(conn, m.conf)
	if err = conn.PingContext(ctx); err != nil {
		conn.Close()
		return nil, err
	}

	m.connection = conn
	m.isConnected = true

	newConn, err := m.NewConn(conn, true)
	if err != nil {
		return nil, err
	}
	newConn.WithContext(ctx)
	return newConn, nil
}

// mysqlSource builds the DSN, supporting both tcp hosts and unix sockets.
func mysqlSource(conf *sqlm.Server) string {
	if strings.HasPrefix(conf.Host, "unix:") {
		socketPath := strings.TrimPrefix(conf.Host, "unix:")
		return fmt.Sprintf("%s:%s@unix(%s)/%s?charset=%s", conf.Username, conf.Password, socketPath, conf.Database, conf.Charset)
	}
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s", conf.Username, conf.Password, conf.Host, conf.Port, conf.Database, conf.Charset)
}

func (m *Mysql) WithContext(ctx context.Context) {
	m.ctx = ctxOrBackground(ctx)
}

func (m *Mysql) Delete(query string, args ...any) (*sql.Rows, error) {
	if err := m.check(); err != nil {
		return nil, err
	}
	return m.connection.QueryContext(ctxOrBackground(m.ctx), query, args...)
}

func (m *Mysql) Prepare(query string) (*sql.Stmt, error) {
	if err := m.check(); err != nil {
		return nil, err
	}
	return m.connection.PrepareContext(ctxOrBackground(m.ctx), query)
}
func (m *Mysql) Query(query string, args ...any) (*sql.Rows, error) {
	if err := m.check(); err != nil {
		return nil, err
	}
	return m.connection.QueryContext(ctxOrBackground(m.ctx), query, args...)
}

func (m *Mysql) Exec(query string, args ...any) (sql.Result, error) {
	if err := m.check(); err != nil {
		return nil, err
	}
	// ExecContext prepares internally (and reuses the prepared statement when
	// available), which saves one round trip versus an explicit Prepare.
	return m.connection.ExecContext(ctxOrBackground(m.ctx), query, args...)
}

func (m *Mysql) Insert(pTable string, columns []string, data []any) (int64, error) {
	if len(columns) != len(data) {
		return 0, errors.New("请确保column长度统一")
	}
	if err := m.check(); err != nil {
		return 0, err
	}
	sqlStr := insertSQL(pTable, columns, 1)
	rst, err := m.connection.ExecContext(ctxOrBackground(m.ctx), sqlStr, data...)
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
	val, err := flattenValues(len(columns), data)
	if err != nil {
		return 0, err
	}
	sqlStr := insertSQL(pTable, columns, len(data))
	rst, err := m.connection.ExecContext(ctxOrBackground(m.ctx), sqlStr, val...)
	if err != nil {
		return 0, err
	}
	return rst.LastInsertId()
}
