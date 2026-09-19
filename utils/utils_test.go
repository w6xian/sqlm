package utils_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/w6xian/sqlm/utils"
)

func TestGetString(t *testing.T) {
	assert.Equal(t, "", utils.GetString(nil))
	assert.Equal(t, "", utils.GetString(""))
	assert.Equal(t, "abc", utils.GetString("abc"))
	assert.Equal(t, "12", utils.GetString(12))
	assert.Equal(t, "12", utils.GetString(int64(12)))
	assert.Equal(t, "12", utils.GetString(uint(12)))
	assert.Equal(t, "12", utils.GetString(uint64(12)))
	assert.Equal(t, "1.5", utils.GetString(1.5))
	assert.Equal(t, "1.5", utils.GetString(float32(1.5)))
	assert.Equal(t, "bytes", utils.GetString([]byte("bytes")))
	assert.Equal(t, "true", utils.GetString(true))

	type inner struct {
		A int `json:"a"`
	}
	assert.JSONEq(t, `{"a":1}`, utils.GetString(inner{A: 1}))
}

func TestGetInt64AndInt(t *testing.T) {
	assert.Equal(t, int64(0), utils.GetInt64("abc"))
	assert.Equal(t, int64(8), utils.GetInt64("8"))
	assert.Equal(t, int64(8), utils.GetInt64(8))
	assert.Equal(t, int64(8), utils.GetInt64(int64(8)))
	assert.Equal(t, int64(8), utils.GetInt64(float64(8.9)))
	assert.Equal(t, int64(0), utils.GetInt64(struct{}{}))

	assert.Equal(t, 0, utils.GetInt("x"))
	assert.Equal(t, 42, utils.GetInt("42"))
}

func TestParseInt64AndFloat(t *testing.T) {
	v, err := utils.ParseInt64("1024")
	require.NoError(t, err)
	assert.Equal(t, int64(1024), v)
	_, err = utils.ParseInt64("nope")
	assert.Error(t, err)

	assert.Equal(t, 3.25, utils.GetFloat64("3.25"))
	assert.Zero(t, utils.GetFloat64("abc"))
}

func TestBuildSqlQ(t *testing.T) {
	assert.Nil(t, utils.BuildSqlQ(0))
	assert.Equal(t, []string{"?"}, utils.BuildSqlQ(1))
	assert.Equal(t, []string{"?", "?", "?"}, utils.BuildSqlQ(3))
}

func TestIsEmpty(t *testing.T) {
	assert.True(t, utils.IsEmpty(0))
	assert.True(t, utils.IsEmpty(int64(0)))
	assert.True(t, utils.IsEmpty(uint(0)))
	assert.True(t, utils.IsEmpty(0.0))
	assert.True(t, utils.IsEmpty(""))
	assert.True(t, utils.IsEmpty(false))
	assert.True(t, utils.IsEmpty(nil))
	assert.True(t, utils.IsEmpty(map[string]string(nil)))
	assert.True(t, utils.IsEmpty([]int(nil)))
	assert.True(t, utils.IsEmpty(time.Time{}))
	assert.True(t, utils.IsEmpty(struct{ A int }{}))

	assert.False(t, utils.IsEmpty(1))
	assert.False(t, utils.IsEmpty("x"))
	assert.False(t, utils.IsEmpty(true))
	assert.False(t, utils.IsEmpty(struct{ A int }{A: 1}))
	assert.False(t, utils.IsEmpty([]int{}))
}

func TestGetOrDefault(t *testing.T) {
	assert.Equal(t, "fallback", utils.GetOrDefault("", "fallback"))
	assert.Equal(t, 5, utils.GetOrDefault(0, 5))
	assert.Equal(t, "kept", utils.GetOrDefault("kept", "fallback"))
	assert.Equal(t, 3, utils.GetOrDefault(3, 5))
	fallback := []string{"fb"}
	assert.Equal(t, fallback, utils.GetOrDefault([]string(nil), fallback))
}

func TestSqlParse(t *testing.T) {
	assert.Equal(t, "SELECT * FROM t WHERE id = 1 AND name = '张三'",
		utils.SqlParse("SELECT * FROM t WHERE id = ? AND name = ?", 1, "张三"))
	assert.Equal(t, "SELECT * FROM t WHERE name = NULL",
		utils.SqlParse("SELECT * FROM t WHERE name = ?", nil))
	assert.Equal(t, "SELECT * FROM t WHERE ratio = 1.5 AND ok = 'true'",
		utils.SqlParse("SELECT * FROM t WHERE ratio = ? AND ok = ?", 1.5, true))

	// 单引号被转义，注入无法跳出字面量
	assert.Equal(t, "SELECT * FROM t WHERE name = ''' OR ''1''=''1'",
		utils.SqlParse("SELECT * FROM t WHERE name = ?", "' OR '1'='1"))

	// 多余的问号、参数不足都不能panic
	assert.Equal(t, "SELECT 1 1 ?", utils.SqlParse("SELECT 1 ? ?", 1))
	assert.Equal(t, "SELECT 1", utils.SqlParse("SELECT 1"))
	assert.Equal(t, "SELECT ?", utils.SqlParse("SELECT ?"))
}

func TestSqlParseDropsNullByte(t *testing.T) {
	assert.Equal(t, "SELECT 'a'", utils.SqlParse("SELECT ?", "a\x00b"[:1]+""))
	assert.NotContains(t, utils.SqlParse("SELECT ?", string([]byte{'a', 0, 'b'})), "\x00")
}

func TestRandBytes(t *testing.T) {
	assert.Empty(t, utils.RandBytes(0))
	assert.Len(t, utils.RandBytes(16), 16)
	a, b := utils.RandBytes(32), utils.RandBytes(32)
	assert.NotEqual(t, a, b)
}

func TestCopyAndDeepCopy(t *testing.T) {
	type payload struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}
	src := &payload{Name: "a", Age: 1}

	dst := &payload{}
	utils.Copy(dst, src)
	assert.Equal(t, "a", dst.Name)
	assert.Equal(t, 1, dst.Age)

	deep := &payload{}
	require.NoError(t, utils.DeepCopy(deep, src))
	assert.Equal(t, src, deep)

	// 目标类型不匹配时报错而不是崩溃
	var wrong int
	assert.Error(t, utils.DeepCopy(&wrong, src))
}

func TestCheckDataDir(t *testing.T) {
	dir := t.TempDir()
	abs, err := utils.CheckDataDir(dir)
	require.NoError(t, err)
	assert.Equal(t, filepath.Clean(dir), filepath.Clean(abs))

	// 结尾的斜杠被去掉
	trimmed, err := utils.CheckDataDir(dir + string(os.PathSeparator))
	require.NoError(t, err)
	assert.Equal(t, filepath.Clean(dir), filepath.Clean(trimmed))

	_, err = utils.CheckDataDir(filepath.Join(dir, "missing-subdir"))
	assert.Error(t, err)
}
