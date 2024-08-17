package sqlm

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
)

var sqlx atomic.Value

const DEFAULT_KEY = "def"

type ActionExec func(tx *Tx, args ...interface{}) (int64, error)

type TxConn interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	Prepare(query string) (*sql.Stmt, error)
	Query(query string, args ...interface{}) (*sql.Rows, error)
}

type DbConn interface {
	TxConn
	Connect(ctx context.Context) (DbConn, error)
	WithContext(ctx context.Context)
	Options() *Options
	Ping() error
	Conn() (*sql.DB, error)
	Close() error
	Conf() *Server
	NewConn(conn *sql.DB, isconnected bool) (DbConn, error)
}

type Sqlm struct {
	dbcon     DbConn
	LogPrefix string
	log       StdLog
}

func (d *Sqlm) getOpts() *Options {
	return d.dbcon.Options()
}

func Slaver(slaver ...int) *Db {
	return SlaverContext(context.Background(), slaver...)
}

func SlaverContext(ctx context.Context, slaver ...int) *Db {
	pos := 0
	if len(slaver) > 0 {
		pos = slaver[0]
	}
	dbcon := &Db{}
	sl := getSqlx(DEFAULT_KEY)
	dbcon.server = sl.getOpts().Slavers[pos]
	dbcon.log = sl.getOpts().log
	dbcon.ctx = ctx
	conn, err := sl.dbcon.Connect(ctx)
	if err != nil {
		sl.getOpts().log.Error(err.Error())
	}
	dbcon.conn = conn
	return dbcon
}

/*
 * [deprecated]请用Major()替代
 */
func Master() *Db {
	return MasterContext(context.Background())
}

/*
 * [deprecated]请用Major()替代
 */
func MasterContext(ctx context.Context) *Db {
	return Major(ctx)
}

func Major(ctx context.Context) *Db {
	dbcon := &Db{}
	sm := getSqlx(DEFAULT_KEY)
	dbcon.server = sm.getOpts().Server
	dbcon.log = sm.getOpts().log
	dbcon.ctx = ctx
	conn, err := sm.dbcon.Connect(ctx)
	if err != nil {
		sm.getOpts().log.Error(err.Error())
	}
	dbcon.conn = conn
	return dbcon
}

func MewInstance(ctx context.Context, name string) *Db {
	dbcon := &Db{}
	sm := getSqlx(name)
	dbcon.server = sm.getOpts().Server
	dbcon.log = sm.getOpts().log
	dbcon.ctx = ctx
	conn, err := sm.dbcon.Connect(ctx)
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

func getSqlx(name string) *Sqlm {
	m := sqlx.Load().(map[string]*Sqlm)
	return m[name]
}

func swapSqlx(sx *Sqlm, k string) bool {
	m := sqlx.Load()
	if m == nil {
		t := make(map[string]*Sqlm)
		t[k] = sx
		sqlx.Store(t)
		return true
	}
	r := m.(map[string]*Sqlm)
	if _, ok := r[k]; ok {
		return false
	}
	r[k] = sx
	sqlx.Store(r)
	return true
}

func New(opt *Options, db DbConn) bool {
	sx := &Sqlm{
		LogPrefix: "[sqlm] ",
		log:       opt.log,
	}
	sx.dbcon = db
	n := opt.Name
	if n == "" {
		n = DEFAULT_KEY
	}
	return swapSqlx(sx, n)
}

func Use(dbs ...DbConn) bool {
	for _, db := range dbs {
		opt := db.Options()
		sx := &Sqlm{
			LogPrefix: "[sqlm] ",
			log:       opt.log,
		}
		sx.dbcon = db
		n := opt.Name
		if n == "" {
			n = DEFAULT_KEY
		}
		swapSqlx(sx, n)
	}
	return true
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
	return Tbx(d.ctx, tbl).UseLog(d.log).Use(d).PreTable(svr.Pretable)
}

func (d *Db) TableName(tbl string) string {
	return d.server.Pretable + tbl
}
func (d *Db) TrimPrefix(tbl string) string {
	return strings.TrimPrefix(tbl, d.server.Pretable)
}

func (d *Db) WithPrefix(tbl string) string {
	if strings.HasPrefix(tbl, d.server.Pretable) {
		return tbl
	}
	return d.TableName(tbl)
}

func (d *Db) Query(query string, args ...interface{}) (*Row, error) {
	rows, err := d.conn.Query(query, args...)
	if err == nil {
		defer rows.Close()
		return GetRow(rows)
	}
	return nil, err
}

func (d *Db) QueryMulti(query string, args ...interface{}) (*Rows, error) {
	rows, err := d.conn.Query(query, args...)
	if err == nil {
		defer rows.Close()
		return GetRows(rows)
	}
	return nil, err
}

func (d *Db) Rows(query string, args ...interface{}) (*sql.Rows, error) {
	return d.conn.Query(query, args...)
}

func (m *Db) MaxId(tbl string, args ...string) sql.NullInt64 {
	if len(args) == 0 {
		args = append(args, "id")
	}
	query := fmt.Sprintf("SELECT max(%s) as id FROM %s", args[0], tbl)
	row, err := m.Query(query)
	if err == nil {
		return row.Get("id").NullInt64()
	}
	return sql.NullInt64{Int64: 0, Valid: false}
}

func (d *Db) Exec(query string, args ...interface{}) (sql.Result, error) {
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
				tx := &Tx{db: d, ctx: d.ctx}
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
