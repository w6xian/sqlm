package sqlm_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/w6xian/sqlm"
)

func TestRowsIteration(t *testing.T) {
	db, _ := newSQLite(t, "rows_iter")
	createUsers(t, db)
	seedUsers(t, db, 4)

	rows, err := db.Table("users").QueryMulti()
	require.NoError(t, err)
	assert.Equal(t, 4, rows.Length())
	assert.Equal(t, "array", rows.Type())

	seen := []string{}
	for iter := rows.Next(); iter != nil; iter = rows.Next() {
		seen = append(seen, iter.Get("name").String())
	}
	assert.Equal(t, []string{"User000", "User001", "User002", "User003"}, seen)

	// 迭代结束后游标停在最后一行，ResetIndex回到开始之前
	assert.NotNil(t, rows.Row())
	require.NoError(t, rows.ResetIndex())
	assert.Nil(t, rows.Row())
	assert.Equal(t, "User000", rows.Next().Get("name").String())
}

func TestRowsCursor(t *testing.T) {
	db, _ := newSQLite(t, "rows_cursor")
	createUsers(t, db)
	seedUsers(t, db, 3)

	rows, err := db.Table("users").QueryMulti()
	require.NoError(t, err)

	require.NoError(t, rows.SetIndex(2))
	assert.Equal(t, "User002", rows.Row().Get("name").String())
	assert.Equal(t, "User002", rows.Get("name").String())

	// 越界的索引是空操作，游标停在原处
	require.NoError(t, rows.SetIndex(99))
	assert.Equal(t, "User002", rows.Row().Get("name").String())
	// 负值回到第一行之前
	require.NoError(t, rows.SetIndex(-5))
	assert.Nil(t, rows.Row())
	assert.Nil(t, rows.Index(99))
	assert.Nil(t, rows.Index(-1))
	assert.NotNil(t, rows.Index(0))
	assert.Nil(t, rows.Next().Get("nope"))

	col, err := rows.Get("id").Int()
	require.NoError(t, err)
	assert.Equal(t, 1, col)
	assert.GreaterOrEqual(t, rows.GetIndex("id"), 0)
	assert.Equal(t, -1, rows.GetIndex("unknown"))
}

func TestRowsConversions(t *testing.T) {
	db, _ := newSQLite(t, "rows_conv")
	createUsers(t, db)
	seedUsers(t, db, 2)

	rows, err := db.Table("users").QueryMulti()
	require.NoError(t, err)

	arr := rows.ToArray()
	require.Len(t, arr, 2)
	assert.Equal(t, "User000", arr[0]["name"])

	var js []map[string]any
	require.NoError(t, json.Unmarshal([]byte(rows.Json()), &js))
	assert.Len(t, js, 2)
	assert.Equal(t, rows.Json(), rows.ToString())

	keyMap := rows.ToKeyMap("name")
	require.Len(t, keyMap, 2)
	assert.Equal(t, "1", keyMap["User000"].Get("id").String())

	kv := rows.ToKeyValueMap("name", "id")
	require.Len(t, kv, 2)
	assert.Equal(t, "2", kv["User001"].String())

	require.NoError(t, rows.SetIndex(0))
	assert.Equal(t, "User000", rows.ToMap()["name"])

	names := rows.Map(func(r *sqlm.Row, idx int) any {
		return r.Get("name").String()
	})
	assert.Equal(t, []any{"User000", "User001"}, names)

	assert.Contains(t, rows.ColumnNames(), "name")
}

func TestRowsScanWithFactory(t *testing.T) {
	db, _ := newSQLite(t, "rows_scan_factory")
	createUsers(t, db)
	seedUsers(t, db, 3)

	rows, err := db.Table("users").QueryMulti()
	require.NoError(t, err)

	list := []User{}
	require.NoError(t, rows.Scan(&list, func(r *sqlm.Row) any { return &User{} }))
	require.Len(t, list, 3)
	assert.Equal(t, "User000", list[0].Name)
	assert.Equal(t, int64(18), list[0].Age)

	// 工厂返回指针，目标是 []*User
	ptrs := []*User{}
	require.NoError(t, rows.Scan(&ptrs, func(r *sqlm.Row) any { return &User{} }))
	require.Len(t, ptrs, 3)
	assert.Equal(t, "User002", ptrs[2].Name)

	// 工厂返回 nil 的行被跳过
	partial := []User{}
	require.NoError(t, rows.Scan(&partial, func(r *sqlm.Row) any {
		if r.Get("name").String() == "User001" {
			return nil
		}
		return &User{}
	}))
	require.Len(t, partial, 3)
	assert.Empty(t, partial[1].Name)
}

