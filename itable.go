package sqlm

type T interface {
	Table(tbl string) *Table
}
