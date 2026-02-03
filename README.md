# sqlm

sqlm is a simple, fast, and fluent SQL mapper for Golang. It supports MySQL and SQLite, providing a chainable API for building SQL queries with ease.

## Features

- **Fluent API**: Chainable methods for building queries (`Table`, `Select`, `Where`, `Limit`, etc.).
- **Multiple Drivers**: Built-in support for MySQL and SQLite.
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

```go
err := db.Action(func(tx *sqlm.Tx) error {
    _, err := tx.Table("users").Insert(map[string]any{"name": "Dave"})
    if err != nil {
        return err // Rollback
    }
    return nil // Commit
})
```

## License

MIT
