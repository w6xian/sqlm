package sqlm

import (
	"database/sql"
	"errors"
	"strconv"
)

type Column []byte

func (col Column) Length() int {
	return len(col)
}
func (col Column) Int() (int, error) {
	if i, err := strconv.Atoi(string(col)); err == nil {
		return i, nil
	}
	return 0, errors.New("字段不存在")
}

func (col Column) Bool() bool {
	if i, err := strconv.Atoi(string(col)); err == nil {
		if i > 0 {
			return true
		}
	}
	return false
}
func (col Column) Int64() (int64, error) {
	return strconv.ParseInt(string(col), 10, 64)
}
func (col Column) NullInt64() sql.NullInt64 {
	i64, err := strconv.ParseInt(string(col), 10, 64)
	return sql.NullInt64{Int64: i64, Valid: err == nil}
}
func (col Column) Uint64() (uint64, error) {
	return strconv.ParseUint(string(col), 10, 64)
}

func (col Column) Float64() (float64, error) {
	return strconv.ParseFloat(string(col), 64)
}

func (col Column) Interface() any {
	return col
}

func (col Column) String() string {
	return string(col)
}

func (col Column) NullString() sql.NullString {
	s := string(col)
	return sql.NullString{String: s, Valid: s != ""}
}
