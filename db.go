package sqlm

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync/atomic"
)

var sqlx atomic.Value
var spanName = "sql"

type ActionExec func(tx *Tx, args ...interface{}) (int64, error)

type TxConn interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	Prepare(query string) (*sql.Stmt, error)
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
}

type DbConn interface {
	TxConn
	Connect() (DbConn, error)
	Ping() error
	Conn() (*sql.DB, error)
	Close() error
	Conf() *Server
	NewConn(conn *sql.DB, isconnected bool) (DbConn, error)
}

type Sqlm struct {
	opts      atomic.Value
	dbcon     DbConn
	LogPrefix string
	log       StdLog
}

func (d *Sqlm) swapOpts(opts *Options) {
	d.opts.Store(opts)
}

func (d *Sqlm) getOpts() *Options {
	return d.opts.Load().(*Options)
}

func Slaver(slaver ...int) *Db {
	pos := 0
	if len(slaver) > 0 {
		pos = slaver[0]
	}
	dbcon := &Db{}
	cf := getSqlx()
	dbcon.conn = cf.dbcon
	dbcon.server = cf.getOpts().Slavers[pos]
	dbcon.log = cf.getOpts().log
	dbcon.ctx = context.Background()
	dbcon.conn.Connect()
	return dbcon
}

/*
 * [deprecated]请用Major()替代
 */
func Master() *Db {
	return MasterWithContext(context.Background())
}

/*
 * [deprecated]请用Major()替代
 */
func MasterWithContext(ctx context.Context) *Db {
	dbcon := &Db{}
	cf := getSqlx()
	dbcon.server = cf.getOpts().Server
	dbcon.log = cf.getOpts().log
	dbcon.ctx = ctx
	conn, err := dbcon.conn.Connect()
	if err != nil {
		cf.getOpts().log.Error(err.Error())
	}
	dbcon.conn = conn
	return dbcon
}

func Major(ctx context.Context) *Db {
	dbcon := &Db{}
	sm := getSqlx()
	dbcon.server = sm.getOpts().Server
	dbcon.log = sm.getOpts().log
	dbcon.ctx = ctx
	conn, err := sm.dbcon.Connect()
	if err != nil {
		sm.getOpts().log.Error(err.Error())
	}
	dbcon.conn = conn
	return dbcon
}

type Db struct {
	conn   DbConn
	server Server
	log    StdLog
	ctx    context.Context
}

func getSqlx() *Sqlm {
	return sqlx.Load().(*Sqlm)
}

func swapSqlx(sx *Sqlm) atomic.Value {
	sqlx.Store(sx)
	return sqlx
}

func New(opt *Options, db DbConn) atomic.Value {
	sx := &Sqlm{
		LogPrefix: "[sqlm] ",
		log:       opt.log,
	}
	sx.swapOpts(opt)
	sx.dbcon = db
	return swapSqlx(sx)
}

func (d *Db) Close() {
	defer func() {
		if err := recover(); err != nil {
			d.log.Error(fmt.Sprintf("%v", err))
		}
	}()
	if d.conn != nil {
		d.conn.Close()
	}
}

func (d *Db) Table(tbl string) *Table {
	svr := d.server
	return Tb(tbl).UseLog(d.log).Use(d).PreTable(svr.Pretable)
}

func (d *Db) Query(query string, args ...interface{}) (*Row, error) {
	return d.QueryContext(context.Background(), query, args...)
}

func (d *Db) QueryContext(ctx context.Context, query string, args ...interface{}) (*Row, error) {
	rows, err := d.conn.QueryContext(ctx, query, args...)
	if err == nil {
		defer rows.Close()
		return GetRow(rows)
	}
	return nil, err
}

func (d *Db) QueryMulti(query string, args ...interface{}) (*Rows, error) {
	return d.QueryMultiContext(context.Background(), query, args...)
}

func (d *Db) QueryMultiContext(ctx context.Context, query string, args ...interface{}) (*Rows, error) {
	rows, err := d.conn.Query(query, args...)
	if err == nil {
		defer rows.Close()
		return GetRows(rows)
	}
	return nil, err
}

func (d *Db) Exec(query string, args ...interface{}) (sql.Result, error) {
	return d.conn.ExecContext(context.Background(), query, args...)
}

func (d *Db) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return d.conn.Exec(query, args...)
}

func (d *Db) Conn() (*sql.DB, error) {
	return d.conn.Conn()
}

func (d *Db) Action(exec ActionExec) (int64, error) {
	db, err := d.conn.Conn()
	if err == nil {
		if err := db.Ping(); err == nil {
			if _tx, err := db.Begin(); err == nil {
				defer func() {
					if err := recover(); err != nil {
						_tx.Rollback()
					}
				}()
				tx := &Tx{db: d}
				tx.Use(_tx)
				if ok, err := exec(tx); err == nil {
					_tx.Commit()
					return ok, err
				} else {
					_tx.Rollback()
					return ok, err
				}

			}
		}
	}
	return 0, errors.New("链接已中断")
}
