package db

import (
	"database/sql"
	"fmt"
	"time"

	"dbmanager/internal/model"

	_ "modernc.org/sqlite"
)

// SQLiteDriver SQLite驱动
type SQLiteDriver struct {
	db *sql.DB
}

func (d *SQLiteDriver) Connect(conn model.DBConnection) error {
	dbPath := conn.FilePath
	if dbPath == "" {
		dbPath = conn.Database
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return err
	}
	db.SetMaxOpenConns(1)
	d.db = db
	return nil
}

func (d *SQLiteDriver) Close() error {
	if d.db != nil {
		return d.db.Close()
	}
	return nil
}

func (d *SQLiteDriver) Ping() (time.Duration, error) {
	start := time.Now()
	err := d.db.Ping()
	return time.Since(start), err
}

func (d *SQLiteDriver) Query(sql string, _ string) (*model.QueryResult, error) {
	rows, err := d.db.Query(sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

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

func (d *SQLiteDriver) Execute(sql string, _ string) (*model.QueryResult, error) {
	result, err := d.db.Exec(sql)
	if err != nil {
		return nil, err
	}
	rowsAffected, _ := result.RowsAffected()
	return &model.QueryResult{
		Success: true,
		Message: fmt.Sprintf("执行成功，影响%d行", rowsAffected),
		Count:   rowsAffected,
	}, nil
}

func (d *SQLiteDriver) ListTables() ([]string, error) {
	rows, err := d.db.Query(`
		SELECT name FROM sqlite_master 
		WHERE type='table' AND name NOT LIKE 'sqlite_%'
		ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			return nil, err
		}
		tables = append(tables, table)
	}
	return tables, nil
}
