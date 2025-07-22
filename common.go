package sqlm

import "time"

func Rows2MapRow(rows *Rows, col string) map[string]*Row {
	rst := make(map[string]*Row)
	for rows.Next() != nil {
		key := rows.Get(col).String()
		rst[key] = rows.Row()
	}
	rows.ResetIndex()
	return rst
}

func UnixTime() int64 {
	t := time.Now()
	return t.Unix()
}
