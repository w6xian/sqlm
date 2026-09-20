package sqlm

import (
	"context"
	"database/sql"

	"github.com/w6xian/sqlm/utils"
)

type Tx struct {
	db         *Db
	connection TxConn
	ctx        context.Context
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
	}
	return t.UseConn(tx.connection)
}

func (tx *Tx) Exec(query string, args ...any) (sql.Result, error) {
	c := tx.conn()
	if c == nil {
		return nil, ErrNoConnection
	}
	return c.Exec(query, args...)
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
	rows, err := c.Query(query, args...)
	if err != nil {
		return nil, err
	}
	return GetRow(rows)
}

func (tx *Tx) QueryMulti(query string, args ...any) (*Rows, error) {
	c := tx.conn()
	if c == nil {
		return nil, ErrNoConnection
	}
	rows, err := c.Query(query, args...)
	if err != nil {
		return nil, err
	}
	return GetRows(rows)
}
