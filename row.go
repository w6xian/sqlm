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
}

func (r *Row) Length() int {
	return len(r.Data)
}
func (r *Row) getIdx(key string) int {
	keys := strings.Split(key, ",")
	key = keys[0]
	for i := 0; i < r.ColumnLen; i++ {
		if key == r.ColumnName[i] {
			return i
		}
	}
	return -1
}
func (r *Row) Get(key string) Column {
	index := r.getIdx(key)
	if index >= 0 {
		return Column(r.Data[index])
	}
	return nil
}

func (r *Row) GetIndex(index int) Column {
	if index >= 0 {
		return Column(r.Data[index])
	}
	return nil
}

func (r *Row) Json() string {
	m := r.ToMap()
	mjson, _ := json.Marshal(m)
	mString := string(mjson)
	return mString
}

func (r *Row) ToString() string {
	return r.Json()
}

func (r *Row) ToMap() map[string]any {
	js := make(map[string]any)
	for i := 0; i < r.ColumnLen; i++ {
		js[r.ColumnName[i]] = Column(r.Data[i]).String()
	}
	return js
}

// 结果直接转结构体,请在Tag里用`json:"id"`绑定数据列名称。
//
//	rst:=&T{}
//	row.Scan(rst)
func (r *Row) Scan(target any) {
	// 可能没有数据
	if r.Length() <= 0 {
		return
	}
	sVal := reflect.ValueOf(target)
	sType := reflect.TypeOf(target)
	if sType.Kind() == reflect.Ptr {
		sVal = sVal.Elem()
		sType = sType.Elem()
	}
	num := sVal.NumField()
	for i := 0; i < num; i++ {
		f := sType.Field(i)
		val := sVal.Field(i)
		key := f.Tag.Get("json")
		if v, ok := f.Tag.Lookup("ignore"); ok {
			if len(v) <= 2 {
				io := strings.Split(v, "")
				if io[0] == "i" || io[1] == "i" {
					continue
				}
			}
		}

		if col := r.Get(key); col != nil {
			// 是否支持
			if supportedColumnType(val) {
				setColumnValue(val, col)
			}
		}
	}
}

func (r *Row) Type() string {
	return "map"
}
