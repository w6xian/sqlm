package sqlm

import (
	"encoding/json"
)

type Row struct {
	Data       [][]byte
	ColumnName []string
	ColumnLen  int
}

func (r *Row) Length() int {
	return len(r.Data)
}
func (r *Row) getIdx(key string) int {
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

func (r *Row) ToMap() map[string]interface{} {
	js := make(map[string]interface{})
	for i := 0; i < r.ColumnLen; i++ {
		js[r.ColumnName[i]] = Column(r.Data[i]).String()
	}
	return js
}

func (r *Row) Type() string {
	return "map"
}
