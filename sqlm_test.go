package sqlm_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSqliteWalksPublicAPI runs one complete round trip through the public API
// (instance -> schema -> write -> query -> update -> delete) on its own sqlite
// file, asserting the data after every step instead of only checking errors.
func TestSqliteWalksPublicAPI(t *testing.T) {
	db, _ := newSQLite(t, "public_api")
	createUsers(t, db)

	for _, row := range []map[string]any{
		{"name": "Alice", "age": 25, "score": 95.5},
		{"name": "Bob", "age": 30, "score": 88.0},
		{"name": "Charlie", "age": 35, "score": 70.0},
	} {
		id, err := db.Table("users").Insert(row)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, id, int64(1))
	}

	total, err := db.Table("users").GetCount()
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)

	// Map 过滤器精确命中一行
	row, err := db.Table("users").Select("name,age").AndFilters(map[string]any{"name": "Alice"}).Query()
	require.NoError(t, err)
	// Row.Length() 是列数：这里只取了两列
	assert.Equal(t, 2, row.Length())
	assert.Equal(t, "Alice", row.Get("name").String())
	age, err := row.Get("age").Int64()
	require.NoError(t, err)
	assert.Equal(t, int64(25), age)

	// 旧式 fmt.Sprintf 传参仍然可用
	row, err = db.Table("users").Select("name").Where("age = %d", 30).Query()
	require.NoError(t, err)
	assert.Equal(t, "Bob", row.Get("name").String())

	// Where 与 AndFilters 可以混用，AND 拼接
	row, err = db.Table("users").Select("name").Where("score > %f", 80.0).AndFilters(map[string]any{"age": 30}).Query()
	require.NoError(t, err)
	assert.Equal(t, "Bob", row.Get("name").String())

	affected, err := db.Table("users").AndFilters(map[string]any{"name": "Charlie"}).
		Update(map[string]any{"score": 80}).Execute()
	require.NoError(t, err)
	assert.Equal(t, int64(1), affected)

	row, err = db.Table("users").Select("score").AndFilters(map[string]any{"name": "Charlie"}).Query()
	require.NoError(t, err)
	score, err := row.Get("score").Float64()
	require.NoError(t, err)
	assert.Equal(t, float64(80), score)

	affected, err = db.Table("users").AndFilters(map[string]any{"name": "Bob"}).Delete().Execute()
	require.NoError(t, err)
	assert.Equal(t, int64(1), affected)

	total, err = db.Table("users").GetCount()
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)

	names := []string{}
	rows, err := db.Table("users").Select("name").Order("name").QueryMulti()
	require.NoError(t, err)
	require.Equal(t, 2, rows.Length())
	for i := 0; i < rows.Length(); i++ {
		names = append(names, rows.Index(i).Get("name").String())
	}
	assert.Equal(t, []string{"Alice", "Charlie"}, names)
}
