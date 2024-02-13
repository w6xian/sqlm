package sqlm

import (
	"database/sql"
)

func GetRows(rows *sql.Rows) (*Rows, error) {
	columns, err := rows.Columns()
	if err == nil {
		collen := len(columns)
		__c := make([]interface{}, collen)
		var _rows *Rows = NewSqlxRows()
		for rows.Next() {
			_c := make([][]byte, collen)
			for i := 0; i < collen; i++ {
				__c[i] = &_c[i]
			}
			rows.Scan(__c...)
			_rows.Append(Row{Data: _c, ColumnName: columns, ColumnLen: collen})
		}
		if _rows.Length() == 0 {
			return nil, ErrNotFound
		}
		return _rows, nil
	}
	return nil, err
}

func GetRow(rows *sql.Rows) (*Row, error) {
	columns, err := rows.Columns()
	if err == nil {
		// 有拿到
		if rows.Next() {
			collen := len(columns)
			__c := make([]interface{}, collen)
			_c := make([][]byte, collen)
			for i := 0; i < collen; i++ {
				__c[i] = &_c[i]
			}
			rows.Scan(__c...)
			return &Row{Data: _c, ColumnName: columns, ColumnLen: collen}, nil
		}

		return nil, ErrNotFound
	}
	return nil, err
}
