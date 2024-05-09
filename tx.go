package sqlm

import "database/sql"

type ITable interface {
	Table(tbl string) *Table
	Exec(query string, args ...interface{}) (sql.Result, error)
}

type Tx struct {
	db         *Db
	connection TxConn
}

func (tx *Tx) Use(dbc TxConn) {
	tx.connection = dbc
}

func (tx *Tx) Table(tbl string) *Table {
	svr := tx.db.server
	return Tb(tbl).UseLog(tx.db.log).Use(tx.db).UseConn(tx.connection).PreTable(svr.Pretable)
}

func (tx *Tx) Exec(query string, args ...interface{}) (sql.Result, error) {
	return tx.db.conn.Exec(query, args...)
}
