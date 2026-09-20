package sqlm_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/w6xian/sqlm"
)

func column(t *testing.T, raw string) sqlm.Column {
	t.Helper()
	return sqlm.Column(raw)
}

func TestColumnConversions(t *testing.T) {
	num := column(t, "42")
	i, err := num.Int()
	require.NoError(t, err)
	assert.Equal(t, 42, i)

	i64, err := num.Int64()
	require.NoError(t, err)
	assert.Equal(t, int64(42), i64)

	u64, err := num.Uint64()
	require.NoError(t, err)
	assert.Equal(t, uint64(42), u64)

	assert.Equal(t, "42", num.String())
	assert.Equal(t, 2, num.Length())
	assert.True(t, num.Bool())
	assert.Equal(t, "42", num.NullString().String)
	assert.True(t, num.NullString().Valid)
	assert.Equal(t, int64(42), num.NullInt64().Int64)
	assert.True(t, num.NullInt64().Valid)
	assert.Equal(t, sqlm.Column("42"), num.Interface())

	f, err := column(t, "3.5").Float64()
	require.NoError(t, err)
	assert.InDelta(t, 3.5, f, 0.000001)
	assert.False(t, column(t, "0").Bool())
	assert.False(t, column(t, "abc").Bool())
}

func TestColumnConversionErrors(t *testing.T) {
	bad := column(t, "abc")
	_, err := bad.Int()
	assert.ErrorIs(t, err, sqlm.ErrNotNumeric)
	_, err = bad.Int64()
	assert.Error(t, err)
	_, err = bad.Uint64()
	assert.Error(t, err)
	_, err = bad.Float64()
	assert.Error(t, err)
	assert.False(t, bad.NullInt64().Valid)

	empty := column(t, "")
	assert.False(t, empty.NullString().Valid)
	assert.Equal(t, "", empty.NullString().String)
	assert.False(t, empty.NullInt64().Valid)
	assert.Equal(t, 0, empty.Length())
}

// TestColumnMissingDoesNotPanic covers columns that were not selected.
func TestColumnMissingDoesNotPanic(t *testing.T) {
	db, _ := newSQLite(t, "missing_col")
	createUsers(t, db)
	_, err := db.Table("users").Insert(map[string]any{"name": "A"})
	require.NoError(t, err)

	row, err := db.Table("users").Select("name").Query()
	require.NoError(t, err)

	assert.False(t, row.Has("mobile"))
	missing := row.Get("mobile")
	assert.Nil(t, missing)
	assert.NotPanics(t, func() {
		_ = missing.String()
		_, _ = missing.Int()
		_ = missing.Length()
		_ = missing.Bool()
		_ = missing.Interface()
	})
	assert.Nil(t, row.GetIndex(99))
	assert.Nil(t, row.GetIndex(-1))
	assert.Nil(t, row.Get(""))
}

func TestRowAccessors(t *testing.T) {
	db, _ := newSQLite(t, "row_accessors")
	createUsers(t, db)
	_, err := db.Table("users").Insert(map[string]any{"name": "A", "age": 7})
	require.NoError(t, err)

	row, err := db.Table("users").Query()
	require.NoError(t, err)

	assert.Equal(t, "A", row.Get("name").String())
	assert.Equal(t, "7", row.Get("age").String())
	// 逗号形式只取第一列
	assert.Equal(t, "A", row.Get("name,age").String())
	assert.True(t, row.Has("name"))
	assert.Contains(t, row.ColumnNames(), "name")
	assert.Equal(t, "map", row.Type())

	m := row.ToMap()
	assert.Equal(t, "A", m["name"])
	assert.Equal(t, "7", m["age"])

	js := row.Json()
	assert.Contains(t, js, `"name":"A"`)
	assert.Equal(t, js, row.ToString())

	idx := row.GetIndex(0)
	assert.NotNil(t, idx)
}

func TestRowSetIndexSharesColumnMap(t *testing.T) {
	db, _ := newSQLite(t, "row_share_index")
	createUsers(t, db)
	_, err := db.Table("users").Insert(map[string]any{"name": "shared"})
	require.NoError(t, err)

	rows, err := db.Table("users").QueryMulti()
	require.NoError(t, err)
	names := rows.ColumnNames()
	require.NotEmpty(t, names)

	r := rows.Index(0)
	// 共享索引让同一次查询的所有行共用一个 map
	shared := make(map[string]int, len(names))
	for i, name := range names {
		shared[name] = i
	}
	r.SetIndex(shared)
	assert.Equal(t, "shared", r.Get("name").String())

	// 索引里没有的列查不到，也不会panic
	partial := rows.Index(0).SetIndex(map[string]int{"name": 1})
	assert.Nil(t, partial.Get("nope"))
}

