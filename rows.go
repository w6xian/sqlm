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

// NewSqlxRowsWithCap builds an empty Rows with a pre-sized backing array.
// It avoids the regrow cost when the row count is known upfront.
func NewSqlxRowsWithCap(capacity int) *Rows {
	if capacity < 0 {
		capacity = 0
	}
	return &Rows{
		Lists: make([]*Row, 0, capacity),
		idx:   -1,
	}
}

func (rs *Rows) Length() int {
	if rs == nil {
		return 0
	}
	return len(rs.Lists)
}

func (rs *Rows) SetIndex(pos int) error {
	if rs == nil {
		return ErrNilArgument
	}
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
	if rs == nil {
		return ErrNilArgument
	}
	rs.idx = -1
	return nil
}

func (rs *Rows) Index(pos int) *Row {
	if rs == nil || pos < 0 || pos >= len(rs.Lists) {
		return nil
	}
	return rs.Lists[pos]
}

func (rs *Rows) Next() *Row {
	if rs == nil {
		return nil
	}
	pos := rs.idx + 1
	if pos >= len(rs.Lists) {
		return nil
	}
	rs.idx = pos
	return rs.Lists[pos]
}

// Row returns the row the cursor currently points at (nil before the first
// Next() call).
func (rs *Rows) Row() *Row {
	if rs == nil || rs.idx < 0 || rs.idx >= len(rs.Lists) {
		return nil
	}
	return rs.Lists[rs.idx]
}

func (rs *Rows) Append(row Row) []*Row {
	if rs == nil {
		return nil
	}
	rs.Lists = append(rs.Lists, &row)
	return rs.Lists
}

func (rs *Rows) Map(call func(res *Row, idx int) any) []any {
	if rs == nil {
		return nil
	}
	list := make([]any, 0, len(rs.Lists))
	for k, v := range rs.Lists {
		list = append(list, call(v, k))
	}
	return list
}

// GetIndex returns the position of the column inside the current row.
func (rs *Rows) GetIndex(key string) int {
	if rs == nil {
		return -1
	}
	r := rs.Row()
	if r == nil {
		return -1
	}
	return r.getIdx(key)
}

func (rs *Rows) Get(key string) Column {
	if rs == nil {
		return nil
	}
	r := rs.Row()
	if r == nil {
		return nil
	}
	return r.Get(key)
}

func (rs *Rows) ColumnNames() []string {
	if rs == nil || len(rs.Lists) == 0 {
		return nil
	}
	return rs.Lists[0].ColumnNames()
}

func (rs *Rows) Json() string {
	if rs == nil {
		return "[]"
	}
	s := make([]map[string]any, 0, len(rs.Lists))
	for _, row := range rs.Lists {
		s = append(s, row.ToMap())
	}
	mjson, _ := json.Marshal(s)
	return string(mjson)
}

func (rs *Rows) ToString() string {
	return rs.Json()
}

func (rs *Rows) Type() string {
	return "array"
}

func (rs *Rows) ToMap() map[string]any {
	if rs == nil {
		return map[string]any{}
	}
	r := rs.Row()
	if r == nil {
		return map[string]any{}
	}
	return r.ToMap()
}

func (rs *Rows) ToArray() []map[string]any {
	if rs == nil {
		return []map[string]any{}
	}
	s := make([]map[string]any, 0, len(rs.Lists))
	for _, row := range rs.Lists {
		s = append(s, row.ToMap())
	}
	return s
}

func (rs *Rows) ToKeyMap(col string) map[string]*Row {
	if rs == nil {
		return map[string]*Row{}
	}
	m := make(map[string]*Row, len(rs.Lists))
	for _, row := range rs.Lists {
		m[row.Get(col).String()] = row
	}
	return m
}

func (rs *Rows) ToKeyValueMap(keyCol, valueCol string) map[string]Column {
	if rs == nil {
		return map[string]Column{}
	}
	m := make(map[string]Column, len(rs.Lists))
	for _, row := range rs.Lists {
		m[row.Get(keyCol).String()] = row.Get(valueCol)
	}
	return m
}

