package db

import (
	"database/sql"
	"fmt"
	"time"

	"dbmanager/internal/model"

	_ "modernc.org/sqlite"
)

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
	return queryRows(rows)
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

func (d *SQLiteDriver) ListTables(_ string) ([]string, error) {
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