func TestRowsScanErrors(t *testing.T) {
	db, _ := newSQLite(t, "rows_scan_err")
	createUsers(t, db)
	seedUsers(t, db, 2)

	rows, err := db.Table("users").QueryMulti()
	require.NoError(t, err)

	assert.ErrorIs(t, rows.Scan(nil, func(r *sqlm.Row) any { return &User{} }), sqlm.ErrNilArgument)
	assert.ErrorIs(t, rows.Scan([]User{}, func(r *sqlm.Row) any { return &User{} }), sqlm.ErrUnsupportedType)
	assert.ErrorIs(t, rows.Scan(0, func(r *sqlm.Row) any { return &User{} }), sqlm.ErrUnsupportedType)

	var nilList *[]User
	assert.ErrorIs(t, rows.Scan(nilList, func(r *sqlm.Row) any { return &User{} }), sqlm.ErrNilArgument)
}

func TestRowsScanMulti(t *testing.T) {
	db, _ := newSQLite(t, "rows_scan_multi")
	createUsers(t, db)
	seedUsers(t, db, 3)

	rows, err := db.Table("users").QueryMulti()
	require.NoError(t, err)

	list := []User{}
	require.NoError(t, rows.ScanMulti(&list))
	require.Len(t, list, 3)
	assert.Equal(t, "User000", list[0].Name)
	assert.InDelta(t, 50.0, list[0].Score, 0.001)

	ptrs := []*User{}
	require.NoError(t, rows.ScanMulti(&ptrs))
	require.Len(t, ptrs, 3)
	assert.Equal(t, "User001", ptrs[1].Name)

	// []byte 字段按原样写入
	raw, err := db.Table("users").Select("avatar").Query()
	require.NoError(t, err)
	blob := &ptrUser{}
	require.NoError(t, raw.Scan(blob))
	assert.Equal(t, []byte("avatar"), blob.Avatar)
}

func TestRowsScanMultiErrors(t *testing.T) {
	db, _ := newSQLite(t, "rows_scan_multi_err")
	createUsers(t, db)
	seedUsers(t, db, 2)

	rows, err := db.Table("users").QueryMulti()
	require.NoError(t, err)

	assert.ErrorIs(t, rows.ScanMulti(nil), sqlm.ErrNilArgument)
	assert.ErrorIs(t, rows.ScanMulti([]User{}), sqlm.ErrUnsupportedType)
	assert.ErrorIs(t, rows.ScanMulti(0), sqlm.ErrUnsupportedType)
	var nilPtr *[]User
	assert.ErrorIs(t, rows.ScanMulti(nilPtr), sqlm.ErrNilArgument)

	// 目标不是结构体：内部的反射panic被转成错误
	strs := []string{}
	assert.ErrorIs(t, rows.ScanMulti(&strs), sqlm.ErrUnsupportedType)
}

func TestRowsGenericScanMulti(t *testing.T) {
	db, _ := newSQLite(t, "rows_generic")
	createUsers(t, db)
	seedUsers(t, db, 2)

	rows, err := db.Table("users").QueryMulti()
	require.NoError(t, err)

	list := sqlm.ScanMulti(rows, User{})
	require.Len(t, list, 2)
	assert.Equal(t, "User000", list[0].Name)

	empty := sqlm.ScanMulti[User](nil, User{})
	assert.Empty(t, empty)
}

func TestRowsNilSafety(t *testing.T) {
	var rows *sqlm.Rows
	assert.Equal(t, 0, rows.Length())
	assert.Nil(t, rows.Next())
	assert.Nil(t, rows.Row())
	assert.Nil(t, rows.Index(0))
	assert.Nil(t, rows.Get("x"))
	assert.Equal(t, -1, rows.GetIndex("x"))
	assert.Nil(t, rows.ColumnNames())
	assert.Equal(t, "[]", rows.Json())
	assert.Empty(t, rows.ToArray())
	assert.Empty(t, rows.ToMap())
	assert.Empty(t, rows.ToKeyMap("x"))
	assert.Empty(t, rows.ToKeyValueMap("x", "y"))
	assert.Nil(t, rows.Map(func(r *sqlm.Row, i int) any { return nil }))
	assert.ErrorIs(t, rows.SetIndex(0), sqlm.ErrNilArgument)
	assert.ErrorIs(t, rows.ResetIndex(), sqlm.ErrNilArgument)
	assert.ErrorIs(t, rows.ScanMulti(&[]User{}), sqlm.ErrNilArgument)
}

func TestNewSqlxRowsHelpers(t *testing.T) {
	rows := sqlm.NewSqlxRows()
	require.NotNil(t, rows)
	assert.Equal(t, 0, rows.Length())

	capped := sqlm.NewSqlxRowsWithCap(10)
	assert.Equal(t, 0, capped.Length())
	assert.Equal(t, 10, cap(capped.Lists))

	// 负数容量不应panic
	assert.NotPanics(t, func() { sqlm.NewSqlxRowsWithCap(-1) })

	rows.Append(sqlm.Row{
		Data:       [][]byte{[]byte("1")},
		ColumnName: []string{"id"},
		ColumnLen:  1,
	})
	require.Equal(t, 1, rows.Length())
	assert.Equal(t, "1", rows.Index(0).Get("id").String())
	assert.Equal(t, `[{"id":"1"}]`, rows.Json())
}
