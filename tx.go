package sqlm

import (
	"context"
	"database/sql"
	"time"

	"github.com/w6xian/sqlm/utils"
)

type Tx struct {
	db         *Db
	connection TxConn
	ctx        context.Context
	// hooks 事务级钩子，由 Db.Action 从实例带过来。
	hooks []Hook
}

func (tx *Tx) Use(dbc TxConn) {
	tx.connection = dbc
}

// conn returns the transactional connection, falling back to the plain
// connection when Tx has not been bound yet.
func (tx *Tx) conn() TxConn {
	if tx.connection != nil {
		return tx.connection
	}
	if tx.db != nil {
		return tx.db.conn
	}
	return nil
}

func (tx *Tx) ctxOrDefault() context.Context {
	if tx.ctx != nil {
		return tx.ctx
	}
	if tx.db != nil {
		return tx.db.Ctx()
	}
	return context.Background()
}

func (tx *Tx) Table(tbl string) *Table {
	var svr *Server
	if tx.db != nil {
		svr = tx.db.server
	}
	if svr == nil {
		svr = &Server{}
	}
	protocol := utils.GetOrDefault(svr.Protocol, MYSQL)
	t := Tbx(tx.ctxOrDefault(), tbl).UseLog(tx.db.Log()).PreTable(svr.Pretable).SetProtocol(protocol)
	if tx.db != nil {
		t.Use(tx.db)
		return t.UseConn(tx.connection)
	}
	// 没有 Db 时钩子只能靠自己带下来
	t.hooks = tx.hooks
	return t.UseConn(tx.connection)
}

func (tx *Tx) Exec(query string, args ...any) (sql.Result, error) {
	c := tx.conn()
	if c == nil {
		return nil, ErrNoConnection
	}
	start := tx.stmtStart()
	rst, err := c.Exec(query, args...)
	rows, known := int64(0), false
	if rst != nil {
		if n, e := rst.RowsAffected(); e == nil {
			rows, known = n, true
		}
	}
	tx.stmtDone(start, OpExec, query, rows, known, err)
	return rst, err
}

// stmtStart / stmtDone 事务里直接执行的原始 SQL：与 Table 的打点不重叠。
func (tx *Tx) stmtStart() time.Time {
	if len(tx.hooks) == 0 {
		return time.Time{}
	}
	return time.Now()
}

func (tx *Tx) stmtDone(start time.Time, op, query string, rows int64, known bool, err error) {
	if start.IsZero() {
		return
	}
	emitStmt(tx.ctxOrDefault(), tx.hooks, &StmtInfo{
		Op:        op,
		Digest:    DigestSQL(query),
		Rows:      rows,
		RowsKnown: known,
		Duration:  time.Since(start),
		StartAt:   start,
		Err:       err,
	}, query)
}

func (tx *Tx) Prepare(query string) (*sql.Stmt, error) {
	c := tx.conn()
	if c == nil {
		return nil, ErrNoConnection
	}
	return c.Prepare(query)
}

func (tx *Tx) Query(query string, args ...any) (*Row, error) {
	c := tx.conn()
	if c == nil {
		return nil, ErrNoConnection
	}
	start := tx.stmtStart()
	rows, err := c.Query(query, args...)
	if err != nil {
		tx.stmtDone(start, OpSelect, query, 0, true, err)
		return nil, err
	}
	row, err := GetRow(rows)
	n := int64(0)
	if row != nil {
		n = 1
	}
	tx.stmtDone(start, OpSelect, query, n, true, err)
	return row, err
}

func (tx *Tx) QueryMulti(query string, args ...any) (*Rows, error) {
	c := tx.conn()
	if c == nil {
		return nil, ErrNoConnection
	}
	start := tx.stmtStart()
	rows, err := c.Query(query, args...)
	if err != nil {
		tx.stmtDone(start, OpSelect, query, 0, true, err)
		return nil, err
	}
	res, err := GetRows(rows)
	n := int64(0)
	if res != nil {
		n = int64(res.Length())
	}
	tx.stmtDone(start, OpSelect, query, n, true, err)
	return res, err
}
