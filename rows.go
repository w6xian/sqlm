package sqlm

import (
	"encoding/json"
	"reflect"
)

type Rows struct {
	Lists []*Row
	idx   int
}

func NewSqlxRows() *Rows {
	return &Rows{
		Lists: []*Row{},
		idx:   -1,
	}
}

func (rs *Rows) Length() int {
	return len(rs.Lists)
}

func (rs *Rows) SetIndex(pos int) error {
	if pos < 0 {
		pos = -1
	}
	if pos >= len(rs.Lists) {
		return nil
	}
	rs.idx = pos
	return nil
}

func (rs *Rows) ResetIndex() error {
	rs.idx = -1
	return nil
}

func (rs *Rows) Index(pos int) *Row {
	if pos < 0 {
		return nil
	}
	if pos >= len(rs.Lists) {
		return nil
	}
	return rs.Lists[pos]
}

func (rs *Rows) Next() *Row {
	pos := rs.idx + 1
	if pos >= len(rs.Lists) {
		return nil
	}
	rs.idx = pos
	return rs.Lists[pos]
}

func (rs *Rows) Row() *Row {
	if rs.idx >= len(rs.Lists) {
		return nil
	}
	return rs.Lists[rs.idx]
}

func (rs *Rows) Append(row Row) []*Row {
	rs.Lists = append(rs.Lists, &row)
	return rs.Lists
}

func (rs *Rows) Map(call func(res *Row, idx int) any) []any {
	var copy []any = []any{}
	for k, v := range rs.Lists {
		copy = append(copy, call(v, k))
	}
	return copy
}

func (rs *Rows) GetIndex(key string) int {
	r := rs.Row()
	for i := 0; i < r.ColumnLen; i++ {
		if key == r.ColumnName[i] {
			return i
		}
	}
	return -1
}
func (rs *Rows) Get(key string) Column {
	r := rs.Row()
	if r == nil {
		return nil
	}
	return r.Get(key)
}

func (r *Rows) Json() string {
	s := []map[string]any{}
	for _, row := range r.Lists {
		s = append(s, row.ToMap())
	}
	mjson, _ := json.Marshal(s)
	mString := string(mjson)
	return mString
}
func (r *Rows) ToString() string {
	return r.Json()
}
func (r *Rows) Type() string {
	return "array"
}

func (r *Rows) ToMap() map[string]any {
	return r.Row().ToMap()
}

func (r *Rows) ToArray() []map[string]any {
	s := []map[string]any{}
	for _, row := range r.Lists {
		s = append(s, row.ToMap())
	}
	return s
}

func (r *Rows) ToKeyMap(col string) map[string]*Row {
	m := map[string]*Row{}
	for _, row := range r.Lists {
		key := row.Get(col).String()
		m[key] = row
	}
	return m
}

func (r *Rows) ToKeyValueMap(keyCol, valueCol string) map[string]Column {
	m := map[string]Column{}
	for _, row := range r.Lists {
		key := row.Get(keyCol).String()
		value := row.Get(valueCol)
		m[key] = value
	}
	return m
}

// 建议用sqml.ScanMulti()
func (r *Rows) Scan(target any, f func() any) {
	ty := reflect.TypeOf(target)
	val := reflect.ValueOf(target).Elem()
	if ty.Kind() != reflect.Pointer {
		panic("Scan use &[]T{}  not []T{}")
	}
	ty = ty.Elem()
	if ty.Kind() != reflect.Slice {
		panic("Scan use *[]T{}  not []T{}")
	}

	val.Set(reflect.MakeSlice(ty, r.Length(), r.Length()))
	for i, row := range r.Lists {
		t := f()
		row.Scan(t)
		val.Index(i).Set(reflect.ValueOf(t))
	}
}

func ScanMulti[T comparable](rows *Rows, target T) []*T {
	ty := reflect.TypeOf(target)
	if ty.Kind() == reflect.Pointer {
		panic("use P{} not &P{}")
	}
	ts := []*T{}
	for _, row := range rows.Lists {
		t := reflect.New(ty).Interface().(*T)
		row.Scan(t)
		ts = append(ts, t)
	}
	return ts
}
