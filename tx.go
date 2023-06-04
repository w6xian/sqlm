package sqlm

type Tx struct {
	db         *Db
	connection TxConn
}

func (tx *Tx) Use(dbc TxConn) {
	tx.connection = dbc
}

func (tx *Tx) Table(tbl string) *Table {
	svr := tx.db.server
	return Tb(tbl).Use(tx.db).UseConn(tx.connection).PreTable(svr.Pretable)
}
