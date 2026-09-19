package sqlm

import (
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
		// 仅支持空接口(any)，有方法的接口无法从字节推断实现
		if v.NumMethod() == 0 && c != nil {
			v.Set(reflect.ValueOf(c.String()))
		}
	case reflect.Slice:
		if v.Type().Elem().Kind() == reflect.Uint8 {
			// []byte 按原始字节写入，避免拷错
			if c == nil {
				v.SetBytes(nil)
				return
			}
			v.SetBytes(append([]byte(nil), c...))
			return
		}
	case reflect.Ptr:
		if len(c) == 0 {
			// NULL 列保持零值(nil)
			v.Set(reflect.Zero(v.Type()))
			return
		}
		ptrVal := reflect.New(v.Type().Elem())
		setColumnValue(ptrVal.Elem(), c)
		v.Set(ptrVal)
	default:
	}
}