// 扫描多行数据到目标对象
//
//	rows.Scan(&rst, func() any {
//		return &Products{}
//	})
func (rs *Rows) Scan(target any, f func(*Row) any) error {
	if rs == nil {
		return ErrNilArgument
	}
	if target == nil {
		return ErrNilArgument
	}
	ty := reflect.TypeOf(target)
	if ty.Kind() != reflect.Ptr {
		return ErrUnsupportedType
	}
	tv := reflect.ValueOf(target)
	if tv.IsNil() {
		return ErrNilArgument
	}
	tv = tv.Elem()
	ty = ty.Elem()
	if ty.Kind() != reflect.Slice {
		return ErrUnsupportedType
	}
	tv.Set(reflect.MakeSlice(ty, rs.Length(), rs.Length()))
	for i, row := range rs.Lists {
		t := f(row)
		if t == nil {
			continue
		}
		if err := row.Scan(t); err != nil {
			return err
		}
		if !assignValue(tv.Index(i), reflect.ValueOf(t)) {
			return ErrUnsupportedType
		}
	}
	return nil
}

// assignValue fills dst with src, dereferencing/addressing src when needed so
// that both []T and []*T work whatever the factory function returns.
func assignValue(dst reflect.Value, src reflect.Value) bool {
	if !dst.CanSet() || !src.IsValid() {
		return false
	}
	if src.Type() == dst.Type() {
		dst.Set(src)
		return true
	}
	if src.Kind() == reflect.Ptr {
		if elem := src.Elem(); elem.IsValid() && elem.Type() == dst.Type() {
			dst.Set(elem)
			return true
		}
	}
	if dst.Kind() == reflect.Ptr && src.Type() == dst.Type().Elem() {
		p := reflect.New(src.Type())
		p.Elem().Set(src)
		dst.Set(p)
		return true
	}
	return false
}

// 扫描多行数据到目标对象
//
//	rst := []*TestRequest{}
//	rst := []TestRequest{}
//	rows.ScanMulti(&rst)
func (rs *Rows) ScanMulti(target any) (err error) {
	if rs == nil {
		return ErrNilArgument
	}
	if rs.Length() == 0 {
		// 空结果集直接返回，避免反射越界
		return nil
	}
	defer func() {
		if e := recover(); e != nil {
			err = ErrUnsupportedType
		}
	}()
	if target == nil {
		return ErrNilArgument
	}
	ty := reflect.TypeOf(target)
	tv := reflect.ValueOf(target)
	if ty.Kind() != reflect.Ptr {
		return ErrUnsupportedType
	}
	if tv.IsNil() {
		return ErrNilArgument
	}
	tv = tv.Elem()
	ty = tv.Type()
	if ty.Kind() != reflect.Slice {
		return ErrUnsupportedType
	}
	tv.Set(reflect.MakeSlice(ty, rs.Length(), rs.Length()))
	sliceType := ty.Elem()
	isNeedPtr := sliceType.Kind() == reflect.Ptr
	if isNeedPtr {
		sliceType = sliceType.Elem()
	}
	for i, row := range rs.Lists {
		t := reflect.New(sliceType)
		if e := row.Scan(t.Interface()); e != nil {
			return e
		}
		if !isNeedPtr {
			t = t.Elem()
		}
		tv.Index(i).Set(t)
	}
	return nil
}

func ScanMulti[T comparable](rows *Rows, target T) []*T {
	if rows == nil {
		return []*T{}
	}
	ty := reflect.TypeOf(target)
	if ty == nil {
		return []*T{}
	}
	if ty.Kind() == reflect.Ptr {
		panic("use P{} not &P{}")
	}
	ts := make([]*T, 0, rows.Length())
	for _, row := range rows.Lists {
		t := reflect.New(ty).Interface().(*T)
		_ = row.Scan(t)
		ts = append(ts, t)
	}
	return ts
}
