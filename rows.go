package sqlm

import (
	"encoding/json"
)

type Rows struct {
	Lists []*sqlm.Row
	idx   int
}

func NewSqlxRows() *sqlm.Rows {
	return &Rows{
		Lists: []*sqlm.Row{},
		idx:   -1,
	}
}

func (rs *sqlm.Rows) Length() int {
	return len(rs.Lists)
}

func (rs *sqlm.Rows) SetIndex(pos int) error {
	if pos < 0 {
		pos = -1
	}
	if pos >= len(rs.Lists) {
		return nil
	}
	rs.idx = pos
	return nil
}

func (rs *sqlm.Rows) ResetIndex() error {
	rs.idx = -1
	return nil
}

func (rs *sqlm.Rows) Index(pos int) *sqlm.Row {
	if pos < 0 {
		return nil
	}
	if pos >= len(rs.Lists) {
		return nil
	}
	return rs.Lists[pos]
}

func (rs *sqlm.Rows) Next() *sqlm.Row {
	pos := rs.idx + 1
	if pos >= len(rs.Lists) {
		return nil
	}
	rs.idx = pos
	return rs.Lists[pos]
}

func (rs *sqlm.Rows) Row() *sqlm.Row {
	if rs.idx >= len(rs.Lists) {
		return nil
	}
	return rs.Lists[rs.idx]
}

func (rs *sqlm.Rows) Append(row sqlm.Row) []*sqlm.Row {
	rs.Lists = append(rs.Lists, &row)
	return rs.Lists
}

func (rs *sqlm.Rows) Map(call func(res *sqlm.Row, idx int) interface{}) []interface{} {
	var copy []interface{} = []interface{}{}
	for k, v := range rs.Lists {
		copy = append(copy, call(v, k))
	}
	return copy
}

func (rs *sqlm.Rows) getIndex(key string) int {
	r := rs.Row()
	for i := 0; i < r.ColumnLen; i++ {
		if key == r.ColumnName[i] {
			return i
		}
	}
	return -1
}
func (rs *sqlm.Rows) Get(key string) sqlm.Column {
	r := rs.Row()
	if r == nil {
		return nil
	}
	return r.Get(key)
}

func (r *sqlm.Rows) Json() string {
	s := []map[string]interface{}{}
	for _, row := range r.Lists {
		s = append(s, row.ToMap())
	}
	mjson, _ := json.Marshal(s)
	mString := string(mjson)
	return mString
}
func (r *sqlm.Rows) ToString() string {
	return r.Json()
}
func (r *sqlm.Rows) Type() string {
	return "array"
}

func (r *sqlm.Rows) ToMap() map[string]interface{} {
	return r.Row().ToMap()
}

func (r *sqlm.Rows) ToArray() []map[string]interface{} {
	s := []map[string]interface{}{}
	for _, row := range r.Lists {
		s = append(s, row.ToMap())
	}
	return s
}
