package sqlm

import "reflect"

// 通过json Tag中的omitempty 过滤结构体
// Intime int64 `json:"intime,omitempty" ignore:"api"`
func Ignore(target any) {
	if target == nil {
		return
	}
	ty := reflect.TypeOf(target)
	val := reflect.ValueOf(target)
	if ty.Kind() == reflect.Pointer {
		if val.IsNil() {
			return
		}
		ty = ty.Elem()
		val = val.Elem()
	} else if val.Kind() == reflect.Pointer {
		// 通过any传入的指针分支不会走到，这里做兜底
		if val.IsNil() {
			return
		}
	}
	if ty.Kind() != reflect.Struct && ty.Kind() != reflect.Slice {
		return
	}
	// 数组
	if ty.Kind() == reflect.Slice {
		for i := 0; i < val.Len(); i++ {
			elem := val.Index(i)
			if elem.Kind() == reflect.Pointer {
				Ignore(elem.Interface())
				continue
			}
			if elem.Kind() != reflect.Struct {
				continue
			}
			if elem.CanAddr() {
				// 传地址进去，否则无法修改切片元素
				Ignore(elem.Addr().Interface())
				continue
			}
			ignoreFields(elem)
		}
		return
	}
	ignoreFields(val)
}

// ignoreFields clears every field tagged with `ignore`.
func ignoreFields(val reflect.Value) {
	if val.Kind() != reflect.Struct {
		return
	}
	ty := val.Type()
	num := val.NumField()
	for i := 0; i < num; i++ {
		f := ty.Field(i)
		if _, ok := f.Tag.Lookup("ignore"); !ok {
			continue
		}
		field := val.Field(i)
		if !field.CanSet() {
			// 未导出字段/不可寻址的值无法写入，直接跳过
			continue
		}
		field.Set(reflect.Zero(f.Type))
	}
}
