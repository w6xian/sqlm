package sqlm

import "database/sql"

type T interface {
	Table(tbl string) *Table
}

type ITable interface {
	Table(tbl string) *Table
	Exec(query string, args ...interface{}) (sql.Result, error)
}
