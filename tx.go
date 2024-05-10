package sqlm

import (
	"context"
	"database/sql"
)

type Tx struct {
	db         *Db
	connection TxConn
	ctx        context.Context
}

func (tx *Tx) Use(dbc TxConn) {
	tx.connection = dbc
}

func (tx *Tx) Table(tbl string) *Table {
	svr := tx.db.server
	return Tbx(tx.ctx, tbl).UseLog(tx.db.log).Use(tx.db).UseConn(tx.connection).PreTable(svr.Pretable)
}

func (tx *Tx) Exec(query string, args ...interface{}) (sql.Result, error) {
	return tx.db.conn.Exec(query, args...)
}
