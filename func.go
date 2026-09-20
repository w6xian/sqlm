package sqlm

import (
	"database/sql"
)

// buildColumnIndex maps column names to their position. Rows produced by the
// same result set share the map (read only) instead of rebuilding it.
func buildColumnIndex(columns []string) map[string]int {
	if len(columns) == 0 {
		return nil
	}
	idx := make(map[string]int, len(columns))
	for i, name := range columns {
		if _, ok := idx[name]; !ok {
			idx[name] = i
		}
	}
	return idx
}

// GetRows drains rows into a Rows and always closes the source rows.
// It takes the ownership of rows: never close them twice yourself.
func GetRows(rows *sql.Rows) (*Rows, error) {
	if rows == nil {
		return nil, ErrNilArgument
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	collen := len(columns)
	idx := buildColumnIndex(columns)
	_rows := NewSqlxRows()
	// Scan destinations are reused: only the row payload is reallocated.
	scanArgs := make([]any, collen)
	for rows.Next() {
		data := make([][]byte, collen)
		for i := 0; i < collen; i++ {
			scanArgs[i] = &data[i]
		}
		if err := rows.Scan(scanArgs...); err != nil {
			return nil, err
		}
		_rows.Append(Row{Data: data, ColumnName: columns, ColumnLen: collen, idx: idx})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if _rows.Length() == 0 {
		return nil, ErrNotFound
	}
	return _rows, nil
}

// GetRow takes the first row of rows and always closes the source rows.
// It takes the ownership of rows: never close them twice yourself.
func GetRow(rows *sql.Rows) (*Row, error) {
	if rows == nil {
		return nil, ErrNilArgument
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	// 有拿到
	if rows.Next() {
		collen := len(columns)
		scanArgs := make([]any, collen)
		data := make([][]byte, collen)
		for i := 0; i < collen; i++ {
			scanArgs[i] = &data[i]
		}
		if err := rows.Scan(scanArgs...); err != nil {
			return nil, err
		}
		return &Row{Data: data, ColumnName: columns, ColumnLen: collen, idx: buildColumnIndex(columns)}, nil
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return nil, ErrNotFound
}
