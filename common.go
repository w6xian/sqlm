package sqlm

import (
	"fmt"
	"time"
)

// Rows2MapRow indexes every row by the value of `col`. The cursor is left at
// the beginning so the Rows stays reusable afterwards.
func Rows2MapRow(rows *Rows, col string) map[string]*Row {
	if rows == nil {
		return map[string]*Row{}
	}
	rst := rows.ToKeyMap(col)
	_ = rows.ResetIndex()
	return rst
}

func UnixTime() int64 {
	t := time.Now()
	return t.Unix()
}

func Alias(table, alias string) string {
	return fmt.Sprintf("%s %s", table, alias)
}
