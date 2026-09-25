package db

import (
	"database/sql"
	"fmt"
	"time"

	"dbmanager/internal/model"

	_ "github.com/lib/pq"
)

// PostgresDriver PostgreSQL驱动
type PostgresDriver struct {
	db *sql.DB
}

func (d *PostgresDriver) Connect(conn model.DBConnection) error {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		conn.Host, conn.Port, conn.Username, conn.Password, conn.Database)
	for k, v := range conn.Params {
		dsn += fmt.Sprintf(" %s=%s", k, v)
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return err
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)
	d.db = db
	return nil
}

func (d *PostgresDriver) Close() error {
	if d.db != nil {
		return d.db.Close()
	}
	return nil
}

func (d *PostgresDriver) Ping() (time.Duration, error) {
	start := time.Now()
	err := d.db.Ping()
	return time.Since(start), err
}

func (d *PostgresDriver) Query(sql string, _ string) (*model.QueryResult, error) {
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

func (d *PostgresDriver) Execute(sql string, _ string) (*model.QueryResult, error) {
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

func (d *PostgresDriver) ListTables() ([]string, error) {
	rows, err := d.db.Query(`
		SELECT tablename FROM pg_tables 
		WHERE schemaname = 'public' 
		ORDER BY tablename
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
