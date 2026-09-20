package sqlm_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/w6xian/sqlm"
)

// ---------------------------------------------------------------------------
// SQL 拼装：这些用例同时充当"重写拼装逻辑"的护栏，三种协议都要逐字节一致
// ---------------------------------------------------------------------------

// golden 用假连接跑一遍，只断言生成的语句文本。
func golden(t *testing.T, protocol string, build func(tb *sqlm.Table) error) string {
	t.Helper()
	tb, conn := pgTable(t, protocol)
	_ = build(tb)
	require.NotEmpty(t, conn.last, "没有生成任何语句")
	return conn.last
}

func TestAssembleGoldenPerProtocol(t *testing.T) {
	data := map[string]any{"name": "a", "age": 18}

	cases := []struct {
		protocol string
		insert   string
		inserts  string
		update   string
	}{
		{
			sqlm.MYSQL,
			"INSERT INTO `mi_users` (`age`,`name`) VALUES (?,?)",
			"INSERT INTO `mi_users` (`name`,`age`) VALUES (?,?),(?,?)",
			"UPDATE mi_users SET `age`='18',`name`='a' WHERE id = 1",
		},
		{
			sqlm.SQLITE,
			`INSERT INTO "mi_users" ("age","name") VALUES (?,?)`,
			`INSERT INTO "mi_users" ("name","age") VALUES (?,?),(?,?)`,
			`UPDATE mi_users SET "age"='18',"name"='a' WHERE id = 1`,
		},
		{
			sqlm.POSTGRES,
			`INSERT INTO "mi_users" ("age","name") VALUES ($1,$2)`,
			`INSERT INTO "mi_users" ("name","age") VALUES ($1,$2),($3,$4)`,
			`UPDATE mi_users SET "age"='18',"name"='a' WHERE id = 1`,
		},
	}

	for _, c := range cases {
		assert.Equal(t, c.insert, golden(t, c.protocol, func(tb *sqlm.Table) error {
			_, err := tb.Insert(data)
			return err
		}), c.protocol)

		assert.Equal(t, c.inserts, golden(t, c.protocol, func(tb *sqlm.Table) error {
			_, err := tb.Inserts([]string{"name", "age"}, [][]any{{"a", 1}, {"b", 2}})
			return err
		}), c.protocol)

		assert.Equal(t, c.update, golden(t, c.protocol, func(tb *sqlm.Table) error {
			_, err := tb.Update(data).Where("id = 1").Execute()
			return err
		}), c.protocol)

		assert.Equal(t, "DELETE FROM mi_users WHERE id = 1", golden(t, c.protocol, func(tb *sqlm.Table) error {
			_, err := tb.Delete().Where("id = 1").Execute()
			return err
		}), c.protocol)
	}
}

func TestAssembleSelectGolden(t *testing.T) {
	for _, protocol := range []string{sqlm.MYSQL, sqlm.SQLITE, sqlm.POSTGRES} {
		sql := pgBuilder(t, protocol).
			Select("id", "name").
			Where("age > %d", 18).
			And("vip = 1").
			GroupBy("shop_id").
			Order("name").
			OrderDESC("id").
			Limit(10).
			SQL()
		assert.Equal(t, "SELECT id,name FROM mi_users WHERE age > 18 AND vip = 1 "+
			"GROUP BY shop_id ORDER BY name ASC,id DESC LIMIT 10", sql, protocol)
	}

	// 空条件与前后空白不应留下多余空格
	db, _ := newSQLite(t, "assemble_spacing")
	assertSQLEqual(t, "SELECT * FROM mi_users WHERE id = 1",
		db.Table("users").Where("  id = 1  ").SQL())
	assertSQLEqual(t, "SELECT * FROM mi_users WHERE id = 1",
		db.Table("users").Where("").And("id = 1").SQL())
	assertSQLEqual(t, "SELECT count(*) as total FROM mi_users", db.Table("users").Count().SQL())
}

// ---------------------------------------------------------------------------
// 回归：拼装过程中被发现的缺陷
// ---------------------------------------------------------------------------

// Inserts 旧实现会就地改写调用方传入的 columns，同一个切片换个协议再用一次，
// 反引号就会留在双引号里面，语句直接写坏。
func TestInsertsLeavesCallerColumnsUntouched(t *testing.T) {
	cols := []string{"name", "age"}

	tb, conn := pgTable(t, sqlm.MYSQL)
	_, _ = tb.Inserts(cols, [][]any{{"a", 1}})
	assert.Equal(t, []string{"name", "age"}, cols, "调用方的切片被改写了")
	assert.Equal(t, "INSERT INTO `mi_users` (`name`,`age`) VALUES (?,?)", conn.last)

	tb, conn = pgTable(t, sqlm.POSTGRES)
	_, _ = tb.Inserts(cols, [][]any{{"a", 1}})
	assert.Equal(t, `INSERT INTO "mi_users" ("name","age") VALUES ($1,$2)`, conn.last)
}

// Set 没有参数时旧实现仍然跑 Sprintf，值里的 % 会被写成 %!(NOVERB)。
func TestSetWithoutArgsKeepsFormatVerbs(t *testing.T) {
	tb, conn := pgTable(t, sqlm.POSTGRES)
	// 取方法值间接调用：表达式里含 %，直接写会让 go vet 把它当成格式化串。
	// Set 只在调用方额外传参时才做格式化。
	set := tb.Set
	_, _ = set("name = '100%'").Where("id = 1").Execute()
	assert.Equal(t, `UPDATE mi_users SET name = '100%' WHERE id = 1`, conn.last)
}

// NUL 字节必须被处理掉：MySQL 写成 \0，标准协议下直接丢弃（否则 sqlite 会截断
// 语句、postgres 会报 invalid byte sequence）。
func TestEscapeHandlesNULPerProtocol(t *testing.T) {
	tb, conn := pgTable(t, sqlm.MYSQL)
	_, _ = tb.Update(map[string]any{"name": "a\x00b"}).Where("id = 1").Execute()
	assert.Equal(t, `UPDATE mi_users SET `+"`name`"+`='a\0b' WHERE id = 1`, conn.last)

	for _, protocol := range []string{sqlm.SQLITE, sqlm.POSTGRES} {
		tb, conn := pgTable(t, protocol)
		_, _ = tb.Update(map[string]any{"name": "a\x00b"}).Where("id = 1").Execute()
		assert.NotContains(t, conn.last, "\x00", protocol)
		assert.Equal(t, `UPDATE mi_users SET "name"='ab' WHERE id = 1`, conn.last, protocol)
	}
}

// 不需要转义的值不应该产生额外分配：这里只验证结果与原串一致。
func TestEscapeKeepsPlainValues(t *testing.T) {
	for _, protocol := range []string{sqlm.MYSQL, sqlm.SQLITE, sqlm.POSTGRES} {
		tb, conn := pgTable(t, protocol)
		_, _ = tb.Update(map[string]any{"name": "plain-value"}).Where("id = 1").Execute()
		assert.Contains(t, conn.last, "'plain-value'", protocol)
	}
}

// 拼装必须只依赖 builder 自身状态，重复调用同一条链应得到同一条语句。
func TestAssembleIsRepeatable(t *testing.T) {
	tb, conn := pgTable(t, sqlm.POSTGRES)
	tb.Select("id", "name").Where("age > 18").OrderDESC("id")
	first := tb.SQL()
	second := tb.SQL()
	assert.Equal(t, first, second)
	assert.Equal(t, "SELECT id,name FROM mi_users WHERE age > 18 ORDER BY id DESC", first)
	_ = conn
}
