package sqlm

import (
	"database/sql/driver"
	"reflect"
)

func supportedColumnType(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32,
		reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32,
		reflect.Uint64, reflect.Float32, reflect.Float64, reflect.Interface,
		reflect.String:
		return true
	case reflect.Ptr, reflect.Slice, reflect.Array:
		ptrVal := reflect.New(v.Type().Elem())
		return supportedColumnType(ptrVal.Elem())
	default:
		return false
	}
}

func isValidSqlValue(v reflect.Value) bool {
	// This method covers two cases in which we know the Value can be converted to sql:
	// 1. It returns true for sql.driver's type check for types like time.Time
	// 2. It implements the driver.Valuer interface allowing conversion directly
	//    into sql statements
	if v.Kind() == reflect.Ptr {
		ptrVal := reflect.New(v.Type().Elem())
		return isValidSqlValue(ptrVal.Elem())
	}

	if driver.IsValue(v.Interface()) {
		return true
	}

	valuerType := reflect.TypeOf((*driver.Valuer)(nil)).Elem()
	return v.Type().Implements(valuerType)
}

func setColumnValue(v reflect.Value, c Column) {
	switch v.Kind() {
	case reflect.Bool:
		v.SetBool(c.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(c.NullInt64().Int64)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		d, _ := c.Uint64()
		v.SetUint(d)
	case reflect.Float32, reflect.Float64:
		f, _ := c.Float64()
		v.SetFloat(f)
	case reflect.String:
		v.SetString(c.String())
	case reflect.Interface:
	case reflect.Ptr, reflect.Slice, reflect.Array:
		ptrVal := reflect.New(v.Type().Elem())
		setColumnValue(ptrVal.Elem(), c)
	default:
	}
}
