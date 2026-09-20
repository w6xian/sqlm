package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/pkg/errors"

	// Import the SQLite driver.
	"github.com/w6xian/sqlm"
	_ "modernc.org/sqlite"
)

type Sqlite struct {
	options     *sqlm.Options
	conf        *sqlm.Server
	connection  *sql.DB
	isConnected bool
	log         sqlm.StdLog
	ctx         context.Context
}

func NewSqlite(opt *sqlm.Options) (*Sqlite, error) {
	if opt == nil || opt.Server == nil {
		return nil, errors.New("sqlite: options is required")
	}
	return &Sqlite{options: opt, conf: opt.Server, log: opt.Logger(), isConnected: false}, nil
}

func (m *Sqlite) NewConn(conn *sql.DB, isConnected bool) (sqlm.DbConn, error) {
	return &Sqlite{options: m.options, conf: m.conf, log: m.log, connection: conn, isConnected: isConnected}, nil
}

// NewServerConn opens an independent connection to another server (replica)
// reusing the driver options. It implements sqlm.ServerSwitcher.
func (m *Sqlite) NewServerConn(ctx context.Context, svr *sqlm.Server) (sqlm.DbConn, error) {
	if svr == nil {
		return nil, errors.New("sqlite: server config is nil")
	}
	opts := cloneOptions(m.options, svr)
	n := &Sqlite{options: opts, conf: svr, log: opts.Logger(), isConnected: false}
	return n.Connect(ctx)
}

func cloneOptions(src *sqlm.Options, svr *sqlm.Server) *sqlm.Options {
	opts := &sqlm.Options{
		Name:    src.Name,
		Server:  svr,
		Slavers: src.Slavers,
		Mode:    src.Mode,
		Data:    src.Data,
	}
	opts.SetLogger(src.Logger())
	return opts
}

func (m *Sqlite) Conf() *sqlm.Server {
	return m.conf
}
func (m *Sqlite) Options() *sqlm.Options {
	return m.options
}

func (m *Sqlite) Ping() error {
	if err := checkConnection(m.connection); err != nil {
		return err
	}
	return m.connection.PingContext(ctxOrBackground(m.ctx))
}
func (m *Sqlite) Conn() (*sql.DB, error) {
	if err := checkConnection(m.connection); err != nil {
		return nil, err
	}
	return m.connection, nil
}
func (m *Sqlite) Close() error {
	if err := checkConnection(m.connection); err != nil {
		return err
	}
	return m.connection.Close()
}

func (m *Sqlite) check() error {
	return checkConnection(m.connection)
}

func (m *Sqlite) Connect(ctx context.Context) (sqlm.DbConn, error) {
	ctx = ctxOrBackground(ctx)
	if m.connection != nil {
		if m.isConnected {
			// 连接池已经就绪，直接复用即可：每次 Connect 都做一次 Ping
			// 会让 NewInstance 白白多一个往返。
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
	// Connect to the database with some sane settings:
	// - No shared-cache: it's obsolete; WAL journal mode is a better solution.
	// - No foreign key constraints: it's currently disabled by default, but it's
	// a good practice to be explicit and prevent future surprises on SQLite upgrades.
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

	source := sqliteSource(m.conf.DSN)

	conn, err := sql.Open(
		m.conf.Protocol,
		source,
	)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open db with dsn: %s", m.conf.DSN)
	}
	applyPool(conn, m.conf)
	if err = conn.PingContext(ctx); err != nil {
		conn.Close()
		return nil, err
	}
	m.isConnected = true
	m.connection = conn
	newconn, _ := m.NewConn(conn, true)
	newconn.WithContext(ctx)
	return newconn, nil
}

// sqliteSource appends the recommended pragmas, keeping any query already
// present in the DSN intact.
func sqliteSource(dsn string) string {
	pragma := "_pragma=foreign_keys(0)&_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)"
	if strings.Contains(dsn, "?") {
		return dsn + "&" + pragma
	}
	return fmt.Sprintf("%s?%s", dsn, pragma)
}

func (m *Sqlite) WithContext(ctx context.Context) {
	m.ctx = ctxOrBackground(ctx)
}

func (m *Sqlite) Delete(query string, args ...any) (*sql.Rows, error) {
	if err := m.check(); err != nil {
		return nil, err
	}
	return m.connection.QueryContext(ctxOrBackground(m.ctx), query, args...)
}

func (m *Sqlite) Prepare(query string) (*sql.Stmt, error) {
	if err := m.check(); err != nil {
		return nil, err
	}
	return m.connection.PrepareContext(ctxOrBackground(m.ctx), query)
}

func (m *Sqlite) Query(query string, args ...any) (*sql.Rows, error) {
	if err := m.check(); err != nil {
		return nil, err
	}
	return m.connection.QueryContext(ctxOrBackground(m.ctx), query, args...)
}

func (m *Sqlite) Exec(query string, args ...any) (sql.Result, error) {
	if err := m.check(); err != nil {
		return nil, err
	}
	// ExecContext prepares internally (and reuses the prepared statement when
	// available), which saves one round trip versus an explicit Prepare.
	return m.connection.ExecContext(ctxOrBackground(m.ctx), query, args...)
}

func (m *Sqlite) Insert(pTable string, columns []string, data []any) (int64, error) {
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
func (m *Sqlite) Inserts(pTable string, columns []string, data [][]any) (int64, error) {
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
