package store

import (
	"context"
	"database/sql"

	"github.com/pkg/errors"
	"github.com/w6xian/sqlm"
)

// Driver is an interface for store driver.
// It contains all methods that store database driver should implement.
type Driver interface {
	NewConn(conn *sql.DB, isConnected bool) (sqlm.DbConn, error)
	Conf() *sqlm.Server
	Ping() error
	Conn() (*sql.DB, error)
	Close() error
	check() error
	Connect() (sqlm.DbConn, error)
	Delete(query string, args ...interface{}) (*sql.Rows, error)
	Prepare(query string) (*sql.Stmt, error)
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	Exec(query string, args ...interface{}) (sql.Result, error)
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	Insert(pTable string, columns []string, data []interface{}) (int64, error)
	/**
	 * 为了执行效率，请自行保证query中需要的参数个数与后面的参数中数组长度相对应
	 */
	Inserts(pTable string, columns []string, data [][]interface{}) (int64, error)
}

// NewDBDriver creates new db driver based on profile.
func NewDriver(opt *sqlm.Options) (Driver, error) {
	var driver Driver
	var err error

	opt, err = sqlm.CheckOption(opt)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create check options")
	}
	switch opt.Server.Protocol {
	case "sqlite":
		driver, err = NewSqlite(opt)
	case "mysql":
		driver, err = NewMysql(opt)
	default:
		return nil, errors.New("unknown db driver")
	}
	if err != nil {
		return nil, errors.Wrap(err, "failed to create db driver")
	}
	return driver, nil
}
