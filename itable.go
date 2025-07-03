package sqlm

import "database/sql"

type T interface {
	Table(tbl string) *Table
}

type ITable interface {
	Table(tbl string) *Table
	Exec(query string, args ...any) (sql.Result, error)
	Query(query string, args ...any) (*Row, error)
	QueryMulti(query string, args ...any) (*Rows, error)
}