func TestRowScanVariants(t *testing.T) {
	db, _ := newSQLite(t, "row_scan")
	createUsers(t, db)
	_, err := db.Table("users").Insert(map[string]any{
		"name": "Bob", "age": 33, "score": 88.5, "mobile": "13800000000", "intime": 11,
	})
	require.NoError(t, err)

	row, err := db.Table("users").Query()
	require.NoError(t, err)

	u := &User{}
	require.NoError(t, row.Scan(u))
	assert.Equal(t, "Bob", u.Name)
	assert.Equal(t, int64(33), u.Age)
	assert.InDelta(t, 88.5, u.Score, 0.0001)
	assert.Equal(t, "13800000000", u.Mobile)
	assert.Equal(t, int64(11), u.Intime)

	// json tag policy / 无tag回退
	o := &optUser{}
	require.NoError(t, row.Scan(o))
	assert.Equal(t, int64(1), o.Id)
	assert.Equal(t, "Bob", o.Name)
	assert.Empty(t, o.Skip)

	// 没有json tag时按字段名精确匹配：库里建议始终写tag保证列名可控
	nt := &noTagUser{}
	require.NoError(t, row.Scan(nt))
	assert.Equal(t, int64(0), nt.Id)
	assert.Empty(t, nt.Name)

	// ignore:"io" 的字段不被写入
	ig := &ignores{}
	require.NoError(t, row.Scan(ig))
	assert.Empty(t, ig.Secret)
	assert.Equal(t, "Bob", ig.Name)

	// 指针字段
	pu := &ptrUser{}
	require.NoError(t, row.Scan(pu))
	require.NotNil(t, pu.Name)
	assert.Equal(t, "Bob", *pu.Name)

	// 切片目标：ScanMulti 只装一行
	list := []User{}
	require.NoError(t, row.ScanMulti(&list))
	require.Len(t, list, 1)
	assert.Equal(t, "Bob", list[0].Name)

	ptrs := []*User{}
	require.NoError(t, row.ScanMulti(&ptrs))
	require.Len(t, ptrs, 1)
	assert.Equal(t, "Bob", ptrs[0].Name)
}

func TestRowScanErrors(t *testing.T) {
	db, _ := newSQLite(t, "row_scan_err")
	createUsers(t, db)

	row, err := db.Table("users").Query()
	assert.Nil(t, row)
	assert.ErrorIs(t, err, sqlm.ErrNotFound)

	_, err = db.Table("users").Insert(map[string]any{"name": "A"})
	require.NoError(t, err)
	row, err = db.Table("users").Query()
	require.NoError(t, err)

	assert.ErrorIs(t, row.Scan(nil), sqlm.ErrNilArgument)
	assert.ErrorIs(t, row.Scan(User{}), sqlm.ErrUnsupportedType)
	var nilPtr *User
	assert.ErrorIs(t, row.Scan(nilPtr), sqlm.ErrNilArgument)
	assert.ErrorIs(t, row.Scan(&[]string{}), sqlm.ErrUnsupportedType)
	assert.ErrorIs(t, row.Scan(0), sqlm.ErrUnsupportedType)

	assert.ErrorIs(t, row.ScanMulti(nil), sqlm.ErrNilArgument)
	assert.ErrorIs(t, row.ScanMulti(User{}), sqlm.ErrUnsupportedType)

	// 空指针不能被解引用
	var nilList *[]User
	assert.ErrorIs(t, row.ScanMulti(nilList), sqlm.ErrNilArgument)

	// 未导出的字段被跳过而不是崩溃
	h := &hidden{}
	require.NoError(t, row.Scan(h))
	assert.Equal(t, int64(1), h.Id)

	// nil Row：所有读取都安全
	var nilRow *sqlm.Row
	assert.Equal(t, 0, nilRow.Length())
	assert.Nil(t, nilRow.Get("name"))
	assert.Empty(t, nilRow.ToMap())
	assert.Nil(t, nilRow.ColumnNames())
	assert.ErrorIs(t, nilRow.Scan(&User{}), sqlm.ErrNotFound)
}

func TestRowScanUnsetColumnKeepsZero(t *testing.T) {
	db, _ := newSQLite(t, "row_scan_partial")
	createUsers(t, db)
	_, err := db.Table("users").Insert(map[string]any{"name": "Partial"})
	require.NoError(t, err)

	row, err := db.Table("users").Select("id", "name").Query()
	require.NoError(t, err)

	u := &User{Score: 9.9}
	require.NoError(t, row.Scan(u))
	assert.Equal(t, "Partial", u.Name)
	assert.InDelta(t, 9.9, u.Score, 0.0001) // 未选中的列保持原值
}
