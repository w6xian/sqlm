// Command postgres 演示如何用 sqlm 操作 PostgreSQL：
// 建库 -> 建表(JSONB/JSON) -> 写入 -> 读回。
//
// 前置条件：
//  1. PostgreSQL 运行在 127.0.0.1:5432，用户 postgres / 密码见 pgPassword；
//  2. lib/pq 必须以 Server.Protocol 指定的名字注册驱动，两者必须一致：
//     Protocol: "postgres"  <->  database/sql 驱动名 "postgres"。
//
// 运行：
//
//	go run ./examples/postgres
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/w6xian/sqlm"
	"github.com/w6xian/sqlm/store"
)

const (
	pgHost     = "127.0.0.1"
	pgPort     = 5432
	pgUser     = "postgres"
	pgPassword = "1Qazxsw2"
	pgDatabase = "cloud"
)

// products 表结构。sqlm 会按协议生成对应的标识符引用与占位符：
// PostgreSQL 收到的是双引号标识符 + $n 编号占位符。
const createProducts = `
CREATE TABLE IF NOT EXISTS products (
	id SERIAL PRIMARY KEY,
	name VARCHAR(100),
	attributes JSONB,
	metadata JSON
)`

// Product 对应 products 表。JSONB/JSON 列在 Go 侧按文本处理，取出后再反序列化。
type Product struct {
	Id         int64  `json:"id"`
	Name       string `json:"name"`
	Attributes string `json:"attributes"`
	Metadata   string `json:"metadata"`
}

func main() {
	ctx := context.Background()

	// 1. 连到默认的 postgres 维护库，创建 cloud 库
	if err := createDatabase(ctx); err != nil {
		log.Fatalf("创建数据库失败: %v", err)
	}

	// 2. 连到 cloud 库
	db, err := connect(ctx, pgDatabase, "cloud")
	if err != nil {
		log.Fatalf("连接 cloud 失败: %v", err)
	}
	defer db.Close()

	// 3. 建表
	if _, err := db.Exec(createProducts); err != nil {
		log.Fatalf("创建 products 表失败: %v", err)
	}

	// 4. 写入 JSONB 数据并读回
	if err := demo(ctx, db); err != nil {
		log.Fatalf("样例执行失败: %v", err)
	}
}

// connect 注册并返回一个 sqlm 实例。
func connect(ctx context.Context, database, name string) (*sqlm.Db, error) {
	opt, err := sqlm.NewOptionsWithServer(sqlm.Server{
		Protocol:     sqlm.POSTGRES,
		Host:         pgHost,
		Port:         pgPort,
		Username:     pgUser,
		Password:     pgPassword,
		Database:     database,
		Pretable:     "", // products 表没有前缀
		MaxOpenConns: 10,
		MaxIdleConns: 5,
		MaxLifetime:  int(time.Minute),
	}, name)
	if err != nil {
		return nil, err
	}
	drv, err := store.NewDriver(opt)
	if err != nil {
		return nil, err
	}
	sqlm.Use(drv)
	db := sqlm.NewInstance(ctx, name)
	if err := db.Err(); err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	return db, nil
}

// createDatabase 在维护库上创建 cloud。PostgreSQL 的 CREATE DATABASE 不支持
// IF NOT EXISTS，所以先查 pg_database 判断。
func createDatabase(ctx context.Context) error {
	admin, err := connect(ctx, "postgres", "cloud-admin")
	if err != nil {
		return err
	}
	defer admin.Close()

	row, err := admin.Query("SELECT count(*) AS total FROM pg_database WHERE datname = $1", pgDatabase)
	if err != nil {
		return err
	}
	total, err := row.Get("total").Int64()
	if err != nil {
		return err
	}
	if total > 0 {
		fmt.Printf("数据库 %s 已存在，跳过创建\n", pgDatabase)
		return nil
	}
	// 标识符是上面的常量，不来自外部输入，所以这里直接拼接是安全的
	if _, err := admin.Exec("CREATE DATABASE " + pgDatabase); err != nil {
		return err
	}
	fmt.Printf("数据库 %s 创建成功\n", pgDatabase)
	return nil
}

func demo(ctx context.Context, db *sqlm.Db) error {
	// 让样例可以反复运行：先清掉上一轮写入的数据
	if _, err := db.Table("products").Delete().AndFilters(map[string]any{"name": "Laptop"}).Execute(); err != nil {
		return fmt.Errorf("清理旧数据失败: %w", err)
	}

	// 写：PostgreSQL 没有 LastInsertId，sqlm 在这种协议下返回受影响行数
	affected, err := db.Table("products").Insert(map[string]any{
		"name":       "Laptop",
		"attributes": `{"brand": "Dell", "cpu": "Intel i7", "ram": "16GB", "storage": "512GB SSD"}`,
		"metadata":   `{"warranty": "2 years", "color": "black"}`,
	})
	if err != nil {
		return fmt.Errorf("写入 products 失败: %w", err)
	}
	fmt.Println("写入行数:", affected)

	// 读一：结构化扫描，按 id 倒序取最近 10 条
	var list []Product
	rows, err := db.Table("products").
		Select("id,name,attributes,metadata").
		OrderDESC("id").
		Limit(10).
		QueryMulti()
	if err != nil {
		return fmt.Errorf("查询 products 失败: %w", err)
	}
	if err := rows.ScanMulti(&list); err != nil {
		return fmt.Errorf("扫描 products 失败: %w", err)
	}
	for _, p := range list {
		fmt.Printf("id=%d name=%s\n", p.Id, p.Name)
		printJSON("  attributes", p.Attributes)
		printJSON("  metadata  ", p.Metadata)
	}

	// 读二：JSONB 字段条件（->> 取文本再比较）
	hit, err := db.Table("products").
		Select("id,name").
		Where("attributes->>'brand' = 'Dell'").
		QueryMulti()
	if err != nil {
		return fmt.Errorf("按 JSONB 条件查询失败: %w", err)
	}
	fmt.Println("JSONB 条件命中条数:", hit.Length())

	// 读三：聚合
	total, err := db.Table("products").GetCount()
	if err != nil {
		return fmt.Errorf("统计行数失败: %w", err)
	}
	fmt.Println("products 总行数:", total)

	// 事务演示：写一条再整体回滚
	return rollbackDemo(db)
}

// rollbackDemo 说明事务内的写入可以被完整撤销。
func rollbackDemo(db *sqlm.Db) error {
	before, err := db.Table("products").GetCount()
	if err != nil {
		return err
	}
	// 回调返回错误 -> 事务回滚
	_, err = db.Action(func(tx sqlm.ITable, args ...any) (int64, error) {
		n, err := tx.Table("products").Insert(map[string]any{
			"name":       "Rollbacked",
			"attributes": `{"brand": "None"}`,
			"metadata":   `{}`,
		})
		if err != nil {
			return 0, err
		}
		return n, fmt.Errorf("故意失败，触发回滚")
	})
	if err == nil {
		return fmt.Errorf("事务应当失败")
	}
	after, err := db.Table("products").GetCount()
	if err != nil {
		return err
	}
	fmt.Printf("事务回滚: 写入前 %d 行，回滚后 %d 行\n", before, after)
	return nil
}

func printJSON(label, raw string) {
	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		fmt.Printf("%s: 解析失败(%v): %s\n", label, err, raw)
		return
	}
	pretty, _ := json.Marshal(out)
	fmt.Printf("%s: %s\n", label, pretty)
}
