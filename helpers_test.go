package sqlm_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/w6xian/sqlm"
)

func TestUnixTimeAndAlias(t *testing.T) {
	before := time.Now().Unix()
	got := sqlm.UnixTime()
	assert.GreaterOrEqual(t, got, before)
	assert.LessOrEqual(t, got, time.Now().Add(time.Minute).Unix())

	assert.Equal(t, "mi_users u", sqlm.Alias("mi_users", "u"))
}

func TestValueHelpers(t *testing.T) {
	// String() 用于手动拼接 SQL 时转义单引号
	assert.Equal(t, "ab", sqlm.String("ab"))
	assert.Equal(t, "a''b", sqlm.String("a'b"))

	assert.Equal(t, "'x'", sqlm.Value("x"))
	assert.Equal(t, 1, sqlm.Int(1))
	assert.Equal(t, uint(1), sqlm.UInt(1))
	assert.Equal(t, int16(1), sqlm.Int16(1))
	assert.Equal(t, uint16(1), sqlm.UInt16(1))
	assert.Equal(t, int8(1), sqlm.Int8(1))
	assert.Equal(t, uint8(1), sqlm.UInt8(1))
}

func TestTableShortcuts(t *testing.T) {
	assert.Equal(t, "SELECT * FROM mi_users", sqlm.Tb("mi_users").SQL())
	assert.Equal(t, "SELECT * FROM mi_users", sqlm.Tbx(nil, "mi_users").SQL())
}

func TestRows2MapRow(t *testing.T) {
	db, _ := newSQLite(t, "rows2map")
	createUsers(t, db)
	seedUsers(t, db, 3)

	rows, err := db.Table("users").QueryMulti()
	require.NoError(t, err)

	byName := sqlm.Rows2MapRow(rows, "name")
	assert.Len(t, byName, 3)
	require.Contains(t, byName, "User000")
	assert.Equal(t, "User000", byName["User000"].Get("name").String())

	// 游标被重置，Rows 可以重新从头遍历
	assert.Equal(t, "User000", rows.Next().Get("name").String())

	// nil Rows 返回空 map 而不是 panic
	assert.Empty(t, sqlm.Rows2MapRow(nil, "name"))
}

func TestOrderASCHelper(t *testing.T) {
	db, _ := newSQLite(t, "order_asc")
	assert.Equal(t, "SELECT * FROM mi_users ORDER BY id ASC", db.Table("users").OrderASC("id").SQL())
	assert.Equal(t, "SELECT * FROM mi_users ORDER BY id DESC", db.Table("users").OrderDESC("id").SQL())
}

func TestOptionWithLogger(t *testing.T) {
	lg := sqlm.NewMemLogger(sqlm.WARN)
	var opt *sqlm.Options
	// WithLogger 可以用0值接收者组装出一份配置
	opt = &sqlm.Options{}
	sqlm.WithLogger(lg)(opt)
	assert.Same(t, lg, opt.Logger())
}

// noJSONTag relies on the ignore tag: fields marked this way are skipped even
// when they carry no json tag.
type noJSONTag struct {
	Id     int64
	Name   string
	Secret string `ignore:"io"`
}

func TestScanSkipsIgnoredFieldWithoutJSONTag(t *testing.T) {
	db, _ := newSQLite(t, "scan_ignore_tag")
	createUsers(t, db)
	_, err := db.Table("users").Insert(map[string]any{"name": "Ign", "age": 3})
	require.NoError(t, err)

	rows, err := db.Table("users").QueryMulti()
	require.NoError(t, err)
	require.NotNil(t, rows.Next())

	target := &noJSONTag{}
	require.NoError(t, rows.Row().Scan(target))
	assert.Empty(t, target.Id)
	assert.Empty(t, target.Name)
	assert.Empty(t, target.Secret)
}
