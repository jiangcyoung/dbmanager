package db

import (
	"database/sql"

	"dbmanager/internal/model"
)

func queryRows(rows *sql.Rows) (*model.QueryResult, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var result []interface{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range columns {
			valuePtrs[i] = &values[i]
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}
		row := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			if b, ok := val.([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = val
			}
		}
		result = append(result, row)
	}
	return &model.QueryResult{
		Success: true,
		Columns: columns,
		Rows:    result,
		Count:   int64(len(result)),
	}, nil
}
