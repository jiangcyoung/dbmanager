package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

var (
	instance *Store
	once     sync.Once
)

func Init(dataDir string) (*Store, error) {
	var initErr error
	once.Do(func() {
		if err := os.MkdirAll(dataDir, 0755); err != nil {
			initErr = fmt.Errorf("创建数据目录失败: %w", err)
			return
		}

		dbPath := filepath.Join(dataDir, "dbmanager.db")
		db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
		if err != nil {
			initErr = fmt.Errorf("打开数据库失败: %w", err)
			return
		}

		db.SetMaxOpenConns(1)
		db.SetMaxIdleConns(1)

		if err := createTables(db); err != nil {
			db.Close()
			initErr = fmt.Errorf("初始化表结构失败: %w", err)
			return
		}

		instance = &Store{db: db}
	})
	return instance, initErr
}

func Get() *Store {
	return instance
}

func (s *Store) DB() *sql.DB {
	return s.db
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func createTables(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			username TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'user',
			status TEXT NOT NULL DEFAULT 'pending',
			created_at DATETIME NOT NULL,
			last_login_at DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS connections (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			type TEXT NOT NULL,
			host TEXT,
			port INTEGER,
			username TEXT,
			password TEXT,
			database_name TEXT,
			file_path TEXT,
			db_index INTEGER DEFAULT 0,
			params_json TEXT,
			created_at DATETIME NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS audit_logs (
			id TEXT PRIMARY KEY,
			time DATETIME NOT NULL,
			username TEXT,
			role TEXT,
			action TEXT,
			resource TEXT,
			detail TEXT,
			ip TEXT,
			user_agent TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_username ON audit_logs(username)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_action ON audit_logs(action)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_time ON audit_logs(time DESC)`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("执行 %q: %w", stmt[:40], err)
		}
	}
	return nil
}
