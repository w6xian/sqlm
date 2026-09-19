package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	// 引入驱动即完成注册，驱动名就是 "postgres"
	_ "github.com/lib/pq"

	"github.com/pkg/errors"

	"github.com/w6xian/sqlm"
)

// Postgres drives PostgreSQL servers. The actual driver (pgx, lib/pq, ...)
// must be registered by the caller under the configured Protocol name.
type Postgres struct {
	options     *sqlm.Options
	conf        *sqlm.Server
	connection  *sql.DB
	isConnected bool
	log         sqlm.StdLog
	ctx         context.Context
}

func NewPostgres(opt *sqlm.Options) (Driver, error) {
	if opt == nil || opt.Server == nil {
		return nil, errors.New("postgres: options is required")
	}
	return &Postgres{options: opt, conf: opt.Server, log: opt.Logger(), isConnected: false}, nil
}

func (m *Postgres) NewConn(conn *sql.DB, isConnected bool) (sqlm.DbConn, error) {
	return &Postgres{options: m.options, conf: m.conf, log: m.log, connection: conn, isConnected: isConnected}, nil
}

// NewServerConn opens an independent connection to another server (replica)
// reusing the driver options. It implements sqlm.ServerSwitcher.
func (m *Postgres) NewServerConn(ctx context.Context, svr *sqlm.Server) (sqlm.DbConn, error) {
	if svr == nil {
		return nil, errors.New("postgres: server config is nil")
	}
	opts := cloneOptions(m.options, svr)
	n := &Postgres{options: opts, conf: svr, log: opts.Logger(), isConnected: false}
	return n.Connect(ctx)
}

func (m *Postgres) Conf() *sqlm.Server {
	return m.conf
}

func (m *Postgres) Options() *sqlm.Options {
	return m.options
}

func (m *Postgres) Ping() error {
	if err := checkConnection(m.connection); err != nil {
		return err
	}
	return m.connection.PingContext(ctxOrBackground(m.ctx))
}

func (m *Postgres) Conn() (*sql.DB, error) {
	if err := checkConnection(m.connection); err != nil {
		return nil, err
	}
	return m.connection, nil
}

func (m *Postgres) Close() error {
	if err := checkConnection(m.connection); err != nil {
		return err
	}
	return m.connection.Close()
}

func (m *Postgres) check() error {
	return checkConnection(m.connection)
}

func (m *Postgres) Connect(ctx context.Context) (sqlm.DbConn, error) {
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

	source, err := postgresSource(m.conf)
	if err != nil {
		return nil, err
	}

	conn, err := sql.Open(m.conf.Protocol, source)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open db with protocol: %s", m.conf.Protocol)
	}
	applyPool(conn, m.conf)
	if err = conn.PingContext(ctx); err != nil {
		_ = conn.Close()
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

// postgresSource builds a PostgreSQL keyword/value connection string. An
// explicit DSN always wins, otherwise unix sockets and tcp hosts are derived
// from the server fields.
func postgresSource(conf *sqlm.Server) (string, error) {
	if conf == nil {
		return "", errors.New("postgres: server config is nil")
	}
	if conf.DSN != "" {
		return conf.DSN, nil
	}

	parts := []string{}
	if strings.HasPrefix(conf.Host, "unix:") || strings.HasPrefix(conf.Host, "/") {
		// libpq expects the socket directory in host, e.g. /var/run/postgresql
		parts = append(parts, "host="+strings.TrimPrefix(conf.Host, "unix:"))
	} else {
		host := conf.Host
		if host == "" {
			host = "127.0.0.1"
		}
		parts = append(parts, "host="+host)
		port := conf.Port
		if port <= 0 {
			port = 5432
		}
		parts = append(parts, fmt.Sprintf("port=%d", port))
	}
	parts = append(parts,
		"user="+conf.Username,
		"password="+conf.Password,
		"dbname="+conf.Database,
		"sslmode=disable",
	)
	if conf.Charset != "" {
		parts = append(parts, "client_encoding="+conf.Charset)
	}
	return strings.Join(parts, " "), nil
}

func (m *Postgres) WithContext(ctx context.Context) {
	m.ctx = ctxOrBackground(ctx)
}

func (m *Postgres) Delete(query string, args ...any) (*sql.Rows, error) {
	if err := m.check(); err != nil {
		return nil, err
	}
	return m.connection.QueryContext(ctxOrBackground(m.ctx), query, args...)
}

func (m *Postgres) Prepare(query string) (*sql.Stmt, error) {
	if err := m.check(); err != nil {
		return nil, err
	}
	return m.connection.PrepareContext(ctxOrBackground(m.ctx), query)
}

func (m *Postgres) Query(query string, args ...any) (*sql.Rows, error) {
	if err := m.check(); err != nil {
		return nil, err
	}
	return m.connection.QueryContext(ctxOrBackground(m.ctx), query, args...)
}

func (m *Postgres) Exec(query string, args ...any) (sql.Result, error) {
	if err := m.check(); err != nil {
		return nil, err
	}
	// ExecContext prepares internally (and reuses the prepared statement when
	// available), which saves one round trip versus an explicit Prepare.
	return m.connection.ExecContext(ctxOrBackground(m.ctx), query, args...)
}

func (m *Postgres) Insert(pTable string, columns []string, data []any) (int64, error) {
	if len(columns) != len(data) {
		return 0, errors.New("请确保column长度统一")
	}
	if err := m.check(); err != nil {
		return 0, err
	}
	if len(columns) == 0 {
		return 0, errors.New("请提供字段")
	}
	rst, err := m.connection.ExecContext(ctxOrBackground(m.ctx), postgresInsertSQL(pTable, columns, 1), data...)
	if err != nil {
		return 0, err
	}
	return rst.LastInsertId()
}

/**
 * 为了执行效率，请自行保证query中需要的参数个数与后面的参数中数组长度相对应
 */
func (m *Postgres) Inserts(pTable string, columns []string, data [][]any) (int64, error) {
	if err := m.check(); err != nil {
		return 0, err
	}
	val, err := flattenValues(len(columns), data)
	if err != nil {
		return 0, err
	}
	rst, err := m.connection.ExecContext(ctxOrBackground(m.ctx), postgresInsertSQL(pTable, columns, len(data)), val...)
	if err != nil {
		return 0, err
	}
	return rst.LastInsertId()
}

// postgresInsertSQL builds a multi-values INSERT using double quoted
// identifiers and the numbered $1..$n placeholders PostgreSQL expects.
func postgresInsertSQL(pTable string, columns []string, rows int) string {
	quoted := make([]string, len(columns))
	for k, v := range columns {
		quoted[k] = `"` + strings.Trim(v, `"`) + `"`
	}
	values := make([]string, rows)
	for i := range values {
		holders := make([]string, len(columns))
		for j := range holders {
			holders[j] = fmt.Sprintf("$%d", i*len(columns)+j+1)
		}
		values[i] = "(" + strings.Join(holders, ",") + ")"
	}
	return fmt.Sprintf(`INSERT INTO "%s" (%s) VALUES %s`, strings.Trim(pTable, `"`),
		strings.Join(quoted, ","), strings.Join(values, ","))
}
