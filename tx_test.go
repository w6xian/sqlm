package sqlm_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/w6xian/sqlm"
)

func TestActionCommits(t *testing.T) {
	db, _ := newSQLite(t, "tx_commit")
	createUsers(t, db)

	n, err := db.Action(func(tx sqlm.ITable, args ...any) (int64, error) {
		affected, err := tx.Table("users").Insert(map[string]any{"name": "TxUser", "age": 21})
		if err != nil {
			return 0, err
		}
		if _, err := tx.Exec("UPDATE mi_users SET age = age + 1 WHERE name = ?", "TxUser"); err != nil {
			return 0, err
		}
		return affected, nil
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)

	row, err := db.Table("users").AndFilters(map[string]any{"name": "TxUser"}).Query()
	require.NoError(t, err)
	assert.Equal(t, int64(22), row.Get("age").NullInt64().Int64)
}

func TestActionRollsBackOnError(t *testing.T) {
	db, _ := newSQLite(t, "tx_rollback")
	createUsers(t, db)

	n, err := db.Action(func(tx sqlm.ITable, args ...any) (int64, error) {
		if _, err := tx.Table("users").Insert(map[string]any{"name": "Ghost"}); err != nil {
			return 0, err
		}
		return 0, errors.New("boom")
	})
	assert.Zero(t, n)
	assert.EqualError(t, err, "boom")

	count, err := db.Table("users").GetCount()
	require.NoError(t, err)
	assert.Equal(t, int64(0), count, "事务内的写入必须回滚")
}

func TestActionRollsBackOnPanic(t *testing.T) {
	db, _ := newSQLite(t, "tx_panic")
	createUsers(t, db)

	assert.Panics(t, func() {
		_, _ = db.Action(func(tx sqlm.ITable, args ...any) (int64, error) {
			if _, err := tx.Table("users").Insert(map[string]any{"name": "Ghost"}); err != nil {
				return 0, err
			}
			panic("boom")
		})
	})

	count, err := db.Table("users").GetCount()
	require.NoError(t, err)
	assert.Equal(t, int64(0), count, "panic发生时也必须回滚")
}

func TestTxIsolationInsideAction(t *testing.T) {
	db, _ := newSQLite(t, "tx_isolation")
	createUsers(t, db)

	// 事务里写入的数据在提交前对外不可见(sqlite默认隔离级别)
	_, err := db.Action(func(tx sqlm.ITable, args ...any) (int64, error) {
		_, err := tx.Table("users").Insert(map[string]any{"name": "Isolated"})
		if err != nil {
			return 0, err
		}
		total, err := tx.Table("users").GetCount()
		require.NoError(t, err)
		assert.Equal(t, int64(1), total, "事务内应能看到自己写入的数据")
		return 0, nil
	})
	require.NoError(t, err)

	total, err := db.Table("users").GetCount()
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
}

func TestTxRawHelpers(t *testing.T) {
	db, _ := newSQLite(t, "tx_raw")
	createUsers(t, db)

	_, err := db.Action(func(tx sqlm.ITable, args ...any) (int64, error) {
		res, err := tx.Exec("INSERT INTO mi_users (name, age) VALUES (?, ?)", "Raw", 5)
		if err != nil {
			return 0, err
		}
		id, err := res.LastInsertId()
		require.NoError(t, err)
		assert.Equal(t, int64(1), id)

		row, err := tx.Query("SELECT id, name FROM mi_users WHERE name = ?", "Raw")
		require.NoError(t, err)
		assert.Equal(t, "Raw", row.Get("name").String())

		rows, err := tx.QueryMulti("SELECT name FROM mi_users")
		require.NoError(t, err)
		assert.Equal(t, 1, rows.Length())

		res, err = tx.Exec("UPDATE mi_users SET age = ? WHERE name = ?", 6, "Raw")
		require.NoError(t, err)
		affected, err := res.RowsAffected()
		require.NoError(t, err)
		assert.Equal(t, int64(1), affected)
		return affected, nil
	})
	require.NoError(t, err)
}

func TestTxScanAndReuse(t *testing.T) {
	db, _ := newSQLite(t, "tx_scan")
	createUsers(t, db)

	_, err := db.Action(func(tx sqlm.ITable, args ...any) (int64, error) {
		_, err := tx.Table("users").Insert(map[string]any{"name": "ScanTx", "age": 30, "score": 1.5})
		if err != nil {
			return 0, err
		}
		u := &User{}
		require.NoError(t, tx.Table("users").AndFilters(map[string]any{"name": "ScanTx"}).Scan(u))
		assert.Equal(t, "ScanTx", u.Name)
		assert.Equal(t, int64(30), u.Age)

		list := []User{}
		require.NoError(t, tx.Table("users").ScanMulti(&list))
		assert.Len(t, list, 1)

		// builder 可以复用多次
		tb := tx.Table("users")
		total, err := tb.GetCount()
		require.NoError(t, err)
		assert.Equal(t, int64(1), total)
		return total, nil
	})
	require.NoError(t, err)
}

func TestTxWithoutConnection(t *testing.T) {
	tx := &sqlm.Tx{}
	_, err := tx.Exec("SELECT 1")
	assert.Error(t, err)
	_, err = tx.Query("SELECT 1")
	assert.Error(t, err)
	_, err = tx.QueryMulti("SELECT 1")
	assert.Error(t, err)
	_, err = tx.Prepare("SELECT 1")
	assert.Error(t, err)
}

func TestTxTableUsesServerPrefix(t *testing.T) {
	db, _ := newSQLite(t, "tx_prefix")
	createUsers(t, db)

	_, err := db.Action(func(tx sqlm.ITable, args ...any) (int64, error) {
		// Tx.Table 必须带上 pretable，否则会写到不存在的表
		_, err := tx.Table("users").Insert(map[string]any{"name": "P"})
		require.NoError(t, err)
		return 1, nil
	})
	require.NoError(t, err)

	count, err := db.Table("users").GetCount()
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}
