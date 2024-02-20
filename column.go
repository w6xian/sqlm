package sqlm

import (
	"errors"
	"strconv"
)

type Column []byte

func (rows Column) Length() int {
	return len(rows)
}
func (rows Column) Int() (int, error) {
	if i, err := strconv.Atoi(string(rows)); err == nil {
		return i, nil
	}
	return 0, errors.New("字段不存在")
}
func (rows Column) Int64() (int64, error) {
	return strconv.ParseInt(string(rows), 10, 64)
}

func (rows Column) Uint64() (uint64, error) {
	return strconv.ParseUint(string(rows), 10, 64)
}

func (rows Column) Float64() (float64, error) {
	return strconv.ParseFloat(string(rows), 64)
}

func (rows Column) Interface() interface{} {
	return rows
}

func (rows Column) String() string {
	return string(rows)
}
