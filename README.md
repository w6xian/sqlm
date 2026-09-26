# sqlm

sqlm is a simple, fast, and fluent SQL mapper for Golang. It supports MySQL, PostgreSQL and SQLite, providing a chainable API for building SQL queries with ease.

## Features

- **Fluent API**: Chainable methods for building queries (`Table`, `Select`, `Where`, `Limit`, etc.).
- **Multiple Drivers**: Built-in support for MySQL, PostgreSQL and SQLite.
- **Connection Pooling**: Efficiently reuses database connections.
- **Dynamic Filtering**: Easily build complex `WHERE` clauses using `AndFilters` with maps.
- **Transactions**: Simple transaction management.
- **Thread Safe**: Designed for concurrent use.

## Installation

```bash
go get github.com/w6xian/sqlm
```

## Usage

### Initialization

Initialize the database connection using `NewOptionsWithServer` and `NewDriver`.

```go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/w6xian/sqlm"
	"github.com/w6xian/sqlm/store"
)

func main() {
	// Configure MySQL connection
	opt, err := sqlm.NewOptionsWithServer(sqlm.Server{
		Protocol:     "mysql",
		Host:         "127.0.0.1",
		Port:         3306,
		Username:     "root",
		Password:     "password",
		Database:     "test_db",
		Charset:      "utf8mb4",
		MaxOpenConns: 10,
		MaxIdleConns: 5,
		MaxLifetime:  int(time.Minute),
	})
	if err != nil {
		panic(err)
	}

	// Create driver
	driver, err := store.NewDriver(opt)
	if err != nil {
		panic(err)
	}

	// Register driver
	sqlm.Use(driver)

	// Create a new instance for operations
	db := sqlm.NewInstance(context.Background(), "def")
	defer db.Close()
}
```

### PostgreSQL

PostgreSQL has no `LastInsertId`, identifier quoting is standard double quotes and parameters are numbered, so sqlm adapts those three points automatically:

- quoted identifiers: `INSERT INTO "products" ("name") VALUES ($1)`
- `Limit`/`LimitOffset` emit `LIMIT .. OFFSET ..` instead of MySQL's `LIMIT m,n`
- `Insert`/`Inserts` return the number of affected rows

The driver has to be registered under the same name as `Server.Protocol`, because that value is used as the `database/sql` driver name:

```go
import (

	"github.com/w6xian/sqlm"
	"github.com/w6xian/sqlm/store"
)

opt, _ := sqlm.NewOptionsWithServer(sqlm.Server{
	Protocol:     sqlm.POSTGRES, // or "pg"
	Host:         "127.0.0.1",
	Port:         5432,
	Username:     "postgres",
	Password:     "secret",
	Database:     "cloud",
	MaxOpenConns: 10,
	MaxIdleConns: 5,
	MaxLifetime:  int(time.Minute),
}, "cloud")
drv, err := store.NewDriver(opt)
sqlm.Use(drv)
db := sqlm.NewInstance(context.Background(), "cloud")
```

A full JSONB example (create database, create table, write and read back) lives in [examples/postgres](examples/postgres/main.go).

### Insert

```go
// Insert a single record
id, err := db.Table("users").Insert(map[string]any{
    "name": "Alice",
    "age":  30,
})

// Insert multiple records
columns := []string{"name", "age"}
values := [][]any{
    {"Bob", 25},
    {"Charlie", 35},
}
count, err := db.Table("users").Inserts(columns, values)
```

### Query (Select)

```go
// Simple Select
rows, err := db.Table("users").Select("name", "age").Where("id = %d", 1).Query()
if err != nil {
    // handle error
}
// Get value from first row
name := rows.Get("name").String()

// Select Multiple Rows
allRows, err := db.Table("users").Select("*").Limit(10).QueryMulti()
// Scan into struct slice
var users []User
err = allRows.ScanMulti(&users)
```

### Update

```go
// Update records
affected, err := db.Table("users").
    Update(map[string]any{"age": 31}).
    Where("name = '%s'", "Alice").
    Execute()
```

### Delete

```go
// Delete records
affected, err := db.Table("users").
    Delete().
    Where("id = %d", 1).
    Execute()
```

### Advanced Filtering

