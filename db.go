package sqlm

import (
	"database/sql"
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/w6xian/sqlm/loog"
)

var sqlx atomic.Value

type ActionExec func(tx *Tx, args ...interface{}) (int64, error)

type TxConn interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	Prepare(query string) (*sql.Stmt, error)
	Query(query string, args ...interface{}) (*sql.Rows, error)
}

type DbConn interface {
	TxConn
	Ping() error
	Connect() error
	Conn() (*sql.DB, error)
	Close() error
	Conf() *Server
	NewConn(opt Server) (DbConn, error)
}

type Sqlm struct {
	opts      atomic.Value
	dbcon     DbConn
	Logger    loog.Logger
	LogPrefix string
}

func (d *Sqlm) Use(dbName ...string) (*Db, error) {
	cfg := d.getOpts()
	svr := cfg.Server
	if len(dbName) > 0 {
		name := dbName[0]
		svr.Database = name
	}
	con, err := d.dbcon.NewConn(svr)
	if err != nil {
		return nil, err
	}
	db := &Db{}
	db.conn = con
	err = con.Connect()
	if err != nil {
		return nil, err
	}
	return db, nil
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
	dbcon.Logger = cf.Logger
	dbcon.server = cf.getOpts().Slavers[pos]
	dbcon.conn.Connect()
	return dbcon
}

func Master() *Db {
	dbcon := &Db{}
	cf := getSqlx()
	dbcon.conn = cf.dbcon
	dbcon.Logger = cf.Logger
	dbcon.server = cf.getOpts().Server
	err := dbcon.conn.Connect()
	if err == nil {
		fmt.Println(err)
	}
	return dbcon
}

type Db struct {
	Logger   loog.Logger
	conn     DbConn
	server   Server
	LogLevel loog.LogLevel
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
	}
	sx.swapOpts(opt)
	sx.dbcon = db
	return swapSqlx(sx)
}

func (d *Db) Close() {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println(err)
		}
	}()
	if d.conn != nil {
		d.conn.Close()
	}
}

func (d *Db) Table(tbl string) *Table {
	svr := d.server
	return Tb(tbl).Use(d).PreTable(svr.Pretable)
}

func (d *Db) Action(exec ActionExec, args ...interface{}) (int64, error) {
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
				if ok, err := exec(tx, args...); err == nil {
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
