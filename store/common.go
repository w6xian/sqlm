package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/w6xian/sqlm"
)

const (
	defaultMaxOpenConns = 64
	defaultMaxIdleConns = 8
	defaultMaxLifetime  = int(time.Minute)
)

// ctxOrBackground makes sure a nil context never reaches database/sql.
func ctxOrBackground(ctx context.Context) context.Context {
	if ctx != nil {
		return ctx
	}
	return context.Background()
}

// checkConnection only validates that the pool exists.
//
// Pinging here cost a full round trip on every statement; connectivity errors
// surface on the statement itself and can be probed explicitly with Ping().
func checkConnection(conn *sql.DB) error {
	if conn == nil {
		return errors.New("请设置数据库链接")
	}
	return nil
}

// applyPool configures the pool with sane fallbacks when the configuration is
// left empty. A zero MaxOpenConns means "unlimited" in database/sql which is
// dangerous, hence the library defaults.
func applyPool(conn *sql.DB, conf *sqlm.Server) {
	if conf == nil || conn == nil {
		return
	}
	open := conf.MaxOpenConns
	if open <= 0 {
		open = defaultMaxOpenConns
	}
	idle := conf.MaxIdleConns
	if idle <= 0 {
		idle = defaultMaxIdleConns
	}
	if idle > open {
		idle = open
	}
	lifetime := conf.MaxLifetime
	if lifetime <= 0 {
		lifetime = defaultMaxLifetime
	}
	conn.SetMaxOpenConns(open)
	conn.SetMaxIdleConns(idle)
	conn.SetConnMaxLifetime(time.Duration(lifetime))
}

// quoteColumns back-quotes every column name exactly once.
func quoteColumns(columns []string) []string {
	quoted := make([]string, len(columns))
	for k, v := range columns {
		quoted[k] = "`" + strings.Trim(v, "`") + "`"
	}
	return quoted
}

// insertSQL builds a multi-values INSERT statement.
func insertSQL(pTable string, columns []string, rows int) string {
	holders := "(" + strings.Join(makePlaceholders(len(columns)), ",") + ")"
	values := make([]string, rows)
	for i := range values {
		values[i] = holders
	}
	return fmt.Sprintf("INSERT INTO `%s` (%s) VALUES %s", strings.Trim(pTable, "`"),
		strings.Join(quoteColumns(columns), ","), strings.Join(values, ","))
}

func makePlaceholders(num int) []string {
	if num <= 0 {
		return nil
	}
	out := make([]string, num)
	for i := range out {
		out[i] = "?"
	}
	return out
}

// flattenValues validates and flattens the batch rows into the flat argument
// list expected by the placeholder list.
func flattenValues(colLen int, data [][]any) ([]any, error) {
	if colLen <= 0 {
		return nil, errors.New("请提供字段")
	}
	if len(data) <= 0 {
		return nil, errors.New("请提供数据")
	}
	val := make([]any, 0, len(data)*colLen)
	for _, row := range data {
		if colLen != len(row) {
			return nil, errors.New("请确保column长度统一")
		}
		val = append(val, row...)
	}
	return val, nil
}
