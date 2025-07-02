package sqlm

import (
	"context"
	"fmt"
	"reflect"
	"strings"
)

func Tb(tbl string) *Table {
	return NewTableWithContext(context.Background(), tbl)
}

func Tbx(ctx context.Context, tbl string) *Table {
	return NewTableWithContext(ctx, tbl)
}

func String(value string) string {
	return strings.ReplaceAll(value, "'", "''")
}

func Value(value interface{}) string {
	return fmt.Sprintf("'%s'", value)
}

func Int(value int) int {
	return value
}

func UInt(value uint) uint {
	return value
}

func Int16(value int16) int16 {
	return value
}

func UInt16(value uint16) uint16 {
	return value
}

func Int8(value int8) int8 {
	return value
}

func UInt8(value uint8) uint8 {
	return value
}

func Scan[T comparable](r *Rows, target T) []T {
	for _, row := range r.Lists {
		t := reflect.New(ty).Interface()
		row.Scan(t)
		ts = append(ts, t)
	}
	return ts
}