Use `AndFilters` for dynamic conditions from a map:

```go
filters := map[string]any{
    "age": 30,
    "status": "active",
}
rows, err := db.Table("users").Select("*").AndFilters(filters).Query()
```

Chain conditions:

```go
db.Table("users").
    Where("age > %d", 18).
    And("status = '%s'", "active").
    Or("role = '%s'", "admin").
    Query()
```

**Note**: `Where`, `And`, and `Or` methods use `fmt.Sprintf` style formatting (e.g., `%d`, `%s`). Please ensure inputs are sanitized if they come from untrusted sources, or use `AndFilters` which handles values safely.

### SQL assembly notes

- Identifiers are quoted per engine: backticks for MySQL, standard double quotes for PostgreSQL/SQLite. Placeholders follow the engine too (`?` vs numbered `$1, $2, ...`, numbered continuously across multi-row inserts).
- `Inserts` never modifies the `columns` slice you pass in, so the same slice can be reused (even across engines).
- `Set("discount = '50%'")` keeps the expression as written; it is only run through `fmt.Sprintf` when you pass extra arguments.
- NUL bytes are never written into a literal: MySQL gets `\0`, PostgreSQL/SQLite drop them (a raw NUL would truncate a SQLite statement or be rejected by PostgreSQL).
- `Limit`/`LimitOffset` emit `LIMIT .. OFFSET ..` on PostgreSQL/SQLite and MySQL's `LIMIT m,n` on MySQL.

### Observability hooks

Every statement can be observed — tracing, slow query logs, metrics — without
touching the call sites. A hook is a read only observer: it cannot change the
statement and its result is ignored.

```go
opt.SetHooks(sqlm.HookFunc(func(ctx context.Context, info *sqlm.StmtInfo) {
    if info.Duration > 200*time.Millisecond {
        log.Warn("slow query", "op", info.Op, "table", info.Table, "digest", info.Digest)
    }
}))
```

`StmtInfo` carries `Op` (`select`/`insert`/`update`/`delete`/`exec`), the
un-prefixed `Table`, `Rows`, `RowsKnown`, `Duration`, `StartAt` and `Err`.
`StartAt` + `Duration` let OpenTelemetry replay a span with
`trace.WithTimestamp`, so a single callback is enough — no Start/End pair:

```go
_, span := tracer.Start(ctx, "db."+info.Op, trace.WithTimestamp(info.StartAt))
defer span.End(trace.WithTimestamp(info.StartAt.Add(info.Duration)))
```

Three registration scopes, all optional:

| Scope | Entry |
| --- | --- |
| every instance created afterwards | `sqlm.WithHooks(h)` / `opt.SetHooks(h)` |
| one instance | `db.SetHooks(h)` |
| one statement | `db.Table("users").UseHook(h).QueryMulti()` |

Notes that matter:

- **`Digest`, not the raw statement.** sqlm builds SQL by concatenation, so the
  statement *is* business data. `Digest` replaces literals with `?` and is the
  only shape safe for span names and metric labels (a raw statement would also
  blow up cardinality). Use `sqlm.NewSQLHook(limit, fn)` when you really need
  the full text — it is opt in and truncated.
- **Request context.** `db.Table(...)` carries the instance context. To get the
  request one (trace id, deadline) use `db.TableWithContext(ctx, "users")` or
  `Table.WithContext(ctx)`.
- **`RowsKnown`.** `Table.Rows()`/`Db.Rows()` hand out a raw cursor: the row
  count is unknown and reported as `false` instead of a fake `0`.
- **Hooks never break the statement.** A panicking hook is recovered; no hook
  registered means no timing, no digest, no allocation on the hot path.

`Db.SetHooks` replaces everything the instance inherited from the options, so
re-add shared hooks (`db.SetHooks(metrics, spans)`) if you still want them.

Runnable sample: `go run ./examples/hook`.

### Transactions

Returning an error (or panicking) rolls the transaction back.

```go
_, err := db.Action(func(tx sqlm.ITable, args ...any) (int64, error) {
    n, err := tx.Table("users").Insert(map[string]any{"name": "Dave"})
    if err != nil {
        return 0, err // Rollback
    }
    return n, nil // Commit
})
```

## License

MIT
