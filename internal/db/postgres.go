package db

import (
	"database/sql"
	"fmt"
	"time"

	"dbmanager/internal/model"

	_ "github.com/lib/pq"
)

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
	return queryRows(rows)
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

func (d *PostgresDriver) ListTables(dbName string) ([]string, error) {
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
