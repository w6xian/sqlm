package sqlm

import (
	"database/sql"
	"encoding/json"
	"reflect"
	"strings"
)

type Row struct {
	Data       [][]byte
	ColumnName []string
	ColumnLen  int
	Base       *sql.Rows

	// idx caches "column name -> position". It may be shared with the sibling
	// rows produced by the same query and must be treated as read only.
	idx map[string]int
}

func (r *Row) Length() int {
	if r == nil {
		return 0
	}
	return len(r.Data)
}

// Has reports whether the row carries the given column.
func (r *Row) Has(key string) bool {
	return r.getIdx(key) >= 0
}

// ColumnNames returns a copy of the column names.
func (r *Row) ColumnNames() []string {
	if r == nil || len(r.ColumnName) == 0 {
		return nil
	}
	names := make([]string, len(r.ColumnName))
	copy(names, r.ColumnName)
	return names
}

// buildIndex pre-computes the column name index. Sharing the map between the
// rows of a same result set avoids rebuilding it for every row.
func (r *Row) buildIndex() map[string]int {
	cols := r.ColumnName
	if len(cols) == 0 {
		return nil
	}
	idx := make(map[string]int, len(cols))
	for i, name := range cols {
		if _, ok := idx[name]; !ok {
			idx[name] = i
		}
	}
	return idx
}

// SetIndex attaches a shared column index to the row.
func (r *Row) SetIndex(idx map[string]int) *Row {
	r.idx = idx
	return r
}

func (r *Row) getIdx(key string) int {
	if r == nil || key == "" {
		return -1
	}
	// "id,name" only targets the first column.
	if i := strings.IndexByte(key, ','); i >= 0 {
		key = key[:i]
	}
	if r.idx != nil {
		if i, ok := r.idx[key]; ok {
			return i
		}
		return -1
	}
	n := r.ColumnLen
	if n > len(r.Data) {
		n = len(r.Data)
	}
	if n > len(r.ColumnName) {
		n = len(r.ColumnName)
	}
	for i := 0; i < n; i++ {
		if key == r.ColumnName[i] {
			return i
		}
	}
	return -1
}

func (r *Row) Get(key string) Column {
	if r == nil {
		return nil
	}
	if index := r.getIdx(key); index >= 0 {
		return Column(r.Data[index])
	}
	return nil
}

func (r *Row) GetIndex(index int) Column {
	if r == nil || index < 0 || index >= len(r.Data) {
		return nil
	}
	return Column(r.Data[index])
}

func (r *Row) Json() string {
	m := r.ToMap()
	mjson, _ := json.Marshal(m)
	return string(mjson)
}

func (r *Row) ToString() string {
	return r.Json()
}

func (r *Row) ToMap() map[string]any {
	if r == nil {
		return map[string]any{}
	}
	n := r.ColumnLen
	if n > len(r.Data) {
		n = len(r.Data)
	}
	if n > len(r.ColumnName) {
		n = len(r.ColumnName)
	}
	js := make(map[string]any, n)
	for i := 0; i < n; i++ {
		js[r.ColumnName[i]] = string(r.Data[i])
	}
	return js
}

// 结果直接转结构体,请在Tag里用`json:"id"`绑定数据列名称。
//
//	rst:=&T{}
//	row.Scan(rst)
func (r *Row) Scan(target any) error {
	if target == nil {
		return ErrNilArgument
	}
	// 可能没有数据
	if r == nil || r.Length() <= 0 {
		return ErrNotFound
	}
	sVal := reflect.ValueOf(target)
	if sVal.Kind() != reflect.Ptr {
		return ErrUnsupportedType
	}
	if sVal.IsNil() {
		return ErrNilArgument
	}
	sVal = sVal.Elem()
	sType := sVal.Type()
	if sType.Kind() != reflect.Struct {
		return ErrUnsupportedType
	}
	// Pre-compute the column index once: a struct usually reads a dozen columns.
	if r.idx == nil {
		r.idx = r.buildIndex()
	}
	num := sVal.NumField()
	for i := 0; i < num; i++ {
		f := sType.Field(i)
		val := sVal.Field(i)
		if !val.CanSet() {
			// 未导出字段无法写入，直接跳过
			continue
		}
		key, skip := scanKey(f)
		if skip {
			continue
		}
		if col := r.Get(key); col != nil {
			// 是否支持
			if supportedColumnType(val) {
				setColumnValue(val, col)
			}
		}
	}
	return nil
}

// scanKey extracts the mapped column name from a struct field.
func scanKey(f reflect.StructField) (key string, skip bool) {
	tag, ok := f.Tag.Lookup("json")
	if ok {
		name := tag
		if i := strings.IndexByte(tag, ','); i >= 0 {
			name = tag[:i]
		}
		name = strings.TrimSpace(name)
		if name == "-" {
			return "", true
		}
		if name != "" {
			return name, false
		}
	}
	if v, ok := f.Tag.Lookup("ignore"); ok && hasReadIgnore(v) {
		return "", true
	}
	return f.Name, false
}

// hasReadIgnore keeps backward compatibility with ignore tags such as
// `ignore:"io"`: when an "i" shows up the field is skipped for Scan().
func hasReadIgnore(v string) bool {
	for i, c := range v {
		if i >= 2 {
			break
		}
		if c == 'i' {
			return true
		}
	}
	return false
}

func (r *Row) ScanMulti(target any) error {
	if target == nil {
		return ErrNilArgument
	}
	ty := reflect.TypeOf(target)
	if ty.Kind() != reflect.Ptr {
		return ErrUnsupportedType
	}
	if reflect.ValueOf(target).IsNil() {
		return ErrNilArgument
	}
	tv := reflect.ValueOf(target).Elem()
	ty = ty.Elem()
	if ty.Kind() == reflect.Slice {
		tv.Set(reflect.MakeSlice(ty, 1, 1))
		sliceType := ty.Elem()
		isNeedPtr := sliceType.Kind() == reflect.Ptr
		if isNeedPtr {
			sliceType = sliceType.Elem()
		}
		t := reflect.New(sliceType)
		if err := r.Scan(t.Interface()); err != nil {
			return err
		}
		if !isNeedPtr {
			t = t.Elem()
		}
		tv.Index(0).Set(reflect.ValueOf(t.Interface()))
		return nil
	}
	return r.Scan(target)
}

func (r *Row) Type() string {
	return "map"
}
