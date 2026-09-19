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
