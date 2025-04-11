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

func (tx *Tx) Query(query string, args ...interface{}) (*Row, error) {
	rows, err := tx.db.conn.Query(query, args...)
	if err == nil {
		defer rows.Close()
		return GetRow(rows)
	}
	return nil, err
}

func (tx *Tx) QueryMulti(query string, args ...interface{}) (*Rows, error) {
	rows, err := tx.db.conn.Query(query, args...)
	if err == nil {
		defer rows.Close()
		return GetRows(rows)
	}
	return nil, err
}
