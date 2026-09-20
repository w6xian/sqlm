package sqlm_test

import (
	"context"
	"errors"
	"testing"

	"github.com/w6xian/sqlm"
)

// 假连接直接失败，语句不会被真正执行。
var errBenchStop = errors.New("benchmark: statement not executed")

// 本文件只测"拼装 SQL"这一段：所有用到连接的地方都换成 recConn（table_test.go
// 里那个只会记录语句、不会真正执行的假连接），避免把数据库往返的耗时混进来。

var sqlSink string

// benchTable 返回一个绑定了假连接的 builder，protocol 可指定。
func benchTable(protocol string) (*sqlm.Table, *recConn) {
	conn := &recConn{err: errBenchStop}
	tb := sqlm.Tbx(context.Background(), "users").
		PreTable("mi_").
		SetProtocol(protocol).
		UseConn(conn)
	return tb, conn
}

func benchInsertData() map[string]any {
	return map[string]any{
		"name":   "BenchUser",
		"age":    18,
		"score":  50.5,
		"mobile": "13800000000",
		"avatar": []byte("avatar"),
		"intime": int64(1700000000),
	}
}

func benchBatch() ([]string, [][]any) {
	cols := []string{"name", "age", "score", "mobile", "avatar", "intime"}
	rows := make([][]any, 50)
	for i := range rows {
		rows[i] = []any{"BenchUser", 18, 50.5, "13800000000", []byte("avatar"), int64(1700000000)}
	}
	return cols, rows
}

// 噪声下限：循环体本身的最小开销，任何差异小于它都只能算噪声。
func BenchmarkNoiseFloor(b *testing.B) {
	n := 0
	for i := 0; i < b.N; i++ {
		n += i & 1
	}
	sqlSink = string(rune('a' + n&1))
}

func BenchmarkAssembleSelectSimple(b *testing.B) {
	db := benchDB(b, 0)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sqlSink = db.Table("users").Select("id", "name", "age").SQL()
	}
}

func BenchmarkAssembleSelectFull(b *testing.B) {
	db := benchDB(b, 0)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sqlSink = db.Table("users").
			Select("id", "name", "age", "score").
			LeftJoin("scores", "scores.user_id = users.id").
			Where("age > %d", 18).
			And("score >= %f", 60.0).
			Or("vip = 1").
			GroupBy("shop_id").
			Order("name").
			OrderDESC("id").
			LimitOffset(10, 20).
			Lock().
			SQL()
	}
}

func BenchmarkAssembleWhereFilters(b *testing.B) {
	db := benchDB(b, 0)
	filters := map[string]any{
		"name": "BenchUser", "age": 18, "score": 50.5, "mobile": "13800000000", "status": 1,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sqlSink = db.Table("users").Select("id", "name").AndFilters(filters).SQL()
	}
}

func BenchmarkAssembleInsert(b *testing.B) {
	for _, protocol := range []string{sqlm.MYSQL, sqlm.SQLITE, sqlm.POSTGRES} {
		b.Run(protocol, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				tb, conn := benchTable(protocol)
				_, _ = tb.Insert(benchInsertData())
				sqlSink = conn.last
			}
		})
	}
}

func BenchmarkAssembleInserts(b *testing.B) {
	cols, rows := benchBatch()
	for _, protocol := range []string{sqlm.MYSQL, sqlm.SQLITE, sqlm.POSTGRES} {
		b.Run(protocol, func(b *testing.B) {
			// 每次迭代都用一份全新的列，避免上一轮的修改影响下一轮
			c := append([]string(nil), cols...)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				tb, conn := benchTable(protocol)
				_, _ = tb.Inserts(c, rows)
				sqlSink = conn.last
			}
		})
	}
}

func BenchmarkAssembleUpdate(b *testing.B) {
	for _, protocol := range []string{sqlm.MYSQL, sqlm.SQLITE, sqlm.POSTGRES} {
		b.Run(protocol, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				tb, conn := benchTable(protocol)
				_, _ = tb.Update(map[string]any{"name": "BenchUser", "age": 18}).
					Where("id = %d", 1).
					Execute()
				sqlSink = conn.last
			}
		})
	}
}
