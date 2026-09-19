package sqlm

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/w6xian/sqlm/utils"
)

// instanceRegistry holds every Sqlm instance created by Use()/New().
//
// The stored map is immutable once published: every write builds a new map and
// swaps it atomically, so readers never need a lock and can never observe a
// partially updated map.
var (
	instanceLock  sync.Mutex
	instanceValue atomic.Value // map[string]*Sqlm
)

func init() {
	instanceValue.Store(map[string]*Sqlm{})
}

const DEFAULT_KEY = "def"

type ActionExec func(tx ITable, args ...any) (int64, error)

type TxConn interface {
	Exec(query string, args ...any) (sql.Result, error)
	Prepare(query string) (*sql.Stmt, error)
	Query(query string, args ...any) (*sql.Rows, error)
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

// ServerSwitcher is optionally implemented by drivers able to open a brand new
// connection to another server (read replica) sharing the driver settings.
//
// Drivers that do not implement it keep using the default (master) connection
// when Slaver() is called.
type ServerSwitcher interface {
	NewServerConn(ctx context.Context, svr *Server) (DbConn, error)
}

type Sqlm struct {
	dbcon     DbConn
	LogPrefix string
	log       StdLog
}

func (d *Sqlm) getOpts() *Options {
	return d.dbcon.Options()
}

// instances returns a snapshot of the registered instances. Never nil.
func instances() map[string]*Sqlm {
	if v := instanceValue.Load(); v != nil {
		if m, ok := v.(map[string]*Sqlm); ok {
			return m
		}
	}
	return map[string]*Sqlm{}
}

// Instances returns the names of all registered instances.
func Instances() []string {
	m := instances()
	names := make([]string, 0, len(m))
	for k := range m {
		names = append(names, k)
	}
	return names
}

// Has reports whether an instance is registered under the given name.
func Has(name string) bool {
	_, ok := instances()[name]
	return ok
}

// Reset drops every registered instance. Mainly useful in tests to get a clean
// registry without restarting the process.
func Reset() {
	instanceLock.Lock()
	defer instanceLock.Unlock()
	instanceValue.Store(map[string]*Sqlm{})
}

func getSqlx(name string) *Sqlm {
	sx, ok := instances()[name]
	if !ok {
		panic(fmt.Sprintf("sqlm instance %s not found,pls Use sqlm.Use() before", name))
	}
	return sx
}

// getSqlxSafe returns nil instead of panicking when the instance is missing.
func getSqlxSafe(name string) *Sqlm {
	sx, ok := instances()[name]
	if !ok {
		return nil
	}
	return sx
}

func swapSqlx(sx *Sqlm, k string) bool {
	instanceLock.Lock()
	defer instanceLock.Unlock()
	curr := map[string]*Sqlm{}
	if v := instanceValue.Load(); v != nil {
		if m, ok := v.(map[string]*Sqlm); ok {
			for name, item := range m {
				curr[name] = item
			}
		}
	}
	if _, ok := curr[k]; ok {
		return false
	}
	curr[k] = sx
	instanceValue.Store(curr)
	return true
}

func New(opt *Options, db DbConn) bool {
	sx := &Sqlm{
		LogPrefix: "[sqlm] ",
		log:       loggerOrDefault(opt),
	}
	sx.dbcon = db
	n := opt.Name
	if n == "" {
		n = DEFAULT_KEY
	}
	return swapSqlx(sx, n)
}

func Use(dbs ...DbConn) bool {
	ok := true
	for _, db := range dbs {
		opt := db.Options()
		sx := &Sqlm{
			LogPrefix: "[sqlm] ",
			log:       loggerOrDefault(opt),
		}
		sx.dbcon = db
		n := opt.Name
		if n == "" {
			n = DEFAULT_KEY
		}
		if !swapSqlx(sx, n) {
			ok = false
		}
	}
	return ok
}

func Slaver(slaver ...int) *Db {
	return SlaverContext(context.Background(), slaver...)
}

// SlaverContext returns a Db bound to one of the configured read replicas.
//
// The instance is always returned; use Db.Err() to check whether the replica
// could actually be reached.
func SlaverContext(ctx context.Context, slaver ...int) *Db {
	pos := 0
	if len(slaver) > 0 {
		pos = slaver[0]
	}
	sm := getSqlx(DEFAULT_KEY)
	return slaverDb(contextOrBackground(ctx), sm, sm.getOpts(), pos)
}

// slaverDb connects to the configured replica, reporting a clear error instead
// of panicking when the replica is missing.
func slaverDb(ctx context.Context, sm *Sqlm, opt *Options, pos int) *Db {
	svs := opt.Slavers
	if len(svs) == 0 {
		return newDb(ctx, opt.Server, sm, errors.New("sqlm: no slaver configured"))
	}
	if pos < 0 {
		pos = 0
	}
	if pos >= len(svs) {
		return newDb(ctx, opt.Server, sm, fmt.Errorf("sqlm: slaver %d out of range (%d slavers)", pos, len(svs)))
	}
	return newDb(ctx, svs[pos], sm, nil)
}

func contextOrBackground(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

// SlaverOf returns a Db bound to a read replica of the named instance.
func SlaverOf(ctx context.Context, name string, slaver ...int) *Db {
	pos := 0
	if len(slaver) > 0 {
		pos = slaver[0]
	}
	if ctx == nil {
		ctx = context.Background()
	}
	sm := getSqlx(name)
	opt := sm.getOpts()
	return slaverDb(ctx, sm, opt, pos)
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

// Major returns a Db bound to the master server of the default instance.
func Major(ctx context.Context) *Db {
	if ctx == nil {
		ctx = context.Background()
	}
	sm := getSqlx(DEFAULT_KEY)
	return newDb(ctx, sm.getOpts().Server, sm, nil)
}

func NewInstance(ctx context.Context, name string) *Db {
	if ctx == nil {
		ctx = context.Background()
	}
	sm := getSqlx(name)
	return newDb(ctx, sm.getOpts().Server, sm, nil)
}

func NewDefaultInstance(ctx context.Context) *Db {
	if ctx == nil {
		ctx = context.Background()
	}
	sm := getSqlx(DEFAULT_KEY)
	return newDb(ctx, sm.getOpts().Server, sm, nil)
}

// TryInstance returns a Db and an error instead of panicking when the instance
// has not been registered with Use()/New().
func TryInstance(ctx context.Context, name string) (*Db, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	sm := getSqlxSafe(name)
	if sm == nil {
		return nil, fmt.Errorf("sqlm instance %s not found,pls Use sqlm.Use() before", name)
	}
	return newDb(ctx, sm.getOpts().Server, sm, nil), nil
}

// newDb connects to `svr` and returns a ready to use Db. `prevErr` short
// circuits the connection phase, keeping the returned object usable (non nil)
// so callers never dereference a nil pointer.
func newDb(ctx context.Context, svr *Server, sm *Sqlm, prevErr error) *Db {
	dbcon := &Db{server: svr, log: sm.log, ctx: ctx}
	if dbcon.log == nil {
		dbcon.log = sm.getOpts().log
	}
	if dbcon.log == nil {
		dbcon.log = NewNoopLogger()
	}
	if prevErr != nil {
		dbcon.err = prevErr
		dbcon.log.Error(prevErr.Error())
		return dbcon
	}
	conn, err := connect(ctx, sm, svr)
	if err != nil {
		dbcon.err = err
		dbcon.log.Error(err.Error())
		return dbcon
	}
	dbcon.conn = conn
	return dbcon
}

// connect opens a connection to svr. When svr is nil, or equal to the server
// already configured on the driver, the driver default connection is reused.
func connect(ctx context.Context, sm *Sqlm, svr *Server) (DbConn, error) {
	drv := sm.dbcon
	if svr == nil {
		return drv.Connect(ctx)
	}
	// Same physical server -> reuse (this keeps the connection pool shared).
	if cur := drv.Conf(); cur != nil && sameServer(cur, svr) {
		return drv.Connect(ctx)
	}
	switcher, ok := drv.(ServerSwitcher)
	if !ok {
		return drv.Connect(ctx)
	}
	return switcher.NewServerConn(ctx, svr)
}

func sameServer(a, b *Server) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Protocol == b.Protocol &&
		a.Database == b.Database &&
		a.Host == b.Host &&
		a.Port == b.Port &&
		a.Username == b.Username &&
		a.Password == b.Password &&
		a.Charset == b.Charset &&
		a.DSN == b.DSN
}

type Db struct {
	conn   DbConn
	server *Server
	log    StdLog
	ctx    context.Context
	err    error
}

// Err returns the error met while creating the instance (if any).
func (d *Db) Err() error {
	return d.err
}

// Ctx returns the context bound to the instance.
func (d *Db) Ctx() context.Context {
	if d.ctx == nil {
		return context.Background()
	}
	return d.ctx
}

func (d *Db) Close() {
	defer func() {
		if err := recover(); err != nil {
			d.Log().Error(fmt.Sprintf("%v", err))
		}
	}()
	if d.conn != nil {
		_ = d.conn.Close()
	}
}

// Log returns the logger bound to the instance, never nil.
func (d *Db) Log() StdLog {
	if d.log != nil {
		return d.log
	}
	return NewNoopLogger()
}

// Table returns a new query builder bound to the instance connection.
func (d *Db) Table(tbl string) *Table {
	svr := d.server
	protocol := utils.GetOrDefault(svr.Protocol, MYSQL)
	return Tbx(d.Ctx(), tbl).UseLog(d.Log()).Use(d).UseConn(d.conn).PreTable(svr.Pretable).SetProtocol(protocol)
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

func (d *Db) checkConn() error {
	if d.conn == nil {
		if d.err != nil {
			return d.err
		}
		return ErrNoConnection
	}
	return nil
}

func (d *Db) Query(query string, args ...any) (*Row, error) {
	if err := d.checkConn(); err != nil {
		return nil, err
	}
	rows, err := d.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	// GetRow takes the ownership of rows and always closes it.
	return GetRow(rows)
}

func (d *Db) QueryMulti(query string, args ...any) (*Rows, error) {
	if err := d.checkConn(); err != nil {
		return nil, err
	}
	rows, err := d.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	// GetRows takes the ownership of rows and always closes it.
	return GetRows(rows)
}

// Rows exposes the raw *sql.Rows. The caller owns and must close them.
func (d *Db) Rows(query string, args ...any) (*sql.Rows, error) {
	if err := d.checkConn(); err != nil {
		return nil, err
	}
	return d.conn.Query(query, args...)
}

func (m *Db) MaxId(tbl string, args ...string) sql.NullInt64 {
	if len(args) == 0 {
		args = append(args, "id")
	}
	query := fmt.Sprintf("SELECT max(%s) as id FROM %s", args[0], tbl)
	row, err := m.Query(query)
	if err == nil && row != nil {
		return row.Get("id").NullInt64()
	}
	return sql.NullInt64{Int64: 0, Valid: false}
}

func (d *Db) Exec(query string, args ...any) (sql.Result, error) {
	if err := d.checkConn(); err != nil {
		return nil, err
	}
	return d.conn.Exec(query, args...)
}

func (d *Db) Conn() (*sql.DB, error) {
	if err := d.checkConn(); err != nil {
		return nil, err
	}
	return d.conn.Conn()
}

// Ping checks that the underlying database is still reachable.
func (d *Db) Ping() error {
	if err := d.checkConn(); err != nil {
		return err
	}
	return d.conn.Ping()
}

// Action runs exec inside a transaction, committing on success and rolling
// back on any error or panic. The panic is propagated to the caller.
func (d *Db) Action(exec ActionExec) (int64, error) {
	if err := d.checkConn(); err != nil {
		return 0, err
	}
	db, err := d.conn.Conn()
	if err != nil {
		return 0, errors.Join(ErrConnectionLost, err)
	}
	if err = db.PingContext(d.Ctx()); err != nil {
		return 0, errors.Join(ErrConnectionLost, err)
	}
	_tx, err := db.BeginTx(d.Ctx(), nil)
	if err != nil {
		return 0, errors.Join(ErrConnectionLost, err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = _tx.Rollback()
			panic(p)
		}
	}()
	tx := &Tx{db: d, ctx: d.Ctx()}
	tx.Use(_tx)
	ok, err := exec(tx)
	if err != nil {
		_ = _tx.Rollback()
		return ok, err
	}
	if err = _tx.Commit(); err != nil {
		_ = _tx.Rollback()
		return ok, err
	}
	return ok, nil
}
